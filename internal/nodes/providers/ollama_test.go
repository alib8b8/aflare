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

package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

func TestOllamaNode(t *testing.T) {
	// Test that ollama node is registered
	node, ok := core.Get("ollama")
	if !ok {
		t.Fatal("ollama node not found in registry")
	}
	if node.Name() != "ollama" {
		t.Errorf("expected node name 'ollama', got '%s'", node.Name())
	}
}

func TestOllamaExecute(t *testing.T) {
	// Create a mock Ollama server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("expected path /api/generate, got %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Decode request
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Verify request
		if req.Model != "llama3" {
			t.Errorf("expected model 'llama3', got '%s'", req.Model)
		}
		if req.Prompt != "Hello, test!" {
			t.Errorf("expected prompt 'Hello, test!', got '%s'", req.Prompt)
		}
		if req.Stream != false {
			t.Errorf("expected stream false, got %v", req.Stream)
		}

		// Send mock response
		resp := ollamaResponse{
			Response: "Hello, this is a mock Ollama response!",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Get ollama node
	node, ok := core.Get("ollama")
	if !ok {
		t.Fatal("ollama node not found")
	}

	// Execute with mock endpoint
	ctx := context.Background()
	input := "Hello, test!"
	params := map[string]string{
		"model":    "llama3",
		"endpoint": mockServer.URL,
	}

	output, err := node.Execute(ctx, input, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expected := "Hello, this is a mock Ollama response!"
	if output != expected {
		t.Errorf("expected output '%s', got '%s'", expected, output)
	}
}

func TestOllamaDefaultParams(t *testing.T) {
	// Create a mock Ollama server that checks for default parameters
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Model != "llama3" {
			t.Errorf("expected default model 'llama3', got '%s'", req.Model)
		}

		resp := ollamaResponse{Response: "OK"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	node, ok := core.Get("ollama")
	if !ok {
		t.Fatal("ollama node not found")
	}

	ctx := context.Background()
	input := "Hello!"
	params := map[string]string{
		"endpoint": mockServer.URL,
	}

	_, err := node.Execute(ctx, input, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

func TestOllamaPromptParamPriority(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if req.Prompt != "Prompt from params" {
			t.Errorf("expected prompt from params 'Prompt from params', got '%s'", req.Prompt)
		}

		resp := ollamaResponse{Response: "OK"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	node, ok := core.Get("ollama")
	if !ok {
		t.Fatal("ollama node not found")
	}

	ctx := context.Background()
	input := "Input value"
	params := map[string]string{
		"endpoint": mockServer.URL,
		"prompt":   "Prompt from params",
	}

	_, err := node.Execute(ctx, input, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

func TestOllamaExecuteStream(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !req.Stream {
			t.Error("ExecuteStream must send stream=true")
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Write([]byte("{\"response\":\"Hel\",\"done\":false}\n"))
		w.Write([]byte("{\"response\":\"lo!\",\"done\":false}\n"))
		w.Write([]byte("not-json\n")) // skipped
		w.Write([]byte("{\"response\":\"\",\"done\":true}\n"))
	}))
	defer mockServer.Close()

	node := &OllamaNode{}

	var chunks []string
	out, err := node.ExecuteStream(context.Background(), "hi", map[string]string{
		"endpoint": mockServer.URL,
	}, func(chunk string) { chunks = append(chunks, chunk) })
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if out != "Hello!" {
		t.Errorf("streamed output = %q, want %q", out, "Hello!")
	}
	if len(chunks) != 2 || chunks[0] != "Hel" || chunks[1] != "lo!" {
		t.Errorf("chunks = %v, want [Hel lo!]", chunks)
	}
}

func TestOllamaExecuteStream_NilOnChunk(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{\"response\":\"ok\",\"done\":true}\n"))
	}))
	defer mockServer.Close()

	node := &OllamaNode{}

	out, err := node.ExecuteStream(context.Background(), "hi", map[string]string{
		"endpoint": mockServer.URL,
	}, nil)
	if err != nil {
		t.Fatalf("ExecuteStream with nil onChunk failed: %v", err)
	}
	if out != "ok" {
		t.Errorf("streamed output = %q, want ok", out)
	}
}

func TestOllamaExecute_ModelNotFound(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"model 'llama3' not found, try pulling it first"}`))
	}))
	defer mockServer.Close()

	node, _ := core.Get("ollama")
	_, err := node.Execute(context.Background(), "hi", map[string]string{
		"endpoint": mockServer.URL,
	})
	if err == nil {
		t.Fatal("model-not-found should return an error")
	}
	if !strings.Contains(err.Error(), "ollama pull") {
		t.Errorf("error should suggest 'ollama pull', got: %v", err)
	}
}

func TestOllamaExecute_APIError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid request"}`))
	}))
	defer mockServer.Close()

	node, _ := core.Get("ollama")
	_, err := node.Execute(context.Background(), "hi", map[string]string{
		"endpoint": mockServer.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid request") {
		t.Fatalf("expected API error surfaced, got %v", err)
	}
}

func TestOllamaExecute_StatusErrorNoBody(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	node, _ := core.Get("ollama")
	_, err := node.Execute(context.Background(), "hi", map[string]string{
		"endpoint": mockServer.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected status 500 error, got %v", err)
	}
}

func TestOllamaExecute_InvalidEndpoint(t *testing.T) {
	node, _ := core.Get("ollama")
	for _, endpoint := range []string{
		"ftp://example.com",            // scheme
		"http://user:pass@localhost:9", // userinfo
		"http://192.168.1.1:9",         // private IP
	} {
		_, err := node.Execute(context.Background(), "hi", map[string]string{"endpoint": endpoint})
		if err == nil {
			t.Errorf("endpoint %q should be rejected", endpoint)
		}
	}
}

func TestOllamaExecute_BadJSONResponse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer mockServer.Close()

	node, _ := core.Get("ollama")
	_, err := node.Execute(context.Background(), "hi", map[string]string{
		"endpoint": mockServer.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestOllamaExecute_UnreachableEndpoint(t *testing.T) {
	node, _ := core.Get("ollama")
	_, err := node.Execute(context.Background(), "hi", map[string]string{
		"endpoint": "http://localhost:1", // nothing listens on port 1
	})
	if err == nil || !strings.Contains(err.Error(), "ollama not running") {
		t.Fatalf("expected connection error with hint, got %v", err)
	}
}
