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

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/meta"
	"golang.org/x/time/rate"
)

// newTestServer returns a Server whose full middleware stack and routing
// table are wired up exactly as Start() would, without listening on a port.
//
// The rate limiter is replaced with an unlimited one: handler tests issue
// many requests from a single address and the production default
// (10 req/min, burst 3) starves them. Rate limiting itself is covered by
// TestRateLimitMiddleware / TestIPRateLimiterBurst against the real limiter.
func newTestServer(apiKey string) *Server {
	s := NewServer("127.0.0.1", "0", apiKey)
	s.rateLimiter = newIPRateLimiter(rate.Inf, 1000)
	s.handler = s.middlewareStack(s.routes())
	return s
}

// serve dispatches a request through the server's middleware + routes and
// returns the recorder.
//
// httptest.NewRequest pre-fills RemoteAddr with "192.0.2.1:1234" (TEST-NET),
// which the auth middleware treats as non-localhost. Tests that don't care
// about the client address get a loopback address so the no-API-key path
// lets them through; tests that do care set RemoteAddr explicitly to
// something else.
func (s *Server) serve(t *testing.T, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	if r.RemoteAddr == "" || r.RemoteAddr == "192.0.2.1:1234" {
		r.RemoteAddr = "127.0.0.1:12345"
	}
	w := httptest.NewRecorder()
	s.handler.ServeHTTP(w, r)
	return w
}

func newJSONRequest(t *testing.T, method, target, body string) *http.Request {
	t.Helper()
	var rdr *strings.Reader
	if body == "" {
		rdr = strings.NewReader("")
	} else {
		rdr = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, target, rdr)
	r.Header.Set("Content-Type", "application/json")
	return r
}

// --- helpers ---

