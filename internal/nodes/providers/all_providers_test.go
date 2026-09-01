// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌‌‌​‌‌‌‌‌‌‌​‌‌‌‌​​​‌​‌​​‌‌‌​​​​​‌​​‌‌‌​​​‌‌‌‌‌​​​​​​​​​​​​​​​​​​‌‌​‌​​​‌​‌​​‌‌​⁠
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

// allProviderNames lists every LLM provider node that the providers
// subpackage is expected to register via init().
var allProviderNames = []string{
	"anthropic", "ark", "ascend", "baichuan", "cambricon", "cerebras", "coze",
	"deepseek", "fastgpt", "fireworks", "gemini", "glm", "groq", "hygon", "hunyuan", "ima", "internlm",
	"kimi", "mimo", "minimax", "mistral", "nvidia", "ollama",
	"openai", "openrouter", "perplexity", "qianfan", "qwen", "siliconflow", "stepfun", "together", "xai", "xverse", "yi",
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
