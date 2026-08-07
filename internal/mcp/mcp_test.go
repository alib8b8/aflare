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

package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alib8b8/llm-box/internal/history"
	"github.com/alib8b8/llm-box/internal/workflow"
	"gopkg.in/yaml.v3"
)

// ------------------------------------------------------------------
// Tool tests
// ------------------------------------------------------------------

func TestToolWorkflowRun_MissingFile(t *testing.T) {
	s := NewServer()
	_, err := s.toolWorkflowRun(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "file") {
		t.Errorf("expected file parameter error, got: %v", err)
	}
}

func TestToolWorkflowRun_InvalidFile(t *testing.T) {
	s := NewServer()
	_, err := s.toolWorkflowRun(map[string]interface{}{"file": "/nonexistent/path.yaml"})
	if err == nil {
		t.Fatal("expected error for invalid file")
	}
}

func TestToolWorkflowCreate_MissingDescription(t *testing.T) {
	s := NewServer()
	_, err := s.toolWorkflowCreate(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing description")
	}
}

func TestToolWorkflowCreate_WithName(t *testing.T) {
	s := NewServer()
	result, err := s.toolWorkflowCreate(map[string]interface{}{
		"description": "a simple test workflow",
		"name":        "my-test-workflow",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "my-test-workflow") {
		t.Errorf("expected workflow name in output, got: %s", result.Content[0].Text)
	}
}

func TestToolWorkflowList_EmptyDir(t *testing.T) {
	s := NewServer()
	tmpDir := t.TempDir()
	result, err := s.toolWorkflowList(map[string]interface{}{"directory": tmpDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "No workflow files found") {
		t.Errorf("expected empty dir message, got: %s", result.Content[0].Text)
	}
}

func TestToolWorkflowList_WithFiles(t *testing.T) {
	s := NewServer()
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "a.yaml"), []byte("name: a"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "b.yml"), []byte("name: b"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "c.txt"), []byte("not a workflow"), 0644)

	result, err := s.toolWorkflowList(map[string]interface{}{"directory": tmpDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "a.yaml") || !strings.Contains(text, "b.yml") {
		t.Errorf("expected yaml files in output, got: %s", text)
	}
	if strings.Contains(text, "c.txt") {
		t.Errorf("unexpected txt file in output: %s", text)
	}
}

func TestToolWorkflowValidate_MissingParams(t *testing.T) {
	s := NewServer()
	_, err := s.toolWorkflowValidate(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error when both file and yaml are missing")
	}
}

func TestToolWorkflowValidate_WithYAML(t *testing.T) {
	s := NewServer()
	wf := &workflow.Workflow{
		Name: "test",
		Steps: []workflow.WorkflowStep{
			{Node: "test", Params: map[string]string{"message": "hello"}},
		},
	}
	data, _ := yaml.Marshal(wf)
	result, err := s.toolWorkflowValidate(map[string]interface{}{"yaml": string(data)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestToolNodeList(t *testing.T) {
	s := NewServer()
	result, err := s.toolNodeList()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Available nodes") {
		t.Errorf("expected header in output, got: %s", text)
	}
}

func TestToolNodeInfo_MissingName(t *testing.T) {
	s := NewServer()
	_, err := s.toolNodeInfo(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestToolNodeInfo_UnknownNode(t *testing.T) {
	s := NewServer()
	_, err := s.toolNodeInfo(map[string]interface{}{"name": "nonexistent_node_xyz"})
	if err == nil {
		t.Fatal("expected error for unknown node")
	}
}

func TestToolNodeInfo_ValidNode(t *testing.T) {
	s := NewServer()
	result, err := s.toolNodeInfo(map[string]interface{}{"name": "file_read"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "file_read") {
		t.Errorf("expected node name in output, got: %s", text)
	}
}

func TestToolHistoryList_Empty(t *testing.T) {
	s := NewServer()
	tmpDir := t.TempDir()
	history.SetHistoryDir(tmpDir)
	defer history.SetHistoryDir("")

	result, err := s.toolHistoryList(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "No history records found") {
		t.Errorf("expected empty history message, got: %s", result.Content[0].Text)
	}
}

func TestToolHistoryList_WithRecords(t *testing.T) {
	s := NewServer()
	tmpDir := t.TempDir()
	history.SetHistoryDir(tmpDir)
	defer history.SetHistoryDir("")

	rec := history.Record{
		ID:        "test-record-1",
		Name:      "test-workflow",
		Trigger:   history.TriggerCLI,
		Success:   true,
		StartedAt: time.Now().Add(-time.Hour),
		Duration:  5 * time.Second,
	}
	if err := history.SaveRecord(rec); err != nil {
		t.Fatalf("failed to save record: %v", err)
	}

	result, err := s.toolHistoryList(map[string]interface{}{"limit": 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "test-workflow") {
		t.Errorf("expected workflow name in output, got: %s", text)
	}
}

func TestToolTemplateList(t *testing.T) {
	s := NewServer()
	result, err := s.toolTemplateList(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "simple-llm") {
		t.Errorf("expected simple-llm template in output, got: %s", text)
	}
}

func TestToolTemplateList_ByCategory(t *testing.T) {
	s := NewServer()
	result, err := s.toolTemplateList(map[string]interface{}{"category": "llm"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "simple-llm") && !strings.Contains(text, "translation") {
		t.Errorf("expected llm templates in output, got: %s", text)
	}
}

func TestToolTemplateRender_MissingName(t *testing.T) {
	s := NewServer()
	_, err := s.toolTemplateRender(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestToolTemplateRender_Valid(t *testing.T) {
	s := NewServer()
	result, err := s.toolTemplateRender(map[string]interface{}{
		"name": "simple-llm",
		"vars": map[string]interface{}{
			"workflow_name": "my-llm",
			"prompt":        "Say hello",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "my-llm") {
		t.Errorf("expected rendered workflow name, got: %s", text)
	}
}

func TestSanitizeError(t *testing.T) {
	// Should redact sensitive keywords
	err := sanitizeError(fmt.Errorf("invalid api key: secret123"))
	if !strings.Contains(err.Error(), "redacted") {
		t.Errorf("expected redacted error, got: %v", err)
	}

	// Should keep normal errors
	err2 := sanitizeError(fmt.Errorf("file not found"))
	if err2.Error() != "file not found" {
		t.Errorf("expected unchanged error, got: %v", err2)
	}
}

func TestRequireString(t *testing.T) {
	_, err := requireString(map[string]interface{}{}, "foo")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	_, err = requireString(map[string]interface{}{"foo": ""}, "foo")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
	v, err := requireString(map[string]interface{}{"foo": "bar"}, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "bar" {
		t.Errorf("expected bar, got %s", v)
	}
}

func TestOptionalInt(t *testing.T) {
	if optionalInt(map[string]interface{}{"n": 42}, "n", 0) != 42 {
		t.Error("expected 42")
	}
	if optionalInt(map[string]interface{}{"n": 3.14}, "n", 0) != 3 {
		t.Error("expected 3")
	}
	if optionalInt(map[string]interface{}{"n": "99"}, "n", 0) != 99 {
		t.Error("expected 99")
	}
	if optionalInt(map[string]interface{}{}, "n", 7) != 7 {
		t.Error("expected default 7")
	}
}

func TestOptionalBool(t *testing.T) {
	if !optionalBool(map[string]interface{}{"b": true}, "b", false) {
		t.Error("expected true")
	}
	if optionalBool(map[string]interface{}{"b": "true"}, "b", false) != true {
		t.Error("expected true from string")
	}
	if optionalBool(map[string]interface{}{"b": "false"}, "b", true) != false {
		t.Error("expected false from string")
	}
	if optionalBool(map[string]interface{}{}, "b", true) != true {
		t.Error("expected default true")
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Error("expected no truncation")
	}
	if truncate("hello world", 8) != "hello..." {
		t.Errorf("expected truncation, got %s", truncate("hello world", 8))
	}
}

// ------------------------------------------------------------------
// Client tests
// ------------------------------------------------------------------

func TestClient_Connect_StreamableHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var resp rpcResponse
		resp.JSONRPC = "2.0"
		resp.ID = req.ID

		switch req.Method {
		case "initialize":
			resp.Result = initializeResult{
				Capabilities: serverCapabilities{Tools: toolsCapability{ListChanged: false}},
				ServerInfo:   serverInfo{Name: "test-server", Version: "1.0.0"},
			}
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
			return
		case "tools/list":
			resp.Result = toolsListResult{Tools: []tool{{Name: "test_tool", Description: "A test tool", InputSchema: inputSchema{Type: "object", Properties: map[string]interface{}{}}}}}
		case "tools/call":
			resp.Result = toolCallResult{Content: []content{{Type: "text", Text: "ok"}}}
		default:
			resp.Error = &rpcError{Code: -32601, Message: "Method not found"}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := Connect(server.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	if !client.isInitialized {
		t.Error("expected client to be initialized")
	}

	tools, err := client.ListTools()
	if err != nil {
		t.Fatalf("failed to list tools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "test_tool" {
		t.Errorf("unexpected tools: %+v", tools)
	}

	result, err := client.CallTool("test_tool", map[string]interface{}{"arg": "value"})
	if err != nil {
		t.Fatalf("failed to call tool: %v", err)
	}
	if len(result.Content) == 0 || result.Content[0].Text != "ok" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestClient_CallTool_Validation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
		if req.Method == "initialize" {
			resp.Result = initializeResult{
				Capabilities: serverCapabilities{Tools: toolsCapability{ListChanged: false}},
				ServerInfo:   serverInfo{Name: "test", Version: "1.0.0"},
			}
		} else if req.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		} else if req.Method == "tools/call" {
			resp.Result = toolCallResult{Content: []content{{Type: "text", Text: "done"}}}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := Connect(server.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	_, err = client.CallTool("", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for empty tool name")
	}
}

func TestClient_CallTool_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
		if req.Method == "initialize" {
			resp.Result = initializeResult{
				Capabilities: serverCapabilities{Tools: toolsCapability{ListChanged: false}},
				ServerInfo:   serverInfo{Name: "test", Version: "1.0.0"},
			}
		} else if req.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		} else if req.Method == "tools/call" {
			resp.Error = &rpcError{Code: -32603, Message: "internal error containing secret token"}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := Connect(server.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	_, err = client.CallTool("some_tool", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "redacted") {
		t.Errorf("expected sanitized error, got: %v", err)
	}
}

func TestClient_SanitizeErrorString(t *testing.T) {
	if sanitizeErrorString("normal error") != "normal error" {
		t.Error("expected normal error to be unchanged")
	}
	if sanitizeErrorString("invalid password") != "tool execution failed (sensitive details redacted)" {
		t.Error("expected sensitive error to be redacted")
	}
}

func TestResolveURL(t *testing.T) {
	cases := []struct {
		base string
		ref  string
		want string
	}{
		{"http://localhost:8080/sse", "/mcp", "http://localhost:8080/mcp"},
		{"http://localhost:8080", "path", "http://localhost:8080/path"},
		{"http://localhost:8080/", "/path", "http://localhost:8080/path"},
	}
	for _, c := range cases {
		got := resolveURL(c.base, c.ref)
		if got != c.want {
			t.Errorf("resolveURL(%q, %q) = %q, want %q", c.base, c.ref, got, c.want)
		}
	}
}

func TestCallExtendedTool_Timeout(t *testing.T) {
	s := NewServer()
	// workflow_run with a very short timeout should fail quickly
	_, err := s.callExtendedTool(&toolCallParams{
		Name:      "workflow_run",
		Arguments: map[string]interface{}{"file": "/nonexistent.yaml", "timeout_seconds": 1},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolWorkflowRun_TimeoutOverride(t *testing.T) {
	s := NewServer()
	// Invalid file but timeout should be clamped
	_, err := s.toolWorkflowRun(map[string]interface{}{"file": "/nonexistent.yaml", "timeout_seconds": 999})
	if err == nil {
		t.Fatal("expected error")
	}
	// Ensure timeout clamping doesn't panic
	_, _ = s.toolWorkflowRun(map[string]interface{}{"file": "/nonexistent.yaml", "timeout_seconds": -5})
}

func TestToolHistoryList_LimitClamp(t *testing.T) {
	s := NewServer()
	tmpDir := t.TempDir()
	history.SetHistoryDir(tmpDir)
	defer history.SetHistoryDir("")

	// limit should be clamped to valid range
	_, err := s.toolHistoryList(map[string]interface{}{"limit": -5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = s.toolHistoryList(map[string]interface{}{"limit": 999})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ------------------------------------------------------------------
// SSE Client tests
// ------------------------------------------------------------------

func TestClient_Connect_SSE_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := Connect(server.URL + "/sse")
	if err == nil {
		t.Fatal("expected error for SSE connection failure")
	}
}

func TestClient_Connect_SSE_NoEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: hello\n\n")
	}))
	defer server.Close()

	_, err := Connect(server.URL + "/sse")
	if err == nil {
		t.Fatal("expected error for missing endpoint event")
	}
}

func TestClient_Connect_Failure(t *testing.T) {
	_, err := Connect("http://nonexistent-server-12345.invalid/mcp")
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
}

func TestClient_HandleSSEEvent(t *testing.T) {
	c := &Client{
		pending: make(map[int]chan *rpcResponse),
	}

	id := 123
	ch := make(chan *rpcResponse, 1)
	c.pending[id] = ch

	respData := `{"jsonrpc":"2.0","id":123,"result":"test"}`
	c.handleSSEEvent(respData)

	select {
	case resp := <-ch:
		if resp == nil {
			t.Error("expected non-nil response")
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("timeout waiting for response")
	}
}

func TestClient_HandleSSEEvent_InvalidJSON(t *testing.T) {
	c := &Client{
		pending: make(map[int]chan *rpcResponse),
	}

	c.handleSSEEvent("invalid json")
}

func TestClient_HandleSSEEvent_NoPending(t *testing.T) {
	c := &Client{
		pending: make(map[int]chan *rpcResponse),
	}

	respData := `{"jsonrpc":"2.0","id":999,"result":"test"}`
	c.handleSSEEvent(respData)
}

func TestClient_HandleSSEEvent_InvalidID(t *testing.T) {
	c := &Client{
		pending: make(map[int]chan *rpcResponse),
	}

	respData := `{"jsonrpc":"2.0","id":"invalid","result":"test"}`
	c.handleSSEEvent(respData)
}

func TestClient_SSEReaderLoop_Stop(t *testing.T) {
	c := &Client{
		sseDone: make(chan struct{}),
		pending: make(map[int]chan *rpcResponse),
	}

	c.sseWg.Add(1)
	close(c.sseDone)

	done := make(chan struct{})
	go func() {
		defer close(done)
		c.sseReaderLoop()
	}()

	select {
	case <-done:
	case <-time.After(time.Millisecond * 100):
		t.Error("timeout waiting for sseReaderLoop to stop")
	}
}

func TestClient_PostSSE_Failure(t *testing.T) {
	c := &Client{
		httpClient:  &http.Client{Timeout: 1 * time.Second},
		sseEndpoint: "http://nonexistent-server.invalid/mcp",
	}

	req := &rpcRequest{JSONRPC: "2.0", Method: "test"}
	err := c.postSSE(req)
	if err == nil {
		t.Fatal("expected error for SSE post failure")
	}
}

func TestClient_Close_WithSSE(t *testing.T) {
	c := &Client{
		sseDone:   make(chan struct{}),
		sseReader: io.NopCloser(strings.NewReader("")),
	}

	c.sseWg.Add(1)
	go func() {
		defer c.sseWg.Done()
		time.Sleep(time.Millisecond * 50)
	}()

	err := c.Close()
	if err != nil {
		t.Errorf("unexpected close error: %v", err)
	}
}

func TestClient_Close_NilSSEReader(t *testing.T) {
	c := &Client{}
	err := c.Close()
	if err != nil {
		t.Errorf("unexpected close error: %v", err)
	}
}

func TestClient_ResolveURL_BaseNoSlash(t *testing.T) {
	result := resolveURL("http://example.com", "/path")
	if result != "http://example.com/path" {
		t.Errorf("expected http://example.com/path, got %s", result)
	}
}

func TestClient_ResolveURL_Relative(t *testing.T) {
	result := resolveURL("http://example.com/base", "relative")
	if result != "http://example.com/base/relative" {
		t.Errorf("expected http://example.com/base/relative, got %s", result)
	}
}

func TestClient_ResolveURL_NoProtocol(t *testing.T) {
	result := resolveURL("example.com", "/path")
	if result != "/path" {
		t.Errorf("expected /path, got %s", result)
	}
}

func TestClient_Close_WithoutSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
		if req.Method == "initialize" {
			resp.Result = initializeResult{
				Capabilities: serverCapabilities{Tools: toolsCapability{ListChanged: false}},
				ServerInfo:   serverInfo{Name: "test", Version: "1.0.0"},
			}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := Connect(server.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Errorf("unexpected close error: %v", err)
	}
}

func TestClient_SendRequest_Timeout(t *testing.T) {
	// This test exercises the request-timeout path. The server stalls the
	// tools/list response for 3s while the client uses a 1s per-request
	// timeout, so the call fails fast instead of waiting on the default 30s
	// client timeout. Skip under -short to keep slow CI from running it.
	if testing.Short() {
		t.Skip("requires timeout behavior")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method == "initialize" {
			resp := rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: initializeResult{
				Capabilities: serverCapabilities{Tools: toolsCapability{ListChanged: false}},
				ServerInfo:   serverInfo{Name: "test", Version: "1.0.0"},
			}}
			_ = json.NewEncoder(w).Encode(resp)
		} else if req.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
		} else {
			// Stall longer than the client's 1s timeout (below) but short
			// enough that the deferred server.Close() doesn't hang the test.
			time.Sleep(3 * time.Second)
		}
	}))
	defer server.Close()

	client, err := Connect(server.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	// Override the shared 30s client with a 1s-timeout client that keeps the
	// SSRF-protecting Transport. initialize already succeeded, so only the
	// tools/list request is bound by this short timeout.
	client.httpClient = &http.Client{
		Timeout:   1 * time.Second,
		Transport: client.httpClient.Transport,
	}

	_, err = client.ListTools()
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

// ------------------------------------------------------------------
// Server handleRequest tests for all tools
// ------------------------------------------------------------------

func TestHandleRequest_ToolsCall_WorkflowRun(t *testing.T) {
	s := NewServer()
	tmpDir := t.TempDir()
	wfPath := filepath.Join(tmpDir, "test.yaml")
	wf := &workflow.Workflow{
		Name: "test",
		Steps: []workflow.WorkflowStep{
			{Node: "test", Params: map[string]string{"message": "hello"}},
		},
	}
	data, _ := yaml.Marshal(wf)
	if err := os.WriteFile(wfPath, data, 0644); err != nil {
		t.Fatalf("failed to write workflow: %v", err)
	}

	params, _ := json.Marshal(map[string]interface{}{"name": "workflow_run", "arguments": map[string]interface{}{"file": wfPath}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestHandleRequest_ToolsCall_WorkflowCreate(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "workflow_create", "arguments": map[string]interface{}{"description": "test workflow"}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}

	done := make(chan *rpcResponse)
	go func() {
		done <- s.handleRequest(req)
	}()
	select {
	case resp := <-done:
		if resp == nil {
			t.Fatal("expected non-nil response")
		}
	case <-time.After(time.Second * 10):
		t.Fatal("timeout")
	}
}

func TestHandleRequest_ToolsCall_WorkflowList(t *testing.T) {
	s := NewServer()
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte("name: test"), 0644)

	params, _ := json.Marshal(map[string]interface{}{"name": "workflow_list", "arguments": map[string]interface{}{"directory": tmpDir}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_ToolsCall_WorkflowValidate(t *testing.T) {
	s := NewServer()
	wf := &workflow.Workflow{
		Name: "test",
		Steps: []workflow.WorkflowStep{
			{Node: "test", Params: map[string]string{"message": "hello"}},
		},
	}
	data, _ := yaml.Marshal(wf)

	params, _ := json.Marshal(map[string]interface{}{"name": "workflow_validate", "arguments": map[string]interface{}{"yaml": string(data)}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_ToolsCall_NodeList(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "node_list", "arguments": map[string]interface{}{}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_ToolsCall_NodeInfo(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "node_info", "arguments": map[string]interface{}{"name": "file_read"}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestHandleRequest_ToolsCall_HistoryList(t *testing.T) {
	s := NewServer()
	tmpDir := t.TempDir()
	history.SetHistoryDir(tmpDir)
	defer history.SetHistoryDir("")

	params, _ := json.Marshal(map[string]interface{}{"name": "history_list", "arguments": map[string]interface{}{}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_ToolsCall_TemplateList(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "template_list", "arguments": map[string]interface{}{}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_ToolsCall_TemplateRender(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "template_render", "arguments": map[string]interface{}{"name": "simple-llm"}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

// ------------------------------------------------------------------
// Concurrency tests
// ------------------------------------------------------------------

func TestClient_SequentialToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var resp rpcResponse
		resp.JSONRPC = "2.0"
		resp.ID = req.ID

		switch req.Method {
		case "initialize":
			resp.Result = initializeResult{
				Capabilities: serverCapabilities{Tools: toolsCapability{ListChanged: false}},
				ServerInfo:   serverInfo{Name: "test", Version: "1.0.0"},
			}
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
			return
		case "tools/list":
			resp.Result = toolsListResult{Tools: []tool{{Name: "seq_tool", Description: "Sequential test", InputSchema: inputSchema{Type: "object"}}}}
		case "tools/call":
			resp.Result = toolCallResult{Content: []content{{Type: "text", Text: "ok"}}}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := Connect(server.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	for i := 0; i < 3; i++ {
		_, err := client.CallTool("seq_tool", map[string]interface{}{})
		if err != nil {
			t.Errorf("sequential call %d error: %v", i, err)
		}
	}
}

func TestServer_ConcurrentRequests(t *testing.T) {
	s := NewServer()
	tmpDir := t.TempDir()
	wfPath := filepath.Join(tmpDir, "test.yaml")
	wf := &workflow.Workflow{
		Name: "test",
		Steps: []workflow.WorkflowStep{
			{Node: "test", Params: map[string]string{"message": "hello"}},
		},
	}
	data, _ := yaml.Marshal(wf)
	if err := os.WriteFile(wfPath, data, 0644); err != nil {
		t.Fatalf("failed to write workflow: %v", err)
	}

	const numGoroutines = 10
	var wg sync.WaitGroup
	errCount := 0

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			params, _ := json.Marshal(map[string]interface{}{"name": "list_nodes", "arguments": map[string]interface{}{}})
			req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
			resp := s.handleRequest(req)
			if resp == nil || resp.Error != nil {
				errCount++
			}
		}()
	}

	wg.Wait()

	if errCount > 0 {
		t.Errorf("got %d errors in concurrent requests", errCount)
	}
}

// ------------------------------------------------------------------
// Additional tools coverage tests
// ------------------------------------------------------------------

func TestToolWorkflowList_DefaultDir(t *testing.T) {
	s := NewServer()
	result, err := s.toolWorkflowList(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestToolWorkflowList_InvalidDir(t *testing.T) {
	s := NewServer()
	_, err := s.toolWorkflowList(map[string]interface{}{"directory": "/nonexistent/dir/path/xyz123"})
	if err == nil {
		t.Fatal("expected error for invalid directory")
	}
}

func TestToolWorkflowValidate_WithFile(t *testing.T) {
	s := NewServer()
	tmpDir := t.TempDir()
	wfPath := filepath.Join(tmpDir, "test.yaml")
	wf := &workflow.Workflow{
		Name: "test",
		Steps: []workflow.WorkflowStep{
			{Node: "test", Params: map[string]string{"message": "hello"}},
		},
	}
	data, _ := yaml.Marshal(wf)
	if err := os.WriteFile(wfPath, data, 0644); err != nil {
		t.Fatalf("failed to write workflow: %v", err)
	}

	result, err := s.toolWorkflowValidate(map[string]interface{}{"file": wfPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestToolWorkflowValidate_InvalidYAML(t *testing.T) {
	s := NewServer()
	_, err := s.toolWorkflowValidate(map[string]interface{}{"yaml": "invalid: ["})
	if err == nil {
		t.Fatal("expected error for invalid yaml")
	}
}

func TestToolHistoryList_WithFilters(t *testing.T) {
	s := NewServer()
	tmpDir := t.TempDir()
	history.SetHistoryDir(tmpDir)
	defer history.SetHistoryDir("")

	rec := history.Record{
		ID:        "test-filter-1",
		Name:      "test-workflow",
		Trigger:   history.TriggerCLI,
		Success:   true,
		StartedAt: time.Now().Add(-time.Hour),
		Duration:  5 * time.Second,
	}
	if err := history.SaveRecord(rec); err != nil {
		t.Fatalf("failed to save record: %v", err)
	}

	_, err := s.toolHistoryList(map[string]interface{}{"success_only": true, "workflow": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolTemplateList_ByKeyword(t *testing.T) {
	s := NewServer()
	result, err := s.toolTemplateList(map[string]interface{}{"keyword": "llm"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestCallExtendedTool_AllTools(t *testing.T) {
	s := NewServer()
	tools := []string{
		"create_workflow",
		"run_workflow",
		"run_workflow_yaml",
		"list_nodes",
		"validate_workflow",
		"workflow_run",
		"workflow_create",
		"workflow_list",
		"workflow_validate",
		"node_list",
		"node_info",
		"history_list",
		"template_list",
		"template_render",
	}

	for _, toolName := range tools {
		t.Run(toolName, func(t *testing.T) {
			var args map[string]interface{}
			switch toolName {
			case "create_workflow", "workflow_create":
				args = map[string]interface{}{"description": "test"}
			case "run_workflow", "workflow_run":
				args = map[string]interface{}{"file": "/nonexistent.yaml"}
			case "run_workflow_yaml":
				args = map[string]interface{}{"yaml": "name: test"}
			case "validate_workflow":
				args = map[string]interface{}{"file": "/nonexistent.yaml"}
			case "node_info":
				args = map[string]interface{}{"name": "file_read"}
			case "template_render":
				args = map[string]interface{}{"name": "simple-llm"}
			default:
				args = map[string]interface{}{}
			}

			done := make(chan struct{})
			go func() {
				defer close(done)
				_, _ = s.callExtendedTool(&toolCallParams{Name: toolName, Arguments: args})
			}()

			select {
			case <-done:
			case <-time.After(time.Second * 15):
				t.Fatal("timeout")
			}
		})
	}
}

func TestValidateMCPURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid http", "http://example.com/mcp", false},
		{"valid https", "https://example.com/mcp", false},
		{"valid localhost", "http://localhost:3000/sse", false},
		{"valid 127.0.0.1", "http://127.0.0.1:3000/mcp", false},
		{"invalid scheme ftp", "ftp://example.com/mcp", true},
		{"invalid scheme file", "file:///etc/passwd", true},
		{"invalid empty", "", true},
		{"invalid no host", "http:///mcp", true},
		{"userinfo in URL", "http://user:pass@example.com/mcp", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMCPURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateMCPURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

// ------------------------------------------------------------------
// Additional helper tests
// ------------------------------------------------------------------

func TestOptionalString(t *testing.T) {
	if optionalString(map[string]interface{}{"a": "hello"}, "a") != "hello" {
		t.Error("expected hello")
	}
	if optionalString(map[string]interface{}{}, "a") != "" {
		t.Error("expected empty for missing key")
	}
	if optionalString(map[string]interface{}{"a": 42}, "a") != "" {
		t.Error("expected empty for non-string")
	}
	if optionalString(map[string]interface{}{"a": ""}, "a") != "" {
		t.Error("expected empty string through")
	}
}

func TestRequireString_NonString(t *testing.T) {
	_, err := requireString(map[string]interface{}{"foo": 42}, "foo")
	if err == nil {
		t.Fatal("expected error for non-string type")
	}
	_, err = requireString(map[string]interface{}{"foo": true}, "foo")
	if err == nil {
		t.Fatal("expected error for bool type")
	}
	_, err = requireString(map[string]interface{}{"foo": nil}, "foo")
	if err == nil {
		t.Fatal("expected error for nil")
	}
}

func TestOptionalInt_EdgeCases(t *testing.T) {
	// Invalid string should return default
	if optionalInt(map[string]interface{}{"n": "not-a-number"}, "n", 5) != 5 {
		t.Error("expected default 5 for invalid string")
	}
	// bool should return default
	if optionalInt(map[string]interface{}{"n": true}, "n", 3) != 3 {
		t.Error("expected default 3 for bool")
	}
	// nil should return default
	if optionalInt(map[string]interface{}{"n": nil}, "n", 10) != 10 {
		t.Error("expected default 10 for nil")
	}
	// negative int
	if optionalInt(map[string]interface{}{"n": -5}, "n", 0) != -5 {
		t.Error("expected -5")
	}
	// float64 with decimal
	if optionalInt(map[string]interface{}{"n": 3.99}, "n", 0) != 3 {
		t.Error("expected 3 (truncated)")
	}
}

func TestOptionalBool_EdgeCases(t *testing.T) {
	// 1/0 as int (not handled, should return default)
	if optionalBool(map[string]interface{}{"b": 1}, "b", true) != true {
		t.Error("expected default true for int")
	}
	// mixed case string
	if !optionalBool(map[string]interface{}{"b": "True"}, "b", false) {
		t.Error("expected true for mixed case")
	}
	if optionalBool(map[string]interface{}{"b": "FALSE"}, "b", true) {
		t.Error("expected false for uppercase false")
	}
	// unrecognized string
	if !optionalBool(map[string]interface{}{"b": "yes"}, "b", true) {
		t.Error("expected default true for unrecognized string")
	}
}

func TestSanitizeError_HomeDir(t *testing.T) {
	// Home directory replacement is tested inline; just verify nil is safe
	if sanitizeError(nil) != nil {
		t.Error("expected nil for nil error")
	}
}

func TestSanitizeError_MorePatterns(t *testing.T) {
	patterns := []string{
		"invalid token: abc123",
		"missing secret key",
		"wrong password provided",
		"invalid credential pair",
	}
	for _, p := range patterns {
		err := sanitizeError(fmt.Errorf("%s", p))
		if !strings.Contains(err.Error(), "redacted") {
			t.Errorf("expected redacted error for pattern %q, got: %v", p, err)
		}
	}
}

func TestTruncate_EdgeCases(t *testing.T) {
	if truncate("", 5) != "" {
		t.Error("expected empty string")
	}
	if truncate("abc", 3) != "abc" {
		t.Error("expected exact match")
	}
	// "abcde" with maxLen=4 => "a..."
	if truncate("abcde", 4) != "a..." {
		t.Errorf("expected a..., got %s", truncate("abcde", 4))
	}
}

// ------------------------------------------------------------------
// Additional workflow tool tests
// ------------------------------------------------------------------

func TestToolWorkflowRun_PathTraversal(t *testing.T) {
	s := NewServer()
	// runWorkflow blocks path traversal; toolWorkflowRun doesn't
	_, err := s.runWorkflow(map[string]interface{}{"file": "../../etc/passwd"})
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("expected path traversal error, got: %v", err)
	}
}

func TestToolWorkflowList_NonDirPath(t *testing.T) {
	s := NewServer()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.yaml")
	_ = os.WriteFile(filePath, []byte("name: test"), 0644)

	// Passing a file path instead of directory should fail
	_, err := s.toolWorkflowList(map[string]interface{}{"directory": filePath})
	if err == nil {
		t.Fatal("expected error for non-directory path")
	}
}

// ------------------------------------------------------------------
// Additional node tool tests
// ------------------------------------------------------------------

func TestToolNodeInfo_EmptyString(t *testing.T) {
	s := NewServer()
	_, err := s.toolNodeInfo(map[string]interface{}{"name": ""})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestToolNodeInfo_WhitespaceOnly(t *testing.T) {
	s := NewServer()
	_, err := s.toolNodeInfo(map[string]interface{}{"name": "   "})
	if err == nil {
		t.Fatal("expected error for whitespace-only name")
	}
}

// ------------------------------------------------------------------
// Additional history tool tests
// ------------------------------------------------------------------

func TestToolHistoryList_NonExistentDir(t *testing.T) {
	s := NewServer()
	history.SetHistoryDir("/nonexistent/path/xyz123")
	defer history.SetHistoryDir("")

	// history.ListRecordsWithFilter may gracefully handle missing dirs;
	// we just check that it doesn't panic.
	result, err := s.toolHistoryList(map[string]interface{}{})
	if err != nil {
		t.Logf("history list error (acceptable): %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestToolHistoryList_TriggerFilter(t *testing.T) {
	s := NewServer()
	tmpDir := t.TempDir()
	history.SetHistoryDir(tmpDir)
	defer history.SetHistoryDir("")

	rec := history.Record{
		ID:        "test-trigger-1",
		Name:      "test-workflow",
		Trigger:   history.TriggerAPI,
		Success:   true,
		StartedAt: time.Now().Add(-time.Hour),
		Duration:  5 * time.Second,
	}
	if err := history.SaveRecord(rec); err != nil {
		t.Fatalf("failed to save record: %v", err)
	}

	result, err := s.toolHistoryList(map[string]interface{}{"limit": 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

// ------------------------------------------------------------------
// Memory tool tests
// ------------------------------------------------------------------

func TestToolMemoryStore_MissingValue(t *testing.T) {
	s := NewServer()
	_, err := s.toolMemoryStore(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing value")
	}
}

func TestToolMemoryStore_WithSession(t *testing.T) {
	s := NewServer()
	result, err := s.toolMemoryStore(map[string]interface{}{
		"session_id": "test-session",
		"key":        "my-key",
		"value":      "some memory content",
		"level":      "short",
		"type":       "fact",
		"tags":       "test,example",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Stored:") {
		t.Errorf("expected stored confirmation, got: %s", text)
	}
}

func TestToolMemoryStore_Defaults(t *testing.T) {
	s := NewServer()
	result, err := s.toolMemoryStore(map[string]interface{}{
		"value": "default session test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestToolMemoryStore_ConfidenceFloat(t *testing.T) {
	s := NewServer()
	result, err := s.toolMemoryStore(map[string]interface{}{
		"value":      "confidence test",
		"confidence": 0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestToolMemoryStore_ConfidenceInt(t *testing.T) {
	s := NewServer()
	result, err := s.toolMemoryStore(map[string]interface{}{
		"value":      "confidence int test",
		"confidence": 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestToolMemoryRetrieve_MissingKey(t *testing.T) {
	s := NewServer()
	_, err := s.toolMemoryRetrieve(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestToolMemoryRetrieve_NotFound(t *testing.T) {
	s := NewServer()
	_, err := s.toolMemoryRetrieve(map[string]interface{}{
		"session_id": "nonexistent-session",
		"key":        "nonexistent-key",
	})
	if err == nil {
		t.Fatal("expected error for non-existent key")
	}
}

func TestToolMemorySearch_MissingQuery(t *testing.T) {
	s := NewServer()
	_, err := s.toolMemorySearch(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

func TestToolMemorySearch_EmptyResults(t *testing.T) {
	s := NewServer()
	result, err := s.toolMemorySearch(map[string]interface{}{
		"session_id": "empty-session",
		"query":      "nonexistent-content-xyz",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "No matching") {
		t.Errorf("expected no matching message, got: %s", result.Content[0].Text)
	}
}

func TestToolMemorySearch_WithLevel(t *testing.T) {
	s := NewServer()
	// Store something first
	_, _ = s.toolMemoryStore(map[string]interface{}{
		"session_id": "search-session",
		"key":        "searchable-key",
		"value":      "searchable content for testing",
		"level":      "long",
	})

	result, err := s.toolMemorySearch(map[string]interface{}{
		"session_id": "search-session",
		"query":      "searchable",
		"level":      "long",
		"top_k":      5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestToolMemoryStats_DefaultSession(t *testing.T) {
	s := NewServer()
	result, err := s.toolMemoryStats(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestToolMemoryStats_Global(t *testing.T) {
	s := NewServer()
	result, err := s.toolMemoryStats(map[string]interface{}{"session_id": "global"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "Global") {
		t.Errorf("expected global stats, got: %s", result.Content[0].Text)
	}
}

func TestToolMemoryListSessions(t *testing.T) {
	s := NewServer()
	result, err := s.toolMemoryListSessions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

// ------------------------------------------------------------------
// Preference tool tests
// ------------------------------------------------------------------

func TestToolPreferenceGet_MissingKey(t *testing.T) {
	s := NewServer()
	_, err := s.toolPreferenceGet(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestToolPreferenceGet_NotFound(t *testing.T) {
	s := NewServer()
	result, err := s.toolPreferenceGet(map[string]interface{}{
		"key": "nonexistent-pref",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "No preference") {
		t.Errorf("expected not found message, got: %s", result.Content[0].Text)
	}
}

func TestToolPreferenceSet_MissingKey(t *testing.T) {
	s := NewServer()
	_, err := s.toolPreferenceSet(map[string]interface{}{"value": "test"})
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestToolPreferenceSet_MissingValue(t *testing.T) {
	s := NewServer()
	_, err := s.toolPreferenceSet(map[string]interface{}{"key": "test"})
	if err == nil {
		t.Fatal("expected error for missing value")
	}
}

func TestToolPreferenceSet_Learn(t *testing.T) {
	s := NewServer()
	result, err := s.toolPreferenceSet(map[string]interface{}{
		"key":      "coding_style",
		"value":    "functional",
		"category": "coding_style",
		"learn":    true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "learned") {
		t.Errorf("expected learned, got: %s", result.Content[0].Text)
	}
}

func TestToolPreferenceSet_Explicit(t *testing.T) {
	s := NewServer()
	result, err := s.toolPreferenceSet(map[string]interface{}{
		"key":      "verbosity",
		"value":    "concise",
		"category": "verbosity",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "set") {
		t.Errorf("expected set, got: %s", result.Content[0].Text)
	}
}

func TestToolPreferenceGet_AfterSet(t *testing.T) {
	s := NewServer()
	_, _ = s.toolPreferenceSet(map[string]interface{}{
		"key":      "test-key",
		"value":    "test-value",
		"category": "custom",
		"user_id":  "test-user",
	})

	result, err := s.toolPreferenceGet(map[string]interface{}{
		"key":      "test-key",
		"category": "custom",
		"user_id":  "test-user",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "test-value") {
		t.Errorf("expected test-value, got: %s", result.Content[0].Text)
	}
}

// ------------------------------------------------------------------
// Compress tool tests
// ------------------------------------------------------------------

func TestToolContextCompress_MissingText(t *testing.T) {
	s := NewServer()
	_, err := s.toolContextCompress(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing text")
	}
}

func TestToolContextCompress_Defaults(t *testing.T) {
	s := NewServer()
	result, err := s.toolContextCompress(map[string]interface{}{
		"text": "This is a test text to compress. It contains multiple sentences. We need to see how well the compression works.",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "Compressed:") {
		t.Errorf("expected compress output, got: %s", result.Content[0].Text)
	}
}

func TestToolContextCompress_WithAlgorithm(t *testing.T) {
	s := NewServer()
	result, err := s.toolContextCompress(map[string]interface{}{
		"text":      "Some text to compress with a specific algorithm.",
		"algorithm": "extract",
		"ratio":     0.5,
		"max_chars": 2000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

// ------------------------------------------------------------------
// Geospatial tool tests
// ------------------------------------------------------------------

func TestToolGeospatialQuery_MissingQuery(t *testing.T) {
	s := NewServer()
	_, err := s.toolGeospatialQuery(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

func TestToolGeospatialQuery_Defaults(t *testing.T) {
	s := NewServer()
	result, err := s.toolGeospatialQuery(map[string]interface{}{
		"query": "Show NDVI trend in Amazon basin",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Content[0].Text, "Geospatial") {
		t.Errorf("expected geospatial output, got: %s", result.Content[0].Text)
	}
}

func TestToolGeospatialQuery_WithRegion(t *testing.T) {
	s := NewServer()
	result, err := s.toolGeospatialQuery(map[string]interface{}{
		"query":      "Show NDVI trend",
		"dataset":    "landsat8",
		"region":     "Amazon",
		"time_start": "2020-01-01",
		"time_end":   "2024-12-31",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Amazon") {
		t.Errorf("expected region in output, got: %s", text)
	}
}

// ------------------------------------------------------------------
// Code graph tool tests
// ------------------------------------------------------------------

func TestToolCodeGraphIndex_Defaults(t *testing.T) {
	s := NewServer()
	_, err := s.toolCodeGraphIndex(map[string]interface{}{})
	if err != nil {
		t.Logf("code_graph_index error (may be expected): %v", err)
	}
}

func TestToolCodeGraphQuery_MissingQuery(t *testing.T) {
	s := NewServer()
	_, err := s.toolCodeGraphQuery(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

func TestToolCodeGraphQuery_WithTopK(t *testing.T) {
	s := NewServer()
	_, err := s.toolCodeGraphQuery(map[string]interface{}{
		"query": "authentication",
		"top_k": 5,
	})
	if err != nil {
		t.Logf("code_graph_query error (may be expected): %v", err)
	}
}

func TestToolCodeGraphStats(t *testing.T) {
	s := NewServer()
	_, err := s.toolCodeGraphStats()
	if err != nil {
		t.Logf("code_graph_stats error (may be expected): %v", err)
	}
}

// ------------------------------------------------------------------
// CallExtendedTool dispatch tests
// ------------------------------------------------------------------

func TestCallExtendedTool_UnknownTool(t *testing.T) {
	s := NewServer()
	_, err := s.callExtendedTool(&toolCallParams{Name: "nonexistent_tool_xyz"})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("expected unknown tool error, got: %v", err)
	}
}

func TestCallExtendedTool_EmptyToolName(t *testing.T) {
	s := NewServer()
	_, err := s.callExtendedTool(&toolCallParams{Name: ""})
	if err == nil {
		t.Fatal("expected error for empty tool name")
	}
}

// ------------------------------------------------------------------
// Additional Client tests
// ------------------------------------------------------------------

func TestClient_ListTools_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
		if req.Method == "initialize" {
			resp.Result = initializeResult{
				Capabilities: serverCapabilities{Tools: toolsCapability{ListChanged: false}},
				ServerInfo:   serverInfo{Name: "test", Version: "1.0.0"},
			}
		} else if req.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		} else if req.Method == "tools/list" {
			resp.Error = &rpcError{Code: -32603, Message: "server error"}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := Connect(server.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	_, err = client.ListTools()
	if err == nil {
		t.Fatal("expected error from ListTools")
	}
}

func TestClient_ListTools_InvalidResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
		if req.Method == "initialize" {
			resp.Result = initializeResult{
				Capabilities: serverCapabilities{Tools: toolsCapability{ListChanged: false}},
				ServerInfo:   serverInfo{Name: "test", Version: "1.0.0"},
			}
		} else if req.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		} else if req.Method == "tools/list" {
			resp.Result = "invalid result type"
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := Connect(server.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	_, err = client.ListTools()
	if err == nil {
		t.Fatal("expected error from ListTools with invalid result")
	}
}

func TestClient_CallTool_NilArgs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
		if req.Method == "initialize" {
			resp.Result = initializeResult{
				Capabilities: serverCapabilities{Tools: toolsCapability{ListChanged: false}},
				ServerInfo:   serverInfo{Name: "test", Version: "1.0.0"},
			}
		} else if req.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		} else if req.Method == "tools/call" {
			resp.Result = toolCallResult{Content: []content{{Type: "text", Text: "ok"}}}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := Connect(server.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	// nil args should be fine, gets marshaled as null
	result, err := client.CallTool("test_tool", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestClient_CallTool_InvalidResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
		if req.Method == "initialize" {
			resp.Result = initializeResult{
				Capabilities: serverCapabilities{Tools: toolsCapability{ListChanged: false}},
				ServerInfo:   serverInfo{Name: "test", Version: "1.0.0"},
			}
		} else if req.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		} else if req.Method == "tools/call" {
			resp.Result = "not a tool result"
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := Connect(server.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	_, err = client.CallTool("some_tool", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for invalid result type")
	}
}

func TestClient_SendRequest_Timeout_Direct(t *testing.T) {
	// Direct timeout test: pending request with no response
	if testing.Short() {
		t.Skip("requires timeout behavior")
	}
	c := &Client{
		pending:   make(map[int]chan *rpcResponse),
		httpClient: &http.Client{Timeout: 1 * time.Second},
	}
	// sendRequest with a real unreachable endpoint
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "test"}
	_, err := c.sendRequest(req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_SanitizeErrorString_EdgeCases(t *testing.T) {
	// Empty string
	if sanitizeErrorString("") != "" {
		t.Error("expected empty string unchanged")
	}
	// All caps sensitive word
	if sanitizeErrorString("KEY not found") != "tool execution failed (sensitive details redacted)" {
		t.Error("expected KEY to be redacted")
	}
	// Mixed case
	if sanitizeErrorString("my Token is invalid") != "tool execution failed (sensitive details redacted)" {
		t.Error("expected Token to be redacted")
	}
}

func TestClient_ValidateMCPURL_EdgeCases(t *testing.T) {
	// IPv6 loopback
	err := validateMCPURL("http://[::1]:8080/mcp")
	if err != nil {
		t.Errorf("expected valid IPv6 loopback URL, got: %v", err)
	}
	// Port
	err = validateMCPURL("http://localhost:9999/sse")
	if err != nil {
		t.Errorf("expected valid localhost with port, got: %v", err)
	}
}

func TestValidateMCPIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		{"loopback 127.0.0.1", "127.0.0.1", false},
		{"loopback ::1", "::1", false},
		{"public 8.8.8.8", "8.8.8.8", false},
		{"private 10.0.0.1", "10.0.0.1", true},
		{"private 192.168.1.1", "192.168.1.1", true},
		{"private 172.16.0.1", "172.16.0.1", true},
		{"link-local 169.254.1.1", "169.254.1.1", true},
		{"unspecified 0.0.0.0", "0.0.0.0", true},
		{"multicast 224.0.0.1", "224.0.0.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("invalid test IP: %s", tt.ip)
			}
			err := validateMCPIP(ip, tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateMCPIP(%q) error = %v, wantErr %v", tt.ip, err, tt.wantErr)
			}
		})
	}
}
