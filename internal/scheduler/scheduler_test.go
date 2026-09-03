// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌​​‌​​​‌‌​‌​‌‌​​​​​​​‌​​‌​‌‌​‌​​‌‌‌​​‌‌‌​​​​‌‌​​​​​​​​​​​​​​​​​‌​​​​​​​​​‌​‌‌‌​⁠
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
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseCronExpr_AllWildcards(t *testing.T) {
	schedule, err := parseCronExpr("* * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schedule.minute) != 60 {
		t.Errorf("expected 60 minutes, got %d", len(schedule.minute))
	}
	if len(schedule.hour) != 24 {
		t.Errorf("expected 24 hours, got %d", len(schedule.hour))
	}
	if len(schedule.dayOfMonth) != 31 {
		t.Errorf("expected 31 days, got %d", len(schedule.dayOfMonth))
	}
	if len(schedule.month) != 12 {
		t.Errorf("expected 12 months, got %d", len(schedule.month))
	}
	if len(schedule.dayOfWeek) != 7 {
		t.Errorf("expected 7 days of week, got %d", len(schedule.dayOfWeek))
	}
}

func TestParseCronExpr_SingleValue(t *testing.T) {
	schedule, err := parseCronExpr("30 14 15 6 3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schedule.minute) != 1 || schedule.minute[0] != 30 {
		t.Errorf("expected minute 30, got %v", schedule.minute)
	}
	if len(schedule.hour) != 1 || schedule.hour[0] != 14 {
		t.Errorf("expected hour 14, got %v", schedule.hour)
	}
	if len(schedule.dayOfMonth) != 1 || schedule.dayOfMonth[0] != 15 {
		t.Errorf("expected day 15, got %v", schedule.dayOfMonth)
	}
	if len(schedule.month) != 1 || schedule.month[0] != 6 {
		t.Errorf("expected month 6, got %v", schedule.month)
	}
	if len(schedule.dayOfWeek) != 1 || schedule.dayOfWeek[0] != 3 {
		t.Errorf("expected weekday 3, got %v", schedule.dayOfWeek)
	}
}

func TestParseCronExpr_List(t *testing.T) {
	schedule, err := parseCronExpr("1,3,5 * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schedule.minute) != 3 {
		t.Fatalf("expected 3 values, got %d", len(schedule.minute))
	}
	expected := []int{1, 3, 5}
	for i, v := range expected {
		if schedule.minute[i] != v {
			t.Errorf("expected %d at index %d, got %d", v, i, schedule.minute[i])
		}
	}
}

func TestParseCronExpr_Range(t *testing.T) {
	schedule, err := parseCronExpr("1-5 * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schedule.minute) != 5 {
		t.Fatalf("expected 5 values, got %d", len(schedule.minute))
	}
	for i := 0; i < 5; i++ {
		if schedule.minute[i] != i+1 {
			t.Errorf("expected %d at index %d, got %d", i+1, i, schedule.minute[i])
		}
	}
}

func TestParseCronExpr_Step(t *testing.T) {
	schedule, err := parseCronExpr("*/15 * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schedule.minute) != 4 {
		t.Fatalf("expected 4 values, got %d", len(schedule.minute))
	}
	expected := []int{0, 15, 30, 45}
	for i, v := range expected {
		if schedule.minute[i] != v {
			t.Errorf("expected %d at index %d, got %d", v, i, schedule.minute[i])
		}
	}
}

func TestParseCronExpr_RangeWithStep(t *testing.T) {
	schedule, err := parseCronExpr("1-10/2 * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schedule.minute) != 5 {
		t.Fatalf("expected 5 values, got %d", len(schedule.minute))
	}
	expected := []int{1, 3, 5, 7, 9}
	for i, v := range expected {
		if schedule.minute[i] != v {
			t.Errorf("expected %d at index %d, got %d", v, i, schedule.minute[i])
		}
	}
}

func TestParseCronExpr_InvalidFieldCount(t *testing.T) {
	_, err := parseCronExpr("* * * *")
	if err == nil {
		t.Error("expected error for 4 fields")
	}

	_, err = parseCronExpr("* * * * * *")
	if err == nil {
		t.Error("expected error for 6 fields")
	}
}

