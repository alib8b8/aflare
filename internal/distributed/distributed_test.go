// Copyright (c) 2026 llm-box Contributors
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

package distributed

import (
	"crypto/subtle"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewCoordinator_DefaultPort(t *testing.T) {
	c := NewCoordinator("", "test-token")
	if c.port != defaultCoordinatorPort {
		t.Errorf("expected default port %s, got %s", defaultCoordinatorPort, c.port)
	}
	if c.authToken != "test-token" {
		t.Error("expected auth token to be set")
	}
}

func TestNewCoordinator_CustomPort(t *testing.T) {
	c := NewCoordinator("9090", "test-token")
	if c.port != "9090" {
		t.Errorf("expected port 9090, got %s", c.port)
	}
}

func TestCoordinatorAuthMiddleware_EmptyTokenRejects(t *testing.T) {
	c := NewCoordinator("8090", "")

	handler := c.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when token is empty")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/nodes", nil)
	req.Header.Set("X-Auth-Token", "some-token")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected StatusServiceUnavailable (503), got %d", w.Code)
	}
}

func TestCoordinatorAuthMiddleware_ValidToken(t *testing.T) {
	c := NewCoordinator("8090", "secret-token-123")

	called := false
	handler := c.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/nodes", nil)
	req.Header.Set("X-Auth-Token", "secret-token-123")
	w := httptest.NewRecorder()

	handler(w, req)

	if !called {
		t.Error("expected handler to be called with valid token")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected StatusOK (200), got %d", w.Code)
	}
}

func TestCoordinatorAuthMiddleware_InvalidToken(t *testing.T) {
	c := NewCoordinator("8090", "secret-token-123")

	called := false
	handler := c.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/nodes", nil)
	req.Header.Set("X-Auth-Token", "wrong-token")
	w := httptest.NewRecorder()

	handler(w, req)

	if called {
		t.Error("handler should not be called with invalid token")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected StatusUnauthorized (401), got %d", w.Code)
	}
}

func TestCoordinatorAuthMiddleware_MissingToken(t *testing.T) {
	c := NewCoordinator("8090", "secret-token-123")

	called := false
	handler := c.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/nodes", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if called {
		t.Error("handler should not be called with missing token")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected StatusUnauthorized (401), got %d", w.Code)
	}
}

func TestWorkerAuthMiddleware_EmptyTokenRejects(t *testing.T) {
	w, err := NewWorker("8091", "http://localhost:8090", "", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := w.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when token is empty")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/api/execute-step", nil)
	req.Header.Set("X-Auth-Token", "some-token")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected StatusServiceUnavailable (503), got %d", rec.Code)
	}
}

