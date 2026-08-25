// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌‌‌‌‌​​‌‌​​‌​‌​​‌​‌​​​​​​‌‌‌​‌‌​‌‌‌‌​​​​​​‌‌​‌​‌​‌​‌‌​​‌‌​‌​‌​​​​​​​​​​​​​​​​​​‌‌‌​​‌​‌‌‌‌​‌​​⁠
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​‌​​‌‌‌​‌‌‌​‌‌​​​​​​​​‌​​​‌‌‌​‌​​​‌‌​​‌‌​‌‌​‌‌‌​​​​​​​​​​​​​​​​​‌‌‌‌‌​‌‌‌​​​‌‌‌⁠
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
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/secrets"
	"github.com/alib8b8/aflare/internal/telemetry"
	"github.com/alib8b8/aflare/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// seqExecState holds the mutable state shared across the sequential workflow
// execution loop. It is created by initExecState and passed to each sub-function
// that handles compound steps, conditions, and regular step execution.
type seqExecState struct {
	timeoutCtx    context.Context
	otelCtx       context.Context
	wf            *Workflow
	reg           *nodes.Registry
	program       *tea.Program
	wfPath        string
	walPath       string
	statePath     string
	engine        *ExpressionEngine
	globalLimiter *ConcurrencyLimiter
	trace         *WorkflowTrace
	results       []StepResult
	data          string
	wal           *WAL
	walState      *WALStateManager // delta-record driver around wal (nil when wal is nil)
	saveCP        func(int)        // saveCheckpointIfEnabled closure
	progressCB    StepProgressFunc // 断点13: CLI 实时进度回调
	safeMode      bool             // recorded into pause RunMeta so resume re-applies the same policy class
}

// initExecState validates the workflow, sets up tracing, timeouts, the
// expression engine, secrets, concurrency limiter, and TUI.
// progressCB (断点13) is an optional CLI progress callback; nil disables it.
func initExecState(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program, timeout time.Duration, progressCB StepProgressFunc) (*seqExecState, context.CancelFunc, error) {
	if len(wf.Steps) > MaxSteps {
		return nil, nil, fmt.Errorf("workflow has too many steps (%d, max %d)", len(wf.Steps), MaxSteps)
	}
	if err := validateInputSchema(wf); err != nil {
		return nil, nil, fmt.Errorf("input validation failed: %w", err)
	}
	logger.Info("workflow execution started", "name", wf.Name, "steps", len(wf.Steps))

	trace := newTrace(wf.Name, "sequential", time.Now(), len(wf.Steps))

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	otelCtx, wfSpan := telemetry.StartWorkflowSpan(timeoutCtx, wf.Name)

	data := ""
	if v, ok := ctx.Value(ifInputKey).(string); ok {
		data = v
	}
	engine := NewExpressionEngine()
	if wf.Vars != nil {
		for k, v := range wf.Vars {
			engine.SetVariable(k, v)
		}
	}
	engine.SetSecretGetter(func(group, key string) (string, error) {
		sm, err := secrets.GetSecretManager()
		if err != nil {
			return "", err
		}
		return sm.GetSecret(group, key)
	})
	globalLimiter, err := NewConcurrencyLimiter(wf.MaxConcurrency)
	if err != nil {
		trace.finish(time.Now())
		wfSpan.End()
		cancel()
		return nil, nil, err
	}

	if program != nil {
		program.Send(tui.WorkflowStartMsg{Name: wf.Name, Path: "", Steps: len(wf.Steps)})
	}

	state := &seqExecState{
		timeoutCtx:    timeoutCtx,
		otelCtx:       otelCtx,
		wf:            wf,
		reg:           reg,
		program:       program,
		engine:        engine,
		globalLimiter: globalLimiter,
		trace:         trace,
		data:          data,
		progressCB:    progressCB,
	}

	// Deferred cleanup: finish trace, end OTel span, cancel context.
	cleanup := func() {
		trace.finish(time.Now())
		if wfSpan != nil {
			wfSpan.End()
		}
		cancel()
	}

	return state, cleanup, nil
}

// initResumeState sets up WAL and checkpoint resume, and populates the
// saveCheckpointIfEnabled closure. Returns the step index to resume from.
func (s *seqExecState) initResumeState(walPath, statePath string) int {
	s.walPath = walPath
	s.statePath = statePath
	resumeFromStep := 0

	if walPath != "" {
		w, err := NewWAL(walPath, walOptionsFromEnv())
		if err != nil {
			logger.Warn("failed to open WAL for writes, starting fresh", "path", walPath, "error", err)
		} else {
			s.wal = w
		}
	}

	if s.wal != nil {
		if state, err := LoadStateWAL(walPath); err == nil && state != nil {
			s.data = RestoreState(state, s.wf, s.engine)
			resumeFromStep = state.StepIndex + 1
			resumeFromStep = clampStep(resumeFromStep, len(s.wf.Steps))
			logger.Info("Resuming workflow from step (WAL)", "name", s.wf.Name, "step", resumeFromStep, "wal", walPath)
			s.walState = NewWALStateManager(s.wal, s.wf, state)
		} else {
			if err != nil {
				logger.Warn("failed to replay WAL, starting fresh", "path", walPath, "error", err)
			}
			s.walState = NewWALStateManager(s.wal, s.wf, nil)
		}
	} else if statePath != "" {
		if state, err := loadCheckpoint(statePath); err == nil && state != nil {
			s.data = RestoreState(state, s.wf, s.engine)
			resumeFromStep = state.StepIndex + 1
			resumeFromStep = clampStep(resumeFromStep, len(s.wf.Steps))
			logger.Info("Resuming workflow from step", "name", s.wf.Name, "step", resumeFromStep, "checkpoint", statePath)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			logger.Warn("failed to load checkpoint, starting fresh", "path", statePath, "error", err)
		}
	}

	s.saveCP = func(stepIndex int) {
		if s.walState != nil {
			if err := s.walState.Save(stepIndex, s.data, s.engine); err != nil {
				logger.Warn("failed to append WAL, continuing without", "path", walPath, "step", stepIndex, "error", err)
			}
			return
		}
		if statePath == "" {
			return
		}
		state := SaveCurrentState(s.wf, stepIndex, s.data, s.engine)
		if err := saveCheckpoint(statePath, state); err != nil {
			logger.Warn("failed to save checkpoint, continuing without", "path", statePath, "step", stepIndex, "error", err)
		}
	}

	return resumeFromStep
}

