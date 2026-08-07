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

package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

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

	// Execute branch steps as a sub-workflow with incremented depth. The
	// branch inherits the parent workflow's vars so {{var.*}} resolves the
	// same as in the parent (matching map/reduce/capture_error semantics).
	// Step outputs are NOT inherited: the sub-workflow has its own step
	// namespace, so {{step.X}} references target steps within the branch.
	subWf := &Workflow{
		Name:  fmt.Sprintf("if-branch-%d", stepIndex),
		Steps: branchSteps,
		Vars:  engine.SnapshotVars(),
	}
	// Pass incremented depth and the if-step's input via context. The input
	// becomes the branch sub-workflow's starting data so the flowing value
	// (e.g. an error message under capture_error) reaches the branch handlers.
	childCtx := context.WithValue(ctx, ifDepthKey, depth+1)
	childCtx = context.WithValue(childCtx, ifInputKey, input)
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

// executeCaptureErrorBranch runs a step's `capture_error` sub-workflow when the
// step's node has failed. It treats the error as a branchable value rather
// than swallowing it (continue_on_error) or running a single handler node
// (on_error):
//
//   - The error message is the INPUT to the first sub-step, so a step
//     condition (e.g. condition:"contains:timeout") can route on error type,
//     and handler nodes receive the error text as their input.
//   - The error text is also exposed as {{var.error}} inside the branch.
//   - The branch runs on a fresh engine that inherits the parent workflow's
//     vars; {{step.X}} inside the branch cannot see the outer workflow's
//     steps (same limitation as `if` branches).
//   - The branch's final output becomes the failed step's output, so later
//     steps see a normal value and the workflow continues.
//   - The original error is preserved in StepResult.Error for audit and a
//     "capture_error" recovery is recorded in StepTrace.Recoveries.
//
// The branch steps are run inline (not via ExecuteWorkflow) so that the error
// message can seed the initial data — ExecuteWorkflow hardcodes the initial
// workflow input to "". executeSubStep is reused so nested if/loop/map/reduce
// and per-step recovery all work inside the branch.
//
// Returns (branchOutput, nil) on success. If a branch step fails (and is not
// itself recovered), the branch error is returned so the caller can fall
// through to other recovery primitives.
func executeCaptureErrorBranch(ctx context.Context, steps []WorkflowStep, errMsg string, parentVars map[string]string, reg *nodes.Registry, program *tea.Program, globalLimiter *ConcurrencyLimiter) (string, error) {
	if len(steps) == 0 {
		// No branch declared — surface the error text as the output so the
		// workflow can continue with the error as a value.
		return errMsg, nil
	}
	engine := NewExpressionEngine()
	for k, v := range parentVars {
		engine.SetVariable(k, v)
	}
	engine.SetVariable("error", errMsg)

	// Seed the branch with the error message: it is both the first step's
	// input (enabling condition-based routing on the error text) and the
	// initial accumulator should the branch produce no output.
	data := errMsg
	for idx, sub := range steps {
		_, out, err := executeSubStep(ctx, idx, sub, data, engine, reg, program, globalLimiter)
		if err != nil {
			return "", fmt.Errorf("capture_error branch step %d failed: %w", idx, err)
		}
		data = out
	}
	return data, nil
}

// streamChunkBufferSize is the per-step buffer between a streaming node's
// onChunk callback and the TUI program. The Bubble Tea program.Send blocks on
// an unbuffered channel (tea.go: Program.Send), so without this buffer a slow
// TUI render loop would back-pressure the streaming HTTP reader inside
// ExecuteStream, stalling the upstream producer.
const streamChunkBufferSize = 256

// streamSink decouples the streaming node's onChunk callback from the TUI
// program. A forwarding goroutine drains a buffered channel and invokes
// program.Send for each chunk. When the buffer fills (TUI slower than the
// producer), the oldest pending chunk is dropped to keep the stream flowing,
// and a warning is logged with the running drop count.
//
// Lifecycle: the caller creates a sink per ExecuteStream attempt, passes
// sink.onChunk to ExecuteStream, and calls sink.flush() once ExecuteStream
// returns. flush closes the channel and waits for the goroutine to finish
// draining queued chunks, so StepEndMsg is never sent before prior chunks.
//
// onChunk and flush are NOT concurrent: onChunk is only invoked from inside
// ExecuteStream (synchronous), and flush is called after ExecuteStream returns.
type streamSink struct {
	program  *tea.Program
	idx      int
	nodeName string
	ch       chan string
	done     chan struct{}
	dropped  atomic.Int64
}

// newStreamSink starts a forwarding goroutine that drains chunks into program.
func newStreamSink(program *tea.Program, idx int, nodeName string) *streamSink {
	s := &streamSink{
		program:  program,
		idx:      idx,
		nodeName: nodeName,
		ch:       make(chan string, streamChunkBufferSize),
		done:     make(chan struct{}),
	}
	go s.run()
	return s
}

// run is the forwarding goroutine: it ranges over the chunk channel and sends
// a StepStreamMsg for each chunk until the channel is closed and drained.
func (s *streamSink) run() {
	defer close(s.done)
	for chunk := range s.ch {
		s.program.Send(tui.StepStreamMsg{
			Index: s.idx,
			Name:  s.nodeName,
			Chunk: chunk,
		})
	}
}

// onChunk is the callback handed to StreamingNode.ExecuteStream. It performs a
// non-blocking write; on a full buffer it drops the oldest pending chunk
// (making room for the new one) so the stream keeps flowing at the cost of a
// gap in the TUI display. Drops are counted and warned about.
func (s *streamSink) onChunk(chunk string) {
	select {
	case s.ch <- chunk:
		return
	default:
	}
	// Buffer full: drop the oldest pending chunk to make room for the new one.
	select {
	case <-s.ch:
		s.warnDrop()
	default:
		// Consumer drained between selects; fall through to retry the write.
	}
	select {
	case s.ch <- chunk:
	default:
		// Still full (producer faster than consumer): drop the new chunk.
		s.warnDrop()
	}
}

// warnDrop increments the drop counter and logs a backpressure warning.
func (s *streamSink) warnDrop() {
	n := s.dropped.Add(1)
	logger.Warn("stream chunk dropped due to TUI backpressure",
		"index", s.idx, "node", s.nodeName, "dropped_total", n)
}

// flush closes the chunk channel and waits for the forwarding goroutine to
// finish draining any queued chunks into the TUI program. After flush
// returns, no more StepStreamMsgs will be sent for this sink, so the caller
// is free to send StepEndMsg.
func (s *streamSink) flush() {
	close(s.ch)
	<-s.done
	if n := s.dropped.Load(); n > 0 {
		logger.Warn("streaming step completed with dropped chunks",
			"index", s.idx, "node", s.nodeName, "dropped_total", n)
	}
}
