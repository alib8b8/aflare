// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​‌‌‌‌​‌‌‌‌​‌‌‌​​‌​‌​​‌‌​​‌‌​‌‌​​‌​​‌​​‌​​‌‌​​‌​​​​​​​​​​​​​​​​‌​‌​‌‌‌​‌‌​‌‌​‌​⁠
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

package core

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- B-1: protocol field roundtrips ---

func TestLLMRequest_B1Fields_JSONRoundtrip(t *testing.T) {
	seed := 42
	req := LLMRequest{
		Model:            "gpt-test",
		Messages:         []LLMMessage{{Role: "user", Content: "hi"}},
		TopP:             0.9,
		FrequencyPenalty: 0.5,
		PresencePenalty:  -0.3,
		Stop:             []string{"\n", "END"},
		Seed:             &seed,
		ResponseFormat:   &ResponseFormat{Type: "json_object"},
		Tools: []ToolDefinition{{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_weather",
				Description: "Get weather",
				Parameters:  map[string]interface{}{"type": "object"},
			},
		}},
		ToolChoice: json.RawMessage(`"auto"`),
		User:       "user-123",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got LLMRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.TopP != 0.9 || got.FrequencyPenalty != 0.5 || got.PresencePenalty != -0.3 {
		t.Errorf("numeric fields mismatch: %+v", got)
	}
	if len(got.Stop) != 2 || got.Stop[0] != "\n" || got.Stop[1] != "END" {
		t.Errorf("Stop mismatch: %v", got.Stop)
	}
	if got.Seed == nil || *got.Seed != 42 {
		t.Errorf("Seed mismatch: %v", got.Seed)
	}
	if got.ResponseFormat == nil || got.ResponseFormat.Type != "json_object" {
		t.Errorf("ResponseFormat mismatch: %+v", got.ResponseFormat)
	}
	if len(got.Tools) != 1 || got.Tools[0].Function.Name != "get_weather" {
		t.Errorf("Tools mismatch: %+v", got.Tools)
	}
	if string(got.ToolChoice) != `"auto"` {
		t.Errorf("ToolChoice mismatch: %s", got.ToolChoice)
	}
	if got.User != "user-123" {
		t.Errorf("User mismatch: %q", got.User)
	}
}

func TestLLMRequest_BackwardCompat_EmptyB1FieldsOmitted(t *testing.T) {
	// A pre-B-1 caller constructs only the legacy fields. The serialized
	// JSON must not contain any of the new B-1 keys, so older providers
	// see byte-identical requests to before the upgrade.
	req := LLMRequest{
		Model:    "m",
		Messages: []LLMMessage{{Role: "user", Content: "hi"}},
		Stream:   false,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, key := range []string{`"top_p"`, `"frequency_penalty"`, `"presence_penalty"`, `"stop":`, `"seed":`, `"response_format"`, `"tools":`, `"tool_choice":`, `"user":`} {
		if strings.Contains(string(data), key) {
			t.Errorf("legacy request should not contain %s, got: %s", key, data)
		}
	}
}

func TestLLMResponse_Usage_ParseAndNil(t *testing.T) {
	// With usage.
	raw := `{"choices":[{"message":{"content":"hi"}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`
	var resp LLMResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if resp.Usage == nil {
		t.Fatal("expected non-nil Usage")
	}
	if resp.Usage.PromptTokens != 10 || resp.Usage.CompletionTokens != 5 || resp.Usage.TotalTokens != 15 {
		t.Errorf("Usage mismatch: %+v", resp.Usage)
	}

	// Without usage (older provider).
	rawNoUsage := `{"choices":[{"message":{"content":"hi"}}]}`
	var resp2 LLMResponse
	if err := json.Unmarshal([]byte(rawNoUsage), &resp2); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if resp2.Usage != nil {
		t.Errorf("expected nil Usage when absent, got %+v", resp2.Usage)
	}
}

func TestLLMResponse_ToolCalls_Parse(t *testing.T) {
	raw := `{"choices":[{"message":{"content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"SF\"}"}}]}}]}`
	var resp LLMResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	tc := resp.Choices[0].Message.ToolCalls
	if len(tc) != 1 || tc[0].ID != "call_1" || tc[0].Function.Name != "get_weather" {
		t.Errorf("ToolCalls mismatch: %+v", tc)
	}
	if tc[0].Function.Arguments != `{"city":"SF"}` {
		t.Errorf("Arguments mismatch: %q", tc[0].Function.Arguments)
	}
}

// --- B-1: applyLLMRequestParams ---

func TestApplyLLMRequestParams_AllFields(t *testing.T) {
	req := &LLMRequest{}
	params := map[string]string{
		"temperature":       "0.7",
		"max_tokens":        "100",
		"top_p":             "0.9",
		"frequency_penalty": "0.5",
		"presence_penalty":  "-0.3",
		"stop":              "\n,END",
		"seed":              "42",
		"response_format":   "json_object",
		"tools":             `[{"type":"function","function":{"name":"f","description":"d","parameters":{"type":"object"}}}]`,
		"tool_choice":       "auto",
		"user":              "u1",
	}
	if err := applyLLMRequestParams(req, params); err != nil {
		t.Fatalf("applyLLMRequestParams: %v", err)
	}
	if req.Temperature != 0.7 {
		t.Errorf("Temperature=%g", req.Temperature)
	}
	if req.MaxTokens != 100 {
		t.Errorf("MaxTokens=%d", req.MaxTokens)
	}
	if req.TopP != 0.9 {
		t.Errorf("TopP=%g", req.TopP)
	}
	if req.FrequencyPenalty != 0.5 {
		t.Errorf("FrequencyPenalty=%g", req.FrequencyPenalty)
	}
	if req.PresencePenalty != -0.3 {
		t.Errorf("PresencePenalty=%g", req.PresencePenalty)
	}
	if len(req.Stop) != 2 || req.Stop[0] != "\n" || req.Stop[1] != "END" {
		t.Errorf("Stop=%v", req.Stop)
	}
	if req.Seed == nil || *req.Seed != 42 {
		t.Errorf("Seed=%v", req.Seed)
	}
	if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_object" {
		t.Errorf("ResponseFormat=%+v", req.ResponseFormat)
	}
	if len(req.Tools) != 1 || req.Tools[0].Function.Name != "f" {
		t.Errorf("Tools=%+v", req.Tools)
	}
	if string(req.ToolChoice) != `"auto"` {
		t.Errorf("ToolChoice=%s", req.ToolChoice)
	}
	if req.User != "u1" {
		t.Errorf("User=%q", req.User)
	}
}

