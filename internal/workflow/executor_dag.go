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
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/secrets"
	"github.com/alib8b8/aflare/internal/telemetry"
	"github.com/alib8b8/aflare/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// dagPreparedStep holds a step's pre-evaluated condition and params.
type dagPreparedStep struct {
	idx             int
	wStep           WorkflowStep
	input           string
	evaluatedParams map[string]string
	evalErr         error
	skipped         bool // condition 为 false，跳过执行
	evalDuration    time.Duration
}

// dagExecResult captures the outcome of a single DAG step execution.
type dagExecResult struct {
	idx             int
	nodeName        string
	output          string
	err             error
	duration        time.Duration
	attempts        int
	llmCalls        []nodes.LLMCallTelemetry
	routerDecisions []nodes.RouterDecision
}

// prepareDAGBatch pre-evaluates conditions and params for all steps in a batch
// on the main goroutine. ExpressionEngine is NOT thread-safe (it holds
// mutable stepOutputs/variables/loopVars), so this must happen BEFORE
// dispatching node.Execute to worker goroutines.
func prepareDAGBatch(batch []int, wf *Workflow, graph *depGraph, resolver *stepInputResolver, initialInput string, engine *ExpressionEngine) []dagPreparedStep {
	prepared := make([]dagPreparedStep, len(batch))
	for j, stepIdx := range batch {
		wStep := wf.Steps[stepIdx]
		input := graph.resolveInput(stepIdx, resolver, initialInput)
		ps := dagPreparedStep{idx: stepIdx, wStep: wStep, input: input}

		evalStart := time.Now()
		if wStep.Condition != "" {
			pass, err := evaluateCondition(wStep.Condition, input, engine)
			if err != nil {
				ps.evalErr = fmt.Errorf("condition evaluation failed: %w", err)
				ps.evalDuration = time.Since(evalStart)
				prepared[j] = ps
				continue
			}
			if !pass {
				ps.skipped = true
				ps.evalDuration = time.Since(evalStart)
				prepared[j] = ps
				continue
			}
		}

		// 参数求值（复合步骤的 params 不在此求值，由子执行器处理）
		if !wStep.IsIf() && !wStep.IsLoop() && !wStep.IsParallel() && !wStep.IsMap() && !wStep.IsReduce() && !wStep.IsSaga() {
			params, err := engine.EvaluateParams(wStep.Params, input)
			if err != nil {
				ps.evalErr = err
				ps.evalDuration = time.Since(evalStart)
				prepared[j] = ps
				continue
			}
			ps.evaluatedParams = params
		}
		ps.evalDuration = time.Since(evalStart)
		prepared[j] = ps
	}
	return prepared
}

// dispatchDAGBatch spawns worker goroutines for all non-skipped, non-errored
// prepared steps and returns a channel of results (closed when all complete).
func dispatchDAGBatch(otelCtx, timeoutCtx context.Context, prepared []dagPreparedStep, reg *nodes.Registry, program *tea.Program, globalLimiter *ConcurrencyLimiter) <-chan dagExecResult {
	resultChan := make(chan dagExecResult, len(prepared))
	var wg sync.WaitGroup
	for _, ps := range prepared {
		if ps.skipped || ps.evalErr != nil {
			continue
		}
		wg.Add(1)
		go func(ps dagPreparedStep) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logger.Error("DAG step panicked",
						"index", ps.idx,
						"node", ps.wStep.Node,
						"panic", r,
						"stack", string(debug.Stack()),
					)
					resultChan <- dagExecResult{
						idx:      ps.idx,
						nodeName: ps.wStep.Node,
						err:      fmt.Errorf("step panicked: %v", r),
						attempts: 1,
					}
				}
			}()

			if globalLimiter != nil {
				if err := globalLimiter.Acquire(timeoutCtx); err != nil {
					resultChan <- dagExecResult{idx: ps.idx, nodeName: ps.wStep.Node, err: err, attempts: 1}
					return
				}
				defer globalLimiter.Release()
			}

			start := time.Now()
			if program != nil {
				program.Send(tui.StepStartMsg{Index: ps.idx, Name: ps.wStep.Node})
			}

			output, attempts, llmCalls, routerDecisions, err := executeDAGStep(otelCtx, ps.wStep, ps.input, ps.evaluatedParams, reg)
			resultChan <- dagExecResult{
				idx:             ps.idx,
				nodeName:        ps.wStep.Node,
				output:          output,
				err:             err,
				duration:        time.Since(start),
				attempts:        attempts,
				llmCalls:        llmCalls,
				routerDecisions: routerDecisions,
			}
		}(ps)
	}
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	return resultChan
}

