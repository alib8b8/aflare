// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌​​‌​​​‌‌​​‌‌‌​‌‌‌​‌‌‌​​‌​‌​‌​‌​‌​‌‌​‌​​​‌‌​​​​​​​​​​​​​​​​​​​‌‌‌​​​​‌‌​‌‌‌‌‌‌⁠
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
	"errors"
	"fmt"
	"os"

	"github.com/alib8b8/aflare/internal/agent"
	"github.com/alib8b8/aflare/internal/i18n"
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
func HandleCreate(args []string, aiMode bool) error {
	// Short-circuit --help/-h before any processing. Without this, --help
	// is treated as the workflow description and a file gets generated
	// (e.g. `aflare create --help` → writes a workflow from prompt "--help").
	for _, a := range args {
		if a == "--help" || a == "-h" {
			printCreateUsage()
			return nil
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
		return exitErr(1)
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
		// createWithKeywordOrFallback may return an ExitError after already
		// printing its message; propagate it unchanged to avoid double output.
		var ee *ExitError
		if errors.As(err, &ee) {
			return err
		}
		fmt.Printf("Error: %v\n", err)
		return exitErr(1)
	}

	fmt.Printf("\n✅ %s\n", i18n.T("create.success", filename))
	fmt.Printf("\n%s\n", i18n.T("create.run_hint"))
	fmt.Printf("  aflare run %s\n", filename)

	// 当未显式使用 --ai 但已配置 LLM 时，提示用户可用更强的生成路径，
	// 避免用户不知道关键词匹配之外还有 LLM 生成 / 交互式 chat 可用。
	if !aiMode && detectLLMConfig() {
		fmt.Println()
		fmt.Println("💡 " + i18n.T("create.llm_hint.title"))
		fmt.Println("   aflare --ai create \"" + i18n.T("create.llm_hint.needs") + "\"   # " + i18n.T("create.llm_hint.ai_desc"))
		fmt.Println("   aflare chat                       # " + i18n.T("create.llm_hint.chat_desc"))
	}

	if interactive {
		fmt.Println("\nEntering interactive chat mode to validate your workflow...")
		fmt.Println("Type /quit to exit.")
		EnterChatMode()
	}
	return nil
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
	return "", exitErr(1)
}

// printCreateSuggestions prints actionable next steps when keyword matching
// fails and no LLM is configured.
func printCreateSuggestions(description string) {
	fmt.Println("未找到完全匹配的模板。你可能需要：")
	fmt.Println()

	// 1. 用自然语言描述生成 (需要 LLM)。
	fmt.Println("  1. 用自然语言描述生成（需要配置 LLM）：")
	fmt.Printf("     aflare create \"%s\" --ai\n", description)
	fmt.Println("     未配置 LLM？运行：aflare init")
	fmt.Println()

	// 2. 手动创建。
	fmt.Println("  2. 手动创建：")
	fmt.Println("     参考 examples/ 目录中的示例，手写工作流 YAML 后用 aflare run 运行")
}

// EnterChatMode starts an interactive chat session for workflow validation.
// This is called from create --interactive.
func EnterChatMode() {
	cfg := agent.DefaultConfig()
	session := agent.NewChatSession(cfg)
	session.Run()
}
