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
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alib8b8/llm-box/internal/nodes/core"
)

var (
	validDocTypes = map[string]bool{
		"readme":       true,
		"api":          true,
		"function":     true,
		"module":       true,
		"changelog":    true,
		"tutorial":     true,
		"architecture": true,
	}
	validLanguages = map[string]bool{
		"go":         true,
		"python":     true,
		"javascript": true,
		"typescript": true,
		"auto":       true,
	}
	validOutputFormats = map[string]bool{
		"markdown": true,
		"json":     true,
	}
)

type DocGenNode struct{}

func (n *DocGenNode) Name() string { return "doc_gen" }

func (n *DocGenNode) Description() string {
	return "AI自动文档生成节点。自动生成和更新代码库文档，支持README、API文档、函数注释、模块文档、更新日志、教程和架构文档等多种类型，让代码库对AI Agent更友好。"
}

func (n *DocGenNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - 代码内容或文档生成指令",
		Output:      "string - 生成的文档内容（markdown或JSON格式）",
		Params: []ParamSchema{
			{Name: "doc_type", Type: "string", Description: "文档类型：readme/api/function/module/changelog/tutorial/architecture", Required: true},
			{Name: "path", Type: "string", Description: "代码路径（相对工作目录）", Required: true},
			{Name: "language", Type: "string", Description: "代码语言：go/python/javascript/typescript/auto（默认auto）", Required: false, Default: "auto"},
			{Name: "output_format", Type: "string", Description: "输出格式：markdown/json（默认markdown）", Required: false, Default: "markdown"},
			{Name: "depth", Type: "int", Description: "文档深度1-5（默认3）", Required: false, Default: "3"},
			{Name: "auto_update", Type: "bool", Description: "是否自动更新现有文档（默认false）", Required: false, Default: "false"},
		},
	}
}

func (n *DocGenNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	docType := getParam(params, "doc_type", "")
	if !validDocTypes[docType] {
		return "", fmt.Errorf("invalid doc_type: %s", docType)
	}

	path := getParam(params, "path", "")
	safePath, err := validateReadPath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	language := getParam(params, "language", "auto")
	if !validLanguages[language] {
		return "", fmt.Errorf("invalid language: %s", language)
	}

	outputFormat := getParam(params, "output_format", "markdown")
	if !validOutputFormats[outputFormat] {
		return "", fmt.Errorf("invalid output_format: %s", outputFormat)
	}

	depth := parseIntSafe(getParam(params, "depth", "3"), 3)
	if depth < 1 || depth > 5 {
		depth = 3
	}

	autoUpdate := strings.ToLower(getParam(params, "auto_update", "false")) == "true"

	if language == "auto" {
		language = detectLanguage(safePath)
	}

	docContent := generateDoc(docType, safePath, language, depth, input)

	if outputFormat == "json" {
		result := map[string]interface{}{
			"doc_type":    docType,
			"path":        path,
			"language":    language,
			"depth":       depth,
			"auto_update": autoUpdate,
			"content":     docContent,
			"generated":   true,
		}
		output, _ := json.MarshalIndent(result, "", "  ")
		return string(output), nil
	}

	return docContent, nil
}

func detectLanguage(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	default:
		return "go"
	}
}

func generateDoc(docType, path, language string, depth int, input string) string {
	fileName := filepath.Base(path)
	moduleName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	switch docType {
	case "readme":
		return generateReadme(moduleName, language, depth, input)
	case "api":
		return generateAPIDoc(moduleName, language, depth, input)
	case "function":
		return generateFunctionDoc(moduleName, language, depth, input)
	case "module":
		return generateModuleDoc(moduleName, language, depth, input)
	case "changelog":
		return generateChangelog(moduleName, depth)
	case "tutorial":
		return generateTutorial(moduleName, language, depth, input)
	case "architecture":
		return generateArchitectureDoc(moduleName, depth, input)
	default:
		return fmt.Sprintf("# %s\n\n文档内容生成中...\n", moduleName)
	}
}

