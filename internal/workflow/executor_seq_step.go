// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌‌‌‌‌​​‌‌​​‌​‌‌​​‌​‌​​​​​‌‌​​​‌​‌​​​‌​​‌‌‌‌​​‌‌​‌​‌‌‌​‌​​​​‌​‌​​​​​​​​​​​​​​​​​‌‌​​​‌​​‌​‌​​​​⁠
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​​‌​​‌​​​​‌​‌‌‌​‌‌‌‌​‌​​​‌‌‌‌​‌‌‌​​​​​​​​​​​​​‌‌‌​‌​​​​‌​​‌​‌​‌‌‌‌​⁠
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
	"github.com/alib8b8/aflare/internal/metrics"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/telemetry"
	"github.com/alib8b8/aflare/internal/tui"
	"go.opentelemetry.io/otel/trace"
)

// handleCompoundStep dispatches if/loop/map/reduce/parallel/saga compound
// steps. Returns (handled, error). When handled is false the caller should
// fall through to regular step execution.
func (s *seqExecState) handleCompoundStep(i int, wStep WorkflowStep, stepStart time.Time) (bool, error) {
	switch {
	case wStep.IsIf():
		_, compSpan := telemetry.StartCompoundStepSpan(s.otelCtx, wStep.Name, "if", i)
		branchResults, output, err := executeIfBranch(s.timeoutCtx, i, wStep.If, s.data, s.engine, s.reg, s.program, s.globalLimiter)
		compDur := time.Since(stepStart)
		telemetry.StepSpanEnd(compSpan, err, compDur.Milliseconds(), len(output), false)
		if err != nil {
			s.results = append(s.results, branchResults...)
			s.failTUI()
			return true, err
		}
		s.results = append(s.results, branchResults...)
		s.data = output
		s.engine.SetStepOutput(i, wStep.Name, output)
		s.trace.recordStep(compoundStepTrace(i, wStep.Node, wStep, compDur, len(s.data), len(output)))
		s.saveCP(i)
		return true, nil

	case wStep.IsLoop():
		_, compSpan := telemetry.StartCompoundStepSpan(s.otelCtx, wStep.Name, "loop", i)
		loopResults, output, err := executeLoopStep(s.timeoutCtx, i, wStep, s.data, s.engine, s.reg, s.program, s.globalLimiter)
		compDur := time.Since(stepStart)
		telemetry.StepSpanEnd(compSpan, err, compDur.Milliseconds(), len(output), false)
		if err != nil {
			s.results = append(s.results, loopResults...)
			s.failTUI()
			return true, err
		}
		s.results = append(s.results, loopResults...)
		s.data = applyOutputStrategy(output, wStep.OutputStrategy)
		s.engine.SetStepOutput(i, wStep.Name, output)
		s.trace.recordStep(compoundStepTrace(i, wStep.Node, wStep, compDur, len(s.data), len(output)))
		s.saveCP(i)
		return true, nil

	case wStep.IsMap():
		_, compSpan := telemetry.StartCompoundStepSpan(s.otelCtx, wStep.Name, "map", i)
		mapResults, output, err := executeMapStep(s.timeoutCtx, i, wStep, s.data, s.engine, s.reg, s.program, s.globalLimiter)
		compDur := time.Since(stepStart)
		telemetry.StepSpanEnd(compSpan, err, compDur.Milliseconds(), len(output), false)
		if err != nil {
			s.results = append(s.results, mapResults...)
			s.failTUI()
			return true, err
		}
		s.results = append(s.results, mapResults...)
		s.data = output
		s.engine.SetStepOutput(i, wStep.Name, output)
		s.trace.recordStep(compoundStepTrace(i, "map", wStep, compDur, len(s.data), len(output)))
		s.saveCP(i)
		return true, nil

	case wStep.IsReduce():
		_, compSpan := telemetry.StartCompoundStepSpan(s.otelCtx, wStep.Name, "reduce", i)
		reduceResults, output, err := executeReduceStep(s.timeoutCtx, i, wStep, s.data, s.engine, s.reg, s.program, s.globalLimiter)
		compDur := time.Since(stepStart)
		telemetry.StepSpanEnd(compSpan, err, compDur.Milliseconds(), len(output), false)
		if err != nil {
			s.results = append(s.results, reduceResults...)
			s.failTUI()
			return true, err
		}
		s.results = append(s.results, reduceResults...)
		s.data = output
		s.engine.SetStepOutput(i, wStep.Name, output)
		s.trace.recordStep(compoundStepTrace(i, "reduce", wStep, compDur, len(s.data), len(output)))
		s.saveCP(i)
		return true, nil

	case wStep.IsParallel():
		_, compSpan := telemetry.StartCompoundStepSpan(s.otelCtx, wStep.Name, "parallel", i)
		parallelResults, output, err := executeParallelStep(s.timeoutCtx, i, wStep, s.data, s.engine, s.reg, s.program, s.globalLimiter)
		compDur := time.Since(stepStart)
		telemetry.StepSpanEnd(compSpan, err, compDur.Milliseconds(), len(output), false)
		if err != nil {
			s.results = append(s.results, parallelResults...)
			s.failTUI()
			return true, err
		}
		s.results = append(s.results, parallelResults...)
		s.data = applyOutputStrategy(output, wStep.OutputStrategy)
		s.engine.SetStepOutput(i, wStep.Name, output)
		s.trace.recordStep(compoundStepTrace(i, wStep.Node, wStep, compDur, len(s.data), len(output)))
		s.saveCP(i)
		return true, nil

	case wStep.IsSaga():
		_, compSpan := telemetry.StartCompoundStepSpan(s.otelCtx, wStep.Name, "saga", i)
		sagaResults, output, err := executeSagaStep(s.timeoutCtx, i, wStep, s.data, s.engine, s.reg, s.program, s.globalLimiter)
		compDur := time.Since(stepStart)
		telemetry.StepSpanEnd(compSpan, err, compDur.Milliseconds(), len(output), false)
		if err != nil {
			s.results = append(s.results, sagaResults...)
			s.failTUI()
			return true, err
		}
		s.results = append(s.results, sagaResults...)
		s.data = output
		s.engine.SetStepOutput(i, wStep.Name, output)
		s.trace.recordStep(compoundStepTrace(i, "saga", wStep, compDur, len(s.data), len(output)))
		s.saveCP(i)
		return true, nil

	default:
		return false, nil
	}
}

