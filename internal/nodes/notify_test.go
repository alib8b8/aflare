// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​​​​​‌​‌‌​‌‌‌​​‌‌‌​​‌‌​​‌‌​​​‌​‌​‌‌​​​​​​‌‌​‌‌​​​​​​​​​​​​​​​​​​​​‌‌‌‌‌‌‌‌‌​​‌⁠
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

package nodes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setupTestServer(handler http.HandlerFunc) (*httptest.Server, func()) {
	server := httptest.NewTLSServer(handler)
	oldClient := notifyHTTPClient
	oldValidator := notifyURLValidator
	notifyHTTPClient = server.Client()
	notifyURLValidator = func(string) error { return nil }
	return server, func() {
		server.Close()
		notifyHTTPClient = oldClient
		notifyURLValidator = oldValidator
	}
}

func TestNotifyNode_Stdout(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "hello", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotifyNode_Stderr(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "hello", map[string]string{"channel": "stderr"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotifyNode_Slack(t *testing.T) {
	var gotBody []byte
	var gotContentType string
	server, cleanup := setupTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer cleanup()

	ctx := context.Background()
	node := &NotifyNode{}
	msg, err := node.Execute(ctx, "hello slack", map[string]string{
		"channel": "slack",
		"url":     server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg != "hello slack" {
		t.Errorf("expected message returned, got %q", msg)
	}

	var payload map[string]string
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if payload["text"] != "hello slack" {
		t.Errorf("expected text 'hello slack', got %q", payload["text"])
	}
	if gotContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", gotContentType)
	}
}

func TestNotifyNode_SlackMissingURL(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "test", map[string]string{"channel": "slack"})
	if err == nil {
		t.Error("expected error for missing slack URL")
	}
}

func TestNotifyNode_SlackServerError(t *testing.T) {
	server, cleanup := setupTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer cleanup()

	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "test", map[string]string{"channel": "slack", "url": server.URL})
	if err == nil {
		t.Error("expected error for slack server error")
	}
}

func TestNotifyNode_Discord(t *testing.T) {
	var gotBody []byte
	server, cleanup := setupTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer cleanup()

	ctx := context.Background()
	node := &NotifyNode{}
	msg, err := node.Execute(ctx, "hello discord", map[string]string{
		"channel":  "discord",
		"url":      server.URL,
		"username": "Bot",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg != "hello discord" {
		t.Errorf("expected message returned, got %q", msg)
	}

	var payload map[string]string
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if payload["content"] != "hello discord" {
		t.Errorf("expected content 'hello discord', got %q", payload["content"])
	}
	if payload["username"] != "Bot" {
		t.Errorf("expected username 'Bot', got %q", payload["username"])
	}
}

func TestNotifyNode_DiscordMissingURL(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "test", map[string]string{"channel": "discord"})
	if err == nil {
		t.Error("expected error for missing discord URL")
	}
}

func TestNotifyNode_Telegram(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "hello telegram", map[string]string{
		"channel": "telegram",
		"token":   "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
		"chat_id": "12345",
	})
	// Will fail because api.telegram.org is unreachable/real network
	if err == nil {
		t.Error("expected error for real telegram endpoint without network")
	}
}

func TestNotifyNode_TelegramMissingToken(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "test", map[string]string{"channel": "telegram", "chat_id": "1"})
	if err == nil {
		t.Error("expected error for missing telegram token")
	}
}

func TestNotifyNode_TelegramMissingChatID(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "test", map[string]string{"channel": "telegram", "token": "abc"})
	if err == nil {
		t.Error("expected error for missing telegram chat_id")
	}
}

