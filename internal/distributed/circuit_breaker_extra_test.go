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
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestCircuitBreaker_HalfOpenProbeRetry 覆盖 AllowRequest 中 HalfOpen 状态下
// halfOpenProbeInFlight=false 的分支:前一次探测成功后(in-flight 置回 false),
// 下一次 AllowRequest 应再次放行并将 in-flight 置 true。
func TestCircuitBreaker_HalfOpenProbeRetry(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker()
	cb.cooldown = 10 * time.Millisecond
	cb.successThreshold = 3 // 需要 3 次成功才恢复

	// 累计失败达阈值,跳闸到 Open
	for i := 0; i < cb.failureThreshold; i++ {
		cb.RecordFailure()
	}
	if cb.State() != BreakerOpen {
		t.Fatalf("expected Open, got %v", cb.State())
	}

	// 冷却期满,进入 HalfOpen 并放行第一个探测
	time.Sleep(20 * time.Millisecond)
	if !cb.AllowRequest() {
		t.Fatal("should allow probe after cooldown")
	}
	if cb.State() != BreakerHalfOpen {
		t.Fatal("expected HalfOpen")
	}

	// 第二次请求:in-flight=true,应被拒绝
	if cb.AllowRequest() {
		t.Error("should block second probe while first is in flight")
	}

	// 第一个探测成功:in-flight=false, consecSuccesses=1
	cb.RecordSuccess()

	// 此处覆盖之前未覆盖的分支:HalfOpen + in-flight=false -> 放行并置 in-flight=true
	if !cb.AllowRequest() {
		t.Error("should allow second probe after first success in HalfOpen")
	}

	// 第二个探测仍在途,第三个应被拒绝
	if cb.AllowRequest() {
		t.Error("should block third probe while second is in flight")
	}

	// 第二个探测成功:consecSuccesses=2 (仍 < 3,未恢复)
	cb.RecordSuccess()
	if cb.State() != BreakerHalfOpen {
		t.Errorf("expected still HalfOpen after 2 successes, got %v", cb.State())
	}

	// 第三个探测
	if !cb.AllowRequest() {
		t.Error("should allow third probe after second success")
	}

	// 第三个探测成功:consecSuccesses=3 >= successThreshold,恢复到 Closed
	cb.RecordSuccess()
	if cb.State() != BreakerClosed {
		t.Errorf("expected Closed after %d successes, got %v", cb.successThreshold, cb.State())
	}
}

// TestCircuitBreaker_OpenBeforeCooldown 显式测试 Open 状态下冷却未满时拒绝请求。
func TestCircuitBreaker_OpenBeforeCooldown(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker()
	cb.cooldown = 1 * time.Hour // 远大于测试时长

	for i := 0; i < cb.failureThreshold; i++ {
		cb.RecordFailure()
	}
	if cb.State() != BreakerOpen {
		t.Fatal("expected Open")
	}
	if cb.AllowRequest() {
		t.Error("should reject request in Open state before cooldown expires")
	}
}

// TestCircuitBreaker_RecordSuccessInOpenState 在 Open 状态下调用 RecordSuccess 应为空操作。
func TestCircuitBreaker_RecordSuccessInOpenState(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker()
	cb.cooldown = 1 * time.Hour

	for i := 0; i < cb.failureThreshold; i++ {
		cb.RecordFailure()
	}
	if cb.State() != BreakerOpen {
		t.Fatal("expected Open")
	}

	// Open 状态下 RecordSuccess 不应改变状态
	cb.RecordSuccess()
	if cb.State() != BreakerOpen {
		t.Errorf("expected still Open after RecordSuccess in Open, got %v", cb.State())
	}
}

// TestCircuitBreaker_RecordFailureInOpenState 在 Open 状态下调用 RecordFailure
// 不应累加失败次数或改变状态(switch 无 BreakerOpen case)。
func TestCircuitBreaker_RecordFailureInOpenState(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker()
	cb.cooldown = 1 * time.Hour

	for i := 0; i < cb.failureThreshold; i++ {
		cb.RecordFailure()
	}
	statsBefore := cb.Stats()

	// Open 状态下再次记录失败,不应跳闸(已经 Open)也不应改变 failures
	if tripped := cb.RecordFailure(); tripped {
		t.Error("should not report trip when already Open")
	}
	statsAfter := cb.Stats()
	if statsAfter.State != BreakerOpen {
		t.Errorf("expected still Open, got %v", statsAfter.State)
	}
	if statsAfter.Failures != statsBefore.Failures {
		t.Errorf("failures should not change in Open state: before=%d after=%d",
			statsBefore.Failures, statsAfter.Failures)
	}
}

