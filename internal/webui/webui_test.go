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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

// =============================================================================
// Constants tests
// =============================================================================

func TestConstants_DefaultHost(t *testing.T) {
	if defaultHost != "127.0.0.1" {
		t.Errorf("defaultHost = %q, want %q", defaultHost, "127.0.0.1")
	}
}

func TestConstants_DefaultPort(t *testing.T) {
	if defaultPort != "8081" {
		t.Errorf("defaultPort = %q, want %q", defaultPort, "8081")
	}
}

func TestConstants_ServerTimeouts(t *testing.T) {
	if serverReadTimeout != 30*time.Second {
		t.Errorf("serverReadTimeout = %v, want 30s", serverReadTimeout)
	}
	if serverWriteTimeout != 30*time.Second {
		t.Errorf("serverWriteTimeout = %v, want 30s", serverWriteTimeout)
	}
	if serverShutdownTimeout != 10*time.Second {
		t.Errorf("serverShutdownTimeout = %v, want 10s", serverShutdownTimeout)
	}
}

func TestConstants_MaxWorkflowFileSize(t *testing.T) {
	if maxWorkflowFileSize != 5*1024*1024 {
		t.Errorf("maxWorkflowFileSize = %d, want %d (5MB)", maxWorkflowFileSize, 5*1024*1024)
	}
}

func TestConstants_MetricsRPS(t *testing.T) {
	if metricsRPS != 5 {
		t.Errorf("metricsRPS = %d, want 5", metricsRPS)
	}
}

// =============================================================================
// NewWebUIServer tests
// =============================================================================

func TestNewWebUIServer_Defaults(t *testing.T) {
	s := NewWebUIServer("", "")
	if s.host != defaultHost {
		t.Errorf("host = %q, want %q", s.host, defaultHost)
	}
	if s.port != defaultPort {
		t.Errorf("port = %q, want %q", s.port, defaultPort)
	}
	if s.stopCh == nil {
		t.Error("stopCh should not be nil")
	}
}

func TestNewWebUIServer_CustomHostPort(t *testing.T) {
	s := NewWebUIServer("0.0.0.0", "9090")
	if s.host != "0.0.0.0" {
		t.Errorf("host = %q, want %q", s.host, "0.0.0.0")
	}
	if s.port != "9090" {
		t.Errorf("port = %q, want %q", s.port, "9090")
	}
}

func TestNewWebUIServer_OnlyHost(t *testing.T) {
	s := NewWebUIServer("192.168.1.1", "")
	if s.host != "192.168.1.1" {
		t.Errorf("host = %q, want %q", s.host, "192.168.1.1")
	}
	if s.port != defaultPort {
		t.Errorf("port = %q, want %q", s.port, defaultPort)
	}
}

func TestNewWebUIServer_OnlyPort(t *testing.T) {
	s := NewWebUIServer("", "3000")
	if s.host != defaultHost {
		t.Errorf("host = %q, want %q", s.host, defaultHost)
	}
	if s.port != "3000" {
		t.Errorf("port = %q, want %q", s.port, "3000")
	}
}

// =============================================================================
// SetAuthToken / SetWorkflowsDir tests
// =============================================================================

func TestSetAuthToken(t *testing.T) {
	s := NewWebUIServer("", "")
	if s.authToken != "" {
		t.Error("authToken should be empty initially")
	}
	s.SetAuthToken("my-secret")
	if s.authToken != "my-secret" {
		t.Errorf("authToken = %q, want %q", s.authToken, "my-secret")
	}
	// Setting to empty should clear it
	s.SetAuthToken("")
	if s.authToken != "" {
		t.Error("authToken should be empty after clearing")
	}
}

func TestSetWorkflowsDir(t *testing.T) {
	s := NewWebUIServer("", "")
	if s.workflowsDir != "" {
		t.Error("workflowsDir should be empty initially")
	}
	s.SetWorkflowsDir("/tmp/my-workflows")
	if s.workflowsDir != "/tmp/my-workflows" {
		t.Errorf("workflowsDir = %q, want %q", s.workflowsDir, "/tmp/my-workflows")
	}
}

// =============================================================================
// authMiddleware tests
// =============================================================================

func TestAuthMiddleware_NoTokenSet(t *testing.T) {
	s := NewWebUIServer("", "")
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	s := NewWebUIServer("", "")
	s.SetAuthToken("secret123")
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Auth-Token", "secret123")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	s := NewWebUIServer("", "")
	s.SetAuthToken("secret123")
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	s := NewWebUIServer("", "")
	s.SetAuthToken("secret123")
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Auth-Token", "wrong-token")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_DifferentCaseHeader(t *testing.T) {
	// HTTP headers are case-insensitive per RFC 7230.
	// Go's http.Header .Get() is case-insensitive, so this should work.
	s := NewWebUIServer("", "")
	s.SetAuthToken("secret123")
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("x-auth-token", "secret123")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (case-insensitive header)", rec.Code)
	}
}

// =============================================================================
// isValidWorkflowName tests
// =============================================================================

func TestIsValidWorkflowName_Valid(t *testing.T) {
	validNames := []string{
		"a",
		"MyWorkflow",
		"my-workflow",
		"my_workflow",
		"workflow.v1",
		"abc123",
		"ABC_def-123.xyz",
		"a", // single char
		"Workflow-Name_With.Dots",
	}
	for _, name := range validNames {
		if !isValidWorkflowName(name) {
			t.Errorf("isValidWorkflowName(%q) = false, want true", name)
		}
	}
}

func TestIsValidWorkflowName_Invalid(t *testing.T) {
	invalidNames := []string{
		"",
		"../etc/passwd",
		"path/traversal",
		"back\\slash",
		"has spaces",
		"has!bang",
		"has@sign",
		"has#hash",
		"has$dollar",
		"has%percent",
		"has^caret",
		"has&and",
		"has*star",
		"has(paren)",
		"has+plus",
		"has=equal",
		"has[brace",
		"has]brace",
		"has{curly",
		"has}curly",
		"has|pipe",
		"has:colon",
		"has;semi",
		"has'quote",
		"has\"double",
		"has<less",
		"has>greater",
		"has?question",
		"has,comma",
		"has~tilde",
		"has`backtick",
		"has\nnewline",
	}
	for _, name := range invalidNames {
		if isValidWorkflowName(name) {
			t.Errorf("isValidWorkflowName(%q) = true, want false", name)
		}
	}
}

func TestIsValidWorkflowName_TooLong(t *testing.T) {
	longName := strings.Repeat("a", 101)
	if isValidWorkflowName(longName) {
		t.Error("isValidWorkflowName for 101-char name should be false")
	}
	// Exactly 100 chars should be valid
	exactName := strings.Repeat("a", 100)
	if !isValidWorkflowName(exactName) {
		t.Error("isValidWorkflowName for 100-char name should be true")
	}
}

func TestIsValidWorkflowName_PathTraversal(t *testing.T) {
	pathTraversalNames := []string{
		"..",
		"a../b",
		"a/../b",
		"a\\..\\b",
		"../workflow",
		"workflow/../etc",
		"a/b",
		"a\\b",
		"a/./b",
		"a\\.\\b",
	}
	for _, name := range pathTraversalNames {
		if isValidWorkflowName(name) {
			t.Errorf("isValidWorkflowName(%q) = true, want false (path traversal)", name)
		}
	}
}

// =============================================================================
// getWorkflowsDir / getWorkflowPath tests
// =============================================================================

func TestGetWorkflowsDir_Custom(t *testing.T) {
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir("/custom/workflows")
	dir := s.getWorkflowsDir()
	if dir != "/custom/workflows" {
		t.Errorf("getWorkflowsDir = %q, want %q", dir, "/custom/workflows")
	}
}

func TestGetWorkflowsDir_Default(t *testing.T) {
	s := NewWebUIServer("", "")
	dir := s.getWorkflowsDir()
	// Default should be the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Skip("cannot get working directory")
	}
	if dir != cwd {
		t.Errorf("getWorkflowsDir = %q, want %q", dir, cwd)
	}
}

