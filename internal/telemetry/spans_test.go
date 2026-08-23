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
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestStartWorkflowSpan_Enabled(t *testing.T) {
	sr := withTracing(t)

	ctx, span := StartWorkflowSpan(context.Background(), "my-workflow")
	if span == nil {
		t.Fatal("span must not be nil when tracing is enabled")
	}
	if !span.IsRecording() {
		t.Fatal("workflow span should be recording when tracing is enabled")
	}
	span.End()

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	got := spans[0]
	if got.Name() != SpanWorkflow {
		t.Errorf("span name = %q, want %q", got.Name(), SpanWorkflow)
	}
	if got.SpanKind() != trace.SpanKindServer {
		t.Errorf("span kind = %v, want %v", got.SpanKind(), trace.SpanKindServer)
	}
	if v, ok := findAttr(t, got, "workflow.name"); !ok || v != "my-workflow" {
		t.Errorf("workflow.name attr = %q (found=%v), want %q", v, ok, "my-workflow")
	}
	// The returned context must carry the span for child spans to attach.
	if sc := trace.SpanContextFromContext(ctx); !sc.IsValid() {
		t.Error("returned context does not carry a valid span context")
	}
}

func TestStartStepSpan_Enabled(t *testing.T) {
	sr := withTracing(t)

	parentCtx, parent := StartWorkflowSpan(context.Background(), "wf")
	_, span := StartStepSpan(parentCtx, "step-1", "openai", 3)
	span.End()
	parent.End()

	spans := sr.Ended()
	if len(spans) != 2 {
		t.Fatalf("recorded %d spans, want 2", len(spans))
	}
	var step sdktrace.ReadOnlySpan
	for _, s := range spans {
		if s.Name() == SpanStep {
			step = s
		}
	}
	if step == nil {
		t.Fatalf("no %q span recorded", SpanStep)
	}
	if step.SpanKind() != trace.SpanKindInternal {
		t.Errorf("span kind = %v, want %v", step.SpanKind(), trace.SpanKindInternal)
	}
	if v, ok := findAttr(t, step, "step.node"); !ok || v != "openai" {
		t.Errorf("step.node = %q (found=%v), want openai", v, ok)
	}
	if v, ok := findAttr(t, step, "step.index"); !ok || v != "3" {
		t.Errorf("step.index = %q (found=%v), want 3", v, ok)
	}
	if v, ok := findAttr(t, step, "step.name"); !ok || v != "step-1" {
		t.Errorf("step.name = %q (found=%v), want step-1", v, ok)
	}
	// Child must share the parent trace.
	if step.SpanContext().TraceID() != parent.SpanContext().TraceID() {
		t.Error("step span trace ID differs from workflow span")
	}
}

func TestStartStepSpan_EmptyNameOmitsAttr(t *testing.T) {
	sr := withTracing(t)

	_, span := StartStepSpan(context.Background(), "", "http", 0)
	span.End()

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	if _, ok := findAttr(t, spans[0], "step.name"); ok {
		t.Error("step.name should be omitted when stepName is empty")
	}
	if v, ok := findAttr(t, spans[0], "step.index"); !ok || v != "0" {
		t.Errorf("step.index = %q (found=%v), want 0", v, ok)
	}
}

func TestStepSpanEnd_RecordsOutcome(t *testing.T) {
	sr := withTracing(t)

	_, span := StartStepSpan(context.Background(), "s", "openai", 0)
	StepSpanEnd(span, errors.New("boom"), 42, 100, true)

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	got := spans[0]
	if got.Status().Code != codes.Error {
		t.Errorf("status code = %v, want Error", got.Status().Code)
	}
	if got.Status().Description != "boom" {
		t.Errorf("status description = %q, want boom", got.Status().Description)
	}
	if len(got.Events()) == 0 {
		t.Error("RecordError should add an exception event")
	}
	for _, key := range []string{"step.skipped", "step.duration_ms", "step.output_len"} {
		if _, ok := findAttr(t, got, key); !ok {
			t.Errorf("expected attribute %q on ended span", key)
		}
	}
}

func TestStepSpanEnd_SuccessOmitsOptionalAttrs(t *testing.T) {
	sr := withTracing(t)

	_, span := StartStepSpan(context.Background(), "s", "openai", 0)
	StepSpanEnd(span, nil, 0, 0, false)

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	got := spans[0]
	if got.Status().Code != codes.Unset {
		t.Errorf("status code = %v, want Unset on success", got.Status().Code)
	}
	for _, key := range []string{"step.skipped", "step.duration_ms", "step.output_len"} {
		if _, ok := findAttr(t, got, key); ok {
			t.Errorf("attribute %q should be omitted when unset", key)
		}
	}
}

func TestStepSpanEnd_NilSpanNoPanic(t *testing.T) {
	StepSpanEnd(nil, errors.New("boom"), 1, 1, true)
	SubStepSpanEnd(nil, nil, 1, 1)
	LLMSpanEnd(nil, nil, 1, 1, 1, 1, 1, 200)
	SetWorkflowError(nil, errors.New("boom"))
}

func TestSetWorkflowError_RecordsError(t *testing.T) {
	sr := withTracing(t)

	_, span := StartWorkflowSpan(context.Background(), "wf")
	SetWorkflowError(span, errors.New("workflow failed"))
	span.End()

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	got := spans[0]
	if got.Status().Code != codes.Error {
		t.Errorf("status code = %v, want Error", got.Status().Code)
	}
	if len(got.Events()) == 0 {
		t.Error("RecordError should add an exception event")
	}
}

