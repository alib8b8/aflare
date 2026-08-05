// Copyright (c) 2026 llm-box Contributors
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

package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alib8b8/llm-box/internal/nodes"
)

// idempCounterNode increments a shared counter on every Execute and returns
// the new count as "call-<n>". It is the side-effecting stand-in (e.g. an HTTP
// POST money transfer) that idempotency must keep from running twice.
type idempCounterNode struct {
	name    string
	counter *int
}

func (n *idempCounterNode) Name() string        { return n.name }
func (n *idempCounterNode) Description() string { return "counter node for idempotency test" }
func (n *idempCounterNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Input: "string", Output: "string"}
}

func (n *idempCounterNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	*n.counter++
	return fmt.Sprintf("call-%d", *n.counter), nil
}

// newIdempTestRegistry builds a registry with a single counter node wired to
// the given counter pointer.
func newIdempTestRegistry(counter *int) *nodes.Registry {
	reg := nodes.NewRegistry()
	reg.Register(&idempCounterNode{name: "counter", counter: counter})
	return reg
}

// twoStepCounterWorkflow returns a 2-step workflow whose final output is
// "call-2" when run from scratch (each step bumps the counter once).
func twoStepCounterWorkflow() *Workflow {
	return &Workflow{
		Name: "idemp-counter-wf",
		Steps: []WorkflowStep{
			{Node: "counter", Name: "s1"},
			{Node: "counter", Name: "s2"},
		},
	}
}

// TestIdempotency_SecondRunSkipsExecution verifies that re-triggering the same
// workflow with the same Idempotency-Key does not re-run side-effecting nodes:
// the counter advances to 2 on the first run and stays at 2 on the second,
// which returns the cached output and ErrIdempotencyHit.
func TestIdempotency_SecondRunSkipsExecution(t *testing.T) {
	t.Parallel()

	var counter int
	reg := newIdempTestRegistry(&counter)
	wf := twoStepCounterWorkflow()
	store := NewFileIdempotencyStore(t.TempDir(), 0)

	exec := NewExecutor().WithIdempotencyKey("transfer-001").WithIdempotencyStore(store)

	// Run 1: executes both steps.
	out1, _, trace1, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("run1: expected success, got: %v", err)
	}
	if counter != 2 {
		t.Fatalf("run1: expected counter=2, got %d", counter)
	}
	if out1 != "call-2" {
		t.Fatalf("run1: expected output \"call-2\", got %q", out1)
	}
	if trace1 == nil || trace1.RunID == "" {
		t.Fatalf("run1: expected non-empty trace.RunID, got %+v", trace1)
	}
	if trace1.IdempotencyHit {
		t.Fatalf("run1: IdempotencyHit should be false on a real execution")
	}
	runID1 := trace1.RunID

	// Run 2: same key → cache hit, no step re-runs.
	out2, results2, trace2, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil)
	if !errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("run2: expected ErrIdempotencyHit, got: %v", err)
	}
	if counter != 2 {
		t.Fatalf("run2: expected counter to stay at 2 (no re-execution), got %d", counter)
	}
	if out2 != out1 {
		t.Fatalf("run2: expected cached output %q, got %q", out1, out2)
	}
	if len(results2) != 0 {
		t.Fatalf("run2: expected 0 step results on cache hit, got %d", len(results2))
	}
	if trace2 == nil || !trace2.IdempotencyHit {
		t.Fatalf("run2: expected trace.IdempotencyHit=true, got %+v", trace2)
	}
	if trace2.RunID != runID1 {
		t.Fatalf("run2: expected cached RunID %q, got %q", runID1, trace2.RunID)
	}
}

// TestIdempotency_DifferentKeyExecutes verifies that a different key is treated
// as a distinct operation: both keys execute independently.
func TestIdempotency_DifferentKeyExecutes(t *testing.T) {
	t.Parallel()

	var counter int
	reg := newIdempTestRegistry(&counter)
	wf := twoStepCounterWorkflow()
	store := NewFileIdempotencyStore(t.TempDir(), 0)

	exec1 := NewExecutor().WithIdempotencyKey("transfer-A").WithIdempotencyStore(store)
	out1, _, _, err := exec1.ExecuteWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("key A: expected success, got: %v", err)
	}
	if counter != 2 {
		t.Fatalf("key A: expected counter=2, got %d", counter)
	}
	if out1 != "call-2" {
		t.Fatalf("key A: expected output \"call-2\", got %q", out1)
	}

	exec2 := NewExecutor().WithIdempotencyKey("transfer-B").WithIdempotencyStore(store)
	out2, _, _, err := exec2.ExecuteWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("key B: expected success, got: %v", err)
	}
	if errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("key B: should not be a cache hit for a different key")
	}
	if counter != 4 {
		t.Fatalf("key B: expected counter=4 (executed again), got %d", counter)
	}
	if out2 != "call-4" {
		t.Fatalf("key B: expected output \"call-4\", got %q", out2)
	}
}

