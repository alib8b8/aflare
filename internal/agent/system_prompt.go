// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌‌‌‌‌​‌‌​​​​‌‌‌​‌​​​​‌​‌‌​‌‌‌‌‌​‌‌​‌‌​‌‌​​‌​​​​​​​​​​​​​​​​​​​​​‌‌‌‌‌​​​​‌‌‌​​⁠
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

package agent

import (
	"fmt"
	"strings"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

// SystemPrompt is the default system prompt for the aflare chat agent.
// It defines the agent's persona, tool usage rules, and behavior guidelines.
const SystemPrompt = `You are aflare, a local-first automation agent running entirely on the user's machine.

You have 300+ pre-built skills (workflow templates) across 17 domains.
Each skill is a ready-to-run workflow. Your job is to match the user's request
to the right skill and execute it.

Your skills cover these domains:
  business, content-creative, data-ai, devops-infra, ecommerce, education,
  finance, healthcare, hr, integrations, iot, legal, lifestyle, marketing,
  software-engineering, supply-chain

When the user gives you a goal:
1. Search for a matching skill with template_list — use keywords from the user's request
2. If found, inspect it with template_info to understand the parameters
3. Run it with run_workflow, passing the template name (e.g. "stock-screener")
4. If no skill matches, compose a new workflow with create_workflow
5. Report results clearly and concisely
6. If something fails, explain what went wrong and suggest alternatives

Safety rules:
- Confirm before executing destructive actions (file delete, system commands)
- All execution stays local — no data leaves the machine unless explicitly configured
- Be transparent about what you're doing at each step
- If unsure, ask the user before proceeding

Keep responses concise and actionable. Focus on getting things done.`

// BuildSystemPrompt constructs the full system prompt with tool descriptions
// and version information. It combines the static persona with a dynamic tool list.
func BuildSystemPrompt(tools []core.AgentTool, version string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("You are aflare v%s, a local-first automation agent.\n\n", version))
	sb.WriteString(SystemPrompt)
	sb.WriteString("\n\nYou have access to these tools:\n")

	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
	}

	sb.WriteString("\nAvailable nodes for composing workflows:\n")
	sb.WriteString("- llm: Call any LLM (ollama, openai, deepseek, qwen, etc.)\n")
	sb.WriteString("- http_request: Make HTTP requests to APIs\n")
	sb.WriteString("- file_read / file_write: Read and write files\n")
	sb.WriteString("- code_interpreter: Execute Python code in a sandbox\n")
	sb.WriteString("- fetch_url: Fetch web page content\n")
	sb.WriteString("- json_parse: Parse and extract fields from JSON\n")
	sb.WriteString("- template_render: Render Go templates with variables\n")
	sb.WriteString("- transform: Transform text (uppercase, lowercase, replace, regex)\n")
	sb.WriteString("- combine: Combine multiple inputs into one\n")
	sb.WriteString("- search_aggregate: Search across multiple platforms\n")
	sb.WriteString("- notify: Send notifications\n")
	sb.WriteString("- supervisor: Multi-agent coordination with specialists\n")
	sb.WriteString("- agent: Run a ReAct agent with tools\n")
	sb.WriteString("- knowledge_graph: Build and query knowledge graphs\n")

	return sb.String()
}

// WelcomeMessage returns the greeting shown when the chat session starts.
func WelcomeMessage(version string) string {
	return fmt.Sprintf(
		"aflare v%s — local-first automation agent\n"+
			"323+ templates | 100+ node types | ReAct agent\n"+
			"Type /help for commands, /exit to quit\n"+
			"Multi-line: end line with \\, then empty line to submit\n",
		version,
	)
}

// OnboardingMessage returns the first-session guidance shown to new users
// (断点15: 首次 chat 无引导). It gives concrete examples the user can try
// immediately, plus pointers to /help and /exit, so the blank-prompt
// confusion is eliminated.
func OnboardingMessage() string {
	return "aflare Agent 已启动。试试这些：\n" +
		"\n" +
		"  • 帮我生成一个监控 BTC 价格的工作流\n" +
		"  • 分析当前目录的 Go 代码质量\n" +
		"  • 每 5 分钟检查 https://example.com 是否可访问\n" +
		"\n" +
		"输入 /help 查看所有命令\n" +
		"输入 /exit 退出"
}
