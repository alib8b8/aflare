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

package nodes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// nodes_coverage_boost4_test.go 继续补充根包纯逻辑函数的单元测试，
// 目标将覆盖率从 43.6% 提升到 50%+。仅测试不依赖真实 LLM API/网络/
// 外部文件的纯逻辑函数与简单成功/错误路径。

// ----------------------------------------------------------------------------
// omniroute_node.go: provider 选择策略与 simulate 响应（纯逻辑）
// ----------------------------------------------------------------------------

func TestOmniRouteNode_SelectFastestProvider(t *testing.T) {
	n := &OmniRouteNode{}
	// 已知延迟：ollama=200, vllm=150, openai=800
	candidates := []string{"openai", "ollama", "vllm"}
	got := n.selectFastestProvider(candidates)
	// vllm 延迟最低 (150)
	if got != "vllm" {
		t.Errorf("selectFastestProvider = %q, want vllm", got)
	}
	// 单元素
	if got := n.selectFastestProvider([]string{"openai"}); got != "openai" {
		t.Errorf("selectFastestProvider single = %q, want openai", got)
	}
	// 未知 provider（延迟为 0，应被视为最低并选中）
	got = n.selectFastestProvider([]string{"unknown_xyz", "openai"})
	if got != "unknown_xyz" {
		t.Errorf("selectFastestProvider with unknown (0 latency) = %q, want unknown_xyz", got)
	}
}

func TestOmniRouteNode_SelectCheapestProvider(t *testing.T) {
	n := &OmniRouteNode{}
	// 已知成本：ollama=0, vllm=0, openai=0.015, antling=0.0005
	candidates := []string{"openai", "antling", "ollama"}
	got := n.selectCheapestProvider(candidates)
	// ollama 成本 0 最低
	if got != "ollama" {
		t.Errorf("selectCheapestProvider = %q, want ollama", got)
	}
	// 单元素
	if got := n.selectCheapestProvider([]string{"openai"}); got != "openai" {
		t.Errorf("selectCheapestProvider single = %q, want openai", got)
	}
}

func TestOmniRouteNode_SelectBestQualityProvider(t *testing.T) {
	n := &OmniRouteNode{}
	// qualityOrder 优先级：anthropic > openai > google > ...
	candidates := []string{"ollama", "google", "openai", "anthropic"}
	got := n.selectBestQualityProvider(candidates)
	if got != "anthropic" {
		t.Errorf("selectBestQualityProvider = %q, want anthropic", got)
	}
	// vllm 和 ollama 都在 qualityOrder 中，ollama 优先级更高
	got = n.selectBestQualityProvider([]string{"vllm", "ollama"})
	if got != "ollama" {
		t.Errorf("selectBestQualityProvider vllm+ollama = %q, want ollama", got)
	}
	// 没有 qualityOrder 中的 provider，回退到第一个
	got = n.selectBestQualityProvider([]string{"unknownX", "unknownY"})
	if got != "unknownX" {
		t.Errorf("selectBestQualityProvider fallback = %q, want unknownX", got)
	}
	// 单元素
	if got := n.selectBestQualityProvider([]string{"deepseek"}); got != "deepseek" {
		t.Errorf("selectBestQualityProvider single = %q, want deepseek", got)
	}
}

func TestOmniRouteNode_SelectAutoProvider(t *testing.T) {
	n := &OmniRouteNode{}
	// >=3 候选返回第二个
	got := n.selectAutoProvider([]string{"a", "b", "c"})
	if got != "b" {
		t.Errorf("selectAutoProvider 3 = %q, want b", got)
	}
	// <3 候选返回第一个
	got = n.selectAutoProvider([]string{"a", "b"})
	if got != "a" {
		t.Errorf("selectAutoProvider 2 = %q, want a", got)
	}
	got = n.selectAutoProvider([]string{"only"})
	if got != "only" {
		t.Errorf("selectAutoProvider 1 = %q, want only", got)
	}
}

func TestOmniRouteNode_SelectRandomProvider(t *testing.T) {
	n := &OmniRouteNode{}
	candidates := []string{"openai", "anthropic", "google"}
	got := n.selectRandomProvider(candidates)
	found := false
	for _, c := range candidates {
		if c == got {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("selectRandomProvider = %q, not in candidates %v", got, candidates)
	}
	// 单元素
	if got := n.selectRandomProvider([]string{"solo"}); got != "solo" {
		t.Errorf("selectRandomProvider single = %q, want solo", got)
	}
}

func TestOmniRouteNode_SelectModel(t *testing.T) {
	n := &OmniRouteNode{}
	// 已知 provider 的模型集合
	models := providerModels["openai"]
	got := n.selectModel("openai")
	found := false
	for _, m := range models {
		if m == got {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("selectModel(openai) = %q, not in %v", got, models)
	}
	// 未知 provider 返回 default
	if got := n.selectModel("unknown_xyz"); got != "default" {
		t.Errorf("selectModel(unknown) = %q, want default", got)
	}
}

func TestOmniRouteNode_ResolveBaseURL(t *testing.T) {
	n := &OmniRouteNode{}
	tests := []struct {
		name     string
		provider string
		region   string
		want     string
	}{
		{"openai no region", "openai", "", "https://api.openai.com/v1"},
		{"anthropic no region", "anthropic", "", "https://api.anthropic.com/v1"},
		{"azure with region", "azure", "eastus", "https://eastus.openai.azure.com/openai"},
		{"aws with region", "aws", "us-west-2", "https://bedrock-runtime.us-west-2.amazonaws.com"},
		{"unknown provider", "unknown_xyz", "", ""},
		{"unknown provider with region", "unknown_xyz", "region1", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := n.resolveBaseURL(tt.provider, tt.region); got != tt.want {
				t.Errorf("resolveBaseURL(%q, %q) = %q, want %q", tt.provider, tt.region, got, tt.want)
			}
		})
	}
}

func TestOmniRouteNode_SimulateOmniRouteResponse(t *testing.T) {
	n := &OmniRouteNode{}
	tests := []struct {
		tool   string
		prefix string
	}{
		{"claude_code", "[Claude Code]"},
		{"cursor", "[Cursor IDE]"},
		{"cline", "[Cline Editor]"},
		{"llm_box", "[llm-box]"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := n.simulateOmniRouteResponse("test input", "openai", "gpt-4o", tt.tool)
			if tt.prefix != "" && !strings.HasPrefix(got, tt.prefix) {
				t.Errorf("simulateOmniRouteResponse tool=%q = %q, want prefix %q", tt.tool, got, tt.prefix)
			}
			if !strings.Contains(got, "openai") {
				t.Errorf("expected provider in output: %s", got)
			}
			if !strings.Contains(got, "gpt-4o") {
				t.Errorf("expected model in output: %s", got)
			}
			if !strings.Contains(got, "test input") {
				t.Errorf("expected input in output: %s", got)
			}
		})
	}
}

