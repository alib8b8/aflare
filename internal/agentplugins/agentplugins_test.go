// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​‌​‌​​‌‌‌‌​​‌​‌‌​​‌‌‌​​​‌‌​​‌​‌‌‌​‌​‌​​​‌​​​‌‌​​​​​​​​​​​​​​​​‌‌​​‌​​​‌​​‌​‌​‌⁠
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

package agentplugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/mcp"
	"github.com/alib8b8/aflare/internal/skills"
	"github.com/alib8b8/aflare/internal/workflow"
)

// writePlugin creates a minimal Agent Plugins 1.0.0 directory with one skill
// and one stdio MCP server, in the root-manifest layout.
func writePlugin(t *testing.T, layout string) string {
	t.Helper()
	dir := t.TempDir()
	pdir := filepath.Join(dir, "demo-plugin")
	if err := os.MkdirAll(filepath.Join(pdir, "skills", "greet"), 0o750); err != nil {
		t.Fatal(err)
	}

	manifest := `{"name":"demo-plugin","version":"1.2.3","description":"demo","author":"tester","keywords":["demo"]}`
	switch layout {
	case "nested":
		if err := os.MkdirAll(filepath.Join(pdir, ".plugin"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pdir, ".plugin", "plugin.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	case "root":
		if err := os.WriteFile(filepath.Join(pdir, "plugin.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	skill := "---\nname: greet\ndescription: Greets a person warmly\n---\n\nGreet the user by name.\nBe concise.\n"
	if err := os.WriteFile(filepath.Join(pdir, "skills", "greet", "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}

	mcpJSON := `{"mcpServers":{"demo-server":{"type":"stdio","command":"${PLUGIN_ROOT}/bin/server","args":["--config","${PLUGIN_ROOT}/conf/x.json"],"env":{"PLUGIN_DATA":"${PLUGIN_ROOT}/data"},"cwd":"./"}}}`
	if err := os.WriteFile(filepath.Join(pdir, "mcp.json"), []byte(mcpJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return pdir
}

func TestLoadManifest_BothLayouts(t *testing.T) {
	for _, layout := range []string{"root", "nested"} {
		pdir := writePlugin(t, layout)
		m, err := LoadManifest(pdir)
		if err != nil {
			t.Fatalf("layout %s: %v", layout, err)
		}
		if m.Name != "demo-plugin" || m.Version != "1.2.3" {
			t.Fatalf("layout %s: unexpected manifest %+v", layout, m)
		}
	}
}

func TestLoadManifest_MissingNameRejected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"version":"1.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(dir); err == nil {
		t.Fatal("manifest without name must be rejected")
	}
}

func TestLoadManifest_TraversalNameRejected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"name":"../../evil"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(dir); err == nil {
		t.Fatal("plugin name with traversal must be rejected")
	}
}

func TestLoadSkills_ParsesFrontmatter(t *testing.T) {
	pdir := writePlugin(t, "root")
	docs, err := LoadSkills(pdir)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(docs))
	}
	d := docs[0]
	if d.Name != "greet" || d.Description != "Greets a person warmly" {
		t.Fatalf("unexpected frontmatter: %+v", d)
	}
	if !strings.Contains(d.Body, "Greet the user by name.") {
		t.Fatalf("body not parsed: %q", d.Body)
	}
}

func TestLoadSkills_NoSkillsDirIsFine(t *testing.T) {
	dir := t.TempDir()
	docs, err := LoadSkills(dir)
	if err != nil || len(docs) != 0 {
		t.Fatalf("missing skills dir must yield empty, got docs=%d err=%v", len(docs), err)
	}
}

func TestLoadSkills_UnsafeDirNameRejected(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "skills", "..evil")
	if err := os.MkdirAll(bad, 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSkills(dir); err == nil {
		t.Fatal("unsafe skill directory name must be rejected")
	}
}

func TestLoadMCP_ExpandsPluginRootAndKeepsStdioOnly(t *testing.T) {
	pdir := writePlugin(t, "root")

	// Add an http-transport entry alongside the stdio one: must be skipped.
	mcpJSON, err := os.ReadFile(filepath.Join(pdir, "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	mcpJSON = []byte(strings.Replace(string(mcpJSON), `}}}`, `},"remote":{"type":"http","command":"ignored"}}}`, 1))
	if err := os.WriteFile(filepath.Join(pdir, "mcp.json"), mcpJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	servers, err := LoadMCP(pdir)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 stdio server (http skipped), got %d", len(servers))
	}
	s := servers[0]
	if s.Name != "demo-server" {
		t.Fatalf("unexpected server name %q", s.Name)
	}
	if !strings.HasPrefix(s.Entry.Command, pdir) {
		t.Fatalf("${PLUGIN_ROOT} not expanded in command: %q", s.Entry.Command)
	}
	if len(s.Entry.Args) != 2 || !strings.HasPrefix(s.Entry.Args[1], pdir) {
		t.Fatalf("args not expanded: %v", s.Entry.Args)
	}
	if !strings.HasPrefix(s.Entry.Env["PLUGIN_DATA"], pdir) {
		t.Fatalf("env not expanded: %v", s.Entry.Env)
	}
	if s.Entry.Cwd != pdir {
		t.Fatalf("cwd must resolve to plugin root, got %q", s.Entry.Cwd)
	}
}

func TestLoadMCP_EscapingCwdRejected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mcp.json"),
		[]byte(`{"mcpServers":{"bad":{"command":"x","cwd":"../../etc"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMCP(dir); err == nil {
		t.Fatal("cwd escaping the plugin root must be rejected")
	}
}

func TestLoadMCP_MissingCommandRejected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mcp.json"),
		[]byte(`{"mcpServers":{"bad":{"type":"stdio"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMCP(dir); err == nil {
		t.Fatal("stdio entry without command must be rejected")
	}
}

func TestInstallPlugin_MaterializesRunnableSkill(t *testing.T) {
	pdir := writePlugin(t, "nested") // exercise the .plugin/ layout end-to-end

	base := t.TempDir()
	mcpCfg := filepath.Join(t.TempDir(), ".mcp.json")

	res, err := InstallPlugin(pdir, InstallOptions{
		SkillsBaseDir: base,
		MCPConfigPath: mcpCfg,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.SkillsInstalled) != 1 || res.SkillsInstalled[0] != "plugin/demo-plugin-greet" {
		t.Fatalf("unexpected skills installed: %v", res.SkillsInstalled)
	}
	if len(res.MCPServers) != 1 || res.MCPServers[0] != "demo-server" {
		t.Fatalf("unexpected mcp servers: %v", res.MCPServers)
	}

	// The materialized skill must be discoverable through the registry.
	reg := skills.NewSkillRegistry(base)
	if err := reg.Load(); err != nil {
		t.Fatal(err)
	}
	meta, err := reg.Get("plugin/demo-plugin-greet")
	if err != nil {
		t.Fatalf("installed skill not in registry: %v", err)
	}
	if meta.Author != "tester" || meta.Version != "1.2.3" {
		t.Fatalf("metadata not carried over: %+v", meta)
	}

	// workflow.yaml must exist, embed the SKILL.md instructions, and parse
	// as a valid aflare workflow.
	wf, err := os.ReadFile(filepath.Join(meta.Path, "workflow.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wf), "Greet the user by name.") {
		t.Fatalf("workflow.yaml missing skill instructions:\n%s", wf)
	}
	if !strings.Contains(string(wf), "node: openai") {
		t.Fatalf("workflow.yaml missing openai step:\n%s", wf)
	}
	if _, err := workflow.ParseWorkflowFromContent(string(wf)); err != nil {
		t.Fatalf("materialized workflow.yaml does not parse: %v\n%s", err, wf)
	}

	// .mcp.json must contain the upserted stdio entry.
	cfg, err := mcp.LoadMCPConfig(mcpCfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.MCPServers["demo-server"]; !ok {
		t.Fatalf("demo-server not registered in %s", mcpCfg)
	}
}

func TestInstallPlugin_IsIdempotent(t *testing.T) {
	pdir := writePlugin(t, "root")
	base := t.TempDir()
	mcpCfg := filepath.Join(t.TempDir(), ".mcp.json")
	opts := InstallOptions{SkillsBaseDir: base, MCPConfigPath: mcpCfg}

	if _, err := InstallPlugin(pdir, opts); err != nil {
		t.Fatal(err)
	}
	res, err := InstallPlugin(pdir, opts) // second install must succeed
	if err != nil {
		t.Fatalf("re-install failed: %v", err)
	}
	if len(res.SkillsInstalled) != 1 {
		t.Fatalf("re-install lost skills: %v", res.SkillsInstalled)
	}
}

func TestInstallPlugin_UserMCPEntryNotOverwritten(t *testing.T) {
	pdir := writePlugin(t, "root")
	base := t.TempDir()
	mcpCfg := filepath.Join(t.TempDir(), ".mcp.json")

	// User already configured demo-server differently.
	if _, err := mcp.UpsertMCPServer(mcpCfg, "demo-server", mcp.ServerEntry{
		Type: "stdio", Command: "my-own-binary",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallPlugin(pdir, InstallOptions{SkillsBaseDir: base, MCPConfigPath: mcpCfg}); err != nil {
		t.Fatal(err)
	}
	cfg, err := mcp.LoadMCPConfig(mcpCfg)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCPServers["demo-server"].Command != "my-own-binary" {
		t.Fatalf("user entry was clobbered: %+v", cfg.MCPServers["demo-server"])
	}
}

// TestLoadSkills_TraversalFrontmatterNameRejected guards against the H1 path
// traversal: a compliant skill directory name must not smuggle an escaping
// frontmatter name into the materialized install path.
func TestLoadSkills_TraversalFrontmatterNameRejected(t *testing.T) {
	pdir := writePlugin(t, "root")
	malicious := "---\nname: ../../../../tmp/evil\ndescription: looks legit\n---\n\ninstructions\n"
	if err := os.WriteFile(filepath.Join(pdir, "skills", "greet", "SKILL.md"), []byte(malicious), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSkills(pdir); err == nil {
		t.Fatal("frontmatter name with traversal must be rejected")
	}
}

func TestInstallPlugin_TraversalNameWritesNothingOutside(t *testing.T) {
	dir := t.TempDir()
	pdir := filepath.Join(dir, "plugin")
	if err := os.MkdirAll(filepath.Join(pdir, "skills", "legit"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "plugin.json"), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	malicious := "---\nname: ../../escape\ndescription: x\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(pdir, "skills", "legit", "SKILL.md"), []byte(malicious), 0o644); err != nil {
		t.Fatal(err)
	}

	base := filepath.Join(dir, "skills-base")
	if _, err := InstallPlugin(pdir, InstallOptions{SkillsBaseDir: base}); err == nil {
		t.Fatal("install with escaping frontmatter name must fail")
	}
	// Nothing may have been written outside the skills base dir.
	if _, err := os.Stat(filepath.Join(dir, "escape")); !os.IsNotExist(err) {
		t.Fatal("escaping path must not be created")
	}
	if _, err := os.Stat(filepath.Join(dir, "workflow.yaml")); !os.IsNotExist(err) {
		t.Fatal("no file may land next to the skills base dir")
	}
}

// TestLoadMCP_CwdSymlinkEscapeRejected guards against the M1 symlink bypass:
// string prefix containment passes while the symlink actually points outside
// the plugin root.
func TestLoadMCP_CwdSymlinkEscapeRejected(t *testing.T) {
	dir := t.TempDir()
	pdir := filepath.Join(dir, "plugin")
	if err := os.MkdirAll(pdir, 0o750); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "outside")
	if err := os.MkdirAll(outside, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(pdir, "data")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "plugin.json"), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mcpJSON := `{"mcpServers":{"s":{"type":"stdio","command":"bin/server","cwd":"./data"}}}`
	if err := os.WriteFile(filepath.Join(pdir, "mcp.json"), []byte(mcpJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadMCP(pdir); err == nil {
		t.Fatal("cwd pointing through a symlink outside the plugin root must be rejected")
	}
}

func TestLoadMCP_CwdInsideSymlinkedPluginRootStillWorks(t *testing.T) {
	// A plugin root referenced through a symlink alias must not break the
	// containment check: EvalSymlinks is applied to both sides, so ./data
	// under the alias still resolves inside the real root.
	dir := t.TempDir()
	pdir := filepath.Join(dir, "plugin")
	if err := os.MkdirAll(filepath.Join(pdir, "data"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "plugin.json"), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mcpJSON := `{"mcpServers":{"s":{"type":"stdio","command":"bin/server","cwd":"./data"}}}`
	if err := os.WriteFile(filepath.Join(pdir, "mcp.json"), []byte(mcpJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(pdir, alias); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	servers, err := LoadMCP(alias)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0].Entry.Cwd == "" {
		t.Fatalf("legitimate ./data cwd must pass: %+v", servers)
	}
}
