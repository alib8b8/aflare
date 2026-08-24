// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌‌‌‌​‌‌‌​​​​‌‌​​‌​​‌‌​​‌‌​​​‌‌​‌‌‌‌​​​‌‌‌​‌‌‌​​​​​​​​​​​​​​​​​‌​​​​‌​‌​‌‌​​​‌​⁠
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

package marketplace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetEmptyName(t *testing.T) {
	reg := NewRegistry()
	if _, err := reg.Get(""); err == nil {
		t.Fatal("Get with empty name should return an error")
	}
}

func TestSearchEmptyKeyword(t *testing.T) {
	reg := NewRegistry()
	if got := reg.Search(""); got != nil {
		t.Errorf("Search with empty keyword = %v, want nil", got)
	}
}

func TestListByCategoryEmpty(t *testing.T) {
	reg := NewRegistry()
	if got := reg.ListByCategory(""); got != nil {
		t.Errorf("ListByCategory with empty category = %v, want nil", got)
	}
}

func TestListSortedByName(t *testing.T) {
	reg := NewRegistry()
	pkgs := reg.List()
	for i := 1; i < len(pkgs); i++ {
		if pkgs[i-1].Name > pkgs[i].Name {
			t.Fatalf("List() not sorted: %q > %q", pkgs[i-1].Name, pkgs[i].Name)
		}
	}
}

func TestCategories(t *testing.T) {
	reg := NewRegistry()
	cats := reg.Categories()
	want := []string{"devops", "finance", "health", "research"}
	if len(cats) != len(want) {
		t.Fatalf("Categories() = %v, want %v", cats, want)
	}
	for i := range want {
		if cats[i] != want[i] {
			t.Errorf("Categories()[%d] = %q, want %q", i, cats[i], want[i])
		}
	}
}

func TestInstallUninstallLifecycle(t *testing.T) {
	reg := NewRegistry()

	// Install writes into the (redirected) data dir workflows folder.
	path, err := reg.Install("btc-monitor")
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(path), "/workflows/btc-monitor.yaml") {
		t.Errorf("Install path = %q, want .../workflows/btc-monitor.yaml", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("installed file missing: %v", err)
	}

	// ListInstalled now reports it.
	installed, err := reg.ListInstalled()
	if err != nil {
		t.Fatalf("ListInstalled failed: %v", err)
	}
	found := false
	for _, n := range installed {
		if n == "btc-monitor" {
			found = true
		}
	}
	if !found {
		t.Errorf("ListInstalled() = %v, want btc-monitor present", installed)
	}

	// Uninstall removes it.
	if err := reg.Uninstall("btc-monitor"); err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("workflow file still present after uninstall: %v", err)
	}

	// Uninstalling again reports "not installed".
	if err := reg.Uninstall("btc-monitor"); err == nil {
		t.Fatal("Uninstall of a not-installed workflow should fail")
	}
}

func TestListInstalledEmptyDir(t *testing.T) {
	reg := &Registry{} // no builtins needed; the data dir workflows folder is empty
	installed, err := reg.ListInstalled()
	if err != nil {
		t.Fatalf("ListInstalled failed: %v", err)
	}
	if len(installed) != 0 {
		t.Errorf("ListInstalled on empty dir = %v, want empty", installed)
	}
}

func TestUninstallInvalidName(t *testing.T) {
	reg := NewRegistry()
	for _, name := range []string{"", "../escape", "has space", "a/b"} {
		if err := reg.Uninstall(name); err == nil {
			t.Errorf("Uninstall(%q) should reject invalid names", name)
		}
	}
}

func TestInstallToEmptyYAML(t *testing.T) {
	reg := &Registry{packages: []Package{{Name: "empty-pkg", Version: "1.0.0"}}}
	if _, err := reg.InstallTo("empty-pkg", t.TempDir()); err == nil {
		t.Fatal("InstallTo with empty WorkflowYAML should fail")
	}
}

func TestInstallToMkdirFailure(t *testing.T) {
	reg := NewRegistry()

	// targetDir occupies an existing file path, MkdirAll must fail.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := reg.InstallTo("btc-monitor", filepath.Join(blocker, "sub")); err == nil {
		t.Fatal("InstallTo under a file path should fail")
	}
}

func TestIsValidPackageName(t *testing.T) {
	valid := []string{"a", "btc-monitor", "pkg_2", "A-B_9", strings.Repeat("n", 100)}
	for _, name := range valid {
		if err := isValidPackageName(name); err != nil {
			t.Errorf("isValidPackageName(%q) = %v, want nil", name, err)
		}
	}
	invalid := []string{"", strings.Repeat("n", 101), "has space", "a/b", "a.b", "中文"}
	for _, name := range invalid {
		if err := isValidPackageName(name); err == nil {
			t.Errorf("isValidPackageName(%q) = nil, want error", name)
		}
	}
}

