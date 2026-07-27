// Copyright (c) 2026 llm-box Contributors
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

	"github.com/alib8b8/llm-box/internal/nodes/core"
)

// allProviderNames lists every LLM provider node that the providers
// subpackage is expected to register via init().
var allProviderNames = []string{
	"andesgpt", "anthropic", "antling", "baichuan", "coze",
	"deepseek", "fastgpt", "gemini", "glm", "ima", "internlm",
	"kimi", "mimo", "minimax", "mistral", "ollama",
	"openai", "qwen", "sensenova", "xverse", "yi",
}

// TestAllProvidersRegistered verifies that every expected provider is
// present in the global registry after the package init() functions run.
func TestAllProvidersRegistered(t *testing.T) {
	for _, name := range allProviderNames {
		t.Run(name, func(t *testing.T) {
			node, ok := core.Get(name)
			if !ok {
				t.Fatalf("provider %q not found in registry", name)
			}
			if node.Name() != name {
				t.Errorf("provider %q: Name() returned %q", name, node.Name())
			}
		})
	}
}

// TestProviderMetadataConsistency checks that every registered provider
// exposes consistent Name()/Description()/Schema() metadata.
func TestProviderMetadataConsistency(t *testing.T) {
	for _, name := range allProviderNames {
		t.Run(name, func(t *testing.T) {
			node, ok := core.Get(name)
			if !ok {
				t.Fatalf("provider %q not found", name)
			}

			if node.Name() != name {
				t.Errorf("Name() = %q, want %q", node.Name(), name)
			}

			desc := node.Description()
			if desc == "" {
				t.Errorf("Description() is empty")
			}

			schema := node.Schema()
			if schema.Name != name {
				t.Errorf("Schema().Name = %q, want %q", schema.Name, name)
			}
			if schema.Description == "" {
				t.Errorf("Schema().Description is empty")
			}
			if schema.Description != desc {
				t.Errorf("Schema().Description %q != Description() %q", schema.Description, desc)
			}
			if schema.Input == "" {
				t.Errorf("Schema().Input is empty")
			}
			if schema.Output == "" {
				t.Errorf("Schema().Output is empty")
			}
			if len(schema.Params) == 0 {
				t.Errorf("Schema().Params is empty")
			}
		})
	}
}

// TestProviderSchemaParams validates that every parameter in each
// provider's schema is well-formed (non-empty name/type/description).
func TestProviderSchemaParams(t *testing.T) {
	for _, name := range allProviderNames {
		t.Run(name, func(t *testing.T) {
			node, ok := core.Get(name)
			if !ok {
				t.Fatalf("provider %q not found", name)
			}
			schema := node.Schema()

			seenNames := make(map[string]bool, len(schema.Params))
			for _, p := range schema.Params {
				if p.Name == "" {
					t.Errorf("param has empty Name")
					continue
				}
				if seenNames[p.Name] {
					t.Errorf("duplicate param %q", p.Name)
				}
				seenNames[p.Name] = true

				if p.Type == "" {
					t.Errorf("param %q has empty Type", p.Name)
				}
				if p.Description == "" {
					t.Errorf("param %q has empty Description", p.Name)
				}
			}

			// Every provider should accept an api_key or a model-related
			// param; this guards against schemas losing their inputs.
			// (andesgpt uses model_size instead of model.)
			hasAPIKey := seenNames["api_key"]
			hasModel := seenNames["model"] || seenNames["model_size"]
			if !hasAPIKey && !hasModel {
				t.Errorf("schema has neither api_key nor model/model_size param")
			}
		})
	}
}