// TestIdempotency_NoKeyBackwardCompat verifies that an Executor with no
// idempotency key behaves exactly as before: every run re-executes and never
// returns ErrIdempotencyHit.
func TestIdempotency_NoKeyBackwardCompat(t *testing.T) {
	t.Parallel()

	var counter int
	reg := newIdempTestRegistry(&counter)
	wf := twoStepCounterWorkflow()

	exec := NewExecutor() // no WithIdempotencyKey

	_, _, _, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("run1: expected success, got: %v", err)
	}
	if errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("run1: plain executor must never return ErrIdempotencyHit")
	}
	if counter != 2 {
		t.Fatalf("run1: expected counter=2, got %d", counter)
	}

	out2, _, trace2, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("run2: expected success, got: %v", err)
	}
	if errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("run2: plain executor must never return ErrIdempotencyHit")
	}
	if counter != 4 {
		t.Fatalf("run2: expected counter=4 (re-executed, no dedup), got %d", counter)
	}
	if out2 != "call-4" {
		t.Fatalf("run2: expected output \"call-4\", got %q", out2)
	}
	if trace2 != nil && trace2.IdempotencyHit {
		t.Fatalf("run2: IdempotencyHit must be false without a key")
	}
}

// TestIdempotency_ClearKeyAllowsRerun verifies that after Clear the same key
// executes again (the dedup entry was removed).
func TestIdempotency_ClearKeyAllowsRerun(t *testing.T) {
	t.Parallel()

	var counter int
	reg := newIdempTestRegistry(&counter)
	wf := twoStepCounterWorkflow()
	store := NewFileIdempotencyStore(t.TempDir(), 0)

	exec := NewExecutor().WithIdempotencyKey("transfer-C").WithIdempotencyStore(store)

	if _, _, _, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil); err != nil {
		t.Fatalf("run1: expected success, got: %v", err)
	}
	if counter != 2 {
		t.Fatalf("run1: expected counter=2, got %d", counter)
	}

	// Without clearing, a second run is a cache hit.
	if _, _, _, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil); !errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("run2 (pre-clear): expected ErrIdempotencyHit, got: %v", err)
	}
	if counter != 2 {
		t.Fatalf("run2 (pre-clear): counter should still be 2, got %d", counter)
	}

	// Clear → next run re-executes.
	if err := store.Clear("transfer-C"); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	out3, _, _, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("run3 (post-clear): expected success, got: %v", err)
	}
	if errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("run3 (post-clear): should not be a cache hit after Clear")
	}
	if counter != 4 {
		t.Fatalf("run3 (post-clear): expected counter=4 (re-executed), got %d", counter)
	}
	if out3 != "call-4" {
		t.Fatalf("run3 (post-clear): expected output \"call-4\", got %q", out3)
	}
}

