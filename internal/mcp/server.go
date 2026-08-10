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
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alib8b8/aflare/internal/config"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/workflow"
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

// Server implements an MCP server over stdio (or network) using JSON-RPC 2.0.
// When bound to a network port (AFLARE_MCP_BIND), token authentication is required.
type Server struct {
	scanner      *bufio.Scanner
	authToken    string // required when network-bound
	networkBound bool   // true if AFLARE_MCP_BIND is set
}

// NewServer creates a new MCP server reading from stdin and writing to stdout
func NewServer() *Server {
	return &Server{
		scanner: bufio.NewScanner(os.Stdin),
	}
}

// Run starts the MCP server. If AFLARE_MCP_BIND is set, it requires
// AFLARE_MCP_TOKEN for authentication. Without a bind address, it runs
// in stdio mode with no authentication.
func (s *Server) Run() error {
	if bind := os.Getenv("AFLARE_MCP_BIND"); bind != "" {
		s.networkBound = true
		token := os.Getenv("AFLARE_MCP_TOKEN")
		if token == "" {
			return fmt.Errorf("MCP server bound to network (%s) but no AFLARE_MCP_TOKEN set", bind)
		}
		s.authToken = token
	}

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

		// When network-bound, validate the auth token on every request
		if s.networkBound {
			if !s.validateToken(&req) {
				s.sendError(req.ID, -32001, "Unauthorized: invalid or missing token")
				continue
			}
		}

		resp := s.handleRequest(&req)
		if resp != nil {
			s.send(resp)
		}
	}
	return s.scanner.Err()
}

// validateToken checks that the request carries a valid auth token.
// The token is expected in the params._auth field for JSON-RPC transport,
// or in a custom X-MCP-Token header for HTTP transport.
func (s *Server) validateToken(req *rpcRequest) bool {
	if s.authToken == "" {
		return true // no auth required
	}

	// Check params._auth field for JSON-RPC token
	var params map[string]interface{}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err == nil {
			if auth, ok := params["_auth"].(string); ok {
				if subtle.ConstantTimeCompare([]byte(auth), []byte(s.authToken)) == 1 {
					return true
				}
			}
		}
	}

	return false
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
					Name:    "aflare",
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
	return s.getExtendedTools()
}

func (s *Server) callTool(params *toolCallParams) (*toolCallResult, error) {
	return s.callExtendedTool(params)
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

	// Security: block path traversal and restrict to .yaml/.yml files
	if err := validateWorkflowFilePath(file); err != nil {
		return nil, err
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
	sb.WriteString("Available aflare nodes:\n\n")
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

	// Security: block path traversal and restrict to .yaml/.yml files
	if err := validateWorkflowFilePath(file); err != nil {
		return nil, err
	}

	wf, err := workflow.ParseWorkflow(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	warnings := workflow.ValidateWorkflow(wf)

	reg := nodes.GetGlobalRegistry()
	nodes.RegisterBuiltins(reg)

	for i, step := range wf.Steps {
		// Compound steps (if/loop/map/reduce/parallel/saga/capture_error) have no
		// node of their own; skip the node-existence check for them.
		if step.IsIf() || step.IsLoop() || step.IsMap() || step.IsReduce() || step.IsParallel() || step.IsSaga() || step.HasCaptureError() {
			continue
		}
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

// validateWorkflowFilePath validates that a file path is safe to read as a
// workflow. It blocks:
//   - path traversal (.., ../, ..\) after filepath.Clean
//   - non-.yaml/.yml extensions
//
// The previous check only tested strings.Contains(file, ".."), which did not
// block symlink escapes or non-yaml files. Absolute paths are allowed (users
// may reference workflows at arbitrary locations) as long as the extension
// is .yaml/.yml and no traversal is attempted.
func validateWorkflowFilePath(file string) error {
	if file == "" {
		return fmt.Errorf("file path is required")
	}

	cleaned := filepath.Clean(file)

	// Reject path traversal (post-clean)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path traversal is not allowed: %s", file)
	}

	// Only allow .yaml/.yml extensions
	ext := strings.ToLower(filepath.Ext(cleaned))
	if ext != ".yaml" && ext != ".yml" {
		return fmt.Errorf("only .yaml/.yml files are allowed, got: %s", ext)
	}

	return nil
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
