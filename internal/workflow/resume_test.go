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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