// TestDefaultEndpointFor verifies core.DefaultEndpointFor returns the
// documented endpoint URL for every provider it knows about, and falls
// back to the local Ollama endpoint for unknown providers.
func TestDefaultEndpointFor(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"ollama", "http://localhost:11434"},
		{"openai", "https://api.openai.com/v1"},
		{"deepseek", "https://api.deepseek.com/v1"},
		{"glm", "https://open.bigmodel.cn/api/paas/v4"},
		{"kimi", "https://api.moonshot.cn/v1"},
		{"qwen", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
		{"mistral", "https://api.mistral.ai/v1"},
		{"yi", "https://api.lingyiwanwu.com/v1"},
		{"anthropic", "https://api.anthropic.com/v1"},
		{"gemini", "https://generativelanguage.googleapis.com/v1beta/openai"},
		// Unknown providers fall back to the local Ollama endpoint.
		{"unknown-provider", "http://localhost:11434"},
		{"", "http://localhost:11434"},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := core.DefaultEndpointFor(tt.provider)
			if got != tt.want {
				t.Errorf("DefaultEndpointFor(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

// TestProviderExecuteMissingAPIKey verifies that OpenAI-compatible
// providers return a clear error when no API key is available (neither
// in params nor in the environment). These providers all delegate to
// core.OpenAICompatibleNode.Execute, which enforces the api_key check.
func TestProviderExecuteMissingAPIKey(t *testing.T) {
	// Providers that use the OpenAI-compatible execute path and therefore
	// require an API key. Each entry maps the provider name to the env var
	// it reads for the key, so the test can guarantee the var is empty.
	cases := []struct {
		provider string
		envVar   string
	}{
		{"openai", "OPENAI_API_KEY"},
		{"anthropic", "ANTHROPIC_API_KEY"},
		{"deepseek", "DEEPSEEK_API_KEY"},
		{"glm", "GLM_API_KEY"},
		{"kimi", "KIMI_API_KEY"},
		{"qwen", "QWEN_API_KEY"},
		{"mistral", "MISTRAL_API_KEY"},
		{"yi", "YI_API_KEY"},
		{"baichuan", "BAICHUAN_API_KEY"},
		{"internlm", "INTERNLM_API_KEY"},
		{"minimax", "MINIMAX_API_KEY"},
		{"xverse", "XVERSE_API_KEY"},
		{"mimo", "MIMO_API_KEY"},
		{"gemini", "GEMINI_API_KEY"},
		// ima/coze are custom structs that delegate to the compat node;
		// without a key they also surface the compat-layer error.
		{"ima", "IMA_API_KEY"},
		{"coze", "COZE_API_KEY"},
		// fastgpt has its own execute path but also requires api_key.
		{"fastgpt", "FASTGPT_API_KEY"},
	}

	ctx := context.Background()
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			// Guarantee the env var is unset so the key truly is missing.
			t.Setenv(tc.envVar, "")

			node, ok := core.Get(tc.provider)
			if !ok {
				t.Fatalf("provider %q not found", tc.provider)
			}

			// Provide a valid local endpoint so that any failure is due
			// to the missing API key, not an invalid endpoint URL.
			params := map[string]string{
				"endpoint": "http://localhost:11434",
			}

			_, err := node.Execute(ctx, "hello", params)
			if err == nil {
				t.Fatalf("expected error for missing api_key, got nil")
			}
			// The error should mention "API key" to be useful.
			low := strings.ToLower(err.Error())
			if !strings.Contains(low, "api key") && !strings.Contains(low, "api_key") {
				t.Errorf("error %q does not mention api key", err)
			}
		})
	}
}

// TestProviderExecuteReadsEnvAPIKey confirms that the custom-struct
// providers (openai/ima/coze) honour their documented env var: when the
// env var is set, the api_key is forwarded to the compat node and the
// request reaches the (mock) server instead of failing on auth.
func TestProviderExecuteReadsEnvAPIKey(t *testing.T) {
	cases := []struct {
		provider string
		envVar   string
	}{
		{"openai", "OPENAI_API_KEY"},
		{"ima", "IMA_API_KEY"},
		{"coze", "COZE_API_KEY"},
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			t.Setenv(tc.envVar, "test-key-from-env")

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				auth := r.Header.Get("Authorization")
				if auth != "Bearer test-key-from-env" {
					t.Errorf("expected Bearer test-key-from-env, got %q", auth)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
			}))
			defer srv.Close()

			node, ok := core.Get(tc.provider)
			if !ok {
				t.Fatalf("provider %q not found", tc.provider)
			}
			params := map[string]string{
				"endpoint": srv.URL,
			}
			if tc.provider == "ima" {
				params["model"] = "ima-model"
			}
			if tc.provider == "coze" {
				params["model"] = "coze-model"
			}

			out, err := node.Execute(context.Background(), "hi", params)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if out != "ok" {
				t.Errorf("expected output %q, got %q", "ok", out)
			}
		})
	}
}

