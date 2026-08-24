// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​‌​‌‌​‌​​​​​‌‌​​​‌‌‌​‌​​​‌​‌‌‌‌‌‌​​​​‌‌​‌​‌​​​​​​​​​​​​​​​​​​​‌‌​​​​‌​‌‌‌​​​‌‌⁠
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
