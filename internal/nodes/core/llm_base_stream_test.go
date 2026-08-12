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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── CallWithToolsStream tests ─────────────────────────────────────────────

func newSSEMockServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(handler))
}

func sseWrite(w http.ResponseWriter, v interface{}) {
	data, _ := json.Marshal(v)
	w.Write([]byte("data: "))
	w.Write(data)
	w.Write([]byte("\n"))
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func newTestNode(srvURL string) *OpenAICompatibleNode {
	return NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "testnode",
		DefaultModel:    "test-model",
		DefaultEndpoint: srvURL,
		EnvAPIKey:       "LLMBOX_TEST_UNIQUE_API_KEY_NEVER_SET",
		ProviderName:    "TestProvider",
	})
}

func TestCallWithToolsStream_ContentStreaming(t *testing.T) {
	srv := newSSEMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{Content: "Hello"}}}})
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{Content: " world"}}}})
		w.Write([]byte("data: [DONE]\n"))
	})
	defer srv.Close()

	n := newTestNode(srv.URL)
	var chunks []string
	resp, err := n.CallWithToolsStream(context.Background(), nil, "test-model", "sk-test", srv.URL, nil, nil, func(chunk string) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("CallWithToolsStream failed: %v", err)
	}
	if len(chunks) != 2 || chunks[0] != "Hello" || chunks[1] != " world" {
		t.Errorf("chunks = %v, want [Hello  world]", chunks)
	}
	content := resp.Choices[0].Message.Content
	if content != "Hello world" {
		t.Errorf("full content = %q, want 'Hello world'", content)
	}
}

func TestCallWithToolsStream_ToolCallAccumulation(t *testing.T) {
	// Normal case: first chunk carries ID + Name + partial arguments
	srv := newSSEMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// First chunk: ID + Name + partial args
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{
			ToolCalls: []LLMToolCall{
				{Index: 0, ID: "call_1", Type: "function", Function: LLMToolCallFunc{Name: "search", Arguments: `{"query"`}},
			},
		}}}})
		// Second chunk: more arguments
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{
			ToolCalls: []LLMToolCall{
				{Index: 0, Function: LLMToolCallFunc{Arguments: `:"weather"}`}},
			},
		}}}})
		w.Write([]byte("data: [DONE]\n"))
	})
	defer srv.Close()

	n := newTestNode(srv.URL)
	resp, err := n.CallWithToolsStream(context.Background(), nil, "test-model", "sk-test", srv.URL, nil, nil, nil)
	if err != nil {
		t.Fatalf("CallWithToolsStream failed: %v", err)
	}
	if len(resp.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Choices[0].Message.ToolCalls))
	}
	tc := resp.Choices[0].Message.ToolCalls[0]
	if tc.ID != "call_1" {
		t.Errorf("ID = %q, want 'call_1'", tc.ID)
	}
	if tc.Function.Name != "search" {
		t.Errorf("Name = %q, want 'search'", tc.Function.Name)
	}
	if tc.Function.Arguments != `{"query":"weather"}` {
		t.Errorf("Arguments = %q, want '{\"query\":\"weather\"}'", tc.Function.Arguments)
	}
}

func TestCallWithToolsStream_SplitIDAndName(t *testing.T) {
	// Regression test: provider sends empty function object first,
	// then sends ID/Name in separate chunks.
	srv := newSSEMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Chunk 1: empty function, establishes the tool call at index 0
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{
			ToolCalls: []LLMToolCall{
				{Index: 0, Function: LLMToolCallFunc{}},
			},
		}}}})
		// Chunk 2: ID and Name arrive later
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{
			ToolCalls: []LLMToolCall{
				{Index: 0, ID: "call_abc", Function: LLMToolCallFunc{Name: "get_weather"}},
			},
		}}}})
		// Chunk 3: arguments arrive
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{
			ToolCalls: []LLMToolCall{
				{Index: 0, Function: LLMToolCallFunc{Arguments: `{"city":"NYC"}`}},
			},
		}}}})
		w.Write([]byte("data: [DONE]\n"))
	})
	defer srv.Close()

	n := newTestNode(srv.URL)
	resp, err := n.CallWithToolsStream(context.Background(), nil, "test-model", "sk-test", srv.URL, nil, nil, nil)
	if err != nil {
		t.Fatalf("CallWithToolsStream failed: %v", err)
	}
	if len(resp.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Choices[0].Message.ToolCalls))
	}
	tc := resp.Choices[0].Message.ToolCalls[0]
	if tc.ID != "call_abc" {
		t.Errorf("ID should be updated from later chunk, got %q", tc.ID)
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("Name should be updated from later chunk, got %q", tc.Function.Name)
	}
	if tc.Function.Arguments != `{"city":"NYC"}` {
		t.Errorf("Arguments = %q", tc.Function.Arguments)
	}
}

