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
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// depGraph 是步骤的有向无环依赖图。
// 节点是步骤的 0-based 索引；边 A→B 表示 "B 依赖 A"（A 必须在 B 之前完成）。
type depGraph struct {
	// nodeCount 是图中的步骤数。
	nodeCount int
	// deps[i] 是步骤 i 等待的步骤索引集合（i 的直接前置依赖）。
	deps []map[int]bool
	// dependents[i] 是依赖步骤 i 的步骤索引集合（i 的直接后继）。
	dependents []map[int]bool
}

// buildDepGraph 基于 WorkflowStep 列表构造依赖图。
// 依赖声明通过 DependsOn 字段表达，支持步骤名（Name）或 1-based 索引（字符串数字）。
// 返回的图已校验：引用的目标必须存在；不存在自环或重复边。
func buildDepGraph(steps []WorkflowStep) (*depGraph, error) {
	n := len(steps)
	g := &depGraph{
		nodeCount:  n,
		deps:       make([]map[int]bool, n),
		dependents: make([]map[int]bool, n),
	}
	for i := 0; i < n; i++ {
		g.deps[i] = make(map[int]bool)
		g.dependents[i] = make(map[int]bool)
	}

	// 构建名字→索引 与 索引→名字 的映射，用于解析 DependsOn 中的名称引用。
	nameToIdx := make(map[string]int, n)
	for i, s := range steps {
		if s.Name != "" {
			nameToIdx[s.Name] = i
		}
	}

	for i, s := range steps {
		for _, dep := range s.DependsOn {
			dep = strings.TrimSpace(dep)
			if dep == "" {
				continue
			}
			depIdx, ok := resolveStepRef(dep, nameToIdx, n)
			if !ok {
				return nil, fmt.Errorf("step %d (%q) depends_on %q: target step not found", i+1, s.Name, dep)
			}
			if depIdx == i {
				return nil, fmt.Errorf("step %d (%q) depends_on itself", i+1, s.Name)
			}
			g.deps[i][depIdx] = true
			g.dependents[depIdx][i] = true
		}
	}

	if err := g.detectCycle(); err != nil {
		return nil, err
	}
	return g, nil
}

// resolveStepRef 解析依赖引用为步骤索引。
// 优先按名称匹配；若为纯数字则按 1-based 索引匹配（用户友好，与CLI step编号一致）。
// 注意：depends_on 数字是 1-based，但表达式 {{step.N}} 是 0-based（数组索引）。
// 例如 depends_on: [1] 引用第一步，而 {{step.0}} 引用第一步输出。
func resolveStepRef(ref string, nameToIdx map[string]int, count int) (int, bool) {
	if idx, ok := nameToIdx[ref]; ok {
		return idx, true
	}
	// 数字引用：1-based（用户友好，与CLI step编号一致）
	if num, err := strconv.Atoi(ref); err == nil {
		idx := num - 1
		if idx >= 0 && idx < count {
			return idx, true
		}
	}
	return 0, false
}

// detectCycle 用 DFS 三色标记法检测环。有环则返回环上的一个步骤索引用于报错。
func (g *depGraph) detectCycle() error {
	const (
		white = 0 // 未访问
		gray  = 1 // 访问中（在当前 DFS 栈上）
		black = 2 // 已完成
	)
	color := make([]int, g.nodeCount)

	var dfs func(i int, path []int) ([]int, bool)
	dfs = func(i int, path []int) ([]int, bool) {
		color[i] = gray
		path = append(path, i)
		for dep := range g.deps[i] {
			if color[dep] == gray {
				// 找到环：从 path 中找到环的起点
				start := 0
				for k, v := range path {
					if v == dep {
						start = k
						break
					}
				}
				return append(path[start:], dep), true
			}
			if color[dep] == white {
				if cyc, has := dfs(dep, path); has {
					return cyc, true
				}
			}
		}
		color[i] = black
		return nil, false
	}

	for i := 0; i < g.nodeCount; i++ {
		if color[i] == white {
			if cyc, has := dfs(i, nil); has {
				names := make([]string, len(cyc))
				for k, v := range cyc {
					names[k] = strconv.Itoa(v + 1)
				}
				return fmt.Errorf("dependency cycle detected: %s", strings.Join(names, " → "))
			}
		}
	}
	return nil
}

