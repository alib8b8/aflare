// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌​​​​​​​‌​​‌​‌‌​‌​​​‌‌‌​‌​‌‌‌​​​‌​​​‌‌‌‌‌​​‌​‌​​​​​​​​​​​​​​​​‌‌‌​​‌‌​​​‌‌​​​‌⁠
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
	"os"
	"regexp"
	"sync"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/telemetry"
)

// traceRedactExtras holds secret patterns that core.RedactSensitive does not
// yet cover (JWT, PEM private key blocks). These are applied BEFORE
// nodes.RedactSensitive so the secret is scrubbed across the full input;
// nodes.RedactSensitive then runs its own patterns and truncates the result.
// Adding them here (rather than in security.go) keeps the change local to the
// trace-persistence path and avoids modifying the core security surface.
var traceRedactExtras = []struct {
	pattern *regexp.Regexp
	replace string
}{
	// JWT: three dot-separated base64url segments, the first two starting
	// with "eyJ" (base64url for `{"`).
	{regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`), "[REDACTED:JWT]"},
	// PEM private key block (RSA / EC / OPENSSH / GENERIC ... PRIVATE KEY).
	// [\s\S] matches newlines so the whole block is captured non-greedily.
	{regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z0-9 ]*PRIVATE KEY-----`), "[REDACTED:PRIVATE_KEY]"},
}

// redactForTrace scrubs s of known secret patterns before it is persisted
// into a StepTrace. It delegates the bulk of the work to
// nodes.RedactSensitive (which handles Bearer tokens, API keys, GitHub/Slack
// tokens, URL credentials, etc., and truncates output to 1000 chars), and
// additionally covers JWT and PEM private key blocks that the core helper
// does not yet recognise.
//
// Redaction is on by default (it is a safety control). Bypassing it requires
// BOTH AFLARE_TRACE_NO_REDACT=1 AND AFLARE_DEBUG_MODE=1 — a dual control so
// a single misconfigured env var in production cannot leak prompts (which may
// carry PII, API keys, or card numbers) into trace files. This is intended
// only for local debugging of trace content; never enable in production.
func redactForTrace(s string) string {
	if s == "" {
		return s
	}
	traceNoRedactWarnOnce.Do(warnTraceNoRedactIfEnabled)
	if traceRedactDisabled() {
		return s
	}
	for _, p := range traceRedactExtras {
		s = p.pattern.ReplaceAllString(s, p.replace)
	}
	return nodes.RedactSensitive(s)
}

// traceRedactDisabled reports whether the AFLARE_TRACE_NO_REDACT escape
// hatch is active. As a production-safety dual control, BOTH
// AFLARE_TRACE_NO_REDACT=1 AND AFLARE_DEBUG_MODE=1 must be set to bypass
// redaction; if either is missing, redaction stays on. A production
// environment typically would not set both, so an accidentally-set
// AFLARE_TRACE_NO_REDACT alone does not leak sensitive data.
//
// L-10: os.Getenv is read on every redactForTrace call (i.e. once per LLM
// call) rather than cached in an atomic.Bool via sync.Once. This is
// intentional, not an oversight:
//   - The cost is negligible. os.Getenv is a cheap, lock-free lookup into
//     the process environment block (a couple of hundred nanoseconds); it
//     is dwarfed by the LLM round-trip that redactForTrace is invoked on.
//   - Caching would break the test contract. Multiple tests in this
//     package flip AFLARE_TRACE_NO_REDACT / AFLARE_DEBUG_MODE via
//     t.Setenv and rely on the next redactForTrace call observing the new
//     value immediately. A sync.Once cache (or an atomic.Bool snapshot)
//     would freeze the first observed value for the lifetime of the
//     process, forcing every such test to call a resetTraceRedactCache()
//     helper — added complexity for no measurable production win.
//   - The env vars are operator-controlled escape hatches, not hot
//     configuration; they are set at process start and never flipped in
//     production. Per-call reads cost nothing in the steady state.
//
// If a future change makes this hot, prefer a sync.Once + atomic.Bool
// cache plus a test-only reset helper over an unconditional cache.
func traceRedactDisabled() bool {
	if os.Getenv("AFLARE_TRACE_NO_REDACT") != "1" {
		return false
	}
	if os.Getenv("AFLARE_DEBUG_MODE") != "1" {
		return false // dual control: debug mode must also be on
	}
	return true
}

