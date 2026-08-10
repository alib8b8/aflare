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
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/metrics"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/secrets"
	"github.com/alib8b8/aflare/internal/telemetry"
	"github.com/alib8b8/aflare/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"
)

// Security limits
const (
	MaxSteps               = 1000             // Maximum number of steps in a workflow
	MaxParallel            = 50               // Maximum parallel steps in a single step
	MaxRetry               = 10               // Maximum retry count per step
	MaxFileSize            = 10 * 1024 * 1024 // 10MB max workflow file size
	MaxStepTimeout         = 30 * time.Minute // Maximum per-step timeout
	MaxRetryDelay          = 5 * time.Minute  // Maximum retry delay
	DefaultWorkflowTimeout = 5 * time.Minute  // Default overall workflow timeout
	MaxIfDepth             = 20               // Maximum nested if/else depth
)

// ifDepthKey propagates the if/else nesting depth through context.
type ifDepthKeyType struct{}

var ifDepthKey = ifDepthKeyType{}

// ifInputKey carries the initial input a branch sub-workflow should start
// from. It is set by executeIfBranch so the chosen then/else branch receives
// the same data the if-step did (instead of starting from an empty string),
// which lets capture_error route on the error text and lets normal if-steps
// keep processing the flowing data. Top-level workflow executions never set
// it, so they default to an empty initial input.
type ifInputKeyType struct{}

var ifInputKey = ifInputKeyType{}

// StepResult stores the result of executing a single step
type StepResult struct {
	StepIndex int
	NodeName  string
	Input     string
	Output    string
	Error     error
	Duration  time.Duration
	// Trace holds detailed per-step telemetry (eval/exec timings, retries,
	// recoveries, DAG batch/dependencies). It is nil when tracing is not
	// requested via ExecuteWorkflowWithTrace.
	Trace *StepTrace
}

func init() {
	nodes.ExecuteWorkflowFunc = func(ctx context.Context, wf interface{}, reg *nodes.Registry) (string, []interface{}, error) {
		var workflow *Workflow
		var err error

		switch v := wf.(type) {
		case *Workflow:
			workflow = v
		case string:
			if len(v) > MaxFileSize {
				return "", nil, fmt.Errorf("workflow content too large (max %d bytes)", MaxFileSize)
			}
			if err := yaml.Unmarshal([]byte(v), &workflow); err != nil {
				return "", nil, fmt.Errorf("failed to parse workflow YAML: %w", err)
			}
		default:
			return "", nil, fmt.Errorf("unsupported workflow type")
		}

		result, stepResults, err := ExecuteWorkflow(ctx, workflow, reg)
		if err != nil {
			return "", nil, err
		}

		results := make([]interface{}, len(stepResults))
		for i, sr := range stepResults {
			results[i] = sr
		}
		return result, results, nil
	}
}

// ExecuteWorkflow executes a workflow step by step
func ExecuteWorkflow(ctx context.Context, wf *Workflow, reg *nodes.Registry) (string, []StepResult, error) {
	return ExecuteWorkflowWithTUI(ctx, wf, reg, nil)
}

// ExecuteWorkflowWithTUI executes the workflow and sends messages to a TUI program.
// It is a thin wrapper around ExecuteWorkflowWithTrace that discards the trace.
func ExecuteWorkflowWithTUI(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program) (string, []StepResult, error) {
	output, results, _, err := ExecuteWorkflowWithTrace(ctx, wf, reg, program)
	return output, results, err
}

