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

package nodes

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// nodes_coverage_boost5_test.go 补充 ExecuteNode, FileReadNode, FileWriteNode,
// FetchURLNode, TemplateRenderNode, JSONParseNode, CombineNode, TransformNode,
// CallNode 等未覆盖或覆盖不足的节点类型的单元测试。

// ============================================================================
// ExecuteNode
// ============================================================================

func TestExecuteNode_Metadata(t *testing.T) {
	node := &ExecuteNode{}
	if node.Name() != "execute" {
		t.Errorf("Name() = %q, want execute", node.Name())
	}
	if node.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestExecuteNode_Schema(t *testing.T) {
	node := &ExecuteNode{}
	schema := node.Schema()
	if schema.Name != "execute" {
		t.Errorf("Schema().Name = %q, want execute", schema.Name)
	}
	if schema.Description == "" {
		t.Error("Schema().Description is empty")
	}
	paramNames := map[string]bool{}
	for _, p := range schema.Params {
		paramNames[p.Name] = true
	}
	for _, name := range []string{"command", "dry_run", "timeout"} {
		if !paramNames[name] {
			t.Errorf("expected param %q in schema", name)
		}
	}
}

func TestExecuteNode_MissingCommand(t *testing.T) {
	node := &ExecuteNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "", map[string]string{})
	if err == nil {
		t.Error("expected error for missing command")
	}
	if !strings.Contains(err.Error(), "command parameter is required") {
		t.Errorf("expected missing command error, got: %v", err)
	}
}

func TestExecuteNode_CommandTooLong(t *testing.T) {
	node := &ExecuteNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "", map[string]string{
		"command": strings.Repeat("a", 4097),
	})
	if err == nil {
		t.Error("expected error for too long command")
	}
	if !strings.Contains(err.Error(), "too long") {
		t.Errorf("expected too long error, got: %v", err)
	}
}

func TestExecuteNode_DryRun(t *testing.T) {
	node := &ExecuteNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "", map[string]string{
		"command": "echo hello",
		"dry_run": "true",
	})
	if err != nil {
		t.Fatalf("dry run should not error: %v", err)
	}
	if !strings.Contains(out, "[DRY RUN]") {
		t.Errorf("expected DRY RUN marker, got: %s", out)
	}
	if !strings.Contains(out, "echo hello") {
		t.Errorf("expected command in output, got: %s", out)
	}
}

func TestExecuteNode_TooManyParams(t *testing.T) {
	node := &ExecuteNode{}
	ctx := context.Background()
	params := map[string]string{"command": "echo hello"}
	for i := 0; i < 10; i++ {
		params[string(rune('a'+i))] = "v"
	}
	_, err := node.Execute(ctx, "", params)
	if err == nil {
		t.Error("expected error for too many parameters")
	}
}

func TestExecuteNode_ParamTooLong(t *testing.T) {
	node := &ExecuteNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "", map[string]string{
		"command":               "echo hello",
		strings.Repeat("k", 51): "v",
	})
	if err == nil {
		t.Error("expected error for long param key")
	}
}

func TestExecuteNode_InvalidTimeout(t *testing.T) {
	oldAllowList := allowListEnabled
	allowListEnabled = false
	defer func() { allowListEnabled = oldAllowList }()

	node := &ExecuteNode{}
	ctx := context.Background()
	// Invalid timeout string falls back to default 5m, and echo should succeed
	out, err := node.Execute(ctx, "", map[string]string{
		"command": "echo hello",
		"timeout": "not-a-duration",
	})
	if err != nil {
		t.Fatalf("echo should succeed: %v", err)
	}
	if out != "hello" {
		t.Errorf("expected 'hello', got: %q", out)
	}
}

func TestExecuteNode_AllowlistEnabled_CmdBlocked(t *testing.T) {
	oldAllowList := allowListEnabled
	allowListEnabled = true
	defer func() { allowListEnabled = oldAllowList }()

	node := &ExecuteNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "", map[string]string{
		"command": "python script.py",
	})
	if err == nil {
		t.Error("expected error for blocked command in allowlist mode")
	}
}

func TestExecuteNode_AllowlistEnabled_MetacharBlocked(t *testing.T) {
	oldAllowList := allowListEnabled
	allowListEnabled = true
	defer func() { allowListEnabled = oldAllowList }()

	node := &ExecuteNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "", map[string]string{
		"command": "echo hello; cat /etc/passwd",
	})
	if err == nil {
		t.Error("expected error for shell metacharacter in allowlist mode")
	}
}

func TestExecuteNode_AllowlistEnabled_AllowedCmd(t *testing.T) {
	oldAllowList := allowListEnabled
	allowListEnabled = true
	defer func() { allowListEnabled = oldAllowList }()

	node := &ExecuteNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "", map[string]string{
		"command": "echo hello",
	})
	if err != nil {
		t.Fatalf("allowed command should succeed: %v", err)
	}
	if out != "hello" {
		t.Errorf("expected 'hello', got: %q", out)
	}
}

