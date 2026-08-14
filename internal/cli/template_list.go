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

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alib8b8/aflare/internal/meta"
	skillsPkg "github.com/alib8b8/aflare/internal/skills"
	"gopkg.in/yaml.v3"
)

const (
	difficultyEasy   = "easy"   // no LLM, no sandbox
	difficultyMedium = "medium" // needs LLM
	difficultyHard   = "hard"   // needs LLM + bubblewrap sandbox
)

// nodes that require a running LLM provider.
var llmNodes = map[string]bool{
	"llm": true, "llm_router": true, "agent": true, "chat": true,
	"summarize": true, "translate": true, "classify": true,
	"researcher": true, "planner": true, "reflector": true,
	"critic": true, "evaluator": true, "supervisor": true,
	"doc_gen": true, "rag": true, "knowledge_graph_llm": true,
	"code_review": true, "code_knowledge_graph": true,
	"skill_distill": true,
	"subagent":      true, "agent_orchestrator": true,
	"clarify": true, "human_in_loop": true,
	"search_aggregate": true, "structured_output": true,
}

// nodes that require bubblewrap sandbox.
var sandboxNodes = map[string]bool{
	"code_interpreter": true, "sandbox": true,
}

// handleTemplateList implements `aflare template list`.
// By default only "easy" templates (no LLM/sandbox) are shown; --all shows
// everything, --category filters by category.
func handleTemplateList(args []string) {
	showAll := false
	category := ""
	for _, a := range args {
		switch {
		case a == "--all":
			showAll = true
		case strings.HasPrefix(a, "--category="):
			category = strings.TrimPrefix(a, "--category=")
		case a == "--category", a == "-c":
			// handled by caller loop; skip
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Printf("Unknown flag: %s\n", a)
			}
		}
	}

	templatesDir := meta.ResolveTemplatesPath()
	_ = skillsPkg.EnsureEmbeddedTemplates(templatesDir)
	registry := skillsPkg.NewSkillRegistry(templatesDir)
	if err := registry.Load(); err != nil {
		fmt.Printf("❌ 加载模板失败：%v\n", err)
		os.Exit(1)
	}

	skills := registry.List()
	if category != "" {
		skills = registry.ListByCategory(category)
	}

	easy, medium, hard := 0, 0, 0
	var rows []string
	for _, s := range skills {
		diff := resolveDifficulty(s)
		switch diff {
		case difficultyEasy:
			easy++
		case difficultyMedium:
			medium++
		case difficultyHard:
			hard++
		}
		if !showAll && diff != difficultyEasy {
			continue
		}
		rows = append(rows, formatTemplateRow(s, diff))
	}

	if len(rows) == 0 {
		if showAll {
			fmt.Println("未找到任何模板。")
		} else {
			fmt.Println("未找到无需额外配置的模板（easy）。使用 --all 查看全部模板。")
		}
		return
	}

	if showAll {
		fmt.Printf("全部模板（共 %d 个）：\n\n", len(rows))
	} else {
		fmt.Printf("可用模板（无需额外配置即可运行，共 %d 个）：\n\n", len(rows))
	}
	fmt.Printf("  %-44s %-8s %s\n", "模板", "难度", "说明")
	fmt.Println("  " + strings.Repeat("-", 76))
	for _, r := range rows {
		fmt.Println(r)
	}

	fmt.Println()
	fmt.Printf("难度统计：easy=%d  medium=%d(需LLM)  hard=%d(需LLM+沙箱)  总计=%d\n",
		easy, medium, hard, easy+medium+hard)
	if !showAll {
		fmt.Println("显示全部模板：aflare template list --all")
	}
	fmt.Println("按分类筛选：aflare template list --category <name>")
}

// resolveDifficulty returns the difficulty of a skill, auto-inferencing from
// the workflow's node types when the skill.json does not specify it.
func resolveDifficulty(s *skillsPkg.SkillMeta) string {
	if s.Difficulty != "" {
		return s.Difficulty
	}
	diff := inferDifficultyFromWorkflow(s.Path)
	s.Difficulty = diff // cache for this run
	return diff
}

// inferDifficultyFromWorkflow scans the workflow.yaml in the given directory
// and returns easy/medium/hard based on the node types used.
func inferDifficultyFromWorkflow(dir string) string {
	if dir == "" {
		return difficultyEasy
	}
	wfPath := filepath.Join(dir, "workflow.yaml")
	data, err := os.ReadFile(wfPath) // #nosec G304 -- internally resolved template path
	if err != nil {
		return difficultyEasy
	}

	// Minimal YAML structure to extract step node types without importing the
	// full workflow parser (avoids import cycles and is robust to schema drift).
	var wf struct {
		Steps []struct {
			Node     string `yaml:"node"`
			Parallel []struct {
				Node string `yaml:"node"`
			} `yaml:"parallel"`
			Map *struct {
				Over string `yaml:"over"`
			} `yaml:"map"`
			Saga *struct {
				Steps []struct {
					Node string `yaml:"node"`
				} `yaml:"steps"`
			} `yaml:"saga"`
			Loop *struct {
				Steps []struct {
					Node string `yaml:"node"`
				} `yaml:"steps"`
			} `yaml:"loop"`
		} `yaml:"steps"`
	}
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return difficultyEasy
	}

	diff := difficultyEasy
	for _, step := range wf.Steps {
		diff = maxDifficulty(diff, nodeDifficulty(step.Node))
		for _, p := range step.Parallel {
			diff = maxDifficulty(diff, nodeDifficulty(p.Node))
		}
		if step.Saga != nil {
			for _, ss := range step.Saga.Steps {
				diff = maxDifficulty(diff, nodeDifficulty(ss.Node))
			}
		}
		if step.Loop != nil {
			for _, ls := range step.Loop.Steps {
				diff = maxDifficulty(diff, nodeDifficulty(ls.Node))
			}
		}
	}
	return diff
}