// traceNoRedactWarnOnce ensures the AFLARE_TRACE_NO_REDACT safety warning is
// emitted at most once per process, on the first redactForTrace call. Using
// sync.Once (rather than init) means tests that flip the env var via t.Setenv
// and then call redactForTrace trigger the warning based on the env value at
// first-call time, while production gets exactly one prominent heads-up per
// startup.
var traceNoRedactWarnOnce sync.Once

// warnTraceNoRedactIfEnabled logs a prominent warning when the trace redaction
// escape hatch is (or would be) active. When AFLARE_TRACE_NO_REDACT=1 is set
// but AFLARE_DEBUG_MODE!=1, redaction remains enabled (the dual control held)
// and the operator is told so; when both are set, redaction is actually
// disabled and the operator is warned that sensitive data will be logged.
func warnTraceNoRedactIfEnabled() {
	if os.Getenv("AFLARE_TRACE_NO_REDACT") != "1" {
		return
	}
	if os.Getenv("AFLARE_DEBUG_MODE") != "1" {
		logger.Warn("AFLARE_TRACE_NO_REDACT=1 set but AFLARE_DEBUG_MODE!=1, redaction remains enabled (production safety)")
		return
	}
	logger.Warn("AFLARE_TRACE_NO_REDACT=1 AND AFLARE_DEBUG_MODE=1 — SENSITIVE DATA WILL BE LOGGED, DO NOT USE IN PRODUCTION")
}

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
	// Wrap the collector with an OTel sink so every LLM call produces a span
	// with model, provider, token count, latency, and cost. When the global
	// tracer provider is not set, the wrapper is a no-op (single bool check).
	sink := nodes.LLMCallSink(&telemetry.OtelLLMCallSink{Inner: c})
	ctx = nodes.WithLLMCallSink(ctx, sink)
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
		// Prefer the upstream Attempt (stamped by the router/executor
		// retry loop) over the slice position i+1. Under parallel LLM
		// calls (e.g. a map node fanning out, or a router trying multiple
		// providers concurrently) the slice order is non-deterministic, so
		// i+1 would mislabel which attempt a telemetry record belongs to.
		// Fall back to i+1 only when the upstream left Attempt at 0 (the
		// single-call case that never set it) so existing traces keep a
		// sensible 1-based index (M-11).
		attempt := c.Attempt
		if attempt == 0 {
			attempt = i + 1
		}
		// Cost attribution: prefer an upstream-supplied CostUSD (e.g. a
		// router that computed pricing from its own table) over the
		// centralised price table. Only when the upstream left it 0 do we
		// compute from token usage + the built-in price table, so a
		// router's authoritative cost is never overwritten by a stale
		// default. This fills the field that recordWorkflowMetrics and
		// WorkflowTrace.TotalCostUSD already consume but that was
		// previously always 0.
		cost := c.CostUSD
		if cost == 0 {
			cost = computeLLMCost(c.Model, pt, ct)
		}
		out[i] = LLMStepTrace{
			NodeName:         c.NodeName,
			Provider:         c.Provider,
			Model:            c.Model,
			Endpoint:         c.Endpoint,
			Latency:          c.Latency,
			Attempt:          attempt,
			Stream:           c.Stream,
			StatusCode:       c.StatusCode,
			ErrText:          c.ErrText,
			PromptTokens:     pt,
			CompletionTokens: ct,
			TotalTokens:      tt,
			CostUSD:          cost,
			Prompt:           redactForTrace(c.Prompt),
			Response:         redactForTrace(c.Response),
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