// ExecuteWorkflowWithTrace executes the workflow and returns a detailed per-step
// WorkflowTrace alongside the standard results.
//
// Routing: when any step declares depends_on, the DAG scheduling path is used
// (topological batching + concurrent execution); otherwise the legacy sequential
// for-loop path runs. Both paths populate the trace with the same StepTrace
// schema — BatchIndex is -1 and Dependencies is nil in sequential mode.
//
// This entry point does not use checkpoint/resume. To enable per-step
// checkpointing and resume, build an Executor via NewExecutor().WithCheckpoint(...).
//
// This legacy entry point applies DefaultWorkflowTimeout. To configure a
// different timeout, build an Executor via NewExecutor().WithTimeout(d)
// (per-executor, no global mutable state).
func ExecuteWorkflowWithTrace(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program) (string, []StepResult, *WorkflowTrace, error) {
	if hasDAGDeclarations(wf.Steps) {
		out, results, trace, err := executeWorkflowDAG(ctx, wf, reg, program, DefaultWorkflowTimeout)
		recordWorkflowMetrics(trace, err)
		return out, results, trace, err
	}
	out, results, trace, err := executeWorkflowSequential(ctx, wf, reg, program, "", "", "", DefaultWorkflowTimeout)
	recordWorkflowMetrics(trace, err)
	return out, results, trace, err
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
func executeWorkflowSequential(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program, statePath string, walPath string, wfPath string, timeout time.Duration) (string, []StepResult, *WorkflowTrace, error) {
	// Validate step count
	if len(wf.Steps) > MaxSteps {
		return "", nil, nil, fmt.Errorf("workflow has too many steps (%d, max %d)", len(wf.Steps), MaxSteps)
	}

	// Validate input schema if defined
	if err := validateInputSchema(wf); err != nil {
		return "", nil, nil, fmt.Errorf("input validation failed: %w", err)
	}

	logger.Info("workflow execution started", "name", wf.Name, "steps", len(wf.Steps))

	trace := newTrace(wf.Name, "sequential", time.Now(), len(wf.Steps))
	defer func() { trace.finish(time.Now()) }()

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// OpenTelemetry: root workflow span. Step spans are children of this span.
	otelCtx, wfSpan := telemetry.StartWorkflowSpan(timeoutCtx, wf.Name)
	defer func() {
		if wfSpan != nil {
			wfSpan.End()
		}
	}()

	var results []StepResult
	data := ""
	// A branch sub-workflow (then/else of an if-step) inherits the if-step's
	// input as its starting data so the flowing value isn't lost. Top-level
	// executions leave data as "".
	if v, ok := ctx.Value(ifInputKey).(string); ok {
		data = v
	}
	engine := NewExpressionEngine()

	// Load workflow-level vars into expression engine
	if wf.Vars != nil {
		for k, v := range wf.Vars {
			engine.SetVariable(k, v)
		}
	}

	// Set up secrets access
	engine.SetSecretGetter(func(group, key string) (string, error) {
		sm, err := secrets.GetSecretManager()
		if err != nil {
			return "", err
		}
		return sm.GetSecret(group, key)
	})

	// Create global concurrency limiter
	globalLimiter := NewConcurrencyLimiter(wf.MaxConcurrency)

	if program != nil {
		program.Send(tui.WorkflowStartMsg{
			Name:  wf.Name,
			Path:  "",
			Steps: len(wf.Steps),
		})
	}

	// ── Resume support ──
	// If a checkpoint file exists at statePath (or a WAL at walPath), restore
	// the engine state (step outputs, variables, flowing data) and continue
	// execution from the step after the one recorded in the checkpoint.
	resumeFromStep := 0

	// Open WAL for appends if configured. The WAL is kept open for the
	// lifetime of this execution and closed via defer.
	var wal *WAL
	if walPath != "" {
		w, err := NewWAL(walPath, WALOptions{})
		if err != nil {
			logger.Warn("failed to open WAL for writes, starting fresh", "path", walPath, "error", err)
		} else {
			wal = w
			defer func() {
				if err := wal.Close(); err != nil {
					logger.Warn("failed to close WAL", "path", walPath, "error", err)
				}
			}()
		}
	}

	if wal != nil {
		// Resume from WAL: replay yields the latest restorable state.
		if state, err := LoadStateWAL(walPath); err == nil && state != nil {
			data = RestoreState(state, engine)
			resumeFromStep = state.StepIndex + 1
			if resumeFromStep < 0 {
				resumeFromStep = 0
			}
			if resumeFromStep > len(wf.Steps) {
				resumeFromStep = len(wf.Steps)
			}
			logger.Info("Resuming workflow from step (WAL)", "name", wf.Name, "step", resumeFromStep, "wal", walPath)
		} else if err != nil {
			logger.Warn("failed to replay WAL, starting fresh", "path", walPath, "error", err)
		}
	} else if statePath != "" {
		if state, err := loadCheckpoint(statePath); err == nil && state != nil {
			data = RestoreState(state, engine)
			// state.StepIndex is the last successfully-completed step; resume
			// from the step after it. Clamp to a valid range.
			resumeFromStep = state.StepIndex + 1
			if resumeFromStep < 0 {
				resumeFromStep = 0
			}
			if resumeFromStep > len(wf.Steps) {
				resumeFromStep = len(wf.Steps)
			}
			logger.Info("Resuming workflow from step", "name", wf.Name, "step", resumeFromStep, "checkpoint", statePath)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			// A corrupt/unreadable checkpoint is logged but non-fatal: we
			// start a fresh run rather than blocking the user.
			logger.Warn("failed to load checkpoint, starting fresh", "path", statePath, "error", err)
		}
	}

	// saveCheckpointIfEnabled writes a per-step checkpoint when statePath is
	// set (or appends to the WAL when walPath is set). Failures are logged
	// but never interrupt the workflow.
	saveCheckpointIfEnabled := func(stepIndex int) {
		if wal != nil {
			if err := SaveStateWAL(wal, wf, stepIndex, data, engine); err != nil {
				logger.Warn("failed to append WAL, continuing without", "path", walPath, "step", stepIndex, "error", err)
			}
			return
		}
		if statePath == "" {
			return
		}
		state := SaveCurrentState(wf, stepIndex, data, engine)
		if err := saveCheckpoint(statePath, state); err != nil {
			logger.Warn("failed to save checkpoint, continuing without", "path", statePath, "step", stepIndex, "error", err)
		}
	}

	for i, wStep := range wf.Steps {
		// Skip steps already completed in a prior (checkpointed) run.
		if i < resumeFromStep {
			continue
		}

		// Graceful shutdown: if a shutdown has been requested, stop starting
		// new steps. The current step (if any) has already completed by the
		// time we reach this check on the next iteration. Deferred cleanup
		// (WAL close → flush, audit finalization) runs when the function
		// returns.
		if IsShuttingDown() {
			logger.Info("shutdown requested, stopping workflow execution", "name", wf.Name, "completed_steps", i)
			break
		}
		stepStart := time.Now()

		// ── Handle if/else branch ──
		if wStep.IsIf() {
			_, compSpan := telemetry.StartCompoundStepSpan(otelCtx, wStep.Name, "if", i)
			branchResults, output, err := executeIfBranch(timeoutCtx, i, wStep.If, data, engine, reg, program, globalLimiter)
			compDur := time.Since(stepStart)
			telemetry.StepSpanEnd(compSpan, err, compDur.Milliseconds(), len(output), false)
			if err != nil {
				results = append(results, branchResults...)
				if program != nil {
					program.Send(tui.WorkflowEndMsg{Success: false})
				}
				return "", results, trace, err
			}
			results = append(results, branchResults...)
			data = output
			engine.SetStepOutput(i, wStep.Name, output)
			// Compound steps record a coarse-grained trace entry.
			trace.recordStep(StepTrace{
				Index:           i,
				NodeName:        wStep.Node,
				StepName:        wStep.Name,
				BatchIndex:      -1,
				ConditionPassed: true,
				Attempts:        1,
				TotalDuration:   compDur,
				InputLen:        len(data),
				OutputLen:       len(output),
			})
			saveCheckpointIfEnabled(i)
			continue
		}

		// Check condition - skip step if condition evaluates to false
		if wStep.Condition != "" {
			evalStart := time.Now()
			pass, err := evaluateCondition(wStep.Condition, data, engine)
			evalDuration := time.Since(evalStart)
			if err != nil {
				logger.Error("condition evaluation failed", "index", i, "error", err)
				result := StepResult{
					StepIndex: i,
					NodeName:  wStep.Node,
					Input:     data,
					Error:     fmt.Errorf("condition evaluation failed: %w", err),
					Duration:  0,
				}
				result.Trace = trace.recordStep(StepTrace{
					Index:           i,
					NodeName:        wStep.Node,
					StepName:        wStep.Name,
					BatchIndex:      -1,
					ConditionExpr:   wStep.Condition,
					ConditionPassed: false,
					EvalDuration:    evalDuration,
					TotalDuration:   time.Since(stepStart),
					InputLen:        len(data),
					ErrorText:       err.Error(),
				})
				results = append(results, result)
				if program != nil {
					program.Send(tui.StepStartMsg{
						Index: i,
						Name:  wStep.Node,
					})
					program.Send(tui.StepEndMsg{
						Index: i,
						Name:  wStep.Node,
						Error: err,
					})
					program.Send(tui.WorkflowEndMsg{Success: false})
				}
				return "", results, trace, err
			}
			if !pass {
				logger.Info("step skipped by condition", "index", i, "node", wStep.Node)
				// Register step output as empty so later step refs still work
				engine.SetStepOutput(i, wStep.Node, "")
				result := StepResult{
					StepIndex: i,
					NodeName:  wStep.Node,
					Input:     data,
					Output:    "",
					Error:     nil,
					Duration:  0,
				}
				result.Trace = trace.recordStep(StepTrace{
					Index:           i,
					NodeName:        wStep.Node,
					StepName:        wStep.Name,
					BatchIndex:      -1,
					Skipped:         true,
					ConditionExpr:   wStep.Condition,
					ConditionPassed: false,
					EvalDuration:    evalDuration,
					TotalDuration:   time.Since(stepStart),
					InputLen:        len(data),
				})
				results = append(results, result)
				if program != nil {
					program.Send(tui.StepStartMsg{
						Index: i,
						Name:  wStep.Node,
					})
					program.Send(tui.StepEndMsg{
						Index:    i,
						Name:     wStep.Node,
						Output:   "",
						Duration: 0,
					})
				}
				continue
			}
		}

		if wStep.IsLoop() {
			_, compSpan := telemetry.StartCompoundStepSpan(otelCtx, wStep.Name, "loop", i)
			loopResults, output, err := executeLoopStep(timeoutCtx, i, wStep, data, engine, reg, program, globalLimiter)
			compDur := time.Since(stepStart)
			telemetry.StepSpanEnd(compSpan, err, compDur.Milliseconds(), len(output), false)
			if err != nil {
				results = append(results, loopResults...)
				if program != nil {
					program.Send(tui.WorkflowEndMsg{Success: false})
				}
				return "", results, trace, err
			}
			results = append(results, loopResults...)
			data = applyOutputStrategy(output, wStep.OutputStrategy)
			engine.SetStepOutput(i, wStep.Name, output)
			trace.recordStep(StepTrace{
				Index:           i,
				NodeName:        wStep.Node,
				StepName:        wStep.Name,
				BatchIndex:      -1,
				ConditionPassed: true,
				Attempts:        1,
				TotalDuration:   compDur,
				InputLen:        len(data),
				OutputLen:       len(output),
			})
			saveCheckpointIfEnabled(i)
			continue
		}

		if wStep.IsMap() {
			_, compSpan := telemetry.StartCompoundStepSpan(otelCtx, wStep.Name, "map", i)
			mapResults, output, err := executeMapStep(timeoutCtx, i, wStep, data, engine, reg, program, globalLimiter)
			compDur := time.Since(stepStart)
			telemetry.StepSpanEnd(compSpan, err, compDur.Milliseconds(), len(output), false)
			if err != nil {
				results = append(results, mapResults...)
				if program != nil {
					program.Send(tui.WorkflowEndMsg{Success: false})
				}
				return "", results, trace, err
			}
			results = append(results, mapResults...)
			data = output
			engine.SetStepOutput(i, wStep.Name, output)
			trace.recordStep(StepTrace{
				Index:           i,
				NodeName:        "map",
				StepName:        wStep.Name,
				BatchIndex:      -1,
				ConditionPassed: true,
				Attempts:        1,
				TotalDuration:   compDur,
				InputLen:        len(data),
				OutputLen:       len(output),
			})
			saveCheckpointIfEnabled(i)
			continue
		}

		if wStep.IsReduce() {
			_, compSpan := telemetry.StartCompoundStepSpan(otelCtx, wStep.Name, "reduce", i)
			reduceResults, output, err := executeReduceStep(timeoutCtx, i, wStep, data, engine, reg, program, globalLimiter)
			compDur := time.Since(stepStart)
			telemetry.StepSpanEnd(compSpan, err, compDur.Milliseconds(), len(output), false)
			if err != nil {
				results = append(results, reduceResults...)
				if program != nil {
					program.Send(tui.WorkflowEndMsg{Success: false})
				}
				return "", results, trace, err
			}
			results = append(results, reduceResults...)
			data = output
			engine.SetStepOutput(i, wStep.Name, output)
			trace.recordStep(StepTrace{
				Index:           i,
				NodeName:        "reduce",
				StepName:        wStep.Name,
				BatchIndex:      -1,
				ConditionPassed: true,
				Attempts:        1,
				TotalDuration:   compDur,
				InputLen:        len(data),
				OutputLen:       len(output),
			})
			saveCheckpointIfEnabled(i)
			continue
		}

		if wStep.IsParallel() {
			_, compSpan := telemetry.StartCompoundStepSpan(otelCtx, wStep.Name, "parallel", i)
			parallelResults, output, err := executeParallelStep(timeoutCtx, i, wStep, data, engine, reg, program, globalLimiter)
			compDur := time.Since(stepStart)
			telemetry.StepSpanEnd(compSpan, err, compDur.Milliseconds(), len(output), false)
			if err != nil {
				results = append(results, parallelResults...)
				if program != nil {
					program.Send(tui.WorkflowEndMsg{Success: false})
				}
				return "", results, trace, err
			}
			results = append(results, parallelResults...)
			data = applyOutputStrategy(output, wStep.OutputStrategy)
			engine.SetStepOutput(i, wStep.Name, output)
			trace.recordStep(StepTrace{
				Index:           i,
				NodeName:        wStep.Node,
				StepName:        wStep.Name,
				BatchIndex:      -1,
				ConditionPassed: true,
				Attempts:        1,
				TotalDuration:   compDur,
				InputLen:        len(data),
				OutputLen:       len(output),
			})
			saveCheckpointIfEnabled(i)
			continue
		}

		if wStep.IsSaga() {
			_, compSpan := telemetry.StartCompoundStepSpan(otelCtx, wStep.Name, "saga", i)
			sagaResults, output, err := executeSagaStep(timeoutCtx, i, wStep, data, engine, reg, program, globalLimiter)
			compDur := time.Since(stepStart)
			telemetry.StepSpanEnd(compSpan, err, compDur.Milliseconds(), len(output), false)
			if err != nil {
				results = append(results, sagaResults...)
				if program != nil {
					program.Send(tui.WorkflowEndMsg{Success: false})
				}
				return "", results, trace, err
			}
			results = append(results, sagaResults...)
			data = output
			engine.SetStepOutput(i, wStep.Name, output)
			trace.recordStep(StepTrace{
				Index:           i,
				NodeName:        "saga",
				StepName:        wStep.Name,
				BatchIndex:      -1,
				ConditionPassed: true,
				Attempts:        1,
				TotalDuration:   compDur,
				InputLen:        len(data),
				OutputLen:       len(output),
			})
			saveCheckpointIfEnabled(i)
			continue
		}

		logger.Info("step started", "index", i, "node", wStep.Node)

		if program != nil {
			program.Send(tui.StepStartMsg{
				Index: i,
				Name:  wStep.Node,
			})
		}

		evalStart := time.Now()
		evaluatedParams, err := engine.EvaluateParams(wStep.Params, data)
		evalDuration := time.Since(evalStart)
		if err != nil {
			logger.Error("expression evaluation failed", "index", i, "error", err)
			result := StepResult{
				StepIndex: i,
				NodeName:  wStep.Node,
				Input:     data,
				Error:     err,
				Duration:  time.Since(stepStart),
			}
			result.Trace = trace.recordStep(StepTrace{
				Index:           i,
				NodeName:        wStep.Node,
				StepName:        wStep.Name,
				BatchIndex:      -1,
				ConditionPassed: true,
				EvalDuration:    evalDuration,
				TotalDuration:   time.Since(stepStart),
				InputLen:        len(data),
				ErrorText:       err.Error(),
			})
			results = append(results, result)
			if program != nil {
				program.Send(tui.StepEndMsg{
					Index:    i,
					Name:     wStep.Node,
					Error:    err,
					Duration: time.Since(stepStart),
				})
				program.Send(tui.WorkflowEndMsg{Success: false})
			}
			return "", results, trace, err
		}

		node, ok := reg.Get(wStep.Node)
		if !ok {
			err := fmt.Errorf("node '%s' not found in registry", wStep.Node)
			logger.Error("node not found", "node", wStep.Node, "error", err)
			result := StepResult{
				StepIndex: i,
				NodeName:  wStep.Node,
				Input:     data,
				Error:     err,
				Duration:  time.Since(stepStart),
			}
			result.Trace = trace.recordStep(StepTrace{
				Index:           i,
				NodeName:        wStep.Node,
				StepName:        wStep.Name,
				BatchIndex:      -1,
				ConditionPassed: true,
				EvalDuration:    evalDuration,
				TotalDuration:   time.Since(stepStart),
				InputLen:        len(data),
				ErrorText:       err.Error(),
			})
			results = append(results, result)

			if program != nil {
				program.Send(tui.StepEndMsg{
					Index:    i,
					Name:     wStep.Node,
					Error:    err,
					Duration: time.Since(stepStart),
				})
				program.Send(tui.WorkflowEndMsg{Success: false})
			}

			return "", results, trace, err
		}

		retryCount := wStep.GetRetryCount()
		if retryCount > MaxRetry {
			retryCount = MaxRetry
		}
		// For resumable steps, use the step-level timeout (e.g. 72h) which
		// may exceed MaxStepTimeout. For non-resumable steps, the standard
		// params._timeout is capped at MaxStepTimeout as before.
		stepTimeout := wStep.GetTimeout()
		if wStep.IsResumable() {
			if rt := wStep.GetResumeTimeout(); rt > 0 {
				stepTimeout = rt
			}
		} else if stepTimeout > MaxStepTimeout {
			stepTimeout = MaxStepTimeout
		}

		// B-2: per-step LLM telemetry collector. stepBaseCtx carries the
		// sink so every retry attempt (and any sub-call) of this step's
		// node publishes to the same collector. Drained into StepTrace
		// after the step finishes.
		stepBaseCtx, llmCollector := withLLMCollector(timeoutCtx)

		var output string
		var execErr error
		var duration time.Duration
		maxAttempts := retryCount + 1
		attemptsMade := 0

		// OpenTelemetry: step span wrapping the node execution (including retries).
		_, stepSpan := telemetry.StartStepSpan(otelCtx, wStep.Name, wStep.Node, i)

		for attempt := 1; attempt <= maxAttempts; attempt++ {
			attemptsMade = attempt
			attemptStart := time.Now()

			var stepCtx context.Context
			var stepCancel context.CancelFunc
			if stepTimeout > 0 {
				stepCtx, stepCancel = context.WithTimeout(stepBaseCtx, stepTimeout)
			} else {
				stepCtx, stepCancel = context.WithCancel(stepBaseCtx)
			}

			if program != nil {
				if streamingNode, ok := node.(nodes.StreamingNode); ok {
					// Decouple the streaming producer (HTTP reader inside
					// ExecuteStream) from the TUI consumer via a buffered
					// channel: program.Send blocks on an unbuffered channel,
					// so a slow TUI would otherwise stall the stream.
					//
					// Wrapped in an IIFE so sink.flush runs via defer at the
					// end of each attempt: if ExecuteStream panics the
					// forwarding goroutine would otherwise leak (it blocks on
					// range s.ch until the channel is closed). onChunk is only
					// invoked synchronously from inside ExecuteStream, so no
					// concurrent senders exist by the time flush runs.
					output, execErr = func() (string, error) {
						sink := newStreamSink(program, i, wStep.Node)
						defer sink.flush()
						return streamingNode.ExecuteStream(stepCtx, data, evaluatedParams, sink.onChunk)
					}()
				} else {
					output, execErr = node.Execute(stepCtx, data, evaluatedParams)
				}
			} else {
				output, execErr = node.Execute(stepCtx, data, evaluatedParams)
			}
			duration = time.Since(attemptStart)

			stepCancel()

			if execErr == nil {
				break
			}

			logger.Warn("step failed, retrying", "index", i, "node", wStep.Node, "attempt", attempt, "max", maxAttempts, "error", nodes.RedactSensitive(execErr.Error()))

			if attempt < maxAttempts {
				// Use backoff delay if configured, otherwise use fixed delay
				retryDelayActual := wStep.GetBackoffDelay(attempt)
				select {
				case <-time.After(retryDelayActual):
				case <-timeoutCtx.Done():
					telemetry.StepSpanEnd(stepSpan, context.DeadlineExceeded, duration.Milliseconds(), len(output), false)
					return "", results, trace, fmt.Errorf("workflow timed out during retry delay")
				}
			}
		}

		// ── Error recovery ──
		// Delegated to applyErrorRecovery (shared with the DAG and map
		// executors) so the four recovery primitives — capture_error,
		// fallback, on_error, continue_on_error — have one implementation.
		// abortErr controls whether the workflow stops (mapped to execErr);
		// traceErr is recorded in StepResult so continue_on_error still
		// honestly reflects the failure in the trace.
		// stepBaseCtx is passed so the capture_error branch and on_error
		// handler run under the same step-scoped context (LLM calls are
		// captured by the step collector, step timeout still applies).
		resultErr := execErr
		var recoveries []string
		if execErr != nil {
			var abortErr error
			recoveries, abortErr, resultErr = applyErrorRecovery(stepBaseCtx, &wStep, &output, execErr, engine, reg, data, program, globalLimiter, "step")
			execErr = abortErr
		}

		engine.SetStepOutput(i, wStep.Name, output)

		errText := ""
		if resultErr != nil {
			errText = resultErr.Error()
		}
		result := StepResult{
			StepIndex: i,
			NodeName:  wStep.Node,
			Input:     data,
			Output:    output,
			Error:     resultErr,
			Duration:  duration,
		}
		result.Trace = trace.recordStep(StepTrace{
			Index:           i,
			NodeName:        wStep.Node,
			StepName:        wStep.Name,
			BatchIndex:      -1,
			ConditionExpr:   wStep.Condition,
			ConditionPassed: true,
			Attempts:        attemptsMade,
			Recoveries:      recoveries,
			EvalDuration:    evalDuration,
			ExecuteDuration: duration,
			TotalDuration:   time.Since(stepStart),
			InputLen:        len(data),
			OutputLen:       len(output),
			ErrorText:       errText,
			LLM:             projectLLMTelemetry(llmCollector.drainCalls()),
			Router:          projectRouterDecisions(llmCollector.drainDecisions()),
		})
		results = append(results, result)

		// End the OTel step span with the final outcome.
		telemetry.StepSpanEnd(stepSpan, resultErr, duration.Milliseconds(), len(output), false)

		if resultErr != nil {
			logger.Error("step failed", "index", i, "node", wStep.Node, "duration", duration, "error", nodes.RedactSensitive(resultErr.Error()))
		} else {
			logger.Info("step completed", "index", i, "node", wStep.Node, "duration", duration)
		}

		if program != nil {
			program.Send(tui.StepEndMsg{
				Index:    i,
				Name:     wStep.Node,
				Output:   output,
				Error:    resultErr,
				Duration: duration,
			})
		}

		if execErr != nil {
			if wStep.IsResumable() {
				// Close the WAL so it's flushed to disk before copying
				if wal != nil {
					_ = wal.Close()
					wal = nil // prevent double-close in defer
				}
				// Save the last checkpoint to the WAL before pausing
				// (the last completed step is i-1, save it now)
				if i > 0 {
					saveCheckpointIfEnabled(i - 1)
				}
				resumeOn := wStep.ResumeOn
				if resumeOn == "" {
					resumeOn = "manual"
				}
				paused, pauseErr := PauseWorkflow(wfPath, wf, i, wStep.Name, resumeOn, execErr.Error(), walPath)
				if pauseErr != nil {
					logger.Error("failed to pause workflow", "name", wf.Name, "step", i, "error", pauseErr)
					if program != nil {
						program.Send(tui.WorkflowEndMsg{Success: false})
					}
					return "", results, trace, fmt.Errorf("step %d (%s) failed: %w", i+1, wStep.Node, execErr)
				}
				logger.Info("workflow paused", "name", wf.Name, "step", i, "node", wStep.Node, "run_id", paused.RunID, "resume_on", resumeOn)
				if program != nil {
					program.Send(tui.WorkflowEndMsg{Success: false})
				}
				return "", results, trace, paused
			}
			if program != nil {
				program.Send(tui.WorkflowEndMsg{Success: false})
			}
			logger.Error("workflow failed", "name", wf.Name, "failed_step", i, "node", wStep.Node, "error", execErr)
			return "", results, trace, fmt.Errorf("step %d (%s) failed: %w", i+1, wStep.Node, execErr)
		}

		data = output
		saveCheckpointIfEnabled(i)
	}

	if program != nil {
		program.Send(tui.WorkflowEndMsg{Success: true})
	}

	logger.Info("workflow completed", "name", wf.Name, "steps", len(wf.Steps))

	// If output expression is defined, evaluate it instead of returning last step output
	if wf.Output != "" {
		finalOutput, err := engine.Evaluate(wf.Output, data)
		if err != nil {
			logger.Warn("failed to evaluate output expression, using last step output", "error", err)
		} else {
			data = finalOutput
		}
	}

	return data, results, trace, nil
}

// Executor is a configurable workflow runner. It wraps the package-level
// ExecuteWorkflow* functions and adds optional checkpoint/resume support.
//
// Build one with NewExecutor and chain WithCheckpoint to enable persistence:
//
//	exec := NewExecutor().WithCheckpoint("~/.aflare/checkpoints/wf.json")
//	out, results, err := exec.Execute(ctx, wf, reg)
//
// When statePath is set and a checkpoint file already exists, execution
// resumes from the step after the one recorded in the checkpoint. After each
// sequential step completes, a fresh snapshot is written to statePath.
//
// Checkpoint/resume is only supported on the sequential execution path.
// Workflows that declare depends_on (DAG mode) ignore statePath.
type Executor struct {
	statePath       string
	walPath         string // when set, use append-only WAL instead of JSON checkpoint
	workflowTimeout time.Duration
	// auditEnabled turns on tamper-evident audit logging of every workflow
	// execution (start, per-step, completion/failure) into the history
	// package's HMAC hash-chain audit log. Off by default.
	auditEnabled bool
	// auditDir, when non-empty, overrides the history/audit directory used
	// for the audit log. When empty the history package default
	// (~/.config/aflare/history) is used.
	auditDir string
	// idempotencyKey, when non-empty, activates workflow idempotency: before
	// executing, the Executor consults idempotencyStore for a prior
	// completed run with this key and returns the cached result on a hit so
	// side-effecting nodes (HTTP POST transfers, file writes, ...) are not
	// re-run. Empty = idempotency off (default, backward-compatible).
	idempotencyKey string
	// idempotencyStore holds the key→run_id ledger. It is auto-instantiated
	// to a FileIdempotencyStore at DefaultIdempotencyDir() by
	// WithIdempotencyKey when nil, and can be overridden via
	// WithIdempotencyStore (mainly for tests).
	idempotencyStore IdempotencyStore
	// wfPath is the original workflow file path, used for pause-resume
	// metadata when a resumable step is paused.
	wfPath string
	// wg tracks in-flight executions so Shutdown can wait for all running
	// steps to complete before returning.
	wg sync.WaitGroup
}

// NewExecutor returns an Executor with no checkpoint configured and the
// workflow timeout initialized to DefaultWorkflowTimeout. Use WithTimeout to
// override the timeout and WithCheckpoint to enable checkpoint/resume.
func NewExecutor() *Executor {
	return &Executor{
		workflowTimeout: DefaultWorkflowTimeout,
	}
}

// WithCheckpoint configures the Executor to persist per-step checkpoints to
// the given path and resume from it on the next run if it exists. Returns the
// receiver for chaining.
func (e *Executor) WithCheckpoint(path string) *Executor {
	e.statePath = path
	return e
}

// WithWAL configures the Executor to use an append-only Write-Ahead Log at
// the given path for durable per-step checkpointing. This is preferred over
// WithCheckpoint for long-running workflows because:
//   - Each step appends a single record (no full-state rewrite).
//   - Records are CRC32-protected against torn tail writes from crashes.
//   - Periodic compaction bounds replay time.
//
// Resume on the next run reads the WAL via LoadStateWAL. When both WithWAL
// and WithCheckpoint are set, WithWAL takes precedence.
func (e *Executor) WithWAL(path string) *Executor {
	e.walPath = path
	return e
}

// WithTimeout configures the overall workflow timeout applied to the derived
// context for this Executor's runs. Returns the receiver for chaining.
//
// Use this instead of mutating the package-level WorkflowTimeout global,
// which is unsafe under parallel tests (t.Parallel) and deprecated.
func (e *Executor) WithTimeout(d time.Duration) *Executor {
	e.workflowTimeout = d
	return e
}

// WithAuditLog enables tamper-evident audit logging of every workflow
// execution into the history package's HMAC hash-chain audit log. For each
// run the recorder writes: a workflow_start record, one workflow_step record
// per completed step (with sanitized params and truncated input/output), and
// a workflow_end (or workflow_failed) record.
//
// When dir is non-empty it overrides the history/audit directory; when empty
// the history package default (~/.config/aflare/history) is used. Audit is
// off by default and must be explicitly enabled.
//
// If AFLARE_AUDIT_HMAC_KEY (or AFLARE_SECRETS_PASSWORD) is not set, audit
// writing is skipped after a single warning (graceful degradation) and the
// workflow is unaffected. Any audit write failure is logged at warn level and
// never blocks execution. Returns the receiver for chaining.
//
// IMPORTANT (H-5): auditDir is process-global state via
// history.SetHistoryDir. Do NOT configure different auditDir values across
// concurrent Executor instances — the last SetHistoryDir call wins, so
// concurrent Executors with different dirs would silently bleed each other's
// audit records into whichever directory was set most recently. Use a single
// audit directory for all workflows in a process, or disable audit
// per-Executor (WithAuditLog(false, "")). The CLI additionally guards
// against cross-process hash-chain corruption via an audit-directory lock
// (see cmd/aflare acquireAuditLock).
func (e *Executor) WithAuditLog(enabled bool, dir string) *Executor {
	e.auditEnabled = enabled
	e.auditDir = dir
	return e
}

// WithIdempotencyKey enables workflow idempotency for this Executor's runs
// using the given key (e.g. an Idempotency-Key header from an incoming HTTP
// request). When set, ExecuteWithTrace consults the configured
// IdempotencyStore before executing:
//
//   - If a record exists for the key with status "completed", the cached
//     final output is returned together with ErrIdempotencyHit and NO step
//     is re-run. This prevents duplicate side effects (e.g. duplicate money
//     transfers in financial flows) when the same workflow is triggered
//     multiple times with the same key.
//   - Otherwise (no record, or a prior "failed"/"in_progress" record) the
//     workflow executes normally and the new run_id + result are recorded so
//     the next trigger for the same key becomes a cache hit.
//
// If no store has been configured via WithIdempotencyStore, a default
// FileIdempotencyStore at DefaultIdempotencyDir() (~/.config/aflare/
// idempotency) is used. Idempotency is otherwise OFF by default, so existing
// callers that do not set a key see no behaviour change.
//
// The generated run_id (one per non-cached execution) is exposed on the
// returned WorkflowTrace.RunID so callers can correlate WAL files, audit
// records, etc. Callers combining idempotency with WAL crash-resume should
// name the WAL file with the run_id so an in-progress run can be resumed.
//
// Returns the receiver for chaining.
func (e *Executor) WithIdempotencyKey(key string) *Executor {
	e.idempotencyKey = key
	if e.idempotencyStore == nil {
		e.idempotencyStore = NewFileIdempotencyStore(defaultIdempotencyDir(), defaultIdempotencyTTL)
	}
	return e
}

// WithIdempotencyStore overrides the IdempotencyStore used for idempotency
// checks. This is primarily a testing hook (e.g. to point at a temp dir or a
// short TTL). The key itself must still be set via WithIdempotencyKey to
// activate idempotency. Returns the receiver for chaining.
func (e *Executor) WithIdempotencyStore(store IdempotencyStore) *Executor {
	e.idempotencyStore = store
	return e
}

// WithWorkflowPath sets the original workflow file path for pause-resume
// metadata. When a resumable step is paused, this path is stored in the
// run metadata so the resume command can locate the original workflow.
func (e *Executor) WithWorkflowPath(path string) *Executor {
	e.wfPath = path
	return e
}

// SetupShutdown registers OS signal handlers (SIGINT, SIGTERM) for standalone
// CLI use. When a signal is received, SignalShutdown is called to mark the
// global shutdown flag, which causes all running workflow executions to stop
// starting new steps after the current one completes. Deferred cleanup (WAL
// flush, audit finalization) runs when each execution's function returns.
//
// This is intended for standalone CLI use (aflare run). When the Executor is
// used through the HTTP server, the server's own signal handler calls
// SignalShutdown and srv.Shutdown, so this method is not needed.
//
// It is safe to call SetupShutdown multiple times on the same Executor.
func (e *Executor) SetupShutdown() {
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("executor received shutdown signal, stopping gracefully...")
		SignalShutdown()
		e.Shutdown()
	}()
}

