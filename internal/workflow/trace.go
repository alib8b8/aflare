// Copyright (c) 2026 llm-box Contributors
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

package workflow

import (
	"time"
)

// WorkflowTrace captures detailed, per-step execution telemetry for a single
// workflow run. It is returned by ExecuteWorkflowWithTrace and is intended for
// observability, debugging, and benchmark validation (A-6).
//
// A trace is cheap to collect: it records only timestamps, durations, indices
// and short status strings — never full step payloads. StepResult.Trace points
// at the same StepTrace recorded in WorkflowTrace.Steps, so callers can read
// per-step telemetry from whichever structure they already hold.
type WorkflowTrace struct {
	Name      string        // workflow name
	Mode      string        // "sequential" or "dag"
	StartedAt time.Time     // run start
	EndedAt   time.Time     // run end
	Duration  time.Duration // total wall-clock duration
	Steps     []StepTrace   // one entry per recorded step result
	Batches   []BatchTrace  // DAG topological batches; nil for sequential mode
}

// BatchTrace records one topological batch of a DAG run. Steps within a batch
// have no mutual dependencies and execute concurrently.
type BatchTrace struct {
	Index       int   // 0-based batch index
	StepIndices []int // step indices executed in this batch
	StartedAt   time.Time
	Duration    time.Duration // batch wall-clock duration
}

// StepTrace records per-step execution detail that supplements StepResult. It
// distinguishes the time spent in expression/condition evaluation from the time
// spent inside node.Execute, and captures DAG scheduling metadata (batch,
// dependencies), retry attempts, and error-recovery actions.
type StepTrace struct {
	Index           int           // 0-based step index
	NodeName        string        // node type name
	StepName        string        // declared step name (may be empty)
	BatchIndex      int           // DAG batch the step ran in; -1 for sequential
	Dependencies    []int         // step indices this step depended on (DAG only)
	Skipped         bool          // condition evaluated false
	ConditionExpr   string        // condition expression, if any
	ConditionPassed bool          // condition result (true if no condition)
	Attempts        int           // retry attempts actually made (>=1)
	Recoveries      []string      // recovery actions applied, e.g. ["fallback","on_error"]
	EvalDuration    time.Duration // condition + param evaluation
	ExecuteDuration time.Duration // node.Execute, including retries
	TotalDuration   time.Duration // end-to-end for this step
	InputLen        int           // length of step input in bytes
	OutputLen       int           // length of step output in bytes
	ErrorText       string        // error text, if the step failed
	// LLM holds per-call LLM telemetry for steps whose node published it
	// (B-2). One entry per LLM call, in call order — a retried LLM step
	// with 2 attempts yields 2 entries. Nil for non-LLM steps or when no
	// sink was attached to the run context.
	LLM []LLMStepTrace
	// Router holds the routing decision for steps whose node was an LLM
	// router that published a decision (B-3). Nil for non-router steps
	// or when no sink was attached. A router step typically also has a
	// non-nil LLM slice (the per-call telemetry from B-2 for each
	// provider attempt).
	Router *RouterDecisionTrace
}

// RouterDecisionTrace is the workflow-side projection of an LLM router
// decision. It mirrors nodes.RouterDecision without forcing trace readers
// to import the nodes package.
type RouterDecisionTrace struct {
	Strategy   string               // routing strategy used
	Candidates []string             // provider names in try order
	Selected   string               // provider that produced the final response; "" if all failed
	Attempts   []RouterAttemptTrace // one entry per provider tried
	FinalError string               // error text if all providers failed; "" on success
}

// RouterAttemptTrace mirrors nodes.RouterAttempt.
type RouterAttemptTrace struct {
	Provider  string // provider name
	Success   bool   // whether this provider produced the final response
	Error     string // error text on failure; "" on success
	LatencyMs int64  // wall-clock duration of this provider attempt
}

// LLMStepTrace is the workflow-side projection of an LLM call. It mirrors
// the fields of nodes.LLMCallTelemetry that matter to workflow consumers;
// keeping a workflow-local copy avoids forcing trace readers to import
// the nodes/core packages.
type LLMStepTrace struct {
	NodeName         string        // node type name, e.g. "openai"
	Provider         string        // human-readable provider name
	Model            string        // resolved model name actually sent
	Endpoint         string        // resolved endpoint URL
	Latency          time.Duration // end-to-end call wall time
	Attempt          int           // 1-based attempt index within the step's retry loop
	Stream           bool          // whether this was a streaming call
	StatusCode       int           // HTTP status code; 0 if request never reached server
	ErrText          string        // error text, empty on success
	PromptTokens     int           // from provider usage, 0 if omitted
	CompletionTokens int           // from provider usage, 0 if omitted
	TotalTokens      int           // from provider usage, 0 if omitted
	CostUSD          float64       // optional cost estimate, 0 if not computed
}

// newTrace creates a WorkflowTrace initialised with the given mode and start
// time. stepCount pre-allocates the Steps slice so that pointers returned by
// recordStep remain stable across all appends (no underlying-array reallocation).
func newTrace(name, mode string, startedAt time.Time, stepCount int) *WorkflowTrace {
	return &WorkflowTrace{Name: name, Mode: mode, StartedAt: startedAt, Steps: make([]StepTrace, 0, stepCount)}
}

// finish stamps the trace end time and total duration.
func (t *WorkflowTrace) finish(endedAt time.Time) {
	if t == nil {
		return
	}
	t.EndedAt = endedAt
	t.Duration = endedAt.Sub(t.StartedAt)
}

// recordStep appends a StepTrace and returns a pointer to the stored copy so
// the caller can also attach it to a StepResult.
func (t *WorkflowTrace) recordStep(st StepTrace) *StepTrace {
	if t == nil {
		return nil
	}
	t.Steps = append(t.Steps, st)
	return &t.Steps[len(t.Steps)-1]
}
