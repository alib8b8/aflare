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
	"sync"
	"time"
)

// circuit_breaker.go — 节点级熔断器
//
// 借鉴 Grok Build 自研熔断器思路，为分布式 Coordinator 增加节点级熔断保护。
// 当某 Worker 节点连续失败达阈值时，熔断器跳闸（Open），selectBestNode 跳过该节点；
// 冷却期后进入半开（HalfOpen）允许试探一个请求，成功则恢复（Closed），失败则继续熔断。
//
// 状态机：
//   Closed --连续 N 次失败--> Open --冷却期满--> HalfOpen
//   HalfOpen --探测成功--> Closed
//   HalfOpen --探测失败--> Open

// BreakerState 熔断器状态
type BreakerState int

const (
	BreakerClosed BreakerState = iota
	BreakerOpen
	BreakerHalfOpen
)

// CircuitBreaker 单个节点的熔断器
type CircuitBreaker struct {
	mu sync.Mutex

	state           BreakerState
	failures        int       // 当前连续失败次数（Closed 状态下累计）
	consecSuccesses int       // HalfOpen 状态下连续成功次数
	lastFailure     time.Time // 最近一次失败时间（用于计算冷却）
	trippedAt       time.Time // 熔断跳闸时间
	// halfOpenProbeInFlight 标记 HalfOpen 状态下是否已有探测请求在途。
	// 半开期间仅允许一个探测请求，避免恢复期被并发请求冲击。
	halfOpenProbeInFlight bool

	// 可调参数
	failureThreshold int           // 跳闸阈值
	cooldown         time.Duration // 冷却时长（Open -> HalfOpen）
	successThreshold int           // 半开恢复阈值（HalfOpen -> Closed）
}

// NewCircuitBreaker 创建熔断器，使用默认参数
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		state:            BreakerClosed,
		failureThreshold: 5,
		cooldown:         30 * time.Second,
		successThreshold: 2,
	}
}

// AllowRequest 判断当前是否允许向该节点发请求（调用方在 selectBestNode 时使用）
// 返回 true 表示放行，false 表示熔断中。
// 该方法会处理 Open -> HalfOpen 的状态转换（冷却期满自动进入半开）。
func (cb *CircuitBreaker) AllowRequest() bool {
	if cb == nil {
		return true
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case BreakerClosed:
		return true
	case BreakerOpen:
		// 冷却期满，转为半开，放行一个探测请求
		if time.Since(cb.trippedAt) >= cb.cooldown {
			cb.state = BreakerHalfOpen
			cb.consecSuccesses = 0
			cb.halfOpenProbeInFlight = true
			return true
		}
		return false
	case BreakerHalfOpen:
		// 半开状态仅允许一个探测请求在途，避免恢复期被并发请求冲击
		if cb.halfOpenProbeInFlight {
			return false
		}
		cb.halfOpenProbeInFlight = true
		return true
	}
	return true
}

// RecordSuccess 记录一次成功
func (cb *CircuitBreaker) RecordSuccess() {
	if cb == nil {
		return
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case BreakerClosed:
		cb.failures = 0
	case BreakerHalfOpen:
		cb.halfOpenProbeInFlight = false
		cb.consecSuccesses++
		if cb.consecSuccesses >= cb.successThreshold {
			// 恢复
			cb.state = BreakerClosed
			cb.failures = 0
			cb.consecSuccesses = 0
		}
	}
}

// RecordFailure 记录一次失败，返回失败后是否触发跳闸
func (cb *CircuitBreaker) RecordFailure() bool {
	if cb == nil {
		return false
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailure = time.Now()

	switch cb.state {
	case BreakerClosed:
		cb.failures++
		if cb.failures >= cb.failureThreshold {
			cb.state = BreakerOpen
			cb.trippedAt = time.Now()
			return true
		}
	case BreakerHalfOpen:
		// 半开探测失败，重新熔断
		cb.halfOpenProbeInFlight = false
		cb.state = BreakerOpen
		cb.trippedAt = time.Now()
		cb.consecSuccesses = 0
		return true
	}
	return false
}

// State 返回当前状态（用于展示/调试）
func (cb *CircuitBreaker) State() BreakerState {
	if cb == nil {
		return BreakerClosed
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Stats 返回熔断器统计快照
type BreakerStats struct {
	State       BreakerState `json:"state"`
	Failures    int          `json:"failures"`
	LastFailure *time.Time   `json:"last_failure,omitempty"`
	TrippedAt   *time.Time   `json:"tripped_at,omitempty"`
	IsTripped   bool         `json:"is_tripped"`
}

// Stats 返回熔断器统计快照
func (cb *CircuitBreaker) Stats() BreakerStats {
	if cb == nil {
		return BreakerStats{State: BreakerClosed}
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()
	s := BreakerStats{
		State:     cb.state,
		Failures:  cb.failures,
		IsTripped: cb.state == BreakerOpen,
	}
	if !cb.lastFailure.IsZero() {
		lf := cb.lastFailure
		s.LastFailure = &lf
	}
	if !cb.trippedAt.IsZero() {
		ta := cb.trippedAt
		s.TrippedAt = &ta
	}
	return s
}

// ------------------------------------------------------------
// BreakerRegistry — Coordinator 持有的按节点熔断器注册表
// ------------------------------------------------------------

// BreakerRegistry 管理所有节点的熔断器
type BreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
}

// NewBreakerRegistry 创建注册表
func NewBreakerRegistry() *BreakerRegistry {
	return &BreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// getOrCreate 返回（或创建）指定节点的熔断器
func (r *BreakerRegistry) getOrCreate(nodeID string) *CircuitBreaker {
	r.mu.RLock()
	cb, ok := r.breakers[nodeID]
	r.mu.RUnlock()
	if ok {
		return cb
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// 双检锁
	if cb, ok := r.breakers[nodeID]; ok {
		return cb
	}
	cb = NewCircuitBreaker()
	r.breakers[nodeID] = cb
	return cb
}

// AllowRequest 判断节点是否可用
func (r *BreakerRegistry) AllowRequest(nodeID string) bool {
	if r == nil {
		return true
	}
	return r.getOrCreate(nodeID).AllowRequest()
}

// RecordSuccess 记录节点成功
func (r *BreakerRegistry) RecordSuccess(nodeID string) {
	if r == nil {
		return
	}
	r.getOrCreate(nodeID).RecordSuccess()
}

// RecordFailure 记录节点失败
func (r *BreakerRegistry) RecordFailure(nodeID string) bool {
	if r == nil {
		return false
	}
	return r.getOrCreate(nodeID).RecordFailure()
}

// Remove 移除节点熔断器（节点注销时调用）
func (r *BreakerRegistry) Remove(nodeID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.breakers, nodeID)
}

// StatsAll 返回所有节点熔断器状态
func (r *BreakerRegistry) StatsAll() map[string]BreakerStats {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]BreakerStats, len(r.breakers))
	for id, cb := range r.breakers {
		out[id] = cb.Stats()
	}
	return out
}