// topoOrder 返回拓扑排序（Kahn 算法）。
// 同一层（无依赖关系的步骤）可并行执行；返回的是分层后的执行批次。
// 每一批内的步骤互不依赖，可安全并发。
// 返回 (batches, error)：batches[i] 是第 i 批可并行的步骤索引集合。
func (g *depGraph) topoBatches() ([][]int, error) {
	// 计算每个节点的入度（未完成的前置数）
	inDegree := make([]int, g.nodeCount)
	for i := 0; i < g.nodeCount; i++ {
		inDegree[i] = len(g.deps[i])
	}

	var batches [][]int
	processed := 0

	for processed < g.nodeCount {
		// 收集当前可执行的步骤（入度为 0 且尚未处理）
		var batch []int
		for i := 0; i < g.nodeCount; i++ {
			if inDegree[i] == 0 {
				batch = append(batch, i)
				inDegree[i] = -1 // 标记为已处理，避免重复入队
			}
		}

		if len(batch) == 0 {
			// 不应发生：detectCycle 已确保无环。防御性返回。
			return nil, fmt.Errorf("topological sort stalled: unreachable steps remain (possible undetected cycle)")
		}

		batches = append(batches, batch)
		processed += len(batch)

		// 移除这些节点：减少其后继的入度
		for _, i := range batch {
			for dep := range g.dependents[i] {
				inDegree[dep]--
			}
		}
	}

	return batches, nil
}

// hasDAGDeclarations 检查工作流中是否有任何步骤声明了 DependsOn。
// 若无，executor 走原顺序循环路径（100% 向后兼容）。
func hasDAGDeclarations(steps []WorkflowStep) bool {
	for _, s := range steps {
		if len(s.DependsOn) > 0 {
			return true
		}
	}
	return false
}

// stepInputResolver 根据依赖关系解析步骤输入。
// 在顺序模式下，输入是上一步输出；在 DAG 模式下，步骤可能没有"上一步"，
// 输入由其依赖步骤的输出组合而成。
type stepInputResolver struct {
	mu      sync.RWMutex
	outputs map[int]string // 步骤索引 → 输出
}

func newStepInputResolver() *stepInputResolver {
	return &stepInputResolver{outputs: make(map[int]string)}
}

func (r *stepInputResolver) set(idx int, out string) {
	r.mu.Lock()
	r.outputs[idx] = out
	r.mu.Unlock()
}

func (r *stepInputResolver) get(idx int) (string, bool) {
	r.mu.RLock()
	out, ok := r.outputs[idx]
	r.mu.RUnlock()
	return out, ok
}

// resolveInput 为 DAG 中的步骤生成输入字符串。
// 若步骤仅有一个直接依赖，输入即该依赖的输出（保持与顺序模式一致的链式语义）。
// 若有多个依赖，输入为各依赖输出以 "\n---\n" 连接（与 parallel 聚合一致）。
// 若无依赖（如根步骤），输入为初始输入。
func (g *depGraph) resolveInput(idx int, resolver *stepInputResolver, initialInput string) string {
	deps := make([]int, 0, len(g.deps[idx]))
	for d := range g.deps[idx] {
		deps = append(deps, d)
	}
	if len(deps) == 0 {
		return initialInput
	}
	if len(deps) == 1 {
		if out, ok := resolver.get(deps[0]); ok {
			return out
		}
		return initialInput
	}
	// 多依赖：按索引排序保证稳定拼接顺序
	// (依赖图构建后 deps 是 map，遍历顺序随机，需排序确保可重现)
	sortedDeps := make([]int, 0, len(deps))
	sortedDeps = append(sortedDeps, deps...)
	// 简单插入排序（dep 集合通常很小）
	for i := 1; i < len(sortedDeps); i++ {
		for j := i; j > 0 && sortedDeps[j] < sortedDeps[j-1]; j-- {
			sortedDeps[j], sortedDeps[j-1] = sortedDeps[j-1], sortedDeps[j]
		}
	}
	var parts []string
	for _, d := range sortedDeps {
		if out, ok := resolver.get(d); ok {
			parts = append(parts, out)
		}
	}
	if len(parts) == 0 {
		return initialInput
	}
	return strings.Join(parts, "\n---\n")
}
