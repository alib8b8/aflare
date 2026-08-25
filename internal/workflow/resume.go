// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​​​‌‌​‌​​​‌‌​‌​‌​​​​‌‌‌‌​‌​‌​‌​‌​‌‌​‌​​​​​‌‌​​​​​​​​​​​​​​​​​​​​‌​‌‌​​​​‌‌‌‌​‌⁠
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

// This file implements the pause-resume mechanism for long-running workflows.
// When a step is marked `resumable: true` and fails (e.g. a human_in_loop
// node waiting for approval), the workflow is paused rather than failed:
//   - The WAL is saved at the last completed step.
//   - A run-id is generated and stored in ~/.aflare/runs/<run-id>/.
//   - The workflow can be resumed later via `aflare resume <run-id>` or
//     a webhook POST to /api/v1/workflows/resume.

package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/meta"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/policy"
)

// ErrWorkflowPaused is returned when a workflow is paused at a resumable step
// rather than failed. The RunID field identifies the paused run so it can be
// resumed later.
type ErrWorkflowPaused struct {
	RunID        string
	StepIndex    int
	StepName     string
	WorkflowName string
	Message      string
}

func (e *ErrWorkflowPaused) Error() string {
	return fmt.Sprintf("workflow paused at step %d (%s): %s (run-id: %s)", e.StepIndex+1, e.StepName, e.Message, e.RunID)
}

// RunMeta stores metadata about a paused workflow run.
type RunMeta struct {
	RunID        string    `json:"run_id"`
	WorkflowName string    `json:"workflow_name"`
	WorkflowPath string    `json:"workflow_path"`
	Status       string    `json:"status"` // "paused", "resumed", "completed", "failed"
	PausedAt     time.Time `json:"paused_at"`
	PausedStep   int       `json:"paused_step"`
	StepName     string    `json:"step_name"`
	ResumeOn     string    `json:"resume_on,omitempty"`
	WebhookToken string    `json:"webhook_token,omitempty"`
	// SafeMode records whether the run started under the strict (safe)
	// policy, so the resume re-applies the same policy class. Absent
	// (false) for runs paused by older versions.
	SafeMode bool `json:"safe_mode,omitempty"`
}

// RunsDir returns the directory for storing paused run metadata.
func RunsDir() string {
	return filepath.Join(meta.DataDir(), "runs")
}

// RunDir returns the directory for a specific run.
func RunDir(runID string) string {
	return filepath.Join(RunsDir(), runID)
}

// WALPath returns the WAL file path for a run.
func WALPath(runID string) string {
	return filepath.Join(RunDir(runID), "wal.log")
}

// MetaPath returns the metadata file path for a run.
func MetaPath(runID string) string {
	return filepath.Join(RunDir(runID), "meta.json")
}

// WorkflowPath returns the workflow file copy path for a run.
func WorkflowPath(runID string) string {
	return filepath.Join(RunDir(runID), "workflow.yaml")
}

// SaveRunMeta saves the run metadata to disk.
func SaveRunMeta(meta *RunMeta) error {
	dir := RunDir(meta.RunID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create run dir: %w", err)
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal run meta: %w", err)
	}
	// Atomic write: write to temp file first, then rename to avoid
	// corrupting the meta file on partial write (e.g. disk full, crash).
	target := MetaPath(meta.RunID)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("failed to write run meta: %w", err)
	}
	return os.Rename(tmp, target)
}

// LoadRunMeta loads the run metadata from disk.
func LoadRunMeta(runID string) (*RunMeta, error) {
	path := MetaPath(runID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read run meta: %w", err)
	}
	var meta RunMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse run meta: %w", err)
	}
	return &meta, nil
}

// UpdateRunMetaStatus updates the status field of a run's metadata.
func UpdateRunMetaStatus(runID, status string) error {
	meta, err := LoadRunMeta(runID)
	if err != nil {
		return err
	}
	meta.Status = status
	return SaveRunMeta(meta)
}

