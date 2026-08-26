// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌‌​​​​‌‌‌​​‌​​‌​​‌‌‌​‌​​​​​‌​​‌​​​‌‌‌‌​​‌‌​​​‌​​​​​​​​​​​​​​​​‌​​‌​​‌​‌​​‌‌‌‌‌⁠
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
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

type Workflow struct {
	Name           string            `yaml:"name,omitempty"`
	Description    string            `yaml:"description,omitempty"`
	Vars           map[string]string `yaml:"vars,omitempty"`
	Steps          []WorkflowStep    `yaml:"steps"`
	Output         string            `yaml:"output,omitempty"`          // expression for final output (default: last step output)
	InputSchema    []InputField      `yaml:"input_schema,omitempty"`    // optional input validation
	MaxConcurrency int               `yaml:"max_concurrency,omitempty"` // global concurrency limit (default: 0=unlimited)
	// Schedule is an optional cron schedule hint. The workflow engine itself
	// does not auto-schedule; this field is parsed & preserved so generated
	// workflows carry their intended cadence, and `aflare run` prints an
	// actionable hint to register it via `aflare schedule add`. (遗留修复:
	// previously a `schedule:` block in YAML was silently dropped by the
	// parser, misleading users who copied it from docs/examples.)
	Schedule *ScheduleConfig `yaml:"schedule,omitempty"`
}

// ScheduleConfig carries an optional cron expression describing how often a
// workflow should run. It is metadata: the engine does not honor it at run
// time (scheduling is done externally via `aflare schedule add`), but
// preserving it in the Workflow struct means it survives parse/save
// round-trips and the CLI can surface an activation hint.
type ScheduleConfig struct {
	Cron    string `yaml:"cron,omitempty"`
	Enabled bool   `yaml:"enabled,omitempty"`
}

// InputField defines an expected input parameter for schema validation.
type InputField struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"` // string, int, bool, json
	Required bool   `yaml:"required,omitempty"`
	Default  string `yaml:"default,omitempty"`
}

