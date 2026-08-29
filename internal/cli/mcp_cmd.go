// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​‌‌‌‌​‌‌​‌‌​‌​‌‌​‌‌​​​‌​‌‌‌​‌​​‌‌​​​‌‌​​​‌​‌‌​​​​​​​​​​​​​​​​​‌​‌​‌​‌‌‌​‌​‌​​‌⁠
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
	"net"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/alib8b8/aflare/internal/mcp"
)

// HandleMCP starts the MCP JSON-RPC server over stdio.
func HandleMCP() error {
	server := mcp.NewServer()
	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		return exitErr(1)
	}
	return nil
}

// HandleMCPCommand dispatches `aflare mcp` subcommands. With no arguments it
// starts the stdio MCP server, exactly matching the historical bare
// `aflare mcp` / `--mcp-server` behavior. HTTP mode is selected by transport
// flags: `aflare mcp --port 8082 [--host 127.0.0.1] [--token <token>]`.
func HandleMCPCommand(args []string) error {
	if len(args) == 0 {
		return HandleMCP()
	}

	// Transport flags select HTTP mode before subcommand dispatch, so
	// `aflare mcp --port 8082` works without a `serve` verb.
	if port, host, token, rest, err := parseMCPTransportFlags(args); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		return exitErr(1)
	} else if port != "" {
		return handleMCPServeHTTP(port, host, token, rest)
	}

	switch args[0] {
	case "install":
		return handleMCPInstall(args[1:])
	case "list":
		return handleMCPList()
	case "help", "-h", "--help":
		fmt.Print(mcpHelpText())
		return nil
	default:
		fmt.Fprintf(os.Stderr, "❌ 未知的 mcp 子命令：%s\n", args[0])
		if suggestions := suggestSubcommand(args[0], []string{"install", "list", "serve"}); len(suggestions) > 0 {
			fmt.Fprintf(os.Stderr, "你是不是想输入：%s\n", strings.Join(suggestions, ", "))
		}
		fmt.Fprint(os.Stderr, mcpHelpText())
		return exitErr(1)
	}
}

// handleMCPServeHTTP starts the MCP server in HTTP mode. A token is required
// (flag or AFLARE_MCP_TOKEN): an HTTP listener is a network surface, unlike
// loopback stdio.
func handleMCPServeHTTP(port, host, token string, rest []string) error {
	if len(rest) > 0 {
		fmt.Fprintf(os.Stderr, "❌ HTTP 模式不支持子命令：%s\n", strings.Join(rest, " "))
		return exitErr(1)
	}
	if token == "" {
		token = os.Getenv("AFLARE_MCP_TOKEN")
	}
	if token == "" {
		fmt.Fprint(os.Stderr, "❌ HTTP 模式必须提供 token：--token <token> 或环境变量 AFLARE_MCP_TOKEN\n")
		return exitErr(1)
	}
	if host == "" {
		// Default to loopback: an empty host in JoinHostPort listens on all
		// interfaces, which would silently expose the MCP server to the
		// network. Opting into a wider bind must be explicit (--host 0.0.0.0).
		host = "127.0.0.1"
	}
	addr := net.JoinHostPort(host, port)
	fmt.Printf("🚀 MCP HTTP server listening on http://%s\n", addr)
	fmt.Printf("   端点：POST /mcp（JSON-RPC 2.0）、POST /v1/call（简化调用）\n")
	fmt.Printf("   认证：请求头 %s\n", "X-MCP-Token")
	if err := mcp.ServeHTTPMode(addr, token); err != nil {
		fmt.Fprintf(os.Stderr, "❌ MCP HTTP server exited: %v\n", err)
		return exitErr(1)
	}
	return nil
}