func TestIsAllowedOrigin(t *testing.T) {
	cases := []struct {
		origin string
		want   bool
	}{
		{"http://localhost:3000", true},
		{"http://127.0.0.1:8080", true},
		{"http://[::1]:9000", true},
		{"https://evil.example.com", false},
		{"http://localhost.evil.com", false},
		{"javascript:alert(1)", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isAllowedOrigin(c.origin); got != c.want {
			t.Errorf("isAllowedOrigin(%q) = %v, want %v", c.origin, got, c.want)
		}
	}
}

func TestIsLocalhost(t *testing.T) {
	cases := []struct {
		remoteAddr string
		want       bool
	}{
		{"127.0.0.1:4444", true},
		{"[::1]:5555", true},
		{"localhost:8080", true},
		{"192.168.1.5:1000", false},
		{"10.0.0.1:2000", false},
		{"no-port-string", false},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/health", nil)
		r.RemoteAddr = c.remoteAddr
		if got := isLocalhost(r); got != c.want {
			t.Errorf("isLocalhost(%q) = %v, want %v", c.remoteAddr, got, c.want)
		}
	}
}

func TestExtractClientIP(t *testing.T) {
	mk := func(xff, xri, remote string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/health", nil)
		r.RemoteAddr = remote
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		if xri != "" {
			r.Header.Set("X-Real-IP", xri)
		}
		return r
	}

	t.Run("default trusts only RemoteAddr", func(t *testing.T) {
		// XFF present but proxy headers not trusted: must fall back to
		// RemoteAddr so clients cannot spoof their rate-limit bucket.
		r := mk("6.6.6.6", "", "192.168.0.10:9999")
		if got := extractClientIP(r); got != "192.168.0.10" {
			t.Errorf("got %q, want RemoteAddr host", got)
		}
	})

	t.Run("RemoteAddr without port", func(t *testing.T) {
		r := mk("", "", "203.0.113.7")
		if got := extractClientIP(r); got != "203.0.113.7" {
			t.Errorf("got %q, want full RemoteAddr", got)
		}
	})

	t.Run("proxy headers trusted", func(t *testing.T) {
		t.Setenv("AFLARE_TRUST_PROXY_HEADERS", "1")

		r := mk("1.2.3.4", "", "10.0.0.1:1000")
		if got := extractClientIP(r); got != "1.2.3.4" {
			t.Errorf("XFF single: got %q, want 1.2.3.4", got)
		}

		r = mk("1.2.3.4, 5.6.7.8", "", "10.0.0.1:1000")
		if got := extractClientIP(r); got != "1.2.3.4" {
			t.Errorf("XFF chain: got %q, want first hop 1.2.3.4", got)
		}

		r = mk("", "9.9.9.9", "10.0.0.1:1000")
		if got := extractClientIP(r); got != "9.9.9.9" {
			t.Errorf("X-Real-IP: got %q, want 9.9.9.9", got)
		}
	})
}

func TestExtractSessionID(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/chat", nil)
	r.Header.Set("X-Session-Id", "from-header")
	if got := extractSessionID(r); got != "from-header" {
		t.Errorf("header: got %q", got)
	}

	r = httptest.NewRequest(http.MethodPost, "/api/v1/chat/from-path", nil)
	if got := extractSessionID(r); got != "from-path" {
		t.Errorf("path: got %q", got)
	}

	r = httptest.NewRequest(http.MethodPost, "/api/v1/chat", nil)
	if got := extractSessionID(r); got != "" {
		t.Errorf("none: got %q, want empty", got)
	}
}

// --- rate limiter ---

func TestIPRateLimiterBurst(t *testing.T) {
	rl := newIPRateLimiter(rate.Limit(100), 3)

	for i := 1; i <= 3; i++ {
		if !rl.allow("1.2.3.4") {
			t.Fatalf("request %d within burst rejected", i)
		}
	}
	if rl.allow("1.2.3.4") {
		t.Fatal("4th request beyond burst should be rate limited")
	}
	// A different IP has its own bucket.
	if !rl.allow("5.6.7.8") {
		t.Fatal("different IP should not be affected by another IP's bucket")
	}
}

func TestIPRateLimiterCleanup(t *testing.T) {
	rl := newIPRateLimiter(rate.Limit(100), 3)
	rl.allow("1.2.3.4")
	rl.allow("5.6.7.8")

	// Age both entries beyond the cleanup cutoff.
	rl.mu.Lock()
	for _, e := range rl.limiters {
		e.lastSeen = time.Now().Add(-rateLimiterMaxAge - time.Minute)
	}
	rl.mu.Unlock()

	rl.cleanup()

	rl.mu.Lock()
	n := len(rl.limiters)
	rl.mu.Unlock()
	if n != 0 {
		t.Errorf("cleanup left %d entries, want 0", n)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := newIPRateLimiter(rate.Limit(100), 2)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := rateLimitMiddleware(rl, next)

	var last *httptest.ResponseRecorder
	for i := 0; i < 3; i++ {
		r := httptest.NewRequest(http.MethodGet, "/health", nil)
		r.RemoteAddr = "1.2.3.4:1000"
		last = httptest.NewRecorder()
		h.ServeHTTP(last, r)
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd request (burst 2): status = %d, want 429", last.Code)
	}
	if got := last.Header().Get("Retry-After"); got != "60" {
		t.Errorf("Retry-After = %q, want 60", got)
	}
	if !strings.Contains(last.Body.String(), "rate limit exceeded") {
		t.Errorf("body missing error JSON: %s", last.Body.String())
	}
}

// --- middleware: auth / cors / metrics ---

func TestAuthMiddlewareNoKeyLocalhostOnly(t *testing.T) {
	s := newTestServer("")

	r := httptest.NewRequest(http.MethodGet, "/api/v1/workflows", nil)
	r.RemoteAddr = "127.0.0.1:1000"
	if w := s.serve(t, r); w.Code != http.StatusOK {
		t.Errorf("localhost without key: status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}

	r = httptest.NewRequest(http.MethodGet, "/api/v1/workflows", nil)
	r.RemoteAddr = "192.168.1.5:1000"
	w := s.serve(t, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("non-localhost without key: status = %d, want 401", w.Code)
	}
	if !strings.Contains(w.Body.String(), "API key required") {
		t.Errorf("body should explain key requirement: %s", w.Body.String())
	}
}

func TestAuthMiddlewareHealthAndMetricsBypass(t *testing.T) {
	s := newTestServer("secret-key")

	// Even with a key configured, health and metrics must stay public.
	for _, path := range []string{"/health", "/api/v1/metrics"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.RemoteAddr = "8.8.8.8:1000" // non-localhost, no key
		if w := s.serve(t, r); w.Code != http.StatusOK {
			t.Errorf("%s without key from non-localhost: status = %d, want 200", path, w.Code)
		}
	}
}

func TestAuthMiddlewareAPIKey(t *testing.T) {
	s := newTestServer("secret-key")

	cases := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"no key", nil, http.StatusUnauthorized},
		{"wrong key", map[string]string{"X-API-Key": "nope"}, http.StatusUnauthorized},
		{"correct X-API-Key", map[string]string{"X-API-Key": "secret-key"}, http.StatusOK},
		{"correct Bearer", map[string]string{"Authorization": "Bearer secret-key"}, http.StatusOK},
		{"wrong Bearer", map[string]string{"Authorization": "Bearer nope"}, http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/workflows", nil)
			r.RemoteAddr = "8.8.8.8:1000"
			for k, v := range c.headers {
				r.Header.Set(k, v)
			}
			if w := s.serve(t, r); w.Code != c.want {
				t.Errorf("status = %d, want %d (body: %s)", w.Code, c.want, w.Body.String())
			}
		})
	}
}

func TestCORSMiddleware(t *testing.T) {
	s := newTestServer("")

	t.Run("preflight returns 204", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodOptions, "/api/v1/workflows", nil)
		w := s.serve(t, r)
		if w.Code != http.StatusNoContent {
			t.Errorf("OPTIONS status = %d, want 204", w.Code)
		}
	})

	t.Run("allowed localhost origin is echoed", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/health", nil)
		r.Header.Set("Origin", "http://localhost:3000")
		w := s.serve(t, r)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
			t.Errorf("ACAO = %q, want origin echoed", got)
		}
	})

	t.Run("foreign origin gets no ACAO header", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/health", nil)
		r.Header.Set("Origin", "https://evil.example.com")
		w := s.serve(t, r)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("ACAO = %q, want empty for foreign origin", got)
		}
	})
}

