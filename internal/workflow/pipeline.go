// Copyright (c) 2026 aflare Contributors
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

// Package workflow provides multi-workflow orchestration through the Pipeline
// abstraction, inspired by Avernet's multi-agent collaboration infrastructure.
//
// A Pipeline chains multiple workflows together with:
//   - Declarative dependencies between stages (DAG scheduling)
//   - Data passing: each stage's output flows as input to downstream stages
//   - Conditional execution: skip stages based on upstream results
//   - Failure policies: stop, continue, or retry on stage failure
//
// This solves the "找不到、对不齐、跑不快、留不住" collaboration problems
// identified by Avernet: workflows can now find each other (via named stages),
// align data formats (via expression-based data passing), run concurrently
// (via DAG scheduling), and persist state (via existing checkpoint/WAL).
package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/nodes"
)

// OnFailurePolicy defines how a pipeline handles a stage failure.
type OnFailurePolicy string

const (
	// OnFailureStop aborts the entire pipeline on the first stage failure.
	OnFailureStop OnFailurePolicy = "stop"
	// OnFailureContinue logs the failure and continues to the next stage.
	OnFailureContinue OnFailurePolicy = "continue"
	// OnFailureRetry retries the failed stage up to MaxRetries times before
	// falling back to the policy defined by OnRetryExhausted.
	OnFailureRetry OnFailurePolicy = "retry"
)

// PipelineStage defines a single stage in a multi-workflow pipeline.
// Each stage wraps a Workflow with dependency, condition, and failure
// handling configuration.
type PipelineStage struct {
	// Name is a unique identifier for this stage within the pipeline.
	// Downstream stages reference it in their DependsOn list.
	Name string `yaml:"name"`

	// Workflow is the workflow to execute in this stage.
	Workflow *Workflow `yaml:"workflow"`

	// DependsOn lists stage names that must complete before this stage
	// can start. When multiple stages declare dependencies, the pipeline
	// schedules them as a DAG: stages with all dependencies satisfied
	// run concurrently. Stages without DependsOn start immediately.
	DependsOn []string `yaml:"depends_on,omitempty"`

	// Condition is an expression evaluated against the pipeline context
	// before executing this stage. If it evaluates to false, the stage
	// is skipped. The expression can reference {{stage.NAME}} to access
	// upstream stage outputs.
	// Example: "{{stage.validate.output}} == 'pass'"
	Condition string `yaml:"condition,omitempty"`

	// InputExpr is an expression that determines the input data passed
	// to this stage's workflow. If empty, the output of the last
	// dependency stage is used. Use {{stage.NAME.output}} to reference
	// specific upstream outputs.
	// Example: "{{stage.fetch.output}}"
	InputExpr string `yaml:"input_expr,omitempty"`

	// OnFailure defines the pipeline's behavior when this stage fails.
	// Default: OnFailureStop.
	OnFailure OnFailurePolicy `yaml:"on_failure,omitempty"`

	// MaxRetries is the number of retry attempts when OnFailure is
	// OnFailureRetry. Default: 3.
	MaxRetries int `yaml:"max_retries,omitempty"`

	// OnRetryExhausted defines the policy after all retries are exhausted.
	// Must be OnFailureStop or OnFailureContinue. Default: OnFailureStop.
	OnRetryExhausted OnFailurePolicy `yaml:"on_retry_exhausted,omitempty"`

	// Timeout is the per-stage timeout. If empty, the pipeline's default
	// timeout is used.
	Timeout string `yaml:"timeout,omitempty"`
}

// Pipeline chains multiple workflows into a coordinated execution graph.
// Stages are scheduled as a DAG based on their DependsOn declarations.
//
// Example YAML:
//
//	pipeline:
//	  name: data-pipeline
//	  stages:
//	    - name: fetch
//	      workflow: { ... }
//	    - name: validate
//	      workflow: { ... }
//	      depends_on: [fetch]
//	    - name: transform
//	      workflow: { ... }
//	      depends_on: [validate]
//	    - name: notify
//	      workflow: { ... }
//	      depends_on: [validate]
//	      condition: "{{stage.transform.output}} != ''"
type Pipeline struct {
	// Name is a human-readable identifier for this pipeline.
	Name string `yaml:"name"`

	// Stages is the ordered list of pipeline stages. The order in the
	// list is used for documentation only; actual execution order is
	// determined by the DependsOn DAG.
	Stages []PipelineStage `yaml:"stages"`

	// DefaultTimeout is the per-stage timeout when a stage does not
	// specify its own Timeout. Default: 5 minutes.
	DefaultTimeout string `yaml:"default_timeout,omitempty"`

	// MaxConcurrency limits the number of stages that may run
	// concurrently. 0 means unlimited.
	MaxConcurrency int `yaml:"max_concurrency,omitempty"`
}