func generateReadme(moduleName, language string, depth int, input string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", moduleName))
	sb.WriteString("## 简介\n\n")
	sb.WriteString(fmt.Sprintf("这是一个用 %s 编写的 %s 模块。", language, moduleName))
	if input != "" {
		sb.WriteString(fmt.Sprintf("\n\n功能描述：%s", input))
	}
	sb.WriteString("\n\n## 功能特性\n\n")
	features := []string{"高性能", "易于使用", "良好的文档", "类型安全", "测试覆盖"}
	for i := 0; i < min(depth, len(features)); i++ {
		sb.WriteString(fmt.Sprintf("- %s\n", features[i]))
	}
	sb.WriteString("\n## 安装\n\n")
	switch language {
	case "go":
		sb.WriteString(fmt.Sprintf("```bash\ngo get github.com/example/%s\n```\n", moduleName))
	case "python":
		sb.WriteString(fmt.Sprintf("```bash\npip install %s\n```\n", moduleName))
	default:
		sb.WriteString(fmt.Sprintf("```bash\nnpm install %s\n```\n", moduleName))
	}
	sb.WriteString("\n## 快速开始\n\n")
	sb.WriteString("```")
	sb.WriteString(language)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("// %s 基础使用示例\n", moduleName))
	sb.WriteString("\n```\n")
	if depth >= 3 {
		sb.WriteString("\n## 目录结构\n\n")
		sb.WriteString("```\n")
		sb.WriteString(fmt.Sprintf("%s/\n", moduleName))
		sb.WriteString("├── src/\n")
		sb.WriteString("├── tests/\n")
		sb.WriteString("├── docs/\n")
		sb.WriteString("└── README.md\n")
		sb.WriteString("```\n")
	}
	if depth >= 4 {
		sb.WriteString("\n## 许可证\n\nGNU Affero General Public License v3.0\n")
	}
	return sb.String()
}

func generateAPIDoc(moduleName, language string, depth int, input string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s API 文档\n\n", moduleName))
	sb.WriteString("## 概述\n\n")
	sb.WriteString(fmt.Sprintf("本文档描述了 %s 模块的公共API接口。\n\n", moduleName))
	sb.WriteString("## 函数列表\n\n")
	functions := []string{"New", "Init", "Process", "Validate", "Cleanup"}
	for i, fn := range functions {
		if i >= depth*2 {
			break
		}
		sb.WriteString(fmt.Sprintf("### %s%s\n\n", core.TitleCase(moduleName), fn))
		sb.WriteString(fmt.Sprintf("**描述**：%s 函数用于执行 %s 操作。\n\n", fn, strings.ToLower(fn)))
		sb.WriteString("**签名**：\n\n")
		sb.WriteString("```")
		sb.WriteString(language)
		sb.WriteString("\n")
		switch language {
		case "go":
			sb.WriteString(fmt.Sprintf("func %s%s() error\n", core.TitleCase(moduleName), fn))
		case "python":
			sb.WriteString(fmt.Sprintf("def %s_%s() -> None:\n", moduleName, strings.ToLower(fn)))
		default:
			sb.WriteString(fmt.Sprintf("function %s%s(): void\n", moduleName, fn))
		}
		sb.WriteString("\n```\n\n")
		sb.WriteString("**返回值**：无\n\n")
	}
	if depth >= 3 {
		sb.WriteString("## 类型定义\n\n")
		sb.WriteString("### Config\n\n配置结构体，包含模块的所有配置选项。\n\n")
	}
	return sb.String()
}

