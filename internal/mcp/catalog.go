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

package mcp

import (
	"encoding/json"
	"fmt"
	"os"
)

// CatalogEntry describes a community MCP server that `aflare mcp install`
// can write into a project-level .mcp.json file. Commands follow the
// official reference-server naming: npm-based servers start via
// `npx -y <pkg>`; the Python-based fetch server starts via
// `uvx mcp-server-fetch`.
type CatalogEntry struct {
	Name        string
	Description string
	Command     string
	Args        []string
	Env         map[string]string
}

// catalogOrder keeps `aflare mcp list` output stable regardless of map
// iteration order.
var catalogOrder = []string{
	"fetch",
	"filesystem",
	"git",
	"memory",
	"sqlite",
	"sequential-thinking",
	"everything",
	"time",
}

// builtinCatalog is the built-in directory of community MCP servers
// supported by `aflare mcp install`.
var builtinCatalog = map[string]CatalogEntry{
	"fetch": {
		Name:        "fetch",
		Description: "抓取网页并转为 Markdown 供模型阅读（官方 Python 实现，经 uvx 启动）",
		Command:     "uvx",
		Args:        []string{"mcp-server-fetch"},
	},
	"filesystem": {
		Name:        "filesystem",
		Description: "受控读写本地文件系统（默认开放当前目录，可在 .mcp.json 中改为指定路径）",
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-filesystem", "."},
	},
	"git": {
		Name:        "git",
		Description: "查询 Git 仓库状态、diff 与提交历史",
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-git"},
	},
	"memory": {
		Name:        "memory",
		Description: "基于知识图谱的持久化记忆存储",
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-memory"},
	},
	"sqlite": {
		Name:        "sqlite",
		Description: "查询与修改 SQLite 数据库（默认库文件 ./data.db，可在 .mcp.json 中调整）",
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-sqlite", "--db-path", "./data.db"},
	},
	"sequential-thinking": {
		Name:        "sequential-thinking",
		Description: "结构化分步推理（思维链）工具",
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-sequential-thinking"},
	},
	"everything": {
		Name:        "everything",
		Description: "MCP 协议测试沙盒，覆盖全部协议特性（prompts/resources/tools）",
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-everything"},
	},
	"time": {
		Name:        "time",
		Description: "获取当前时间与格式化时区转换",
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-time"},
	},
}

// CatalogEntries returns the built-in catalog in stable display order.
func CatalogEntries() []CatalogEntry {
	entries := make([]CatalogEntry, 0, len(catalogOrder))
	for _, name := range catalogOrder {
		if e, ok := builtinCatalog[name]; ok {
			entries = append(entries, e)
		}
	}
	return entries
}

// CatalogNames returns the names of all built-in catalog servers.
func CatalogNames() []string {
	names := make([]string, 0, len(catalogOrder))
	for _, name := range catalogOrder {
		if _, ok := builtinCatalog[name]; ok {
			names = append(names, name)
		}
	}
	return names
}

// LookupCatalog finds a built-in catalog entry by name (exact match first,
// then case-insensitive).
func LookupCatalog(name string) (CatalogEntry, bool) {
	if e, ok := builtinCatalog[name]; ok {
		return e, true
	}
	for _, key := range catalogOrder {
		if equalFoldASCII(name, key) {
			return builtinCatalog[key], true
		}
	}
	return CatalogEntry{}, false
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// ServerEntry is one `mcpServers` entry in a .mcp.json file, following the
// schema used by mainstream MCP clients (Claude Code / Cursor / opencode):
// {"type":"stdio","command":...,"args":[...],"env":{...},"cwd":...}.
type ServerEntry struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	// Cwd is the working directory for the spawned stdio server process
	// (Agent Plugins 1.0 passes plugin-relative cwd values here). Optional;
	// empty inherits the client's working directory.
	Cwd string `json:"cwd,omitempty"`
}

// MCPServersFile mirrors the .mcp.json document shape.
type MCPServersFile struct {
	MCPServers map[string]ServerEntry `json:"mcpServers"`
}

// LoadMCPConfig reads a .mcp.json file. A missing file yields an empty
// config (not an error), so `aflare mcp list` works in fresh projects.
func LoadMCPConfig(path string) (*MCPServersFile, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from the user's project dir
	if err != nil {
		if os.IsNotExist(err) {
			return &MCPServersFile{MCPServers: make(map[string]ServerEntry)}, nil
		}
		return nil, fmt.Errorf("failed to read MCP config %s: %w", path, err)
	}
	cfg := &MCPServersFile{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse MCP config %s: %w", path, err)
	}
	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]ServerEntry)
	}
	return cfg, nil
}

// Save writes the config back with stable 2-space indentation.
func (c *MCPServersFile) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal MCP config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write MCP config %s: %w", path, err)
	}
	return nil
}

// UpsertMCPServer inserts the named server into the .mcp.json at path.
// It is idempotent: if the name already exists the file is left untouched
// and created=false is returned, so a repeated install never clobbers a
// user's local edits.
func UpsertMCPServer(path, name string, entry ServerEntry) (created bool, err error) {
	cfg, err := LoadMCPConfig(path)
	if err != nil {
		return false, err
	}
	if _, exists := cfg.MCPServers[name]; exists {
		return false, nil
	}
	cfg.MCPServers[name] = entry
	if err := cfg.Save(path); err != nil {
		return false, err
	}
	return true, nil
}

// EntryFromCatalog converts a catalog entry into a .mcp.json server entry.
func EntryFromCatalog(e CatalogEntry) ServerEntry {
	return ServerEntry{
		Type:    "stdio",
		Command: e.Command,
		Args:    e.Args,
		Env:     e.Env,
	}
}