// PipelineResult holds the outcome of a pipeline execution.
type PipelineResult struct {
	// Name is the pipeline name.
	Name string

	// StageResults maps stage name to its execution result.
	StageResults map[string]StageResult

	// Success is true if all stages completed without error.
	Success bool

	// Error is the first error that caused the pipeline to stop (if any).
	Error error

	// Duration is the total pipeline execution time.
	Duration time.Duration
}

// StageResult holds the outcome of a single pipeline stage execution.
type StageResult struct {
	// StageName is the name of the stage.
	StageName string

	// Output is the final data produced by the stage's workflow.
	Output string

	// StepResults contains per-step details from the workflow execution.
	StepResults []StepResult

	// Error is the execution error (nil on success).
	Error error

	// Skipped is true when the stage's condition evaluated to false.
	Skipped bool

	// Duration is the stage execution time.
	Duration time.Duration
}

// PipelineExecutor executes a Pipeline, managing DAG scheduling, data
// passing between stages, and failure handling.
type PipelineExecutor struct {
	registry *nodes.Registry
}

// NewPipelineExecutor creates a PipelineExecutor with the given node registry.
func NewPipelineExecutor(reg *nodes.Registry) *PipelineExecutor {
	return &PipelineExecutor{registry: reg}
}

// Execute runs the pipeline and returns the combined result.
func (pe *PipelineExecutor) Execute(ctx context.Context, p *Pipeline) (*PipelineResult, error) {
	start := time.Now()
	result := &PipelineResult{
		Name:         p.Name,
		StageResults: make(map[string]StageResult),
	}

	// Validate stage names are unique.
	seen := make(map[string]bool)
	for _, stage := range p.Stages {
		if seen[stage.Name] {
			return nil, fmt.Errorf("duplicate stage name: %s", stage.Name)
		}
		seen[stage.Name] = true
	}

	// Validate dependencies reference existing stages.
	for _, stage := range p.Stages {
		for _, dep := range stage.DependsOn {
			if !seen[dep] {
				return nil, fmt.Errorf("stage %q depends on unknown stage %q", stage.Name, dep)
			}
		}
	}

	// Build the execution DAG.
	execOrder, err := pe.resolveDAG(p)
	if err != nil {
		return nil, fmt.Errorf("pipeline DAG resolution failed: %w", err)
	}

	// Execute stages in topological order.
	completed := make(map[string]StageResult)
	remaining := make(map[string]*PipelineStage)
	for i := range p.Stages {
		remaining[p.Stages[i].Name] = &p.Stages[i]
	}

	sem := make(chan struct{}, 1)
	if p.MaxConcurrency > 1 {
		sem = make(chan struct{}, p.MaxConcurrency)
	}

	for _, batch := range execOrder {
		var wg sync.WaitGroup
		var mu sync.Mutex
		batchErrors := make([]error, 0)

		for _, stageName := range batch {
			stage, ok := remaining[stageName]
			if !ok {
				continue
			}

			// Check condition.
			if stage.Condition != "" {
				condEngine := NewExpressionEngine()
				for name, sr := range completed {
					condEngine.SetVariable("stage."+name+".output", sr.Output)
				}
				// Use the last dependency's output as the condition input.
				condInput := ""
				if len(stage.DependsOn) > 0 {
					lastDep := stage.DependsOn[len(stage.DependsOn)-1]
					if sr, ok := completed[lastDep]; ok {
						condInput = sr.Output
					}
				}
				pass, evalErr := evaluateCondition(stage.Condition, condInput, condEngine)
				if evalErr != nil {
					logger.Warn("pipeline stage condition evaluation failed",
						"pipeline", p.Name,
						"stage", stage.Name,
						"condition", stage.Condition,
						"error", evalErr,
					)
				}
				if evalErr == nil && !pass {
					result.StageResults[stage.Name] = StageResult{
						StageName: stage.Name,
						Skipped:   true,
					}
					delete(remaining, stageName)
					continue
				}
			}

			wg.Add(1)
			go func(s *PipelineStage) {
				defer wg.Done()

				if sem != nil {
					sem <- struct{}{}
					defer func() { <-sem }()
				}

				stageStart := time.Now()

				// Determine input data from upstream stages.
				input := ""
				if s.InputExpr != "" {
					engine := NewExpressionEngine()
					for name, sr := range completed {
						engine.SetVariable("stage."+name+".output", sr.Output)
					}
					var evalErr error
					input, evalErr = engine.Evaluate(s.InputExpr, "")
					if evalErr != nil {
						logger.Warn("pipeline stage input expression failed",
							"pipeline", p.Name,
							"stage", s.Name,
							"error", evalErr,
						)
					}
				} else if len(s.DependsOn) > 0 {
					// Default: use the output of the last dependency.
					lastDep := s.DependsOn[len(s.DependsOn)-1]
					if sr, ok := completed[lastDep]; ok {
						input = sr.Output
					}
				}

				// Execute the stage's workflow.
				sr, execErr := pe.executeStage(ctx, s, input, p.DefaultTimeout)

				mu.Lock()
				sr.Duration = time.Since(stageStart)
				result.StageResults[s.Name] = sr
				completed[s.Name] = sr
				if execErr != nil {
					batchErrors = append(batchErrors, execErr)
				}
				mu.Unlock()
			}(stage)
		}

		wg.Wait()

		// Handle batch errors.
		for _, bErr := range batchErrors {
			if result.Error == nil {
				result.Error = bErr
			}
		}
		if result.Error != nil {
			result.Duration = time.Since(start)
			return result, result.Error
		}
	}

	result.Success = true
	result.Duration = time.Since(start)
	return result, nil
}

