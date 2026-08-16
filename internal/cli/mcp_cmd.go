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
	"io"
	"os"
	"sort"
	"strings"

	"github.com/alib8b8/aflare/internal/mcp"
)

// HandleMCP starts the MCP JSON-RPC server over stdio.
func HandleMCP() {
	server := mcp.NewServer()
	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}

// HandleMCPCommand dispatches `aflare mcp` subcommands. With no arguments it
// starts the stdio MCP server, exactly matching the historical bare
// `aflare mcp` / `--mcp-server` behavior.
func HandleMCPCommand(args []string) {
	if len(args) == 0 {
		HandleMCP()
		return
	}
	switch args[0] {
	case "install":
		handleMCPInstall(args[1:])
	case "list":
		handleMCPList()
	case "help", "-h", "--help":
		fmt.Print(mcpHelpText())
	default:
		fmt.Fprintf(os.Stderr, "❌ 未知的 mcp 子命令：%s\n", args[0])
		if suggestions := suggestSubcommand(args[0], []string{"install", "list"}); len(suggestions) > 0 {
			fmt.Fprintf(os.Stderr, "你是不是想输入：%s\n", strings.Join(suggestions, ", "))
		}
		fmt.Fprint(os.Stderr, mcpHelpText())
		os.Exit(1)
	}
}

// defaultMCPConfigPath returns the project-level .mcp.json in the current
// working directory — the same file mainstream MCP clients (Claude Code /
// Cursor / opencode) read, and the schema already used by this repo's own
// root .mcp.json.
func defaultMCPConfigPath() string {
	return ".mcp.json"
}

func handleMCPInstall(args []string) {
	if len(args) != 1 {
		fmt.Fprint(os.Stderr, "用法：aflare mcp install <name>\n")
		fmt.Fprintf(os.Stderr, "可用名称见 aflare mcp list\n")
		os.Exit(1)
	}
	name := args[0]
	created, err := installMCPServer(name, defaultMCPConfigPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
	if !created {
		fmt.Printf("ℹ️  %s 已存在于 %s，无需重复安装\n", name, defaultMCPConfigPath())
		return
	}
	entry, _ := mcp.LookupCatalog(name)
	fmt.Printf("✅ 已安装 MCP server：%s\n", entry.Name)
	fmt.Printf("   说明：%s\n", entry.Description)
	fmt.Printf("   启动命令：%s %s\n", entry.Command, strings.Join(entry.Args, " "))
	fmt.Printf("   配置文件：%s\n", defaultMCPConfigPath())
}

// installMCPServer resolves a catalog name and upserts it into the config.
// It returns a user-actionable error for unknown names (with a
// did-you-mean hint) or config I/O failures.
func installMCPServer(name, configPath string) (created bool, err error) {
	entry, ok := mcp.LookupCatalog(name)
	if !ok {
		msg := fmt.Sprintf("未知的 MCP server：%s", name)
		if suggestions := suggestSubcommand(name, mcp.CatalogNames()); len(suggestions) > 0 {
			msg += fmt.Sprintf("（你是不是想输入：%s）", strings.Join(suggestions, ", "))
		}
		msg += "，运行 aflare mcp list 查看可用清单"
		return false, fmt.Errorf("%s", msg)
	}
	return mcp.UpsertMCPServer(configPath, entry.Name, mcp.EntryFromCatalog(entry))
}

func handleMCPList() {
	if err := renderMCPList(defaultMCPConfigPath(), os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
}

// renderMCPList writes the built-in catalog with installed markers to w.
// Installed state is read from the actual config file, and any configured
// servers outside the catalog are listed as custom entries.
func renderMCPList(configPath string, w io.Writer) error {
	cfg, err := mcp.LoadMCPConfig(configPath)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "内置 MCP server 目录（配置文件：%s）\n\n", configPath)
	for _, entry := range mcp.CatalogEntries() {
		mark := "⬜ 未安装"
		if _, installed := cfg.MCPServers[entry.Name]; installed {
			mark = "✅ 已安装"
		}
		fmt.Fprintf(w, "  %s  %-20s %s\n", mark, entry.Name, entry.Description)
	}

	var custom []string
	for name := range cfg.MCPServers {
		if _, ok := mcp.LookupCatalog(name); !ok {
			custom = append(custom, name)
		}
	}
	if len(custom) > 0 {
		sort.Strings(custom)
		fmt.Fprintf(w, "\n配置文件中的其他 server（手动添加）：\n")
		for _, name := range custom {
			fmt.Fprintf(w, "  ✅ 已安装  %s\n", name)
		}
	}

	fmt.Fprintf(w, "\n安装：aflare mcp install <name>\n")
	fmt.Fprintf(w, "说明：除 fetch（官方 Python 实现，需 uvx）外，其余 server 经 npx -y 启动（需 Node.js）\n")
	return nil
}

// suggestSubcommand returns "did-you-mean" candidates for an unknown name,
// mirroring SuggestCommand: prefix matches first, then edit distance <= 2.
func suggestSubcommand(name string, candidates []string) []string {
	if name == "" {
		return nil
	}
	lower := strings.ToLower(name)

	var prefixMatches []string
	for _, c := range candidates {
		if strings.HasPrefix(c, lower) {
			prefixMatches = append(prefixMatches, c)
		}
	}
	if len(prefixMatches) > 0 {
		return trimSuggestions(prefixMatches, 3)
	}

	var typoMatches []string
	for _, c := range candidates {
		if levenshtein(lower, c) <= 2 {
			typoMatches = append(typoMatches, c)
		}
	}
	return trimSuggestions(typoMatches, 3)
}

// mcpHelpText returns the `aflare mcp --help` text (user-facing, Chinese).
func mcpHelpText() string {
	var b strings.Builder
	b.WriteString("aflare mcp — MCP Server 模式与内置 server 安装\n\n")
	b.WriteString("用法：\n")
	b.WriteString("  aflare mcp                    以 stdio MCP Server 模式启动（等价 --mcp-server）\n")
	b.WriteString("  aflare mcp install <name>     安装内置 MCP server 到当前目录 .mcp.json\n")
	b.WriteString("  aflare mcp list               列出内置 MCP server 及安装状态\n")
	b.WriteString("  aflare mcp help               显示本帮助\n\n")
	b.WriteString("说明：\n")
	b.WriteString("  - install 幂等：重复安装会提示已存在，不会覆盖 .mcp.json 中的本地修改\n")
	b.WriteString("  - 内置 server 除 fetch（官方 Python 实现，需 uvx）外均经 npx -y 启动（需 Node.js）\n")
	b.WriteString("  - 可用名称见 aflare mcp list；信创专用连接器见 mcp/xinchuang/README.md（规划中）\n")
	return b.String()
}
