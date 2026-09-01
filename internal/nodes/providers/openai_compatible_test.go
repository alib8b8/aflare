// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​​​​‌‌​‌‌​‌​​​​​​​​‌​‌‌​‌‌‌​‌​​​​​​‌‌​​‌‌​​‌‌‌​​​​​​​​​​​​​​​​‌​‌​​‌‌‌‌​‌‌‌‌​‌⁠
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
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

// TestOpenAICompatibleConfigs verifies the exported config table: every
// entry must be registered in the core registry and the returned slice must
// be a defensive copy.
func TestOpenAICompatibleConfigs(t *testing.T) {
	configs := OpenAICompatibleConfigs()
	if len(configs) == 0 {
		t.Fatal("OpenAICompatibleConfigs() is empty")
	}

	for _, cfg := range configs {
		t.Run(cfg.Name, func(t *testing.T) {
			if cfg.Name == "" {
				t.Error("config has empty Name")
			}
			if cfg.ProviderName == "" {
				t.Error("config has empty ProviderName")
			}
			// coze/ima deliberately leave DefaultModel empty (model must
			// be supplied per call); ima also requires IMA_API_BASE.
			if cfg.DefaultModel == "" && cfg.Name != "coze" && cfg.Name != "ima" {
				t.Error("config has empty DefaultModel")
			}
			if cfg.DefaultEndpoint == "" && cfg.Name != "ima" {
				t.Error("config has empty DefaultEndpoint")
			}
			// Every config must correspond to a registered node.
			node, ok := core.Get(cfg.Name)
			if !ok {
				t.Fatalf("node %q from config table not found in registry", cfg.Name)
			}
			if node.Name() != cfg.Name {
				t.Errorf("node Name() = %q, want %q", node.Name(), cfg.Name)
			}
		})
	}

	// Mutating the returned slice must not affect the package table.
	configs[0].DefaultModel = "mutated"
	fresh := OpenAICompatibleConfigs()
	if fresh[0].DefaultModel == "mutated" {
		t.Error("OpenAICompatibleConfigs must return a defensive copy")
	}
}

// newProviderContracts pins the name / env-var / endpoint / default-model
// contract of the v0.12.x provider additions. These fields are public API:
// users set the env vars in shells and CI secrets and pin endpoints in
// config files, so a silent rename (e.g. ARK_API_KEY → VOLCENGINE_API_KEY)
// would break existing deployments. Endpoints are the providers' officially
// documented OpenAI-compatible base URLs (verified against vendor docs).
func newProviderContracts() map[string]core.LLMNodeConfig {
	return map[string]core.LLMNodeConfig{
		"ark":         {EnvAPIKey: "ARK_API_KEY", DefaultEndpoint: "https://ark.cn-beijing.volces.com/api/v3", DefaultModel: "doubao-seed-2-1-pro-260628"},
		"siliconflow": {EnvAPIKey: "SILICONFLOW_API_KEY", DefaultEndpoint: "https://api.siliconflow.cn/v1", DefaultModel: "Qwen/Qwen3-32B"},
		"qianfan":     {EnvAPIKey: "QIANFAN_API_KEY", DefaultEndpoint: "https://qianfan.baidubce.com/v2", DefaultModel: "ernie-4.5-turbo-128k"},
		"hunyuan":     {EnvAPIKey: "HUNYUAN_API_KEY", DefaultEndpoint: "https://api.hunyuan.cloud.tencent.com/v1", DefaultModel: "hunyuan-pro"},
		"stepfun":     {EnvAPIKey: "STEPFUN_API_KEY", DefaultEndpoint: "https://api.stepfun.com/v1", DefaultModel: "step-2-16k"},
		"xai":         {EnvAPIKey: "XAI_API_KEY", DefaultEndpoint: "https://api.x.ai/v1", DefaultModel: "grok-4"},
		"openrouter":  {EnvAPIKey: "OPENROUTER_API_KEY", DefaultEndpoint: "https://openrouter.ai/api/v1", DefaultModel: "openrouter/auto"},
		"groq":        {EnvAPIKey: "GROQ_API_KEY", DefaultEndpoint: "https://api.groq.com/openai/v1", DefaultModel: "llama-3.3-70b-versatile"},
		"cerebras":    {EnvAPIKey: "CEREBRAS_API_KEY", DefaultEndpoint: "https://api.cerebras.ai/v1", DefaultModel: "llama-3.3-70b"},
		"perplexity":  {EnvAPIKey: "PERPLEXITY_API_KEY", DefaultEndpoint: "https://api.perplexity.ai", DefaultModel: "sonar"},
		"together":    {EnvAPIKey: "TOGETHER_API_KEY", DefaultEndpoint: "https://api.together.xyz/v1", DefaultModel: "meta-llama/Llama-3.3-70B-Instruct-Turbo"},
		"fireworks":   {EnvAPIKey: "FIREWORKS_API_KEY", DefaultEndpoint: "https://api.fireworks.ai/inference/v1", DefaultModel: "accounts/fireworks/models/llama-v3p3-70b-instruct"},
		"nvidia":      {EnvAPIKey: "NVIDIA_API_KEY", DefaultEndpoint: "https://integrate.api.nvidia.com/v1", DefaultModel: "meta/llama-3.3-70b-instruct"},
	}
}

// TestNewProviderContracts verifies each v0.12.x provider entry matches its
// pinned contract. The env-var names follow the router's UPPER(name)+"_API_KEY"
// convention (see llm_router.callProvider), which is itself load-bearing: a
// provider whose EnvAPIKey deviates from that convention breaks router
// auto-detection.
func TestNewProviderContracts(t *testing.T) {
	contracts := newProviderContracts()
	if len(contracts) != 13 {
		t.Fatalf("expected 13 pinned provider contracts, got %d", len(contracts))
	}

	byName := make(map[string]core.LLMNodeConfig, len(openAICompatibleConfigs))
	for _, cfg := range OpenAICompatibleConfigs() {
		byName[cfg.Name] = cfg
	}

	for name, want := range contracts {
		t.Run(name, func(t *testing.T) {
			got, ok := byName[name]
			if !ok {
				t.Fatalf("provider %q missing from OpenAICompatibleConfigs()", name)
			}
			if got.EnvAPIKey != want.EnvAPIKey {
				t.Errorf("EnvAPIKey = %q, want %q (renaming this breaks users' env-var config)", got.EnvAPIKey, want.EnvAPIKey)
			}
			if got.DefaultEndpoint != want.DefaultEndpoint {
				t.Errorf("DefaultEndpoint = %q, want %q", got.DefaultEndpoint, want.DefaultEndpoint)
			}
			if got.DefaultModel != want.DefaultModel {
				t.Errorf("DefaultModel = %q, want %q", got.DefaultModel, want.DefaultModel)
			}
			// The router derives env vars as UPPER(name)+"_API_KEY"; a
			// deviation breaks router auto-detection for this provider.
			if expected := strings.ToUpper(name) + "_API_KEY"; got.EnvAPIKey != expected {
				t.Errorf("EnvAPIKey %q deviates from router convention %q", got.EnvAPIKey, expected)
			}
		})
	}
}
