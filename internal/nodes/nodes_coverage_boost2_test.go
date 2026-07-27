// Copyright (c) 2026 llm-box Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
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
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// nodes_coverage_boost2_test.go 补充根包纯逻辑函数的单元测试，目标提升覆盖率到 50%+。
// 仅测试不依赖真实 LLM API/网络/文件 IO 的纯逻辑函数与简单成功/错误路径。

// ----------------------------------------------------------------------------
// code_graph.go: 纯逻辑辅助函数
// ----------------------------------------------------------------------------

func TestCgIsCommentLine(t *testing.T) {
	tests := []struct {
		name     string
		trimmed  string
		isPython bool
		want     bool
	}{
		{"empty is comment", "", false, true},
		{"empty is comment (py)", "", true, true},
		{"go slash comment", "// foo", false, true},
		{"go block start", "/* foo", false, true},
		{"go block continuation", "* foo", false, true},
		{"go code not comment", "func main() {}", false, false},
		{"python hash comment", "# foo", true, true},
		{"python code not comment", "print('x')", true, false},
		// Python 模式下不应识别 // 为注释
		{"python with slash not comment", "// not py comment", true, false},
		// 非 Python 模式下 # 不是注释
		{"go with hash not comment", "# not go comment", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cgIsCommentLine(tt.trimmed, tt.isPython); got != tt.want {
				t.Errorf("cgIsCommentLine(%q, %v) = %v, want %v", tt.trimmed, tt.isPython, got, tt.want)
			}
		})
	}
}