// failTUI sends a failure message to the TUI program if one is attached.
func (s *seqExecState) failTUI() {
	if s.program != nil {
		s.program.Send(tui.WorkflowEndMsg{Success: false})
	}
}

// compoundStepTrace builds a StepTrace for a compound step.
func compoundStepTrace(i int, nodeName string, wStep WorkflowStep, compDur time.Duration, inputLen, outputLen int) StepTrace {
	return StepTrace{
		Index:           i,
		NodeName:        nodeName,
		StepName:        wStep.Name,
		BatchIndex:      -1,
		ConditionPassed: true,
		Attempts:        1,
		TotalDuration:   compDur,
		InputLen:        inputLen,
		OutputLen:       outputLen,
	}
}

// handleStepCondition evaluates the step's condition expression. Returns
// (handled, error). When handled is true (condition failed or errored), the
// step was fully processed and the caller should continue to the next step.
// When handled is false, the condition passed and the caller should proceed
// to regular step execution.
func (s *seqExecState) handleStepCondition(i int, wStep WorkflowStep, stepStart time.Time) (bool, error) {
	if wStep.Condition == "" {
		return false, nil
	}

	evalStart := time.Now()
	pass, err := evaluateCondition(wStep.Condition, s.data, s.engine)
	evalDuration := time.Since(evalStart)

	if err != nil {
		logger.Error("condition evaluation failed", "index", i, "error", err)
		result := StepResult{
			StepIndex: i, NodeName: wStep.Node, Input: s.data,
			Error: fmt.Errorf("condition evaluation failed: %w", err), Duration: 0,
		}
		result.Trace = s.trace.recordStep(StepTrace{
			Index: i, NodeName: wStep.Node, StepName: wStep.Name, BatchIndex: -1,
			ConditionExpr: wStep.Condition, ConditionPassed: false,
			EvalDuration: evalDuration, TotalDuration: time.Since(stepStart),
			InputLen: len(s.data), ErrorText: err.Error(),
		})
		s.results = append(s.results, result)
		s.sendStepTUI(i, wStep.Node, err, 0)
		s.failTUI()
		return true, err
	}

	if !pass {
		logger.Info("step skipped by condition", "index", i, "node", wStep.Node)
		s.engine.SetStepOutput(i, wStep.Node, "")
		result := StepResult{
			StepIndex: i, NodeName: wStep.Node, Input: s.data, Output: "", Duration: 0,
		}
		result.Trace = s.trace.recordStep(StepTrace{
			Index: i, NodeName: wStep.Node, StepName: wStep.Name, BatchIndex: -1,
			Skipped: true, ConditionExpr: wStep.Condition, ConditionPassed: false,
			EvalDuration: evalDuration, TotalDuration: time.Since(stepStart),
			InputLen: len(s.data),
		})
		s.results = append(s.results, result)
		s.sendStepTUI(i, wStep.Node, "", 0)
		s.emitProgress(StepProgressEvent{
			Index: i, Total: len(s.wf.Steps), NodeName: wStep.Node, StepName: wStep.Name,
			Status: StepProgressSkipped, Duration: time.Since(stepStart),
		})
		return true, nil
	}

	return false, nil
}

