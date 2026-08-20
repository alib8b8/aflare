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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/nodes"
)

// --- 依赖图构建与环检测 ---

func TestBuildDepGraph_NoDeps(t *testing.T) {
	steps := []WorkflowStep{
		{Node: "a"},
		{Node: "b"},
		{Node: "c"},
	}
	g, err := buildDepGraph(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	batches, err := g.topoBatches()
	if err != nil {
		t.Fatalf("topoBatches error: %v", err)
	}
	// 无依赖：所有步骤在同一批，可全部并行
	if len(batches) != 1 {
		t.Errorf("expected 1 batch for independent steps, got %d", len(batches))
	}
	if len(batches[0]) != 3 {
		t.Errorf("expected 3 steps in batch, got %d", len(batches[0]))
	}
}

func TestBuildDepGraph_ChainByName(t *testing.T) {
	steps := []WorkflowStep{
		{Node: "a", Name: "step_a"},
		{Node: "b", Name: "step_b", DependsOn: []string{"step_a"}},
		{Node: "c", Name: "step_c", DependsOn: []string{"step_b"}},
	}
	g, err := buildDepGraph(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	batches, err := g.topoBatches()
	if err != nil {
		t.Fatalf("topoBatches error: %v", err)
	}
	if len(batches) != 3 {
		t.Errorf("expected 3 batches for chain, got %d", len(batches))
	}
	if batches[0][0] != 0 {
		t.Errorf("expected step 0 (a) in batch 0, got %d", batches[0][0])
	}
}

func TestBuildDepGraph_ByIndex(t *testing.T) {
	steps := []WorkflowStep{
		{Node: "a"},
		{Node: "b", DependsOn: []string{"1"}},
	}
	g, err := buildDepGraph(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !g.deps[1][0] {
		t.Error("expected step 1 to depend on step 0")
	}
}

func TestBuildDepGraph_DiamondDeps(t *testing.T) {
	steps := []WorkflowStep{
		{Node: "a", Name: "a"},
		{Node: "b", Name: "b", DependsOn: []string{"a"}},
		{Node: "c", Name: "c", DependsOn: []string{"a"}},
		{Node: "d", Name: "d", DependsOn: []string{"b", "c"}},
	}
	g, err := buildDepGraph(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	batches, err := g.topoBatches()
	if err != nil {
		t.Fatalf("topoBatches error: %v", err)
	}
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches for diamond, got %d", len(batches))
	}
	if len(batches[0]) != 1 || len(batches[1]) != 2 || len(batches[2]) != 1 {
		t.Errorf("batch sizes = %v, want [1,2,1]", []int{len(batches[0]), len(batches[1]), len(batches[2])})
	}
}

func TestBuildDepGraph_CycleDetected(t *testing.T) {
	steps := []WorkflowStep{
		{Node: "a", Name: "a", DependsOn: []string{"c"}},
		{Node: "b", Name: "b", DependsOn: []string{"a"}},
		{Node: "c", Name: "c", DependsOn: []string{"b"}},
	}
	_, err := buildDepGraph(steps)
	if err == nil {
		t.Fatal("expected cycle detection error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle, got: %v", err)
	}
}

// TestBuildDepGraph_CyclePathAfterBacktrack exercises the DFS backtracking
// path where the first dependency subtree is cycle-free and the second
// contains a cycle back to the root. This forces detectCycle's internal
// `path` slice to be reused across backtracks (shared backing array) before
// the cycle is found on the second branch. The test locks in that the
// reported cycle path is correct and not corrupted by stale backing data.
//
// Graph (depends_on semantics — X depends on Y means Y must run first):
//
//	root → {safe, cyc}   (root depends on safe and cyc)
//	safe → {}            (leaf, no cycle)
//	cyc  → {root}        (cycle: root → cyc → root)
//
// DFS visits root, recurses into safe (no cycle, backtracks), then into cyc
// where it finds root on the gray stack. The reported path must be the
// root→cyc→root cycle, not a corrupted root→safe→cyc→root.
func TestBuildDepGraph_CyclePathAfterBacktrack(t *testing.T) {
	steps := []WorkflowStep{
		{Node: "root", Name: "root", DependsOn: []string{"safe", "cyc"}},
		{Node: "safe", Name: "safe"}, // no deps; forces a backtrack before the cycle
		{Node: "cyc", Name: "cyc", DependsOn: []string{"root"}},
	}
	_, err := buildDepGraph(steps)
	if err == nil {
		t.Fatal("expected cycle detection error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "cycle") {
		t.Fatalf("error should mention cycle, got: %v", err)
	}
	// The cycle is root → cyc → root (steps 1 and 3 in 1-based indexing).
	// `safe` (step 2) must NOT appear in the reported cycle path: if it did,
	// the shared backing array would be corrupting the path on backtrack.
	if strings.Contains(msg, "2") {
		t.Errorf("cycle path should not include the non-cyclic step 'safe' (2), got: %s", msg)
	}
	// Both cycle members (root=1, cyc=3) must be present.
	if !strings.Contains(msg, "1") || !strings.Contains(msg, "3") {
		t.Errorf("cycle path should include both root (1) and cyc (3), got: %s", msg)
	}
}

func TestBuildDepGraph_SelfDep(t *testing.T) {
	steps := []WorkflowStep{
		{Node: "a", Name: "a", DependsOn: []string{"a"}},
	}
	_, err := buildDepGraph(steps)
	if err == nil {
		t.Fatal("expected self-dependency error")
	}
}

func TestBuildDepGraph_MissingTarget(t *testing.T) {
	steps := []WorkflowStep{
		{Node: "a", DependsOn: []string{"nonexistent"}},
	}
	_, err := buildDepGraph(steps)
	if err == nil {
		t.Fatal("expected error for missing dependency target")
	}
}

// --- topoBatches 完整性 ---

func TestTopoBatches_PreservesAllSteps(t *testing.T) {
	steps := []WorkflowStep{
		{Node: "a", Name: "a"},
		{Node: "b", Name: "b", DependsOn: []string{"a"}},
		{Node: "c", Name: "c"},
		{Node: "d", Name: "d", DependsOn: []string{"b", "c"}},
		{Node: "e", Name: "e", DependsOn: []string{"a"}},
	}
	g, err := buildDepGraph(steps)
	if err != nil {
		t.Fatal(err)
	}
	batches, err := g.topoBatches()
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[int]bool)
	for _, batch := range batches {
		for _, idx := range batch {
			if seen[idx] {
				t.Errorf("step %d appeared in multiple batches", idx)
			}
			seen[idx] = true
		}
	}
	if len(seen) != len(steps) {
		t.Errorf("expected %d steps across batches, got %d", len(steps), len(seen))
	}
}

// --- hasDAGDeclarations ---

func TestHasDAGDeclarations(t *testing.T) {
	cases := []struct {
		name  string
		steps []WorkflowStep
		want  bool
	}{
		{"no deps", []WorkflowStep{{Node: "a"}, {Node: "b"}}, false},
		{"one dep", []WorkflowStep{{Node: "a"}, {Node: "b", DependsOn: []string{"a"}}}, true},
		{"empty deps slice", []WorkflowStep{{Node: "a", DependsOn: []string{}}}, false},
		{"empty string dep", []WorkflowStep{{Node: "a", DependsOn: []string{""}}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasDAGDeclarations(c.steps); got != c.want {
				t.Errorf("hasDAGDeclarations = %v, want %v", got, c.want)
			}
		})
	}
}

// --- resolveInput ---

func TestResolveInput_NoDeps(t *testing.T) {
	g := &depGraph{nodeCount: 1, deps: []map[int]bool{{}}}
	r := newStepInputResolver()
	if got := g.resolveInput(0, r, "initial"); got != "initial" {
		t.Errorf("expected initial input, got %q", got)
	}
}

func TestResolveInput_SingleDep(t *testing.T) {
	g := &depGraph{nodeCount: 2, deps: []map[int]bool{{}, {0: true}}}
	r := newStepInputResolver()
	r.set(0, "output-of-0")
	if got := g.resolveInput(1, r, "initial"); got != "output-of-0" {
		t.Errorf("expected dep output, got %q", got)
	}
}

func TestResolveInput_MultiDeps(t *testing.T) {
	g := &depGraph{nodeCount: 3, deps: []map[int]bool{{}, {}, {0: true, 1: true}}}
	r := newStepInputResolver()
	r.set(0, "out0")
	r.set(1, "out1")
	got := g.resolveInput(2, r, "initial")
	if want := "out0\n---\nout1"; got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// --- 端到端 DAG 执行 ---

// dagSlowNode 可控延迟节点，用于验证并行加速。
type dagSlowNode struct {
	name      string
	delay     time.Duration
	callCount *int32
}

func (n *dagSlowNode) Name() string        { return n.name }
func (n *dagSlowNode) Description() string { return "slow test node" }
func (n *dagSlowNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Input: "string", Output: "string"}
}
func (n *dagSlowNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	atomic.AddInt32(n.callCount, 1)
	select {
	case <-time.After(n.delay):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return n.name + "-result", nil
}

func TestDAG_ParallelExecutionFaster(t *testing.T) {
	var calls int32
	reg := nodes.NewRegistry()
	reg.Register(&dagSlowNode{name: "slow1", delay: 100 * time.Millisecond, callCount: &calls})
	reg.Register(&dagSlowNode{name: "slow2", delay: 100 * time.Millisecond, callCount: &calls})
	reg.Register(&dagSlowNode{name: "slow3", delay: 100 * time.Millisecond, callCount: &calls})

	wf := &Workflow{
		Name: "dag-parallel-test",
		Steps: []WorkflowStep{
			{Node: "slow1", Name: "s1"},
			{Node: "slow2", Name: "s2"},
			{Node: "slow3", Name: "s3"},
			{Node: "slow1", Name: "s4", DependsOn: []string{"s1", "s2", "s3"}},
		},
	}

	start := time.Now()
	_, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("DAG execution failed: %v", err)
	}
	if len(results) != 4 {
		t.Errorf("expected 4 results, got %d", len(results))
	}
	// 并行：3×100ms 并行 + 100ms 串行 ≈ 200ms；串行需 400ms。阈值 350ms。
	if elapsed > 350*time.Millisecond {
		t.Errorf("DAG too slow (%v), parallel execution may not be working", elapsed)
	}
}

// dagEchoNode 回显 name-result
type dagEchoNode struct{ name string }

func (n *dagEchoNode) Name() string        { return n.name }
func (n *dagEchoNode) Description() string { return "echo" }
func (n *dagEchoNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Input: "string", Output: "string"}
}
func (n *dagEchoNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	return n.name + "-result", nil
}

func TestDAG_SequentialByDefault(t *testing.T) {
	// 无 depends_on → 走原顺序路径（向后兼容验证）
	reg := nodes.NewRegistry()
	reg.Register(&dagEchoNode{name: "n1"})
	reg.Register(&dagEchoNode{name: "n2"})

	wf := &Workflow{
		Name: "sequential-test",
		Steps: []WorkflowStep{
			{Node: "n1"},
			{Node: "n2"},
		},
	}
	_, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("sequential execution failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// 顺序模式下 n2 输入应为 n1 输出
	if results[1].Input != "n1-result" {
		t.Errorf("expected n2 input = n1 output, got %q", results[1].Input)
	}
}

func TestDAG_DataDependencyChain(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&dagEchoNode{name: "echo_a"})
	reg.Register(&dagEchoNode{name: "echo_b"})
	reg.Register(&dagEchoNode{name: "echo_c"})

	wf := &Workflow{
		Name: "dag-chain-test",
		Steps: []WorkflowStep{
			{Node: "echo_a", Name: "a"},
			{Node: "echo_b", Name: "b", DependsOn: []string{"a"}},
			{Node: "echo_c", Name: "c", DependsOn: []string{"b"}},
		},
	}
	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("chain execution failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[1].Input != "echo_a-result" {
		t.Errorf("expected b input = 'echo_a-result', got %q", results[1].Input)
	}
	if output != "echo_c-result" {
		t.Errorf("expected final output 'echo_c-result', got %q", output)
	}
}

func TestDAG_ErrorStopsWorkflow(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&dagErrorNode{name: "failer"})
	reg.Register(&dagEchoNode{name: "after"})

	wf := &Workflow{
		Name: "dag-error-test",
		Steps: []WorkflowStep{
			{Node: "failer", Name: "f"},
			{Node: "after", Name: "a", DependsOn: []string{"f"}},
		},
	}
	_, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Fatal("expected error from failing step")
	}
}

