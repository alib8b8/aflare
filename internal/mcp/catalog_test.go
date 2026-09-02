// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌​​​‌‌‌‌‌​‌​‌‌‌​​​​‌‌​‌​​​​‌​​​‌​‌​​​​​‌​​​‌‌‌​​​​​​​​​​​​​​​​‌‌‌‌‌​​​‌​​​​​‌‌⁠
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
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCatalogEntries(t *testing.T) {
	entries := CatalogEntries()
	if len(entries) != 5 {
		t.Fatalf("expected 5 catalog entries, got %d", len(entries))
	}
	wantOrder := []string{
		"fetch", "filesystem", "memory",
		"sequential-thinking", "everything",
	}
	for i, e := range entries {
		if e.Name != wantOrder[i] {
			t.Errorf("entry %d: expected %q, got %q", i, wantOrder[i], e.Name)
		}
		if e.Command == "" || len(e.Args) == 0 || e.Description == "" {
			t.Errorf("entry %q has empty command/args/description", e.Name)
		}
	}
}

// TestCatalogEntriesPinned is the supply-chain nail for the MCP catalog:
// every entry must pin an exact registry version. An unpinned
// `npx -y <pkg>` / `uvx <pkg>` resolves to whatever npm/PyPI serve on the
// day of install — the catalog is aflare's only distribution surface for
// third-party code, and a workflow engine that sells auditable,
// deterministic execution cannot ship floating dependencies. It also
// blocks the three npm-unpublished packages (git/sqlite/time, 404 since
// 2026-09) from sneaking back in via a stale doc or memory.
func TestCatalogEntriesPinned(t *testing.T) {
	pinRe := regexp.MustCompile(`@\d+\.\d+\.\d+$`)
	dead := map[string]bool{
		"@modelcontextprotocol/server-git":    true, // npm 404 since 2026-09
		"@modelcontextprotocol/server-sqlite": true,
		"@modelcontextprotocol/server-time":   true,
	}
	for _, e := range CatalogEntries() {
		pinned := false
		for _, arg := range e.Args {
			if dead[pinRe.ReplaceAllString(arg, "")] {
				t.Errorf("%s: package %q is unpublished from npm (404) — removed from the catalog 2026-09, do not re-add", e.Name, arg)
			}
			if pinRe.MatchString(arg) {
				pinned = true
			}
		}
		if !pinned {
			t.Errorf("%s: no pinned package arg in %v — pin the exact registry version (e.g. <pkg>@2026.8.31); floating installs are unauditable", e.Name, e.Args)
		}
	}
}

func TestLookupCatalog(t *testing.T) {
	if e, ok := LookupCatalog("fetch"); !ok || e.Command != "uvx" {
		t.Errorf("expected fetch via uvx, got %+v (ok=%v)", e, ok)
	}
	if _, ok := LookupCatalog("Fetch"); !ok {
		t.Error("expected case-insensitive match for Fetch")
	}
	if _, ok := LookupCatalog("no-such-server"); ok {
		t.Error("expected miss for no-such-server")
	}
}

func TestLoadMCPConfigMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".mcp.json")
	cfg, err := LoadMCPConfig(path)
	if err != nil {
		t.Fatalf("missing config should yield empty config, got error: %v", err)
	}
	if cfg.MCPServers == nil || len(cfg.MCPServers) != 0 {
		t.Errorf("expected empty non-nil MCPServers map, got %#v", cfg.MCPServers)
	}
}

func TestLoadMCPConfigInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".mcp.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMCPConfig(path); err == nil {
		t.Error("expected parse error for invalid JSON")
	}
}

func TestUpsertMCPServerIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".mcp.json")
	entry := EntryFromCatalog(CatalogEntries()[0]) // fetch

	created, err := UpsertMCPServer(path, "fetch", entry)
	if err != nil || !created {
		t.Fatalf("first install: created=%v err=%v", created, err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	created, err = UpsertMCPServer(path, "fetch", entry)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if created {
		t.Error("second install should be a no-op (created=false)")
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Error("idempotent install must not rewrite the config file")
	}
}

func TestUpsertMCPServerPreservesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".mcp.json")
	existing := `{"mcpServers":{"aflare":{"type":"stdio","command":"aflare","args":["--mcp-server"]},"custom":{"command":"my-server"}}}`
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	entry, _ := LookupCatalog("memory")
	if _, err := UpsertMCPServer(path, "memory", EntryFromCatalog(entry)); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadMCPConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"aflare", "custom", "memory"} {
		if _, ok := cfg.MCPServers[name]; !ok {
			t.Errorf("server %q lost after upsert", name)
		}
	}
	if got := cfg.MCPServers["aflare"].Args; len(got) != 1 || got[0] != "--mcp-server" {
		t.Errorf("existing aflare entry was modified: %v", got)
	}
	if got := cfg.MCPServers["memory"]; got.Type != "stdio" || got.Command != "npx" {
		t.Errorf("unexpected memory entry: %+v", got)
	}
}

func TestSaveFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".mcp.json")
	cfg := &MCPServersFile{MCPServers: map[string]ServerEntry{
		"memory": {Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-memory@2026.8.31"}},
	}}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.HasPrefix(s, "{\n  \"mcpServers\"") {
		t.Errorf("unexpected file layout: %q", s)
	}
	if !strings.HasSuffix(s, "}\n") {
		t.Error("config file should end with a newline")
	}
}