func TestStartCompoundStepSpan_Enabled(t *testing.T) {
	sr := withTracing(t)

	_, span := StartCompoundStepSpan(context.Background(), "saga-1", "saga", 2)
	span.End()

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	got := spans[0]
	if got.Name() != SpanCompoundStep {
		t.Errorf("span name = %q, want %q", got.Name(), SpanCompoundStep)
	}
	if v, ok := findAttr(t, got, "step.type"); !ok || v != "saga" {
		t.Errorf("step.type = %q (found=%v), want saga", v, ok)
	}
	if v, ok := findAttr(t, got, "step.name"); !ok || v != "saga-1" {
		t.Errorf("step.name = %q (found=%v), want saga-1", v, ok)
	}
}

func TestStartCompoundStepSpan_EmptyNameOmitsAttr(t *testing.T) {
	sr := withTracing(t)

	_, span := StartCompoundStepSpan(context.Background(), "", "map", 0)
	span.End()

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	if _, ok := findAttr(t, spans[0], "step.name"); ok {
		t.Error("step.name should be omitted when stepName is empty")
	}
}

func TestStartSubStepSpan_Enabled(t *testing.T) {
	sr := withTracing(t)

	parentCtx, parent := StartCompoundStepSpan(context.Background(), "map-1", "map", 0)
	_, span := StartSubStepSpan(parentCtx, "iter-1", "openai", 1)
	SubStepSpanEnd(span, errors.New("iter failed"), 10, 20)
	parent.End()

	spans := sr.Ended()
	var sub sdktrace.ReadOnlySpan
	for _, s := range spans {
		if s.Name() == SpanSubStep {
			sub = s
		}
	}
	if sub == nil {
		t.Fatalf("no %q span recorded", SpanSubStep)
	}
	if v, ok := findAttr(t, sub, "substep.node"); !ok || v != "openai" {
		t.Errorf("substep.node = %q (found=%v), want openai", v, ok)
	}
	if v, ok := findAttr(t, sub, "substep.name"); !ok || v != "iter-1" {
		t.Errorf("substep.name = %q (found=%v), want iter-1", v, ok)
	}
	if sub.Status().Code != codes.Error {
		t.Errorf("status code = %v, want Error", sub.Status().Code)
	}
	for _, key := range []string{"substep.duration_ms", "substep.output_len"} {
		if _, ok := findAttr(t, sub, key); !ok {
			t.Errorf("expected attribute %q on ended sub-step span", key)
		}
	}
	if sub.SpanContext().TraceID() != parent.SpanContext().TraceID() {
		t.Error("sub-step trace ID differs from compound step parent")
	}
}

func TestStartLLMSpan_Enabled(t *testing.T) {
	sr := withTracing(t)

	_, span := StartLLMSpan(context.Background(), "gpt-4o", "openai")
	LLMSpanEnd(span, nil, 150, 10, 20, 30, 0.01, 200)

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	got := spans[0]
	if got.Name() != SpanLLMCall {
		t.Errorf("span name = %q, want %q", got.Name(), SpanLLMCall)
	}
	if got.SpanKind() != trace.SpanKindClient {
		t.Errorf("span kind = %v, want %v", got.SpanKind(), trace.SpanKindClient)
	}
	want := map[string]string{
		"llm.model":             "gpt-4o",
		"llm.provider":          "openai",
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

func TestStartLLMSpan_Error(t *testing.T) {
	sr := withTracing(t)

	_, span := StartLLMSpan(context.Background(), "glm-4", "glm")
	LLMSpanEnd(span, errors.New("rate limited"), 0, 0, 0, 0, 0, 429)

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	got := spans[0]
	if got.Status().Code != codes.Error {
		t.Errorf("status code = %v, want Error", got.Status().Code)
	}
	if got.Status().Description != "rate limited" {
		t.Errorf("status description = %q, want rate limited", got.Status().Description)
	}
	if len(got.Events()) == 0 {
		t.Error("RecordError should add an exception event")
	}
	// Zero-valued metrics must be omitted; the status code is non-zero.
	for _, key := range []string{"llm.latency_ms", "llm.tokens.prompt", "llm.cost_usd"} {
		if _, ok := findAttr(t, got, key); ok {
			t.Errorf("attribute %q should be omitted when zero", key)
		}
	}
	if v, ok := findAttr(t, got, "llm.status_code"); !ok || v != "429" {
		t.Errorf("llm.status_code = %q (found=%v), want 429", v, ok)
	}
}

func TestSpansDisabledAreNoOp(t *testing.T) {
	if IsEnabled() {
		t.Skip("tracing already enabled by another test in this package")
	}
	ctx := context.Background()
	if _, span := StartWorkflowSpan(ctx, "wf"); span.IsRecording() {
		t.Error("disabled StartWorkflowSpan should return a non-recording noop span")
	}
	if _, span := StartStepSpan(ctx, "s", "openai", 0); span.IsRecording() {
		t.Error("disabled StartStepSpan should return a non-recording noop span")
	}
	if _, span := StartCompoundStepSpan(ctx, "s", "map", 0); span.IsRecording() {
		t.Error("disabled StartCompoundStepSpan should return a non-recording noop span")
	}
	if _, span := StartSubStepSpan(ctx, "s", "openai", 0); span.IsRecording() {
		t.Error("disabled StartSubStepSpan should return a non-recording noop span")
	}
	if _, span := StartLLMSpan(ctx, "m", "p"); span.IsRecording() {
		t.Error("disabled StartLLMSpan should return a non-recording noop span")
	}
	if noopSpan == nil {
		t.Error("noopSpan must be non-nil so callers never need nil checks")
	}
}
