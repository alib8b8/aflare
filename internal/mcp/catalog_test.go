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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogEntries(t *testing.T) {
	entries := CatalogEntries()
	if len(entries) != 8 {
		t.Fatalf("expected 8 catalog entries, got %d", len(entries))
	}
	wantOrder := []string{
		"fetch", "filesystem", "git", "memory", "sqlite",
		"sequential-thinking", "everything", "time",
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

	entry, _ := LookupCatalog("git")
	if _, err := UpsertMCPServer(path, "git", EntryFromCatalog(entry)); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadMCPConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"aflare", "custom", "git"} {
		if _, ok := cfg.MCPServers[name]; !ok {
			t.Errorf("server %q lost after upsert", name)
		}
	}
	if got := cfg.MCPServers["aflare"].Args; len(got) != 1 || got[0] != "--mcp-server" {
		t.Errorf("existing aflare entry was modified: %v", got)
	}
	if got := cfg.MCPServers["git"]; got.Type != "stdio" || got.Command != "npx" {
		t.Errorf("unexpected git entry: %+v", got)
	}
}

func TestSaveFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".mcp.json")
	cfg := &MCPServersFile{MCPServers: map[string]ServerEntry{
		"time": {Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-time"}},
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
