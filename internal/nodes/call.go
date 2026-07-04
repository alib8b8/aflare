package nodes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// MaxCallDepth limits recursive workflow calls to prevent stack overflow
const MaxCallDepth = 10

// callDepthKey is used to propagate the recursion depth through the context
// chain. Unlike a per-ctx map, this survives context derivation
// (WithTimeout/WithCancel) because derived contexts inherit parent values.
var callDepthKey = struct{}{}

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

	// Validate path to prevent arbitrary file read
	safePath, err := validateReadPath(workflowPath)
	if err != nil {
		// If validation fails, try absolute path with explicit check
		if filepath.IsAbs(workflowPath) {
			return "", fmt.Errorf("absolute paths not allowed for workflow calls, use relative paths: %w", err)
		}
		return "", fmt.Errorf("invalid workflow path: %w", err)
	}
	workflowPath = safePath

	// Check recursion depth to prevent stack overflow from circular calls.
	// The depth is propagated via context.Value, which is inherited by derived
	// contexts (WithTimeout/WithCancel) created downstream by the executor,
	// so nested calls correctly accumulate depth.
	depth := 0
	if v, ok := ctx.Value(callDepthKey).(int); ok {
		depth = v
	}
	if depth >= MaxCallDepth {
		return "", fmt.Errorf("maximum workflow call depth (%d) exceeded - possible circular call detected", MaxCallDepth)
	}
	// Propagate incremented depth to the called workflow via a child context.
	childCtx := context.WithValue(ctx, callDepthKey, depth+1)

	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return "", fmt.Errorf("failed to read workflow file %q: %w", workflowPath, err)
	}

	// Limit workflow file size to prevent DoS
	if len(data) > 10*1024*1024 {
		return "", fmt.Errorf("workflow file too large (max 10MB)")
	}

	result, _, err := ExecuteWorkflowFunc(childCtx, string(data), GetGlobalRegistry())
	if err != nil {
		return "", fmt.Errorf("failed to execute workflow %q: %w", workflowPath, err)
	}

	return result, nil
}