// TestCircuitBreaker_Concurrent 并发读写熔断器,验证 -race 下无数据竞争。
func TestCircuitBreaker_Concurrent(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker()
	cb.cooldown = 5 * time.Millisecond
	cb.failureThreshold = 10
	cb.successThreshold = 2

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if cb.AllowRequest() {
					if i%3 == 0 {
						cb.RecordFailure()
					} else {
						cb.RecordSuccess()
					}
				}
				_ = cb.State()
				_ = cb.Stats()
			}
		}(i)
	}
	wg.Wait()
}

// TestBreakerRegistry_Concurrent 并发操作注册表,验证 getOrCreate 双检锁与 -race 安全。
func TestBreakerRegistry_Concurrent(t *testing.T) {
	t.Parallel()
	reg := NewBreakerRegistry()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// 使用 i%5 让多个 goroutine 竞争同一 nodeID,触发 getOrCreate 双检锁
			nodeID := fmt.Sprintf("node-%d", i%5)
			for j := 0; j < 50; j++ {
				reg.AllowRequest(nodeID)
				reg.RecordSuccess(nodeID)
				reg.RecordFailure(nodeID)
			}
		}(i)
	}
	wg.Wait()

	stats := reg.StatsAll()
	if len(stats) != 5 {
		t.Errorf("expected 5 breakers, got %d", len(stats))
	}

	// 并发移除与查询
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			reg.Remove(fmt.Sprintf("node-%d", i))
			reg.AllowRequest(fmt.Sprintf("node-%d", i))
		}(i)
	}
	wg.Wait()
}

// TestBreakerRegistry_StatsAllEmpty 空注册表 StatsAll 返回空(非 nil)map。
func TestBreakerRegistry_StatsAllEmpty(t *testing.T) {
	t.Parallel()
	reg := NewBreakerRegistry()
	stats := reg.StatsAll()
	if stats == nil {
		t.Error("expected non-nil empty map")
	}
	if len(stats) != 0 {
		t.Errorf("expected 0 breakers, got %d", len(stats))
	}
}

// TestBreakerRegistry_RemoveNonexistent 移除不存在的节点不应 panic。
func TestBreakerRegistry_RemoveNonexistent(t *testing.T) {
	t.Parallel()
	reg := NewBreakerRegistry()
	reg.Remove("nonexistent")
	// 验证注册表仍可用
	if !reg.AllowRequest("new-node") {
		t.Error("should allow request for new node after removing nonexistent")
	}
}

// TestCircuitBreaker_FullStateTransition 覆盖完整状态机:
// Closed -> Open -> HalfOpen -> Open -> HalfOpen -> Closed。
func TestCircuitBreaker_FullStateTransition(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker()
	cb.cooldown = 10 * time.Millisecond

	// Closed -> Open
	for i := 0; i < cb.failureThreshold; i++ {
		cb.RecordFailure()
	}
	if cb.State() != BreakerOpen {
		t.Fatalf("expected Open, got %v", cb.State())
	}

	// Open -> HalfOpen (冷却期满)
	time.Sleep(20 * time.Millisecond)
	if !cb.AllowRequest() {
		t.Fatal("should allow probe after cooldown")
	}
	if cb.State() != BreakerHalfOpen {
		t.Fatal("expected HalfOpen")
	}

	// HalfOpen -> Open (探测失败)
	if tripped := cb.RecordFailure(); !tripped {
		t.Error("should re-trip on HalfOpen failure")
	}
	if cb.State() != BreakerOpen {
		t.Fatalf("expected Open after HalfOpen failure, got %v", cb.State())
	}

	// Open -> HalfOpen (再次冷却期满)
	time.Sleep(20 * time.Millisecond)
	if !cb.AllowRequest() {
		t.Fatal("should allow probe after second cooldown")
	}
	if cb.State() != BreakerHalfOpen {
		t.Fatal("expected HalfOpen after second cooldown")
	}

	// HalfOpen -> Closed (连续成功达阈值)
	// 上方 AllowRequest 已将 halfOpenProbeInFlight 置 true,首个 RecordSuccess 对应这次探测。
	// 之后每次成功后若仍处于 HalfOpen,需再次 AllowRequest 放行下一探测。
	for i := 0; i < cb.successThreshold; i++ {
		cb.RecordSuccess()
		if cb.State() == BreakerHalfOpen {
			if !cb.AllowRequest() {
				t.Fatal("should allow next probe after success in HalfOpen")
			}
		}
	}
	if cb.State() != BreakerClosed {
		t.Errorf("expected Closed after recovery, got %v", cb.State())
	}
}
