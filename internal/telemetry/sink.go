// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​‌‌​‌‌​‌‌‌‌​​‌​​‌‌‌​​‌​​​‌‌‌‌​‌‌‌‌​​​‌​‌​‌‌​​​​​​​​​​​​​​​​​​​‌‌​​​​‌‌​​​‌​‌​‌⁠
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

package telemetry

import (
	"context"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

// OtelLLMCallSink wraps a core.LLMCallSink and additionally creates an
// OpenTelemetry span for each LLM call. When tracing is disabled, the OTel
// overhead is a single boolean check per call.
//
// Usage:
//
//	// When attaching a collector to a step context:
//	collector := newLLMCallCollector()
//	ctx = core.WithLLMCallSink(ctx, &OtelLLMCallSink{Inner: collector})
type OtelLLMCallSink struct {
	Inner core.LLMCallSink
}

// RecordLLMCall implements core.LLMCallSink. It creates an OTel span for the
// LLM call and delegates to the inner sink.
//
// The span is created here rather than inside the LLM node because the node
// already publishes telemetry synchronously at the end of each call; creating
// a span at that point captures the full latency and result. The span is
// self-contained (started and ended in this call) because the telemetry is
// published after the HTTP round-trip completes.
func (s *OtelLLMCallSink) RecordLLMCall(t core.LLMCallTelemetry) {
	if enabled && s.Inner != nil {
		// Create a background span — the step context is not available at
		// the sink level, so we use context.Background(). The span is still
		// exported with the same trace ID if the exporter batches them
		// together. For full parent-child linking, the executor would need
		// to inject the step span context into the sink, which adds
		// complexity for marginal gain. The attributes (model, provider,
		// tokens, cost) are the high-value data for Jaeger/Tempo.
		_, span := StartLLMSpan(context.Background(), t.Model, t.Provider)

		var err error
		if t.ErrText != "" {
			err = &llmError{text: t.ErrText}
		}
		var pt, ct, tt int
		if t.Usage != nil {
			pt = t.Usage.PromptTokens
			ct = t.Usage.CompletionTokens
			tt = t.Usage.TotalTokens
		}
		LLMSpanEnd(span, err, t.Latency.Milliseconds(), pt, ct, tt, t.CostUSD, t.StatusCode)
	}

	// Delegate to inner sink even when tracing is disabled.
	if s.Inner != nil {
		s.Inner.RecordLLMCall(t)
	}
}

// llmError is a lightweight error type that carries the error text from an
// LLMCallTelemetry record. It exists so LLMSpanEnd can use RecordError with a
// real error value rather than a string.
type llmError struct{ text string }

func (e *llmError) Error() string { return e.text }
