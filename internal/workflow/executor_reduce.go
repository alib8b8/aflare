// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌‌‌​‌‌​​‌‌​‌​‌‌​​​​​‌‌‌‌‌‌​​​​‌‌​​‌‌‌​​​‌‌​​‌​​​​​​​​​​​​​​​​​​​​​‌‌​​​‌​‌​​‌‌⁠
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

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// executeReduceStep folds a list into a single accumulated value.
//
// For each item in the list evaluated from ReduceConfig.Over, the sub-workflow
// (ReduceConfig.Steps) runs with:
//   - {{loop.acc}}  = the current accumulator (Initial for the first item)
//   - {{loop.item}} = the current item (also passed as the input to the first
//     sub-step, matching map semantics)
//   - {{loop.index}} / {{loop.count}} = iteration metadata
//
// The sub-workflow's final output becomes the accumulator for the next item.
// Reduce is always sequential: each iteration depends on the previous
// accumulator, so there is no concurrency knob. The final accumulator is the
// step's output.
//
// An empty items list yields the Initial accumulator unchanged. A sub-step
// failure (after its own fallback/capture_error/continue_on_error recovery)
// aborts the whole reduce, since a missing accumulator makes subsequent
// iterations meaningless.
func executeReduceStep(ctx context.Context, stepIndex int, wStep WorkflowStep, input string, parentEngine *ExpressionEngine, reg *nodes.Registry, program *tea.Program, globalLimiter *ConcurrencyLimiter) ([]StepResult, string, error) {
	cfg := wStep.Reduce

	// 1. Evaluate `over` in the caller goroutine (engine is not thread-safe).
	overStr, err := parentEngine.Evaluate(cfg.Over, input)
	if err != nil {
		return nil, "", fmt.Errorf("reduce: failed to evaluate `over`: %w", err)
	}

	// 2. Resolve items (reuse map's resolver: JSON array or split-by).
	items, err := resolveMapItems(overStr, cfg.GetSplitBy())
	if err != nil {
		return nil, "", fmt.Errorf("reduce: failed to resolve items: %w", err)
	}

	maxIter := cfg.GetMaxIterations()
	if len(items) > maxIter {
		return nil, "", fmt.Errorf("reduce: items (%d) exceed max_iterations (%d)", len(items), maxIter)
	}

	// 3. Evaluate the initial accumulator (once, before the first item).
	acc, err := parentEngine.Evaluate(cfg.Initial, input)
	if err != nil {
		return nil, "", fmt.Errorf("reduce: failed to evaluate `initial`: %w", err)
	}

	// Empty list → return the initial accumulator unchanged.
	if len(items) == 0 {
		return nil, acc, nil
	}

	// 4. Snapshot parent vars so each iteration inherits the outer context.
	parentVars := parentEngine.SnapshotVars()

	var results []StepResult

	// 5. Sequential left fold.
	for idx, item := range items {
		iterStart := time.Now()

		// Build a fresh engine for this iteration.
		iterEngine := NewExpressionEngine()
		for k, v := range parentVars {
			iterEngine.SetVariable(k, v)
		}
		iterEngine.SetReduceVars(acc, item, idx, len(items))

		if program != nil {
			program.Send(tui.StepStartMsg{Index: stepIndex*MaxParallel + idx, Name: "reduce[" + fmt.Sprintf("%d", idx) + "]"})
		}

		// Run the sub-workflow sequentially. `data` starts as the item
		// (consistent with map); each step transforms it, and the final
		// value becomes the new accumulator.
		data := item
		var iterErr error
		for _, subStep := range cfg.Steps {
			subResults, out, sErr := executeSubStep(ctx, stepIndex*MaxParallel+idx, subStep, data, iterEngine, reg, program, globalLimiter)
			_ = subResults // sub-step results are flattened into the reduce result
			if sErr != nil {
				iterErr = sErr
				break
			}
			data = out
		}
		iterEngine.ClearLoopVars()
		iterDur := time.Since(iterStart)

		if iterErr != nil {
			sr := StepResult{
				StepIndex: stepIndex*MaxParallel + idx,
				NodeName:  "reduce",
				Input:     item,
				Output:    data,
				Error:     iterErr,
				Duration:  iterDur,
			}
			results = append(results, sr)
			if program != nil {
				program.Send(tui.StepEndMsg{Index: stepIndex*MaxParallel + idx, Name: "reduce", Output: data, Error: iterErr, Duration: iterDur})
			}
			logger.Error("reduce iteration failed", "index", idx, "error", nodes.RedactSensitive(iterErr.Error()))
			return results, "", fmt.Errorf("reduce iteration %d failed: %w", idx+1, iterErr)
		}

		// The sub-workflow's final output is the new accumulator.
		acc = data

		sr := StepResult{
			StepIndex: stepIndex*MaxParallel + idx,
			NodeName:  "reduce",
			Input:     item,
			Output:    acc,
			Error:     nil,
			Duration:  iterDur,
		}
		results = append(results, sr)
		if program != nil {
			program.Send(tui.StepEndMsg{Index: stepIndex*MaxParallel + idx, Name: "reduce", Output: acc, Error: nil, Duration: iterDur})
		}
	}

	return results, acc, nil
}