// TestIdempotency_PersistedAcrossInstances verifies that the dedup ledger
// survives across Executor / store instances pointing at the same directory:
// instance B sees instance A's persisted record and returns a cache hit.
func TestIdempotency_PersistedAcrossInstances(t *testing.T) {
	t.Parallel()

	var counter int
	reg := newIdempTestRegistry(&counter)
	wf := twoStepCounterWorkflow()
	dir := t.TempDir()

	// Instance A writes the record to dir.
	execA := NewExecutor().
		WithIdempotencyKey("transfer-D").
		WithIdempotencyStore(NewFileIdempotencyStore(dir, 0))
	outA, _, traceA, err := execA.ExecuteWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("instance A: expected success, got: %v", err)
	}
	if counter != 2 {
		t.Fatalf("instance A: expected counter=2, got %d", counter)
	}
	if outA != "call-2" {
		t.Fatalf("instance A: expected output \"call-2\", got %q", outA)
	}

	// Verify a record file actually landed on disk.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 persisted record file, got %d", len(entries))
	}

	// Instance B (a brand-new Executor + store at the SAME dir) must observe
	// A's record and return a cache hit without re-running any step.
	execB := NewExecutor().
		WithIdempotencyKey("transfer-D").
		WithIdempotencyStore(NewFileIdempotencyStore(dir, 0))
	outB, resultsB, traceB, err := execB.ExecuteWithTrace(context.Background(), wf, reg, nil)
	if !errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("instance B: expected ErrIdempotencyHit from persisted record, got: %v", err)
	}
	if counter != 2 {
		t.Fatalf("instance B: counter must stay at 2 (no re-execution), got %d", counter)
	}
	if outB != outA {
		t.Fatalf("instance B: expected cached output %q, got %q", outA, outB)
	}
	if len(resultsB) != 0 {
		t.Fatalf("instance B: expected 0 step results on cache hit, got %d", len(resultsB))
	}
	if traceB == nil || !traceB.IdempotencyHit {
		t.Fatalf("instance B: expected trace.IdempotencyHit=true, got %+v", traceB)
	}
	if traceB.RunID != traceA.RunID {
		t.Fatalf("instance B: expected cached RunID %q, got %q", traceA.RunID, traceB.RunID)
	}
}

// TestIdempotency_TTLExpiry verifies that a record older than the store's TTL
// is reaped on the next Check, so the workflow re-executes instead of being
// treated as a completed cache hit.
func TestIdempotency_TTLExpiry(t *testing.T) {
	t.Parallel()

	var counter int
	reg := newIdempTestRegistry(&counter)
	wf := twoStepCounterWorkflow()

	// Short TTL so the test can expire it without slowing the suite. The TTL
	// is comfortably longer than the time between Record and the immediate
	// re-Check below, so the first run's record is fresh when written.
	store := NewFileIdempotencyStore(t.TempDir(), 150*time.Millisecond)
	exec := NewExecutor().WithIdempotencyKey("transfer-E").WithIdempotencyStore(store)

	if _, _, _, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil); err != nil {
		t.Fatalf("run1: expected success, got: %v", err)
	}
	if counter != 2 {
		t.Fatalf("run1: expected counter=2, got %d", counter)
	}

	// Immediately, a second run is a cache hit (record is fresh).
	if _, _, _, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil); !errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("run2 (fresh): expected ErrIdempotencyHit, got: %v", err)
	}
	if counter != 2 {
		t.Fatalf("run2 (fresh): counter should still be 2, got %d", counter)
	}

	// Wait past the TTL, then re-trigger: the record is reaped and the
	// workflow re-executes.
	time.Sleep(250 * time.Millisecond)
	out3, _, trace3, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("run3 (expired): expected success after TTL expiry, got: %v", err)
	}
	if errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("run3 (expired): should not be a cache hit after TTL expiry")
	}
	if counter != 4 {
		t.Fatalf("run3 (expired): expected counter=4 (re-executed), got %d", counter)
	}
	if out3 != "call-4" {
		t.Fatalf("run3 (expired): expected output \"call-4\", got %q", out3)
	}
	if trace3 == nil || trace3.IdempotencyHit {
		t.Fatalf("run3 (expired): expected a fresh execution trace, got %+v", trace3)
	}

	// After re-execution a new record was written, so a follow-up is a hit.
	if _, _, _, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil); !errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("run4 (post-refresh): expected ErrIdempotencyHit, got: %v", err)
	}
	if counter != 4 {
		t.Fatalf("run4 (post-refresh): counter should stay at 4, got %d", counter)
	}
}

// Compile-time check that *FileIdempotencyStore satisfies IdempotencyStore.
var _ IdempotencyStore = (*FileIdempotencyStore)(nil)

// idempBlockingCounterNode increments a shared atomic counter on Execute and
// then blocks until the release channel is closed. It lets a concurrency test
// hold a run in the in_progress state long enough for rival runs to observe it
// and be rejected, modelling a side-effecting node (e.g. a slow money
// transfer) that must not fire twice.
type idempBlockingCounterNode struct {
	name    string
	counter *atomic.Int32
	release <-chan struct{}
}

func (n *idempBlockingCounterNode) Name() string { return n.name }
func (n *idempBlockingCounterNode) Description() string {
	return "blocking counter node for idempotency concurrency test"
}
func (n *idempBlockingCounterNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Input: "string", Output: "string"}
}

