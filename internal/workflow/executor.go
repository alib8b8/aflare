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
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
	"github.com/alib8b8/llm-box/internal/metrics"
	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/secrets"
	"github.com/alib8b8/llm-box/internal/tui"
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
	out, results, trace, err := executeWorkflowSequential(ctx, wf, reg, program, "", DefaultWorkflowTimeout)
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
func executeWorkflowSequential(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program, statePath string, timeout time.Duration) (string, []StepResult, *WorkflowTrace, error) {
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
	// If a checkpoint file exists at statePath, restore the engine state
	// (step outputs, variables, flowing data) and continue execution from
	// the step after the one recorded in the checkpoint.
	resumeFromStep := 0
	if statePath != "" {
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
	// set. Failures are logged but never interrupt the workflow.
	saveCheckpointIfEnabled := func(stepIndex int) {
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
		stepStart := time.Now()

		// ── Handle if/else branch ──
		if wStep.IsIf() {
			branchResults, output, err := executeIfBranch(timeoutCtx, i, wStep.If, data, engine, reg, program, globalLimiter)
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
				TotalDuration:   time.Since(stepStart),
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
			loopResults, output, err := executeLoopStep(timeoutCtx, i, wStep, data, engine, reg, program, globalLimiter)
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
				TotalDuration:   time.Since(stepStart),
				InputLen:        len(data),
				OutputLen:       len(output),
			})
			saveCheckpointIfEnabled(i)
			continue
		}

		if wStep.IsMap() {
			mapResults, output, err := executeMapStep(timeoutCtx, i, wStep, data, engine, reg, program, globalLimiter)
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
				TotalDuration:   time.Since(stepStart),
				InputLen:        len(data),
				OutputLen:       len(output),
			})
			saveCheckpointIfEnabled(i)
			continue
		}

		if wStep.IsReduce() {
			reduceResults, output, err := executeReduceStep(timeoutCtx, i, wStep, data, engine, reg, program, globalLimiter)
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
				TotalDuration:   time.Since(stepStart),
				InputLen:        len(data),
				OutputLen:       len(output),
			})
			saveCheckpointIfEnabled(i)
			continue
		}

		if wStep.IsParallel() {
			parallelResults, output, err := executeParallelStep(timeoutCtx, i, wStep, data, engine, reg, program, globalLimiter)
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
				TotalDuration:   time.Since(stepStart),
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
		stepTimeout := wStep.GetTimeout()
		if stepTimeout > MaxStepTimeout {
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
//	exec := NewExecutor().WithCheckpoint("~/.llm-box/checkpoints/wf.json")
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
	workflowTimeout time.Duration
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

// WithTimeout configures the overall workflow timeout applied to the derived
// context for this Executor's runs. Returns the receiver for chaining.
//
// Use this instead of mutating the package-level WorkflowTimeout global,
// which is unsafe under parallel tests (t.Parallel) and deprecated.
func (e *Executor) WithTimeout(d time.Duration) *Executor {
	e.workflowTimeout = d
	return e
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
	if hasDAGDeclarations(wf.Steps) {
		// DAG mode does not support checkpoint/resume; fall through to the
		// standard DAG executor which ignores statePath.
		if e.statePath != "" {
			logger.Warn("checkpoint/resume is not supported in DAG mode, ignoring statePath", "path", e.statePath)
		}
		out, results, trace, err := executeWorkflowDAG(ctx, wf, reg, program, e.workflowTimeout)
		recordWorkflowMetrics(trace, err)
		return out, results, trace, err
	}
	out, results, trace, err := executeWorkflowSequential(ctx, wf, reg, program, e.statePath, e.workflowTimeout)
	recordWorkflowMetrics(trace, err)
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
