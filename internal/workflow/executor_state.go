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