func (n *idempBlockingCounterNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	n.counter.Add(1)
	select {
	case <-n.release:
		return fmt.Sprintf("call-%d", n.counter.Load()), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// idempFailNode always fails. It is used to drive a workflow into the
// status=failed idempotency branch so a subsequent retry can be exercised.
type idempFailNode struct {
	name string
}

func (n *idempFailNode) Name() string        { return n.name }
func (n *idempFailNode) Description() string { return "failing node for idempotency test" }
func (n *idempFailNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Input: "string", Output: "string"}
}

func (n *idempFailNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	return "", errors.New("intentional failure")
}

// TestIdempotency_ConcurrentSameKeyExecutesOnce verifies the core fix for
// C-2: when N goroutines trigger the same idempotency key simultaneously, the
// side-effecting node runs EXACTLY ONCE. The other N-1 runs are rejected with
// ErrIdempotencyInProgress (or served as a cache hit once the winner finishes).
// Before the Reserve placeholder, all N would have observed "not found" and all
// N would have executed — i.e. duplicate side effects (duplicate charges).
func TestIdempotency_ConcurrentSameKeyExecutesOnce(t *testing.T) {
	t.Parallel()

	var counter atomic.Int32
	release := make(chan struct{})
	reg := nodes.NewRegistry()
	reg.Register(&idempBlockingCounterNode{name: "blocker", counter: &counter, release: release})
	wf := &Workflow{Name: "idemp-concurrent-wf", Steps: []WorkflowStep{{Node: "blocker", Name: "s1"}}}
	// In-memory store: its sync.Mutex serialises the goroutines' Reserve calls
	// so exactly one wins the in_progress placeholder.
	store := NewMemoryIdempotencyStore(0)

	const n = 10
	type result struct{ err error }
	resultCh := make(chan result, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		exec := NewExecutor().WithIdempotencyKey("transfer-concurrent").WithIdempotencyStore(store)
		go func() {
			<-start
			_, _, _, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil)
			resultCh <- result{err}
		}()
	}
	close(start) // release all goroutines simultaneously

	// Collect the N-1 losers. The winner is blocked inside the blocking node
	// waiting on `release`, so it cannot return until we close it below.
	losers := 0
	loseTimeout := time.After(3 * time.Second)
	for losers < n-1 {
		select {
		case r := <-resultCh:
			if r.err == nil {
				t.Fatalf("a run completed before release; only the winner should run, got a nil error from a loser")
			}
			if !errors.Is(r.err, ErrIdempotencyInProgress) && !errors.Is(r.err, ErrIdempotencyHit) {
				t.Fatalf("expected ErrIdempotencyInProgress/Hit for a losing run, got: %v", r.err)
			}
			losers++
		case <-loseTimeout:
			t.Fatalf("timed out waiting for %d losers, got %d", n-1, losers)
		}
	}

	// Release the winner so it can finish and record its completed outcome.
	close(release)

	var winnerErr error
	select {
	case r := <-resultCh:
		winnerErr = r.err
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for the winning run to finish")
	}
	if winnerErr != nil {
		t.Fatalf("winner: expected success, got: %v", winnerErr)
	}

	if got := counter.Load(); got != 1 {
		t.Fatalf("expected the side-effecting node to execute exactly once (counter=1), got %d", got)
	}
	if losers != n-1 {
		t.Fatalf("expected %d losing runs, got %d", n-1, losers)
	}
}

