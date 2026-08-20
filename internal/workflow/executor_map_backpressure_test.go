// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/nodes"
)

// TestBackpressurePool_BlockMode submits more items than the queue capacity
// with a slow consumer and verifies every item is eventually delivered:
// block mode must never drop.
func TestBackpressurePool_BlockMode(t *testing.T) {
	pool := newBackpressurePool(2, "block")

	var mu sync.Mutex
	var got []int
	consumed := make(chan struct{})
	go func() {
		defer close(consumed)
		for mi := range pool.queue {
			time.Sleep(2 * time.Millisecond) // slow consumer
			mu.Lock()
			got = append(got, mi.idx)
			mu.Unlock()
		}
	}()

	const total = 10
	for i := 0; i < total; i++ {
		if !pool.submit(mapItem{idx: i, item: "x"}) {
			t.Fatalf("block mode dropped item %d", i)
		}
	}
	pool.close()
	<-consumed

	mu.Lock()
	defer mu.Unlock()
	if len(got) != total {
		t.Fatalf("block mode delivered %d items, want %d", len(got), total)
	}
	if atomic.LoadInt64(&pool.drops) != 0 {
		t.Errorf("block mode counted %d drops, want 0", pool.drops)
	}
}

// TestBackpressurePool_DropMode verifies drop mode returns false once the
// bounded queue is full, counts the drops, and still delivers the items
// that were accepted before close.
func TestBackpressurePool_DropMode(t *testing.T) {
	pool := newBackpressurePool(2, "drop")

	accepted := 0
	for i := 0; i < 5; i++ {
		if pool.submit(mapItem{idx: i, item: "x"}) {
			accepted++
		}
	}
	if accepted != 2 {
		t.Fatalf("drop mode accepted %d items, want 2 (queue capacity)", accepted)
	}
	if got := atomic.LoadInt64(&pool.drops); got != 3 {
		t.Fatalf("drop counter = %d, want 3", got)
	}

	pool.close()
	drained := 0
	for range pool.queue {
		drained++
	}
	if drained != 2 {
		t.Errorf("drained %d queued items, want 2", drained)
	}
}

// slowMapNode blocks long enough to keep the single map worker busy while
// the producer floods the bounded queue.
type slowMapNode struct {
	name    string
	counter *int32
}

func (n *slowMapNode) Name() string        { return n.name }
func (n *slowMapNode) Description() string { return "slow map test node" }
func (n *slowMapNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Description: "slow map test node", Input: "string", Output: "string"}
}
func (n *slowMapNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	atomic.AddInt32(n.counter, 1)
	time.Sleep(50 * time.Millisecond)
	return "ok:" + input, nil
}

// TestMap_DropBackpressureSkipsItems verifies the map executor's drop mode
// end-to-end: with two slow workers and a queue of one, a flood of items is
// partially skipped, the workflow still succeeds, and each dropped slot
// yields an empty output (best-effort semantics). Drop mode only applies
// when Concurrency > 1 (the sequential path bypasses the pool).
func TestMap_DropBackpressureSkipsItems(t *testing.T) {
	reg := nodes.NewRegistry()
	var processed int32
	reg.Register(&slowMapNode{name: "slow", counter: &processed})

	const total = 12
	items := make([]string, total)
	for i := range items {
		items[i] = "x"
	}

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "batch",
				Map: &MapConfig{
					Over:         strings.Join(items, "\n"),
					Concurrency:  2,
					QueueSize:    1,
					Backpressure: "drop",
					Steps: []WorkflowStep{
						{Node: "slow"},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("drop-mode map failed: %v", err)
	}

	var arr []string
	if err := json.Unmarshal([]byte(output), &arr); err != nil {
		t.Fatalf("output not json array: %v (output=%s)", err, output)
	}
	if len(arr) != total {
		t.Fatalf("expected %d output slots, got %d", total, len(arr))
	}

	got := atomic.LoadInt32(&processed)
	if got >= total {
		t.Fatalf("all %d items processed; drop mode skipped nothing", total)
	}
	nonEmpty := 0
	for _, s := range arr {
		if s != "" {
			nonEmpty++
		}
	}
	if int32(nonEmpty) != got {
		t.Errorf("non-empty outputs (%d) != processed items (%d)", nonEmpty, got)
	}
}
