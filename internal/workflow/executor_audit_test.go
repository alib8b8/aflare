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

package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/history"
	"github.com/alib8b8/aflare/internal/nodes"
)

// restoreAuditDir restores the global history directory captured as an audit
// log path (or "" when unset) once a test is done. history.SetHistoryDir is
// process-global, so each test must restore it to avoid leaking into siblings.
func restoreAuditDir(origPath string) {
	if origPath == "" {
		history.SetHistoryDir("")
		return
	}
	history.SetHistoryDir(filepath.Dir(origPath))
}

// captureAndIsolateAudit captures the current audit log path, redirects the
// global history directory to dir, and registers a deferred restore. It
// returns a cleanup function.
func captureAndIsolateAudit(t *testing.T, dir string) {
	t.Helper()
	origPath := history.GetAuditLogPath()
	history.SetHistoryDir(dir)
	t.Cleanup(func() { restoreAuditDir(origPath) })
}

// readAuditFileLines reads the audit log file at path and returns its
// non-empty lines in file order (oldest first).
func readAuditFileLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}
	var lines []string
	for _, l := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// TestExecutor_AuditLog_RecordsAllSteps runs a 3-step workflow with audit
// enabled and verifies the audit log contains exactly 5 records
// (workflow_start + 3 workflow_step + workflow_end) and that the HMAC chain
// is intact. It also checks that a sensitive step parameter (api_key) is
// sanitized before being written.
func TestExecutor_AuditLog_RecordsAllSteps(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	captureAndIsolateAudit(t, t.TempDir())

	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wf := &Workflow{
		Name: "audit-3step",
		Steps: []WorkflowStep{
			{Name: "s1", Node: "test", Params: map[string]string{"prefix": "first", "api_key": "supersecret"}},
			{Name: "s2", Node: "test", Params: map[string]string{"prefix": "second"}},
			{Name: "s3", Node: "test", Params: map[string]string{"prefix": "third"}},
		},
	}

	exec := NewExecutor().WithAuditLog(true, "")
	out, results, err := exec.Execute(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 step results, got %d", len(results))
	}
	if out != "third second first " {
		t.Errorf("unexpected output %q", out)
	}

	auditPath := history.GetAuditLogPath()
	if auditPath == "" {
		t.Fatal("audit log path is empty")
	}
	lines := readAuditFileLines(t, auditPath)
	if len(lines) != 5 {
		t.Fatalf("expected 5 audit records (start+3steps+complete), got %d", len(lines))
	}

	// Verify action distribution and ordering (file order: start, 3 steps, end).
	expectedActions := []string{"workflow_start", "workflow_step", "workflow_step", "workflow_step", "workflow_end"}
	for i, l := range lines {
		var entry history.AuditLog
		if err := json.Unmarshal([]byte(l), &entry); err != nil {
			t.Fatalf("line %d: failed to parse audit record: %v", i, err)
		}
		if string(entry.Action) != expectedActions[i] {
			t.Errorf("line %d: expected action %q, got %q", i, expectedActions[i], entry.Action)
		}
		if entry.PrevHash == "" || entry.CurrHash == "" {
			t.Errorf("line %d: audit record missing hash fields", i)
		}
	}

	// The whole chain must verify.
	valid, brokenAt, verr := history.VerifyAuditChain(auditPath)
	if verr != nil {
		t.Fatalf("VerifyAuditChain error: %v", verr)
	}
	if !valid {
		t.Errorf("expected valid audit chain, broken at line %d", brokenAt)
	}

	// The sensitive api_key value must never appear in the audit log.
	raw, _ := os.ReadFile(auditPath)
	if strings.Contains(string(raw), "supersecret") {
		t.Error("audit log leaked unsanitized api_key value (\"supersecret\")")
	}
	// Parse the step-0 audit record's detail and verify the api_key param was
	// sanitized while the non-sensitive prefix was preserved. Detail is stored
	// as a JSON-encoded string, so it must be unmarshaled twice.
	sawSanitizedStep0 := false
	for _, l := range lines {
		var entry history.AuditLog
		if err := json.Unmarshal([]byte(l), &entry); err != nil {
			continue
		}
		if entry.Action != "workflow_step" {
			continue
		}
		var detail map[string]interface{}
		if err := json.Unmarshal([]byte(entry.Detail), &detail); err != nil {
			continue
		}
		if fmt.Sprint(detail["step_index"]) != "0" {
			continue
		}
		params, _ := detail["params"].(map[string]interface{})
		if params["api_key"] != "***" {
			t.Errorf("expected api_key sanitized to \"***\", got %v", params["api_key"])
		}
		if params["prefix"] != "first" {
			t.Errorf("expected non-sensitive prefix preserved as \"first\", got %v", params["prefix"])
		}
		sawSanitizedStep0 = true
		break
	}
	if !sawSanitizedStep0 {
		t.Error("did not find step_index 0 audit record to verify sanitization")
	}
}