func TestCallWithToolsStream_MultipleToolCalls(t *testing.T) {
	// Multiple tool calls with out-of-order indices
	srv := newSSEMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Tool call at index 2 arrives first
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{
			ToolCalls: []LLMToolCall{
				{Index: 2, ID: "c2", Function: LLMToolCallFunc{Name: "third", Arguments: `{}`}},
			},
		}}}})
		// Tool call at index 0 arrives second
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{
			ToolCalls: []LLMToolCall{
				{Index: 0, ID: "c0", Function: LLMToolCallFunc{Name: "first", Arguments: `{}`}},
			},
		}}}})
		// Tool call at index 1 arrives last
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{
			ToolCalls: []LLMToolCall{
				{Index: 1, ID: "c1", Function: LLMToolCallFunc{Name: "second", Arguments: `{}`}},
			},
		}}}})
		w.Write([]byte("data: [DONE]\n"))
	})
	defer srv.Close()

	n := newTestNode(srv.URL)
	resp, err := n.CallWithToolsStream(context.Background(), nil, "test-model", "sk-test", srv.URL, nil, nil, nil)
	if err != nil {
		t.Fatalf("CallWithToolsStream failed: %v", err)
	}
	if len(resp.Choices[0].Message.ToolCalls) != 3 {
		t.Fatalf("expected 3 tool calls, got %d", len(resp.Choices[0].Message.ToolCalls))
	}
	// Should be sorted by index: 0, 1, 2
	if resp.Choices[0].Message.ToolCalls[0].Function.Name != "first" {
		t.Errorf("index 0 = %q, want 'first'", resp.Choices[0].Message.ToolCalls[0].Function.Name)
	}
	if resp.Choices[0].Message.ToolCalls[1].Function.Name != "second" {
		t.Errorf("index 1 = %q, want 'second'", resp.Choices[0].Message.ToolCalls[1].Function.Name)
	}
	if resp.Choices[0].Message.ToolCalls[2].Function.Name != "third" {
		t.Errorf("index 2 = %q, want 'third'", resp.Choices[0].Message.ToolCalls[2].Function.Name)
	}
}

func TestCallWithToolsStream_NoToolCalls(t *testing.T) {
	srv := newSSEMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{Content: "Just text"}}}})
		w.Write([]byte("data: [DONE]\n"))
	})
	defer srv.Close()

	n := newTestNode(srv.URL)
	resp, err := n.CallWithToolsStream(context.Background(), nil, "test-model", "sk-test", srv.URL, nil, nil, nil)
	if err != nil {
		t.Fatalf("CallWithToolsStream failed: %v", err)
	}
	if len(resp.Choices[0].Message.ToolCalls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(resp.Choices[0].Message.ToolCalls))
	}
	if resp.Choices[0].Message.Content != "Just text" {
		t.Errorf("content = %q, want 'Just text'", resp.Choices[0].Message.Content)
	}
}

func TestCallWithToolsStream_IgnoresNonDataLines(t *testing.T) {
	srv := newSSEMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Non-data lines should be ignored
		w.Write([]byte(": comment line\n"))
		w.Write([]byte("\n")) // empty line
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{Content: "Real content"}}}})
		w.Write([]byte("data: [DONE]\n"))
	})
	defer srv.Close()

	n := newTestNode(srv.URL)
	resp, err := n.CallWithToolsStream(context.Background(), nil, "test-model", "sk-test", srv.URL, nil, nil, nil)
	if err != nil {
		t.Fatalf("CallWithToolsStream failed: %v", err)
	}
	if resp.Choices[0].Message.Content != "Real content" {
		t.Errorf("content = %q, want 'Real content'", resp.Choices[0].Message.Content)
	}
}

func TestCallWithToolsStream_ContentAndToolCalls(t *testing.T) {
	// Content and tool calls interleaved in the same stream
	srv := newSSEMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{Content: "Let me search"}}}})
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{
			ToolCalls: []LLMToolCall{
				{Index: 0, ID: "tc1", Function: LLMToolCallFunc{Name: "search", Arguments: `{"q":"test"}`}},
			},
		}}}})
		w.Write([]byte("data: [DONE]\n"))
	})
	defer srv.Close()

	n := newTestNode(srv.URL)
	resp, err := n.CallWithToolsStream(context.Background(), nil, "test-model", "sk-test", srv.URL, nil, nil, nil)
	if err != nil {
		t.Fatalf("CallWithToolsStream failed: %v", err)
	}
	if resp.Choices[0].Message.Content != "Let me search" {
		t.Errorf("content = %q, want 'Let me search'", resp.Choices[0].Message.Content)
	}
	if len(resp.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Choices[0].Message.ToolCalls))
	}
	if resp.Choices[0].Message.ToolCalls[0].Function.Name != "search" {
		t.Errorf("tool call name = %q, want 'search'", resp.Choices[0].Message.ToolCalls[0].Function.Name)
	}
}

