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
	"testing"
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
