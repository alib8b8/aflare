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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alib8b8/llm-box/internal/workflow"
)

// ─── 共享测试辅助 ───

// newTestListener 在 127.0.0.1 上 bind 一个随机端口并返回该 listener
// 及其端口号字符串。listener 保持已 bind 状态(不 Close),供
// SetServeListener 复用,从而彻底消除旧 freeTestPort 那种
// Listen→Close→重新 Listen 的 TOCTOU 端口竞争:多个并行测试不会再
// 抢到同一端口,请求也不会串到错误的 bus 上。
func newTestListener(t *testing.T) (net.Listener, string) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind test listener: %v", err)
	}
	port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	return l, port
}

// waitForHTTPReady 轮询 URL 直到返回 HTTP 响应或超时。
func waitForHTTPReady(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", url)
}

// ─── NewWorker 默认端口 ───

func TestNewWorker_DefaultPort(t *testing.T) {
	w, err := NewWorker("", "http://localhost:8090", "token", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.port != defaultWorkerPort {
		t.Errorf("expected default port %s, got %s", defaultWorkerPort, w.port)
	}
}

// ─── selectBestNodeLocked(带节点) ───

func TestSelectBestNodeLocked_PicksLeastLoaded(t *testing.T) {
	c := NewCoordinator("0", "token")
	c.mu.Lock()
	c.nodes["n1"] = &NodeInfo{ID: "n1", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 5}
	c.nodes["n2"] = &NodeInfo{ID: "n2", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 2}
	c.nodes["n3"] = &NodeInfo{ID: "n3", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 8}
	best := c.selectBestNodeLocked()
	c.mu.Unlock()
	if best != "n2" {
		t.Errorf("expected n2 (lowest load=2), got %s", best)
	}
}

func TestSelectBestNodeLocked_SkipOffline(t *testing.T) {
	c := NewCoordinator("0", "token")
	c.mu.Lock()
	c.nodes["n1"] = &NodeInfo{ID: "n1", Status: NodeStatusOffline, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 0}
	c.nodes["n2"] = &NodeInfo{ID: "n2", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 5}
	best := c.selectBestNodeLocked()
	c.mu.Unlock()
	if best != "n2" {
		t.Errorf("expected n2 (n1 offline), got %s", best)
	}
}

func TestSelectBestNodeLocked_SkipStaleHeartbeat(t *testing.T) {
	c := NewCoordinator("0", "token")
	c.mu.Lock()
	c.nodes["n1"] = &NodeInfo{ID: "n1", Status: NodeStatusIdle, LastHeartbeat: time.Now().Add(-60 * time.Second), Capacity: 10, CurrentLoad: 0}
	c.nodes["n2"] = &NodeInfo{ID: "n2", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 5}
	best := c.selectBestNodeLocked()
	c.mu.Unlock()
	if best != "n2" {
		t.Errorf("expected n2 (n1 stale heartbeat), got %s", best)
	}
}

func TestSelectBestNodeLocked_SkipOverCapacity(t *testing.T) {
	c := NewCoordinator("0", "token")
	c.mu.Lock()
	c.nodes["n1"] = &NodeInfo{ID: "n1", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 5, CurrentLoad: 5}
	c.nodes["n2"] = &NodeInfo{ID: "n2", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 5}
	best := c.selectBestNodeLocked()
	c.mu.Unlock()
	if best != "n2" {
		t.Errorf("expected n2 (n1 at capacity), got %s", best)
	}
}

func TestSelectBestNodeLocked_SkipBreakerBlocked(t *testing.T) {
	c := NewCoordinator("0", "token")
	c.mu.Lock()
	c.nodes["n1"] = &NodeInfo{ID: "n1", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 0}
	c.nodes["n2"] = &NodeInfo{ID: "n2", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 5}
	c.mu.Unlock()
	// 熔断 n1
	for i := 0; i < 5; i++ {
		c.breakers.RecordFailure("n1")
	}
	c.mu.RLock()
	best := c.selectBestNodeLocked()
	c.mu.RUnlock()
	if best != "n2" {
		t.Errorf("expected n2 (n1 breaker open), got %s", best)
	}
}

func TestSelectBestNodeLocked_AllFiltered(t *testing.T) {
	c := NewCoordinator("0", "token")
	c.mu.Lock()
	c.nodes["n1"] = &NodeInfo{ID: "n1", Status: NodeStatusOffline, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 0}
	best := c.selectBestNodeLocked()
	c.mu.Unlock()
	if best != "" {
		t.Errorf("expected empty (all filtered), got %s", best)
	}
}

// ─── HTTP Handler 测试 ───

func TestCoordinator_HandleRegister(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		body     string
		wantCode int
	}{
		{"WrongMethod", "GET", "", http.StatusMethodNotAllowed},
		{"InvalidJSON", "POST", "not-json", http.StatusBadRequest},
		{"EmptyHost", "POST", `{"host":"","port":"8080","capacity":1}`, http.StatusBadRequest},
		{"InvalidPort", "POST", `{"host":"localhost","port":"","capacity":1}`, http.StatusBadRequest},
		{"NonNumericPort", "POST", `{"host":"localhost","port":"abc","capacity":1}`, http.StatusBadRequest},
		{"CapacityZero", "POST", `{"host":"localhost","port":"8080","capacity":0}`, http.StatusBadRequest},
		{"CapacityTooLarge", "POST", `{"host":"localhost","port":"8080","capacity":1001}`, http.StatusBadRequest},
		{"Valid", "POST", `{"host":"localhost","port":"8080","capacity":5}`, http.StatusCreated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCoordinator("0", "token")
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, "/api/register", body)
			rec := httptest.NewRecorder()
			c.handleRegister(rec, req)
			if rec.Code != tt.wantCode {
				t.Errorf("expected %d, got %d (body: %s)", tt.wantCode, rec.Code, rec.Body.String())
			}
			if tt.wantCode == http.StatusCreated {
				var resp map[string]string
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp["node_id"] == "" {
					t.Error("expected non-empty node_id")
				}
			}
		})
	}
}