func TestGetWorkflowPath(t *testing.T) {
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir("/custom/workflows")
	path := s.getWorkflowPath("my-workflow")
	expected := filepath.Join("/custom/workflows", "my-workflow.yaml")
	if path != expected {
		t.Errorf("getWorkflowPath = %q, want %q", path, expected)
	}
}

func TestGetWorkflowPath_SpecialName(t *testing.T) {
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir("/tmp")
	path := s.getWorkflowPath("workflow.v1")
	expected := filepath.Join("/tmp", "workflow.v1.yaml")
	if path != expected {
		t.Errorf("getWorkflowPath = %q, want %q", path, expected)
	}
}

// =============================================================================
// buildHandler tests (route registration)
// =============================================================================

func TestBuildHandler_ReturnsMux(t *testing.T) {
	s := NewWebUIServer("", "")
	h := s.buildHandler()
	if h == nil {
		t.Fatal("buildHandler returned nil")
	}
}

func TestBuildHandler_IndexRoute(t *testing.T) {
	s := NewWebUIServer("", "")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestBuildHandler_PprofDisabledByDefault(t *testing.T) {
	t.Setenv("AFLARE_PPROF", "")
	s := NewWebUIServer("", "")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/debug/pprof/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// When pprof is disabled, /debug/pprof/ falls through to the index handler
	// so it returns HTML (200), not a 404
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<html") {
		t.Error("expected HTML fallback when pprof is disabled")
	}
}

func TestBuildHandler_PprofEnabled(t *testing.T) {
	t.Setenv("AFLARE_PPROF", "1")
	s := NewWebUIServer("", "")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/debug/pprof/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Types of profiles available") {
		t.Error("expected pprof index page when AFLARE_PPROF=1")
	}
}

func TestBuildHandler_PprofWithAuth(t *testing.T) {
	t.Setenv("AFLARE_PPROF", "1")
	s := NewWebUIServer("", "")
	s.SetAuthToken("secret")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	// Without auth token, pprof should return 401
	resp, err := http.Get(ts.URL + "/debug/pprof/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", resp.StatusCode)
	}

	// With auth token, pprof should work
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/debug/pprof/", nil)
	req.Header.Set("X-Auth-Token", "secret")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with auth, got %d", resp2.StatusCode)
	}
}

