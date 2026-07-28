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
	StartedAt time.Time    // run start
	EndedAt   time.Time    // run end
	Duration  time.Duration // total wall-clock duration
	Steps     []StepTrace  // one entry per recorded step result
	Batches   []BatchTrace // DAG topological batches; nil for sequential mode
}

// BatchTrace records one topological batch of a DAG run. Steps within a batch
// have no mutual dependencies and execute concurrently.
type BatchTrace struct {
	Index       int           // 0-based batch index
	StepIndices []int         // step indices executed in this batch
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
