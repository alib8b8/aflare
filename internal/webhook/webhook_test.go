// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌​​​‌​‌​‌​​​‌‌​​‌​‌‌​‌‌‌‌‌​‌​‌​​‌​​​​​‌​​​​‌​‌​​​​​​​​​​​​​​​​‌‌​​‌‌‌‌‌​‌​‌​​​⁠
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

package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/history"
	"github.com/alib8b8/aflare/internal/nodes"
)

type echoNode struct {
	name string
}

func (n *echoNode) Name() string        { return n.name }
func (n *echoNode) Description() string { return "echo node" }
func (n *echoNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{
		Name:        n.name,
		Description: "echo node",
		Input:       "string",
		Output:      "string",
		Params: []nodes.ParamSchema{
			{Name: "prefix", Type: "string", Description: "prefix", Required: false},
		},
	}
}

func (n *echoNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if prefix, ok := params["prefix"]; ok {
		return prefix + " " + input, nil
	}
	return "echo: " + input, nil
}

type failNode struct {
	name string
}

func (n *failNode) Name() string        { return n.name }
func (n *failNode) Description() string { return "fail node" }
func (n *failNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Description: "fail node", Input: "string", Output: "string"}
}

func (n *failNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	return "", fmt.Errorf("intentional failure")
}

func setupTestServer(t *testing.T) (*WebhookServer, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "webhook-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	reg := nodes.NewRegistry()
	reg.Register(&echoNode{name: "echo"})
	reg.Register(&failNode{name: "fail"})

	srv := NewWebhookServer("", "", reg)
	srv.SetWorkflowsDir(tmpDir)

	cleanup := func() {
		_ = srv.Stop()
		os.RemoveAll(tmpDir)
	}

	return srv, tmpDir, cleanup
}

func createWorkflowFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}
}

func waitForTask(t *testing.T, srv *WebhookServer, taskID string) *Task {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if task, ok := srv.getTask(taskID); ok && (task.Status == TaskCompleted || task.Status == TaskFailed) {
			return task
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("task %s did not complete in time", taskID)
	return nil
}

func TestWebhookServer_Health(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/webhook/health")
	if err != nil {
		t.Fatalf("health check request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}
	if result["status"] != "healthy" {
		t.Errorf("expected healthy status, got %v", result)
	}
}

func TestWebhookServer_TriggerAndQueryStatus(t *testing.T) {
	srv, tmpDir, cleanup := setupTestServer(t)
	defer cleanup()

	wfContent := `name: test-workflow
steps:
  - node: echo
    params:
      prefix: "{{var.input}}"
`
	createWorkflowFile(t, tmpDir, "test", wfContent)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	body := []byte("hello")
	resp, err := http.Post(ts.URL+"/webhook/test", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", resp.StatusCode)
	}

	var triggerResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&triggerResp); err != nil {
		t.Fatalf("failed to decode trigger response: %v", err)
	}
	taskID := triggerResp["task_id"]
	if taskID == "" {
		t.Fatal("expected task_id in response")
	}

	task := waitForTask(t, srv, taskID)
	if task.Status != TaskCompleted {
		t.Errorf("expected task completed, got %s", task.Status)
	}
	if task.Output != "hello " {
		t.Errorf("expected output 'hello ', got %q", task.Output)
	}
	if task.Error != "" {
		t.Errorf("expected no error, got %q", task.Error)
	}

	// Query status endpoint
	statusResp, err := http.Get(ts.URL + "/webhook/status/" + taskID)
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	defer statusResp.Body.Close()

	if statusResp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", statusResp.StatusCode)
	}

	var statusTask Task
	if err := json.NewDecoder(statusResp.Body).Decode(&statusTask); err != nil {
		t.Fatalf("failed to decode status response: %v", err)
	}
	if statusTask.ID != taskID {
		t.Errorf("expected task id %s, got %s", taskID, statusTask.ID)
	}
}

// TestWebhookServer_StatusRequiresSecret 验证配置 secret 时,
// /webhook/status/ 端点也必须校验 X-Webhook-Secret。
func TestWebhookServer_StatusRequiresSecret(t *testing.T) {
	srv, tmpDir, cleanup := setupTestServer(t)
	defer cleanup()

	srv.secret = "status-secret"

	wfContent := `name: status-secret-workflow
steps:
  - node: echo
`
	createWorkflowFile(t, tmpDir, "statuswf", wfContent)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	// 触发任务(带正确 secret)以获得 task_id
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/webhook/statuswf", strings.NewReader("{}"))
	req.Header.Set("X-Webhook-Secret", "status-secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected status 202 for trigger, got %d", resp.StatusCode)
	}
	var triggerResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&triggerResp); err != nil {
		t.Fatalf("failed to decode trigger response: %v", err)
	}
	taskID := triggerResp["task_id"]
	if taskID == "" {
		t.Fatal("expected task_id in response")
	}
	_ = waitForTask(t, srv, taskID)

	// 无 secret 查询 status => 401
	statusResp, err := http.Get(ts.URL + "/webhook/status/" + taskID)
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for status without secret, got %d", statusResp.StatusCode)
	}

	// 错误 secret => 401
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/webhook/status/"+taskID, nil)
	req.Header.Set("X-Webhook-Secret", "wrong-secret")
	statusResp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for status with wrong secret, got %d", statusResp.StatusCode)
	}

	// 正确 secret => 200
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/webhook/status/"+taskID, nil)
	req.Header.Set("X-Webhook-Secret", "status-secret")
	statusResp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	defer statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for status with correct secret, got %d", statusResp.StatusCode)
	}
}

