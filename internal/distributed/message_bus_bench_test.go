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

package distributed

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

// drainBusChannel continuously drains a subscriber channel until stopCh is
// closed. It keeps subscriber buffers empty so Publish's non-blocking sends
// succeed (mimicking active consumers), isolating the distribution cost.
func drainBusChannel(ch <-chan BusMessage, stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ch:
		case <-stopCh:
			return
		}
	}
}

// benchBusWithSubscribers builds a MessageBus with n subscribers on the same
// topic plus background drainers. The returned stop func must be called to
// release the drainers.
func benchBusWithSubscribers(b *testing.B, topic string, n int) (*MessageBus, func()) {
	b.Helper()
	bus := NewMessageBus("bench-node", "0")
	bus.SetMaxPending(0) // disable backpressure for throughput benchmarks

	stopCh := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		ch := bus.Subscribe(topic)
		wg.Add(1)
		go drainBusChannel(ch, stopCh, &wg)
	}
	return bus, func() {
		close(stopCh)
		wg.Wait()
	}
}

// BenchmarkPublish measures local Publish throughput at varying subscriber
// counts. Publish only fans out to local subscribers (no network).
func BenchmarkPublish(b *testing.B) {
	for _, n := range []int{1, 10, 50} {
		b.Run("subscribers_"+strconv.Itoa(n), func(b *testing.B) {
			bus, stop := benchBusWithSubscribers(b, "bench-topic", n)
			defer stop()

			msg := BusMessage{Topic: "bench-topic", Content: "hello", Type: "text"}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := bus.Publish(msg); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkBroadcast measures Broadcast throughput with no remote peers
// (local fan-out only). Remote peer delivery is network-bound (HTTP) and is
// intentionally not benchmarked; with zero peers Broadcast degrades to a
// local Publish plus hop-limit bookkeeping.
func BenchmarkBroadcast(b *testing.B) {
	for _, n := range []int{1, 10, 50} {
		b.Run("subscribers_"+strconv.Itoa(n), func(b *testing.B) {
			bus, stop := benchBusWithSubscribers(b, "bench-topic", n)
			defer stop()

			msg := BusMessage{Topic: "bench-topic", Content: "hello", Type: "text"}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := bus.Broadcast(msg); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkPublish_Parallel measures Publish throughput under concurrent
// publishers sharing one bus with a single subscriber, exercising the
// subscriber-map read lock contention path.
func BenchmarkPublish_Parallel(b *testing.B) {
	bus, stop := benchBusWithSubscribers(b, "bench-topic", 1)
	defer stop()

	msg := BusMessage{Topic: "bench-topic", Content: "hello", Type: "text"}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := bus.Publish(msg); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkSignBusMessage measures the SHA-256 message signing that runs on
// every Publish/Broadcast/SendTo when the signature is unset.
func BenchmarkSignBusMessage(b *testing.B) {
	msg := BusMessage{
		From:      "node-1",
		To:        "node-2",
		Topic:     "bench-topic",
		Content:   "hello world",
		Type:      "text",
		Timestamp: time.Now(),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = signBusMessage(msg)
	}
}
