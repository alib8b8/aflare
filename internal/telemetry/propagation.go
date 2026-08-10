// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// propagationCarrier adapts a map[string]string (or http.Header) to the
// propagation.TextMapCarrier interface so the global propagator can
// inject / extract trace context.
type propagationCarrier map[string]string

func (c propagationCarrier) Get(key string) string {
	return c[key]
}

func (c propagationCarrier) Set(key, value string) {
	c[key] = value
}

func (c propagationCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// InjectTraceContext injects the trace context from the given context into the
// provided headers map. This is used before making an outbound HTTP request to
// propagate the current trace to a downstream service.
//
// Example:
//
//	headers := make(map[string]string)
//	telemetry.InjectTraceContext(ctx, headers)
//	for k, v := range headers {
//	    req.Header.Set(k, v)
//	}
func InjectTraceContext(ctx context.Context, headers map[string]string) {
	if !enabled {
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, propagationCarrier(headers))
}

// InjectTraceContextToHTTP injects the trace context from the given context
// into the provided http.Header. This is a convenience wrapper for use with
// net/http clients.
func InjectTraceContextToHTTP(ctx context.Context, header http.Header) {
	if !enabled {
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(header))
}

// ExtractTraceContext extracts trace context from the given headers map and
// returns a child context that continues the remote trace. Use this in an HTTP
// handler to join a distributed trace initiated by an upstream caller.
//
// Example:
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    ctx := telemetry.ExtractTraceContext(r.Context(), headersFromRequest(r))
//	    // ... use ctx for all downstream operations ...
//	}
func ExtractTraceContext(ctx context.Context, headers map[string]string) context.Context {
	if !enabled {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagationCarrier(headers))
}

// ExtractTraceContextFromHTTP extracts trace context from the incoming HTTP
// request headers and returns a child context that continues the remote trace.
func ExtractTraceContextFromHTTP(ctx context.Context, header http.Header) context.Context {
	if !enabled {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(header))
}

// StartSpanFromRemote creates a span that is a child of a remote parent trace,
// extracted from the given headers. This is the typical pattern for RPC/HTTP
// handlers that receive traces from upstream services.
//
// Returns the new context with the span attached, and the span itself.
// The caller MUST call span.End().
func StartSpanFromRemote(ctx context.Context, headers map[string]string, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !enabled {
		return ctx, noopSpan
	}
	remoteCtx := ExtractTraceContext(ctx, headers)
	tracer := otel.Tracer(tracerName)
	return tracer.Start(remoteCtx, spanName, opts...)
}