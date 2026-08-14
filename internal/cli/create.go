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
	"strings"

	"github.com/alib8b8/aflare/internal/agent"
	"github.com/alib8b8/aflare/internal/i18n"
	"github.com/alib8b8/aflare/internal/meta"
	skillsPkg "github.com/alib8b8/aflare/internal/skills"
	"github.com/alib8b8/aflare/internal/workflow"
)

// HandleCreate handles the "create" command.
//
// 断点9: aflare create 关键词匹配太弱。流程如下:
//  1. --ai 显式走 LLM 生成路径,不依赖关键词匹配。
//  2. 默认先走关键词匹配快速路径。
//  3. 关键词未匹配到有意义步骤时:
//     - 若已配置 LLM (aflare init 配置或环境变量), 自动降级到 LLM 生成。
//     - 若未配置 LLM, 打印可操作的建议 (改造已有模板 / --ai / 手动创建),
//     而不是静默生成一个只有占位 combine 节点的无意义工作流。
func HandleCreate(args []string, aiMode bool) {
	// Short-circuit --help/-h before any processing. Without this, --help
	// is treated as the workflow description and a file gets generated
	// (e.g. `aflare create --help` → writes a workflow from prompt "--help").
	for _, a := range args {
		if a == "--help" || a == "-h" {
			printCreateUsage()
			return
		}
	}

	interactive := false

	// Filter out --interactive flag from args
	filteredArgs := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--interactive" || a == "-i" {
			interactive = true
		} else {
			filteredArgs = append(filteredArgs, a)
		}
	}

	if len(filteredArgs) < 1 {
		printCreateUsage()
		os.Exit(1)
	}

	description := SummarizeCommand("", filteredArgs)
	fmt.Printf("%s\n", i18n.T("create.start", description))

	var filename string
	var err error
	if aiMode {
		// 显式 --ai: 直接走 LLM 生成路径,不依赖关键词匹配。
		filename, err = workflow.CreateWorkflowFromDescriptionWithAI(description, true)
	} else {
		filename, err = createWithKeywordOrFallback(description)
	}
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ %s\n", i18n.T("create.success", filename))
	fmt.Printf("\n%s\n", i18n.T("create.run_hint"))
	fmt.Printf("  aflare run %s\n", filename)

	if interactive {
		fmt.Println("\nEntering interactive chat mode to validate your workflow...")
		fmt.Println("Type /quit to exit.")
		EnterChatMode()
	}
}

// printCreateUsage prints the create command usage and examples. Shared by the
// --help short-circuit and the missing-prompt error path.
func printCreateUsage() {
	fmt.Println(i18n.T("create.usage"))
	fmt.Println("\nExamples:")
	fmt.Println("  aflare create \"fetch example.com and save to file\"")
	fmt.Println("  aflare create \"fetch Hacker News and save to hn.txt\"")
	fmt.Println("  aflare create \"summarize article and write to summary.md\"")
	fmt.Println("  aflare --ai create \"generate a weekly report from github commits\"")
	fmt.Println("  aflare create --interactive \"fetch example.com\"")
}

// createWithKeywordOrFallback runs the rule-based keyword matcher first (fast
// path). When the matcher produces only the placeholder combine step, it
// either falls back to LLM generation (if an LLM provider is configured) or
// returns an error instructing the caller to print actionable suggestions.
func createWithKeywordOrFallback(description string) (string, error) {
	wf, err := workflow.GenerateWorkflow(description)
	if err != nil {
		return "", err
	}

	if workflow.HasMeaningfulSteps(wf) {
		// 关键词匹配成功,直接保存。
		fname := workflow.GetSuggestedFilename(description)
		if err := workflow.SaveWorkflow(wf, fname); err != nil {
			return "", err
		}
		return "." + string(os.PathSeparator) + fname, nil
	}

	// 关键词未匹配到有意义步骤。
	if detectLLMConfig() {
		fmt.Println("ℹ️  关键词未匹配，已自动改用 LLM 生成工作流（用 --ai 可显式指定）…")
		return workflow.CreateWorkflowFromDescriptionWithAI(description, true)
	}

	// 未配置 LLM: 打印建议而不是静默生成无意义工作流。
	printCreateSuggestions(description)
	os.Exit(1)
	return "", nil
}