// sendStepTUI sends step start/end messages to the TUI program.
func (s *seqExecState) sendStepTUI(i int, nodeName string, output interface{}, duration time.Duration) {
	if s.program == nil {
		return
	}
	s.program.Send(tui.StepStartMsg{Index: i, Name: nodeName})

	var err error
	if e, ok := output.(error); ok {
		err = e
		output = ""
	}
	var outStr string
	if s, ok := output.(string); ok {
		outStr = s
	}
	s.program.Send(tui.StepEndMsg{Index: i, Name: nodeName, Output: outStr, Error: err, Duration: duration})
}

// executeRegularStep runs a single regular (non-compound) workflow step,
// including parameter evaluation, node lookup, retry loop, error recovery,
// trace recording, and TUI updates.
func (s *seqExecState) executeRegularStep(i int, wStep WorkflowStep, stepStart time.Time) error {
	logger.Info("step started", "index", i, "node", wStep.Node)
	s.sendStepStartTUI(i, wStep.Node)
	s.emitProgress(StepProgressEvent{
		Index: i, Total: len(s.wf.Steps), NodeName: wStep.Node, StepName: wStep.Name, Status: StepProgressStarted,
	})

	// Parameter evaluation.
	evalStart := time.Now()
	evaluatedParams, err := s.engine.EvaluateParams(wStep.Params, s.data)
	evalDuration := time.Since(evalStart)
	if err != nil {
		return s.recordStepError(i, wStep, stepStart, evalDuration, fmt.Errorf("expression evaluation failed: %w", err))
	}

	// Node lookup.
	node, ok := s.reg.Get(wStep.Node)
	if !ok {
		err := fmt.Errorf("node '%s' not found in registry", wStep.Node)
		logger.Error("node not found", "node", wStep.Node, "error", err)
		return s.recordStepError(i, wStep, stepStart, evalDuration, err)
	}

	// Retry configuration.
	retryCount := wStep.GetRetryCount()
	if retryCount > MaxRetry {
		retryCount = MaxRetry
	}
	stepTimeout := wStep.GetTimeout()
	if wStep.IsResumable() {
		if rt := wStep.GetResumeTimeout(); rt > 0 {
			stepTimeout = rt
		}
	} else if stepTimeout > MaxStepTimeout {
		stepTimeout = MaxStepTimeout
	}

	stepBaseCtx, llmCollector := withLLMCollector(s.timeoutCtx)
	_, stepSpan := telemetry.StartStepSpan(s.otelCtx, wStep.Name, wStep.Node, i)

	// Bounded preview (pass-by-reference for LLM steps): when the step opts
	// in via preview_input and the incoming payload exceeds PreviewMaxBytes,
	// the node sees a head/tail sample while the full value stays in
	// workflow state for every other step. s.data already carries the
	// step-level `input:` override applied by the sequential loop.
	stepInput := s.data
	if wStep.PreviewInput {
		stepInput = BoundedPreview(stepInput, PreviewMaxBytes)
	}

	// Retry loop.
	var output string
	var execErr error
	var duration time.Duration
	maxAttempts := retryCount + 1
	attemptsMade := 0

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attemptsMade = attempt
		attemptStart := time.Now()

		stepCtx, stepCancel := s.newStepContext(stepBaseCtx, stepTimeout)
		output, execErr = s.executeNode(stepCtx, i, wStep, node, evaluatedParams, stepInput)
		duration = time.Since(attemptStart)
		stepCancel()

		// Typed output contract: validate against the step's declared
		// output_schema (same JSON Schema subset as structured_output). A
		// violation counts as a step failure so it flows through the
		// regular retry/backoff/on_error/capture_error machinery.
		if execErr == nil && wStep.OutputSchema != "" {
			if verr := nodes.ValidateJSONAgainstSchema(output, wStep.OutputSchema); verr != nil {
				execErr = fmt.Errorf("output contract violated: %w", verr)
			}
		}

		if execErr == nil {
			break
		}
		logger.Warn("step failed, retrying", "index", i, "node", wStep.Node, "attempt", attempt, "max", maxAttempts, "error", nodes.RedactSensitive(execErr.Error()))

		if attempt < maxAttempts {
			if timedOut := s.waitRetryDelay(wStep, attempt, stepSpan, duration, len(output)); timedOut {
				return fmt.Errorf("workflow timed out during retry delay")
			}
		}
	}

	// Prometheus node metric: one observation per executed step. The workflow
	// executor dispatches node.Execute directly (not through
	// Registry.ExecuteWithStats), so without this the node latency/failure
	// series are blind to workflow runs. The raw pre-recovery error is used:
	// a node failure masked by fallback/capture_error is still a node
	// failure for ops. Condition-skipped and eval-failed steps never reach
	// this point (no node execution happened).
	metrics.RecordNodeExecution(wStep.Node, duration, execErr)

	// Error recovery.
	resultErr := execErr
	var recoveries []string
	if execErr != nil {
		var abortErr error
		recoveries, abortErr, resultErr = applyErrorRecovery(stepBaseCtx, &wStep, &output, execErr, s.engine, s.reg, s.data, s.program, s.globalLimiter, "step")
		execErr = abortErr
	}

	s.engine.SetStepOutput(i, wStep.Name, output)

	errText := ""
	if resultErr != nil {
		errText = resultErr.Error()
	}
	result := StepResult{
		StepIndex: i, NodeName: wStep.Node, Input: s.data,
		Output: output, Error: resultErr, Duration: duration,
	}
	result.Trace = s.trace.recordStep(StepTrace{
		Index: i, NodeName: wStep.Node, StepName: wStep.Name, BatchIndex: -1,
		ConditionExpr: wStep.Condition, ConditionPassed: true,
		Attempts: attemptsMade, Recoveries: recoveries,
		EvalDuration: evalDuration, ExecuteDuration: duration,
		TotalDuration: time.Since(stepStart),
		InputLen:      len(s.data), OutputLen: len(output), ErrorText: errText,
		LLM:    projectLLMTelemetry(llmCollector.drainCalls()),
		Router: projectRouterDecisions(llmCollector.drainDecisions()),
	})
	s.results = append(s.results, result)

	telemetry.StepSpanEnd(stepSpan, resultErr, duration.Milliseconds(), len(output), false)

	if resultErr != nil {
		logger.Error("step failed", "index", i, "node", wStep.Node, "duration", duration, "error", nodes.RedactSensitive(resultErr.Error()))
	} else {
		logger.Info("step completed", "index", i, "node", wStep.Node, "duration", duration)
	}

	s.sendStepEndTUI(i, wStep.Node, output, resultErr, duration)

	// 断点13: 实时进度回调（完成或失败）。
	if resultErr != nil {
		s.emitProgress(StepProgressEvent{
			Index: i, Total: len(s.wf.Steps), NodeName: wStep.Node, StepName: wStep.Name,
			Status: StepProgressFailed, Duration: duration, Error: resultErr,
		})
	} else {
		s.emitProgress(StepProgressEvent{
			Index: i, Total: len(s.wf.Steps), NodeName: wStep.Node, StepName: wStep.Name,
			Status: StepProgressCompleted, Duration: duration,
		})
	}

	if execErr != nil {
		return s.handleStepFailure(i, wStep, execErr)
	}

	s.data = output
	s.saveCP(i)
	return nil
}

