package workflow

import (
	"context"
	"testing"

	"github.com/alib8b8/llm-box/internal/nodes"
)

type testNode struct {
	name string
}

func (n *testNode) Name() string {
	return n.name
}

func (n *testNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if msg, ok := params["prefix"]; ok {
		return msg + " " + input, nil
	}
	return "processed: " + input, nil
}

func TestExecuteWorkflow_Simple(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wf := &Workflow{
		Name: "test workflow",
		Steps: []Step{
			{Node: "test", Params: map[string]string{"prefix": "first"}},
			{Node: "test", Params: map[string]string{"prefix": "second"}},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	expected := "second first "
	if output != expected {
		t.Errorf("expected output '%s', got '%s'", expected, output)
	}

	if results[0].Duration <= 0 {
		t.Error("first step duration should be positive")
	}
}

func TestExecuteWorkflow_NodeNotFound(t *testing.T) {
	reg := nodes.NewRegistry()

	wf := &Workflow{
		Steps: []Step{
			{Node: "nonexistent"},
		},
	}

	_, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestExecuteWorkflow_StepError(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&errorNode{name: "err"})

	wf := &Workflow{
		Steps: []Step{
			{Node: "err"},
		},
	}

	_, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Error("expected error from error node")
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if results[0].Error == nil {
		t.Error("expected error in result")
	}
}

type errorNode struct {
	name string
}

func (n *errorNode) Name() string {
	return n.name
}

func (n *errorNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	return "", &testError{msg: "test error"}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestExecuteWorkflow_EmptyWorkflow(t *testing.T) {
	reg := nodes.NewRegistry()

	wf := &Workflow{
		Steps: []Step{},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output != "" {
		t.Errorf("expected empty output, got '%s'", output)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParseWorkflow_InvalidFile(t *testing.T) {
	_, err := ParseWorkflow("/nonexistent/file.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
