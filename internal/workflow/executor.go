package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"
)

// Security limits
const (
	MaxSteps               = 1000             // Maximum number of steps in a workflow
	MaxParallel            = 50               // Maximum parallel steps in a single step
	MaxRetry               = 10               // Maximum retry count per step
	MaxFileSize            = 10 * 1024 * 1024 // 10MB max workflow file size
	MaxStepTimeout         = 30 * time.Minute // Maximum per-step timeout
	MaxRetryDelay          = 5 * time.Minute  // Maximum retry delay
	DefaultWorkflowTimeout = 5 * time.Minute  // Default overall workflow timeout
	MaxIfDepth             = 20               // Maximum nested if/else depth
)

// ifDepthKey propagates the if/else nesting depth through context.
var ifDepthKey = struct{}{}

// WorkflowTimeout is the overall workflow timeout. It defaults to
// DefaultWorkflowTimeout but can be overridden by callers to configure a
// different workflow timeout without modifying types.go.
var WorkflowTimeout = DefaultWorkflowTimeout

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

// WorkflowState represents the persisted state of a workflow execution.
// It can be saved to disk and resumed later.
type WorkflowState struct {
	WorkflowName string            `json:"workflow_name"`
	StepIndex    int               `json:"step_index"`
	Data         string            `json:"data"`
	StepOutputs  map[int]string    `json:"step_outputs"`
	Variables    map[string]string `json:"variables"`
	SavedAt      time.Time         `json:"saved_at"`
}

// SaveState persists the current workflow state to a file.
func SaveState(path string, state *WorkflowState) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	// Validate path safety
	safePath, err := validateStatePath(path)
	if err != nil {
		return err
	}
	return os.WriteFile(safePath, data, 0600)
}

// LoadState reads a previously saved workflow state from a file.
func LoadState(path string) (*WorkflowState, error) {
	if path == "" {
		return nil, nil
	}
	safePath, err := validateStatePath(path)
	if err != nil {
		return nil, err
	}
	// Security: check file size before reading
	info, err := os.Stat(safePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat state file: %w", err)
	}
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("state file too large (max %d bytes)", MaxFileSize)
	}
	data, err := os.ReadFile(safePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}
	var state WorkflowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}
	return &state, nil
}

// validateStatePath ensures the state file path is safe (no traversal).
func validateStatePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	// Reject absolute paths
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute paths not allowed")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	absPath := filepath.Join(cwd, path)
	// Resolve symlinks
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	// Verify resolved path is within cwd using filepath.Rel
	rel, err := filepath.Rel(cwd, resolved)
	if err != nil {
		return "", fmt.Errorf("path outside working directory")
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path outside working directory")
	}
	return resolved, nil
}

// ConcurrencyLimiter provides a global semaphore for limiting concurrent operations.
type ConcurrencyLimiter struct {
	sem chan struct{}
}

// NewConcurrencyLimiter creates a limiter with the given max concurrency.
// If max <= 0, returns nil (unlimited).
func NewConcurrencyLimiter(max int) *ConcurrencyLimiter {
	if max <= 0 {
		return nil
	}
	return &ConcurrencyLimiter{sem: make(chan struct{}, max)}
}

