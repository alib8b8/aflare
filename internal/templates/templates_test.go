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

package templates

import (
	"strings"
	"testing"
)

func TestNewTemplateManager(t *testing.T) {
	tm := NewTemplateManager()
	if tm == nil {
		t.Fatal("NewTemplateManager returned nil")
	}
	templates := tm.List()
	if len(templates) < 6 {
		t.Errorf("expected at least 6 built-in templates, got %d", len(templates))
	}
}

func TestTemplateList(t *testing.T) {
	tm := NewTemplateManager()
	templates := tm.List()
	if len(templates) == 0 {
		t.Fatal("no templates found")
	}
	names := make(map[string]bool)
	for _, tpl := range templates {
		if tpl.Name == "" {
			t.Error("template with empty name found")
		}
		if names[tpl.Name] {
			t.Errorf("duplicate template name: %s", tpl.Name)
		}
		names[tpl.Name] = true
	}
	expectedNames := []string{
		"simple-llm",
		"code-review",
		"data-processing",
		"web-scraper",
		"translation",
		"batch-processor",
	}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("expected template not found: %s", name)
		}
	}
}

func TestTemplateGet(t *testing.T) {
	tm := NewTemplateManager()

	tpl, err := tm.Get("simple-llm")
	if err != nil {
		t.Fatalf("failed to get simple-llm template: %v", err)
	}
	if tpl.Name != "simple-llm" {
		t.Errorf("expected name 'simple-llm', got '%s'", tpl.Name)
	}
	if tpl.Category == "" {
		t.Error("template category is empty")
	}
	if tpl.Version == "" {
		t.Error("template version is empty")
	}
	if len(tpl.Variables) == 0 {
		t.Error("template has no variables")
	}

	_, err = tm.Get("nonexistent-template")
	if err == nil {
		t.Error("expected error for nonexistent template, got nil")
	}
}

func TestTemplateSearch(t *testing.T) {
	tm := NewTemplateManager()

	results := tm.Search("llm")
	if len(results) == 0 {
		t.Error("expected results for 'llm' search")
	}
	found := false
	for _, r := range results {
		if r.Name == "simple-llm" {
			found = true
			break
		}
	}
	if !found {
		t.Error("simple-llm not found in 'llm' search results")
	}

	results = tm.Search("data")
	if len(results) == 0 {
		t.Error("expected results for 'data' search")
	}

	results = tm.Search("nonexistentkeyword12345")
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent keyword, got %d", len(results))
	}
}

func TestTemplateCategories(t *testing.T) {
	tm := NewTemplateManager()
	categories := tm.Categories()
	if len(categories) == 0 {
		t.Fatal("no categories found")
	}
	catSet := make(map[string]bool)
	for _, cat := range categories {
		catSet[cat] = true
	}
	expectedCats := []string{"llm", "development", "data", "web"}
	for _, cat := range expectedCats {
		if !catSet[cat] {
			t.Errorf("expected category not found: %s", cat)
		}
	}
}

func TestTemplateListByCategory(t *testing.T) {
	tm := NewTemplateManager()

	llmTemplates := tm.ListByCategory("llm")
	if len(llmTemplates) < 2 {
		t.Errorf("expected at least 2 llm templates, got %d", len(llmTemplates))
	}
	for _, tpl := range llmTemplates {
		if tpl.Category != "llm" {
			t.Errorf("template %s has category %s, expected llm", tpl.Name, tpl.Category)
		}
	}

	emptyTemplates := tm.ListByCategory("nonexistent-category")
	if len(emptyTemplates) != 0 {
		t.Errorf("expected 0 templates for nonexistent category, got %d", len(emptyTemplates))
	}
}

func TestTemplateRenderSimpleLLM(t *testing.T) {
	tm := NewTemplateManager()

	vars := map[string]string{
		"prompt": "What is Go?",
		"model":  "gpt-4",
	}
	output, err := tm.Render("simple-llm", vars)
	if err != nil {
		t.Fatalf("failed to render simple-llm: %v", err)
	}
	if !strings.Contains(output, "name:") {
		t.Error("rendered output missing 'name:' field")
	}
	if !strings.Contains(output, "What is Go?") {
		t.Error("rendered output missing prompt value")
	}
	if !strings.Contains(output, "gpt-4") {
		t.Error("rendered output missing model value")
	}
	if !strings.Contains(output, "node: ollama") {
		t.Error("rendered output missing ollama node")
	}
}