type WorkflowStep struct {
	Node            string            `yaml:"node,omitempty"`
	Name            string            `yaml:"name,omitempty"` // optional step name for {{step.name}} reference
	ID              string            `yaml:"id,omitempty"`   // alias for name (docs/dataflow.md); normalized into Name at parse time
	Params          map[string]string `yaml:"params,omitempty"`
	Condition       string            `yaml:"condition,omitempty"`
	Retry           int               `yaml:"retry,omitempty"`
	Delay           string            `yaml:"delay,omitempty"`
	Backoff         *BackoffConfig    `yaml:"backoff,omitempty"`
	Parallel        []Step            `yaml:"parallel,omitempty"`
	ContinueOnError bool              `yaml:"continue_on_error,omitempty"`
	Fallback        string            `yaml:"fallback,omitempty"`
	OnError         *Step             `yaml:"on_error,omitempty"`
	Loop            *LoopConfig       `yaml:"loop,omitempty"`
	MaxFailures     int               `yaml:"max_failures,omitempty"`
	If              *IfConfig         `yaml:"if,omitempty"`
	Map             *MapConfig        `yaml:"map,omitempty"`
	// Input explicitly overrides the step's input (default: the previous
	// step's output). It accepts a template string or a list of template
	// strings (rendered individually and joined with "\n---\n"). Templates
	// are evaluated with the same engine as params, so {{step.*}},
	// {{var.*}}, {{input}}, {{env.*}} and {{secret.*}} all resolve inside
	// an input expression. Documented in docs/dataflow.md.
	Input *StepInput `yaml:"input,omitempty"`
	// Reduce folds a list (`over`) into a single accumulated value by running
	// a sub-workflow per item with access to the running accumulator via
	// {{loop.acc}}. The sub-workflow's final output becomes the accumulator
	// for the next iteration. Reduce is inherently sequential (each iteration
	// depends on the previous accumulator), so there is no concurrency knob.
	Reduce *ReduceConfig `yaml:"reduce,omitempty"`
	// Saga runs a sequence of forward transactional steps, each with an
	// optional compensation step. If any forward step fails (after its own
	// retry/fallback/capture_error recovery), the already-completed forward
	// steps are compensated in REVERSE order. Compensation is best-effort:
	// a compensating step failure is recorded but does not abort the
	// compensation of earlier steps. The saga step's output is the last
	// successful forward step's output (or "" if none completed). This is
	// the cross-step transaction primitive — unlike continue_on_error (which
	// ignores a failure) or capture_error (which routes on a single step's
	// error), saga guarantees that partial side effects are rolled back.
	Saga *SagaConfig `yaml:"saga,omitempty"`
	// CaptureError is a sub-workflow branch executed when the step's node
	// fails. Unlike `continue_on_error` (which swallows the error) or
	// `on_error` (which runs a single handler node), capture_error treats
	// the error as a branching condition: the error message flows in as the
	// input to the first sub-step, and the branch's final output becomes the
	// step's output. The original error is preserved in StepResult for audit.
	// This is the primary "error as a value, not a crash" primitive.
	CaptureError   []WorkflowStep `yaml:"capture_error,omitempty"`
	OutputStrategy string         `yaml:"output_strategy,omitempty"` // parallel/loop/map: join(default), first, last, json_array, longest, shortest
	// DependsOn declares explicit dependencies on other steps by name or
	// 1-based index (as a string). When any step in a workflow declares
	// DependsOn, the executor switches to DAG scheduling: steps with all
	// dependencies satisfied run concurrently on a worker pool. Steps
	// without any DependsOn remain backward compatible with the original
	// sequential execution model.
	DependsOn []string `yaml:"depends_on,omitempty"`
	// Resumable marks this step as resumable: when the step fails (e.g. a
	// human_in_loop node waiting for approval), the workflow is paused rather
	// than failed. The WAL is saved and the workflow can be resumed later
	// via `aflare resume <run-id>` or a webhook.
	Resumable bool `yaml:"resumable,omitempty"`
	// Timeout is the step-level timeout for resumable steps. When set, the
	// step runs with this timeout; if it expires, the workflow is paused.
	// This is separate from the params._timeout mechanism and supports
	// durations like 72h that exceed MaxStepTimeout.
	Timeout string `yaml:"timeout,omitempty"`
	// ResumeOn controls how a paused workflow can be resumed. Supported
	// values: "webhook" (resume via HTTP webhook), "manual" (resume via
	// `aflare resume` CLI). Defaults to "manual".
	ResumeOn string `yaml:"resume_on,omitempty"`
	// WebhookToken is a secret token required to resume via webhook. When
	// ResumeOn is "webhook" and this is empty, a random token is generated.
	WebhookToken string `yaml:"webhook_token,omitempty"`
	// OutputSchema is a typed output contract for this step: a JSON Schema
	// (draft-07 subset, same validator as the structured_output node) that
	// the step's output must conform to. When set, the executor validates
	// the node output after every attempt; a violation is treated as a step
	// failure with the JSON-pointer location of the first violation, so it
	// flows through the regular retry/backoff/on_error/capture_error paths.
	// This brings NOOA-style enforced type contracts to any node without
	// wrapping it in a structured_output call. Sequential steps only;
	// DAG-mode steps (depends_on) currently ignore it.
	OutputSchema string `yaml:"output_schema,omitempty"`
	// PreviewInput replaces this step's input with a bounded preview when
	// the incoming payload exceeds PreviewMaxBytes (default 16 KiB):
	// type + total length + head/tail samples, with the middle elided. The
	// full payload is preserved in workflow state and passed untouched to
	// every other step — this is pass-by-reference for LLM steps: the model
	// sees a bounded preview, deterministic nodes still operate on the
	// complete value. Sequential steps only; DAG-mode steps (depends_on)
	// currently ignore it.
	PreviewInput bool `yaml:"preview_input,omitempty"`
}