func TestToPluginManifest(t *testing.T) {
	pkg := &Package{
		Name:        "demo",
		Version:     "2.0.0",
		Description: "A demo workflow",
		Category:    "finance",
		Author:      "tester",
	}
	m := pkg.ToPluginManifest()
	if m.Schema != "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json" {
		t.Errorf("schema = %q", m.Schema)
	}
	if m.Name != "demo" || m.Version != "2.0.0" || m.Description != "A demo workflow" || m.Author != "tester" {
		t.Errorf("manifest fields not copied: %+v", m)
	}
	if m.License != "AGPL-3.0" {
		t.Errorf("license = %q, want AGPL-3.0", m.License)
	}
	wantKeywords := []string{"aflare", "workflow", "finance"}
	if len(m.Keywords) != len(wantKeywords) {
		t.Fatalf("keywords = %v, want %v", m.Keywords, wantKeywords)
	}
	for i := range wantKeywords {
		if m.Keywords[i] != wantKeywords[i] {
			t.Errorf("keywords[%d] = %q, want %q", i, m.Keywords[i], wantKeywords[i])
		}
	}
}

func TestExportPlugin(t *testing.T) {
	reg := NewRegistry()
	target := t.TempDir()

	pluginDir, err := reg.ExportPlugin("btc-monitor", target)
	if err != nil {
		t.Fatalf("ExportPlugin failed: %v", err)
	}
	if pluginDir != filepath.Join(target, "btc-monitor") {
		t.Errorf("plugin dir = %q, want %q", pluginDir, filepath.Join(target, "btc-monitor"))
	}

	// plugin.json parses and carries the Agent Plugins metadata.
	data, err := os.ReadFile(filepath.Join(pluginDir, "plugin.json"))
	if err != nil {
		t.Fatalf("plugin.json missing: %v", err)
	}
	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("plugin.json invalid JSON: %v", err)
	}
	if manifest.Name != "btc-monitor" {
		t.Errorf("manifest.Name = %q, want btc-monitor", manifest.Name)
	}
	if manifest.Schema == "" {
		t.Error("manifest.Schema must be set")
	}

	// skills/<name>/SKILL.md and mcp.json exist.
	for _, p := range []string{
		filepath.Join(pluginDir, "skills", "btc-monitor", "SKILL.md"),
		filepath.Join(pluginDir, "mcp.json"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s: %v", p, err)
		}
	}

	// mcp.json contains the aflare stdio server entry.
	mcpData, err := os.ReadFile(filepath.Join(pluginDir, "mcp.json"))
	if err != nil {
		t.Fatalf("mcp.json missing: %v", err)
	}
	var mcp map[string]any
	if err := json.Unmarshal(mcpData, &mcp); err != nil {
		t.Fatalf("mcp.json invalid JSON: %v", err)
	}
	servers, ok := mcp["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcp.json has no mcpServers map: %v", mcp)
	}
	aflare, ok := servers["aflare"].(map[string]any)
	if !ok {
		t.Fatalf("mcp.json has no aflare server: %v", servers)
	}
	if aflare["command"] != "aflare" {
		t.Errorf("mcp aflare command = %v, want aflare", aflare["command"])
	}
}

func TestExportPluginUnknown(t *testing.T) {
	reg := NewRegistry()
	if _, err := reg.ExportPlugin("no-such-package", t.TempDir()); err == nil {
		t.Fatal("ExportPlugin of unknown package should fail")
	}
}

func TestImportPluginRoundTrip(t *testing.T) {
	reg := NewRegistry()
	target := t.TempDir()

	pluginDir, err := reg.ExportPlugin("github-alert", target)
	if err != nil {
		t.Fatalf("ExportPlugin failed: %v", err)
	}

	manifest, err := ImportPlugin(pluginDir)
	if err != nil {
		t.Fatalf("ImportPlugin failed: %v", err)
	}
	if manifest.Name != "github-alert" {
		t.Errorf("manifest.Name = %q, want github-alert", manifest.Name)
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("manifest.Version = %q, want 1.0.0", manifest.Version)
	}
}

func TestImportPluginErrors(t *testing.T) {
	dir := t.TempDir()

	// Missing plugin.json.
	if _, err := ImportPlugin(dir); err == nil {
		t.Error("ImportPlugin without plugin.json should fail")
	}

	// Invalid JSON.
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte("{not json"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := ImportPlugin(dir); err == nil {
		t.Error("ImportPlugin with invalid JSON should fail")
	}

	// Missing required fields.
	bad, _ := json.Marshal(PluginManifest{Version: "1.0.0"})
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), bad, 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := ImportPlugin(dir); err == nil {
		t.Error("ImportPlugin without $schema/name should fail")
	}
}
