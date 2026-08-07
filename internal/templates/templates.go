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

package templates

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/alib8b8/aflare/internal/logger"
)

type TemplateVar struct {
	Name        string
	Description string
	Default     string
	Required    bool
}

type Template struct {
	Name        string
	Description string
	Category    string
	Tags        []string
	Version     string
	Variables   []TemplateVar
	Content     string
	// parsed is the pre-compiled form of Content. text/template is safe for
	// concurrent Execute calls once parsed (the parsed tree is read-only), so
	// caching it avoids re-parsing the immutable builtin content on every
	// Render. nil for templates that failed to pre-parse; Render falls back
	// to lazy parsing in that case so the error is surfaced to the caller.
	parsed *template.Template
}

type TemplateManager struct {
	templates map[string]*Template
}

// NewTemplateManager 创建模板管理器并注册内置模板。
func NewTemplateManager() *TemplateManager {
	tm := &TemplateManager{
		templates: make(map[string]*Template),
	}
	tm.registerBuiltins()
	return tm
}

func (tm *TemplateManager) registerBuiltins() {
	builtins := []*Template{
		buildSimpleLLMTemplate(),
		buildCodeReviewTemplate(),
		buildDataProcessingTemplate(),
		buildWebScraperTemplate(),
		buildTranslationTemplate(),
		buildBatchProcessorTemplate(),
		buildSecurityAuditTemplate(),
		buildSecurityPathTraversalTestTemplate(),
		buildSecuritySSRFTestTemplate(),
		buildSecurityCommandInjectionTestTemplate(),
	}
	for _, t := range builtins {
		// Pre-parse immutable builtin content once so Render avoids
		// re-parsing on every call. A parse failure here indicates a bug in
		// a builtin template; we log it and leave parsed nil so Render
		// surfaces the error to the caller rather than panicking at startup.
		if pt, err := template.New(t.Name).Parse(t.Content); err == nil {
			t.parsed = pt
		} else {
			logger.Error("failed to pre-parse builtin template; Render will lazy-parse and surface the error",
				"template", t.Name, "error", err)
		}
		tm.templates[t.Name] = t
	}
}