func TestApplyLLMRequestParams_EmptyParamsNoOp(t *testing.T) {
	// Empty params map → no fields set, no error (backward compat).
	req := &LLMRequest{}
	if err := applyLLMRequestParams(req, map[string]string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.TopP != 0 || req.MaxTokens != 0 || req.Seed != nil || req.ResponseFormat != nil {
		t.Errorf("expected zero values, got %+v", req)
	}
}

func TestApplyLLMRequestParams_InvalidNumeric(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]string
		want   string
	}{
		{"bad temperature", map[string]string{"temperature": "abc"}, "temperature"},
		{"temp out of range", map[string]string{"temperature": "3"}, "temperature"},
		{"temp negative", map[string]string{"temperature": "-0.1"}, "temperature"},
		{"bad max_tokens", map[string]string{"max_tokens": "x"}, "max_tokens"},
		{"max_tokens zero", map[string]string{"max_tokens": "0"}, "max_tokens"},
		{"top_p out of range", map[string]string{"top_p": "1.5"}, "top_p"},
		{"freq penalty out of range", map[string]string{"frequency_penalty": "3"}, "frequency_penalty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := applyLLMRequestParams(&LLMRequest{}, c.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q should contain %q", err, c.want)
			}
		})
	}
}

func TestApplyLLMRequestParams_ResponseFormatVariants(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantErr  bool
		wantType string
	}{
		{"json_object keyword", "json_object", false, "json_object"},
		{"raw json object", `{"type":"json_object"}`, false, "json_object"},
		{"json_schema form", `json_schema:{"name":"s","schema":{"type":"object"}}`, false, "json_schema"},
		{"empty", "", false, ""}, // no error, returns nil
		{"unknown keyword", "yaml_object", true, ""},
		{"bad json_schema payload", "json_schema:not json", true, ""},
		{"raw json missing type", `{"foo":"bar"}`, true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := &LLMRequest{}
			err := applyLLMRequestParams(req, map[string]string{"response_format": c.input})
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.wantType == "" {
				if req.ResponseFormat != nil {
					t.Errorf("expected nil ResponseFormat, got %+v", req.ResponseFormat)
				}
				return
			}
			if req.ResponseFormat == nil || req.ResponseFormat.Type != c.wantType {
				t.Errorf("ResponseFormat.Type=%+v want %q", req.ResponseFormat, c.wantType)
			}
		})
	}
}

