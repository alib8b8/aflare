package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
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
	return ExecuteWorkflowWithTUI(ctx, wf, reg, nil)
}

// ExecuteWorkflowWithTUI executes the workflow and sends messages to a TUI program
func ExecuteWorkflowWithTUI(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program) (string, []StepResult, error) {
	// Set default timeout: 5 minutes
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var results []StepResult
	data := ""

	// Send workflow start message
	if program != nil {
		program.Send(tui.WorkflowStartMsg{
			Name:  wf.Name,
			Path:  "",
			Steps: len(wf.Steps),
		})
	}

	for i, step := range wf.Steps {
		stepStart := time.Now()

		// Send step start message
		if program != nil {
			program.Send(tui.StepStartMsg{
				Index: i,
				Name:  step.Node,
			})
		}

		// Find node in registry
		node, ok := reg.Get(step.Node)
		if !ok {
			err := fmt.Errorf("node '%s' not found in registry", step.Node)
			result := StepResult{
				StepIndex: i,
				NodeName:  step.Node,
				Input:     data,
				Error:     err,
				Duration:  time.Since(stepStart),
			}
			results = append(results, result)

			if program != nil {
				program.Send(tui.StepEndMsg{
					Index:    i,
					Name:     step.Node,
					Error:    err,
					Duration: time.Since(stepStart),
				})
				program.Send(tui.WorkflowEndMsg{Success: false})
			}

			return "", results, err
		}

		// Execute the node
		output, err := node.Execute(timeoutCtx, data, step.Params)
		duration := time.Since(stepStart)

		result := StepResult{
			StepIndex: i,
			NodeName:  step.Node,
			Input:     data,
			Output:    output,
			Error:     err,
			Duration:  duration,
		}
		results = append(results, result)

		// Send step end message
		if program != nil {
			program.Send(tui.StepEndMsg{
				Index:    i,
				Name:     step.Node,
				Output:   output,
				Error:    err,
				Duration: duration,
			})
		}

		// Check for errors
		if err != nil {
			if program != nil {
				program.Send(tui.WorkflowEndMsg{Success: false})
			}
			return "", results, fmt.Errorf("step %d (%s) failed: %w", i+1, step.Node, err)
		}

		// Pass output to next step
		data = output
	}

	// Send workflow end message
	if program != nil {
		program.Send(tui.WorkflowEndMsg{Success: true})
	}

	return data, results, nil
}
