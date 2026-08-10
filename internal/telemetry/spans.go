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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const (
	tracerName = "github.com/alib8b8/aflare"
)

// Span names for the three levels of tracing.
const (
	SpanWorkflow     = "workflow.run"
	SpanStep         = "step.execute"
	SpanCompoundStep = "step.compound"
	SpanSubStep      = "step.sub"
	SpanLLMCall      = "llm.call"
)

// StartWorkflowSpan creates the root span for a workflow execution. It is the
// parent of every step span for this run. Returns the child context and the
// span; the caller MUST call span.End().
func StartWorkflowSpan(ctx context.Context, wfName string) (context.Context, trace.Span) {
	if !enabled {
		return ctx, noopSpan
	}
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, SpanWorkflow,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("workflow.name", wfName),
		),
	)
	return ctx, span
}

// StartStepSpan creates a child span for a single workflow step. It is a
// child of the workflow span. Returns the child context and the span; the
// caller MUST call span.End().
func StartStepSpan(ctx context.Context, stepName, nodeName string, stepIndex int) (context.Context, trace.Span) {
	if !enabled {
		return ctx, noopSpan
	}
	tracer := otel.Tracer(tracerName)
	attrs := []attribute.KeyValue{
		attribute.String("step.node", nodeName),
		attribute.Int("step.index", stepIndex),
	}
	if stepName != "" {
		attrs = append(attrs, attribute.String("step.name", stepName))
	}
	ctx, span := tracer.Start(ctx, SpanStep,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attrs...),
	)
	return ctx, span
}

// StepSpanEnd is a convenience wrapper that records an error (if any) on the
// span and ends it. durationMs and outputLen are optional (0 means skip).
func StepSpanEnd(span trace.Span, err error, durationMs int64, outputLen int, skipped bool) {
	if span == nil {
		return
	}
	if skipped {
		span.SetAttributes(attribute.Bool("step.skipped", true))
	}
	if durationMs > 0 {
		span.SetAttributes(attribute.Int64("step.duration_ms", durationMs))
	}
	if outputLen > 0 {
		span.SetAttributes(attribute.Int("step.output_len", outputLen))
	}
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
	span.End()
}

// SetWorkflowError records an error on the workflow-level span.
func SetWorkflowError(span trace.Span, err error) {
	if span == nil {
		return
	}
	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)
}

// StartCompoundStepSpan creates a span for a compound step (saga, map, parallel,
// loop, reduce, if). It is a child of the parent context (typically the workflow
// span or the parent step's span). Returns the child context and the span; the
// caller MUST call span.End().
func StartCompoundStepSpan(ctx context.Context, stepName, compoundType string, stepIndex int) (context.Context, trace.Span) {
	if !enabled {
		return ctx, noopSpan
	}
	tracer := otel.Tracer(tracerName)
	attrs := []attribute.KeyValue{
		attribute.String("step.type", compoundType),
		attribute.Int("step.index", stepIndex),
	}
	if stepName != "" {
		attrs = append(attrs, attribute.String("step.name", stepName))
	}
	ctx, span := tracer.Start(ctx, SpanCompoundStep,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attrs...),
	)
	return ctx, span
}

// StartSubStepSpan creates a span for a sub-step within a compound step (e.g.
// a single map iteration, a saga forward step, a parallel branch). It is a
// child of the compound step span. Returns the child context and the span; the
// caller MUST call span.End().
func StartSubStepSpan(ctx context.Context, name, nodeName string, subIndex int) (context.Context, trace.Span) {
	if !enabled {
		return ctx, noopSpan
	}
	tracer := otel.Tracer(tracerName)
	attrs := []attribute.KeyValue{
		attribute.String("substep.node", nodeName),
		attribute.Int("substep.index", subIndex),
	}
	if name != "" {
		attrs = append(attrs, attribute.String("substep.name", name))
	}
	ctx, span := tracer.Start(ctx, SpanSubStep,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attrs...),
	)
	return ctx, span
}

// SubStepSpanEnd is a convenience wrapper that records an error (if any) on the
// sub-step span, sets duration and output length, and ends it.
func SubStepSpanEnd(span trace.Span, err error, durationMs int64, outputLen int) {
	if span == nil {
		return
	}
	if durationMs > 0 {
		span.SetAttributes(attribute.Int64("substep.duration_ms", durationMs))
	}
	if outputLen > 0 {
		span.SetAttributes(attribute.Int("substep.output_len", outputLen))
	}
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
	span.End()
}

// StartLLMSpan creates a child span for a single LLM API call within a step.
// The returned context and span should be used by the caller; the span MUST be
// ended via LLMSpanEnd.
func StartLLMSpan(ctx context.Context, model, provider string) (context.Context, trace.Span) {
	if !enabled {
		return ctx, noopSpan
	}
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, SpanLLMCall,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("llm.model", model),
			attribute.String("llm.provider", provider),
		),
	)
	return ctx, span
}

// LLMSpanEnd records the outcome of an LLM call on its span and ends it.
func LLMSpanEnd(span trace.Span, err error, latencyMs int64, promptTokens, completionTokens, totalTokens int, costUSD float64, statusCode int) {
	if span == nil {
		return
	}
	if latencyMs > 0 {
		span.SetAttributes(attribute.Int64("llm.latency_ms", latencyMs))
	}
	if promptTokens > 0 {
		span.SetAttributes(attribute.Int("llm.tokens.prompt", promptTokens))
	}
	if completionTokens > 0 {
		span.SetAttributes(attribute.Int("llm.tokens.completion", completionTokens))
	}
	if totalTokens > 0 {
		span.SetAttributes(attribute.Int("llm.tokens.total", totalTokens))
	}
	if costUSD > 0 {
		// Use float64 attribute; most UIs format it as-is.
		span.SetAttributes(attribute.Float64("llm.cost_usd", costUSD))
	}
	if statusCode > 0 {
		span.SetAttributes(attribute.Int("llm.status_code", statusCode))
	}
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
	span.End()
}

// noopSpan is a trace.Span that does nothing. It is returned when tracing is
// disabled so callers can unconditionally call span.End() without nil checks.
// We use the noop tracer provider to get a real noop span rather than
// implementing the interface ourselves (the interface has unexported methods
// in newer OTel versions).
var noopSpan trace.Span

func init() {
	_, noopSpan = noop.NewTracerProvider().Tracer(tracerName).Start(context.Background(), "noop")
}
