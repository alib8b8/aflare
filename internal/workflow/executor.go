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
	"fmt"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
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

// WorkflowTimeout is the overall workflow timeout. It defaults to
// DefaultWorkflowTimeout but can be overridden by callers to configure a
// different workflow timeout without modifying types.go.
var WorkflowTimeout = DefaultWorkflowTimeout

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
func ExecuteWorkflowWithTrace(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program) (string, []StepResult, *WorkflowTrace, error) {
	if hasDAGDeclarations(wf.Steps) {
		return executeWorkflowDAG(ctx, wf, reg, program)
	}
	return executeWorkflowSequential(ctx, wf, reg, program)
}

// executeWorkflowSequential runs the workflow step-by-step in declaration order
// and records per-step telemetry into a WorkflowTrace (Mode="sequential").
func executeWorkflowSequential(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program) (string, []StepResult, *WorkflowTrace, error) {
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

	timeoutCtx, cancel := context.WithTimeout(ctx, WorkflowTimeout)
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

	for i, wStep := range wf.Steps {
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
					onChunk := func(chunk string) {
						program.Send(tui.StepStreamMsg{
							Index: i,
							Name:  wStep.Node,
							Chunk: chunk,
						})
					}
					output, execErr = streamingNode.ExecuteStream(stepCtx, data, evaluatedParams, onChunk)
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
		// resultErr tracks the error shown in StepResult. For fallback/on_error,
		// the step is recovered so resultErr is cleared. For continue_on_error,
		// the step genuinely failed so resultErr is preserved.
		resultErr := execErr
		var recoveries []string
		if execErr != nil {
			// 0. capture_error: run the error branch (treats the error as a
			// value/branch condition rather than swallowing it). Checked first
			// because it is the most expressive recovery primitive.
			if wStep.HasCaptureError() {
				branchOut, bErr := executeCaptureErrorBranch(stepBaseCtx, wStep.CaptureError, execErr.Error(), engine.SnapshotVars(), reg, program, globalLimiter)
				if bErr == nil {
					logger.Info("step recovered via capture_error branch", "index", i, "node", wStep.Node)
					output = branchOut
					execErr = nil
					resultErr = nil
					recoveries = append(recoveries, "capture_error")
				} else {
					logger.Warn("capture_error branch failed, falling through to other recovery", "index", i, "node", wStep.Node, "error", nodes.RedactSensitive(bErr.Error()))
				}
			}
			// 1. Try fallback value
			if execErr != nil && wStep.Fallback != "" {
				fallbackVal, ferr := engine.Evaluate(wStep.Fallback, data)
				if ferr == nil {
					logger.Info("step recovered via fallback", "index", i, "node", wStep.Node)
					output = fallbackVal
					execErr = nil
					resultErr = nil
					recoveries = append(recoveries, "fallback")
				}
			}
			// 2. Try on_error handler node
			if execErr != nil && wStep.OnError != nil {
				errStep := *wStep.OnError
				errParams, eerr := engine.EvaluateParams(errStep.Params, data)
				if eerr == nil {
					if errNode, ok := reg.Get(errStep.Node); ok {
						// Use stepBaseCtx so the handler's LLM calls (if any)
						// are captured by the same step collector.
						errOutput, errExecErr := errNode.Execute(stepBaseCtx, data, errParams)
						if errExecErr == nil {
							logger.Info("step recovered via on_error handler", "index", i, "handler", errStep.Node)
							output = errOutput
							execErr = nil
							resultErr = nil
							recoveries = append(recoveries, "on_error")
						}
					}
				}
			}
			// 3. Check continue_on_error: clear execErr so workflow continues,
			// but keep resultErr so StepResult reflects the actual failure.
			if execErr != nil && wStep.ContinueOnError {
				logger.Warn("step failed but continue_on_error is set, continuing", "index", i, "node", wStep.Node, "error", nodes.RedactSensitive(execErr.Error()))
				output = ""
				execErr = nil
				recoveries = append(recoveries, "continue_on_error")
			}
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
