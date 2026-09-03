// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌‌​​​‌‌​​​​​​‌​​​‌​‌​‌​​​‌‌​​‌‌​​​​‌​‌‌‌‌​‌‌​​‌​‌‌​​‌​‌‌​​‌‌​​​​​​​​​​​​​​​​​​​​​‌‌​​​‌‌‌‌‌‌​‌‌​​⁠
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

package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/history"
)

// isolateAudit redirects the process-global history directory to a temp dir
// for the duration of one test, restoring the original afterwards. Needed
// because the API run handler enables the audit log with the default dir.
func isolateAudit(t *testing.T) string {
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

// readAuditActions parses the audit log in dir and returns its actions in
// file order (oldest first). A missing file means no records were written
// (e.g. a workflow blocked by policy before execution) and yields nil.
func readAuditActions(t *testing.T, dir string) []string {
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

// TestHandleRunWorkflow_WritesAuditRecords is the regression guard for the
// "remote entry point runs unaudited" bug class (self-test finding: the API
// run handler previously executed workflows with a bare executor, leaving
// zero audit records). Executing a workflow through POST
// /api/v1/workflows/run must leave a start + per-step + end trail in the
// tamper-evident audit log, exactly like `aflare run`.
func TestHandleRunWorkflow_WritesAuditRecords(t *testing.T) {
	auditDir := isolateAudit(t)
	s := newTestServer("")

	wf := `name: api-audit-test
steps:
  - node: template_render
    params:
      template: "hello audit"
`
	r := newJSONRequest(t, http.MethodPost, "/api/v1/workflows/run",
		`{"workflow":`+mustJSON(t, wf)+`}`)
	w := s.serve(t, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (body: %s)", w.Code, w.Body.String())
	}

	actions := readAuditActions(t, auditDir)
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

// TestHandleRunWorkflow_PolicyBlocks is the regression guard for the "remote
// entry point bypasses policy" bug class. file_delete is approval-required
// under DefaultPolicy and the API path has no human to approve, so the
// workflow must be rejected with 400 BEFORE any step executes.
func TestHandleRunWorkflow_PolicyBlocks(t *testing.T) {
	auditDir := isolateAudit(t)
	s := newTestServer("")

	wf := `name: api-policy-test
steps:
  - node: template_render
    params:
      template: "first"
  - node: file_delete
    params:
      path: /tmp/aflare-policy-test-target
`
	r := newJSONRequest(t, http.MethodPost, "/api/v1/workflows/run",
		`{"workflow":`+mustJSON(t, wf)+`}`)
	w := s.serve(t, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "policy violation") {
		t.Errorf("body should report policy violation: %s", w.Body.String())
	}

	// Nothing executed, so nothing was audited either.
	if actions := readAuditActions(t, auditDir); len(actions) != 0 {
		t.Errorf("audit actions = %v, want none (workflow was blocked pre-execution)", actions)
	}
}
