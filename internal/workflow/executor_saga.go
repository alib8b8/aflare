// Copyright (c) 2026 llm-box Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
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
	"github.com/alib8b8/llm-box/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// sagaCompletedStep tracks a successfully completed forward step so it can be
// compensated in reverse order on rollback.
type sagaCompletedStep struct {
	idx    int    // position in SagaConfig.Steps
	output string // the forward step's output, passed as input to Compensate
}

// executeSagaStep runs a cross-step saga transaction: forward steps execute in
// order, and on any forward-step failure the already-completed steps are
// compensated in reverse order.
//
// Semantics:
//
//  1. Each SagaStep.Forward runs via executeSubStep, so it inherits the full
//     sub-step machinery — condition, retry, fallback, on_error,
//     capture_error, continue_on_error, and nested if/loop/map/reduce/parallel.
//  2. The output of forward step N becomes the input to forward step N+1,
//     matching the sequential data-flow model. {{step.X}} resolves to saga
//     sub-step outputs; {{var.*}} inherits the parent workflow's vars.
//  3. If a forward step fails (after its own recovery primitives are
//     exhausted), execution stops and COMPENSATION begins: the completed
//     forward steps (1..N-1) have their Compensate step run in REVERSE order.
//     The failed forward step itself is never compensated — it did not
//     complete, so it has no committed side effect to undo.
//  4. A Compensate step receives the corresponding Forward step's output as
//     its input, and {{var.error}} is set to the failure that triggered
//     rollback (so a compensation can branch on the cause). A missing
//     Compensate means the forward step is side-effect-free and needs no
//     rollback.
//  5. Compensation is BEST-EFFORT: a compensating step that itself fails is
//     logged at warn level and skipped, and the rollback of earlier steps
//     continues. This prevents one un-undoable side effect from blocking the
//     cleanup of others. Operators must monitor compensation failures — they
//     indicate state that requires manual reconciliation.
//  6. The saga step returns the error from the failed forward step. The last
//     successful forward step's output is returned as the step output so a
//     caller's capture_error branch can inspect partial progress. If no
//     forward step completed, the output is "".
//
// The forward step's own recovery primitives (retry/fallback/capture_error)
// run BEFORE the saga treats the step as failed — saga compensation is the
// last resort, not the first. This means a forward step with capture_error
// that succeeds in its branch does NOT trigger compensation.
func executeSagaStep(ctx context.Context, stepIndex int, wStep WorkflowStep, input string, parentEngine *ExpressionEngine, reg *nodes.Registry, program *tea.Program, globalLimiter *ConcurrencyLimiter) ([]StepResult, string, error) {
	cfg := wStep.Saga

	// Snapshot parent vars so each forward/compensate sub-step inherits the
	// outer workflow's context (same model as map/reduce/capture_error).
	parentVars := parentEngine.SnapshotVars()

	var results []StepResult
	data := input // flowing input/output across forward steps

	// Track completed forward steps for compensation. We store the index and
	// the forward step's output so the compensating step receives the right
	// input. Only steps that completed successfully are tracked — a step
	// whose recovery failed is the trigger, not a candidate for compensation.
	var completed []sagaCompletedStep

	for idx, sgStep := range cfg.Steps {
		forwardStart := time.Now()

		// Each forward step gets a fresh engine inheriting parent vars, so
		// {{var.*}} resolves consistently and {{step.X}} is scoped to saga
		// sub-steps (set by executeSubStep via SetStepOutput).
		fwdEngine := NewExpressionEngine()
		for k, v := range parentVars {
			fwdEngine.SetVariable(k, v)
		}

		if program != nil {
			program.Send(tui.StepStartMsg{Index: stepIndex*MaxParallel + idx, Name: fmt.Sprintf("saga[%d].forward", idx)})
		}

		subResults, out, fwdErr := executeSubStep(ctx, stepIndex*MaxParallel+idx, sgStep.Forward, data, fwdEngine, reg, program, globalLimiter)
		fwdDur := time.Since(forwardStart)
		results = append(results, subResults...)

		if fwdErr != nil {
			// The forward step failed (its own recovery did not save it).
			// Record the failure and begin compensation.
			logger.Warn("saga forward step failed, starting compensation",
				"saga_step", idx, "error", nodes.RedactSensitive(fwdErr.Error()))
			if program != nil {
				program.Send(tui.StepEndMsg{Index: stepIndex*MaxParallel + idx, Name: "saga.forward", Output: out, Error: fwdErr, Duration: fwdDur})
			}

			compErr := compensateSaga(ctx, stepIndex, completed, cfg.Steps, parentVars, fwdErr.Error(), reg, program, globalLimiter, &results)
			if compErr != nil {
				// Compensation itself should not return an error (it is
				// best-effort), but guard against unexpected panics surfacing.
				logger.Error("saga compensation encountered an unexpected error", "error", compErr)
			}
			// The saga is aborted. Return the last successful forward output
			// (or "" if none) and the triggering forward error so the outer
			// workflow's recovery (capture_error/on_error) can react.
			lastOutput := ""
			if len(completed) > 0 {
				lastOutput = completed[len(completed)-1].output
			}
			return results, lastOutput, fmt.Errorf("saga forward step %d failed: %w", idx, fwdErr)
		}

		// Forward step succeeded: track for compensation and flow its output
		// to the next forward step.
		completed = append(completed, sagaCompletedStep{idx: idx, output: out})
		data = out
		if program != nil {
			program.Send(tui.StepEndMsg{Index: stepIndex*MaxParallel + idx, Name: "saga.forward", Output: out, Error: nil, Duration: fwdDur})
		}
	}

	// All forward steps succeeded: the saga commits. The last forward step's
	// output is the saga output (matching reduce/map semantics where the
	// final sub-step's output is the step output).
	return results, data, nil
}