// TestExecutor_AuditLog_HMACChain runs the same workflow twice and verifies
// that the second execution's records extend the first execution's hash chain
// continuously (no broken link across the run boundary).
func TestExecutor_AuditLog_HMACChain(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "chain-key")
	captureAndIsolateAudit(t, t.TempDir())

	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wf := &Workflow{
		Name: "chain-wf",
		Steps: []WorkflowStep{
			{Node: "test", Params: map[string]string{"prefix": "a"}},
			{Node: "test", Params: map[string]string{"prefix": "b"}},
		},
	}
	exec := NewExecutor().WithAuditLog(true, "")

	// Each run: start + 2 steps + complete = 4 records.
	if _, _, err := exec.Execute(context.Background(), wf, reg); err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	if _, _, err := exec.Execute(context.Background(), wf, reg); err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	auditPath := history.GetAuditLogPath()
	lines := readAuditFileLines(t, auditPath)
	if len(lines) != 8 {
		t.Fatalf("expected 8 audit records (2 runs x 4), got %d", len(lines))
	}

	// Explicitly check the cross-run boundary: the first record of run 2
	// (line index 4) must link its prev_hash to the curr_hash of run 1's
	// last record (line index 3).
	var lastOfRun1, firstOfRun2 history.AuditLog
	if err := json.Unmarshal([]byte(lines[3]), &lastOfRun1); err != nil {
		t.Fatalf("parse line 4: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[4]), &firstOfRun2); err != nil {
		t.Fatalf("parse line 5: %v", err)
	}
	if firstOfRun2.PrevHash != lastOfRun1.CurrHash {
		t.Errorf("chain broken across runs: prev_hash=%s != curr_hash=%s", firstOfRun2.PrevHash, lastOfRun1.CurrHash)
	}

	// The entire chain (both runs) must verify as a single continuous chain.
	valid, brokenAt, verr := history.VerifyAuditChain(auditPath)
	if verr != nil {
		t.Fatalf("VerifyAuditChain error: %v", verr)
	}
	if !valid {
		t.Errorf("expected continuous chain across two runs, broken at line %d", brokenAt)
	}
}

// TestExecutor_AuditLog_AutoKeyFileWhenEnvUnset verifies the 0.9.0 behavior:
// with neither AFLARE_AUDIT_HMAC_KEY nor AFLARE_SECRETS_PASSWORD set, the
// workflow still executes successfully AND audit records are written, signed
// with the per-install key file auto-generated by the history package. (The
// pre-0.9.0 recorder skipped all records without an env key, which silently
// disabled workflow audit on fresh installs after auto-key-files landed.)
func TestExecutor_AuditLog_AutoKeyFileWhenEnvUnset(t *testing.T) {
	// Force both key sources to be empty.
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "")
	t.Setenv("AFLARE_SECRETS_PASSWORD", "")
	dir := t.TempDir()
	captureAndIsolateAudit(t, dir)

	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wf := &Workflow{
		Name: "auto-key-wf",
		Steps: []WorkflowStep{
			{Node: "test", Params: map[string]string{"prefix": "x"}},
			{Node: "test", Params: map[string]string{"prefix": "y"}},
		},
	}

	exec := NewExecutor().WithAuditLog(true, "")
	out, results, err := exec.Execute(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow must succeed without HMAC key, got: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if out != "y x " {
		t.Errorf("unexpected output %q", out)
	}

	logs, err := history.ReadAuditLogs()
	if err != nil {
		t.Fatalf("ReadAuditLogs: %v", err)
	}
	// workflow_start + 2 steps + workflow_end
	if len(logs) != 4 {
		t.Fatalf("expected 4 audit records signed with the auto key file, got %d", len(logs))
	}

	// The auto-generated key file must exist in the isolated history dir and
	// the chain it produced must verify.
	if _, err := os.Stat(filepath.Join(dir, "audit-hmac.key")); err != nil {
		t.Fatalf("auto key file not generated in history dir: %v", err)
	}
	auditPath := filepath.Join(dir, "audit.log.jsonl")
	valid, brokenAt, verr := history.VerifyAuditChain(auditPath)
	if verr != nil || !valid {
		t.Fatalf("audit chain must verify under the auto key file: valid=%v brokenAt=%d err=%v", valid, brokenAt, verr)
	}
}

