// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​​‌‌‌‌​​​‌‌‌​​​‌​‌‌​‌‌​‌‌‌​​​​​‌​‌​‌​‌‌‌​‌‌‌​‌​​​​​​​​​​​​​​​​‌‌​‌​‌‌‌​​‌‌​​​​⁠
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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/history"
	"github.com/alib8b8/aflare/internal/nodes"
)

// TestHasResumableSteps covers the analysis helper the CLI uses to decide
// whether a fresh run should auto-enable a WAL: only top-level resumable
// flags count, because compound steps cannot pause the workflow.
func TestHasResumableSteps(t *testing.T) {
	if HasResumableSteps(nil) {
		t.Error("nil workflow should have no resumable steps")
	}
	if HasResumableSteps(&Workflow{Name: "empty"}) {
		t.Error("workflow with no steps should have no resumable steps")
	}
	if HasResumableSteps(&Workflow{Steps: []WorkflowStep{
		{Node: "template_render"},
		{Node: "human_in_loop"},
	}}) {
		t.Error("plain steps should not be resumable")
	}
	if !HasResumableSteps(&Workflow{Steps: []WorkflowStep{
		{Node: "template_render"},
		{Node: "human_in_loop", Resumable: true},
	}}) {
		t.Error("top-level resumable flag should be detected")
	}
	// A resumable flag on a nested sub-step must NOT count: only top-level
	// regular steps route through handleStepFailure's pause path.
	if HasResumableSteps(&Workflow{Steps: []WorkflowStep{
		{Node: "http_request", CaptureError: []WorkflowStep{
			{Node: "human_in_loop", Resumable: true},
		}},
	}}) {
		t.Error("nested resumable flag should not count (compound steps cannot pause)")
	}
}

// TestPauseResume_EndToEnd drives the full pause→resume lifecycle through
// the public Executor API with a resumable human_in_loop gate:
//
//  1. Run pauses at the gate; the run dir must contain meta.json (with the
//     source workflow path recorded), a workflow.yaml copy, and a WAL.
//  2. Once the approval env var is set, ResumeWorkflow replays the WAL,
//     skips the completed first step, and restores its output by NAME so
//     the final template's {{step.first}} reference resolves.
//  3. The run metadata ends up "completed".
//
// HOME is redirected to a temp dir so run metadata never lands in the real
// user home; the test therefore cannot run in parallel.
func TestPauseResume_EndToEnd(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// env-mode approval keeps the test filesystem-clean: the gate fails
	// while RESUME_TEST_APPROVED is unset and approves once it is set.
	wfYAML := `name: resume-pause-test
steps:
  - node: template_render
    name: first
    params:
      template: "static-marker"
  - node: human_in_loop
    name: gate
    resumable: true
    resume_on: manual
    params:
      mode: env
      approval_env: RESUME_TEST_APPROVED
  - node: template_render
    name: last
    params:
      template: "done-{{step.first}}"
`
	wfPath := filepath.Join(tmp, "wf.yaml")
	if err := os.WriteFile(wfPath, []byte(wfYAML), 0o600); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	wf, err := ParseWorkflow(wfPath)
	if err != nil {
		t.Fatalf("parse workflow: %v", err)
	}
	// ResumeWorkflow always runs against the global registry; the builtins
	// this workflow needs self-register there via init().
	reg := nodes.GetGlobalRegistry()
	for _, name := range []string{"template_render", "human_in_loop"} {
		if _, ok := reg.Get(name); !ok {
			t.Fatalf("required node %q missing from global registry", name)
		}
	}

	walPath := filepath.Join(tmp, "work.wal")
	exec := NewExecutor().WithWAL(walPath).WithWorkflowPath(wfPath)
	_, _, err = exec.Execute(context.Background(), wf, reg)

	// Run 1 must pause (not plain-fail) at the gate step.
	var paused *ErrWorkflowPaused
	if !errors.As(err, &paused) {
		t.Fatalf("run1: expected ErrWorkflowPaused, got: %v", err)
	}
	if paused.StepIndex != 1 || paused.StepName != "gate" {
		t.Errorf("run1: paused at step %d (%s), want 1 (gate)", paused.StepIndex, paused.StepName)
	}

	// Run dir artifacts: meta.json records the source workflow path and a
	// copy of the workflow file exists alongside the WAL snapshot.
	meta, err := LoadRunMeta(paused.RunID)
	if err != nil {
		t.Fatalf("LoadRunMeta: %v", err)
	}
	if meta.Status != "paused" {
		t.Errorf("meta status = %q, want paused", meta.Status)
	}
	if meta.WorkflowPath != wfPath {
		t.Errorf("meta workflow_path = %q, want %q", meta.WorkflowPath, wfPath)
	}
	if _, err := os.Stat(WorkflowPath(paused.RunID)); err != nil {
		t.Errorf("workflow copy missing from run dir: %v", err)
	}
	if _, err := os.Stat(WALPath(paused.RunID)); err != nil {
		t.Errorf("WAL copy missing from run dir: %v", err)
	}

	// Approve, then resume. The WAL must replay step 0's output so the
	// final template's {{step.first}} name reference resolves — this is the
	// regression guard for RestoreState resolving names from the workflow
	// definition instead of the (empty) engine stepNames map.
	t.Setenv("RESUME_TEST_APPROVED", "1")
	out, _, err := ResumeWorkflow(context.Background(), paused.RunID)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if !strings.Contains(out, "done-static-marker") {
		t.Errorf("resume output = %q, want it to contain %q (restored step output by name)", out, "done-static-marker")
	}

	meta, err = LoadRunMeta(paused.RunID)
	if err != nil {
		t.Fatalf("LoadRunMeta after resume: %v", err)
	}
	if meta.Status != "completed" {
		t.Errorf("meta status after resume = %q, want completed", meta.Status)
	}
}