// TestIdempotency_ReserveRejectsConcurrent exercises the Reserve contract
// directly (independent of the Executor) against both store implementations:
// the first Reserve claims the placeholder, the second is rejected; a
// completed record becomes a cache hit; a failed record allows a retry.
func TestIdempotency_ReserveRejectsConcurrent(t *testing.T) {
	t.Parallel()

	stores := []struct {
		name  string
		build func() IdempotencyStore
	}{
		{"memory", func() IdempotencyStore { return NewMemoryIdempotencyStore(0) }},
		{"file", func() IdempotencyStore { return NewFileIdempotencyStore(t.TempDir(), 0) }},
	}
	for _, tc := range stores {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store := tc.build()
			const key = "reserve-concurrent"

			// First Reserve wins and stamps an in_progress placeholder.
			rec1, ok1, err1 := store.Reserve(key, "run-1")
			if err1 != nil || !ok1 {
				t.Fatalf("first Reserve: expected (rec, true, nil), got (%+v, %v, %v)", rec1, ok1, err1)
			}
			if rec1.Status != idempotencyStatusInProgress || rec1.RunID != "run-1" {
				t.Fatalf("first Reserve: unexpected record %+v", rec1)
			}

			// Second Reserve loses: the existing in_progress blocks it.
			rec2, ok2, err2 := store.Reserve(key, "run-2")
			if ok2 {
				t.Fatalf("second Reserve: expected reserved=false, got true")
			}
			if !errors.Is(err2, ErrIdempotencyInProgress) {
				t.Fatalf("second Reserve: expected ErrIdempotencyInProgress, got: %v", err2)
			}
			if rec2.RunID != "run-1" {
				t.Fatalf("second Reserve: expected the existing run_id run-1, got %q", rec2.RunID)
			}

			// Completing the run makes a follow-up Reserve a cache hit.
			if err := store.Record(IdempotencyRecord{Key: key, RunID: "run-1", Status: idempotencyStatusCompleted, FinalOutput: "done"}); err != nil {
				t.Fatalf("Record completed: %v", err)
			}
			rec3, ok3, err3 := store.Reserve(key, "run-3")
			if err3 != nil || ok3 {
				t.Fatalf("post-completed Reserve: expected (rec, false, nil), got (%+v, %v, %v)", rec3, ok3, err3)
			}
			if rec3.Status != idempotencyStatusCompleted || rec3.FinalOutput != "done" {
				t.Fatalf("post-completed Reserve: unexpected record %+v", rec3)
			}

			// A failed run allows a retry: Reserve overwrites and re-claims.
			if err := store.Record(IdempotencyRecord{Key: key, RunID: "run-1", Status: idempotencyStatusFailed, Error: "boom"}); err != nil {
				t.Fatalf("Record failed: %v", err)
			}
			rec4, ok4, err4 := store.Reserve(key, "run-4")
			if err4 != nil || !ok4 {
				t.Fatalf("post-failed Reserve: expected (rec, true, nil), got (%+v, %v, %v)", rec4, ok4, err4)
			}
			if rec4.Status != idempotencyStatusInProgress || rec4.RunID != "run-4" {
				t.Fatalf("post-failed Reserve: unexpected record %+v", rec4)
			}
		})
	}
}

// TestIdempotency_InProgressBlocksSecondRun verifies that while a run is
// in_progress, a second trigger of the same key is rejected without executing
// any side-effecting node.
func TestIdempotency_InProgressBlocksSecondRun(t *testing.T) {
	t.Parallel()

	var counter int
	reg := newIdempTestRegistry(&counter)
	wf := twoStepCounterWorkflow()
	store := NewMemoryIdempotencyStore(0)
	exec := NewExecutor().WithIdempotencyKey("transfer-inprogress").WithIdempotencyStore(store)

	// Simulate another run being mid-flight by Reserving an in_progress
	// placeholder directly, bypassing execution.
	rec, ok, err := store.Reserve("transfer-inprogress", "run-other")
	if err != nil || !ok {
		t.Fatalf("Reserve: expected (rec, true, nil), got (%v, %v)", ok, err)
	}
	if rec.RunID != "run-other" {
		t.Fatalf("Reserve: expected run_id run-other, got %q", rec.RunID)
	}

	// A second trigger with the same key must NOT execute: it is rejected.
	out, results, trace, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil)
	if !errors.Is(err, ErrIdempotencyInProgress) {
		t.Fatalf("expected ErrIdempotencyInProgress, got: %v", err)
	}
	if counter != 0 {
		t.Fatalf("expected counter=0 (no side-effecting execution), got %d", counter)
	}
	if out != "" {
		t.Fatalf("expected empty output, got %q", out)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 step results, got %d", len(results))
	}
	if trace == nil {
		t.Fatalf("expected a non-nil trace")
	}
	if trace.RunID != "run-other" {
		t.Fatalf("expected trace.RunID=run-other, got %q", trace.RunID)
	}
	if trace.IdempotencyHit {
		t.Fatalf("IdempotencyHit must be false for an in-progress rejection")
	}
}