// TestExecutor_AuditLog_WriteFailureDoesNotBlock verifies that an audit write
// failure (here: history dir points at a regular file so MkdirAll fails) does
// not prevent the workflow from completing.
func TestExecutor_AuditLog_WriteFailureDoesNotBlock(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")

	// Point the history dir at a regular file: AppendAuditLog's MkdirAll will
	// fail with ENOTDIR, exercising the non-blocking error path. This is
	// root-safe (unlike a chmod 0555 trick).
	blockFile, err := os.CreateTemp("", "audit-block-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	blockPath := blockFile.Name()
	if err := blockFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(blockPath) })

	origPath := history.GetAuditLogPath()
	history.SetHistoryDir(blockPath)
	t.Cleanup(func() { restoreAuditDir(origPath) })

	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wf := &Workflow{
		Name: "block-wf",
		Steps: []WorkflowStep{
			{Node: "test", Params: map[string]string{"prefix": "p1"}},
			{Node: "test", Params: map[string]string{"prefix": "p2"}},
		},
	}

	exec := NewExecutor().WithAuditLog(true, "")
	out, results, err := exec.Execute(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow must complete despite audit write failure, got: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if out != "p2 p1 " {
		t.Errorf("unexpected output %q", out)
	}

	// No audit log file should have been creatable under the file-as-dir path.
	// os.Stat returns ENOTDIR (not os.IsNotExist) when the parent is a file,
	// so any non-nil stat error means the file was not created.
	if _, statErr := os.Stat(filepath.Join(blockPath, "audit.log.jsonl")); statErr == nil {
		t.Error("expected no audit log file to be created under the unwritable path")
	}
}