// Shutdown waits for all in-flight executions tracked by this Executor to
// complete (current step finishes, WAL flushed, audit finalized), then
// returns. It is called automatically by SetupShutdown's signal handler or
// can be called directly by the caller.
func (e *Executor) Shutdown() {
	logger.Info("executor shutting down, waiting for current steps to complete...")
	e.wg.Wait()
	logger.Info("executor shutdown complete")
}

// Execute runs the workflow without a TUI program. It is the checkpoint-aware
// equivalent of ExecuteWorkflow.
func (e *Executor) Execute(ctx context.Context, wf *Workflow, reg *nodes.Registry) (string, []StepResult, error) {
	output, results, _, err := e.ExecuteWithTrace(ctx, wf, reg, nil)
	return output, results, err
}

// ExecuteWithTrace runs the workflow and returns a detailed per-step trace.
// It is the checkpoint-aware equivalent of ExecuteWorkflowWithTrace.
func (e *Executor) ExecuteWithTrace(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program) (string, []StepResult, *WorkflowTrace, error) {
	// Track this execution in the WaitGroup so Shutdown can wait for it.
	e.wg.Add(1)
	defer e.wg.Done()

	// Idempotency: guard against duplicate side effects when the same
	// Idempotency-Key is re-triggered, including concurrently. The previous
	// implementation did a non-atomic Check → execute → Record: two concurrent
	// requests with the same key could both observe "not found" and both
	// execute (e.g. double-charging a transfer). We now atomically Reserve an
	// in_progress placeholder before executing, so a concurrent same-key
	// request is rejected (ErrIdempotencyInProgress) or served from cache
	// (ErrIdempotencyHit) instead of re-running side-effecting nodes.
	runID := ""
	reserved := false
	// The audit recorder is built before the idempotency check so that a
	// cache hit or a concurrent-run rejection — both of which return before
	// the normal recordStart/recordCompletion path — can still leave an
	// audit trail. In financial scenarios "a transfer was served from cache"
	// and "a duplicate trigger was suppressed" are themselves auditable
	// events. recordStart is still only called for real executions below.
	audit := e.newAuditRecorder(wf)
	if e.idempotencyKey != "" && e.idempotencyStore != nil {
		// 1. Fast path: a completed record is served from cache without
		//    acquiring the cross-process lock; an in_progress record means
		//    another run is mid-flight and this request is rejected. A failed
		//    record (or no record) falls through to Reserve, which re-reads
		//    authoritatively under the lock. A Check read failure is non-fatal:
		//    we log and proceed to Reserve, the single source of truth.
		if rec, found, cerr := e.idempotencyStore.Check(e.idempotencyKey); cerr != nil {
			logger.Warn("idempotency check failed, proceeding to reserve", "key", e.idempotencyKey, "error", cerr)
		} else if found {
			switch rec.Status {
			case idempotencyStatusCompleted:
				logger.Info("idempotency hit, returning cached result", "key", e.idempotencyKey, "run_id", rec.RunID)
				audit.recordIdempotencyHit(rec)
				trace := newTrace(wf.Name, "idempotent", time.Now(), 0)
				trace.RunID = rec.RunID
				trace.IdempotencyHit = true
				trace.finish(time.Now())
				return rec.FinalOutput, nil, trace, ErrIdempotencyHit
			case idempotencyStatusInProgress:
				logger.Info("idempotency in-progress, rejecting concurrent run", "key", e.idempotencyKey, "run_id", rec.RunID)
				audit.recordIdempotencyRejected(rec)
				trace := newTrace(wf.Name, "idempotent", time.Now(), 0)
				trace.RunID = rec.RunID
				trace.finish(time.Now())
				return "", nil, trace, ErrIdempotencyInProgress
			}
		}

		// 2. Atomic placeholder: prevents a concurrent same-key request from
		//    also executing. Reserve is the authoritative check — it re-reads
		//    under the lock and wins or loses atomically, closing the race that
		//    a standalone Check leaves open.
		runID = newRunID()
		rec, ok, rerr := e.idempotencyStore.Reserve(e.idempotencyKey, runID)
		if rerr != nil {
			// ErrIdempotencyInProgress (a run started between our Check and
			// Reserve) or a real store error: either way we must not execute.
			// A completed record that appeared in the race window is surfaced
			// as a cache hit.
			if rec.Status == idempotencyStatusCompleted {
				logger.Info("idempotency hit after reserve race, returning cached result", "key", e.idempotencyKey, "run_id", rec.RunID)
				audit.recordIdempotencyHit(rec)
				trace := newTrace(wf.Name, "idempotent", time.Now(), 0)
				trace.RunID = rec.RunID
				trace.IdempotencyHit = true
				trace.finish(time.Now())
				return rec.FinalOutput, nil, trace, ErrIdempotencyHit
			}
			if rec.Status == idempotencyStatusInProgress {
				logger.Info("idempotency in-progress after reserve, rejecting concurrent run", "key", e.idempotencyKey, "run_id", rec.RunID)
				audit.recordIdempotencyRejected(rec)
			} else {
				logger.Warn("idempotency reserve failed, rejecting run", "key", e.idempotencyKey, "error", rerr)
			}
			trace := newTrace(wf.Name, "idempotent", time.Now(), 0)
			trace.RunID = rec.RunID
			trace.finish(time.Now())
			return "", nil, trace, rerr
		}
		if !ok {
			// Lost the reservation race; rec holds the winning record.
			if rec.Status == idempotencyStatusCompleted {
				logger.Info("idempotency hit after reserve race, returning cached result", "key", e.idempotencyKey, "run_id", rec.RunID)
				audit.recordIdempotencyHit(rec)
				trace := newTrace(wf.Name, "idempotent", time.Now(), 0)
				trace.RunID = rec.RunID
				trace.IdempotencyHit = true
				trace.finish(time.Now())
				return rec.FinalOutput, nil, trace, ErrIdempotencyHit
			}
			logger.Info("idempotency in-progress after reserve race, rejecting concurrent run", "key", e.idempotencyKey, "run_id", rec.RunID)
			audit.recordIdempotencyRejected(rec)
			trace := newTrace(wf.Name, "idempotent", time.Now(), 0)
			trace.RunID = rec.RunID
			trace.finish(time.Now())
			return "", nil, trace, ErrIdempotencyInProgress
		}
		reserved = true
	}

	audit.recordStart()

	var (
		out     string
		results []StepResult
		trace   *WorkflowTrace
		err     error
	)
	if hasDAGDeclarations(wf.Steps) {
		// DAG mode does not support checkpoint/resume; fall through to the
		// standard DAG executor which ignores statePath.
		if e.statePath != "" {
			logger.Warn("checkpoint/resume is not supported in DAG mode, ignoring statePath", "path", e.statePath)
		}
		if e.walPath != "" {
			logger.Warn("WAL checkpoint/resume is not supported in DAG mode, ignoring walPath", "path", e.walPath)
		}
		out, results, trace, err = executeWorkflowDAG(ctx, wf, reg, program, e.workflowTimeout)
	} else {
		// WAL takes precedence over JSON checkpoint when both are configured.
		statePath := e.statePath
		walPath := e.walPath
		if walPath != "" {
			statePath = "" // WAL path is the source of truth
		}
		out, results, trace, err = executeWorkflowSequential(ctx, wf, reg, program, statePath, walPath, e.wfPath, e.workflowTimeout)
	}
	recordWorkflowMetrics(trace, err)
	audit.recordCompletion(results, trace, err)

	// Persist the idempotency outcome so a repeat trigger for this key is a
	// cache hit. The run_id is stamped on the trace for correlation. Only the
	// run that won the Reserve writes the final record; a failed workflow
	// records status=failed so the next trigger may retry. Record failures are
	// non-fatal: the workflow has already run, so we log and move on (the next
	// trigger will simply re-execute).
	if reserved {
		if trace != nil {
			trace.RunID = runID
		}
		status := idempotencyStatusCompleted
		errMsg := ""
		if err != nil {
			status = idempotencyStatusFailed
			errMsg = err.Error()
		}
		now := time.Now().UTC()
		rec := IdempotencyRecord{
			Key:          e.idempotencyKey,
			RunID:        runID,
			WorkflowPath: wf.Name,
			Status:       status,
			FinalOutput:  out,
			Error:        errMsg,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if rerr := e.idempotencyStore.Record(rec); rerr != nil {
			logger.Warn("idempotency record failed, next trigger will re-execute", "key", e.idempotencyKey, "error", rerr)
		}
	}
	return out, results, trace, err
}

// traceLLMError is the error reconstructed from an LLMStepTrace.ErrText
// when recording per-call metrics. The original error object is not
// preserved in the trace (traces are JSON-serialised, so they carry only
// the error's text), so this type cannot Unwrap the real error — it
// intentionally has no Unwrap method to avoid implying a chain that
// doesn't exist. Carrying StatusCode lets future metrics distinguish
// provider HTTP errors (5xx) from client-side failures (status 0,
// e.g. context cancellation / connection refused) without re-parsing
// the error text.
type traceLLMError struct {
	text       string
	statusCode int
}

func (e *traceLLMError) Error() string { return e.text }

// recordWorkflowMetrics publishes Prometheus metrics for a completed workflow
// run: the overall execution counter/duration and the per-call LLM telemetry
// aggregated in trace.Steps[*].LLM (provider/model/tokens/cost). It is
// lightweight — direct Inc/Observe calls, no goroutine — and safe to call with
// a nil trace (only the workflow counter is updated, with zero duration).
func recordWorkflowMetrics(trace *WorkflowTrace, runErr error) {
	var duration time.Duration
	if trace != nil {
		duration = trace.Duration
		for _, step := range trace.Steps {
			for _, call := range step.LLM {
				var callErr error
				if call.ErrText != "" {
					// Reconstruct a typed error rather than a bare
					// errors.New(string): the typed form carries the
					// HTTP status code and is identifiable as a
					// trace-originated error. metrics.RecordLLMCall
					// currently only checks err != nil, but the typed
					// form means a future metrics evolution (e.g.
					// counting 5xx vs client-side failures separately)
					// can switch on the type without re-stringifying.
					callErr = &traceLLMError{text: call.ErrText, statusCode: call.StatusCode}
				}
				metrics.RecordLLMCall(call.Provider, call.Model, callErr,
					call.PromptTokens, call.CompletionTokens, call.CostUSD)
			}
		}
	}
	metrics.RecordWorkflowExecution(duration, runErr)
}
