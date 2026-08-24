// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​‌‌‌​​‌​​​‌‌‌​‌‌‌‌​​‌‌​​‌​‌‌​‌‌​‌​‌‌​​​​‌‌​​​​​‌‌​​​​​​​​​​​​​​​​‌​​​​‌‌​​​‌‌‌​‌‌⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/i18n"
	"github.com/alib8b8/aflare/internal/workflow"
)

// isolateResumeData points the aflare data dir (where run metadata lives)
// at a fresh temp dir. Note that meta.DataDir is resolved once per process,
// so if an earlier test already initialized it the env vars are advisory —
// the assertions below are written to hold in both cases.
func isolateResumeData(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AFLARE_DATA", dir)
	t.Setenv("HOME", dir)
	return dir
}

func TestResumeCmd_NoArgs(t *testing.T) {
	i18n.Init("en")
	var err error
	out := captureOutput(func() {
		err = HandleResume(nil)
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for no args, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Usage: aflare resume") {
		t.Errorf("expected usage output, got: %s", out)
	}
}

func TestResumeCmd_Help(t *testing.T) {
	i18n.Init("en")
	for _, arg := range []string{"help", "--help", "-h"} {
		var err error
		out := captureOutput(func() {
			err = HandleResume([]string{arg})
		})
		if code := exitCodeForErr(err); code != 0 {
			t.Errorf("%s: expected exit code 0, got %d (err=%v)", arg, code, err)
		}
		if !strings.Contains(out, "Usage: aflare resume") {
			t.Errorf("%s: expected usage output, got: %s", arg, out)
		}
	}
}

// TestResumeCmd_ListEmpty covers `aflare resume list` against an empty (or
// nonexistent) runs directory: it must succeed and report no paused
// workflows. If the process-wide data dir was already pinned to a real home
// with paused runs, the listing path is exercised instead — either way the
// command must not error.
func TestResumeCmd_ListEmpty(t *testing.T) {
	isolateResumeData(t)

	var err error
	out := captureOutput(func() {
		err = HandleResume([]string{"list"})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for resume list, got %d (err=%v)", code, err)
	}
	if !strings.Contains(strings.ToLower(out), "paused") {
		t.Errorf("expected paused-workflows output, got: %s", out)
	}
}

// TestResumeCmd_UnknownRunID covers the default branch: an argument that is
// not a known subcommand is treated as a run-id, and a nonexistent run-id
// fails cleanly with exit code 1.
func TestResumeCmd_UnknownRunID(t *testing.T) {
	isolateResumeData(t)

	var err error
	_ = captureOutput(func() {
		err = HandleResume([]string{"abc"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for unknown run-id, got %d (err=%v)", code, err)
	}
}

// TestResumeCmd_RunIDValidation covers the path-traversal guard in
// HandleResumeRun: run-ids containing separators or ".." are rejected
// before any filesystem access.
func TestResumeCmd_RunIDValidation(t *testing.T) {
	for _, runID := range []string{"../etc", "a/b", `a\b`, "run..id"} {
		var err error
		_ = captureOutput(func() {
			err = HandleResume([]string{runID})
		})
		if code := exitCodeForErr(err); code != 1 {
			t.Errorf("run-id %q: expected exit code 1, got %d (err=%v)", runID, code, err)
		}
	}
}

// resetRunsDir isolates the paused-runs store for one test.
//
// workflow.RunsDir resolves through meta.DataDir, which caches its result
// via sync.Once for the lifetime of the whole test binary. Setting
// AFLARE_DATA therefore only takes effect if this test is the first one to
// trigger that cache; once pinned, every test shares the same path. Either
// way the effective runs dir is cleared so fixtures start from a known
// state (the same pattern setupSchedulesStore uses for the schedules
// store, which lives in a different subdirectory).
func resetRunsDir(t *testing.T) string {
	t.Helper()

	dataDir := t.TempDir()
	t.Setenv("AFLARE_DATA", dataDir)
	t.Setenv("HOME", dataDir)

	runsDir := workflow.RunsDir()
	if err := os.RemoveAll(runsDir); err != nil {
		t.Fatalf("clear runs dir %s: %v", runsDir, err)
	}
	if err := os.MkdirAll(runsDir, 0o750); err != nil {
		t.Fatalf("recreate runs dir %s: %v", runsDir, err)
	}
	return runsDir
}

// writeRunMetaFixture persists a run metadata entry under the runs dir.
func writeRunMetaFixture(t *testing.T, m *workflow.RunMeta) {
	t.Helper()
	if err := workflow.SaveRunMeta(m); err != nil {
		t.Fatalf("saving run meta for %s: %v", m.RunID, err)
	}
}

// TestResumeCmd_NotPaused covers the status guard in HandleResumeRun: a run
// whose metadata exists but is no longer in the "paused" state is rejected
// with exit code 1 instead of being resumed.
func TestResumeCmd_NotPaused(t *testing.T) {
	i18n.Init("en")
	resetRunsDir(t)

	writeRunMetaFixture(t, &workflow.RunMeta{
		RunID:        "finished-run",
		WorkflowName: "demo",
		Status:       "completed",
		StepName:     "save",
		PausedAt:     time.Now(),
	})

	var err error
	_ = captureOutput(func() {
		err = HandleResume([]string{"finished-run"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for non-paused run, got %d (err=%v)", code, err)
	}
}

// TestResumeCmd_ListShowsPausedRun covers the populated branch of
// HandleResumeList: an existing paused run is listed with its run-id and
// workflow name.
func TestResumeCmd_ListShowsPausedRun(t *testing.T) {
	i18n.Init("en")
	resetRunsDir(t)

	writeRunMetaFixture(t, &workflow.RunMeta{
		RunID:        "paused-run",
		WorkflowName: "demo-workflow",
		Status:       "paused",
		PausedStep:   1,
		StepName:     "approval",
		PausedAt:     time.Now(),
		ResumeOn:     "manual",
	})

	var err error
	out := captureOutput(func() {
		err = HandleResume([]string{"list"})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for resume list, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Paused workflows (1)") {
		t.Errorf("expected paused-workflows header, got: %s", out)
	}
	if !strings.Contains(out, "paused-run") || !strings.Contains(out, "demo-workflow") {
		t.Errorf("expected run details in listing, got: %s", out)
	}
}

// TestResumeCmd_ResumeUnparseableWorkflow covers the resume failure branch:
// a paused run whose workflow can neither be read from the recorded path nor
// parsed from the copy saved at pause time fails with exit code 1.
func TestResumeCmd_ResumeUnparseableWorkflow(t *testing.T) {
	i18n.Init("en")
	runsDir := resetRunsDir(t)

	writeRunMetaFixture(t, &workflow.RunMeta{
		RunID:        "paused-broken",
		WorkflowName: "broken",
		WorkflowPath: filepath.Join(runsDir, "missing-workflow.yaml"),
		Status:       "paused",
		StepName:     "save",
		PausedAt:     time.Now(),
	})
	if err := os.WriteFile(workflow.WorkflowPath("paused-broken"), []byte("[unclosed flow"), 0o600); err != nil {
		t.Fatalf("writing broken workflow copy: %v", err)
	}

	var err error
	_ = captureOutput(func() {
		err = HandleResume([]string{"paused-broken"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for unresumable run, got %d (err=%v)", code, err)
	}
}

// TestResumeCmd_ResumeSuccess covers the happy path of HandleResumeRun: a
// paused run whose workflow contains only the non-LLM template_render node
// is resumed, runs to completion and reports success. The fixture has no
// WAL, so the executor starts from the first step — no network, provider
// or TTY is involved.
func TestResumeCmd_ResumeSuccess(t *testing.T) {
	i18n.Init("en")
	resetRunsDir(t)

	dir := t.TempDir()
	wfPath := filepath.Join(dir, "wf.yaml")
	wfYAML := `name: cli-resume-demo
steps:
  - node: template_render
    name: greet
    params:
      template: "hello-from-resume"
`
	if err := os.WriteFile(wfPath, []byte(wfYAML), 0o600); err != nil {
		t.Fatalf("writing workflow: %v", err)
	}

	writeRunMetaFixture(t, &workflow.RunMeta{
		RunID:        "paused-ok",
		WorkflowName: "cli-resume-demo",
		WorkflowPath: wfPath,
		Status:       "paused",
		PausedStep:   0,
		StepName:     "greet",
		PausedAt:     time.Now(),
	})

	var err error
	out := captureOutput(func() {
		err = HandleResume([]string{"paused-ok"})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for resumable run, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Resuming workflow") {
		t.Errorf("expected resuming output, got: %s", out)
	}
	if !strings.Contains(out, "Workflow resumed and completed successfully.") {
		t.Errorf("expected completion output, got: %s", out)
	}
	if !strings.Contains(out, "hello-from-resume") {
		t.Errorf("expected workflow output in report, got: %s", out)
	}
}

// TestResumeCmd_ListStoreUnreadable covers the load-error branch of
// HandleResumeList (and its propagation through HandleResume): when the
// runs directory cannot be read — here it is replaced by a regular file —
// listing fails with exit code 1. The runs dir is restored on cleanup so
// later tests in the binary are not affected.
func TestResumeCmd_ListStoreUnreadable(t *testing.T) {
	i18n.Init("en")
	runsDir := resetRunsDir(t)
	t.Cleanup(func() {
		_ = os.Remove(runsDir)
		_ = os.MkdirAll(runsDir, 0o750)
	})

	if err := os.Remove(runsDir); err != nil {
		t.Fatalf("removing runs dir: %v", err)
	}
	if err := os.WriteFile(runsDir, []byte("not a dir"), 0o600); err != nil {
		t.Fatalf("placing file at runs dir path: %v", err)
	}

	var err error
	_ = captureOutput(func() {
		err = HandleResume([]string{"list"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for unreadable runs store, got %d (err=%v)", code, err)
	}
}
