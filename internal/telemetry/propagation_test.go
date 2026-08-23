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
	"net/http"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestInjectExtractRoundTrip_Enabled(t *testing.T) {
	withTracing(t)

	ctx, span := StartWorkflowSpan(context.Background(), "wf")
	defer span.End()

	headers := map[string]string{}
	InjectTraceContext(ctx, headers)
	tp, ok := headers["traceparent"]
	if !ok || tp == "" {
		t.Fatalf("InjectTraceContext did not write traceparent, headers = %v", headers)
	}
	// traceparent format: version-traceid-spanid-flags; the trace ID must
	// match the active span.
	wantTrace := span.SpanContext().TraceID().String()
	parts := strings.Split(tp, "-")
	if len(parts) != 4 {
		t.Fatalf("traceparent %q has %d parts, want 4", tp, len(parts))
	}
	if parts[1] != wantTrace {
		t.Errorf("injected trace ID %s, want %s", parts[1], wantTrace)
	}

	extracted := ExtractTraceContext(context.Background(), headers)
	sc := trace.SpanContextFromContext(extracted)
	if !sc.IsValid() {
		t.Fatal("ExtractTraceContext did not yield a valid span context")
	}
	if sc.TraceID() != span.SpanContext().TraceID() {
		t.Errorf("extracted trace ID %s, want %s", sc.TraceID(), span.SpanContext().TraceID())
	}
}

func TestInjectExtractHTTPRoundTrip_Enabled(t *testing.T) {
	withTracing(t)

	ctx, span := StartWorkflowSpan(context.Background(), "wf")
	defer span.End()

	header := http.Header{}
	InjectTraceContextToHTTP(ctx, header)
	if header.Get("traceparent") == "" {
		t.Fatalf("InjectTraceContextToHTTP did not write traceparent, header = %v", header)
	}

	extracted := ExtractTraceContextFromHTTP(context.Background(), header)
	sc := trace.SpanContextFromContext(extracted)
	if !sc.IsValid() {
		t.Fatal("ExtractTraceContextFromHTTP did not yield a valid span context")
	}
	if sc.TraceID() != span.SpanContext().TraceID() {
		t.Errorf("extracted trace ID %s, want %s", sc.TraceID(), span.SpanContext().TraceID())
	}
}

func TestStartSpanFromRemote_Enabled(t *testing.T) {
	sr := withTracing(t)

	// Simulate an upstream service: create a span and inject its context.
	upstreamCtx, upstream := StartWorkflowSpan(context.Background(), "upstream")
	headers := map[string]string{}
	InjectTraceContext(upstreamCtx, headers)
	upstream.End()

	remoteCtx, span := StartSpanFromRemote(context.Background(), headers, "downstream-op")
	if span == nil || !span.IsRecording() {
		t.Fatal("StartSpanFromRemote should return a recording span when enabled")
	}
	span.End()

	if sc := trace.SpanContextFromContext(remoteCtx); !sc.IsValid() {
		t.Error("returned context should carry the remote span")
	}

	spans := sr.Ended()
	var downstream trace.TraceID
	for _, s := range spans {
		if s.Name() == "downstream-op" {
			downstream = s.SpanContext().TraceID()
		}
	}
	if downstream.IsValid() && downstream != upstream.SpanContext().TraceID() {
		t.Errorf("downstream trace ID %s, want %s (join the upstream trace)", downstream, upstream.SpanContext().TraceID())
	}
}
