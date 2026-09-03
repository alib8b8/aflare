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

package cli

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/agent"
)

func TestOpsEnabled_DefaultOff(t *testing.T) {
	t.Setenv("AFLARE_METRICS", "")
	t.Setenv("AFLARE_PPROF", "")
	if opsEnabled() {
		t.Error("ops endpoints must be disabled by default")
	}
}

func TestOpsEnabled_EnvGates(t *testing.T) {
	t.Setenv("AFLARE_METRICS", "1")
	t.Setenv("AFLARE_PPROF", "")
	if !opsEnabled() {
		t.Error("AFLARE_METRICS=1 must enable ops")
	}
	t.Setenv("AFLARE_METRICS", "")
	t.Setenv("AFLARE_PPROF", "1")
	if !opsEnabled() {
		t.Error("AFLARE_PPROF=1 must enable ops")
	}
}

func TestBuildOpsHandler_DisabledByDefault(t *testing.T) {
	t.Setenv("AFLARE_METRICS", "")
	t.Setenv("AFLARE_PPROF", "")
	ts := httptest.NewServer(buildOpsHandler())
	defer ts.Close()

	for _, path := range []string{"/metrics", "/debug/pprof/"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s (disabled) = %d, want 404", path, resp.StatusCode)
		}
	}
}

func TestBuildOpsHandler_MetricsEnabled(t *testing.T) {
	t.Setenv("AFLARE_METRICS", "1")
	t.Setenv("AFLARE_PPROF", "")
	ts := httptest.NewServer(buildOpsHandler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	// Ops gauges are registered and must be exposed even at zero. (Labelled
	// counters like aflare_node_failures_total only appear once a label
	// combination has been touched, so they are not asserted here.)
	for _, name := range []string{"aflare_runs_active", "aflare_queue_depth"} {
		if !strings.Contains(string(body), name) {
			t.Errorf("/metrics output missing %s", name)
		}
	}

	// pprof stays off when only AFLARE_METRICS is set.
	resp2, err := http.Get(ts.URL + "/debug/pprof/")
	if err != nil {
		t.Fatalf("GET /debug/pprof/: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("GET /debug/pprof/ (pprof off) = %d, want 404", resp2.StatusCode)
	}
}

func TestBuildOpsHandler_PprofEnabled(t *testing.T) {
	t.Setenv("AFLARE_METRICS", "")
	t.Setenv("AFLARE_PPROF", "1")
	ts := httptest.NewServer(buildOpsHandler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/debug/pprof/")
	if err != nil {
		t.Fatalf("GET /debug/pprof/: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /debug/pprof/ = %d, want 200", resp.StatusCode)
	}

	// /metrics stays off when only AFLARE_PPROF is set.
	resp2, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("GET /metrics (metrics off) = %d, want 404", resp2.StatusCode)
	}
}

func TestStartOpsServer_DisabledByDefault(t *testing.T) {
	t.Setenv("AFLARE_METRICS", "")
	t.Setenv("AFLARE_PPROF", "")
	if addr := startOpsServer(context.Background(), 0); addr != "" {
		t.Errorf("startOpsServer with no env = %q, want empty", addr)
	}
}

func TestStartOpsServer_ServesAndClosesOnContextCancel(t *testing.T) {
	t.Setenv("AFLARE_METRICS", "1")
	t.Setenv("AFLARE_PPROF", "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := startOpsServer(ctx, 0) // ephemeral port
	if addr == "" {
		t.Fatal("expected bound address, got empty")
	}

	resp, err := http.Get("http://" + addr + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(body), "aflare_queue_depth") {
		t.Error("/metrics output missing aflare_queue_depth")
	}

	// Cancel must close the listener promptly.
	cancel()
	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, derr := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if derr != nil {
			break // listener closed — done
		}
		conn.Close()
		if time.Now().After(deadline) {
			t.Fatal("ops listener still open 2s after context cancel")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestParseAgentArgs_OpsPort(t *testing.T) {
	cfg := agent.DefaultConfig()
	var watchDir string
	opsPort := DefaultOpsPort

	if err := parseAgentArgs([]string{"--ops-port", "9199"}, &cfg, &watchDir, &opsPort); err != nil {
		t.Fatalf("parseAgentArgs: %v", err)
	}
	if opsPort != 9199 {
		t.Errorf("opsPort = %d, want 9199", opsPort)
	}

	// Invalid values keep the default instead of failing the daemon.
	opsPort = DefaultOpsPort
	if err := parseAgentArgs([]string{"--ops-port", "not-a-port"}, &cfg, &watchDir, &opsPort); err != nil {
		t.Fatalf("parseAgentArgs: %v", err)
	}
	if opsPort != DefaultOpsPort {
		t.Errorf("opsPort after invalid value = %d, want default %d", opsPort, DefaultOpsPort)
	}
}
