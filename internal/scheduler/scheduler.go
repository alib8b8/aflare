// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌‌​‌‌‌​​​‌‌‌​​​​​​​‌​​‌‌‌​‌‌‌​‌​​‌‌​‌​​‌‌‌‌​​​​​​​​​​​​​​​​​​​​‌‌‌‌​​​‌​‌‌​​​‌⁠
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
//
// TaskFunc cannot report failure (no error return), so tasks registered via
// AddTask are executed once per scheduled time with no retry. Prefer
// AddRetryingTask with a RetryTaskFunc when failures should be retried.
type TaskFunc func(context.Context)

// RetryTaskFunc is a task body that reports failure. A non-nil error (or a
// panic) triggers the task's RetryPolicy; context cancellation aborts the
// retry loop without counting as a failure.
type RetryTaskFunc func(context.Context) error

// RetryPolicy controls automatic retries of a failing scheduled task.
// MaxRetries is the number of retries AFTER the initial attempt
// (0 = no retry, matching the historical execute-once behaviour).
// Delay is the base backoff delay; retries use Delay, 2×Delay, 4×Delay, ...
// capped at MaxTaskRetryDelay.
type RetryPolicy struct {
	MaxRetries int
	Delay      time.Duration
}

// Bounds for RetryPolicy. MaxTaskRetries mirrors the workflow executor's
// step-retry cap; MaxTaskRetryDelay keeps a wedged task from blocking
// scheduler shutdown for unbounded time.
const (
	MaxTaskRetries    = 10
	MaxTaskRetryDelay = 5 * time.Minute

	// DefaultTaskRetryDelay is used when RetryPolicy.Delay is unset.
	DefaultTaskRetryDelay = 30 * time.Second
)

type Task struct {
	ID   string
	Expr string
	Func TaskFunc
	// ErrFunc, when set, takes precedence over Func and enables retry on
	// failure per Retry.
	ErrFunc RetryTaskFunc
	Retry   RetryPolicy

	schedule *cronSchedule
	nextRun  time.Time
	// lastRun is the most recent fire time: set by the tick loop on every
	// dispatch, or restored from the persisted last-run store on startup.
	lastRun time.Time
	// missedRuns counts scheduled fire times that were skipped between the
	// restored lastRun and process start (daemon downtime). Marked, never
	// replayed — see RestoreLastRun.
	missedRuns int
}

// TaskInfo is a read-only snapshot of a scheduled task, exposed via ListTasks.
type TaskInfo struct {
	ID      string
	Expr    string
	NextRun time.Time
	// LastRun is the most recent fire time restored from or recorded by
	// this process (zero when the task has never fired).
	LastRun time.Time
	// MissedRuns counts scheduled fire times between LastRun and process
	// start that were skipped (daemon downtime). Marked, not replayed.
	MissedRuns int
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
	// onFire, when set, is invoked (outside the scheduler lock) each time
	// a task is dispatched, with the fire timestamp. Callers persist it as
	// the task's last-run so the next restart can detect missed runs.
	onFire func(taskID string, firedAt time.Time)
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

// AddRetryingTask registers a scheduled task whose failures are automatically
// retried per policy. MaxRetries is clamped to [0, MaxTaskRetries]; a
// non-positive Delay falls back to DefaultTaskRetryDelay.
func (s *Scheduler) AddRetryingTask(id string, expr string, fn RetryTaskFunc, policy RetryPolicy) error {
	if fn == nil {
		return fmt.Errorf("task %q: retry task function must not be nil", id)
	}
	if policy.MaxRetries < 0 {
		policy.MaxRetries = 0
	}
	if policy.MaxRetries > MaxTaskRetries {
		policy.MaxRetries = MaxTaskRetries
	}
	if policy.Delay <= 0 {
		policy.Delay = DefaultTaskRetryDelay
	}

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
	s.tasks[id] = &Task{
		ID:       id,
		Expr:     expr,
		ErrFunc:  fn,
		Retry:    policy,
		schedule: schedule,
		nextRun:  nextRunTime(schedule, now),
	}
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
			ID:         task.ID,
			Expr:       task.Expr,
			NextRun:    task.nextRun,
			LastRun:    task.lastRun,
			MissedRuns: task.missedRuns,
		})
	}
	sortTasksByID(tasks)
	return tasks
}

