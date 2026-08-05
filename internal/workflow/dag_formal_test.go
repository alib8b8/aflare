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
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

// This file is the executable companion to dag_tla_spec.tla. It performs
// bounded model-checking of the DAG scheduler's safety and liveness
// invariants over randomly generated dependency graphs.
//
// The TLA+ spec proves the invariants for all graphs; this test verifies
// them empirically on a large sample of random DAGs, catching implementation
// bugs that the spec alone cannot (e.g. an off-by-one in topoBatches).
//
// Invariants checked (mirroring the TLA+ spec):
//   1. SafeBatch      — every step in batch b has ALL deps in earlier batches
//   2. NoDoubleExec   — no step appears in two batches
//   3. AllScheduled   — every step appears in exactly one batch
//   4. AcyclicAccept  — acyclic graphs are accepted (no error)
//   5. CyclicReject   — cyclic graphs are rejected with an error
//   6. BatchOrdering  — batches are topologically ordered (deps before dependents)

// randomDAG generates a random DAG on n nodes. It only adds edges i->j where
// i < j (ensuring acyclicity). Each possible forward edge is included with
// probability edgeProb. Returns the dependency sets as map[int]map[int]bool
// (deps[j] = set of nodes j depends on).
func randomDAG(rng *rand.Rand, n int, edgeProb float64) []map[int]bool {
	deps := make([]map[int]bool, n)
	for i := 0; i < n; i++ {
		deps[i] = make(map[int]bool)
	}
	for j := 1; j < n; j++ {
		for i := 0; i < j; i++ {
			if rng.Float64() < edgeProb {
				deps[j][i] = true
			}
		}
	}
	return deps
}

// randomCyclicGraph generates a graph that contains at least one cycle.
// It builds a random DAG, then creates a guaranteed 2-cycle between nodes 0
// and 1 (0 depends on 1 AND 1 depends on 0), which is always a cycle
// regardless of the other edges.
func randomCyclicGraph(rng *rand.Rand, n int) []map[int]bool {
	deps := randomDAG(rng, n, 0.3)
	if n >= 2 {
		// Create a guaranteed 2-cycle: 0 → 1 and 1 → 0.
		deps[0][1] = true
		deps[1][0] = true
	}
	return deps
}

// depsToSteps converts a dependency map into a []WorkflowStep suitable for
// buildDepGraph. Steps are named "s0".."sN-1" and depend_on uses names.
func depsToSteps(deps []map[int]bool) []WorkflowStep {
	n := len(deps)
	steps := make([]WorkflowStep, n)
	for i := 0; i < n; i++ {
		steps[i] = WorkflowStep{
			Name: fmt.Sprintf("s%d", i),
			Node: "echo",
		}
		for d := range deps[i] {
			steps[i].DependsOn = append(steps[i].DependsOn, fmt.Sprintf("s%d", d))
		}
		sort.Strings(steps[i].DependsOn)
	}
	return steps
}

// checkSafeBatch verifies invariant 1: every step in batch b has all its
// deps in batches 0..b-1.
func checkSafeBatch(t *testing.T, batches [][]int, deps []map[int]bool) {
	t.Helper()
	// Map step -> batch index
	batchOf := make(map[int]int)
	for b, batch := range batches {
		for _, step := range batch {
			batchOf[step] = b
		}
	}
	for b, batch := range batches {
		for _, step := range batch {
			for d := range deps[step] {
				db, ok := batchOf[d]
				if !ok {
					t.Errorf("SafeBatch violated: step %d in batch %d depends on %d, which is not in any batch", step, b, d)
					return
				}
				if db >= b {
					t.Errorf("SafeBatch violated: step %d in batch %d depends on %d in batch %d (not earlier)", step, b, d, db)
					return
				}
			}
		}
	}
}