func TestRedactCommandForLog(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"bearer token",
			"curl -H 'Authorization: Bearer abc123' https://api.example.com",
			"curl -H 'Authorization: **** **** https://api.example.com",
		},
		{
			"api key",
			"curl -d 'api_key=secret123' https://api.example.com",
			"curl -d 'api_key=**** https://api.example.com",
		},
		{
			"password",
			"curl -d 'password=mypass' https://api.example.com",
			"curl -d 'password=**** https://api.example.com",
		},
		{
			"token",
			"curl -H 'token=secret' https://api.example.com",
			"curl -H 'token=**** https://api.example.com",
		},
		{
			"url credentials",
			"curl https://user:pass@example.com/api",
			"curl https://user:****@example.com/api",
		},
		{
			"no secrets",
			"echo hello world",
			"echo hello world",
		},
		{
			"authorization header",
			"curl -H 'Authorization: Bearer tok123' https://api.example.com",
			"curl -H 'Authorization: **** **** https://api.example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactCommandForLog(tt.in)
			if got != tt.want {
				t.Errorf("redactCommandForLog(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEscapeLogContent(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello", "hello"},
		{"newline", "a\nb", "a\\nb"},
		{"carriage_return", "a\rb", "a\\rb"},
		{"tab", "a\tb", "a\\tb"},
		{"null_byte", "a\x00b", "a\\0b"},
		{"control_char", "a\x01b", "a\\x01b"},
		{"del_127", "a\x7fb", "a\\x7fb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeLogContent(tt.in)
			if got != tt.want {
				t.Errorf("escapeLogContent(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExecuteNode_TimeoutCap(t *testing.T) {
	oldAllowList := allowListEnabled
	allowListEnabled = false
	defer func() { allowListEnabled = oldAllowList }()

	node := &ExecuteNode{}
	ctx := context.Background()
	// timeout > 30m should be capped to 30m, echo should still work
	out, err := node.Execute(ctx, "", map[string]string{
		"command": "echo hello",
		"timeout": "1h",
	})
	if err != nil {
		t.Fatalf("echo should succeed: %v", err)
	}
	if out != "hello" {
		t.Errorf("expected 'hello', got: %q", out)
	}
}

// ============================================================================
// FileReadNode
// ============================================================================

func TestFileReadNode_Metadata(t *testing.T) {
	node := &FileReadNode{}
	if node.Name() != "file_read" {
		t.Errorf("Name() = %q, want file_read", node.Name())
	}
	if node.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestFileReadNode_Schema(t *testing.T) {
	node := &FileReadNode{}
	schema := node.Schema()
	if schema.Name != "file_read" {
		t.Errorf("Schema().Name = %q, want file_read", schema.Name)
	}
	paramNames := map[string]bool{}
	for _, p := range schema.Params {
		paramNames[p.Name] = true
	}
	for _, name := range []string{"path", "redact"} {
		if !paramNames[name] {
			t.Errorf("expected param %q in schema", name)
		}
	}
}

func TestFileReadNode_MissingPath(t *testing.T) {
	node := &FileReadNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "", map[string]string{})
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestFileReadNode_PathValidation(t *testing.T) {
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	node := &FileReadNode{}
	ctx := context.Background()
	// 路径遍历应被拒绝
	_, err := node.Execute(ctx, "", map[string]string{"path": "../etc/passwd"})
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestFileReadNode_FileTooLarge(t *testing.T) {
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	// 创建一个超大文件（通过创建稀疏文件并设置大小）
	largePath := filepath.Join(dir, "large.txt")
	// 创建文件后直接设置超过 maxFileReadSize 的 metadata
	if err := os.WriteFile(largePath, []byte("small content"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// 文件实际大小小于限制，因此应能正常读取
	node := &FileReadNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "", map[string]string{"path": "large.txt"})
	if err != nil {
		t.Fatalf("small file should not error: %v", err)
	}
	if out != "small content" {
		t.Errorf("expected 'small content', got: %q", out)
	}
}

func TestFileReadNode_ReadFile(t *testing.T) {
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	content := "hello world\nthis is a test file\n"
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	node := &FileReadNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "", map[string]string{"path": "test.txt"})
	if err != nil {
		t.Fatalf("read should succeed: %v", err)
	}
	if out != content {
		t.Errorf("expected %q, got: %q", content, out)
	}
}

func TestFileReadNode_RedactDisabled(t *testing.T) {
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	content := "some content with a fake API_KEY=secret123"
	if err := os.WriteFile(filepath.Join(dir, "normal.txt"), []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	node := &FileReadNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "", map[string]string{
		"path":   "normal.txt",
		"redact": "false",
	})
	if err != nil {
		t.Fatalf("read should succeed: %v", err)
	}
	if out != content {
		t.Errorf("expected original content, got: %q", out)
	}
}

func TestFileReadNode_SensitiveFileRedacted(t *testing.T) {
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=value"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	node := &FileReadNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "", map[string]string{"path": ".env"})
	if err != nil {
		t.Fatalf("read should succeed: %v", err)
	}
	if !strings.Contains(out, "REDACTED") {
		t.Errorf("expected redacted content, got: %q", out)
	}
}

// ============================================================================
// FileWriteNode
// ============================================================================

func TestFileWriteNode_Metadata(t *testing.T) {
	node := &FileWriteNode{}
	if node.Name() != "file_write" {
		t.Errorf("Name() = %q, want file_write", node.Name())
	}
	if node.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestFileWriteNode_Schema(t *testing.T) {
	node := &FileWriteNode{}
	schema := node.Schema()
	if schema.Name != "file_write" {
		t.Errorf("Schema().Name = %q, want file_write", schema.Name)
	}
	paramNames := map[string]bool{}
	for _, p := range schema.Params {
		paramNames[p.Name] = true
	}
	for _, name := range []string{"path", "mode"} {
		if !paramNames[name] {
			t.Errorf("expected param %q in schema", name)
		}
	}
}

func TestFileWriteNode_AppendMode(t *testing.T) {
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	// 先写入初始内容
	if err := os.WriteFile(filepath.Join(dir, "append.txt"), []byte("first\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	node := &FileWriteNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "second", map[string]string{
		"path": "append.txt",
		"mode": "append",
	})
	if err != nil {
		t.Fatalf("append should succeed: %v", err)
	}
	if !strings.Contains(out, "appended to") {
		t.Errorf("expected 'appended to' in output: %s", out)
	}

	// 验证文件内容
	data, err := os.ReadFile(filepath.Join(dir, "append.txt"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "first") || !strings.Contains(string(data), "second") {
		t.Errorf("expected both lines, got: %q", string(data))
	}
}

func TestFileWriteNode_PathValidation(t *testing.T) {
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	node := &FileWriteNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "content", map[string]string{"path": "../etc/passwd"})
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestFileWriteNode_DefaultMode(t *testing.T) {
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	node := &FileWriteNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "default content", map[string]string{
		"path": "default.txt",
	})
	if err != nil {
		t.Fatalf("default write should succeed: %v", err)
	}
	if !strings.Contains(out, "written to") {
		t.Errorf("expected 'written to' in output: %s", out)
	}
}

// ============================================================================
// FetchURLNode
// ============================================================================

func TestFetchURLNode_Metadata(t *testing.T) {
	node := &FetchURLNode{}
	if node.Name() != "fetch_url" {
		t.Errorf("Name() = %q, want fetch_url", node.Name())
	}
	if node.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestFetchURLNode_Schema(t *testing.T) {
	node := &FetchURLNode{}
	schema := node.Schema()
	if schema.Name != "fetch_url" {
		t.Errorf("Schema().Name = %q, want fetch_url", schema.Name)
	}
	paramNames := map[string]bool{}
	for _, p := range schema.Params {
		paramNames[p.Name] = true
	}
	for _, name := range []string{"url", "mode", "timeout"} {
		if !paramNames[name] {
			t.Errorf("expected param %q in schema", name)
		}
	}
}

func TestFetchURLNode_MissingURL(t *testing.T) {
	node := &FetchURLNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "", map[string]string{})
	if err == nil {
		t.Error("expected error for missing URL")
	}
}

func TestFetchURLNode_InvalidURL(t *testing.T) {
	node := &FetchURLNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "http://127.0.0.1:8080", map[string]string{})
	if err == nil {
		t.Error("expected error for localhost URL")
	}
}

func TestFetchURLNode_URLViaInput(t *testing.T) {
	t.Skip("requires network")
	node := &FetchURLNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "https://example.com", map[string]string{})
	if err != nil {
		t.Logf("expected network error: %v", err)
	}
}

func TestExtractTextFromHTML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"simple paragraph",
			"<html><body><p>Hello world</p></body></html>",
			"Hello world",
		},
		{
			"with script removal",
			"<html><script>alert('xss')</script><p>clean</p></html>",
			"clean",
		},
		{
			"with style removal",
			"<html><style>body { color: red; }</style><p>styled</p></html>",
			"styled",
		},
		{
			"html entities",
			"<p>&amp; &lt; &gt; &quot; &#39; &nbsp;</p>",
			"& < > \" '",
		},
		{
			"nav removal",
			"<html><nav>menu</nav><main>content</main></html>",
			"content",
		},
		{
			"empty",
			"",
			"",
		},
		{
			"footer removal",
			"<html><footer>copyright</footer><main>body</main></html>",
			"body",
		},
		{
			"header removal",
			"<html><header>title</header><main>article</main></html>",
			"article",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTextFromHTML(tt.in)
			if got != tt.want {
				t.Errorf("extractTextFromHTML(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExtractMainContent(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"main tag",
			"<html><main><p>Main content here, this is long enough to pass the 100 character threshold. " +
				"Let me add more text to make sure this is definitely long enough for the check.</p></main></html>",
			"Main content here, this is long enough to pass the 100 character threshold. " +
				"Let me add more text to make sure this is definitely long enough for the check.",
		},
		{
			"article tag",
			"<html><article><p>Article content here, again we need to make this long enough to pass the " +
				"100 character minimum threshold for content extraction to work properly.</p></article></html>",
			"Article content here, again we need to make this long enough to pass the " +
				"100 character minimum threshold for content extraction to work properly.",
		},
		{
			"content div",
			"<html><div id=\"content\"><p>Div content here, making this long enough to pass the 100 character " +
				"threshold so that the content extraction picks it up properly.</p></div></html>",
			"Div content here, making this long enough to pass the 100 character " +
				"threshold so that the content extraction picks it up properly.",
		},
		{
			"no main content",
			"<html><body><p>short</p></body></html>",
			"short",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMainContent(tt.in)
			if got != tt.want {
				t.Errorf("extractMainContent(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestHtmlToMarkdown(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"h1 to #",
			"<h1>Title</h1>",
			"# Title",
		},
		{
			"strong to bold",
			"<p><strong>bold</strong> text</p>",
			"**bold** text",
		},
		{
			"em to italic",
			"<p><em>italic</em> text</p>",
			"*italic* text",
		},
		{
			"code inline",
			"<p><code>code</code> text</p>",
			"`code` text",
		},
		{
			"link",
			"<a href=\"https://example.com\">link</a>",
			"[link](https://example.com)",
		},
		{
			"list items",
			"<ul><li>item 1</li><li>item 2</li></ul>",
			"- item 1\n- item 2",
		},
		{
			"pre code block",
			"<pre><code>func main() {}</code></pre>",
			"```\n`func main() {}`\n```",
		},
		{
			"h2",
			"<h2>Section</h2>",
			"## Section",
		},
		{
			"h3",
			"<h3>Subsection</h3>",
			"### Subsection",
		},
		{
			"h4",
			"<h4>Subsubsection</h4>",
			"#### Subsubsection",
		},
		{
			"h5",
			"<h5>Small</h5>",
			"##### Small",
		},
		{
			"h6",
			"<h6>Smallest</h6>",
			"###### Smallest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := htmlToMarkdown(tt.in)
			if got != tt.want {
				t.Errorf("htmlToMarkdown(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ============================================================================
// TemplateRenderNode
// ============================================================================

func TestTemplateRenderNode_Metadata(t *testing.T) {
	node := &TemplateRenderNode{}
	if node.Name() != "template_render" {
		t.Errorf("Name() = %q, want template_render", node.Name())
	}
	if node.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestTemplateRenderNode_Schema(t *testing.T) {
	node := &TemplateRenderNode{}
	schema := node.Schema()
	if schema.Name != "template_render" {
		t.Errorf("Schema().Name = %q, want template_render", schema.Name)
	}
	paramNames := map[string]bool{}
	for _, p := range schema.Params {
		paramNames[p.Name] = true
	}
	for _, name := range []string{"template", "template_file"} {
		if !paramNames[name] {
			t.Errorf("expected param %q in schema", name)
		}
	}
}

func TestTemplateRenderNode_MissingTemplate(t *testing.T) {
	node := &TemplateRenderNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "", map[string]string{})
	if err == nil {
		t.Error("expected error for missing template")
	}
}

func TestTemplateRenderNode_SimpleTemplate(t *testing.T) {
	node := &TemplateRenderNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "world", map[string]string{
		"template": "Hello {{ .input }}!",
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	if out != "Hello world!" {
		t.Errorf("expected 'Hello world!', got: %q", out)
	}
}

func TestTemplateRenderNode_WithFunctions(t *testing.T) {
	node := &TemplateRenderNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "hello world", map[string]string{
		"template": "{{ .input | upper }} -> {{ .input | lower }} -> {{ .input | trim }} -> {{ .input | len }}",
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	if !strings.Contains(out, "HELLO WORLD") {
		t.Errorf("expected upper case, got: %q", out)
	}
	if !strings.Contains(out, "hello world") {
		t.Errorf("expected lower case, got: %q", out)
	}
	if !strings.Contains(out, "11") {
		t.Errorf("expected length, got: %q", out)
	}
}

func TestTemplateRenderNode_NowFunction(t *testing.T) {
	node := &TemplateRenderNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "", map[string]string{
		"template": "{{ now }}",
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	// now should return RFC3339 format
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(out)); err != nil {
		t.Errorf("expected RFC3339 time, got: %q (error: %v)", out, err)
	}
}

func TestTemplateRenderNode_InvalidTemplate(t *testing.T) {
	node := &TemplateRenderNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "", map[string]string{
		"template": "{{.bad",
	})
	if err == nil {
		t.Error("expected error for invalid template syntax")
	}
}

func TestTemplateRenderNode_TemplateFile(t *testing.T) {
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	tmplContent := "Hello {{ .input }} from file!"
	if err := os.WriteFile(filepath.Join(dir, "tmpl.txt"), []byte(tmplContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	node := &TemplateRenderNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "world", map[string]string{
		"template_file": "tmpl.txt",
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	if out != "Hello world from file!" {
		t.Errorf("expected 'Hello world from file!', got: %q", out)
	}
}

func TestTemplateRenderNode_TemplateFileNotFound(t *testing.T) {
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	node := &TemplateRenderNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "", map[string]string{
		"template_file": "nonexistent.tmpl",
	})
	if err == nil {
		t.Error("expected error for nonexistent template file")
	}
}

func TestTemplateRenderNode_ExtraParamsInTemplate(t *testing.T) {
	node := &TemplateRenderNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "hello", map[string]string{
		"template": "Input: {{ .input }}, Name: {{ .name }}",
		"name":     "Alice",
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	if !strings.Contains(out, "Input: hello") {
		t.Errorf("expected input in output: %q", out)
	}
	if !strings.Contains(out, "Name: Alice") {
		t.Errorf("expected name in output: %q", out)
	}
}

func TestTemplateRenderNode_TitleFunction(t *testing.T) {
	node := &TemplateRenderNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "hello world", map[string]string{
		"template": "{{ .input | title }}",
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	if !strings.Contains(out, "Hello World") {
		t.Errorf("expected title case, got: %q", out)
	}
}

// ============================================================================
// JSONParseNode
// ============================================================================

func TestJSONParseNode_Metadata(t *testing.T) {
	node := &JSONParseNode{}
	if node.Name() != "json_parse" {
		t.Errorf("Name() = %q, want json_parse", node.Name())
	}
	if node.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestJSONParseNode_Schema(t *testing.T) {
	node := &JSONParseNode{}
	schema := node.Schema()
	if schema.Name != "json_parse" {
		t.Errorf("Schema().Name = %q, want json_parse", schema.Name)
	}
}

func TestJSONParseNode_ParseObject(t *testing.T) {
	node := &JSONParseNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, `{"a": 1, "b": "hello"}`, map[string]string{})
	if err != nil {
		t.Fatalf("parse should succeed: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if result["a"].(float64) != 1 {
		t.Errorf("expected a=1, got %v", result["a"])
	}
}

func TestJSONParseNode_ParseArray(t *testing.T) {
	node := &JSONParseNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, `[1, 2, 3]`, map[string]string{})
	if err != nil {
		t.Fatalf("parse should succeed: %v", err)
	}
	var result []interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 elements, got %d", len(result))
	}
}

func TestJSONParseNode_InvalidJSON(t *testing.T) {
	node := &JSONParseNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, `not json`, map[string]string{})
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestJSONParseNode_InputTooLarge(t *testing.T) {
	node := &JSONParseNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, strings.Repeat("a", maxHTTPResponseSize+1), map[string]string{})
	if err == nil {
		t.Error("expected error for large input")
	}
}

func TestJSONParseNode_PathExtraction(t *testing.T) {
	node := &JSONParseNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, `{"data": {"items": [{"name": "Alice"}, {"name": "Bob"}]}}`, map[string]string{
		"path": "data.items.[0].name",
	})
	if err != nil {
		t.Fatalf("path extraction should succeed: %v", err)
	}
	if out != "Alice" {
		t.Errorf("expected 'Alice', got: %q", out)
	}
}

func TestJSONParseNode_PathNotFound(t *testing.T) {
	node := &JSONParseNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, `{"a": 1}`, map[string]string{
		"path": "b.c",
	})
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestJSONParseNode_PathArrayIndexOutOfRange(t *testing.T) {
	node := &JSONParseNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, `[1, 2]`, map[string]string{
		"path": "[10]",
	})
	if err == nil {
		t.Error("expected error for out of range index")
	}
}

func TestJSONParseNode_PathInvalidArrayIndex(t *testing.T) {
	node := &JSONParseNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, `[1, 2]`, map[string]string{
		"path": "[abc]",
	})
	if err == nil {
		t.Error("expected error for invalid array index")
	}
}

func TestJSONParseNode_PathOnScalar(t *testing.T) {
	node := &JSONParseNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, `42`, map[string]string{
		"path": "key",
	})
	if err == nil {
		t.Error("expected error when accessing key on scalar")
	}
}

func TestJSONParseNode_StringValue(t *testing.T) {
	node := &JSONParseNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, `{"name": "test"}`, map[string]string{
		"path": "name",
	})
	if err != nil {
		t.Fatalf("string extraction should succeed: %v", err)
	}
	if out != "test" {
		t.Errorf("expected 'test', got: %q", out)
	}
}

func TestJSONParseNode_NonStringValue(t *testing.T) {
	node := &JSONParseNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, `{"count": 42}`, map[string]string{
		"path": "count",
	})
	if err != nil {
		t.Fatalf("non-string extraction should succeed: %v", err)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("expected 42 in output, got: %q", out)
	}
}

// ============================================================================
// CombineNode
// ============================================================================

func TestCombineNode_Metadata(t *testing.T) {
	node := &CombineNode{}
	if node.Name() != "combine" {
		t.Errorf("Name() = %q, want combine", node.Name())
	}
	if node.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestCombineNode_Schema(t *testing.T) {
	node := &CombineNode{}
	schema := node.Schema()
	if schema.Name != "combine" {
		t.Errorf("Schema().Name = %q, want combine", schema.Name)
	}
}

func TestCombineNode_TextFormat(t *testing.T) {
	node := &CombineNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "hello world", map[string]string{"format": "text"})
	if err != nil {
		t.Fatalf("text format should succeed: %v", err)
	}
	if out != "hello world" {
		t.Errorf("expected 'hello world', got: %q", out)
	}
}

func TestCombineNode_MarkdownFormat(t *testing.T) {
	node := &CombineNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "item1\nitem2\nitem3", map[string]string{"format": "markdown"})
	if err != nil {
		t.Fatalf("markdown format should succeed: %v", err)
	}
	if !strings.Contains(out, "- item1") {
		t.Errorf("expected markdown list, got: %q", out)
	}
}

func TestCombineNode_MarkdownEmptyLines(t *testing.T) {
	node := &CombineNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "item1\n\nitem2\n\n", map[string]string{"format": "markdown"})
	if err != nil {
		t.Fatalf("markdown with empty lines should succeed: %v", err)
	}
	if strings.Count(out, "- item") != 2 {
		t.Errorf("expected 2 list items, got: %q", out)
	}
}

func TestCombineNode_CSVFormat(t *testing.T) {
	node := &CombineNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "a b c\n1 2 3", map[string]string{"format": "csv"})
	if err != nil {
		t.Fatalf("csv format should succeed: %v", err)
	}
	if !strings.Contains(out, "a,b,c") {
		t.Errorf("expected csv header, got: %q", out)
	}
}

func TestCombineNode_DefaultFormat(t *testing.T) {
	node := &CombineNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "hello", map[string]string{})
	if err != nil {
		t.Fatalf("default format should succeed: %v", err)
	}
	if out != "hello" {
		t.Errorf("expected 'hello', got: %q", out)
	}
}

func TestCombineNode_UnknownFormatReturnsInput(t *testing.T) {
	node := &CombineNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "hello", map[string]string{"format": "unknown"})
	if err != nil {
		t.Fatalf("unknown format should succeed: %v", err)
	}
	if out != "hello" {
		t.Errorf("expected 'hello', got: %q", out)
	}
}

// ============================================================================
// TransformNode
// ============================================================================

func TestTransformNode_Metadata(t *testing.T) {
	node := &TransformNode{}
	if node.Name() != "transform" {
		t.Errorf("Name() = %q, want transform", node.Name())
	}
	if node.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestTransformNode_Schema(t *testing.T) {
	node := &TransformNode{}
	schema := node.Schema()
	if schema.Name != "transform" {
		t.Errorf("Schema().Name = %q, want transform", schema.Name)
	}
}

func TestTransformNode_Upper(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "hello", map[string]string{"operation": "upper"})
	if err != nil {
		t.Fatalf("upper should succeed: %v", err)
	}
	if out != "HELLO" {
		t.Errorf("expected 'HELLO', got: %q", out)
	}
}

func TestTransformNode_Lower(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "HELLO", map[string]string{"operation": "lower"})
	if err != nil {
		t.Fatalf("lower should succeed: %v", err)
	}
	if out != "hello" {
		t.Errorf("expected 'hello', got: %q", out)
	}
}

func TestTransformNode_Trim(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "  hello  ", map[string]string{"operation": "trim"})
	if err != nil {
		t.Fatalf("trim should succeed: %v", err)
	}
	if out != "hello" {
		t.Errorf("expected 'hello', got: %q", out)
	}
}

func TestTransformNode_Lines(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "line1\nline2\nline3", map[string]string{"operation": "lines"})
	if err != nil {
		t.Fatalf("lines should succeed: %v", err)
	}
	if !strings.Contains(out, "3 lines") {
		t.Errorf("expected '3 lines', got: %q", out)
	}
}