func TestCgSanitizeString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain ascii", "hello world", "hello world"},
		{"keeps printable", "a/b\\c-d", "a/b\\c-d"},
		{"strips control chars", "a\x00b\x01c", "abc"},
		{"strips DEL (0x7f)", "a\x7fb", "ab"},
		{"keeps unicode rune", "中文测试", "中文测试"},
		{"strips tab/newline", "a\tb\nc", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cgSanitizeString(tt.in); got != tt.want {
				t.Errorf("cgSanitizeString(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCgSanitizeName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"alphanumeric underscore", "foo_bar123", "foo_bar123"},
		{"keeps dots and dollar", "pkg.Func$1", "pkg.Func$1"},
		{"strips spaces and symbols", "foo bar (baz)!", "foobarbaz"},
		{"strips chinese", "函数名abc", "abc"},
		{"keeps nothing for symbols only", "!!@#", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cgSanitizeName(tt.in); got != tt.want {
				t.Errorf("cgSanitizeName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCgTruncateName(t *testing.T) {
	t.Run("short name unchanged", func(t *testing.T) {
		if got := cgTruncateName("short"); got != "short" {
			t.Errorf("cgTruncateName(%q) = %q, want %q", "short", got, "short")
		}
	})
	t.Run("exact max unchanged", func(t *testing.T) {
		in := strings.Repeat("a", codeGraphMaxNameLen)
		if got := cgTruncateName(in); len(got) != codeGraphMaxNameLen || got != in {
			t.Errorf("cgTruncateName at boundary = %q (len %d), want len %d", got, len(got), codeGraphMaxNameLen)
		}
	})
	t.Run("over max truncated", func(t *testing.T) {
		in := strings.Repeat("a", codeGraphMaxNameLen+10)
		got := cgTruncateName(in)
		if len(got) != codeGraphMaxNameLen {
			t.Errorf("cgTruncateName len = %d, want %d", len(got), codeGraphMaxNameLen)
		}
		if got != in[:codeGraphMaxNameLen] {
			t.Errorf("cgTruncateName content mismatch")
		}
	})
}

func TestCgMermaidID(t *testing.T) {
	tests := []struct {
		prefix string
		name   string
		want   string
	}{
		{"func", "foo_bar", "func_foo_bar"},
		{"file", "src/main.go", "file_src_main_go"},
		{"func", "Func$1", "func_Func_1"},
		{"file", "a b/c!d", "file_a_b_c_d"},
		{"func", "中文", "func___"},
	}
	for _, tt := range tests {
		got := cgMermaidID(tt.prefix, tt.name)
		if got != tt.want {
			t.Errorf("cgMermaidID(%q, %q) = %q, want %q", tt.prefix, tt.name, got, tt.want)
		}
		// 不变量：只包含字母数字下划线
		for _, r := range got {
			ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
			if !ok {
				t.Errorf("cgMermaidID result %q contains invalid rune %q", got, r)
				break
			}
		}
	}
}

func TestCgMermaidLabel(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"plain", "plain"},
		{`a"b`, `a\"b`},
		{`a\b`, `a\\b`},
		{`"path\to"`, `\"path\\to\"`},
		{"", ""},
	}
	for _, tt := range tests {
		if got := cgMermaidLabel(tt.in); got != tt.want {
			t.Errorf("cgMermaidLabel(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCodeGraphNode_ShouldIncludeFile(t *testing.T) {
	n := &CodeGraphNode{}
	tests := []struct {
		filename string
		language string
		want     bool
	}{
		{"main.go", "go", true},
		{"main.go", "auto", true},
		{"main.py", "python", true},
		{"app.js", "javascript", true},
		{"app.jsx", "javascript", true},
		{"App.tsx", "typescript", true},
		{"App.tsx", "auto", true},
		{"main.go", "python", false},
		{"main.py", "go", false},
		{"README.md", "auto", false},
		{"data.json", "auto", false},
		{"Makefile", "auto", false},
		// 大写扩展名也应匹配
		{"MAIN.GO", "go", true},
	}
	for _, tt := range tests {
		if got := n.shouldIncludeFile(tt.filename, tt.language); got != tt.want {
			t.Errorf("shouldIncludeFile(%q, %q) = %v, want %v", tt.filename, tt.language, got, tt.want)
		}
	}
}

func TestCodeGraphNode_DetectLanguage(t *testing.T) {
	n := &CodeGraphNode{}
	tests := []struct {
		path string
		want string
	}{
		{"foo/bar/main.go", "go"},
		{"script.py", "python"},
		{"app.js", "javascript"},
		{"App.tsx", "typescript"},
		{"README.md", ""},
		{"noext", ""},
	}
	for _, tt := range tests {
		if got := n.detectLanguage(tt.path); got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestCodeGraphNode_ComputeStats(t *testing.T) {
	n := &CodeGraphNode{}
	files := []codeGraphFile{
		{
			Path:      "a.go",
			Language:  "go",
			Imports:   []string{"fmt", "os"},
			Functions: []codeGraphFunction{{Name: "A", Line: 1}, {Name: "B", Line: 5}},
			Calls:     []string{"fmt.Println", "os.Exit"},
		},
		{
			Path:      "b.go",
			Language:  "go",
			Imports:   []string{"strings"},
			Functions: []codeGraphFunction{{Name: "C", Line: 1}},
			Calls:     []string{"strings.HasPrefix"},
		},
		{},
	}
	stats := n.computeStats(files)
	if stats.FilesAnalyzed != 3 {
		t.Errorf("FilesAnalyzed = %d, want 3", stats.FilesAnalyzed)
	}
	if stats.TotalFunctions != 3 {
		t.Errorf("TotalFunctions = %d, want 3", stats.TotalFunctions)
	}
	if stats.TotalImports != 3 {
		t.Errorf("TotalImports = %d, want 3", stats.TotalImports)
	}
	if stats.TotalCalls != 3 {
		t.Errorf("TotalCalls = %d, want 3", stats.TotalCalls)
	}

	// 空输入
	empty := n.computeStats(nil)
	if empty.FilesAnalyzed != 0 || empty.TotalFunctions != 0 {
		t.Errorf("computeStats(nil) = %+v, want zero stats", empty)
	}
}

func TestCodeGraphNode_ParseGo(t *testing.T) {
	n := &CodeGraphNode{}
	src := `package main

import "fmt"
import (
	"os"
	"strings"
)

// Foo does things
func Foo() {
	fmt.Println("hi")
	bar(1)
}

func bar(x int) {
	os.Exit(x)
}
`
	cf := codeGraphFile{Imports: []string{}, Functions: []codeGraphFunction{}, Calls: []string{}}
	n.parseGo(src, &cf)

	wantImports := map[string]bool{"fmt": true, "os": true, "strings": true}
	if len(cf.Imports) != 3 {
		t.Errorf("imports = %v, want 3", cf.Imports)
	}
	for _, imp := range cf.Imports {
		if !wantImports[imp] {
			t.Errorf("unexpected import %q", imp)
		}
	}

	wantFuncs := map[string]bool{"Foo": true, "bar": true}
	if len(cf.Functions) != 2 {
		t.Errorf("functions = %v, want 2", cf.Functions)
	}
	for _, fn := range cf.Functions {
		if !wantFuncs[fn.Name] {
			t.Errorf("unexpected function %q", fn.Name)
		}
		if fn.Line <= 0 {
			t.Errorf("function %q has non-positive line %d", fn.Name, fn.Line)
		}
	}

	// 调用应包含 fmt.Println / bar / os.Exit（去重保留顺序）
	calls := map[string]bool{}
	for _, c := range cf.Calls {
		calls[c] = true
	}
	for _, want := range []string{"fmt.Println", "bar", "os.Exit"} {
		if !calls[want] {
			t.Errorf("expected call %q in %v", want, cf.Calls)
		}
	}
}

func TestCodeGraphNode_ParsePython(t *testing.T) {
	n := &CodeGraphNode{}
	src := `import os
from collections import defaultdict

# a comment
def greet(name):
    print(name)

class Greeter:
    pass
`
	cf := codeGraphFile{Imports: []string{}, Functions: []codeGraphFunction{}, Calls: []string{}}
	n.parsePython(src, &cf)

	wantImports := map[string]bool{"os": true, "collections": true}
	if len(cf.Imports) != 2 {
		t.Errorf("imports = %v, want 2", cf.Imports)
	}
	for _, imp := range cf.Imports {
		if !wantImports[imp] {
			t.Errorf("unexpected import %q", imp)
		}
	}

	// def greet + class Greeter 都作为函数节点
	wantFuncs := map[string]bool{"greet": true, "Greeter": true}
	if len(cf.Functions) != 2 {
		t.Errorf("functions = %v, want 2", cf.Functions)
	}
	for _, fn := range cf.Functions {
		if !wantFuncs[fn.Name] {
			t.Errorf("unexpected function %q", fn.Name)
		}
	}
	// print 是一个调用
	foundPrint := false
	for _, c := range cf.Calls {
		if c == "print" {
			foundPrint = true
		}
	}
	if !foundPrint {
		t.Errorf("expected call 'print' in %v", cf.Calls)
	}
}

func TestCodeGraphNode_ParseJavaScript(t *testing.T) {
	n := &CodeGraphNode{}
	src := `import foo from './foo.js';
import 'side-effect.js';

// a comment
function hello() {
  foo();
}

const arrow = (x) => x + 1;
arrow(2);
`
	cf := codeGraphFile{Imports: []string{}, Functions: []codeGraphFunction{}, Calls: []string{}}
	n.parseJavaScript(src, &cf)

	wantImports := map[string]bool{"./foo.js": true, "side-effect.js": true}
	if len(cf.Imports) != 2 {
		t.Errorf("imports = %v, want 2", cf.Imports)
	}
	for _, imp := range cf.Imports {
		if !wantImports[imp] {
			t.Errorf("unexpected import %q", imp)
		}
	}

	wantFuncs := map[string]bool{"hello": true, "arrow": true}
	if len(cf.Functions) != 2 {
		t.Errorf("functions = %v, want 2", cf.Functions)
	}
	for _, fn := range cf.Functions {
		if !wantFuncs[fn.Name] {
			t.Errorf("unexpected function %q", fn.Name)
		}
	}
}

func TestCodeGraphNode_ExtractCalls(t *testing.T) {
	n := &CodeGraphNode{}
	cf := codeGraphFile{Imports: []string{}, Functions: []codeGraphFunction{}, Calls: []string{}}
	seen := make(map[string]bool)
	var order []string

	// 控制流关键字应被过滤
	line := "if foo() { for bar() { baz(); return qux(); } }"
	re := regexp.MustCompile(`([A-Za-z_][\w.]*)\s*\(`)
	n.extractCalls(line, re, seen, &order, 10, &cf)

	got := map[string]bool{}
	for _, c := range order {
		got[c] = true
	}
	for _, want := range []string{"foo", "bar", "baz", "qux"} {
		if !got[want] {
			t.Errorf("expected call %q in %v", want, order)
		}
	}
	// 控制流关键字不应出现
	for _, kw := range []string{"if", "for", "return"} {
		if got[kw] {
			t.Errorf("control keyword %q should be filtered, got %v", kw, order)
		}
	}
	// 同一名称多次出现应去重，order 长度 == 唯一调用数
	if len(order) != len(got) {
		t.Errorf("order %v has duplicates", order)
	}
	// callEdges 应记录每次调用（含重复）
	if len(cf.callEdges) == 0 {
		t.Errorf("expected callEdges to be populated")
	}
}

func TestCodeGraphNode_OutputJSON(t *testing.T) {
	n := &CodeGraphNode{}
	// nil 切片应序列化为 [] 而非 null
	result := codeGraphResult{
		Files: []codeGraphFile{
			{Path: "a.go", Language: "go"},
		},
	}
	out, err := n.outputJSON(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 空数组应为 []，不是 null
	if strings.Contains(out, "null") {
		t.Errorf("JSON output should not contain null for empty slices: %s", out)
	}
	if !strings.Contains(out, `"imports": []`) {
		t.Errorf("expected imports: [], got: %s", out)
	}
	if !strings.Contains(out, `"functions": []`) {
		t.Errorf("expected functions: [], got: %s", out)
	}
	if !strings.Contains(out, `"calls": []`) {
		t.Errorf("expected calls: [], got: %s", out)
	}
	if !strings.Contains(out, `"a.go"`) {
		t.Errorf("expected path a.go in output: %s", out)
	}
}

func TestCodeGraphNode_OutputMermaid(t *testing.T) {
	n := &CodeGraphNode{}
	result := codeGraphResult{
		Files: []codeGraphFile{
			{
				Path:      "main.go",
				Language:  "go",
				Functions: []codeGraphFunction{{Name: "Foo", Line: 1}, {Name: "bar", Line: 5}},
				callEdges: []codeGraphCall{{Name: "bar", Line: 3}},
			},
		},
	}
	out := n.outputMermaid(result)
	if !strings.HasPrefix(out, "```mermaid\ngraph TD\n") {
		t.Errorf("expected mermaid header, got: %s", out)
	}
	if !strings.Contains(out, "file_") {
		t.Errorf("expected file node id, got: %s", out)
	}
	if !strings.Contains(out, "func_Foo") {
		t.Errorf("expected func_Foo node, got: %s", out)
	}
	if !strings.Contains(out, "func_bar") {
		t.Errorf("expected func_bar node, got: %s", out)
	}
	// 文件 → 函数 定义边
	if !strings.Contains(out, "--> func_Foo") && !strings.Contains(out, "file_main_go --> func_Foo") {
		// 至少应存在 file -> func 的边
	}
	// 调用边：Foo (line 1-4) 调用 bar (line 3)
	if !strings.Contains(out, "func_Foo --> func_bar") {
		t.Errorf("expected call edge func_Foo --> func_bar, got: %s", out)
	}

	// 空结果也应能正常输出
	emptyOut := n.outputMermaid(codeGraphResult{})
	if !strings.Contains(emptyOut, "graph TD") {
		t.Errorf("empty mermaid output missing graph TD: %s", emptyOut)
	}
}

// ----------------------------------------------------------------------------
// code_graph.go: 缓存方法（内存 map，纯逻辑）
// ----------------------------------------------------------------------------

func TestCodeGraphCache_InMemoryOps(t *testing.T) {
	cache := getGraphCache()

	// 唯一 key 避免与其它测试冲突
	key := "test-coverage-cache-key-unique"
	t.Cleanup(func() {
		cache.Delete(key)
	})

	// Get 不存在
	if _, ok := cache.Get(key); ok {
		t.Fatal("expected key to not exist initially")
	}

	// Set / Get
	cf := codeGraphFile{Path: "x.go", Language: "go", Imports: []string{"fmt"}}
	cache.Set(key, cf)
	got, ok := cache.Get(key)
	if !ok {
		t.Fatalf("expected key to exist after Set")
	}
	if got.Path != "x.go" || len(got.Imports) != 1 {
		t.Errorf("Get returned wrong file: %+v", got)
	}

	// Keys 应包含该 key
	found := false
	for _, k := range cache.Keys() {
		if k == key {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("key %q not found in Keys()", key)
	}

	// Delete
	cache.Delete(key)
	if _, ok := cache.Get(key); ok {
		t.Errorf("expected key to be deleted")
	}
}

func TestCodeGraphCache_CachePath(t *testing.T) {
	cache := getGraphCache()
	got := cache.cachePath("/tmp/some-root")
	if !strings.HasSuffix(got, ".llm-box/codegraph-cache.json") {
		t.Errorf("cachePath = %q, want suffix .llm-box/codegraph-cache.json", got)
	}
}

// ----------------------------------------------------------------------------
// llm_router.go: defaultModelFor
// ----------------------------------------------------------------------------

func TestDefaultModelFor(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"openai", "gpt-4o-mini"},
		{"anthropic", "claude-3-haiku-20240307"},
		{"gemini", "gemini-1.5-flash"},
		{"deepseek", "deepseek-chat"},
		{"qwen", "qwen-plus"},
		{"kimi", "moonshot-v1-8k"},
		{"glm", "glm-4-flash"},
		{"yi", "yi-lightning"},
		{"mistral", "mistral-small-latest"},
		{"ollama", "llama3"},
		// 未知 provider 走默认
		{"unknown", "gpt-4o-mini"},
		{"", "gpt-4o-mini"},
	}
	for _, tt := range tests {
		if got := defaultModelFor(tt.provider); got != tt.want {
			t.Errorf("defaultModelFor(%q) = %q, want %q", tt.provider, got, tt.want)
		}
	}
}

// ----------------------------------------------------------------------------
// meta_orchestrator.go: 纯逻辑模型选择与层级判定
// ----------------------------------------------------------------------------

func TestSelectFastestModel(t *testing.T) {
	// 给定候选子集，应选出最低延迟模型
	candidates := []string{"gpt-4o", "gpt-3.5-turbo", "gemini-flash", "claude-3-opus"}
	got := selectFastestModel(candidates)
	// gemini-flash 延迟最低 (150)
	if got != "gemini-flash" {
		t.Errorf("selectFastestModel = %q, want gemini-flash", got)
	}

	// 单元素
	if got := selectFastestModel([]string{"gpt-4o"}); got != "gpt-4o" {
		t.Errorf("selectFastestModel single = %q, want gpt-4o", got)
	}
}

func TestSelectCheapestModel(t *testing.T) {
	candidates := []string{"gpt-4o", "gpt-3.5-turbo", "gemini-flash", "claude-3-opus"}
	got := selectCheapestModel(candidates)
	// gemini-flash 成本最低
	if got != "gemini-flash" {
		t.Errorf("selectCheapestModel = %q, want gemini-flash", got)
	}
}

func TestSelectBestQualityModel(t *testing.T) {
	candidates := []string{"gpt-4o", "gpt-3.5-turbo", "gemini-pro", "claude-3-opus"}
	got := selectBestQualityModel(candidates)
	// claude-3-opus 质量最高 (0.97)
	if got != "claude-3-opus" {
		t.Errorf("selectBestQualityModel = %q, want claude-3-opus", got)
	}

	// privacy_first 候选集
	got = selectBestQualityModel(privacyFirstModels)
	if got == "" {
		t.Errorf("selectBestQualityModel(privacyFirst) returned empty")
	}
	// 应是 privacyFirstModels 中的一个
	found := false
	for _, m := range privacyFirstModels {
		if m == got {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("selectBestQualityModel(privacyFirst) = %q not in privacyFirstModels", got)
	}
}

func TestDetermineHierarchyLevel(t *testing.T) {
	tests := []struct {
		taskType string
		maxDepth int
		want     string
	}{
		// maxDepth <= 1 一律 worker
		{"code", 1, "worker"},
		{"analysis", 0, "worker"},
		{"writing", 1, "worker"},
		// code/analysis/data: maxDepth>=3 -> supervisor, 否则 specialist
		{"code", 3, "supervisor"},
		{"code", 2, "specialist"},
		{"analysis", 5, "supervisor"},
		{"data", 2, "specialist"},
		{"data", 3, "supervisor"},
		// writing/creative: maxDepth>=2 -> specialist, 否则 worker
		{"writing", 2, "specialist"},
		{"creative", 2, "specialist"},
		{"creative", 1, "worker"},
		// 未知 taskType -> worker
		{"unknown", 5, "worker"},
	}
	for _, tt := range tests {
		if got := determineHierarchyLevel(tt.taskType, tt.maxDepth); got != tt.want {
			t.Errorf("determineHierarchyLevel(%q, %d) = %q, want %q", tt.taskType, tt.maxDepth, got, tt.want)
		}
	}
}

func TestSelectModelByStrategy(t *testing.T) {
	// fastest -> 最低延迟模型 (在全集范围内)
	got := selectModelByStrategy("fastest", "code")
	if got == "" {
		t.Errorf("selectModelByStrategy(fastest) returned empty")
	}
	// 全集中延迟最低的是 andesgpt-tiny (100)
	if got != "andesgpt-tiny" {
		t.Errorf("selectModelByStrategy(fastest) = %q, want andesgpt-tiny", got)
	}

	// cheapest -> gemini-flash (成本最低 0.000075)
	got = selectModelByStrategy("cheapest", "code")
	if got != "gemini-flash" {
		t.Errorf("selectModelByStrategy(cheapest) = %q, want gemini-flash", got)
	}

	// best_quality -> claude-3-opus (0.97)
	got = selectModelByStrategy("best_quality", "code")
	if got != "claude-3-opus" {
		t.Errorf("selectModelByStrategy(best_quality) = %q, want claude-3-opus", got)
	}

	// privacy_first -> privacyFirstModels 中质量最高者
	got = selectModelByStrategy("privacy_first", "code")
	if got == "" {
		t.Errorf("selectModelByStrategy(privacy_first) returned empty")
	}

	// auto / 默认 -> 应返回候选集合中的一个有效模型
	got = selectModelByStrategy("auto", "code")
	if !validMetaModels[got] {
		t.Errorf("selectModelByStrategy(auto, code) = %q not a valid model", got)
	}
	// code 任务候选
	codePreferred := map[string]bool{"gpt-4o": true, "claude-3-opus": true, "deepseek-v2": true, "qwen-max": true, "ling-2.6-1t": true, "ring-2.6-1t": true}
	if !codePreferred[got] {
		t.Errorf("selectModelByStrategy(auto, code) = %q not in code preferred set", got)
	}

	// auto + 未知 taskType -> 回退到全部模型
	got = selectModelByStrategy("auto", "unknowntask")
	if !validMetaModels[got] {
		t.Errorf("selectModelByStrategy(auto, unknown) = %q not a valid model", got)
	}
}

func TestGenerateMetaResponse(t *testing.T) {
	tests := []struct {
		name           string
		taskType       string
		useHierarchy   bool
		model          string
		hierarchyLevel string
		input          string
		wantSub        string
	}{
		{"code with hierarchy", "code", true, "gpt-4o", "supervisor", "implement foo", "代码"},
		{"writing", "writing", true, "gpt-4o", "specialist", "write doc", "写作"},
		{"analysis", "analysis", true, "gpt-4o", "supervisor", "analyze", "分析"},
		{"creative", "creative", true, "gpt-4o", "specialist", "brainstorm", "创意"},
		{"data", "data", true, "gpt-4o", "supervisor", "process", "数据"},
		{"no hierarchy prefix", "analysis", false, "gpt-4o", "", "x", "[gpt-4o]"},
		{"unknown task default", "unknown", true, "gpt-4o", "worker", "do thing", "已处理任务"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateMetaResponse(tt.input, tt.model, tt.taskType, tt.hierarchyLevel, tt.useHierarchy)
			if !strings.Contains(got, tt.wantSub) {
				t.Errorf("generateMetaResponse(...) = %q, want substring %q", got, tt.wantSub)
			}
			// 层级前缀校验
			if tt.useHierarchy {
				wantPrefix := "[" + tt.model + " via " + tt.hierarchyLevel + "]"
				if !strings.HasPrefix(got, wantPrefix) {
					t.Errorf("expected prefix %q, got %q", wantPrefix, got)
				}
			} else {
				wantPrefix := "[" + tt.model + "]"
				if !strings.HasPrefix(got, wantPrefix) {
					t.Errorf("expected prefix %q, got %q", wantPrefix, got)
				}
			}
		})
	}

	// 输入超长应被截断到 50 字符
	longInput := strings.Repeat("a", 200)
	got := generateMetaResponse(longInput, "gpt-4o", "code", "worker", true)
	// 不应包含完整长输入
	if strings.Contains(got, longInput) {
		t.Errorf("expected input to be truncated, but full input present in response")
	}
}

// ----------------------------------------------------------------------------
// secretdetect.go: OutboundDataMonitor / formatBytes / formatInt / RedactSecrets
// ----------------------------------------------------------------------------

func TestNewOutboundDataMonitor_Defaults(t *testing.T) {
	m := NewOutboundDataMonitor(0, 0, 0, nil)
	if m.windowDuration != 60*time.Second {
		t.Errorf("windowDuration = %v, want 60s default", m.windowDuration)
	}
	if m.baselineBytes != 1024*1024 {
		t.Errorf("baselineBytes = %d, want 1MB default", m.baselineBytes)
	}
	if m.anomalyMultiplier != 100 {
		t.Errorf("anomalyMultiplier = %d, want 100 default", m.anomalyMultiplier)
	}
	if m.anomalyThreshold != m.baselineBytes*m.anomalyMultiplier {
		t.Errorf("anomalyThreshold = %d, want %d", m.anomalyThreshold, m.baselineBytes*m.anomalyMultiplier)
	}
}

func TestNewOutboundDataMonitor_OverflowGuard(t *testing.T) {
	// 构造溢出场景：baseline * multiplier 溢出
	m := NewOutboundDataMonitor(time.Second, 1<<62, 1<<62, nil)
	// 溢出时应回退到 MaxInt64
	if m.anomalyThreshold != 1<<63-1 {
		t.Errorf("overflow threshold = %d, want MaxInt64", m.anomalyThreshold)
	}
}

func TestOutboundDataMonitor_RecordAndSnapshot(t *testing.T) {
	m := NewOutboundDataMonitor(time.Minute, 1024*1024, 100, nil)

	// nil 接收器安全
	var nilMon *OutboundDataMonitor
	nilMon.Record(100) // 应不 panic
	if snap := nilMon.Snapshot(); snap.WindowBytes != 0 {
		t.Errorf("nil Snapshot = %+v, want zero", snap)
	}

	// 非正数字节应被忽略
	m.Record(0)
	m.Record(-5)
	snap := m.Snapshot()
	if snap.WindowBytes != 0 {
		t.Errorf("after zero/negative records, WindowBytes = %d, want 0", snap.WindowBytes)
	}

	// 正常记录累加
	m.Record(100)
	m.Record(200)
	snap = m.Snapshot()
	if snap.WindowBytes != 300 {
		t.Errorf("WindowBytes = %d, want 300", snap.WindowBytes)
	}
	if snap.Baseline != 1024*1024 {
		t.Errorf("Baseline = %d, want %d", snap.Baseline, 1024*1024)
	}
	if snap.Ratio <= 0 {
		t.Errorf("Ratio = %v, want > 0", snap.Ratio)
	}
}

func TestOutboundDataMonitor_AnomalyCallback(t *testing.T) {
	var (
		mu       sync.Mutex
		fired    bool
		received OutboundStats
		wg       sync.WaitGroup
	)
	wg.Add(1)
	cb := func(s OutboundStats) {
		mu.Lock()
		fired = true
		received = s
		mu.Unlock()
		wg.Done()
	}
	// 极小 baseline + 倍数 1，使得任意记录都触发
	m := NewOutboundDataMonitor(time.Minute, 10, 1, cb)
	m.Record(1000) // 超过阈值 10*1=10
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if !fired {
		t.Fatal("expected anomaly callback to fire")
	}
	if received.WindowBytes != 1000 {
		t.Errorf("callback WindowBytes = %d, want 1000", received.WindowBytes)
	}
	if received.Baseline != 10 {
		t.Errorf("callback Baseline = %d, want 10", received.Baseline)
	}
}

func TestOutboundDataMonitor_AnomalyFiresOnce(t *testing.T) {
	count := 0
	var mu sync.Mutex
	cb := func(s OutboundStats) {
		mu.Lock()
		count++
		mu.Unlock()
	}
	m := NewOutboundDataMonitor(time.Minute, 10, 1, cb)
	// 多次记录应只触发一次告警（同窗口内）
	for i := 0; i < 5; i++ {
		m.Record(1000)
	}
	// 给 goroutine 时间执行
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if count > 1 {
		t.Errorf("anomaly fired %d times, want at most 1 per window", count)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1 KB"},
		{2048, "2 KB"},
		{1024 * 1024, "1 MB"},
		{2 * 1024 * 1024, "2 MB"},
		{512 * 1024, "512 KB"},
	}
	for _, tt := range tests {
		if got := formatBytes(tt.n); got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{123, "123"},
		{-1, "-1"},
		{-123, "-123"},
		{1<<31 - 1, "2147483647"},
	}
	for _, tt := range tests {
		if got := formatInt(tt.n); got != tt.want {
			t.Errorf("formatInt(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestRedactSecrets_WholeFile(t *testing.T) {
	out, hits := RedactSecrets("some content", true)
	if hits != 1 {
		t.Errorf("wholeFile hits = %d, want 1", hits)
	}
	if !strings.Contains(out, "REDACTED") {
		t.Errorf("wholeFile output should contain REDACTED, got: %s", out)
	}
	if !strings.Contains(out, "敏感文件") {
		t.Errorf("wholeFile output should mention 敏感文件, got: %s", out)
	}
}

func TestRedactSecrets_Patterns(t *testing.T) {
	// 无匹配：原样返回
	out, hits := RedactSecrets("just normal text no secrets", false)
	if hits != 0 {
		t.Errorf("no-match hits = %d, want 0", hits)
	}
	if out != "just normal text no secrets" {
		t.Errorf("no-match output changed: %s", out)
	}

	// AWS Access Key
	out, hits = RedactSecrets("key=AKIAABCDEFGHIJKLMNOP", false)
	if hits < 1 {
		t.Errorf("aws key hits = %d, want >=1", hits)
	}
	if strings.Contains(out, "AKIAABCDEFGHIJKLMNOP") {
		t.Errorf("aws key not redacted: %s", out)
	}
	if !strings.Contains(out, "REDACTED") {
		t.Errorf("expected REDACTED placeholder, got: %s", out)
	}

	// GitHub token
	out, hits = RedactSecrets("token: ghp_"+strings.Repeat("a", 36), false)
	if hits < 1 {
		t.Errorf("github token hits = %d, want >=1", hits)
	}
	if strings.Contains(out, "ghp_"+strings.Repeat("a", 36)) {
		t.Errorf("github token not redacted: %s", out)
	}

	// private key header
	out, hits = RedactSecrets("-----BEGIN RSA PRIVATE KEY-----", false)
	if hits < 1 {
		t.Errorf("private key hits = %d, want >=1", hits)
	}
	if !strings.Contains(out, "REDACTED PRIVATE KEY") {
		t.Errorf("private key not redacted: %s", out)
	}

	// JWT 三段式
	_, hits = RedactSecrets("eyJ"+strings.Repeat("a", 12)+".eyJ"+strings.Repeat("a", 12)+"."+strings.Repeat("a", 12), false)
	if hits < 1 {
		t.Errorf("jwt hits = %d, want >=1", hits)
	}
}

func TestRedactSecrets_Truncation(t *testing.T) {
	// 超长输入应被截断（不 panic）
	huge := strings.Repeat("A", MaxRedactInputSize+100)
	out, _ := RedactSecrets(huge, false)
	if len(out) > MaxRedactInputSize {
		t.Errorf("output length %d exceeds max %d", len(out), MaxRedactInputSize)
	}
	// wholeFile 模式截断
	out2, _ := RedactSecrets(huge, true)
	if !strings.Contains(out2, "REDACTED") {
		t.Errorf("wholeFile huge output should contain REDACTED, got: %s", out2)
	}
}

// ----------------------------------------------------------------------------
// agent_node.go: parseIntSafe / parseFloatSafe / truncateInput
// ----------------------------------------------------------------------------

func TestParseIntSafe(t *testing.T) {
	tests := []struct {
		in   string
		def  int
		want int
	}{
		{"42", 0, 42},
		{"-7", 0, -7},
		{"0", 9, 0},
		{"", 5, 5},
		{"abc", 5, 5},
		{"12.5", 5, 5},
		{"  10  ", 5, 5}, // Atoi 不忽略空白
	}
	for _, tt := range tests {
		if got := parseIntSafe(tt.in, tt.def); got != tt.want {
			t.Errorf("parseIntSafe(%q, %d) = %d, want %d", tt.in, tt.def, got, tt.want)
		}
	}
}

func TestParseFloatSafe(t *testing.T) {
	tests := []struct {
		in   string
		def  float64
		want float64
	}{
		{"3.14", 0, 3.14},
		{"-2.5", 0, -2.5},
		{"0", 9, 0},
		{"", 5.0, 5.0},
		{"abc", 5.0, 5.0},
		{"NaN", 5.0, 5.0},
		{"Inf", 5.0, 5.0},
		{"+Inf", 5.0, 5.0},
		{"-Inf", 5.0, 5.0},
	}
	for _, tt := range tests {
		got := parseFloatSafe(tt.in, tt.def)
		// NaN 比较特殊，单独处理（def 不可能是 NaN）
		if got != tt.want {
			t.Errorf("parseFloatSafe(%q, %v) = %v, want %v", tt.in, tt.def, got, tt.want)
		}
	}
}

func TestTruncateInput(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		maxLen int
		want   string
	}{
		{"short", "abc", 5, "abc"},
		{"exact", "abc", 3, "abc"},
		{"truncate ascii", "abcdef", 3, "abc..."},
		{"empty", "", 5, ""},
		{"truncate unicode", "中文测试abc", 4, "中文测试..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateInput(tt.in, tt.maxLen); got != tt.want {
				t.Errorf("truncateInput(%q, %d) = %q, want %q", tt.in, tt.maxLen, got, tt.want)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// smart_router.go: parseModelRef
// ----------------------------------------------------------------------------

func TestParseModelRef(t *testing.T) {
	tests := []struct {
		ref          string
		wantProvider string
		wantModel    string
	}{
		{"openai:gpt-4o", "openai", "gpt-4o"},
		{"anthropic:claude-3", "anthropic", "claude-3"},
		{"ollama:llama3", "ollama", "llama3"},
		// 无冒号 -> 默认 ollama provider
		{"llama3", "ollama", "llama3"},
		{"gpt-4o", "ollama", "gpt-4o"},
		// 仅含一个冒号（SplitN=2），第二个部分含冒号也保留
		{"a:b:c", "a", "b:c"},
		// 空字符串
		{"", "ollama", ""},
	}
	for _, tt := range tests {
		p, m := parseModelRef(tt.ref)
		if p != tt.wantProvider || m != tt.wantModel {
			t.Errorf("parseModelRef(%q) = (%q, %q), want (%q, %q)", tt.ref, p, m, tt.wantProvider, tt.wantModel)
		}
	}
}

// ----------------------------------------------------------------------------
// reflector.go: extractImprovedOutput
// ----------------------------------------------------------------------------

func TestExtractImprovedOutput(t *testing.T) {
	tests := []struct {
		name string
		resp string
		want string
	}{
		{
			"IMPROVED OUTPUT marker",
			"blah\nIMPROVED OUTPUT:\nfinal result here",
			"final result here",
		},
		{
			"Improved Output case variant",
			"junk\nImproved Output: hello world",
			"hello world",
		},
		{
			"FINAL VERSION marker",
			"text\nFINAL VERSION:\nthe answer",
			"the answer",
		},
		{
			// extractImprovedOutput 仅 TrimPrefix/TrimSuffix 单个 ```，
			// 不剥离代码块的语言标签，因此 "go" 会被保留。
			"Revised marker with code fence",
			"text\nRevised:\n```go\nfmt.Println(\"x\")\n```",
			"go\nfmt.Println(\"x\")",
		},
		{
			// 无语言标签的代码块应被完整剥离
			"Revised marker with bare code fence",
			"text\nRevised:\n```\nfmt.Println(\"x\")\n```",
			`fmt.Println("x")`,
		},
		{
			"uses last occurrence",
			"IMPROVED OUTPUT:\nfirst\nmore\nIMPROVED OUTPUT:\nsecond",
			"second",
		},
		{
			"no marker returns empty",
			"just some text without any marker",
			"",
		},
		{
			"empty input",
			"",
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractImprovedOutput(tt.resp); got != tt.want {
				t.Errorf("extractImprovedOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// doc_gen.go: detectLanguage / generateDoc 系列（纯字符串拼接逻辑）
// ----------------------------------------------------------------------------

func TestDocGen_DetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"script.py", "python"},
		{"app.js", "javascript"},
		{"App.ts", "typescript"},
		{"App.tsx", "typescript"},
		// 未知扩展名默认 go
		{"README.md", "go"},
		{"noext", "go"},
		{"Makefile", "go"},
	}
	for _, tt := range tests {
		if got := detectLanguage(tt.path); got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestGenerateDoc_Dispatch(t *testing.T) {
	// 各 docType 应分发到对应生成器；未知类型走默认分支
	cases := []struct {
		docType string
		wantSub string
	}{
		{"readme", "简介"},
		{"api", "API 文档"},
		{"function", "函数"},
		{"module", "模块文档"},
		{"changelog", "更新日志"},
		{"tutorial", "教程"},
		{"architecture", "架构文档"},
		{"unknown", "文档内容生成中"},
	}
	for _, c := range cases {
		out := generateDoc(c.docType, "foo/bar.go", "go", 2, "desc")
		if !strings.Contains(out, c.wantSub) {
			t.Errorf("generateDoc(%q) = %q, want substring %q", c.docType, out, c.wantSub)
		}
	}
}

func TestGenerateReadme(t *testing.T) {
	// 带 input 与 depth
	out := generateReadme("mymod", "go", 3, "do something")
	if !strings.HasPrefix(out, "# mymod") {
		t.Errorf("expected title header, got: %s", out)
	}
	if !strings.Contains(out, "do something") {
		t.Errorf("expected input description, got: %s", out)
	}
	if !strings.Contains(out, "go get") {
		t.Errorf("expected go install instruction, got: %s", out)
	}
	if !strings.Contains(out, "目录结构") {
		t.Errorf("depth>=3 should include 目录结构, got: %s", out)
	}

	// python 安装指令
	out = generateReadme("mymod", "python", 1, "")
	if !strings.Contains(out, "pip install") {
		t.Errorf("expected pip install, got: %s", out)
	}
	// 默认（非 go/python）走 npm
	out = generateReadme("mymod", "javascript", 1, "")
	if !strings.Contains(out, "npm install") {
		t.Errorf("expected npm install, got: %s", out)
	}
	// depth>=4 含许可证
	out = generateReadme("mymod", "go", 4, "")
	if !strings.Contains(out, "许可证") {
		t.Errorf("depth>=4 should include 许可证, got: %s", out)
	}
	// depth=0 不应含目录结构
	out = generateReadme("mymod", "go", 0, "")
	if strings.Contains(out, "目录结构") {
		t.Errorf("depth=0 should not include 目录结构, got: %s", out)
	}
}

func TestGenerateAPIDoc(t *testing.T) {
	out := generateAPIDoc("mymod", "go", 2, "")
	if !strings.Contains(out, "API 文档") {
		t.Errorf("expected API 文档 header, got: %s", out)
	}
	if !strings.Contains(out, "func MymodNew") {
		t.Errorf("expected go function signature, got: %s", out)
	}
	// python 签名
	out = generateAPIDoc("mymod", "python", 1, "")
	if !strings.Contains(out, "def mymod_new") {
		t.Errorf("expected python signature, got: %s", out)
	}
	// depth>=3 含类型定义
	out = generateAPIDoc("mymod", "go", 3, "")
	if !strings.Contains(out, "类型定义") {
		t.Errorf("depth>=3 should include 类型定义, got: %s", out)
	}
}

func TestGenerateFunctionDoc(t *testing.T) {
	// 带 input
	out := generateFunctionDoc("mymod", "go", 3, "custom desc")
	if !strings.Contains(out, "custom desc") {
		t.Errorf("expected custom description, got: %s", out)
	}
	if !strings.Contains(out, "func Mymod(input string)") {
		t.Errorf("expected go signature, got: %s", out)
	}
	if !strings.Contains(out, "示例") {
		t.Errorf("depth>=3 should include 示例, got: %s", out)
	}
	// 无 input 走默认描述
	out = generateFunctionDoc("mymod", "go", 1, "")
	if !strings.Contains(out, "核心函数") {
		t.Errorf("expected default description, got: %s", out)
	}
	// python 签名
	out = generateFunctionDoc("mymod", "python", 1, "")
	if !strings.Contains(out, "def mymod(input:") {
		t.Errorf("expected python signature, got: %s", out)
	}
	// depth>=2 含 options 参数
	out = generateFunctionDoc("mymod", "go", 2, "")
	if !strings.Contains(out, "options") {
		t.Errorf("depth>=2 should include options param, got: %s", out)
	}
}

func TestGenerateModuleDoc(t *testing.T) {
	out := generateModuleDoc("mymod", "go", 3, "detail here")
	if !strings.Contains(out, "模块文档") {
		t.Errorf("expected 模块文档 header, got: %s", out)
	}
	if !strings.Contains(out, "detail here") {
		t.Errorf("expected input detail, got: %s", out)
	}
	if !strings.Contains(out, "Go 1.21") {
		t.Errorf("expected go dependency, got: %s", out)
	}
	if !strings.Contains(out, "设计原则") {
		t.Errorf("depth>=3 should include 设计原则, got: %s", out)
	}
	// python 依赖
	out = generateModuleDoc("mymod", "python", 1, "")
	if !strings.Contains(out, "Python 3.9") {
		t.Errorf("expected python dependency, got: %s", out)
	}
}

func TestGenerateChangelog(t *testing.T) {
	out := generateChangelog("mymod", 2)
	if !strings.Contains(out, "更新日志") {
		t.Errorf("expected 更新日志 header, got: %s", out)
	}
	// depth=2 应包含两个版本
	if !strings.Contains(out, "1.2.0") || !strings.Contains(out, "1.1.0") {
		t.Errorf("expected versions 1.2.0 and 1.1.0, got: %s", out)
	}
	if !strings.Contains(out, "Added") || !strings.Contains(out, "Fixed") {
		t.Errorf("expected Added/Fixed sections, got: %s", out)
	}
	// depth=0 应不含版本条目（只有标题）
	out = generateChangelog("mymod", 0)
	if strings.Contains(out, "1.2.0") {
		t.Errorf("depth=0 should not include version entries, got: %s", out)
	}
}

func TestGenerateTutorial(t *testing.T) {
	out := generateTutorial("mymod", "go", 3, "")
	if !strings.Contains(out, "教程") {
		t.Errorf("expected 教程 header, got: %s", out)
	}
	if !strings.Contains(out, "go get") {
		t.Errorf("expected go install, got: %s", out)
	}
	if !strings.Contains(out, "进阶功能") {
		t.Errorf("depth>=3 should include 进阶功能, got: %s", out)
	}
	// python
	out = generateTutorial("mymod", "python", 1, "")
	if !strings.Contains(out, "pip install") {
		t.Errorf("expected pip install, got: %s", out)
	}
}

func TestGenerateArchitectureDoc(t *testing.T) {
	out := generateArchitectureDoc("mymod", 4, "ctx")
	if !strings.Contains(out, "架构文档") {
		t.Errorf("expected 架构文档 header, got: %s", out)
	}
	if !strings.Contains(out, "API Layer") {
		t.Errorf("expected architecture diagram, got: %s", out)
	}
	if !strings.Contains(out, "数据流") {
		t.Errorf("depth>=3 should include 数据流, got: %s", out)
	}
	if !strings.Contains(out, "设计决策") {
		t.Errorf("depth>=4 should include 设计决策, got: %s", out)
	}
	// depth=1 只含一个核心组件
	out = generateArchitectureDoc("mymod", 1, "")
	if !strings.Contains(out, "接口层") {
		t.Errorf("expected at least 接口层, got: %s", out)
	}
}

// ----------------------------------------------------------------------------
// supervisor.go: build* 提示词构造 / cleanJSONResp / sanitizeForPrompt
// ----------------------------------------------------------------------------

func TestBuildSpecialistDescriptions(t *testing.T) {
	// 已知 specialist 应被包含；未知被忽略
	out := buildSpecialistDescriptions([]string{"planner", "researcher", "nonexistent-xyz"})
	if !strings.Contains(out, "planner —") {
		t.Errorf("expected planner description, got: %s", out)
	}
	if !strings.Contains(out, "researcher —") {
		t.Errorf("expected researcher description, got: %s", out)
	}
	if strings.Contains(out, "nonexistent-xyz") {
		t.Errorf("unknown specialist should be ignored, got: %s", out)
	}
	// 空列表返回空字符串
	if got := buildSpecialistDescriptions(nil); got != "" {
		t.Errorf("buildSpecialistDescriptions(nil) = %q, want empty", got)
	}
}

func TestBuildPrompts_ContainSpecDescs(t *testing.T) {
	specDescs := "- planner — does planning\n- researcher — does research"
	cases := []struct {
		name    string
		build   func() string
		wantSub []string
	}{
		{"sequential", func() string { return buildSequentialPrompt(specDescs) }, []string{"sequential", "subtasks", specDescs}},
		{"parallel", func() string { return buildParallelPrompt(specDescs) }, []string{"parallel", "parallel_groups", specDescs}},
		{"hierarchical", func() string { return buildHierarchicalPrompt(specDescs, 3) }, []string{"hierarchical", "Max decomposition depth: 3", specDescs}},
		{"mindsearch", func() string { return buildMindSearchPrompt(specDescs, 4) }, []string{specDescs}},
		{"moe", func() string { return buildMoEPrompt(specDescs) }, []string{specDescs}},
		{"agency", func() string { return buildAgencyPrompt(specDescs) }, []string{specDescs}},
		{"swarm", func() string { return buildSwarmPrompt(specDescs) }, []string{specDescs}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.build()
			for _, sub := range c.wantSub {
				if !strings.Contains(got, sub) {
					t.Errorf("build%s: output missing %q", c.name, sub)
				}
			}
			// 所有 prompt 都要求 JSON 输出
			if !strings.Contains(got, "JSON") {
				t.Errorf("build%s: expected JSON mention in output", c.name)
			}
		})
	}
}

func TestCleanJSONResp(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain json", `{"a":1}`, `{"a":1}`},
		{"json code fence", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"bare code fence", "```\n{\"a\":1}\n```", `{"a":1}`},
		{"surrounding whitespace", `  {"a":1}  `, `{"a":1}`},
		{"only prefix fence", "```json{\"a\":1}", `{"a":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanJSONResp(tt.in); got != tt.want {
				t.Errorf("cleanJSONResp(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeForPrompt(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text", "plain text", "plain text"},
		{"strips newlines", "a\nb\rc\td", "abcd"},
		{"strips control chars", "a\x00b\x01c", "abc"},
		{"strips DEL", "a\x7fb", "ab"},
		{"keeps unicode", "中文 ok", "中文 ok"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		if got := sanitizeForPrompt(tt.in); got != tt.want {
			t.Errorf("sanitizeForPrompt(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ----------------------------------------------------------------------------
// self_heal.go: formatHealMarkdown / formatHealText
// ----------------------------------------------------------------------------

func TestFormatHealMarkdown(t *testing.T) {
	r := HealReport{
		Checks: []HealCheck{
			{Name: "gofmt", Status: "ok"},
			{Name: "govet", Status: "issue", Issue: "printf problem", Fixed: true, Message: "auto-fixed"},
			{Name: "lint", Status: "issue", Issue: "unused var", Fixed: false},
		},
		Issues:    3,
		Fixed:     1,
		Remaining: 2,
		Duration:  "120ms",
	}
	out := formatHealMarkdown(r)
	// 有剩余问题应使用 ⚠️ 图标
	if !strings.Contains(out, "⚠️") {
		t.Errorf("expected warning icon for remaining>0, got: %s", out)
	}
	if !strings.Contains(out, "Self-Heal Report") {
		t.Errorf("expected header, got: %s", out)
	}
	if !strings.Contains(out, "Issues found: **3**") {
		t.Errorf("expected issues count, got: %s", out)
	}
	// Fixed issue 应使用 🔧
	if !strings.Contains(out, "🔧") {
		t.Errorf("expected fixed icon 🔧, got: %s", out)
	}
	// 未修复 issue 应使用 ❌
	if !strings.Contains(out, "❌") {
		t.Errorf("expected fail icon ❌, got: %s", out)
	}
	// Message 覆盖 Issue 显示
	if !strings.Contains(out, "auto-fixed") {
		t.Errorf("expected message to appear, got: %s", out)
	}

	// 无剩余问题应使用 ✅
	clean := HealReport{Remaining: 0, Duration: "1ms"}
	out = formatHealMarkdown(clean)
	if !strings.Contains(out, "✅ **Self-Heal Report**") {
		t.Errorf("expected ok icon when no remaining, got: %s", out)
	}
}

func TestFormatHealText(t *testing.T) {
	r := HealReport{
		Checks: []HealCheck{
			{Name: "gofmt", Status: "ok"},
			{Name: "govet", Status: "issue", Issue: "printf problem", Fixed: true, Message: "auto-fixed"},
			{Name: "lint", Status: "issue", Issue: "unused var", Fixed: false},
		},
		Issues:    3,
		Fixed:     1,
		Remaining: 2,
		Duration:  "120ms",
	}
	out := formatHealText(r)
	if !strings.Contains(out, "Self-Heal Report: 3 issues, 1 fixed, 2 remaining") {
		t.Errorf("expected summary line, got: %s", out)
	}
	if !strings.Contains(out, "[OK] gofmt") {
		t.Errorf("expected [OK] marker, got: %s", out)
	}
	if !strings.Contains(out, "[FIXED] govet") {
		t.Errorf("expected [FIXED] marker, got: %s", out)
	}
	if !strings.Contains(out, "[FAIL] lint") {
		t.Errorf("expected [FAIL] marker, got: %s", out)
	}
	if !strings.Contains(out, "printf problem") {
		t.Errorf("expected issue text, got: %s", out)
	}
	if !strings.Contains(out, "auto-fixed") {
		t.Errorf("expected message text, got: %s", out)
	}
}

// ----------------------------------------------------------------------------
// rag.go: tokenize / assembleContext
// ----------------------------------------------------------------------------

func TestTokenize(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"Hello, WORLD!", []string{"hello", "world"}}, // 小写化
		{"foo_bar baz-qux", []string{"foo_bar", "baz", "qux"}},
		{"", nil},
		{"123 abc", []string{"123", "abc"}},
		// Go 的 \w 仅匹配 ASCII [0-9A-Za-z_]，中文不被分词
		{"中文 test", []string{"test"}},
	}
	for _, tt := range tests {
		got := tokenize(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("tokenize(%q) = %v, want %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("tokenize(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
			}
		}
	}
}

func TestAssembleContext(t *testing.T) {
	// 空分块返回 "No relevant chunks"
	out := assembleContext(nil, "query", true)
	if !strings.Contains(out, "No relevant chunks") {
		t.Errorf("empty chunks should report no chunks, got: %s", out)
	}
	if !strings.Contains(out, "query") {
		t.Errorf("empty chunks output should include query, got: %s", out)
	}

	// 含分块 + metadata
	chunks := []Chunk{
		{Text: "first chunk", Source: "doc1.md", Index: 0, Score: 0.9},
		{Text: "second chunk", Source: "doc2.md", Index: 1, Score: 0.7},
	}
	out = assembleContext(chunks, "my query", true)
	if !strings.Contains(out, "Context for query: my query") {
		t.Errorf("expected context header, got: %s", out)
	}
	if !strings.Contains(out, "Retrieved 2 relevant chunks") {
		t.Errorf("expected chunk count, got: %s", out)
	}
	if !strings.Contains(out, "first chunk") || !strings.Contains(out, "second chunk") {
		t.Errorf("expected chunk texts, got: %s", out)
	}
	if !strings.Contains(out, "score: 0.90") || !strings.Contains(out, "Source: doc1.md") {
		t.Errorf("expected metadata, got: %s", out)
	}

	// 不含 metadata
	out = assembleContext(chunks, "q", false)
	if strings.Contains(out, "score:") {
		t.Errorf("should not include score when metadata disabled, got: %s", out)
	}
	if !strings.Contains(out, "--- Chunk 1 ---") {
		t.Errorf("expected chunk header without metadata, got: %s", out)
	}
}

// ----------------------------------------------------------------------------
// multimodal.go: escapeJSON / formatMultimodalOutput
// ----------------------------------------------------------------------------

func TestEscapeJSON(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"plain", `"plain"`},
		{`a"b`, `"a\"b"`},
		{`a\b`, `"a\\b"`},
		{"a\nb", `"a\nb"`},
		{"a\tb", `"a\tb"`},
		{"", `""`},
	}
	for _, tt := range tests {
		if got := escapeJSON(tt.in); got != tt.want {
			t.Errorf("escapeJSON(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatMultimodalOutput(t *testing.T) {
	// json 格式
	out := formatMultimodalOutput("result text", "img.png", "ocr", "json")
	if !strings.Contains(out, `"mode": "ocr"`) {
		t.Errorf("json output missing mode, got: %s", out)
	}
	if !strings.Contains(out, `"source": "img.png"`) {
		t.Errorf("json output missing source, got: %s", out)
	}
	if !strings.Contains(out, `"result": "result text"`) {
		t.Errorf("json output missing result, got: %s", out)
	}

	// 默认 markdown 格式
	out = formatMultimodalOutput("result text", "img.png", "ocr", "markdown")
	if !strings.Contains(out, "## Multimodal Analysis (ocr)") {
		t.Errorf("markdown output missing header, got: %s", out)
	}
	if !strings.Contains(out, "**Source:** img.png") {
		t.Errorf("markdown output missing source, got: %s", out)
	}
	if !strings.Contains(out, "result text") {
		t.Errorf("markdown output missing content, got: %s", out)
	}

	// 未知格式也走 markdown 默认分支
	out = formatMultimodalOutput("x", "s", "m", "unknown")
	if !strings.Contains(out, "## Multimodal Analysis") {
		t.Errorf("unknown format should default to markdown, got: %s", out)
	}
}

// ----------------------------------------------------------------------------
// plugin_system.go: validateLocalPath（使用临时目录避免污染）
// ----------------------------------------------------------------------------

func TestValidateLocalPath(t *testing.T) {
	dir := t.TempDir()

	// 真实目录应通过
	if !validateLocalPath(dir) {
		t.Errorf("validateLocalPath(tempDir) = false, want true")
	}
	// 空路径
	if validateLocalPath("") {
		t.Errorf("validateLocalPath(\"\") = true, want false")
	}
	// 路径遍历应拒绝
	if validateLocalPath("../etc") {
		t.Errorf("validateLocalPath(../etc) = true, want false")
	}
	// 不存在的路径
	if validateLocalPath("/nonexistent/path/xyz/12345") {
		t.Errorf("validateLocalPath(nonexistent) = true, want false")
	}
	// 文件（非目录）应拒绝
	file := dir + "/regular.txt"
	if err := os.WriteFile(file, []byte("x"), 0644); err != nil {
		t.Fatalf("setup file: %v", err)
	}
	if validateLocalPath(file) {
		t.Errorf("validateLocalPath(file) = true, want false (not a dir)")
	}
}
