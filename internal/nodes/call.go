package nodes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

var ExecuteWorkflowFunc func(ctx context.Context, wf interface{}, reg *Registry) (string, []interface{}, error)

type CallNode struct{}

func init() {
	Register(&CallNode{})
}

func (n *CallNode) Name() string {
	return "call"
}

func (n *CallNode) Description() string {
	return "Call another workflow"
}

func (n *CallNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "call",
		Description: "Call another workflow file",
		Input:       "string - input data to pass to the called workflow",
		Output:      "string - output from the called workflow",
		Params: []ParamSchema{
			{Name: "workflow", Type: "string", Description: "Path to the workflow file to call", Required: true},
		},
	}
}

func (n *CallNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if ExecuteWorkflowFunc == nil {
		return "", fmt.Errorf("workflow execution function not registered")
	}

	workflowPath, ok := params["workflow"]
	if !ok || workflowPath == "" {
		return "", fmt.Errorf("workflow parameter is required")
	}

	if !filepath.IsAbs(workflowPath) {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
		workflowPath = filepath.Join(wd, workflowPath)
	}

	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return "", fmt.Errorf("failed to read workflow file %q: %w", workflowPath, err)
	}

	result, _, err := ExecuteWorkflowFunc(ctx, string(data), GetGlobalRegistry())
	if err != nil {
		return "", fmt.Errorf("failed to execute workflow %q: %w", workflowPath, err)
	}

	return result, nil
}