func TestWorkerAuthMiddleware_ValidToken(t *testing.T) {
	w, err := NewWorker("8091", "http://localhost:8090", "worker-secret", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	called := false
	handler := w.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/api/execute-step", nil)
	req.Header.Set("X-Auth-Token", "worker-secret")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if !called {
		t.Error("expected handler to be called with valid token")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected StatusOK (200), got %d", rec.Code)
	}
}

func TestWorkerAuthMiddleware_InvalidToken(t *testing.T) {
	w, err := NewWorker("8091", "http://localhost:8090", "worker-secret", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	called := false
	handler := w.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/api/execute-step", nil)
	req.Header.Set("X-Auth-Token", "wrong-token")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if called {
		t.Error("handler should not be called with invalid token")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected StatusUnauthorized (401), got %d", rec.Code)
	}
}

func TestConstantTimeCompare_Usage(t *testing.T) {
	token := make([]byte, 32)
	for i := range token {
		token[i] = byte(i)
	}
	valid := make([]byte, 32)
	copy(valid, token)
	invalid := make([]byte, 32)
	copy(invalid, token)
	invalid[0] ^= 0x01

	if subtle.ConstantTimeCompare(token, valid) != 1 {
		t.Error("expected valid tokens to match")
	}
	if subtle.ConstantTimeCompare(token, invalid) != 0 {
		t.Error("expected invalid tokens to not match")
	}
	if subtle.ConstantTimeCompare(token, []byte("")) != 0 {
		t.Error("expected empty token to not match")
	}
}

func TestIsValidPort(t *testing.T) {
	tests := []struct {
		port string
		want bool
	}{
		{"8080", true},
		{"80", true},
		{"65535", true},
		{"", false},
		{"abc", false},
		{"80a", false},
		{"-1", false},
	}

	for _, tt := range tests {
		got := isValidPort(tt.port)
		if got != tt.want {
			t.Errorf("isValidPort(%q) = %v, want %v", tt.port, got, tt.want)
		}
	}
}

func TestIsValidCoordinatorURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"http://localhost:8090", true},
		{"https://coordinator.example.com", true},
		{"ftp://example.com", false},
		{"localhost:8090", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isValidCoordinatorURL(tt.url)
		if got != tt.want {
			t.Errorf("isValidCoordinatorURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestNewWorker_InvalidPort(t *testing.T) {
	_, err := NewWorker("abc", "http://localhost:8090", "token", 1)
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestNewWorker_InvalidCoordinatorURL(t *testing.T) {
	_, err := NewWorker("8091", "ftp://localhost:8090", "token", 1)
	if err == nil {
		t.Error("expected error for invalid coordinator URL")
	}
}

func TestSelectBestNode_EmptyNodes(t *testing.T) {
	c := NewCoordinator("8090", "token")
	best := c.selectBestNode()
	if best != "" {
		t.Errorf("expected empty string for no nodes, got %q", best)
	}
}

func TestSelectBestNodeLocked_EmptyNodes(t *testing.T) {
	c := NewCoordinator("8090", "token")
	c.mu.RLock()
	defer c.mu.RUnlock()
	best := c.selectBestNodeLocked()
	if best != "" {
		t.Errorf("expected empty string for no nodes, got %q", best)
	}
}

func TestGetCoordinatorAddress_Default(t *testing.T) {
	t.Setenv("LLM_BOX_COORDINATOR", "")
	addr := GetCoordinatorAddress()
	expected := "http://localhost:" + defaultCoordinatorPort
	if addr != expected {
		t.Errorf("expected %s, got %s", expected, addr)
	}
}

func TestGetCoordinatorAddress_Custom(t *testing.T) {
	t.Setenv("LLM_BOX_COORDINATOR", "http://custom:9090")
	addr := GetCoordinatorAddress()
	if addr != "http://custom:9090" {
		t.Errorf("expected custom address, got %s", addr)
	}
}

func TestHealthEndpoint_NoAuthRequired(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected health endpoint to return 200 without auth, got %d", w.Code)
	}
}

func TestNodeStatusConstants(t *testing.T) {
	if NodeStatusIdle != "idle" {
		t.Error("unexpected NodeStatusIdle")
	}
	if NodeStatusBusy != "busy" {
		t.Error("unexpected NodeStatusBusy")
	}
	if NodeStatusOffline != "offline" {
		t.Error("unexpected NodeStatusOffline")
	}
}

func TestTaskStatusConstants(t *testing.T) {
	if TaskStatusPending != "pending" {
		t.Error("unexpected TaskStatusPending")
	}
	if TaskStatusRunning != "running" {
		t.Error("unexpected TaskStatusRunning")
	}
	if TaskStatusCompleted != "completed" {
		t.Error("unexpected TaskStatusCompleted")
	}
	if TaskStatusFailed != "failed" {
		t.Error("unexpected TaskStatusFailed")
	}
}

func TestMaxRequestBodySize(t *testing.T) {
	if maxRequestBodySize != 10*1024*1024 {
		t.Errorf("expected maxRequestBodySize to be 10MB, got %d", maxRequestBodySize)
	}
}