// walOptionsFromEnv builds the WAL durability options from environment
// variables (see WALOptions for the level semantics):
//
//	AFLARE_WAL_SYNC_EVERY_WRITE=1|true  fsync every append (most durable)
//	AFLARE_WAL_SYNC_INTERVAL=<duration>  background fsync loop (e.g. 100ms)
//	neither set                          page cache only + fsync on Close
func walOptionsFromEnv() WALOptions {
	var opts WALOptions
	switch strings.ToLower(os.Getenv("AFLARE_WAL_SYNC_EVERY_WRITE")) {
	case "1", "true", "yes":
		opts.SyncEveryWrite = true
	}
	if v := os.Getenv("AFLARE_WAL_SYNC_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			opts.SyncInterval = d
		} else {
			logger.Warn("invalid AFLARE_WAL_SYNC_INTERVAL, ignoring", "value", v)
		}
	}
	return opts
}

// clampStep bounds a step index to [0, totalSteps].
func clampStep(idx, totalSteps int) int {
	if idx < 0 {
		return 0
	}
	if idx > totalSteps {
		return totalSteps
	}
	return idx
}

// executeWorkflowSequential runs the workflow step-by-step in declaration order
// and records per-step telemetry into a WorkflowTrace (Mode="sequential").
//
// When statePath is non-empty, the sequential path enables checkpoint/resume:
//   - Resume: if a checkpoint file already exists at statePath, the engine
//     state (step outputs, variables, flowing data) is restored and execution
//     continues from the step after the one recorded in the checkpoint.
//   - Checkpoint: after each step completes successfully, a new snapshot is
//     written to statePath so a subsequent run can resume from there.
//
// Checkpoint I/O failures are logged but never interrupt the workflow.
// statePath is ignored by the DAG path; checkpoint/resume is sequential-only.
//
// timeout is the overall workflow timeout applied to the derived context.
// Callers that go through an Executor pass e.workflowTimeout; the legacy
// ExecuteWorkflowWithTrace global entry point passes the package-level
// WorkflowTimeout.
func executeWorkflowSequential(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program, statePath string, walPath string, wfPath string, safeMode bool, timeout time.Duration, progressCB StepProgressFunc) (string, []StepResult, *WorkflowTrace, error) {
	state, cleanup, err := initExecState(ctx, wf, reg, program, timeout, progressCB)
	if err != nil {
		return "", nil, nil, err
	}
	defer cleanup()

	state.wfPath = wfPath
	state.safeMode = safeMode
	resumeFromStep := state.initResumeState(walPath, statePath)

	// Deferred WAL close.
	if state.wal != nil {
		defer func() {
			if state.wal != nil {
				if err := state.wal.Close(); err != nil {
					logger.Warn("failed to close WAL", "path", walPath, "error", err)
				}
			}
		}()
	}

	for i, wStep := range wf.Steps {
		if i < resumeFromStep {
			continue
		}
		if IsShuttingDown() {
			logger.Info("shutdown requested, stopping workflow execution", "name", wf.Name, "completed_steps", i)
			break
		}
		stepStart := time.Now()

		// Compound steps: if/loop/map/reduce/parallel/saga.
		if handled, err := state.handleCompoundStep(i, wStep, stepStart); handled {
			if err != nil {
				return "", state.results, state.trace, err
			}
			continue
		}

		// Condition evaluation.
		if handled, err := state.handleStepCondition(i, wStep, stepStart); handled {
			if err != nil {
				return "", state.results, state.trace, err
			}
			continue
		}

		// Regular step execution.
		if err := state.executeRegularStep(i, wStep, stepStart); err != nil {
			var paused *ErrWorkflowPaused
			if errors.As(err, &paused) {
				return "", state.results, state.trace, paused
			}
			return "", state.results, state.trace, err
		}
	}

	if program != nil {
		program.Send(tui.WorkflowEndMsg{Success: true})
	}
	logger.Info("workflow completed", "name", wf.Name, "steps", len(wf.Steps))

	// Evaluate output expression if defined.
	if wf.Output != "" {
		if finalOutput, err := state.engine.Evaluate(wf.Output, state.data); err != nil {
			logger.Warn("failed to evaluate output expression, using last step output", "error", err)
		} else {
			state.data = finalOutput
		}
	}

	return state.data, state.results, state.trace, nil
}
