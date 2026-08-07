// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package scheduler

import (
	"context"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
)

// TaskFunc is the body of a scheduled task. The context is cancelled when the
// scheduler stops, so long-running tasks can abort promptly on shutdown
// rather than blocking Stop() indefinitely.
type TaskFunc func(context.Context)

type Task struct {
	ID       string
	Expr     string
	Func     TaskFunc
	schedule *cronSchedule
	nextRun  time.Time
}

// TaskInfo is a read-only snapshot of a scheduled task, exposed via ListTasks.
type TaskInfo struct {
	ID      string
	Expr    string
	NextRun time.Time
}

type Scheduler struct {
	tasks   map[string]*Task
	mu      sync.RWMutex
	running bool
	stop    chan struct{}
	done    chan struct{}
	// taskWg tracks in-flight task goroutines so Stop() can wait for them
	// (and, via taskCancel, signal them to abort) instead of leaving them
	// running after the scheduler has supposedly shut down.
	taskWg     sync.WaitGroup
	taskCtx    context.Context
	taskCancel context.CancelFunc
}

type cronSchedule struct {
	minute     []int
	hour       []int
	dayOfMonth []int
	month      []int
	dayOfWeek  []int
}

func New() *Scheduler {
	return &Scheduler{
		tasks: make(map[string]*Task),
	}
}

func (s *Scheduler) AddTask(id string, expr string, fn TaskFunc) error {
	schedule, err := parseCronExpr(expr)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; exists {
		return fmt.Errorf("task with id %q already exists", id)
	}

	now := time.Now()
	task := &Task{
		ID:       id,
		Expr:     expr,
		Func:     fn,
		schedule: schedule,
		nextRun:  nextRunTime(schedule, now),
	}
	s.tasks[id] = task
	return nil
}

func (s *Scheduler) RemoveTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; !exists {
		return fmt.Errorf("task with id %q not found", id)
	}

	delete(s.tasks, id)
	return nil
}

// ListTasks returns a snapshot of all registered tasks sorted by ID.
// It is safe for concurrent use.
func (s *Scheduler) ListTasks() []TaskInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]TaskInfo, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, TaskInfo{
			ID:      task.ID,
			Expr:    task.Expr,
			NextRun: task.nextRun,
		})
	}
	sortTasksByID(tasks)
	return tasks
}

func sortTasksByID(tasks []TaskInfo) {
	for i := 1; i < len(tasks); i++ {
		for j := i; j > 0 && tasks[j-1].ID > tasks[j].ID; j-- {
			tasks[j-1], tasks[j] = tasks[j], tasks[j-1]
		}
	}
}

func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.taskCtx, s.taskCancel = context.WithCancel(context.Background())
	s.mu.Unlock()

	go s.run()
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stop)
	taskCancel := s.taskCancel
	s.mu.Unlock()

	// Wait for the main tick loop to exit, then cancel the task context so
	// in-flight tasks observe shutdown, and finally wait for those task
	// goroutines to actually return. Ordering matters: cancel after <-s.done
	// so no new task can be spawned (checkAndRunTasks only runs inside run)
	// while we wait on taskWg.
	<-s.done
	taskCancel()
	s.taskWg.Wait()
}

func (s *Scheduler) run() {
	defer close(s.done)

	// taskCtx is written once in Start() before this goroutine is launched,
	// so reading it here is race-free (goroutine creation establishes
	// happens-before). Capturing it locally also keeps the hot loop off the
	// shared field.
	taskCtx := s.taskCtx

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case now := <-ticker.C:
			s.checkAndRunTasks(now, taskCtx)
		}
	}
}

func (s *Scheduler) checkAndRunTasks(now time.Time, taskCtx context.Context) {
	s.mu.RLock()
	var tasksToRun []*Task
	for _, task := range s.tasks {
		if !task.nextRun.After(now) {
			tasksToRun = append(tasksToRun, task)
		}
	}
	s.mu.RUnlock()

	for _, task := range tasksToRun {
		// Track each task goroutine so Stop() can wait for in-flight work
		// and cancel it via taskCtx. Add before the goroutine starts so the
		// Done() is always paired with an Add() (no race where Stop sees a
		// zero counter and returns early).
		s.taskWg.Add(1)
		go func(t *Task) {
			defer s.taskWg.Done()
			defer func() {
				if r := recover(); r != nil {
					logger.Error("scheduled task panicked",
						"task_id", t.ID,
						"cron", t.Expr,
						"panic", r,
						"stack", string(debug.Stack()),
					)
				}
			}()
			t.Func(taskCtx)
		}(task)
		s.mu.Lock()
		if t, ok := s.tasks[task.ID]; ok {
			t.nextRun = nextRunTime(t.schedule, now.Add(time.Minute))
		}
		s.mu.Unlock()
	}
}