// TestNotifyNode_CNGroupBots covers the feishu/dingtalk/wecom channels over a
// local TLS test server: payload shape per platform protocol, success path,
// missing url, and — critically — the HTTP-200-with-error-code case (all three
// platforms answer 200 even for an invalid token, so the body's errcode/code
// is the only failure signal).
func TestNotifyNode_CNGroupBots(t *testing.T) {
	cases := []struct {
		channel   string
		respBody  string
		checkBody func(t *testing.T, body []byte)
	}{
		{
			channel:  "feishu",
			respBody: `{"code":0,"msg":"success"}`,
			checkBody: func(t *testing.T, body []byte) {
				var p struct {
					MsgType string `json:"msg_type"`
					Content struct {
						Text string `json:"text"`
					} `json:"content"`
				}
				if err := json.Unmarshal(body, &p); err != nil {
					t.Fatalf("invalid feishu payload: %v", err)
				}
				if p.MsgType != "text" || p.Content.Text != "你好，茅台 1400" {
					t.Errorf("unexpected feishu payload: %s", body)
				}
			},
		},
		{
			channel:  "dingtalk",
			respBody: `{"errcode":0,"errmsg":"ok"}`,
			checkBody: func(t *testing.T, body []byte) {
				var p struct {
					Msgtype string `json:"msgtype"`
					Text    struct {
						Content string `json:"content"`
					} `json:"text"`
				}
				if err := json.Unmarshal(body, &p); err != nil {
					t.Fatalf("invalid dingtalk payload: %v", err)
				}
				if p.Msgtype != "text" || p.Text.Content != "你好，茅台 1400" {
					t.Errorf("unexpected dingtalk payload: %s", body)
				}
			},
		},
		{
			channel:  "wecom",
			respBody: `{"errcode":0,"errmsg":"ok"}`,
			checkBody: func(t *testing.T, body []byte) {
				var p struct {
					Msgtype string `json:"msgtype"`
					Text    struct {
						Content string `json:"content"`
					} `json:"text"`
				}
				if err := json.Unmarshal(body, &p); err != nil {
					t.Fatalf("invalid wecom payload: %v", err)
				}
				if p.Msgtype != "text" || p.Text.Content != "你好，茅台 1400" {
					t.Errorf("unexpected wecom payload: %s", body)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.channel, func(t *testing.T) {
			var gotBody []byte
			var gotContentType string
			server, cleanup := setupTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotBody, _ = io.ReadAll(r.Body)
				gotContentType = r.Header.Get("Content-Type")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tc.respBody))
			}))
			defer cleanup()

			ctx := context.Background()
			node := &NotifyNode{}
			msg, err := node.Execute(ctx, "你好，茅台 1400", map[string]string{
				"channel": tc.channel,
				"url":     server.URL,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if msg != "你好，茅台 1400" {
				t.Errorf("expected message returned, got %q", msg)
			}
			if gotContentType != "application/json" {
				t.Errorf("expected Content-Type application/json, got %q", gotContentType)
			}
			tc.checkBody(t, gotBody)
		})
	}
}

func TestNotifyNode_CNGroupBotsMissingURL(t *testing.T) {
	for _, channel := range []string{"feishu", "dingtalk", "wecom"} {
		_, err := (&NotifyNode{}).Execute(context.Background(), "test", map[string]string{"channel": channel})
		if err == nil {
			t.Errorf("expected error for missing %s url", channel)
		}
	}
}

func TestNotifyNode_CNGroupBotsAPIError(t *testing.T) {
	// All three platforms return HTTP 200 with a non-zero error code for an
	// invalid token — the send must fail, not silently report success.
	cases := []struct {
		name     string
		channel  string
		respBody string
	}{
		{"dingtalk-errcode", "dingtalk", `{"errcode":310000,"errmsg":"signature not match"}`},
		{"wecom-errcode", "wecom", `{"errcode":93003,"errmsg":"invalid webhook url"}`},
		{"feishu-code", "feishu", `{"code":19021,"msg":"sign match fail"}`},
		{"feishu-legacy-statuscode", "feishu", `{"StatusCode":1024,"StatusMessage":"bad request"}`},
		{"non-json-body", "feishu", `<html>proxy error page</html>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, cleanup := setupTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tc.respBody))
			}))
			defer cleanup()
			_, err := (&NotifyNode{}).Execute(context.Background(), "test", map[string]string{
				"channel": tc.channel,
				"url":     server.URL,
			})
			if err == nil {
				t.Errorf("expected error for %s with body %s", tc.channel, tc.respBody)
			}
		})
	}
}

func TestNotifyNode_CNGroupBotsHTTPError(t *testing.T) {
	server, cleanup := setupTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errcode":0}`))
	}))
	defer cleanup()
	_, err := (&NotifyNode{}).Execute(context.Background(), "test", map[string]string{
		"channel": "dingtalk",
		"url":     server.URL,
	})
	if err == nil {
		t.Error("expected error for HTTP 500 even with errcode 0 in body")
	}
}

