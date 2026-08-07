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

package webui

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

// echoMetricsNode is a minimal Node used to drive ExecuteWithStats so the
// node_executions_total counter is incremented.
type echoMetricsNode struct{}

func (echoMetricsNode) Name() string        { return "echo_metrics_test" }
func (echoMetricsNode) Description() string { return "echo node for metrics tests" }
func (echoMetricsNode) Schema() core.NodeSchema {
	return core.NodeSchema{Name: "echo_metrics_test", Input: "string", Output: "string"}
}
func (echoMetricsNode) Execute(_ context.Context, input string, _ map[string]string) (string, error) {
	return input, nil
}

func TestMetricsEndpoint_DisabledByDefault(t *testing.T) {
	t.Setenv("AFLARE_METRICS", "")
	s := NewWebUIServer("127.0.0.1", "0")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// With metrics disabled, /metrics is not registered; the ServeMux "/"
	// catch-all serves the index HTML, so the body is HTML, not Prometheus text.
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (index fallback), got %d", resp.StatusCode)
	}
	if strings.Contains(string(body), "llmbox_node_executions_total") {
		t.Error("/metrics should not expose prometheus text when disabled")
	}
}

func TestMetricsEndpoint_Enabled(t *testing.T) {
	t.Setenv("AFLARE_METRICS", "1")
	s := NewWebUIServer("127.0.0.1", "0")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("expected text/plain content-type, got %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	// Plain (non-Vec) counters are always emitted, even at zero, so they are
	// a stable marker that the endpoint is serving Prometheus format. Labelled
	// CounterVecs only appear once a label combination has been touched (see
	// TestMetricsEndpoint_IncrementAfterNodeExecution).
	if !strings.Contains(string(body), "llmbox_cache_hits_total") {
		t.Errorf("expected llmbox_cache_hits_total in /metrics output\n%s", string(body))
	}
	if !strings.Contains(string(body), "# TYPE llmbox_cache_hits_total counter") {
		t.Errorf("expected prometheus TYPE line for cache_hits_total")
	}
}

func TestMetricsEndpoint_IncrementAfterNodeExecution(t *testing.T) {
	t.Setenv("AFLARE_METRICS", "1")
	s := NewWebUIServer("127.0.0.1", "0")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	// Execute a node through ExecuteWithStats; this direct-Incs
	// llmbox_node_executions_total{node_name="echo_metrics_test",status="success"}.
	reg := core.NewRegistry()
	reg.Register(echoMetricsNode{})
	if _, err := reg.ExecuteWithStats("echo_metrics_test", context.Background(), "hello", nil); err != nil {
		t.Fatalf("ExecuteWithStats error: %v", err)
	}

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	needle := `llmbox_node_executions_total{node_name="echo_metrics_test",status="success"}`
	if !strings.Contains(string(body), needle) {
		t.Errorf("expected %q in /metrics output after node execution\nbody:\n%s", needle, string(body))
	}
}

func TestMetricsEndpoint_NoAuthRequired(t *testing.T) {
	// /metrics must be reachable without an X-Auth-Token even when an auth
	// token is configured on the server (scrapers typically carry no token).
	t.Setenv("AFLARE_METRICS", "1")
	s := NewWebUIServer("127.0.0.1", "0")
	s.SetAuthToken("secret-token")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 without auth token, got %d", resp.StatusCode)
	}
}