type dagErrorNode struct{ name string }

func (n *dagErrorNode) Name() string        { return n.name }
func (n *dagErrorNode) Description() string { return "always errors" }
func (n *dagErrorNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Input: "string", Output: "string"}
}
func (n *dagErrorNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	return "", errDAGTestFailure
}

var errDAGTestFailure = &dagTestError{}

type dagTestError struct{}

func (e *dagTestError) Error() string { return "intentional DAG test failure" }

// ── P1-6：动态就绪队列调度专项测试 ──
//
// 验证点：
//   1. 关键路径加速：快分支的跨层后续步骤无需等待同层慢步骤（消除批间
//      队头阻塞），makespan 接近关键路径而非静态批次总和。
//   2. 调度确定性：结果按派发顺序（依赖图拓扑序的纯函数）处理，
//      trace/engine 写入顺序与完成时序无关。
//   3. 中止语义：失败后停止派发新步骤，但已派发步骤被收集并记入
//      trace；依赖失败步骤的后续步骤永不执行。
//   4. 大规模：接近 MaxSteps 的步骤数下调度正确（无缺失/重复）。

// dagFailAfterNode 延迟后返回错误，用于中止场景时序控制。
type dagFailAfterNode struct {
	name      string
	delay     time.Duration
	callCount *int32
}