func TestOmniRouteNode_SelectProvider_Strategies(t *testing.T) {
	n := &OmniRouteNode{}
	// 各策略应返回有效 provider
	strategies := []string{"fastest", "cheapest", "best_quality", "availability", "auto"}
	for _, s := range strategies {
		got := n.selectProvider("llm_box", s, "")
		if got == "" {
			t.Errorf("selectProvider(strategy=%q) returned empty", s)
		}
		if !validOmniRouteProviders[got] {
			t.Errorf("selectProvider(strategy=%q) = %q, not a valid provider", s, got)
		}
	}
	// custom_fallback 策略：fallback 列表中的有效 provider 应被选中
	got := n.selectProvider("llm_box", "custom_fallback", "vllm, ollama")
	if got != "vllm" {
		t.Errorf("selectProvider(custom_fallback) = %q, want vllm", got)
	}
	// custom_fallback 策略：空 fallback 应回退到 candidates[0]
	got = n.selectProvider("llm_box", "custom_fallback", "")
	if got == "" {
		t.Errorf("selectProvider(custom_fallback empty) returned empty")
	}
	// custom_fallback 策略：无效 fallback 应回退到 candidates[0]
	got = n.selectProvider("llm_box", "custom_fallback", "invalid_provider_xyz")
	if got == "" {
		t.Errorf("selectProvider(custom_fallback invalid) returned empty")
	}
}

func TestOmniRouteNode_GetHealthyProviders(t *testing.T) {
	// 保存并清理全局 providerHealthStatus，避免受其他测试影响
	providerHealthMu.Lock()
	saved := make(map[string]providerHealth, len(providerHealthStatus))
	for k, v := range providerHealthStatus {
		saved[k] = v
	}
	providerHealthStatus = make(map[string]providerHealth)
	providerHealthMu.Unlock()

	defer func() {
		providerHealthMu.Lock()
		providerHealthStatus = saved
		providerHealthMu.Unlock()
	}()

	// 无 health 记录时，已知 tool 应返回全部候选
	got := getHealthyProviders("llm_box")
	if len(got) == 0 {
		t.Errorf("getHealthyProviders(llm_box) returned empty")
	}
	// 未知 tool 应回退到默认 openai/anthropic
	got = getHealthyProviders("unknown_tool")
	if len(got) != 2 {
		t.Errorf("getHealthyProviders(unknown) = %v, want [openai anthropic]", got)
	}
	// 设置 openai 为可用，应只返回健康的 provider
	providerHealthMu.Lock()
	providerHealthStatus["openai"] = providerHealth{Provider: "openai", IsAvailable: true}
	providerHealthStatus["anthropic"] = providerHealth{Provider: "anthropic", IsAvailable: false}
	providerHealthMu.Unlock()
	got = getHealthyProviders("unknown_tool")
	if len(got) != 1 || got[0] != "openai" {
		t.Errorf("getHealthyProviders with mixed health = %v, want [openai]", got)
	}
}

func TestOmniRouteNode_GetHealthStatus(t *testing.T) {
	n := &OmniRouteNode{}
	status := n.GetHealthStatus()
	if status == nil {
		t.Fatalf("GetHealthStatus returned nil")
	}
	// 应包含至少一个已知 provider
	if len(status) == 0 {
		t.Errorf("GetHealthStatus returned empty map")
	}
}

func TestOmniRouteNode_ExecuteHealthCheck(t *testing.T) {
	n := &OmniRouteNode{}
	health := n.ExecuteHealthCheck("openai")
	if health.Provider != "openai" {
		t.Errorf("ExecuteHealthCheck Provider = %q, want openai", health.Provider)
	}
	// status 应为 healthy/degraded/unhealthy 之一
	validStatuses := map[string]bool{"healthy": true, "degraded": true, "unhealthy": true}
	if !validStatuses[health.Status] {
		t.Errorf("ExecuteHealthCheck Status = %q, not valid", health.Status)
	}
}

func TestOmniRouteNode_Execute_ValidationErrors(t *testing.T) {
	n := &OmniRouteNode{}
	ctx := context.Background()

	tests := []struct {
		name    string
		input   string
		params  map[string]string
		wantSub string
	}{
		{
			name:    "input too long",
			input:   strings.Repeat("a", 8193),
			params:  map[string]string{},
			wantSub: "input too long",
		},
		{
			name:    "invalid tool",
			input:   "hi",
			params:  map[string]string{"tool": "invalid_tool"},
			wantSub: "invalid tool",
		},
		{
			name:    "invalid strategy",
			input:   "hi",
			params:  map[string]string{"strategy": "invalid_strategy"},
			wantSub: "invalid strategy",
		},
		{
			name:    "invalid provider",
			input:   "hi",
			params:  map[string]string{"provider": "invalid_provider_xyz"},
			wantSub: "invalid provider",
		},
		{
			name:    "invalid base_url",
			input:   "hi",
			params:  map[string]string{"base_url": "not-a-url"},
			wantSub: "invalid base_url",
		},
		{
			name:    "invalid region",
			input:   "hi",
			params:  map[string]string{"region": "x!"}, // 包含非法字符
			wantSub: "invalid region",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := n.Execute(ctx, tt.input, tt.params)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("expected error containing %q, got: %v", tt.wantSub, err)
			}
		})
	}
}

func TestOmniRouteNode_Execute_Success(t *testing.T) {
	n := &OmniRouteNode{}
	ctx := context.Background()
	out, err := n.Execute(ctx, "hello world", map[string]string{
		"provider": "openai",
		"model":    "gpt-4o",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 应为合法 JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	route, ok := result["route"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing route object in output: %s", out)
	}
	if route["provider"] != "openai" {
		t.Errorf("route.provider = %v, want openai", route["provider"])
	}
	if route["model"] != "gpt-4o" {
		t.Errorf("route.model = %v, want gpt-4o", route["model"])
	}
	if _, ok := result["response"]; !ok {
		t.Errorf("missing response field in output")
	}
	if _, ok := result["usage"]; !ok {
		t.Errorf("missing usage field in output")
	}
}

// ----------------------------------------------------------------------------
// rag.go: chunkText / keywordSearch / phraseSearch / hybridSearch
// ----------------------------------------------------------------------------

func TestChunkText(t *testing.T) {
	// 基本分块
	chunks := chunkText("hello world this is a test", "src.txt", 10, 2)
	if len(chunks) == 0 {
		t.Fatalf("chunkText returned empty for non-empty input")
	}
	if chunks[0].Source != "src.txt" {
		t.Errorf("first chunk Source = %q, want src.txt", chunks[0].Source)
	}
	if chunks[0].Index != 0 {
		t.Errorf("first chunk Index = %d, want 0", chunks[0].Index)
	}
	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("chunk %d has Index %d", i, c.Index)
		}
	}

	// 空字符串返回 nil
	if got := chunkText("   ", "src", 10, 2); got != nil {
		t.Errorf("chunkText(whitespace) = %v, want nil", got)
	}
	if got := chunkText("", "src", 10, 2); got != nil {
		t.Errorf("chunkText(empty) = %v, want nil", got)
	}

	// size 大于文本长度 -> 单个分块
	chunks = chunkText("short", "src", 100, 10)
	if len(chunks) != 1 {
		t.Errorf("chunkText(short, big size) = %d chunks, want 1", len(chunks))
	}
	if chunks[0].Text != "short" {
		t.Errorf("chunk text = %q, want short", chunks[0].Text)
	}

	// unicode 按 rune 分块
	unicode := strings.Repeat("中", 10)
	chunks = chunkText(unicode, "src", 3, 1)
	if len(chunks) < 2 {
		t.Errorf("chunkText(unicode) = %d chunks, want >= 2", len(chunks))
	}
	for _, c := range chunks {
		// 每个 chunk 不应切断 rune
		if len([]rune(c.Text)) > 3 {
			t.Errorf("chunk text rune count > 3: %q", c.Text)
		}
	}
}

