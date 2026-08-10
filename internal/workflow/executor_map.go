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
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/telemetry"
	"github.com/alib8b8/aflare/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// executeMapStep runs a sub-workflow once per item in the list evaluated
// from MapConfig.Over. Each iteration gets its own ExpressionEngine so
// {{step.X}} / {{item}} / {{index}} / {{count}} resolve within the
// iteration; outer workflow vars are inherited.
//
// Per-item outputs are collected in order and combined via
// applyOutputStrategy (default: json_array).
func executeMapStep(ctx context.Context, stepIndex int, wStep WorkflowStep, input string, parentEngine *ExpressionEngine, reg *nodes.Registry, program *tea.Program, globalLimiter *ConcurrencyLimiter) ([]StepResult, string, error) {
	cfg := wStep.Map

	// 1. Evaluate `over` in the main goroutine (engine is not thread-safe).
	overStr, err := parentEngine.Evaluate(cfg.Over, input)
	if err != nil {
		return nil, "", fmt.Errorf("map: failed to evaluate `over`: %w", err)
	}

	// 2. Resolve items: prefer JSON array, fall back to split-by.
	items, err := resolveMapItems(overStr, cfg.GetSplitBy())
	if err != nil {
		return nil, "", fmt.Errorf("map: failed to resolve items: %w", err)
	}

	maxIter := cfg.GetMaxIterations()
	if len(items) > maxIter {
		return nil, "", fmt.Errorf("map: items (%d) exceed max_iterations (%d)", len(items), maxIter)
	}
	if len(items) == 0 {
		return nil, "", nil
	}

	// 3. Snapshot the parent engine's vars/step-outputs so each iteration
	// inherits the outer context but mutates its own copy. We capture the
	// current var map; step outputs are NOT inherited (sub-workflow has its
	// own step namespace) per the documented semantics.
	parentVars := parentEngine.SnapshotVars()

	concurrency := cfg.GetConcurrency()
	stopOnError := cfg.GetStopOnError()

	// 4. Per-item execution. The sub-workflow runs sequentially (its own
	// steps in order); items themselves may run concurrently up to
	// `concurrency`. Because each iteration has its own engine, parallel
	// iterations are safe.
	type iterResult struct {
		output string
		err    error
		dur    time.Duration
	}

	runOne := func(idx int, item string) iterResult {
		start := time.Now()
		defer func() {
			if r := recover(); r != nil {
				logger.Error("map iteration panicked",
					"index", idx, "panic", r, "stack", string(debug.Stack()))
			}
		}()

		// Build a fresh engine for this iteration.
		iterEngine := NewExpressionEngine()
		for k, v := range parentVars {
			iterEngine.SetVariable(k, v)
		}
		iterEngine.SetLoopVars(item, idx, len(items))

		// Run the sub-workflow steps sequentially.
		data := item
		var iterErr error
		for _, subStep := range cfg.Steps {
			// Sub-steps may themselves be compound (if/loop/parallel/map);
			// delegate to the same helpers used by the top-level executor.
			subResults, out, sErr := executeSubStep(ctx, stepIndex*MaxParallel+idx, subStep, data, iterEngine, reg, program, globalLimiter)
			_ = subResults // sub-step results are flattened into the parent map result
			if sErr != nil {
				iterErr = sErr
				break
			}
			data = out
		}
		iterEngine.ClearLoopVars()

		if iterErr != nil {
			return iterResult{err: iterErr, dur: time.Since(start)}
		}
		return iterResult{output: data, dur: time.Since(start)}
	}

	var results []StepResult
	outputs := make([]string, len(items)) // preserve order even with failures

	if concurrency <= 1 {
		// ── Sequential ──
		for idx, item := range items {
			if program != nil {
				program.Send(tui.StepStartMsg{Index: stepIndex*MaxParallel + idx, Name: "map[" + fmt.Sprintf("%d", idx) + "]"})
			}
			r := runOne(idx, item)
			sr := StepResult{
				StepIndex: stepIndex*MaxParallel + idx,
				NodeName:  "map",
				Input:     item,
				Output:    r.output,
				Error:     r.err,
				Duration:  r.dur,
			}
			results = append(results, sr)
			if program != nil {
				program.Send(tui.StepEndMsg{Index: stepIndex*MaxParallel + idx, Name: "map", Output: r.output, Error: r.err, Duration: r.dur})
			}
			if r.err != nil {
				logger.Error("map iteration failed", "index", idx, "error", nodes.RedactSensitive(r.err.Error()))
				if stopOnError {
					return results, "", fmt.Errorf("map iteration %d failed: %w", idx+1, r.err)
				}
			} else {
				outputs[idx] = r.output
			}
		}
	} else {
		// ── Concurrent with bounded backpressure pool ──
		// Instead of spawning one goroutine per item (which can create
		// 10,000 goroutines for a 10,000-item map, all blocked on the
		// semaphore), we use a fixed-size worker pool fed by a bounded
		// channel. This limits live goroutines to concurrency and provides
		// backpressure to the producer when the queue is full.
		pool := newBackpressurePool(cfg.GetQueueSize(), cfg.GetBackpressure())
		iterResults := make([]iterResult, len(items))

		// Start fixed-size worker pool. Workers exit when the queue is
		// closed (after all items are submitted or skipped).
		var workerWg sync.WaitGroup
		for range concurrency {
			workerWg.Add(1)
			go func() {
				defer workerWg.Done()
				for mi := range pool.queue {
					idx, item := mi.idx, mi.item
					if globalLimiter != nil {
						if err := globalLimiter.Acquire(ctx); err != nil {
							iterResults[idx] = iterResult{err: err}
							continue
						}
					}
					if program != nil {
						program.Send(tui.StepStartMsg{Index: stepIndex*MaxParallel + idx, Name: "map[" + fmt.Sprintf("%d", idx) + "]"})
					}
					iterResults[idx] = runOne(idx, item)
					if globalLimiter != nil {
						globalLimiter.Release()
					}
				}
			}()
		}

		// Submit items. In "block" mode the producer blocks when the
		// queue is full (backpressure); in "drop" mode the item is
		// skipped with an empty result.
		var drops int64
		for idx, item := range items {
			if !pool.submit(mapItem{idx, item}) {
				drops++
				iterResults[idx] = iterResult{} // skipped
			}
		}
		pool.close()
		workerWg.Wait()

		if drops > 0 {
			logger.Warn("map dropped items due to backpressure", "dropped", drops, "total", len(items))
		}

		// Process in order.
		for idx, r := range iterResults {
			sr := StepResult{
				StepIndex: stepIndex*MaxParallel + idx,
				NodeName:  "map",
				Input:     items[idx],
				Output:    r.output,
				Error:     r.err,
				Duration:  r.dur,
			}
			results = append(results, sr)
			if program != nil {
				program.Send(tui.StepEndMsg{Index: stepIndex*MaxParallel + idx, Name: "map", Output: r.output, Error: r.err, Duration: r.dur})
			}
			if r.err != nil {
				logger.Error("map iteration failed", "index", idx, "error", nodes.RedactSensitive(r.err.Error()))
				if stopOnError {
					return results, "", fmt.Errorf("map iteration %d failed: %w", idx+1, r.err)
				}
			} else {
				outputs[idx] = r.output
			}
		}
	}

	// 5. Combine outputs. Default for map is json_array (structured),
	// not the loop's "\n---\n" join, because map is meant for structured
	// batch processing. Other strategies apply via applyOutputStrategy.
	joined := strings.Join(outputs, "\n---\n")
	strategy := wStep.OutputStrategy
	if strategy == "" {
		strategy = "json_array"
	}
	finalOutput := applyOutputStrategy(joined, strategy)
	return results, finalOutput, nil
}