func TestParseCronExpr_InvalidValue(t *testing.T) {
	_, err := parseCronExpr("60 * * * *")
	if err == nil {
		t.Error("expected error for minute 60")
	}

	_, err = parseCronExpr("* 24 * * *")
	if err == nil {
		t.Error("expected error for hour 24")
	}

	_, err = parseCronExpr("* * 32 * *")
	if err == nil {
		t.Error("expected error for day 32")
	}

	_, err = parseCronExpr("* * * 13 *")
	if err == nil {
		t.Error("expected error for month 13")
	}

	_, err = parseCronExpr("* * * * 7")
	if err == nil {
		t.Error("expected error for weekday 7")
	}
}

func TestNextRunTime_EveryMinute(t *testing.T) {
	schedule, _ := parseCronExpr("* * * * *")
	from := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	next := nextRunTime(schedule, from)

	expected := time.Date(2025, 1, 15, 10, 31, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestNextRunTime_SpecificMinute(t *testing.T) {
	schedule, _ := parseCronExpr("30 * * * *")
	from := time.Date(2025, 1, 15, 10, 15, 0, 0, time.UTC)
	next := nextRunTime(schedule, from)

	expected := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestNextRunTime_NextHour(t *testing.T) {
	schedule, _ := parseCronExpr("30 * * * *")
	from := time.Date(2025, 1, 15, 10, 45, 0, 0, time.UTC)
	next := nextRunTime(schedule, from)

	expected := time.Date(2025, 1, 15, 11, 30, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestNextRunTime_NextDay(t *testing.T) {
	schedule, _ := parseCronExpr("0 0 * * *")
	from := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	next := nextRunTime(schedule, from)

	expected := time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestScheduler_AddRemoveTask(t *testing.T) {
	s := New()

	err := s.AddTask("test1", "* * * * *", func(context.Context) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = s.AddTask("test1", "* * * * *", func(context.Context) {})
	if err == nil {
		t.Error("expected error for duplicate task")
	}

	err = s.RemoveTask("test1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = s.RemoveTask("test1")
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestScheduler_AddTask_InvalidExpr(t *testing.T) {
	s := New()
	err := s.AddTask("bad", "invalid expr", func(context.Context) {})
	if err == nil {
		t.Error("expected error for invalid expression")
	}
}

func TestScheduler_StartStop(t *testing.T) {
	s := New()

	var count int64
	s.AddTask("test", "* * * * *", func(context.Context) {
		atomic.AddInt64(&count, 1)
	})

	// Smoke test: Stop() waits for the run loop to exit, so no sleep is
	// needed to prove the start/stop lifecycle works (issue #85).
	s.Start()
	s.Stop()
}

// TestScheduler_StopWaitsAndCancelsInFlightTask verifies that Stop() (1)
// cancels the context handed to a running task so it can abort, and (2) waits
// for that task goroutine to actually return before Stop() returns. This is
// the regression guard for the old fire-and-forget behavior where in-flight
// task goroutines outlived the scheduler.
func TestScheduler_StopWaitsAndCancelsInFlightTask(t *testing.T) {
	s := New()

	taskStarted := make(chan struct{})
	taskCtxCancelled := make(chan struct{})

	_ = s.AddTask("blocking", "* * * * *", func(ctx context.Context) {
		close(taskStarted)
		<-ctx.Done() // block until Stop cancels the task context
		close(taskCtxCancelled)
	})

	// Force the task to be due now so the next 1s tick fires it immediately
	// (AddTask otherwise schedules it for the next minute boundary).
	s.mu.Lock()
	if task, ok := s.tasks["blocking"]; ok {
		task.nextRun = time.Now().Add(-time.Second)
	}
	s.mu.Unlock()

	s.Start()

	select {
	case <-taskStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("scheduled task did not start in time")
	}

	// Stop() should cancel the task context (unblocking the task) and then
	// wait on taskWg until the task goroutine returns.
	stopDone := make(chan struct{})
	go func() {
		s.Stop()
		close(stopDone)
	}()
	select {
	case <-taskCtxCancelled:
		// task observed its context being cancelled
	case <-time.After(3 * time.Second):
		t.Fatal("task context was not cancelled on Stop")
	}
	select {
	case <-stopDone:
		// Stop returned → it waited for the in-flight task goroutine
	case <-time.After(3 * time.Second):
		t.Fatal("Stop did not return after cancelling in-flight task (taskWg not waited)")
	}
}

func TestScheduler_StartTwice(t *testing.T) {
	s := New()
	s.Start()
	s.Start()
	s.Stop()
}

func TestScheduler_StopTwice(t *testing.T) {
	s := New()
	s.Start()
	s.Stop()
	s.Stop()
}

func TestScheduler_ThreadSafety(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			_ = s.AddTask(string(rune('a'+i%26))+string(rune('0'+i/26)), "* * * * *", func(context.Context) {})
		}(i)
		go func(i int) {
			defer wg.Done()
			_ = s.RemoveTask(string(rune('a'+i%26)) + string(rune('0'+i/26)))
		}(i)
	}
	wg.Wait()
}

func TestContains(t *testing.T) {
	slice := []int{1, 3, 5, 7, 9}

	if !contains(slice, 3) {
		t.Error("expected contains 3")
	}
	if contains(slice, 4) {
		t.Error("expected not contains 4")
	}
}

// ─── Retry tasks ────────────────────────────────────────────────────────────

// forceDue makes a registered task due immediately so the next 1s scheduler
// tick fires it without waiting for the minute boundary.
func forceDue(s *Scheduler, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task, ok := s.tasks[id]; ok {
		task.nextRun = time.Now().Add(-time.Second)
	}
}

func TestAddRetryingTask_Validation(t *testing.T) {
	s := New()

	if err := s.AddRetryingTask("nil", "* * * * *", nil, RetryPolicy{}); err == nil {
		t.Error("expected error for nil function")
	}

	if err := s.AddRetryingTask("bad", "invalid", func(context.Context) error { return nil }, RetryPolicy{}); err == nil {
		t.Error("expected error for invalid cron")
	}

	// Duplicate ID (against both AddTask and AddRetryingTask).
	_ = s.AddTask("dup", "* * * * *", func(context.Context) {})
	if err := s.AddRetryingTask("dup", "* * * * *", func(context.Context) error { return nil }, RetryPolicy{}); err == nil {
		t.Error("expected error for duplicate id")
	}

	// Policy clamping: negative → 0, over-cap → MaxTaskRetries, delay ≤0 → default.
	err := s.AddRetryingTask("clamp", "* * * * *", func(context.Context) error { return nil },
		RetryPolicy{MaxRetries: -5, Delay: -time.Second})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.mu.RLock()
	task := s.tasks["clamp"]
	s.mu.RUnlock()
	if task.Retry.MaxRetries != 0 {
		t.Errorf("MaxRetries = %d, want 0", task.Retry.MaxRetries)
	}
	if task.Retry.Delay != DefaultTaskRetryDelay {
		t.Errorf("Delay = %v, want default %v", task.Retry.Delay, DefaultTaskRetryDelay)
	}

	err = s.AddRetryingTask("clamp2", "* * * * *", func(context.Context) error { return nil },
		RetryPolicy{MaxRetries: MaxTaskRetries + 100, Delay: time.Millisecond})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.mu.RLock()
	task2 := s.tasks["clamp2"]
	s.mu.RUnlock()
	if task2.Retry.MaxRetries != MaxTaskRetries {
		t.Errorf("MaxRetries = %d, want clamp to %d", task2.Retry.MaxRetries, MaxTaskRetries)
	}
}

func TestAddRetryingTask_FailsThenSucceeds(t *testing.T) {
	s := New()

	var calls atomic.Int64
	err := s.AddRetryingTask("flaky", "* * * * *", func(context.Context) error {
		// Fail the first two attempts, succeed on the third.
		if calls.Add(1) <= 2 {
			return errors.New("transient failure")
		}
		return nil
	}, RetryPolicy{MaxRetries: 3, Delay: time.Millisecond})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	forceDue(s, "flaky")
	s.Start()
	defer s.Stop()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if calls.Load() >= 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task did not reach 3 attempts, got %d", calls.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("calls = %d, want exactly 3 (no retry after success)", got)
	}
}

func TestAddRetryingTask_ExhaustsRetries(t *testing.T) {
	s := New()

	var calls atomic.Int64
	err := s.AddRetryingTask("always-fails", "* * * * *", func(context.Context) error {
		calls.Add(1)
		return errors.New("permanent failure")
	}, RetryPolicy{MaxRetries: 2, Delay: time.Millisecond})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	forceDue(s, "always-fails")
	s.Start()
	defer s.Stop()

	// MaxRetries=2 → initial attempt + 2 retries = 3 calls total.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if calls.Load() >= 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task did not reach 3 attempts, got %d", calls.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Allow a grace period to catch a spurious 4th attempt.
	time.Sleep(100 * time.Millisecond)
	if got := calls.Load(); got != 3 {
		t.Errorf("calls = %d, want 3 (MaxRetries=2 must stop after 3 attempts)", got)
	}
}

func TestAddRetryingTask_NoRetryByDefault(t *testing.T) {
	s := New()

	var calls atomic.Int64
	err := s.AddRetryingTask("once", "* * * * *", func(context.Context) error {
		calls.Add(1)
		return errors.New("failure is not retried")
	}, RetryPolicy{}) // MaxRetries=0 → execute once, historical behaviour
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	forceDue(s, "once")
	s.Start()
	defer s.Stop()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if calls.Load() >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task did not run, calls = %d", calls.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Errorf("calls = %d, want 1 (MaxRetries=0 must not retry)", got)
	}
}

func TestAddRetryingTask_PanicIsRetried(t *testing.T) {
	s := New()

	var calls atomic.Int64
	err := s.AddRetryingTask("panicky", "* * * * *", func(context.Context) error {
		if calls.Add(1) == 1 {
			panic("boom on first attempt")
		}
		return nil
	}, RetryPolicy{MaxRetries: 2, Delay: time.Millisecond})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	forceDue(s, "panicky")
	s.Start()
	defer s.Stop()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if calls.Load() >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task did not reach 2 attempts (panic not retried?), got %d", calls.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if got := calls.Load(); got != 2 {
		t.Errorf("calls = %d, want 2 (panic must be converted to a retryable failure)", got)
	}
}

// TestAddRetryingTask_StopAbortsRetry verifies that scheduler shutdown aborts
// the retry loop instead of blocking Stop() for the full backoff delay.
func TestAddRetryingTask_StopAbortsRetry(t *testing.T) {
	s := New()

	var calls atomic.Int64
	first := make(chan struct{})
	err := s.AddRetryingTask("long-backoff", "* * * * *", func(context.Context) error {
		if calls.Add(1) == 1 {
			close(first)
		}
		return errors.New("keep failing")
	}, RetryPolicy{MaxRetries: 5, Delay: 10 * time.Second}) // backoff >> Stop tolerance
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	forceDue(s, "long-backoff")
	s.Start()

	select {
	case <-first:
	case <-time.After(3 * time.Second):
		t.Fatal("task did not start in time")
	}

	// Stop must return well before the 10s backoff elapses.
	stopDone := make(chan struct{})
	go func() {
		s.Stop()
		close(stopDone)
	}()
	select {
	case <-stopDone:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop blocked on retry backoff instead of aborting")
	}
}

func TestRetryDelay(t *testing.T) {
	base := 100 * time.Millisecond
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
	}
	for _, tc := range cases {
		if got := retryDelay(base, tc.attempt); got != tc.want {
			t.Errorf("retryDelay(base, %d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
	// Capped at MaxTaskRetryDelay for large attempts.
	if got := retryDelay(MaxTaskRetryDelay, 5); got != MaxTaskRetryDelay {
		t.Errorf("retryDelay(max, 5) = %v, want cap %v", got, MaxTaskRetryDelay)
	}
}

func TestDaysInMonth(t *testing.T) {
	if daysInMonth(2025, time.February) != 28 {
		t.Error("expected 28 days in Feb 2025")
	}
	if daysInMonth(2024, time.February) != 29 {
		t.Error("expected 29 days in Feb 2024")
	}
	if daysInMonth(2025, time.January) != 31 {
		t.Error("expected 31 days in January")
	}
	if daysInMonth(2025, time.April) != 30 {
		t.Error("expected 30 days in April")
	}
}

// ── last-run restore & misfire marking ──────────────────────────────────

// TestRestoreLastRun_MarksMissedRuns verifies that restoring a last-run
// timestamp from before a downtime counts the skipped fire times and
// exposes them via ListTasks (marked — the scheduler never replays them).
func TestRestoreLastRun_MarksMissedRuns(t *testing.T) {
	s := New()
	_ = s.AddTask("hourly", "0 * * * *", func(context.Context) {})

	// "Fired" three hours ago: with an hourly cron that is ~3 missed runs
	// (the exact count depends on the current minute, so assert a range).
	lastRun := time.Now().Add(-3 * time.Hour)
	if err := s.RestoreLastRun("hourly", lastRun); err != nil {
		t.Fatalf("RestoreLastRun: %v", err)
	}

	tasks := s.ListTasks()
	var found bool
	for _, ti := range tasks {
		if ti.ID != "hourly" {
			continue
		}
		found = true
		if !ti.LastRun.Equal(lastRun) {
			t.Errorf("LastRun = %v, want %v", ti.LastRun, lastRun)
		}
		if ti.MissedRuns < 2 || ti.MissedRuns > 4 {
			t.Errorf("MissedRuns = %d, want 2–4 for 3h downtime on hourly cron", ti.MissedRuns)
		}
	}
	if !found {
		t.Fatal("task hourly not in ListTasks")
	}
}

func TestRestoreLastRun_UnknownTask(t *testing.T) {
	s := New()
	if err := s.RestoreLastRun("ghost", time.Now()); err == nil {
		t.Error("expected error for unknown task")
	}
}

// TestRestoreLastRun_ZeroTimeIsNoop verifies that a zero timestamp (never
// fired) restores nothing and fabricates no misfires.
func TestRestoreLastRun_ZeroTimeIsNoop(t *testing.T) {
	s := New()
	_ = s.AddTask("fresh", "0 * * * *", func(context.Context) {})

	if err := s.RestoreLastRun("fresh", time.Time{}); err != nil {
		t.Fatalf("zero time should be a no-op, got %v", err)
	}
	for _, ti := range s.ListTasks() {
		if ti.ID == "fresh" && (ti.MissedRuns != 0 || !ti.LastRun.IsZero()) {
			t.Errorf("zero restore must not touch state: %+v", ti)
		}
	}
}

// TestRestoreLastRun_RecentRunNoMissed is intentionally NOT wall-clock
// based: for any cron there exists a wall-clock moment where "one minute
// ago" genuinely straddles a fire time (e.g. just past midnight for a daily
// cron), which would make the assertion flaky. The no-fabrication property
// is instead pinned deterministically by TestCountMissedRuns_ExactCount
// (half-open window: a fire AT "now" is not missed).

// TestCountMissedRuns_CappedAtMax verifies the iteration cap: a last-run
// far in the past must not spin the counter unbounded.
func TestCountMissedRuns_CappedAtMax(t *testing.T) {
	schedule, err := parseCronExpr("* * * * *")
	if err != nil {
		t.Fatal(err)
	}
	got := countMissedRuns(schedule, time.Now().AddDate(-10, 0, 0), time.Now())
	if got != MaxMissedRunCount {
		t.Errorf("countMissedRuns for 10y downtime = %d, want cap %d", got, MaxMissedRunCount)
	}
}

func TestCountMissedRuns_ExactCount(t *testing.T) {
	schedule, err := parseCronExpr("*/30 * * * *")
	if err != nil {
		t.Fatal(err)
	}
	// Half-open window (09:00, 11:00) with fires at :00/:30 — the fires
	// strictly inside are 09:30, 10:00, 10:30 = 3 (11:00 itself is not
	// "missed", it belongs to the next live dispatch).
	from := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 1, 11, 0, 0, 0, time.UTC)
	if got := countMissedRuns(schedule, from, to); got != 3 {
		t.Errorf("countMissedRuns = %d, want 3", got)
	}
	// No missed runs when lastRun >= now.
	if got := countMissedRuns(schedule, to, from); got != 0 {
		t.Errorf("countMissedRuns(reversed) = %d, want 0", got)
	}
	// Nil schedule is tolerated.
	if got := countMissedRuns(nil, from, to); got != 0 {
		t.Errorf("countMissedRuns(nil) = %d, want 0", got)
	}
}

// TestSetOnFire_FiresOnDispatch verifies the onFire callback fires with the
// task ID and fire timestamp when a due task is dispatched.
func TestSetOnFire_FiresOnDispatch(t *testing.T) {
	s := New()

	fired := make(chan string, 1)
	s.SetOnFire(func(taskID string, firedAt time.Time) {
		select {
		case fired <- taskID:
		default:
		}
		if firedAt.IsZero() {
			t.Error("onFire got zero timestamp")
		}
	})

	_ = s.AddTask("watchme", "* * * * *", func(context.Context) {})

	// Force the task due now so the next 1s tick fires it.
	s.mu.Lock()
	if task, ok := s.tasks["watchme"]; ok {
		task.nextRun = time.Now().Add(-time.Second)
	}
	s.mu.Unlock()

	s.Start()
	defer s.Stop()

	select {
	case id := <-fired:
		if id != "watchme" {
			t.Errorf("onFire task ID = %q, want %q", id, "watchme")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("onFire was not invoked for a dispatched task")
	}

	// lastRun must be recorded alongside the callback.
	for _, ti := range s.ListTasks() {
		if ti.ID == "watchme" && ti.LastRun.IsZero() {
			t.Error("lastRun not recorded on dispatch")
		}
	}
}