// TestResumeWorkflow_WritesAuditRecords is the regression guard for resume
// audit parity with `aflare run`: a resumed run must extend the HMAC
// hash-chained audit log (workflow_start ... workflow_end) rather than
// executing invisibly. Before the fix, ResumeWorkflow built a bare executor
// with no WithAuditLog, so resumed runs left zero audit records.
func TestResumeWorkflow_WritesAuditRecords(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "resume-audit-test-key")
	auditDir := t.TempDir()
	captureAndIsolateAudit(t, auditDir)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	wfYAML := `name: resume-audit-test
steps:
  - node: human_in_loop
    name: gate
    resumable: true
    params:
      mode: env
      approval_env: RESUME_AUDIT_APPROVED
  - node: template_render
    name: last
    params:
      template: "ok"
`
	wfPath := filepath.Join(tmp, "wf.yaml")
	if err := os.WriteFile(wfPath, []byte(wfYAML), 0o600); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	wf, err := ParseWorkflow(wfPath)
	if err != nil {
		t.Fatalf("parse workflow: %v", err)
	}
	reg := nodes.GetGlobalRegistry()

	exec := NewExecutor().WithWAL(filepath.Join(tmp, "work.wal")).WithWorkflowPath(wfPath)
	_, _, err = exec.Execute(context.Background(), wf, reg)
	var paused *ErrWorkflowPaused
	if !errors.As(err, &paused) {
		t.Fatalf("run1: expected ErrWorkflowPaused, got: %v", err)
	}

	auditPath := history.GetAuditLogPath()
	// The pausing run had no audit, so the log may not exist yet: treat a
	// missing file as zero records.
	before := 0
	if raw, rerr := os.ReadFile(auditPath); rerr == nil {
		for _, l := range strings.Split(string(raw), "\n") {
			if strings.TrimSpace(l) != "" {
				before++
			}
		}
	} else if !os.IsNotExist(rerr) {
		t.Fatalf("read audit log: %v", rerr)
	}

	t.Setenv("RESUME_AUDIT_APPROVED", "1")
	if _, _, err := ResumeWorkflow(context.Background(), paused.RunID); err != nil {
		t.Fatalf("resume: %v", err)
	}

	lines := readAuditFileLines(t, auditPath)
	if len(lines) <= before {
		t.Fatalf("resume wrote no audit records: before=%d after=%d", before, len(lines))
	}
	var sawStart, sawEnd bool
	for _, l := range lines {
		var entry history.AuditLog
		if err := json.Unmarshal([]byte(l), &entry); err != nil {
			t.Fatalf("parse audit record: %v", err)
		}
		if entry.Action == "workflow_start" {
			sawStart = true
		}
		if entry.Action == "workflow_end" {
			sawEnd = true
		}
	}
	if !sawStart || !sawEnd {
		t.Errorf("resume audit records incomplete: workflow_start=%v workflow_end=%v", sawStart, sawEnd)
	}

	// The extended chain must still verify as a whole.
	if valid, brokenAt, verr := history.VerifyAuditChain(auditPath); verr != nil || !valid {
		t.Errorf("audit chain broken after resume: valid=%v brokenAt=%d err=%v", valid, brokenAt, verr)
	}
}

