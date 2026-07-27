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
	"strings"
	"testing"
	"time"
)

func TestGenerateSessionID(t *testing.T) {
	id1 := generateSessionID()
	if !strings.HasPrefix(id1, "cli-session-") {
		t.Errorf("expected cli-session- prefix, got %s", id1)
	}
	id2 := generateSessionID()
	if id1 == id2 {
		t.Error("expected different session IDs")
	}
}

func TestSimulateCLISessionResponse(t *testing.T) {
	tests := []struct {
		input   string
		wantSub string
	}{
		{"clear", "cleared"},
		{"清屏", "cleared"},
		{"exit", "会话结束"},
		{"quit", "会话结束"},
		{"退出", "会话结束"},
		{"history", "历史记录"},
		{"历史", "历史记录"},
		{"help", "可用命令"},
		{"帮助", "可用命令"},
		{"model", "当前模型"},
		{"theme", "主题切换"},
		{"ls", "模拟命令执行"},
		{"echo hello", "hello"},
		{"random input here", "gpt-4"},
	}
	for _, tt := range tests {
		got := simulateCLISessionResponse(tt.input, "gpt-4", 5)
		if !strings.Contains(got, tt.wantSub) {
			t.Errorf("simulateCLISessionResponse(%q) = %q, want substring %q", tt.input, got, tt.wantSub)
		}
	}
}

func TestCLISessionNode_Metadata(t *testing.T) {
	node := &CLISessionNode{}
	if node.Name() == "" {
		t.Error("expected non-empty name")
	}
	if node.Description() == "" {
		t.Error("expected non-empty description")
	}
	schema := node.Schema()
	if schema.Name == "" {
		t.Error("expected non-empty schema name")
	}
}

func TestCLISessionNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &CLISessionNode{}
	_, err := node.Execute(ctx, "", map[string]string{"theme": "invalid"})
	if err == nil {
		t.Error("expected error for invalid theme")
	}
	_, err = node.Execute(ctx, strings.Repeat("a", 10001), map[string]string{})
	if err == nil {
		t.Error("expected error for too long input")
	}
}

func TestMoreNodeMetadata(t *testing.T) {
	nodes := []Node{
		&AgentBrowserNode{},
		&CodeReviewNode{},
		&SearchAggregateNode{},
		&OutputQualityNode{},
		&LLMRouterNode{},
		&SwarmCommNode{},
		&ClarifyNode{},
	}
	for _, n := range nodes {
		name := n.Name()
		if name == "" {
			t.Error("node has empty name")
		}
		if n.Description() == "" {
			t.Errorf("node %s has empty description", name)
		}
		schema := n.Schema()
		if schema.Name == "" {
			t.Errorf("node %s has empty schema name", name)
		}
	}
}

func TestClarifyNode_Execute(t *testing.T) {
	ctx := context.Background()
	node := &ClarifyNode{}
	_, err := node.Execute(ctx, "test", map[string]string{})
	if err != nil {
		t.Logf("ClarifyNode error: %v", err)
	}
}

func TestCleanupExpiredCLISessions(t *testing.T) {
	cleanupExpiredCLISessions()
	cliSessionLastUsedMu.Lock()
	for i := 0; i < 600; i++ {
		cliSessionLastUsed[generateSessionID()] = time.Now().Add(-48 * time.Hour)
	}
	cliSessionLastUsedMu.Unlock()
	cleanupExpiredCLISessions()
}

func TestAllRegisteredNodesMetadata(t *testing.T) {
	reg := GetGlobalRegistry()
	infos := reg.ListNodes()
	if len(infos) == 0 {
		t.Fatal("no registered nodes")
	}
	t.Logf("Testing %d registered nodes", len(infos))

	for _, info := range infos {
		node, ok := reg.Get(info.Name)
		if !ok {
			t.Errorf("node %q not found", info.Name)
			continue
		}
		if node.Name() == "" {
			t.Errorf("node %q has empty name", info.Name)
		}
		if node.Description() == "" {
			t.Errorf("node %q has empty description", info.Name)
		}
		schema := node.Schema()
		if schema.Name == "" {
			t.Errorf("node %q has empty schema name", info.Name)
		}
	}
}