// resolveMapItems turns the evaluated `over` string into a slice of item
// strings. If the string parses as a JSON array, each element is
// serialized back to a string (objects/arrays stay as compact JSON,
// scalars become their bare representation). Otherwise the string is
// split by the delimiter (default newline) and empties are dropped.
func resolveMapItems(over, splitBy string) ([]string, error) {
	over = strings.TrimSpace(over)
	if over == "" {
		return nil, nil
	}

	// Try JSON array first.
	if strings.HasPrefix(over, "[") {
		var raw []json.RawMessage
		if err := json.Unmarshal([]byte(over), &raw); err == nil {
			items := make([]string, 0, len(raw))
			for _, r := range raw {
				// Unquote JSON strings; keep objects/arrays/numbers as-is.
				var s string
				if err := json.Unmarshal(r, &s); err == nil {
					items = append(items, s)
				} else {
					items = append(items, string(r))
				}
			}
			return items, nil
		}
		// Fall through to split on malformed "[" input.
	}

	// String split.
	var items []string
	for _, part := range strings.Split(over, splitBy) {
		if strings.TrimSpace(part) != "" {
			items = append(items, part)
		}
	}
	return items, nil
}

// executeSubStep runs a single sub-workflow step inside a map iteration.
// It mirrors the top-level executor's dispatch but operates on the
// iteration's engine and returns a flat result slice + final output.
func executeSubStep(ctx context.Context, baseIdx int, subStep WorkflowStep, input string, engine *ExpressionEngine, reg *nodes.Registry, program *tea.Program, globalLimiter *ConcurrencyLimiter) ([]StepResult, string, error) {
	// Compound steps delegate to the existing executors.
	if subStep.IsIf() {
		_, subSpan := telemetry.StartSubStepSpan(ctx, subStep.Name, "if", baseIdx)
		results, out, err := executeIfBranch(ctx, baseIdx, subStep.If, input, engine, reg, program, globalLimiter)
		telemetry.SubStepSpanEnd(subSpan, err, 0, len(out))
		return results, out, err
	}
	if subStep.IsLoop() {
		_, subSpan := telemetry.StartSubStepSpan(ctx, subStep.Name, subStep.Node, baseIdx)
		results, out, err := executeLoopStep(ctx, baseIdx, subStep, input, engine, reg, program, globalLimiter)
		telemetry.SubStepSpanEnd(subSpan, err, 0, len(out))
		return results, out, err
	}
	if subStep.IsParallel() {
		_, subSpan := telemetry.StartSubStepSpan(ctx, subStep.Name, subStep.Node, baseIdx)
		results, out, err := executeParallelStep(ctx, baseIdx, subStep, input, engine, reg, program, globalLimiter)
		telemetry.SubStepSpanEnd(subSpan, err, 0, len(out))
		return results, out, err
	}
	if subStep.IsMap() {
		_, subSpan := telemetry.StartSubStepSpan(ctx, subStep.Name, "map", baseIdx)
		results, out, err := executeMapStep(ctx, baseIdx, subStep, input, engine, reg, program, globalLimiter)
		telemetry.SubStepSpanEnd(subSpan, err, 0, len(out))
		return results, out, err
	}
	if subStep.IsReduce() {
		_, subSpan := telemetry.StartSubStepSpan(ctx, subStep.Name, "reduce", baseIdx)
		results, out, err := executeReduceStep(ctx, baseIdx, subStep, input, engine, reg, program, globalLimiter)
		telemetry.SubStepSpanEnd(subSpan, err, 0, len(out))
		return results, out, err
	}
	if subStep.IsSaga() {
		_, subSpan := telemetry.StartSubStepSpan(ctx, subStep.Name, "saga", baseIdx)
		results, out, err := executeSagaStep(ctx, baseIdx, subStep, input, engine, reg, program, globalLimiter)
		telemetry.SubStepSpanEnd(subSpan, err, 0, len(out))
		return results, out, err
	}

	// Condition check.
	if subStep.Condition != "" {
		pass, err := evaluateCondition(subStep.Condition, input, engine)
		if err != nil {
			return nil, "", fmt.Errorf("sub-step condition failed: %w", err)
		}
		if !pass {
			return nil, "", nil
		}
	}

	// Plain node execution.
	evaluatedParams, err := engine.EvaluateParams(subStep.Params, input)
	if err != nil {
		return nil, "", fmt.Errorf("sub-step param eval failed: %w", err)
	}
	node, ok := reg.Get(subStep.Node)
	if !ok {
		return nil, "", fmt.Errorf("node '%s' not found in registry", subStep.Node)
	}

	retryCount := subStep.GetRetryCount()
	if retryCount > MaxRetry {
		retryCount = MaxRetry
	}
	stepTimeout := subStep.GetTimeout()
	if stepTimeout > MaxStepTimeout {
		stepTimeout = MaxStepTimeout
	}
	retryDelay := subStep.GetRetryDelay()

	start := time.Now()
	_, subSpan := telemetry.StartSubStepSpan(ctx, subStep.Name, subStep.Node, baseIdx)
	output, execErr := executeWithRetry(ctx, node, input, evaluatedParams, retryCount, retryDelay, stepTimeout)
	subDur := time.Since(start)
	telemetry.SubStepSpanEnd(subSpan, execErr, subDur.Milliseconds(), len(output))
	sr := StepResult{
		StepIndex: baseIdx,
		NodeName:  subStep.Node,
		Input:     input,
		Output:    output,
		Error:     execErr,
		Duration:  subDur,
	}
	if execErr != nil {
		// Error recovery is delegated to applyErrorRecovery (shared with the
		// sequential and DAG executors) so map/reduce sub-workflows support
		// the same four primitives — capture_error, fallback, on_error,
		// continue_on_error — with identical semantics. Previously this was
		// a third copy that silently dropped on_error and could drift from
		// the other paths. traceErr keeps the StepResult error for audit
		// traceability on continue_on_error; abortErr controls whether the
		// iteration fails. (Recoveries are not recorded in the map sub-step
		// trace because sr.Trace is populated by the caller, not here.)
		var abortErr, traceErr error
		_, abortErr, traceErr = applyErrorRecovery(ctx, &subStep, &output, execErr, engine, reg, input, program, globalLimiter, "map sub-step")
		sr.Output = output
		sr.Error = traceErr
		execErr = abortErr
	}
	if execErr != nil {
		return []StepResult{sr}, "", fmt.Errorf("sub-step node '%s' failed: %w", subStep.Node, execErr)
	}
	engine.SetStepOutput(baseIdx, subStep.Name, output)
	return []StepResult{sr}, output, nil
}
