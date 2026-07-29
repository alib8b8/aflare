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

package workflow

import (
	"os"
	"strings"
	"testing"
)

func TestExpressionEngine_StepRef(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch_url", "https://example.com content")

	result, err := engine.Evaluate("{{step.0}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "https://example.com content" {
		t.Errorf("expected step output, got %q", result)
	}
}

func TestExpressionEngine_StepName(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch_url", "hello world")

	result, err := engine.Evaluate("{{step.fetch_url}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Errorf("expected step output by name, got %q", result)
	}
}

func TestExpressionEngine_Variable(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetVariable("api_url", "https://api.example.com")

	result, err := engine.Evaluate("POST to {{var.api_url}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "POST to https://api.example.com" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExpressionEngine_Input(t *testing.T) {
	engine := NewExpressionEngine()

	result, err := engine.Evaluate("Processing: {{input}}", "raw text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Processing: raw text" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExpressionEngine_MultipleExpressions(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch", "data1")
	engine.SetStepOutput(1, "process", "data2")

	result, err := engine.Evaluate("First: {{step.0}}, Second: {{step.1}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "First: data1, Second: data2"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestExpressionEngine_NoExpressions(t *testing.T) {
	engine := NewExpressionEngine()

	result, err := engine.Evaluate("plain text without expressions", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "plain text without expressions" {
		t.Errorf("expected unchanged text, got %q", result)
	}
}

func TestExpressionEngine_EmptyExpr(t *testing.T) {
	engine := NewExpressionEngine()

	result, err := engine.Evaluate("", "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestExpressionEngine_VariableNotFound(t *testing.T) {
	engine := NewExpressionEngine()

	result, err := engine.Evaluate("{{var.missing}}", "")
	if err == nil {
		t.Fatalf("expected error for missing variable, got nil")
	}
	if result != "{{var.missing}}" {
		t.Errorf("expected unchanged expression for missing variable, got %q", result)
	}
}

func TestExpressionEngine_StepNotFound(t *testing.T) {
	engine := NewExpressionEngine()

	result, err := engine.Evaluate("{{step.99}}", "")
	if err == nil {
		t.Fatalf("expected error for missing step, got nil")
	}
	if result != "{{step.99}}" {
		t.Errorf("expected unchanged expression for missing step, got %q", result)
	}
}

func TestExpressionEngine_UnknownPrefix(t *testing.T) {
	engine := NewExpressionEngine()

	result, err := engine.Evaluate("{{unknown.value}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "{{unknown.value}}" {
		t.Errorf("expected unchanged expression for unknown prefix, got %q", result)
	}
}

func TestExpressionEngine_GoTemplateSyntax(t *testing.T) {
	engine := NewExpressionEngine()

	result, err := engine.Evaluate("Hello {{.input}} - {{.Name}}", "world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "Hello {{.input}} - {{.Name}}"
	if result != expected {
		t.Errorf("expected Go template syntax preserved, got %q", result)
	}
}

func TestEvaluateParams(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch", "fetched data")
	engine.SetVariable("api_key", "secret123")

	params := map[string]string{
		"url":     "{{step.0}}",
		"headers": "Authorization: Bearer {{var.api_key}}",
		"plain":   "static value",
	}

	evaluated, err := engine.EvaluateParams(params, "input data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if evaluated["url"] != "fetched data" {
		t.Errorf("expected url='fetched data', got %q", evaluated["url"])
	}
	if evaluated["headers"] != "Authorization: Bearer secret123" {
		t.Errorf("expected headers with API key, got %q", evaluated["headers"])
	}
	if evaluated["plain"] != "static value" {
		t.Errorf("expected plain unchanged, got %q", evaluated["plain"])
	}
}

func TestEvaluateParams_Nil(t *testing.T) {
	engine := NewExpressionEngine()
	evaluated, err := engine.EvaluateParams(nil, "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != nil {
		t.Errorf("expected nil, got %v", evaluated)
	}
}

func TestContainsExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"hello {{name}}", true},
		{"{{step.0}}", true},
		{"plain text", false},
		{"{{ }}", true},
		{"", false},
	}

	for _, tt := range tests {
		result := ContainsExpression(tt.input)
		if result != tt.expected {
			t.Errorf("ContainsExpression(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

// ── New tests for uncovered expression functionality ──

func TestExpressionEngine_GetVariable(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetVariable("key", "val")
	v, ok := engine.GetVariable("key")
	if !ok || v != "val" {
		t.Error("expected to get variable")
	}
	_, ok = engine.GetVariable("missing")
	if ok {
		t.Error("expected missing variable to not be found")
	}
}

func TestExpressionEngine_EnvVar(t *testing.T) {
	os.Setenv("LLM_BOX_TEST_VAR", "hello")
	defer os.Unsetenv("LLM_BOX_TEST_VAR")

	engine := NewExpressionEngine()
	result, err := engine.Evaluate("{{env.LLM_BOX_TEST_VAR}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got %s", result)
	}
}

func TestExpressionEngine_EnvVar_Disallowed(t *testing.T) {
	engine := NewExpressionEngine()
	_, err := engine.Evaluate("{{env.SECRET}}", "")
	if err == nil {
		t.Error("expected error for disallowed env var")
	}
}

func TestExpressionEngine_EnvVar_NotFound(t *testing.T) {
	engine := NewExpressionEngine()
	_, err := engine.Evaluate("{{env.LLM_BOX_NONEXISTENT_VAR_XYZ}}", "")
	if err == nil {
		t.Error("expected error for missing env var")
	}
}

func TestExpressionEngine_FileExpr(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	if err := os.WriteFile("test.txt", []byte("file content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	engine := NewExpressionEngine()
	result, err := engine.Evaluate("{{file.test.txt}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "file content" {
		t.Errorf("expected 'file content', got %s", result)
	}
}

func TestExpressionEngine_FileExpr_TooLarge(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	huge := make([]byte, maxExprFileSize+1)
	os.WriteFile("huge.txt", huge, 0644)

	engine := NewExpressionEngine()
	_, err := engine.Evaluate("{{file.huge.txt}}", "")
	if err == nil {
		t.Error("expected error for too large file")
	}
}

func TestExpressionEngine_LoopVars(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetLoopVars("apple", 2, 5)

	result, err := engine.Evaluate("item={{loop.item}},index={{loop.index}},count={{loop.count}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "item=apple,index=2,count=5"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}

	engine.ClearLoopVars()
	_, err = engine.Evaluate("{{loop.item}}", "")
	if err == nil {
		t.Error("expected error after clearing loop vars")
	}
}

func TestExpressionEngine_JSONPath(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "data", `{"users":[{"name":"Alice"},{"name":"Bob"}]}`)

	result, err := engine.Evaluate("{{step.0.jsonpath:$.users[0].name}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Alice" {
		t.Errorf("expected 'Alice', got %s", result)
	}
}

func TestExpressionEngine_JSONPath_Wildcard(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "data", `{"items":[{"name":"a"},{"name":"b"}]}`)

	result, err := engine.Evaluate("{{step.0.jsonpath:$.items[*]}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "name") || !strings.Contains(result, "a") || !strings.Contains(result, "b") {
		t.Errorf("expected JSON array items, got %s", result)
	}
}

func TestValidateExprFilePath_Absolute(t *testing.T) {
	_, err := validateExprFilePath("/etc/passwd")
	if err == nil {
		t.Error("expected error for absolute path")
	}
}

func TestValidateExprFilePath_Traversal(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	_, err := validateExprFilePath("../escape")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestValidateExprFilePath_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	// validateExprFilePath requires the file to exist
	os.WriteFile("file.txt", []byte{}, 0644)

	path, err := validateExprFilePath("file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(path, "file.txt") {
		t.Errorf("expected path to contain file.txt, got %s", path)
	}
}

func TestExtractJSONPath_TooLarge(t *testing.T) {
	jsonStr := strings.Repeat(" ", maxExprFileSize+1)
	_, err := extractJSONPath(jsonStr, "$.a")
	if err == nil {
		t.Error("expected error for too large JSON")
	}
}

func TestExtractJSONPath_PathTooLong(t *testing.T) {
	jsonStr := `{"a":1}`
	path := strings.Repeat("a", 1025)
	_, err := extractJSONPath(jsonStr, path)
	if err == nil {
		t.Error("expected error for too long path")
	}
}

func TestExtractJSONPath_Recursive(t *testing.T) {
	jsonStr := `{"a": {"name": "x"}, "b": {"name": "y"}}`
	result, err := extractJSONPath(jsonStr, "$..name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "x") || !strings.Contains(result, "y") {
		t.Errorf("expected x and y, got %s", result)
	}
}

func TestExtractJSONPath_InvalidJSON(t *testing.T) {
	_, err := extractJSONPath("not json", "$.a")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestEvalJSONPathDepth_ExceedsMax(t *testing.T) {
	data := map[string]interface{}{"a": map[string]interface{}{}}
	_, err := evalJSONPathDepth(data, "$.a", maxJSONPathDepth+1)
	if err == nil {
		t.Error("expected error for exceeding max depth")
	}
}

func TestEvalJSONPathDepth_MultipleRecursive(t *testing.T) {
	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": "x",
			},
		},
	}
	_, err := evalJSONPath(data, "$..a..b..c")
	if err == nil {
		t.Error("expected error for multiple recursive descent segments")
	}
}

func TestEvalJSONPathDepth_Root(t *testing.T) {
	data := map[string]interface{}{"a": 1}
	result, err := evalJSONPathDepth(data, "$", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestParsePathSegments_Malformed(t *testing.T) {
	_, err := parsePathSegments("foo[]")
	if err == nil {
		t.Error("expected error for empty array index")
	}

	_, err = parsePathSegments("foo[abc]")
	if err == nil {
		t.Error("expected error for invalid array index")
	}
}

func TestRecursiveFind_MaxDepth(t *testing.T) {
	data := map[string]interface{}{"a": map[string]interface{}{}}
	results := recursiveFind(data, "x", maxRecursiveDepth+1)
	if results != nil {
		t.Error("expected nil for exceeding max depth")
	}
}

func TestJSONPathResultToString(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{"str", "str"},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
		{true, "true"},
		{false, "false"},
		{nil, ""},
		{[]interface{}{"a", "b"}, "a\nb"},
	}
	for _, tt := range tests {
		result, err := jsonPathResultToString(tt.input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, result)
		}
	}
}

func TestJSONPathResultToString_Object(t *testing.T) {
	result, err := jsonPathResultToString(map[string]interface{}{"a": 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "a") {
		t.Errorf("expected JSON object string, got %s", result)
	}
}

func TestIsAllowedEnvVar(t *testing.T) {
	if !isAllowedEnvVar("PATH") {
		t.Error("PATH should be allowed")
	}
	if !isAllowedEnvVar("llm_box_custom") {
		t.Error("LLM_BOX_ prefix should be allowed")
	}
	if isAllowedEnvVar("SECRET") {
		t.Error("SECRET should not be allowed")
	}
}

// TestTemplateLRU_Eviction verifies the template cache evicts the
// least-recently-used entry once capacity is exceeded, and that a read
// promotes an entry so it is not evicted next.
func TestTemplateLRU_Eviction(t *testing.T) {
	c := newTemplateLRU(3)

	mk := func(s string) *compiledTemplate { return &compiledTemplate{literal: s} }

	// Fill to capacity: order is a(MRU) .. c(LRU).
	c.loadOrStore("a", mk("A"))
	c.loadOrStore("b", mk("B"))
	c.loadOrStore("c", mk("C"))

	if v, ok := c.load("a"); !ok || v.literal != "A" {
		t.Fatalf("expected a present, got %v ok=%v", v, ok)
	}
	if v, ok := c.load("c"); !ok || v.literal != "C" {
		t.Fatalf("expected c present, got %v ok=%v", v, ok)
	}

	// Promote "c" to MRU by reading it, so "b" becomes the LRU candidate.
	if _, ok := c.load("c"); !ok {
		t.Fatal("expected c present before promotion")
	}

	// Inserting a 4th entry must evict the LRU ("b"), not "c".
	c.loadOrStore("d", mk("D"))

	if _, ok := c.load("b"); ok {
		t.Error("expected b to be evicted as the least-recently-used entry")
	}
	if v, ok := c.load("c"); !ok || v.literal != "C" {
		t.Errorf("expected c to survive after promotion, got %v ok=%v", v, ok)
	}
	if v, ok := c.load("d"); !ok || v.literal != "D" {
		t.Errorf("expected d present, got %v ok=%v", v, ok)
	}
	if c.order.Len() > c.cap {
		t.Errorf("cache size %d exceeds capacity %d", c.order.Len(), c.cap)
	}
}

// TestTemplateLRU_LoadOrStore_Dedup ensures a second store of the same key
// returns the originally-cached value rather than replacing it (matching the
// original sync.Map.LoadOrStore semantics).
func TestTemplateLRU_LoadOrStore_Dedup(t *testing.T) {
	c := newTemplateLRU(4)
	first := &compiledTemplate{literal: "first"}
	got := c.loadOrStore("k", first)
	if got != first {
		t.Fatal("first store should return the stored value")
	}
	second := &compiledTemplate{literal: "second"}
	got = c.loadOrStore("k", second)
	if got != first {
		t.Errorf("dedup should return original value; got %v want %v", got, first)
	}
}