// nodeDifficulty returns the difficulty contributed by a single node type.
func nodeDifficulty(node string) string {
	node = strings.TrimSpace(node)
	if sandboxNodes[node] {
		return difficultyHard
	}
	if llmNodes[node] {
		return difficultyMedium
	}
	return difficultyEasy
}

// maxDifficulty returns the higher of two difficulty levels.
func maxDifficulty(a, b string) string {
	order := map[string]int{difficultyEasy: 0, difficultyMedium: 1, difficultyHard: 2}
	if order[b] > order[a] {
		return b
	}
	return a
}

// formatTemplateRow formats a single template for list output.
func formatTemplateRow(s *skillsPkg.SkillMeta, diff string) string {
	desc := s.Description
	if len(desc) > 40 {
		desc = desc[:37] + "..."
	}
	return fmt.Sprintf("  %-44s %-8s %s", s.ID, diff, desc)
}

// handleTemplateNew implements `aflare template new <name>`.
// It creates a minimal workflow skeleton in the current directory.
func handleTemplateNew(args []string) {
	if len(args) == 0 {
		fmt.Println("Error: template new requires a <name>")
		fmt.Println("Usage: aflare template new <name>")
		os.Exit(1)
	}
	name := args[0]
	// name is joined into a filesystem path (./<name>/workflow.yaml);
	// validate it is a single safe component to prevent path traversal
	// (e.g. "../evil" would write outside the current directory).
	if err := validateTemplateNameComponent(name, "name"); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	dir := filepath.Join(".", name)
	wfPath := filepath.Join(dir, "workflow.yaml")

	if _, err := os.Stat(dir); err == nil {
		fmt.Printf("❌ 目录已存在：%s\n", dir)
		os.Exit(1)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("❌ 创建目录失败：%v\n", err)
		os.Exit(1)
	}

	skeleton := fmt.Sprintf(`name: %s
description: TODO: describe what this workflow does

# Input parameters (optional): define expected inputs for validation
# input_schema:
#   - name: target
#     type: string
#     required: true

steps:
  - node: fetch_url
    name: fetch
    params:
      url: "https://example.com"

  - node: transform
    name: process
    params:
      operation: extract_text

# Output: last step's output by default, or specify an expression
# output: "{{process.output}}"
`, name)

	if err := os.WriteFile(wfPath, []byte(skeleton), 0644); err != nil {
		fmt.Printf("❌ 写入失败：%v\n", err)
		os.Exit(1)
	}

	fmt.Println("已创建模板骨架：./" + filepath.Join(name, "workflow.yaml"))
	fmt.Println()
	fmt.Println("步骤：")
	fmt.Println("  1. 编辑 workflow.yaml，定义你的步骤")
	fmt.Println("  2. aflare validate " + wfPath)
	fmt.Println("  3. aflare run " + wfPath)
	fmt.Println()
	fmt.Println("文档：https://github.com/alib8b8/aflare/blob/main/docs/getting-started.md")
}

// handleTemplateClone implements `aflare template clone <source> <dest>`.
// It copies an existing template's workflow.yaml to a new local directory.
func handleTemplateClone(args []string) {
	if len(args) < 2 {
		fmt.Println("Error: template clone requires <source> <dest>")
		fmt.Println("Usage: aflare template clone <source-id> <dest-name>")
		os.Exit(1)
	}
	sourceID := args[0]
	destName := args[1]

	// destName is joined into a filesystem path (./<destName>/workflow.yaml);
	// validate it is a single safe component to prevent path traversal.
	if err := validateTemplateNameComponent(destName, "destination name"); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	templatesDir := meta.ResolveTemplatesPath()
	_ = skillsPkg.EnsureEmbeddedTemplates(templatesDir)
	registry := skillsPkg.NewSkillRegistry(templatesDir)
	if err := registry.Load(); err != nil {
		fmt.Printf("❌ 加载模板失败：%v\n", err)
		os.Exit(1)
	}

	src, err := registry.Get(sourceID)
	if err != nil {
		// Try fuzzy match by name suffix.
		all := registry.List()
		for _, s := range all {
			if s.Name == sourceID || strings.HasSuffix(s.ID, "/"+sourceID) {
				src = s
				break
			}
		}
		if src == nil {
			fmt.Printf("❌ 未找到模板：%s\n", sourceID)
			fmt.Println("提示：使用 aflare template list --all 查看所有可用模板")
			os.Exit(1)
		}
	}

	srcWf := filepath.Join(src.Path, "workflow.yaml")
	data, err := os.ReadFile(srcWf) // #nosec G304 -- internally resolved template path
	if err != nil {
		fmt.Printf("❌ 读取源模板失败：%v\n", err)
		os.Exit(1)
	}

	destDir := filepath.Join(".", destName)
	if _, err := os.Stat(destDir); err == nil {
		fmt.Printf("❌ 目标目录已存在：%s\n", destDir)
		os.Exit(1)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		fmt.Printf("❌ 创建目录失败：%v\n", err)
		os.Exit(1)
	}

	destWf := filepath.Join(destDir, "workflow.yaml")
	if err := os.WriteFile(destWf, data, 0644); err != nil {
		fmt.Printf("❌ 写入失败：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("已复制 %s → ./%s/workflow.yaml\n", sourceID, destName)
	fmt.Println("修改后运行：aflare run " + destWf)
}
