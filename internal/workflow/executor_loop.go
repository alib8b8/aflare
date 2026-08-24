// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​​​‌​‌‌‌‌‌‌​‌‌​‌​‌‌​‌​​​‌‌​​​‌‌​‌‌​​‌​​‌‌​​​​​​​​​​​​​​​​​​​​​​​‌​‌​‌‌‌‌​‌‌​‌‌​⁠
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
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/telemetry"
	"github.com/alib8b8/aflare/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// executeLoopStep executes a step in a loop over a list of items.
func executeLoopStep(ctx context.Context, stepIndex int, wStep WorkflowStep, input string, engine *ExpressionEngine, reg *nodes.Registry, program *tea.Program, globalLimiter *ConcurrencyLimiter) ([]StepResult, string, error) {
	loopCfg := wStep.Loop

	// Evaluate items expression
	itemsStr, err := engine.Evaluate(loopCfg.Items, input)
	if err != nil {
		return nil, "", fmt.Errorf("failed to evaluate loop items: %w", err)
	}

	// Split items
	splitBy := loopCfg.GetSplitBy()
	var items []string
	if splitBy == "\n" {
		items = strings.Split(itemsStr, "\n")
	} else {
		items = strings.Split(itemsStr, splitBy)
	}

	// Filter empty items
	var validItems []string
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			validItems = append(validItems, item)
		}
	}
	items = validItems

	// Safety limits
	maxIter := loopCfg.GetMaxIterations()
	if len(items) > maxIter {
		return nil, "", fmt.Errorf("loop items (%d) exceed max_iterations (%d)", len(items), maxIter)
	}
	if len(items) == 0 {
		return nil, "", nil
	}

	// Get node
	node, ok := reg.Get(wStep.Node)
	if !ok {
		return nil, "", fmt.Errorf("node '%s' not found in registry", wStep.Node)
	}

	// Pre-evaluate params for each iteration in the main goroutine
	// (ExpressionEngine is not thread-safe)
	type iterEval struct {
		item   string
		params map[string]string
		err    error
	}
	iterEvals := make([]iterEval, len(items))
	for idx, item := range items {
		engine.SetLoopVars(item, idx, len(items))
		evaluatedParams, eerr := engine.EvaluateParams(wStep.Params, input)
		if evaluatedParams == nil {
			evaluatedParams = make(map[string]string)
		}
		evaluatedParams[loopCfg.GetVar()] = item
		iterEvals[idx] = iterEval{item: item, params: evaluatedParams, err: eerr}
	}
	engine.ClearLoopVars()

	retryCount := wStep.GetRetryCount()
	retryDelay := wStep.GetRetryDelay()
	stepTimeout := wStep.GetTimeout()
	concurrency := loopCfg.GetConcurrency()
	stopOnError := loopCfg.GetStopOnError()

	var results []StepResult
	var outputs []string

	if concurrency <= 1 {
		// ── Sequential execution ──
		for idx, ie := range iterEvals {
			if program != nil {
				program.Send(tui.StepStartMsg{
					Index: stepIndex*MaxParallel + idx,
					Name:  wStep.Node,
				})
			}

			start := time.Now()
			var output string
			var execErr error
			if ie.err != nil {
				execErr = ie.err
			} else {
				_, subSpan := telemetry.StartSubStepSpan(ctx, wStep.Name, wStep.Node, stepIndex*MaxParallel+idx)
				output, execErr = executeWithRetry(ctx, node, ie.item, ie.params, retryCount, retryDelay, stepTimeout)
				telemetry.SubStepSpanEnd(subSpan, execErr, time.Since(start).Milliseconds(), len(output))
			}
			duration := time.Since(start)

			sr := StepResult{
				StepIndex: stepIndex*MaxParallel + idx,
				NodeName:  wStep.Node,
				Input:     ie.item,
				Output:    output,
				Error:     execErr,
				Duration:  duration,
			}
			results = append(results, sr)

			if program != nil {
				program.Send(tui.StepEndMsg{
					Index:    stepIndex*MaxParallel + idx,
					Name:     wStep.Node,
					Output:   output,
					Error:    execErr,
					Duration: duration,
				})
			}

			if execErr != nil {
				logger.Error("loop iteration failed", "index", idx, "node", wStep.Node, "error", nodes.RedactSensitive(execErr.Error()))
				if stopOnError {
					return results, "", fmt.Errorf("loop iteration %d failed: %w", idx+1, execErr)
				}
			} else {
				outputs = append(outputs, output)
			}
		}
	} else {
		// ── Concurrent execution ──
		sem := make(chan struct{}, concurrency)
		type loopResult struct {
			idx    int
			output string
			err    error
			dur    time.Duration
		}
		resultsChan := make(chan loopResult, len(items))

		for idx, ie := range iterEvals {
			sem <- struct{}{}
			go func(idx int, item string, params map[string]string, perr error) {
				defer func() { <-sem }()
				defer func() {
					if r := recover(); r != nil {
						logger.Error("loop iteration panicked",
							"index", idx,
							"node", wStep.Node,
							"panic", r,
							"stack", string(debug.Stack()),
						)
						resultsChan <- loopResult{
							idx: idx,
							err: fmt.Errorf("loop iteration panicked: %v", r),
							dur: 0,
						}
					}
				}()
				// Acquire global concurrency slot if configured
				if globalLimiter != nil {
					if err := globalLimiter.Acquire(ctx); err != nil {
						resultsChan <- loopResult{idx: idx, err: err, dur: 0}
						return
					}
					defer globalLimiter.Release()
				}
				start := time.Now()

				if program != nil {
					program.Send(tui.StepStartMsg{
						Index: stepIndex*MaxParallel + idx,
						Name:  wStep.Node,
					})
				}

				if perr != nil {
					resultsChan <- loopResult{idx: idx, err: perr, dur: time.Since(start)}
					return
				}
				_, subSpan := telemetry.StartSubStepSpan(ctx, wStep.Name, wStep.Node, stepIndex*MaxParallel+idx)
				output, execErr := executeWithRetry(ctx, node, item, params, retryCount, retryDelay, stepTimeout)
				dur := time.Since(start)
				telemetry.SubStepSpanEnd(subSpan, execErr, dur.Milliseconds(), len(output))
				resultsChan <- loopResult{idx: idx, output: output, err: execErr, dur: dur}
			}(idx, ie.item, ie.params, ie.err)
		}

		// Collect all results
		loopResults := make([]loopResult, len(items))
		for i := 0; i < len(items); i++ {
			lr := <-resultsChan
			loopResults[lr.idx] = lr
		}

		// Process in order
		for _, lr := range loopResults {
			sr := StepResult{
				StepIndex: stepIndex*MaxParallel + lr.idx,
				NodeName:  wStep.Node,
				Input:     iterEvals[lr.idx].item,
				Output:    lr.output,
				Error:     lr.err,
				Duration:  lr.dur,
			}
			results = append(results, sr)

			if program != nil {
				program.Send(tui.StepEndMsg{
					Index:    stepIndex*MaxParallel + lr.idx,
					Name:     wStep.Node,
					Output:   lr.output,
					Error:    lr.err,
					Duration: lr.dur,
				})
			}

			if lr.err != nil {
				logger.Error("loop iteration failed", "index", lr.idx, "node", wStep.Node, "error", nodes.RedactSensitive(lr.err.Error()))
				if stopOnError {
					return results, "", fmt.Errorf("loop iteration %d failed: %w", lr.idx+1, lr.err)
				}
			} else {
				outputs = append(outputs, lr.output)
			}
		}
	}

	// Combine outputs
	finalOutput := strings.Join(outputs, "\n---\n")
	return results, finalOutput, nil
}