// SetOnFire registers a callback invoked each time a task is dispatched.
// Set it before Start to observe every fire. The callback runs on the tick
// loop goroutine (outside the scheduler lock) and must not call back into
// the Scheduler.
func (s *Scheduler) SetOnFire(fn func(taskID string, firedAt time.Time)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onFire = fn
}

// MaxMissedRunCount caps how many missed fire times are counted for a
// single task after downtime. A per-minute task that was down for weeks
// would otherwise iterate millions of cron slots; beyond "many" the exact
// count is not actionable.
const MaxMissedRunCount = 1000

// RestoreLastRun restores a task's last fire time (typically loaded from
// the persisted last-run store) and counts how many scheduled fire times
// were missed between then and now. Missed runs are marked (visible via
// ListTasks and a warning log) but NOT replayed — honest semantics for a
// scheduler that must not silently pretend downtime never happened, nor
// flood the agent with a backlog of stale triggers.
func (s *Scheduler) RestoreLastRun(id string, lastRun time.Time) error {
	if lastRun.IsZero() {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task with id %q not found", id)
	}

	missed := countMissedRuns(task.schedule, lastRun, time.Now())
	task.lastRun = lastRun
	task.missedRuns = missed
	if missed > 0 {
		logger.Warn("scheduled task missed runs during downtime (marked, not replayed)",
			"task_id", id,
			"cron", task.Expr,
			"missed_runs", missed,
			"last_run", lastRun.Format(time.RFC3339),
		)
	}
	return nil
}

// countMissedRuns counts scheduled fire times strictly between lastRun and
// now, capped at MaxMissedRunCount.
func countMissedRuns(schedule *cronSchedule, lastRun, now time.Time) int {
	if schedule == nil || !lastRun.Before(now) {
		return 0
	}
	count := 0
	t := lastRun
	for count < MaxMissedRunCount {
		nxt := nextRunTime(schedule, t)
		if nxt.IsZero() || !nxt.Before(now) {
			break
		}
		count++
		t = nxt
	}
	return count
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
	onFire := s.onFire
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
			if t.ErrFunc != nil {
				s.runTaskWithRetry(taskCtx, t)
				return
			}
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
			t.lastRun = now
		}
		s.mu.Unlock()
		// Persist the fire time (outside the lock): recording at dispatch —
		// not at completion — means a crash mid-task is still counted as
		// "fired", so a restart never double-triggers it.
		if onFire != nil {
			onFire(task.ID, now)
		}
	}
}

// runTaskWithRetry executes t.ErrFunc and retries failures per t.Retry with
// exponential backoff. Panics are converted to errors so a crashing task body
// is retried like any other failure. Context cancellation aborts the loop:
// the pending attempt is abandoned and NOT counted as a failure, because
// shutdown is not the task's fault.
func (s *Scheduler) runTaskWithRetry(ctx context.Context, t *Task) {
	for attempt := 0; ; attempt++ {
		err := callTaskFunc(ctx, t)
		if err == nil {
			return
		}

		if attempt >= t.Retry.MaxRetries {
			logger.Error("scheduled task failed permanently",
				"task_id", t.ID,
				"cron", t.Expr,
				"attempts", attempt+1,
				"error", err,
			)
			return
		}

		delay := retryDelay(t.Retry.Delay, attempt)
		logger.Warn("scheduled task failed, retrying",
			"task_id", t.ID,
			"cron", t.Expr,
			"attempt", attempt+1,
			"max_attempts", t.Retry.MaxRetries+1,
			"error", err,
			"retry_in", delay.String(),
		)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// callTaskFunc invokes t.ErrFunc, converting a panic into an error (with the
// stack in the message) so the retry loop can treat it as a retryable
// failure instead of losing the task silently.
func callTaskFunc(ctx context.Context, t *Task) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("task panicked: %v\n%s", r, debug.Stack())
		}
	}()
	return t.ErrFunc(ctx)
}

// retryDelay returns the backoff before retry number `attempt` (0-based):
// delay × 2^attempt, capped at MaxTaskRetryDelay.
func retryDelay(base time.Duration, attempt int) time.Duration {
	d := base
	for i := 0; i < attempt && d < MaxTaskRetryDelay; i++ {
		d *= 2
	}
	if d > MaxTaskRetryDelay {
		return MaxTaskRetryDelay
	}
	return d
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

	switch {
	case part == "*":
		start = min
		end = max
	case strings.Contains(part, "-"):
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
	default:
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
