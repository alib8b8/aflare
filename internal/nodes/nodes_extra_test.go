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
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test all node Name/Description/Schema methods to quickly boost coverage.
func TestNodeMetadata(t *testing.T) {
	tests := []struct {
		name        string
		node        Node
		wantName    string
		wantDescLen int
	}{
		{"test", &TestNode{}, "test_node", 0},
		{"openai", &OpenAINode{}, "openai", 0},
		{"coze", &CozeNode{}, "coze", 0},
		{"fastgpt", &FastGPTNode{}, "fastgpt", 0},
		{"ima", &IMANode{}, "ima", 0},
		{"http_request", &HTTPRequestNode{}, "http_request", 0},
		{"fetch_url", &FetchURLNode{}, "fetch_url", 0},
		{"file_read", &FileReadNode{}, "file_read", 0},
		{"file_write", &FileWriteNode{}, "file_write", 0},
		{"template_render", &TemplateRenderNode{}, "template_render", 0},
		{"json_parse", &JSONParseNode{}, "json_parse", 0},
		{"notify", &NotifyNode{}, "notify", 0},
		{"combine", &CombineNode{}, "combine", 0},
		{"condition", &ConditionNode{}, "condition", 0},
		{"transform", &TransformNode{}, "transform", 0},
		{"execute", &ExecuteNode{}, "execute", 0},
		{"call", &CallNode{}, "call", 0},
		{"ollama", &OllamaNode{}, "ollama", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.node.Name(); got != tt.wantName {
				t.Errorf("Name() = %q, want %q", got, tt.wantName)
			}
			if got := tt.node.Description(); got == "" {
				t.Errorf("Description() is empty")
			}
			if schema := tt.node.Schema(); schema.Description == "" {
				t.Errorf("Schema() description is empty")
			}
		})
	}
}

func TestRegistryConcurrent(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&TestNode{})

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			reg.Get("test")
		}()
	}
	for i := 0; i < 10; i++ {
		go func() {
			reg.ListNodes()
		}()
	}
	// Wait for goroutines (not strictly necessary for coverage but good practice)
	// We just need to ensure no panic
}

func TestGetGlobalRegistry(t *testing.T) {
	reg := GetGlobalRegistry()
	if reg == nil {
		t.Fatal("expected non-nil global registry")
	}
	// Registering again should not panic
	RegisterBuiltins(reg)
}

func TestRedactSensitive(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"no secrets here", "no secrets here"},
		{"Bearer abcdef123456", "Bearer ****"},
		{"Authorization: secret-token", "Authorization: ****"},
		{"api_key=shhh&other=1", "api_key=****&other=1"},
		{"password=secret123", "password=****"},
		{"passwd=secret123", "passwd=****"},
		{"token=secret123", "token=****"},
		{"secret=secret123", "secret=****"},
		{"https://user:pass@example.com", "https://user:****@example.com"},
		{"ghp_abcdefghijklmnopqrstuvwxyz1234", "ghp_****"},
		{"sk-abcdefghijklmnopqrstuvwxyz1234", "sk-****"},
		{"xoxb-1234567890-abcdefghij", "xoxb-****"},
	}

	for _, tt := range tests {
		got := RedactSensitive(tt.input)
		if got != tt.expected {
			t.Errorf("RedactSensitive(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestRedactSensitive_Truncate(t *testing.T) {
	long := strings.Repeat("a", 2000)
	got := RedactSensitive(long)
	if !strings.HasSuffix(got, "... (truncated)") {
		t.Error("expected truncation for long input")
	}
}

func TestIsSensitiveKey(t *testing.T) {
	if !isSensitiveKey("api_key") {
		t.Error("expected api_key to be sensitive")
	}
	if !isSensitiveKey("my_token") {
		t.Error("expected my_token to be sensitive")
	}
	if !isSensitiveKey("auth-header") {
		t.Error("expected auth-header to be sensitive")
	}
	if isSensitiveKey("name") {
		t.Error("expected name not to be sensitive")
	}
}

func TestHttpRedirectValidator(t *testing.T) {
	validator := httpRedirectValidator(func(string) error { return nil })
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	// 9 redirects should be fine
	via := make([]*http.Request, 9)
	for i := range via {
		via[i] = req
	}
	if err := validator(req, via); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// 10 redirects should fail
	via = make([]*http.Request, 10)
	for i := range via {
		via[i] = req
	}
	if err := validator(req, via); err == nil {
		t.Error("expected error for too many redirects")
	}
}

func TestValidateIP(t *testing.T) {
	tests := []struct {
		ip      string
		wantErr bool
	}{
		{"8.8.8.8", false},
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"169.254.1.1", true},
		{"0.0.0.0", true},
		{"224.0.0.1", true},
		{"::1", true},
		{"fe80::1", true},
		{"2001:4860:4860::8888", false},
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if ip == nil {
			t.Fatalf("invalid IP: %s", tt.ip)
		}
		err := validateIP(ip, tt.ip)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateIP(%q) error = %v, wantErr %v", tt.ip, err, tt.wantErr)
		}
	}
}

func TestIsReservedIP(t *testing.T) {
	tests := []struct {
		ip       string
		reserved bool
	}{
		{"0.1.2.3", true},
		{"169.254.1.1", true},
		{"192.0.2.1", true},
		{"198.51.100.1", true},
		{"203.0.113.1", true},
		{"100.64.0.1", true},
		{"100.127.255.255", true},
		{"8.8.8.8", false},
		{"fc00::1", true},
		{"2001:db8::1", false},
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if ip == nil {
			t.Fatalf("invalid IP: %s", tt.ip)
		}
		got := isReservedIP(ip)
		if got != tt.reserved {
			t.Errorf("isReservedIP(%q) = %v, want %v", tt.ip, got, tt.reserved)
		}
	}
}

func TestSafeJoinPath_Symlink(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	os.WriteFile(target, []byte("x"), 0644)
	link := filepath.Join(tmpDir, "link.txt")
	os.Symlink(target, link)

	got, err := safeJoinPath(tmpDir, "link.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Error("expected non-empty path")
	}
}

func TestValidateWritePath_Dotfile(t *testing.T) {
	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	_, err := validateWritePath(".secret")
	if err == nil {
		t.Error("expected error for dotfile")
	}
}

func TestValidateWritePath_DisallowedExt(t *testing.T) {
	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	_, err := validateWritePath("data.exe")
	if err == nil {
		t.Error("expected error for disallowed extension")
	}
}

func TestParseVarsParam(t *testing.T) {
	tests := []struct {
		input    string
		expected map[string]string
	}{
		{"key1=val1,key2=val2", map[string]string{"key1": "val1", "key2": "val2"}},
		{"a=1", map[string]string{"a": "1"}},
		{"", map[string]string{}},
		{"no_equals", map[string]string{}},
		{"a=1,", map[string]string{"a": "1"}},
	}
	for _, tt := range tests {
		got := parseVarsParam(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("parseVarsParam(%q) = %v, want %v", tt.input, got, tt.expected)
		}
		for k, v := range tt.expected {
			if got[k] != v {
				t.Errorf("parseVarsParam(%q)[%s] = %q, want %q", tt.input, k, got[k], v)
			}
		}
	}
}

func TestInjectVarsIntoWorkflow(t *testing.T) {
	wf := "steps:\n  - node: test\n"
	vars := map[string]string{"name": "World"}
	got, err := injectVarsIntoWorkflow(wf, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "World") {
		t.Errorf("expected injected var, got %q", got)
	}
}

func TestCallNode_MissingWorkflow(t *testing.T) {
	ctx := context.Background()
	node := &CallNode{}
	_, err := node.Execute(ctx, "", map[string]string{})
	if err == nil {
		t.Error("expected error for missing workflow param")
	}
}