// processDAGStepResult processes a single step's result: applies error
// recovery, writes output to engine/resolver, records trace, and appends to
// allResults. Returns updated allResults, lastOutput, and abortErr (non-nil
// only when no recovery applied and the workflow must stop).
func processDAGStepResult(
	stepIdx, batchIdx int,
	ps dagPreparedStep,
	wStep WorkflowStep,
	batchOutputs map[int]dagExecResult,
	timeoutCtx context.Context,
	engine *ExpressionEngine,
	resolver *stepInputResolver,
	reg *nodes.Registry,
	graph *depGraph,
	trace *WorkflowTrace,
	program *tea.Program,
	allResults []StepResult,
	lastOutput string,
) ([]StepResult, string, error) {
	var output string
	// resultErr is the TRACE error: recorded in StepResult/trace and used for
	// logging. For continue_on_error it keeps the original error so the trace
	// honestly reflects the failure.
	var resultErr error
	// abortErr is the ABORT error: non-nil only when no recovery applied and
	// the workflow must stop. Separated from resultErr so continue_on_error
	// can keep resultErr (for trace) while clearing abortErr (batch proceeds).
	var abortErr error
	var duration time.Duration
	stepInput := ps.input
	var attempts int
	var recoveries []string
	var skipped bool
	condPassed := true
	var errText string
	var llmCalls []nodes.LLMCallTelemetry
	var routerDecisions []nodes.RouterDecision

	if ps.evalErr != nil {
		resultErr = ps.evalErr
		abortErr = ps.evalErr
		errText = resultErr.Error()
	} else if ps.skipped {
		output = ""
		skipped = true
		condPassed = false
		logger.Info("DAG step skipped by condition", "index", stepIdx, "node", wStep.Node)
	} else {
		res := batchOutputs[stepIdx]
		output = res.output
		resultErr = res.err
		abortErr = res.err
		duration = res.duration
		attempts = res.attempts
		llmCalls = res.llmCalls
		routerDecisions = res.routerDecisions

		// 错误恢复：capture_error / fallback / on_error / continue_on_error.
		// applyErrorRecovery returns abortErr (controls batch stop) separately
		// from traceErr (recorded in StepResult/trace).
		if resultErr != nil {
			var rec []string
			rec, abortErr, resultErr = applyErrorRecovery(timeoutCtx, &wStep, &output, resultErr, engine, reg, stepInput, nil, nil, "DAG step")
			recoveries = rec
			if resultErr != nil {
				errText = resultErr.Error()
			} else {
				errText = ""
			}
		}

		if program != nil {
			program.Send(tui.StepEndMsg{
				Index:    stepIdx,
				Name:     wStep.Node,
				Output:   output,
				Error:    resultErr,
				Duration: duration,
			})
		}
	}

	// 写回 engine 和 resolver（主 goroutine 独占）
	engine.SetStepOutput(stepIdx, wStep.Name, output)
	resolver.set(stepIdx, output)

	// 收集依赖索引（DAG 专属信息），按索引排序保证稳定顺序
	deps := make([]int, 0, len(graph.deps[stepIdx]))
	for d := range graph.deps[stepIdx] {
		deps = append(deps, d)
	}
	for i := 1; i < len(deps); i++ {
		for j := i; j > 0 && deps[j] < deps[j-1]; j-- {
			deps[j], deps[j-1] = deps[j-1], deps[j]
		}
	}

	tracePtr := trace.recordStep(StepTrace{
		Index:           stepIdx,
		NodeName:        wStep.Node,
		StepName:        wStep.Name,
		BatchIndex:      batchIdx,
		Dependencies:    deps,
		Skipped:         skipped,
		ConditionExpr:   wStep.Condition,
		ConditionPassed: condPassed,
		Attempts:        attempts,
		Recoveries:      recoveries,
		EvalDuration:    ps.evalDuration,
		ExecuteDuration: duration,
		TotalDuration:   ps.evalDuration + duration,
		InputLen:        len(stepInput),
		OutputLen:       len(output),
		ErrorText:       errText,
		LLM:             projectLLMTelemetry(llmCalls),
		Router:          projectRouterDecisions(routerDecisions),
	})

	allResults = append(allResults, StepResult{
		StepIndex: stepIdx,
		NodeName:  wStep.Node,
		Input:     stepInput,
		Output:    output,
		Error:     resultErr,
		Duration:  duration,
		Trace:     tracePtr,
	})

	if resultErr == nil && output != "" {
		lastOutput = output
	}

	if resultErr != nil {
		logger.Error("DAG step failed", "index", stepIdx, "node", wStep.Node, "error", nodes.RedactSensitive(resultErr.Error()))
	} else {
		logger.Info("DAG step completed", "index", stepIdx, "node", wStep.Node, "duration", duration)
	}

	return allResults, lastOutput, abortErr
}