// ListPausedRuns returns all runs with status "paused".
func ListPausedRuns() ([]RunMeta, error) {
	dir := RunsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var runs []RunMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		meta, err := LoadRunMeta(entry.Name())
		if err != nil {
			continue
		}
		if meta.Status == "paused" {
			runs = append(runs, *meta)
		}
	}
	return runs, nil
}

// ResumeWorkflow resumes a paused workflow from the WAL and runs it to
// completion. The original workflow file is re-parsed, and the WAL state
// is restored so execution continues from the paused step. Audit logging
// is enabled (the default history directory is used); use
// ResumeWorkflowWith to supply a caller-configured executor (e.g. one
// holding the CLI's cross-process audit lock).
func ResumeWorkflow(ctx context.Context, runID string) (string, []StepResult, error) {
	return ResumeWorkflowWith(ctx, runID, nil)
}

// ResumeWorkflowWith resumes a paused workflow using the caller-supplied
// executor. When base is nil a default executor with audit logging enabled
// is built; when non-nil only the resume specifics (WAL path, 7-day timeout,
// workflow path, safe-mode metadata) are applied on top, so the caller keeps
// control of the audit configuration (a base executor with audit disabled
// stays disabled — see the CLI's audit-lock fallback).
//
// Regardless of the executor, the resumed workflow is validated against the
// policy class the run started under (RunMeta.SafeMode, recorded at pause
// time; legacy metas default to the permissive DefaultPolicy). This keeps
// resume on par with `aflare run` and blocks resuming a workflow whose file
// was edited to add policy-restricted steps (e.g. shell) after the pause.
func ResumeWorkflowWith(ctx context.Context, runID string, base *Executor) (string, []StepResult, error) {
	meta, err := LoadRunMeta(runID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load run %q: %w", runID, err)
	}
	if meta.Status != "paused" {
		return "", nil, fmt.Errorf("run %q is not paused (status: %s)", runID, meta.Status)
	}

	// Mark as resumed
	if err := UpdateRunMetaStatus(runID, "resumed"); err != nil {
		logger.Warn("failed to update run status", "run_id", runID, "error", err)
	}

	// Parse the workflow
	wf, err := ParseWorkflow(meta.WorkflowPath)
	if err != nil {
		// Try the copy we saved
		wf, err = ParseWorkflow(WorkflowPath(runID))
		if err != nil {
			return "", nil, fmt.Errorf("failed to parse workflow for run %q: %w", runID, err)
		}
	}

	reg := nodes.GetGlobalRegistry()

	// Build an executor with WAL-based resume
	var exec *Executor
	if base != nil {
		exec = base
	} else {
		exec = NewExecutor().WithAuditLog(true, "")
	}
	exec = exec.WithWAL(WALPath(runID))
	// Use a very long timeout for resumed workflows (7 days)
	exec = exec.WithTimeout(7 * 24 * time.Hour)
	// Record the workflow path so a second pause during this resume still
	// copies the workflow into the new run dir. Prefer the recorded source
	// path; fall back to the copy saved at pause time for runs whose meta
	// lacks it.
	resumeWfPath := meta.WorkflowPath
	if resumeWfPath == "" {
		if _, err := os.Stat(WorkflowPath(runID)); err == nil {
			resumeWfPath = WorkflowPath(runID)
		}
	}
	if resumeWfPath != "" {
		exec = exec.WithWorkflowPath(resumeWfPath)
	}
	// Keep the policy context stamped for a second pause during this resume.
	exec = exec.WithSafeMode(meta.SafeMode)

	// Policy parity with the run path: validate every step under the policy
	// class this run started under BEFORE executing anything. This blocks
	// resuming a workflow whose file was edited after the pause to add
	// policy-restricted steps (e.g. shell under --safe).
	var engine *policy.Engine
	if meta.SafeMode {
		engine = policy.NewEngine(policy.StrictPolicy(), nil)
	} else {
		engine = policy.NewEngine(policy.DefaultPolicy(), nil)
	}
	if perr := NewPolicyExecutor(exec, engine).ValidateWorkflow(ctx, wf); perr != nil {
		if uerr := UpdateRunMetaStatus(runID, "paused"); uerr != nil {
			logger.Warn("failed to restore run status to paused", "run_id", runID, "err", uerr)
		}
		return "", nil, fmt.Errorf("resume blocked by policy: %w", perr)
	}

	out, results, trace, err := exec.ExecuteWithTrace(ctx, wf, reg, nil)
	if err != nil {
		// If it's paused again, update the meta
		var pausedErr *ErrWorkflowPaused
		if errors.As(err, &pausedErr) && pausedErr != nil {
			if uerr := UpdateRunMetaStatus(runID, "paused"); uerr != nil {
				logger.Warn("failed to update run meta status to paused", "run_id", runID, "err", uerr)
			}
		} else {
			if uerr := UpdateRunMetaStatus(runID, "failed"); uerr != nil {
				logger.Warn("failed to update run meta status to failed", "run_id", runID, "err", uerr)
			}
		}
		return out, results, err
	}
	if err := UpdateRunMetaStatus(runID, "completed"); err != nil {
		logger.Warn("failed to update run meta status to completed", "run_id", runID, "err", err)
	}
	_ = trace // trace is used internally by the executor
	return out, results, nil
}

