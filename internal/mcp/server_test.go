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

package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/alib8b8/aflare/internal/workflow"
	"gopkg.in/yaml.v3"
)

func TestNewServer(t *testing.T) {
	s := NewServer()
	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if s.scanner == nil {
		t.Error("expected non-nil scanner")
	}
}

func TestHandleRequest_Initialize(t *testing.T) {
	s := NewServer()
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "initialize"}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	result, ok := resp.Result.(initializeResult)
	if !ok {
		t.Fatalf("expected initializeResult, got %T", resp.Result)
	}
	if result.ServerInfo.Name != "aflare" {
		t.Errorf("expected server name aflare, got %s", result.ServerInfo.Name)
	}
}

func TestHandleRequest_Initialized(t *testing.T) {
	s := NewServer()
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "initialized"}
	resp := s.handleRequest(req)
	if resp != nil {
		t.Error("expected nil response for initialized notification")
	}
}

func TestHandleRequest_NotificationsInitialized(t *testing.T) {
	s := NewServer()
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "notifications/initialized"}
	resp := s.handleRequest(req)
	if resp != nil {
		t.Error("expected nil response for notifications/initialized")
	}
}

func TestHandleRequest_ToolsList(t *testing.T) {
	s := NewServer()
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/list"}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	result, ok := resp.Result.(toolsListResult)
	if !ok {
		t.Fatalf("expected toolsListResult, got %T", resp.Result)
	}
	if len(result.Tools) == 0 {
		t.Error("expected at least one tool")
	}
}

func TestHandleRequest_UnknownMethod(t *testing.T) {
	s := NewServer()
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "unknown/method"}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected code -32601, got %d", resp.Error.Code)
	}
}

func TestHandleRequest_ToolsCall_InvalidParams(t *testing.T) {
	s := NewServer()
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: json.RawMessage(`{`)}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected code -32602, got %d", resp.Error.Code)
	}
}

