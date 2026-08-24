// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​‌​‌​​‌​​‌​​‌‌​‌‌​​​​‌‌‌‌​​​‌​​‌‌​‌‌​​‌​​​‌​​‌​​​​​​​​​​​​​​​​​​​‌​​‌​‌‌​‌​​‌‌​​​⁠
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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixturePluginManifest mirrors the Agent Plugins 1.0.0 plugin.json fields
// that marketplace.ImportPlugin validates.
type fixturePluginManifest struct {
	Schema      string   `json:"$schema"`
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	Description string   `json:"description,omitempty"`
	Author      string   `json:"author,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
}

// writePluginFixture creates a minimal Agent Plugins directory containing
// the given plugin.json and returns its path.
func writePluginFixture(t *testing.T, manifest *fixturePluginManifest) string {
	t.Helper()
	dir := t.TempDir()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal plugin.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), data, 0o644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}
	return dir
}

func TestMarketplaceNoArgs(t *testing.T) {
	var err error
	out := captureOutput(func() {
		err = HandleMarketplace(nil)
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleMarketplace(nil) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(out, "Usage: aflare marketplace") {
		t.Errorf("expected usage output, got:\n%s", out)
	}
}

func TestMarketplaceUnknownSubcommand(t *testing.T) {
	var err error
	out := captureOutput(func() {
		err = HandleMarketplace([]string{"zz-bogus"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleMarketplace(zz-bogus) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(out, "Unknown marketplace subcommand: zz-bogus") {
		t.Errorf("expected unknown-subcommand message, got:\n%s", out)
	}
}

func TestMarketplaceHelp(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		var err error
		out := captureOutput(func() {
			err = HandleMarketplace([]string{arg})
		})
		if code := exitCodeForErr(err); code != 0 {
			t.Errorf("HandleMarketplace(%s) exit code = %d, want 0 (err=%v)", arg, code, err)
		}
		if !strings.Contains(out, "Usage: aflare marketplace") {
			t.Errorf("expected usage output for %s, got:\n%s", arg, out)
		}
	}
}

func TestMarketplaceExportNoPackageName(t *testing.T) {
	// Through the dispatcher: "export" with no package name.
	var err error
	out := captureOutput(func() {
		err = HandleMarketplace([]string{"export"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleMarketplace(export) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(out, "Usage: aflare marketplace export") {
		t.Errorf("expected export usage, got:\n%s", out)
	}

	// Directly against the subcommand handler with no arguments.
	var directErr error
	directOut := captureOutput(func() {
		directErr = HandleMarketplaceExport(nil)
	})
	if code := exitCodeForErr(directErr); code != 1 {
		t.Errorf("HandleMarketplaceExport(nil) exit code = %d, want 1 (err=%v)", code, directErr)
	}
	if !strings.Contains(directOut, "Usage: aflare marketplace export") {
		t.Errorf("expected export usage, got:\n%s", directOut)
	}
}

func TestMarketplaceExportUnknownPackage(t *testing.T) {
	// The registry is seeded with built-in packages in memory, so looking up
	// an unknown name fails locally — no network access is involved.
	var err error
	out := captureOutput(func() {
		err = HandleMarketplaceExport([]string{"zz-no-such-package", "--dir", t.TempDir()})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleMarketplaceExport(unknown) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(out, "Export failed") {
		t.Errorf("expected export failure message, got:\n%s", out)
	}
}

func TestMarketplaceExportBuiltinPackage(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"long dir flag", []string{"--dir"}},
		{"short dir flag", []string{"-d"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outDir := t.TempDir()
			var err error
			out := captureOutput(func() {
				err = HandleMarketplaceExport([]string{"btc-monitor", tc.args[0], outDir})
			})
			if code := exitCodeForErr(err); code != 0 {
				t.Fatalf("HandleMarketplaceExport exit code = %d, want 0 (err=%v)", code, err)
			}
			if !strings.Contains(out, `Exported "btc-monitor"`) {
				t.Errorf("expected export confirmation, got:\n%s", out)
			}
			pluginDir := filepath.Join(outDir, "btc-monitor")
			for _, f := range []string{
				"plugin.json",
				"mcp.json",
				filepath.Join("skills", "btc-monitor", "SKILL.md"),
			} {
				if _, statErr := os.Stat(filepath.Join(pluginDir, f)); statErr != nil {
					t.Errorf("expected %s in the exported plugin dir: %v", f, statErr)
				}
			}
		})
	}

	t.Run("help", func(t *testing.T) {
		var err error
		out := captureOutput(func() {
			err = HandleMarketplaceExport([]string{"btc-monitor", "--help"})
		})
		if code := exitCodeForErr(err); code != 0 {
			t.Errorf("HandleMarketplaceExport(--help) exit code = %d, want 0 (err=%v)", code, err)
		}
		if !strings.Contains(out, "Export a workflow package") {
			t.Errorf("expected export help text, got:\n%s", out)
		}
	})
}

func TestMarketplaceImportErrors(t *testing.T) {
	cases := []struct {
		name    string
		dir     string // empty → no plugin dir argument at all
		wantOut string
	}{
		// No argument prints usage before failing.
		{name: "no plugin dir", wantOut: "Usage: aflare marketplace import"},
		{name: "missing dir", dir: filepath.Join(t.TempDir(), "zz-missing-plugin"), wantOut: "Import failed"},
		{name: "missing name", dir: writePluginFixture(t, &fixturePluginManifest{Schema: "https://example.invalid/schema.json"}), wantOut: "Import failed"},
		{name: "invalid json", dir: writeRawPluginJSON(t, "{not valid json"), wantOut: "Import failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var args []string
			if tc.dir != "" {
				args = []string{tc.dir}
			}
			var err error
			out := captureOutput(func() {
				err = HandleMarketplaceImport(args)
			})
			if code := exitCodeForErr(err); code != 1 {
				t.Errorf("HandleMarketplaceImport(%v) exit code = %d, want 1 (err=%v)", args, code, err)
			}
			if !strings.Contains(out, tc.wantOut) {
				t.Errorf("expected %q in output, got:\n%s", tc.wantOut, out)
			}
		})
	}
}

// writeRawPluginJSON creates a plugin directory with the raw given
// plugin.json content (used for invalid-JSON fixtures).
func writeRawPluginJSON(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}
	return dir
}

func TestMarketplaceImportPlugin(t *testing.T) {
	dir := writePluginFixture(t, &fixturePluginManifest{
		Schema:      "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		Name:        "zz-plugin",
		Version:     "1.2.3",
		Description: "A fixture plugin",
		Author:      "Tester",
		Keywords:    []string{"fixture", "aflare"},
	})

	var err error
	out := captureOutput(func() {
		err = HandleMarketplaceImport([]string{dir})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Fatalf("HandleMarketplaceImport exit code = %d, want 0 (err=%v)", code, err)
	}
	for _, want := range []string{
		"Imported Agent Plugin: zz-plugin",
		"Version:     1.2.3",
		"Description: A fixture plugin",
		"Author:      Tester",
		"Keywords:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in import output, got:\n%s", want, out)
		}
	}
}

func TestMarketplaceInstallUsage(t *testing.T) {
	// Through the dispatcher: "install" with no plugin dir.
	var err error
	out := captureOutput(func() {
		err = HandleMarketplace([]string{"install"})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("HandleMarketplace(install) exit code = %d, want 0 (err=%v)", code, err)
	}
	if !strings.Contains(out, "Usage: aflare marketplace install") {
		t.Errorf("expected install usage, got:\n%s", out)
	}

	// Directly against the subcommand handler.
	for _, args := range [][]string{nil, {"--help"}, {"-h"}} {
		var directErr error
		directOut := captureOutput(func() {
			directErr = HandleMarketplaceInstall(args)
		})
		if code := exitCodeForErr(directErr); code != 0 {
			t.Errorf("HandleMarketplaceInstall(%v) exit code = %d, want 0 (err=%v)", args, code, directErr)
		}
		if !strings.Contains(directOut, "Usage: aflare marketplace install") {
			t.Errorf("expected install usage for %v, got:\n%s", args, directOut)
		}
	}
}

// writeInstallablePluginFixture extends writePluginFixture with one skill
// (skills/zz-greet/SKILL.md) and one stdio MCP server (mcp.json) so the
// plugin has installable components. Nothing in it is ever executed.
func writeInstallablePluginFixture(t *testing.T) string {
	t.Helper()
	dir := writePluginFixture(t, &fixturePluginManifest{
		Schema:      "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		Name:        "zz-plugin",
		Version:     "0.1.0",
		Description: "A fixture plugin",
		Author:      "Tester",
	})
	skillDir := filepath.Join(dir, "skills", "zz-greet")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	skillMD := "---\nname: zz-greet\ndescription: Greet someone warmly\n---\nSay hello politely.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	mcpJSON := `{"mcpServers":{"zz-echo-server":{"type":"stdio","command":"echo","args":["hi"]}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "mcp.json"), []byte(mcpJSON), 0o644); err != nil {
		t.Fatalf("write mcp.json: %v", err)
	}
	return dir
}

