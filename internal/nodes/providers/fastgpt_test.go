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

package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

func TestFastGPTExecute(t *testing.T) {
	t.Setenv("FASTGPT_API_KEY", "")
	t.Setenv("FASTGPT_BASE_URL", "")

	var gotAuth, gotPath string
	var gotBody fastGPTRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("bad request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		resp := fastGPTResponse{}
		resp.Choices = append(resp.Choices, fastGPTChoice{})
		resp.Choices[0].Message.Content = "FastGPT says hi"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	node := &FastGPTNode{}
	out, err := node.Execute(context.Background(), "hello", map[string]string{
		"api_key":  "sk-test",
		"app_id":   "app-1",
		"chat_id":  "chat-9",
		"endpoint": srv.URL,
		"system":   "be brief",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if out != "FastGPT says hi" {
		t.Errorf("output = %q, want %q", out, "FastGPT says hi")
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("Authorization = %q, want Bearer sk-test", gotAuth)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path = %q, want /chat/completions", gotPath)
	}
	if gotBody.AppId != "app-1" || gotBody.ChatId != "chat-9" {
		t.Errorf("appId/chatId = %q/%q, want app-1/chat-9", gotBody.AppId, gotBody.ChatId)
	}
	if gotBody.Stream {
		t.Error("non-streaming Execute must send stream=false")
	}
	if len(gotBody.Messages) != 2 || gotBody.Messages[0].Role != "system" || gotBody.Messages[1].Role != "user" {
		t.Errorf("messages = %+v, want [system, user]", gotBody.Messages)
	}
}

func TestFastGPTExecute_APIKeyFromEnv(t *testing.T) {
	t.Setenv("FASTGPT_API_KEY", "sk-env")
	t.Setenv("FASTGPT_BASE_URL", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-env" {
			t.Errorf("Authorization = %q, want Bearer sk-env", r.Header.Get("Authorization"))
		}
		resp := fastGPTResponse{}
		resp.Choices = append(resp.Choices, fastGPTChoice{})
		resp.Choices[0].Message.Content = "ok"
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	node := &FastGPTNode{}
	if _, err := node.Execute(context.Background(), "hi", map[string]string{"endpoint": srv.URL}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

func TestFastGPTExecute_EndpointFromEnv(t *testing.T) {
	t.Setenv("FASTGPT_API_KEY", "sk-env")
	t.Setenv("FASTGPT_BASE_URL", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := fastGPTResponse{}
		resp.Choices = append(resp.Choices, fastGPTChoice{})
		resp.Choices[0].Message.Content = "ok"
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	t.Setenv("FASTGPT_BASE_URL", srv.URL)

	node := &FastGPTNode{}
	if _, err := node.Execute(context.Background(), "hi", nil); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

func TestFastGPTExecute_NoAPIKey(t *testing.T) {
	t.Setenv("FASTGPT_API_KEY", "")
	t.Setenv("FASTGPT_BASE_URL", "")

	node := &FastGPTNode{}
	_, err := node.Execute(context.Background(), "hi", map[string]string{"endpoint": "http://localhost:1"})
	if err == nil || !strings.Contains(err.Error(), "API key required") {
		t.Fatalf("expected API key error, got %v", err)
	}
}

func TestFastGPTExecute_InvalidEndpoint(t *testing.T) {
	t.Setenv("FASTGPT_API_KEY", "sk-test")
	t.Setenv("FASTGPT_BASE_URL", "")

	node := &FastGPTNode{}
	for _, endpoint := range []string{
		"ftp://example.com",               // scheme
		"http://user:pass@localhost:9/v1", // userinfo
		"http://192.168.1.1:9/v1",         // private IP
	} {
		_, err := node.Execute(context.Background(), "hi", map[string]string{"endpoint": endpoint})
		if err == nil {
			t.Errorf("endpoint %q should be rejected", endpoint)
		}
	}
}

func TestFastGPTExecute_APIError(t *testing.T) {
	t.Setenv("FASTGPT_API_KEY", "")
	t.Setenv("FASTGPT_BASE_URL", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()

	node := &FastGPTNode{}
	_, err := node.Execute(context.Background(), "hi", map[string]string{
		"api_key":  "sk-bad",
		"endpoint": srv.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("expected API error message, got %v", err)
	}
}

func TestFastGPTExecute_StatusErrorNoBody(t *testing.T) {
	t.Setenv("FASTGPT_API_KEY", "")
	t.Setenv("FASTGPT_BASE_URL", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	node := &FastGPTNode{}
	_, err := node.Execute(context.Background(), "hi", map[string]string{
		"api_key":  "sk-test",
		"endpoint": srv.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected status 500 error, got %v", err)
	}
}

func TestFastGPTExecute_NoChoices(t *testing.T) {
	t.Setenv("FASTGPT_API_KEY", "")
	t.Setenv("FASTGPT_BASE_URL", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	node := &FastGPTNode{}
	_, err := node.Execute(context.Background(), "hi", map[string]string{
		"api_key":  "sk-test",
		"endpoint": srv.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Fatalf("expected no-choices error, got %v", err)
	}
}

func TestFastGPTExecute_BadJSON(t *testing.T) {
	t.Setenv("FASTGPT_API_KEY", "")
	t.Setenv("FASTGPT_BASE_URL", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	node := &FastGPTNode{}
	_, err := node.Execute(context.Background(), "hi", map[string]string{
		"api_key":  "sk-test",
		"endpoint": srv.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestFastGPTExecuteStream(t *testing.T) {
	t.Setenv("FASTGPT_API_KEY", "")
	t.Setenv("FASTGPT_BASE_URL", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body fastGPTRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("bad request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !body.Stream {
			t.Error("ExecuteStream must send stream=true")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo \"}}]}\n\n"))
		w.Write([]byte("data: not-json\n\n"))                       // skipped
		w.Write([]byte("data: {\"choices\":[{\"delta\":{}}]}\n\n")) // empty delta skipped
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	var chunks []string
	node := &FastGPTNode{}
	out, err := node.ExecuteStream(context.Background(), "hi", map[string]string{
		"api_key":  "sk-test",
		"endpoint": srv.URL,
	}, func(chunk string) { chunks = append(chunks, chunk) })
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}
	if out != "Hello " {
		t.Errorf("streamed output = %q, want %q", out, "Hello ")
	}
	if len(chunks) != 2 || chunks[0] != "Hel" || chunks[1] != "lo " {
		t.Errorf("chunks = %v, want [Hel lo ]", chunks)
	}
}

func TestFastGPTExecuteStream_NilOnChunk(t *testing.T) {
	t.Setenv("FASTGPT_API_KEY", "")
	t.Setenv("FASTGPT_BASE_URL", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	node := &FastGPTNode{}
	out, err := node.ExecuteStream(context.Background(), "hi", map[string]string{
		"api_key":  "sk-test",
		"endpoint": srv.URL,
	}, nil)
	if err != nil {
		t.Fatalf("ExecuteStream with nil onChunk failed: %v", err)
	}
	if out != "a" {
		t.Errorf("streamed output = %q, want a", out)
	}
}

// TestFastGPTRegisteredInCore guards the init() registration.
func TestFastGPTRegisteredInCore(t *testing.T) {
	node, ok := core.Get("fastgpt")
	if !ok {
		t.Fatal("fastgpt node not found in registry")
	}
	if node.Name() != "fastgpt" {
		t.Errorf("Name() = %q, want fastgpt", node.Name())
	}
}