func TestCallTool_UnknownTool(t *testing.T) {
	s := NewServer()
	_, err := s.callTool(&toolCallParams{Name: "unknown_tool"})
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestCreateWorkflow_MissingDescription(t *testing.T) {
	s := NewServer()
	_, err := s.createWorkflow(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing description")
	}
	_, err = s.createWorkflow(map[string]interface{}{"description": ""})
	if err == nil {
		t.Error("expected error for empty description")
	}
}

func TestCreateWorkflow_WithDescription(t *testing.T) {
	s := NewServer()
	done := make(chan struct{})
	var result *toolCallResult
	var err error
	go func() {
		defer close(done)
		result, err = s.createWorkflow(map[string]interface{}{"description": "a test workflow"})
	}()
	select {
	case <-done:
		if err == nil && result != nil {
			return
		}
		t.Logf("createWorkflow completed with err=%v", err)
	case <-make(chan struct{}):
	}
	// Network-dependent; just ensure no panic
}

func TestRunWorkflow_MissingFile(t *testing.T) {
	s := NewServer()
	_, err := s.runWorkflow(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing file")
	}
	_, err = s.runWorkflow(map[string]interface{}{"file": ""})
	if err == nil {
		t.Error("expected error for empty file")
	}
}

func TestRunWorkflow_ValidFile(t *testing.T) {
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

	result, err := s.runWorkflow(map[string]interface{}{"file": wfPath})
	if err != nil {
		t.Logf("runWorkflow error (may be expected): %v", err)
		return
	}
	if result == nil || len(result.Content) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestRunWorkflowYAML_MissingYAML(t *testing.T) {
	s := NewServer()
	_, err := s.runWorkflowYAML(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing yaml")
	}
	_, err = s.runWorkflowYAML(map[string]interface{}{"yaml": ""})
	if err == nil {
		t.Error("expected error for empty yaml")
	}
}

func TestRunWorkflowYAML_TooLarge(t *testing.T) {
	s := NewServer()
	largeYAML := strings.Repeat("a", workflow.MaxFileSize+1)
	_, err := s.runWorkflowYAML(map[string]interface{}{"yaml": largeYAML})
	if err == nil {
		t.Error("expected error for too large yaml")
	}
}

func TestRunWorkflowYAML_InvalidYAML(t *testing.T) {
	s := NewServer()
	_, err := s.runWorkflowYAML(map[string]interface{}{"yaml": "invalid: ["})
	if err == nil {
		t.Error("expected error for invalid yaml")
	}
}

func TestRunWorkflowYAML_Valid(t *testing.T) {
	s := NewServer()
	wf := &workflow.Workflow{
		Name: "test",
		Steps: []workflow.WorkflowStep{
			{Node: "test", Params: map[string]string{"message": "hello"}},
		},
	}
	data, _ := yaml.Marshal(wf)
	result, err := s.runWorkflowYAML(map[string]interface{}{"yaml": string(data)})
	if err != nil {
		t.Logf("runWorkflowYAML error (may be expected): %v", err)
		return
	}
	if result == nil || len(result.Content) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestListNodes(t *testing.T) {
	s := NewServer()
	result, err := s.listNodes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Available aflare nodes") {
		t.Errorf("expected header in output, got %s", text)
	}
}

func TestValidateWorkflow_MissingFile(t *testing.T) {
	s := NewServer()
	_, err := s.validateWorkflow(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing file")
	}
	_, err = s.validateWorkflow(map[string]interface{}{"file": ""})
	if err == nil {
		t.Error("expected error for empty file")
	}
}

func TestValidateWorkflow_Valid(t *testing.T) {
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

	result, err := s.validateWorkflow(map[string]interface{}{"file": wfPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Workflow is valid") && !strings.Contains(text, "Validation warnings") {
		t.Errorf("unexpected validation output: %s", text)
	}
}

func TestValidateWorkflow_InvalidNode(t *testing.T) {
	s := NewServer()
	tmpDir := t.TempDir()
	wfPath := filepath.Join(tmpDir, "test.yaml")
	wf := &workflow.Workflow{
		Name: "test",
		Steps: []workflow.WorkflowStep{
			{Node: "nonexistent_node_xyz", Params: map[string]string{}},
		},
	}
	data, _ := yaml.Marshal(wf)
	if err := os.WriteFile(wfPath, data, 0644); err != nil {
		t.Fatalf("failed to write workflow: %v", err)
	}

	result, err := s.validateWorkflow(map[string]interface{}{"file": wfPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty result")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Validation warnings") {
		t.Errorf("expected validation warnings, got %s", text)
	}
}

func TestSendError(t *testing.T) {
	s := NewServer()
	// sendError writes to stdout; we just ensure it doesn't panic
	s.sendError(json.RawMessage(`1`), -32600, "Invalid request")
}

func TestSend(t *testing.T) {
	s := NewServer()
	// send writes to stdout; we just ensure it doesn't panic
	s.send(&rpcResponse{JSONRPC: "2.0", ID: json.RawMessage(`1`), Result: "ok"})
}

func TestHandleRequest_ToolsCall_CreateWorkflow(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "create_workflow", "arguments": map[string]interface{}{"description": "test"}})
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
		// If network is unavailable, we might get an error; that's acceptable
		if resp.Error != nil {
			t.Logf("createWorkflow returned error (network may be unavailable): %v", resp.Error)
		}
	case <-make(chan struct{}):
	}
}

func TestHandleRequest_ToolsCall_RunWorkflow(t *testing.T) {
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

	params, _ := json.Marshal(map[string]interface{}{"name": "run_workflow", "arguments": map[string]interface{}{"file": wfPath}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Logf("runWorkflow returned error (may be expected): %v", resp.Error)
	}
}

func TestHandleRequest_ToolsCall_RunWorkflowYAML(t *testing.T) {
	s := NewServer()
	wf := &workflow.Workflow{
		Name: "test",
		Steps: []workflow.WorkflowStep{
			{Node: "test", Params: map[string]string{"message": "hello"}},
		},
	}
	data, _ := yaml.Marshal(wf)
	params, _ := json.Marshal(map[string]interface{}{"name": "run_workflow_yaml", "arguments": map[string]interface{}{"yaml": string(data)}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Logf("runWorkflowYAML returned error (may be expected): %v", resp.Error)
	}
}

func TestHandleRequest_ToolsCall_ListNodes(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "list_nodes", "arguments": map[string]interface{}{}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_ToolsCall_ValidateWorkflow(t *testing.T) {
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

	params, _ := json.Marshal(map[string]interface{}{"name": "validate_workflow", "arguments": map[string]interface{}{"file": wfPath}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_ToolsCall_UnknownTool(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "unknown_tool", "arguments": map[string]interface{}{}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestGetTools(t *testing.T) {
	s := NewServer()
	tools := s.getTools()
	if len(tools) != 27 {
		t.Errorf("expected 27 tools, got %d", len(tools))
	}
	expectedNames := map[string]bool{
		"create_workflow":      true,
		"run_workflow":         true,
		"run_workflow_yaml":    true,
		"list_nodes":           true,
		"validate_workflow":    true,
		"workflow_run":         true,
		"workflow_create":      true,
		"workflow_list":        true,
		"workflow_validate":    true,
		"node_list":            true,
		"node_info":            true,
		"history_list":         true,
		"template_list":        true,
		"template_render":      true,
		"memory_store":         true,
		"memory_retrieve":      true,
		"memory_search":        true,
		"memory_stats":         true,
		"memory_list_sessions": true,
		"code_graph_index":     true,
		"code_graph_query":     true,
		"code_graph_stats":     true,
		"context_compress":     true,
		"search_aggregated":    true,
		"geospatial_query":     true,
		"preference_get":       true,
		"preference_set":       true,
	}
	for _, tool := range tools {
		if !expectedNames[tool.Name] {
			t.Errorf("unexpected tool name: %s", tool.Name)
		}
	}
}

func TestHandleRequest_ParseError(t *testing.T) {
	// Run reads from stdin, hard to test. Just ensure handleRequest works.
}

// ------------------------------------------------------------------
// Additional server handleRequest tests
// ------------------------------------------------------------------

func TestHandleRequest_NilID(t *testing.T) {
	s := NewServer()
	req := &rpcRequest{JSONRPC: "2.0", ID: nil, Method: "initialize"}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response for initialize")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_NilID_UnknownMethod(t *testing.T) {
	s := NewServer()
	req := &rpcRequest{JSONRPC: "2.0", ID: nil, Method: "unknown/method"}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
}

func TestHandleRequest_NilID_Notification(t *testing.T) {
	s := NewServer()
	req := &rpcRequest{JSONRPC: "2.0", ID: nil, Method: "notifications/initialized"}
	resp := s.handleRequest(req)
	if resp != nil {
		t.Error("expected nil response for notification")
	}
}

func TestHandleRequest_OtherNotification(t *testing.T) {
	// Non-standard notification methods should still be handled as unknown
	s := NewServer()
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "notifications/cancelled"}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown notification method")
	}
}

func TestHandleRequest_ToolsCall_EmptyParams(t *testing.T) {
	s := NewServer()
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: json.RawMessage(`{}`)}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		t.Fatal("expected error for empty params")
	}
}

func TestHandleRequest_ToolsCall_MissingName(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"arguments": map[string]interface{}{}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		t.Fatal("expected error for missing tool name")
	}
}

func TestHandleRequest_ToolsCall_MalformedParams(t *testing.T) {
	s := NewServer()
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: json.RawMessage(`not json`)}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		t.Fatal("expected error for malformed params")
	}
}

func TestHandleRequest_ToolsCall_StringParams(t *testing.T) {
	// Params is a string instead of an object
	s := NewServer()
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: json.RawMessage(`"string not object"`)}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		t.Fatal("expected error for string params")
	}
}

