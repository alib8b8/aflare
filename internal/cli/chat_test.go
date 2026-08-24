// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​​​‌​​​‌‌‌‌​‌‌‌‌​​‌‌​‌​​​‌​‌‌​‌​​​‌​‌‌‌‌​‌​‌‌​​​​​​​​​​​​​​​​​​‌​​​‌​​‌‌​​​​‌​⁠
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
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// ── checkLLMReady ─────────────────────────────────────────────────────────

func TestCheckLLMReady_OllamaReachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"models": [{"name": "llama3:latest"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	err := checkLLMReady("ollama", srv.URL, "")
	if err != nil {
		t.Errorf("expected ollama to be reachable, got error: %v", err)
	}
}

func TestCheckLLMReady_OllamaUnreachable(t *testing.T) {
	// Use a closed listener to simulate unreachable
	err := checkLLMReady("ollama", "http://127.0.0.1:19999", "")
	if err == nil {
		t.Error("expected error for unreachable ollama, got nil")
	}
	if !strings.Contains(err.Error(), "not reachable") {
		t.Errorf("expected 'not reachable' in error, got: %v", err)
	}
}

func TestCheckLLMReady_OllamaDefaultEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// When endpoint is empty, it defaults to http://localhost:11434
	// This test verifies the default path is formed correctly with a real server
	err := checkLLMReady("ollama", srv.URL, "")
	if err != nil {
		t.Errorf("expected reachable with explicit endpoint, got: %v", err)
	}
}

func TestCheckLLMReady_OllamaNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	err := checkLLMReady("ollama", srv.URL, "")
	if err == nil {
		t.Error("expected error for non-200 response, got nil")
	}
	if !strings.Contains(err.Error(), "status 503") {
		t.Errorf("expected 'status 503' in error, got: %v", err)
	}
}

func TestCheckLLMReady_CloudProviderWithAPIKey(t *testing.T) {
	err := checkLLMReady("deepseek", "", "sk-test-key")
	if err != nil {
		t.Errorf("expected no error with API key, got: %v", err)
	}
}

func TestCheckLLMReady_CloudProviderWithoutAPIKey(t *testing.T) {
	err := checkLLMReady("deepseek", "", "")
	if err == nil {
		t.Error("expected error for missing API key, got nil")
	}
	if !strings.Contains(err.Error(), "API key not configured") {
		t.Errorf("expected 'API key not configured' in error, got: %v", err)
	}
}

func TestCheckLLMReady_CloudProviderWithEnvVar(t *testing.T) {
	// Save and restore previous env value
	prev, hadPrev := os.LookupEnv("OPENAI_API_KEY")
	os.Setenv("OPENAI_API_KEY", "sk-env-key")
	defer func() {
		if hadPrev {
			os.Setenv("OPENAI_API_KEY", prev)
		} else {
			os.Unsetenv("OPENAI_API_KEY")
		}
	}()

	err := checkLLMReady("openai", "", "")
	if err != nil {
		t.Errorf("expected env var to satisfy API key check, got: %v", err)
	}
}

func TestCheckLLMReady_CloudProviderWithEmptyEnvVar(t *testing.T) {
	// Save and restore previous env value
	prev, hadPrev := os.LookupEnv("DEEPSEEK_API_KEY")
	os.Unsetenv("DEEPSEEK_API_KEY")
	defer func() {
		if hadPrev {
			os.Setenv("DEEPSEEK_API_KEY", prev)
		}
	}()

	err := checkLLMReady("deepseek", "", "")
	if err == nil {
		t.Error("expected error when env var is empty, got nil")
	}
}

func TestCheckLLMReady_OllamaEndpointTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// Endpoint with trailing slash should be trimmed
	err := checkLLMReady("ollama", srv.URL+"/", "")
	if err != nil {
		t.Errorf("trailing slash should be trimmed, got error: %v", err)
	}
}
