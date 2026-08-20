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
	"sort"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/metrics"
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

// prepareDAGStep pre-evaluates ONE step's condition and params on the main
// goroutine. ExpressionEngine is NOT thread-safe (it holds mutable
// stepOutputs/variables/loopVars), so this must happen BEFORE dispatching
// node.Execute to a worker goroutine.
func prepareDAGStep(stepIdx int, wf *Workflow, graph *depGraph, resolver *stepInputResolver, initialInput string, engine *ExpressionEngine) dagPreparedStep {
	wStep := wf.Steps[stepIdx]
	input := graph.resolveInput(stepIdx, resolver, initialInput)
	ps := dagPreparedStep{idx: stepIdx, wStep: wStep, input: input}

	evalStart := time.Now()
	if wStep.Condition != "" {
		pass, err := evaluateCondition(wStep.Condition, input, engine)
		if err != nil {
			ps.evalErr = fmt.Errorf("condition evaluation failed: %w", err)
			ps.evalDuration = time.Since(evalStart)
			return ps
		}
		if !pass {
			ps.skipped = true
			ps.evalDuration = time.Since(evalStart)
			return ps
		}
	}

	// 参数求值（复合步骤的 params 不在此求值，由子执行器处理）
	if !wStep.IsIf() && !wStep.IsLoop() && !wStep.IsParallel() && !wStep.IsMap() && !wStep.IsReduce() && !wStep.IsSaga() {
		params, err := engine.EvaluateParams(wStep.Params, input)
		if err != nil {
			ps.evalErr = err
			ps.evalDuration = time.Since(evalStart)
			return ps
		}
		ps.evaluatedParams = params
	}
	ps.evalDuration = time.Since(evalStart)
	return ps
}