// MapConfig configures iteration over a list of items where each item is
// processed by a sub-workflow (a sequence of steps) rather than a single
// node. Unlike Loop (which iterates a single node over a string-split
// array), Map evaluates `over` to a list and runs `steps` once per item,
// exposing the item as a loop variable. The collected per-item outputs
// are combined via OutputStrategy (default: json_array).
//
// This is the primary primitive for batch-processing structured data:
// fetching N URLs, analyzing N log lines, summarizing N documents, etc.
type MapConfig struct {
	// Over is an expression that evaluates to the items to iterate. It
	// may be a {{step.X}} reference whose output is a JSON array, a
	// newline/comma-separated list, or a {{var.X}} holding such a value.
	Over string `yaml:"over"`
	// Steps is the sub-workflow executed once per item. Inside, {{item}}
	// / {{index}} / {{count}} resolve to the current iteration context,
	// and {{step.NAME}} resolves to outputs of steps within this sub-
	// workflow (not the outer workflow).
	Steps []WorkflowStep `yaml:"steps"`
	// SplitBy, when set, splits a string `over` value by this delimiter
	// (default: "\n"). Ignored when `over` evaluates to a JSON array.
	SplitBy string `yaml:"split_by,omitempty"`
	// Concurrency limits parallel iterations (default: 1 = sequential,
	// capped at MaxParallel). Set > 1 to process items in parallel.
	Concurrency int `yaml:"concurrency,omitempty"`
	// StopOnError controls whether the first per-item failure aborts the
	// whole map (default: true). When false, failed items contribute an
	// empty string and the map continues.
	StopOnError *bool `yaml:"stop_on_error,omitempty"`
	// MaxIterations is a safety cap on the number of items (default:
	// 100, capped at 10000) to prevent runaway expansion.
	MaxIterations int `yaml:"max_iterations,omitempty"`
	// Backpressure controls how the map step handles a full work queue
	// when running concurrently. Two modes:
	//   "block" (default) — producer blocks until a consumer drains a slot,
	//     providing backpressure to the upstream step.
	//   "drop"  — producer skips the item when the queue is full, suitable
	//     for best-effort monitoring data where losing a sample is acceptable.
	// Backpressure is only meaningful when Concurrency > 1.
	Backpressure string `yaml:"backpressure,omitempty"`
	// QueueSize is the capacity of the bounded work queue (default:
	// Concurrency * 2, capped at 1000). A larger queue buffers more items
	// in memory before backpressure or drop logic kicks in.
	QueueSize int `yaml:"queue_size,omitempty"`
}

// ReduceConfig configures a left fold over a list. Each item is processed by
// a sub-workflow (Steps) that has access to the running accumulator via
// {{loop.acc}} and the current item via {{loop.item}} (also passed as the
// input to the first sub-step, matching map semantics). The sub-workflow's
// final output becomes the accumulator for the next item; the final
// accumulator is the reduce step's output.
//
// Reduce is the natural aggregation partner to map: map produces a structured
// array (one output per item), reduce folds that array into a single value
// (sum, count, top-N, merged summary, etc.). It is always sequential.
type ReduceConfig struct {
	// Over is an expression evaluating to the items to fold. Accepts a JSON
	// array or a split-by delimited string (default: "\n"), same as map.
	Over string `yaml:"over"`
	// Initial is the starting accumulator value (an expression). Defaults to
	// the empty string when omitted. Evaluated once before the first item.
	Initial string `yaml:"initial,omitempty"`
	// Steps is the sub-workflow executed once per item. {{loop.acc}} resolves
	// to the current accumulator and {{loop.item}} to the current item; the
	// last step's output is the new accumulator.
	Steps []WorkflowStep `yaml:"steps"`
	// SplitBy splits a string `over` value by this delimiter (default: "\n").
	// Ignored when `over` evaluates to a JSON array.
	SplitBy string `yaml:"split_by,omitempty"`
	// MaxIterations caps the number of items (default: 100, capped at 10000).
	MaxIterations int `yaml:"max_iterations,omitempty"`
}

// BackoffConfig configures exponential backoff for retries.
type BackoffConfig struct {
	Exponential bool   `yaml:"exponential,omitempty"` // enable exponential backoff
	Base        string `yaml:"base,omitempty"`        // base delay (default: same as delay)
	MaxDelay    string `yaml:"max_delay,omitempty"`   // max delay cap (default: MaxRetryDelay)
	Jitter      bool   `yaml:"jitter,omitempty"`      // add random jitter
}

