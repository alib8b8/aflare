package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/alib8b8/llm-box/internal/config"
	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/workflow"
	"gopkg.in/yaml.v3"
)

// JSON-RPC 2.0 types

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP-specific types

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type serverCapabilities struct {
	Tools toolsCapability `json:"tools"`
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
	ServerInfo   serverInfo         `json:"serverInfo"`
}

type tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema inputSchema `json:"inputSchema"`
}

type inputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

type toolsListResult struct {
	Tools []tool `json:"tools"`
}

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type toolCallResult struct {
	Content []content `json:"content"`
}

type content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Server implements an MCP server over stdio using JSON-RPC 2.0
type Server struct {
	scanner *bufio.Scanner
}

// NewServer creates a new MCP server reading from stdin and writing to stdout
func NewServer() *Server {
	return &Server{
		scanner: bufio.NewScanner(os.Stdin),
	}
}

// Run starts the MCP server, reading JSON-RPC messages from stdin
func (s *Server) Run() error {
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if line == "" {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendError(req.ID, -32700, "Parse error")
			continue
		}

		resp := s.handleRequest(&req)
		if resp != nil {
			s.send(resp)
		}
	}
	return s.scanner.Err()
}

func (s *Server) handleRequest(req *rpcRequest) *rpcResponse {
	switch req.Method {
	case "initialize":
		return &rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: initializeResult{
				Capabilities: serverCapabilities{
					Tools: toolsCapability{ListChanged: false},
				},
				ServerInfo: serverInfo{
					Name:    "llm-box",
					Version: "1.0.0",
				},
			},
		}

	case "initialized", "notifications/initialized":
		// notification, no response needed
		return nil

	case "tools/list":
		return &rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  toolsListResult{Tools: s.getTools()},
		}

	case "tools/call":
		var params toolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return &rpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &rpcError{Code: -32602, Message: "Invalid params"},
			}
		}
		result, err := s.callTool(&params)
		if err != nil {
			return &rpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &rpcError{Code: -32603, Message: err.Error()},
			}
		}
		return &rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}

	default:
		return &rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: "Method not found: " + req.Method},
		}
	}
}

func (s *Server) getTools() []tool {
	return []tool{
		{
			Name:        "create_workflow",
			Description: "Generate a YAML workflow from a plain English description. Returns the workflow YAML content.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Plain English description of the workflow to generate (e.g., 'fetch Hacker News and save to file')",
					},
				},
				Required: []string{"description"},
			},
		},
		{
			Name:        "run_workflow",
			Description: "Execute a llm-box workflow from a YAML file path. Returns the final output of the workflow.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"file": map[string]interface{}{
						"type":        "string",
						"description": "Path to the YAML workflow file to execute",
					},
				},
				Required: []string{"file"},
			},
		},
		{
			Name:        "run_workflow_yaml",
			Description: "Execute a llm-box workflow from raw YAML content. Returns the final output of the workflow.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"yaml": map[string]interface{}{
						"type":        "string",
						"description": "Raw YAML content of the workflow to execute",
					},
				},
				Required: []string{"yaml"},
			},
		},
		{
			Name:        "list_nodes",
			Description: "List all available llm-box nodes with their descriptions. Call this to discover what nodes can be used in workflows.",
			InputSchema: inputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
		{
			Name:        "validate_workflow",
			Description: "Validate a llm-box workflow YAML file without executing it. Returns validation warnings if any.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"file": map[string]interface{}{
						"type":        "string",
						"description": "Path to the YAML workflow file to validate",
					},
				},
				Required: []string{"file"},
			},
		},
	}
}

func (s *Server) callTool(params *toolCallParams) (*toolCallResult, error) {
	switch params.Name {
	case "create_workflow":
		return s.createWorkflow(params.Arguments)

	case "run_workflow":
		return s.runWorkflow(params.Arguments)

	case "run_workflow_yaml":
		return s.runWorkflowYAML(params.Arguments)

	case "list_nodes":
		return s.listNodes()

	case "validate_workflow":
		return s.validateWorkflow(params.Arguments)

	default:
		return nil, fmt.Errorf("unknown tool: %s", params.Name)
	}
}