// TestExecutor_AuditLog_IdempotencyHitRecorded verifies the H-7 fix: when an
// idempotency key is served from the cache, a workflow_idempotent_hit audit
// record is written (instead of the normal start/steps/end sequence that the
// cache-hit path skips). Financial scenarios require that "a transfer was
// served from cache" leaves an auditable trail.
//
// Not parallel: it sets AFLARE_AUDIT_HMAC_KEY via t.Setenv.
func TestExecutor_AuditLog_IdempotencyHitRecorded(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "idemp-audit-hit-key")
	captureAndIsolateAudit(t, t.TempDir())

	var counter int
	reg := newIdempTestRegistry(&counter)
	wf := twoStepCounterWorkflow()
	store := NewFileIdempotencyStore(t.TempDir(), 0)
	exec := NewExecutor().
		WithAuditLog(true, "").
		WithIdempotencyKey("transfer-audit-hit").
		WithIdempotencyStore(store)

	// Run 1: real execution writes start + 2 steps + end = 4 records.
	if _, _, err := exec.Execute(context.Background(), wf, reg); err != nil {
		t.Fatalf("run1: expected success, got: %v", err)
	}
	if counter != 2 {
		t.Fatalf("run1: expected counter=2, got %d", counter)
	}

	// Run 2: idempotency hit — must write exactly one workflow_idempotent_hit
	// record and no start/steps/end (the cache-hit path returns before
	// recordStart).
	if _, _, err := exec.Execute(context.Background(), wf, reg); !errors.Is(err, ErrIdempotencyHit) {
		t.Fatalf("run2: expected ErrIdempotencyHit, got: %v", err)
	}
	if counter != 2 {
		t.Fatalf("run2: counter must stay 2 (no re-execution), got %d", counter)
	}

	auditPath := history.GetAuditLogPath()
	if auditPath == "" {
		t.Fatal("audit log path is empty")
	}
	lines := readAuditFileLines(t, auditPath)
	// 4 records from run1 + 1 idempotent_hit from run2.
	if len(lines) != 5 {
		t.Fatalf("expected 5 audit records (4 from run1 + 1 idempotent_hit), got %d", len(lines))
	}

	var hitEntry history.AuditLog
	if err := json.Unmarshal([]byte(lines[4]), &hitEntry); err != nil {
		t.Fatalf("parse hit entry: %v", err)
	}
	if string(hitEntry.Action) != "workflow_idempotent_hit" {
		t.Errorf("expected action workflow_idempotent_hit, got %q", hitEntry.Action)
	}
	if !hitEntry.Success {
		t.Errorf("idempotent_hit record should have Success=true, got false")
	}

	// Detail must carry the cached run_id, status, and a sanitized key prefix,
	// but NOT the raw key.
	var detail map[string]interface{}
	if err := json.Unmarshal([]byte(hitEntry.Detail), &detail); err != nil {
		t.Fatalf("parse hit detail: %v", err)
	}
	if _, ok := detail["idempotency_key"]; !ok {
		t.Errorf("detail missing idempotency_key: %v", detail)
	}
	if _, ok := detail["run_id"]; !ok {
		t.Errorf("detail missing run_id: %v", detail)
	}
	if detail["status"] != "completed" {
		t.Errorf("expected status=completed, got %v", detail["status"])
	}
	// The raw business key must never appear in the audit log.
	raw, _ := os.ReadFile(auditPath)
	if strings.Contains(string(raw), "transfer-audit-hit") {
		t.Errorf("audit log leaked raw idempotency key %q", "transfer-audit-hit")
	}

	// The whole chain (run1 + hit) must still verify as a single continuous
	// chain — the hit record extends it, it does not break it.
	valid, brokenAt, verr := history.VerifyAuditChain(auditPath)
	if verr != nil {
		t.Fatalf("VerifyAuditChain error: %v", verr)
	}
	if !valid {
		t.Errorf("expected valid audit chain including the idempotent_hit record, broken at line %d", brokenAt)
	}
}