func TestApplyLLMRequestParams_ToolsInvalidJSON(t *testing.T) {
	err := applyLLMRequestParams(&LLMRequest{}, map[string]string{
		"tools": "not json",
	})
	if err == nil {
		t.Fatal("expected error for malformed tools JSON")
	}
	if !strings.Contains(err.Error(), "tools") {
		t.Errorf("error should mention tools: %v", err)
	}
}

func TestApplyLLMRequestParams_ToolsEmptyArray(t *testing.T) {
	err := applyLLMRequestParams(&LLMRequest{}, map[string]string{
		"tools": "[]",
	})
	if err == nil {
		t.Fatal("expected error for empty tools array")
	}
}

func TestApplyLLMRequestParams_ToolChoiceVariants(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"none", "none", `"none"`},
		{"auto", "auto", `"auto"`},
		{"object", `{"type":"function","function":{"name":"f"}}`, `{"type":"function","function":{"name":"f"}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := &LLMRequest{}
			err := applyLLMRequestParams(req, map[string]string{"tool_choice": c.input})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(req.ToolChoice) != c.want {
				t.Errorf("ToolChoice=%s want %s", req.ToolChoice, c.want)
			}
		})
	}
}

func TestApplyLLMRequestParams_ToolChoiceInvalidJSON(t *testing.T) {
	err := applyLLMRequestParams(&LLMRequest{}, map[string]string{
		"tool_choice": "not json not keyword",
	})
	if err == nil {
		t.Fatal("expected error for malformed tool_choice")
	}
}

// --- B-1: end-to-end via mock server ---

func TestOpenAICompatibleNode_Execute_B1ParamsReachServer(t *testing.T) {
	var gotBody LLMRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Errorf("unmarshal request: %v", err)
		}
		resp := LLMResponse{
			Choices: []LLMChoice{{Message: LLMChoiceMessage{Content: "ok"}}},
			Usage:   &LLMUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := LLMNodeConfig{
		Name:            "testnode",
		DefaultModel:    "test-model",
		DefaultEndpoint: "http://localhost:11434",
		EnvAPIKey:       "AFLARE_TEST_UNIQUE_API_KEY_NEVER_SET",
		ProviderName:    "TestProvider",
	}
	n := NewOpenAICompatibleNode(cfg)
	_, err := n.Execute(context.Background(), "hi", map[string]string{
		"api_key":         "sk-test",
		"endpoint":        srv.URL,
		"temperature":     "0.5",
		"max_tokens":      "50",
		"top_p":           "0.9",
		"seed":            "7",
		"response_format": "json_object",
		"stop":            "END",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody.Temperature != 0.5 {
		t.Errorf("server received Temperature=%g want 0.5", gotBody.Temperature)
	}
	if gotBody.MaxTokens != 50 {
		t.Errorf("server received MaxTokens=%d want 50", gotBody.MaxTokens)
	}
	if gotBody.TopP != 0.9 {
		t.Errorf("server received TopP=%g want 0.9", gotBody.TopP)
	}
	if gotBody.Seed == nil || *gotBody.Seed != 7 {
		t.Errorf("server received Seed=%v want 7", gotBody.Seed)
	}
	if gotBody.ResponseFormat == nil || gotBody.ResponseFormat.Type != "json_object" {
		t.Errorf("server received ResponseFormat=%+v want json_object", gotBody.ResponseFormat)
	}
	if len(gotBody.Stop) != 1 || gotBody.Stop[0] != "END" {
		t.Errorf("server received Stop=%v want [END]", gotBody.Stop)
	}
}

func TestOpenAICompatibleNode_Execute_B1InvalidParamErrors(t *testing.T) {
	// Invalid temperature should fail fast without hitting the network.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be reached on param validation failure")
	}))
	defer srv.Close()

	cfg := LLMNodeConfig{
		Name:            "testnode",
		DefaultModel:    "test-model",
		DefaultEndpoint: "http://localhost:11434",
		EnvAPIKey:       "AFLARE_TEST_UNIQUE_API_KEY_NEVER_SET",
		ProviderName:    "TestProvider",
	}
	n := NewOpenAICompatibleNode(cfg)
	_, err := n.Execute(context.Background(), "hi", map[string]string{
		"api_key":     "sk-test",
		"endpoint":    srv.URL,
		"temperature": "not-a-number",
	})
	if err == nil {
		t.Fatal("expected error for invalid temperature")
	}
	if !strings.Contains(err.Error(), "temperature") {
		t.Errorf("error should mention temperature: %v", err)
	}
}
