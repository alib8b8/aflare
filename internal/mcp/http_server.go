// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌​​‌​‌‌​‌‌​‌​​​​‌​​‌‌‌​​​​‌‌​​‌‌‌​‌‌​‌​​‌‌​​​‌‌​‌​​​‌​​‌​‌‌‌‌​​​​‌​​​​​​​​​​​​​​​​​​​‌‌​​‌‌‌​​​‌​​​‌​⁠
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​‌‌​‌‌‌​‌‌​‌​‌‌‌​​‌‌‌​​​​​​​‌​‌​‌‌​‌​‌​‌​‌‌‌​‌​‌​‌‌‌​​​​​‌‌‌‌‌​‌‌​​‌‌​​​‌‌‌‌‌‌​‌‌‌‌​‌​‌‌​‌⁠
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
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTP transport for the MCP server.
//
// Two endpoints share the same handlers as the stdio server:
//
//	POST /mcp     — standard JSON-RPC 2.0 message body, identical to what the
//	                stdio transport accepts (MCP Streamable HTTP clients).
//	POST /v1/call — simplified direct tool call: {"name": "...", "arguments": {...}}
//	                wrapped into a tools/call JSON-RPC request internally.
//
// Authentication: every request must carry the token in the X-MCP-Token
// header (constant-time compared). Unlike loopback stdio, an HTTP listener
// is a network surface, so a token is REQUIRED — there is no auth-free mode.

const (
	// httpTokenHeader is the header checked for the shared bearer token.
	httpTokenHeader = "X-MCP-Token"
	// httpMaxBodyBytes bounds the request body (guards the JSON decoder
	// against unbounded reads).
	httpMaxBodyBytes = 1 << 20 // 1 MiB
)

// httpError writes a plain-text error with the given status code. Errors are
// intentionally terse: the HTTP surface is unauthenticated before the token
// check, so responses must not leak internals.
func httpError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintln(w, msg)
}

// ServeHTTPMode runs the MCP server over HTTP on addr (e.g. "127.0.0.1:8082")
// until the listener fails. token must be non-empty; RunHTTPMode refuses to
// start without one because an HTTP listener is reachable by any local
// process (and, with a non-loopback bind, by the network).
func ServeHTTPMode(addr, token string) error {
	if addr == "" {
		return fmt.Errorf("HTTP mode requires a listen address")
	}
	if token == "" {
		return fmt.Errorf("HTTP mode requires a token: set --token or AFLARE_MCP_TOKEN")
	}

	s := &Server{authToken: token}
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleMCPJSONRPC)
	mux.HandleFunc("/v1/call", s.handleV1Call)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       0, // tool calls may stream long-running workflows
		WriteTimeout:      0,
	}
	return srv.ListenAndServe()
}

// handleMCPJSONRPC serves POST /mcp: one JSON-RPC 2.0 request per POST body,
// answered with one JSON-RPC response — the same messages the stdio
// transport exchanges, minus the line framing.
func (s *Server) handleMCPJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "method not allowed: use POST")
		return
	}
	if !s.authorizeHTTP(w, r) {
		return
	}

	body, ok := s.readBody(w, r)
	if !ok {
		return
	}

	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeRPC(w, http.StatusOK, &rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32700, Message: "Parse error"},
		})
		return
	}

	// Notifications (no id) get no body, per JSON-RPC 2.0.
	resp := s.handleRequest(&req)
	if resp == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.writeRPC(w, http.StatusOK, resp)
}

// v1CallRequest is the simplified call body for POST /v1/call:
// {"name": "workflow_run", "arguments": {"file": "test.yaml"}}.
type v1CallRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// handleV1Call serves POST /v1/call: a direct tool call without the JSON-RPC
// envelope. The response body is the inner toolCallResult JSON.
func (s *Server) handleV1Call(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed, "method not allowed: use POST")
		return
	}
	if !s.authorizeHTTP(w, r) {
		return
	}

	body, ok := s.readBody(w, r)
	if !ok {
		return
	}

	var call v1CallRequest
	if err := json.Unmarshal(body, &call); err != nil || call.Name == "" {
		httpError(w, http.StatusBadRequest, `invalid request: expected {"name": "...", "arguments": {...}}`)
		return
	}

	result, err := s.callTool(&toolCallParams{Name: call.Name, Arguments: call.Arguments})
	if err != nil {
		// Tool errors are reported as JSON-RPC errors with 200 status, so
		// clients can parse the message uniformly (same convention as /mcp).
		s.writeRPC(w, http.StatusOK, &rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32603, Message: err.Error()},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// authorizeHTTP enforces the X-MCP-Token header with a constant-time compare.
func (s *Server) authorizeHTTP(w http.ResponseWriter, r *http.Request) bool {
	if subtle.ConstantTimeCompare([]byte(r.Header.Get(httpTokenHeader)), []byte(s.authToken)) == 1 {
		return true
	}
	httpError(w, http.StatusUnauthorized, "unauthorized: invalid or missing X-MCP-Token")
	return false
}

// readBody reads and closes the request body with a hard size cap.
func (s *Server) readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, httpMaxBodyBytes+1))
	if err != nil {
		httpError(w, http.StatusBadRequest, "failed to read request body")
		return nil, false
	}
	if len(body) > httpMaxBodyBytes {
		httpError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return nil, false
	}
	return body, true
}

// writeRPC marshals resp as the HTTP response body.
func (s *Server) writeRPC(w http.ResponseWriter, status int, resp *rpcResponse) {
	writeJSON(w, status, resp)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to encode response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
