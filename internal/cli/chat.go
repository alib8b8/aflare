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
	"strconv"
	"strings"

	"github.com/alib8b8/aflare/internal/agent"
	"github.com/alib8b8/aflare/internal/i18n"
)

// HandleChat handles the "chat" command — interactive agent REPL.
func HandleChat(args []string) {
	cfg := agent.DefaultConfig()

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider", "-p":
			if i+1 < len(args) {
				cfg.Provider = args[i+1]
				i++
			}
		case "--model", "-m":
			if i+1 < len(args) {
				cfg.Model = args[i+1]
				i++
			}
		case "--api-key", "-k":
			if i+1 < len(args) {
				cfg.APIKey = args[i+1]
				i++
			}
		case "--endpoint", "-e":
			if i+1 < len(args) {
				cfg.Endpoint = args[i+1]
				i++
			}
		case "--tools", "-t":
			if i+1 < len(args) {
				cfg.Tools = parseToolsArg(args[i+1])
				i++
			}
		case "--capabilities", "-c":
			if i+1 < len(args) {
				cfg.Capabilities = agent.ParseCapabilities(args[i+1])
				i++
			}
		case "--max-iterations", "-n":
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil && n > 0 {
					cfg.MaxIterations = n
				}
				i++
			}
		case "--safe-mode", "-s":
			cfg.SafeMode = true
		case "--help", "-h":
			PrintChatUsage()
			return
		default:
			fmt.Printf("Unknown argument: %s\n", args[i])
			PrintChatUsage()
			os.Exit(1)
		}
	}

	session := agent.NewChatSession(cfg)
	session.Run()
}

// parseToolsArg parses a comma-separated tool list, adding the chat default
// tools as a base. Special value "all" enables all available tools.
func parseToolsArg(raw string) []string {
	if raw == "all" {
		return []string{
			"fetch_url", "http_request", "file_read", "file_write",
			"json_parse", "transform", "combine", "template",
			"code_interpreter", "execute",
		}
	}
	return strings.Split(raw, ",")
}

// PrintChatUsage prints help for the chat command.
func PrintChatUsage() {
	fmt.Println(i18n.T("usage.chat"))
	fmt.Println()
	fmt.Println("Usage: aflare chat [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --provider, -p <name>     LLM provider (default: ollama)")
	fmt.Println("  --model, -m <name>        Model name (default: llama3)")
	fmt.Println("  --api-key, -k <key>       API key for cloud providers")
	fmt.Println("  --endpoint, -e <url>      Custom API endpoint")
	fmt.Println("  --tools, -t <list>        Comma-separated tool names, or 'all'")
	fmt.Println("  --capabilities, -c <list> Comma-separated capability names, or 'all'")
	fmt.Println("  --max-iterations, -n <n>  Max agent iterations per turn (default: 10)")
	fmt.Println("  --safe-mode, -s            Block execute and destructive tools")
	fmt.Println("  --help, -h                 Show this help")
	fmt.Println()
	fmt.Println("Capabilities (--capabilities):")
	fmt.Println("  reflection     Self-reflection and self-correction")
	fmt.Println("  human-in-loop  Pause at critical decisions for human approval")
	fmt.Println("  bdi            Belief-Desire-Intention goal management")
	fmt.Println("  utility        Utility-driven optimization of decisions")
	fmt.Println("  adaptive, memory, planning, multi-agent, workflow, simulation")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  aflare chat                                    # local ollama (default)")
	fmt.Println("  aflare chat -p deepseek -m deepseek-chat       # DeepSeek")
	fmt.Println("  aflare chat -p openai -k $OPENAI_API_KEY       # OpenAI")
	fmt.Println("  aflare chat -t fetch_url,file_read,execute     # custom tools")
	fmt.Println("  aflare chat -c reflection,bdi,utility          # with capabilities")
	fmt.Println("  aflare chat -s                                 # safe mode")
	fmt.Println()
	fmt.Println("Chat commands:")
	fmt.Println("  /help          Show commands")
	fmt.Println("  /tools         List available tools")
	fmt.Println("  /capabilities  List active capabilities")
	fmt.Println("  /history       Show conversation state")
	fmt.Println("  /clear         Clear conversation history")
	fmt.Println("  /exit          Exit chat")
}