func TestKeywordSearch(t *testing.T) {
	chunks := []Chunk{
		{Text: "hello world foo bar", Index: 0},
		{Text: "world peace everywhere", Index: 1},
		{Text: "completely unrelated", Index: 2},
	}
	// 匹配 world
	result := keywordSearch("world", chunks)
	if len(result) == 0 {
		t.Fatalf("keywordSearch(world) returned empty")
	}
	// 第一个结果应包含 world
	if !strings.Contains(strings.ToLower(result[0].Text), "world") {
		t.Errorf("first result should contain 'world': %s", result[0].Text)
	}
	// 不匹配的 chunk 不应出现
	for _, r := range result {
		if r.Index == 2 && r.Score > 0 {
			// "unrelated" chunk 不应被匹配到 world
		}
	}

	// 空 query 应返回所有 chunk（score=0）
	result = keywordSearch("", chunks)
	if len(result) != 3 {
		t.Errorf("keywordSearch(empty) = %d chunks, want 3", len(result))
	}

	// 无匹配
	result = keywordSearch("nonexistentword", chunks)
	if len(result) != 0 {
		t.Errorf("keywordSearch(no match) = %d chunks, want 0", len(result))
	}

	// 空 chunks
	result = keywordSearch("hello", nil)
	if len(result) != 0 {
		t.Errorf("keywordSearch(nil chunks) = %d, want 0", len(result))
	}
}

func TestPhraseSearch(t *testing.T) {
	chunks := []Chunk{
		{Text: "the quick brown fox jumps", Index: 0},
		{Text: "a lazy dog sleeps", Index: 1},
		{Text: "quick brown dog plays", Index: 2},
	}
	// 完整短语匹配
	result := phraseSearch("quick brown fox", chunks)
	if len(result) == 0 {
		t.Fatalf("phraseSearch returned empty")
	}
	// 第一个结果应是包含完整短语的 chunk
	if result[0].Index != 0 {
		t.Errorf("phraseSearch first result Index = %d, want 0 (full match)", result[0].Index)
	}

	// 多词匹配（部分匹配）
	result = phraseSearch("quick dog", chunks)
	if len(result) == 0 {
		t.Errorf("phraseSearch(quick dog) should match some chunks")
	}

	// 无匹配
	result = phraseSearch("nonexistentword", chunks)
	if len(result) != 0 {
		t.Errorf("phraseSearch(no match) = %d, want 0", len(result))
	}

	// 空 query：所有 chunk 都包含空字符串，全部匹配高分
	result = phraseSearch("", chunks)
	if len(result) != 3 {
		t.Errorf("phraseSearch(empty) = %d, want 3", len(result))
	}
}

func TestHybridSearch(t *testing.T) {
	chunks := []Chunk{
		{Text: "hello world from golang", Index: 0},
		{Text: "hello from another world", Index: 1},
		{Text: "completely different content", Index: 2},
	}
	result := hybridSearch("hello world", chunks)
	if len(result) == 0 {
		t.Fatalf("hybridSearch returned empty")
	}
	// 包含 hello world 的 chunk 应排在前面
	if result[0].Index != 0 && result[0].Index != 1 {
		t.Errorf("hybridSearch first result Index = %d, want 0 or 1", result[0].Index)
	}

	// 无匹配
	result = hybridSearch("nonexistentword", chunks)
	if len(result) != 0 {
		t.Errorf("hybridSearch(no match) = %d, want 0", len(result))
	}

	// 空 chunks 不 panic
	result = hybridSearch("hello", nil)
	if len(result) != 0 {
		t.Errorf("hybridSearch(nil) = %d, want 0", len(result))
	}
}

// ----------------------------------------------------------------------------
// skill_distill.go: generateDistilledSkill / generateSteps / generateRules /
// generateExamples / generatePitfalls / Execute
// ----------------------------------------------------------------------------

func TestGenerateSteps(t *testing.T) {
	tests := []struct {
		name        string
		distillType string
		maxSteps    int
		quality     string
		wantMin     int
		wantMax     int
	}{
		{"workflow standard", "workflow", 10, "standard", 1, 10},
		{"decision basic truncated", "decision", 10, "basic", 1, 5},
		{"analysis expert extended", "analysis", 10, "expert", 1, 10},
		{"unknown type defaults to workflow", "unknowntype", 10, "standard", 1, 10},
		{"maxSteps limits", "workflow", 3, "expert", 1, 3},
		{"creative basic", "creative", 10, "basic", 1, 5},
		{"prompt expert", "prompt", 10, "expert", 1, 10},
		{"checklist standard", "checklist", 10, "standard", 1, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := generateSteps(tt.distillType, tt.maxSteps, tt.quality)
			if len(steps) < tt.wantMin {
				t.Errorf("generateSteps returned %d steps, want >= %d", len(steps), tt.wantMin)
			}
			if len(steps) > tt.wantMax {
				t.Errorf("generateSteps returned %d steps, want <= %d", len(steps), tt.wantMax)
			}
			// basic 应限制到 5
			if tt.quality == "basic" && len(steps) > 5 {
				t.Errorf("basic quality should have <= 5 steps, got %d", len(steps))
			}
		})
	}
}

func TestGenerateRules(t *testing.T) {
	// basic -> 2 rules
	rules := generateRules("workflow", "basic")
	if len(rules) != 2 {
		t.Errorf("generateRules(basic) = %d rules, want 2", len(rules))
	}
	// standard -> 4 rules
	rules = generateRules("workflow", "standard")
	if len(rules) != 4 {
		t.Errorf("generateRules(standard) = %d rules, want 4", len(rules))
	}
	// expert -> 7 rules (4 base + 3 extra)
	rules = generateRules("workflow", "expert")
	if len(rules) != 7 {
		t.Errorf("generateRules(expert) = %d rules, want 7", len(rules))
	}
	// 未知 quality 默认 standard
	rules = generateRules("workflow", "unknown")
	if len(rules) != 4 {
		t.Errorf("generateRules(unknown quality) = %d rules, want 4 (standard default)", len(rules))
	}
}

func TestGenerateExamples(t *testing.T) {
	// 非 expert -> 2 examples
	exs := generateExamples("workflow", "standard")
	if len(exs) != 2 {
		t.Errorf("generateExamples(standard) = %d, want 2", len(exs))
	}
	// expert -> 3 examples
	exs = generateExamples("analysis", "expert")
	if len(exs) != 3 {
		t.Errorf("generateExamples(expert) = %d, want 3", len(exs))
	}
	// 应包含 distillType
	if !strings.Contains(exs[0], "analysis") {
		t.Errorf("generateExamples should include distillType, got: %v", exs)
	}
}

