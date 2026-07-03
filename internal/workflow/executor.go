package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"
)

// Security limits
const (
	MaxSteps        = 1000  // Maximum number of steps in a workflow
	MaxParallel     = 50    // Maximum parallel steps in a single step
	MaxRetry        = 10    // Maximum retry count per step
	MaxFileSize     = 10 * 1024 * 1024 // 10MB max workflow file size
	MaxStepTimeout  = 30 * time.Minute // Maximum per-step timeout
	MaxRetryDelay   = 5 * time.Minute  // Maximum retry delay
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

func init() {
	nodes.ExecuteWorkflowFunc = func(ctx context.Context, wf interface{}, reg *nodes.Registry) (string, []interface{}, error) {
		var workflow *Workflow
		var err error

		switch v := wf.(type) {
		case *Workflow:
			workflow = v
		case string:
			if len(v) > MaxFileSize {
				return "", nil, fmt.Errorf("workflow content too large (max %d bytes)", MaxFileSize)
			}
			if err := yaml.Unmarshal([]byte(v), &workflow); err != nil {
				return "", nil, fmt.Errorf("failed to parse workflow YAML: %w", err)
			}
		default:
			return "", nil, fmt.Errorf("unsupported workflow type")
		}

		result, stepResults, err := ExecuteWorkflow(ctx, workflow, reg)
		if err != nil {
			return "", nil, err
		}

		results := make([]interface{}, len(stepResults))
		for i, sr := range stepResults {
			results[i] = sr
		}
		return result, results, nil
	}
}

// ExecuteWorkflow executes a workflow step by step
func ExecuteWorkflow(ctx context.Context, wf *Workflow, reg *nodes.Registry) (string, []StepResult, error) {
	return ExecuteWorkflowWithTUI(ctx, wf, reg, nil)
}

// ExecuteWorkflowWithTUI executes the workflow and sends messages to a TUI program
func ExecuteWorkflowWithTUI(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program) (string, []StepResult, error) {
	// Validate step count
	if len(wf.Steps) > MaxSteps {
		return "", nil, fmt.Errorf("workflow has too many steps (%d, max %d)", len(wf.Steps), MaxSteps)
	}

	logger.Info("workflow execution started", "name", wf.Name, "steps", len(wf.Steps))

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var results []StepResult
	data := ""
	engine := NewExpressionEngine()

	if program != nil {
		program.Send(tui.WorkflowStartMsg{
			Name:  wf.Name,
			Path:  "",
			Steps: len(wf.Steps),
		})
	}

	for i, wStep := range wf.Steps {
		if wStep.IsParallel() {
			parallelResults, output, err := executeParallelStep(timeoutCtx, i, wStep, data, engine, reg, program)
			if err != nil {
				results = append(results, parallelResults...)
				if program != nil {
					program.Send(tui.WorkflowEndMsg{Success: false})
				}
				return "", results, err
			}
			results = append(results, parallelResults...)
			data = output
			// Register parallel output for expression reference
			engine.SetStepOutput(i, "parallel", output)
			continue
		}

		stepStart := time.Now()
		logger.Info("step started", "index", i, "node", wStep.Node)

		if program != nil {
			program.Send(tui.StepStartMsg{
				Index: i,
				Name:  wStep.Node,
			})
		}

		evaluatedParams, err := engine.EvaluateParams(wStep.Params, data)
		if err != nil {
			logger.Error("expression evaluation failed", "index", i, "error", err)
			result := StepResult{
				StepIndex: i,
				NodeName:  wStep.Node,
				Input:     data,
				Error:     err,
				Duration:  time.Since(stepStart),
			}
			results = append(results, result)
			if program != nil {
				program.Send(tui.StepEndMsg{
					Index:    i,
					Name:     wStep.Node,
					Error:    err,
					Duration: time.Since(stepStart),
				})
				program.Send(tui.WorkflowEndMsg{Success: false})
			}
			return "", results, err
		}

		node, ok := reg.Get(wStep.Node)
		if !ok {
			err := fmt.Errorf("node '%s' not found in registry", wStep.Node)
			logger.Error("node not found", "node", wStep.Node, "error", err)
			result := StepResult{
				StepIndex: i,
				NodeName:  wStep.Node,
				Input:     data,
				Error:     err,
				Duration:  time.Since(stepStart),
			}
			results = append(results, result)

			if program != nil {
				program.Send(tui.StepEndMsg{
					Index:    i,
					Name:     wStep.Node,
					Error:    err,
					Duration: time.Since(stepStart),
				})
				program.Send(tui.WorkflowEndMsg{Success: false})
			}

			return "", results, err
		}

		retryCount := wStep.GetRetryCount()
		if retryCount > MaxRetry {
			retryCount = MaxRetry
		}
		retryDelay := wStep.GetRetryDelay()
		if retryDelay > MaxRetryDelay {
			retryDelay = MaxRetryDelay
		}
		stepTimeout := wStep.GetTimeout()
		if stepTimeout > MaxStepTimeout {
			stepTimeout = MaxStepTimeout
		}

		var output string
		var execErr error
		var duration time.Duration
		maxAttempts := retryCount + 1

		for attempt := 1; attempt <= maxAttempts; attempt++ {
			attemptStart := time.Now()

			var stepCtx context.Context
			var stepCancel context.CancelFunc
			if stepTimeout > 0 {
				stepCtx, stepCancel = context.WithTimeout(timeoutCtx, stepTimeout)
			} else {
				stepCtx, stepCancel = context.WithCancel(timeoutCtx)
			}

			if program != nil {
				if streamingNode, ok := node.(nodes.StreamingNode); ok {
					onChunk := func(chunk string) {
						program.Send(tui.StepStreamMsg{
							Index: i,
							Name:  wStep.Node,
							Chunk: chunk,
						})
					}
					output, execErr = streamingNode.ExecuteStream(stepCtx, data, evaluatedParams, onChunk)
				} else {
					output, execErr = node.Execute(stepCtx, data, evaluatedParams)
				}
			} else {
				output, execErr = node.Execute(stepCtx, data, evaluatedParams)
			}
			duration = time.Since(attemptStart)

			stepCancel()

			if execErr == nil {
				break
			}

			logger.Warn("step failed, retrying", "index", i, "node", wStep.Node, "attempt", attempt, "max", maxAttempts, "error", execErr)

			if attempt < maxAttempts {
				// Use context-aware sleep instead of time.Sleep
				select {
				case <-time.After(retryDelay):
				case <-timeoutCtx.Done():
					return "", results, fmt.Errorf("workflow timed out during retry delay")
				}
			}
		}

		engine.SetStepOutput(i, wStep.Node, output)

		result := StepResult{
			StepIndex: i,
			NodeName:  wStep.Node,
			Input:     data,
			Output:    output,
			Error:     execErr,
			Duration:  duration,
		}
		results = append(results, result)

		if execErr != nil {
			logger.Error("step failed", "index", i, "node", wStep.Node, "duration", duration, "error", execErr)
		} else {
			logger.Info("step completed", "index", i, "node", wStep.Node, "duration", duration)
		}

		if program != nil {
			program.Send(tui.StepEndMsg{
				Index:    i,
				Name:     wStep.Node,
				Output:   output,
				Error:    execErr,
				Duration: duration,
			})
		}

		if execErr != nil {
			if program != nil {
				program.Send(tui.WorkflowEndMsg{Success: false})
			}
			logger.Error("workflow failed", "name", wf.Name, "failed_step", i, "node", wStep.Node, "error", execErr)
			return "", results, fmt.Errorf("step %d (%s) failed: %w", i+1, wStep.Node, execErr)
		}

		data = output
	}

	if program != nil {
		program.Send(tui.WorkflowEndMsg{Success: true})
	}

	logger.Info("workflow completed", "name", wf.Name, "steps", len(wf.Steps))
	return data, results, nil
}

func executeParallelStep(ctx context.Context, stepIndex int, wStep WorkflowStep, input string, engine *ExpressionEngine, reg *nodes.Registry, program *tea.Program) ([]StepResult, string, error) {
	// Limit parallel step count
	if len(wStep.Parallel) > MaxParallel {
		return nil, "", fmt.Errorf("too many parallel steps (%d, max %d)", len(wStep.Parallel), MaxParallel)
	}

	logger.Info("parallel step started", "index", stepIndex, "parallel_count", len(wStep.Parallel))

	type parallelResult struct {
		stepIndex int
		nodeName  string
		output    string
		err       error
		duration  time.Duration
	}

	resultsChan := make(chan parallelResult, len(wStep.Parallel))

	for j, step := range wStep.Parallel {
		go func(j int, step Step) {
			start := time.Now()
			nodeName := step.Node

			if program != nil {
				program.Send(tui.StepStartMsg{
					Index: stepIndex*100 + j,
					Name:  nodeName,
				})
			}

			evaluatedParams, err := engine.EvaluateParams(step.Params, input)
			if err != nil {
				resultsChan <- parallelResult{
					stepIndex: stepIndex*100 + j,
					nodeName:  nodeName,
					err:       err,
					duration:  time.Since(start),
				}
				return
			}

			node, ok := reg.Get(nodeName)
			if !ok {
				resultsChan <- parallelResult{
					stepIndex: stepIndex*100 + j,
					nodeName:  nodeName,
					err:       fmt.Errorf("node '%s' not found in registry", nodeName),
					duration:  time.Since(start),
				}
				return
			}

			retryCount := step.GetRetryCount()
			if retryCount > MaxRetry {
				retryCount = MaxRetry
			}
			retryDelay := step.GetRetryDelay()
			if retryDelay > MaxRetryDelay {
				retryDelay = MaxRetryDelay
			}

			var output string
			var execErr error
			maxAttempts := retryCount + 1

			for attempt := 1; attempt <= maxAttempts; attempt++ {
				var stepCtx context.Context
				var stepCancel context.CancelFunc
				stepTimeout := step.GetTimeout()
				if stepTimeout > 0 {
					stepCtx, stepCancel = context.WithTimeout(ctx, stepTimeout)
				} else {
					stepCtx, stepCancel = context.WithCancel(ctx)
				}

				output, execErr = node.Execute(stepCtx, input, evaluatedParams)
				stepCancel()

				if execErr == nil {
					break
				}
				if attempt < maxAttempts {
					select {
					case <-time.After(retryDelay):
					case <-ctx.Done():
						execErr = ctx.Err()
						break
					}
				}
			}

			resultsChan <- parallelResult{
				stepIndex: stepIndex*100 + j,
				nodeName:  nodeName,
				output:    output,
				err:       execErr,
				duration:  time.Since(start),
			}
		}(j, step)
	}

	var stepResults []StepResult
	var outputs []string
	var firstErr error

	for i := 0; i < len(wStep.Parallel); i++ {
		res := <-resultsChan
		sr := StepResult{
			StepIndex: res.stepIndex,
			NodeName:  res.nodeName,
			Input:     input,
			Output:    res.output,
			Error:     res.err,
			Duration:  res.duration,
		}
		stepResults = append(stepResults, sr)

		if program != nil {
			program.Send(tui.StepEndMsg{
				Index:    res.stepIndex,
				Name:     res.nodeName,
				Output:   res.output,
				Error:    res.err,
				Duration: res.duration,
			})
		}

		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
			}
			logger.Error("parallel step failed", "index", res.stepIndex, "node", res.nodeName, "error", res.err)
		} else {
			outputs = append(outputs, res.output)
			logger.Info("parallel step completed", "index", res.stepIndex, "node", res.nodeName, "duration", res.duration)
		}
	}

	if firstErr != nil {
		return stepResults, "", firstErr
	}

	finalOutput := ""
	for _, out := range outputs {
		if finalOutput != "" {
			finalOutput += "\n---\n"
		}
		finalOutput += out
	}

	return stepResults, finalOutput, nil
}
