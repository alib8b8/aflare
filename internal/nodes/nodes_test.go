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
	"os"
	"path/filepath"
	"testing"
)

func TestSafeJoinPath_Basic(t *testing.T) {
	tmpDir := t.TempDir()

	safePath, err := safeJoinPath(tmpDir, "test.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(tmpDir, "test.txt")
	if safePath != expected {
		t.Errorf("expected '%s', got '%s'", expected, safePath)
	}
}

func TestSafeJoinPath_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := safeJoinPath(tmpDir, "../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal with ..")
	}
}

func TestSafeJoinPath_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := safeJoinPath(tmpDir, "/etc/passwd")
	if err == nil {
		t.Error("expected error for absolute path")
	}
}

func TestSafeJoinPath_EmptyPath(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := safeJoinPath(tmpDir, "")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestValidateWritePath_AllowedExtensions(t *testing.T) {
	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	// Note: .py/.sh/.go are intentionally NOT allowed for file_write to prevent
	// writing executable/script files; only data/document extensions are allowed.
	allowedExts := []string{".txt", ".md", ".yaml", ".yml", ".json", ".csv", ".xml", ".log"}
	for _, ext := range allowedExts {
		path := "test" + ext
		_, err := validateWritePath(path)
		if err != nil {
			t.Errorf("unexpected error for %s: %v", ext, err)
		}
	}
}

func TestFileReadWrite_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	ctx := context.Background()
	writeNode := &FileWriteNode{}
	readNode := &FileReadNode{}

	testContent := "hello world"

	_, err := writeNode.Execute(ctx, testContent, map[string]string{"path": "test_output.txt"})
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	output, err := readNode.Execute(ctx, "", map[string]string{"path": "test_output.txt"})
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if output != testContent {
		t.Errorf("expected '%s', got '%s'", testContent, output)
	}
}