// SagaConfig configures a cross-step saga transaction: a sequence of forward
// steps executed in order, each with an optional compensating step. On any
// forward-step failure (after the step's own recovery primitives are
// exhausted), the executor rolls back by running each completed step's
// Compensate in reverse order. Compensation is best-effort and never aborts
// earlier compensations; a compensating step that itself fails is logged and
// skipped so the rollback of prior steps still proceeds.
//
// Each SagaStep runs as a sub-workflow step (reusing executeSubStep), so it
// supports its own condition/retry/fallback/on_error/capture_error and may
// itself be an if/loop/map/reduce/parallel compound step. {{step.X}} inside a
// saga resolves to outputs of saga sub-steps (not the outer workflow), and
// {{var.*}} inherits the parent workflow's vars.
//
// This is the primitive for multi-step financial transactions (debit then
// credit then notify), distributed writes that must be rolled back on partial
// failure, and any pipeline where a mid-stream failure leaves inconsistent
// state unless earlier side effects are undone.
type SagaConfig struct {
	// Steps is the ordered list of forward transactional steps. Each is
	// executed in sequence; the output of step N becomes the input to step
	// N+1. On failure of step N, steps 1..N-1 are compensated in reverse.
	Steps []SagaStep `yaml:"steps"`
}

// SagaStep is a single forward step inside a saga, paired with an optional
// compensating step that reverses its side effects on rollback.
type SagaStep struct {
	// Forward is the transactional step to execute. It is a full WorkflowStep,
	// so it carries its own node/params/condition/retry/recovery primitives.
	Forward WorkflowStep `yaml:"forward"`
	// Compensate is the step run on rollback to undo Forward's side effects.
	// It receives the Forward step's output as its input, and {{var.error}}
	// is set to the failure that triggered the rollback. A missing Compensate
	// means the forward step has no side effects to undo (e.g. a pure read or
	// an idempotent call that is safe to leave in place). Compensate is
	// best-effort: if it fails, the failure is recorded and the rollback of
	// earlier steps continues.
	Compensate *WorkflowStep `yaml:"compensate,omitempty"`
}

// IfConfig defines an if/else branch.
type IfConfig struct {
	Condition string         `yaml:"condition"`
	Then      []WorkflowStep `yaml:"then"`
	Else      []WorkflowStep `yaml:"else,omitempty"`
}

// LoopConfig configures batch iteration over a list of items.
type LoopConfig struct {
	Items         string `yaml:"items"`
	SplitBy       string `yaml:"split_by,omitempty"`
	Var           string `yaml:"var,omitempty"`
	Concurrency   int    `yaml:"concurrency,omitempty"`
	StopOnError   *bool  `yaml:"stop_on_error,omitempty"`
	MaxIterations int    `yaml:"max_iterations,omitempty"`
}

type Step struct {
	Node      string            `yaml:"node"`
	Params    map[string]string `yaml:"params,omitempty"`
	Condition string            `yaml:"condition,omitempty"`
	Retry     int               `yaml:"retry,omitempty"`
	Delay     string            `yaml:"delay,omitempty"`
}

func (s *WorkflowStep) IsParallel() bool {
	return len(s.Parallel) > 0
}

func (s *WorkflowStep) IsLoop() bool {
	return s.Loop != nil
}

func (s *WorkflowStep) IsIf() bool {
	return s.If != nil
}

// IsMap reports whether this step is a map (sub-workflow per item) step.
func (s *WorkflowStep) IsMap() bool {
	return s.Map != nil
}

// IsReduce reports whether this step is a reduce (fold with accumulator) step.
func (s *WorkflowStep) IsReduce() bool {
	return s.Reduce != nil
}

// IsSaga reports whether this step is a saga (cross-step transaction with
// compensation) step.
func (s *WorkflowStep) IsSaga() bool {
	return s.Saga != nil
}

// HasCaptureError reports whether this step declares a capture_error branch.
func (s *WorkflowStep) HasCaptureError() bool {
	return len(s.CaptureError) > 0
}

// StepInput is a step-level `input:` override. YAML accepts either form:
//
//	input: "Users: {{step.fetch_users}}"       # template string
//	input:                                     # list of templates, joined
//	  - "{{step.fetch_users}}"                 # with "\n---\n" after
//	  - "{{step.fetch_products}}"              # rendering
//
// Both forms go through the expression engine, so all placeholders
// ({{step.*}}, {{var.*}}, {{input}}, {{env.*}}, {{secret.*}}) resolve.
type StepInput struct {
	parts []string
}

// UnmarshalYAML implements yaml.v3's unmarshaler interface, accepting a
// scalar string or a sequence of strings.
func (si *StepInput) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		si.parts = []string{node.Value}
		return nil
	case yaml.SequenceNode:
		parts := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			if item.Kind != yaml.ScalarNode {
				return fmt.Errorf("input list items must be strings (line %d)", item.Line)
			}
			parts = append(parts, item.Value)
		}
		si.parts = parts
		return nil
	default:
		return fmt.Errorf("input must be a string or a list of strings (line %d)", node.Line)
	}
}