func TestDetermineExecLocation(t *testing.T) {
	tests := []struct {
		modelSize    string
		endCloudMode string
		input        string
		expected     string
	}{
		{"tiny", "auto", "test", "on_device"},
		{"titan", "auto", "test", "cloud"},
		{"turbo", "auto", "short", "on_device"},
		{"turbo", "auto", strings.Repeat("a", 150), "end_cloud_collaboration"},
		{"any", "force_end", "test", "on_device"},
		{"any", "force_cloud", "test", "cloud"},
		{"unknown", "auto", "test", "cloud"},
	}
	for _, tt := range tests {
		got := determineExecLocation(tt.modelSize, tt.endCloudMode, tt.input)
		if got != tt.expected {
			t.Errorf("determineExecLocation(%q, %q, %q) = %q, want %q",
				tt.modelSize, tt.endCloudMode, tt.input, got, tt.expected)
		}
	}
}

func TestSimulateAndesResponse(t *testing.T) {
	tests := []struct {
		scene   string
		wantSub string
	}{
		{"life", "生活助手"},
		{"imaging", "影像AI"},
		{"productivity", "效率助手"},
		{"creative", "创作助手"},
		{"knowledge", "知识问答"},
		{"unknown", "AndesGPT"},
	}
	for _, tt := range tests {
		got := simulateAndesResponse("test input", "turbo", tt.scene, "", false, "", 100)
		if !strings.Contains(got, tt.wantSub) {
			t.Errorf("simulateAndesResponse(scene=%q) = %q, want substring %q", tt.scene, got, tt.wantSub)
		}
	}

	got := simulateAndesResponse("推荐餐厅", "turbo", "life", "user123", true, "", 100)
	if !strings.Contains(got, "个人偏好") {
		t.Errorf("expected memory+persona response, got %q", got)
	}
}

func TestSimulateAntLingResponse(t *testing.T) {
	tests := []struct {
		scene   string
		wantSub string
	}{
		{"chat", "蚂蚁百灵"},
		{"code", "```"},
		{"analysis", "深度分析"},
		{"creative", "创意内容"},
		{"unknown", "蚂蚁百灵"},
	}
	for _, tt := range tests {
		got := simulateAntLingResponse("test", "ling-max", tt.scene, "", 100)
		if !strings.Contains(got, tt.wantSub) {
			t.Errorf("simulateAntLingResponse(scene=%q) = %q, want substring %q", tt.scene, got, tt.wantSub)
		}
	}

	got := simulateAntLingResponse("image task", "ming-flash-omni-2.0", "multimodal", "", 100)
	if !strings.Contains(got, "多模态") {
		t.Errorf("expected multimodal response, got %q", got)
	}

	got = simulateAntLingResponse("image task", "ling-max", "multimodal", "", 100)
	if !strings.Contains(got, "不支持原生多模态") {
		t.Errorf("expected non-multimodal response, got %q", got)
	}
}

func TestDetectStructureIssues(t *testing.T) {
	node := &OutputQualityNode{}

	uniformText := "aaaaaaaaaa\nbbbbbbbbbb\ncccccccccc\ndddddddddd\neeeeeeeeee"
	findings := node.detectStructureIssues(uniformText, "en")
	if len(findings) == 0 {
		t.Log("uniform text expected to trigger STR-001")
	}

	uniformSentences := "Hello world. How are you. I am fine. This is test. Another day here."
	findings = node.detectStructureIssues(uniformSentences, "en")
	if len(findings) == 0 {
		t.Log("uniform sentences expected to trigger STR-002")
	}

	shortText := "short"
	findings = node.detectStructureIssues(shortText, "en")
	if len(findings) != 0 {
		t.Errorf("short text should have no structure issues, got %d", len(findings))
	}
}

func TestTruncateOutput(t *testing.T) {
	short := "short text"
	result := truncateOutput(short, 100)
	if result != short {
		t.Errorf("short text should not be truncated, got %q", result)
	}

	long := strings.Repeat("a", 200)
	result = truncateOutput(long, 100)
	if len(result) > 100+20 {
		t.Errorf("long text should be truncated, got %d chars", len(result))
	}
}

