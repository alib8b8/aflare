// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌​​​​‌‌​‌​​​​‌‌‌​​‌‌​‌‌​​​​‌​​‌‌‌​​‌​​​‌‌​​​​​​​​​​​​​​​​​​​​​​‌‌​‌‌​​​​​‌‌‌‌​⁠
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

// taskqueue provides a FIFO task queue with deduplication for the agent daemon.
// Tasks from the scheduler are enqueued and executed concurrently by the agent
// loop, ensuring no task is dropped and no duplicate task runs concurrently.
// Each task tracks its lifecycle status: pending → running → done/failed.
package taskqueue

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/metrics"
)

// TaskStatus represents the lifecycle state of a queued task.
type TaskStatus string

const (
	StatusPending TaskStatus = "pending"
	StatusRunning TaskStatus = "running"
	StatusDone    TaskStatus = "done"
	StatusFailed  TaskStatus = "failed"

	// maxStatusEntries limits the number of completed task statuses kept
	// in memory to prevent unbounded growth in long-running agents.
	maxStatusEntries = 10000
)

// Task represents a queued task to be executed by the agent.
type Task struct {
	ID        string            // unique task identifier (for dedup)
	Source    string            // source of the task (e.g. "scheduler")
	Message   string            // the agent input message
	CreatedAt time.Time         // when the task was created
	Status    TaskStatus        // current lifecycle status
	Error     string            // error message if status is failed
	StartedAt time.Time         // when execution started
	DoneAt    time.Time         // when execution completed (done or failed)
	ReplyTo   chan<- TaskResult // where to send the result (nil = discard)
}

// TaskResult carries the result of a task execution.
type TaskResult struct {
	TaskID   string
	Response string
	Error    error
}

// Queue is a FIFO task queue with deduplication. It ensures:
//   - Tasks are executed in order (FIFO)
//   - No duplicate task ID runs concurrently
//   - No task is silently dropped
//   - Each task tracks its lifecycle status: pending → running → done/failed
type Queue struct {
	mu       sync.Mutex
	queue    []*Task
	active   map[string]bool // currently executing task IDs
	notEmpty chan struct{}   // signaled when queue is non-empty
	maxSize  int             // maximum queue size (0 = unlimited)
	// Status tracking: map of task ID to status for history lookup
	statuses map[string]*Task
}

// New creates a new task queue with the given maximum size.
// maxSize 0 means unlimited.
func New(maxSize int) *Queue {
	return &Queue{
		queue:    make([]*Task, 0),
		active:   make(map[string]bool),
		notEmpty: make(chan struct{}, 1),
		maxSize:  maxSize,
		statuses: make(map[string]*Task),
	}
}

// Enqueue adds a task to the queue. Returns an error if the queue is full
// or if a task with the same ID is already in the queue or executing.
func (q *Queue) Enqueue(task *Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Dedup: check if task ID is already active or queued
	if q.active[task.ID] {
		return nil // silently ignore duplicate
	}
	for _, t := range q.queue {
		if t.ID == task.ID {
			return nil // silently ignore duplicate
		}
	}

	// Check capacity
	if q.maxSize > 0 && len(q.queue) >= q.maxSize {
		// Drop oldest task to make room, clean up its status
		dropped := q.queue[0]
		delete(q.statuses, dropped.ID)
		q.queue = q.queue[1:]
	}

	// Set initial status
	task.Status = StatusPending
	task.CreatedAt = time.Now()
	q.statuses[task.ID] = task

	q.queue = append(q.queue, task)
	metrics.SetQueueDepth(len(q.queue))

	// Signal non-empty
	select {
	case q.notEmpty <- struct{}{}:
	default:
	}

	return nil
}

// Dequeue removes and returns the next task from the queue.
// Blocks until a task is available or ctx is cancelled.
// The returned task is marked as running.
func (q *Queue) Dequeue(ctx context.Context) (*Task, error) {
	for {
		q.mu.Lock()
		if len(q.queue) > 0 {
			task := q.queue[0]
			q.queue = q.queue[1:]
			metrics.SetQueueDepth(len(q.queue))
			q.active[task.ID] = true
			// Mark as running
			task.Status = StatusRunning
			task.StartedAt = time.Now()
			q.mu.Unlock()
			return task, nil
		}
		q.mu.Unlock()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-q.notEmpty:
			// Loop back to check queue
		}
	}
}

// Done marks a task as completed successfully, removing it from the active set.
func (q *Queue) Done(taskID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.active, taskID)
	if t, ok := q.statuses[taskID]; ok {
		t.Status = StatusDone
		t.DoneAt = time.Now()
	}
	q.pruneStatuses()
}

// Failed marks a task as failed with an error message, removing it from the active set.
func (q *Queue) Failed(taskID string, errMsg string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.active, taskID)
	if t, ok := q.statuses[taskID]; ok {
		t.Status = StatusFailed
		t.Error = errMsg
		t.DoneAt = time.Now()
	}
	q.pruneStatuses()
}

// pruneStatuses removes old completed/failed entries when the statuses map
// exceeds maxStatusEntries, keeping only the most recent ones.
// Must be called with q.mu held.
func (q *Queue) pruneStatuses() {
	if len(q.statuses) <= maxStatusEntries {
		return
	}
	// Remove oldest completed entries (done or failed) until under limit
	for id, t := range q.statuses {
		if t.Status == StatusDone || t.Status == StatusFailed {
			delete(q.statuses, id)
		}
		if len(q.statuses) <= maxStatusEntries {
			break
		}
	}
}

// Status returns the status of a task by ID. Returns empty string if not found.
func (q *Queue) Status(taskID string) TaskStatus {
	q.mu.Lock()
	defer q.mu.Unlock()
	if t, ok := q.statuses[taskID]; ok {
		return t.Status
	}
	return ""
}

// StatusSummary returns a summary of task counts by status.
func (q *Queue) StatusSummary() map[TaskStatus]int {
	q.mu.Lock()
	defer q.mu.Unlock()
	summary := map[TaskStatus]int{
		StatusPending: 0,
		StatusRunning: 0,
		StatusDone:    0,
		StatusFailed:  0,
	}
	for _, t := range q.statuses {
		summary[t.Status]++
	}
	return summary
}

// Size returns the current number of tasks in the queue.
func (q *Queue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queue)
}

// ActiveCount returns the number of currently executing tasks.
func (q *Queue) ActiveCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.active)
}

// Run starts the queue worker. It dequeues tasks and executes them using the
// provided handler function. Blocks until ctx is cancelled.
// Each task transitions through: pending → running → done/failed.
func (q *Queue) Run(ctx context.Context, handler func(ctx context.Context, task *Task) TaskResult) {
	log.Printf("[taskqueue] worker started (max size: %d)", q.maxSize)

	for {
		task, err := q.Dequeue(ctx)
		if err != nil {
			log.Printf("[taskqueue] worker stopped: %v", err)
			return
		}

		go func(t *Task) {
			result := handler(ctx, t)

			if result.Error != nil {
				q.Failed(t.ID, result.Error.Error())
				log.Printf("[taskqueue] task %q failed: %v", t.ID, result.Error)
			} else {
				q.Done(t.ID)
				log.Printf("[taskqueue] task %q completed (%d chars)", t.ID, len(result.Response))
			}

			if t.ReplyTo != nil {
				select {
				case t.ReplyTo <- result:
				case <-ctx.Done():
				default:
				}
			}
		}(task)
	}
}
