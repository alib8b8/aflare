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
	"fmt"
	"runtime/debug"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/telemetry"
	"github.com/alib8b8/aflare/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

//nolint:funlen // TODO(#54): split into smaller steps by 2026-09-13
func executeParallelStep(ctx context.Context, stepIndex int, wStep WorkflowStep, input string, engine *ExpressionEngine, reg *nodes.Registry, program *tea.Program, globalLimiter *ConcurrencyLimiter) ([]StepResult, string, error) {
	// Limit parallel step count
	if len(wStep.Parallel) > MaxParallel {
		return nil, "", fmt.Errorf("too many parallel steps (%d, max %d)", len(wStep.Parallel), MaxParallel)
	}

	logger.Info("parallel step started", "index", stepIndex, "parallel_count", len(wStep.Parallel))

	type parallelResult struct {
		stepIndex int
		nodeName  string
		output    string
		err       error
		duration  time.Duration
	}

	// Pre-evaluate conditions and params in the main goroutine before spawning
	// goroutines. The ExpressionEngine reads its internal maps (stepOutputs,
	// variables, etc.) without synchronization, so concurrent access from
	// parallel goroutines would cause a data race.
	type preEval struct {
		nodeName        string
		evaluatedParams map[string]string
		paramsErr       error
		condPass        bool // valid only when hasCond && condErr == nil
		condErr         error
		hasCond         bool
	}

	preEvals := make([]preEval, len(wStep.Parallel))
	for j, step := range wStep.Parallel {
		pe := preEval{nodeName: step.Node}
		if step.Condition != "" {
			pe.hasCond = true
			pe.condPass, pe.condErr = evaluateCondition(step.Condition, input, engine)
		}
		// Only evaluate params when the condition allows execution
		if pe.condErr == nil && (!pe.hasCond || pe.condPass) {
			pe.evaluatedParams, pe.paramsErr = engine.EvaluateParams(step.Params, input)
		}
		preEvals[j] = pe
	}

	resultsChan := make(chan parallelResult, len(wStep.Parallel))

	for j, step := range wStep.Parallel {
		pe := preEvals[j]
		go func(j int, step Step, pe preEval) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("parallel step panicked",
						"step_index", stepIndex*MaxParallel+j,
						"node", pe.nodeName,
						"panic", r,
						"stack", string(debug.Stack()),
					)
					resultsChan <- parallelResult{
						stepIndex: stepIndex*MaxParallel + j,
						nodeName:  pe.nodeName,
						err:       fmt.Errorf("parallel step panicked: %v", r),
						duration:  0,
					}
				}
			}()
			// Acquire global concurrency slot if configured
			if globalLimiter != nil {
				if err := globalLimiter.Acquire(ctx); err != nil {
					resultsChan <- parallelResult{
						stepIndex: stepIndex*MaxParallel + j,
						nodeName:  pe.nodeName,
						err:       err,
						duration:  0,
					}
					return
				}
				defer globalLimiter.Release()
			}
			start := time.Now()
			nodeName := pe.nodeName
			// Use compound index (stepIndex*MaxParallel+j) to distinguish
			// parallel sub-steps from main steps in the TUI display.

			// Handle pre-evaluated condition
			if pe.hasCond {
				if pe.condErr != nil {
					resultsChan <- parallelResult{
						stepIndex: stepIndex*MaxParallel + j,
						nodeName:  nodeName,
						err:       fmt.Errorf("condition evaluation failed: %w", pe.condErr),
						duration:  time.Since(start),
					}
					return
				}
				if !pe.condPass {
					if program != nil {
						program.Send(tui.StepStartMsg{
							Index: stepIndex*MaxParallel + j,
							Name:  nodeName,
						})
						program.Send(tui.StepEndMsg{
							Index:    stepIndex*MaxParallel + j,
							Name:     nodeName,
							Output:   "",
							Duration: 0,
						})
					}
					resultsChan <- parallelResult{
						stepIndex: stepIndex*MaxParallel + j,
						nodeName:  nodeName,
						output:    "",
						duration:  0,
					}
					return
				}
			}

			if program != nil {
				program.Send(tui.StepStartMsg{
					Index: stepIndex*MaxParallel + j,
					Name:  nodeName,
				})
			}

			// Handle pre-evaluated params
			if pe.paramsErr != nil {
				resultsChan <- parallelResult{
					stepIndex: stepIndex*MaxParallel + j,
					nodeName:  nodeName,
					err:       pe.paramsErr,
					duration:  time.Since(start),
				}
				return
			}

			node, ok := reg.Get(nodeName)
			if !ok {
				resultsChan <- parallelResult{
					stepIndex: stepIndex*MaxParallel + j,
					nodeName:  nodeName,
					err:       fmt.Errorf("node '%s' not found in registry", nodeName),
					duration:  time.Since(start),
				}
				return
			}

			retryCount := step.GetRetryCount()
			if retryCount > MaxRetry {
				retryCount = MaxRetry
			}
			retryDelay := step.GetRetryDelay()
			if retryDelay > MaxRetryDelay {
				retryDelay = MaxRetryDelay
			}

			_, subSpan := telemetry.StartSubStepSpan(ctx, "", nodeName, stepIndex*MaxParallel+j)
			var output string
			var execErr error
			maxAttempts := retryCount + 1

		retryLoop:
			for attempt := 1; attempt <= maxAttempts; attempt++ {
				var stepCtx context.Context
				var stepCancel context.CancelFunc
				stepTimeout := step.GetTimeout()
				if stepTimeout > 0 {
					stepCtx, stepCancel = context.WithTimeout(ctx, stepTimeout)
				} else {
					stepCtx, stepCancel = context.WithCancel(ctx)
				}

				// Note: parallel steps do not support streaming output.
				// Interleaving chunks from multiple concurrent streams in the
				// TUI would be confusing, so we always use the non-streaming
				// Execute API here.
				output, execErr = node.Execute(stepCtx, input, pe.evaluatedParams)
				stepCancel()

				if execErr == nil {
					break
				}
				if attempt < maxAttempts {
					select {
					case <-time.After(retryDelay):
					case <-ctx.Done():
						execErr = ctx.Err()
						break retryLoop
					}
				}
			}
			dur := time.Since(start)
			telemetry.SubStepSpanEnd(subSpan, execErr, dur.Milliseconds(), len(output))

			resultsChan <- parallelResult{
				stepIndex: stepIndex*MaxParallel + j,
				nodeName:  nodeName,
				output:    output,
				err:       execErr,
				duration:  dur,
			}
		}(j, step, pe)
	}

	var stepResults []StepResult
	var outputs []string
	var firstErr error

	for i := 0; i < len(wStep.Parallel); i++ {
		res := <-resultsChan
		sr := StepResult{
			StepIndex: res.stepIndex,
			NodeName:  res.nodeName,
			Input:     input,
			Output:    res.output,
			Error:     res.err,
			Duration:  res.duration,
		}
		stepResults = append(stepResults, sr)

		if program != nil {
			program.Send(tui.StepEndMsg{
				Index:    res.stepIndex,
				Name:     res.nodeName,
				Output:   res.output,
				Error:    res.err,
				Duration: res.duration,
			})
		}

		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
			}
			logger.Error("parallel step failed", "index", res.stepIndex, "node", res.nodeName, "error", nodes.RedactSensitive(res.err.Error()))
		} else {
			outputs = append(outputs, res.output)
			logger.Info("parallel step completed", "index", res.stepIndex, "node", res.nodeName, "duration", res.duration)
		}
	}

	if firstErr != nil {
		// Check max_failures threshold for parallel groups
		maxFailures := wStep.MaxFailures
		if maxFailures < 0 {
			maxFailures = 0
		}
		failureCount := 0
		for _, sr := range stepResults {
			if sr.Error != nil {
				failureCount++
			}
		}
		if failureCount > maxFailures {
			return stepResults, "", fmt.Errorf("parallel step failed: %d/%d sub-steps failed (max_failures: %d): %w", failureCount, len(wStep.Parallel), maxFailures, firstErr)
		}
		logger.Warn("parallel step had failures but within max_failures limit", "failures", failureCount, "max", maxFailures)
	}

	finalOutput := ""
	for _, out := range outputs {
		if finalOutput != "" {
			finalOutput += "\n---\n"
		}
		finalOutput += out
	}

	return stepResults, finalOutput, nil
}

// executeWithRetry runs a node with retry logic. Used by loop iterations.
func executeWithRetry(ctx context.Context, node nodes.Node, input string, params map[string]string, retryCount int, retryDelay, timeout time.Duration) (string, error) {
	if retryCount > MaxRetry {
		retryCount = MaxRetry
	}
	if retryDelay > MaxRetryDelay {
		retryDelay = MaxRetryDelay
	}
	if timeout > MaxStepTimeout {
		timeout = MaxStepTimeout
	}

	maxAttempts := retryCount + 1
	var output string
	var execErr error

retryLoop:
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var stepCtx context.Context
		var stepCancel context.CancelFunc
		if timeout > 0 {
			stepCtx, stepCancel = context.WithTimeout(ctx, timeout)
		} else {
			stepCtx, stepCancel = context.WithCancel(ctx)
		}

		output, execErr = node.Execute(stepCtx, input, params)
		stepCancel()

		if execErr == nil {
			break
		}
		if attempt < maxAttempts {
			select {
			case <-time.After(retryDelay):
			case <-ctx.Done():
				execErr = ctx.Err()
				break retryLoop
			}
		}
	}
	return output, execErr
}