// executeStage runs a single pipeline stage's workflow with retry support.
func (pe *PipelineExecutor) executeStage(ctx context.Context, stage *PipelineStage, input string, defaultTimeout string) (StageResult, error) {
	onFailure := stage.OnFailure
	if onFailure == "" {
		onFailure = OnFailureStop
	}

	maxRetries := stage.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	onRetryExhausted := stage.OnRetryExhausted
	if onRetryExhausted == "" {
		onRetryExhausted = OnFailureStop
	}

	timeout := stage.Timeout
	if timeout == "" {
		timeout = defaultTimeout
	}
	if timeout == "" {
		timeout = "5m"
	}

	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		timeoutDuration = 5 * time.Minute
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			logger.Info("retrying pipeline stage",
				"stage", stage.Name,
				"attempt", attempt,
				"max_retries", maxRetries,
			)
			time.Sleep(time.Duration(attempt) * time.Second) // linear backoff
		}

		stageCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
		output, stepResults, execErr := ExecuteWorkflow(stageCtx, stage.Workflow, pe.registry)
		cancel()

		if execErr == nil {
			return StageResult{
				StageName:   stage.Name,
				Output:      output,
				StepResults: stepResults,
			}, nil
		}

		lastErr = execErr
		logger.Warn("pipeline stage failed",
			"stage", stage.Name,
			"attempt", attempt,
			"error", execErr,
		)

		if onFailure != OnFailureRetry {
			break
		}
	}

	// All attempts exhausted.
	sr := StageResult{
		StageName: stage.Name,
		Error:     lastErr,
	}

	if onRetryExhausted == OnFailureContinue || onFailure == OnFailureContinue {
		logger.Warn("pipeline stage failed, continuing",
			"stage", stage.Name,
			"error", lastErr,
		)
		return sr, nil // continue despite failure
	}

	return sr, lastErr
}

// resolveDAG topologically sorts pipeline stages into execution batches.
// Stages in the same batch have no dependencies on each other and can run
// concurrently. Returns an error on circular dependencies.
func (pe *PipelineExecutor) resolveDAG(p *Pipeline) ([][]string, error) {
	// Build adjacency and in-degree maps.
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	for _, stage := range p.Stages {
		graph[stage.Name] = stage.DependsOn
		if _, ok := inDegree[stage.Name]; !ok {
			inDegree[stage.Name] = 0
		}
		for range stage.DependsOn {
			inDegree[stage.Name]++
		}
	}

	// Kahn's algorithm for topological sort with batching.
	var batches [][]string
	queue := make([]string, 0)

	// Start with nodes that have no dependencies.
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	processed := 0
	for len(queue) > 0 {
		batch := make([]string, len(queue))
		copy(batch, queue)
		batches = append(batches, batch)

		nextQueue := make([]string, 0)
		for _, name := range queue {
			processed++
			// Reduce in-degree for all nodes that depend on this one.
			for _, stage := range p.Stages {
				for _, dep := range stage.DependsOn {
					if dep == name {
						inDegree[stage.Name]--
						if inDegree[stage.Name] == 0 {
							nextQueue = append(nextQueue, stage.Name)
						}
					}
				}
			}
		}
		queue = nextQueue
	}

	if processed != len(p.Stages) {
		return nil, fmt.Errorf("circular dependency detected in pipeline stages")
	}

	return batches, nil
}