func TestCombineNode_Formats(t *testing.T) {
	ctx := context.Background()
	node := &CombineNode{}
	input := "line1\nline2"

	formats := []string{"csv", "json", "text", "markdown"}
	for _, f := range formats {
		output, err := node.Execute(ctx, input, map[string]string{"format": f})
		if err != nil {
			t.Errorf("format %s failed: %v", f, err)
		}
		if output == "" {
			t.Errorf("format %s returned empty", f)
		}
	}
}

func TestCombineNode_InvalidFormat(t *testing.T) {
	ctx := context.Background()
	node := &CombineNode{}
	output, err := node.Execute(ctx, "x", map[string]string{"format": "unknown"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Unknown format returns input as-is
	if output != "x" {
		t.Errorf("expected passthrough for unknown format, got %q", output)
	}
}

func TestFetchURLNode_LooksLikeURL(t *testing.T) {
	if !looksLikeURL("https://example.com") {
		t.Error("expected URL detection")
	}
	if looksLikeURL("not a url") {
		t.Error("expected non-URL")
	}
}

func TestFetchURLNode_ExtractTextFromHTML(t *testing.T) {
	html := "<html><body><p>Hello</p></body></html>"
	got := extractTextFromHTML(html)
	if !strings.Contains(got, "Hello") {
		t.Errorf("expected Hello in text, got %q", got)
	}
}

func TestFetchURLNode_ExtractMainContent(t *testing.T) {
	html := `<article><p>Main content</p></article>`
	got := extractMainContent(html)
	if !strings.Contains(got, "Main content") {
		t.Errorf("expected main content, got %q", got)
	}
}

func TestFetchURLNode_HtmlToMarkdown(t *testing.T) {
	html := "<h1>Title</h1><p>Text</p>"
	got := htmlToMarkdown(html)
	if !strings.Contains(got, "Title") {
		t.Errorf("expected Title in markdown, got %q", got)
	}
}

func TestExecuteNode_RedactCommandForLog(t *testing.T) {
	cmd := redactCommandForLog("echo secret")
	// redactCommandForLog only redacts bearer/auth/api_key/password patterns
	if cmd != "echo secret" {
		t.Errorf("unexpected redaction: %q", cmd)
	}
	cmd = redactCommandForLog("curl -H authorization: secret-token https://example.com")
	if strings.Contains(cmd, "secret-token") {
		t.Error("expected authorization token to be redacted")
	}
}

func TestExecuteNode_EscapeLogContent(t *testing.T) {
	got := escapeLogContent("hello\nworld\r\n")
	if strings.Contains(got, "\n") {
		t.Error("expected newlines escaped")
	}
}

func TestExecuteNode_AuditLog(t *testing.T) {
	// auditLog writes to stdout; just ensure no panic
	auditLog("test")
}

func TestExecuteNode_UnsafeMode(t *testing.T) {
	oldSafeMode := IsSafeMode()
	defer SetSafeMode(oldSafeMode)
	SetSafeMode(false)

	ctx := context.Background()
	node := &ExecuteNode{}
	// In unsafe mode, it will try to execute "echo hello" and should succeed on most systems
	output, err := node.Execute(ctx, "", map[string]string{"command": "echo hello"})
	if err != nil {
		t.Logf("Execute error (may be OS-specific): %v", err)
	} else {
		if !strings.Contains(output, "hello") {
			t.Errorf("expected hello in output, got %q", output)
		}
	}
}

func TestFileWriteNode_AppendToFile(t *testing.T) {
	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	path := filepath.Join(tmpDir, "append.txt")
	os.WriteFile(path, []byte("first"), 0644)

	if err := appendToFile(path, []byte("second")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "firstsecond\n" {
		t.Errorf("expected firstsecond\\n, got %q", data)
	}
}

func TestNotifyNode(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}

	// Test stdout notify (default channel)
	_, err := node.Execute(ctx, "test message", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test unknown channel
	_, err = node.Execute(ctx, "test", map[string]string{"channel": "unknown"})
	if err == nil {
		t.Error("expected error for unknown notify channel")
	}
}

func TestTransformNode_Operations(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}

	tests := []struct {
		op       string
		input    string
		expected string
	}{
		{"upper", "hello", "HELLO"},
		{"lower", "HELLO", "hello"},
		{"reverse", "abc", "cba"},
		{"trim", "  hello  ", "hello"},
		{"lines", "a\nb\nc", "3 lines"},
		{"words", "hello world", "2 words"},
		{"chars", "hello", "5 characters"},
		{"first_line", "hello\nworld", "hello"},
		{"first_500", strings.Repeat("a", 600), strings.Repeat("a", 500) + "..."},
		{"first_1000", strings.Repeat("a", 1200), strings.Repeat("a", 1000) + "..."},
		{"summary", strings.Repeat("word ", 100), strings.Repeat("word ", 50)[:200] + "..."},
		{"unique_lines", "a\na\nb", "a\nb"},
		{"sort_lines", "c\na\nb", "a\nb\nc"},
		{"remove_blank_lines", "a\n\nb", "a\nb"},
		{"filter_errors", "ok\nerror here\nfine", "error here"},
		{"extract_urls", "visit https://example.com and http://test.org", "https://example.com\nhttp://test.org"},
		{"extract_emails", "contact a@b.com or c@d.net", "a@b.com\nc@d.net"},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			output, err := node.Execute(ctx, tt.input, map[string]string{"operation": tt.op})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.TrimSpace(output) != strings.TrimSpace(tt.expected) {
				t.Errorf("%s: expected %q, got %q", tt.op, tt.expected, output)
			}
		})
	}
}