func TestTemplateRenderWithOutputFile(t *testing.T) {
	tm := NewTemplateManager()

	vars := map[string]string{
		"prompt":      "Test prompt",
		"output_file": "result.txt",
	}
	output, err := tm.Render("simple-llm", vars)
	if err != nil {
		t.Fatalf("failed to render simple-llm with output_file: %v", err)
	}
	if !strings.Contains(output, "file_write") {
		t.Error("rendered output missing file_write node when output_file is set")
	}
	if !strings.Contains(output, "result.txt") {
		t.Error("rendered output missing output_file path")
	}
}

func TestTemplateRenderWithoutOutputFile(t *testing.T) {
	tm := NewTemplateManager()

	vars := map[string]string{
		"prompt": "Test prompt",
	}
	output, err := tm.Render("simple-llm", vars)
	if err != nil {
		t.Fatalf("failed to render simple-llm without output_file: %v", err)
	}
	if strings.Contains(output, "file_write") {
		t.Error("rendered output should not contain file_write when output_file is empty")
	}
}

func TestTemplateRenderRequiredVariableMissing(t *testing.T) {
	tm := NewTemplateManager()

	vars := map[string]string{
		"target_language": "",
	}
	_, err := tm.Render("translation", vars)
	if err == nil {
		t.Error("expected error for missing required variable 'target_language', got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "target_language") {
		t.Errorf("error should mention 'target_language', got: %v", err)
	}
}

func TestTemplateRenderNonexistentTemplate(t *testing.T) {
	tm := NewTemplateManager()

	_, err := tm.Render("nonexistent", map[string]string{})
	if err == nil {
		t.Error("expected error for nonexistent template, got nil")
	}
}

func TestTemplateRenderCodeReview(t *testing.T) {
	tm := NewTemplateManager()

	vars := map[string]string{
		"file_path":       "main.go",
		"review_language": "English",
		"output_file":     "review.md",
	}
	output, err := tm.Render("code-review", vars)
	if err != nil {
		t.Fatalf("failed to render code-review: %v", err)
	}
	if !strings.Contains(output, "file_read") {
		t.Error("rendered output missing file_read node")
	}
	if !strings.Contains(output, "ollama") {
		t.Error("rendered output missing ollama node")
	}
	if !strings.Contains(output, "file_write") {
		t.Error("rendered output missing file_write node")
	}
	if !strings.Contains(output, "review.md") {
		t.Error("rendered output missing output_file")
	}
}

func TestTemplateRenderDataProcessing(t *testing.T) {
	tm := NewTemplateManager()

	vars := map[string]string{
		"input_file":  "in.txt",
		"output_file": "out.txt",
		"operation":   "uppercase",
	}
	output, err := tm.Render("data-processing", vars)
	if err != nil {
		t.Fatalf("failed to render data-processing: %v", err)
	}
	if !strings.Contains(output, "uppercase") {
		t.Error("rendered output missing operation value")
	}
	if !strings.Contains(output, "transform") {
		t.Error("rendered output missing transform node")
	}
}

func TestTemplateRenderDataProcessingReplace(t *testing.T) {
	tm := NewTemplateManager()

	vars := map[string]string{
		"input_file":  "in.txt",
		"output_file": "out.txt",
		"operation":   "replace",
		"find":        "foo",
		"replace":     "bar",
	}
	output, err := tm.Render("data-processing", vars)
	if err != nil {
		t.Fatalf("failed to render data-processing replace: %v", err)
	}
	if !strings.Contains(output, "find:") {
		t.Error("rendered output missing find param")
	}
	if !strings.Contains(output, "foo") {
		t.Error("rendered output missing find value")
	}
	if !strings.Contains(output, "bar") {
		t.Error("rendered output missing replace value")
	}
}

func TestTemplateRenderWebScraper(t *testing.T) {
	tm := NewTemplateManager()

	vars := map[string]string{
		"url": "https://example.com",
	}
	output, err := tm.Render("web-scraper", vars)
	if err != nil {
		t.Fatalf("failed to render web-scraper: %v", err)
	}
	if !strings.Contains(output, "fetch_url") {
		t.Error("rendered output missing fetch_url node")
	}
	if !strings.Contains(output, "https://example.com") {
		t.Error("rendered output missing url value")
	}
}

func TestTemplateRenderWebScraperWithExtract(t *testing.T) {
	tm := NewTemplateManager()

	vars := map[string]string{
		"url":             "https://example.com",
		"extract_pattern": "\\d+",
	}
	output, err := tm.Render("web-scraper", vars)
	if err != nil {
		t.Fatalf("failed to render web-scraper with extract: %v", err)
	}
	if !strings.Contains(output, "regex") {
		t.Error("rendered output missing regex operation when extract_pattern is set")
	}
}

func TestTemplateRenderTranslation(t *testing.T) {
	tm := NewTemplateManager()

	vars := map[string]string{
		"input_file":      "source.txt",
		"output_file":     "target.txt",
		"target_language": "French",
	}
	output, err := tm.Render("translation", vars)
	if err != nil {
		t.Fatalf("failed to render translation: %v", err)
	}
	if !strings.Contains(output, "Translate") {
		t.Error("rendered output missing translation prompt")
	}
	if !strings.Contains(output, "French") {
		t.Error("rendered output missing target language")
	}
}

func TestTemplateRenderBatchProcessor(t *testing.T) {
	tm := NewTemplateManager()

	vars := map[string]string{
		"items":           "a\nb\nc",
		"processing_type": "summarize",
	}
	output, err := tm.Render("batch-processor", vars)
	if err != nil {
		t.Fatalf("failed to render batch-processor: %v", err)
	}
	if !strings.Contains(output, "loop:") {
		t.Error("rendered output missing loop config")
	}
	if !strings.Contains(output, "Summarize") {
		t.Error("rendered output missing summarize prompt")
	}
}

func TestTemplateRenderBatchProcessorClassify(t *testing.T) {
	tm := NewTemplateManager()

	vars := map[string]string{
		"items":           "item1",
		"processing_type": "classify",
	}
	output, err := tm.Render("batch-processor", vars)
	if err != nil {
		t.Fatalf("failed to render batch-processor classify: %v", err)
	}
	if !strings.Contains(output, "Classify") {
		t.Error("rendered output missing classify prompt")
	}
}

func TestTemplateVarsDefaultValues(t *testing.T) {
	tm := NewTemplateManager()

	tpl, err := tm.Get("simple-llm")
	if err != nil {
		t.Fatalf("failed to get template: %v", err)
	}

	hasDefault := false
	for _, v := range tpl.Variables {
		if v.Name == "model" && v.Default != "" {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		t.Error("expected 'model' variable to have a default value")
	}
}

func TestTemplateStructFields(t *testing.T) {
	tm := NewTemplateManager()
	templates := tm.List()

	for _, tpl := range templates {
		if tpl.Name == "" {
			t.Error("template has empty Name")
		}
		if tpl.Description == "" {
			t.Errorf("template %s has empty Description", tpl.Name)
		}
		if tpl.Category == "" {
			t.Errorf("template %s has empty Category", tpl.Name)
		}
		if tpl.Version == "" {
			t.Errorf("template %s has empty Version", tpl.Name)
		}
		if tpl.Content == "" {
			t.Errorf("template %s has empty Content", tpl.Name)
		}
		if len(tpl.Tags) == 0 {
			t.Errorf("template %s has no Tags", tpl.Name)
		}
	}
}

func TestTemplateVarStruct(t *testing.T) {
	tv := TemplateVar{
		Name:        "test_var",
		Description: "A test variable",
		Default:     "default_value",
		Required:    true,
	}
	if tv.Name != "test_var" {
		t.Error("TemplateVar Name field mismatch")
	}
	if tv.Description != "A test variable" {
		t.Error("TemplateVar Description field mismatch")
	}
	if tv.Default != "default_value" {
		t.Error("TemplateVar Default field mismatch")
	}
	if !tv.Required {
		t.Error("TemplateVar Required field mismatch")
	}
}
