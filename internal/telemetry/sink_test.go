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

package telemetry

import (
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/nodes/core"
	"go.opentelemetry.io/otel/codes"
)

// recordingSink captures LLMCallTelemetry records for assertions.
type recordingSink struct {
	calls []core.LLMCallTelemetry
}

func (r *recordingSink) RecordLLMCall(t core.LLMCallTelemetry) {
	r.calls = append(r.calls, t)
}

func TestOtelLLMCallSink_EnabledCreatesSpan(t *testing.T) {
	sr := withTracing(t)
	inner := &recordingSink{}
	sink := &OtelLLMCallSink{Inner: inner}

	sink.RecordLLMCall(core.LLMCallTelemetry{
		NodeName:   "openai",
		Provider:   "OpenAI",
		Model:      "gpt-4o",
		Latency:    150 * time.Millisecond,
		Usage:      &core.LLMUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
		CostUSD:    0.01,
		StatusCode: 200,
	})

	if len(inner.calls) != 1 {
		t.Fatalf("inner sink got %d calls, want 1", len(inner.calls))
	}
	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	got := spans[0]
	if got.Name() != SpanLLMCall {
		t.Errorf("span name = %q, want %q", got.Name(), SpanLLMCall)
	}
	want := map[string]string{
		"llm.model":             "gpt-4o",
		"llm.provider":          "OpenAI",
		"llm.latency_ms":        "150",
		"llm.tokens.prompt":     "10",
		"llm.tokens.completion": "20",
		"llm.tokens.total":      "30",
		"llm.cost_usd":          "0.01",
		"llm.status_code":       "200",
	}
	for key, wantVal := range want {
		if v, ok := findAttr(t, got, key); !ok || v != wantVal {
			t.Errorf("attr %s = %q (found=%v), want %q", key, v, ok, wantVal)
		}
	}
	if got.Status().Code != codes.Unset {
		t.Errorf("status code = %v, want Unset on success", got.Status().Code)
	}
}

func TestOtelLLMCallSink_EnabledErrorRecorded(t *testing.T) {
	sr := withTracing(t)
	inner := &recordingSink{}
	sink := &OtelLLMCallSink{Inner: inner}

	sink.RecordLLMCall(core.LLMCallTelemetry{
		NodeName:   "openai",
		Provider:   "OpenAI",
		Model:      "gpt-4o",
		Latency:    5 * time.Millisecond,
		StatusCode: 500,
		ErrText:    "internal error",
	})

	if len(inner.calls) != 1 {
		t.Fatalf("inner sink got %d calls, want 1", len(inner.calls))
	}
	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	got := spans[0]
	if got.Status().Code != codes.Error {
		t.Errorf("status code = %v, want Error", got.Status().Code)
	}
	if got.Status().Description != "internal error" {
		t.Errorf("status description = %q, want internal error", got.Status().Description)
	}
	if len(got.Events()) == 0 {
		t.Error("RecordError should add an exception event")
	}
}

func TestOtelLLMCallSink_NilUsageNoTokenAttrs(t *testing.T) {
	sr := withTracing(t)
	inner := &recordingSink{}
	sink := &OtelLLMCallSink{Inner: inner}

	sink.RecordLLMCall(core.LLMCallTelemetry{
		Model:   "m",
		Latency: time.Millisecond,
	})

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	for _, key := range []string{"llm.tokens.prompt", "llm.tokens.completion", "llm.tokens.total"} {
		if _, ok := findAttr(t, spans[0], key); ok {
			t.Errorf("attribute %q should be omitted when Usage is nil", key)
		}
	}
}

func TestOtelLLMCallSink_NilInnerNoPanic(t *testing.T) {
	withTracing(t)
	sink := &OtelLLMCallSink{}
	sink.RecordLLMCall(core.LLMCallTelemetry{Model: "m"})
	// Also the disabled path.
	sink.RecordLLMCall(core.LLMCallTelemetry{Model: "m"})
}

func TestOtelLLMCallSink_DisabledStillDelegates(t *testing.T) {
	if IsEnabled() {
		t.Skip("tracing already enabled by another test in this package")
	}
	inner := &recordingSink{}
	sink := &OtelLLMCallSink{Inner: inner}

	sink.RecordLLMCall(core.LLMCallTelemetry{Model: "m"})

	if len(inner.calls) != 1 {
		t.Fatalf("inner sink got %d calls, want 1 even with tracing disabled", len(inner.calls))
	}
}