func parseCronExpr(expr string) (*cronSchedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d", len(fields))
	}

	minute, err := parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute field: %w", err)
	}

	hour, err := parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour field: %w", err)
	}

	dayOfMonth, err := parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("day of month field: %w", err)
	}

	month, err := parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month field: %w", err)
	}

	dayOfWeek, err := parseField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("day of week field: %w", err)
	}

	return &cronSchedule{
		minute:     minute,
		hour:       hour,
		dayOfMonth: dayOfMonth,
		month:      month,
		dayOfWeek:  dayOfWeek,
	}, nil
}

func parseField(field string, min, max int) ([]int, error) {
	var result []int
	seen := make(map[int]bool)

	parts := strings.Split(field, ",")
	for _, part := range parts {
		values, err := parsePart(part, min, max)
		if err != nil {
			return nil, err
		}
		for _, v := range values {
			if !seen[v] {
				seen[v] = true
				result = append(result, v)
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid values in field %q", field)
	}

	return result, nil
}

func parsePart(part string, min, max int) ([]int, error) {
	var start, end, step int
	step = 1

	if strings.Contains(part, "/") {
		stepParts := strings.SplitN(part, "/", 2)
		part = stepParts[0]
		var err error
		step, err = strconv.Atoi(stepParts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid step value %q", stepParts[1])
		}
		if step <= 0 {
			return nil, fmt.Errorf("step must be positive, got %d", step)
		}
	}

	if part == "*" {
		start = min
		end = max
	} else if strings.Contains(part, "-") {
		rangeParts := strings.SplitN(part, "-", 2)
		var err error
		start, err = strconv.Atoi(rangeParts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid range start %q", rangeParts[0])
		}
		end, err = strconv.Atoi(rangeParts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid range end %q", rangeParts[1])
		}
	} else {
		val, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid value %q", part)
		}
		start = val
		end = val
	}

	if start < min || start > max {
		return nil, fmt.Errorf("value %d out of range [%d, %d]", start, min, max)
	}
	if end < min || end > max {
		return nil, fmt.Errorf("value %d out of range [%d, %d]", end, min, max)
	}
	if start > end {
		return nil, fmt.Errorf("start %d greater than end %d", start, end)
	}

	var result []int
	for i := start; i <= end; i += step {
		result = append(result, i)
	}

	return result, nil
}

func nextRunTime(schedule *cronSchedule, from time.Time) time.Time {
	t := from.Truncate(time.Minute).Add(time.Minute)

	for year := t.Year(); year <= t.Year()+5; year++ {
		for month := int(t.Month()); month <= 12; month++ {
			if !contains(schedule.month, month) {
				continue
			}
			for day := t.Day(); day <= daysInMonth(year, time.Month(month)); day++ {
				if !contains(schedule.dayOfMonth, day) {
					continue
				}
				date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, t.Location())
				if !contains(schedule.dayOfWeek, int(date.Weekday())) {
					continue
				}
				for hour := t.Hour(); hour <= 23; hour++ {
					if !contains(schedule.hour, hour) {
						continue
					}
					for minute := t.Minute(); minute <= 59; minute++ {
						if contains(schedule.minute, minute) {
							return time.Date(year, time.Month(month), day, hour, minute, 0, 0, t.Location())
						}
					}
					t = time.Date(year, time.Month(month), day, 0, 0, 0, 0, t.Location())
				}
				t = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, t.Location())
			}
			t = time.Date(year, time.Month(1), 1, 0, 0, 0, 0, t.Location())
		}
		t = time.Date(year+1, time.Month(1), 1, 0, 0, 0, 0, t.Location())
	}

	return time.Time{}
}

func contains(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
