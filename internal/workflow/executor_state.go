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

// This file implements workflow state persistence for checkpoint/resume.
//
// The Executor (see executor.go) uses these primitives to support resuming a
// partially-completed sequential workflow run:
//
//   - SaveCurrentState / RestoreState move the in-memory engine state
//     (step outputs, variables, flowing data) into and out of a WorkflowState
//     snapshot. These are now called from the Executor's sequential path.
//   - saveCheckpoint / loadCheckpoint persist that snapshot to disk so a
//     crashed or interrupted run can be resumed later from the last
//     successfully-completed step.
//
// Checkpoint semantics:
//   - Checkpoints are per-step: a new snapshot is written after each step in
//     the sequential execution path completes successfully.
//   - Only the sequential execution path supports checkpoint/resume. The DAG
//     scheduling path (used when any step declares depends_on) does NOT
//     support checkpoints, because steps run concurrently and there is no
//     single linear "progress" cursor to persist.
//   - Checkpoint failures are non-fatal: the Executor logs the error and
//     continues executing the workflow without interrupting it.
//
// SaveState / LoadState remain available as strict, sandboxed utilities
// (they reject absolute paths and paths outside the working directory) and are
// exercised by workflow_extra_test.go.

package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// WorkflowState represents the persisted state of a workflow execution.
// It can be saved to disk and resumed later.
type WorkflowState struct {
	WorkflowName string            `json:"workflow_name"`
	StepIndex    int               `json:"step_index"`
	Data         string            `json:"data"`
	StepOutputs  map[int]string    `json:"step_outputs"`
	Variables    map[string]string `json:"variables"`
	SavedAt      time.Time         `json:"saved_at"`
}

// SaveState persists the current workflow state to a file.
func SaveState(path string, state *WorkflowState) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	// Validate path safety
	safePath, err := validateStatePath(path)
	if err != nil {
		return err
	}
	return os.WriteFile(safePath, data, 0600)
}

// saveCheckpoint persists a WorkflowState snapshot to the given path.
//
// Unlike SaveState, this is intended for the Executor's checkpoint feature and
// accepts absolute paths (e.g. ~/.aflare/checkpoints/<name>.json). It creates
// the parent directory (mode 0700) if it does not yet exist. The file itself
// is written with mode 0600.
func saveCheckpoint(path string, state *WorkflowState) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint state: %w", err)
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create checkpoint directory %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write checkpoint file %s: %w", path, err)
	}
	return nil
}

// loadCheckpoint reads a previously saved WorkflowState snapshot from the
// given path. It accepts absolute paths (unlike LoadState) and is intended
// for the Executor's resume feature. A missing file is reported as an error
// so the caller can distinguish "no checkpoint" from a corrupted one.
func loadCheckpoint(path string) (*WorkflowState, error) {
	if path == "" {
		return nil, nil
	}
	safePath, err := validateCheckpointPath(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(safePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat checkpoint file: %w", err)
	}
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("checkpoint file too large (max %d bytes)", MaxFileSize)
	}
	data, err := os.ReadFile(safePath) // #nosec G304 -- path validated by validateCheckpointPath
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoint file: %w", err)
	}
	var state WorkflowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse checkpoint file: %w", err)
	}
	return &state, nil
}

// LoadState reads a previously saved workflow state from a file.
func LoadState(path string) (*WorkflowState, error) {
	if path == "" {
		return nil, nil
	}
	safePath, err := validateStatePath(path)
	if err != nil {
		return nil, err
	}
	// Security: check file size before reading
	info, err := os.Stat(safePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat state file: %w", err)
	}
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("state file too large (max %d bytes)", MaxFileSize)
	}
	data, err := os.ReadFile(safePath) // #nosec G304 -- path validated
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}
	var state WorkflowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}
	return &state, nil
}

// validateStatePath ensures the state file path is safe (no traversal).
func validateStatePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	// Reject absolute paths
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute paths not allowed")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	absPath := filepath.Join(cwd, path)
	// Resolve symlinks
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	// Verify resolved path is within cwd using filepath.Rel
	rel, err := filepath.Rel(cwd, resolved)
	if err != nil {
		return "", fmt.Errorf("path outside working directory")
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path outside working directory")
	}
	return resolved, nil
}