func TestWebhookServer_WorkflowNotFound(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/webhook/missing", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", resp.StatusCode)
	}

	var triggerResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&triggerResp); err != nil {
		t.Fatalf("failed to decode trigger response: %v", err)
	}
	taskID := triggerResp["task_id"]

	task := waitForTask(t, srv, taskID)
	if task.Status != TaskFailed {
		t.Errorf("expected task failed, got %s", task.Status)
	}
	if task.Error == "" {
		t.Error("expected error message for missing workflow")
	}
}

func TestWebhookServer_InvalidWorkflowName(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/webhook/invalid.name", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestWebhookServer_SecretAuth(t *testing.T) {
	srv, tmpDir, cleanup := setupTestServer(t)
	defer cleanup()

	srv.secret = "my-secret"

	wfContent := `name: secret-workflow
steps:
  - node: echo
`
	createWorkflowFile(t, tmpDir, "secret", wfContent)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	// Missing secret
	resp, err := http.Post(ts.URL+"/webhook/secret", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401 for missing secret, got %d", resp.StatusCode)
	}

	// Wrong secret
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/webhook/secret", strings.NewReader("{}"))
	req.Header.Set("X-Webhook-Secret", "wrong-secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401 for wrong secret, got %d", resp.StatusCode)
	}

	// Correct secret
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/webhook/secret", strings.NewReader("{}"))
	req.Header.Set("X-Webhook-Secret", "my-secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("expected status 202 for correct secret, got %d", resp.StatusCode)
	}
}

func TestWebhookServer_BodyTooLarge(t *testing.T) {
	srv, tmpDir, cleanup := setupTestServer(t)
	defer cleanup()

	wfContent := `name: large-workflow
steps:
  - node: echo
`
	createWorkflowFile(t, tmpDir, "large", wfContent)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	largeBody := make([]byte, maxBodySize+1)
	resp, err := http.Post(ts.URL+"/webhook/large", "application/octet-stream", bytes.NewReader(largeBody))
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d", resp.StatusCode)
	}
}

func TestWebhookServer_QueryParamsAsVars(t *testing.T) {
	srv, tmpDir, cleanup := setupTestServer(t)
	defer cleanup()

	wfContent := `name: query-workflow
steps:
  - node: echo
    params:
      prefix: "{{var.greeting}}"
`
	createWorkflowFile(t, tmpDir, "query", wfContent)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/webhook/query?greeting=hi", "application/json", strings.NewReader("world"))
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", resp.StatusCode)
	}

	var triggerResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&triggerResp); err != nil {
		t.Fatalf("failed to decode trigger response: %v", err)
	}
	taskID := triggerResp["task_id"]

	task := waitForTask(t, srv, taskID)
	if task.Status != TaskCompleted {
		t.Errorf("expected task completed, got %s", task.Status)
	}
	if task.Output != "hi " {
		t.Errorf("expected output 'hi ', got %q", task.Output)
	}
}

