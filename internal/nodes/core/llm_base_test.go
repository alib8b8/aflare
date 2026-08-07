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

package core

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Type JSON roundtrips ---

func TestLLMMessage_JSONRoundtrip(t *testing.T) {
	msg := LLMMessage{Role: "user", Content: "hello"}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	want := `{"role":"user","content":"hello"}`
	if string(data) != want {
		t.Errorf("Marshal = %s, want %s", data, want)
	}
	var got LLMMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if got != msg {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", got, msg)
	}
}

func TestLLMRequest_JSONRoundtrip(t *testing.T) {
	req := LLMRequest{
		Model:       "gpt-test",
		Messages:    []LLMMessage{{Role: "system", Content: "sys"}, {Role: "user", Content: "hi"}},
		Temperature: 0.5,
		MaxTokens:   100,
		Stream:      false,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var got LLMRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if got.Model != req.Model || len(got.Messages) != 2 || got.Stream != false {
		t.Errorf("roundtrip mismatch: got %+v", got)
	}
	// Stream:false is omitted via ,omitempty on Temperature/MaxTokens, but
	// Stream has no omitempty so it is always present.
	if !strings.Contains(string(data), `"stream":false`) {
		t.Errorf("expected stream:false in JSON: %s", data)
	}
}

func TestLLMResponse_JSONRoundtrip(t *testing.T) {
	// Response with choices.
	raw := `{"choices":[{"message":{"content":"hi"},"delta":{"content":""}}]}`
	var resp LLMResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "hi" {
		t.Errorf("content = %q, want hi", resp.Choices[0].Message.Content)
	}

	// Response with error.
	rawErr := `{"choices":[],"error":{"message":"bad request"}}`
	var respErr LLMResponse
	if err := json.Unmarshal([]byte(rawErr), &respErr); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if respErr.Error == nil || respErr.Error.Message != "bad request" {
		t.Errorf("error = %+v", respErr.Error)
	}
}

func TestLLMNodeConfig_Fields(t *testing.T) {
	cfg := LLMNodeConfig{
		Name:            "testnode",
		DefaultModel:    "test-model",
		DefaultEndpoint: "https://example.com/v1",
		EnvAPIKey:       "TESTNODE_API_KEY",
		ProviderName:    "TestProvider",
	}
	if cfg.Name != "testnode" || cfg.DefaultModel != "test-model" || cfg.EnvAPIKey != "TESTNODE_API_KEY" {
		t.Errorf("LLMNodeConfig fields = %+v", cfg)
	}
}

// --- OpenAICompatibleNode construction + accessors ---

func TestNewOpenAICompatibleNode_Accessors(t *testing.T) {
	cfg := LLMNodeConfig{
		Name:            "testnode",
		DefaultModel:    "test-model",
		DefaultEndpoint: "https://example.com/v1",
		EnvAPIKey:       "TESTNODE_API_KEY",
		ProviderName:    "TestProvider",
	}
	n := NewOpenAICompatibleNode(cfg)
	if n.Name() != "testnode" {
		t.Errorf("Name() = %q, want testnode", n.Name())
	}
	if want := "Call TestProvider LLM API"; n.Description() != want {
		t.Errorf("Description() = %q, want %q", n.Description(), want)
	}
}

func TestOpenAICompatibleNode_Schema(t *testing.T) {
	cfg := LLMNodeConfig{
		Name:            "testnode",
		DefaultModel:    "test-model",
		DefaultEndpoint: "https://example.com/v1",
		EnvAPIKey:       "TESTNODE_API_KEY",
		ProviderName:    "TestProvider",
	}
	n := NewOpenAICompatibleNode(cfg)
	schema := n.Schema()
	if schema.Name != "testnode" {
		t.Errorf("Schema.Name = %q", schema.Name)
	}
	if schema.Input != "string - user message content" {
		t.Errorf("Schema.Input = %q", schema.Input)
	}
	if schema.Output != "string - AI response content" {
		t.Errorf("Schema.Output = %q", schema.Output)
	}
	// Expect params: model, api_key, endpoint, system
	paramNames := map[string]bool{}
	for _, p := range schema.Params {
		paramNames[p.Name] = true
	}
	for _, want := range []string{"model", "api_key", "endpoint", "system"} {
		if !paramNames[want] {
			t.Errorf("expected param %q in schema", want)
		}
	}
}

// --- Execute error paths ---

func TestOpenAICompatibleNode_Execute_NoAPIKey(t *testing.T) {
	// Use a unique env var name that is guaranteed unset, and ensure it is
	// empty so config.GetAPIKey falls through to an unknown provider.
	t.Setenv("LLMBOX_TEST_UNIQUE_API_KEY_NEVER_SET", "")
	cfg := LLMNodeConfig{
		Name:            "testnode",
		DefaultModel:    "test-model",
		DefaultEndpoint: "http://localhost:11434",
		EnvAPIKey:       "LLMBOX_TEST_UNIQUE_API_KEY_NEVER_SET",
		ProviderName:    "TestProvider",
	}
	n := NewOpenAICompatibleNode(cfg)
	_, err := n.Execute(context.Background(), "hi", map[string]string{})
	if err == nil {
		t.Fatal("expected error when no API key is provided, got nil")
	}
	if !strings.Contains(err.Error(), "API key required") {
		t.Errorf("error should mention API key required, got: %v", err)
	}
}

func TestOpenAICompatibleNode_Execute_InvalidEndpoint(t *testing.T) {
	cfg := LLMNodeConfig{
		Name:            "testnode",
		DefaultModel:    "test-model",
		DefaultEndpoint: "http://localhost:11434",
		EnvAPIKey:       "LLMBOX_TEST_UNIQUE_API_KEY_NEVER_SET",
		ProviderName:    "TestProvider",
	}
	n := NewOpenAICompatibleNode(cfg)
	// api_key is set so we get past the key check, but endpoint is a private
	// non-loopback address which ValidateLMLEndpoint rejects.
	_, err := n.Execute(context.Background(), "hi", map[string]string{
		"api_key":  "sk-test",
		"endpoint": "http://192.168.1.1:11434",
	})
	if err == nil {
		t.Fatal("expected error for invalid endpoint, got nil")
	}
	if !strings.Contains(err.Error(), "endpoint URL validation failed") {
		t.Errorf("error should mention endpoint validation, got: %v", err)
	}
}

// --- Execute success via httptest ---

func newMockLLMServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

func TestOpenAICompatibleNode_Execute_Success(t *testing.T) {
	var gotBody LLMRequest
	srv := newMockLLMServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected path /chat/completions, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("Authorization = %q, want Bearer sk-test", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		resp := LLMResponse{
			Choices: []LLMChoice{{Message: LLMChoiceMessage{Content: "hello from mock"}}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()

	cfg := LLMNodeConfig{
		Name:            "testnode",
		DefaultModel:    "test-model",
		DefaultEndpoint: "http://localhost:11434",
		EnvAPIKey:       "LLMBOX_TEST_UNIQUE_API_KEY_NEVER_SET",
		ProviderName:    "TestProvider",
	}
	n := NewOpenAICompatibleNode(cfg)
	out, err := n.Execute(context.Background(), "user message", map[string]string{
		"api_key":  "sk-test",
		"endpoint": srv.URL,
		"system":   "you are helpful",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if out != "hello from mock" {
		t.Errorf("output = %q, want hello from mock", out)
	}
	// Verify request body contents.
	if gotBody.Model != "test-model" {
		t.Errorf("request model = %q, want test-model", gotBody.Model)
	}
	if len(gotBody.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(gotBody.Messages))
	}
	if gotBody.Messages[0].Role != "system" || gotBody.Messages[0].Content != "you are helpful" {
		t.Errorf("system message = %+v", gotBody.Messages[0])
	}
	if gotBody.Messages[1].Role != "user" || gotBody.Messages[1].Content != "user message" {
		t.Errorf("user message = %+v", gotBody.Messages[1])
	}
	if gotBody.Stream != false {
		t.Errorf("expected stream=false, got true")
	}
}

func TestOpenAICompatibleNode_Execute_DefaultModelAndEndpoint(t *testing.T) {
	// When model/endpoint params are absent, defaults from config should apply.
	srv := newMockLLMServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body LLMRequest
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		if body.Model != "test-model" {
			t.Errorf("expected default model test-model, got %q", body.Model)
		}
		resp := LLMResponse{Choices: []LLMChoice{{Message: LLMChoiceMessage{Content: "ok"}}}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()

	cfg := LLMNodeConfig{
		Name:            "testnode",
		DefaultModel:    "test-model",
		DefaultEndpoint: srv.URL, // default endpoint points to the mock
		EnvAPIKey:       "LLMBOX_TEST_UNIQUE_API_KEY_NEVER_SET",
		ProviderName:    "TestProvider",
	}
	n := NewOpenAICompatibleNode(cfg)
	// Pass api_key but not model/endpoint to exercise defaults.
	out, err := n.Execute(context.Background(), "hi", map[string]string{
		"api_key": "sk-test",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if out != "ok" {
		t.Errorf("output = %q, want ok", out)
	}
}

// --- Execute error responses ---

func TestOpenAICompatibleNode_Execute_APIError(t *testing.T) {
	srv := newMockLLMServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	})
	defer srv.Close()

	n := NewOpenAICompatibleNode(LLMNodeConfig{
		Name: "testnode", DefaultModel: "m", DefaultEndpoint: srv.URL,
		EnvAPIKey: "LLMBOX_TEST_UNIQUE_API_KEY_NEVER_SET", ProviderName: "TestProvider",
	})
	_, err := n.Execute(context.Background(), "hi", map[string]string{
		"api_key":  "sk-test",
		"endpoint": srv.URL,
	})
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("error should include status and message, got: %v", err)
	}
}

func TestOpenAICompatibleNode_Execute_APIErrorNoMessage(t *testing.T) {
	srv := newMockLLMServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{}`))
	})
	defer srv.Close()

	n := NewOpenAICompatibleNode(LLMNodeConfig{
		Name: "testnode", DefaultModel: "m", DefaultEndpoint: srv.URL,
		EnvAPIKey: "LLMBOX_TEST_UNIQUE_API_KEY_NEVER_SET", ProviderName: "TestProvider",
	})
	_, err := n.Execute(context.Background(), "hi", map[string]string{
		"api_key":  "sk-test",
		"endpoint": srv.URL,
	})
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should include status 500, got: %v", err)
	}
}

func TestOpenAICompatibleNode_Execute_NoChoices(t *testing.T) {
	srv := newMockLLMServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(LLMResponse{Choices: nil})
	})
	defer srv.Close()

	n := NewOpenAICompatibleNode(LLMNodeConfig{
		Name: "testnode", DefaultModel: "m", DefaultEndpoint: srv.URL,
		EnvAPIKey: "LLMBOX_TEST_UNIQUE_API_KEY_NEVER_SET", ProviderName: "TestProvider",
	})
	_, err := n.Execute(context.Background(), "hi", map[string]string{
		"api_key":  "sk-test",
		"endpoint": srv.URL,
	})
	if err == nil {
		t.Fatal("expected error for no choices, got nil")
	}
	if !strings.Contains(err.Error(), "no choices") {
		t.Errorf("error should mention no choices, got: %v", err)
	}
}

func TestOpenAICompatibleNode_Execute_InvalidJSON(t *testing.T) {
	srv := newMockLLMServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	})
	defer srv.Close()

	n := NewOpenAICompatibleNode(LLMNodeConfig{
		Name: "testnode", DefaultModel: "m", DefaultEndpoint: srv.URL,
		EnvAPIKey: "LLMBOX_TEST_UNIQUE_API_KEY_NEVER_SET", ProviderName: "TestProvider",
	})
	_, err := n.Execute(context.Background(), "hi", map[string]string{
		"api_key":  "sk-test",
		"endpoint": srv.URL,
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("error should mention parse, got: %v", err)
	}
}

// --- ExecuteStream via httptest ---

func TestOpenAICompatibleNode_ExecuteStream_Success(t *testing.T) {
	srv := newMockLLMServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body LLMRequest
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		if !body.Stream {
			t.Errorf("expected stream=true in request, got false")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		// First chunk
		chunk1 := LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{Content: "Hello"}}}}
		_, _ = w.Write([]byte("data: "))
		enc := json.NewEncoder(w)
		_ = enc.Encode(chunk1)
		if flusher != nil {
			flusher.Flush()
		}
		// Second chunk
		chunk2 := LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{Content: " world"}}}}
		_, _ = w.Write([]byte("data: "))
		_ = enc.Encode(chunk2)
		if flusher != nil {
			flusher.Flush()
		}
		// Done marker
		_, _ = w.Write([]byte("data: [DONE]\n"))
	})
	defer srv.Close()

	n := NewOpenAICompatibleNode(LLMNodeConfig{
		Name: "testnode", DefaultModel: "m", DefaultEndpoint: srv.URL,
		EnvAPIKey: "LLMBOX_TEST_UNIQUE_API_KEY_NEVER_SET", ProviderName: "TestProvider",
	})

	var chunks []string
	out, err := n.ExecuteStream(context.Background(), "hi", map[string]string{
		"api_key":  "sk-test",
		"endpoint": srv.URL,
	}, func(chunk string) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if out != "Hello world" {
		t.Errorf("full output = %q, want Hello world", out)
	}
	if len(chunks) != 2 || chunks[0] != "Hello" || chunks[1] != " world" {
		t.Errorf("chunks = %v, want [Hello  world]", chunks)
	}
}

func TestOpenAICompatibleNode_ExecuteStream_Empty(t *testing.T) {
	// Server immediately sends [DONE] without any content chunks.
	srv := newMockLLMServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n"))
	})
	defer srv.Close()

	n := NewOpenAICompatibleNode(LLMNodeConfig{
		Name: "testnode", DefaultModel: "m", DefaultEndpoint: srv.URL,
		EnvAPIKey: "LLMBOX_TEST_UNIQUE_API_KEY_NEVER_SET", ProviderName: "TestProvider",
	})
	out, err := n.ExecuteStream(context.Background(), "hi", map[string]string{
		"api_key":  "sk-test",
		"endpoint": srv.URL,
	}, func(chunk string) {})
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output, got %q", out)
	}
}