// TestPauseWorkflow_RecordsSafeMode verifies the pause metadata captures the
// executor's safe-mode flag, so a later resume re-applies the same policy
// class the run started under.
func TestPauseWorkflow_RecordsSafeMode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	wfYAML := `name: safe-pause-test
steps:
  - node: human_in_loop
    name: gate
    resumable: true
    params:
      mode: env
      approval_env: NEVER_SET
`
	wfPath := filepath.Join(tmp, "wf.yaml")
	if err := os.WriteFile(wfPath, []byte(wfYAML), 0o600); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	wf, err := ParseWorkflow(wfPath)
	if err != nil {
		t.Fatalf("parse workflow: %v", err)
	}
	reg := nodes.GetGlobalRegistry()

	exec := NewExecutor().WithWAL(filepath.Join(tmp, "work.wal")).WithWorkflowPath(wfPath).WithSafeMode(true)
	_, _, err = exec.Execute(context.Background(), wf, reg)
	var paused *ErrWorkflowPaused
	if !errors.As(err, &paused) {
		t.Fatalf("expected ErrWorkflowPaused, got: %v", err)
	}

	meta, err := LoadRunMeta(paused.RunID)
	if err != nil {
		t.Fatalf("LoadRunMeta: %v", err)
	}
	if !meta.SafeMode {
		t.Error("pause meta should record safe_mode=true for a --safe run")
	}
}

// TestResumeWorkflow_SafeModeBlocksPolicyViolation verifies the resumed
// workflow is validated under the policy class recorded at pause time: a
// run paused under --safe must refuse to resume a workflow whose file now
// contains a shell step (e.g. edited after the pause), and the run must stay
// paused rather than being marked failed.
func TestResumeWorkflow_SafeModeBlocksPolicyViolation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	wfYAML := `name: edited-after-pause
steps:
  - node: shell
    params:
      command: "echo hacked"
`
	wfPath := filepath.Join(tmp, "wf.yaml")
	if err := os.WriteFile(wfPath, []byte(wfYAML), 0o600); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	runID := "test-safe-mode-policy"
	if err := os.MkdirAll(RunDir(runID), 0o700); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	meta := &RunMeta{
		RunID:        runID,
		WorkflowName: "edited-after-pause",
		WorkflowPath: wfPath,
		Status:       "paused",
		PausedStep:   0,
		StepName:     "gate",
		SafeMode:     true,
	}
	if err := SaveRunMeta(meta); err != nil {
		t.Fatalf("SaveRunMeta: %v", err)
	}

	_, _, err := ResumeWorkflow(context.Background(), runID)
	if err == nil {
		t.Fatal("resume should be blocked by the strict policy, but succeeded")
	}
	if !strings.Contains(err.Error(), "resume blocked by policy") {
		t.Errorf("expected policy-block error, got: %v", err)
	}

	// The run must remain paused so it can be resumed again after the
	// workflow is fixed (not silently marked failed/resumed).
	after, err := LoadRunMeta(runID)
	if err != nil {
		t.Fatalf("LoadRunMeta after blocked resume: %v", err)
	}
	if after.Status != "paused" {
		t.Errorf("run status after blocked resume = %q, want paused", after.Status)
	}
}