// Parts returns the raw template parts (nil when unset).
func (si *StepInput) Parts() []string {
	if si == nil {
		return nil
	}
	return si.parts
}

// normalizeStepIDs promotes the documented `id:` alias to `name:` so
// {{step.<id>}} references, traces, and WAL records all resolve. Runs
// recursively through compound-step sub-workflows. `name:` wins when both
// are present (it is the canonical field).
func normalizeStepIDs(steps []WorkflowStep) {
	for i := range steps {
		normalizeStepID(&steps[i])
	}
}

func normalizeStepID(s *WorkflowStep) {
	if s.Name == "" && s.ID != "" {
		s.Name = s.ID
	}
	if s.If != nil {
		normalizeStepIDs(s.If.Then)
		normalizeStepIDs(s.If.Else)
	}
	if s.Map != nil {
		normalizeStepIDs(s.Map.Steps)
	}
	if s.Reduce != nil {
		normalizeStepIDs(s.Reduce.Steps)
	}
	if s.Saga != nil {
		for j := range s.Saga.Steps {
			normalizeStepID(&s.Saga.Steps[j].Forward)
			if s.Saga.Steps[j].Compensate != nil {
				normalizeStepID(s.Saga.Steps[j].Compensate)
			}
		}
	}
	normalizeStepIDs(s.CaptureError)
}

// IsResumable reports whether this step is marked as resumable.
func (s *WorkflowStep) IsResumable() bool {
	return s.Resumable
}

// GetResumeTimeout returns the step-level timeout duration for resumable steps.
// Supports durations like "72h" that exceed MaxStepTimeout. Returns 0 if not set.
func (s *WorkflowStep) GetResumeTimeout() time.Duration {
	if s.Timeout == "" {
		return 0
	}
	d, err := time.ParseDuration(s.Timeout)
	if err != nil {
		return 0
	}
	if d > 0 {
		return d
	}
	return 0
}

// GetSplitBy returns the delimiter for splitting loop items (default: newline).
func (l *LoopConfig) GetSplitBy() string {
	if l.SplitBy == "" {
		return "\n"
	}
	return l.SplitBy
}

// GetVar returns the loop variable name (default: "item").
func (l *LoopConfig) GetVar() string {
	if l.Var == "" {
		return "item"
	}
	return l.Var
}

// GetConcurrency returns the max concurrent iterations (default: 1, capped at MaxParallel).
func (l *LoopConfig) GetConcurrency() int {
	if l.Concurrency <= 0 {
		return 1
	}
	if l.Concurrency > MaxParallel {
		return MaxParallel
	}
	return l.Concurrency
}

// GetStopOnError returns whether to stop on first error (default: true).
func (l *LoopConfig) GetStopOnError() bool {
	if l.StopOnError == nil {
		return true
	}
	return *l.StopOnError
}

// GetMaxIterations returns the safety limit (default: 100, capped at 10000).
func (l *LoopConfig) GetMaxIterations() int {
	if l.MaxIterations <= 0 {
		return 100
	}
	if l.MaxIterations > 10000 {
		return 10000
	}
	return l.MaxIterations
}

// GetSplitBy returns the delimiter for splitting map items (default: newline).
func (m *MapConfig) GetSplitBy() string {
	if m.SplitBy == "" {
		return "\n"
	}
	return m.SplitBy
}

// GetConcurrency returns the max concurrent iterations (default: 1, capped at MaxParallel).
func (m *MapConfig) GetConcurrency() int {
	if m.Concurrency <= 0 {
		return 1
	}
	if m.Concurrency > MaxParallel {
		return MaxParallel
	}
	return m.Concurrency
}

// GetStopOnError returns whether to stop on first item failure (default: true).
func (m *MapConfig) GetStopOnError() bool {
	if m.StopOnError == nil {
		return true
	}
	return *m.StopOnError
}

// GetMaxIterations returns the safety cap (default: 100, capped at 10000).
func (m *MapConfig) GetMaxIterations() int {
	if m.MaxIterations <= 0 {
		return 100
	}
	if m.MaxIterations > 10000 {
		return 10000
	}
	return m.MaxIterations
}