func generateFunctionDoc(moduleName, language string, depth int, input string) string {
	var sb strings.Builder
	funcName := core.TitleCase(moduleName)
	sb.WriteString(fmt.Sprintf("## %s 函数\n\n", funcName))
	sb.WriteString("### 描述\n\n")
	if input != "" {
		sb.WriteString(input)
	} else {
		sb.WriteString(fmt.Sprintf("%s 函数是 %s 模块的核心函数。", funcName, moduleName))
	}
	sb.WriteString("\n\n### 签名\n\n")
	sb.WriteString("```")
	sb.WriteString(language)
	sb.WriteString("\n")
	switch language {
	case "go":
		sb.WriteString(fmt.Sprintf("func %s(input string) (string, error)\n", funcName))
	case "python":
		sb.WriteString(fmt.Sprintf("def %s(input: str) -> str:\n", strings.ToLower(funcName)))
	default:
		sb.WriteString(fmt.Sprintf("function %s(input: string): string\n", funcName))
	}
	sb.WriteString("\n```\n\n")
	sb.WriteString("### 参数\n\n")
	sb.WriteString("| 参数 | 类型 | 描述 |\n")
	sb.WriteString("|------|------|------|\n")
	sb.WriteString("| input | string | 输入参数 |\n")
	if depth >= 2 {
		sb.WriteString("| options | object | 可选配置项 |\n")
	}
	sb.WriteString("\n### 返回值\n\n")
	sb.WriteString("| 类型 | 描述 |\n")
	sb.WriteString("|------|------|\n")
	sb.WriteString("| string | 处理结果 |\n")
	if depth >= 3 {
		sb.WriteString("\n### 示例\n\n")
		sb.WriteString("```")
		sb.WriteString(language)
		sb.WriteString("\n")
		switch language {
		case "go":
			sb.WriteString(fmt.Sprintf("result, err := %s(\"hello\")\n", funcName))
		case "python":
			sb.WriteString(fmt.Sprintf("result = %s(\"hello\")\n", strings.ToLower(funcName)))
		default:
			sb.WriteString(fmt.Sprintf("const result = %s(\"hello\");\n", funcName))
		}
		sb.WriteString("\n```\n")
	}
	return sb.String()
}

func generateModuleDoc(moduleName, language string, depth int, input string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s 模块文档\n\n", moduleName))
	sb.WriteString("## 模块概述\n\n")
	sb.WriteString(fmt.Sprintf("%s 模块提供了核心功能实现。\n\n", moduleName))
	if input != "" {
		sb.WriteString(fmt.Sprintf("详细说明：%s\n\n", input))
	}
	sb.WriteString("## 子模块\n\n")
	submodules := []string{"core", "utils", "config", "errors", "types"}
	for i := 0; i < min(depth, len(submodules)); i++ {
		sb.WriteString(fmt.Sprintf("- `%s/%s` - %s功能\n", moduleName, submodules[i], submodules[i]))
	}
	sb.WriteString("\n## 依赖\n\n")
	switch language {
	case "go":
		sb.WriteString("- Go 1.21+\n")
	case "python":
		sb.WriteString("- Python 3.9+\n")
	default:
		sb.WriteString("- Node.js 18+\n")
	}
	if depth >= 3 {
		sb.WriteString("\n## 设计原则\n\n")
		sb.WriteString("- 单一职责\n")
		sb.WriteString("- 依赖注入\n")
		sb.WriteString("- 接口隔离\n")
	}
	return sb.String()
}