// checkNoDoubleExec verifies invariant 2: no step appears in two batches.
func checkNoDoubleExec(t *testing.T, batches [][]int, n int) {
	t.Helper()
	seen := make(map[int]int) // step -> count
	for _, batch := range batches {
		for _, step := range batch {
			seen[step]++
		}
	}
	for step, count := range seen {
		if count > 1 {
			t.Errorf("NoDoubleExec violated: step %d appears in %d batches", step, count)
			return
		}
	}
}

// checkAllScheduled verifies invariant 3: every step 0..n-1 appears exactly once.
func checkAllScheduled(t *testing.T, batches [][]int, n int) {
	t.Helper()
	seen := make(map[int]bool)
	total := 0
	for _, batch := range batches {
		for _, step := range batch {
			if step < 0 || step >= n {
				t.Errorf("AllScheduled violated: step %d out of range [0,%d)", step, n)
				return
			}
			if seen[step] {
				t.Errorf("AllScheduled violated: step %d scheduled twice", step)
				return
			}
			seen[step] = true
			total++
		}
	}
	if total != n {
		t.Errorf("AllScheduled violated: %d steps scheduled, expected %d", total, n)
		return
	}
	for i := 0; i < n; i++ {
		if !seen[i] {
			t.Errorf("AllScheduled violated: step %d never scheduled", i)
			return
		}
	}
}

// checkAcyclicAccept verifies invariant 4: acyclic graphs are accepted.
func checkAcyclicAccept(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("AcyclicAccept violated: acyclic graph rejected with error: %v", err)
	}
}

// checkCyclicReject verifies invariant 5: cyclic graphs are rejected.
func checkCyclicReject(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Error("CyclicReject violated: cyclic graph was accepted (expected error)")
	}
}

// TestDAGFormal_SafetyInvariants performs bounded model-checking of the DAG
// scheduler over many randomly generated acyclic graphs. For each graph it
// verifies all four safety invariants.
func TestDAGFormal_SafetyInvariants(t *testing.T) {
	const iterations = 500
	const maxNodes = 20

	rng := rand.New(rand.NewSource(42)) // deterministic for reproducibility

	for iter := 0; iter < iterations; iter++ {
		n := 2 + rng.Intn(maxNodes-1) // 2..maxNodes
		edgeProb := 0.1 + rng.Float64()*0.5

		deps := randomDAG(rng, n, edgeProb)
		steps := depsToSteps(deps)

		graph, err := buildDepGraph(steps)
		if err != nil {
			t.Fatalf("iter %d: buildDepGraph failed on acyclic graph (n=%d): %v", iter, n, err)
		}

		batches, err := graph.topoBatches()
		checkAcyclicAccept(t, err)
		if err != nil {
			continue
		}

		// Re-derive the deps map from the graph to ensure we check against
		// the same structure the scheduler used.
		graphDeps := make([]map[int]bool, n)
		for i := 0; i < n; i++ {
			graphDeps[i] = make(map[int]bool)
			for d := range graph.deps[i] {
				graphDeps[i][d] = true
			}
		}

		checkSafeBatch(t, batches, graphDeps)
		checkNoDoubleExec(t, batches, n)
		checkAllScheduled(t, batches, n)
	}
}

// TestDAGFormal_CycleRejection verifies that cyclic graphs are always rejected.
func TestDAGFormal_CycleRejection(t *testing.T) {
	const iterations = 200
	const maxNodes = 15

	rng := rand.New(rand.NewSource(123))

	for iter := 0; iter < iterations; iter++ {
		n := 2 + rng.Intn(maxNodes-1)
		deps := randomCyclicGraph(rng, n)
		steps := depsToSteps(deps)

		_, err := buildDepGraph(steps)
		checkCyclicReject(t, err)
	}
}