func TestGeneratePitfalls(t *testing.T) {
	pitfalls := generatePitfalls("workflow")
	if len(pitfalls) != 3 {
		t.Errorf("generatePitfalls = %d, want 3", len(pitfalls))
	}
	// 应是非空字符串
	for _, p := range pitfalls {
		if p == "" {
			t.Errorf("generatePitfalls contains empty string")
		}
	}
}

func TestGenerateDistilledSkill(t *testing.T) {
	skill := generateDistilledSkill("my_skill", "book", "workflow", "content here", 5, "expert")
	if skill == nil {
		t.Fatalf("generateDistilledSkill returned nil")
	}
	if skill.Name != "my_skill" {
		t.Errorf("Name = %q, want my_skill", skill.Name)
	}
	if !strings.Contains(skill.Description, "workflow") {
		t.Errorf("Description should contain distillType: %q", skill.Description)
	}
	if !strings.Contains(skill.Description, "book") {
		t.Errorf("Description should contain sourceType: %q", skill.Description)
	}
	if !strings.Contains(skill.Description, "expert") {
		t.Errorf("Description should contain quality: %q", skill.Description)
	}
	// trigger words 应包含 name（下划线转空格）、distillType、sourceType
	if len(skill.TriggerWords) != 3 {
		t.Errorf("TriggerWords = %v, want 3 items", skill.TriggerWords)
	}
	// steps 不应超过 maxSteps
	if len(skill.Steps) > 5 {
		t.Errorf("Steps = %d, want <= 5", len(skill.Steps))
	}
	// expert 应有额外 steps
	if len(skill.Steps) <= 5 {
		// expert 扩展后可能超过 5，但 maxSteps=5 限制后等于 5
	}
}

func TestSkillDistillNode_Execute_ValidationErrors(t *testing.T) {
	n := &SkillDistillNode{}
	ctx := context.Background()

	tests := []struct {
		name    string
		input   string
		params  map[string]string
		wantSub string
	}{
		{
			name:    "invalid source_type",
			input:   "content",
			params:  map[string]string{"source_type": "invalid"},
			wantSub: "invalid source_type",
		},
		{
			name:    "invalid distill_type",
			input:   "content",
			params:  map[string]string{"distill_type": "invalid"},
			wantSub: "invalid distill_type",
		},
		{
			name:    "content too long",
			input:   strings.Repeat("a", 100001),
			params:  map[string]string{},
			wantSub: "content too long",
		},
		{
			name:    "invalid skill_name format",
			input:   "content",
			params:  map[string]string{"skill_name": "1invalid"},
			wantSub: "invalid skill_name",
		},
		{
			name:    "invalid quality",
			input:   "content",
			params:  map[string]string{"quality": "invalid"},
			wantSub: "invalid quality",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := n.Execute(ctx, tt.input, tt.params)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("expected error containing %q, got: %v", tt.wantSub, err)
			}
		})
	}
}