func TestSimulateQualityAssessment(t *testing.T) {
	types := []string{"ai_detection", "design_quality", "code_quality", "unknown"}
	for _, at := range types {
		score, issues, suggestions, aiProb, _ := simulateQualityAssessment("test content", at)
		if score < 0 || score > 1 {
			t.Errorf("score out of range for %s: %f", at, score)
		}
		if aiProb < 0 || aiProb > 1 {
			t.Errorf("aiProb out of range for %s: %f", at, aiProb)
		}
		_ = issues
		_ = suggestions
	}

	_, issues, suggestions, _, _ := simulateQualityAssessment("short", "design_quality")
	if len(issues) == 0 {
		t.Log("short design content expected to have issues")
	}
	_ = suggestions

	_, issues, _, _, _ = simulateQualityAssessment("code with TODO", "code_quality")
	if len(issues) == 0 {
		t.Log("code with TODO expected to have issues")
	}
}

func TestSimulateAutoFix(t *testing.T) {
	result := simulateAutoFix("original content", "code_quality", []string{"add tests"})
	if result == "" {
		t.Error("expected non-empty result")
	}
	if !strings.Contains(result, "original") {
		t.Error("expected original content to be included")
	}
}

func TestSimulateSenseNovaResponse(t *testing.T) {
	scenes := []string{"chat", "code", "image", "document", "data", "workflow", "unknown"}
	for _, s := range scenes {
		got := simulateSenseNovaResponse("test", "sense-max", s, "", 100)
		if got == "" {
			t.Errorf("empty response for scene %s", s)
		}
	}

	got := simulateSenseNovaResponse("test", "u1-max", "image", "", 100)
	if !strings.Contains(got, "U1") {
		t.Errorf("expected U1 in image response, got %q", got)
	}
}

func TestSimulateSkillApplication(t *testing.T) {
	skill := SkillDetail{Name: "test-skill"}
	result := simulateSkillApplication(skill, "do something")
	if result == "" {
		t.Error("expected non-empty result")
	}
	if !strings.Contains(result, "test-skill") {
		t.Errorf("expected skill name in result, got %q", result)
	}
}

func TestFormatBytesAndInt(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{2048, "2 KB"},
		{1048576, "1 MB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.n)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}

	if formatInt(-42) != "-42" {
		t.Error("formatInt(-42) should be -42")
	}
}

func TestIsSensitiveFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"", false},
		{".env", true},
		{".env.local", true},
		{"credentials.json", true},
		{"id_rsa.pem", true},
		{"server.key", true},
		{"private_key.pem", true},
		{"main.go", false},
		{"README.md", false},
	}
	for _, tt := range tests {
		got := IsSensitiveFile(tt.filename)
		if got != tt.want {
			t.Errorf("IsSensitiveFile(%q) = %v, want %v", tt.filename, got, tt.want)
		}
	}
}

func TestValidateGitURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"", false},
		{"git@github.com:user/repo.git", true},
		{"https://github.com/user/repo", true},
		{"https://gitlab.com/user/repo", true},
		{"https://gitee.com/user/repo", true},
		{"ftp://example.com/repo", false},
	}
	for _, tt := range tests {
		got := validateGitURL(tt.url)
		if got != tt.want {
			t.Errorf("validateGitURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestValidatePluginURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"", false},
		{"https://example.com/plugin", true},
		{"http://example.com/plugin", false},
		{"https://localhost/plugin", false},
		{"https://127.0.0.1/plugin", false},
	}
	for _, tt := range tests {
		got := validatePluginURL(tt.url)
		if got != tt.want {
			t.Errorf("validatePluginURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestValidateMarketID(t *testing.T) {
	if validateMarketID("") {
		t.Error("empty id should be invalid")
	}
	if !validateMarketID("valid-id-123") {
		t.Error("valid id should pass")
	}
}

func TestLooksLikeURL(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"https://example.com", true},
		{"http://example.com", true},
		{"not a url", false},
		{"", false},
	}
	for _, tt := range tests {
		got := looksLikeURL(tt.s)
		if got != tt.want {
			t.Errorf("looksLikeURL(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}
