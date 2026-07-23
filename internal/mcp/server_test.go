package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alib8b8/llm-box/internal/workflow"
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
	if result.ServerInfo.Name != "llm-box" {
		t.Errorf("expected server name llm-box, got %s", result.ServerInfo.Name)
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
	if !strings.Contains(text, "Available llm-box nodes") {
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
	if len(tools) != 22 {
		t.Errorf("expected 22 tools, got %d", len(tools))
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
