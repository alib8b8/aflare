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
	"testing"
	"time"
)

// The HealthMonitor state machine is tick-driven: checkHealth(now) takes the
// current time as an argument, so tests step it deterministically with
// synthetic timestamps instead of sleeping on real timers (issue #85).

func TestHealthMonitor_RegisterAndHeartbeat(t *testing.T) {
	mon := NewHealthMonitor(100 * time.Millisecond)

	mon.Register("worker-1", 500*time.Millisecond, func(id string) {
		t.Errorf("unexpected death for %s", id)
	}, nil, 0)

	status, ok := mon.GetStatus("worker-1")
	if !ok {
		t.Fatal("worker-1 not found")
	}
	if status != WorkerHealthy {
		t.Errorf("expected healthy, got %s", status)
	}

	// Send heartbeat, then check health at a point well within the timeout.
	mon.Heartbeat("worker-1")
	afterBeat := time.Now()
	mon.checkHealth(afterBeat.Add(150 * time.Millisecond))

	status, _ = mon.GetStatus("worker-1")
	if status != WorkerHealthy {
		t.Errorf("expected healthy after heartbeat, got %s", status)
	}
}

func TestHealthMonitor_TimeoutTriggersOnDead(t *testing.T) {
	mon := NewHealthMonitor(50 * time.Millisecond)

	var deadIDs []string
	mon.Register("worker-1", 100*time.Millisecond, func(id string) {
		deadIDs = append(deadIDs, id)
	}, nil, 0)

	base := time.Now()
	// checkHealth invokes onDead synchronously, so no waiting is needed.
	mon.checkHealth(base.Add(300 * time.Millisecond))

	if len(deadIDs) == 0 {
		t.Error("expected worker-1 to be declared dead")
	}
	if len(deadIDs) > 0 && deadIDs[0] != "worker-1" {
		t.Errorf("expected dead worker-1, got %s", deadIDs[0])
	}

	status, _ := mon.GetStatus("worker-1")
	if status != WorkerDead {
		t.Errorf("expected dead, got %s", status)
	}
}

func TestHealthMonitor_AutoRestart(t *testing.T) {
	mon := NewHealthMonitor(50 * time.Millisecond)

	// restarts signals each restart; the callback heartbeats first so that
	// receiving from the channel guarantees the heartbeat is recorded.
	restarts := make(chan string, 4)
	deadCount := 0

	mon.Register("worker-1", 100*time.Millisecond,
		func(id string) {
			deadCount++
		},
		func(id string) {
			mon.Heartbeat(id)
			restarts <- id
		},
		2, // max 2 restarts
	)

	base := time.Now()

	// First timeout: dead + restart #1.
	mon.checkHealth(base.Add(200 * time.Millisecond))
	select {
	case id := <-restarts:
		if id != "worker-1" {
			t.Errorf("expected restart of worker-1, got %s", id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("restart #1 did not happen")
	}

	// Second timeout: dead again + restart #2.
	mon.checkHealth(base.Add(400 * time.Millisecond))
	select {
	case <-restarts:
	case <-time.After(5 * time.Second):
		t.Fatal("restart #2 did not happen")
	}

	// Third timeout: restart budget exhausted — no more restarts.
	mon.checkHealth(base.Add(600 * time.Millisecond))
	select {
	case id := <-restarts:
		t.Errorf("unexpected restart %s beyond max", id)
	case <-time.After(100 * time.Millisecond):
	}

	if deadCount != 3 {
		t.Errorf("expected 3 deaths, got %d", deadCount)
	}
	if got := len(restarts); got != 0 {
		t.Errorf("expected exactly 2 restarts, got %d extra", got)
	}

	status, _ := mon.GetStatus("worker-1")
	if status != WorkerDead {
		t.Errorf("expected dead after restart budget exhausted, got %s", status)
	}
}

func TestHealthMonitor_UnhealthyThenRecover(t *testing.T) {
	mon := NewHealthMonitor(50 * time.Millisecond)

	mon.Register("worker-1", 200*time.Millisecond, nil, nil, 0)

	base := time.Now()

	// Past half the timeout but not timed out yet: unhealthy.
	mon.checkHealth(base.Add(150 * time.Millisecond))
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
