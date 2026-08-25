// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​‌‌​‌​​‌‌​​‌​​‌‌​‌‌‌​​​‌​​‌‌‌‌‌​​‌‌​‌​‌​‌‌​‌‌​‌‌‌‌​‌​‌‌‌​‌​​​‌‌‌‌‌​​​​​​​​​​​​​​​​‌​​‌‌​​​‌‌‌‌​​‌‌⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation; either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package agentx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// a2aTestServer builds a fake A2A agent. sendMethod selects the JSON-RPC
// method it accepts ("message/send", "tasks/send"); the other one
// returns -32601 so the fallback path is exercised.
func a2aTestServer(t *testing.T, sendMethod string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":        "research-agent",
			"description": "test research agent",
			"url":         "http://example.invalid/",
			"skills":      []map[string]any{{"id": "s1", "name": "research"}},
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		switch req.Method {
		case sendMethod:
			calls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"id":        "task-1",
					"contextId": "ctx-1",
					"status":    map[string]any{"state": "completed"},
					"artifacts": []map[string]any{
						{"name": "report", "parts": []map[string]any{{"kind": "text", "text": "research findings"}}},
					},
				},
			})
		case "tasks/get":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"id":     "task-1",
					"status": map[string]any{"state": "completed"},
				},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"error":   map[string]any{"code": -32601, "message": "Method not found"},
			})
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestSendMessage_ModernMethod(t *testing.T) {
	srv, calls := a2aTestServer(t, "message/send")
	def := AgentDef{Name: "r", Driver: DriverA2A, URL: srv.URL + "/"}

	out, err := SendMessage(context.Background(), def, Task{Prompt: "research topic X"})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if !strings.Contains(out, "research findings") {
		t.Errorf("output = %q, want artifact text", out)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("send calls = %d, want 1 (modern method must not fall back)", got)
	}
}

func TestSendMessage_LegacyFallback(t *testing.T) {
	// Server only accepts the older draft method name: message/send must
	// fail with -32601 and the client must retry with tasks/send.
	srv, calls := a2aTestServer(t, "tasks/send")
	def := AgentDef{Name: "r", Driver: DriverA2A, URL: srv.URL + "/"}

	out, err := SendMessage(context.Background(), def, Task{Prompt: "research topic X"})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if !strings.Contains(out, "research findings") {
		t.Errorf("output = %q, want artifact text", out)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("send calls = %d, want exactly one successful legacy send", got)
	}
}

func TestSendMessage_AuthHeaderFromEnv(t *testing.T) {
	t.Setenv("AFLARE_TEST_A2A_KEY", "sekrit")
	var gotAuth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{
				"id": "task-1",
				"status": map[string]any{"state": "completed", "message": map[string]any{
					"role":  "agent",
					"parts": []map[string]any{{"kind": "text", "text": "done"}},
				}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	def := AgentDef{Name: "r", Driver: DriverA2A, URL: srv.URL + "/", APIKeyEnv: "AFLARE_TEST_A2A_KEY"}
	out, err := SendMessage(context.Background(), def, Task{Prompt: "x"})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if out != "done" {
		t.Errorf("output = %q, want status message text", out)
	}
	if auth, _ := gotAuth.Load().(string); auth != "Bearer sekrit" {
		t.Errorf("Authorization = %q, want Bearer sekrit", auth)
	}
}

func TestSendMessage_FailedTask(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{
				"id": "task-9",
				"status": map[string]any{"state": "failed", "message": map[string]any{
					"role":  "agent",
					"parts": []map[string]any{{"kind": "text", "text": "could not comply"}},
				}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	def := AgentDef{Name: "r", Driver: DriverA2A, URL: srv.URL + "/"}
	_, err := SendMessage(context.Background(), def, Task{Prompt: "x"})
	if err == nil || !strings.Contains(err.Error(), "could not comply") {
		t.Fatalf("err = %v, want failed task with message", err)
	}
}

func TestSendMessage_RejectedTask(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{
				"id":     "task-9",
				"status": map[string]any{"state": "rejected"},
			},
		})
	}))
	t.Cleanup(srv.Close)

	def := AgentDef{Name: "r", Driver: DriverA2A, URL: srv.URL + "/"}
	_, err := SendMessage(context.Background(), def, Task{Prompt: "x"})
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("err = %v, want rejected task error", err)
	}
}

func TestSendMessage_TimeoutWhileWorking(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "message/send" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": 1,
				"result": map[string]any{
					"id":     "task-1",
					"status": map[string]any{"state": "working"},
				},
			})
			return
		}
		// tasks/get: stays working forever.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{
				"id":     "task-1",
				"status": map[string]any{"state": "working"},
			},
		})
	}))
	t.Cleanup(srv.Close)

	def := AgentDef{Name: "r", Driver: DriverA2A, URL: srv.URL + "/"}
	start := time.Now()
	_, err := SendMessage(context.Background(), def, Task{Prompt: "x", Timeout: 3 * time.Second})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want timeout while polling", err)
	}
	if time.Since(start) > 15*time.Second {
		t.Fatalf("timeout not enforced promptly")
	}
}

func TestSendMessage_BadURLRejected(t *testing.T) {
	tests := []string{"ftp://example.com/", "http://user:pass@example.com/", "not a url", "http://example.com/#frag"}
	for _, raw := range tests {
		def := AgentDef{Name: "r", Driver: DriverA2A, URL: raw}
		if _, err := SendMessage(context.Background(), def, Task{Prompt: "x"}); err == nil {
			t.Errorf("URL %q accepted, want rejection", raw)
		}
	}
}

func TestSendMessage_EmptyPromptRejected(t *testing.T) {
	def := AgentDef{Name: "r", Driver: DriverA2A, URL: "http://127.0.0.1:1/"}
	if _, err := SendMessage(context.Background(), def, Task{Prompt: "  "}); err == nil {
		t.Fatal("empty prompt accepted, want rejection")
	}
}

func TestSendMessage_AuditFailClosed(t *testing.T) {
	def := AgentDef{Name: "r", Driver: DriverA2A, URL: "http://127.0.0.1:1/"}
	_, err := SendMessage(context.Background(), def, Task{
		Prompt: "x",
		Audit:  func(string) error { return context.Canceled },
	})
	if err == nil || !strings.Contains(err.Error(), "audit") {
		t.Fatalf("err = %v, want fail-closed audit error", err)
	}
}

func TestFetchAgentCard(t *testing.T) {
	srv, _ := a2aTestServer(t, "message/send")
	def := AgentDef{Name: "r", Driver: DriverA2A, URL: srv.URL + "/"}

	card, err := FetchAgentCard(context.Background(), def)
	if err != nil {
		t.Fatalf("FetchAgentCard: %v", err)
	}
	if card.Name != "research-agent" {
		t.Errorf("card.Name = %q", card.Name)
	}
	if len(card.Skills) != 1 || card.Skills[0].ID != "s1" {
		t.Errorf("card.Skills = %+v", card.Skills)
	}
}

func TestFetchAgentCard_NoCardAnywhere(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	def := AgentDef{Name: "r", Driver: DriverA2A, URL: srv.URL + "/"}
	if _, err := FetchAgentCard(context.Background(), def); err == nil {
		t.Fatal("missing agent card accepted, want error")
	}
}