// TestIdempotency_FailedAllowsRerun verifies that a failed run does not poison
// the key forever: the status=failed record lets a subsequent trigger
// re-Reserve and re-execute (e.g. a retry after a transient error).
func TestIdempotency_FailedAllowsRerun(t *testing.T) {
	t.Parallel()

	store := NewMemoryIdempotencyStore(0)
	exec := NewExecutor().WithIdempotencyKey("transfer-failed").WithIdempotencyStore(store)

	// Run 1: the workflow fails. The Executor records status=failed so the key
	// is not permanently poisoned.
	reg1 := nodes.NewRegistry()
	reg1.Register(&idempFailNode{name: "worker"})
	wf := &Workflow{Name: "idemp-fail-wf", Steps: []WorkflowStep{{Node: "worker", Name: "s1"}}}
	_, _, _, err := exec.ExecuteWithTrace(context.Background(), wf, reg1, nil)
	if err == nil {
		t.Fatalf("run1: expected the workflow to fail, got nil")
	}
	if errors.Is(err, ErrIdempotencyInProgress) || errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("run1: expected a real workflow failure, got an idempotency error %v", err)
	}

	// Verify the failed record was persisted.
	rec, found, _ := store.Check("transfer-failed")
	if !found || rec.Status != idempotencyStatusFailed {
		t.Fatalf("expected a failed record after run1, got found=%v rec=%+v", found, rec)
	}

	// Run 2: same key, but the workflow now succeeds. The failed record allows
	// a retry: Reserve overwrites it and the workflow re-executes.
	var counter int
	reg2 := nodes.NewRegistry()
	reg2.Register(&idempCounterNode{name: "worker", counter: &counter})
	out2, _, trace2, err2 := exec.ExecuteWithTrace(context.Background(), wf, reg2, nil)
	if err2 != nil {
		t.Fatalf("run2: expected success after a failed run, got: %v", err2)
	}
	if counter != 1 {
		t.Fatalf("run2: expected counter=1 (re-executed after failure), got %d", counter)
	}
	if out2 != "call-1" {
		t.Fatalf("run2: expected output call-1, got %q", out2)
	}
	if trace2 == nil || trace2.IdempotencyHit {
		t.Fatalf("run2: expected a fresh execution trace, got %+v", trace2)
	}

	// Run 3: the completed record now makes a repeat a cache hit.
	if _, _, _, err := exec.ExecuteWithTrace(context.Background(), wf, reg2, nil); !errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("run3: expected ErrIdempotencyHit, got: %v", err)
	}
	if counter != 1 {
		t.Fatalf("run3: expected counter to stay 1 (cache hit), got %d", counter)
	}
}

// Compile-time check that *MemoryIdempotencyStore satisfies IdempotencyStore.
var _ IdempotencyStore = (*MemoryIdempotencyStore)(nil)