func TestNotifyNode_WebhookPost(t *testing.T) {
	var gotBody []byte
	var gotMethod string
	var gotHeader string
	server, cleanup := setupTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotBody, _ = io.ReadAll(r.Body)
		gotHeader = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
	}))
	defer cleanup()

	ctx := context.Background()
	node := &NotifyNode{}
	msg, err := node.Execute(ctx, "hello webhook", map[string]string{
		"channel": "webhook",
		"url":     server.URL,
		"headers": `{"X-Custom":"value"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg != "hello webhook" {
		t.Errorf("expected message returned, got %q", msg)
	}
	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotHeader != "value" {
		t.Errorf("expected X-Custom header 'value', got %q", gotHeader)
	}

	var payload map[string]string
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if payload["message"] != "hello webhook" {
		t.Errorf("expected message 'hello webhook', got %q", payload["message"])
	}
}

func TestNotifyNode_WebhookGet(t *testing.T) {
	var gotMethod string
	server, cleanup := setupTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer cleanup()

	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "hello", map[string]string{
		"channel": "webhook",
		"url":     server.URL,
		"method":  "GET",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "GET" {
		t.Errorf("expected GET, got %s", gotMethod)
	}
}

func TestNotifyNode_WebhookCustomBody(t *testing.T) {
	var gotBody []byte
	server, cleanup := setupTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer cleanup()

	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "ignored", map[string]string{
		"channel": "webhook",
		"url":     server.URL,
		"body":    `{"custom":"data"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]string
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if payload["custom"] != "data" {
		t.Errorf("expected custom body, got %q", string(gotBody))
	}
}

func TestNotifyNode_WebhookDeprecatedURL(t *testing.T) {
	var gotBody []byte
	server, cleanup := setupTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer cleanup()

	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "legacy", map[string]string{
		"channel":     "webhook",
		"webhook_url": server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]string
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if payload["text"] != "legacy" {
		t.Errorf("expected text 'legacy', got %q", payload["text"])
	}
}

func TestNotifyNode_WebhookMissingURL(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "test", map[string]string{"channel": "webhook"})
	if err == nil {
		t.Error("expected error for missing webhook URL")
	}
}

func TestNotifyNode_WebhookInvalidMethod(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "test", map[string]string{
		"channel": "webhook",
		"url":     "https://example.com",
		"method":  "DELETE",
	})
	if err == nil {
		t.Error("expected error for invalid webhook method")
	}
}

func TestNotifyNode_WebhookServerError(t *testing.T) {
	server, cleanup := setupTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer cleanup()

	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "test", map[string]string{"channel": "webhook", "url": server.URL})
	if err == nil {
		t.Error("expected error for webhook server error")
	}
}

func TestNotifyNode_WebhookInvalidHeaders(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "test", map[string]string{
		"channel": "webhook",
		"url":     "https://example.com",
		"headers": "not-json",
	})
	if err == nil {
		t.Error("expected error for invalid headers JSON")
	}
}

func TestNotifyNode_WebhookSizeLimit(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	largeBody := strings.Repeat("a", maxNotifyBodySize+1)
	_, err := node.Execute(ctx, "test", map[string]string{
		"channel": "webhook",
		"url":     "https://example.com",
		"body":    largeBody,
	})
	if err == nil {
		t.Error("expected error for payload exceeding size limit")
	}
}

func TestNotifyNode_SlackSizeLimit(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	largeMessage := strings.Repeat("a", maxNotifyBodySize+1)
	_, err := node.Execute(ctx, largeMessage, map[string]string{
		"channel": "slack",
		"url":     "https://example.com",
	})
	if err == nil {
		t.Error("expected error for slack payload exceeding size limit")
	}
}

func TestNotifyNode_URLValidation_HTTP(t *testing.T) {
	oldValidator := notifyURLValidator
	defer func() { notifyURLValidator = oldValidator }()
	notifyURLValidator = validateNotifyURL

	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "test", map[string]string{
		"channel": "webhook",
		"url":     "http://example.com",
	})
	if err == nil {
		t.Error("expected error for HTTP URL")
	}
}

func TestNotifyNode_URLValidation_Localhost(t *testing.T) {
	oldValidator := notifyURLValidator
	defer func() { notifyURLValidator = oldValidator }()
	notifyURLValidator = validateNotifyURL

	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "test", map[string]string{
		"channel": "webhook",
		"url":     "https://localhost:8080/hook",
	})
	if err == nil {
		t.Error("expected error for localhost URL")
	}
}

func TestNotifyNode_URLValidation_PrivateIP(t *testing.T) {
	oldValidator := notifyURLValidator
	defer func() { notifyURLValidator = oldValidator }()
	notifyURLValidator = validateNotifyURL

	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "test", map[string]string{
		"channel": "webhook",
		"url":     "https://192.168.1.1/hook",
	})
	if err == nil {
		t.Error("expected error for private IP URL")
	}
}

func TestNotifyNode_UnknownChannel(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	_, err := node.Execute(ctx, "test", map[string]string{"channel": "unknown"})
	if err == nil {
		t.Error("expected error for unknown channel")
	}
}
