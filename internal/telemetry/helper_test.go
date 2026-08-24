// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​​‌‌‌​‌​​‌‌‌​​​‌​‌​‌​‌‌‌‌​‌‌‌‌‌‌‌​‌‌​​‌‌​​‌‌‌​‌​​​​​​​​​​​​​​​​‌‌‌‌​​​​​‌‌‌​‌​​⁠
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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// withTracing enables the package's tracing path for the duration of a test
// and installs a SpanRecorder-backed tracer provider as the otel global, so
// spans created via Start*Span are recorded and inspectable. All previous
// package/global state (enabled, provider, global tracer provider, global
// propagator) is restored on test cleanup.
func withTracing(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	oldEnabled := enabled
	oldProvider := provider
	oldTP := otel.GetTracerProvider()
	oldPropagator := otel.GetTextMapPropagator()

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	enabled = true

	t.Cleanup(func() {
		enabled = oldEnabled
		provider = oldProvider
		otel.SetTracerProvider(oldTP)
		otel.SetTextMapPropagator(oldPropagator)
	})

	return sr
}

// findAttr returns the value of the named attribute from a recorded span.
func findAttr(t *testing.T, span sdktrace.ReadOnlySpan, key string) (string, bool) {
	t.Helper()
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.String(), true
		}
	}
	return "", false
}