// parseMCPTransportFlags scans args for HTTP-mode flags. It returns
// (port, host, token, remainingArgs, error). port is empty when no transport
// flag is present (stdio mode). Unknown flags are left in rest for the
// subcommand dispatch to reject.
func parseMCPTransportFlags(args []string) (port, host, token string, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--port" || arg == "--host" || arg == "--token":
			if i+1 >= len(args) {
				return "", "", "", nil, fmt.Errorf("%s 需要一个值", arg)
			}
			value := args[i+1]
			i++
			switch arg {
			case "--port":
				if _, perr := strconv.Atoi(value); perr != nil {
					return "", "", "", nil, fmt.Errorf("--port 必须是端口号，收到 %q", value)
				}
				port = value
			case "--host":
				host = value
			case "--token":
				token = value
			}
		case strings.HasPrefix(arg, "--port="):
			value := strings.TrimPrefix(arg, "--port=")
			if _, perr := strconv.Atoi(value); perr != nil {
				return "", "", "", nil, fmt.Errorf("--port 必须是端口号，收到 %q", value)
			}
			port = value
		case strings.HasPrefix(arg, "--host="):
			host = strings.TrimPrefix(arg, "--host=")
		case strings.HasPrefix(arg, "--token="):
			token = strings.TrimPrefix(arg, "--token=")
		default:
			rest = append(rest, arg)
		}
	}
	return port, host, token, rest, nil
}

// defaultMCPConfigPath returns the project-level .mcp.json in the current
// working directory — the same file mainstream MCP clients (Claude Code /
// Cursor / opencode) read, and the schema already used by this repo's own
// root .mcp.json.
func defaultMCPConfigPath() string {
	return ".mcp.json"
}

func handleMCPInstall(args []string) error {
	if len(args) != 1 {
		fmt.Fprint(os.Stderr, "用法：aflare mcp install <name>\n")
		fmt.Fprintf(os.Stderr, "可用名称见 aflare mcp list\n")
		return exitErr(1)
	}
	name := args[0]
	created, err := installMCPServer(name, defaultMCPConfigPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		return exitErr(1)
	}
	if !created {
		fmt.Printf("ℹ️  %s 已存在于 %s，无需重复安装\n", name, defaultMCPConfigPath())
		return nil
	}
	entry, _ := mcp.LookupCatalog(name)
	fmt.Printf("✅ 已安装 MCP server：%s\n", entry.Name)
	fmt.Printf("   说明：%s\n", entry.Description)
	fmt.Printf("   启动命令：%s %s\n", entry.Command, strings.Join(entry.Args, " "))
	fmt.Printf("   配置文件：%s\n", defaultMCPConfigPath())
	// The config is written regardless; warn that the runtime the entry needs
	// is missing so the failure surfaces at install time, not at first launch.
	if _, ok := detectCommand(entry.Command); !ok {
		if entry.Command == "npx" {
			fmt.Fprintf(os.Stderr, "⚠️  未检测到 npx（Node.js），MCP client 实际启动该 server 时会失败。安装：https://nodejs.org\n")
		} else {
			fmt.Fprintf(os.Stderr, "⚠️  未检测到 %s，MCP client 实际启动该 server 时会失败。请先安装 %s\n", entry.Command, entry.Command)
		}
	}
	return nil
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

func handleMCPList() error {
	if err := renderMCPList(defaultMCPConfigPath(), os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		return exitErr(1)
	}
	return nil
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
	b.WriteString("  aflare mcp --port 8082        以 HTTP 模式启动（需 --token 或 AFLARE_MCP_TOKEN）\n")
	b.WriteString("                                HTTP 选项：--host <host>（默认 127.0.0.1）、--port、--token\n")
	b.WriteString("  aflare mcp install <name>     安装内置 MCP server 到当前目录 .mcp.json\n")
	b.WriteString("  aflare mcp list               列出内置 MCP server 及安装状态\n")
	b.WriteString("  aflare mcp help               显示本帮助\n\n")
	b.WriteString("说明：\n")
	b.WriteString("  - HTTP 模式端点：POST /mcp（JSON-RPC 2.0）、POST /v1/call（简化调用 {\"name\",\"arguments\"}）\n")
	b.WriteString("  - HTTP 模式认证：请求头 X-MCP-Token，token 为必填（HTTP 是网络暴露面，不支持免认证）\n")
	b.WriteString("  - install 幂等：重复安装会提示已存在，不会覆盖 .mcp.json 中的本地修改\n")
	b.WriteString("  - 内置 server 除 fetch（官方 Python 实现，需 uvx）外均经 npx -y 启动（需 Node.js）\n")
	b.WriteString("  - 可用名称见 aflare mcp list；信创专用连接器见 mcp/xinchuang/README.md（规划中）\n")
	return b.String()
}