func TestMetricsMiddlewareCountsRequests(t *testing.T) {
	s := newTestServer("")
	before := atomic.LoadUint64(&s.requestsTotal)
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	s.serve(t, r)
	if got := atomic.LoadUint64(&s.requestsTotal); got != before+1 {
		t.Errorf("requestsTotal = %d, want %d", got, before+1)
	}
}

// --- handlers ---

func TestHandleHealth(t *testing.T) {
	s := newTestServer("")

	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := s.serve(t, r)
	if w.Code != http.StatusOK {
		t.Fatalf("GET status = %d", w.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %v, want ok", body["status"])
	}
	if body["version"] != meta.Version {
		t.Errorf("version field = %v, want %q (must report the real build version, not a hardcoded string)", body["version"], meta.Version)
	}

	r = httptest.NewRequest(http.MethodPost, "/health", nil)
	if w := s.serve(t, r); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", w.Code)
	}
}

func TestHandleMetrics(t *testing.T) {
	s := newTestServer("")

	r := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	w := s.serve(t, r)
	if w.Code != http.StatusOK {
		t.Fatalf("GET status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "# HELP") && !strings.Contains(w.Body.String(), "# TYPE") {
		t.Error("body does not look like Prometheus exposition format")
	}

	r = httptest.NewRequest(http.MethodPost, "/api/v1/metrics", nil)
	if w := s.serve(t, r); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", w.Code)
	}
}

// --- workflow handlers ---

// writeWorkflowDir creates a temp directory with one valid and one
// unparseable workflow file.
func writeWorkflowDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	valid := `name: Greeting Workflow
description: renders a greeting
steps:
  - node: template_render
    params:
      template: "hello"
`
	if err := os.WriteFile(filepath.Join(dir, "greeting.yaml"), []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.yaml"), []byte(":\tnot: [valid"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestHandleListWorkflows(t *testing.T) {
	s := newTestServer("")
	s.SetWorkflowsDir(writeWorkflowDir(t))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/workflows", nil)
	w := s.serve(t, r)
	if w.Code != http.StatusOK {
		t.Fatalf("GET status = %d (body: %s)", w.Code, w.Body.String())
	}
	var resp struct {
		Workflows []workflowInfo `json:"workflows"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if len(resp.Workflows) != 2 {
		t.Fatalf("got %d workflows, want 2 (valid + broken)", len(resp.Workflows))
	}

	byName := map[string]workflowInfo{}
	for _, wf := range resp.Workflows {
		byName[wf.File] = wf
	}
	if g, ok := byName["greeting.yaml"]; !ok {
		t.Fatal("greeting.yaml missing from listing")
	} else if g.Steps != 1 || g.Name != "Greeting Workflow" {
		t.Errorf("greeting.yaml = %+v, want 1 step with name", g)
	}
	if b, ok := byName["broken.yaml"]; !ok {
		t.Fatal("broken.yaml missing from listing")
	} else if !strings.Contains(b.Description, "parse error") {
		t.Errorf("broken.yaml description = %q, want parse-error note", b.Description)
	}

	r = httptest.NewRequest(http.MethodPost, "/api/v1/workflows", nil)
	if w := s.serve(t, r); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", w.Code)
	}
}

func TestHandleGetWorkflow(t *testing.T) {
	s := newTestServer("")
	s.SetWorkflowsDir(writeWorkflowDir(t))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/greeting", nil)
	w := s.serve(t, r)
	if w.Code != http.StatusOK {
		t.Fatalf("GET valid workflow: status = %d (body: %s)", w.Code, w.Body.String())
	}
	var resp struct {
		Name      string `json:"name"`
		StepCount int    `json:"step_count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if resp.Name != "Greeting Workflow" || resp.StepCount != 1 {
		t.Errorf("resp = %+v, want Greeting Workflow / 1 step", resp)
	}

	// Path traversal must be rejected before any filesystem access.
	for _, name := range []string{"..%2F..%2Fetc%2Fpasswd", "a/b", `a\b`} {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+name, nil)
		if w := s.serve(t, r); w.Code != http.StatusBadRequest {
			t.Errorf("GET %q: status = %d, want 400", name, w.Code)
		}
	}

	r = httptest.NewRequest(http.MethodGet, "/api/v1/workflows/does-not-exist", nil)
	if w := s.serve(t, r); w.Code != http.StatusNotFound {
		t.Errorf("GET missing workflow: status = %d, want 404", w.Code)
	}

	r = httptest.NewRequest(http.MethodPost, "/api/v1/workflows/greeting", nil)
	if w := s.serve(t, r); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", w.Code)
	}
}