// TestOpenAIExecuteWithMockServer exercises the full happy path of the
// OpenAI provider's Execute, including endpoint/env var resolution and
// response parsing through the compat layer.
func TestOpenAIExecuteWithMockServer(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")

	var receivedModel, receivedMsg string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected path /chat/completions, got %s", r.URL.Path)
		}
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if m, ok := body["model"].(string); ok {
			receivedModel = m
		}
		if msgs, ok := body["messages"].([]interface{}); ok && len(msgs) > 0 {
			if msg, ok := msgs[len(msgs)-1].(map[string]interface{}); ok {
				receivedMsg, _ = msg["content"].(string)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Hello from OpenAI!"}}]}`))
	}))
	defer srv.Close()

	node, ok := core.Get("openai")
	if !ok {
		t.Fatal("openai provider not found")
	}
	params := map[string]string{
		"endpoint": srv.URL,
		"model":    "gpt-4o",
		"system":   "be concise",
	}
	out, err := node.Execute(context.Background(), "ping", params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if out != "Hello from OpenAI!" {
		t.Errorf("expected %q, got %q", "Hello from OpenAI!", out)
	}
	if receivedModel != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %q", receivedModel)
	}
	if receivedMsg != "ping" {
		t.Errorf("expected message ping, got %q", receivedMsg)
	}
}

// --- Simulated provider Execute tests ---------------------------------------
//
// andesgpt, antling and sensenova are "simulated" providers: they do not
// make HTTP calls, they synthesise a response locally. Their Execute
// methods contain the bulk of the providers-package logic that is not
// covered by the metadata tests above, so exercising both the success
// and validation-error branches gives a large coverage boost.

func TestAndesGPTExecuteSuccess(t *testing.T) {
	node, ok := core.Get("andesgpt")
	if !ok {
		t.Fatal("andesgpt provider not found")
	}
	ctx := context.Background()

	// Default params success path.
	out, err := node.Execute(ctx, "推荐一家餐厅", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if result["type"] != "andesgpt" {
		t.Errorf("expected type andesgpt, got %v", result["type"])
	}
	if result["exec_location"] == "" {
		t.Errorf("expected exec_location to be set")
	}

	// With persona + memory enabled (exercises the persona branch).
	params := map[string]string{
		"model_size":    "turbo",
		"scene":         "life",
		"persona_id":    "user-123",
		"use_memory":    "true",
		"max_tokens":    "512",
		"system_prompt": "be helpful",
	}
	out, err = node.Execute(ctx, "记住我喜欢辣的", params)
	if err != nil {
		t.Fatalf("Execute with persona failed: %v", err)
	}
	if !strings.Contains(out, "个人偏好") {
		t.Errorf("expected persona+memory response, got %s", out)
	}
}

func TestAndesGPTExecuteErrors(t *testing.T) {
	node, ok := core.Get("andesgpt")
	if !ok {
		t.Fatal("andesgpt provider not found")
	}
	ctx := context.Background()

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"invalid model_size", map[string]string{"model_size": "huge"}, "invalid model_size"},
		{"invalid scene", map[string]string{"scene": "rocket"}, "invalid scene"},
		{"invalid persona_id", map[string]string{"persona_id": "bad space!"}, "invalid persona_id format"},
		{"system_prompt too long", map[string]string{"system_prompt": strings.Repeat("a", 4001)}, "system_prompt too long"},
		{"invalid end_cloud_mode", map[string]string{"end_cloud_mode": "magic"}, "invalid end_cloud_mode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "x", tt.params)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.errSub)
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("expected error containing %q, got %q", tt.errSub, err)
			}
		})
	}
}

