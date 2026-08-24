// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​​‌‌‌‌‌​​‌‌​​‌​‌​​​‌‌‌‌‌​​‌​‌‌​‌​‌‌​‌‌​​​​​‌​‌​​​​​​​​​​​​​​​​​‌​​​​‌‌‌‌‌‌​‌​‌⁠
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
	"sync/atomic"
)

// backpressurePool is a bounded work queue with concurrency control for map
// step execution. It replaces the unbounded-goroutine model (where every item
// spawns a goroutine that blocks on a semaphore) with a fixed-size worker pool
// fed by a bounded channel, so the number of live goroutines never exceeds the
// configured concurrency.
//
// Two modes are supported:
//
//	"block" — submit blocks when the queue is full, providing backpressure
//	          to the producer (upstream step). This is the default.
//	"drop"  — submit returns false when the queue is full, incrementing a
//	          drop counter. The caller skips the item. Suitable for best-
//	          effort monitoring data where losing a sample is acceptable.
type backpressurePool struct {
	queue chan mapItem // bounded work queue
	drops int64        // atomic drop counter (drop mode only)
	mode  string       // "block" or "drop"
}

// mapItem is a single unit of work submitted to the backpressurePool.
type mapItem struct {
	idx  int
	item string
}

// newBackpressurePool creates a bounded work pool. queueSize is the capacity
// of the work queue; mode must be "block" or "drop".
func newBackpressurePool(queueSize int, mode string) *backpressurePool {
	return &backpressurePool{
		queue: make(chan mapItem, queueSize),
		mode:  mode,
	}
}

// submit enqueues an item for processing. In "block" mode it blocks until a
// consumer drains a slot. In "drop" mode it returns false when the queue is
// full, and the caller should skip the item.
func (p *backpressurePool) submit(mi mapItem) bool {
	if p.mode == "drop" {
		select {
		case p.queue <- mi:
			return true
		default:
			atomic.AddInt64(&p.drops, 1)
			return false
		}
	}
	// "block" mode: block until a consumer drains a slot.
	p.queue <- mi
	return true
}

// close signals that no more items will be submitted. Consumers drain the
// remaining items and then exit.
func (p *backpressurePool) close() {
	close(p.queue)
}