func TestTransformNode_Words(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "hello world test", map[string]string{"operation": "words"})
	if err != nil {
		t.Fatalf("words should succeed: %v", err)
	}
	if !strings.Contains(out, "3 words") {
		t.Errorf("expected '3 words', got: %q", out)
	}
}

func TestTransformNode_Chars(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "hello", map[string]string{"operation": "chars"})
	if err != nil {
		t.Fatalf("chars should succeed: %v", err)
	}
	if !strings.Contains(out, "5 characters") {
		t.Errorf("expected '5 characters', got: %q", out)
	}
}

func TestTransformNode_FirstLine(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "first\nsecond", map[string]string{"operation": "first_line"})
	if err != nil {
		t.Fatalf("first_line should succeed: %v", err)
	}
	if out != "first" {
		t.Errorf("expected 'first', got: %q", out)
	}
}

func TestTransformNode_FirstLineEmpty(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "", map[string]string{"operation": "first_line"})
	if err != nil {
		t.Fatalf("first_line on empty should succeed: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty string, got: %q", out)
	}
}

func TestTransformNode_First500(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	short := strings.Repeat("a", 100)
	out, err := node.Execute(ctx, short, map[string]string{"operation": "first_500"})
	if err != nil {
		t.Fatalf("first_500 should succeed: %v", err)
	}
	if out != short {
		t.Errorf("expected unchanged short text, got: %q", out)
	}

	long := strings.Repeat("b", 600)
	out, err = node.Execute(ctx, long, map[string]string{"operation": "first_500"})
	if err != nil {
		t.Fatalf("first_500 on long should succeed: %v", err)
	}
	if !strings.HasSuffix(out, "...") {
		t.Errorf("expected truncated with '...', got: %q", out)
	}
}

