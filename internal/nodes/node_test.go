package nodes

import (
	"context"
	"os"
	"testing"

	"github.com/yourusername/llm-box/internal/workflow"
)

func TestNodeRegistry(t *testing.T) {
	// Test that test_node is registered
	node, ok := Get("test_node")
	if !ok {
		t.Fatal("test_node not found in registry")
	}
	if node.Name() != "test_node" {
		t.Errorf("expected node name 'test_node', got '%s'", node.Name())
	}

	// Test List() returns registered nodes
	names := List()
	found := false
	for _, name := range names {
		if name == "test_node" {
			found = true
			break
		}
	}
	if !found {
		t.Error("test_node not found in List() output")
	}
}

func TestTestNodeExecute(t *testing.T) {
	node, ok := Get("test_node")
	if !ok {
		t.Fatal("test_node not found")
	}

	ctx := context.Background()
	input := "test input"
	params := map[string]string{"message": "test message"}

	output, err := node.Execute(ctx, input, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expected := "Input: test input\nMessage: test message"
	if output != expected {
		t.Errorf("expected output '%s', got '%s'", expected, output)
	}
}

func TestWorkflowParsing(t *testing.T) {
	// Create a temporary YAML file
	yamlContent := `name: Test Workflow
description: A test workflow
steps:
  - node: test_node
    params:
      message: "Hello test!"
`
	tmpFile, err := os.CreateTemp("", "test-workflow-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Parse the workflow
	wf, err := workflow.ParseWorkflow(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to parse workflow: %v", err)
	}

	if wf.Name != "Test Workflow" {
		t.Errorf("expected name 'Test Workflow', got '%s'", wf.Name)
	}

	if len(wf.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(wf.Steps))
	}

	// Check that node exists in registry
	node, ok := Get(wf.Steps[0].Node)
	if !ok {
		t.Errorf("node '%s' not found in registry", wf.Steps[0].Node)
	}
	if node.Name() != "test_node" {
		t.Errorf("expected node name 'test_node', got '%s'", node.Name())
	}
}