// TestIdempotency_TamperedRecordRejected verifies the H-4 fix: when an HMAC
// key is configured, a persisted completed record whose FinalOutput has been
// rewritten on disk (with the stale HMAC left in place) is rejected by Check —
// it is treated as not-found and the workflow re-executes instead of returning
// the forged cached result. This is the financial-tampering threat model: an
// attacker who can modify the idempotency file must NOT be able to inject a
// fake transfer confirmation that the next trigger serves as a cache hit.
//
// This test does not call t.Parallel because it sets LLM_BOX_AUDIT_HMAC_KEY
// (the shared idempotency HMAC key source) via t.Setenv, which is incompatible
// with parallel tests.
func TestIdempotency_TamperedRecordRejected(t *testing.T) {
	t.Setenv("LLM_BOX_AUDIT_HMAC_KEY", "idemp-tamper-key")

	var counter int
	reg := newIdempTestRegistry(&counter)
	wf := twoStepCounterWorkflow()
	dir := t.TempDir()
	store := NewFileIdempotencyStore(dir, 0)
	exec := NewExecutor().WithIdempotencyKey("transfer-tamper").WithIdempotencyStore(store)

	// Run 1: executes both steps and persists a signed completed record.
	out1, _, _, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("run1: expected success, got: %v", err)
	}
	if counter != 2 {
		t.Fatalf("run1: expected counter=2, got %d", counter)
	}
	if out1 != "call-2" {
		t.Fatalf("run1: expected output \"call-2\", got %q", out1)
	}

	// Run 2 (pre-tamper): the signed record is a valid cache hit.
	if _, _, _, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil); !errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("run2 (pre-tamper): expected ErrIdempotencyHit, got: %v", err)
	}
	if counter != 2 {
		t.Fatalf("run2 (pre-tamper): counter should stay 2, got %d", counter)
	}

	// Read the persisted record (Check verifies it, so it must be returned
	// intact with a populated HMAC), then rewrite it to disk with a FORGED
	// FinalOutput while keeping the stale HMAC. This bypasses the store's
	// signing path to simulate an on-disk tamper.
	rec, found, err := store.Check("transfer-tamper")
	if err != nil || !found {
		t.Fatalf("Check before tamper: found=%v err=%v", found, err)
	}
	if rec.HMAC == "" {
		t.Fatalf("expected a signed record (non-empty HMAC) when key is configured")
	}
	tampered := rec
	tampered.FinalOutput = "FORGED-transfer-999999"
	// tampered.HMAC intentionally left as the stale value from the original
	// record — this is the tampering the MAC is meant to detect.
	data, err := json.MarshalIndent(tampered, "", "  ")
	if err != nil {
		t.Fatalf("marshal tampered: %v", err)
	}
	path := store.pathFor("transfer-tamper")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write tampered: %v", err)
	}

	// Run 3 (post-tamper): Check must reject the tampered record (HMAC no
	// longer matches) and treat it as not-found, so the workflow re-executes
	// instead of returning the forged output.
	out3, _, trace3, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("run3 (post-tamper): expected re-execution success, got: %v", err)
	}
	if errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("run3 (post-tamper): tampered record must NOT be served as a cache hit")
	}
	if counter != 4 {
		t.Fatalf("run3 (post-tamper): expected counter=4 (re-executed), got %d", counter)
	}
	if out3 != "call-4" {
		t.Fatalf("run3 (post-tamper): expected output \"call-4\", got %q", out3)
	}
	if trace3 == nil || trace3.IdempotencyHit {
		t.Fatalf("run3 (post-tamper): expected a fresh execution trace, got %+v", trace3)
	}

	// Run 4: the re-execution wrote a fresh signed record, so a follow-up is
	// a valid cache hit again (proving the store still functions after the
	// rejected tamper).
	if _, _, _, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil); !errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("run4 (post-refresh): expected ErrIdempotencyHit, got: %v", err)
	}
	if counter != 4 {
		t.Fatalf("run4 (post-refresh): counter should stay 4, got %d", counter)
	}
}

// TestIdempotency_NoHMACKeyDegradesGracefully verifies the graceful-degrade
// branch of H-4: when neither LLM_BOX_AUDIT_HMAC_KEY nor
// LLM_BOX_SECRETS_PASSWORD is set, signing is skipped (HMAC left empty) and
// verification is skipped on read, so records still round-trip and cache hits
// still work. The workflow is never blocked by the absence of a key.
//
// Not parallel: it sets env vars via t.Setenv.
func TestIdempotency_NoHMACKeyDegradesGracefully(t *testing.T) {
	// Force both key sources empty so signing/verification degrade to no-ops.
	t.Setenv("LLM_BOX_AUDIT_HMAC_KEY", "")
	t.Setenv("LLM_BOX_SECRETS_PASSWORD", "")

	var counter int
	reg := newIdempTestRegistry(&counter)
	wf := twoStepCounterWorkflow()
	store := NewFileIdempotencyStore(t.TempDir(), 0)
	exec := NewExecutor().WithIdempotencyKey("transfer-nokey").WithIdempotencyStore(store)

	// Run 1: executes and persists a record with an empty HMAC.
	if _, _, _, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil); err != nil {
		t.Fatalf("run1: expected success, got: %v", err)
	}
	if counter != 2 {
		t.Fatalf("run1: expected counter=2, got %d", counter)
	}

	// Run 2: cache hit despite no HMAC key (verification skipped, so the
	// empty-HMAC record is trusted).
	if _, _, trace2, err := exec.ExecuteWithTrace(context.Background(), wf, reg, nil); !errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("run2: expected ErrIdempotencyHit, got: %v", err)
	} else if trace2 == nil || !trace2.IdempotencyHit {
		t.Fatalf("run2: expected IdempotencyHit=true, got %+v", trace2)
	}
	if counter != 2 {
		t.Fatalf("run2: counter should stay 2, got %d", counter)
	}

	// The persisted record carries an empty HMAC field (graceful degrade).
	rec, found, err := store.Check("transfer-nokey")
	if err != nil || !found {
		t.Fatalf("Check: found=%v err=%v", found, err)
	}
	if rec.HMAC != "" {
		t.Errorf("expected empty HMAC when no key configured, got %q", rec.HMAC)
	}
}
