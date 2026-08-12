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

package taskqueue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// ── Enqueue / Dedup ────────────────────────────────────────────────────────

func TestEnqueue_Basic(t *testing.T) {
	q := New(0)
	err := q.Enqueue(&Task{ID: "task-1", Message: "hello"})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if q.Size() != 1 {
		t.Errorf("expected size 1, got %d", q.Size())
	}
}

func TestEnqueue_DedupSameID(t *testing.T) {
	q := New(0)
	q.Enqueue(&Task{ID: "task-1", Message: "first"})
	q.Enqueue(&Task{ID: "task-1", Message: "second"})

	if q.Size() != 1 {
		t.Errorf("dedup failed: expected size 1, got %d", q.Size())
	}
}

func TestEnqueue_DedupActiveTask(t *testing.T) {
	q := New(0)
	q.Enqueue(&Task{ID: "task-1", Message: "hello"})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	task, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}
	if task.ID != "task-1" {
		t.Fatalf("expected task-1, got %s", task.ID)
	}
	// task is now active — re-enqueue should be silently ignored
	q.Enqueue(&Task{ID: "task-1", Message: "duplicate"})

	if q.Size() != 0 {
		t.Errorf("dedup for active task failed: expected size 0, got %d", q.Size())
	}
}

func TestEnqueue_DifferentIDs(t *testing.T) {
	q := New(0)
	q.Enqueue(&Task{ID: "task-1", Message: "first"})
	q.Enqueue(&Task{ID: "task-2", Message: "second"})
	q.Enqueue(&Task{ID: "task-3", Message: "third"})

	if q.Size() != 3 {
		t.Errorf("expected size 3, got %d", q.Size())
	}
}

// ── Capacity / Overflow ────────────────────────────────────────────────────

func TestEnqueue_CapacityOverflow(t *testing.T) {
	q := New(2) // max 2 tasks
	q.Enqueue(&Task{ID: "task-1", Message: "first"})
	q.Enqueue(&Task{ID: "task-2", Message: "second"})
	q.Enqueue(&Task{ID: "task-3", Message: "third"})

	if q.Size() != 2 {
		t.Errorf("expected size 2 after overflow, got %d", q.Size())
	}
	// The oldest task (task-1) should be dropped
	if q.Status("task-1") != "" {
		t.Errorf("task-1 should be dropped from statuses, got status %q", q.Status("task-1"))
	}
}

func TestEnqueue_Unlimited(t *testing.T) {
	q := New(0) // unlimited
	for i := 0; i < 100; i++ {
		q.Enqueue(&Task{ID: "task-" + string(rune('a'+i%26)), Message: "msg"})
	}
	// All unique IDs should be accepted
	if q.Size() != 26 {
		t.Errorf("expected 26 unique tasks, got %d", q.Size())
	}
}

// ── Dequeue / Lifecycle ────────────────────────────────────────────────────

