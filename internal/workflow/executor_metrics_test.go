// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌​​​​​​​​‌‌​​‌​​​​‌‌​‌‌‌​‌​‌‌​‌​​‌​‌​‌‌‌‌‌​‌‌‌​​‌​‌​​​​​​‌​‌​‌‌​​​​​​​​​​​​​​​​​​‌​​‌‌​​‌‌‌‌​​‌​‌⁠
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
	"testing"

	"github.com/alib8b8/aflare/internal/metrics"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// ── Ops observability wiring ──
//
// The workflow executor dispatches node.Execute directly (not via
// Registry.ExecuteWithStats), so node-level Prometheus series only cover
// workflow runs because the executor records them itself at step completion
// (sequential path: after the retry loop; DAG path: the executed branch of
// the result switch). These tests pin that wiring via the default registry.

// gatherMetricValue reads one labelled series from the default Prometheus
// registry. Returns (value, found).
func gatherMetricValue(name string, labels map[string]string) (float64, bool) {
	metrics.Register() // idempotent; needed once per process
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return 0, false
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if !labelsMatch(m.GetLabel(), labels) {
				continue
			}
			switch {
			case m.GetCounter() != nil:
				return m.GetCounter().GetValue(), true
			case m.GetGauge() != nil:
				return m.GetGauge().GetValue(), true
			}
		}
	}
	return 0, false
}

func labelsMatch(got []*dto.LabelPair, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, lp := range got {
		v, ok := want[lp.GetName()]
		if !ok || v != lp.GetValue() {
			return false
		}
	}
	return true
}

// activeRunsProbe is a node that samples aflare_runs_active while it is
// mid-execution, so the test can assert the gauge was non-zero during the
// run (not just restored afterwards).
type activeRunsProbe struct {
	observed float64
	found    bool
}

func (p *activeRunsProbe) Name() string        { return "ops_probe" }
func (p *activeRunsProbe) Description() string { return "samples runs_active mid-run" }
func (p *activeRunsProbe) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: "ops_probe", Input: "string", Output: "string"}
}
func (p *activeRunsProbe) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	p.observed, p.found = gatherMetricValue(metrics.RunsActiveName, nil)
	return "probe-done", nil
}

func TestWorkflowRun_RecordsNodeAndRunMetrics(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "ops_ok", successOn: 1})
	reg.Register(&traceTestNode{name: "ops_fail", successOn: 0})
	probe := &activeRunsProbe{}
	reg.Register(probe)

	wf := &Workflow{
		Name: "ops-metrics-seq",
		Steps: []WorkflowStep{
			{Node: "ops_probe", Name: "probe"},
			{Node: "ops_ok", Name: "ok"},
			{Node: "ops_fail", Name: "fail"},
		},
	}

	beforeActive, _ := gatherMetricValue(metrics.RunsActiveName, nil)
	_, _, _, _ = ExecuteWorkflowWithTrace(context.Background(), wf, reg, nil)
	afterActive, _ := gatherMetricValue(metrics.RunsActiveName, nil)

	// Node execution counters: the probe and ok nodes succeeded, the fail
	// node errored (fresh names, so exactly one observation each).
	if v, ok := gatherMetricValue(metrics.NodeExecutionsName, map[string]string{"node_name": "ops_ok", "status": "success"}); !ok || v != 1 {
		t.Errorf("node_executions{ops_ok,success} = %v (found=%v), want 1", v, ok)
	}
	if v, ok := gatherMetricValue(metrics.NodeExecutionsName, map[string]string{"node_name": "ops_fail", "status": "error"}); !ok || v != 1 {
		t.Errorf("node_executions{ops_fail,error} = %v (found=%v), want 1", v, ok)
	}
	// Failure classification: plain errors.New → "other".
	if v, ok := gatherMetricValue(metrics.NodeFailuresName, map[string]string{"node_name": "ops_fail", "error_class": "other"}); !ok || v != 1 {
		t.Errorf("node_failures{ops_fail,other} = %v (found=%v), want 1", v, ok)
	}
	// No failure recorded for the successful node.
	if v, ok := gatherMetricValue(metrics.NodeFailuresName, map[string]string{"node_name": "ops_ok", "error_class": "other"}); ok && v != 0 {
		t.Errorf("node_failures{ops_ok,...} = %v, want 0/absent", v)
	}

	// runs_active: non-zero while a step was executing, restored on exit.
	if !probe.found || probe.observed <= beforeActive {
		t.Errorf("runs_active during run = %v (found=%v), want > %v", probe.observed, probe.found, beforeActive)
	}
	if afterActive != beforeActive {
		t.Errorf("runs_active after run = %v, want restored to %v", afterActive, beforeActive)
	}
}

func TestWorkflowRun_DAGRecordsNodeMetrics(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "ops_dag_a", successOn: 1})
	reg.Register(&traceTestNode{name: "ops_dag_b", successOn: 1})
	reg.Register(&traceTestNode{name: "ops_dag_c", successOn: 1})

	wf := &Workflow{
		Name: "ops-metrics-dag",
		Steps: []WorkflowStep{
			{Node: "ops_dag_a", Name: "a"},
			{Node: "ops_dag_b", Name: "b", DependsOn: []string{"a"}},
			{Node: "ops_dag_c", Name: "c", DependsOn: []string{"a"}},
		},
	}

	_, _, _, err := ExecuteWorkflowWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("DAG run failed: %v", err)
	}
	for _, node := range []string{"ops_dag_a", "ops_dag_b", "ops_dag_c"} {
		if v, ok := gatherMetricValue(metrics.NodeExecutionsName, map[string]string{"node_name": node, "status": "success"}); !ok || v != 1 {
			t.Errorf("node_executions{%s,success} = %v (found=%v), want 1", node, v, ok)
		}
	}
}

func TestWorkflowRun_SkippedStepsNotRecordedAsNodeExecutions(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&traceTestNode{name: "ops_skip_gate", successOn: 1})
	reg.Register(&traceTestNode{name: "ops_skipped", successOn: 1})

	wf := &Workflow{
		Name: "ops-metrics-skip",
		Vars: map[string]string{"flag": "false"},
		Steps: []WorkflowStep{
			{Node: "ops_skip_gate", Name: "gate"},
			// var.flag is "false" → step is skipped without executing the
			// node, so no node-execution observation may be recorded.
			{Node: "ops_skipped", Name: "cond", Condition: "{{var.flag}}"},
		},
	}

	if _, _, _, err := ExecuteWorkflowWithTrace(context.Background(), wf, reg, nil); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	if v, ok := gatherMetricValue(metrics.NodeExecutionsName, map[string]string{"node_name": "ops_skipped", "status": "success"}); ok && v != 0 {
		t.Errorf("skipped node must not be recorded: node_executions{ops_skipped,success} = %v", v)
	}
}