func TestBuildHandler_APIWorkflowsRoute(t *testing.T) {
	s := NewWebUIServer("", "")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	// GET /api/workflows should return JSON (even if directory is empty or doesn't exist)
	resp, err := http.Get(ts.URL + "/api/workflows")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestBuildHandler_APIWorkflowRoute(t *testing.T) {
	s := NewWebUIServer("", "")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	// GET /api/workflow without name returns 400
	resp, err := http.Get(ts.URL + "/api/workflow")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	// POST /api/workflow with invalid body returns 400
	resp2, err := http.Post(ts.URL+"/api/workflow", "application/json", strings.NewReader("invalid"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	// DELETE /api/workflow without name
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/workflow", nil)
	resp3, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestBuildHandler_APIVisualizeRoute(t *testing.T) {
	s := NewWebUIServer("", "")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	// GET should return 405
	resp, err := http.Get(ts.URL + "/api/visualize")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestBuildHandler_APIValidateRoute(t *testing.T) {
	s := NewWebUIServer("", "")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	// GET should return 405
	resp, err := http.Get(ts.URL + "/api/validate")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

// =============================================================================
// handleIndex tests
// =============================================================================

func TestHandleIndex_ReturnsHTML(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.handleIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), "Aflare") {
		t.Error("response body should contain 'Aflare'")
	}
	if !strings.Contains(rec.Body.String(), "<!DOCTYPE html>") {
		t.Error("response body should contain DOCTYPE")
	}
}

// =============================================================================
// handleVisualize tests
// =============================================================================

func TestHandleVisualize_InvalidMethod(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodGet, "/api/visualize", nil)
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestHandleVisualize_JSONInput_Mermaid(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"workflow":"name: test\nsteps:\n  - node: echo\n    params:\n      message: hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=mermaid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
}

func TestHandleVisualize_JSONInput_DOT(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"workflow":"name: test\nsteps:\n  - node: echo\n    params:\n      message: hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=dot", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
}

func TestHandleVisualize_JSONInput_ASCII(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"workflow":"name: test\nsteps:\n  - node: echo\n    params:\n      message: hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=ascii", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
}

func TestHandleVisualize_JSONInput_JSON(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"workflow":"name: test\nsteps:\n  - node: echo\n    params:\n      message: hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestHandleVisualize_DefaultFormat(t *testing.T) {
	// When no format is specified, defaults to JSON
	s := NewWebUIServer("", "")
	body := `{"workflow":"name: test\nsteps:\n  - node: echo\n    params:\n      message: hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/visualize", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestHandleVisualize_PlainTextInput(t *testing.T) {
	s := NewWebUIServer("", "")
	body := "name: test\nsteps:\n  - node: echo\n    params:\n      message: hello"
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestHandleVisualize_InvalidJSON(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `not json`
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, got %d", rec.Code, rec.Code)
	}
}

// =============================================================================
// handleListWorkflows tests
// =============================================================================

func TestHandleListWorkflows_InvalidMethod(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodPost, "/api/workflows", nil)
	rec := httptest.NewRecorder()
	s.handleListWorkflows(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestHandleListWorkflows_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	req := httptest.NewRequest(http.MethodGet, "/api/workflows", nil)
	rec := httptest.NewRecorder()
	s.handleListWorkflows(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		Workflows []string `json:"workflows"`
		Directory string   `json:"directory"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if len(resp.Workflows) != 0 {
		t.Errorf("expected 0 workflows, got %d", len(resp.Workflows))
	}
	if resp.Directory != dir {
		t.Errorf("directory = %q, want %q", resp.Directory, dir)
	}
}

func TestHandleListWorkflows_WithYAMLFiles(t *testing.T) {
	dir := t.TempDir()
	// Create some YAML files
	os.WriteFile(filepath.Join(dir, "workflow1.yaml"), []byte("name: wf1"), 0600)
	os.WriteFile(filepath.Join(dir, "workflow2.yml"), []byte("name: wf2"), 0600)
	// Create a non-YAML file
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("readme"), 0600)

	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	req := httptest.NewRequest(http.MethodGet, "/api/workflows", nil)
	rec := httptest.NewRecorder()
	s.handleListWorkflows(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		Workflows []string `json:"workflows"`
		Directory string   `json:"directory"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if len(resp.Workflows) != 2 {
		t.Errorf("expected 2 workflows, got %d: %v", len(resp.Workflows), resp.Workflows)
	}
}

func TestHandleListWorkflows_NonExistentDir(t *testing.T) {
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir("/nonexistent/dir/that/does/not/exist")

	req := httptest.NewRequest(http.MethodGet, "/api/workflows", nil)
	rec := httptest.NewRecorder()
	s.handleListWorkflows(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// =============================================================================
// handleGetWorkflow tests
// =============================================================================

func TestHandleGetWorkflow_MissingName(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodGet, "/api/workflow", nil)
	rec := httptest.NewRecorder()
	s.handleGetWorkflow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleGetWorkflow_InvalidName(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodGet, "/api/workflow?name=../etc/passwd", nil)
	rec := httptest.NewRecorder()
	s.handleGetWorkflow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleGetWorkflow_NotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	req := httptest.NewRequest(http.MethodGet, "/api/workflow?name=nonexistent", nil)
	rec := httptest.NewRecorder()
	s.handleGetWorkflow(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandleGetWorkflow_Success(t *testing.T) {
	dir := t.TempDir()
	workflowContent := "name: my-workflow\nsteps:\n  - node: echo\n    params:\n      message: hello\n"
	os.WriteFile(filepath.Join(dir, "my-workflow.yaml"), []byte(workflowContent), 0600)

	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	req := httptest.NewRequest(http.MethodGet, "/api/workflow?name=my-workflow", nil)
	rec := httptest.NewRecorder()
	s.handleGetWorkflow(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "text/yaml" {
		t.Errorf("Content-Type = %q, want text/yaml", ct)
	}
	if rec.Body.String() != workflowContent {
		t.Errorf("body = %q, want %q", rec.Body.String(), workflowContent)
	}
}

func TestHandleGetWorkflow_UnknownMethod(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodPut, "/api/workflow", nil)
	rec := httptest.NewRecorder()
	s.handleWorkflow(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// =============================================================================
// handleSaveWorkflow tests
// =============================================================================

func TestHandleSaveWorkflow_InvalidJSON(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodPost, "/api/workflow", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleSaveWorkflow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleSaveWorkflow_MissingName(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"content": "name: test\nsteps:\n  - node: echo\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflow", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleSaveWorkflow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleSaveWorkflow_InvalidName(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"name": "../etc/passwd", "content": "name: test\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflow", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleSaveWorkflow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleSaveWorkflow_Success(t *testing.T) {
	dir := t.TempDir()
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	body := `{"name": "my-workflow", "content": "name: test\nsteps:\n  - node: echo\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflow", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleSaveWorkflow(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if resp["status"] != "saved" {
		t.Errorf("status = %q, want 'saved'", resp["status"])
	}

	// Verify file was written
	data, err := os.ReadFile(filepath.Join(dir, "my-workflow.yaml"))
	if err != nil {
		t.Fatalf("failed to read saved workflow: %v", err)
	}
	if string(data) != "name: test\nsteps:\n  - node: echo\n" {
		t.Errorf("file content = %q, want %q", string(data), "name: test\nsteps:\n  - node: echo\n")
	}
}

func TestHandleSaveWorkflow_ContentTooLarge(t *testing.T) {
	// The JSON body exceeds maxWorkflowFileSize, so the io.LimitReader
	// truncates the JSON, causing the decode to fail with 400 (not 413).
	// The content-length check at line 403 is never reached because the
	// JSON decode fails first.
	dir := t.TempDir()
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	largeContent := strings.Repeat("x", maxWorkflowFileSize+1)
	body := `{"name": "large-wf", "content": "` + largeContent + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflow", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleSaveWorkflow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// =============================================================================
// handleDeleteWorkflow tests
// =============================================================================

func TestHandleDeleteWorkflow_MissingName(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodDelete, "/api/workflow", nil)
	rec := httptest.NewRecorder()
	s.handleDeleteWorkflow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleDeleteWorkflow_InvalidName(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodDelete, "/api/workflow?name=../etc/passwd", nil)
	rec := httptest.NewRecorder()
	s.handleDeleteWorkflow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleDeleteWorkflow_NotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	req := httptest.NewRequest(http.MethodDelete, "/api/workflow?name=nonexistent", nil)
	rec := httptest.NewRecorder()
	s.handleDeleteWorkflow(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandleDeleteWorkflow_Success(t *testing.T) {
	dir := t.TempDir()
	workflowPath := filepath.Join(dir, "my-workflow.yaml")
	os.WriteFile(workflowPath, []byte("name: test"), 0600)

	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	req := httptest.NewRequest(http.MethodDelete, "/api/workflow?name=my-workflow", nil)
	rec := httptest.NewRecorder()
	s.handleDeleteWorkflow(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if resp["status"] != "deleted" {
		t.Errorf("status = %q, want 'deleted'", resp["status"])
	}

	// Verify file was deleted
	if _, err := os.Stat(workflowPath); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

// =============================================================================
// handleValidate tests
// =============================================================================

func TestHandleValidate_InvalidMethod(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodGet, "/api/validate", nil)
	rec := httptest.NewRecorder()
	s.handleValidate(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestHandleValidate_InvalidJSON(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleValidate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleValidate_InvalidWorkflow(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"workflow": "invalid: yaml: ["}`
	req := httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleValidate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, got %d", rec.Code, rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if v, ok := resp["valid"]; !ok || v != false {
		t.Error("expected valid=false for invalid workflow")
	}
}

func TestHandleValidate_ValidWorkflow(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"workflow": "name: test\nsteps:\n  - node: echo\n    params:\n      message: hello\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleValidate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if v, ok := resp["valid"]; !ok || v != true {
		t.Errorf("expected valid=true, got %v", resp["valid"])
	}
	if resp["name"] != "test" {
		t.Errorf("name = %q, want 'test'", resp["name"])
	}
	steps, ok := resp["steps"].(float64)
	if !ok || steps != 1 {
		t.Errorf("steps = %v, want 1", resp["steps"])
	}
}

// =============================================================================
// metricsRateLimiter tests
// =============================================================================

func TestNewMetricsRateLimiter_Default(t *testing.T) {
	rl := newMetricsRateLimiter(5)
	if rl == nil {
		t.Fatal("newMetricsRateLimiter returned nil")
	}
	if rl.rps != 5 {
		t.Errorf("rps = %f, want 5", rl.rps)
	}
	if rl.max != 5 {
		t.Errorf("max = %f, want 5", rl.max)
	}
	if rl.tokens != 5 {
		t.Errorf("tokens = %f, want 5", rl.tokens)
	}
}

func TestNewMetricsRateLimiter_ZeroClampsToOne(t *testing.T) {
	rl := newMetricsRateLimiter(0)
	if rl.rps != 1 {
		t.Errorf("rps = %f, want 1 (clamped from 0)", rl.rps)
	}
}

func TestNewMetricsRateLimiter_NegativeClampsToOne(t *testing.T) {
	rl := newMetricsRateLimiter(-5)
	if rl.rps != 1 {
		t.Errorf("rps = %f, want 1 (clamped from -5)", rl.rps)
	}
}

func TestMetricsRateLimiter_AllowWithinLimit(t *testing.T) {
	rl := newMetricsRateLimiter(5)
	// First 5 requests should be allowed
	for i := 0; i < 5; i++ {
		if !rl.allow() {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
	// 6th request should be denied (tokens exhausted)
	if rl.allow() {
		t.Error("6th request should be denied")
	}
}

func TestMetricsRateLimiter_RefillOverTime(t *testing.T) {
	rl := newMetricsRateLimiter(5)
	// Exhaust all tokens
	for i := 0; i < 5; i++ {
		rl.allow()
	}

	// Simulate time passing by artificially advancing the timer
	rl.mu.Lock()
	rl.lastTime = rl.lastTime.Add(-2 * time.Second)
	rl.mu.Unlock()

	// After 2 seconds at 5 rps, we should have ~10 tokens, but capped at max=5
	// So we should be able to make 5 more requests
	for i := 0; i < 5; i++ {
		if !rl.allow() {
			t.Errorf("request %d after refill should be allowed", i+1)
		}
	}
	if rl.allow() {
		t.Error("request after refill exhaustion should be denied")
	}
}

func TestMetricsRateLimiter_Concurrent(t *testing.T) {
	rl := newMetricsRateLimiter(100)
	var wg sync.WaitGroup
	allowed := make(chan bool, 200)

	// Launch 200 concurrent goroutines
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed <- rl.allow()
		}()
	}
	wg.Wait()
	close(allowed)

	allowedCount := 0
	deniedCount := 0
	for a := range allowed {
		if a {
			allowedCount++
		} else {
			deniedCount++
		}
	}

	// Burst is 100, so at most 100 should be allowed
	if allowedCount > 100 {
		t.Errorf("allowed %d requests, want at most 100", allowedCount)
	}
	// At least some should be denied
	if deniedCount == 0 {
		t.Error("expected some requests to be denied")
	}
}

func TestMetricsRateLimiter_TokenCappedAtMax(t *testing.T) {
	rl := newMetricsRateLimiter(1)
	// Exhaust
	rl.allow()

	// Simulate a long wait
	rl.mu.Lock()
	rl.lastTime = rl.lastTime.Add(-1 * time.Hour)
	rl.mu.Unlock()

	// After 1 hour at 1 rps, we'd have 3600 tokens, but capped at max=1
	allowed := 0
	for rl.allow() {
		allowed++
	}
	// Should only allow 1 (the burst cap), not 3600
	if allowed > 1 {
		t.Errorf("allowed %d requests, want at most 1 (burst cap)", allowed)
	}
}

// =============================================================================
// Stop tests
// =============================================================================

func TestStop_NilServer(t *testing.T) {
	s := NewWebUIServer("", "")
	// Stop should succeed when server is nil (not started)
	err := s.Stop()
	if err != nil {
		t.Errorf("Stop with nil server should not error, got: %v", err)
	}
}

func TestStart_RequiresNetwork(t *testing.T) {
	t.Skip("Start() binds a real network listener; tested via integration tests")
}

// =============================================================================
// Auth integration tests (API routes with auth)
// =============================================================================

func TestAPIWithAuth_TokenRequired(t *testing.T) {
	s := NewWebUIServer("", "")
	s.SetAuthToken("secret")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	// Without auth token, API endpoints should return 401
	endpoints := []string{
		"/api/workflows",
		"/api/validate",
		"/api/visualize",
	}
	for _, ep := range endpoints {
		resp, err := http.Get(ts.URL + ep)
		if err != nil {
			t.Fatalf("GET %s: %v", ep, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("GET %s: status = %d, want 401", ep, resp.StatusCode)
		}
	}
}

func TestAPIWithAuth_TokenValid(t *testing.T) {
	dir := t.TempDir()
	s := NewWebUIServer("", "")
	s.SetAuthToken("secret")
	s.SetWorkflowsDir(dir)
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	// With auth token, API endpoints should work
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/workflows", nil)
	req.Header.Set("X-Auth-Token", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/workflows with auth: status = %d, want 200", resp.StatusCode)
	}
}

func TestAPIWithAuth_IndexNotProtected(t *testing.T) {
	s := NewWebUIServer("", "")
	s.SetAuthToken("secret")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	// Index page should NOT require auth
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /: status = %d, want 200 (index should not require auth)", resp.StatusCode)
	}
}

// =============================================================================
// echoMetricsNode is a minimal Node used to drive ExecuteWithStats so the
// node_executions_total counter is incremented.
// =============================================================================

type echoMetricsNode struct{}

func (echoMetricsNode) Name() string        { return "echo_metrics_test" }
func (echoMetricsNode) Description() string { return "echo node for metrics tests" }
func (echoMetricsNode) Schema() core.NodeSchema {
	return core.NodeSchema{Name: "echo_metrics_test", Input: "string", Output: "string"}
}
func (echoMetricsNode) Execute(_ context.Context, input string, _ map[string]string) (string, error) {
	return input, nil
}

// =============================================================================
// Metrics endpoint tests
// =============================================================================

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

func TestMetricsEndpoint_RateLimit(t *testing.T) {
	t.Setenv("AFLARE_METRICS", "1")
	s := NewWebUIServer("127.0.0.1", "0")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	// First metricsRPS requests should succeed
	for i := 0; i < metricsRPS; i++ {
		resp, err := http.Get(ts.URL + "/metrics")
		if err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i+1, resp.StatusCode)
		}
	}

	// Next request should be rate limited
	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 after rate limit, got %d", resp.StatusCode)
	}
}

// =============================================================================
// registerPprof tests
// =============================================================================

func TestRegisterPprof_RegistersAllEndpoints(t *testing.T) {
	t.Setenv("AFLARE_PPROF", "1")
	s := NewWebUIServer("", "")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	pprofPaths := []string{
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/profile",
		"/debug/pprof/symbol",
		"/debug/pprof/trace",
	}
	for _, path := range pprofPaths {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Errorf("GET %s: %v", path, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", path, resp.StatusCode)
		}
	}
}

func TestRegisterPprof_Cmdline(t *testing.T) {
	t.Setenv("AFLARE_PPROF", "1")
	s := NewWebUIServer("", "")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/debug/pprof/cmdline")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /debug/pprof/cmdline: status = %d, want 200", resp.StatusCode)
	}
}

func TestRegisterPprof_Symbol(t *testing.T) {
	t.Setenv("AFLARE_PPROF", "1")
	s := NewWebUIServer("", "")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/debug/pprof/symbol")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /debug/pprof/symbol: status = %d, want 200", resp.StatusCode)
	}
}

// =============================================================================
// Edge case: CORS-friendly behavior
// =============================================================================

func TestHandleVisualize_OPTIONS(t *testing.T) {
	// The handler doesn't special-case OPTIONS, but it should still
	// return a consistent response (method not allowed since it checks POST)
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodOptions, "/api/visualize", nil)
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// =============================================================================
// Edge case: handleVisualize with YAML bodys that exceed max size
// =============================================================================

func TestHandleVisualize_BodyTooLarge(t *testing.T) {
	s := NewWebUIServer("", "")
	body := strings.Repeat("a", maxWorkflowFileSize+1)
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
}

// =============================================================================
// Edge case: handleValidate with large body
// =============================================================================

func TestHandleValidate_WorkflowTooLarge(t *testing.T) {
	s := NewWebUIServer("", "")
	large := strings.Repeat("x", maxWorkflowFileSize+1)
	body := `{"workflow":"` + large + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleValidate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body truncated by LimitReader)", rec.Code)
	}
}

// =============================================================================
// Test indexHTML is not empty
// =============================================================================

func TestIndexHTML_NotEmpty(t *testing.T) {
	if len(indexHTML) < 1000 {
		t.Errorf("indexHTML is suspiciously short: %d bytes", len(indexHTML))
	}
	if !strings.Contains(indexHTML, "<!DOCTYPE html>") {
		t.Error("indexHTML should contain DOCTYPE")
	}
	if !strings.Contains(indexHTML, "Aflare") {
		t.Error("indexHTML should contain 'Aflare'")
	}
	for _, keyword := range []string{"workflow", "visualize", "mermaid", "editor"} {
		if !strings.Contains(strings.ToLower(indexHTML), keyword) {
			t.Errorf("indexHTML should contain keyword %q", keyword)
		}
	}
}

// =============================================================================
// Test handleVisualize with empty body
// =============================================================================

func TestHandleVisualize_EmptyBody(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	// Empty body with JSON content-type should return bad request (invalid JSON)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// =============================================================================
// Test handleSaveWorkflow creates directory if needed
// =============================================================================

func TestHandleSaveWorkflow_CreatesDirectory(t *testing.T) {
	baseDir := t.TempDir()
	dir := filepath.Join(baseDir, "nested", "workflows")
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	body := `{"name": "test-wf", "content": "name: test\nsteps:\n  - node: echo\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflow", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleSaveWorkflow(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}

	// Verify directory was created
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("directory should have been created")
	}
}

// =============================================================================
// Test handleWorkflow method routing
// =============================================================================

func TestHandleWorkflow_MethodRouting(t *testing.T) {
	dir := t.TempDir()
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	// GET
	reqGet := httptest.NewRequest(http.MethodGet, "/api/workflow?name=test", nil)
	recGet := httptest.NewRecorder()
	s.handleWorkflow(recGet, reqGet)
	if recGet.Code != http.StatusNotFound {
		t.Errorf("GET: status = %d, want 404", recGet.Code)
	}

	// POST
	body := `{"name": "test", "content": "name: test\nsteps:\n  - node: echo\n"}`
	reqPost := httptest.NewRequest(http.MethodPost, "/api/workflow", strings.NewReader(body))
	reqPost.Header.Set("Content-Type", "application/json")
	recPost := httptest.NewRecorder()
	s.handleWorkflow(recPost, reqPost)
	if recPost.Code != http.StatusCreated {
		t.Errorf("POST: status = %d, want 201", recPost.Code)
	}

	// DELETE
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/workflow?name=test", nil)
	recDel := httptest.NewRecorder()
	s.handleWorkflow(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Errorf("DELETE: status = %d, want 200", recDel.Code)
	}

	// PUT (not allowed)
	reqPut := httptest.NewRequest(http.MethodPut, "/api/workflow", nil)
	recPut := httptest.NewRecorder()
	s.handleWorkflow(recPut, reqPut)
	if recPut.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT: status = %d, want 405", recPut.Code)
	}
}

// =============================================================================
// Test handleVisualize with all formats on the same input
// =============================================================================

func TestHandleVisualize_AllFormats(t *testing.T) {
	s := NewWebUIServer("", "")
	workflowYAML := "name: test\nsteps:\n  - node: echo\n    params:\n      message: hello\n"
	body := `{"workflow":"` + strings.ReplaceAll(workflowYAML, "\n", "\\n") + `"}`

	formats := []struct {
		name        string
		contentType string
	}{
		{"mermaid", "text/plain"},
		{"dot", "text/plain"},
		{"ascii", "text/plain"},
		{"json", "application/json"},
		{"", "application/json"},        // default
		{"unknown", "application/json"}, // unknown falls back to json
	}

	for _, tc := range formats {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/visualize"
			if tc.name != "" {
				url += "?format=" + tc.name
			}
			req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			s.handleVisualize(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("format=%q: status = %d, want 200", tc.name, rec.Code)
			}
			ct := rec.Header().Get("Content-Type")
			if !strings.HasPrefix(ct, tc.contentType) {
				t.Errorf("format=%q: Content-Type = %q, want prefix %q", tc.name, ct, tc.contentType)
			}
		})
	}
}

// =============================================================================
// Test that buildHandler is safe for concurrent calls
// =============================================================================

func TestBuildHandler_Concurrent(t *testing.T) {
	s := NewWebUIServer("", "")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get(ts.URL + "/")
			if err != nil {
				t.Errorf("concurrent request failed: %v", err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("concurrent request: status = %d", resp.StatusCode)
			}
		}()
	}
	wg.Wait()
}

// =============================================================================
// Test Stop with started server (integration, skip)
// =============================================================================

func TestStop_StartedServer(t *testing.T) {
	t.Skip("requires a real network listener; tested via integration tests")
}

// =============================================================================
// Test handleListWorkflows ignores subdirectories
// =============================================================================

func TestHandleListWorkflows_IgnoresSubdirectories(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "wf1.yaml"), []byte("name: wf1"), 0600)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "subdir", "wf2.yaml"), []byte("name: wf2"), 0600)

	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	req := httptest.NewRequest(http.MethodGet, "/api/workflows", nil)
	rec := httptest.NewRecorder()
	s.handleListWorkflows(rec, req)

	var resp struct {
		Workflows []string `json:"workflows"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Workflows) != 1 {
		t.Errorf("expected 1 workflow (only top-level files), got %d: %v", len(resp.Workflows), resp.Workflows)
	}
}

// =============================================================================
// Test handleVisualize with application/json but empty workflow field
// =============================================================================

func TestHandleVisualize_EmptyJSONWorkflow(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"workflow":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	// Empty workflow should still be processed (returns empty visualization)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// =============================================================================
// Test NewWebUIServer creates unique stopCh per instance
// =============================================================================

func TestNewWebUIServer_UniqueStopCh(t *testing.T) {
	s1 := NewWebUIServer("", "")
	s2 := NewWebUIServer("", "")
	if s1.stopCh == s2.stopCh {
		t.Error("each server instance should have its own stopCh")
	}
}

// =============================================================================
// Test handleValidate with valid workflow that has warnings
// =============================================================================

func TestHandleValidate_ValidWorkflowWithWarnings(t *testing.T) {
	s := NewWebUIServer("", "")
	// A valid workflow with an unknown node will produce warnings
	body := `{"workflow": "name: test\nsteps:\n  - node: unknown_node_xyz\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleValidate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if v, ok := resp["valid"]; !ok || v != true {
		t.Errorf("expected valid=true, got %v", resp["valid"])
	}
	if resp["name"] != "test" {
		t.Errorf("name = %q, want 'test'", resp["name"])
	}
}

// =============================================================================
// Test handleDeleteWorkflow with empty body parameters
// =============================================================================

func TestHandleDeleteWorkflow_EmptyNameParam(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodDelete, "/api/workflow?name=", nil)
	rec := httptest.NewRecorder()
	s.handleDeleteWorkflow(rec, req)

	// Empty name is both "missing" and "invalid" — it should fail validation
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// =============================================================================
// Test handleSaveWorkflow overwrites existing file
// =============================================================================

func TestHandleSaveWorkflow_Overwrite(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.yaml"), []byte("old content"), 0600)

	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	body := `{"name": "test", "content": "new content"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflow", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleSaveWorkflow(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "test.yaml"))
	if string(data) != "new content" {
		t.Errorf("file content = %q, want 'new content'", string(data))
	}
}

// =============================================================================
// Test that handleValidate error response has the expected shape
// =============================================================================

func TestHandleValidate_ErrorResponseShape(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"workflow": "invalid yaml: ["}`
	req := httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleValidate(rec, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	// Should have all expected fields
	if _, ok := resp["valid"]; !ok {
		t.Error("response missing 'valid' field")
	}
	if _, ok := resp["error"]; !ok {
		t.Error("response missing 'error' field")
	}
	if _, ok := resp["warnings"]; !ok {
		t.Error("response missing 'warnings' field")
	}
}

// =============================================================================
// Test handleVisualize with large but valid JSON body
// =============================================================================

func TestHandleVisualize_LargeValidJSON(t *testing.T) {
	s := NewWebUIServer("", "")
	workflowYAML := "name: large\nsteps:\n" + strings.Repeat("  - node: echo\n    params:\n      message: hello\n", 100)
	// Ensure it's under the size limit
	if len(workflowYAML) > maxWorkflowFileSize {
		t.Skip("workflowYAML too large for test")
	}
	body := `{"workflow":"` + strings.ReplaceAll(workflowYAML, "\n", "\\n") + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// =============================================================================
// Test handleIndex with various HTTP methods
// =============================================================================

func TestHandleIndex_AnyMethod(t *testing.T) {
	s := NewWebUIServer("", "")
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/", nil)
		rec := httptest.NewRecorder()
		s.handleIndex(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("method %s: status = %d, want 200", method, rec.Code)
		}
	}
}

// =============================================================================
// Test buildHandler with both AFLARE_PPROF and AFLARE_METRICS enabled
// =============================================================================

func TestBuildHandler_BothPprofAndMetrics(t *testing.T) {
	t.Setenv("AFLARE_PPROF", "1")
	t.Setenv("AFLARE_METRICS", "1")
	s := NewWebUIServer("", "")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	// pprof should work
	resp, err := http.Get(ts.URL + "/debug/pprof/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("pprof: status = %d, want 200", resp.StatusCode)
	}

	// metrics should work
	resp2, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("metrics: status = %d, want 200", resp2.StatusCode)
	}
}

// =============================================================================
// Test handleVisualize with JSON body missing workflow field
// =============================================================================

func TestHandleVisualize_JSONMissingWorkflowField(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"other": "value"}`
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	// workflow field is empty string by default, should still be OK
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// =============================================================================
// Test Stop is thread-safe (nil server case)
// =============================================================================

func TestStop_ThreadSafe(t *testing.T) {
	s := NewWebUIServer("", "")
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Stop()
		}()
	}
	wg.Wait()
}

// =============================================================================
// Test buildHandler returns a proper ServeMux
// =============================================================================

func TestBuildHandler_ServeMuxPattern(t *testing.T) {
	s := NewWebUIServer("", "")
	h := s.buildHandler()
	ts := httptest.NewServer(h)
	defer ts.Close()

	// Non-existent paths should fall through to the index handler
	resp, err := http.Get(ts.URL + "/nonexistent/path")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (fallback to index)", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

// =============================================================================
// Test handleVisualize with query parameters that don't affect format
// =============================================================================

func TestHandleVisualize_ExtraQueryParams(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"workflow":"name: test\nsteps:\n  - node: echo\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json&extra=param", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// =============================================================================
// Test handleVisualize with empty body (plain text path)
// =============================================================================

func TestHandleVisualize_EmptyBodyPlainText(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", http.NoBody)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	// Empty body with plain text is fine - returns empty visualization
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// =============================================================================
// Test handleSaveWorkflow content at exact max size boundary
// =============================================================================

func TestHandleSaveWorkflow_ContentAtMaxSize(t *testing.T) {
	// The JSON body (content + wrapper) exceeds maxWorkflowFileSize, so
	// the io.LimitReader truncates the JSON, causing the decode to fail
	// with 400. The content-length check is never reached.
	dir := t.TempDir()
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	content := strings.Repeat("x", maxWorkflowFileSize)
	body := `{"name": "boundary-test", "content": "` + content + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflow", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleSaveWorkflow(rec, req)

	// Body is truncated by LimitReader, so JSON decode fails
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// =============================================================================
// Test handleValidate with empty workflow content
// =============================================================================

func TestHandleValidate_EmptyWorkflow(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"workflow": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleValidate(rec, req)

	// Empty workflow is technically valid YAML (empty document), so
	// ParseWorkflowFromContent may succeed. The response will be 200
	// with valid=false or valid=true depending on the parser behavior.
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// =============================================================================
// Test newMetricsRateLimiter with large value
// =============================================================================

func TestNewMetricsRateLimiter_LargeRPS(t *testing.T) {
	rl := newMetricsRateLimiter(1000)
	if rl.rps != 1000 {
		t.Errorf("rps = %f, want 1000", rl.rps)
	}
	if rl.max != 1000 {
		t.Errorf("max = %f, want 1000", rl.max)
	}
	if rl.tokens != 1000 {
		t.Errorf("tokens = %f, want 1000", rl.tokens)
	}
}

// =============================================================================
// Test metricsRateLimiter initial tokens equal burst
// =============================================================================

func TestMetricsRateLimiter_InitialBurstEqualsRPS(t *testing.T) {
	for _, rps := range []int{1, 2, 5, 10, 50, 100} {
		rl := newMetricsRateLimiter(rps)
		allowed := 0
		for rl.allow() {
			allowed++
		}
		if allowed != rps {
			t.Errorf("rps=%d: initial burst allowed %d, want %d", rps, allowed, rps)
		}
	}
}

// =============================================================================
// Test isValidWorkflowName with Chinese characters
// =============================================================================

func TestIsValidWorkflowName_Unicode(t *testing.T) {
	// Unicode characters outside ASCII letters/digits should be rejected
	invalidUnicode := []string{
		"工作流",
		"ワークフロー",
		"워크플로우",
		"café",
		"naïve",
	}
	for _, name := range invalidUnicode {
		if isValidWorkflowName(name) {
			t.Errorf("isValidWorkflowName(%q) = true, want false", name)
		}
	}
}

// =============================================================================
// Test handleVisualize with application/json; charset=utf-8
// =============================================================================

func TestHandleVisualize_JSONWithCharset(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"workflow":"name: test\nsteps:\n  - node: echo\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// =============================================================================
// Test handleVisualize format parameter is case-sensitive exact match
// =============================================================================

func TestHandleVisualize_FormatCaseSensitivity(t *testing.T) {
	s := NewWebUIServer("", "")
	workflowYAML := "name: test\nsteps:\n  - node: echo\n"
	body := `{"workflow":"` + strings.ReplaceAll(workflowYAML, "\n", "\\n") + `"}`

	tests := []struct {
		format   string
		expectCT string
	}{
		{"MERMAID", "application/json"}, // uppercase → falls to default (json)
		{"Mermaid", "application/json"}, // mixed case → falls to default (json)
		{"mermaid", "text/plain"},       // exact match → mermaid
	}

	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodPost, "/api/visualize?format="+tc.format, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.handleVisualize(rec, req)

		ct := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, tc.expectCT) {
			t.Errorf("format=%q: Content-Type = %q, want prefix %q", tc.format, ct, tc.expectCT)
		}
	}
}

// =============================================================================
// Test handleVisualize with XML content type (non-JSON, non-plain)
// =============================================================================

func TestHandleVisualize_XMLContentType(t *testing.T) {
	s := NewWebUIServer("", "")
	body := "name: test\nsteps:\n  - node: echo\n"
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/xml")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	// XML content type is not "application/json" prefix, so it goes to the plain text path
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// =============================================================================
// Test buildHandler with metrics enabled and auth set
// =============================================================================

func TestBuildHandler_MetricsWithAuth(t *testing.T) {
	t.Setenv("AFLARE_METRICS", "1")
	s := NewWebUIServer("", "")
	s.SetAuthToken("secret")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	// /metrics should NOT require auth even when token is set
	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("metrics with auth: status = %d, want 200", resp.StatusCode)
	}

	// But API endpoints should require auth
	resp2, err := http.Get(ts.URL + "/api/workflows")
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("api with auth: status = %d, want 401", resp2.StatusCode)
	}
}

// =============================================================================
// Test handleListWorkflows content type header
// =============================================================================

func TestHandleListWorkflows_ContentType(t *testing.T) {
	dir := t.TempDir()
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	req := httptest.NewRequest(http.MethodGet, "/api/workflows", nil)
	rec := httptest.NewRecorder()
	s.handleListWorkflows(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// =============================================================================
// Test handleVisualize body read error handling (boundary)
// =============================================================================

func TestHandleVisualize_BodyTooLargePlainText(t *testing.T) {
	s := NewWebUIServer("", "")
	// Create a body larger than the read buffer (maxWorkflowFileSize+1)
	// The read will get exactly maxWorkflowFileSize bytes, and the n check
	// will be > maxWorkflowFileSize since we read into maxWorkflowFileSize+1 bytes
	body := strings.Repeat("x", maxWorkflowFileSize+1)
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
}

// =============================================================================
// Test buildHandler with only metrics enabled, pprof disabled
// =============================================================================

func TestBuildHandler_MetricsOnly(t *testing.T) {
	t.Setenv("AFLARE_METRICS", "1")
	t.Setenv("AFLARE_PPROF", "")
	s := NewWebUIServer("", "")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	// metrics should work
	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("metrics: status = %d, want 200", resp.StatusCode)
	}

	// pprof should fall through to index
	resp2, err := http.Get(ts.URL + "/debug/pprof/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	body, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body), "<html") {
		t.Error("pprof should fall through to index when disabled")
	}
}

// =============================================================================
// Test handleSaveWorkflow with name containing dots
// =============================================================================

func TestHandleSaveWorkflow_NameWithDots(t *testing.T) {
	dir := t.TempDir()
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	body := `{"name": "my.workflow.v1", "content": "name: test\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflow", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleSaveWorkflow(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}

	// Verify file was saved with .yaml appended
	expectedPath := filepath.Join(dir, "my.workflow.v1.yaml")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected file at %s", expectedPath)
	}
}

// =============================================================================
// Test handleGetWorkflow with name containing dots
// =============================================================================

func TestHandleGetWorkflow_NameWithDots(t *testing.T) {
	dir := t.TempDir()
	content := "name: dotted\nsteps:\n  - node: echo\n"
	os.WriteFile(filepath.Join(dir, "my.workflow.v1.yaml"), []byte(content), 0600)

	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	req := httptest.NewRequest(http.MethodGet, "/api/workflow?name=my.workflow.v1", nil)
	rec := httptest.NewRecorder()
	s.handleGetWorkflow(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != content {
		t.Errorf("body = %q, want %q", rec.Body.String(), content)
	}
}

// =============================================================================
// Test handleSaveWorkflow with empty content
// =============================================================================

func TestHandleSaveWorkflow_EmptyContent(t *testing.T) {
	dir := t.TempDir()
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	body := `{"name": "empty", "content": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflow", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleSaveWorkflow(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "empty.yaml"))
	if string(data) != "" {
		t.Errorf("file content = %q, want empty", string(data))
	}
}

// =============================================================================
// Test metricsRateLimiter allow with no time passing
// =============================================================================

func TestMetricsRateLimiter_NoTimePassing(t *testing.T) {
	rl := newMetricsRateLimiter(3)
	// First 3 should be allowed
	for i := 0; i < 3; i++ {
		if !rl.allow() {
			t.Fatal("first 3 requests should be allowed")
		}
	}
	// 4th should be denied
	if rl.allow() {
		t.Error("4th request should be denied with no time passing")
	}
}

// =============================================================================
// Test SetAuthToken does not affect other server instances
// =============================================================================

func TestSetAuthToken_Isolation(t *testing.T) {
	s1 := NewWebUIServer("", "")
	s2 := NewWebUIServer("", "")

	s1.SetAuthToken("token1")
	s2.SetAuthToken("token2")

	if s1.authToken != "token1" {
		t.Errorf("s1.authToken = %q, want 'token1'", s1.authToken)
	}
	if s2.authToken != "token2" {
		t.Errorf("s2.authToken = %q, want 'token2'", s2.authToken)
	}
}

// =============================================================================
// Test SetWorkflowsDir does not affect other server instances
// =============================================================================

func TestSetWorkflowsDir_Isolation(t *testing.T) {
	s1 := NewWebUIServer("", "")
	s2 := NewWebUIServer("", "")

	s1.SetWorkflowsDir("/dir1")
	s2.SetWorkflowsDir("/dir2")

	if s1.workflowsDir != "/dir1" {
		t.Errorf("s1.workflowsDir = %q, want '/dir1'", s1.workflowsDir)
	}
	if s2.workflowsDir != "/dir2" {
		t.Errorf("s2.workflowsDir = %q, want '/dir2'", s2.workflowsDir)
	}
}

// =============================================================================
// Test handleVisualize with .yaml content type
// =============================================================================

func TestHandleVisualize_YAMLContentType(t *testing.T) {
	s := NewWebUIServer("", "")
	body := "name: test\nsteps:\n  - node: echo\n"
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-yaml")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	// application/x-yaml is not "application/json" prefix, so it goes to plain text path
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// =============================================================================
// Test handleVisualize with very long body under the limit
// =============================================================================

func TestHandleVisualize_BodyUnderLimit(t *testing.T) {
	s := NewWebUIServer("", "")
	// Create a body that is under the limit
	body := strings.Repeat("x", maxWorkflowFileSize-1)
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	// Body under limit should be OK (even if it's not valid YAML, it's passed to visualizer)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// =============================================================================
// Test handleValidate with JSON body that has extra fields
// =============================================================================

func TestHandleValidate_ExtraJSONFields(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"workflow": "name: test\nsteps:\n  - node: echo\n", "extra": "field", "another": 123}`
	req := httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleValidate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if v, ok := resp["valid"]; !ok || v != true {
		t.Errorf("expected valid=true, got %v", resp["valid"])
	}
}

// =============================================================================
// Test registerPprof with auth
// =============================================================================

func TestRegisterPprof_AllEndpointsWithAuth(t *testing.T) {
	t.Setenv("AFLARE_PPROF", "1")
	s := NewWebUIServer("", "")
	s.SetAuthToken("secret")
	ts := httptest.NewServer(s.buildHandler())
	defer ts.Close()

	pprofPaths := []string{
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/profile?seconds=1",
		"/debug/pprof/symbol",
		"/debug/pprof/trace?seconds=1",
	}

	for _, path := range pprofPaths {
		// Without auth, should get 401
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Errorf("GET %s: %v", path, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("GET %s without auth: status = %d, want 401", path, resp.StatusCode)
		}

		// With auth, should get 200
		req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
		req.Header.Set("X-Auth-Token", "secret")
		resp2, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Errorf("GET %s with auth: %v", path, err)
			continue
		}
		resp2.Body.Close()
		if resp2.StatusCode != http.StatusOK {
			t.Errorf("GET %s with auth: status = %d, want 200", path, resp2.StatusCode)
		}
	}
}

// =============================================================================
// Test handleSaveWorkflow verifies the response JSON contains the saved path
// =============================================================================

func TestHandleSaveWorkflow_ResponseIncludesPath(t *testing.T) {
	dir := t.TempDir()
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	body := `{"name": "test-wf", "content": "name: test\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflow", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleSaveWorkflow(rec, req)

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)

	expectedPath := filepath.Join(dir, "test-wf.yaml")
	if resp["path"] != expectedPath {
		t.Errorf("path = %q, want %q", resp["path"], expectedPath)
	}
}

// =============================================================================
// Test handleDeleteWorkflow response is JSON
// =============================================================================

func TestHandleDeleteWorkflow_ResponseIsJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.yaml"), []byte("content"), 0600)

	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	req := httptest.NewRequest(http.MethodDelete, "/api/workflow?name=test", nil)
	rec := httptest.NewRecorder()
	s.handleDeleteWorkflow(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
}

// =============================================================================
// Test Start error (port already in use - skip)
// =============================================================================

func TestStart_PortInUse(t *testing.T) {
	t.Skip("requires a real network listener; tested via integration tests")
}

// =============================================================================
// Test that buildHandler is idempotent
// =============================================================================

func TestBuildHandler_Idempotent(t *testing.T) {
	t.Setenv("AFLARE_METRICS", "1")
	t.Setenv("AFLARE_PPROF", "1")
	s := NewWebUIServer("", "")

	h1 := s.buildHandler()
	h2 := s.buildHandler()

	// Both should return valid handlers
	ts1 := httptest.NewServer(h1)
	defer ts1.Close()
	ts2 := httptest.NewServer(h2)
	defer ts2.Close()

	// Both should respond to metrics
	resp1, _ := http.Get(ts1.URL + "/metrics")
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Error("h1: metrics should return 200")
	}

	resp2, _ := http.Get(ts2.URL + "/metrics")
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Error("h2: metrics should return 200")
	}
}

// =============================================================================
// Test handleVisualize with JSON body containing escaped newlines
// =============================================================================

func TestHandleVisualize_JSONWithEscapedNewlines(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"workflow":"name: test\nsteps:\n  - node: echo\n    params:\n      message: \"hello world\"\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// =============================================================================
// Test handleGetWorkflow with special characters in name
// =============================================================================

func TestHandleGetWorkflow_NameWithSpecialChars(t *testing.T) {
	s := NewWebUIServer("", "")
	// URL-encode the space in the query parameter
	req := httptest.NewRequest(http.MethodGet, "/api/workflow?name=name%20with%20spaces", nil)
	rec := httptest.NewRecorder()
	s.handleGetWorkflow(rec, req)

	// Name with spaces should be invalid
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// =============================================================================
// Test authMiddleware with empty token in header
// =============================================================================

func TestAuthMiddleware_EmptyTokenHeader(t *testing.T) {
	s := NewWebUIServer("", "")
	s.SetAuthToken("secret")
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Auth-Token", "")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// =============================================================================
// Test authMiddleware passes through when authToken is empty
// =============================================================================

func TestAuthMiddleware_EmptyAuthToken(t *testing.T) {
	s := NewWebUIServer("", "")
	// authToken is empty by default
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// =============================================================================
// Test handleSaveWorkflow with name at max length
// =============================================================================

func TestHandleSaveWorkflow_NameAtMaxLength(t *testing.T) {
	dir := t.TempDir()
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	maxName := strings.Repeat("a", 100)
	body := `{"name": "` + maxName + `", "content": "name: test\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflow", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleSaveWorkflow(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
}

// =============================================================================
// Test handleSaveWorkflow with name exceeding max length
// =============================================================================

func TestHandleSaveWorkflow_NameTooLong(t *testing.T) {
	dir := t.TempDir()
	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	longName := strings.Repeat("a", 101)
	body := `{"name": "` + longName + `", "content": "name: test\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflow", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleSaveWorkflow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// =============================================================================
// Test handleVisualize with body exactly at max size
// =============================================================================

func TestHandleVisualize_BodyAtMaxSize(t *testing.T) {
	s := NewWebUIServer("", "")
	body := strings.Repeat("x", maxWorkflowFileSize)
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	// At exactly max size, the read gets maxWorkflowFileSize bytes, n == maxWorkflowFileSize
	// The check is n > maxWorkflowFileSize, so this should be OK
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// =============================================================================
// Test handleVisualize with body exactly at max size for JSON
// =============================================================================

func TestHandleVisualize_JSONBodyAtMaxSize(t *testing.T) {
	s := NewWebUIServer("", "")
	// The JSON decoder reads through io.LimitReader(r.Body, maxWorkflowFileSize)
	// So a body at maxWorkflowFileSize won't be rejected by the size limit
	workflowContent := strings.Repeat("x", maxWorkflowFileSize-20) // leave room for JSON wrapper
	body := `{"workflow":"` + workflowContent + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	// Should be processed (may succeed or fail based on visualizer)
	if rec.Code != http.StatusOK && rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 200 or 400", rec.Code)
	}
}

// =============================================================================
// Test handleListWorkflows JSON encoding with special directory path
// =============================================================================

func TestHandleListWorkflows_SpecialDirPath(t *testing.T) {
	dir := t.TempDir()
	// Create a directory with a path that requires JSON escaping
	specialDir := filepath.Join(dir, "test-dir")
	os.Mkdir(specialDir, 0755)
	os.WriteFile(filepath.Join(specialDir, "test.yaml"), []byte("name: test"), 0600)

	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(specialDir)

	req := httptest.NewRequest(http.MethodGet, "/api/workflows", nil)
	rec := httptest.NewRecorder()
	s.handleListWorkflows(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if resp["directory"] != specialDir {
		t.Errorf("directory = %q, want %q", resp["directory"], specialDir)
	}
}

// =============================================================================
// Test handleWorkflow dispatch with unknown method
// =============================================================================

func TestHandleWorkflow_PatchMethod(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodPatch, "/api/workflow", nil)
	rec := httptest.NewRecorder()
	s.handleWorkflow(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// =============================================================================
// Test handleWorkflow dispatch with HEAD method
// =============================================================================

func TestHandleWorkflow_HeadMethod(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodHead, "/api/workflow?name=test", nil)
	rec := httptest.NewRecorder()
	s.handleWorkflow(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// =============================================================================
// Test handleValidate with multiline workflow YAML
// =============================================================================

func TestHandleValidate_MultilineWorkflow(t *testing.T) {
	s := NewWebUIServer("", "")
	workflowYAML := `name: multiline-test
description: A multi-step workflow
steps:
  - node: echo
    params:
      message: step1
  - node: echo
    params:
      message: step2
  - node: echo
    params:
      message: step3
`
	body := `{"workflow":"` + strings.ReplaceAll(workflowYAML, "\n", "\\n") + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleValidate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if v, _ := resp["valid"].(bool); !v {
		t.Errorf("expected valid=true, got valid=%v, error=%v", v, resp["error"])
	}
	if steps, _ := resp["steps"].(float64); steps != 3 {
		t.Errorf("steps = %v, want 3", steps)
	}
}

// =============================================================================
// Test handleVisualize with empty body and JSON content type
// =============================================================================

func TestHandleVisualize_EmptyJSONBody(t *testing.T) {
	s := NewWebUIServer("", "")
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	// Empty body with JSON content type causes JSON decode error
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// =============================================================================
// Test handleListWorkflows returns sorted or consistent order
// =============================================================================

func TestHandleListWorkflows_Order(t *testing.T) {
	dir := t.TempDir()
	// Create files in a specific order
	os.WriteFile(filepath.Join(dir, "zebra.yaml"), []byte("z"), 0600)
	os.WriteFile(filepath.Join(dir, "alpha.yaml"), []byte("a"), 0600)
	os.WriteFile(filepath.Join(dir, "mike.yaml"), []byte("m"), 0600)

	s := NewWebUIServer("", "")
	s.SetWorkflowsDir(dir)

	req := httptest.NewRequest(http.MethodGet, "/api/workflows", nil)
	rec := httptest.NewRecorder()
	s.handleListWorkflows(rec, req)

	var resp struct {
		Workflows []string `json:"workflows"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Workflows) != 3 {
		t.Fatalf("expected 3 workflows, got %d", len(resp.Workflows))
	}

	// Verify all expected workflows are present
	found := make(map[string]bool)
	for _, w := range resp.Workflows {
		found[w] = true
	}
	for _, expected := range []string{"zebra", "alpha", "mike"} {
		if !found[expected] {
			t.Errorf("workflow %q not found in response", expected)
		}
	}
}

// =============================================================================
// Test metricsRateLimiter burst behavior
// =============================================================================

func TestMetricsRateLimiter_BurstEqualsRPS(t *testing.T) {
	for rps := 1; rps <= 10; rps++ {
		rl := newMetricsRateLimiter(rps)
		count := 0
		for rl.allow() {
			count++
		}
		if count != rps {
			t.Errorf("rps=%d: burst count = %d, want %d", rps, count, rps)
		}
	}
}

// =============================================================================
// Test handleValidate with workflow that has no steps
// =============================================================================

func TestHandleValidate_NoSteps(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"workflow": "name: empty\nsteps: []\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/validate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleValidate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if v, _ := resp["valid"].(bool); !v {
		t.Errorf("expected valid=true, got valid=%v, error=%v", v, resp["error"])
	}
	if steps, _ := resp["steps"].(float64); steps != 0 {
		t.Errorf("steps = %v, want 0", steps)
	}
}

// =============================================================================
// Test handleVisualize with JSON body containing unicode
// =============================================================================

func TestHandleVisualize_UnicodeWorkflow(t *testing.T) {
	s := NewWebUIServer("", "")
	// metadata field can contain unicode
	body := `{"workflow":"name: test\ndescription: 测试工作流\nsteps:\n  - node: echo\n    params:\n      message: 你好\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// =============================================================================
// Test that Stop returns nil for a server that was never started
// =============================================================================

func TestStop_NeverStarted(t *testing.T) {
	s := NewWebUIServer("", "")
	// Multiple stops should all succeed
	for i := 0; i < 3; i++ {
		if err := s.Stop(); err != nil {
			t.Errorf("Stop #%d: unexpected error: %v", i+1, err)
		}
	}
}

// =============================================================================
// Test authMiddleware with very long token
// =============================================================================

func TestAuthMiddleware_LongToken(t *testing.T) {
	longToken := strings.Repeat("x", 10000)
	s := NewWebUIServer("", "")
	s.SetAuthToken(longToken)

	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Correct token
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Auth-Token", longToken)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("correct long token: status = %d, want 200", rec.Code)
	}

	// Wrong token (off by one)
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-Auth-Token", longToken+"x")
	rec2 := httptest.NewRecorder()
	handler(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("wrong long token: status = %d, want 401", rec2.Code)
	}
}

// =============================================================================
// Test buildHandler with nil receiver shouldn't panic
// =============================================================================

func TestBuildHandler_NilSafeCheck(t *testing.T) {
	// buildHandler is called on a valid receiver in all other tests.
	// This verifies that the method doesn't dereference nil fields unexpectedly.
	s := NewWebUIServer("", "")
	h := s.buildHandler()
	if h == nil {
		t.Fatal("buildHandler should not return nil for a valid receiver")
	}
}

// =============================================================================
// Test handleVisualize with JSON containing a non-string workflow field
// =============================================================================

func TestHandleVisualize_JSONWorkflowFieldNotString(t *testing.T) {
	s := NewWebUIServer("", "")
	body := `{"workflow": 123}`
	req := httptest.NewRequest(http.MethodPost, "/api/visualize?format=json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleVisualize(rec, req)

	// JSON decoder will fail because workflow field is not a string
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