func TestMarketplaceInstallPlugin(t *testing.T) {
	// The handler installs skills into meta.ResolveTemplatesPath(); seeding
	// the registry snapshot keeps the real templates dir hermetic.
	tplDir := seedTemplatesRegistry(t)
	installedSkill := filepath.Join(tplDir, "plugin", "zz-plugin-zz-greet")
	t.Cleanup(func() {
		_ = os.RemoveAll(installedSkill)
		_ = os.Remove(filepath.Join(tplDir, "plugin")) // no-op unless now empty
	})

	dir := writeInstallablePluginFixture(t)

	// .mcp.json is written relative to the current directory; redirect it so
	// the test never touches the repository checkout.
	work := t.TempDir()
	t.Chdir(work)

	var err error
	out := captureOutput(func() {
		err = HandleMarketplace([]string{"install", dir})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Fatalf("HandleMarketplace(install) exit code = %d, want 0 (err=%v)", code, err)
	}
	for _, want := range []string{
		"Installed Agent Plugin: zz-plugin",
		"Version: 0.1.0",
		"Skill:    plugin/zz-plugin-zz-greet",
		"MCP:      zz-echo-server",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in install output, got:\n%s", want, out)
		}
	}

	// The skill is materialized under the resolved templates dir.
	for _, f := range []string{"workflow.yaml", "SKILL.md"} {
		if _, statErr := os.Stat(filepath.Join(installedSkill, f)); statErr != nil {
			t.Errorf("expected %s in %s: %v", f, installedSkill, statErr)
		}
	}

	// The stdio server is registered into .mcp.json in the working dir.
	mcpData, readErr := os.ReadFile(filepath.Join(work, ".mcp.json"))
	if readErr != nil {
		t.Fatalf("read .mcp.json: %v", readErr)
	}
	if !strings.Contains(string(mcpData), "zz-echo-server") {
		t.Errorf("expected zz-echo-server in .mcp.json, got:\n%s", mcpData)
	}
}

