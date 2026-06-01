package nodes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaNode(t *testing.T) {
	// Test that ollama node is registered
	node, ok := Get("ollama")
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
	node, ok := Get("ollama")
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

	node, ok := Get("ollama")
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
