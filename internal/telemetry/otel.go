// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌​‌‌‌​‌‌​​‌​​​​​‌​‌​‌​​​​‌​​​​‌​‌‌‌​​‌‌‌​‌​‌​‌​​​​​​​​​​​​​​​​‌‌‌​‌‌‌‌​‌​​‌‌​‌⁠
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

// Package telemetry provides OpenTelemetry tracing integration for aflare
// workflows. It exports traces via OTLP (HTTP) to a collector (e.g. Jaeger,
// Tempo, or an OTel Collector), so workflow runs can be visualised alongside
// business-system traces.
//
// Quick start:
//
//	// At process startup (main.go):
//	shutdown, err := telemetry.InitTracer(ctx, telemetry.Config{
//	    Endpoint:    "http://localhost:4318/v1/traces",
//	    ServiceName: "aflare",
//	})
//	if err != nil { /* handle */ }
//	defer shutdown(ctx)
//
//	// On the Executor:
//	exec := workflow.NewExecutor().WithOTelTracing(true)
//
// When no endpoint is configured, tracing is a no-op (no spans created, no
// network calls). The Executor's WithOTelTracing controls whether per-step
// spans are created; it is a no-op when the global tracer provider is not set.
package telemetry

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.28.0"
)

// Config holds OpenTelemetry initialisation parameters.
type Config struct {
	// Endpoint is the OTLP HTTP endpoint for trace export, e.g.
	// "http://localhost:4318/v1/traces". When empty, InitTracer is a no-op.
	Endpoint string
	// ServiceName is the service.name resource attribute (default: "aflare").
	ServiceName string
	// BatchTimeout controls how often batched spans are flushed (default: 5s).
	BatchTimeout time.Duration
}

// DefaultEndpointEnv is the env var read for the OTLP endpoint when Config
// does not set one explicitly. It mirrors the standard OTel variable name
// OTEL_EXPORTER_OTLP_ENDPOINT so users can configure both aflare and any
// auto-instrumented library with a single env var.
const DefaultEndpointEnv = "OTEL_EXPORTER_OTLP_ENDPOINT"

var (
	provider *sdktrace.TracerProvider
	initMu   sync.Mutex
	enabled  bool
)

// InitTracer creates an OTLP HTTP exporter, a TracerProvider, and sets it as
// the global tracer. Returns a shutdown function that callers should defer.
//
// When endpoint is empty and OTEL_EXPORTER_OTLP_ENDPOINT is also empty, the
// call is a no-op (no spans created, no network calls). The returned shutdown
// is non-nil and safe to call regardless.
//
// InitTracer is safe to call multiple times; subsequent calls are no-ops.
func InitTracer(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	initMu.Lock()
	defer initMu.Unlock()

	if provider != nil {
		// Already initialised.
		return func(ctx context.Context) error { return provider.Shutdown(ctx) }, nil
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = os.Getenv(DefaultEndpointEnv)
	}
	if endpoint == "" {
		// No endpoint configured: tracing is a silent no-op.
		enabled = false
		return func(_ context.Context) error { return nil }, nil
	}

	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = "aflare"
	}

	batchTimeout := cfg.BatchTimeout
	if batchTimeout <= 0 {
		batchTimeout = 5 * time.Second
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(endpoint),
		otlptracehttp.WithTimeout(10*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: failed to create OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version()),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: failed to create resource: %w", err)
	}

	provider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(batchTimeout),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	enabled = true
	return func(ctx context.Context) error {
		initMu.Lock()
		defer initMu.Unlock()
		enabled = false
		if provider != nil {
			if err := provider.Shutdown(ctx); err != nil {
				return err
			}
			provider = nil
		}
		return nil
	}, nil
}

// IsEnabled reports whether the global tracer provider has been initialised
// and tracing is active. Callers should check this before creating spans to
// avoid unnecessary allocations when tracing is disabled.
func IsEnabled() bool {
	return enabled
}

// version returns the aflare version string, or "dev" if not set at build
// time. This mirrors the version in meta.Version but avoids a circular import.
func version() string {
	if v := os.Getenv("AFLARE_VERSION"); v != "" {
		return v
	}
	return "dev"
}