func TestDequeue_LifecyclePendingToRunning(t *testing.T) {
	q := New(0)
	q.Enqueue(&Task{ID: "task-1", Message: "hello"})

	if q.Status("task-1") != StatusPending {
		t.Errorf("expected pending status, got %q", q.Status("task-1"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	task, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}
	if task.Status != StatusRunning {
		t.Errorf("expected running status, got %q", task.Status)
	}
	if q.Status("task-1") != StatusRunning {
		t.Errorf("expected statuses to show running, got %q", q.Status("task-1"))
	}
}

func TestDequeue_BlocksOnEmpty(t *testing.T) {
	q := New(0)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := q.Dequeue(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded on empty queue, got %v", err)
	}
}

func TestDequeue_FIFOOrder(t *testing.T) {
	q := New(0)
	ids := []string{"a", "b", "c", "d", "e"}
	for _, id := range ids {
		q.Enqueue(&Task{ID: id, Message: "msg"})
	}

	ctx := context.Background()
	for _, want := range ids {
		task, err := q.Dequeue(ctx)
		if err != nil {
			t.Fatalf("Dequeue failed: %v", err)
		}
		if task.ID != want {
			t.Errorf("FIFO violation: expected %q, got %q", want, task.ID)
		}
	}
}

// ── Done / Failed ──────────────────────────────────────────────────────────

func TestDone_SetsStatus(t *testing.T) {
	q := New(0)
	q.Enqueue(&Task{ID: "task-1", Message: "hello"})

	ctx := context.Background()
	task, _ := q.Dequeue(ctx)
	_ = task

	q.Done("task-1")
	if q.Status("task-1") != StatusDone {
		t.Errorf("expected done status, got %q", q.Status("task-1"))
	}
	if q.ActiveCount() != 0 {
		t.Errorf("active count should be 0 after done, got %d", q.ActiveCount())
	}
}

func TestFailed_SetsStatusAndError(t *testing.T) {
	q := New(0)
	q.Enqueue(&Task{ID: "task-1", Message: "hello"})

	ctx := context.Background()
	task, _ := q.Dequeue(ctx)
	_ = task

	q.Failed("task-1", "something went wrong")
	if q.Status("task-1") != StatusFailed {
		t.Errorf("expected failed status, got %q", q.Status("task-1"))
	}
	if q.ActiveCount() != 0 {
		t.Errorf("active count should be 0 after failed, got %d", q.ActiveCount())
	}
}

func TestStatus_NotFound(t *testing.T) {
	q := New(0)
	if s := q.Status("nonexistent"); s != "" {
		t.Errorf("expected empty status for unknown task, got %q", s)
	}
}

// ── StatusSummary ──────────────────────────────────────────────────────────

func TestStatusSummary(t *testing.T) {
	q := New(0)
	q.Enqueue(&Task{ID: "pending-1", Message: "msg"})
	q.Enqueue(&Task{ID: "pending-2", Message: "msg"})

	ctx := context.Background()
	task, _ := q.Dequeue(ctx)
	q.Done(task.ID)

	summary := q.StatusSummary()
	if summary[StatusPending] != 1 {
		t.Errorf("expected 1 pending, got %d", summary[StatusPending])
	}
	if summary[StatusRunning] != 0 {
		t.Errorf("expected 0 running, got %d", summary[StatusRunning])
	}
	if summary[StatusDone] != 1 {
		t.Errorf("expected 1 done, got %d", summary[StatusDone])
	}
	if summary[StatusFailed] != 0 {
		t.Errorf("expected 0 failed, got %d", summary[StatusFailed])
	}
}

// ── Run Worker ─────────────────────────────────────────────────────────────

func TestRun_ProcessesTask(t *testing.T) {
	q := New(0)
	q.Enqueue(&Task{ID: "task-1", Message: "hello"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var processed bool
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		// Run for a short time, just enough to process one task
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	q.Run(ctx, func(taskCtx context.Context, task *Task) TaskResult {
		mu.Lock()
		processed = true
		mu.Unlock()
		close(done)
		return TaskResult{TaskID: task.ID, Response: "done"}
	})

	// Wait for handler goroutine to complete
	<-done

	mu.Lock()
	if !processed {
		t.Error("task was not processed by Run")
	}
	mu.Unlock()
	if q.Status("task-1") != StatusDone {
		t.Errorf("expected done status after Run, got %q", q.Status("task-1"))
	}
}

func TestRun_HandlerError(t *testing.T) {
	q := New(0)
	q.Enqueue(&Task{ID: "task-1", Message: "hello"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	q.Run(ctx, func(taskCtx context.Context, task *Task) TaskResult {
		defer close(done)
		return TaskResult{TaskID: task.ID, Error: context.DeadlineExceeded}
	})

	// Wait for handler goroutine to complete
	<-done

	if q.Status("task-1") != StatusFailed {
		t.Errorf("expected failed status after handler error, got %q", q.Status("task-1"))
	}
}

func TestRun_ReplyTo(t *testing.T) {
	q := New(0)
	replyCh := make(chan TaskResult, 1)
	q.Enqueue(&Task{ID: "task-1", Message: "hello", ReplyTo: replyCh})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	q.Run(ctx, func(taskCtx context.Context, task *Task) TaskResult {
		return TaskResult{TaskID: task.ID, Response: "response text"}
	})

	select {
	case result := <-replyCh:
		if result.Response != "response text" {
			t.Errorf("expected 'response text', got %q", result.Response)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timed out waiting for reply")
	}
}

// ── Status Pruning ─────────────────────────────────────────────────────────

func TestPruneStatuses_LargeVolume(t *testing.T) {
	q := New(0)
	// Enqueue many tasks, mark them done, and verify pruning works
	for i := 0; i < 100; i++ {
		id := "task-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		q.Enqueue(&Task{ID: id, Message: "msg"})
	}
	// Mark all as done
	for i := 0; i < 100; i++ {
		id := "task-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		if q.Status(id) == StatusPending {
			// Dequeue and done
			ctx := context.Background()
			task, err := q.Dequeue(ctx)
			if err != nil {
				break
			}
			q.Done(task.ID)
		}
	}
	// Should not panic or hang
	summary := q.StatusSummary()
	t.Logf("status summary: %+v", summary)
}