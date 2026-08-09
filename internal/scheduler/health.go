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

// Package scheduler provides cron-based task scheduling and worker health
// monitoring for aflare workflow orchestration.
//
// HealthMonitor implements the JiuwenSwarm-inspired cluster-watch pattern:
// workers (long-running workflow executors, sandbox nodes, etc.) register
// with the monitor and send periodic heartbeats. When a heartbeat timeout
// expires, the monitor declares the worker dead and triggers an auto-restart
// callback. This enables 7×24 production-grade reliability without external
// process supervisors.
package scheduler

import (
	"fmt"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
)

// WorkerStatus represents the health state of a registered worker.
type WorkerStatus int

const (
	// WorkerHealthy means the worker is sending heartbeats on time.
	WorkerHealthy WorkerStatus = iota
	// WorkerUnhealthy means the worker has missed one heartbeat interval
	// but has not yet reached its timeout.
	WorkerUnhealthy
	// WorkerDead means the heartbeat timeout has expired and the worker
	// has been declared dead.
	WorkerDead
)

func (s WorkerStatus) String() string {
	switch s {
	case WorkerHealthy:
		return "healthy"
	case WorkerUnhealthy:
		return "unhealthy"
	case WorkerDead:
		return "dead"
	default:
		return "unknown"
	}
}

// WorkerInfo holds the health state of a registered worker.
type WorkerInfo struct {
	ID            string
	LastHeartbeat time.Time
	Timeout       time.Duration
	// OnDead is called when the heartbeat timeout expires. It receives
	// the worker ID so a single handler can serve multiple workers.
	OnDead func(workerID string)
	// OnRestart is called after OnDead to attempt automatic recovery.
	// If nil, no auto-restart is attempted.
	OnRestart    func(workerID string)
	MaxRestarts  int
	restartCount int
	Status       WorkerStatus
}

// HealthMonitor tracks worker heartbeats and triggers automatic failover
// when a worker stops responding. It is safe for concurrent use.
//
// Typical usage:
//
//	mon := scheduler.NewHealthMonitor(5 * time.Second)
//	mon.Register("sandbox-1", 30*time.Second, func(id string) {
//	    log.Printf("worker %s died, restarting", id)
//	}, func(id string) {
//	    restartWorker(id)
//	}, 3)
//	mon.Start()
//	defer mon.Stop()
//
//	// In the worker goroutine:
//	for {
//	    mon.Heartbeat("sandbox-1")
//	    // ... do work ...
//	    time.Sleep(10 * time.Second)
//	}
type HealthMonitor struct {
	workers       map[string]*WorkerInfo
	mu            sync.RWMutex
	running       bool
	stop          chan struct{}
	done          chan struct{}
	checkInterval time.Duration
}

// NewHealthMonitor creates a HealthMonitor that checks worker health every
// checkInterval. A reasonable default is 5 seconds.
func NewHealthMonitor(checkInterval time.Duration) *HealthMonitor {
	if checkInterval <= 0 {
		checkInterval = 5 * time.Second
	}
	return &HealthMonitor{
		workers:       make(map[string]*WorkerInfo),
		checkInterval: checkInterval,
	}
}

// Register adds a worker to the health monitor. The worker must send
// Heartbeat(id) at least once every timeout duration. If timeout elapses
// without a heartbeat, onDead is called, followed by onRestart (if non-nil).
// maxRestarts caps the number of auto-restart attempts; 0 means unlimited.
//
// Registering an already-registered worker updates its configuration.
func (m *HealthMonitor) Register(id string, timeout time.Duration, onDead, onRestart func(string), maxRestarts int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.workers[id] = &WorkerInfo{
		ID:            id,
		LastHeartbeat: time.Now(),
		Timeout:       timeout,
		OnDead:        onDead,
		OnRestart:     onRestart,
		MaxRestarts:   maxRestarts,
		Status:        WorkerHealthy,
	}
}

// Unregister removes a worker from the health monitor.
func (m *HealthMonitor) Unregister(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.workers, id)
}