func TestWebhookServer_RateLimit(t *testing.T) {
	srv, tmpDir, cleanup := setupTestServer(t)
	defer cleanup()

	// Use a tight rate limit for testing
	srv.rateLimiter = NewRateLimiter(3, time.Second)

	wfContent := `name: rate-workflow
steps:
  - node: echo
`
	createWorkflowFile(t, tmpDir, "rate", wfContent)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	var accepted int
	var limited int
	for i := 0; i < 5; i++ {
		resp, err := http.Post(ts.URL+"/webhook/rate", "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("webhook request failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusAccepted {
			accepted++
		} else if resp.StatusCode == http.StatusTooManyRequests {
			limited++
		}
	}

	if accepted != 3 {
		t.Errorf("expected 3 accepted requests, got %d", accepted)
	}
	if limited != 2 {
		t.Errorf("expected 2 limited requests, got %d", limited)
	}
}

func TestWebhookServer_MethodNotAllowed(t *testing.T) {
	srv, tmpDir, cleanup := setupTestServer(t)
	defer cleanup()

	wfContent := `name: method-workflow
steps:
  - node: echo
`
	createWorkflowFile(t, tmpDir, "method", wfContent)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	// GET on webhook trigger path
	resp, err := http.Get(ts.URL + "/webhook/method")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", resp.StatusCode)
	}

	// POST on status path
	resp, err = http.Post(ts.URL+"/webhook/status/abc", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", resp.StatusCode)
	}

	// POST on health path
	resp, err = http.Post(ts.URL+"/webhook/health", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", resp.StatusCode)
	}
}

func TestWebhookServer_TaskCleanup(t *testing.T) {
	srv, _, cleanup := setupTestServer(t)
	defer cleanup()

	oldTime := time.Now().Add(-2 * time.Hour)
	recentTime := time.Now().Add(-30 * time.Minute)

	srv.mu.Lock()
	srv.tasks["old"] = &Task{ID: "old", Status: TaskCompleted, CompletedAt: &oldTime}
	srv.tasks["recent"] = &Task{ID: "recent", Status: TaskCompleted, CompletedAt: &recentTime}
	srv.tasks["pending"] = &Task{ID: "pending", Status: TaskPending}
	srv.mu.Unlock()

	// Simulate cleanup logic inline
	srv.mu.Lock()
	now := time.Now()
	for id, task := range srv.tasks {
		if task.Status == TaskCompleted || task.Status == TaskFailed {
			if task.CompletedAt != nil && now.Sub(*task.CompletedAt) > taskMaxAge {
				delete(srv.tasks, id)
			}
		}
	}
	srv.mu.Unlock()

	if _, ok := srv.tasks["old"]; ok {
		t.Error("old completed task should be cleaned up")
	}
	if _, ok := srv.tasks["recent"]; !ok {
		t.Error("recent completed task should remain")
	}
	if _, ok := srv.tasks["pending"]; !ok {
		t.Error("pending task should remain")
	}
}

func TestWebhookServer_FailingWorkflow(t *testing.T) {
	srv, tmpDir, cleanup := setupTestServer(t)
	defer cleanup()

	wfContent := `name: fail-workflow
steps:
  - node: fail
`
	createWorkflowFile(t, tmpDir, "failwf", wfContent)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/webhook/failwf", "application/json", strings.NewReader("test"))
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	defer resp.Body.Close()

	var triggerResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&triggerResp); err != nil {
		t.Fatalf("failed to decode trigger response: %v", err)
	}
	taskID := triggerResp["task_id"]

	task := waitForTask(t, srv, taskID)
	if task.Status != TaskFailed {
		t.Errorf("expected task failed, got %s", task.Status)
	}
	if task.Error == "" {
		t.Error("expected error message")
	}
}

// isolateWebhookAudit redirects the process-global history directory to a
// temp dir for one test, restoring the original afterwards. Needed because
// the webhook run path enables the audit log with the default dir.
func isolateWebhookAudit(t *testing.T) string {
	t.Helper()
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	origPath := history.GetAuditLogPath()
	dir := t.TempDir()
	history.SetHistoryDir(dir)
	t.Cleanup(func() {
		if origPath == "" {
			history.SetHistoryDir("")
			return
		}
		history.SetHistoryDir(filepath.Dir(origPath))
	})
	return dir
}

// readWebhookAuditActions parses the audit log in dir and returns its
// actions in file order. A missing file (nothing executed) yields nil.
func readWebhookAuditActions(t *testing.T, dir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "audit.log.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("failed to read audit log: %v", err)
	}
	var actions []string
	for _, l := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		var entry struct {
			Action string `json:"action"`
		}
		if err := json.Unmarshal([]byte(l), &entry); err != nil {
			t.Fatalf("failed to parse audit record %q: %v", l, err)
		}
		actions = append(actions, entry.Action)
	}
	return actions
}