// newStepContext creates a context with the step-level timeout.
func (s *seqExecState) newStepContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(parent, timeout)
	}
	return context.WithCancel(parent)
}

// executeNode dispatches node execution, handling streaming nodes when a TUI
// program is attached.
func (s *seqExecState) executeNode(ctx context.Context, i int, wStep WorkflowStep, node nodes.Node, evaluatedParams map[string]string, stepInput string) (string, error) {
	// Checkpoint scope: when statePath is set, stamp the step's identity
	// into its context so checkpoint-aware nodes (supervisor delegation
	// resume) can persist sub-step progress next to the workflow
	// checkpoint.
	if s.statePath != "" {
		ctx = nodes.WithStepCheckpoint(ctx, nodes.StepCheckpoint{
			StatePath: s.statePath,
			Step:      wStep.Name,
		})
	}
	if s.program != nil {
		if streamingNode, ok := node.(nodes.StreamingNode); ok {
			sink := newStreamSink(s.program, i, wStep.Node)
			defer sink.flush()
			return streamingNode.ExecuteStream(ctx, stepInput, evaluatedParams, sink.onChunk)
		}
	}
	return node.Execute(ctx, stepInput, evaluatedParams)
}

// waitRetryDelay blocks for the retry backoff delay or until the workflow
// context is cancelled.
func (s *seqExecState) waitRetryDelay(wStep WorkflowStep, attempt int, stepSpan trace.Span, duration time.Duration, outputLen int) bool {
	retryDelay := wStep.GetBackoffDelay(attempt)
	select {
	case <-time.After(retryDelay):
		return false
	case <-s.timeoutCtx.Done():
		telemetry.StepSpanEnd(stepSpan, context.DeadlineExceeded, duration.Milliseconds(), outputLen, false)
		return true
	}
}