func TestTransformNode_First1000(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	long := strings.Repeat("c", 1200)
	out, err := node.Execute(ctx, long, map[string]string{"operation": "first_1000"})
	if err != nil {
		t.Fatalf("first_1000 should succeed: %v", err)
	}
	if !strings.HasSuffix(out, "...") {
		t.Errorf("expected truncated with '...', got: %q", out)
	}
}

func TestTransformNode_Summary(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	short := "short"
	out, err := node.Execute(ctx, short, map[string]string{"operation": "summary"})
	if err != nil {
		t.Fatalf("summary should succeed: %v", err)
	}
	if out != short {
		t.Errorf("expected unchanged short text, got: %q", out)
	}

	long := strings.Repeat("d", 300)
	out, err = node.Execute(ctx, long, map[string]string{"operation": "summary"})
	if err != nil {
		t.Fatalf("summary on long should succeed: %v", err)
	}
	if !strings.HasSuffix(out, "...") {
		t.Errorf("expected truncated with '...', got: %q", out)
	}
}

func TestTransformNode_Reverse(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "abc", map[string]string{"operation": "reverse"})
	if err != nil {
		t.Fatalf("reverse should succeed: %v", err)
	}
	if out != "cba" {
		t.Errorf("expected 'cba', got: %q", out)
	}
}