// TestDAGFormal_Liveness verifies that for acyclic graphs, topoBatches
// terminates and schedules ALL nodes (no deadlock). This is the executable
// counterpart to the TLA+ EventuallyDone invariant.
func TestDAGFormal_Liveness(t *testing.T) {
	const iterations = 300
	const maxNodes = 25

	rng := rand.New(rand.NewSource(999))

	for iter := 0; iter < iterations; iter++ {
		n := 1 + rng.Intn(maxNodes)
		edgeProb := rng.Float64() * 0.6

		deps := randomDAG(rng, n, edgeProb)
		steps := depsToSteps(deps)

		graph, err := buildDepGraph(steps)
		if err != nil {
			t.Fatalf("iter %d: acyclic graph rejected: %v", iter, err)
		}

		batches, err := graph.topoBatches()
		if err != nil {
			t.Fatalf("iter %d (n=%d): topoBatches failed on acyclic graph: %v", iter, n, err)
		}

		// Liveness: all n steps must be scheduled.
		totalScheduled := 0
		for _, batch := range batches {
			totalScheduled += len(batch)
		}
		if totalScheduled != n {
			t.Fatalf("iter %d (n=%d): Liveness violated — only %d steps scheduled", iter, n, totalScheduled)
		}
	}
}

// TestDAGFormal_EmptyGraph verifies the degenerate case: zero steps.
func TestDAGFormal_EmptyGraph(t *testing.T) {
	graph, err := buildDepGraph(nil)
	if err != nil {
		t.Fatalf("empty graph rejected: %v", err)
	}
	batches, err := graph.topoBatches()
	if err != nil {
		t.Fatalf("empty graph topoBatches failed: %v", err)
	}
	if len(batches) != 0 {
		t.Errorf("empty graph produced %d batches, expected 0", len(batches))
	}
}

// TestDAGFormal_SingleNode verifies a graph with one node and no deps.
func TestDAGFormal_SingleNode(t *testing.T) {
	steps := []WorkflowStep{{Name: "only", Node: "echo"}}
	graph, err := buildDepGraph(steps)
	if err != nil {
		t.Fatalf("single node graph rejected: %v", err)
	}
	batches, err := graph.topoBatches()
	if err != nil {
		t.Fatalf("single node topoBatches failed: %v", err)
	}
	if len(batches) != 1 || len(batches[0]) != 1 {
		t.Errorf("single node: expected 1 batch of 1 step, got %v", batches)
	}
}

// TestDAGFormal_SelfLoop verifies self-dependencies are rejected.
func TestDAGFormal_SelfLoop(t *testing.T) {
	steps := []WorkflowStep{{Name: "s0", Node: "echo", DependsOn: []string{"s0"}}}
	_, err := buildDepGraph(steps)
	if err == nil {
		t.Error("self-loop accepted, expected error")
	}
}

// TestDAGFormal_DiamondDependency verifies a classic diamond:
//
//	s0 → s1, s0 → s2, s1 → s3, s2 → s3
//
// Expected: batch0={s0}, batch1={s1,s2}, batch2={s3}
func TestDAGFormal_DiamondDependency(t *testing.T) {
	steps := []WorkflowStep{
		{Name: "s0", Node: "echo"},
		{Name: "s1", Node: "echo", DependsOn: []string{"s0"}},
		{Name: "s2", Node: "echo", DependsOn: []string{"s0"}},
		{Name: "s3", Node: "echo", DependsOn: []string{"s1", "s2"}},
	}
	graph, err := buildDepGraph(steps)
	if err != nil {
		t.Fatalf("diamond rejected: %v", err)
	}
	batches, err := graph.topoBatches()
	if err != nil {
		t.Fatalf("diamond topoBatches failed: %v", err)
	}
	if len(batches) != 3 {
		t.Fatalf("diamond: expected 3 batches, got %d: %v", len(batches), batches)
	}
	if len(batches[0]) != 1 || len(batches[1]) != 2 || len(batches[2]) != 1 {
		t.Errorf("diamond: batch sizes = %v, expected [1,2,1]", batchSizes(batches))
	}
}

func batchSizes(batches [][]int) []int {
	sizes := make([]int, len(batches))
	for i, b := range batches {
		sizes[i] = len(b)
	}
	return sizes
}