// executeWorkflowDAG 在有步骤声明 depends_on 时启用 DAG 调度。
//
// 设计要点：
//   - 主调度 goroutine 独占访问 ExpressionEngine（无锁，与 parallel 实现一致），
//     在派发每批步骤前预求值 condition 和 params。
//   - worker goroutine 只执行 node.Execute，不触碰 engine，避免数据竞争。
//   - 每批内的步骤互不依赖，可安全并发；批次间严格顺序。
//   - 复合步骤（if/loop/parallel）作为整体单元参与调度，执行时调用现有实现。
//   - 100% 向后兼容：无 depends_on 声明的工作流走原顺序 for 循环路径。
//
// 最终输出规则：
//   - 若 wf.Output 有表达式，求值它。
//   - 否则取最后一个完成的步骤输出。
//   - 建议 DAG 模式用 wf.output 显式指定，语义最清晰。
//
// timeout is the overall workflow timeout applied to the derived context.
// Callers that go through an Executor pass e.workflowTimeout; the legacy
// ExecuteWorkflowWithTrace global entry point passes DefaultWorkflowTimeout.
func executeWorkflowDAG(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program, timeout time.Duration) (string, []StepResult, *WorkflowTrace, error) {
	if len(wf.Steps) > MaxSteps {
		return "", nil, nil, fmt.Errorf("workflow has too many steps (%d, max %d)", len(wf.Steps), MaxSteps)
	}
	if err := validateInputSchema(wf); err != nil {
		return "", nil, nil, fmt.Errorf("input validation failed: %w", err)
	}

	// 构建依赖图（含环检测）
	graph, err := buildDepGraph(wf.Steps)
	if err != nil {
		return "", nil, nil, fmt.Errorf("DAG build failed: %w", err)
	}

	// 拓扑分批：每批内的步骤互不依赖，可并发执行
	batches, err := graph.topoBatches()
	if err != nil {
		return "", nil, nil, fmt.Errorf("DAG scheduling failed: %w", err)
	}

	logger.Info("DAG workflow started", "name", wf.Name, "steps", len(wf.Steps), "batches", len(batches))

	trace := newTrace(wf.Name, "dag", time.Now(), len(wf.Steps))
	defer func() { trace.finish(time.Now()) }()

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// OpenTelemetry: create the root workflow span. Every step span will be a
	// child of this span. The span is ended in the deferred function below.
	otelCtx, wfSpan := telemetry.StartWorkflowSpan(timeoutCtx, wf.Name)
	defer func() {
		if wfSpan != nil {
			wfSpan.End()
		}
	}()

	var allResults []StepResult
	engine := NewExpressionEngine()
	resolver := newStepInputResolver()

	// 加载工作流变量
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

	globalLimiter := NewConcurrencyLimiter(wf.MaxConcurrency)
	initialInput := ""
	// Branch sub-workflows (if then/else) seed their initial input via context.
	if v, ok := ctx.Value(ifInputKey).(string); ok {
		initialInput = v
	}

	if program != nil {
		program.Send(tui.WorkflowStartMsg{
			Name:  wf.Name,
			Path:  "",
			Steps: len(wf.Steps),
		})
	}

	// lastOutput 跟踪"最后一个完成的步骤输出"，用于无 wf.Output 时的兜底。
	lastOutput := ""

	for batchIdx, batch := range batches {
		// Graceful shutdown: if a shutdown has been requested, stop starting
		// new batches. Steps in the current batch will complete or be
		// cancelled by their context.
		if IsShuttingDown() {
			logger.Info("shutdown requested, stopping DAG workflow execution", "name", wf.Name, "completed_batches", batchIdx, "total_batches", len(batches))
			break
		}

		batchStart := time.Now()
		logger.Info("DAG batch started", "batch", batchIdx, "steps", len(batch))

		// 阶段 1：主 goroutine 预求值（独占 engine，线程安全）
		prepared := prepareDAGBatch(batch, wf, graph, resolver, initialInput, engine)

		// 阶段 2：worker pool 并发执行通过 condition 的步骤
		resultChan := dispatchDAGBatch(otelCtx, timeoutCtx, prepared, reg, program, globalLimiter)

		// 阶段 3：主 goroutine 收集结果并写回 engine（独占，线程安全）
		batchOutputs := make(map[int]dagExecResult, len(prepared))
		for res := range resultChan {
			batchOutputs[res.idx] = res
		}

		preparedByIdx := make(map[int]dagPreparedStep, len(prepared))
		for _, ps := range prepared {
			preparedByIdx[ps.idx] = ps
		}

		var batchFirstErr error
		for _, stepIdx := range batch {
			ps := preparedByIdx[stepIdx]
			wStep := wf.Steps[stepIdx]
			var abortErr error
			allResults, lastOutput, abortErr = processDAGStepResult(
				stepIdx, batchIdx, ps, wStep, batchOutputs,
				timeoutCtx, engine, resolver, reg, graph, trace, program,
				allResults, lastOutput,
			)
			if abortErr != nil && batchFirstErr == nil {
				batchFirstErr = abortErr
			}
		}

		// 记录本批次 trace
		trace.Batches = append(trace.Batches, BatchTrace{
			Index:       batchIdx,
			StepIndices: append([]int(nil), batch...),
			StartedAt:   batchStart,
			Duration:    time.Since(batchStart),
		})

		// 批次内任一步骤失败则终止整个工作流（与顺序模式语义一致）。
		// continue_on_error 已在 applyErrorRecovery 中处理（abortErr 被清零）。
		if batchFirstErr != nil {
			if program != nil {
				program.Send(tui.WorkflowEndMsg{Success: false})
			}
			return "", allResults, trace, fmt.Errorf("DAG batch %d failed: %w", batchIdx, batchFirstErr)
		}
	}

	if program != nil {
		program.Send(tui.WorkflowEndMsg{Success: true})
	}

	logger.Info("DAG workflow completed", "name", wf.Name, "steps", len(wf.Steps))

	// 最终输出
	finalOutput := lastOutput
	if wf.Output != "" {
		if evaluated, err := engine.Evaluate(wf.Output, lastOutput); err == nil {
			finalOutput = evaluated
		} else {
			logger.Warn("failed to evaluate DAG output expression, using last step output", "error", err)
		}
	}

	return finalOutput, allResults, trace, nil
}

