// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​​‌‌‌‌‌‌‌​‌‌​‌​​​‌‌​​​‌‌​‌‌​‌​‌​‌​​​‌​‌‌​​‌​​​​​​​​​​​​​​​​​​​‌​‌‌​​‌​​​‌​‌​‌‌⁠
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
	"testing"

	"github.com/alib8b8/aflare/internal/nodes"
)

func TestPipeline_DAGResolution(t *testing.T) {
	pe := NewPipelineExecutor(nodes.NewRegistry())

	p := &Pipeline{
		Name: "test",
		Stages: []PipelineStage{
			{Name: "a", Workflow: &Workflow{Name: "a", Steps: []WorkflowStep{}}},
			{Name: "b", Workflow: &Workflow{Name: "b", Steps: []WorkflowStep{}}, DependsOn: []string{"a"}},
			{Name: "c", Workflow: &Workflow{Name: "c", Steps: []WorkflowStep{}}, DependsOn: []string{"a"}},
			{Name: "d", Workflow: &Workflow{Name: "d", Steps: []WorkflowStep{}}, DependsOn: []string{"b", "c"}},
		},
	}

	batches, err := pe.resolveDAG(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: [a] -> [b, c] -> [d]
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches, got %d: %v", len(batches), batches)
	}

	if len(batches[0]) != 1 || batches[0][0] != "a" {
		t.Errorf("batch 0: expected [a], got %v", batches[0])
	}
	if len(batches[1]) != 2 {
		t.Errorf("batch 1: expected 2 stages, got %v", batches[1])
	}
	if len(batches[2]) != 1 || batches[2][0] != "d" {
		t.Errorf("batch 2: expected [d], got %v", batches[2])
	}
}

func TestPipeline_CircularDependency(t *testing.T) {
	pe := NewPipelineExecutor(nodes.NewRegistry())

	p := &Pipeline{
		Name: "test",
		Stages: []PipelineStage{
			{Name: "a", Workflow: &Workflow{Name: "a", Steps: []WorkflowStep{}}, DependsOn: []string{"b"}},
			{Name: "b", Workflow: &Workflow{Name: "b", Steps: []WorkflowStep{}}, DependsOn: []string{"a"}},
		},
	}

	_, err := pe.resolveDAG(p)
	if err == nil {
		t.Fatal("expected circular dependency error")
	}
}

func TestPipeline_DuplicateStageName(t *testing.T) {
	pe := NewPipelineExecutor(nodes.NewRegistry())

	p := &Pipeline{
		Name: "test",
		Stages: []PipelineStage{
			{Name: "a", Workflow: &Workflow{Name: "a", Steps: []WorkflowStep{}}},
			{Name: "a", Workflow: &Workflow{Name: "a2", Steps: []WorkflowStep{}}},
		},
	}

	_, err := pe.Execute(context.Background(), p)
	if err == nil {
		t.Fatal("expected duplicate stage name error")
	}
}

func TestPipeline_UnknownDependency(t *testing.T) {
	pe := NewPipelineExecutor(nodes.NewRegistry())

	p := &Pipeline{
		Name: "test",
		Stages: []PipelineStage{
			{Name: "a", Workflow: &Workflow{Name: "a", Steps: []WorkflowStep{}}, DependsOn: []string{"nonexistent"}},
		},
	}

	_, err := pe.Execute(context.Background(), p)
	if err == nil {
		t.Fatal("expected unknown dependency error")
	}
}

func TestPipeline_SequentialExecution(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testEchoNode{})
	pe := NewPipelineExecutor(reg)

	p := &Pipeline{
		Name: "test",
		Stages: []PipelineStage{
			{
				Name: "stage1",
				Workflow: &Workflow{
					Name: "stage1",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{"message": "hello"}},
					},
				},
			},
			{
				Name:      "stage2",
				DependsOn: []string{"stage1"},
				Workflow: &Workflow{
					Name: "stage2",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{"message": "world"}},
					},
				},
			},
		},
	}

	result, err := pe.Execute(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected pipeline to succeed")
	}

	if sr, ok := result.StageResults["stage1"]; !ok || sr.Error != nil {
		t.Errorf("stage1 failed: %v", sr.Error)
	}
	if sr, ok := result.StageResults["stage2"]; !ok || sr.Error != nil {
		t.Errorf("stage2 failed: %v", sr.Error)
	}
}