func TestMarketplaceInstallNoComponents(t *testing.T) {
	// A manifest-only plugin installs successfully but ships nothing.
	dir := writePluginFixture(t, &fixturePluginManifest{Name: "zz-empty-plugin"})

	var err error
	out := captureOutput(func() {
		err = HandleMarketplaceInstall([]string{dir})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Fatalf("HandleMarketplaceInstall exit code = %d, want 0 (err=%v)", code, err)
	}
	for _, want := range []string{
		"Installed Agent Plugin: zz-empty-plugin",
		"plugin shipped no installable components",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in install output, got:\n%s", want, out)
		}
	}
}

func TestMarketplaceInstallErrors(t *testing.T) {
	cases := []struct {
		name    string
		dir     string
		wantOut string
	}{
		{name: "missing dir", dir: filepath.Join(t.TempDir(), "zz-missing-plugin"), wantOut: "Install failed"},
		// A manifest without a name fails plugin.json validation locally.
		{name: "missing name", dir: writePluginFixture(t, &fixturePluginManifest{Schema: "https://example.invalid/schema.json"}), wantOut: "Install failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			out := captureOutput(func() {
				err = HandleMarketplaceInstall([]string{tc.dir})
			})
			if code := exitCodeForErr(err); code != 1 {
				t.Errorf("HandleMarketplaceInstall(%s) exit code = %d, want 1 (err=%v)", tc.name, code, err)
			}
			if !strings.Contains(out, tc.wantOut) {
				t.Errorf("expected %q in output, got:\n%s", tc.wantOut, out)
			}
		})
	}
}

// TestMarketplaceDispatchErrorPropagation checks that subcommand failures
// propagate as ExitError through the HandleMarketplace dispatcher.
func TestMarketplaceDispatchErrorPropagation(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "zz-missing-plugin")
	for _, tc := range []struct {
		sub     string
		wantOut string
	}{
		{"import", "Import failed"},
		{"install", "Install failed"},
	} {
		t.Run(tc.sub+" failure", func(t *testing.T) {
			var err error
			out := captureOutput(func() {
				err = HandleMarketplace([]string{tc.sub, missing})
			})
			if code := exitCodeForErr(err); code != 1 {
				t.Errorf("HandleMarketplace(%s missing) exit code = %d, want 1 (err=%v)", tc.sub, code, err)
			}
			if !strings.Contains(out, tc.wantOut) {
				t.Errorf("expected %q in output, got:\n%s", tc.wantOut, out)
			}
		})
	}
}