// executeDAGStep 执行单个步骤（普通节点或复合步骤），供 worker goroutine 调用。
// 注意：此函数不触碰 ExpressionEngine，所有求值已在主 goroutine 完成。
// 复合步骤（if/loop/parallel）的子步骤求值在子执行器内进行，与主 engine 隔离。
// 返回值中的 attempts 为实际尝试次数（>=1），用于 trace 记录。
// executeDAGStep executes one step's node (with retries) and returns the
// collected LLM telemetry and router decisions so the caller can attach
// them to StepTrace. The slices are nil when the node published nothing.
func executeDAGStep(ctx context.Context, wStep WorkflowStep, input string, params map[string]string, reg *nodes.Registry) (output string, attemptsMade int, llmCalls []nodes.LLMCallTelemetry, routerDecisions []nodes.RouterDecision, execErr error) {
	stepStart := time.Now()

	// B-2/B-3: per-step collector (LLM calls + router decisions), scoped
	// to this step's calls. Drained via the deferred return so every exit
	// path carries the telemetry gathered so far.
	stepBaseCtx, llmCollector := withLLMCollector(ctx)
	ctx = stepBaseCtx
	defer func() {
		llmCalls = llmCollector.drainCalls()
		routerDecisions = llmCollector.drainDecisions()
	}()

	// OpenTelemetry: create a step span as a child of the workflow span.
	// The span is ended via the deferred function at the end of this block.
	stepCtx, stepSpan := telemetry.StartStepSpan(ctx, wStep.Name, wStep.Node, 0)
	defer func() {
		stepDur := time.Since(stepStart)
		telemetry.StepSpanEnd(stepSpan, execErr, stepDur.Milliseconds(), len(output), false)
	}()
	_ = stepCtx // used below for compound step sub-contexts

	// 复合步骤：委托给现有执行器（它们内部会自建 engine，与主 engine 隔离）
	if wStep.IsIf() {
		// if 分支用独立 engine 求值条件，子步骤递归执行自建 engine。
		// 注意：DAG 下 if 的 {{step.X}} 引用无法访问主 workflow 步骤输出，
		// 这是已知限制——DAG 模式下复合步骤应通过 input 传递数据。
		subEngine := NewExpressionEngine()
		pass, err := evaluateCondition(wStep.If.Condition, input, subEngine)
		if err != nil {
			execErr = fmt.Errorf("if condition failed: %w", err)
			attemptsMade = 1
			return
		}
		var branchSteps []WorkflowStep
		if pass {
			branchSteps = wStep.If.Then
		} else {
			branchSteps = wStep.If.Else
		}
		subWf := &Workflow{Name: fmt.Sprintf("dag-if-branch"), Steps: branchSteps}
		output, _, execErr = ExecuteWorkflow(ctx, subWf, reg)
		attemptsMade = 1
		return
	}
	if wStep.IsLoop() {
		// loop 用独立 engine；DAG 下 loop 的 {{step.X}} 引用无法访问主 workflow 步骤输出。
		loopEngine := NewExpressionEngine()
		_, output, execErr = executeLoopStep(ctx, 0, wStep, input, loopEngine, reg, nil, NewConcurrencyLimiter(0))
		attemptsMade = 1
		return
	}
	if wStep.IsParallel() {
		// parallel 自带预求值机制，用独立 engine。
		parEngine := NewExpressionEngine()
		_, output, execErr = executeParallelStep(ctx, 0, wStep, input, parEngine, reg, nil, NewConcurrencyLimiter(0))
		attemptsMade = 1
		return
	}
	if wStep.IsMap() {
		// map 用独立 engine；{{item}}/{{index}} 在迭代内生效。
		mapEngine := NewExpressionEngine()
		_, output, execErr = executeMapStep(ctx, 0, wStep, input, mapEngine, reg, nil, NewConcurrencyLimiter(0))
		attemptsMade = 1
		return
	}
	if wStep.IsReduce() {
		// reduce 用独立 engine；{{loop.acc}}/{{loop.item}} 在迭代内生效。
		reduceEngine := NewExpressionEngine()
		_, output, execErr = executeReduceStep(ctx, 0, wStep, input, reduceEngine, reg, nil, NewConcurrencyLimiter(0))
		attemptsMade = 1
		return
	}
	if wStep.IsSaga() {
		// saga 用独立 engine；forward/compensate 子步骤在内部自建 engine。
		// DAG 下 saga 的 {{step.X}} 引用无法访问主 workflow 步骤输出，
		// 与其他复合步骤一致，应通过 input 传递数据。
		sagaEngine := NewExpressionEngine()
		_, output, execErr = executeSagaStep(ctx, 0, wStep, input, sagaEngine, reg, nil, NewConcurrencyLimiter(0))
		attemptsMade = 1
		return
	}

	// 普通节点：直接执行
	node, ok := reg.Get(wStep.Node)
	if !ok {
		execErr = fmt.Errorf("node '%s' not found in registry", wStep.Node)
		attemptsMade = 1
		return
	}

	retryCount := wStep.GetRetryCount()
	if retryCount > MaxRetry {
		retryCount = MaxRetry
	}
	stepTimeout := wStep.GetTimeout()
	if stepTimeout > MaxStepTimeout {
		stepTimeout = MaxStepTimeout
	}

	maxAttempts := retryCount + 1

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attemptsMade = attempt
		var stepCtx context.Context
		var stepCancel context.CancelFunc
		if stepTimeout > 0 {
			stepCtx, stepCancel = context.WithTimeout(ctx, stepTimeout)
		} else {
			stepCtx, stepCancel = context.WithCancel(ctx)
		}

		// Use defer via an immediately-invoked closure so stepCancel
		// always runs even if node.Execute panics (the worker's recover
		// would otherwise leave the context and its timer leaked).
		func() {
			defer stepCancel()
			output, execErr = node.Execute(stepCtx, input, params)
		}()

		if execErr == nil {
			break
		}

		if attempt < maxAttempts {
			retryDelay := wStep.GetBackoffDelay(attempt)
			select {
			case <-time.After(retryDelay):
			case <-ctx.Done():
				execErr = ctx.Err()
				return
			}
		}
	}

	return
}