func TestCoordinator_HandleRegister_MultipleIncrement(t *testing.T) {
	c := NewCoordinator("0", "token")
	for i := 0; i < 3; i++ {
		body := strings.NewReader(`{"host":"localhost","port":"8080","capacity":5}`)
		req := httptest.NewRequest("POST", "/api/register", body)
		rec := httptest.NewRecorder()
		c.handleRegister(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("register %d failed: %d", i, rec.Code)
		}
	}
	c.mu.RLock()
	count := len(c.nodes)
	c.mu.RUnlock()
	if count != 3 {
		t.Errorf("expected 3 nodes, got %d", count)
	}
}

func TestCoordinator_HandleHeartbeat(t *testing.T) {
	t.Run("WrongMethod", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		req := httptest.NewRequest("GET", "/api/heartbeat", nil)
		rec := httptest.NewRecorder()
		c.handleHeartbeat(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		req := httptest.NewRequest("POST", "/api/heartbeat", strings.NewReader("bad"))
		rec := httptest.NewRecorder()
		c.handleHeartbeat(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("UnknownNode", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		body := strings.NewReader(`{"node_id":"unknown","current_load":3}`)
		req := httptest.NewRequest("POST", "/api/heartbeat", body)
		rec := httptest.NewRecorder()
		c.handleHeartbeat(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 even for unknown node, got %d", rec.Code)
		}
	})

	t.Run("ExistingNode", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		c.mu.Lock()
		c.nodes["n1"] = &NodeInfo{ID: "n1", Status: NodeStatusBusy, LastHeartbeat: time.Now().Add(-1 * time.Minute), Capacity: 10, CurrentLoad: 0}
		c.mu.Unlock()

		body := strings.NewReader(`{"node_id":"n1","current_load":3}`)
		req := httptest.NewRequest("POST", "/api/heartbeat", body)
		rec := httptest.NewRecorder()
		c.handleHeartbeat(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		c.mu.RLock()
		node := c.nodes["n1"]
		load := node.CurrentLoad
		status := node.Status
		c.mu.RUnlock()
		if load != 3 {
			t.Errorf("expected load 3, got %d", load)
		}
		if status != NodeStatusIdle {
			t.Errorf("expected status idle, got %s", status)
		}
		if time.Since(node.LastHeartbeat) > time.Second {
			t.Error("LastHeartbeat should be recent")
		}
	})
}

func TestCoordinator_HandleListNodes(t *testing.T) {
	t.Run("WrongMethod", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		req := httptest.NewRequest("POST", "/api/nodes", nil)
		rec := httptest.NewRecorder()
		c.handleListNodes(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		req := httptest.NewRequest("GET", "/api/nodes", nil)
		rec := httptest.NewRecorder()
		c.handleListNodes(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["count"].(float64) != 0 {
			t.Errorf("expected count 0, got %v", resp["count"])
		}
	})

	t.Run("WithNodes", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		c.mu.Lock()
		c.nodes["n1"] = &NodeInfo{ID: "n1", Status: NodeStatusIdle, Capacity: 5, CurrentLoad: 1}
		c.nodes["n2"] = &NodeInfo{ID: "n2", Status: NodeStatusBusy, Capacity: 10, CurrentLoad: 5}
		c.mu.Unlock()

		req := httptest.NewRequest("GET", "/api/nodes", nil)
		rec := httptest.NewRecorder()
		c.handleListNodes(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["count"].(float64) != 2 {
			t.Errorf("expected count 2, got %v", resp["count"])
		}
	})
}

func TestCoordinator_HandleTask_Routing(t *testing.T) {
	c := NewCoordinator("0", "token")

	t.Run("GET_delegatesToGetTask", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/task?task_id=nonexistent", nil)
		rec := httptest.NewRecorder()
		c.handleTask(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 (task not found), got %d", rec.Code)
		}
	})

	t.Run("POST_delegatesToAssignTask", func(t *testing.T) {
		body := strings.NewReader(`{"step_index":0,"step":{"node":"x"}}`)
		req := httptest.NewRequest("POST", "/api/task", body)
		rec := httptest.NewRecorder()
		c.handleTask(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503 (no nodes), got %d", rec.Code)
		}
	})

	t.Run("PUT_delegatesToUpdateTask", func(t *testing.T) {
		body := strings.NewReader(`{"task_id":"x","status":"running"}`)
		req := httptest.NewRequest("PUT", "/api/task", body)
		rec := httptest.NewRecorder()
		c.handleTask(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("DELETE_notAllowed", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/task", nil)
		rec := httptest.NewRecorder()
		c.handleTask(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})
}

func TestCoordinator_HandleGetTask(t *testing.T) {
	t.Run("MissingTaskID", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		req := httptest.NewRequest("GET", "/api/task", nil)
		rec := httptest.NewRecorder()
		c.handleGetTask(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		req := httptest.NewRequest("GET", "/api/task?task_id=ghost", nil)
		rec := httptest.NewRecorder()
		c.handleGetTask(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("Found", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		c.mu.Lock()
		c.tasks["t1"] = &Task{ID: "t1", StepIndex: 0, Status: TaskStatusPending, AssignedTo: "n1"}
		c.mu.Unlock()

		req := httptest.NewRequest("GET", "/api/task?task_id=t1", nil)
		rec := httptest.NewRecorder()
		c.handleGetTask(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var task Task
		json.NewDecoder(rec.Body).Decode(&task)
		if task.ID != "t1" {
			t.Errorf("expected task ID t1, got %s", task.ID)
		}
	})
}

func TestCoordinator_HandleAssignTask(t *testing.T) {
	t.Run("InvalidJSON", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		req := httptest.NewRequest("POST", "/api/task", strings.NewReader("bad"))
		rec := httptest.NewRecorder()
		c.handleAssignTask(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("NoNodes", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		body := strings.NewReader(`{"step_index":0,"step":{"node":"x"}}`)
		req := httptest.NewRequest("POST", "/api/task", body)
		rec := httptest.NewRecorder()
		c.handleAssignTask(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("Valid", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		c.mu.Lock()
		c.nodes["n1"] = &NodeInfo{ID: "n1", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 0}
		c.mu.Unlock()

		body := strings.NewReader(`{"step_index":2,"step":{"node":"test"}}`)
		req := httptest.NewRequest("POST", "/api/task", body)
		rec := httptest.NewRecorder()
		c.handleAssignTask(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["task_id"] == nil {
			t.Error("expected task_id in response")
		}
		if resp["assigned_to"] != "n1" {
			t.Errorf("expected assigned_to n1, got %v", resp["assigned_to"])
		}

		c.mu.RLock()
		if len(c.tasks) != 1 {
			t.Errorf("expected 1 task, got %d", len(c.tasks))
		}
		if c.nodes["n1"].CurrentLoad != 1 {
			t.Errorf("expected load 1, got %d", c.nodes["n1"].CurrentLoad)
		}
		c.mu.RUnlock()
	})
}

func TestCoordinator_HandleUpdateTask(t *testing.T) {
	t.Run("InvalidJSON", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		req := httptest.NewRequest("PUT", "/api/task", strings.NewReader("bad"))
		rec := httptest.NewRecorder()
		c.handleUpdateTask(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("TaskNotFound", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		body := strings.NewReader(`{"task_id":"ghost","status":"running"}`)
		req := httptest.NewRequest("PUT", "/api/task", body)
		rec := httptest.NewRecorder()
		c.handleUpdateTask(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 even for unknown task, got %d", rec.Code)
		}
	})

	t.Run("Running_SetsStartTime", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		c.mu.Lock()
		c.tasks["t1"] = &Task{ID: "t1", Status: TaskStatusPending, AssignedTo: "n1"}
		c.mu.Unlock()

		body := strings.NewReader(`{"task_id":"t1","status":"running"}`)
		req := httptest.NewRequest("PUT", "/api/task", body)
		rec := httptest.NewRecorder()
		c.handleUpdateTask(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		c.mu.RLock()
		task := c.tasks["t1"]
		c.mu.RUnlock()
		if task.StartTime == nil {
			t.Error("expected StartTime to be set for running task")
		}
		if task.Status != TaskStatusRunning {
			t.Errorf("expected status running, got %s", task.Status)
		}
	})

	t.Run("Completed_DecrementsLoad", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		c.mu.Lock()
		c.nodes["n1"] = &NodeInfo{ID: "n1", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 3}
		c.tasks["t1"] = &Task{ID: "t1", Status: TaskStatusRunning, AssignedTo: "n1"}
		c.mu.Unlock()

		body := strings.NewReader(`{"task_id":"t1","status":"completed","output":"done"}`)
		req := httptest.NewRequest("PUT", "/api/task", body)
		rec := httptest.NewRecorder()
		c.handleUpdateTask(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		c.mu.RLock()
		task := c.tasks["t1"]
		load := c.nodes["n1"].CurrentLoad
		c.mu.RUnlock()
		if task.EndTime == nil {
			t.Error("expected EndTime to be set for completed task")
		}
		if task.Output != "done" {
			t.Errorf("expected output 'done', got %s", task.Output)
		}
		if load != 2 {
			t.Errorf("expected load decremented to 2, got %d", load)
		}
		// 完成应记录 breaker 成功(不影响 AllowRequest)
		if !c.breakers.AllowRequest("n1") {
			t.Error("n1 should still be allowed after success")
		}
	})

	t.Run("Failed_RecordsBreakerFailure", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		c.mu.Lock()
		c.nodes["n1"] = &NodeInfo{ID: "n1", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 1}
		c.tasks["t1"] = &Task{ID: "t1", Status: TaskStatusRunning, AssignedTo: "n1"}
		c.mu.Unlock()

		body := strings.NewReader(`{"task_id":"t1","status":"failed","error":"boom"}`)
		req := httptest.NewRequest("PUT", "/api/task", body)
		rec := httptest.NewRecorder()
		c.handleUpdateTask(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		c.mu.RLock()
		task := c.tasks["t1"]
		load := c.nodes["n1"].CurrentLoad
		c.mu.RUnlock()
		if task.Error != "boom" {
			t.Errorf("expected error 'boom', got %s", task.Error)
		}
		if load != 0 {
			t.Errorf("expected load 0, got %d", load)
		}
	})

	t.Run("Completed_LoadNotNegative", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		c.mu.Lock()
		c.nodes["n1"] = &NodeInfo{ID: "n1", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 0}
		c.tasks["t1"] = &Task{ID: "t1", Status: TaskStatusRunning, AssignedTo: "n1"}
		c.mu.Unlock()

		body := strings.NewReader(`{"task_id":"t1","status":"completed"}`)
		req := httptest.NewRequest("PUT", "/api/task", body)
		rec := httptest.NewRecorder()
		c.handleUpdateTask(rec, req)
		c.mu.RLock()
		load := c.nodes["n1"].CurrentLoad
		c.mu.RUnlock()
		if load != 0 {
			t.Errorf("expected load to stay at 0 (not negative), got %d", load)
		}
	})
}

func TestCoordinator_HandleBreakers(t *testing.T) {
	t.Run("WrongMethod", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		req := httptest.NewRequest("POST", "/api/breakers", nil)
		rec := httptest.NewRecorder()
		c.handleBreakers(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		req := httptest.NewRequest("GET", "/api/breakers", nil)
		rec := httptest.NewRecorder()
		c.handleBreakers(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["count"].(float64) != 0 {
			t.Errorf("expected count 0, got %v", resp["count"])
		}
	})

	t.Run("WithBreakers", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		c.breakers.RecordFailure("n1")
		c.breakers.RecordSuccess("n2")

		req := httptest.NewRequest("GET", "/api/breakers", nil)
		rec := httptest.NewRecorder()
		c.handleBreakers(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["count"].(float64) != 2 {
			t.Errorf("expected count 2, got %v", resp["count"])
		}
	})
}

func TestCoordinator_HandleListTasks(t *testing.T) {
	t.Run("WrongMethod", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		req := httptest.NewRequest("POST", "/api/tasks", nil)
		rec := httptest.NewRecorder()
		c.handleListTasks(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		req := httptest.NewRequest("GET", "/api/tasks", nil)
		rec := httptest.NewRecorder()
		c.handleListTasks(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["count"].(float64) != 0 {
			t.Errorf("expected count 0, got %v", resp["count"])
		}
	})

	t.Run("WithFilter", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		c.mu.Lock()
		c.tasks["t1"] = &Task{ID: "t1", Status: TaskStatusPending}
		c.tasks["t2"] = &Task{ID: "t2", Status: TaskStatusRunning}
		c.tasks["t3"] = &Task{ID: "t3", Status: TaskStatusCompleted}
		c.mu.Unlock()

		// 无过滤
		req := httptest.NewRequest("GET", "/api/tasks", nil)
		rec := httptest.NewRecorder()
		c.handleListTasks(rec, req)
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["count"].(float64) != 3 {
			t.Errorf("expected count 3, got %v", resp["count"])
		}

		// 过滤 running
		req = httptest.NewRequest("GET", "/api/tasks?status=running", nil)
		rec = httptest.NewRecorder()
		c.handleListTasks(rec, req)
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp["count"].(float64) != 1 {
			t.Errorf("expected count 1 for running filter, got %v", resp["count"])
		}
	})
}

func TestCoordinator_HandleExecute(t *testing.T) {
	t.Run("WrongMethod", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		req := httptest.NewRequest("GET", "/api/execute", nil)
		rec := httptest.NewRecorder()
		c.handleExecute(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		req := httptest.NewRequest("POST", "/api/execute", strings.NewReader("bad"))
		rec := httptest.NewRecorder()
		c.handleExecute(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("InvalidWorkflow", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		// 使用 tab 缩进,这在 YAML 中是非法的
		body := strings.NewReader(`{"workflow":"\tbad: yaml"}`)
		req := httptest.NewRequest("POST", "/api/execute", body)
		rec := httptest.NewRecorder()
		c.handleExecute(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid workflow, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("NoNodes", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		body := strings.NewReader(`{"workflow":"name: test\n"}`)
		req := httptest.NewRequest("POST", "/api/execute", body)
		rec := httptest.NewRecorder()
		c.handleExecute(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("ValidEmptyWorkflow", func(t *testing.T) {
		c := NewCoordinator("0", "token")
		c.mu.Lock()
		c.nodes["n1"] = &NodeInfo{ID: "n1", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 0}
		c.mu.Unlock()

		body := strings.NewReader(`{"workflow":"name: empty\n"}`)
		req := httptest.NewRequest("POST", "/api/execute", body)
		rec := httptest.NewRecorder()
		c.handleExecute(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d", rec.Code)
		}
		// 等待后台 goroutine 完成(0 步,应立即返回)
		time.Sleep(100 * time.Millisecond)
	})
}

// ─── Start/Stop 测试 ───

func TestCoordinator_StartStop(t *testing.T) {
	l, port := newTestListener(t)
	c := NewCoordinator(port, "token")
	c.SetServeListener(l)

	errCh := make(chan error, 1)
	go func() { errCh <- c.Start() }()

	waitForHTTPReady(t, "http://127.0.0.1:"+port+"/health", 2*time.Second)

	if err := c.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("Start returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Stop")
	}
}

func TestCoordinator_StopWithoutStart(t *testing.T) {
	c := NewCoordinator("0", "token")
	if err := c.Stop(); err != nil {
		t.Errorf("Stop without Start should return nil, got %v", err)
	}
}

func TestCoordinator_StopIdempotent(t *testing.T) {
	l, port := newTestListener(t)
	c := NewCoordinator(port, "token")
	c.SetServeListener(l)

	errCh := make(chan error, 1)
	go func() { errCh <- c.Start() }()
	waitForHTTPReady(t, "http://127.0.0.1:"+port+"/health", 2*time.Second)

	if err := c.Stop(); err != nil {
		t.Fatalf("first Stop failed: %v", err)
	}
	<-errCh
	// 重复 Stop 应幂等
	if err := c.Stop(); err != nil {
		t.Errorf("repeated Stop should be idempotent, got %v", err)
	}
}

// ─── dispatchTask 测试 ───

func TestCoordinator_DispatchTask_Success(t *testing.T) {
	workerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer workerSrv.Close()

	host, port, _ := net.SplitHostPort(strings.TrimPrefix(workerSrv.URL, "http://"))
	c := NewCoordinator("0", "token")
	c.mu.Lock()
	c.nodes["n1"] = &NodeInfo{ID: "n1", Host: host, Port: port, Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 0}
	c.mu.Unlock()

	task := &Task{ID: "t1", StepIndex: 0, Status: TaskStatusPending}
	c.dispatchTask("n1", task)

	time.Sleep(100 * time.Millisecond)
	if !c.breakers.AllowRequest("n1") {
		t.Error("n1 should still be allowed after successful dispatch")
	}
}

func TestCoordinator_DispatchTask_5xx(t *testing.T) {
	workerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer workerSrv.Close()

	host, port, _ := net.SplitHostPort(strings.TrimPrefix(workerSrv.URL, "http://"))
	c := NewCoordinator("0", "token")
	c.mu.Lock()
	c.nodes["n1"] = &NodeInfo{ID: "n1", Host: host, Port: port, Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 0}
	c.mu.Unlock()

	// 默认 5 次失败跳闸
	for i := 0; i < 5; i++ {
		task := &Task{ID: fmt.Sprintf("t%d", i), Status: TaskStatusPending}
		c.dispatchTask("n1", task)
	}
	time.Sleep(200 * time.Millisecond)
	if c.breakers.AllowRequest("n1") {
		t.Error("n1 should be blocked after 5 dispatch failures (5xx)")
	}
}

func TestCoordinator_DispatchTask_ConnectionRefused(t *testing.T) {
	c := NewCoordinator("0", "token")
	c.mu.Lock()
	// 9999 端口通常无服务
	c.nodes["n1"] = &NodeInfo{ID: "n1", Host: "127.0.0.1", Port: "9999", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 0}
	c.mu.Unlock()

	for i := 0; i < 5; i++ {
		task := &Task{ID: fmt.Sprintf("t%d", i), Status: TaskStatusPending}
		c.dispatchTask("n1", task)
	}
	time.Sleep(200 * time.Millisecond)
	if c.breakers.AllowRequest("n1") {
		t.Error("n1 should be blocked after 5 connection failures")
	}
}

func TestCoordinator_DispatchTask_UnknownNode(t *testing.T) {
	c := NewCoordinator("0", "token")
	task := &Task{ID: "t1", Status: TaskStatusPending}
	// 不应 panic
	c.dispatchTask("unknown-node", task)
	time.Sleep(50 * time.Millisecond)
}

// ─── cleanupOfflineNodes 测试 ───

func TestCoordinator_CleanupOfflineNodes_Stop(t *testing.T) {
	c := NewCoordinator("0", "token")
	done := make(chan struct{})
	go func() {
		c.cleanupOfflineNodes()
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	c.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanupOfflineNodes did not exit after Stop")
	}
}

// ─── executeWorkflowDistributed 测试 ───

func TestCoordinator_ExecuteWorkflowDistributed_NoNodes(t *testing.T) {
	c := NewCoordinator("0", "token")
	wf := &workflow.Workflow{Name: "test", Steps: []workflow.WorkflowStep{{Node: "x"}}}
	c.executeWorkflowDistributed(wf)
	// 无节点,应直接退出(无 panic)
}

func TestCoordinator_ExecuteWorkflowDistributed_EmptySteps(t *testing.T) {
	c := NewCoordinator("0", "token")
	wf := &workflow.Workflow{Name: "empty", Steps: nil}
	c.executeWorkflowDistributed(wf)
	// 0 步,应直接退出
}

func TestCoordinator_ExecuteWorkflowDistributed_WithTaskUpdate(t *testing.T) {
	c := NewCoordinator("0", "token")
	c.mu.Lock()
	c.nodes["n1"] = &NodeInfo{ID: "n1", Host: "127.0.0.1", Port: "9999", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 0}
	c.mu.Unlock()

	wf := &workflow.Workflow{Name: "single", Steps: []workflow.WorkflowStep{{Node: "s1"}}}

	done := make(chan struct{})
	go func() {
		c.executeWorkflowDistributed(wf)
		close(done)
	}()

	// 等待任务出现后标记完成,打破轮询循环
	if !waitForTaskAndSetStatus(c, 2*time.Second, TaskStatusCompleted) {
		t.Fatal("timed out waiting for task creation")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("executeWorkflowDistributed did not complete")
	}
}

func TestCoordinator_ExecuteWorkflowDistributed_StepFailed(t *testing.T) {
	c := NewCoordinator("0", "token")
	c.mu.Lock()
	c.nodes["n1"] = &NodeInfo{ID: "n1", Host: "127.0.0.1", Port: "9999", Status: NodeStatusIdle, LastHeartbeat: time.Now(), Capacity: 10, CurrentLoad: 0}
	c.mu.Unlock()

	// 2 步工作流;第一步失败后第二步不应执行
	wf := &workflow.Workflow{
		Name:  "multi",
		Steps: []workflow.WorkflowStep{{Node: "s1"}, {Node: "s2"}},
	}

	done := make(chan struct{})
	go func() {
		c.executeWorkflowDistributed(wf)
		close(done)
	}()

	if !waitForTaskAndSetStatus(c, 2*time.Second, TaskStatusFailed) {
		t.Fatal("timed out waiting for task creation")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("executeWorkflowDistributed did not complete after step failure")
	}

	// 失败后应只创建 1 个任务(第二步被跳过)
	c.mu.RLock()
	count := len(c.tasks)
	c.mu.RUnlock()
	if count != 1 {
		t.Errorf("expected 1 task (second step skipped after failure), got %d", count)
	}
}

// waitForTaskAndSetStatus 查找第一个 Pending 任务并将其状态设为指定值。
func waitForTaskAndSetStatus(c *Coordinator, timeout time.Duration, status TaskStatus) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		for _, task := range c.tasks {
			if task.Status == TaskStatusPending {
				task.Status = status
				if status == TaskStatusCompleted || status == TaskStatusFailed {
					end := time.Now()
					task.EndTime = &end
				}
				c.mu.Unlock()
				return true
			}
		}
		c.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// ─── executeStepLocally 测试 ───

func TestExecuteStepLocally(t *testing.T) {
	output, err := executeStepLocally(workflow.WorkflowStep{Node: "test"})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if output != "" {
		t.Errorf("expected empty output, got %s", output)
	}
}

// ─── Worker 测试 ───

func TestWorker_StartStop(t *testing.T) {
	coordSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/register" {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"node_id": "test-worker-1"})
			return
		}
		if r.URL.Path == "/api/heartbeat" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer coordSrv.Close()

	l, port := newTestListener(t)
	w, err := NewWorker(port, coordSrv.URL, "token", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w.SetServeListener(l)

	errCh := make(chan error, 1)
	go func() { errCh <- w.Start() }()

	waitForHTTPReady(t, "http://127.0.0.1:"+port+"/health", 2*time.Second)

	if err := w.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("Start returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Stop")
	}

	if w.nodeID != "test-worker-1" {
		t.Errorf("expected nodeID test-worker-1, got %s", w.nodeID)
	}
}

func TestWorker_StopWithoutStart(t *testing.T) {
	w, err := NewWorker("0", "http://localhost:8090", "token", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := w.Stop(); err != nil {
		t.Errorf("Stop without Start should return nil, got %v", err)
	}
}

func TestWorker_RegisterWithCoordinator_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/register" {
			// 验证请求体
			var req struct {
				Host     string `json:"host"`
				Port     string `json:"port"`
				Capacity int    `json:"capacity"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if req.Capacity != 5 {
				t.Errorf("expected capacity 5, got %d", req.Capacity)
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"node_id": "worker-42"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	w, _ := NewWorker("0", srv.URL, "token", 5)
	if err := w.registerWithCoordinator(); err != nil {
		t.Fatalf("registerWithCoordinator failed: %v", err)
	}
	if w.nodeID != "worker-42" {
		t.Errorf("expected nodeID worker-42, got %s", w.nodeID)
	}
}

func TestWorker_RegisterWithCoordinator_Failures(t *testing.T) {
	t.Run("Non201Status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer srv.Close()

		w, _ := NewWorker("0", srv.URL, "token", 1)
		if err := w.registerWithCoordinator(); err == nil {
			t.Error("expected error for non-201 status")
		}
	})

	t.Run("InvalidJSONResponse", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte("not-json"))
		}))
		defer srv.Close()

		w, _ := NewWorker("0", srv.URL, "token", 1)
		if err := w.registerWithCoordinator(); err == nil {
			t.Error("expected error for invalid JSON response")
		}
	})

	t.Run("ConnectionError", func(t *testing.T) {
		w, _ := NewWorker("0", "http://127.0.0.1:9999", "token", 1)
		if err := w.registerWithCoordinator(); err == nil {
			t.Error("expected error for connection failure")
		}
	})
}

func TestWorker_SendHeartbeats_Stop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	w, _ := NewWorker("0", srv.URL, "token", 1)
	w.nodeID = "test-node"

	done := make(chan struct{})
	go func() {
		w.sendHeartbeats()
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)

	if err := w.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sendHeartbeats did not exit after Stop")
	}
}

func TestWorker_HandleExecuteStep(t *testing.T) {
	t.Run("WrongMethod", func(t *testing.T) {
		w, _ := NewWorker("0", "http://localhost:8090", "token", 1)
		req := httptest.NewRequest("GET", "/api/execute-step", nil)
		rec := httptest.NewRecorder()
		w.handleExecuteStep(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		w, _ := NewWorker("0", "http://localhost:8090", "token", 1)
		req := httptest.NewRequest("POST", "/api/execute-step", strings.NewReader("bad"))
		rec := httptest.NewRecorder()
		w.handleExecuteStep(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("Valid", func(t *testing.T) {
		coordSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}))
		defer coordSrv.Close()

		w, _ := NewWorker("0", coordSrv.URL, "token", 1)
		body := bytes.NewReader([]byte(`{"task_id":"t1","step":{"node":"test"}}`))
		req := httptest.NewRequest("POST", "/api/execute-step", body)
		rec := httptest.NewRecorder()
		w.handleExecuteStep(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d", rec.Code)
		}

		// 等待后台 executeStep goroutine 完成
		time.Sleep(100 * time.Millisecond)

		w.mu.Lock()
		load := w.currentLoad
		w.mu.Unlock()
		if load != 0 {
			t.Errorf("expected currentLoad 0 after executeStep, got %d", load)
		}
	})
}

func TestWorker_ExecuteStep(t *testing.T) {
	coordSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer coordSrv.Close()

	w, _ := NewWorker("0", coordSrv.URL, "token", 1)
	w.mu.Lock()
	w.currentLoad = 1
	w.mu.Unlock()

	w.executeStep("task-1", workflow.WorkflowStep{Node: "test"})

	w.mu.Lock()
	load := w.currentLoad
	w.mu.Unlock()
	if load != 0 {
		t.Errorf("expected currentLoad 0 after executeStep, got %d", load)
	}
}

func TestWorker_UpdateTaskStatus(t *testing.T) {
	var mu sync.Mutex
	var receivedPath, receivedTaskID, receivedStatus string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedPath = r.URL.Path
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		receivedTaskID, _ = body["task_id"].(string)
		receivedStatus, _ = body["status"].(string)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	w, _ := NewWorker("0", srv.URL, "token", 1)
	w.updateTaskStatus("task-99", TaskStatusRunning)

	mu.Lock()
	defer mu.Unlock()
	if receivedPath != "/api/task" {
		t.Errorf("expected /api/task, got %s", receivedPath)
	}
	if receivedTaskID != "task-99" {
		t.Errorf("expected task_id task-99, got %s", receivedTaskID)
	}
	if receivedStatus != "running" {
		t.Errorf("expected status running, got %s", receivedStatus)
	}
}

func TestWorker_UpdateTaskResult(t *testing.T) {
	var mu sync.Mutex
	var receivedOutput, receivedError string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		receivedOutput, _ = body["output"].(string)
		receivedError, _ = body["error"].(string)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	w, _ := NewWorker("0", srv.URL, "token", 1)
	w.updateTaskResult("task-1", "result-data", "")

	mu.Lock()
	defer mu.Unlock()
	if receivedOutput != "result-data" {
		t.Errorf("expected output 'result-data', got %s", receivedOutput)
	}
	if receivedError != "" {
		t.Errorf("expected empty error, got %s", receivedError)
	}
}

func TestWorker_UpdateTaskResult_ConnectionError(t *testing.T) {
	w, _ := NewWorker("0", "http://127.0.0.1:9999", "token", 1)
	// 不应 panic;返回的错误被忽略
	w.updateTaskResult("task-1", "data", "err")
}

func TestWorker_HttpPost(t *testing.T) {
	var receivedToken, receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get("X-Auth-Token")
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	w, _ := NewWorker("0", srv.URL, "my-token", 1)
	resp, err := w.httpPost("/api/test", []byte(`{"key":"value"}`))
	if err != nil {
		t.Fatalf("httpPost failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if receivedToken != "my-token" {
		t.Errorf("expected token my-token, got %s", receivedToken)
	}
	if receivedPath != "/api/test" {
		t.Errorf("expected path /api/test, got %s", receivedPath)
	}
}

func TestWorker_HttpPut(t *testing.T) {
	var receivedMethod, receivedToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedToken = r.Header.Get("X-Auth-Token")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	w, _ := NewWorker("0", srv.URL, "put-token", 1)
	resp, err := w.httpPut("/api/task", []byte(`{"task_id":"t1"}`))
	if err != nil {
		t.Fatalf("httpPut failed: %v", err)
	}
	defer resp.Body.Close()
	if receivedMethod != http.MethodPut {
		t.Errorf("expected PUT, got %s", receivedMethod)
	}
	if receivedToken != "put-token" {
		t.Errorf("expected token put-token, got %s", receivedToken)
	}
}

func TestWorker_HttpPost_NoToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth-Token") != "" {
			t.Error("expected empty auth token header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	w, _ := NewWorker("0", srv.URL, "", 1)
	resp, err := w.httpPost("/api/test", []byte(`{}`))
	if err != nil {
		t.Fatalf("httpPost failed: %v", err)
	}
	defer resp.Body.Close()
}