// compensateSaga runs the Compensate steps of completed forward steps in
// REVERSE order. It is best-effort: a compensating step failure is logged and
// skipped, and earlier steps are still compensated. It appends compensation
// StepResults to *results for audit and returns nil unless an unexpected
// (non-step) error occurs.
//
// The triggering failure message is exposed to each compensating step via
// {{var.error}} so a compensation can branch on the rollback cause (e.g.
// only refund if the failure was a downstream timeout, not a validation
// error). The compensating step's input is the original forward step's
// output, so it can reference the resources that forward step created.
func compensateSaga(ctx context.Context, stepIndex int, completed []sagaCompletedStep, sagaSteps []SagaStep, parentVars map[string]string, triggerErr string, reg *nodes.Registry, program *tea.Program, globalLimiter *ConcurrencyLimiter, results *[]StepResult) error {
	for i := len(completed) - 1; i >= 0; i-- {
		done := completed[i]
		sgStep := sagaSteps[done.idx]
		if sgStep.Compensate == nil {
			// No compensation declared: the forward step was side-effect-free
			// (e.g. a read) or idempotent and safe to leave in place.
			logger.Info("saga compensation skipped (no compensate step)",
				"saga_step", done.idx)
			continue
		}

		compStart := time.Now()
		compEngine := NewExpressionEngine()
		for k, v := range parentVars {
			compEngine.SetVariable(k, v)
		}
		// Expose the triggering failure so the compensation can branch on it.
		compEngine.SetVariable("error", triggerErr)

		if program != nil {
			program.Send(tui.StepStartMsg{Index: stepIndex*MaxParallel + done.idx, Name: fmt.Sprintf("saga[%d].compensate", done.idx)})
		}

		// The compensating step receives the forward step's output as input
		// (the resource it needs to undo), NOT the triggering failure text.
		compSub, compOut, compErr := executeSubStep(ctx, stepIndex*MaxParallel+done.idx, *sgStep.Compensate, done.output, compEngine, reg, program, globalLimiter)
		compDur := time.Since(compStart)
		*results = append(*results, compSub...)

		if compErr != nil {
			// Best-effort: log and continue compensating earlier steps.
			// A compensation failure means manual reconciliation is required
			// for this step's side effect; the operator must be alerted.
			logger.Warn("saga compensation step failed (manual reconciliation required)",
				"saga_step", done.idx, "error", nodes.RedactSensitive(compErr.Error()))
		}
		if program != nil {
			program.Send(tui.StepEndMsg{Index: stepIndex*MaxParallel + done.idx, Name: "saga.compensate", Output: compOut, Error: compErr, Duration: compDur})
		}
	}
	return nil
}
