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

package core

import (
	"context"
	"time"
)

// LLMCallTelemetry is the per-call observability record published by an LLM
// node after a single Execute / ExecuteStream invocation. It is consumed by
// the workflow executor (B-2) to enrich StepTrace with token / cost / latency
// detail, and may also be consumed by routers (B-3) for adaptive decisions.
//
// All fields are safe to leave zero-valued when the provider did not return
// the corresponding data (e.g. Usage is nil for providers that omit usage).
type LLMCallTelemetry struct {
	NodeName   string        // node type name (e.g. "openai", "deepseek")
	Provider   string        // human-readable provider name from LLMNodeConfig
	Model      string        // resolved model name actually sent
	Endpoint   string        // resolved endpoint URL
	Latency    time.Duration // end-to-end call wall time (excluding retries)
	Attempt    int           // 1-based attempt index within a retry loop
	Usage      *LLMUsage     // token accounting, nil if provider omitted
	Stream     bool          // whether this was a streaming call
	StatusCode int           // HTTP status code; 0 if request never reached server
	ErrText    string        // error text, empty on success
	// CostUSD is an optional, provider-supplied cost estimate in USD. The
	// LLM node itself does not compute pricing; routers or callers may
	// populate this from a price table keyed by Model. Kept here so the
	// trace carries a single cost field regardless of who computed it.
	CostUSD float64
	// Prompt is the raw user prompt text sent to the provider for this
	// call. The workflow executor redacts it (via RedactSensitive) before
	// persisting it into StepTrace, so callers that publish telemetry
	// should set the unredacted value — redaction happens downstream.
	Prompt string
	// Response is the raw response content received from the provider.
	// Like Prompt, it is redacted by the workflow executor before
	// persistence into StepTrace.
	Response string
}

// LLMCallSink receives LLMCallTelemetry records. Implementations must be
// safe for concurrent use: the DAG scheduler runs steps in parallel, and
// each step may issue multiple LLM calls (retries, sub-calls). A sink is
// scoped to a single workflow run via the context.
type LLMCallSink interface {
	RecordLLMCall(t LLMCallTelemetry)
}

// noopLLMCallSink discards all telemetry. Used as the default when no sink
// is attached to the context, so LLM nodes can publish unconditionally
// without nil checks.
type noopLLMCallSink struct{}

func (noopLLMCallSink) RecordLLMCall(LLMCallTelemetry) {}

type llmSinkCtxKey struct{}

// WithLLMCallSink returns a new context carrying sink. LLM nodes descended
// from ctx will publish telemetry to sink. Passing nil restores the
// no-op default, so callers can explicitly disable collection.
func WithLLMCallSink(ctx context.Context, sink LLMCallSink) context.Context {
	if sink == nil {
		sink = noopLLMCallSink{}
	}
	return context.WithValue(ctx, llmSinkCtxKey{}, sink)
}

// LLMCallSinkFrom returns the sink attached to ctx, or a no-op sink if none
// is present. The return is therefore always non-nil and safe to call.
func LLMCallSinkFrom(ctx context.Context) LLMCallSink {
	if s, ok := ctx.Value(llmSinkCtxKey{}).(LLMCallSink); ok {
		return s
	}
	return noopLLMCallSink{}
}