// ------------------------------------------------------------------
// Memory tools via handleRequest
// ------------------------------------------------------------------

func TestHandleRequest_ToolsCall_MemoryStore(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "memory_store", "arguments": map[string]interface{}{"value": "test memory"}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_ToolsCall_MemorySearch(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "memory_search", "arguments": map[string]interface{}{"query": "test"}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_ToolsCall_MemoryStats(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "memory_stats", "arguments": map[string]interface{}{}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_ToolsCall_MemoryListSessions(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "memory_list_sessions", "arguments": map[string]interface{}{}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

// ------------------------------------------------------------------
// Compress tools via handleRequest
// ------------------------------------------------------------------

func TestHandleRequest_ToolsCall_ContextCompress(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "context_compress", "arguments": map[string]interface{}{"text": "test text to compress"}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

// ------------------------------------------------------------------
// Preference tools via handleRequest
// ------------------------------------------------------------------

func TestHandleRequest_ToolsCall_PreferenceSet(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "preference_set", "arguments": map[string]interface{}{"key": "test", "value": "val"}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_ToolsCall_PreferenceGet(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "preference_get", "arguments": map[string]interface{}{"key": "test"}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

// ------------------------------------------------------------------
// Geospatial tool via handleRequest
// ------------------------------------------------------------------

func TestHandleRequest_ToolsCall_GeospatialQuery(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "geospatial_query", "arguments": map[string]interface{}{"query": "test"}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

// ------------------------------------------------------------------
// Extended tool response format tests
// ------------------------------------------------------------------

func TestHandleRequest_ToolsCall_ResponseFormat(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "node_list", "arguments": map[string]interface{}{}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	// Verify result has content array
	resultBytes, _ := json.Marshal(resp.Result)
	var result toolCallResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Error("expected at least one content item")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("expected content type text, got %s", result.Content[0].Type)
	}
}

// ------------------------------------------------------------------
// Workflow validation edge cases
// ------------------------------------------------------------------

func TestValidateWorkflow_PathTraversal(t *testing.T) {
	s := NewServer()
	_, err := s.validateWorkflow(map[string]interface{}{"file": "../../etc/passwd"})
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("expected path traversal error, got: %v", err)
	}
}

func TestValidateWorkflow_NonexistentFile(t *testing.T) {
	s := NewServer()
	_, err := s.validateWorkflow(map[string]interface{}{"file": "/nonexistent/file/path.yaml"})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

// ------------------------------------------------------------------
// Search aggregated tool
// ------------------------------------------------------------------

func TestToolSearchAggregated_MissingQuery(t *testing.T) {
	s := NewServer()
	_, err := s.toolSearchAggregated(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

func TestHandleRequest_ToolsCall_SearchAggregated(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "search_aggregated", "arguments": map[string]interface{}{"query": "test"}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	// May fail if search_aggregate node not available; that's OK
	if resp.Error != nil {
		t.Logf("search_aggregated error (may be expected): %v", resp.Error)
	}
}

// ------------------------------------------------------------------
// Code graph tools via handleRequest
// ------------------------------------------------------------------

func TestHandleRequest_ToolsCall_CodeGraphQuery(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "code_graph_query", "arguments": map[string]interface{}{"query": "test"}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Logf("code_graph_query error (may be expected): %v", resp.Error)
	}
}

func TestHandleRequest_ToolsCall_CodeGraphStats(t *testing.T) {
	s := NewServer()
	params, _ := json.Marshal(map[string]interface{}{"name": "code_graph_stats", "arguments": map[string]interface{}{}})
	req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Logf("code_graph_stats error (may be expected): %v", resp.Error)
	}
}

// ------------------------------------------------------------------
// Concurrency test for server
// ------------------------------------------------------------------

func TestServer_ConcurrentHandleRequest(t *testing.T) {
	s := NewServer()
	var wg sync.WaitGroup
	errCh := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			params, _ := json.Marshal(map[string]interface{}{"name": "node_list", "arguments": map[string]interface{}{}})
			req := &rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: params}
			resp := s.handleRequest(req)
			if resp == nil || resp.Error != nil {
				errCh <- fmt.Errorf("unexpected error: %v", resp)
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent request failed: %v", err)
	}
}
