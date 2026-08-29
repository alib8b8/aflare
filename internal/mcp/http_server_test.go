// Copyright (c) 2026 aflare Contributors
//
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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newHTTPTestServer wires an authenticated MCP HTTP mux for tests.
func newHTTPTestServer(t *testing.T, token string) *httptest.Server {
	t.Helper()
	s := &Server{authToken: token}
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleMCPJSONRPC)
	mux.HandleFunc("/v1/call", s.handleV1Call)
	return httptest.NewServer(mux)
}

func postJSON(t *testing.T, url, token string, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-MCP-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestHTTPMCP_Initialize(t *testing.T) {
	ts := newHTTPTestServer(t, "secret")
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/mcp", "secret",
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var rpc struct {
		Result struct {
			ServerInfo struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		t.Fatal(err)
	}
	if rpc.Result.ServerInfo.Name != "aflare" {
		t.Errorf("serverInfo.name = %q, want aflare", rpc.Result.ServerInfo.Name)
	}
}

func TestHTTPMCP_ToolsList(t *testing.T) {
	ts := newHTTPTestServer(t, "secret")
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/mcp", "secret",
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var rpc struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		t.Fatal(err)
	}
	if len(rpc.Result.Tools) == 0 {
		t.Error("expected at least one tool over HTTP")
	}
}

func TestHTTPMCP_NotificationReturnsNoContent(t *testing.T) {
	ts := newHTTPTestServer(t, "secret")
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/mcp", "secret",
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204 for notification", resp.StatusCode)
	}
}

func TestHTTPMCP_ParseError(t *testing.T) {
	ts := newHTTPTestServer(t, "secret")
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/mcp", "secret", `{not json`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (JSON-RPC error envelope)", resp.StatusCode)
	}
	var rpc rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		t.Fatal(err)
	}
	if rpc.Error == nil || rpc.Error.Code != -32700 {
		t.Errorf("expected -32700 parse error, got %+v", rpc.Error)
	}
}

func TestHTTPMCP_AuthRequired(t *testing.T) {
	ts := newHTTPTestServer(t, "secret")
	defer ts.Close()

	cases := []struct {
		name  string
		token string
	}{
		{"missing token", ""},
		{"wrong token", "wrong"},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/mcp", func(t *testing.T) {
			resp := postJSON(t, ts.URL+"/mcp", tc.token, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("/mcp status = %d, want 401", resp.StatusCode)
			}
		})
		t.Run(tc.name+"/v1/call", func(t *testing.T) {
			resp := postJSON(t, ts.URL+"/v1/call", tc.token, `{"name":"node_list"}`)
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("/v1/call status = %d, want 401", resp.StatusCode)
			}
		})
	}
}

func TestHTTPMCP_MethodNotAllowed(t *testing.T) {
	ts := newHTTPTestServer(t, "secret")
	defer ts.Close()

	for _, path := range []string{"/mcp", "/v1/call"} {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
		req.Header.Set("X-MCP-Token", "secret")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("GET %s status = %d, want 405", path, resp.StatusCode)
		}
	}
}

func TestHTTPMCP_BodyTooLarge(t *testing.T) {
	ts := newHTTPTestServer(t, "secret")
	defer ts.Close()

	big := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","arguments":{"pad":"` +
		strings.Repeat("a", httpMaxBodyBytes) + `"}}}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", bytes.NewReader([]byte(big)))
	req.Header.Set("X-MCP-Token", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", resp.StatusCode)
	}
}

func TestHTTPV1Call_Success(t *testing.T) {
	ts := newHTTPTestServer(t, "secret")
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/v1/call", "secret",
		`{"name":"node_list","arguments":{}}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result toolCallResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 || !strings.Contains(strings.ToLower(result.Content[0].Text), "node") {
		t.Errorf("unexpected node_list result: %+v", result.Content)
	}
}

func TestHTTPV1Call_UnknownTool(t *testing.T) {
	ts := newHTTPTestServer(t, "secret")
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/v1/call", "secret", `{"name":"no_such_tool"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (error envelope)", resp.StatusCode)
	}
	var rpc rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		t.Fatal(err)
	}
	if rpc.Error == nil {
		t.Error("expected JSON-RPC error for unknown tool")
	}
}

func TestHTTPV1Call_InvalidBody(t *testing.T) {
	ts := newHTTPTestServer(t, "secret")
	defer ts.Close()

	cases := []struct {
		name string
		body string
	}{
		{"not json", `nope`},
		{"missing name", `{"arguments":{}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postJSON(t, ts.URL+"/v1/call", "secret", tc.body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestServeHTTPMode_RequiresTokenAndAddr(t *testing.T) {
	if err := ServeHTTPMode("127.0.0.1:0", ""); err == nil {
		t.Error("expected error for empty token")
	}
	if err := ServeHTTPMode("", "token"); err == nil {
		t.Error("expected error for empty address")
	}
}