func TestPipeline_ParallelExecution(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testEchoNode{})
	pe := NewPipelineExecutor(reg)

	p := &Pipeline{
		Name:           "test",
		MaxConcurrency: 2,
		Stages: []PipelineStage{
			{
				Name: "base",
				Workflow: &Workflow{
					Name: "base",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{"message": "base"}},
					},
				},
			},
			{
				Name:      "parallel_a",
				DependsOn: []string{"base"},
				Workflow: &Workflow{
					Name: "parallel_a",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{"message": "a"}},
					},
				},
			},
			{
				Name:      "parallel_b",
				DependsOn: []string{"base"},
				Workflow: &Workflow{
					Name: "parallel_b",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{"message": "b"}},
					},
				},
			},
		},
	}

	result, err := pe.Execute(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected pipeline to succeed")
	}

	if sr, ok := result.StageResults["parallel_a"]; !ok || sr.Error != nil {
		t.Errorf("parallel_a failed: %v", sr.Error)
	}
	if sr, ok := result.StageResults["parallel_b"]; !ok || sr.Error != nil {
		t.Errorf("parallel_b failed: %v", sr.Error)
	}
}

func TestPipeline_ConditionalSkip(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testEchoNode{})
	pe := NewPipelineExecutor(reg)

	p := &Pipeline{
		Name: "test",
		Stages: []PipelineStage{
			{
				Name: "stage1",
				Workflow: &Workflow{
					Name: "stage1",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{"message": "skip_me"}},
					},
				},
			},
			{
				Name:      "stage2",
				DependsOn: []string{"stage1"},
				Condition: "equals:never",
				Workflow: &Workflow{
					Name: "stage2",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{"message": "should_not_run"}},
					},
				},
			},
		},
	}

	result, err := pe.Execute(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected pipeline to succeed")
	}

	if sr, ok := result.StageResults["stage2"]; !ok {
		t.Error("stage2 result missing")
	} else if !sr.Skipped {
		t.Error("expected stage2 to be skipped")
	}
}

func TestPipeline_OnFailureContinue(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testFailingNode{})
	reg.Register(&testEchoNode{})
	pe := NewPipelineExecutor(reg)

	p := &Pipeline{
		Name: "test",
		Stages: []PipelineStage{
			{
				Name:      "stage1",
				OnFailure: OnFailureContinue,
				Workflow: &Workflow{
					Name: "stage1",
					Steps: []WorkflowStep{
						{Node: "fail", Params: map[string]string{}},
					},
				},
			},
			{
				Name:      "stage2",
				DependsOn: []string{"stage1"},
				Workflow: &Workflow{
					Name: "stage2",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{"message": "still_runs"}},
					},
				},
			},
		},
	}

	result, err := pe.Execute(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected pipeline to succeed despite stage1 failure")
	}

	if sr, ok := result.StageResults["stage1"]; !ok || sr.Error == nil {
		t.Error("expected stage1 to have error")
	}
	if sr, ok := result.StageResults["stage2"]; !ok || sr.Error != nil {
		t.Errorf("expected stage2 to succeed, got: %v", sr.Error)
	}
}

func TestPipeline_NoStages(t *testing.T) {
	pe := NewPipelineExecutor(nodes.NewRegistry())

	p := &Pipeline{
		Name:   "empty",
		Stages: []PipelineStage{},
	}

	result, err := pe.Execute(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected empty pipeline to succeed")
	}
}

func TestPipeline_DataPassing(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testEchoNode{})
	pe := NewPipelineExecutor(reg)

	p := &Pipeline{
		Name: "test",
		Stages: []PipelineStage{
			{
				Name: "stage1",
				Workflow: &Workflow{
					Name: "stage1",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{"message": "hello world"}},
					},
				},
			},
			{
				Name:      "stage2",
				DependsOn: []string{"stage1"},
				InputExpr: "{{stage.stage1.output}}",
				Workflow: &Workflow{
					Name: "stage2",
					Steps: []WorkflowStep{
						{Node: "echo", Params: map[string]string{"message": "{{input}}"}},
					},
				},
			},
		},
	}

	result, err := pe.Execute(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected pipeline to succeed")
	}

	if sr, ok := result.StageResults["stage1"]; !ok || sr.Error != nil {
		t.Errorf("stage1 failed: %v", sr.Error)
	}
	if sr, ok := result.StageResults["stage2"]; !ok || sr.Error != nil {
		t.Errorf("stage2 failed: %v", sr.Error)
	}
}

// testEchoNode is a simple node that echoes its input or a message param.
type testEchoNode struct{}

func (n *testEchoNode) Name() string        { return "echo" }
func (n *testEchoNode) Description() string { return "echoes input" }
func (n *testEchoNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{
		Params: []nodes.ParamSchema{
			{Name: "message", Type: "string", Required: false},
		},
	}
}
func (n *testEchoNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if msg, ok := params["message"]; ok {
		return msg, nil
	}
	return input, nil
}

// testFailingNode always returns an error.
type testFailingNode struct{}

func (n *testFailingNode) Name() string        { return "fail" }
func (n *testFailingNode) Description() string { return "always fails" }
func (n *testFailingNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{}
}
func (n *testFailingNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	return "", fmt.Errorf("intentional failure")
}
