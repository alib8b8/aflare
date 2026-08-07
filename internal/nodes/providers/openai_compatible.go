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

// openai_compatible.go consolidates every OpenAI-compatible LLM provider
// that previously lived in its own ~30-line file (openai.go, glm.go,
// kimi.go, qwen.go, deepseek.go, anthropic.go, gemini.go, mistral.go,
// yi.go, baichuan.go, internlm.go, minimax.go, xverse.go, mimo.go,
// coze.go, ima.go). Each of those files was a thin wrapper that did
// nothing but register a core.OpenAICompatibleNode with a handful of
// config values. A single config-table-driven init() replaces all 16
// files without changing any provider's name, default model, default
// endpoint, or env-var contract.
//
// Providers with bespoke request/response shapes (ollama, fastgpt) or
// simulated implementations (sensenova, antling, andesgpt) remain in
// their own files because they cannot be expressed as a plain
// OpenAI-compatible config entry.
package providers

import (
	"github.com/alib8b8/llm-box/internal/nodes/core"
)

// openAICompatibleConfigs lists every OpenAI-compatible provider that
// should be registered with the global core.Registry. To add a new
// provider whose API follows the OpenAI /chat/completions contract,
// append one entry here — no new file is needed.
//
// Fields:
//   - Name:            node name used in workflows (also the registry key)
//   - DefaultModel:    model sent when the caller omits the "model" param
//   - DefaultEndpoint: base URL when no env var / param overrides it
//   - EnvAPIKey:       env var read for the API key
//   - EnvAPIBase:      optional env var read for the base URL (e.g.
//     OPENAI_API_BASE / IMA_API_BASE); takes precedence
//     over the generic "{EnvAPIKey}_ENDPOINT" lookup
//   - ProviderName:    human-readable label used in descriptions/errors
var openAICompatibleConfigs = []core.LLMNodeConfig{
	{
		Name:            "openai",
		DefaultModel:    "gpt-3.5-turbo",
		DefaultEndpoint: "https://api.openai.com/v1",
		EnvAPIKey:       "OPENAI_API_KEY",  // #nosec G101 -- env var name, not a credential value
		EnvAPIBase:      "OPENAI_API_BASE", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "OpenAI",
	},
	{
		Name:            "anthropic",
		DefaultModel:    "claude-3-5-sonnet-latest",
		DefaultEndpoint: "https://api.anthropic.com/v1",
		EnvAPIKey:       "ANTHROPIC_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Anthropic",
	},
	{
		Name:            "gemini",
		DefaultModel:    "gemini-2.0-flash",
		DefaultEndpoint: "https://generativelanguage.googleapis.com/v1beta/openai",
		EnvAPIKey:       "GEMINI_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Google Gemini",
	},
	{
		Name:            "glm",
		DefaultModel:    "glm-4",
		DefaultEndpoint: "https://open.bigmodel.cn/api/paas/v4",
		EnvAPIKey:       "GLM_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "GLM",
	},
	{
		Name:            "kimi",
		DefaultModel:    "kimi-k3",
		DefaultEndpoint: "https://api.moonshot.cn/v1",
		EnvAPIKey:       "KIMI_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Kimi",
	},
	{
		Name:            "qwen",
		DefaultModel:    "qwen-turbo",
		DefaultEndpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		EnvAPIKey:       "QWEN_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Qwen",
	},
	{
		Name:            "deepseek",
		DefaultModel:    "deepseek-chat",
		DefaultEndpoint: "https://api.deepseek.com/v1",
		EnvAPIKey:       "DEEPSEEK_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "DeepSeek",
	},
	{
		Name:            "mistral",
		DefaultModel:    "mistral-large-latest",
		DefaultEndpoint: "https://api.mistral.ai/v1",
		EnvAPIKey:       "MISTRAL_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Mistral",
	},
	{
		Name:            "yi",
		DefaultModel:    "yi-lightning",
		DefaultEndpoint: "https://api.lingyiwanwu.com/v1",
		EnvAPIKey:       "YI_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Yi",
	},
	{
		Name:            "baichuan",
		DefaultModel:    "Baichuan4",
		DefaultEndpoint: "https://api.baichuan-ai.com/v1",
		EnvAPIKey:       "BAICHUAN_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Baichuan",
	},
	{
		Name:            "internlm",
		DefaultModel:    "internlm3-latest",
		DefaultEndpoint: "https://internlm-chat.intern-ai.org.cn/api/v1",
		EnvAPIKey:       "INTERNLM_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "InternLM",
	},
	{
		Name:            "minimax",
		DefaultModel:    "abab6.5s-chat",
		DefaultEndpoint: "https://api.minimax.chat/v1",
		EnvAPIKey:       "MINIMAX_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "MiniMax",
	},
	{
		Name:            "xverse",
		DefaultModel:    "XVERSE-7B-Chat",
		DefaultEndpoint: "https://api.xverse.cn/v1",
		EnvAPIKey:       "XVERSE_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "XVERSE",
	},
	{
		Name:            "mimo",
		DefaultModel:    "mimo-v2.5-pro",
		DefaultEndpoint: "https://api.xiaomimimo.com/v1",
		EnvAPIKey:       "MIMO_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "MiMo",
	},
	{
		Name:            "coze",
		DefaultModel:    "",
		DefaultEndpoint: "https://api.coze.cn/v1",
		EnvAPIKey:       "COZE_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Coze",
	},
	{
		Name:            "ima",
		DefaultModel:    "",
		DefaultEndpoint: "",
		EnvAPIKey:       "IMA_API_KEY",  // #nosec G101 -- env var name, not a credential value
		EnvAPIBase:      "IMA_API_BASE", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "IMA Copilot",
	},
	{
		Name:            "ascend",
		DefaultModel:    "qwen2.5-7b",
		DefaultEndpoint: "http://localhost:8080/v1",
		EnvAPIKey:       "ASCEND_API_KEY",  // #nosec G101 -- env var name, not a credential value
		EnvAPIBase:      "ASCEND_API_BASE", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Ascend",
	},
	{
		Name:            "cambricon",
		DefaultModel:    "qwen2.5-7b",
		DefaultEndpoint: "http://localhost:8081/v1",
		EnvAPIKey:       "CAMBRICON_API_KEY",  // #nosec G101 -- env var name, not a credential value
		EnvAPIBase:      "CAMBRICON_API_BASE", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Cambricon MLU",
	},
	{
		Name:            "hygon",
		DefaultModel:    "qwen2.5-7b",
		DefaultEndpoint: "http://localhost:8082/v1",
		EnvAPIKey:       "HYGON_API_KEY",  // #nosec G101 -- env var name, not a credential value
		EnvAPIBase:      "HYGON_API_BASE", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Hygon DCU",
	},
}

func init() {
	for _, cfg := range openAICompatibleConfigs {
		core.Register(core.NewOpenAICompatibleNode(cfg))
	}
}

// OpenAICompatibleConfigs returns a copy of the consolidated provider
// config table. It is exported so that callers (e.g. RegisterBuiltins in
// the parent nodes package) can construct local copies of these nodes
// without duplicating the config values.
func OpenAICompatibleConfigs() []core.LLMNodeConfig {
	out := make([]core.LLMNodeConfig, len(openAICompatibleConfigs))
	copy(out, openAICompatibleConfigs)
	return out
}
