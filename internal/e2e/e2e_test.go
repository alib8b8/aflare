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

package e2e

import (
	"context"
	"os"
	"testing"

	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/workflow"
)

func TestE2E_SimpleWorkflow(t *testing.T) {
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	wf := &workflow.Workflow{
		Name:        "E2E Simple Test",
		Description: "Test basic workflow execution",
		Steps: []workflow.WorkflowStep{
			{Node: "execute", Params: map[string]string{"command": "echo 'hello world'"}},
			{Node: "transform", Params: map[string]string{"operation": "upper"}},
		},
	}

	output, results, err := workflow.ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	if output != "HELLO WORLD" {
		t.Errorf("expected 'HELLO WORLD', got '%s'", output)
	}

	if results[0].Error != nil {
		t.Errorf("step 0 failed: %v", results[0].Error)
	}

	if results[1].Error != nil {
		t.Errorf("step 1 failed: %v", results[1].Error)
	}
}

func TestE2E_FileReadWrite(t *testing.T) {
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	testFile := "e2e_test_file.txt"
	defer os.Remove(testFile)

	wf := &workflow.Workflow{
		Name: "E2E File Test",
		Steps: []workflow.WorkflowStep{
			{Node: "execute", Params: map[string]string{"command": "echo 'test content'"}},
			{Node: "file_write", Params: map[string]string{"path": testFile}},
			{Node: "file_read", Params: map[string]string{"path": testFile}},
		},
	}

	output, results, err := workflow.ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	if output != "test content" {
		t.Errorf("expected 'test content', got '%s'", output)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	if string(data) != "test content" {
		t.Errorf("file content mismatch: got '%s'", string(data))
	}
}

func TestE2E_JSONParse(t *testing.T) {
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	wf := &workflow.Workflow{
		Name: "E2E JSON Test",
		Steps: []workflow.WorkflowStep{
			{Node: "execute", Params: map[string]string{"command": "echo '{\"name\": \"test\", \"value\": 42}'"}},
			{Node: "json_parse", Params: map[string]string{"path": "name"}},
		},
	}

	output, results, err := workflow.ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	if output != "test" {
		t.Errorf("expected 'test', got '%s'", output)
	}
}

func TestE2E_TemplateRender(t *testing.T) {
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	wf := &workflow.Workflow{
		Name: "E2E Template Test",
		Steps: []workflow.WorkflowStep{
			{Node: "execute", Params: map[string]string{"command": "echo 'John'"}},
			{Node: "template_render", Params: map[string]string{"template": "Hello World"}},
		},
	}

	output, _, err := workflow.ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	if output != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", output)
	}
}

func TestE2E_ConditionNode(t *testing.T) {
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	wf := &workflow.Workflow{
		Name: "E2E Condition Test",
		Steps: []workflow.WorkflowStep{
			{Node: "execute", Params: map[string]string{"command": "echo 'hello world'"}},
			{Node: "condition", Params: map[string]string{"expr": "contains:hello"}},
		},
	}

	output, _, err := workflow.ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	if output != "true" {
		t.Errorf("expected 'true', got '%s'", output)
	}
}

func TestE2E_ParallelExecution(t *testing.T) {
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	wf := &workflow.Workflow{
		Name: "E2E Parallel Test",
		Steps: []workflow.WorkflowStep{
			{
				Parallel: []workflow.Step{
					{Node: "execute", Params: map[string]string{"command": "echo 'task1'"}},
					{Node: "execute", Params: map[string]string{"command": "echo 'task2'"}},
					{Node: "execute", Params: map[string]string{"command": "echo 'task3'"}},
				},
			},
		},
	}

	output, results, err := workflow.ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results from parallel execution, got %d", len(results))
	}

	if output == "" {
		t.Error("expected non-empty output from parallel execution")
	}
}

func TestE2E_ChainedWorkflow(t *testing.T) {
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	// Use a relative path so the call node's path validation (which rejects
	// absolute paths to prevent arbitrary file reads) accepts it.
	subWorkflowPath := "e2e_sub_workflow.yaml"
	defer os.Remove(subWorkflowPath)

	subWorkflowContent := `
name: "Sub Workflow"
steps:
  - node: execute
    params:
      command: "echo 'sub workflow output'"
`
	if err := os.WriteFile(subWorkflowPath, []byte(subWorkflowContent), 0644); err != nil {
		t.Fatalf("failed to create sub workflow: %v", err)
	}

	wf := &workflow.Workflow{
		Name: "E2E Chain Test",
		Steps: []workflow.WorkflowStep{
			{Node: "call", Params: map[string]string{"workflow": subWorkflowPath}},
		},
	}

	output, results, err := workflow.ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if output != "sub workflow output" {
		t.Errorf("expected 'sub workflow output', got '%s'", output)
	}
}

func TestE2E_ExpressionEvaluation(t *testing.T) {
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	wf := &workflow.Workflow{
		Name: "E2E Expression Test",
		Steps: []workflow.WorkflowStep{
			{Node: "execute", Params: map[string]string{"command": "echo 'first'"}},
			{Node: "execute", Params: map[string]string{"command": "echo '{{step.0}}-second'"}},
		},
	}

	output, _, err := workflow.ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	if output != "first-second" {
		t.Errorf("expected 'first-second', got '%s'", output)
	}
}

func TestE2E_CombineNode(t *testing.T) {
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	wf := &workflow.Workflow{
		Name: "E2E Combine Test",
		Steps: []workflow.WorkflowStep{
			{Node: "execute", Params: map[string]string{"command": "echo 'line1'"}},
			{Node: "execute", Params: map[string]string{"command": "echo 'line2'"}},
			{Node: "combine", Params: map[string]string{"format": "text"}},
		},
	}

	output, _, err := workflow.ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	if output == "" {
		t.Error("expected non-empty output from combine node")
	}
}

func TestE2E_NotifyNode(t *testing.T) {
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	wf := &workflow.Workflow{
		Name: "E2E Notify Test",
		Steps: []workflow.WorkflowStep{
			{Node: "execute", Params: map[string]string{"command": "echo 'test message'"}},
			{Node: "notify", Params: map[string]string{"channel": "stdout"}},
		},
	}

	_, results, err := workflow.ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	if results[1].Error != nil {
		t.Errorf("notify step failed: %v", results[1].Error)
	}
}
