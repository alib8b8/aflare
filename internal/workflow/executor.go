package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/yourusername/llm-box/internal/nodes"
)

// StepResult stores the result of executing a single step
type StepResult struct {
	StepIndex int
	NodeName  string
	Input     string
	Output    string
	Error     error
	Duration  time.Duration
}

// ExecuteWorkflow executes a workflow step by step
func ExecuteWorkflow(ctx context.Context, wf *Workflow, reg *nodes.Registry) (string, []StepResult, error) {
	// Set default timeout: 5 minutes
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var results []StepResult
	data := ""

	for i, step := range wf.Steps {
		stepStart := time.Now()

		// Find node in registry
		node, ok := reg.Get(step.Node)
		if !ok {
			result := StepResult{
				StepIndex: i,
				NodeName:  step.Node,
				Input:     data,
				Error:     fmt.Errorf("node '%s' not found in registry", step.Node),
				Duration:  time.Since(stepStart),
			}
			results = append(results, result)
			return "", results, result.Error
		}

		// Execute the node
		output, err := node.Execute(timeoutCtx, data, step.Params)

		result := StepResult{
			StepIndex: i,
			NodeName:  step.Node,
			Input:     data,
			Output:    output,
			Error:     err,
			Duration:  time.Since(stepStart),
		}
		results = append(results, result)

		// Check for errors
		if err != nil {
			return "", results, fmt.Errorf("step %d (%s) failed: %w", i+1, step.Node, err)
		}

		// Pass output to next step
		data = output
	}

	return data, results, nil
}
