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
