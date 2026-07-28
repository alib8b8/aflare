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
	"runtime/debug"
	"sync"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/secrets"
	"github.com/alib8b8/llm-box/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

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
func executeWorkflowDAG(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program) (string, []StepResult, *WorkflowTrace, error) {
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

	timeoutCtx, cancel := context.WithTimeout(ctx, WorkflowTimeout)
	defer cancel()

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
		batchStart := time.Now()
		logger.Info("DAG batch started", "batch", batchIdx, "steps", len(batch))

		// ── 阶段 1：主 goroutine 预求值（独占 engine，线程安全）──
		type preparedStep struct {
			idx             int
			wStep           WorkflowStep
			input           string
			evaluatedParams map[string]string
			condPass        bool
			evalErr         error
			skipped         bool // condition 为 false，跳过执行
			evalDuration    time.Duration
		}
		prepared := make([]preparedStep, len(batch))

		for j, stepIdx := range batch {
			wStep := wf.Steps[stepIdx]
			input := graph.resolveInput(stepIdx, resolver, initialInput)

			ps := preparedStep{idx: stepIdx, wStep: wStep, input: input}

			evalStart := time.Now()
			// 条件求值
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
			if !wStep.IsIf() && !wStep.IsLoop() && !wStep.IsParallel() {
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

		// ── 阶段 2：worker pool 并发执行通过 condition 的步骤 ──
		type execResult struct {
			idx      int
			nodeName string
			output   string
			err      error
			duration time.Duration
			attempts int
		}
		resultChan := make(chan execResult, len(prepared))

		var wg sync.WaitGroup
		for _, ps := range prepared {
			// 跳过的步骤和预求值失败的步骤不派发 worker
			if ps.skipped || ps.evalErr != nil {
				continue
			}
			wg.Add(1)
			go func(ps preparedStep) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						logger.Error("DAG step panicked",
							"index", ps.idx,
							"node", ps.wStep.Node,
							"panic", r,
							"stack", string(debug.Stack()),
						)
						resultChan <- execResult{
							idx:      ps.idx,
							nodeName: ps.wStep.Node,
							err:      fmt.Errorf("step panicked: %v", r),
							attempts: 1,
						}
					}
				}()

				if globalLimiter != nil {
					if err := globalLimiter.Acquire(timeoutCtx); err != nil {
						resultChan <- execResult{idx: ps.idx, nodeName: ps.wStep.Node, err: err, attempts: 1}
						return
					}
					defer globalLimiter.Release()
				}

				start := time.Now()

				if program != nil {
					program.Send(tui.StepStartMsg{Index: ps.idx, Name: ps.wStep.Node})
				}

				output, err, attempts := executeDAGStep(timeoutCtx, ps.wStep, ps.input, ps.evaluatedParams, reg)
				resultChan <- execResult{
					idx:      ps.idx,
					nodeName: ps.wStep.Node,
					output:   output,
					err:      err,
					duration: time.Since(start),
					attempts: attempts,
				}
			}(ps)
		}

		// 等待本批全部完成（在单独 goroutine 里关闭 channel）
		go func() {
			wg.Wait()
			close(resultChan)
		}()

		// ── 阶段 3：主 goroutine 收集结果并写回 engine（独占，线程安全）──
		batchOutputs := make(map[int]execResult, len(prepared))
		for res := range resultChan {
			batchOutputs[res.idx] = res
		}

		// 按索引顺序处理本批结果，保证 StepResult 顺序稳定
		var batchFirstErr error
		// 建立 stepIdx → prepared 的映射便于查找
		preparedByIdx := make(map[int]preparedStep, len(prepared))
		for _, ps := range prepared {
			preparedByIdx[ps.idx] = ps
		}

		for _, stepIdx := range batch {
			ps := preparedByIdx[stepIdx]
			wStep := wf.Steps[stepIdx]
			var output string
			var resultErr error
			var duration time.Duration
			var stepInput string = ps.input
			var attempts int
			var recoveries []string
			var skipped bool
			var condPassed bool = true
			var errText string

			if ps.evalErr != nil {
				resultErr = ps.evalErr
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
				duration = res.duration
				attempts = res.attempts

				// 错误恢复：fallback / on_error / continue_on_error
				if resultErr != nil {
					resultErr, recoveries = applyErrorRecovery(timeoutCtx, &wStep, &output, resultErr, engine, reg, stepInput)
					if resultErr != nil {
						errText = resultErr.Error()
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
				if batchFirstErr == nil {
					batchFirstErr = resultErr
				}
			} else {
				logger.Info("DAG step completed", "index", stepIdx, "node", wStep.Node, "duration", duration)
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
		// continue_on_error 已在 applyErrorRecovery 中处理（resultErr 被清零）。
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
func executeDAGStep(ctx context.Context, wStep WorkflowStep, input string, params map[string]string, reg *nodes.Registry) (string, error, int) {
	// 复合步骤：委托给现有执行器（它们内部会自建 engine，与主 engine 隔离）
	if wStep.IsIf() {
		// if 分支用独立 engine 求值条件，子步骤递归执行自建 engine。
		// 注意：DAG 下 if 的 {{step.X}} 引用无法访问主 workflow 步骤输出，
		// 这是已知限制——DAG 模式下复合步骤应通过 input 传递数据。
		subEngine := NewExpressionEngine()
		pass, err := evaluateCondition(wStep.If.Condition, input, subEngine)
		if err != nil {
			return "", fmt.Errorf("if condition failed: %w", err), 1
		}
		var branchSteps []WorkflowStep
		if pass {
			branchSteps = wStep.If.Then
		} else {
			branchSteps = wStep.If.Else
		}
		subWf := &Workflow{Name: fmt.Sprintf("dag-if-branch"), Steps: branchSteps}
		out, _, err := ExecuteWorkflow(ctx, subWf, reg)
		return out, err, 1
	}
	if wStep.IsLoop() {
		// loop 用独立 engine；DAG 下 loop 的 {{step.X}} 引用无法访问主 workflow 步骤输出。
		loopEngine := NewExpressionEngine()
		_, output, err := executeLoopStep(ctx, 0, wStep, input, loopEngine, reg, nil, NewConcurrencyLimiter(0))
		return output, err, 1
	}
	if wStep.IsParallel() {
		// parallel 自带预求值机制，用独立 engine。
		parEngine := NewExpressionEngine()
		_, output, err := executeParallelStep(ctx, 0, wStep, input, parEngine, reg, nil, NewConcurrencyLimiter(0))
		return output, err, 1
	}

	// 普通节点：直接执行
	node, ok := reg.Get(wStep.Node)
	if !ok {
		return "", fmt.Errorf("node '%s' not found in registry", wStep.Node), 1
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
	var output string
	var execErr error
	attemptsMade := 0

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attemptsMade = attempt
		var stepCtx context.Context
		var stepCancel context.CancelFunc
		if stepTimeout > 0 {
			stepCtx, stepCancel = context.WithTimeout(ctx, stepTimeout)
		} else {
			stepCtx, stepCancel = context.WithCancel(ctx)
		}

		output, execErr = node.Execute(stepCtx, input, params)
		stepCancel()

		if execErr == nil {
			break
		}

		if attempt < maxAttempts {
			retryDelay := wStep.GetBackoffDelay(attempt)
			select {
			case <-time.After(retryDelay):
			case <-ctx.Done():
				return "", ctx.Err(), attemptsMade
			}
		}
	}

	return output, execErr, attemptsMade
}

// applyErrorRecovery 处理步骤失败后的 fallback/on_error/continue_on_error。
// 逻辑与 executor.go 中的错误恢复一致，抽取为函数供 DAG 路径复用。
// 返回处理后的 resultErr：恢复成功返回 nil，否则返回原错误。
// 返回的 recoveries 列表记录已应用的恢复动作（用于 trace）。
func applyErrorRecovery(ctx context.Context, wStep *WorkflowStep, output *string, execErr error, engine *ExpressionEngine, reg *nodes.Registry, input string) (error, []string) {
	var recoveries []string

	// 1. fallback 值
	if wStep.Fallback != "" {
		if fallbackVal, ferr := engine.Evaluate(wStep.Fallback, input); ferr == nil {
			logger.Info("DAG step recovered via fallback", "node", wStep.Node)
			*output = fallbackVal
			recoveries = append(recoveries, "fallback")
			return nil, recoveries
		}
	}

	// 2. on_error 处理节点
	if wStep.OnError != nil {
		errStep := *wStep.OnError
		errParams, eerr := engine.EvaluateParams(errStep.Params, input)
		if eerr == nil {
			if errNode, ok := reg.Get(errStep.Node); ok {
				if errOut, err := errNode.Execute(ctx, input, errParams); err == nil {
					logger.Info("DAG step recovered via on_error handler", "node", wStep.Node, "handler", errStep.Node)
					*output = errOut
					recoveries = append(recoveries, "on_error")
					return nil, recoveries
				}
			}
		}
	}

	// 3. continue_on_error：清零错误使工作流继续
	if wStep.ContinueOnError {
		logger.Warn("DAG step failed but continue_on_error set, continuing", "node", wStep.Node, "error", nodes.RedactSensitive(execErr.Error()))
		*output = ""
		recoveries = append(recoveries, "continue_on_error")
		return nil, recoveries
	}

	return execErr, recoveries
}
