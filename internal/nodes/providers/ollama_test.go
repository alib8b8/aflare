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