func (n *dagFailAfterNode) Name() string        { return n.name }
func (n *dagFailAfterNode) Description() string { return "delayed failure node" }
func (n *dagFailAfterNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Input: "string", Output: "string"}
}
func (n *dagFailAfterNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	atomic.AddInt32(n.callCount, 1)
	select {
	case <-time.After(n.delay):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return "", errDAGTestFailure
}

// TestDAG_DynamicReadyQueue_CriticalPathSpeedup 构造跨层加速场景：
//
//	静态层级  L0: slow(500ms) | fast(10ms)
//	         L1: after_slow(20ms) | after_fast(200ms)
//
// 静态分批需 500+200=700ms（L1 整批等 after_fast）；动态调度下
// after_fast 在 fast 完成后立即派发（t=10→210 完成），与 slow 并行，
// makespan ≈ 520ms（slow→after_slow 关键路径）。阈值 620ms 区分两者。
func TestDAG_DynamicReadyQueue_CriticalPathSpeedup(t *testing.T) {
	var calls int32
	reg := nodes.NewRegistry()
	reg.Register(&dagSlowNode{name: "slow", delay: 500 * time.Millisecond, callCount: &calls})
	reg.Register(&dagSlowNode{name: "fast", delay: 10 * time.Millisecond, callCount: &calls})
	reg.Register(&dagSlowNode{name: "after_slow", delay: 20 * time.Millisecond, callCount: &calls})
	reg.Register(&dagSlowNode{name: "after_fast", delay: 200 * time.Millisecond, callCount: &calls})

	wf := &Workflow{
		Name: "dag-dynamic-critical-path",
		Steps: []WorkflowStep{
			{Node: "slow", Name: "s_slow"},
			{Node: "fast", Name: "s_fast"},
			{Node: "after_slow", Name: "s_as", DependsOn: []string{"s_slow"}},
			{Node: "after_fast", Name: "s_af", DependsOn: []string{"s_fast"}},
		},
	}

	start := time.Now()
	_, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("DAG execution failed: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	if calls != 4 {
		t.Errorf("expected 4 node calls, got %d", calls)
	}
	// 静态分批 ≈700ms；动态调度 ≈520ms。620ms 阈值排除调度开销波动。
	if elapsed > 620*time.Millisecond {
		t.Errorf("DAG too slow (%v): after_fast likely waited for the slow sibling (batch head-of-line blocking)", elapsed)
	}
}

// TestDAG_DynamicReadyQueue_DispatchOrderDeterministic 完成顺序与派发
// 顺序不同（slow 最晚完成，但其依赖链 fast→s2→s3 早已全部完成），
// 验证 trace 与 results 仍严格按派发顺序记录，且 lastOutput 取派发序
// 最后一个成功步骤（而非最后完成的步骤）。
func TestDAG_DynamicReadyQueue_DispatchOrderDeterministic(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&dagSlowNode{name: "dyn_slow", delay: 200 * time.Millisecond, callCount: new(int32)})
	reg.Register(&dagSlowNode{name: "dyn1", delay: 10 * time.Millisecond, callCount: new(int32)})
	reg.Register(&dagSlowNode{name: "dyn2", delay: 10 * time.Millisecond, callCount: new(int32)})
	reg.Register(&dagSlowNode{name: "dyn3", delay: 10 * time.Millisecond, callCount: new(int32)})

	wf := &Workflow{
		Name: "dag-dynamic-order",
		Steps: []WorkflowStep{
			{Node: "dyn_slow", Name: "s0"}, // 最慢，最后完成
			{Node: "dyn1", Name: "s1"},     // 快
			{Node: "dyn2", Name: "s2", DependsOn: []string{"s1"}},
			{Node: "dyn3", Name: "s3", DependsOn: []string{"s2"}},
		},
	}

	out, results, trace, err := ExecuteWorkflowWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("DAG execution failed: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// 派发序（拓扑序纯函数）：[0, 1, 2, 3]。完成序是 [1, 2, 3, 0]
	// （dyn_slow 最慢），若按完成序处理则顺序会不同——此处验证重排序
	// 缓冲生效。
	wantOrder := []int{0, 1, 2, 3}
	for i, want := range wantOrder {
		if got := results[i].StepIndex; got != want {
			t.Errorf("results[%d].StepIndex = %d, want %d (dispatch order)", i, got, want)
		}
	}
	if len(trace.Steps) != 4 {
		t.Fatalf("expected 4 trace steps, got %d", len(trace.Steps))
	}
	for i, want := range wantOrder {
		if got := trace.Steps[i].Index; got != want {
			t.Errorf("trace.Steps[%d].Index = %d, want %d (dispatch order)", i, got, want)
		}
		if got := trace.Steps[i].NodeName; got == "" {
			t.Errorf("trace.Steps[%d].NodeName empty", i)
		}
	}

	// lastOutput 按处理顺序（= 派发序）取最后成功步骤：s3/dyn3。
	if out != "dyn3-result" {
		t.Errorf("final output = %q, want %q (last dispatched successful step)", out, "dyn3-result")
	}
}

// TestDAG_DynamicReadyQueue_AbortedDispatchedStepsCollected 失败中止场景：
// fastfail(10ms) 失败时 slowok(100ms) 仍在飞行中——已派发步骤必须被
// 收集并记入 trace（保持 trace 完整），依赖失败步骤的 never 步骤
// 永不执行。
func TestDAG_DynamicReadyQueue_AbortedDispatchedStepsCollected(t *testing.T) {
	neverCalls := int32(0)
	reg := nodes.NewRegistry()
	reg.Register(&dagSlowNode{name: "slowok", delay: 100 * time.Millisecond, callCount: new(int32)})
	reg.Register(&dagFailAfterNode{name: "fastfail", delay: 10 * time.Millisecond, callCount: new(int32)})
	reg.Register(&dagSlowNode{name: "never", delay: 1 * time.Millisecond, callCount: &neverCalls})

	wf := &Workflow{
		Name: "dag-dynamic-abort",
		Steps: []WorkflowStep{
			{Node: "slowok", Name: "s0"},
			{Node: "fastfail", Name: "s1"},
			{Node: "never", Name: "s2", DependsOn: []string{"s1"}},
		},
	}

	out, results, trace, err := ExecuteWorkflowWithTrace(context.Background(), wf, reg, nil)
	if err == nil {
		t.Fatal("expected workflow error from failing step")
	}
	if !strings.Contains(err.Error(), "DAG step 1 failed") {
		t.Errorf("error should identify failing step 1, got: %v", err)
	}
	if out != "" {
		t.Errorf("aborted workflow output = %q, want empty", out)
	}

	// slowok 已派发：必须完成并记入 trace（处理序在 fastfail 之前，
	// 因为派发序 [0, 1]）。
	if len(results) != 2 {
		t.Fatalf("expected 2 collected results (slowok + fastfail), got %d", len(results))
	}
	if len(trace.Steps) != 2 {
		t.Fatalf("expected 2 trace steps, got %d", len(trace.Steps))
	}
	if trace.Steps[0].NodeName != "slowok" || trace.Steps[0].ErrorText != "" {
		t.Errorf("trace.Steps[0] should be successful slowok, got %+v", trace.Steps[0])
	}
	if trace.Steps[1].NodeName != "fastfail" || trace.Steps[1].ErrorText == "" {
		t.Errorf("trace.Steps[1] should be failed fastfail with error text, got %+v", trace.Steps[1])
	}
	if neverCalls != 0 {
		t.Errorf("step depending on failed step executed %d times, want 0", neverCalls)
	}
}

// TestDAG_DynamicReadyQueue_ManySteps 接近 MaxSteps 的调度正确性：
// 10 条独立链 × 100 步 = 1000 步（= MaxSteps 上限）。瞬时节点，
// 验证全部步骤恰好执行一次、结果索引无缺失无重复、整体无错误。
func TestDAG_DynamicReadyQueue_ManySteps(t *testing.T) {
	const chains = 10
	const chainLen = 100 // chains * chainLen == MaxSteps
	n := chains * chainLen

	reg := nodes.NewRegistry()
	reg.Register(&dagEchoNode{name: "tick"})
	reg.Register(&dagEchoNode{name: "tock"})

	steps := make([]WorkflowStep, 0, n)
	for c := 0; c < chains; c++ {
		for p := 0; p < chainLen; p++ {
			step := WorkflowStep{Node: "tick", Name: fmt.Sprintf("c%dp%d", c, p)}
			if p > 0 {
				step.DependsOn = []string{fmt.Sprintf("c%dp%d", c, p-1)}
			}
			steps = append(steps, step)
		}
	}

	wf := &Workflow{Name: "dag-dynamic-many-steps", Steps: steps}
	_, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("large DAG execution failed: %v", err)
	}
	if len(results) != n {
		t.Fatalf("expected %d results, got %d", n, len(results))
	}
	seen := make(map[int]bool, n)
	for _, r := range results {
		if r.Error != nil {
			t.Fatalf("unexpected step error: %v", r.Error)
		}
		if seen[r.StepIndex] {
			t.Fatalf("step %d appears twice in results", r.StepIndex)
		}
		seen[r.StepIndex] = true
	}
	for i := 0; i < n; i++ {
		if !seen[i] {
			t.Fatalf("step %d missing from results", i)
		}
	}
}