func TestHandleRunWorkflow(t *testing.T) {
	s := newTestServer("")

	t.Run("method not allowed", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/run", nil)
		if w := s.serve(t, r); w.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", w.Code)
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		r := newJSONRequest(t, http.MethodPost, "/api/v1/workflows/run", "{not json")
		if w := s.serve(t, r); w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400 (body: %s)", w.Code, w.Body.String())
		}
	})

	t.Run("empty workflow rejected", func(t *testing.T) {
		r := newJSONRequest(t, http.MethodPost, "/api/v1/workflows/run", `{"workflow":""}`)
		if w := s.serve(t, r); w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("unparseable workflow rejected", func(t *testing.T) {
		r := newJSONRequest(t, http.MethodPost, "/api/v1/workflows/run", `{"workflow":":\tnot: [valid"}`)
		w := s.serve(t, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body: %s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "failed to parse workflow") {
			t.Errorf("body should report parse failure: %s", w.Body.String())
		}
	})

	t.Run("valid workflow executes", func(t *testing.T) {
		wf := `name: inline-test
steps:
  - node: template_render
    params:
      template: "hello api"
`
		r := newJSONRequest(t, http.MethodPost, "/api/v1/workflows/run",
			`{"workflow":`+mustJSON(t, wf)+`}`)
		w := s.serve(t, r)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d (body: %s)", w.Code, w.Body.String())
		}
		var resp runWorkflowResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response not JSON: %v", err)
		}
		if !resp.Success {
			t.Errorf("success = false, error = %q", resp.Error)
		}
		if len(resp.StepResults) != 1 || resp.StepResults[0].NodeName != "template_render" {
			t.Errorf("step results = %+v, want one template_render step", resp.StepResults)
		}
	})
}

func TestHandleResumeWorkflow(t *testing.T) {
	s := newTestServer("")

	t.Run("method not allowed", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/resume", nil)
		if w := s.serve(t, r); w.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", w.Code)
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		r := newJSONRequest(t, http.MethodPost, "/api/v1/workflows/resume", "{broken")
		if w := s.serve(t, r); w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("empty run_id rejected", func(t *testing.T) {
		r := newJSONRequest(t, http.MethodPost, "/api/v1/workflows/resume", `{"run_id":""}`)
		if w := s.serve(t, r); w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("path traversal in run_id rejected", func(t *testing.T) {
		for _, id := range []string{"../escape", `a\b`, "sub/dir"} {
			r := newJSONRequest(t, http.MethodPost, "/api/v1/workflows/resume",
				`{"run_id":`+mustJSON(t, id)+`}`)
			if w := s.serve(t, r); w.Code != http.StatusBadRequest {
				t.Errorf("run_id %q: status = %d, want 400", id, w.Code)
			}
		}
	})

	t.Run("unknown run_id is 404", func(t *testing.T) {
		r := newJSONRequest(t, http.MethodPost, "/api/v1/workflows/resume",
			`{"run_id":"no-such-run-xyz"}`)
		if w := s.serve(t, r); w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404 (body: %s)", w.Code, w.Body.String())
		}
	})
}

// --- chat handler (validation paths) ---

func TestHandleChatValidation(t *testing.T) {
	s := newTestServer("")

	t.Run("method not allowed", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/chat", nil)
		if w := s.serve(t, r); w.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", w.Code)
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		r := newJSONRequest(t, http.MethodPost, "/api/v1/chat", "{broken")
		if w := s.serve(t, r); w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("empty message rejected", func(t *testing.T) {
		r := newJSONRequest(t, http.MethodPost, "/api/v1/chat", `{"message":""}`)
		if w := s.serve(t, r); w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})
}

// mustJSON marshals v for embedding in a request body.
func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
