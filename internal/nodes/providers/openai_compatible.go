// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌​​‌‌‌‌​‌‌​‌​​‌​​​​‌​​​​​‌‌‌‌‌​​​‌‌‌​​‌​‌‌​‌​‌​​​​​​​​​​​​​​​​​‌‌‌‌​​‌​‌‌​​​​‌⁠
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
// Providers with bespoke request/response shapes (ollama, fastgpt) remain
// in their own files because they cannot be expressed as a plain
// OpenAI-compatible config entry.
package providers

import (
	"github.com/alib8b8/aflare/internal/nodes/core"
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
		DefaultModel:    "claude-sonnet-5",
		DefaultEndpoint: "https://api.anthropic.com/v1",
		EnvAPIKey:       "ANTHROPIC_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Anthropic",
		// The native Anthropic API is NOT OpenAI-compatible: it uses
		// POST /v1/messages with x-api-key + anthropic-version headers,
		// not /chat/completions with Authorization: Bearer. This entry
		// drives the request through the OpenAI-compatible code path, so
		// calls to the default endpoint will 404. To actually use Claude,
		// point `endpoint` at an OpenAI-compatible proxy such as
		// LiteLLM, one-api, or LiteLLM proxy that translates the
		// protocol. See docs/llm-providers.md (added separately) for
		// setup instructions. The description makes this limit visible
		// in `aflare list` so users are not surprised by 404s.
		// Default model: claude-sonnet-5 is the current mainstream Claude
		// model ($2/$10 per MTok). The previous default
		// (claude-3-5-sonnet-latest) was retired off the direct API in
		// 2026 — proxies reject it for accounts created after the
		// retirement. Note: on Claude Fable 5.1 / Mythos 5.1 the
		// tool_choice values "any"/"tool" return a 400 ("auto"/"none"
		// still work) — prefer strict tool use or structured outputs.
		DescriptionOverride: "Call Anthropic Claude via an OpenAI-compatible proxy (LiteLLM/one-api). Native api.anthropic.com is NOT OpenAI-compatible and will 404; configure `endpoint` to point at a proxy that translates the protocol.",
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
	// ---- Cloud aggregators & international providers (verified against
	// each provider's published OpenAI-compatible docs) ----
	{
		Name:            "openrouter",
		DefaultModel:    "openrouter/auto",
		DefaultEndpoint: "https://openrouter.ai/api/v1",
		EnvAPIKey:       "OPENROUTER_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "OpenRouter",
	},
	{
		Name:            "groq",
		DefaultModel:    "llama-3.3-70b-versatile",
		DefaultEndpoint: "https://api.groq.com/openai/v1",
		EnvAPIKey:       "GROQ_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Groq",
	},
	{
		Name:            "cerebras",
		DefaultModel:    "llama-3.3-70b",
		DefaultEndpoint: "https://api.cerebras.ai/v1",
		EnvAPIKey:       "CEREBRAS_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Cerebras",
	},
	{
		Name:            "perplexity",
		DefaultModel:    "sonar",
		DefaultEndpoint: "https://api.perplexity.ai",
		EnvAPIKey:       "PERPLEXITY_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Perplexity",
	},
	{
		Name:            "together",
		DefaultModel:    "meta-llama/Llama-3.3-70B-Instruct-Turbo",
		DefaultEndpoint: "https://api.together.xyz/v1",
		EnvAPIKey:       "TOGETHER_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Together AI",
	},
	{
		Name:            "fireworks",
		DefaultModel:    "accounts/fireworks/models/llama-v3p3-70b-instruct",
		DefaultEndpoint: "https://api.fireworks.ai/inference/v1",
		EnvAPIKey:       "FIREWORKS_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Fireworks AI",
	},
	{
		Name:            "nvidia",
		DefaultModel:    "meta/llama-3.3-70b-instruct",
		DefaultEndpoint: "https://integrate.api.nvidia.com/v1",
		EnvAPIKey:       "NVIDIA_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "NVIDIA NIM",
	},
	{
		Name:            "xai",
		DefaultModel:    "grok-4",
		DefaultEndpoint: "https://api.x.ai/v1",
		EnvAPIKey:       "XAI_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "xAI Grok",
	},
	// ---- Chinese cloud providers with native OpenAI-compatible
	// endpoints (verified against each vendor's official docs) ----
	{
		Name:            "ark",
		DefaultModel:    "doubao-seed-2-1-pro-260628",
		DefaultEndpoint: "https://ark.cn-beijing.volces.com/api/v3",
		EnvAPIKey:       "ARK_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Volcengine Ark",
	},
	{
		Name:            "siliconflow",
		DefaultModel:    "Qwen/Qwen3-32B",
		DefaultEndpoint: "https://api.siliconflow.cn/v1",
		EnvAPIKey:       "SILICONFLOW_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "SiliconFlow",
	},
	{
		Name:            "qianfan",
		DefaultModel:    "ernie-4.5-turbo-128k",
		DefaultEndpoint: "https://qianfan.baidubce.com/v2",
		EnvAPIKey:       "QIANFAN_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Baidu Qianfan",
	},
	{
		Name:            "hunyuan",
		DefaultModel:    "hunyuan-pro",
		DefaultEndpoint: "https://api.hunyuan.cloud.tencent.com/v1",
		EnvAPIKey:       "HUNYUAN_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Tencent Hunyuan",
	},
	{
		Name:            "stepfun",
		DefaultModel:    "step-2-16k",
		DefaultEndpoint: "https://api.stepfun.com/v1",
		EnvAPIKey:       "STEPFUN_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "StepFun",
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