// TestExecutor_AuditLog_IdempotencyRejectedRecorded verifies the H-7 fix for
// the in_progress branch: when a same-key request is suppressed because another
// run is in progress, a workflow_idempotent_rejected audit record is written
// with Success=false and the in-progress run_id, so the operator can attribute
// the suppression to the live run.
//
// Not parallel: it sets AFLARE_AUDIT_HMAC_KEY via t.Setenv.
func TestExecutor_AuditLog_IdempotencyRejectedRecorded(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "idemp-audit-rej-key")
	captureAndIsolateAudit(t, t.TempDir())

	var counter int
	reg := newIdempTestRegistry(&counter)
	wf := twoStepCounterWorkflow()
	store := NewMemoryIdempotencyStore(0)
	exec := NewExecutor().
		WithAuditLog(true, "").
		WithIdempotencyKey("transfer-audit-rej").
		WithIdempotencyStore(store)

	// Simulate a concurrent in-progress run by reserving the placeholder
	// directly, bypassing execution. This stamps an in_progress record that
	// the Executor's Check will observe and reject against.
	if _, ok, err := store.Reserve("transfer-audit-rej", "run-other"); err != nil || !ok {
		t.Fatalf("Reserve: expected (rec, true, nil), got (ok=%v, err=%v)", ok, err)
	}

	// The executor must reject without executing and write a single
	// workflow_idempotent_rejected audit record.
	_, _, err := exec.Execute(context.Background(), wf, reg)
	if !errors.Is(err, ErrIdempotencyInProgress) {
		t.Fatalf("expected ErrIdempotencyInProgress, got: %v", err)
	}
	if counter != 0 {
		t.Fatalf("counter must stay 0 (no execution), got %d", counter)
	}

	auditPath := history.GetAuditLogPath()
	if auditPath == "" {
		t.Fatal("audit log path is empty")
	}
	lines := readAuditFileLines(t, auditPath)
	if len(lines) != 1 {
		t.Fatalf("expected 1 audit record (idempotent_rejected), got %d", len(lines))
	}
	var rejEntry history.AuditLog
	if err := json.Unmarshal([]byte(lines[0]), &rejEntry); err != nil {
		t.Fatalf("parse rejected entry: %v", err)
	}
	if string(rejEntry.Action) != "workflow_idempotent_rejected" {
		t.Errorf("expected action workflow_idempotent_rejected, got %q", rejEntry.Action)
	}
	if rejEntry.Success {
		t.Errorf("idempotent_rejected should have Success=false (the run was not served)")
	}
	// Detail must reference the in-progress run_id and status, but NOT the raw key.
	var detail map[string]interface{}
	if err := json.Unmarshal([]byte(rejEntry.Detail), &detail); err != nil {
		t.Fatalf("parse rejected detail: %v", err)
	}
	if detail["run_id"] != "run-other" {
		t.Errorf("expected run_id=run-other, got %v", detail["run_id"])
	}
	if detail["status"] != "in_progress" {
		t.Errorf("expected status=in_progress, got %v", detail["status"])
	}
	raw, _ := os.ReadFile(auditPath)
	if strings.Contains(string(raw), "transfer-audit-rej") {
		t.Errorf("audit log leaked raw idempotency key %q", "transfer-audit-rej")
	}
}

// TestExecutor_AuditLog_RecordsCostAttribution verifies that the workflow_end
// audit record carries cost_usd and total_tokens fields, proving the
// trace→audit cost-attribution wiring. This is the end-to-end integration
// check that complements the unit tests in llm_pricing_test.go: the unit
// tests prove cost is computed correctly, this test proves it reaches the
// tamper-evident audit log. The workflow here uses a non-LLM testNode, so the
// cost is 0 — but the fields must still be PRESENT (a missing field would mean
// the trace was not passed to the recorder, which would hide real cost on LLM
// workflows). The field-presence assertion is the contract; the value is
// exercised by computeLLMCost's unit tests.
func TestExecutor_AuditLog_RecordsCostAttribution(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	captureAndIsolateAudit(t, t.TempDir())

	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wf := &Workflow{
		Name: "audit-cost",
		Steps: []WorkflowStep{
			{Name: "s1", Node: "test", Params: map[string]string{"prefix": "ok"}},
		},
	}

	exec := NewExecutor().WithAuditLog(true, "")
	if _, _, err := exec.Execute(context.Background(), wf, reg); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	auditPath := history.GetAuditLogPath()
	lines := readAuditFileLines(t, auditPath)
	// Find the workflow_end record (last line).
	var endDetail map[string]interface{}
	for _, l := range lines {
		var entry history.AuditLog
		if err := json.Unmarshal([]byte(l), &entry); err != nil {
			continue
		}
		if string(entry.Action) == "workflow_end" {
			if err := json.Unmarshal([]byte(entry.Detail), &endDetail); err != nil {
				t.Fatalf("failed to parse workflow_end detail: %v", err)
			}
			break
		}
	}
	if endDetail == nil {
		t.Fatal("no workflow_end audit record found")
	}
	// cost_usd must be present (value 0 here because testNode is not an LLM
	// node; the field's PRESENCE is the contract — it proves the trace was
	// passed to the recorder, so real LLM workflows will carry real cost).
	if _, ok := endDetail["cost_usd"]; !ok {
		t.Errorf("workflow_end audit detail missing cost_usd field; detail=%v", endDetail)
	}
	if _, ok := endDetail["total_tokens"]; !ok {
		t.Errorf("workflow_end audit detail missing total_tokens field; detail=%v", endDetail)
	}
}
