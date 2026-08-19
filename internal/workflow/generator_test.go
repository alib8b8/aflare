// Copyright (c) 2026 aflare Contributors
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

package workflow

import (
	"os"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/i18n"
)

func TestGenerateWorkflow_DefaultModel(t *testing.T) {
	wf, err := GenerateWorkflow("summarize this text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should default to ollama
	foundLLM := false
	for _, step := range wf.Steps {
		if step.Node == "ollama" {
			foundLLM = true
			if step.Params["model"] != "llama3" {
				t.Errorf("expected default model llama3, got %s", step.Params["model"])
			}
		}
	}
	if !foundLLM {
		t.Error("expected ollama step in summarize workflow")
	}
}

func TestGenerateWorkflow_DeepSeekModel(t *testing.T) {
	wf, err := GenerateWorkflow("use deepseek to summarize")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundDeepSeek := false
	for _, step := range wf.Steps {
		if step.Node == "deepseek" {
			foundDeepSeek = true
			if step.Params["model"] != "deepseek-chat" {
				t.Errorf("expected deepseek-chat model, got %s", step.Params["model"])
			}
		}
	}
	if !foundDeepSeek {
		t.Error("expected deepseek step")
	}
}

func TestGenerateWorkflow_GLMModel(t *testing.T) {
	wf, err := GenerateWorkflow("使用智谱GLM总结")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundGLM := false
	for _, step := range wf.Steps {
		if step.Node == "glm" {
			foundGLM = true
		}
	}
	if !foundGLM {
		t.Error("expected glm step")
	}
}

func TestGenerateWorkflow_KimiModel(t *testing.T) {
	wf, err := GenerateWorkflow("use kimi to translate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundKimi := false
	for _, step := range wf.Steps {
		if step.Node == "kimi" {
			foundKimi = true
		}
	}
	if !foundKimi {
		t.Error("expected kimi step")
	}
}

func TestGenerateWorkflow_ExtractURL(t *testing.T) {
	wf, err := GenerateWorkflow("fetch https://example.com and summarize")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundFetch := false
	for _, step := range wf.Steps {
		if step.Node == "fetch_url" {
			foundFetch = true
			if step.Params["url"] != "https://example.com" {
				t.Errorf("expected https://example.com, got %s", step.Params["url"])
			}
		}
	}
	if !foundFetch {
		t.Error("expected fetch_url step")
	}
}

func TestGenerateWorkflow_ExtractDomain(t *testing.T) {
	wf, err := GenerateWorkflow("fetch example.com and summarize")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundFetch := false
	for _, step := range wf.Steps {
		if step.Node == "fetch_url" {
			foundFetch = true
			if step.Params["url"] != "https://example.com" {
				t.Errorf("expected https://example.com, got %s", step.Params["url"])
			}
		}
	}
	if !foundFetch {
		t.Error("expected fetch_url step")
	}
}

func TestGenerateWorkflow_ExtractFilePath(t *testing.T) {
	wf, err := GenerateWorkflow("summarize and save to output.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundWrite := false
	for _, step := range wf.Steps {
		if step.Node == "file_write" {
			foundWrite = true
			if step.Params["path"] != "output.txt" {
				t.Errorf("expected output.txt, got %s", step.Params["path"])
			}
		}
	}
	if !foundWrite {
		t.Error("expected file_write step")
	}
}

func TestGenerateWorkflow_SummarizeAction(t *testing.T) {
	wf, err := GenerateWorkflow("summarize this article")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundSummarize := false
	for _, step := range wf.Steps {
		if step.Node == "ollama" {
			foundSummarize = true
			if !strings.Contains(step.Params["system"], "summariz") {
				t.Errorf("expected summarize system prompt, got %s", step.Params["system"])
			}
		}
	}
	if !foundSummarize {
		t.Error("expected ollama summarize step")
	}
}

func TestGenerateWorkflow_TranslateAction(t *testing.T) {
	wf, err := GenerateWorkflow("translate this text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundTranslate := false
	for _, step := range wf.Steps {
		if step.Node == "ollama" {
			foundTranslate = true
			if !strings.Contains(step.Params["system"], "translator") {
				t.Errorf("expected translator system prompt, got %s", step.Params["system"])
			}
		}
	}
	if !foundTranslate {
		t.Error("expected ollama translate step")
	}
}

func TestGenerateWorkflow_ChineseSummarize(t *testing.T) {
	wf, err := GenerateWorkflow("总结这篇文章")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundSummarize := false
	for _, step := range wf.Steps {
		if step.Node == "ollama" {
			foundSummarize = true
		}
	}
	if !foundSummarize {
		t.Error("expected ollama step for 总结")
	}
}

func TestGenerateWorkflow_GitAction(t *testing.T) {
	wf, err := GenerateWorkflow("show git commits")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundGit := false
	for _, step := range wf.Steps {
		if step.Node == "execute" {
			if strings.Contains(step.Params["command"], "git") {
				foundGit = true
			}
		}
	}
	if !foundGit {
		t.Error("expected execute git step")
	}
}

func TestGenerateWorkflow_DefaultStep(t *testing.T) {
	wf, err := GenerateWorkflow("do something")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(wf.Steps) == 0 {
		t.Fatal("expected at least one default step")
	}

	if wf.Steps[0].Node != "combine" {
		t.Errorf("expected default combine step, got %s", wf.Steps[0].Node)
	}
}

func TestGenerateWorkflow_StepOrder(t *testing.T) {
	wf, err := GenerateWorkflow("fetch https://example.com and summarize and save to out.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(wf.Steps) < 3 {
		t.Fatalf("expected at least 3 steps, got %d", len(wf.Steps))
	}

	// fetch_url should be first
	if wf.Steps[0].Node != "fetch_url" {
		t.Errorf("expected fetch_url first, got %s", wf.Steps[0].Node)
	}

	// summarize should be before file_write
	if wf.Steps[len(wf.Steps)-1].Node != "file_write" {
		t.Error("expected file_write to be last")
	}
}

func TestGetSuggestedFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"fetch example.com", "example.com.yaml"},
		{"summarize article", "summarize_article.yaml"},
		{"translate text to english", "translate_text_english.yaml"},
		{"a", "workflow.yaml"},
	}

	for _, tt := range tests {
		result := GetSuggestedFilename(tt.input)
		if result != tt.expected {
			t.Errorf("GetSuggestedFilename(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestGenerateWorkflowName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"fetch example.com and summarize", "Example.com Summarize"},
		{"translate text", "Translate Text"},
		{"a", "Custom Workflow"},
	}

	for _, tt := range tests {
		result := generateWorkflowName(tt.input)
		if result != tt.expected {
			t.Errorf("generateWorkflowName(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestToYAML(t *testing.T) {
	wf := &Workflow{
		Name:        "Test Workflow",
		Description: "A test workflow",
		Steps: []WorkflowStep{
			{Node: "fetch_url", Params: map[string]string{"url": "https://example.com"}},
		},
	}

	yaml := wf.ToYAML()

	if !strings.Contains(yaml, "name: Test Workflow") {
		t.Error("YAML missing name")
	}
	if !strings.Contains(yaml, "description: A test workflow") {
		t.Error("YAML missing description")
	}
	if !strings.Contains(yaml, "node: fetch_url") {
		t.Error("YAML missing step")
	}
}

func TestValidateWorkflow(t *testing.T) {
	wf := &Workflow{
		Steps: []WorkflowStep{
			{Node: "fetch_url", Params: map[string]string{"url": "https://example.com"}},
		},
	}

	suggestions := ValidateWorkflow(wf)

	// Should suggest adding a name
	hasNameSuggestion := false
	for _, s := range suggestions {
		if strings.Contains(s, "name") {
			hasNameSuggestion = true
		}
	}
	if !hasNameSuggestion {
		t.Error("expected suggestion about workflow name")
	}

	// Should suggest adding file_write
	hasOutputSuggestion := false
	for _, s := range suggestions {
		if strings.Contains(s, "file_write") {
			hasOutputSuggestion = true
		}
	}
	if !hasOutputSuggestion {
		t.Error("expected suggestion about file_write")
	}
}

func TestValidateWorkflow_NoSteps(t *testing.T) {
	wf := &Workflow{}

	suggestions := ValidateWorkflow(wf)

	found := false
	for _, s := range suggestions {
		if strings.Contains(s, "no steps") {
			found = true
		}
	}
	if !found {
		t.Error("expected suggestion about missing steps")
	}
}

func TestGetWorkflowFilename(t *testing.T) {
	wf := &Workflow{Name: "My Test Workflow"}
	result := GetWorkflowFilename(wf)
	if result != "my_test_workflow.yaml" {
		t.Errorf("expected my_test_workflow.yaml, got %s", result)
	}
}

// ── More model tests ──

func TestGenerateWorkflow_MoreModels(t *testing.T) {
	tests := []struct {
		desc     string
		expected string
	}{
		{"use coze to summarize", "coze"},
		{"use minimax to summarize", "minimax"},
		{"use ima to summarize", "ima"},
		{"use xverse to summarize", "xverse"},
		{"use yi to summarize", "yi"},
		{"use baichuan to summarize", "baichuan"},
		{"use internlm to summarize", "internlm"},
		{"use mistral to summarize", "mistral"},
		{"use mimo to summarize", "mimo"},
		{"use qwen to summarize", "qwen"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			wf, err := GenerateWorkflow(tt.desc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			found := false
			for _, step := range wf.Steps {
				if step.Node == tt.expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %s step", tt.expected)
			}
		})
	}
}

// ── More action tests ──

func TestGenerateWorkflow_MoreActions(t *testing.T) {
	tests := []struct {
		desc         string
		expectedNode string
		checkKey     string
		checkValue   string
	}{
		{"explain this code", "ollama", "system", "expert educator"},
		{"rewrite text", "ollama", "system", "skilled writer"},
		{"write code", "ollama", "system", "senior software engineer"},
		{"send email", "ollama", "system", "professional writer"},
		{"generate report", "ollama", "system", "research analyst"},
		{"create doc", "ollama", "system", "technical writer"},
		{"write test", "ollama", "system", "QA engineer"},
		{"parse json", "json_parse", "", ""},
		{"show git log", "execute", "command", "git"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedNode, func(t *testing.T) {
			wf, err := GenerateWorkflow(tt.desc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			found := false
			for _, step := range wf.Steps {
				if step.Node == tt.expectedNode {
					if tt.checkKey != "" && !strings.Contains(step.Params[tt.checkKey], tt.checkValue) {
						t.Errorf("expected %s containing %q, got %s", tt.checkKey, tt.checkValue, step.Params[tt.checkKey])
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %s step", tt.expectedNode)
			}
		})
	}
}

// ── SaveWorkflow and CreateWorkflowFromDescription tests ──

func TestSaveWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	wf := &Workflow{
		Name: "Test",
		Steps: []WorkflowStep{
			{Node: "fetch_url", Params: map[string]string{"url": "https://example.com"}},
		},
	}

	err := SaveWorkflow(wf, "my_workflow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile("my_workflow.yaml")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !strings.Contains(string(content), "fetch_url") {
		t.Error("expected YAML to contain fetch_url")
	}
}

func TestCreateWorkflowFromDescription(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	path, err := CreateWorkflowFromDescription("summarize this article")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to exist at %s", path)
	}
}

// ── getSystemPrompt tests ──

func TestGetSystemPrompt_Zh(t *testing.T) {
	i18n.Init("zh")
	defer i18n.Init("en")

	prompt := getSystemPrompt("summarize")
	if !strings.Contains(prompt, "总结") {
		t.Errorf("expected Chinese prompt, got %s", prompt)
	}
}

func TestGetSystemPrompt_UnknownLang(t *testing.T) {
	i18n.Init("xx")
	defer i18n.Init("en")

	prompt := getSystemPrompt("summarize")
	if !strings.Contains(prompt, "summariz") {
		t.Errorf("expected English fallback, got %s", prompt)
	}
}

func TestGetSystemPrompt_UnknownAction(t *testing.T) {
	prompt := getSystemPrompt("nonexistent")
	if prompt != "" {
		t.Errorf("expected empty prompt, got %s", prompt)
	}
}

// ── Direct keyword helper tests ──

func TestContainsWord_Boundary(t *testing.T) {
	if containsWord("digital", "git") {
		t.Error("git should not match digital")
	}
	if !containsWord("git push", "git") {
		t.Error("git should match 'git push'")
	}
}

func TestContainsWord_Chinese(t *testing.T) {
	if !containsWord("请翻译这段文字", "翻译") {
		t.Error("翻译 should match Chinese text")
	}
}

func TestContainsLLMKeyword_UnknownProvider(t *testing.T) {
	if containsLLMKeyword("anything", "unknown") {
		t.Error("unknown provider should not match")
	}
}

func TestContainsActionKeyword_UnknownAction(t *testing.T) {
	if containsActionKeyword("anything", "unknown") {
		t.Error("unknown action should not match")
	}
}

// TestHasMeaningfulSteps verifies the signal used by the CLI (断点9) to decide
// whether keyword matching produced a real workflow or only the placeholder
// combine fallback.
func TestHasMeaningfulSteps(t *testing.T) {
	tests := []struct {
		name string
		wf   *Workflow
		want bool
	}{
		{
			name: "nil workflow",
			wf:   nil,
			want: false,
		},
		{
			name: "no steps",
			wf:   &Workflow{Steps: nil},
			want: false,
		},
		{
			name: "only placeholder combine",
			wf: &Workflow{Steps: []WorkflowStep{{
				Node:   "combine",
				Params: map[string]string{"format": "text"},
			}}},
			want: false,
		},
		{
			name: "single real fetch_url step",
			wf: &Workflow{Steps: []WorkflowStep{{
				Node:   "fetch_url",
				Params: map[string]string{"url": "https://example.com"},
			}}},
			want: true,
		},
		{
			name: "combine plus real step",
			wf: &Workflow{Steps: []WorkflowStep{
				{Node: "combine", Params: map[string]string{"format": "text"}},
				{Node: "file_write", Params: map[string]string{"path": "out.txt"}},
			}},
			want: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasMeaningfulSteps(tc.wf); got != tc.want {
				t.Errorf("HasMeaningfulSteps = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestGenerateWorkflow_UnmatchedDescriptionProducesPlaceholder verifies that a
// description matching no keyword yields a non-meaningful (placeholder) workflow,
// which is the precondition for the CLI's suggestion/LLM-fallback path (断点9).
func TestGenerateWorkflow_UnmatchedDescriptionProducesPlaceholder(t *testing.T) {
	// "安排明天的会议日程" contains none of the llm/action/domain/url/file
	// keywords, so the rule-based generator falls back to the placeholder
	// combine step.
	wf, err := GenerateWorkflow("安排明天的会议日程")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if HasMeaningfulSteps(wf) {
		t.Errorf("unmatched description should produce placeholder workflow, got steps: %+v", wf.Steps)
	}
}

// ── 遗留修复: price / condition / schedule keyword tests ──

func TestGenerateWorkflow_PriceKeyword(t *testing.T) {
	wf, err := GenerateWorkflow("检查 BTC 价格")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var foundHTTP, foundJSONParse bool
	for _, step := range wf.Steps {
		if step.Node == "http_request" {
			foundHTTP = true
			if !strings.Contains(step.Params["url"], "coingecko") {
				t.Errorf("expected coingecko url, got %s", step.Params["url"])
			}
		}
		if step.Node == "json_parse" {
			foundJSONParse = true
			if step.Params["path"] != "bitcoin.usd" {
				t.Errorf("expected path bitcoin.usd, got %s", step.Params["path"])
			}
		}
	}
	if !foundHTTP || !foundJSONParse {
		t.Errorf("expected http_request + json_parse steps, got: %+v", wf.Steps)
	}
	if !HasMeaningfulSteps(wf) {
		t.Error("price workflow should be meaningful")
	}
}

func TestGenerateWorkflow_PriceKeywordETH(t *testing.T) {
	wf, err := GenerateWorkflow("获取以太坊 ETH 价格")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, step := range wf.Steps {
		if step.Node == "json_parse" && step.Params["path"] != "ethereum.usd" {
			t.Errorf("expected path ethereum.usd for ETH, got %s", step.Params["path"])
		}
	}
}

func TestGenerateWorkflow_NotifyCNChannels(t *testing.T) {
	// CN group-bot channels are detected from Chinese (or English) channel
	// words in the description and carry a webhook URL var named after the
	// channel, so the generated step is runnable after --set <channel>_webhook_url.
	cases := []struct {
		desc        string
		wantChannel string
		wantURLVar  string
	}{
		{"每 10 分钟检查贵州茅台 600519 股价，超过 1400 发飞书通知", "feishu", "{{var.feishu_webhook_url}}"},
		{"监控 600519 股价，超过 1400 发钉钉通知", "dingtalk", "{{var.dingtalk_webhook_url}}"},
		{"监控 600519 股价，超过 1400 发企业微信通知", "wecom", "{{var.wecom_webhook_url}}"},
		{"监控 600519 股价，超过 1400 发微信通知", "wecom", "{{var.wecom_webhook_url}}"},
		{"notify me via feishu", "feishu", "{{var.feishu_webhook_url}}"},
		{"notify via dingtalk", "dingtalk", "{{var.dingtalk_webhook_url}}"},
		{"notify via wecom", "wecom", "{{var.wecom_webhook_url}}"},
		{"notify via slack", "slack", "{{var.slack_webhook_url}}"},
	}
	for _, tc := range cases {
		wf, err := GenerateWorkflow(tc.desc)
		if err != nil {
			t.Fatalf("GenerateWorkflow(%q): %v", tc.desc, err)
		}
		var notify *WorkflowStep
		for i := range wf.Steps {
			if wf.Steps[i].Node == "notify" {
				notify = &wf.Steps[i]
			}
			if wf.Steps[i].If != nil {
				for j := range wf.Steps[i].If.Then {
					if wf.Steps[i].If.Then[j].Node == "notify" {
						notify = &wf.Steps[i].If.Then[j]
					}
				}
			}
		}
		if notify == nil {
			t.Fatalf("GenerateWorkflow(%q): no notify step found", tc.desc)
		}
		if got := notify.Params["channel"]; got != tc.wantChannel {
			t.Errorf("GenerateWorkflow(%q): channel = %q, want %q", tc.desc, got, tc.wantChannel)
		}
		if got := notify.Params["url"]; got != tc.wantURLVar {
			t.Errorf("GenerateWorkflow(%q): url = %q, want %q", tc.desc, got, tc.wantURLVar)
		}
	}
}

func TestGenerateWorkflow_ConditionWrapsNotify(t *testing.T) {
	wf, err := GenerateWorkflow("超过 70000 发 telegram 通知")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expect an if-step whose condition is gt:70000 and whose then-branch
	// contains a notify step with channel=telegram.
	var ifStep *WorkflowStep
	for i := range wf.Steps {
		if wf.Steps[i].If != nil {
			ifStep = &wf.Steps[i]
			break
		}
	}
	if ifStep == nil {
		t.Fatalf("expected an if-step, got: %+v", wf.Steps)
	}
	if ifStep.If.Condition != "gt:70000" {
		t.Errorf("expected condition gt:70000, got %s", ifStep.If.Condition)
	}
	if len(ifStep.If.Then) != 1 || ifStep.If.Then[0].Node != "notify" {
		t.Errorf("expected then-branch with one notify step, got: %+v", ifStep.If.Then)
	}
	if ifStep.If.Then[0].Params["channel"] != "telegram" {
		t.Errorf("expected telegram channel, got %s", ifStep.If.Then[0].Params["channel"])
	}
	// The notify step should NOT also appear as a top-level step.
	for _, step := range wf.Steps {
		if step.Node == "notify" {
			t.Error("notify should be inside the if-branch, not a top-level step")
		}
	}
}

func TestGenerateWorkflow_ConditionBelow(t *testing.T) {
	wf, err := GenerateWorkflow("低于 50000 发通知")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := range wf.Steps {
		if wf.Steps[i].If != nil {
			if wf.Steps[i].If.Condition != "lt:50000" {
				t.Errorf("expected lt:50000, got %s", wf.Steps[i].If.Condition)
			}
			return
		}
	}
	t.Error("expected an if-step for 低于 50000")
}

func TestGenerateWorkflow_NotifyWithoutCondition(t *testing.T) {
	wf, err := GenerateWorkflow("发 telegram 通知")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Without a condition, notify should be a top-level step (not wrapped in if).
	for _, step := range wf.Steps {
		if step.Node == "notify" {
			return // found top-level notify
		}
	}
	t.Error("expected a top-level notify step when no condition is present")
}

func TestGenerateWorkflow_ScheduleEveryNMinutes(t *testing.T) {
	wf, err := GenerateWorkflow("每 10 分钟检查 BTC 价格")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Schedule == nil {
		t.Fatal("expected wf.Schedule to be set")
	}
	if wf.Schedule.Cron != "*/10 * * * *" {
		t.Errorf("expected cron */10 * * * *, got %s", wf.Schedule.Cron)
	}
	if !wf.Schedule.Enabled {
		t.Error("expected schedule enabled")
	}
}

func TestGenerateWorkflow_ScheduleEveryHour(t *testing.T) {
	wf, err := GenerateWorkflow("每 2 小时运行")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Schedule == nil || wf.Schedule.Cron != "0 */2 * * *" {
		t.Errorf("expected cron 0 */2 * * *, got: %+v", wf.Schedule)
	}
}

func TestGenerateWorkflow_FullAShareExample(t *testing.T) {
	// The A-share counterpart of the BTC example:
	// "每 10 分钟检查贵州茅台 600519 股价，超过 1400 发 Telegram 通知"
	wf, err := GenerateWorkflow("每 10 分钟检查贵州茅台 600519 股价，超过 1400 发 Telegram 通知")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !HasMeaningfulSteps(wf) {
		t.Fatal("expected a meaningful workflow for the A-share example")
	}
	// 1. schedule set
	if wf.Schedule == nil || wf.Schedule.Cron != "*/10 * * * *" {
		t.Errorf("expected schedule cron */10 * * * *, got: %+v", wf.Schedule)
	}
	// 2. http_request hits the Tencent quote API, json_parse extracts the qt live price
	var foundHTTP, foundJSON, foundIf bool
	for i := range wf.Steps {
		s := &wf.Steps[i]
		if s.Node == "http_request" {
			foundHTTP = true
			if !strings.Contains(s.Params["url"], "web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=sh600519,") {
				t.Errorf("expected Tencent quote API url with sh600519, got %s", s.Params["url"])
			}
		}
		if s.Node == "json_parse" {
			foundJSON = true
			if s.Params["path"] != "data.sh600519.qt.sh600519.[3]" {
				t.Errorf("expected qt live-price path, got %s", s.Params["path"])
			}
		}
		if s.If != nil {
			foundIf = true
			if s.If.Condition != "gt:1400" {
				t.Errorf("expected if condition gt:1400, got %s", s.If.Condition)
			}
			if len(s.If.Then) != 1 || s.If.Then[0].Node != "notify" || s.If.Then[0].Params["channel"] != "telegram" {
				t.Errorf("expected telegram notify in then-branch, got: %+v", s.If.Then)
			}
		}
	}
	if !foundHTTP {
		t.Error("expected http_request step for A-share price fetch")
	}
	if !foundJSON {
		t.Error("expected json_parse step for A-share price")
	}
	if !foundIf {
		t.Error("expected if-step for threshold condition")
	}
}

func TestGenerateWorkflow_FullHKStockExample(t *testing.T) {
	// HK counterpart of the A-share example:
	// "每 10 分钟检查港股腾讯 hk00700 股价，低于 440 发 Telegram 通知"
	wf, err := GenerateWorkflow("每 10 分钟检查港股腾讯 hk00700 股价，低于 440 发 Telegram 通知")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !HasMeaningfulSteps(wf) {
		t.Fatal("expected a meaningful workflow for the HK stock example")
	}
	if wf.Schedule == nil || wf.Schedule.Cron != "*/10 * * * *" {
		t.Errorf("expected schedule cron */10 * * * *, got: %+v", wf.Schedule)
	}
	var foundHTTP, foundJSON bool
	for i := range wf.Steps {
		s := &wf.Steps[i]
		if s.Node == "http_request" {
			foundHTTP = true
			if !strings.Contains(s.Params["url"], "get?param=hk00700,") {
				t.Errorf("expected Tencent quote API url with hk00700, got %s", s.Params["url"])
			}
		}
		if s.Node == "json_parse" {
			foundJSON = true
			if s.Params["path"] != "data.hk00700.qt.hk00700.[3]" {
				t.Errorf("expected qt live-price path, got %s", s.Params["path"])
			}
		}
	}
	if !foundHTTP || !foundJSON {
		t.Errorf("expected http_request+json_parse for HK stock, got http=%v json=%v", foundHTTP, foundJSON)
	}
}

func TestGenerateWorkflow_FullUSStockExample(t *testing.T) {
	// US counterpart of the BTC example:
	// "check usAAPL stock price every 10 minutes, alert via Telegram when > 320"
	wf, err := GenerateWorkflow("check usAAPL stock price every 10 minutes, alert via Telegram when > 320")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !HasMeaningfulSteps(wf) {
		t.Fatal("expected a meaningful workflow for the US stock example")
	}
	if wf.Schedule == nil || wf.Schedule.Cron != "*/10 * * * *" {
		t.Errorf("expected schedule cron */10 * * * *, got: %+v", wf.Schedule)
	}
	var foundHTTP, foundJSON bool
	for i := range wf.Steps {
		s := &wf.Steps[i]
		if s.Node == "http_request" {
			foundHTTP = true
			if !strings.Contains(s.Params["url"], "get?param=usAAPL,") {
				t.Errorf("expected Tencent quote API url with usAAPL, got %s", s.Params["url"])
			}
		}
		if s.Node == "json_parse" {
			foundJSON = true
			if s.Params["path"] != "data.usAAPL.qt.usAAPL.[3]" {
				t.Errorf("expected qt live-price path, got %s", s.Params["path"])
			}
		}
	}
	if !foundHTTP || !foundJSON {
		t.Errorf("expected http_request+json_parse for US stock, got http=%v json=%v", foundHTTP, foundJSON)
	}
}

func TestExtractStockSymbol(t *testing.T) {
	cases := []struct {
		desc string
		want string
		ok   bool
	}{
		// A-share
		{"贵州茅台 600519 股价", "sh600519", true}, // 6xx → Shanghai
		{"平安银行 000001 行情", "sz000001", true}, // 0xx → Shenzhen
		{"宁德时代 300750 股票", "sz300750", true}, // 3xx ChiNext → Shenzhen
		{"检查 sh600519 股价", "sh600519", true}, // explicit prefix
		{"检查 SZ000001 股价", "sz000001", true}, // explicit prefix, case-insensitive
		// HK stock
		{"检查港股 hk00700 股价", "hk00700", true}, // explicit 5-digit prefix
		{"检查港股 hk700 股价", "hk00700", true},   // 4-digit prefix → zero-padded
		{"检查 HK:0700 股价", "hk00700", true},   // colon form
		{"港股 00700 股价", "hk00700", true},     // bare 5-digit leading-zero code
		{"监控 00700 价格", "hk00700", true},     // bare HK code in price context
		// US stock
		{"check usAAPL price", "usAAPL", true},         // adjacent form
		{"check US:AAPL price", "usAAPL", true},        // colon form
		{"check US AAPL price", "usAAPL", true},        // space form
		{"monitor usTSLA stock price", "usTSLA", true}, // another ticker
		// rejected / not stocks
		{"check US MARKET price", "", false},         // blacklisted non-ticker word
		{"monitor USDT price", "", false},            // stablecoin, not a US stock
		{"useful tool for price checks", "", false},  // "useful" is not us+UPPERCASE
		{"每 10 分钟检查 BTC 价格，超过 100000 通知", "", false}, // crypto hint wins over 6-digit number
		{"比特币价格超过 600000", "", false},                // crypto hint in Chinese
		{"总结这篇文章", "", false},                        // no code at all
	}
	for _, c := range cases {
		got, ok := extractStockSymbol(c.desc)
		if ok != c.ok || got != c.want {
			t.Errorf("extractStockSymbol(%q) = (%q,%v), want (%q,%v)", c.desc, got, ok, c.want, c.ok)
		}
	}
}

func TestGenerateWorkflow_PriceKeywordNoCryptoNoStock(t *testing.T) {
	// 安全自检回归: a price query that names neither a stock symbol nor a
	// crypto asset ("check gold price" / "监控油价") used to fall through to
	// the CoinGecko branch and silently generate a BITCOIN monitor. The
	// CoinGecko route must only fire for descriptions that actually name a
	// crypto asset; other symbol-less price queries generate no price steps.
	for _, desc := range []string{
		"check gold price",
		"监控黄金价格",
		"check the price and notify me",
	} {
		wf, err := GenerateWorkflow(desc)
		if err != nil {
			t.Fatalf("GenerateWorkflow(%q): %v", desc, err)
		}
		for i := range wf.Steps {
			s := &wf.Steps[i]
			if s.Node == "http_request" {
				t.Errorf("GenerateWorkflow(%q): unexpected http_request step %v — symbol-less non-crypto price queries must not fetch any price feed", desc, s.Params["url"])
			}
			if s.Node == "json_parse" {
				t.Errorf("GenerateWorkflow(%q): unexpected json_parse step — symbol-less non-crypto price queries must not fetch any price feed", desc)
			}
		}
	}
}

func TestGenerateWorkflow_FullBTCExample(t *testing.T) {
	// The exact example from the user's original complaint:
	// "每 10 分钟检查 BTC 价格，超过 70000 发 Telegram 通知"
	wf, err := GenerateWorkflow("每 10 分钟检查 BTC 价格，超过 70000 发 Telegram 通知")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !HasMeaningfulSteps(wf) {
		t.Fatal("expected a meaningful workflow for the BTC example")
	}
	// 1. schedule set
	if wf.Schedule == nil || wf.Schedule.Cron != "*/10 * * * *" {
		t.Errorf("expected schedule cron */10 * * * *, got: %+v", wf.Schedule)
	}
	// 2. http_request + json_parse present
	var foundHTTP, foundJSON, foundIf bool
	for i := range wf.Steps {
		s := &wf.Steps[i]
		if s.Node == "http_request" {
			foundHTTP = true
		}
		if s.Node == "json_parse" {
			foundJSON = true
		}
		if s.If != nil {
			foundIf = true
			if s.If.Condition != "gt:70000" {
				t.Errorf("expected if condition gt:70000, got %s", s.If.Condition)
			}
			if len(s.If.Then) != 1 || s.If.Then[0].Node != "notify" {
				t.Errorf("expected notify in then-branch, got: %+v", s.If.Then)
			}
			if s.If.Then[0].Params["channel"] != "telegram" {
				t.Errorf("expected telegram channel, got %s", s.If.Then[0].Params["channel"])
			}
		}
	}
	if !foundHTTP {
		t.Error("expected http_request step for BTC price fetch")
	}
	if !foundJSON {
		t.Error("expected json_parse step for BTC price")
	}
	if !foundIf {
		t.Error("expected if-step for threshold condition")
	}
}

func TestExtractCondition(t *testing.T) {
	cases := []struct {
		desc   string
		want   string
		wantOk bool
	}{
		{"超过 70000", "gt:70000", true},
		{"价格大于 50000", "gt:50000", true},
		{"above 100", "gt:100", true},
		{"over 99.5", "gt:99.5", true},
		{"低于 30000", "lt:30000", true},
		{"below 50", "lt:50", true},
		{"检查价格", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := extractCondition(c.desc)
		if got != c.want || ok != c.wantOk {
			t.Errorf("extractCondition(%q) = (%q, %v), want (%q, %v)", c.desc, got, ok, c.want, c.wantOk)
		}
	}
}

func TestParseScheduleCron(t *testing.T) {
	cases := []struct {
		desc string
		want string
	}{
		{"每 10 分钟", "*/10 * * * *"},
		{"每隔 5 分钟", "*/5 * * * *"},
		{"每 2 小时", "0 */2 * * *"},
		{"每小时", "0 * * * *"},
		{"每分钟", "* * * * *"},
		{"每天", "0 9 * * *"},
		{"检查价格", ""},
		// English forms
		{"every 10 minutes", "*/10 * * * *"},
		{"every 15 min", "*/15 * * * *"},
		{"every 2 hours", "0 */2 * * *"},
		{"every minute", "* * * * *"},
		// degenerate intervals are rejected instead of emitting a broken cron
		{"每 0 分钟", ""},
		{"every 0 minutes", ""},
		{"每 99 分钟", ""},
		{"every 99 minutes", ""},
		{"每 0 小时", ""},
		{"every 100 hours", ""},
	}
	for _, c := range cases {
		got := parseScheduleCron(c.desc)
		if got != c.want {
			t.Errorf("parseScheduleCron(%q) = %q, want %q", c.desc, got, c.want)
		}
	}
}