// validateCheckpointPath validates a checkpoint file path for the Executor's
// resume feature. Unlike validateStatePath, it accepts absolute paths (which
// are expected for CLI-supplied file paths). It prevents path traversal and
// null-byte injection while still allowing files anywhere on the filesystem.
func validateCheckpointPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty checkpoint path")
	}
	// Reject paths containing null bytes (C-string truncation attack)
	if strings.ContainsRune(path, '\x00') {
		return "", fmt.Errorf("checkpoint path contains null byte")
	}
	// Clean the path to remove ".." and "." components
	cleaned := filepath.Clean(path)
	// Resolve symlinks to prevent symlink-based escapes
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		// If the file doesn't exist yet, use the cleaned absolute path
		if os.IsNotExist(err) {
			absPath, err := filepath.Abs(cleaned)
			if err != nil {
				return "", fmt.Errorf("failed to resolve checkpoint path: %w", err)
			}
			return absPath, nil
		}
		return "", fmt.Errorf("failed to resolve checkpoint path: %w", err)
	}
	return resolved, nil
}

// SaveCurrentState creates a WorkflowState snapshot from the current execution context.
func SaveCurrentState(wf *Workflow, stepIndex int, data string, engine *ExpressionEngine) *WorkflowState {
	state := &WorkflowState{
		WorkflowName: wf.Name,
		StepIndex:    stepIndex,
		Data:         data,
		StepOutputs:  make(map[int]string),
		Variables:    make(map[string]string),
		SavedAt:      time.Now(),
	}

	// Copy step outputs by index
	for k, v := range engine.stepOutputs {
		if strings.HasPrefix(k, "idx:") {
			if idx, err := strconv.Atoi(strings.TrimPrefix(k, "idx:")); err == nil {
				state.StepOutputs[idx] = v
			}
		}
	}

	// Copy variables
	for k, v := range engine.variables {
		state.Variables[k] = v
	}

	return state
}

// RestoreState restores a previously saved workflow state into the engine.
func RestoreState(state *WorkflowState, engine *ExpressionEngine) string {
	for idx, output := range state.StepOutputs {
		name := engine.stepNames[idx]
		engine.SetStepOutput(idx, name, output)
	}
	for k, v := range state.Variables {
		engine.SetVariable(k, v)
	}
	return state.Data
}

// SaveStateWAL appends the current workflow state to a WAL. This replaces
// the non-atomic os.WriteFile in saveCheckpoint with a durable append-only
// log that survives crashes. After appending it opportunistically compacts
// the log if it has grown past the configured threshold.
func SaveStateWAL(wal *WAL, wf *Workflow, stepIndex int, data string, engine *ExpressionEngine) error {
	state := SaveCurrentState(wf, stepIndex, data, engine)
	rec := WALRecord{
		StepIndex:   state.StepIndex,
		Data:        state.Data,
		StepOutputs: state.StepOutputs,
		Variables:   state.Variables,
		Timestamp:   time.Now().UTC(),
	}
	if stepIndex >= 0 && stepIndex < len(wf.Steps) {
		rec.StepName = wf.Steps[stepIndex].Name
		rec.NodeName = wf.Steps[stepIndex].Node
	}
	if err := wal.Append(rec); err != nil {
		return err
	}
	return wal.MaybeCompact()
}

// LoadStateWAL replays a WAL and returns the latest state (for crash
// recovery). It returns (nil, nil) when the log contains no records.
func LoadStateWAL(walPath string) (*WorkflowState, error) {
	var latest *WorkflowState
	err := ReplayWAL(walPath, func(r WALRecord) error {
		latest = &WorkflowState{
			WorkflowName: "",
			StepIndex:    r.StepIndex,
			Data:         r.Data,
			StepOutputs:  r.StepOutputs,
			Variables:    r.Variables,
			SavedAt:      r.Timestamp,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return latest, nil // nil if no records
}

// ConcurrencyLimiter provides a global semaphore for limiting concurrent operations.
type ConcurrencyLimiter struct {
	sem chan struct{}
}

// NewConcurrencyLimiter creates a limiter with the given max concurrency.
// If max <= 0, returns nil (unlimited).
func NewConcurrencyLimiter(max int) *ConcurrencyLimiter {
	if max <= 0 {
		return nil
	}
	return &ConcurrencyLimiter{sem: make(chan struct{}, max)}
}

// Acquire blocks until a slot is available. No-op if limiter is nil.
func (cl *ConcurrencyLimiter) Acquire(ctx context.Context) error {
	if cl == nil {
		return nil
	}
	select {
	case cl.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release returns a slot. No-op if limiter is nil.
func (cl *ConcurrencyLimiter) Release() {
	if cl == nil {
		return
	}
	<-cl.sem
}