// GetBackpressure returns the backpressure mode: "block" or "drop" (default: "block").
func (m *MapConfig) GetBackpressure() string {
	switch m.Backpressure {
	case "drop":
		return "drop"
	default:
		return "block"
	}
}

// GetQueueSize returns the bounded work queue capacity (default: Concurrency * 2, capped at 1000).
func (m *MapConfig) GetQueueSize() int {
	concurrency := m.GetConcurrency()
	if concurrency <= 1 {
		return 1
	}
	if m.QueueSize <= 0 {
		n := concurrency * 2
		if n > 1000 {
			return 1000
		}
		return n
	}
	if m.QueueSize > 1000 {
		return 1000
	}
	return m.QueueSize
}

// GetSplitBy returns the delimiter for splitting reduce items (default: newline).
func (r *ReduceConfig) GetSplitBy() string {
	if r.SplitBy == "" {
		return "\n"
	}
	return r.SplitBy
}

// GetMaxIterations returns the safety cap (default: 100, capped at 10000).
func (r *ReduceConfig) GetMaxIterations() int {
	if r.MaxIterations <= 0 {
		return 100
	}
	if r.MaxIterations > 10000 {
		return 10000
	}
	return r.MaxIterations
}

func (s *WorkflowStep) GetTimeout() time.Duration {
	if timeout, ok := s.Params["_timeout"]; ok {
		d, err := time.ParseDuration(timeout)
		if err == nil && d > 0 {
			return d
		}
	}
	return 0
}

func (s *WorkflowStep) GetRetryCount() int {
	if s.Retry < 0 {
		return 0
	}
	return s.Retry
}

func (s *WorkflowStep) GetRetryDelay() time.Duration {
	if s.Delay == "" {
		return 1 * time.Second
	}
	d, err := time.ParseDuration(s.Delay)
	if err != nil {
		return 1 * time.Second
	}
	return d
}

// GetBackoffDelay computes the retry delay for a given attempt using backoff config.
// attempt is 1-indexed (1 = first retry).
func (s *WorkflowStep) GetBackoffDelay(attempt int) time.Duration {
	baseDelay := s.GetRetryDelay()
	if s.Backoff == nil || !s.Backoff.Exponential || attempt <= 1 {
		return baseDelay
	}

	// Parse custom base if provided, capped at MaxRetryDelay
	if s.Backoff.Base != "" {
		if d, err := time.ParseDuration(s.Backoff.Base); err == nil && d > 0 {
			if d > MaxRetryDelay {
				d = MaxRetryDelay
			}
			baseDelay = d
		}
	}

	// Determine max delay cap
	maxDelay := MaxRetryDelay
	if s.Backoff.MaxDelay != "" {
		if d, err := time.ParseDuration(s.Backoff.MaxDelay); err == nil && d > 0 {
			if d > MaxRetryDelay {
				d = MaxRetryDelay
			}
			maxDelay = d
		}
	}

	// Exponential: base * 2^(attempt-1), with overflow protection
	delay := baseDelay
	for i := 1; i < attempt; i++ {
		// Check for overflow before multiplying
		if delay > maxDelay/2 {
			delay = maxDelay
			break
		}
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
			break
		}
	}

	// Jitter: add up to 25% random variation
	if s.Backoff.Jitter && delay > 0 {
		delay = time.Duration(float64(delay) * (0.75 + 0.25*pseudoRand()))
	}

	return delay
}

// GetTimeout returns the per-step timeout from params._timeout, defaulting to 0 (no timeout)
func (s *Step) GetTimeout() time.Duration {
	if timeout, ok := s.Params["_timeout"]; ok {
		d, err := time.ParseDuration(timeout)
		if err == nil && d > 0 {
			return d
		}
	}
	return 0
}

// GetRetryCount returns the retry count, defaulting to 0
func (s *Step) GetRetryCount() int {
	if s.Retry < 0 {
		return 0
	}
	return s.Retry
}

// GetRetryDelay returns the delay between retries, defaulting to 1 second
func (s *Step) GetRetryDelay() time.Duration {
	if s.Delay == "" {
		return 1 * time.Second
	}
	d, err := time.ParseDuration(s.Delay)
	if err != nil {
		return 1 * time.Second
	}
	return d
}
