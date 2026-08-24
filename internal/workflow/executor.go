// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌​‌‌​​‌‌‌‌‌​​‌‌‌​‌​​​​‌​​​​​‌​​​​‌​​​‌‌‌​‌​​‌​​​​​​​​​​​​​​​​​​​​​​‌‌‌​‌​‌‌‌‌​⁠
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

package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/nodes"
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
type ifDepthKeyType struct{}

var ifDepthKey = ifDepthKeyType{}

// ifInputKey carries the initial input a branch sub-workflow should start
// from. It is set by executeIfBranch so the chosen then/else branch receives
// the same data the if-step did (instead of starting from an empty string),
// which lets capture_error route on the error text and lets normal if-steps
// keep processing the flowing data. Top-level workflow executions never set
// it, so they default to an empty initial input.
type ifInputKeyType struct{}

var ifInputKey = ifInputKeyType{}

// StepProgressStatus enumerates the lifecycle events emitted via
// StepProgressFunc (断点13: 实时进度输出).
const (
	StepProgressStarted   = "started"
	StepProgressCompleted = "completed"
	StepProgressFailed    = "failed"
	StepProgressSkipped   = "skipped"
)

// StepProgressEvent is passed to StepProgressFunc for each step lifecycle
// event, enabling real-time progress output in the CLI (断点13).
type StepProgressEvent struct {
	Index    int           // 0-based step index
	Total    int           // total number of steps in the workflow
	NodeName string        // node type (e.g. "http_request")
	StepName string        // human-readable step name (may be empty)
	Status   string        // one of StepProgress*
	Duration time.Duration // only meaningful for completed/failed/skipped
	Error    error         // only meaningful for failed
}

// StepProgressFunc is a callback invoked at each step lifecycle event
// (started/completed/failed/skipped). It is called synchronously from the
// executor goroutine, so implementations must be non-blocking.
type StepProgressFunc func(ev StepProgressEvent)

// StepResult stores the result of executing a single step
type StepResult struct {
	StepIndex int
	NodeName  string
	Input     string
	Output    string
	Error     error
	Duration  time.Duration
	// Trace holds detailed per-step telemetry (eval/exec timings, retries,
	// recoveries, DAG batch/dependencies). It is nil when tracing is not
	// requested via ExecuteWorkflowWithTrace.
	Trace *StepTrace
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

// ExecuteWorkflowWithTUI executes the workflow and sends messages to a TUI program.
// It is a thin wrapper around ExecuteWorkflowWithTrace that discards the trace.
func ExecuteWorkflowWithTUI(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program) (string, []StepResult, error) {
	output, results, _, err := ExecuteWorkflowWithTrace(ctx, wf, reg, program)
	return output, results, err
}

// ExecuteWorkflowWithTrace executes the workflow and returns a detailed per-step
// WorkflowTrace alongside the standard results.
//
// Routing: when any step declares depends_on, the DAG scheduling path is used
// (topological batching + concurrent execution); otherwise the legacy sequential
// for-loop path runs. Both paths populate the trace with the same StepTrace
// schema — BatchIndex is -1 and Dependencies is nil in sequential mode.
//
// This entry point does not use checkpoint/resume. To enable per-step
// checkpointing and resume, build an Executor via NewExecutor().WithCheckpoint(...).
//
// This legacy entry point applies DefaultWorkflowTimeout. To configure a
// different timeout, build an Executor via NewExecutor().WithTimeout(d)
// (per-executor, no global mutable state).
func ExecuteWorkflowWithTrace(ctx context.Context, wf *Workflow, reg *nodes.Registry, program *tea.Program) (string, []StepResult, *WorkflowTrace, error) {
	if hasDAGDeclarations(wf.Steps) {
		out, results, trace, err := executeWorkflowDAG(ctx, wf, reg, program, DefaultWorkflowTimeout)
		recordWorkflowMetrics(trace, err)
		return out, results, trace, err
	}
	out, results, trace, err := executeWorkflowSequential(ctx, wf, reg, program, "", "", "", DefaultWorkflowTimeout, nil)
	recordWorkflowMetrics(trace, err)
	return out, results, trace, err
}

// Executor is a configurable workflow runner. It wraps the package-level
// ExecuteWorkflow* functions and adds optional checkpoint/resume support.
//
// Build one with NewExecutor and chain WithCheckpoint to enable persistence:
//
//	exec := NewExecutor().WithCheckpoint("~/.aflare/checkpoints/wf.json")
//	out, results, err := exec.Execute(ctx, wf, reg)
//
// When statePath is set and a checkpoint file already exists, execution
// resumes from the step after the one recorded in the checkpoint. After each
// sequential step completes, a fresh snapshot is written to statePath.
//
// Checkpoint/resume is only supported on the sequential execution path.
// Workflows that declare depends_on (DAG mode) ignore statePath.
type Executor struct {
	statePath       string
	walPath         string // when set, use append-only WAL instead of JSON checkpoint
	workflowTimeout time.Duration
	// auditEnabled turns on tamper-evident audit logging of every workflow
	// execution (start, per-step, completion/failure) into the history
	// package's HMAC hash-chain audit log. Off by default.
	auditEnabled bool
	// auditDir, when non-empty, overrides the history/audit directory used
	// for the audit log. When empty the history package default
	// (~/.config/aflare/history) is used.
	auditDir string
	// idempotencyKey, when non-empty, activates workflow idempotency: before
	// executing, the Executor consults idempotencyStore for a prior
	// completed run with this key and returns the cached result on a hit so
	// side-effecting nodes (HTTP POST transfers, file writes, ...) are not
	// re-run. Empty = idempotency off (default, backward-compatible).
	idempotencyKey string
	// idempotencyStore holds the key→run_id ledger. It is auto-instantiated
	// to a FileIdempotencyStore at DefaultIdempotencyDir() by
	// WithIdempotencyKey when nil, and can be overridden via
	// WithIdempotencyStore (mainly for tests).
	idempotencyStore IdempotencyStore
	// wfPath is the original workflow file path, used for pause-resume
	// metadata when a resumable step is paused.
	wfPath string
	// progressCB (断点13) is an optional CLI progress callback invoked at
	// each step lifecycle event. nil disables progress output.
	progressCB StepProgressFunc
	// wg tracks in-flight executions so Shutdown can wait for all running
	// steps to complete before returning.
	wg sync.WaitGroup
}

// NewExecutor returns an Executor with no checkpoint configured and the
// workflow timeout initialized to DefaultWorkflowTimeout. Use WithTimeout to
// override the timeout and WithCheckpoint to enable checkpoint/resume.
func NewExecutor() *Executor {
	return &Executor{
		workflowTimeout: DefaultWorkflowTimeout,
	}
}