// PauseWorkflow saves the current WAL state and creates the run metadata,
// returning an ErrWorkflowPaused with the run-id.
//
// If sourceWALPath is non-empty, the WAL file is copied to the run directory
// so that a subsequent resume can replay from the last completed step.
func PauseWorkflow(wfPath string, wf *Workflow, stepIndex int, stepName string, resumeOn string, message string, sourceWALPath string, safeMode bool) (*ErrWorkflowPaused, error) {
	runID := newRunID()
	dir := RunDir(runID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create run dir: %w", err)
	}

	// Copy the workflow file for reference (atomic write).
	if wfPath != "" {
		wfData, err := os.ReadFile(wfPath)
		if err == nil {
			wfDest := WorkflowPath(runID)
			wfTmp := wfDest + ".tmp"
			if writeErr := os.WriteFile(wfTmp, wfData, 0600); writeErr == nil {
				_ = os.Rename(wfTmp, wfDest) // best-effort: atomic workflow copy
			} else {
				_ = os.Remove(wfTmp) // best-effort cleanup
			}
		}
	}

	// Copy the WAL file so the resume can replay from the last checkpoint
	// (atomic write to prevent corruption on partial copy).
	if sourceWALPath != "" {
		walData, err := os.ReadFile(sourceWALPath)
		if err == nil {
			walDest := WALPath(runID)
			walTmp := walDest + ".tmp"
			if writeErr := os.WriteFile(walTmp, walData, 0600); writeErr == nil {
				_ = os.Rename(walTmp, walDest) // best-effort: atomic WAL copy
			} else {
				_ = os.Remove(walTmp) // best-effort cleanup
			}
		}
	}

	// Save run metadata
	meta := &RunMeta{
		RunID:        runID,
		WorkflowName: wf.Name,
		WorkflowPath: wfPath,
		Status:       "paused",
		PausedAt:     time.Now(),
		PausedStep:   stepIndex,
		StepName:     stepName,
		ResumeOn:     resumeOn,
		SafeMode:     safeMode,
	}
	if err := SaveRunMeta(meta); err != nil {
		return nil, fmt.Errorf("failed to save run meta: %w", err)
	}

	return &ErrWorkflowPaused{
		RunID:        runID,
		StepIndex:    stepIndex,
		StepName:     stepName,
		WorkflowName: wf.Name,
		Message:      message,
	}, nil
}
