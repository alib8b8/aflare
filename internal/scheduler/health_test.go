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

package scheduler

import (
	"sync"
	"testing"
	"time"
)

func TestHealthMonitor_RegisterAndHeartbeat(t *testing.T) {
	mon := NewHealthMonitor(100 * time.Millisecond)

	heartbeatReceived := make(chan struct{}, 1)
	mon.Register("worker-1", 500*time.Millisecond, func(id string) {
		t.Errorf("unexpected death for %s", id)
	}, nil, 0)

	mon.Start()
	defer mon.Stop()

	status, ok := mon.GetStatus("worker-1")
	if !ok {
		t.Fatal("worker-1 not found")
	}
	if status != WorkerHealthy {
		t.Errorf("expected healthy, got %s", status)
	}

	// Send heartbeat before timeout
	mon.Heartbeat("worker-1")
	time.Sleep(150 * time.Millisecond)

	status, _ = mon.GetStatus("worker-1")
	if status != WorkerHealthy {
		t.Errorf("expected healthy after heartbeat, got %s", status)
	}

	// Ensure no false death
	select {
	case <-heartbeatReceived:
		t.Error("unexpected heartbeat received")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHealthMonitor_TimeoutTriggersOnDead(t *testing.T) {
	mon := NewHealthMonitor(50 * time.Millisecond)

	var mu sync.Mutex
	var deadIDs []string
	mon.Register("worker-1", 100*time.Millisecond, func(id string) {
		mu.Lock()
		deadIDs = append(deadIDs, id)
		mu.Unlock()
	}, nil, 0)

	mon.Start()
	defer mon.Stop()

	// Wait for timeout to expire
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	if len(deadIDs) == 0 {
		t.Error("expected worker-1 to be declared dead")
	}
	if len(deadIDs) > 0 && deadIDs[0] != "worker-1" {
		t.Errorf("expected dead worker-1, got %s", deadIDs[0])
	}
	mu.Unlock()

	status, _ := mon.GetStatus("worker-1")
	if status != WorkerDead {
		t.Errorf("expected dead, got %s", status)
	}
}

func TestHealthMonitor_AutoRestart(t *testing.T) {
	mon := NewHealthMonitor(50 * time.Millisecond)

	var mu sync.Mutex
	restartCount := 0
	deadCount := 0

	mon.Register("worker-1", 100*time.Millisecond,
		func(id string) {
			mu.Lock()
			deadCount++
			mu.Unlock()
		},
		func(id string) {
			mu.Lock()
			restartCount++
			mu.Unlock()
			// Simulate recovery: send heartbeat
			mon.Heartbeat(id)
		},
		2, // max 2 restarts
	)

	mon.Start()
	defer mon.Stop()

	// Wait for timeout + restart
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	if restartCount == 0 {
		t.Error("expected at least 1 restart")
	}
	if restartCount > 2 {
		t.Errorf("expected at most 2 restarts, got %d", restartCount)
	}
	mu.Unlock()
}

func TestHealthMonitor_UnhealthyThenRecover(t *testing.T) {
	mon := NewHealthMonitor(50 * time.Millisecond)

	mon.Register("worker-1", 200*time.Millisecond, nil, nil, 0)

	mon.Start()
	defer mon.Stop()

	// Wait for unhealthy state (half timeout)
	time.Sleep(150 * time.Millisecond)

	status, _ := mon.GetStatus("worker-1")
	if status != WorkerUnhealthy {
		t.Errorf("expected unhealthy, got %s", status)
	}

	// Recover with heartbeat
	mon.Heartbeat("worker-1")

	status, _ = mon.GetStatus("worker-1")
	if status != WorkerHealthy {
		t.Errorf("expected healthy after recovery, got %s", status)
	}
}

func TestHealthMonitor_Unregister(t *testing.T) {
	mon := NewHealthMonitor(50 * time.Millisecond)

	mon.Register("worker-1", 100*time.Millisecond, nil, nil, 0)
	mon.Unregister("worker-1")

	_, ok := mon.GetStatus("worker-1")
	if ok {
		t.Error("worker-1 should be unregistered")
	}
}

func TestHealthMonitor_StatusSummary(t *testing.T) {
	mon := NewHealthMonitor(50 * time.Millisecond)

	mon.Register("worker-1", 100*time.Millisecond, nil, nil, 0)
	mon.Register("worker-2", 100*time.Millisecond, nil, nil, 0)

	if mon.WorkerCount() != 2 {
		t.Errorf("expected 2 workers, got %d", mon.WorkerCount())
	}
	if mon.HealthyCount() != 2 {
		t.Errorf("expected 2 healthy, got %d", mon.HealthyCount())
	}

	summary := mon.StatusSummary()
	if summary != "2 healthy, 0 unhealthy, 0 dead" {
		t.Errorf("unexpected summary: %s", summary)
	}
}

func TestHealthMonitor_HeartbeatUnknownWorker(t *testing.T) {
	mon := NewHealthMonitor(50 * time.Millisecond)

	// Should not panic
	mon.Heartbeat("nonexistent")

	status, ok := mon.GetStatus("nonexistent")
	if ok {
		t.Error("nonexistent worker should not be registered")
	}
	if status != WorkerDead {
		t.Errorf("expected dead for unknown, got %s", status)
	}
}

func TestHealthMonitor_ListWorkers(t *testing.T) {
	mon := NewHealthMonitor(50 * time.Millisecond)

	mon.Register("worker-1", 100*time.Millisecond, nil, nil, 0)
	mon.Register("worker-2", 200*time.Millisecond, nil, nil, 0)

	workers := mon.ListWorkers()
	if len(workers) != 2 {
		t.Errorf("expected 2 workers, got %d", len(workers))
	}
}

func TestHealthMonitor_StopResume(t *testing.T) {
	mon := NewHealthMonitor(50 * time.Millisecond)

	mon.Register("worker-1", 100*time.Millisecond, nil, nil, 0)

	mon.Start()
	mon.Stop()

	// Should be able to start again
	mon.Start()
	defer mon.Stop()

	status, _ := mon.GetStatus("worker-1")
	if status != WorkerHealthy {
		t.Errorf("expected healthy after restart, got %s", status)
	}
}
