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
	return os.WriteFile(MetaPath(meta.RunID), data, 0600)
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
// is restored so execution continues from the paused step.
func ResumeWorkflow(ctx context.Context, runID string) (string, []StepResult, error) {
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
	exec := NewExecutor().WithWAL(WALPath(runID))
	// Use a very long timeout for resumed workflows (7 days)
	exec = exec.WithTimeout(7 * 24 * time.Hour)

	out, results, trace, err := exec.ExecuteWithTrace(ctx, wf, reg, nil)
	if err != nil {
		// If it's paused again, update the meta
		var pausedErr *ErrWorkflowPaused
		if errors.As(err, &pausedErr) && pausedErr != nil {
			_ = UpdateRunMetaStatus(runID, "paused")
		} else {
			_ = UpdateRunMetaStatus(runID, "failed")
		}
		return out, results, err
	}
	_ = UpdateRunMetaStatus(runID, "completed")
	_ = trace // trace is used internally by the executor
	return out, results, nil
}

// PauseWorkflow saves the current WAL state and creates the run metadata,
// returning an ErrWorkflowPaused with the run-id.
//
// If sourceWALPath is non-empty, the WAL file is copied to the run directory
// so that a subsequent resume can replay from the last completed step.
func PauseWorkflow(wfPath string, wf *Workflow, stepIndex int, stepName string, resumeOn string, message string, sourceWALPath string) (*ErrWorkflowPaused, error) {
	runID := newRunID()
	dir := RunDir(runID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create run dir: %w", err)
	}

	// Copy the workflow file for reference
	if wfPath != "" {
		wfData, err := os.ReadFile(wfPath)
		if err == nil {
			_ = os.WriteFile(WorkflowPath(runID), wfData, 0600)
		}
	}

	// Copy the WAL file so the resume can replay from the last checkpoint
	if sourceWALPath != "" {
		walData, err := os.ReadFile(sourceWALPath)
		if err == nil {
			_ = os.WriteFile(WALPath(runID), walData, 0600)
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