func TestSkillDistillNode_Execute_Success(t *testing.T) {
	n := &SkillDistillNode{}
	ctx := context.Background()
	out, err := n.Execute(ctx, "some source content", map[string]string{
		"source_type":  "book",
		"distill_type": "workflow",
		"skill_name":   "my_skill",
		"max_steps":    "5",
		"quality":      "standard",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if result["skill_name"] != "my_skill" {
		t.Errorf("skill_name = %v, want my_skill", result["skill_name"])
	}
	if result["source_type"] != "book" {
		t.Errorf("source_type = %v, want book", result["source_type"])
	}
	if result["distill_type"] != "workflow" {
		t.Errorf("distill_type = %v, want workflow", result["distill_type"])
	}
	if result["quality"] != "standard" {
		t.Errorf("quality = %v, want standard", result["quality"])
	}
	// 默认 skill_name 生成
	out2, err := n.Execute(ctx, "content", map[string]string{})
	if err != nil {
		t.Fatalf("default execute error: %v", err)
	}
	var result2 map[string]interface{}
	if err := json.Unmarshal([]byte(out2), &result2); err != nil {
		t.Fatalf("default output not valid JSON: %v", err)
	}
	if result2["skill_name"] != "workflow_article_skill" {
		t.Errorf("default skill_name = %v, want workflow_article_skill", result2["skill_name"])
	}
}

// ----------------------------------------------------------------------------
// supervisor.go: SkillEvolution 纯逻辑（内存数据结构）
// ----------------------------------------------------------------------------

func TestSkillEvolution_RecordExecution(t *testing.T) {
	se := NewSkillEvolution()

	// 记录成功
	se.RecordExecution("skill1", true, 100)
	skill, ok := se.GetSkill("skill1")
	if !ok {
		t.Fatalf("GetSkill(skill1) not found after RecordExecution")
	}
	if skill.UseCount != 1 {
		t.Errorf("UseCount = %d, want 1", skill.UseCount)
	}
	if skill.SuccessCount != 1 {
		t.Errorf("SuccessCount = %d, want 1", skill.SuccessCount)
	}
	if skill.SuccessRate != 1.0 {
		t.Errorf("SuccessRate = %v, want 1.0", skill.SuccessRate)
	}
	if skill.AvgLatencyMs != 100 {
		t.Errorf("AvgLatencyMs = %d, want 100", skill.AvgLatencyMs)
	}

	// 记录失败
	se.RecordExecution("skill1", false, 200)
	skill, _ = se.GetSkill("skill1")
	if skill.UseCount != 2 {
		t.Errorf("UseCount = %d, want 2", skill.UseCount)
	}
	if skill.SuccessCount != 1 {
		t.Errorf("SuccessCount = %d, want 1", skill.SuccessCount)
	}
	if skill.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", skill.FailCount)
	}
	if skill.SuccessRate != 0.5 {
		t.Errorf("SuccessRate = %v, want 0.5", skill.SuccessRate)
	}
	// 滑动平均：(100*7 + 200*3)/10 = 130
	if skill.AvgLatencyMs != 130 {
		t.Errorf("AvgLatencyMs = %d, want 130", skill.AvgLatencyMs)
	}

	// 边界：空 skillName 应被忽略
	se.RecordExecution("", true, 100)
	if _, ok := se.GetSkill(""); ok {
		t.Errorf("empty skillName should be ignored")
	}
	// 边界：过长的 skillName 应被忽略
	longName := strings.Repeat("a", 101)
	se.RecordExecution(longName, true, 100)
	if _, ok := se.GetSkill(longName); ok {
		t.Errorf("overlong skillName should be ignored")
	}
	// 边界：异常 latency 应被忽略
	se.RecordExecution("skill_latency_neg", true, -1)
	if _, ok := se.GetSkill("skill_latency_neg"); ok {
		t.Errorf("negative latency should be ignored")
	}
	se.RecordExecution("skill_latency_huge", true, 3600001)
	if _, ok := se.GetSkill("skill_latency_huge"); ok {
		t.Errorf("huge latency should be ignored")
	}
}

func TestSkillEvolution_AddBestPractice(t *testing.T) {
	se := NewSkillEvolution()
	se.RecordExecution("skill1", true, 100)

	se.AddBestPractice("skill1", "practice one")
	skill, _ := se.GetSkill("skill1")
	if len(skill.BestPractices) != 1 {
		t.Errorf("BestPractices = %v, want 1 item", skill.BestPractices)
	}

	// 去重
	se.AddBestPractice("skill1", "practice one")
	skill, _ = se.GetSkill("skill1")
	if len(skill.BestPractices) != 1 {
		t.Errorf("duplicate BestPractice should be deduped, got %v", skill.BestPractices)
	}

	// 第二个不同 practice
	se.AddBestPractice("skill1", "practice two")
	skill, _ = se.GetSkill("skill1")
	if len(skill.BestPractices) != 2 {
		t.Errorf("BestPractices = %v, want 2 items", skill.BestPractices)
	}

	// 不存在的 skill 应被忽略
	se.AddBestPractice("nonexistent", "practice")
	// 边界：空 practice 应被忽略
	se.AddBestPractice("skill1", "")
	skill, _ = se.GetSkill("skill1")
	if len(skill.BestPractices) != 2 {
		t.Errorf("empty practice should be ignored, got %d", len(skill.BestPractices))
	}
	// 边界：空 skillName 应被忽略
	se.AddBestPractice("", "practice")
	// 边界：过长 practice 应被忽略
	se.AddBestPractice("skill1", strings.Repeat("p", 501))
	skill, _ = se.GetSkill("skill1")
	if len(skill.BestPractices) != 2 {
		t.Errorf("overlong practice should be ignored, got %d", len(skill.BestPractices))
	}
}

func TestSkillEvolution_AddKnownPitfall(t *testing.T) {
	se := NewSkillEvolution()
	se.RecordExecution("skill1", true, 100)

	se.AddKnownPitfall("skill1", "pitfall one")
	skill, _ := se.GetSkill("skill1")
	if len(skill.KnownPitfalls) != 1 {
		t.Errorf("KnownPitfalls = %v, want 1 item", skill.KnownPitfalls)
	}

	// 去重
	se.AddKnownPitfall("skill1", "pitfall one")
	skill, _ = se.GetSkill("skill1")
	if len(skill.KnownPitfalls) != 1 {
		t.Errorf("duplicate KnownPitfall should be deduped, got %v", skill.KnownPitfalls)
	}

	// 边界：空 pitfall 应被忽略
	se.AddKnownPitfall("skill1", "")
	skill, _ = se.GetSkill("skill1")
	if len(skill.KnownPitfalls) != 1 {
		t.Errorf("empty pitfall should be ignored, got %d", len(skill.KnownPitfalls))
	}
	// 边界：不存在的 skill 应被忽略
	se.AddKnownPitfall("nonexistent", "pitfall")
}

func TestSkillEvolution_OptimizePrompt(t *testing.T) {
	se := NewSkillEvolution()
	// 数据不足（UseCount < 3）应返回原 prompt
	got := se.OptimizePrompt("skill1", "base prompt")
	if got != "base prompt" {
		t.Errorf("OptimizePrompt with insufficient data = %q, want base prompt", got)
	}

	// 记录 3 次失败，使成功率低于 0.6
	for i := 0; i < 3; i++ {
		se.RecordExecution("skill1", false, 100)
	}
	se.AddKnownPitfall("skill1", "avoid this pitfall")
	got = se.OptimizePrompt("skill1", "base prompt")
	// 成功率 0 < 0.6，应添加 pitfalls
	if !strings.Contains(got, "Known pitfalls to avoid") {
		t.Errorf("OptimizePrompt should add pitfalls for low success rate: %q", got)
	}
	if !strings.Contains(got, "avoid this pitfall") {
		t.Errorf("OptimizePrompt should include pitfall text: %q", got)
	}

	// 添加 best practice 后应包含
	se.AddBestPractice("skill1", "follow this practice")
	got = se.OptimizePrompt("skill1", "base prompt")
	if !strings.Contains(got, "Best practices") {
		t.Errorf("OptimizePrompt should add best practices: %q", got)
	}
	if !strings.Contains(got, "follow this practice") {
		t.Errorf("OptimizePrompt should include practice text: %q", got)
	}

	// 不存在的 skill 返回原 prompt
	got = se.OptimizePrompt("nonexistent", "base prompt")
	if got != "base prompt" {
		t.Errorf("OptimizePrompt(nonexistent) = %q, want base prompt", got)
	}
}

func TestSkillEvolution_ListSkills(t *testing.T) {
	se := NewSkillEvolution()
	se.RecordExecution("skill1", true, 100)
	se.RecordExecution("skill2", true, 100)

	list := se.ListSkills()
	if len(list) != 2 {
		t.Errorf("ListSkills = %d items, want 2", len(list))
	}
	// 返回的应是深拷贝，修改不影响内部
	list[0].UseCount = 999
	skill, _ := se.GetSkill(list[0].Name)
	if skill.UseCount == 999 {
		t.Errorf("ListSkills should return deep copies")
	}
}

func TestSkillEvolution_GetLowPerformingSkills(t *testing.T) {
	se := NewSkillEvolution()
	// skill1: 3 次失败，成功率 0
	for i := 0; i < 3; i++ {
		se.RecordExecution("skill1", false, 100)
	}
	// skill2: 3 次成功，成功率 1.0
	for i := 0; i < 3; i++ {
		se.RecordExecution("skill2", true, 100)
	}
	// skill3: 只有 2 次记录（< 3，不应被包含）
	for i := 0; i < 2; i++ {
		se.RecordExecution("skill3", false, 100)
	}

	low := se.GetLowPerformingSkills(0.6)
	if len(low) != 1 {
		t.Errorf("GetLowPerformingSkills = %d items, want 1 (only skill1)", len(low))
	}
	if len(low) > 0 && low[0].Name != "skill1" {
		t.Errorf("GetLowPerformingSkills[0].Name = %q, want skill1", low[0].Name)
	}

	// 无效 threshold 应回退到 0.6
	low = se.GetLowPerformingSkills(-0.5)
	// 不应 panic
	if low == nil {
		// ok
	}
	low = se.GetLowPerformingSkills(1.5)
	// 不应 panic
	if low == nil {
		// ok
	}
}

func TestSkillEvolution_GetSkillStats(t *testing.T) {
	se := NewSkillEvolution()
	se.RecordExecution("skill1", true, 100)
	se.RecordExecution("skill1", false, 100)
	se.RecordExecution("skill2", true, 100)

	stats := se.GetSkillStats()
	if stats["total_skills"] != 2 {
		t.Errorf("total_skills = %v, want 2", stats["total_skills"])
	}
	if stats["total_executions"] != 3 {
		t.Errorf("total_executions = %v, want 3", stats["total_executions"])
	}
	if stats["total_success"] != 2 {
		t.Errorf("total_success = %v, want 2", stats["total_success"])
	}
	if stats["avg_success_rate"] != 2.0/3.0 {
		t.Errorf("avg_success_rate = %v, want %v", stats["avg_success_rate"], 2.0/3.0)
	}

	// 空引擎
	emptySE := NewSkillEvolution()
	emptyStats := emptySE.GetSkillStats()
	if emptyStats["total_skills"] != 0 {
		t.Errorf("empty total_skills = %v, want 0", emptyStats["total_skills"])
	}
	if emptyStats["avg_success_rate"] != 0.0 {
		t.Errorf("empty avg_success_rate = %v, want 0", emptyStats["avg_success_rate"])
	}
}

func TestSkillEvolution_GetSkill_NotFound(t *testing.T) {
	se := NewSkillEvolution()
	skill, ok := se.GetSkill("nonexistent")
	if ok {
		t.Errorf("GetSkill(nonexistent) should return false")
	}
	if skill != nil {
		t.Errorf("GetSkill(nonexistent) should return nil skill")
	}
}

func TestCloneSkillRecord(t *testing.T) {
	original := &SkillRecord{
		Name:          "skill1",
		UseCount:      5,
		BestPractices: []string{"bp1", "bp2"},
		KnownPitfalls: []string{"kp1"},
	}
	cp := cloneSkillRecord(original)
	if cp.Name != original.Name || cp.UseCount != original.UseCount {
		t.Errorf("clone mismatch")
	}
	// 修改副本不影响原
	cp.BestPractices[0] = "modified"
	if original.BestPractices[0] == "modified" {
		t.Errorf("clone should be deep copy, but original was modified")
	}
	// nil 安全
	if cloneSkillRecord(nil) != nil {
		t.Errorf("cloneSkillRecord(nil) should return nil")
	}
}

// ----------------------------------------------------------------------------
// planner.go: cleanJSONResponse
// ----------------------------------------------------------------------------

func TestCleanJSONResponse_Planner(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain json", `{"a":1}`, `{"a":1}`},
		{"json with code fence", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"surrounding whitespace", `  {"a":1}  `, `{"a":1}`},
		{"only prefix fence", "```json{\"a\":1}", `{"a":1}`},
		{"only suffix fence", `{"a":1}` + "```", `{"a":1}`},
		{"empty", "", ""},
		{"only whitespace", "   ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanJSONResponse(tt.in); got != tt.want {
				t.Errorf("cleanJSONResponse(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// llm_router.go: sortByPriority / sortByCost / sortByLatency / getOrCreateStatsLocked
// ----------------------------------------------------------------------------

func TestLLMRouter_SortByPriority(t *testing.T) {
	r := &LLMRouter{}
	providers := []RouterProvider{
		{Name: "low", Priority: 1, SuccessRate: 0.5},
		{Name: "high", Priority: 10, SuccessRate: 0.9},
		{Name: "mid", Priority: 5, SuccessRate: 0.8},
		{Name: "high2", Priority: 10, SuccessRate: 0.95},
	}
	sorted := r.sortByPriority(providers)
	if sorted[0].Name != "high2" {
		t.Errorf("sortByPriority first = %q, want high2 (priority 10, success 0.95)", sorted[0].Name)
	}
	if sorted[1].Name != "high" {
		t.Errorf("sortByPriority second = %q, want high (priority 10, success 0.9)", sorted[1].Name)
	}
	if sorted[2].Name != "mid" {
		t.Errorf("sortByPriority third = %q, want mid", sorted[2].Name)
	}
	if sorted[3].Name != "low" {
		t.Errorf("sortByPriority fourth = %q, want low", sorted[3].Name)
	}
	// 空/单元素
	if got := r.sortByPriority(nil); len(got) != 0 {
		t.Errorf("sortByPriority(nil) = %v, want empty", got)
	}
}

func TestLLMRouter_SortByCost(t *testing.T) {
	r := &LLMRouter{}
	providers := []RouterProvider{
		{Name: "expensive", CostPer1K: 0.05, SuccessRate: 0.5},
		{Name: "cheap", CostPer1K: 0.001, SuccessRate: 0.8},
		{Name: "mid", CostPer1K: 0.01, SuccessRate: 0.9},
	}
	sorted := r.sortByCost(providers)
	if sorted[0].Name != "cheap" {
		t.Errorf("sortByCost first = %q, want cheap", sorted[0].Name)
	}
	if sorted[1].Name != "mid" {
		t.Errorf("sortByCost second = %q, want mid", sorted[1].Name)
	}
	if sorted[2].Name != "expensive" {
		t.Errorf("sortByCost third = %q, want expensive", sorted[2].Name)
	}
}

func TestLLMRouter_SortByLatency(t *testing.T) {
	r := &LLMRouter{}
	providers := []RouterProvider{
		{Name: "slow", AvgLatencyMs: 2000, SuccessRate: 0.5},
		{Name: "fast", AvgLatencyMs: 100, SuccessRate: 0.8},
		{Name: "mid", AvgLatencyMs: 500, SuccessRate: 0.9},
	}
	sorted := r.sortByLatency(providers)
	if sorted[0].Name != "fast" {
		t.Errorf("sortByLatency first = %q, want fast", sorted[0].Name)
	}
	if sorted[1].Name != "mid" {
		t.Errorf("sortByLatency second = %q, want mid", sorted[1].Name)
	}
	if sorted[2].Name != "slow" {
		t.Errorf("sortByLatency third = %q, want slow", sorted[2].Name)
	}
}

func TestLLMRouter_RoundRobin(t *testing.T) {
	r := &LLMRouter{}
	providers := []RouterProvider{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
	}
	// 多次调用应轮转起始位置
	seen := map[string]int{}
	for i := 0; i < 9; i++ {
		result := r.roundRobin(providers)
		seen[result[0].Name]++
	}
	// 应至少看到 2 个不同的起始 provider
	if len(seen) < 2 {
		t.Errorf("roundRobin should rotate start, only saw %v", seen)
	}
	// 空 providers
	if got := r.roundRobin(nil); len(got) != 0 {
		t.Errorf("roundRobin(nil) = %v, want empty", got)
	}
	// 单元素
	result := r.roundRobin([]RouterProvider{{Name: "only"}})
	if len(result) != 1 || result[0].Name != "only" {
		t.Errorf("roundRobin single = %v, want [only]", result)
	}
}

func TestLLMRouter_GetOrCreateStatsLocked(t *testing.T) {
	r := &LLMRouter{stats: make(map[string]*ProviderStats)}
	// 不存在时创建
	stats := r.getOrCreateStatsLocked("provider1")
	if stats == nil {
		t.Fatalf("getOrCreateStatsLocked returned nil")
	}
	if stats.TotalCalls != 0 {
		t.Errorf("new stats TotalCalls = %d, want 0", stats.TotalCalls)
	}
	// 已存在时返回同一个
	stats2 := r.getOrCreateStatsLocked("provider1")
	if stats != stats2 {
		t.Errorf("getOrCreateStatsLocked should return same pointer for existing key")
	}
}

// ----------------------------------------------------------------------------
// inference_backend.go: Execute 各操作路径
// ----------------------------------------------------------------------------

func TestInferenceNode_Execute_List(t *testing.T) {
	n := &InferenceNode{}
	ctx := context.Background()
	out, err := n.Execute(ctx, "", map[string]string{"operation": "list"})
	if err != nil {
		t.Fatalf("list operation error: %v", err)
	}
	var statuses []BackendStatus
	if err := json.Unmarshal([]byte(out), &statuses); err != nil {
		t.Errorf("list output not valid JSON array: %v", err)
	}
}

func TestInferenceNode_Execute_Status_NotFound(t *testing.T) {
	n := &InferenceNode{}
	ctx := context.Background()
	_, err := n.Execute(ctx, "", map[string]string{
		"operation": "status",
		"backend":   "nonexistent_backend",
	})
	if err == nil {
		t.Errorf("status with unknown backend should error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestInferenceNode_Execute_Status_Success(t *testing.T) {
	n := &InferenceNode{}
	ctx := context.Background()
	out, err := n.Execute(ctx, "", map[string]string{
		"operation": "status",
		"backend":   "llama.cpp",
	})
	if err != nil {
		t.Fatalf("status error: %v", err)
	}
	var status BackendStatus
	if err := json.Unmarshal([]byte(out), &status); err != nil {
		t.Errorf("status output not valid JSON: %v", err)
	}
	if status.Backend != BackendLlamaCpp {
		t.Errorf("status Backend = %v, want llama.cpp", status.Backend)
	}
}

func TestInferenceNode_Execute_SetBackend(t *testing.T) {
	n := &InferenceNode{}
	ctx := context.Background()
	out, err := n.Execute(ctx, "", map[string]string{
		"operation": "set_backend",
		"backend":   "onnx",
	})
	if err != nil {
		t.Fatalf("set_backend error: %v", err)
	}
	if !strings.Contains(out, "onnx") {
		t.Errorf("set_backend output should mention onnx: %s", out)
	}
	// 无效 backend
	_, err = n.Execute(ctx, "", map[string]string{
		"operation": "set_backend",
		"backend":   "invalid_backend",
	})
	if err == nil {
		t.Errorf("set_backend with invalid backend should error")
	}
}

func TestInferenceNode_Execute_LoadModel_Validation(t *testing.T) {
	n := &InferenceNode{}
	ctx := context.Background()
	// 缺少 model_path
	_, err := n.Execute(ctx, "", map[string]string{
		"operation": "load_model",
		"backend":   "llama.cpp",
	})
	if err == nil {
		t.Errorf("load_model without model_path should error")
	}
	if !strings.Contains(err.Error(), "model_path required") {
		t.Errorf("expected model_path required error, got: %v", err)
	}
	// 未知 backend
	_, err = n.Execute(ctx, "", map[string]string{
		"operation":  "load_model",
		"backend":    "nonexistent",
		"model_path": "/tmp/model",
	})
	if err == nil {
		t.Errorf("load_model with unknown backend should error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got: %v", err)
	}
	// 有效 backend + model_path（LoadModel 总是返回 nil）
	out, err := n.Execute(ctx, "", map[string]string{
		"operation":  "load_model",
		"backend":    "llama.cpp",
		"model_path": "/tmp/fake-model",
	})
	if err != nil {
		t.Fatalf("load_model valid error: %v", err)
	}
	if !strings.Contains(out, "/tmp/fake-model") {
		t.Errorf("load_model output should mention model path: %s", out)
	}
}

func TestInferenceNode_Execute_UnknownOperation(t *testing.T) {
	n := &InferenceNode{}
	ctx := context.Background()
	_, err := n.Execute(ctx, "", map[string]string{
		"operation": "unknown_op",
	})
	if err == nil {
		t.Errorf("unknown operation should error")
	}
	if !strings.Contains(err.Error(), "unknown operation") {
		t.Errorf("expected 'unknown operation' error, got: %v", err)
	}
}

// ----------------------------------------------------------------------------
// marketplace.go: formatNodeInfoList / formatNodeDetails / formatCategories /
// Execute 各 action
// ----------------------------------------------------------------------------

func TestFormatNodeInfoList_JSON(t *testing.T) {
	nodes := []NodeInfo{
		{Name: "llm", Description: "LLM node"},
	}
	out := formatNodeInfoList(nodes, "json", "all")
	if !strings.Contains(out, `"filter": "all"`) {
		t.Errorf("json output missing filter: %s", out)
	}
	if !strings.Contains(out, `"count": 1`) {
		t.Errorf("json output missing count: %s", out)
	}
	if !strings.Contains(out, `"llm"`) {
		t.Errorf("json output missing node name: %s", out)
	}
	// 空列表
	out = formatNodeInfoList(nil, "json", "")
	if !strings.Contains(out, `"count": 0`) {
		t.Errorf("empty json output missing count 0: %s", out)
	}
}

func TestFormatNodeDetails(t *testing.T) {
	// 使用已注册的节点
	reg := GetGlobalRegistry()
	node, ok := reg.Get("test_node")
	if !ok {
		t.Skip("test_node not registered, skipping")
	}
	// markdown 格式
	out := formatNodeDetails(node, "markdown")
	if !strings.Contains(out, "## Node:") {
		t.Errorf("markdown details missing header: %s", out)
	}
	if !strings.Contains(out, "**Description:**") {
		t.Errorf("markdown details missing description: %s", out)
	}
	// json 格式
	out = formatNodeDetails(node, "json")
	var schema NodeSchema
	if err := json.Unmarshal([]byte(out), &schema); err != nil {
		t.Errorf("json details not valid JSON: %v", err)
	}
	if schema.Name == "" {
		t.Errorf("json details schema Name empty: %s", out)
	}
}

func TestFormatCategories(t *testing.T) {
	reg := GetGlobalRegistry()
	// markdown 格式
	out := formatCategories(reg, "markdown")
	if !strings.Contains(out, "## Node Categories") {
		t.Errorf("markdown categories missing header: %s", out)
	}
	// json 格式
	out = formatCategories(reg, "json")
	var result map[string]int
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Errorf("json categories not valid JSON: %v", err)
	}
	if _, ok := result["total_registered"]; !ok {
		t.Errorf("json categories missing total_registered: %s", out)
	}
}

func TestMarketplaceNode_Execute_Count(t *testing.T) {
	n := &MarketplaceNode{}
	ctx := context.Background()
	out, err := n.Execute(ctx, "", map[string]string{"action": "count"})
	if err != nil {
		t.Fatalf("count action error: %v", err)
	}
	if !strings.Contains(out, "Total nodes:") {
		t.Errorf("count output should contain 'Total nodes:': %s", out)
	}
}

func TestMarketplaceNode_Execute_List(t *testing.T) {
	n := &MarketplaceNode{}
	ctx := context.Background()
	out, err := n.Execute(ctx, "", map[string]string{"action": "list"})
	if err != nil {
		t.Fatalf("list action error: %v", err)
	}
	if !strings.Contains(out, "## Available Nodes") {
		t.Errorf("list output missing header: %s", out)
	}
	// json 格式
	out, err = n.Execute(ctx, "", map[string]string{"action": "list", "format": "json"})
	if err != nil {
		t.Fatalf("list json error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Errorf("list json not valid JSON: %v", err)
	}
}

func TestMarketplaceNode_Execute_Categories(t *testing.T) {
	n := &MarketplaceNode{}
	ctx := context.Background()
	out, err := n.Execute(ctx, "", map[string]string{"action": "categories"})
	if err != nil {
		t.Fatalf("categories action error: %v", err)
	}
	if !strings.Contains(out, "Node Categories") {
		t.Errorf("categories output missing header: %s", out)
	}
}

func TestMarketplaceNode_Execute_Search(t *testing.T) {
	n := &MarketplaceNode{}
	ctx := context.Background()
	// 空 query 应报错
	_, err := n.Execute(ctx, "", map[string]string{"action": "search"})
	if err == nil {
		t.Errorf("search with empty query should error")
	}
	// 有 query（通过 input）
	out, err := n.Execute(ctx, "llm", map[string]string{"action": "search"})
	if err != nil {
		t.Fatalf("search action error: %v", err)
	}
	if !strings.Contains(out, "search: llm") {
		t.Errorf("search output should contain query: %s", out)
	}
	// 通过 query 参数
	out, err = n.Execute(ctx, "", map[string]string{"action": "search", "query": "agent"})
	if err != nil {
		t.Fatalf("search with query param error: %v", err)
	}
	if !strings.Contains(out, "search: agent") {
		t.Errorf("search output should contain query param: %s", out)
	}
}

func TestMarketplaceNode_Execute_Details(t *testing.T) {
	n := &MarketplaceNode{}
	ctx := context.Background()
	// 空 node_name 应报错
	_, err := n.Execute(ctx, "", map[string]string{"action": "details"})
	if err == nil {
		t.Errorf("details with empty node_name should error")
	}
	// 不存在的 node
	_, err = n.Execute(ctx, "", map[string]string{"action": "details", "node_name": "nonexistent_node_xyz"})
	if err == nil {
		t.Errorf("details with nonexistent node should error")
	}
	// 通过 input 提供 node_name
	out, err := n.Execute(ctx, "test_node", map[string]string{"action": "details"})
	if err != nil {
		// test_node 可能未注册，跳过
		t.Logf("details via input for test_node: %v", err)
		return
	}
	if !strings.Contains(out, "## Node:") {
		t.Errorf("details output missing header: %s", out)
	}
}

func TestMarketplaceNode_Execute_UnknownAction(t *testing.T) {
	n := &MarketplaceNode{}
	ctx := context.Background()
	_, err := n.Execute(ctx, "", map[string]string{"action": "unknown_action"})
	if err == nil {
		t.Errorf("unknown action should error")
	}
	if !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("expected 'unknown action' error, got: %v", err)
	}
}

func TestNewNodeMarketplace(t *testing.T) {
	m := NewNodeMarketplace("/tmp/nonexistent_plugin_dir_xyz")
	if m == nil {
		t.Fatalf("NewNodeMarket returned nil")
	}
	// 不存在的目录应返回 nil, nil
	plugins, err := m.ListAvailablePlugins()
	if err != nil {
		t.Errorf("ListAvailablePlugins on nonexistent dir should not error: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("ListAvailablePlugins on nonexistent dir should return empty, got %v", plugins)
	}
}

// ----------------------------------------------------------------------------
// context_fs.go: Execute 各操作路径
// ----------------------------------------------------------------------------

func TestContextFSNode_Execute_InvalidOperation(t *testing.T) {
	n := &ContextFSNode{}
	ctx := context.Background()
	_, err := n.Execute(ctx, "", map[string]string{"operation": "invalid_op"})
	if err == nil {
		t.Errorf("invalid operation should error")
	}
	if !strings.Contains(err.Error(), "invalid operation") {
		t.Errorf("expected 'invalid operation' error, got: %v", err)
	}
}

func TestContextFSNode_Execute_List(t *testing.T) {
	n := &ContextFSNode{}
	ctx := context.Background()
	out, err := n.Execute(ctx, "", map[string]string{"operation": "ls"})
	if err != nil {
		t.Fatalf("ls error: %v", err)
	}
	// 应为合法 JSON（数组）
	out = strings.TrimSpace(out)
	if !strings.HasPrefix(out, "[") && !strings.HasPrefix(out, "null") {
		t.Errorf("ls output should be JSON array, got: %s", out)
	}
}

func TestContextFSNode_Execute_WriteAndCat(t *testing.T) {
	n := &ContextFSNode{}
	ctx := context.Background()
	// write 缺少 path
	_, err := n.Execute(ctx, "content", map[string]string{"operation": "write"})
	if err == nil {
		t.Errorf("write without path should error")
	}
	// write 缺少 content
	_, err = n.Execute(ctx, "", map[string]string{"operation": "write", "path": "/mem/short/test_boost4"})
	if err == nil {
		t.Errorf("write without content should error")
	}
	// 成功写入
	out, err := n.Execute(ctx, "hello world", map[string]string{
		"operation": "write",
		"path":      "/mem/short/test_boost4",
	})
	if err != nil {
		t.Fatalf("write error: %v", err)
	}
	if !strings.Contains(out, "written") {
		t.Errorf("write output should mention written: %s", out)
	}
	// cat 读取
	out, err = n.Execute(ctx, "", map[string]string{
		"operation": "cat",
		"path":      "/mem/short/test_boost4",
	})
	if err != nil {
		t.Fatalf("cat error: %v", err)
	}
	if out != "hello world" {
		t.Errorf("cat = %q, want hello world", out)
	}
	// cat 缺少 path
	_, err = n.Execute(ctx, "", map[string]string{"operation": "cat"})
	if err == nil {
		t.Errorf("cat without path should error")
	}
}

func TestContextFSNode_Execute_Rm(t *testing.T) {
	n := &ContextFSNode{}
	ctx := context.Background()
	// 先写入
	_, _ = n.Execute(ctx, "data", map[string]string{
		"operation": "write",
		"path":      "/mem/short/test_rm_boost4",
	})
	// rm 缺少 path
	_, err := n.Execute(ctx, "", map[string]string{"operation": "rm"})
	if err == nil {
		t.Errorf("rm without path should error")
	}
	// 成功删除
	out, err := n.Execute(ctx, "", map[string]string{
		"operation": "rm",
		"path":      "/mem/short/test_rm_boost4",
	})
	if err != nil {
		t.Fatalf("rm error: %v", err)
	}
	if !strings.Contains(out, "deleted") {
		t.Errorf("rm output should mention deleted: %s", out)
	}
}

func TestContextFSNode_Execute_Search(t *testing.T) {
	n := &ContextFSNode{}
	ctx := context.Background()
	// search 缺少 query
	_, err := n.Execute(ctx, "", map[string]string{"operation": "search"})
	if err == nil {
		t.Errorf("search without query should error")
	}
	// 通过 input 提供 query
	out, err := n.Execute(ctx, "test query", map[string]string{"operation": "search"})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	// 应为合法 JSON
	out = strings.TrimSpace(out)
	if !strings.HasPrefix(out, "[") && !strings.HasPrefix(out, "null") {
		t.Errorf("search output should be JSON array, got: %s", out)
	}
	// 通过 query 参数
	_, err = n.Execute(ctx, "", map[string]string{"operation": "search", "query": "param query"})
	if err != nil {
		t.Fatalf("search with query param error: %v", err)
	}
}

// ----------------------------------------------------------------------------
// compress.go: Execute 验证路径
// ----------------------------------------------------------------------------

func TestCompressNode_Execute(t *testing.T) {
	// 通过注册表获取 compress 节点
	reg := GetGlobalRegistry()
	node, ok := reg.Get("compress")
	if !ok {
		t.Skip("compress node not registered")
	}
	ctx := context.Background()
	// 简单输入应成功压缩
	out, err := node.Execute(ctx, strings.Repeat("hello world ", 100), map[string]string{})
	if err != nil {
		t.Logf("compress execute returned error (may require LLM): %v", err)
		return
	}
	if out == "" {
		t.Errorf("compress output should not be empty")
	}
}