// TestWebhookServer_RunWritesAuditRecords is the regression guard for the
// "entry point runs unaudited" bug class (self-test finding: the webhook
// run path previously used the bare package-level ExecuteWorkflow, leaving
// zero audit records). A triggered workflow must leave a start + per-step +
// end trail in the tamper-evident audit log, exactly like `aflare run`.
func TestWebhookServer_RunWritesAuditRecords(t *testing.T) {
	auditDir := isolateWebhookAudit(t)
	srv, tmpDir, cleanup := setupTestServer(t)
	defer cleanup()

	createWorkflowFile(t, tmpDir, "auditwf", `name: audit-workflow
steps:
  - node: echo
`)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/webhook/auditwf", "application/json", strings.NewReader("test"))
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	defer resp.Body.Close()

	var triggerResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&triggerResp); err != nil {
		t.Fatalf("failed to decode trigger response: %v", err)
	}
	task := waitForTask(t, srv, triggerResp["task_id"])
	if task.Status != TaskCompleted {
		t.Fatalf("expected task completed, got %s (error: %s)", task.Status, task.Error)
	}

	actions := readWebhookAuditActions(t, auditDir)
	want := []string{"workflow_start", "workflow_step", "workflow_end"}
	if len(actions) != len(want) {
		t.Fatalf("audit actions = %v, want %v", actions, want)
	}
	for i := range want {
		if actions[i] != want[i] {
			t.Errorf("audit action[%d] = %q, want %q", i, actions[i], want[i])
		}
	}
}

// TestWebhookServer_PolicyBlocks is the regression guard for the "entry
// point bypasses policy" bug class. file_delete is approval-required under
// DefaultPolicy and no human is on the webhook path, so the workflow must
// fail with a policy message BEFORE any step executes.
func TestWebhookServer_PolicyBlocks(t *testing.T) {
	auditDir := isolateWebhookAudit(t)
	srv, tmpDir, cleanup := setupTestServer(t)
	defer cleanup()

	createWorkflowFile(t, tmpDir, "delwf", `name: delete-workflow
steps:
  - node: echo
  - node: file_delete
    params:
      path: /tmp/aflare-webhook-policy-target
`)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/webhook/delwf", "application/json", strings.NewReader("test"))
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	defer resp.Body.Close()

	var triggerResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&triggerResp); err != nil {
		t.Fatalf("failed to decode trigger response: %v", err)
	}
	task := waitForTask(t, srv, triggerResp["task_id"])
	if task.Status != TaskFailed {
		t.Fatalf("expected task failed, got %s", task.Status)
	}
	if !strings.Contains(task.Error, "blocked by policy") {
		t.Errorf("task error should report policy block, got %q", task.Error)
	}

	// Blocked before execution → no audit records at all.
	if actions := readWebhookAuditActions(t, auditDir); len(actions) != 0 {
		t.Errorf("audit actions = %v, want none (workflow was blocked pre-execution)", actions)
	}
}

func TestWebhookServer_ConcurrentExecutions(t *testing.T) {
	srv, tmpDir, cleanup := setupTestServer(t)
	defer cleanup()

	wfContent := `name: concurrent-workflow
steps:
  - node: echo
    params:
      prefix: "{{var.input}}"
`
	createWorkflowFile(t, tmpDir, "concurrent", wfContent)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	var completed atomic.Int32
	for i := 0; i < 10; i++ {
		go func(idx int) {
			body := fmt.Sprintf("req-%d", idx)
			resp, err := http.Post(ts.URL+"/webhook/concurrent", "application/json", strings.NewReader(body))
			if err != nil {
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusAccepted {
				completed.Add(1)
			}
		}(i)
	}

	time.Sleep(2 * time.Second)
	if completed.Load() != 10 {
		t.Errorf("expected 10 accepted requests, got %d", completed.Load())
	}

	// Wait for all tasks to complete
	srv.mu.RLock()
	count := len(srv.tasks)
	srv.mu.RUnlock()

	if count != 10 {
		t.Errorf("expected 10 tasks, got %d", count)
	}
}

func TestWebhookServer_StopWaitsForTasks(t *testing.T) {
	srv, tmpDir, cleanup := setupTestServer(t)
	defer cleanup()

	wfContent := `name: stop-workflow
steps:
  - node: echo
`
	createWorkflowFile(t, tmpDir, "stopwf", wfContent)

	go func() {
		_ = srv.Start()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Trigger a task
	go func() {
		_, _ = http.Post("http://localhost:"+srv.port+"/webhook/stopwf", "application/json", strings.NewReader("test"))
	}()

	// Give task time to start
	time.Sleep(100 * time.Millisecond)

	// Stop should wait for task to complete
	done := make(chan error, 1)
	go func() {
		done <- srv.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("stop failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not complete in time")
	}
}

func TestRateLimiter(t *testing.T) {
	// Test with a 1-second window and 2 requests max
	limiter := NewRateLimiter(2, time.Second)

	if !limiter.Allow("192.168.1.1") {
		t.Error("first request should be allowed")
	}
	if !limiter.Allow("192.168.1.1") {
		t.Error("second request should be allowed")
	}
	if limiter.Allow("192.168.1.1") {
		t.Error("third request should be denied")
	}

	// Different IP should be allowed
	if !limiter.Allow("192.168.1.2") {
		t.Error("request from different IP should be allowed")
	}
}

func TestIsValidWorkflowName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"hello", true},
		{"hello-world", true},
		{"hello_world", true},
		{"hello123", true},
		{"", false},
		{"hello/world", false},
		{"../secret", false},
		{"hello.world", false},
		{strings.Repeat("a", 101), false},
	}

	for _, tt := range tests {
		if got := isValidWorkflowName(tt.name); got != tt.valid {
			t.Errorf("isValidWorkflowName(%q) = %v, want %v", tt.name, got, tt.valid)
		}
	}
}

