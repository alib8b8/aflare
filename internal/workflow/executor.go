package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
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
	logger.Info("workflow execution started", "name", wf.Name, "steps", len(wf.Steps))

	// Set default timeout: 5 minutes
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var results []StepResult
	data := ""
	engine := NewExpressionEngine()

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
		logger.Info("step started", "index", i, "node", step.Node)

		// Send step start message
		if program != nil {
			program.Send(tui.StepStartMsg{
				Index: i,
				Name:  step.Node,
			})
		}

		// Evaluate param expressions ({{step.0}}, {{var.name}}, etc.)
		evaluatedParams, err := engine.EvaluateParams(step.Params, data)
		if err != nil {
			logger.Error("expression evaluation failed", "index", i, "error", err)
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

		// Find node in registry
		node, ok := reg.Get(step.Node)
		if !ok {
			err := fmt.Errorf("node '%s' not found in registry", step.Node)
			logger.Error("node not found", "node", step.Node, "error", err)
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
		output, err := node.Execute(timeoutCtx, data, evaluatedParams)
		duration := time.Since(stepStart)

		// Store output for future expression references
		engine.SetStepOutput(i, step.Node, output)

		result := StepResult{
			StepIndex: i,
			NodeName:  step.Node,
			Input:     data,
			Output:    output,
			Error:     err,
			Duration:  duration,
		}
		results = append(results, result)

		if err != nil {
			logger.Error("step failed", "index", i, "node", step.Node, "duration", duration, "error", err)
		} else {
			logger.Info("step completed", "index", i, "node", step.Node, "duration", duration)
		}

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
			logger.Error("workflow failed", "name", wf.Name, "failed_step", i, "node", step.Node, "error", err)
			return "", results, fmt.Errorf("step %d (%s) failed: %w", i+1, step.Node, err)
		}

		// Pass output to next step
		data = output
	}

	// Send workflow end message
	if program != nil {
		program.Send(tui.WorkflowEndMsg{Success: true})
	}

	logger.Info("workflow completed", "name", wf.Name, "steps", len(wf.Steps))
	return data, results, nil
}