// List 返回按名称排序的全部模板列表。
func (tm *TemplateManager) List() []*Template {
	result := make([]*Template, 0, len(tm.templates))
	for _, t := range tm.templates {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Get 按名称获取模板，不存在时返回错误。
func (tm *TemplateManager) Get(name string) (*Template, error) {
	t, ok := tm.templates[name]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", name)
	}
	return t, nil
}

// Search 在模板名称、描述、分类与标签中模糊匹配关键词，返回按名称排序的结果。
func (tm *TemplateManager) Search(keyword string) []*Template {
	keyword = strings.ToLower(keyword)
	var result []*Template
	for _, t := range tm.templates {
		if strings.Contains(strings.ToLower(t.Name), keyword) ||
			strings.Contains(strings.ToLower(t.Description), keyword) ||
			strings.Contains(strings.ToLower(t.Category), keyword) {
			result = append(result, t)
			continue
		}
		for _, tag := range t.Tags {
			if strings.Contains(strings.ToLower(tag), keyword) {
				result = append(result, t)
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Categories 返回去重并按字母序排序的全部模板分类。
func (tm *TemplateManager) Categories() []string {
	catSet := make(map[string]struct{})
	for _, t := range tm.templates {
		catSet[t.Category] = struct{}{}
	}
	result := make([]string, 0, len(catSet))
	for cat := range catSet {
		result = append(result, cat)
	}
	sort.Strings(result)
	return result
}

// ListByCategory 返回属于指定分类的模板，按名称排序。
func (tm *TemplateManager) ListByCategory(category string) []*Template {
	var result []*Template
	for _, t := range tm.templates {
		if t.Category == category {
			result = append(result, t)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Render 渲染指定模板，应用默认值与传入变量，并校验必填项。
// name 指定模板名，vars 为运行时变量映射，缺失必填变量将返回错误。
func (tm *TemplateManager) Render(name string, vars map[string]string) (string, error) {
	t, err := tm.Get(name)
	if err != nil {
		return "", err
	}

	renderVars := make(map[string]string)
	for _, v := range t.Variables {
		if v.Default != "" {
			renderVars[v.Name] = v.Default
		}
	}
	for k, v := range vars {
		renderVars[k] = v
	}

	for _, v := range t.Variables {
		if v.Required {
			if _, ok := renderVars[v.Name]; !ok || renderVars[v.Name] == "" {
				return "", fmt.Errorf("required variable missing: %s", v.Name)
			}
		}
	}

	// Use the pre-parsed template when available (builtin content is
	// immutable and text/template is safe for concurrent Execute). Fall
	// back to lazy parsing only if pre-parse failed at registration.
	tmpl := t.parsed
	if tmpl == nil {
		var err error
		tmpl, err = template.New(name).Parse(t.Content)
		if err != nil {
			return "", fmt.Errorf("failed to parse template: %w", err)
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, renderVars); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return buf.String(), nil
}

func buildSimpleLLMTemplate() *Template {
	return &Template{
		Name:        "simple-llm",
		Description: "简单的 LLM 调用工作流，使用指定模型处理用户输入",
		Category:    "llm",
		Tags:        []string{"llm", "ai", "chat"},
		Version:     "1.0.0",
		Variables: []TemplateVar{
			{Name: "workflow_name", Description: "工作流名称", Default: "simple-llm-workflow", Required: false},
			{Name: "model", Description: "LLM 模型名称", Default: "llama3", Required: false},
			{Name: "prompt", Description: "提示词", Default: "Hello, how are you?", Required: true},
			{Name: "temperature", Description: "采样温度", Default: "0.7", Required: false},
			{Name: "output_file", Description: "输出文件路径", Default: "", Required: false},
		},
		Content: `name: {{.workflow_name}}
description: Simple LLM call workflow
vars:
  model: "{{.model}}"
  prompt: "{{.prompt}}"
  temperature: "{{.temperature}}"
steps:
  - node: ollama
    params:
      model: "{{.vars.model}}"
      prompt: "{{.vars.prompt}}"
      temperature: "{{.vars.temperature}}"
{{- if .output_file}}
  - node: file_write
    params:
      path: "{{.output_file}}"
{{- end}}
`,
	}
}

func buildCodeReviewTemplate() *Template {
	return &Template{
		Name:        "code-review",
		Description: "代码审查工作流，自动分析代码质量并生成审查报告",
		Category:    "development",
		Tags:        []string{"code-review", "development", "quality"},
		Version:     "1.0.0",
		Variables: []TemplateVar{
			{Name: "workflow_name", Description: "工作流名称", Default: "code-review-workflow", Required: false},
			{Name: "file_path", Description: "要审查的代码文件路径", Default: "main.go", Required: true},
			{Name: "model", Description: "LLM 模型名称", Default: "llama3", Required: false},
			{Name: "review_language", Description: "审查报告语言", Default: "Chinese", Required: false},
			{Name: "output_file", Description: "输出报告文件路径", Default: "code-review-report.md", Required: false},
		},
		Content: `name: {{.workflow_name}}
description: Automated code review workflow
vars:
  file_path: "{{.file_path}}"
  model: "{{.model}}"
  review_language: "{{.review_language}}"
  output_file: "{{.output_file}}"
steps:
  - node: file_read
    params:
      path: "{{.vars.file_path}}"
  - node: ollama
    params:
      model: "{{.vars.model}}"
      prompt: |
        Please review the following code and provide a detailed code review report in {{.vars.review_language}}.
        Focus on:
        1. Code quality and readability
        2. Potential bugs or issues
        3. Performance considerations
        4. Security vulnerabilities
        5. Suggestions for improvement

        Code:
        {{ "{{" }}.steps[0].output{{ "}}" }}
  - node: file_write
    params:
      path: "{{.vars.output_file}}"
  - node: notify
    params:
      channel: stdout
      message: "Code review completed. Report saved to {{.vars.output_file}}"
`,
	}
}

func buildDataProcessingTemplate() *Template {
	return &Template{
		Name:        "data-processing",
		Description: "数据处理工作流，读取、转换并保存数据",
		Category:    "data",
		Tags:        []string{"data", "processing", "transform"},
		Version:     "1.0.0",
		Variables: []TemplateVar{
			{Name: "workflow_name", Description: "工作流名称", Default: "data-processing-workflow", Required: false},
			{Name: "input_file", Description: "输入文件路径", Default: "input.txt", Required: true},
			{Name: "output_file", Description: "输出文件路径", Default: "output.txt", Required: true},
			{Name: "operation", Description: "转换操作 (uppercase, lowercase, trim, base64_encode, base64_decode)", Default: "trim", Required: false},
			{Name: "find", Description: "查找字符串（用于 replace 操作）", Default: "", Required: false},
			{Name: "replace", Description: "替换字符串（用于 replace 操作）", Default: "", Required: false},
		},
		Content: `name: {{.workflow_name}}
description: Data processing workflow
vars:
  input_file: "{{.input_file}}"
  output_file: "{{.output_file}}"
  operation: "{{.operation}}"
steps:
  - node: file_read
    params:
      path: "{{.vars.input_file}}"
{{- if eq .operation "replace"}}
  - node: transform
    params:
      operation: replace
      find: "{{.find}}"
      replace: "{{.replace}}"
{{- else}}
  - node: transform
    params:
      operation: "{{.vars.operation}}"
{{- end}}
  - node: file_write
    params:
      path: "{{.vars.output_file}}"
  - node: notify
    params:
      channel: stdout
      message: "Data processing completed. Output saved to {{.vars.output_file}}"
`,
	}
}

func buildWebScraperTemplate() *Template {
	return &Template{
		Name:        "web-scraper",
		Description: "网页抓取工作流，抓取网页内容并提取信息",
		Category:    "web",
		Tags:        []string{"web", "scraper", "fetch"},
		Version:     "1.0.0",
		Variables: []TemplateVar{
			{Name: "workflow_name", Description: "工作流名称", Default: "web-scraper-workflow", Required: false},
			{Name: "url", Description: "要抓取的网页 URL", Default: "https://example.com", Required: true},
			{Name: "mode", Description: "抓取模式 (text, raw)", Default: "text", Required: false},
			{Name: "output_file", Description: "输出文件路径", Default: "scraped-content.txt", Required: false},
			{Name: "extract_pattern", Description: "正则提取模式（可选）", Default: "", Required: false},
		},
		Content: `name: {{.workflow_name}}
description: Web scraping workflow
vars:
  url: "{{.url}}"
  mode: "{{.mode}}"
  output_file: "{{.output_file}}"
steps:
  - node: fetch_url
    params:
      url: "{{.vars.url}}"
      mode: "{{.vars.mode}}"
{{- if .extract_pattern}}
  - node: transform
    params:
      operation: regex
      pattern: "{{.extract_pattern}}"
{{- end}}
{{- if .output_file}}
  - node: file_write
    params:
      path: "{{.vars.output_file}}"
{{- end}}
  - node: notify
    params:
      channel: stdout
      message: "Web scraping completed for {{.vars.url}}"
`,
	}
}

func buildTranslationTemplate() *Template {
	return &Template{
		Name:        "translation",
		Description: "翻译工作流，将文本从一种语言翻译成另一种语言",
		Category:    "llm",
		Tags:        []string{"translation", "llm", "language"},
		Version:     "1.0.0",
		Variables: []TemplateVar{
			{Name: "workflow_name", Description: "工作流名称", Default: "translation-workflow", Required: false},
			{Name: "input_file", Description: "输入文件路径", Default: "input.txt", Required: true},
			{Name: "output_file", Description: "输出文件路径", Default: "translated.txt", Required: true},
			{Name: "source_language", Description: "源语言", Default: "English", Required: false},
			{Name: "target_language", Description: "目标语言", Default: "Chinese", Required: true},
			{Name: "model", Description: "LLM 模型名称", Default: "llama3", Required: false},
		},
		Content: `name: {{.workflow_name}}
description: Translation workflow
vars:
  input_file: "{{.input_file}}"
  output_file: "{{.output_file}}"
  source_language: "{{.source_language}}"
  target_language: "{{.target_language}}"
  model: "{{.model}}"
steps:
  - node: file_read
    params:
      path: "{{.vars.input_file}}"
  - node: ollama
    params:
      model: "{{.vars.model}}"
      prompt: |
        Translate the following text from {{.vars.source_language}} to {{.vars.target_language}}.
        Only provide the translation, no additional explanation.

        Text:
        {{ "{{" }}.steps[0].output{{ "}}" }}
  - node: file_write
    params:
      path: "{{.vars.output_file}}"
  - node: notify
    params:
      channel: stdout
      message: "Translation completed from {{.vars.source_language}} to {{.vars.target_language}}"
`,
	}
}

func buildBatchProcessorTemplate() *Template {
	return &Template{
		Name:        "batch-processor",
		Description: "批量处理工作流，使用循环批量处理多个项目",
		Category:    "data",
		Tags:        []string{"batch", "processing", "loop"},
		Version:     "1.0.0",
		Variables: []TemplateVar{
			{Name: "workflow_name", Description: "工作流名称", Default: "batch-processor-workflow", Required: false},
			{Name: "items", Description: "要处理的项目列表（每行一个）", Default: "item1\nitem2\nitem3", Required: true},
			{Name: "model", Description: "LLM 模型名称", Default: "llama3", Required: false},
			{Name: "processing_type", Description: "处理类型 (summarize, classify, analyze)", Default: "summarize", Required: false},
			{Name: "output_file", Description: "输出文件路径", Default: "batch-results.txt", Required: false},
			{Name: "concurrency", Description: "并发数", Default: "2", Required: false},
		},
		Content: `name: {{.workflow_name}}
description: Batch processing workflow
vars:
  items: "{{.items}}"
  model: "{{.model}}"
  processing_type: "{{.processing_type}}"
  output_file: "{{.output_file}}"
  concurrency: "{{.concurrency}}"
steps:
  - name: process-items
    node: ollama
    loop:
      items: "{{.vars.items}}"
      var: item
      concurrency: {{.concurrency}}
    params:
      model: "{{.vars.model}}"
      prompt: |
        {{- if eq .processing_type "summarize"}}
        Summarize the following item in one sentence: {{ "{{" }}.item{{ "}}" }}
        {{- else if eq .processing_type "classify"}}
        Classify the following item into one category (tech, business, sports, entertainment, other): {{ "{{" }}.item{{ "}}" }}
        {{- else}}
        Analyze the following item and provide key insights: {{ "{{" }}.item{{ "}}" }}
        {{- end}}
{{- if .output_file}}
  - node: file_write
    params:
      path: "{{.vars.output_file}}"
{{- end}}
  - node: notify
    params:
      channel: stdout
      message: "Batch processing completed"
`,
	}
}

func buildSecurityAuditTemplate() *Template {
	return &Template{
		Name:        "security-audit",
		Description: "全面安全审计工作流，自动运行代码审查、漏洞扫描和安全自检",
		Category:    "security",
		Tags:        []string{"security", "audit", "vulnerability", "safety"},
		Version:     "1.0.0",
		Variables: []TemplateVar{
			{Name: "workflow_name", Description: "工作流名称", Default: "security-audit-workflow", Required: false},
			{Name: "target_dir", Description: "要审计的目标目录", Default: ".", Required: false},
			{Name: "model", Description: "用于代码审查的 LLM 模型", Default: "llama3", Required: false},
			{Name: "output_file", Description: "审计报告输出路径", Default: "security-audit-report.md", Required: false},
			{Name: "depth", Description: "审计深度: basic, standard, deep (default: standard)", Default: "standard", Required: false},
		},
		Content: `name: {{.workflow_name}}
description: Security audit workflow - automated code review and vulnerability scanning
vars:
  target_dir: "{{.target_dir}}"
  model: "{{.model}}"
  output_file: "{{.output_file}}"
  depth: "{{.depth}}"
steps:
  - name: collect-source-files
    node: file_read
    params:
      path: "{{.vars.target_dir}}"
  - name: llm-security-review
    node: code_review
    params:
      model: "{{.vars.model}}"
      focus: security
  - name: verify-findings
    node: verify
    params:
      verifier_type: security
      output_format: detailed
  - name: generate-report
    node: combine
    params:
      separator: "\n\n---\n\n"
{{- if .output_file}}
  - node: file_write
    params:
      path: "{{.vars.output_file}}"
{{- end}}
  - node: notify
    params:
      channel: stdout
      message: "Security audit completed"
`,
	}
}

func buildSecurityPathTraversalTestTemplate() *Template {
	return &Template{
		Name:        "security-test-path-traversal",
		Description: "路径穿越安全自测套件，验证文件访问防护是否有效",
		Category:    "security",
		Tags:        []string{"security", "test", "path-traversal", "owasp"},
		Version:     "1.0.0",
		Variables: []TemplateVar{
			{Name: "workflow_name", Description: "工作流名称", Default: "security-path-traversal-test", Required: false},
			{Name: "base_dir", Description: "测试基准目录", Default: "tmp/security-test", Required: false},
			{Name: "report_file", Description: "测试报告路径", Default: "path-traversal-test-report.md", Required: false},
		},
		Content: `name: {{.workflow_name}}
description: Path traversal security self-test suite
vars:
  base_dir: "{{.base_dir}}"
  report_file: "{{.report_file}}"
steps:
  - name: test-dotdot-slash
    node: file_read
    params:
      path: "{{.vars.base_dir}}/../../../etc/passwd"
  - name: test-absolute-path
    node: file_read
    params:
      path: "/etc/passwd"
  - name: test-encoded-path
    node: file_read
    params:
      path: "{{.vars.base_dir}}/%2e%2e/%2e%2e/etc/passwd"
  - name: test-symlink-bypass
    node: file_read
    params:
      path: "{{.vars.base_dir}}/symlink_to_etc"
  - name: test-double-encoding
    node: file_read
    params:
      path: "{{.vars.base_dir}}/%252e%252e/etc/passwd"
  - name: test-null-byte
    node: file_read
    params:
      path: "{{.vars.base_dir}}/../../../etc/passwd%00.txt"
  - name: test-dotfile-write
    node: file_write
    params:
      path: "{{.vars.base_dir}}/.ssh/authorized_keys"
  - name: test-env-write
    node: file_write
    params:
      path: "{{.vars.base_dir}}/.env"
  - name: aggregate-results
    node: combine
    params:
      separator: "\n"
{{- if .report_file}}
  - node: file_write
    params:
      path: "{{.vars.report_file}}"
{{- end}}
  - node: notify
    params:
      channel: stdout
      message: "Path traversal security test suite completed"
`,
	}
}

func buildSecuritySSRFTestTemplate() *Template {
	return &Template{
		Name:        "security-test-ssrf",
		Description: "SSRF 安全自测套件，验证服务器端请求伪造防护是否有效",
		Category:    "security",
		Tags:        []string{"security", "test", "ssrf", "owasp"},
		Version:     "1.0.0",
		Variables: []TemplateVar{
			{Name: "workflow_name", Description: "工作流名称", Default: "security-ssrf-test", Required: false},
			{Name: "report_file", Description: "测试报告路径", Default: "ssrf-test-report.md", Required: false},
		},
		Content: `name: {{.workflow_name}}
description: SSRF security self-test suite
vars:
  report_file: "{{.report_file}}"
steps:
  - name: test-localhost
    node: http_request
    params:
      url: "http://localhost:8080/secret"
  - name: test-127-0-0-1
    node: http_request
    params:
      url: "http://127.0.0.1/admin"
  - name: test-metadata-service
    node: http_request
    params:
      url: "http://169.254.169.254/latest/meta-data/"
  - name: test-internal-private
    node: http_request
    params:
      url: "http://192.168.1.1/admin"
  - name: test-10-range
    node: http_request
    params:
      url: "http://10.0.0.1/internal"
  - name: test-172-range
    node: http_request
    params:
      url: "http://172.16.0.1/admin"
  - name: test-file-scheme
    node: http_request
    params:
      url: "file:///etc/passwd"
  - name: test-ftp-scheme
    node: http_request
    params:
      url: "ftp://internal-server/data"
  - name: test-dns-rebinding
    node: http_request
    params:
      url: "http://127.0.0.1.nip.io/admin"
  - name: test-ipv6-loopback
    node: http_request
    params:
      url: "http://[::1]/admin"
  - name: aggregate-results
    node: combine
    params:
      separator: "\n"
{{- if .report_file}}
  - node: file_write
    params:
      path: "{{.vars.report_file}}"
{{- end}}
  - node: notify
    params:
      channel: stdout
      message: "SSRF security test suite completed"
`,
	}
}

func buildSecurityCommandInjectionTestTemplate() *Template {
	return &Template{
		Name:        "security-test-command-injection",
		Description: "命令注入安全自测套件，验证命令注入防护是否有效",
		Category:    "security",
		Tags:        []string{"security", "test", "command-injection", "owasp"},
		Version:     "1.0.0",
		Variables: []TemplateVar{
			{Name: "workflow_name", Description: "工作流名称", Default: "security-command-injection-test", Required: false},
			{Name: "report_file", Description: "测试报告路径", Default: "command-injection-test-report.md", Required: false},
		},
		Content: `name: {{.workflow_name}}
description: Command injection security self-test suite
vars:
  report_file: "{{.vars.report_file}}"
steps:
  - name: test-semicolon-injection
    node: code_interpreter
    params:
      code: "print('hello'); import os; os.system('cat /etc/passwd')"
  - name: test-backtick-injection
    node: code_interpreter
    params:
      code: "print('hello [backtick injection test]')"
  - name: test-dollar-paren
    node: code_interpreter
    params:
      code: "import subprocess; subprocess.run('$(cat /etc/passwd)', shell=True)"
  - name: test-python-eval-exec
    node: code_interpreter
    params:
      code: "eval('__import__(\"os\").system(\"id\")')"
  - name: test-network-access
    node: code_interpreter
    params:
      code: "import urllib.request; print(urllib.request.urlopen('http://169.254.169.254/').read())"
      network: "false"
  - name: test-file-escape
    node: code_interpreter
    params:
      code: "with open('/etc/passwd') as f: print(f.read())"
  - name: test-shell-metachar
    node: code_interpreter
    params:
      code: "import subprocess; subprocess.run(['echo', 'test; id'], shell=False)"
  - name: test-python-subprocess-shell
    node: code_interpreter
    params:
      code: "import subprocess; subprocess.run('id; whoami', shell=True)"
  - name: aggregate-results
    node: combine
    params:
      separator: "\n"
{{- if .report_file}}
  - node: file_write
    params:
      path: "{{.vars.report_file}}"
{{- end}}
  - node: notify
    params:
      channel: stdout
      message: "Command injection security test suite completed"
`,
	}
}