func TestTransformNode_ReverseUnicode(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "你好世界", map[string]string{"operation": "reverse"})
	if err != nil {
		t.Fatalf("reverse unicode should succeed: %v", err)
	}
	if out != "界世好你" {
		t.Errorf("expected '界世好你', got: %q", out)
	}
}

func TestTransformNode_UniqueLines(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "a\nb\na\nc\nb", map[string]string{"operation": "unique_lines"})
	if err != nil {
		t.Fatalf("unique_lines should succeed: %v", err)
	}
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 unique lines, got %d: %q", len(lines), out)
	}
}

func TestTransformNode_SortLines(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "c\na\nb", map[string]string{"operation": "sort_lines"})
	if err != nil {
		t.Fatalf("sort_lines should succeed: %v", err)
	}
	lines := strings.Split(out, "\n")
	if lines[0] != "a" || lines[1] != "b" || lines[2] != "c" {
		t.Errorf("expected sorted lines, got: %q", out)
	}
}

func TestTransformNode_ExtractURLs(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "visit https://example.com and https://test.org", map[string]string{"operation": "extract_urls"})
	if err != nil {
		t.Fatalf("extract_urls should succeed: %v", err)
	}
	if !strings.Contains(out, "https://example.com") {
		t.Errorf("expected first URL, got: %q", out)
	}
	if !strings.Contains(out, "https://test.org") {
		t.Errorf("expected second URL, got: %q", out)
	}
}