func TestFileWrite_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	ctx := context.Background()
	writeNode := &FileWriteNode{}

	_, err := writeNode.Execute(ctx, "evil", map[string]string{"path": "../evil.txt"})
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestFileRead_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	ctx := context.Background()
	readNode := &FileReadNode{}

	_, err := readNode.Execute(ctx, "", map[string]string{"path": "../../etc/passwd"})
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestTemplateRender_InlineTemplate(t *testing.T) {
	ctx := context.Background()
	node := &TemplateRenderNode{}

	output, err := node.Execute(ctx, "world", map[string]string{
		"template": "Hello {{.input | upper}}!",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Hello WORLD!"
	if output != expected {
		t.Errorf("expected '%s', got '%s'", expected, output)
	}
}

func TestTemplateRender_TemplateFile(t *testing.T) {
	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	templateContent := "Greeting: {{.greeting}}, Name: {{.input}}"
	err := os.WriteFile(filepath.Join(tmpDir, "test.tmpl"), []byte(templateContent), 0644)
	if err != nil {
		t.Fatalf("failed to write template file: %v", err)
	}

	ctx := context.Background()
	node := &TemplateRenderNode{}

	output, err := node.Execute(ctx, "Alice", map[string]string{
		"template_file": "test.tmpl",
		"greeting":      "Hi",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Greeting: Hi, Name: Alice"
	if output != expected {
		t.Errorf("expected '%s', got '%s'", expected, output)
	}
}

func TestJSONParse_Basic(t *testing.T) {
	ctx := context.Background()
	node := &JSONParseNode{}

	input := `{"name": "test", "value": 42}`
	output, err := node.Execute(ctx, input, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestJSONParse_WithPath(t *testing.T) {
	ctx := context.Background()
	node := &JSONParseNode{}

	input := `{"user": {"name": "Alice", "age": 30}}`
	output, err := node.Execute(ctx, input, map[string]string{"path": "user.name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Alice"
	if output != expected {
		t.Errorf("expected '%s', got '%s'", expected, output)
	}
}

func TestJSONParse_ArrayIndex(t *testing.T) {
	ctx := context.Background()
	node := &JSONParseNode{}

	input := `{"items": ["a", "b", "c"]}`
	output, err := node.Execute(ctx, input, map[string]string{"path": "items.[1]"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "b"
	if output != expected {
		t.Errorf("expected '%s', got '%s'", expected, output)
	}
}

func TestJSONParse_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	node := &JSONParseNode{}

	_, err := node.Execute(ctx, "not json", map[string]string{})
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestExecuteNode_SafeMode(t *testing.T) {
	oldSafeMode := IsSafeMode()
	defer SetSafeMode(oldSafeMode)

	SetSafeMode(true)

	ctx := context.Background()
	node := &ExecuteNode{}

	_, err := node.Execute(ctx, "", map[string]string{"command": "echo hello"})
	if err == nil {
		t.Error("expected error in safe mode")
	}
}

func TestRegistry_SafeMode(t *testing.T) {
	reg := NewRegistry()

	if reg.IsSafeMode() {
		t.Error("safe mode should be off by default")
	}

	reg.SetSafeMode(true)
	if !reg.IsSafeMode() {
		t.Error("safe mode should be on after SetSafeMode(true)")
	}

	reg.SetSafeMode(false)
	if reg.IsSafeMode() {
		t.Error("safe mode should be off after SetSafeMode(false)")
	}
}

func TestValidateURL_Valid(t *testing.T) {
	// Note: URLs with userinfo (user:pass@host) are blocked to prevent
	// credential injection. LLM endpoints that need loopback should use
	// validateLMLEndpoint instead.
	validURLs := []string{
		"https://example.com",
		"http://example.com/path",
		"https://api.openai.com/v1/chat/completions",
	}
	for _, u := range validURLs {
		err := validateURL(u)
		if err != nil {
			t.Errorf("expected no error for %s, got: %v", u, err)
		}
	}
}

func TestValidateURL_Localhost(t *testing.T) {
	blocked := []string{
		"http://localhost/",
		"http://localhost:8080/",
		"http://127.0.0.1/",
		"http://127.0.0.1:3000/",
		"https://localhost.localdomain/",
	}
	for _, u := range blocked {
		err := validateURL(u)
		if err == nil {
			t.Errorf("expected error for %s", u)
		}
	}
}

func TestValidateURL_PrivateIP(t *testing.T) {
	blocked := []string{
		"http://192.168.1.1/",
		"http://10.0.0.1/",
		"http://172.16.0.1/",
		"http://172.31.255.255/",
		"http://169.254.1.1/",
	}
	for _, u := range blocked {
		err := validateURL(u)
		if err == nil {
			t.Errorf("expected error for %s", u)
		}
	}
}

func TestValidateURL_InvalidScheme(t *testing.T) {
	blocked := []string{
		"file:///etc/passwd",
		"ftp://example.com",
		"gopher://example.com",
		"javascript:alert(1)",
	}
	for _, u := range blocked {
		err := validateURL(u)
		if err == nil {
			t.Errorf("expected error for %s", u)
		}
	}
}

func TestValidateURL_UserInfoBlocked(t *testing.T) {
	// URLs with userinfo (user:pass@host) must be blocked to prevent
	// credential injection.
	blocked := []string{
		"https://user:pass@example.com",
		"http://admin:secret@127.0.0.1/",
	}
	for _, u := range blocked {
		err := validateURL(u)
		if err == nil {
			t.Errorf("expected error for userinfo URL %s", u)
		}
	}
}

func TestValidateLMLEndpoint_AllowsLoopback(t *testing.T) {
	// LLM endpoints (e.g. Ollama) commonly run on localhost/loopback and
	// must be allowed by validateLMLEndpoint.
	allowed := []string{
		"http://localhost:11434",
		"http://127.0.0.1:11434",
		"https://localhost:8080",
		"http://[::1]:11434",
	}
	for _, u := range allowed {
		err := validateLMLEndpoint(u)
		if err != nil {
			t.Errorf("expected no error for LLM endpoint %s, got: %v", u, err)
		}
	}
}

func TestValidateLMLEndpoint_BlocksPrivate(t *testing.T) {
	// Non-loopback private/reserved ranges must still be blocked to prevent
	// SSRF via the LLM endpoint parameter.
	blocked := []string{
		"http://192.168.1.1:11434",
		"http://10.0.0.1:11434",
		"http://172.16.0.1:11434",
		"http://169.254.1.1:11434",
		"http://0.0.0.0:11434",
	}
	for _, u := range blocked {
		err := validateLMLEndpoint(u)
		if err == nil {
			t.Errorf("expected error for private LLM endpoint %s", u)
		}
	}
}

func TestValidateLMLEndpoint_BlocksBadScheme(t *testing.T) {
	blocked := []string{
		"file:///etc/passwd",
		"ftp://localhost:11434",
		"gopher://localhost",
	}
	for _, u := range blocked {
		err := validateLMLEndpoint(u)
		if err == nil {
			t.Errorf("expected error for %s", u)
		}
	}
}

func TestValidateLMLEndpoint_BlocksUserinfo(t *testing.T) {
	// Even for LLM endpoints, userinfo must be blocked to avoid credential
	// leakage.
	if err := validateLMLEndpoint("http://user:pass@localhost:11434"); err == nil {
		t.Error("expected error for userinfo in LLM endpoint")
	}
}

func TestRedactAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abcdefgh12345678", "abcd****5678"},
		{"short", "****"},
		{"exactly8", "****"},
		{"", "****"},
	}

	for _, tt := range tests {
		result := redactAPIKey(tt.input)
		if result != tt.expected {
			t.Errorf("redactAPIKey(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestCombineNode(t *testing.T) {
	ctx := context.Background()
	node := &CombineNode{}

	output, err := node.Execute(ctx, "line1\nline2\nline3", map[string]string{
		"format": "markdown",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "- line1\n- line2\n- line3\n"
	if output != expected {
		t.Errorf("expected '%s', got '%s'", expected, output)
	}
}

func TestConditionNode_Contains(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}

	output, err := node.Execute(ctx, "hello world", map[string]string{
		"expr": "contains:world",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "true" {
		t.Errorf("expected 'true', got %q", output)
	}

	output, _ = node.Execute(ctx, "hello world", map[string]string{
		"expr": "contains:foo",
	})
	if output != "false" {
		t.Errorf("expected 'false', got %q", output)
	}
}

func TestConditionNode_Equals(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}

	output, _ := node.Execute(ctx, "yes", map[string]string{
		"expr": "equals:yes",
	})
	if output != "true" {
		t.Errorf("expected 'true', got %q", output)
	}

	output, _ = node.Execute(ctx, "no", map[string]string{
		"expr": "equals:yes",
	})
	if output != "false" {
		t.Errorf("expected 'false', got %q", output)
	}
}

func TestConditionNode_StartsWith(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}

	output, _ := node.Execute(ctx, "https://example.com", map[string]string{
		"expr": "starts_with:https",
	})
	if output != "true" {
		t.Errorf("expected 'true', got %q", output)
	}
}

func TestConditionNode_EndsWith(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}

	output, _ := node.Execute(ctx, "report.pdf", map[string]string{
		"expr": "ends_with:.pdf",
	})
	if output != "true" {
		t.Errorf("expected 'true', got %q", output)
	}
}

func TestConditionNode_Regex(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}

	output, _ := node.Execute(ctx, "test123", map[string]string{
		"expr": `regex:\d+`,
	})
	if output != "true" {
		t.Errorf("expected 'true' for regex match, got %q", output)
	}

	output, _ = node.Execute(ctx, "no digits", map[string]string{
		"expr": `regex:\d+`,
	})
	if output != "false" {
		t.Errorf("expected 'false' for no regex match, got %q", output)
	}
}

func TestConditionNode_Empty(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}

	output, _ := node.Execute(ctx, "", map[string]string{
		"expr": "empty",
	})
	if output != "true" {
		t.Errorf("expected 'true' for empty input, got %q", output)
	}

	output, _ = node.Execute(ctx, "x", map[string]string{
		"expr": "empty",
	})
	if output != "false" {
		t.Errorf("expected 'false' for non-empty input, got %q", output)
	}
}

func TestConditionNode_NotEmpty(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}

	output, _ := node.Execute(ctx, "data", map[string]string{
		"expr": "not_empty",
	})
	if output != "true" {
		t.Errorf("expected 'true', got %q", output)
	}

	output, _ = node.Execute(ctx, "", map[string]string{
		"expr": "not_empty",
	})
	if output != "false" {
		t.Errorf("expected 'false', got %q", output)
	}
}

func TestConditionNode_Negate(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}

	output, _ := node.Execute(ctx, "hello", map[string]string{
		"expr": "not contains:world",
	})
	if output != "true" {
		t.Errorf("expected 'true' for negated contains, got %q", output)
	}

	output, _ = node.Execute(ctx, "hello world", map[string]string{
		"expr": "not contains:world",
	})
	if output != "false" {
		t.Errorf("expected 'false', got %q", output)
	}
}

func TestConditionNode_MissingParam(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}

	_, err := node.Execute(ctx, "input", map[string]string{})
	if err == nil {
		t.Error("expected error for missing expr")
	}
}

func TestConditionNode_InvalidOp(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}

	_, err := node.Execute(ctx, "input", map[string]string{
		"expr": "unknown:value",
	})
	if err == nil {
		t.Error("expected error for unknown operator")
	}
}

func TestTransformNode(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}

	output, err := node.Execute(ctx, "  Hello World  ", map[string]string{
		"operation": "trim",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Hello World"
	if output != expected {
		t.Errorf("expected '%s', got '%s'", expected, output)
	}
}