func (s *Server) createWorkflow(args map[string]interface{}) (*toolCallResult, error) {
	desc, ok := args["description"].(string)
	if !ok || desc == "" {
		return nil, fmt.Errorf("description parameter is required")
	}

	wf, err := workflow.GenerateWorkflow(desc)
	if err != nil {
		return nil, fmt.Errorf("failed to generate workflow: %w", err)
	}

	yamlBytes, err := yaml.Marshal(wf)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow: %w", err)
	}

	return &toolCallResult{
		Content: []content{
			{Type: "text", Text: string(yamlBytes)},
		},
	}, nil
}

func (s *Server) runWorkflow(args map[string]interface{}) (*toolCallResult, error) {
	file, ok := args["file"].(string)
	if !ok || file == "" {
		return nil, fmt.Errorf("file parameter is required")
	}

	wf, err := workflow.ParseWorkflow(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	reg := nodes.GetGlobalRegistry()
	if config.IsSafeMode() {
		reg.SetSafeMode(true)
	}

	result, _, err := workflow.ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		return nil, fmt.Errorf("workflow execution failed: %w", err)
	}

	return &toolCallResult{
		Content: []content{
			{Type: "text", Text: result},
		},
	}, nil
}

func (s *Server) runWorkflowYAML(args map[string]interface{}) (*toolCallResult, error) {
	yamlStr, ok := args["yaml"].(string)
	if !ok || yamlStr == "" {
		return nil, fmt.Errorf("yaml parameter is required")
	}

	if len(yamlStr) > workflow.MaxFileSize {
		return nil, fmt.Errorf("workflow YAML too large (max %d bytes)", workflow.MaxFileSize)
	}

	var wf workflow.Workflow
	if err := yaml.Unmarshal([]byte(yamlStr), &wf); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	reg := nodes.GetGlobalRegistry()
	if config.IsSafeMode() {
		reg.SetSafeMode(true)
	}

	result, _, err := workflow.ExecuteWorkflow(context.Background(), &wf, reg)
	if err != nil {
		return nil, fmt.Errorf("workflow execution failed: %w", err)
	}

	return &toolCallResult{
		Content: []content{
			{Type: "text", Text: result},
		},
	}, nil
}

func (s *Server) listNodes() (*toolCallResult, error) {
	reg := nodes.GetGlobalRegistry()
	nodes.RegisterBuiltins(reg)

	nodeList := reg.ListNodes()

	var sb strings.Builder
	sb.WriteString("Available llm-box nodes:\n\n")
	sb.WriteString(fmt.Sprintf("%-20s %s\n", "NAME", "DESCRIPTION"))
	sb.WriteString(strings.Repeat("-", 70))
	sb.WriteString("\n")
	for _, info := range nodeList {
		sb.WriteString(fmt.Sprintf("%-20s %s\n", info.Name, info.Description))
	}

	return &toolCallResult{
		Content: []content{
			{Type: "text", Text: sb.String()},
		},
	}, nil
}

func (s *Server) validateWorkflow(args map[string]interface{}) (*toolCallResult, error) {
	file, ok := args["file"].(string)
	if !ok || file == "" {
		return nil, fmt.Errorf("file parameter is required")
	}

	wf, err := workflow.ParseWorkflow(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	warnings := workflow.ValidateWorkflow(wf)

	reg := nodes.GetGlobalRegistry()
	nodes.RegisterBuiltins(reg)

	for i, step := range wf.Steps {
		if _, ok := reg.Get(step.Node); !ok {
			warnings = append(warnings, fmt.Sprintf("Step %d: unknown node '%s'", i+1, step.Node))
		}
	}

	var sb strings.Builder
	if len(warnings) == 0 {
		sb.WriteString("✅ Workflow is valid. No issues found.")
	} else {
		sb.WriteString("⚠️ Validation warnings:\n")
		for _, w := range warnings {
			sb.WriteString(fmt.Sprintf("  - %s\n", w))
		}
	}

	return &toolCallResult{
		Content: []content{
			{Type: "text", Text: sb.String()},
		},
	}, nil
}

func (s *Server) send(resp *rpcResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	fmt.Fprintln(os.Stdout, string(data))
}

func (s *Server) sendError(id json.RawMessage, code int, message string) {
	s.send(&rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	})
}