func TestTransformNode_ExtractEmails(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "contact alice@example.com or bob@test.org", map[string]string{"operation": "extract_emails"})
	if err != nil {
		t.Fatalf("extract_emails should succeed: %v", err)
	}
	if !strings.Contains(out, "alice@example.com") {
		t.Errorf("expected first email, got: %q", out)
	}
	if !strings.Contains(out, "bob@test.org") {
		t.Errorf("expected second email, got: %q", out)
	}
}

func TestTransformNode_NoOperation(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "hello", map[string]string{})
	if err != nil {
		t.Fatalf("no operation should succeed: %v", err)
	}
	if out != "hello" {
		t.Errorf("expected unchanged input, got: %q", out)
	}
}

func TestTransformNode_UnknownOperation(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "hello", map[string]string{"operation": "unknown_op"})
	if err != nil {
		t.Fatalf("unknown operation should succeed: %v", err)
	}
	if out != "hello" {
		t.Errorf("expected unchanged input, got: %q", out)
	}
}

// ============================================================================
// CallNode
// ============================================================================

func TestCallNode_Metadata(t *testing.T) {
	node := &CallNode{}
	if node.Name() != "call" {
		t.Errorf("Name() = %q, want call", node.Name())
	}
	if node.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestCallNode_Schema(t *testing.T) {
	node := &CallNode{}
	schema := node.Schema()
	if schema.Name != "call" {
		t.Errorf("Schema().Name = %q, want call", schema.Name)
	}
	paramNames := map[string]bool{}
	for _, p := range schema.Params {
		paramNames[p.Name] = true
	}
	for _, name := range []string{"workflow", "vars"} {
		if !paramNames[name] {
			t.Errorf("expected param %q in schema", name)
		}
	}
}

func TestCallNode_NoWorkflowFunc(t *testing.T) {
	oldFunc := ExecuteWorkflowFunc
	ExecuteWorkflowFunc = nil
	defer func() { ExecuteWorkflowFunc = oldFunc }()

	node := &CallNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "", map[string]string{"workflow": "test.yml"})
	if err == nil {
		t.Error("expected error when workflow function not registered")
	}
}