// sendStepStartTUI sends a step-start message to the TUI program.
func (s *seqExecState) sendStepStartTUI(i int, nodeName string) {
	if s.program != nil {
		s.program.Send(tui.StepStartMsg{Index: i, Name: nodeName})
	}
}

// sendStepEndTUI sends a step-end message to the TUI program.
func (s *seqExecState) sendStepEndTUI(i int, nodeName, output string, resultErr error, duration time.Duration) {
	if s.program == nil {
		return
	}
	s.program.Send(tui.StepEndMsg{
		Index: i, Name: nodeName, Output: output, Error: resultErr, Duration: duration,
	})
}

// emitProgress invokes the CLI progress callback if one is registered (断点13).
// It is safe to call when progressCB is nil (no-op).
func (s *seqExecState) emitProgress(ev StepProgressEvent) {
	if s.progressCB != nil {
		s.progressCB(ev)
	}
}

// recordStepError records a step evaluation or lookup error and returns it.
func (s *seqExecState) recordStepError(i int, wStep WorkflowStep, stepStart time.Time, evalDuration time.Duration, err error) error {
	logger.Error("step error", "index", i, "error", err)
	result := StepResult{
		StepIndex: i, NodeName: wStep.Node, Input: s.data,
		Error: err, Duration: time.Since(stepStart),
	}
	result.Trace = s.trace.recordStep(StepTrace{
		Index: i, NodeName: wStep.Node, StepName: wStep.Name, BatchIndex: -1,
		ConditionPassed: true, EvalDuration: evalDuration,
		TotalDuration: time.Since(stepStart),
		InputLen:      len(s.data), ErrorText: err.Error(),
	})
	s.results = append(s.results, result)
	s.sendStepEndTUI(i, wStep.Node, "", err, time.Since(stepStart))
	s.emitProgress(StepProgressEvent{
		Index: i, Total: len(s.wf.Steps), NodeName: wStep.Node, StepName: wStep.Name,
		Status: StepProgressFailed, Duration: time.Since(stepStart), Error: err,
	})
	s.failTUI()
	return err
}

// handleStepFailure processes a step execution failure, including resumable
// pause and WAL cleanup.
func (s *seqExecState) handleStepFailure(i int, wStep WorkflowStep, execErr error) error {
	if wStep.IsResumable() {
		if s.wal != nil {
			if err := s.wal.Close(); err != nil {
				logger.Error("failed to close WAL during step failure", "err", err)
			}
			s.wal = nil
			s.walState = nil
		}
		if i > 0 {
			s.saveCP(i - 1)
		}
		resumeOn := wStep.ResumeOn
		if resumeOn == "" {
			resumeOn = "manual"
		}
		paused, pauseErr := PauseWorkflow(s.wfPath, s.wf, i, wStep.Name, resumeOn, execErr.Error(), s.walPath, s.safeMode)
		if pauseErr != nil {
			logger.Error("failed to pause workflow", "name", s.wf.Name, "step", i, "error", pauseErr)
			s.failTUI()
			return fmt.Errorf("step %d (%s) failed: %w", i+1, wStep.Node, execErr)
		}
		logger.Info("workflow paused", "name", s.wf.Name, "step", i, "node", wStep.Node, "run_id", paused.RunID, "resume_on", resumeOn)
		s.failTUI()
		return paused
	}
	s.failTUI()
	logger.Error("workflow failed", "name", s.wf.Name, "failed_step", i, "node", wStep.Node, "error", execErr)
	return fmt.Errorf("step %d (%s) failed: %w", i+1, wStep.Node, execErr)
}