// Heartbeat records a heartbeat for the given worker. If the worker is not
// registered, it is a no-op (so callers do not need to check registration
// status on every beat).
func (m *HealthMonitor) Heartbeat(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if w, ok := m.workers[id]; ok {
		w.LastHeartbeat = time.Now()
		if w.Status != WorkerHealthy {
			logger.Info("worker recovered",
				"worker_id", id,
				"previous_status", w.Status.String(),
			)
			w.Status = WorkerHealthy
		}
	}
}

// GetStatus returns the current status of a worker. Returns WorkerDead and
// false if the worker is not registered.
func (m *HealthMonitor) GetStatus(id string) (WorkerStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	w, ok := m.workers[id]
	if !ok {
		return WorkerDead, false
	}
	return w.Status, true
}

// ListWorkers returns a snapshot of all registered workers and their statuses.
func (m *HealthMonitor) ListWorkers() []WorkerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]WorkerInfo, 0, len(m.workers))
	for _, w := range m.workers {
		result = append(result, *w)
	}
	return result
}

// Start begins the health check loop in a background goroutine.
func (m *HealthMonitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stop = make(chan struct{})
	m.done = make(chan struct{})
	m.mu.Unlock()

	go m.run()
}

// Stop stops the health check loop and waits for the goroutine to exit.
func (m *HealthMonitor) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.stop)
	m.mu.Unlock()

	<-m.done
}

func (m *HealthMonitor) run() {
	defer close(m.done)

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stop:
			return
		case now := <-ticker.C:
			m.checkHealth(now)
		}
	}
}

func (m *HealthMonitor) checkHealth(now time.Time) {
	// Collect dead workers and their callbacks under the lock, then invoke
	// callbacks outside the lock to prevent deadlocks if the callback tries
	// to call Heartbeat/Register/Unregister on the same monitor.
	type deadWorker struct {
		id      string
		onDead  func(string)
		restart func(string)
	}
	var deadWorkers []deadWorker

	m.mu.Lock()
	for id, w := range m.workers {
		elapsed := now.Sub(w.LastHeartbeat)

		switch {
		case elapsed > w.Timeout && w.Status != WorkerDead:
			// Worker has timed out — declare dead.
			w.Status = WorkerDead
			logger.Error("worker heartbeat timeout, declared dead",
				"worker_id", id,
				"timeout", w.Timeout,
				"elapsed", elapsed,
				"restart_count", w.restartCount,
			)

			dw := deadWorker{id: id, onDead: w.OnDead}

			// Attempt auto-restart if within the restart limit.
			if w.OnRestart != nil && (w.MaxRestarts == 0 || w.restartCount < w.MaxRestarts) {
				w.restartCount++
				logger.Info("auto-restarting worker",
					"worker_id", id,
					"attempt", w.restartCount,
					"max_restarts", w.MaxRestarts,
				)
				w.LastHeartbeat = now // reset so we don't immediately re-dead
				w.Status = WorkerHealthy
				dw.restart = w.OnRestart
			}

			deadWorkers = append(deadWorkers, dw)

		case elapsed > w.Timeout/2 && w.Status == WorkerHealthy:
			// Worker is late but not yet timed out.
			w.Status = WorkerUnhealthy
			logger.Warn("worker heartbeat late",
				"worker_id", id,
				"elapsed", elapsed,
				"timeout", w.Timeout,
			)
		}
	}
	m.mu.Unlock()

	// Invoke callbacks outside the lock to avoid deadlocks.
	for _, dw := range deadWorkers {
		if dw.onDead != nil {
			dw.onDead(dw.id)
		}
		if dw.restart != nil {
			go dw.restart(dw.id)
		}
	}
}

// WorkerCount returns the number of registered workers.
func (m *HealthMonitor) WorkerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.workers)
}

// HealthyCount returns the number of healthy workers.
func (m *HealthMonitor) HealthyCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, w := range m.workers {
		if w.Status == WorkerHealthy {
			count++
		}
	}
	return count
}

// StatusSummary returns a human-readable summary of all workers.
func (m *HealthMonitor) StatusSummary() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	healthy := 0
	unhealthy := 0
	dead := 0
	for _, w := range m.workers {
		switch w.Status {
		case WorkerHealthy:
			healthy++
		case WorkerUnhealthy:
			unhealthy++
		case WorkerDead:
			dead++
		}
	}
	return fmt.Sprintf("%d healthy, %d unhealthy, %d dead", healthy, unhealthy, dead)
}