// applyErrorRecovery evaluates a step's recovery primitives in priority
// order — capture_error → fallback → on_error → continue_on_error — and is
// the single implementation shared by the sequential, DAG, and map
// executors. Centralizing it here prevents the three paths from drifting
// (previously the DAG path cleared the trace error on continue_on_error
// while sequential/map kept it, and map lacked on_error support entirely).
//
// Returns:
//   - recoveries: labels of recovery actions applied (for trace)
//   - abortErr:   nil if the workflow should continue past this step;
//     non-nil if no recovery applied and the workflow must abort
//   - traceErr:   the error to record in StepResult/trace. nil only on a
//     full recovery (capture_error/fallback/on_error success);
//     for continue_on_error this is the ORIGINAL error so the
//     trace honestly reflects that the step failed even though
//     the workflow proceeds.
//
// output is updated in place with the recovered output (if any). program
// and globalLimiter are forwarded to executeCaptureErrorBranch so the
// capture_error sub-workflow gets TUI output and concurrency limiting;
// pass nil when running outside a TUI/limiter context. label prefixes log
// messages ("step", "DAG step", "map sub-step") for caller identification.
func applyErrorRecovery(ctx context.Context, wStep *WorkflowStep, output *string, execErr error, engine *ExpressionEngine, reg *nodes.Registry, input string, program *tea.Program, globalLimiter *ConcurrencyLimiter, label string) ([]string, error, error) {
	var recoveries []string

	// 0. capture_error: run the error branch (treats the error as a
	// value/branch condition rather than swallowing it). Checked first
	// because it is the most expressive recovery primitive.
	if wStep.HasCaptureError() {
		branchOut, bErr := executeCaptureErrorBranch(ctx, wStep.CaptureError, execErr.Error(), engine.SnapshotVars(), reg, program, globalLimiter)
		if bErr == nil {
			logger.Info(label+" recovered via capture_error branch", "node", wStep.Node)
			*output = branchOut
			recoveries = append(recoveries, "capture_error")
			return recoveries, nil, nil
		}
		logger.Warn(label+" capture_error branch failed, falling through to other recovery", "node", wStep.Node, "error", nodes.RedactSensitive(bErr.Error()))
	}

	// 1. fallback value
	if wStep.Fallback != "" {
		if fallbackVal, ferr := engine.Evaluate(wStep.Fallback, input); ferr == nil {
			logger.Info(label+" recovered via fallback", "node", wStep.Node)
			*output = fallbackVal
			recoveries = append(recoveries, "fallback")
			return recoveries, nil, nil
		}
	}

	// 2. on_error handler node
	if wStep.OnError != nil {
		errStep := *wStep.OnError
		errParams, eerr := engine.EvaluateParams(errStep.Params, input)
		if eerr == nil {
			if errNode, ok := reg.Get(errStep.Node); ok {
				if errOut, err := errNode.Execute(ctx, input, errParams); err == nil {
					logger.Info(label+" recovered via on_error handler", "node", wStep.Node, "handler", errStep.Node)
					*output = errOut
					recoveries = append(recoveries, "on_error")
					return recoveries, nil, nil
				}
			}
		}
	}

	// 3. continue_on_error: clear the abort error so the workflow
	// continues, but return the original error as traceErr so the
	// StepResult/trace honestly records that this step failed.
	if wStep.ContinueOnError {
		logger.Warn(label+" failed but continue_on_error is set, continuing", "node", wStep.Node, "error", nodes.RedactSensitive(execErr.Error()))
		*output = ""
		recoveries = append(recoveries, "continue_on_error")
		return recoveries, nil, execErr
	}

	return recoveries, execErr, execErr
}