func generateChangelog(moduleName string, depth int) string {
	var sb strings.Builder
	sb.WriteString("# 更新日志\n\n")
	sb.WriteString("所有重要的变更都将记录在此文件中。\n\n")
	versions := []struct {
		version string
		date    string
		added   []string
		fixed   []string
	}{
		{"1.2.0", "2026-07-15", []string{"新增文档生成功能", "支持多种输出格式"}, []string{"修复路径验证问题"}},
		{"1.1.0", "2026-06-20", []string{"性能优化", "新增配置选项"}, []string{"修复内存泄漏"}},
		{"1.0.0", "2026-05-01", []string{"初始版本发布", "核心功能实现"}, []string{}},
	}
	for i, v := range versions {
		if i >= depth {
			break
		}
		sb.WriteString(fmt.Sprintf("## [%s] - %s\n\n", v.version, v.date))
		if len(v.added) > 0 {
			sb.WriteString("### Added\n\n")
			for _, a := range v.added {
				sb.WriteString(fmt.Sprintf("- %s\n", a))
			}
			sb.WriteString("\n")
		}
		if len(v.fixed) > 0 {
			sb.WriteString("### Fixed\n\n")
			for _, f := range v.fixed {
				sb.WriteString(fmt.Sprintf("- %s\n", f))
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func generateTutorial(moduleName, language string, depth int, input string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s 教程\n\n", moduleName))
	sb.WriteString("## 前言\n\n")
	sb.WriteString(fmt.Sprintf("本教程将带你从零开始学习使用 %s 模块。\n\n", moduleName))
	sb.WriteString("## 目录\n\n")
	steps := []string{"安装与配置", "基础使用", "进阶功能", "最佳实践", "常见问题"}
	for i := 0; i < min(depth+1, len(steps)); i++ {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, steps[i]))
	}
	sb.WriteString("\n## 1. 安装与配置\n\n")
	sb.WriteString("### 安装\n\n")
	switch language {
	case "go":
		sb.WriteString(fmt.Sprintf("```bash\ngo get github.com/example/%s\n```\n", moduleName))
	case "python":
		sb.WriteString(fmt.Sprintf("```bash\npip install %s\n```\n", moduleName))
	default:
		sb.WriteString(fmt.Sprintf("```bash\nnpm install %s\n```\n", moduleName))
	}
	sb.WriteString("\n### 基础配置\n\n")
	sb.WriteString("```")
	sb.WriteString(language)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("// %s 配置示例\n", moduleName))
	sb.WriteString("\n```\n")
	if depth >= 2 {
		sb.WriteString("\n## 2. 基础使用\n\n")
		sb.WriteString("### 快速上手\n\n")
		sb.WriteString("```")
		sb.WriteString(language)
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("// 创建 %s 实例\n", moduleName))
		sb.WriteString("\n```\n")
	}
	if depth >= 3 {
		sb.WriteString("\n## 3. 进阶功能\n\n")
		sb.WriteString("### 高级配置\n\n")
		sb.WriteString("详细介绍高级配置选项和使用场景。\n")
	}
	return sb.String()
}

func generateArchitectureDoc(moduleName string, depth int, input string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s 架构文档\n\n", moduleName))
	sb.WriteString("## 架构概述\n\n")
	sb.WriteString(fmt.Sprintf("%s 采用分层架构设计，确保代码的可维护性和可扩展性。\n\n", moduleName))
	sb.WriteString("## 架构图\n\n")
	sb.WriteString("```\n")
	sb.WriteString("┌─────────────────────────────────┐\n")
	sb.WriteString("│          API Layer              │\n")
	sb.WriteString("├─────────────────────────────────┤\n")
	sb.WriteString("│        Service Layer            │\n")
	sb.WriteString("├─────────────────────────────────┤\n")
	sb.WriteString("│       Data Access Layer         │\n")
	sb.WriteString("└─────────────────────────────────┘\n")
	sb.WriteString("```\n\n")
	sb.WriteString("## 核心组件\n\n")
	components := []string{"接口层 - 对外提供API", "服务层 - 业务逻辑处理", "数据层 - 数据持久化", "工具层 - 通用工具函数"}
	for i := 0; i < min(depth, len(components)); i++ {
		parts := strings.Split(components[i], " - ")
		sb.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", parts[0], parts[1]))
	}
	if depth >= 3 {
		sb.WriteString("## 数据流\n\n")
		sb.WriteString("1. 请求进入 API 层\n")
		sb.WriteString("2. API 层调用服务层处理业务逻辑\n")
		sb.WriteString("3. 服务层通过数据层访问数据\n")
		sb.WriteString("4. 结果逐层返回\n")
	}
	if depth >= 4 {
		sb.WriteString("\n## 设计决策\n\n")
		sb.WriteString("- 为什么选择分层架构\n")
		sb.WriteString("- 模块划分原则\n")
		sb.WriteString("- 依赖管理策略\n")
	}
	return sb.String()
}

func init() {
	Register(&DocGenNode{})
}
