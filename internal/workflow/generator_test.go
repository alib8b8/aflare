package workflow

import (
	"strings"
	"testing"
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

	if wf.Steps[0].Node != "execute" {
		t.Errorf("expected default execute step, got %s", wf.Steps[0].Node)
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

	if !strings.Contains(yaml, "name: \"Test Workflow\"") {
		t.Error("YAML missing name")
	}
	if !strings.Contains(yaml, "description: \"A test workflow\"") {
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