// printCreateSuggestions prints actionable next steps when keyword matching
// fails and no LLM is configured. It searches the template registry for the
// closest existing templates so the user can clone-and-modify instead of
// starting from scratch.
func printCreateSuggestions(description string) {
	fmt.Println("未找到完全匹配的模板。你可能需要：")
	fmt.Println()

	// 1. 用已有模板改造: 从描述中提取关键词搜索最接近的模板。
	matches := searchTemplatesForSuggestion(description)
	if len(matches) > 0 {
		fmt.Println("  1. 用已有模板改造：")
		shown := 0
		for _, s := range matches {
			if shown >= 3 {
				break
			}
			// 跳过无效条目。
			if s.ID == "" {
				continue
			}
			label := s.Name
			if label == "" {
				label = s.ID
			}
			fmt.Printf("     - %s （%s）\n", s.ID, label)
			shown++
		}
		if shown > 0 {
			cloneID := matches[0].ID
			fmt.Println()
			fmt.Printf("     复制并修改：aflare template clone %s my-workflow\n", cloneID)
		}
		fmt.Println()
	}

	// 2. 用自然语言描述生成 (需要 LLM)。
	fmt.Println("  2. 用自然语言描述生成（需要配置 LLM）：")
	fmt.Printf("     aflare create \"%s\" --ai\n", description)
	fmt.Println("     未配置 LLM？运行：aflare init")
	fmt.Println()

	// 3. 手动创建。
	skeletonName := suggestSkeletonName(description)
	fmt.Println("  3. 手动创建：")
	fmt.Printf("     aflare template new %s\n", skeletonName)
}

// createStopwords are common filler words (EN + ZH) stripped when extracting
// keywords for template search and skeleton-name suggestion, so the resulting
// hints reflect the user's intent rather than surrounding grammar.
var createStopwords = map[string]bool{
	"the": true, "a": true, "an": true, "to": true, "and": true, "or": true,
	"of": true, "for": true, "with": true, "in": true, "on": true, "at": true,
	"my": true, "me": true, "please": true, "help": true, "帮我": true,
	"自动": true, "一个": true, "的": true,
}

// searchTemplatesForSuggestion loads the template registry and searches for
// templates whose id/name/description/tags match keywords extracted from the
// user's description. Returns at most a handful of candidates ordered by id.
func searchTemplatesForSuggestion(description string) []*skillsPkg.SkillMeta {
	templatesDir := meta.ResolveTemplatesPath()
	if templatesDir == "" {
		return nil
	}
	registry := skillsPkg.NewSkillRegistry(templatesDir)
	if err := registry.Load(); err != nil {
		return nil
	}

	keywords := extractKeywords(description)

	var results []*skillsPkg.SkillMeta
	seen := make(map[string]bool)
	for _, kw := range keywords {
		for _, s := range registry.Search(kw) {
			if seen[s.ID] {
				continue
			}
			seen[s.ID] = true
			results = append(results, s)
		}
	}
	return results
}

// extractKeywords returns lowercase content words (length >= 2, not in
// createStopwords) from a natural-language description.
func extractKeywords(description string) []string {
	keywords := make([]string, 0, 8)
	for _, w := range strings.Fields(strings.ToLower(description)) {
		w = strings.Trim(w, ".,;:!?\"'()[]{}，。；：！？")
		if len(w) < 2 || createStopwords[w] {
			continue
		}
		keywords = append(keywords, w)
	}
	return keywords
}

// suggestSkeletonName derives a short kebab-case name for `aflare template new`
// from the description, falling back to "my-workflow". Stopwords are skipped
// so the name reflects intent (e.g. "auto-reply") rather than filler words.
func suggestSkeletonName(description string) string {
	var parts []string
	for _, w := range extractKeywords(description) {
		parts = append(parts, w)
		if len(parts) >= 2 {
			break
		}
	}
	if len(parts) == 0 {
		return "my-workflow"
	}
	return strings.Join(parts, "-")
}

// EnterChatMode starts an interactive chat session for workflow validation.
// This is called from create --interactive.
func EnterChatMode() {
	cfg := agent.DefaultConfig()
	session := agent.NewChatSession(cfg)
	session.Run()
}