func TestCallWithToolsStream_OnChunkCalled(t *testing.T) {
	srv := newSSEMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{Content: "Chunk A"}}}})
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{Content: "Chunk B"}}}})
		w.Write([]byte("data: [DONE]\n"))
	})
	defer srv.Close()

	n := newTestNode(srv.URL)
	var chunks []string
	_, err := n.CallWithToolsStream(context.Background(), nil, "test-model", "sk-test", srv.URL, nil, nil, func(chunk string) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("CallWithToolsStream failed: %v", err)
	}
	if len(chunks) != 2 || chunks[0] != "Chunk A" || chunks[1] != "Chunk B" {
		t.Errorf("chunks = %v, want [Chunk A Chunk B]", chunks)
	}
}

func TestCallWithToolsStream_EmptyStream(t *testing.T) {
	srv := newSSEMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: [DONE]\n"))
	})
	defer srv.Close()

	n := newTestNode(srv.URL)
	resp, err := n.CallWithToolsStream(context.Background(), nil, "test-model", "sk-test", srv.URL, nil, nil, nil)
	if err != nil {
		t.Fatalf("CallWithToolsStream failed: %v", err)
	}
	if resp.Choices[0].Message.Content != "" {
		t.Errorf("expected empty content, got %q", resp.Choices[0].Message.Content)
	}
	if len(resp.Choices[0].Message.ToolCalls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(resp.Choices[0].Message.ToolCalls))
	}
}

func TestCallWithToolsStream_ToolCallArgumentOverMultipleChunks(t *testing.T) {
	// Arguments arrive across 3 chunks
	srv := newSSEMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{
			ToolCalls: []LLMToolCall{
				{Index: 0, ID: "a", Function: LLMToolCallFunc{Name: "run", Arguments: `{"cmd"`}},
			},
		}}}})
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{
			ToolCalls: []LLMToolCall{
				{Index: 0, Function: LLMToolCallFunc{Arguments: `:"ls`}},
			},
		}}}})
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{
			ToolCalls: []LLMToolCall{
				{Index: 0, Function: LLMToolCallFunc{Arguments: ` -la"}`}},
			},
		}}}})
		w.Write([]byte("data: [DONE]\n"))
	})
	defer srv.Close()

	n := newTestNode(srv.URL)
	resp, err := n.CallWithToolsStream(context.Background(), nil, "test-model", "sk-test", srv.URL, nil, nil, nil)
	if err != nil {
		t.Fatalf("CallWithToolsStream failed: %v", err)
	}
	tc := resp.Choices[0].Message.ToolCalls[0]
	if tc.Function.Arguments != `{"cmd":"ls -la"}` {
		t.Errorf("accumulated arguments = %q", tc.Function.Arguments)
	}
}

func TestCallWithToolsStream_InvalidJSON(t *testing.T) {
	// Invalid JSON in a data line should be silently skipped
	srv := newSSEMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {not valid json}\n"))
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{Content: "Valid"}}}})
		w.Write([]byte("data: [DONE]\n"))
	})
	defer srv.Close()

	n := newTestNode(srv.URL)
	resp, err := n.CallWithToolsStream(context.Background(), nil, "test-model", "sk-test", srv.URL, nil, nil, nil)
	if err != nil {
		t.Fatalf("CallWithToolsStream failed: %v", err)
	}
	if !strings.Contains(resp.Choices[0].Message.Content, "Valid") {
		t.Errorf("expected 'Valid' in content, got %q", resp.Choices[0].Message.Content)
	}
}

func TestCallWithToolsStream_NoAPIKey(t *testing.T) {
	n := newTestNode("http://localhost:1")
	_, err := n.CallWithToolsStream(context.Background(), nil, "test-model", "", "http://localhost:1", nil, nil, nil)
	if err == nil {
		t.Error("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "API key required") {
		t.Errorf("error = %v, want 'API key required'", err)
	}
}

func TestCallWithToolsStream_NoChoicesInChunk(t *testing.T) {
	srv := newSSEMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Chunk with no choices
		sseWrite(w, LLMResponse{Choices: nil})
		sseWrite(w, LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{Content: "After empty"}}}})
		w.Write([]byte("data: [DONE]\n"))
	})
	defer srv.Close()

	n := newTestNode(srv.URL)
	resp, err := n.CallWithToolsStream(context.Background(), nil, "test-model", "sk-test", srv.URL, nil, nil, nil)
	if err != nil {
		t.Fatalf("CallWithToolsStream failed: %v", err)
	}
	if resp.Choices[0].Message.Content != "After empty" {
		t.Errorf("content = %q, want 'After empty'", resp.Choices[0].Message.Content)
	}
}

func TestCallWithToolsStream_HTTPError(t *testing.T) {
	srv := newSSEMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"message":"internal error"}}`))
	})
	defer srv.Close()

	n := newTestNode(srv.URL)
	_, err := n.CallWithToolsStream(context.Background(), nil, "test-model", "sk-test", srv.URL, nil, nil, nil)
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "internal error") {
		t.Errorf("error = %v, want 'internal error'", err)
	}
}