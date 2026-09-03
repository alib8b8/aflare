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
	"errors"
	"fmt"
	"net"
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

// TestSendMessage_PollRetriesTransient5xx verifies that one transient
// 502 on tasks/get no longer kills the whole delegation: the poll is an
// idempotent read, so it is retried and the delegation completes.
func TestSendMessage_PollRetriesTransient5xx(t *testing.T) {
	var polls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "message/send":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": 1,
				"result": map[string]any{
					"id":     "task-1",
					"status": map[string]any{"state": "working"},
				},
			})
		case "tasks/get":
			if polls.Add(1) == 1 {
				http.Error(w, "bad gateway", http.StatusBadGateway)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": 1,
				"result": map[string]any{
					"id": "task-1",
					"status": map[string]any{"state": "completed", "message": map[string]any{
						"role":  "agent",
						"parts": []map[string]any{{"kind": "text", "text": "recovered"}},
					}},
				},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": 1,
				"error": map[string]any{"code": -32601, "message": "Method not found"},
			})
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	def := AgentDef{Name: "r", Driver: DriverA2A, URL: srv.URL + "/"}
	out, err := SendMessage(context.Background(), def, Task{Prompt: "x"})
	if err != nil {
		t.Fatalf("SendMessage: %v (want transient 502 on poll to be retried)", err)
	}
	if !strings.Contains(out, "recovered") {
		t.Errorf("output = %q, want recovered task text", out)
	}
	if got := polls.Load(); got != 2 {
		t.Errorf("polls = %d, want 2 (first 502 then success)", got)
	}
}

// TestSendMessage_SubmitDoesNotRetryServerErrors verifies the submit
// retry policy: message/send is not idempotent, so a server-side 5xx
// (request was delivered) must NOT be retried.
func TestSendMessage_SubmitDoesNotRetryServerErrors(t *testing.T) {
	var sends atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method == "message/send" {
			sends.Add(1)
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"error": map[string]any{"code": -32601, "message": "Method not found"},
		})
	}))
	t.Cleanup(srv.Close)

	def := AgentDef{Name: "r", Driver: DriverA2A, URL: srv.URL + "/"}
	_, err := SendMessage(context.Background(), def, Task{Prompt: "x"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("err = %v, want HTTP 500 failure surfaced", err)
	}
	if got := sends.Load(); got != 1 {
		t.Errorf("send attempts = %d, want 1 (delivered submits must not be retried)", got)
	}
}

// TestSendMessage_SubmitRetriesDialErrors verifies the other half of
// the submit retry policy: a dial failure means the request never
// reached the agent, so the submit IS retried (three attempts total).
func TestSendMessage_SubmitRetriesDialErrors(t *testing.T) {
	// Reserve a port and close it: nothing listens, so every attempt
	// fails at connection establishment.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot reserve port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	origDelays := a2aRetryDelays
	a2aRetryDelays = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}
	t.Cleanup(func() { a2aRetryDelays = origDelays })

	def := AgentDef{Name: "r", Driver: DriverA2A, URL: fmt.Sprintf("http://127.0.0.1:%d/", port)}
	start := time.Now()
	_, err = SendMessage(context.Background(), def, Task{Prompt: "x"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("dial to dead port accepted, want error")
	}
	if !isA2ADialError(err) {
		t.Errorf("err = %v, want dial error classification", err)
	}
	// Three attempts with 10ms + 20ms backoff must take at least 30ms;
	// a single attempt would return almost instantly.
	if elapsed < 30*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 30ms (submit retried on dial failure)", elapsed)
	}
}

// TestA2AErrorClassification pins the retry classifiers: dial failures
// are pre-delivery (safe for submits); transport failures and 5xx are
// retryable only for idempotent reads; 4xx never are.
func TestA2AErrorClassification(t *testing.T) {
	dialErr := fmt.Errorf("message/send request failed: %w", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("refused")})
	readErr := fmt.Errorf("tasks/get request failed: %w", &net.OpError{Op: "read", Net: "tcp", Err: errors.New("reset")})
	http5xx := &a2aHTTPStatusError{Method: "tasks/get", Status: 502, Body: "bad gateway"}
	http4xx := &a2aHTTPStatusError{Method: "tasks/get", Status: 404, Body: "nope"}

	if !isA2ADialError(dialErr) {
		t.Error("dial error not classified as dial")
	}
	if isA2ADialError(readErr) || isA2ADialError(http5xx) || isA2ADialError(http4xx) {
		t.Error("non-dial errors classified as dial")
	}
	for _, err := range []error{dialErr, readErr, http5xx} {
		if !isA2ARetryableReadError(err) {
			t.Errorf("err = %v, want retryable read", err)
		}
	}
	if isA2ARetryableReadError(http4xx) {
		t.Error("HTTP 404 classified as retryable, want permanent")
	}
}

// TestSendMessage_TimeoutCancelsRemoteTask pins the failure-isolation
// contract for abandoned tasks: when the delegation deadline hits while
// the remote task is still working, aflare must stop waiting AND fire a
// best-effort tasks/cancel so the remote agent does not keep running.
func TestSendMessage_TimeoutCancelsRemoteTask(t *testing.T) {
	var canceled atomic.Value
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		result := map[string]any{}
		switch req.Method {
		case "message/send":
			result = map[string]any{"id": "t-timeout", "status": map[string]any{"state": "working"}}
		case "tasks/get":
			result = map[string]any{"id": "t-timeout", "status": map[string]any{"state": "working"}}
		case "tasks/cancel":
			canceled.Store(req.Params["id"])
			result = map[string]any{"id": "t-timeout", "status": map[string]any{"state": "canceled"}}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	def := AgentDef{Name: "slowpoke", Driver: DriverA2A, URL: srv.URL + "/"}
	_, err := SendMessage(context.Background(), def, Task{
		Prompt:  "long task",
		Timeout: 50 * time.Millisecond,
	})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want timeout error", err)
	}
	if !strings.Contains(err.Error(), "remote task canceled") {
		t.Errorf("error = %v, want cancel confirmation note", err)
	}
	if got, _ := canceled.Load().(string); got != "t-timeout" {
		t.Errorf("tasks/cancel received id = %v, want t-timeout", got)
	}
}