// freePort returns a stringified free TCP port on 127.0.0.1 for test servers.
// There is an inherent race between closing the listener and binding again,
// but it is acceptable for tests.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("failed to close listener: %v", err)
	}
	return fmt.Sprintf("%d", port)
}

// TestWebhookDefaultLocalhost 验证 authToken(secret)为空时,默认绑 127.0.0.1。
func TestWebhookDefaultLocalhost(t *testing.T) {
	reg := nodes.NewRegistry()
	srv := NewWebhookServer("9090", "", reg) // 空 secret => 无认证模式

	if addr := srv.resolveAddr(); addr != "127.0.0.1:9090" {
		t.Errorf("expected 127.0.0.1:9090 when no auth secret, got %s", addr)
	}
}

// TestWebhookCustomHost 验证用户显式设置 host 时,host 优先于默认 localhost。
func TestWebhookCustomHost(t *testing.T) {
	reg := nodes.NewRegistry()
	srv := NewWebhookServer("9090", "", reg)
	srv.SetHost("0.0.0.0")

	if addr := srv.resolveAddr(); addr != "0.0.0.0:9090" {
		t.Errorf("expected 0.0.0.0:9090 when host set, got %s", addr)
	}

	// 已配置 secret 但 host 为空时,应绑全接口(由认证保护)
	secretSrv := NewWebhookServer("9091", "some-secret", reg)
	if addr := secretSrv.resolveAddr(); addr != ":9091" {
		t.Errorf("expected :9091 when secret set and host empty, got %s", addr)
	}
}

// TestWebhookAuthToken 验证 secret(auth token)设置时,
// 无 token 请求返回 401,正确 token 通过。
func TestWebhookAuthToken(t *testing.T) {
	srv, tmpDir, cleanup := setupTestServer(t)
	defer cleanup()

	srv.secret = "test-auth-token"

	wfContent := `name: auth-workflow
steps:
  - node: echo
`
	createWorkflowFile(t, tmpDir, "authwf", wfContent)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	// 无 token => 401
	resp, err := http.Post(ts.URL+"/webhook/authwf", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing token, got %d", resp.StatusCode)
	}

	// 错误 token => 401
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/webhook/authwf", strings.NewReader("{}"))
	req.Header.Set("X-Webhook-Secret", "wrong-token")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong token, got %d", resp.StatusCode)
	}

	// 正确 token => 202
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/webhook/authwf", strings.NewReader("{}"))
	req.Header.Set("X-Webhook-Secret", "test-auth-token")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("webhook request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("expected 202 for correct token, got %d", resp.StatusCode)
	}
}

// TestWebhookCleanupTasksGracefulShutdown 验证启动后立即 Stop 不阻塞、不 panic,
// 即 cleanupTasks goroutine 已正确纳入 WaitGroup。
func TestWebhookCleanupTasksGracefulShutdown(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&echoNode{name: "echo"})
	port := freePort(t)
	srv := NewWebhookServer(port, "", reg)

	startErr := make(chan error, 1)
	go func() {
		startErr <- srv.Start()
	}()

	// 给服务器时间启动并运行 cleanupTasks goroutine
	time.Sleep(150 * time.Millisecond)

	done := make(chan error, 1)
	go func() {
		done <- srv.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not complete in time (cleanupTasks wg not draining)")
	}

	// Start 应已因 Shutdown 返回(http.ErrServerClosed 是预期行为)
	select {
	case err := <-startErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Start returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start goroutine did not return after Stop")
	}
}