func TestTransformNode_DefaultNoOp(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	output, err := node.Execute(ctx, "hello", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "hello" {
		t.Errorf("expected passthrough, got %q", output)
	}
}

func TestTransformNode_UnknownOpPassthrough(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	output, err := node.Execute(ctx, "hello", map[string]string{"operation": "unknown"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "hello" {
		t.Errorf("expected passthrough for unknown op, got %q", output)
	}
}

func TestTransformNode_MarkdownToHTML(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	output, err := node.Execute(ctx, "# Hello", map[string]string{"operation": "markdown_to_html"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "<h1>") {
		t.Errorf("expected HTML h1, got %q", output)
	}
}

func TestTransformNode_HTMLToMarkdown(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	output, err := node.Execute(ctx, "<h1>Hello</h1>", map[string]string{"operation": "html_to_markdown"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Hello") {
		t.Errorf("expected markdown, got %q", output)
	}
}

func TestTransformNode_ExtractReposAndActivity(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	input := `<a href="/user/repo1">Repo1</a> <a href="/user/repo2">Repo2</a>`
	output, err := node.Execute(ctx, input, map[string]string{"operation": "extract_repos_and_activity"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestTransformNode_CombineAndSummarize(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	input := "part1\n---\npart2"
	output, err := node.Execute(ctx, input, map[string]string{"operation": "combine_and_summarize"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestTransformNode_ExtractFunctionsAndTypes(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	input := "func Add(a, b int) int { return a + b }\ntype User struct { Name string }"
	output, err := node.Execute(ctx, input, map[string]string{"operation": "extract_functions_and_types"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Add") {
		t.Errorf("expected function name in output, got %q", output)
	}
}

func TestTransformNode_GroupByCommitType(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	input := "feat: new feature\nfix: bug fix\nfeat: another feature"
	output, err := node.Execute(ctx, input, map[string]string{"operation": "group_by_commit_type"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestTransformNode_GroupByExtension(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	input := "file.go\nfile_test.go\nreadme.md"
	output, err := node.Execute(ctx, input, map[string]string{"operation": "group_by_extension"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestTransformNode_CountByLabel(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	input := "bug: something\nfeature: new\nbug: another"
	output, err := node.Execute(ctx, input, map[string]string{"operation": "count_by_label"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestConditionNode_SafeRegexMatch(t *testing.T) {
	matched, err := SafeRegexMatch(`\d+`, "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Error("expected match")
	}
	matched, err = SafeRegexMatch(`\d+`, "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched {
		t.Error("expected no match")
	}
}

func TestExternalNode_Methods(t *testing.T) {
	node := NewExternalNode(NodeMetadata{
		Name:        "ext",
		Description: "desc",
	}, "/nonexistent")
	if node.Name() != "ext" {
		t.Error("name mismatch")
	}
	if node.Description() != "desc" {
		t.Error("description mismatch")
	}
	schema := node.Schema()
	if schema.Name != "ext" {
		t.Error("schema name mismatch")
	}
}

func TestExternalNode_Execute(t *testing.T) {
	node := NewExternalNode(NodeMetadata{
		Name: "ext",
	}, "/nonexistent/script.sh")
	ctx := context.Background()
	_, err := node.Execute(ctx, "input", map[string]string{})
	if err == nil {
		t.Error("expected error for missing script")
	}
}

func TestLoadExternalNodes(t *testing.T) {
	tmpDir := t.TempDir()
	// Create nodes dir with a dummy file
	os.WriteFile(filepath.Join(tmpDir, "dummy.yaml"), []byte("invalid yaml"), 0644)

	// Should not panic even with invalid files
	_ = LoadExternalNodes(tmpDir)
}

type dummyNode struct{ name string }

func (d *dummyNode) Name() string        { return d.name }
func (d *dummyNode) Description() string { return "dummy" }
func (d *dummyNode) Schema() NodeSchema  { return NodeSchema{} }
func (d *dummyNode) Execute(context.Context, string, map[string]string) (string, error) {
	return "", nil
}

func TestListNodes(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&dummyNode{name: "a"})
	reg.Register(&dummyNode{name: "b"})
	names := reg.ListNodes()
	if len(names) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(names))
	}
}

func TestCombineNode_EmptyInput(t *testing.T) {
	ctx := context.Background()
	node := &CombineNode{}
	output, err := node.Execute(ctx, "", map[string]string{"format": "text"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Errorf("expected empty output, got %q", output)
	}
}

func TestCombineNode_CSVWithComma(t *testing.T) {
	ctx := context.Background()
	node := &CombineNode{}
	output, err := node.Execute(ctx, "a,b\nc,d", map[string]string{"format": "csv"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "a,b") {
		t.Errorf("expected csv content, got %q", output)
	}
}

func TestCallNode_ExecuteFileNotFound(t *testing.T) {
	ctx := context.Background()
	node := &CallNode{}
	_, err := node.Execute(ctx, "", map[string]string{"workflow": "/nonexistent.yaml"})
	if err == nil {
		t.Error("expected error for missing workflow file")
	}
}

func TestConditionNode_Expressions(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}

	tests := []struct {
		expr     string
		input    string
		expected string
	}{
		{"contains:hello", "hello world", "true"},
		{"contains:xyz", "hello world", "false"},
		{"equals:hello", "hello", "true"},
		{"equals:hello", "world", "false"},
		{"starts_with:he", "hello", "true"},
		{"ends_with:lo", "hello", "true"},
		{"empty", "", "true"},
		{"empty", "x", "false"},
		{"not_empty", "x", "true"},
		{"not_empty", "", "false"},
		{"true", "", "true"},
		{"false", "", "false"},
		{"not contains:xyz", "hello", "true"},
		{"regex:^h.*o$", "hello", "true"},
		{"regex:^x.*z$", "hello", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			output, err := node.Execute(ctx, tt.input, map[string]string{"expr": tt.expr})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, output)
			}
		})
	}
}

func TestConditionNode_MissingExpr(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}
	_, err := node.Execute(ctx, "input", map[string]string{})
	if err == nil {
		t.Error("expected error for missing expr")
	}
}

func TestConditionNode_InvalidFormat(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}
	_, err := node.Execute(ctx, "input", map[string]string{"expr": "invalid"})
	if err == nil {
		t.Error("expected error for invalid condition format")
	}
}

func TestConditionNode_UnsupportedOp(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}
	_, err := node.Execute(ctx, "input", map[string]string{"expr": "unknown:value"})
	if err == nil {
		t.Error("expected error for unsupported operator")
	}
}

func TestFileReadNode(t *testing.T) {
	ctx := context.Background()
	node := &FileReadNode{}

	// Missing path
	_, err := node.Execute(ctx, "", map[string]string{})
	if err == nil {
		t.Error("expected error for missing path")
	}

	// Valid file
	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)
	output, err := node.Execute(ctx, "", map[string]string{"path": "test.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "hello world" {
		t.Errorf("expected hello world, got %q", output)
	}
}

func TestFileWriteNode_WriteMode(t *testing.T) {
	ctx := context.Background()
	node := &FileWriteNode{}

	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	output, err := node.Execute(ctx, "hello", map[string]string{"path": "test.txt", "mode": "write"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "written") {
		t.Errorf("expected written message, got %q", output)
	}

	data, _ := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
	if string(data) != "hello" {
		t.Errorf("expected hello, got %q", data)
	}
}

func TestFileWriteNode_InvalidMode(t *testing.T) {
	ctx := context.Background()
	node := &FileWriteNode{}

	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	_, err := node.Execute(ctx, "x", map[string]string{"path": "test.txt", "mode": "invalid"})
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestFileWriteNode_MissingPath(t *testing.T) {
	ctx := context.Background()
	node := &FileWriteNode{}
	_, err := node.Execute(ctx, "x", map[string]string{})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestJSONParseNode(t *testing.T) {
	ctx := context.Background()
	node := &JSONParseNode{}

	// Pretty print
	output, err := node.Execute(ctx, `{"name":"test"}`, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "name") {
		t.Errorf("expected pretty JSON, got %q", output)
	}

	// Extract path
	output, err = node.Execute(ctx, `{"user":{"name":"alice"}}`, map[string]string{"path": "user.name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "alice" {
		t.Errorf("expected alice, got %q", output)
	}

	// Array index
	output, err = node.Execute(ctx, `["a","b","c"]`, map[string]string{"path": "[1]"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "b" {
		t.Errorf("expected b, got %q", output)
	}

	// Invalid JSON
	_, err = node.Execute(ctx, `not json`, map[string]string{})
	if err == nil {
		t.Error("expected error for invalid JSON")
	}

	// Missing key
	_, err = node.Execute(ctx, `{"a":1}`, map[string]string{"path": "b"})
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestTemplateRenderNode(t *testing.T) {
	ctx := context.Background()
	node := &TemplateRenderNode{}

	// Inline template
	output, err := node.Execute(ctx, "world", map[string]string{"template": "Hello {{.input}}"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "Hello world" {
		t.Errorf("expected Hello world, got %q", output)
	}

	// Missing template
	_, err = node.Execute(ctx, "", map[string]string{})
	if err == nil {
		t.Error("expected error for missing template")
	}

	// Invalid template
	_, err = node.Execute(ctx, "", map[string]string{"template": "{{.bad"})
	if err == nil {
		t.Error("expected error for invalid template syntax")
	}
}

func TestTemplateRenderNode_FromFile(t *testing.T) {
	ctx := context.Background()
	node := &TemplateRenderNode{}

	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	tmplFile := filepath.Join(tmpDir, "tmpl.txt")
	os.WriteFile(tmplFile, []byte("Value: {{.input}}"), 0644)

	output, err := node.Execute(ctx, "42", map[string]string{"template_file": "tmpl.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "42") {
		t.Errorf("expected 42 in output, got %q", output)
	}
}

func TestValidateLMLEndpoint(t *testing.T) {
	// Allow localhost
	if err := validateLMLEndpoint("http://localhost:11434"); err != nil {
		t.Errorf("unexpected error for localhost: %v", err)
	}
	// Allow loopback IP
	if err := validateLMLEndpoint("http://127.0.0.1:11434"); err != nil {
		t.Errorf("unexpected error for loopback IP: %v", err)
	}
	// Block private IP
	if err := validateLMLEndpoint("http://10.0.0.1:11434"); err == nil {
		t.Error("expected error for private IP")
	}
	// Block non-HTTP scheme
	if err := validateLMLEndpoint("ftp://example.com"); err == nil {
		t.Error("expected error for non-HTTP scheme")
	}
	// Block userinfo
	if err := validateLMLEndpoint("http://user:pass@example.com"); err == nil {
		t.Error("expected error for userinfo")
	}
}

func TestValidateLMLEndpointIP(t *testing.T) {
	ip := net.ParseIP("8.8.8.8")
	if err := validateLMLEndpointIP(ip, "8.8.8.8"); err != nil {
		t.Errorf("unexpected error for public IP: %v", err)
	}
	ip = net.ParseIP("10.0.0.1")
	if err := validateLMLEndpointIP(ip, "10.0.0.1"); err == nil {
		t.Error("expected error for private IP")
	}
}

func TestSafeRegexMatch_Limits(t *testing.T) {
	// Too long pattern
	longPattern := strings.Repeat("a", maxRegexPatternLength+1)
	_, err := SafeRegexMatch(longPattern, "test")
	if err == nil {
		t.Error("expected error for long pattern")
	}

	// Too long input
	longInput := strings.Repeat("a", maxRegexInputLength+1)
	_, err = SafeRegexMatch("a", longInput)
	if err == nil {
		t.Error("expected error for long input")
	}

	// Invalid regex
	_, err = SafeRegexMatch("[invalid", "test")
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestCallNode_MaxDepth(t *testing.T) {
	ctx := context.WithValue(context.Background(), callDepthKey, MaxCallDepth)
	node := &CallNode{}
	_, err := node.Execute(ctx, "", map[string]string{"workflow": "test.yaml"})
	if err == nil {
		t.Error("expected error for max depth exceeded")
	}
}

func TestCallNode_AbsolutePath(t *testing.T) {
	ctx := context.Background()
	node := &CallNode{}
	_, err := node.Execute(ctx, "", map[string]string{"workflow": "/etc/passwd"})
	if err == nil {
		t.Error("expected error for absolute path")
	}
}

func TestAtomicWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "atomic.txt")
	if err := atomicWriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "data" {
		t.Errorf("expected data, got %q", data)
	}
}

func TestAtomicWriteFile_InvalidDir(t *testing.T) {
	// Write to a non-existent directory should fail
	err := atomicWriteFile("/nonexistent/dir/file.txt", []byte("data"), 0644)
	if err == nil {
		t.Error("expected error for invalid directory")
	}
}

func TestCallNode_ExecuteWorkflowFuncNotSet(t *testing.T) {
	oldFunc := ExecuteWorkflowFunc
	ExecuteWorkflowFunc = nil
	defer func() { ExecuteWorkflowFunc = oldFunc }()

	ctx := context.Background()
	node := &CallNode{}
	_, err := node.Execute(ctx, "", map[string]string{"workflow": "test.yaml"})
	if err == nil {
		t.Error("expected error when ExecuteWorkflowFunc is nil")
	}
}

func TestCallNode_ExecuteWorkflowFunc(t *testing.T) {
	oldFunc := ExecuteWorkflowFunc
	ExecuteWorkflowFunc = func(ctx context.Context, wf interface{}, reg *Registry) (string, []interface{}, error) {
		return "called", nil, nil
	}
	defer func() { ExecuteWorkflowFunc = oldFunc }()

	ctx := context.Background()
	node := &CallNode{}

	// Valid relative path
	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	wfPath := filepath.Join(tmpDir, "test.yaml")
	os.WriteFile(wfPath, []byte("test"), 0644)

	output, err := node.Execute(ctx, "hello", map[string]string{"workflow": "test.yaml"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "called" {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestCallNode_WithVars(t *testing.T) {
	oldFunc := ExecuteWorkflowFunc
	ExecuteWorkflowFunc = func(ctx context.Context, wf interface{}, reg *Registry) (string, []interface{}, error) {
		return "ok", nil, nil
	}
	defer func() { ExecuteWorkflowFunc = oldFunc }()

	ctx := context.Background()
	node := &CallNode{}

	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	wfPath := filepath.Join(tmpDir, "test.yaml")
	os.WriteFile(wfPath, []byte("steps:\n"), 0644)

	output, err := node.Execute(ctx, "hello", map[string]string{"workflow": "test.yaml", "vars": "key=world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "ok" {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestExternalNode_Execute_Script(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "script.sh")
	os.WriteFile(scriptPath, []byte("#!/bin/sh\necho 'hello from script'"), 0755)

	node := NewExternalNode(NodeMetadata{
		Name: "ext",
	}, scriptPath)
	ctx := context.Background()
	output, err := node.Execute(ctx, "input", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "hello from script") {
		t.Errorf("expected script output, got %q", output)
	}
}

func TestLoadExternalNodes_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	nodeDir := filepath.Join(tmpDir, "myext")
	os.MkdirAll(nodeDir, 0755)

	scriptPath := filepath.Join(nodeDir, "ext.sh")
	os.WriteFile(scriptPath, []byte("#!/bin/sh\necho ok"), 0755)

	yamlPath := filepath.Join(nodeDir, "metadata.yaml")
	os.WriteFile(yamlPath, []byte("name: myext\ndescription: test ext\nentry: ext.sh\n"), 0644)

	reg := NewRegistry()
	if err := reg.LoadExternalNodes(tmpDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := reg.Get("myext"); !ok {
		t.Error("expected myext to be loaded")
	}
}

func TestLoadExternalNodes_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	nodeDir := filepath.Join(tmpDir, "bad")
	os.MkdirAll(nodeDir, 0755)
	os.WriteFile(filepath.Join(nodeDir, "metadata.yaml"), []byte("invalid: ["), 0644)
	reg := NewRegistry()
	if err := reg.LoadExternalNodes(tmpDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadExternalNodes_MissingEntry(t *testing.T) {
	tmpDir := t.TempDir()
	nodeDir := filepath.Join(tmpDir, "bad")
	os.MkdirAll(nodeDir, 0755)
	os.WriteFile(filepath.Join(nodeDir, "metadata.yaml"), []byte("name: bad\ndescription: test\n"), 0644)
	reg := NewRegistry()
	if err := reg.LoadExternalNodes(tmpDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteNode_CommandNotAllowed(t *testing.T) {
	oldSafeMode := IsSafeMode()
	defer SetSafeMode(oldSafeMode)
	SetSafeMode(true)

	ctx := context.Background()
	node := &ExecuteNode{}
	_, err := node.Execute(ctx, "", map[string]string{"command": "malicious"})
	if err == nil {
		t.Error("expected error for disallowed command in safe mode")
	}
}

func TestNotifyNode_MailError(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	// Mail without required env vars should fail
	_, err := node.Execute(ctx, "test", map[string]string{"channel": "mail"})
	if err == nil {
		t.Error("expected error for mail without SMTP config")
	}
}

func TestNotifyNode_WebhookError(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	// Webhook without URL should fail
	_, err := node.Execute(ctx, "test", map[string]string{"channel": "webhook"})
	if err == nil {
		t.Error("expected error for webhook without URL")
	}
}

func TestValidateReadPath_OutsideWorkDir(t *testing.T) {
	_, err := validateReadPath("../../etc/passwd")
	if err == nil {
		t.Error("expected error for path outside work dir")
	}
}

func TestValidateReadPath_SymlinkOutside(t *testing.T) {
	tmpDir := t.TempDir()
	outside := filepath.Join(tmpDir, "outside.txt")
	os.WriteFile(outside, []byte("x"), 0644)
	link := filepath.Join(tmpDir, "link.txt")
	os.Symlink(outside, link)

	oldWorkDir := workDir
	workDir = filepath.Join(tmpDir, "subdir")
	os.MkdirAll(workDir, 0750)
	defer func() { workDir = oldWorkDir }()

	// The symlink points outside workDir
	_, err := validateReadPath(filepath.Join("..", "link.txt"))
	if err == nil {
		t.Error("expected error for symlink pointing outside work dir")
	}
}

func TestFileReadNode_OutsideWorkDir(t *testing.T) {
	ctx := context.Background()
	node := &FileReadNode{}
	_, err := node.Execute(ctx, "", map[string]string{"path": "../../etc/passwd"})
	if err == nil {
		t.Error("expected error for path outside work dir")
	}
}

func TestFileWriteNode_OutsideWorkDir(t *testing.T) {
	ctx := context.Background()
	node := &FileWriteNode{}
	_, err := node.Execute(ctx, "x", map[string]string{"path": "../../etc/passwd"})
	if err == nil {
		t.Error("expected error for path outside work dir")
	}
}

func TestJSONParseNode_InvalidPath(t *testing.T) {
	ctx := context.Background()
	node := &JSONParseNode{}
	_, err := node.Execute(ctx, `{"a":1}`, map[string]string{"path": "["})
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestJSONParseNode_NestedArray(t *testing.T) {
	ctx := context.Background()
	node := &JSONParseNode{}
	output, err := node.Execute(ctx, `{"items":[{"name":"a"},{"name":"b"}]}`, map[string]string{"path": "items.[0].name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "a" {
		t.Errorf("expected a, got %q", output)
	}
}

func TestConditionNode_NotContains(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}
	output, err := node.Execute(ctx, "hello", map[string]string{"expr": "not contains:xyz"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "true" {
		t.Errorf("expected true for not contains, got %s", output)
	}
}

func TestConditionNode_RegexCompileError(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}
	_, err := node.Execute(ctx, "test", map[string]string{"expr": "regex:[invalid"})
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestTransformNode_First500Short(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	output, err := node.Execute(ctx, "hi", map[string]string{"operation": "first_500"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "hi" {
		t.Errorf("expected hi, got %q", output)
	}
}

func TestTransformNode_First1000Short(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	output, err := node.Execute(ctx, "hi", map[string]string{"operation": "first_1000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "hi" {
		t.Errorf("expected hi, got %q", output)
	}
}

func TestTransformNode_SummaryShort(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	output, err := node.Execute(ctx, "hi", map[string]string{"operation": "summary"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "hi" {
		t.Errorf("expected hi, got %q", output)
	}
}

func TestTransformNode_RemoveBlankLines(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	output, err := node.Execute(ctx, "a\n\nb\n\n\nc", map[string]string{"operation": "remove_blank_lines"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(output, "\n\n") {
		t.Errorf("expected no blank lines, got %q", output)
	}
}

func TestTransformNode_FilterErrors(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	output, err := node.Execute(ctx, "ok\nfine", map[string]string{"operation": "filter_errors"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Errorf("expected empty output when no errors, got %q", output)
	}
}

func TestTransformNode_ExtractURLs_None(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	output, err := node.Execute(ctx, "no urls here", map[string]string{"operation": "extract_urls"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Errorf("expected empty output, got %q", output)
	}
}

func TestTransformNode_ExtractEmails_None(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	output, err := node.Execute(ctx, "no emails here", map[string]string{"operation": "extract_emails"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Errorf("expected empty output, got %q", output)
	}
}

func TestCombineNode_JSONFormat(t *testing.T) {
	ctx := context.Background()
	node := &CombineNode{}
	output, err := node.Execute(ctx, "a\nb", map[string]string{"format": "json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output == "" {
		t.Error("expected non-empty JSON output")
	}
}

func TestExecuteNode_AllowList(t *testing.T) {
	oldSafeMode := IsSafeMode()
	oldAllowList := allowListEnabled
	defer func() {
		SetSafeMode(oldSafeMode)
		allowListEnabled = oldAllowList
	}()
	SetSafeMode(false)
	allowListEnabled = true

	ctx := context.Background()
	node := &ExecuteNode{}

	// Disallowed command
	_, err := node.Execute(ctx, "", map[string]string{"command": "malicious"})
	if err == nil {
		t.Error("expected error for disallowed command")
	}

	// Allowed command with metacharacters
	_, err = node.Execute(ctx, "", map[string]string{"command": "echo hello; rm -rf /"})
	if err == nil {
		t.Error("expected error for shell metacharacters")
	}

	// Allowed command without metacharacters
	output, err := node.Execute(ctx, "", map[string]string{"command": "echo hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "hello") {
		t.Errorf("expected hello, got %q", output)
	}
}

func TestNotifyNode_Webhook(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	// Webhook with invalid URL
	_, err := node.Execute(ctx, "test", map[string]string{"channel": "webhook", "url": "not-a-url"})
	if err == nil {
		t.Error("expected error for invalid webhook URL")
	}
}

func TestNotifyNode_MailMissingEnv(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	// Mail without env vars should fail
	_, err := node.Execute(ctx, "test", map[string]string{"channel": "mail", "to": "test@example.com"})
	if err == nil {
		t.Error("expected error for mail without SMTP config")
	}
}

func TestValidateReadPath_Symlink(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	os.WriteFile(target, []byte("x"), 0644)
	link := filepath.Join(tmpDir, "link.txt")
	os.Symlink(target, link)

	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	got, err := validateReadPath("link.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Error("expected non-empty path")
	}
}

func TestFileReadNode_NotFound(t *testing.T) {
	oldWorkDir := workDir
	workDir = t.TempDir()
	defer func() { workDir = oldWorkDir }()

	ctx := context.Background()
	node := &FileReadNode{}
	_, err := node.Execute(ctx, "", map[string]string{"path": "nonexistent.txt"})
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestValidateLMLEndpoint_PublicIP(t *testing.T) {
	if err := validateLMLEndpoint("http://8.8.8.8:11434"); err != nil {
		t.Errorf("unexpected error for public IP: %v", err)
	}
}

func TestValidateLMLEndpoint_InvalidScheme(t *testing.T) {
	if err := validateLMLEndpoint("ftp://example.com"); err == nil {
		t.Error("expected error for invalid scheme")
	}
}

func TestValidateLMLEndpoint_InvalidURL(t *testing.T) {
	if err := validateLMLEndpoint("://bad-url"); err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestValidateLMLEndpoint_NonStandardPort(t *testing.T) {
	if err := validateLMLEndpoint("http://localhost:8080"); err != nil {
		t.Errorf("unexpected error for localhost with port: %v", err)
	}
}

func TestOllamaNode_Metadata(t *testing.T) {
	node := &OllamaNode{}
	if node.Name() != "ollama" {
		t.Errorf("expected ollama, got %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("expected non-empty description")
	}
	schema := node.Schema()
	if schema.Name != "ollama" {
		t.Errorf("expected schema name ollama, got %s", schema.Name)
	}
}

func TestOllamaNode_Execute(t *testing.T) {
	// OllamaNode.Execute requires an Ollama server; just test error path
	ctx := context.Background()
	node := &OllamaNode{}
	_, err := node.Execute(ctx, "hello", map[string]string{})
	if err == nil {
		t.Error("expected error when no Ollama server is available")
	}
}

func TestFetchURLNode_Execute(t *testing.T) {
	ctx := context.Background()
	node := &FetchURLNode{}
	// Empty input should error
	_, err := node.Execute(ctx, "", map[string]string{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestHTTPRequestNode_Execute(t *testing.T) {
	ctx := context.Background()
	node := &HTTPRequestNode{}
	// Missing URL should error
	_, err := node.Execute(ctx, "", map[string]string{})
	if err == nil {
		t.Error("expected error for missing URL")
	}
}

func TestOpenAINode_Metadata(t *testing.T) {
	node := &OpenAINode{}
	if node.Name() != "openai" {
		t.Errorf("expected openai, got %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("expected non-empty description")
	}
	schema := node.Schema()
	if schema.Name != "openai" {
		t.Errorf("expected schema name openai, got %s", schema.Name)
	}
}

func TestCozeNode_Metadata(t *testing.T) {
	node := &CozeNode{}
	if node.Name() != "coze" {
		t.Errorf("expected coze, got %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestFastGPTNode_Metadata(t *testing.T) {
	node := &FastGPTNode{}
	if node.Name() != "fastgpt" {
		t.Errorf("expected fastgpt, got %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestIMANode_Metadata(t *testing.T) {
	node := &IMANode{}
	if node.Name() != "ima" {
		t.Errorf("expected ima, got %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestSecurity_ValidateLMLEndpointIP_Multicast(t *testing.T) {
	ip := net.ParseIP("224.0.0.1")
	if err := validateLMLEndpointIP(ip, "224.0.0.1"); err == nil {
		t.Error("expected error for multicast IP")
	}
}

func TestSecurity_ValidateLMLEndpointIP_LinkLocal(t *testing.T) {
	ip := net.ParseIP("169.254.1.1")
	if err := validateLMLEndpointIP(ip, "169.254.1.1"); err == nil {
		t.Error("expected error for link-local IP")
	}
}

func TestSecurity_ValidateLMLEndpointIP_Loopback(t *testing.T) {
	ip := net.ParseIP("127.0.0.1")
	if err := validateLMLEndpointIP(ip, "127.0.0.1"); err != nil {
		t.Errorf("unexpected error for loopback: %v", err)
	}
}

func TestSecurity_ValidateIP_Private(t *testing.T) {
	ip := net.ParseIP("192.168.1.1")
	if err := validateIP(ip, "192.168.1.1"); err == nil {
		t.Error("expected error for private IP")
	}
}

func TestSecurity_ValidateIP_Loopback(t *testing.T) {
	ip := net.ParseIP("127.0.0.1")
	if err := validateIP(ip, "127.0.0.1"); err == nil {
		t.Error("expected error for loopback IP")
	}
}

func TestSecurity_ValidateIP_Unspecified(t *testing.T) {
	ip := net.ParseIP("0.0.0.0")
	if err := validateIP(ip, "0.0.0.0"); err == nil {
		t.Error("expected error for unspecified IP")
	}
}

func TestSecurity_ValidateIP_Multicast(t *testing.T) {
	ip := net.ParseIP("224.0.0.1")
	if err := validateIP(ip, "224.0.0.1"); err == nil {
		t.Error("expected error for multicast IP")
	}
}

func TestSecurity_ValidateLMLEndpointIP_Loopback_Allowed(t *testing.T) {
	ip := net.ParseIP("127.0.0.1")
	if err := validateLMLEndpointIP(ip, "127.0.0.1"); err != nil {
		t.Errorf("unexpected error for loopback LML endpoint: %v", err)
	}
}

func TestAtomicWriteFile_RenameFail(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a directory with the target name so rename fails
	os.MkdirAll(filepath.Join(tmpDir, "target.txt"), 0755)
	err := atomicWriteFile(filepath.Join(tmpDir, "target.txt"), []byte("data"), 0644)
	if err == nil {
		t.Error("expected error when target is a directory")
	}
}

func TestNotifyNode_WebhookValidURL(t *testing.T) {
	ctx := context.Background()
	node := &NotifyNode{}
	// Use a valid-looking URL to unreachable host
	_, err := node.Execute(ctx, "test", map[string]string{"channel": "webhook", "url": "https://nonexistent-host-12345.example.com/webhook"})
	if err == nil {
		t.Error("expected error for unreachable webhook")
	}
}

func TestExternalNode_Execute_ScriptFail(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "fail.sh")
	os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 1"), 0755)

	node := NewExternalNode(NodeMetadata{
		Name: "ext",
	}, scriptPath)
	ctx := context.Background()
	_, err := node.Execute(ctx, "input", map[string]string{})
	if err == nil {
		t.Error("expected error when script exits with non-zero")
	}
}

func TestJSONParseNode_FloatValue(t *testing.T) {
	ctx := context.Background()
	node := &JSONParseNode{}
	output, err := node.Execute(ctx, `{"value":3.14}`, map[string]string{"path": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "3.14" {
		t.Errorf("expected 3.14, got %q", output)
	}
}

func TestJSONParseNode_BoolValue(t *testing.T) {
	ctx := context.Background()
	node := &JSONParseNode{}
	output, err := node.Execute(ctx, `{"ok":true}`, map[string]string{"path": "ok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "true" {
		t.Errorf("expected true, got %q", output)
	}
}

func TestJSONParseNode_NullValue(t *testing.T) {
	ctx := context.Background()
	node := &JSONParseNode{}
	output, err := node.Execute(ctx, `{"x":null}`, map[string]string{"path": "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "null" {
		t.Errorf("expected null, got %q", output)
	}
}

func TestJSONParseNode_EmptyPath(t *testing.T) {
	ctx := context.Background()
	node := &JSONParseNode{}
	output, err := node.Execute(ctx, `{"a":1}`, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "a") {
		t.Errorf("expected pretty JSON, got %q", output)
	}
}

func TestTransformNode_GroupByCommitType_Fix(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	input := "fix: bug1\nfeat: feature1\ndocs: doc1"
	output, err := node.Execute(ctx, input, map[string]string{"operation": "group_by_commit_type"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "fix") || !strings.Contains(output, "feat") {
		t.Errorf("expected grouped output, got %q", output)
	}
}

func TestTransformNode_GroupByExtension_NoExt(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	input := "README\nMakefile"
	output, err := node.Execute(ctx, input, map[string]string{"operation": "group_by_extension"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestTransformNode_CountByLabel_NoMatch(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	input := "no labels here"
	output, err := node.Execute(ctx, input, map[string]string{"operation": "count_by_label"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Label count summary") {
		t.Errorf("expected summary header, got %q", output)
	}
}

func TestTransformNode_ExtractReposAndActivity_NoRepos(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	input := "just some text without repos"
	output, err := node.Execute(ctx, input, map[string]string{"operation": "extract_repos_and_activity"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "No repositories found") {
		t.Errorf("expected no repos message, got %q", output)
	}
}

func TestTransformNode_CombineAndSummarize_Short(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	input := "short text"
	output, err := node.Execute(ctx, input, map[string]string{"operation": "combine_and_summarize"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestTransformNode_ExtractFunctionsAndTypes_NoMatch(t *testing.T) {
	ctx := context.Background()
	node := &TransformNode{}
	input := "just text"
	output, err := node.Execute(ctx, input, map[string]string{"operation": "extract_functions_and_types"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestCallNode_ExecuteWithInputAndTimeout(t *testing.T) {
	oldFunc := ExecuteWorkflowFunc
	ExecuteWorkflowFunc = func(ctx context.Context, wf interface{}, reg *Registry) (string, []interface{}, error) {
		return "ok", nil, nil
	}
	defer func() { ExecuteWorkflowFunc = oldFunc }()

	ctx := context.Background()
	node := &CallNode{}

	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	wfPath := filepath.Join(tmpDir, "test.yaml")
	os.WriteFile(wfPath, []byte("steps:\n"), 0644)

	output, err := node.Execute(ctx, "myinput", map[string]string{"workflow": "test.yaml", "timeout": "5s"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "ok" {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestFileWriteNode_ValidatePath_Dotfile(t *testing.T) {
	ctx := context.Background()
	node := &FileWriteNode{}

	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	_, err := node.Execute(ctx, "x", map[string]string{"path": ".hidden", "mode": "write"})
	if err == nil {
		t.Error("expected error for dotfile")
	}
}

func TestFileWriteNode_ValidatePath_DisallowedExt(t *testing.T) {
	ctx := context.Background()
	node := &FileWriteNode{}

	tmpDir := t.TempDir()
	oldWorkDir := workDir
	workDir = tmpDir
	defer func() { workDir = oldWorkDir }()

	_, err := node.Execute(ctx, "x", map[string]string{"path": "data.exe", "mode": "write"})
	if err == nil {
		t.Error("expected error for disallowed extension")
	}
}

func TestValidateLMLEndpoint_DNSFailure(t *testing.T) {
	// Use a non-existent hostname
	if err := validateLMLEndpoint("http://this-host-does-not-exist-12345.local"); err == nil {
		t.Error("expected error for unresolvable hostname")
	}
}

func TestValidateLMLEndpoint_ResolvedPrivateIP(t *testing.T) {
	// localhost resolves to 127.0.0.1 which is allowed for LML
	if err := validateLMLEndpoint("http://localhost:11434"); err != nil {
		t.Errorf("unexpected error for localhost LML: %v", err)
	}
}

func TestConditionNode_RegexMatch(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}
	output, err := node.Execute(ctx, "hello123", map[string]string{"expr": "regex:^[a-z]+[0-9]+$"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "true" {
		t.Errorf("expected true, got %s", output)
	}
}

func TestConditionNode_NotRegexMatch(t *testing.T) {
	ctx := context.Background()
	node := &ConditionNode{}
	output, err := node.Execute(ctx, "hello", map[string]string{"expr": "not regex:^[0-9]+$"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "true" {
		t.Errorf("expected true, got %s", output)
	}
}

func TestLLMBaseNode_Metadata(t *testing.T) {
	node := &OpenAICompatibleNode{}
	if node.Description() == "" {
		t.Error("expected non-empty description")
	}
	schema := node.Schema()
	if schema.Description == "" {
		t.Error("expected non-empty schema description")
	}
}

func TestLLMBaseNode_Execute(t *testing.T) {
	node := &OpenAICompatibleNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "hello", map[string]string{})
	if err == nil {
		t.Error("expected error when no API key is configured")
	}
}

func TestRedactSensitive_URL(t *testing.T) {
	input := "https://user:password@example.com/path"
	output := RedactSensitive(input)
	if strings.Contains(output, "password") {
		t.Error("expected password to be redacted")
	}
}

func TestRedactSensitive_GitHubToken(t *testing.T) {
	input := "ghp_1234567890abcdefghijklmnopqrstuvwxyz"
	output := RedactSensitive(input)
	if strings.Contains(output, "ghijklmnopqrstuvwxyz") {
		t.Error("expected token suffix to be redacted")
	}
}

func TestRedactSensitive_OpenAIKey(t *testing.T) {
	input := "sk-1234567890abcdefghijklmnopqrstuvwxyz"
	output := RedactSensitive(input)
	if strings.Contains(output, "ghijklmnopqrstuvwxyz") {
		t.Error("expected key suffix to be redacted")
	}
}

func TestRedactSensitive_SlackToken(t *testing.T) {
	input := "xoxb-1234567890-abcdefghij"
	output := RedactSensitive(input)
	if strings.Contains(output, "abcdefghij") {
		t.Error("expected token suffix to be redacted")
	}
}

func TestRedactSensitive_NoMatch(t *testing.T) {
	input := "hello world"
	output := RedactSensitive(input)
	if output != input {
		t.Errorf("expected no change, got %q", output)
	}
}

func TestDefaultEndpointFor(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"ollama", "http://localhost:11434"},
		{"openai", "https://api.openai.com/v1"},
		{"deepseek", "https://api.deepseek.com/v1"},
		{"glm", "https://open.bigmodel.cn/api/paas/v4"},
		{"kimi", "https://api.moonshot.cn/v1"},
		{"qwen", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
		{"mistral", "https://api.mistral.ai/v1"},
		{"yi", "https://api.lingyiwanwu.com/v1"},
		{"anthropic", "https://api.anthropic.com/v1"},
		{"gemini", "https://generativelanguage.googleapis.com/v1beta/openai"},
		{"unknown", "http://localhost:11434"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := defaultEndpointFor(tt.provider)
			if got != tt.want {
				t.Errorf("defaultEndpointFor(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestParseToolsList(t *testing.T) {
	tests := []struct {
		input    string
		wantLen  int
		wantName string
	}{
		{"fetch_url,json_parse", 2, "fetch_url"},
		{"ollama", 1, "ollama_llm"},
		{"invalid_tool", 2, "fetch_url"},
		{"", 2, "fetch_url"},
		{"  fetch_url , json_parse  ", 2, "fetch_url"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tools := parseToolsList(tt.input)
			if len(tools) != tt.wantLen {
				t.Errorf("parseToolsList(%q) len = %d, want %d", tt.input, len(tools), tt.wantLen)
			}
			if len(tools) > 0 && tools[0].Name != tt.wantName {
				t.Errorf("parseToolsList(%q)[0].Name = %q, want %q", tt.input, tools[0].Name, tt.wantName)
			}
		})
	}
}

func TestParamInt(t *testing.T) {
	params := map[string]string{
		"valid":   "42",
		"low":     "1",
		"high":    "200",
		"invalid": "abc",
	}

	tests := []struct {
		key        string
		defaultVal int
		min        int
		max        int
		want       int
	}{
		{"valid", 10, 0, 100, 42},
		{"low", 10, 10, 100, 10},
		{"high", 10, 0, 100, 100},
		{"invalid", 10, 0, 100, 10},
		{"missing", 50, 0, 100, 50},
		{"valid", 10, 100, 0, 42},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := paramInt(params, tt.key, tt.defaultVal, tt.min, tt.max)
			if got != tt.want {
				t.Errorf("paramInt(%q, %d, %d, %d) = %d, want %d",
					tt.key, tt.defaultVal, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestParamFloat(t *testing.T) {
	params := map[string]string{
		"valid":   "3.14",
		"low":     "0.5",
		"high":    "99.9",
		"invalid": "abc",
	}

	tests := []struct {
		key        string
		defaultVal float64
		min        float64
		max        float64
		want       float64
	}{
		{"valid", 1.0, 0, 100, 3.14},
		{"low", 1.0, 1.0, 100, 1.0},
		{"high", 1.0, 0, 50, 50},
		{"invalid", 2.5, 0, 100, 2.5},
		{"missing", 5.0, 0, 100, 5.0},
		{"valid", 1.0, 100, 0, 3.14},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := paramFloat(params, tt.key, tt.defaultVal, tt.min, tt.max)
			if got != tt.want {
				t.Errorf("paramFloat(%q, %f, %f, %f) = %f, want %f",
					tt.key, tt.defaultVal, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 10, ""},
		{"a", 0, "..."},
		{"abcdef", 3, "abc..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestBuildToolDescriptions(t *testing.T) {
	tools := []AgentTool{
		{Name: "fetch", Description: "Fetch a URL"},
		{Name: "parse", Description: "Parse JSON"},
	}

	a := &ReActAgent{tools: tools}
	result := a.buildToolDescriptions()

	expected := "- fetch: Fetch a URL\n- parse: Parse JSON"
	if result != expected {
		t.Errorf("buildToolDescriptions() = %q, want %q", result, expected)
	}
}

func TestBuildToolDescriptions_Empty(t *testing.T) {
	a := &ReActAgent{tools: []AgentTool{}}
	result := a.buildToolDescriptions()
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	a := &ReActAgent{enableThinking: false}
	prompt := a.buildSystemPrompt("- tool1: desc")

	if !strings.Contains(prompt, "tool1: desc") {
		t.Error("expected prompt to contain tool descriptions")
	}
	if strings.Contains(prompt, "Thinking mode is ENABLED") {
		t.Error("expected no thinking instruction when disabled")
	}
	if !strings.Contains(prompt, "ReAct (Reason + Act)") {
		t.Error("expected prompt to contain ReAct pattern")
	}
}

func TestBuildSystemPrompt_WithThinking(t *testing.T) {
	a := &ReActAgent{enableThinking: true}
	prompt := a.buildSystemPrompt("- tool1: desc")

	if !strings.Contains(prompt, "Thinking mode is ENABLED") {
		t.Error("expected thinking instruction when enabled")
	}
}

func TestBuildSystemPrompt_WithCustom(t *testing.T) {
	a := &ReActAgent{systemPrompt: "Custom system prompt", enableThinking: false}
	prompt := a.buildSystemPrompt("- tool1: desc")

	if !strings.HasPrefix(prompt, "Custom system prompt") {
		t.Error("expected prompt to start with custom system prompt")
	}
}

func TestParseReActResponse_ValidJSON(t *testing.T) {
	response := `{"thought": "let me fetch", "action": "fetch", "action_input": "http://example.com"}`
	thought, err := parseReActResponse(response)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if thought.Thought != "let me fetch" {
		t.Errorf("expected thought 'let me fetch', got %q", thought.Thought)
	}
	if thought.Action != "fetch" {
		t.Errorf("expected action 'fetch', got %q", thought.Action)
	}
	if thought.ActionInput != "http://example.com" {
		t.Errorf("expected action_input 'http://example.com', got %q", thought.ActionInput)
	}
}

func TestParseReActResponse_WithFinalAnswer(t *testing.T) {
	response := `{"action": "final_answer", "final_answer": "42 is the answer"}`
	thought, err := parseReActResponse(response)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if thought.FinalAnswer != "42 is the answer" {
		t.Errorf("expected final_answer '42 is the answer', got %q", thought.FinalAnswer)
	}
}

func TestParseReActResponse_WithCodeFences(t *testing.T) {
	response := "```json\n{\"action\": \"final_answer\", \"final_answer\": \"hello\"}\n```"
	thought, err := parseReActResponse(response)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if thought.FinalAnswer != "hello" {
		t.Errorf("expected final_answer 'hello', got %q", thought.FinalAnswer)
	}
}

func TestParseReActResponse_ExtractJSON(t *testing.T) {
	response := "Some text here\n{\"action\": \"final_answer\", \"final_answer\": \"extracted\"}\nMore text"
	thought, err := parseReActResponse(response)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if thought.FinalAnswer != "extracted" {
		t.Errorf("expected final_answer 'extracted', got %q", thought.FinalAnswer)
	}
}

func TestParseReActResponse_InvalidJSON(t *testing.T) {
	_, err := parseReActResponse("not json at all")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseReActResponse_MissingAction(t *testing.T) {
	_, err := parseReActResponse(`{"thought": "no action here"}`)
	if err == nil {
		t.Error("expected error when neither action nor final_answer present")
	}
}

func TestToolNames(t *testing.T) {
	a := &ReActAgent{tools: []AgentTool{
		{Name: "fetch"},
		{Name: "parse"},
		{Name: "summarize"},
	}}
	result := a.toolNames()
	if result != "fetch, parse, summarize" {
		t.Errorf("expected 'fetch, parse, summarize', got %q", result)
	}
}

func TestBuildConversationPrompt(t *testing.T) {
	messages := []LLMMessage{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}
	result := buildConversationPrompt(messages)

	expected := "system: You are helpful\n\nuser: Hello\n\nassistant: Hi there"
	if result != expected {
		t.Errorf("buildConversationPrompt() = %q, want %q", result, expected)
	}
}

func TestNewReActAgent_DefaultMaxIters(t *testing.T) {
	agent := NewReActAgent("ollama", "llama3", "", "", "", 0, nil, nil, false, false)
	if agent.maxIters != defaultMaxAgentIterations {
		t.Errorf("expected default maxIters %d, got %d", defaultMaxAgentIterations, agent.maxIters)
	}
}

func TestNewReActAgent_CustomMaxIters(t *testing.T) {
	agent := NewReActAgent("ollama", "llama3", "", "", "", 5, nil, nil, false, false)
	if agent.maxIters != 5 {
		t.Errorf("expected maxIters 5, got %d", agent.maxIters)
	}
}

func TestNewReActAgent_AllFields(t *testing.T) {
	tools := []AgentTool{{Name: "t1", Description: "d1", NodeName: "n1"}}
	reg := NewRegistry()
	agent := NewReActAgent("openai", "gpt-4", "key1", "https://api.example.com", "sys", 3, tools, reg, true, true)

	if agent.provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", agent.provider)
	}
	if agent.model != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got %q", agent.model)
	}
	if agent.apiKey != "key1" {
		t.Errorf("expected apiKey 'key1', got %q", agent.apiKey)
	}
	if agent.endpoint != "https://api.example.com" {
		t.Errorf("expected endpoint 'https://api.example.com', got %q", agent.endpoint)
	}
	if agent.systemPrompt != "sys" {
		t.Errorf("expected systemPrompt 'sys', got %q", agent.systemPrompt)
	}
	if agent.maxIters != 3 {
		t.Errorf("expected maxIters 3, got %d", agent.maxIters)
	}
	if len(agent.tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(agent.tools))
	}
	if agent.registry != reg {
		t.Error("expected registry to be set")
	}
	if !agent.enableThinking {
		t.Error("expected enableThinking true")
	}
	if !agent.showThinking {
		t.Error("expected showThinking true")
	}
}

func TestRegexpFindAllStringSubmatch(t *testing.T) {
	matches := regexpFindAllStringSubmatch(`(\w+)=(\d+)`, "a=1 b=2 c=3", -1)
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}
	if matches[0][1] != "a" || matches[0][2] != "1" {
		t.Errorf("expected first match [a, 1], got %v", matches[0])
	}
}

func TestRegexpFindAllStringSubmatch_Limit(t *testing.T) {
	matches := regexpFindAllStringSubmatch(`\d+`, "1 2 3 4 5", 2)
	if len(matches) != 2 {
		t.Errorf("expected 2 matches with limit 2, got %d", len(matches))
	}
}

func TestRegexpFindAllStringSubmatch_InvalidPattern(t *testing.T) {
	matches := regexpFindAllStringSubmatch(`[invalid`, "test", -1)
	if matches != nil {
		t.Errorf("expected nil for invalid pattern, got %v", matches)
	}
}

func TestRegexpFindAllStringSubmatch_NoMatch(t *testing.T) {
	matches := regexpFindAllStringSubmatch(`\d+`, "no digits here", -1)
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestExtractLinksFromText_MarkdownLinks(t *testing.T) {
	text := "Check [Google](https://google.com) and [GitHub](https://github.com)"
	links := extractLinksFromText(text, "")

	if len(links) < 2 {
		t.Fatalf("expected at least 2 links, got %d", len(links))
	}
	if links[0].Title != "Google" || links[0].URL != "https://google.com" {
		t.Errorf("unexpected first link: %+v", links[0])
	}
}

func TestExtractLinksFromText_RawURLs(t *testing.T) {
	text := "Visit https://example.com for more info"
	links := extractLinksFromText(text, "")

	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].URL != "https://example.com" {
		t.Errorf("expected URL https://example.com, got %q", links[0].URL)
	}
}

func TestExtractLinksFromText_Deduplication(t *testing.T) {
	text := "Visit https://same.com twice https://same.com"
	links := extractLinksFromText(text, "")

	if len(links) != 1 {
		t.Errorf("expected 1 deduplicated link, got %d", len(links))
	}
}

func TestExtractLinksFromText_LongURLTitle(t *testing.T) {
	longURL := "https://example.com/" + strings.Repeat("a", 100)
	text := "Check " + longURL
	links := extractLinksFromText(text, "")

	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if len(links[0].Title) > 83 {
		t.Errorf("expected title truncated to 80+3 chars, got %d", len(links[0].Title))
	}
	if !strings.HasSuffix(links[0].Title, "...") {
		t.Errorf("expected title to end with '...', got %q", links[0].Title)
	}
}

func TestExtractLinksFromText_NoLinks(t *testing.T) {
	text := "Just plain text without any links"
	links := extractLinksFromText(text, "")
	if len(links) != 0 {
		t.Errorf("expected 0 links, got %d", len(links))
	}
}

func TestBaseAgentParams(t *testing.T) {
	params := baseAgentParams()

	if len(params) != 4 {
		t.Fatalf("expected 4 params, got %d", len(params))
	}

	names := []string{"provider", "model", "api_key", "endpoint"}
	for i, name := range names {
		if params[i].Name != name {
			t.Errorf("expected param[%d].Name = %q, got %q", i, name, params[i].Name)
		}
	}

	if params[0].Default != "ollama" {
		t.Errorf("expected provider default 'ollama', got %q", params[0].Default)
	}
	if params[1].Default != "llama3" {
		t.Errorf("expected model default 'llama3', got %q", params[1].Default)
	}
}