// dispatchDAGStep spawns ONE worker goroutine for a prepared step and sends
// its dagExecResult into resultChan. The channel is owned by the scheduler
// and must be buffered with enough capacity for every dispatched step, so
// workers never block on send. Skipped / eval-errored steps are NOT
// dispatched — their outcome is consumed directly from dagPreparedStep.
func dispatchDAGStep(otelCtx, timeoutCtx context.Context, ps dagPreparedStep, reg *nodes.Registry, program *tea.Program, globalLimiter *ConcurrencyLimiter, resultChan chan<- dagExecResult) {
	go func(ps dagPreparedStep) {
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

// processDAGStepResult processes a single step's result: applies error
// recovery, writes output to engine/resolver, records trace, and appends to
// allResults. levelIdx is the step's static topological level (used as the
// trace BatchIndex). Returns updated allResults, lastOutput, and abortErr
// (non-nil only when no recovery applied and the workflow must stop).
func processDAGStepResult(
	stepIdx, levelIdx int,
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
		BatchIndex:      levelIdx,
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
// 设计要点（P1-6：动态就绪队列调度）：
//   - 主调度 goroutine 独占访问 ExpressionEngine（无锁，与 parallel 实现一致），
//     在派发每个步骤前预求值 condition 和 params。
//   - worker goroutine 只执行 node.Execute，不触碰 engine，避免数据竞争。
//   - 调度采用动态就绪队列而非静态分批：步骤在其全部依赖完成后立即派发，
//     无需等待同"层"的慢步骤完成（消除批间队头阻塞，缩短 makespan 至
//     接近关键路径）。
//   - 派发/处理两层机制：
//       派发层（时序敏感）：成功 worker 结果（或 skipped 步骤）到达时立即
//       写回 engine/resolver 并传播就绪状态（"轻量完成"），下游步骤随之
//       派发——这是消除队头阻塞的关键。失败结果不提前：错误恢复
//       （fallback/on_error 等）需按序求值后才知最终输出与是否继续。
//       处理层（顺序敏感）：错误恢复、trace 记录、lastOutput 严格按派发
//       顺序处理（重排序缓冲）。派发序列是依赖图与各依赖完成时序的
//       函数；对固定工作流与节点时序，trace 顺序、lastOutput 与首个中止
//       错误均确定。处理本身仅做 map 写入与 trace 追加（微秒级），重排序
//       屏障对调度的影响可忽略。
//   - 轻量完成的 engine 写入按完成时序发生（非派发序）。engine 是按
//     stepIdx/name 索引的 map，写入顺序不影响值；唯一可见性差异是
//     "引用未声明依赖（depends_on）的步骤输出"的表达式（condition/
//     params/恢复表达式）——其结果受完成时序影响，属未定义行为，DAG
//     模式下数据依赖应通过 depends_on 声明（与复合步骤的限制一致）。
//   - 任一步骤失败（且无恢复）即停止派发新步骤，但已派发步骤会被收集并
//     记入 trace，随后整个工作流中止（与原批模式语义一致）。
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

	// 静态拓扑层级：仅用于 trace 元数据（BatchIndex / Batches 时间线）与
	// 防御性环校验。实际调度使用动态就绪队列。
	levels, err := graph.topoBatches()
	if err != nil {
		return "", nil, nil, fmt.Errorf("DAG scheduling failed: %w", err)
	}
	n := len(wf.Steps)
	levelOf := make([]int, n)
	for lv, batch := range levels {
		for _, idx := range batch {
			levelOf[idx] = lv
		}
	}

	logger.Info("DAG workflow started", "name", wf.Name, "steps", n, "levels", len(levels))

	trace := newTrace(wf.Name, "dag", time.Now(), n)
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
			Steps: n,
		})
	}

	// lastOutput 跟踪"最后一个完成的步骤输出"，用于无 wf.Output 时的兜底。
	lastOutput := ""

	// ── P1-6 动态就绪队列调度状态 ──
	//
	//   pending[i]     步骤 i 尚未完成的依赖数；降为 0 时入就绪队列
	//   readyQueue     依赖全部就绪、待派发的步骤（FIFO；索引序入队）
	//   dispatchSeq    派发顺序（见函数头注释）
	//   results        结果重排序缓冲：按完成顺序暂存，按派发顺序消费
	//   finalized      已"轻量完成"（提前写回并传播就绪）的步骤集合
	//   levelStart/End 各拓扑层级的首次派发与最后处理时间（trace 时间线）
	pending := make([]int, n)
	for i := 0; i < n; i++ {
		pending[i] = len(graph.deps[i])
	}
	readyQueue := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if pending[i] == 0 {
			readyQueue = append(readyQueue, i)
		}
	}

	resultChan := make(chan dagExecResult, n)
	dispatchSeq := make([]int, 0, n)
	results := make(map[int]dagExecResult, n)
	preparedByStep := make(map[int]dagPreparedStep, n)
	finalized := make(map[int]bool, n)
	dispatchAt := make(map[int]time.Time, n)
	levelStart := make([]time.Time, len(levels))
	levelEnd := make([]time.Time, len(levels))

	// propagateDeps 将步骤 idx 的完成通知其依赖方：pending 减一，降为
	// 0 者按索引序入就绪队列（确定性）。新就绪步骤在回到主循环阶段 1
	// 时立即派发。
	propagateDeps := func(idx int) {
		dependents := make([]int, 0, len(graph.dependents[idx]))
		for d := range graph.dependents[idx] {
			dependents = append(dependents, d)
		}
		sort.Ints(dependents)
		for _, d := range dependents {
			pending[d]--
			if pending[d] == 0 {
				readyQueue = append(readyQueue, d)
			}
		}
	}

	processPos := 0 // dispatchSeq 中下一个待处理的步骤
	aborted := false
	var firstAbortErr error

	for processPos < len(dispatchSeq) || len(readyQueue) > 0 {
		// 阶段 1：派发所有就绪步骤（主 goroutine 预求值 → worker 执行）。
		// 中止或 graceful shutdown 后不再启动新步骤。
		for len(readyQueue) > 0 && !aborted && !IsShuttingDown() {
			idx := readyQueue[0]
			readyQueue = readyQueue[1:]

			ps := prepareDAGStep(idx, wf, graph, resolver, initialInput, engine)
			preparedByStep[idx] = ps

			lv := levelOf[idx]
			if levelStart[lv].IsZero() {
				levelStart[lv] = time.Now()
			}
			dispatchSeq = append(dispatchSeq, idx)
			dispatchAt[idx] = time.Now()

			if ps.evalErr != nil {
				// abort 源：不提前完成，按序处理触发中止。
				continue
			}
			if ps.skipped {
				// skipped：output 恒为空且无恢复路径，立即轻量完成
				// （写回 + 传播），避免其下游被处理序上的慢步骤阻塞。
				engine.SetStepOutput(idx, ps.wStep.Name, "")
				resolver.set(idx, "")
				finalized[idx] = true
				propagateDeps(idx)
				continue
			}
			dispatchDAGStep(otelCtx, timeoutCtx, ps, reg, program, globalLimiter, resultChan)
		}

		if processPos >= len(dispatchSeq) {
			// 已派发步骤全部处理完毕且（中止/停机后）不再派发新步骤。
			break
		}

		// 阶段 2：确保"下一个待处理步骤"的结果可用。结果按完成顺序到达：
		// 先完成的其他步骤暂存 results，此处阻塞等待任一 worker 完成。
		// 跳过/求值失败的步骤没有 worker，无需等待。
		want := dispatchSeq[processPos]
		wantPs := preparedByStep[want]
		if !wantPs.skipped && wantPs.evalErr == nil {
			if _, ok := results[want]; !ok {
				res := <-resultChan
				results[res.idx] = res
				// 成功结果到达即轻量完成：立即写回 engine/resolver 并
				// 传播就绪，下游步骤无需等待本步骤轮到按序处理（消除
				// 处理序队头阻塞的关键）。失败结果不提前——错误恢复
				// 按序求值后才知最终输出与是否继续传播。中止后不再
				// 传播（语义：失败即停止派发新步骤）。
				if res.err == nil && !aborted {
					engine.SetStepOutput(res.idx, wf.Steps[res.idx].Name, res.output)
					resolver.set(res.idx, res.output)
					finalized[res.idx] = true
					propagateDeps(res.idx)
				}
				continue
			}
		}

		// 阶段 3：按派发顺序处理结果（错误恢复、写回 engine/resolver、
		// 记录 trace、更新 lastOutput）。未提前完成的步骤（失败后经恢复
		// 才成功的）在此传播就绪，用的是恢复后的最终输出。
		var abortErr error
		if at, ok := dispatchAt[want]; ok {
			metrics.RecordDAGStepReorderWait(time.Since(at))
		}
		allResults, lastOutput, abortErr = processDAGStepResult(
			want, levelOf[want], wantPs, wf.Steps[want], results,
			timeoutCtx, engine, resolver, reg, graph, trace, program,
			allResults, lastOutput,
		)
		levelEnd[levelOf[want]] = time.Now()
		processPos++
		delete(results, want)

		if abortErr != nil && firstAbortErr == nil {
			firstAbortErr = abortErr
			aborted = true
			// 丢弃未派发步骤；已派发步骤继续收集并处理（保持 trace 完整）。
			readyQueue = readyQueue[:0]
			continue
		}
		if aborted {
			continue
		}

		if !finalized[want] {
			// 失败但恢复成功的步骤：以恢复后输出传播就绪
			// （engine/resolver 已在 processDAGStepResult 中写入）。
			propagateDeps(want)
		}
	}

	if IsShuttingDown() {
		logger.Info("shutdown requested, DAG workflow stopped early", "name", wf.Name,
			"dispatched", len(dispatchSeq), "steps", n)
	}

	// 记录各拓扑层级的 trace 时间线（层级内全部步骤被跳过或未派发时该层
	// 不出现——与中止语义一致）。
	for lv := range levels {
		if levelStart[lv].IsZero() {
			continue
		}
		end := levelEnd[lv]
		if end.IsZero() {
			end = time.Now()
		}
		trace.Batches = append(trace.Batches, BatchTrace{
			Index:       lv,
			StepIndices: append([]int(nil), levels[lv]...),
			StartedAt:   levelStart[lv],
			Duration:    end.Sub(levelStart[lv]),
		})
	}

	// 任一步骤失败（且无恢复）则整个工作流中止（与顺序模式语义一致）。
	// continue_on_error 已在 applyErrorRecovery 中处理（abortErr 被清零）。
	if firstAbortErr != nil {
		if program != nil {
			program.Send(tui.WorkflowEndMsg{Success: false})
		}
		return "", allResults, trace, fmt.Errorf("DAG step %d failed: %w", processPos-1, firstAbortErr)
	}

	if program != nil {
		program.Send(tui.WorkflowEndMsg{Success: true})
	}

	logger.Info("DAG workflow completed", "name", wf.Name, "steps", n)

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