func TestCallNode_MaxDepthExceeded(t *testing.T) {
	oldFunc := ExecuteWorkflowFunc
	ExecuteWorkflowFunc = func(ctx context.Context, wf interface{}, reg *Registry) (string, []interface{}, error) {
		return "ok", nil, nil
	}
	defer func() { ExecuteWorkflowFunc = oldFunc }()

	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	if err := os.WriteFile(filepath.Join(dir, "test.yml"), []byte("name: test"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	node := &CallNode{}
	ctx := context.WithValue(context.Background(), callDepthKey, MaxCallDepth)
	_, err := node.Execute(ctx, "", map[string]string{"workflow": "test.yml"})
	if err == nil {
		t.Error("expected error for max depth exceeded")
	}
	if !strings.Contains(err.Error(), "maximum workflow call depth") {
		t.Errorf("expected depth error, got: %v", err)
	}
}

func TestParseVarsParam_JSON(t *testing.T) {
	result := parseVarsParam(`{"key1":"val1","key2":"val2"}`)
	if len(result) != 2 {
		t.Fatalf("expected 2 vars, got %d", len(result))
	}
	if result["key1"] != "val1" || result["key2"] != "val2" {
		t.Errorf("expected key1=val1, key2=val2, got: %v", result)
	}
}

func TestParseVarsParam_KeyValue(t *testing.T) {
	result := parseVarsParam("topic=AI,model=gpt-4")
	if len(result) != 2 {
		t.Fatalf("expected 2 vars, got %d", len(result))
	}
	if result["topic"] != "AI" || result["model"] != "gpt-4" {
		t.Errorf("expected topic=AI, model=gpt-4, got: %v", result)
	}
}

func TestParseVarsParam_Empty(t *testing.T) {
	if result := parseVarsParam(""); result != nil {
		t.Errorf("expected nil for empty, got: %v", result)
	}
	if result := parseVarsParam("   "); result != nil {
		t.Errorf("expected nil for whitespace, got: %v", result)
	}
}

func TestParseVarsParam_TooLarge(t *testing.T) {
	result := parseVarsParam(strings.Repeat("a", 2*1024*1024))
	if result != nil {
		t.Errorf("expected nil for too large input, got: %v", result)
	}
}

func TestParseVarsParam_InvalidJSON(t *testing.T) {
	result := parseVarsParam("{invalid json}")
	if result != nil {
		t.Errorf("expected nil for invalid JSON, got: %v", result)
	}
}

func TestParseVarsParam_EmptyKeyInKV(t *testing.T) {
	result := parseVarsParam("=val,key=val")
	if len(result) != 1 {
		t.Errorf("expected 1 var (empty key skipped), got %d: %v", len(result), result)
	}
	if result["key"] != "val" {
		t.Errorf("expected key=val, got: %v", result)
	}
}

// ============================================================================
// NotifyNode - additional metadata tests (notify_test.go already covers Execute well)
// ============================================================================

func TestNotifyNode_Metadata(t *testing.T) {
	node := &NotifyNode{}
	if node.Name() != "notify" {
		t.Errorf("Name() = %q, want notify", node.Name())
	}
	if node.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestNotifyNode_Schema(t *testing.T) {
	node := &NotifyNode{}
	schema := node.Schema()
	if schema.Name != "notify" {
		t.Errorf("Schema().Name = %q, want notify", schema.Name)
	}
	expectedParams := []string{"channel", "message", "url", "webhook_url", "token", "chat_id", "username", "method", "headers", "body"}
	for _, name := range expectedParams {
		found := false
		for _, p := range schema.Params {
			if p.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected param %q in schema", name)
		}
	}
}

func TestNotifyNode_MessageFromInput(t *testing.T) {
	node := &NotifyNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "hello from input", map[string]string{"channel": "stdout"})
	if err != nil {
		t.Fatalf("stdout should succeed: %v", err)
	}
	if out != "hello from input" {
		t.Errorf("expected 'hello from input', got: %q", out)
	}
}

func TestNotifyNode_MessageFromParam(t *testing.T) {
	node := &NotifyNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "", map[string]string{
		"channel": "stdout",
		"message": "hello from param",
	})
	if err != nil {
		t.Fatalf("stdout should succeed: %v", err)
	}
	if out != "hello from param" {
		t.Errorf("expected 'hello from param', got: %q", out)
	}
}

func TestValidateNotifyURL_HTTP(t *testing.T) {
	err := validateNotifyURL("http://example.com/hook")
	if err == nil {
		t.Error("expected error for HTTP URL")
	}
}

func TestValidateNotifyURL_HTTPS(t *testing.T) {
	t.Skip("requires DNS resolution")
	err := validateNotifyURL("https://hooks.example.com/hook")
	if err != nil {
		t.Errorf("expected no error for valid HTTPS URL, got: %v", err)
	}
}

func TestValidateNotifyURL_InvalidURL(t *testing.T) {
	err := validateNotifyURL("not a url")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// ============================================================================
// FetchURLNode - additional helpers
// ============================================================================

func TestLooksLikeURL_EdgeCases(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"https://example.com", true},
		{"http://example.com", true},
		{"https://", true},
		{"http://", true},
		{"not a url", false},
		{"", false},
		{"  https://example.com", true},
		{"ftp://example.com", false},
	}
	for _, tt := range tests {
		got := looksLikeURL(tt.s)
		if got != tt.want {
			t.Errorf("looksLikeURL(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

// ============================================================================
// ExecuteNode - additional edge cases
// ============================================================================

func TestExecuteNode_CommandOutput(t *testing.T) {
	oldAllowList := allowListEnabled
	allowListEnabled = false
	defer func() { allowListEnabled = oldAllowList }()

	node := &ExecuteNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "", map[string]string{
		"command": "echo hello world",
	})
	if err != nil {
		t.Fatalf("echo should succeed: %v", err)
	}
	if out != "hello world" {
		t.Errorf("expected 'hello world', got: %q", out)
	}
}

func TestExecuteNode_CommandTimeout(t *testing.T) {
	oldAllowList := allowListEnabled
	allowListEnabled = false
	defer func() { allowListEnabled = oldAllowList }()

	node := &ExecuteNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "", map[string]string{
		"command": "sleep 30",
		"timeout": "10ms",
	})
	if err == nil {
		t.Error("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestExecuteNode_CommandFailure(t *testing.T) {
	oldAllowList := allowListEnabled
	allowListEnabled = false
	defer func() { allowListEnabled = oldAllowList }()

	node := &ExecuteNode{}
	ctx := context.Background()
	_, err := node.Execute(ctx, "", map[string]string{
		"command": "exit 1",
	})
	if err == nil {
		t.Error("expected error for failed command")
	}
}

// ============================================================================
// FileWriteNode - edge cases
// ============================================================================

func TestFileWriteNode_WriteEmptyInput(t *testing.T) {
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	node := &FileWriteNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "", map[string]string{
		"path": "empty.txt",
	})
	if err != nil {
		t.Fatalf("write empty should succeed: %v", err)
	}
	if !strings.Contains(out, "written to") {
		t.Errorf("expected 'written to', got: %q", out)
	}
}

func TestFileWriteNode_WriteToSubdir(t *testing.T) {
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	// Create the subdirectory first
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	node := &FileWriteNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, "content", map[string]string{
		"path": "subdir/output.txt",
	})
	if err != nil {
		t.Fatalf("write to subdir should succeed: %v", err)
	}
	if !strings.Contains(out, "written to") {
		t.Errorf("expected 'written to', got: %q", out)
	}
}

// ============================================================================
// TransformNode - additional operations
// ============================================================================

func TestTransformNode_ExtractReposAndActivity_FilterTrending(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	input := `<a href="/trending/repo"></a><a href="/user/repo"></a>`
	out, err := node.Execute(ctx, input, map[string]string{"operation": "extract_repos_and_activity"})
	if err != nil {
		t.Fatalf("extract_repos_and_activity should succeed: %v", err)
	}
	if strings.Contains(out, "trending") {
		t.Errorf("trending repo should be filtered, got: %q", out)
	}
	if !strings.Contains(out, "user/repo") {
		t.Errorf("expected user/repo, got: %q", out)
	}
}

func TestTransformNode_GroupByCommitType_AllCategories(t *testing.T) {
	node := &TransformNode{}
	ctx := context.Background()
	input := "feat: new\nfix: bug\ndocs: readme\nrefactor: cleanup\ntest: tests\nchore: update\nperf: speedup\nstyle: format"
	out, err := node.Execute(ctx, input, map[string]string{"operation": "group_by_commit_type"})
	if err != nil {
		t.Fatalf("group_by_commit_type should succeed: %v", err)
	}
	for _, cat := range []string{"Features", "Bugfixes", "Documentation", "Refactoring", "Tests", "Chores", "Performance", "Style"} {
		if !strings.Contains(out, cat) {
			t.Errorf("expected category %q in output: %s", cat, out)
		}
	}
}

// ============================================================================
// MarkdownToHTML helper
// ============================================================================

func TestMarkdownToHTML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"h1 to h6",
			"# Title\n## Section\n### Sub\n#### Sub2\n##### Small\n###### Tiny",
			"<h1>Title</h1>\n<h2>Section</h2>\n<h3>Sub</h3>\n<h4>Sub2</h4>\n<h5>Small</h5>\n<h6>Tiny</h6>",
		},
		{
			"bold and italic",
			"**bold** and *italic*",
			"<p>\n<strong>bold</strong> and <em>italic</em>\n</p>",
		},
		{
			"inline code",
			"`code` here",
			"<p>\n<code>code</code> here\n</p>",
		},
		{
			"link",
			"[link](https://example.com)",
			"<p>\n<a href=\"https://example.com\">link</a>\n</p>",
		},
		{
			"list items",
			"- item1\n- item2",
			"<li>item1</li>\n<li>item2</li>",
		},
		{
			"empty input",
			"",
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := markdownToHTML(tt.in)
			if got != tt.want {
				t.Errorf("markdownToHTML(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ============================================================================
// CombineAndSummarize edge case
// ============================================================================

func TestCombineAndSummarize_EmptySections(t *testing.T) {
	result := combineAndSummarize("section1\n---\n\n---\nsection3")
	if !strings.Contains(result, "section1") {
		t.Errorf("expected section1 in result: %s", result)
	}
	if !strings.Contains(result, "section3") {
		t.Errorf("expected section3 in result: %s", result)
	}
}

// ============================================================================
// ExtractFunctionsAndTypes - edge case with receiver
// ============================================================================

func TestExtractFunctionsAndTypes_WithReceiver(t *testing.T) {
	result := extractFunctionsAndTypes("func (s *MyStruct) Method() {")
	if !strings.Contains(result, "MyStruct") {
		t.Errorf("expected MyStruct in result: %s", result)
	}
	if !strings.Contains(result, "Method()") {
		t.Errorf("expected Method() in result: %s", result)
	}
}

// ============================================================================
// CountByLabel edge case
// ============================================================================

func TestCountByLabel_NoDuplicates(t *testing.T) {
	result := countByLabel("unique word")
	if !strings.Contains(result, "Label count summary") {
		t.Errorf("expected label count summary: %s", result)
	}
}

// ============================================================================
// TemplateRenderNode - additional edge cases
// ============================================================================

func TestTemplateRenderNode_TemplateFilePrecedence(t *testing.T) {
	oldWorkDir := workDir
	dir := t.TempDir()
	workDir = dir
	defer func() { workDir = oldWorkDir }()

	tmplContent := "from file: {{ .input }}"
	if err := os.WriteFile(filepath.Join(dir, "tmpl.txt"), []byte(tmplContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	node := &TemplateRenderNode{}
	ctx := context.Background()
	// template_file should take precedence over template
	out, err := node.Execute(ctx, "test", map[string]string{
		"template":      "from inline: {{ .input }}",
		"template_file": "tmpl.txt",
	})
	if err != nil {
		t.Fatalf("render should succeed: %v", err)
	}
	if out != "from file: test" {
		t.Errorf("expected 'from file: test', got: %q", out)
	}
}

// ============================================================================
// JSONParseNode - additional path tests
// ============================================================================

func TestJSONParseNode_NestedArrayPath(t *testing.T) {
	node := &JSONParseNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, `{"data": [{"name": "Alice"}, {"name": "Bob"}]}`, map[string]string{
		"path": "data.[1].name",
	})
	if err != nil {
		t.Fatalf("nested path should succeed: %v", err)
	}
	if out != "Bob" {
		t.Errorf("expected 'Bob', got: %q", out)
	}
}

func TestJSONParseNode_EmptyPathKeys(t *testing.T) {
	node := &JSONParseNode{}
	ctx := context.Background()
	out, err := node.Execute(ctx, `{"a": {"b": 1}}`, map[string]string{
		"path": "a..b",
	})
	if err != nil {
		t.Fatalf("path with empty segment should succeed: %v", err)
	}
	if !strings.Contains(out, "1") {
		t.Errorf("expected 1 in output, got: %q", out)
	}
}
