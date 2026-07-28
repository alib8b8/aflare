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
	"context"
	"sync"

	"github.com/alib8b8/llm-box/internal/nodes"
)

// llmCallCollector is both an LLMCallSink (B-2) and a RouterDecisionSink
// (B-3). It accumulates every telemetry record and routing decision
// published during a single step's execution (including all retry attempts).
// It is safe for concurrent use: a step's sub-calls (e.g. parallel loop
// iterations) may publish in parallel.
//
// The collector is scoped to one step: a fresh collector is created per
// step attempt set, attached to the step's context, drained after the step
// finishes, and projected into StepTrace.LLM / StepTrace.Router.
type llmCallCollector struct {
	mu        sync.Mutex
	calls     []nodes.LLMCallTelemetry
	decisions []nodes.RouterDecision
}

// newLLMCallCollector returns an empty collector.
func newLLMCallCollector() *llmCallCollector {
	return &llmCallCollector{}
}

// RecordLLMCall implements nodes.LLMCallSink.
func (c *llmCallCollector) RecordLLMCall(t nodes.LLMCallTelemetry) {
	c.mu.Lock()
	c.calls = append(c.calls, t)
	c.mu.Unlock()
}

// RecordRouterDecision implements nodes.RouterDecisionSink.
func (c *llmCallCollector) RecordRouterDecision(d nodes.RouterDecision) {
	c.mu.Lock()
	c.decisions = append(c.decisions, d)
	c.mu.Unlock()
}

// drainCalls returns the accumulated LLM calls and resets that slot.
func (c *llmCallCollector) drainCalls() []nodes.LLMCallTelemetry {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.calls
	c.calls = nil
	return out
}

// drainDecisions returns the accumulated router decisions and resets that
// slot. Returns nil if none were collected.
func (c *llmCallCollector) drainDecisions() []nodes.RouterDecision {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.decisions
	c.decisions = nil
	return out
}

// withLLMCollector returns a new context derived from ctx that carries a
// fresh collector (as both LLMCallSink and RouterDecisionSink), plus the
// collector itself. LLM and router nodes descended from the returned
// context will publish to the collector.
func withLLMCollector(ctx context.Context) (context.Context, *llmCallCollector) {
	c := newLLMCallCollector()
	ctx = nodes.WithLLMCallSink(ctx, c)
	ctx = nodes.WithRouterDecisionSink(ctx, c)
	return ctx, c
}

// projectLLMTelemetry converts the collected nodes.LLMCallTelemetry records
// into the workflow-local LLMStepTrace form, stamping each with its 1-based
// Attempt index (call order within the step). Returns nil if calls is empty
// so that non-LLM steps keep StepTrace.LLM == nil.
func projectLLMTelemetry(calls []nodes.LLMCallTelemetry) []LLMStepTrace {
	if len(calls) == 0 {
		return nil
	}
	out := make([]LLMStepTrace, len(calls))
	for i, c := range calls {
		var pt, ct, tt int
		if c.Usage != nil {
			pt = c.Usage.PromptTokens
			ct = c.Usage.CompletionTokens
			tt = c.Usage.TotalTokens
		}
		out[i] = LLMStepTrace{
			NodeName:         c.NodeName,
			Provider:         c.Provider,
			Model:            c.Model,
			Endpoint:         c.Endpoint,
			Latency:          c.Latency,
			Attempt:          i + 1,
			Stream:           c.Stream,
			StatusCode:       c.StatusCode,
			ErrText:          c.ErrText,
			PromptTokens:     pt,
			CompletionTokens: ct,
			TotalTokens:      tt,
			CostUSD:          c.CostUSD,
		}
	}
	return out
}

// projectRouterDecisions projects the collected router decisions into the
// workflow-local form. Returns nil if none were collected. If a step
// published multiple decisions (unusual but possible with nested routers),
// the LAST one is used — it reflects the final outcome of the step.
func projectRouterDecisions(decisions []nodes.RouterDecision) *RouterDecisionTrace {
	if len(decisions) == 0 {
		return nil
	}
	d := decisions[len(decisions)-1]
	attempts := make([]RouterAttemptTrace, len(d.Attempts))
	for i, a := range d.Attempts {
		attempts[i] = RouterAttemptTrace{
			Provider:  a.Provider,
			Success:   a.Success,
			Error:     a.Error,
			LatencyMs: a.LatencyMs,
		}
	}
	return &RouterDecisionTrace{
		Strategy:   d.Strategy,
		Candidates: d.Candidates,
		Selected:   d.Selected,
		Attempts:   attempts,
		FinalError: d.FinalError,
	}
}
