// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌​‌‌​​‌‌​​‌​​‌​‌​‌​‌‌‌‌​​‌​‌‌​‌‌‌‌​‌​​​​‌‌‌​‌​​​​​​​​​​​​​​​​​​​​​​‌​​‌‌​‌​‌‌​‌⁠
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
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallMCPServerUnknownNameSuggests(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".mcp.json")
	_, err := installMCPServer("fetsh", path)
	if err == nil {
		t.Fatal("expected error for unknown server name")
	}
	if !strings.Contains(err.Error(), "fetsh") {
		t.Errorf("error should echo the unknown name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "fetch") {
		t.Errorf("error should contain a did-you-mean hint for fetch, got: %v", err)
	}
	if !strings.Contains(err.Error(), "aflare mcp list") {
		t.Errorf("error should point to aflare mcp list, got: %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("failed install must not create the config file")
	}
}

func TestInstallMCPServerIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".mcp.json")

	created, err := installMCPServer("filesystem", path)
	if err != nil || !created {
		t.Fatalf("first install: created=%v err=%v", created, err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	created, err = installMCPServer("filesystem", path)
	if err != nil {
		t.Fatalf("repeat install: %v", err)
	}
	if created {
		t.Error("repeat install should report created=false")
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Error("repeat install must not modify the config file")
	}

	// Case-insensitive install of the same server is still idempotent.
	created, err = installMCPServer("Filesystem", path)
	if err != nil || created {
		t.Errorf("case-insensitive repeat: created=%v err=%v", created, err)
	}
}

func TestRenderMCPList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mcp.json")
	existing := `{"mcpServers":{"git":{"type":"stdio","command":"npx","args":["-y","x"]},"my-own":{"command":"my-server"}}}`
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := renderMCPList(path, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, "已安装") || !strings.Contains(out, "未安装") {
		t.Errorf("list output should mark installed/uninstalled, got:\n%s", out)
	}
	if !strings.Contains(out, "fetch") || !strings.Contains(out, "sequential-thinking") {
		t.Errorf("list output should contain all catalog names, got:\n%s", out)
	}
	if strings.Count(out, "git") < 1 {
		t.Error("list output should mention git as installed")
	}
	if !strings.Contains(out, "my-own") {
		t.Error("list output should list custom (non-catalog) servers")
	}
	if !strings.Contains(out, "aflare mcp install") {
		t.Error("list output should include the install hint")
	}
}

func TestSuggestSubcommandPrefix(t *testing.T) {
	got := suggestSubcommand("seq", []string{"fetch", "sequential-thinking"})
	if len(got) != 1 || got[0] != "sequential-thinking" {
		t.Errorf("expected prefix match sequential-thinking, got %v", got)
	}
}

func TestMCPHelpText(t *testing.T) {
	help := mcpHelpText()
	for _, want := range []string{"aflare mcp install", "aflare mcp list", ".mcp.json", "uvx", "npx"} {
		if !strings.Contains(help, want) {
			t.Errorf("help text should mention %q, got:\n%s", want, help)
		}
	}
}