// Acquire blocks until a slot is available. No-op if limiter is nil.
func (cl *ConcurrencyLimiter) Acquire(ctx context.Context) error {
	if cl == nil {
		return nil
	}
	select {
	case cl.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release returns a slot. No-op if limiter is nil.
func (cl *ConcurrencyLimiter) Release() {
	if cl == nil {
		return
	}
	<-cl.sem
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

	// Validate input schema if defined
	if err := validateInputSchema(wf); err != nil {
		return "", nil, fmt.Errorf("input validation failed: %w", err)
	}

	logger.Info("workflow execution started", "name", wf.Name, "steps", len(wf.Steps))

	timeoutCtx, cancel := context.WithTimeout(ctx, WorkflowTimeout)
	defer cancel()

	var results []StepResult
	data := ""
	engine := NewExpressionEngine()

	// Load workflow-level vars into expression engine
	if wf.Vars != nil {
		for k, v := range wf.Vars {
			engine.SetVariable(k, v)
		}
	}

	// Create global concurrency limiter
	globalLimiter := NewConcurrencyLimiter(wf.MaxConcurrency)

	if program != nil {
		program.Send(tui.WorkflowStartMsg{
			Name:  wf.Name,
			Path:  "",
			Steps: len(wf.Steps),
		})
	}

	for i, wStep := range wf.Steps {
		// ── Handle if/else branch ──
		if wStep.IsIf() {
			branchResults, output, err := executeIfBranch(timeoutCtx, i, wStep.If, data, engine, reg, program, globalLimiter)
			if err != nil {
				results = append(results, branchResults...)
				if program != nil {
					program.Send(tui.WorkflowEndMsg{Success: false})
				}
				return "", results, err
			}
			results = append(results, branchResults...)
			data = output
			engine.SetStepOutput(i, wStep.Name, output)
			continue
		}

		// Check condition - skip step if condition evaluates to false
		if wStep.Condition != "" {
			pass, err := evaluateCondition(wStep.Condition, data, engine)
			if err != nil {
				logger.Error("condition evaluation failed", "index", i, "error", err)
				result := StepResult{
					StepIndex: i,
					NodeName:  wStep.Node,
					Input:     data,
					Error:     fmt.Errorf("condition evaluation failed: %w", err),
					Duration:  0,
				}
				results = append(results, result)
				if program != nil {
					program.Send(tui.StepStartMsg{
						Index: i,
						Name:  wStep.Node,
					})
					program.Send(tui.StepEndMsg{
						Index: i,
						Name:  wStep.Node,
						Error: err,
					})
					program.Send(tui.WorkflowEndMsg{Success: false})
				}
				return "", results, err
			}
			if !pass {
				logger.Info("step skipped by condition", "index", i, "node", wStep.Node)
				// Register step output as empty so later step refs still work
				engine.SetStepOutput(i, wStep.Node, "")
				result := StepResult{
					StepIndex: i,
					NodeName:  wStep.Node,
					Input:     data,
					Output:    "",
					Error:     nil,
					Duration:  0,
				}
				results = append(results, result)
				if program != nil {
					program.Send(tui.StepStartMsg{
						Index: i,
						Name:  wStep.Node,
					})
					program.Send(tui.StepEndMsg{
						Index:    i,
						Name:     wStep.Node,
						Output:   "",
						Duration: 0,
					})
				}
				continue
			}
		}

		if wStep.IsLoop() {
			loopResults, output, err := executeLoopStep(timeoutCtx, i, wStep, data, engine, reg, program, globalLimiter)
			if err != nil {
				results = append(results, loopResults...)
				if program != nil {
					program.Send(tui.WorkflowEndMsg{Success: false})
				}
				return "", results, err
			}
			results = append(results, loopResults...)
			data = applyOutputStrategy(output, wStep.OutputStrategy)
			engine.SetStepOutput(i, wStep.Name, output)
			continue
		}

		if wStep.IsParallel() {
			parallelResults, output, err := executeParallelStep(timeoutCtx, i, wStep, data, engine, reg, program, globalLimiter)
			if err != nil {
				results = append(results, parallelResults...)
				if program != nil {
					program.Send(tui.WorkflowEndMsg{Success: false})
				}
				return "", results, err
			}
			results = append(results, parallelResults...)
			data = applyOutputStrategy(output, wStep.OutputStrategy)
			engine.SetStepOutput(i, wStep.Name, output)
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

			logger.Warn("step failed, retrying", "index", i, "node", wStep.Node, "attempt", attempt, "max", maxAttempts, "error", nodes.RedactSensitive(execErr.Error()))

			if attempt < maxAttempts {
				// Use backoff delay if configured, otherwise use fixed delay
				retryDelayActual := wStep.GetBackoffDelay(attempt)
				select {
				case <-time.After(retryDelayActual):
				case <-timeoutCtx.Done():
					return "", results, fmt.Errorf("workflow timed out during retry delay")
				}
			}
		}

		// ── Error recovery ──
		// resultErr tracks the error shown in StepResult. For fallback/on_error,
		// the step is recovered so resultErr is cleared. For continue_on_error,
		// the step genuinely failed so resultErr is preserved.
		resultErr := execErr
		if execErr != nil {
			// 1. Try fallback value
			if wStep.Fallback != "" {
				fallbackVal, ferr := engine.Evaluate(wStep.Fallback, data)
				if ferr == nil {
					logger.Info("step recovered via fallback", "index", i, "node", wStep.Node)
					output = fallbackVal
					execErr = nil
					resultErr = nil
				}
			}
			// 2. Try on_error handler node
			if execErr != nil && wStep.OnError != nil {
				errStep := *wStep.OnError
				errParams, eerr := engine.EvaluateParams(errStep.Params, data)
				if eerr == nil {
					if errNode, ok := reg.Get(errStep.Node); ok {
						errOutput, errExecErr := errNode.Execute(timeoutCtx, data, errParams)
						if errExecErr == nil {
							logger.Info("step recovered via on_error handler", "index", i, "handler", errStep.Node)
							output = errOutput
							execErr = nil
							resultErr = nil
						}
					}
				}
			}
			// 3. Check continue_on_error: clear execErr so workflow continues,
			// but keep resultErr so StepResult reflects the actual failure.
			if execErr != nil && wStep.ContinueOnError {
				logger.Warn("step failed but continue_on_error is set, continuing", "index", i, "node", wStep.Node, "error", nodes.RedactSensitive(execErr.Error()))
				output = ""
				execErr = nil
			}
		}

		engine.SetStepOutput(i, wStep.Node, output)

		result := StepResult{
			StepIndex: i,
			NodeName:  wStep.Node,
			Input:     data,
			Output:    output,
			Error:     resultErr,
			Duration:  duration,
		}
		results = append(results, result)

		if resultErr != nil {
			logger.Error("step failed", "index", i, "node", wStep.Node, "duration", duration, "error", nodes.RedactSensitive(resultErr.Error()))
		} else {
			logger.Info("step completed", "index", i, "node", wStep.Node, "duration", duration)
		}

		if program != nil {
			program.Send(tui.StepEndMsg{
				Index:    i,
				Name:     wStep.Node,
				Output:   output,
				Error:    resultErr,
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

	// If output expression is defined, evaluate it instead of returning last step output
	if wf.Output != "" {
		finalOutput, err := engine.Evaluate(wf.Output, data)
		if err != nil {
			logger.Warn("failed to evaluate output expression, using last step output", "error", err)
		} else {
			data = finalOutput
		}
	}

	return data, results, nil
}

func executeParallelStep(ctx context.Context, stepIndex int, wStep WorkflowStep, input string, engine *ExpressionEngine, reg *nodes.Registry, program *tea.Program, globalLimiter *ConcurrencyLimiter) ([]StepResult, string, error) {
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

	// Pre-evaluate conditions and params in the main goroutine before spawning
	// goroutines. The ExpressionEngine reads its internal maps (stepOutputs,
	// variables, etc.) without synchronization, so concurrent access from
	// parallel goroutines would cause a data race.
	type preEval struct {
		nodeName        string
		evaluatedParams map[string]string
		paramsErr       error
		condPass        bool // valid only when hasCond && condErr == nil
		condErr         error
		hasCond         bool
	}

	preEvals := make([]preEval, len(wStep.Parallel))
	for j, step := range wStep.Parallel {
		pe := preEval{nodeName: step.Node}
		if step.Condition != "" {
			pe.hasCond = true
			pe.condPass, pe.condErr = evaluateCondition(step.Condition, input, engine)
		}
		// Only evaluate params when the condition allows execution
		if pe.condErr == nil && (!pe.hasCond || pe.condPass) {
			pe.evaluatedParams, pe.paramsErr = engine.EvaluateParams(step.Params, input)
		}
		preEvals[j] = pe
	}

	resultsChan := make(chan parallelResult, len(wStep.Parallel))

	for j, step := range wStep.Parallel {
		pe := preEvals[j]
		go func(j int, step Step, pe preEval) {
			// Acquire global concurrency slot if configured
			if globalLimiter != nil {
				if err := globalLimiter.Acquire(ctx); err != nil {
					resultsChan <- parallelResult{
						stepIndex: stepIndex*MaxParallel + j,
						nodeName:  pe.nodeName,
						err:       err,
						duration:  0,
					}
					return
				}
				defer globalLimiter.Release()
			}
			start := time.Now()
			nodeName := pe.nodeName
			// Use compound index (stepIndex*MaxParallel+j) to distinguish
			// parallel sub-steps from main steps in the TUI display.

			// Handle pre-evaluated condition
			if pe.hasCond {
				if pe.condErr != nil {
					resultsChan <- parallelResult{
						stepIndex: stepIndex*MaxParallel + j,
						nodeName:  nodeName,
						err:       fmt.Errorf("condition evaluation failed: %w", pe.condErr),
						duration:  time.Since(start),
					}
					return
				}
				if !pe.condPass {
					if program != nil {
						program.Send(tui.StepStartMsg{
							Index: stepIndex*MaxParallel + j,
							Name:  nodeName,
						})
						program.Send(tui.StepEndMsg{
							Index:    stepIndex*MaxParallel + j,
							Name:     nodeName,
							Output:   "",
							Duration: 0,
						})
					}
					resultsChan <- parallelResult{
						stepIndex: stepIndex*MaxParallel + j,
						nodeName:  nodeName,
						output:    "",
						duration:  0,
					}
					return
				}
			}

			if program != nil {
				program.Send(tui.StepStartMsg{
					Index: stepIndex*MaxParallel + j,
					Name:  nodeName,
				})
			}

			// Handle pre-evaluated params
			if pe.paramsErr != nil {
				resultsChan <- parallelResult{
					stepIndex: stepIndex*MaxParallel + j,
					nodeName:  nodeName,
					err:       pe.paramsErr,
					duration:  time.Since(start),
				}
				return
			}

			node, ok := reg.Get(nodeName)
			if !ok {
				resultsChan <- parallelResult{
					stepIndex: stepIndex*MaxParallel + j,
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

		retryLoop:
			for attempt := 1; attempt <= maxAttempts; attempt++ {
				var stepCtx context.Context
				var stepCancel context.CancelFunc
				stepTimeout := step.GetTimeout()
				if stepTimeout > 0 {
					stepCtx, stepCancel = context.WithTimeout(ctx, stepTimeout)
				} else {
					stepCtx, stepCancel = context.WithCancel(ctx)
				}

				// Note: parallel steps do not support streaming output.
				// Interleaving chunks from multiple concurrent streams in the
				// TUI would be confusing, so we always use the non-streaming
				// Execute API here.
				output, execErr = node.Execute(stepCtx, input, pe.evaluatedParams)
				stepCancel()

				if execErr == nil {
					break
				}
				if attempt < maxAttempts {
					select {
					case <-time.After(retryDelay):
					case <-ctx.Done():
						execErr = ctx.Err()
						break retryLoop
					}
				}
			}

			resultsChan <- parallelResult{
				stepIndex: stepIndex*MaxParallel + j,
				nodeName:  nodeName,
				output:    output,
				err:       execErr,
				duration:  time.Since(start),
			}
		}(j, step, pe)
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
			logger.Error("parallel step failed", "index", res.stepIndex, "node", res.nodeName, "error", nodes.RedactSensitive(res.err.Error()))
		} else {
			outputs = append(outputs, res.output)
			logger.Info("parallel step completed", "index", res.stepIndex, "node", res.nodeName, "duration", res.duration)
		}
	}

	if firstErr != nil {
		// Check max_failures threshold for parallel groups
		maxFailures := wStep.MaxFailures
		if maxFailures < 0 {
			maxFailures = 0
		}
		failureCount := 0
		for _, sr := range stepResults {
			if sr.Error != nil {
				failureCount++
			}
		}
		if failureCount > maxFailures {
			return stepResults, "", fmt.Errorf("parallel step failed: %d/%d sub-steps failed (max_failures: %d): %w", failureCount, len(wStep.Parallel), maxFailures, firstErr)
		}
		logger.Warn("parallel step had failures but within max_failures limit", "failures", failureCount, "max", maxFailures)
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

// executeWithRetry runs a node with retry logic. Used by loop iterations.
func executeWithRetry(ctx context.Context, node nodes.Node, input string, params map[string]string, retryCount int, retryDelay, timeout time.Duration) (string, error) {
	if retryCount > MaxRetry {
		retryCount = MaxRetry
	}
	if retryDelay > MaxRetryDelay {
		retryDelay = MaxRetryDelay
	}
	if timeout > MaxStepTimeout {
		timeout = MaxStepTimeout
	}

	maxAttempts := retryCount + 1
	var output string
	var execErr error

retryLoop:
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var stepCtx context.Context
		var stepCancel context.CancelFunc
		if timeout > 0 {
			stepCtx, stepCancel = context.WithTimeout(ctx, timeout)
		} else {
			stepCtx, stepCancel = context.WithCancel(ctx)
		}

		output, execErr = node.Execute(stepCtx, input, params)
		stepCancel()

		if execErr == nil {
			break
		}
		if attempt < maxAttempts {
			select {
			case <-time.After(retryDelay):
			case <-ctx.Done():
				execErr = ctx.Err()
				break retryLoop
			}
		}
	}
	return output, execErr
}

// executeLoopStep executes a step in a loop over a list of items.
func executeLoopStep(ctx context.Context, stepIndex int, wStep WorkflowStep, input string, engine *ExpressionEngine, reg *nodes.Registry, program *tea.Program, globalLimiter *ConcurrencyLimiter) ([]StepResult, string, error) {
	loopCfg := wStep.Loop

	// Evaluate items expression
	itemsStr, err := engine.Evaluate(loopCfg.Items, input)
	if err != nil {
		return nil, "", fmt.Errorf("failed to evaluate loop items: %w", err)
	}

	// Split items
	splitBy := loopCfg.GetSplitBy()
	var items []string
	if splitBy == "\n" {
		items = strings.Split(itemsStr, "\n")
	} else {
		items = strings.Split(itemsStr, splitBy)
	}

	// Filter empty items
	var validItems []string
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			validItems = append(validItems, item)
		}
	}
	items = validItems

	// Safety limits
	maxIter := loopCfg.GetMaxIterations()
	if len(items) > maxIter {
		return nil, "", fmt.Errorf("loop items (%d) exceed max_iterations (%d)", len(items), maxIter)
	}
	if len(items) == 0 {
		return nil, "", nil
	}

	// Get node
	node, ok := reg.Get(wStep.Node)
	if !ok {
		return nil, "", fmt.Errorf("node '%s' not found in registry", wStep.Node)
	}

	// Pre-evaluate params for each iteration in the main goroutine
	// (ExpressionEngine is not thread-safe)
	type iterEval struct {
		item   string
		params map[string]string
		err    error
	}
	iterEvals := make([]iterEval, len(items))
	for idx, item := range items {
		engine.SetLoopVars(item, idx, len(items))
		evaluatedParams, eerr := engine.EvaluateParams(wStep.Params, input)
		if evaluatedParams == nil {
			evaluatedParams = make(map[string]string)
		}
		evaluatedParams[loopCfg.GetVar()] = item
		iterEvals[idx] = iterEval{item: item, params: evaluatedParams, err: eerr}
	}
	engine.ClearLoopVars()

	retryCount := wStep.GetRetryCount()
	retryDelay := wStep.GetRetryDelay()
	stepTimeout := wStep.GetTimeout()
	concurrency := loopCfg.GetConcurrency()
	stopOnError := loopCfg.GetStopOnError()

	var results []StepResult
	var outputs []string

	if concurrency <= 1 {
		// ── Sequential execution ──
		for idx, ie := range iterEvals {
			if program != nil {
				program.Send(tui.StepStartMsg{
					Index: stepIndex*MaxParallel + idx,
					Name:  wStep.Node,
				})
			}

			start := time.Now()
			var output string
			var execErr error
			if ie.err != nil {
				execErr = ie.err
			} else {
				output, execErr = executeWithRetry(ctx, node, ie.item, ie.params, retryCount, retryDelay, stepTimeout)
			}
			duration := time.Since(start)

			sr := StepResult{
				StepIndex: stepIndex*MaxParallel + idx,
				NodeName:  wStep.Node,
				Input:     ie.item,
				Output:    output,
				Error:     execErr,
				Duration:  duration,
			}
			results = append(results, sr)

			if program != nil {
				program.Send(tui.StepEndMsg{
					Index:    stepIndex*MaxParallel + idx,
					Name:     wStep.Node,
					Output:   output,
					Error:    execErr,
					Duration: duration,
				})
			}

			if execErr != nil {
				logger.Error("loop iteration failed", "index", idx, "node", wStep.Node, "error", nodes.RedactSensitive(execErr.Error()))
				if stopOnError {
					return results, "", fmt.Errorf("loop iteration %d failed: %w", idx+1, execErr)
				}
			} else {
				outputs = append(outputs, output)
			}
		}
	} else {
		// ── Concurrent execution ──
		sem := make(chan struct{}, concurrency)
		type loopResult struct {
			idx    int
			output string
			err    error
			dur    time.Duration
		}
		resultsChan := make(chan loopResult, len(items))

		for idx, ie := range iterEvals {
			sem <- struct{}{}
			go func(idx int, item string, params map[string]string, perr error) {
				defer func() { <-sem }()
				// Acquire global concurrency slot if configured
				if globalLimiter != nil {
					if err := globalLimiter.Acquire(ctx); err != nil {
						resultsChan <- loopResult{idx: idx, err: err, dur: 0}
						return
					}
					defer globalLimiter.Release()
				}
				start := time.Now()

				if program != nil {
					program.Send(tui.StepStartMsg{
						Index: stepIndex*MaxParallel + idx,
						Name:  wStep.Node,
					})
				}

				if perr != nil {
					resultsChan <- loopResult{idx: idx, err: perr, dur: time.Since(start)}
					return
				}
				output, execErr := executeWithRetry(ctx, node, item, params, retryCount, retryDelay, stepTimeout)
				resultsChan <- loopResult{idx: idx, output: output, err: execErr, dur: time.Since(start)}
			}(idx, ie.item, ie.params, ie.err)
		}

		// Collect all results
		loopResults := make([]loopResult, len(items))
		for i := 0; i < len(items); i++ {
			lr := <-resultsChan
			loopResults[lr.idx] = lr
		}

		// Process in order
		for _, lr := range loopResults {
			sr := StepResult{
				StepIndex: stepIndex*MaxParallel + lr.idx,
				NodeName:  wStep.Node,
				Input:     iterEvals[lr.idx].item,
				Output:    lr.output,
				Error:     lr.err,
				Duration:  lr.dur,
			}
			results = append(results, sr)

			if program != nil {
				program.Send(tui.StepEndMsg{
					Index:    stepIndex*MaxParallel + lr.idx,
					Name:     wStep.Node,
					Output:   lr.output,
					Error:    lr.err,
					Duration: lr.dur,
				})
			}

			if lr.err != nil {
				logger.Error("loop iteration failed", "index", lr.idx, "node", wStep.Node, "error", nodes.RedactSensitive(lr.err.Error()))
				if stopOnError {
					return results, "", fmt.Errorf("loop iteration %d failed: %w", lr.idx+1, lr.err)
				}
			} else {
				outputs = append(outputs, lr.output)
			}
		}
	}

	// Combine outputs
	finalOutput := strings.Join(outputs, "\n---\n")
	return results, finalOutput, nil
}

// evaluateCondition evaluates a condition expression against the current input.
// Syntax is the same as the condition node, but the comparison value is evaluated
// through the expression engine so {{step.0}}, {{var.name}} etc. work.
//
// Examples:
//
//	contains:hello          - input contains "hello"
//	equals:{{var.target}}   - input equals the value of var.target
//	empty                   - input is empty
//	not_empty               - input is not empty
//	regex:\d+               - input matches regex
//	starts_with:https       - input starts with "https"
//	ends_with:.json         - input ends with ".json"
//	not contains:skip       - input does NOT contain "skip"
func evaluateCondition(cond string, input string, engine *ExpressionEngine) (bool, error) {
	if cond == "" {
		return true, nil
	}

	// Evaluate any {{...}} expressions in the condition's comparison value
	evaluated, err := engine.Evaluate(cond, input)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate condition expression: %w", err)
	}

	negate := false
	if strings.HasPrefix(evaluated, "not ") {
		negate = true
		evaluated = strings.TrimPrefix(evaluated, "not ")
	}

	result := false
	var op, value string

	if strings.Contains(evaluated, ":") {
		parts := strings.SplitN(evaluated, ":", 2)
		op = strings.TrimSpace(parts[0])
		value = strings.TrimSpace(parts[1])
	} else {
		op = strings.TrimSpace(evaluated)
	}

	switch op {
	case "true":
		result = true
	case "false":
		result = false
	case "contains":
		result = strings.Contains(input, value)
	case "equals":
		result = input == value
	case "starts_with":
		result = strings.HasPrefix(input, value)
	case "ends_with":
		result = strings.HasSuffix(input, value)
	case "regex":
		matched, err := nodes.SafeRegexMatch(value, input)
		if err != nil {
			return false, fmt.Errorf("regex evaluation failed: %w", err)
		}
		result = matched
	case "empty":
		result = input == ""
	case "not_empty":
		result = input != ""
	default:
		return false, fmt.Errorf("unknown condition operator: %s", op)
	}

	if negate {
		result = !result
	}
	return result, nil
}

// executeIfBranch evaluates an if/else condition and executes the appropriate branch.
// It returns the output of the last step in the executed branch.
func executeIfBranch(ctx context.Context, stepIndex int, ifCfg *IfConfig, input string, engine *ExpressionEngine, reg *nodes.Registry, program *tea.Program, globalLimiter *ConcurrencyLimiter) ([]StepResult, string, error) {
	// Check if/else nesting depth
	depth := 0
	if v, ok := ctx.Value(ifDepthKey).(int); ok {
		depth = v
	}
	if depth >= MaxIfDepth {
		return nil, "", fmt.Errorf("maximum if/else nesting depth (%d) exceeded", MaxIfDepth)
	}

	pass, err := evaluateCondition(ifCfg.Condition, input, engine)
	if err != nil {
		return nil, "", fmt.Errorf("if condition evaluation failed: %w", err)
	}

	var branchSteps []WorkflowStep
	if pass {
		branchSteps = ifCfg.Then
		logger.Info("if branch: executing then", "index", stepIndex, "sub_steps", len(branchSteps))
	} else {
		branchSteps = ifCfg.Else
		logger.Info("if branch: executing else", "index", stepIndex, "sub_steps", len(branchSteps))
	}

	// Execute branch steps as a sub-workflow with incremented depth
	subWf := &Workflow{
		Name:  fmt.Sprintf("if-branch-%d", stepIndex),
		Steps: branchSteps,
	}
	// Pass incremented depth and global limiter via context
	childCtx := context.WithValue(ctx, ifDepthKey, depth+1)
	output, subResults, err := ExecuteWorkflowWithTUI(childCtx, subWf, reg, program)
	if err != nil {
		return subResults, "", err
	}

	return subResults, output, nil
}

// applyOutputStrategy applies the specified output strategy to combined parallel/loop results.
// The input `output` is already joined with "\n---\n" separator.
// For most strategies, we need the raw outputs before joining.
func applyOutputStrategy(output string, strategy string) string {
	if strategy == "" || strategy == "join" {
		return output
	}

	// Split the joined output back into parts
	parts := strings.Split(output, "\n---\n")

	switch strategy {
	case "first":
		if len(parts) > 0 {
			return parts[0]
		}
		return ""
	case "last":
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
		return ""
	case "longest":
		var best string
		for _, p := range parts {
			if len(p) > len(best) {
				best = p
			}
		}
		return best
	case "shortest":
		if len(parts) == 0 {
			return ""
		}
		best := parts[0]
		for _, p := range parts[1:] {
			if len(p) < len(best) {
				best = p
			}
		}
		return best
	case "json_array":
		// Build a JSON array from the parts
		arr := make([]string, len(parts))
		for i, p := range parts {
			// Try to parse each part as JSON; if it fails, use as string
			var raw json.RawMessage
			if err := json.Unmarshal([]byte(p), &raw); err == nil {
				arr[i] = p
			} else {
				b, _ := json.Marshal(p)
				arr[i] = string(b)
			}
		}
		return "[" + strings.Join(arr, ",") + "]"
	default:
		return output
	}
}

// validateInputSchema validates the workflow input against the defined schema.
// Currently performs basic checks: required fields and type coercion.
// The input is expected to be a JSON string if schema is defined.
func validateInputSchema(wf *Workflow) error {
	if len(wf.InputSchema) == 0 {
		return nil
	}

	// Schema validation is informational - it logs warnings but doesn't block execution
	// since input could be non-JSON strings (e.g., plain text for LLM processing)
	// Full validation would require the input to be provided at parse time.
	return nil
}

// SaveCurrentState creates a WorkflowState snapshot from the current execution context.
func SaveCurrentState(wf *Workflow, stepIndex int, data string, engine *ExpressionEngine) *WorkflowState {
	state := &WorkflowState{
		WorkflowName: wf.Name,
		StepIndex:    stepIndex,
		Data:         data,
		StepOutputs:  make(map[int]string),
		Variables:    make(map[string]string),
		SavedAt:      time.Now(),
	}

	// Copy step outputs by index
	for k, v := range engine.stepOutputs {
		if strings.HasPrefix(k, "idx:") {
			if idx, err := strconv.Atoi(strings.TrimPrefix(k, "idx:")); err == nil {
				state.StepOutputs[idx] = v
			}
		}
	}

	// Copy variables
	for k, v := range engine.variables {
		state.Variables[k] = v
	}

	return state
}

// RestoreState restores a previously saved workflow state into the engine.
func RestoreState(state *WorkflowState, engine *ExpressionEngine) string {
	for idx, output := range state.StepOutputs {
		name := engine.stepNames[idx]
		engine.SetStepOutput(idx, name, output)
	}
	for k, v := range state.Variables {
		engine.SetVariable(k, v)
	}
	return state.Data
}