func TestAntLingExecuteSuccess(t *testing.T) {
	node, ok := core.Get("antling")
	if !ok {
		t.Fatal("antling provider not found")
	}
	ctx := context.Background()

	out, err := node.Execute(ctx, "帮我写段代码", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if result["type"] != "antling" {
		t.Errorf("expected type antling, got %v", result["type"])
	}
	if result["model"] != "ling-2.6-flash" {
		t.Errorf("expected default model ling-2.6-flash, got %v", result["model"])
	}

	// Multimodal with ming model (exercises the multimodal branch).
	params := map[string]string{
		"model": "ming-flash-omni-2.0",
		"scene": "multimodal",
	}
	out, err = node.Execute(ctx, "describe this image", params)
	if err != nil {
		t.Fatalf("Execute multimodal failed: %v", err)
	}
	if !strings.Contains(out, "多模态") {
		t.Errorf("expected multimodal response, got %s", out)
	}
}

func TestAntLingExecuteErrors(t *testing.T) {
	node, ok := core.Get("antling")
	if !ok {
		t.Fatal("antling provider not found")
	}
	ctx := context.Background()

	tests := []struct {
		name   string
		input  string
		params map[string]string
		errSub string
	}{
		{"input too long", strings.Repeat("a", 8193), nil, "input too long"},
		{"invalid model", "x", map[string]string{"model": "nope"}, "invalid model"},
		{"invalid scene", "x", map[string]string{"scene": "nope"}, "invalid scene"},
		{"invalid api_key format", "x", map[string]string{"api_key": "short"}, "invalid api_key format"},
		{"base_url too long", "x", map[string]string{"base_url": strings.Repeat("h", 513)}, "base_url too long"},
		{"system_prompt too long", "x", map[string]string{"system_prompt": strings.Repeat("a", 8001)}, "system_prompt too long"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, tt.input, tt.params)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.errSub)
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("expected error containing %q, got %q", tt.errSub, err)
			}
		})
	}
}

func TestSenseNovaExecuteSuccess(t *testing.T) {
	node, ok := core.Get("sensenova")
	if !ok {
		t.Fatal("sensenova provider not found")
	}
	ctx := context.Background()

	out, err := node.Execute(ctx, "你好", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if result["type"] != "sensenova" {
		t.Errorf("expected type sensenova, got %v", result["type"])
	}
	if result["model"] != "flash-lite" {
		t.Errorf("expected default model flash-lite, got %v", result["model"])
	}

	// U1 image scene exercises the U1 multimodal branch.
	params := map[string]string{
		"model":  "u1-pro",
		"scene":  "image",
		"vision": "true",
	}
	out, err = node.Execute(ctx, "画一只猫", params)
	if err != nil {
		t.Fatalf("Execute image failed: %v", err)
	}
	if !strings.Contains(out, "U1") {
		t.Errorf("expected U1 in image response, got %s", out)
	}
}

func TestSenseNovaExecuteErrors(t *testing.T) {
	node, ok := core.Get("sensenova")
	if !ok {
		t.Fatal("sensenova provider not found")
	}
	ctx := context.Background()

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"invalid model", map[string]string{"model": "nope"}, "invalid model"},
		{"invalid scene", map[string]string{"scene": "nope"}, "invalid scene"},
		{"invalid api_key format", map[string]string{"api_key": "short"}, "invalid api_key format"},
		{"system_prompt too long", map[string]string{"system_prompt": strings.Repeat("a", 8001)}, "system_prompt too long"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "x", tt.params)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.errSub)
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("expected error containing %q, got %q", tt.errSub, err)
			}
		})
	}
}

// TestFastGPTExecuteMissingAPIKey confirms the FastGPT custom execute
// path surfaces a clear error when no api_key is supplied.
func TestFastGPTExecuteMissingAPIKey(t *testing.T) {
	t.Setenv("FASTGPT_API_KEY", "")
	node, ok := core.Get("fastgpt")
	if !ok {
		t.Fatal("fastgpt provider not found")
	}
	params := map[string]string{
		"endpoint": "http://localhost:11434",
	}
	_, err := node.Execute(context.Background(), "hi", params)
	if err == nil {
		t.Fatalf("expected error for missing api_key, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "api key") {
		t.Errorf("error %q does not mention api key", err)
	}
}

// TestFastGPTExecuteWithMockServer covers FastGPT's happy path through
// its custom HTTP client, including api_key resolution from params and
// response parsing.
func TestFastGPTExecuteWithMockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected path /chat/completions, got %s", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer fg-key" {
			t.Errorf("expected Bearer fg-key, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"fastgpt ok"}}]}`))
	}))
	defer srv.Close()

	node, ok := core.Get("fastgpt")
	if !ok {
		t.Fatal("fastgpt provider not found")
	}
	params := map[string]string{
		"api_key":  "fg-key",
		"endpoint": srv.URL,
	}
	out, err := node.Execute(context.Background(), "hi", params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if out != "fastgpt ok" {
		t.Errorf("expected %q, got %q", "fastgpt ok", out)
	}
}
