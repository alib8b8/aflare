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
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestInitTracer_NoEndpointIsNoOp verifies the local-first guarantee: with no
// endpoint configured (Config zero value and env unset), InitTracer performs
// no network setup, returns a safe shutdown, and leaves tracing disabled.
func TestInitTracer_NoEndpointIsNoOp(t *testing.T) {
	t.Setenv(DefaultEndpointEnv, "")

	shutdown, err := InitTracer(context.Background(), Config{})
	if err != nil {
		t.Fatalf("InitTracer with no endpoint returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown must be non-nil even in no-op mode")
	}
	if IsEnabled() {
		t.Error("IsEnabled should be false when no endpoint is configured")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("no-op shutdown returned error: %v", err)
	}
}

// TestInitTracer_NilSafeRepeatedShutdown guards the documented contract that
// the returned shutdown is safe to call regardless of prior state.
func TestInitTracer_NilSafeRepeatedShutdown(t *testing.T) {
	t.Setenv(DefaultEndpointEnv, "")
	shutdown, err := InitTracer(context.Background(), Config{})
	if err != nil {
		t.Fatalf("InitTracer error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("first shutdown: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("second shutdown should be safe, got: %v", err)
	}
}

// TestPropagationDisabledIsNoOp checks the cheap-path contract: when tracing
// is disabled, inject/extract return the input unchanged (no headers written,
// same context back).
func TestPropagationDisabledIsNoOp(t *testing.T) {
	if IsEnabled() {
		t.Skip("tracing already enabled by another test in this package")
	}

	headers := map[string]string{}
	ctx := context.Background()
	InjectTraceContext(ctx, headers)
	if len(headers) != 0 {
		t.Errorf("InjectTraceContext while disabled wrote %d headers, want 0", len(headers))
	}

	httpHeader := http.Header{}
	InjectTraceContextToHTTP(ctx, httpHeader)
	if len(httpHeader) != 0 {
		t.Errorf("InjectTraceContextToHTTP while disabled wrote %d headers, want 0", len(httpHeader))
	}

	in := map[string]string{"traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"}
	if got := ExtractTraceContext(ctx, in); got != ctx {
		t.Error("ExtractTraceContext while disabled should return the input context")
	}

	reqHeader := http.Header{}
	reqHeader.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	if got := ExtractTraceContextFromHTTP(ctx, reqHeader); got != ctx {
		t.Error("ExtractTraceContextFromHTTP while disabled should return the input context")
	}

	spanCtx, span := StartSpanFromRemote(ctx, in, "noop")
	if span == nil {
		t.Fatal("StartSpanFromRemote while disabled must still return a non-nil (noop) span")
	}
	span.End()
	_ = spanCtx // only verifying the disabled path does not panic
}

// TestVersionReadsEnv covers the service.version resource attribute source.
func TestVersionReadsEnv(t *testing.T) {
	t.Setenv("AFLARE_VERSION", "9.9.9-test")
	if got := version(); got != "9.9.9-test" {
		t.Errorf("version() = %q, want %q", got, "9.9.9-test")
	}
	t.Setenv("AFLARE_VERSION", "")
	if got := version(); got != "dev" {
		t.Errorf("version() with empty env = %q, want \"dev\"", got)
	}
}

// TestPropagationCarrier covers the map adapter used by inject/extract.
func TestPropagationCarrier(t *testing.T) {
	c := propagationCarrier{}
	c.Set("k", "v")
	if got := c.Get("k"); got != "v" {
		t.Errorf("Get after Set = %q, want %q", got, "v")
	}
	if got := c.Get("missing"); got != "" {
		t.Errorf("Get missing key = %q, want empty", got)
	}
	keys := c.Keys()
	if len(keys) != 1 || keys[0] != "k" {
		t.Errorf("Keys() = %v, want [k]", keys)
	}
}

// TestDefaultEndpointEnv guards the constant against accidental renames; the
// value mirrors the standard OpenTelemetry env var.
func TestDefaultEndpointEnv(t *testing.T) {
	if DefaultEndpointEnv != "OTEL_EXPORTER_OTLP_ENDPOINT" {
		t.Errorf("DefaultEndpointEnv = %q, want OTEL_EXPORTER_OTLP_ENDPOINT", DefaultEndpointEnv)
	}
}

// newOTLPCollector starts a local HTTP server that counts OTLP trace export
// POSTs, mimicking a minimal OTLP receiver.
func newOTLPCollector(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	var mu sync.Mutex
	var posts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			mu.Lock()
			posts++
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &posts
}

// TestInitTracer_EnabledLifecycle covers the full enabled path: exporter and
// provider creation, span recording, the already-initialised fast path, and
// shutdown flushing batched spans to the OTLP endpoint.
func TestInitTracer_EnabledLifecycle(t *testing.T) {
	t.Setenv(DefaultEndpointEnv, "")
	srv, posts := newOTLPCollector(t)

	shutdown, err := InitTracer(context.Background(), Config{
		Endpoint:     srv.URL,
		ServiceName:  "aflare-test",
		BatchTimeout: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("InitTracer with endpoint returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown must be non-nil")
	}
	if !IsEnabled() {
		t.Fatal("IsEnabled should be true after successful init")
	}

	// Create a real span so the batcher has something to flush.
	_, span := StartWorkflowSpan(context.Background(), "wf")
	if !span.IsRecording() {
		t.Fatal("workflow span should be recording after init")
	}
	span.End()

	// Second call while initialised: no-op that returns a working shutdown.
	shutdown2, err := InitTracer(context.Background(), Config{Endpoint: srv.URL})
	if err != nil {
		t.Fatalf("second InitTracer returned error: %v", err)
	}
	if shutdown2 == nil {
		t.Fatal("second shutdown must be non-nil")
	}
	if !IsEnabled() {
		t.Fatal("IsEnabled should stay true after repeated init")
	}

	// Shutting down flushes the recorded span to the collector.
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
	if IsEnabled() {
		t.Error("IsEnabled should be false after shutdown")
	}
	if *posts == 0 {
		t.Error("shutdown should flush batched spans to the OTLP endpoint")
	}

	// After shutdown the package can be re-initialised (provider reset).
	shutdown3, err := InitTracer(context.Background(), Config{Endpoint: srv.URL, BatchTimeout: 5 * time.Millisecond})
	if err != nil {
		t.Fatalf("re-init after shutdown returned error: %v", err)
	}
	if !IsEnabled() {
		t.Error("IsEnabled should be true after re-init")
	}
	if err := shutdown3(context.Background()); err != nil {
		t.Fatalf("re-init shutdown returned error: %v", err)
	}
	if IsEnabled() {
		t.Error("IsEnabled should be false after re-init shutdown")
	}
}

// TestInitTracer_EndpointFromEnv verifies the env fallback: an empty Config
// endpoint falls back to OTEL_EXPORTER_OTLP_ENDPOINT.
func TestInitTracer_EndpointFromEnv(t *testing.T) {
	srv, _ := newOTLPCollector(t)
	t.Setenv(DefaultEndpointEnv, srv.URL)

	shutdown, err := InitTracer(context.Background(), Config{BatchTimeout: 5 * time.Millisecond})
	if err != nil {
		t.Fatalf("InitTracer with env endpoint returned error: %v", err)
	}
	if !IsEnabled() {
		t.Fatal("IsEnabled should be true when endpoint comes from env")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
	if IsEnabled() {
		t.Error("IsEnabled should be false after shutdown")
	}
}

// TestInitTracer_InvalidEndpointIsLoggedNotFatal verifies the documented
// contract for malformed endpoints: the OTLP HTTP exporter defers URL parsing
// to first export, so init succeeds and the failure surfaces at shutdown.
func TestInitTracer_InvalidEndpointIsLoggedNotFatal(t *testing.T) {
	t.Setenv(DefaultEndpointEnv, "")
	if provider != nil {
		t.Skip("tracer already initialised by another test in this package")
	}

	shutdown, err := InitTracer(context.Background(), Config{
		Endpoint:     "://not-a-url",
		BatchTimeout: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("InitTracer defers endpoint parsing to export time, got error: %v", err)
	}
	if !IsEnabled() {
		t.Fatal("IsEnabled should be true even with a malformed endpoint")
	}
	// Shutdown flushes to the bad endpoint; the export error is reported
	// through the returned error.
	_ = shutdown(context.Background())
	if IsEnabled() {
		t.Error("IsEnabled should be false after shutdown")
	}
}
