package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TaskFunc func()

type Task struct {
	ID       string
	Expr     string
	Func     TaskFunc
	schedule *cronSchedule
	nextRun  time.Time
}

type Scheduler struct {
	tasks   map[string]*Task
	mu      sync.RWMutex
	running bool
	stop    chan struct{}
	done    chan struct{}
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

func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
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
	s.mu.Unlock()

	<-s.done
}

func (s *Scheduler) run() {
	defer close(s.done)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case now := <-ticker.C:
			s.checkAndRunTasks(now)
		}
	}
}

func (s *Scheduler) checkAndRunTasks(now time.Time) {
	s.mu.RLock()
	var tasksToRun []*Task
	for _, task := range s.tasks {
		if !task.nextRun.After(now) {
			tasksToRun = append(tasksToRun, task)
		}
	}
	s.mu.RUnlock()

	for _, task := range tasksToRun {
		go task.Func()
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
