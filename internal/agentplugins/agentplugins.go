// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​‌​​​​‌​‌​‌​‌​​​‌​​​​​‌‌‌‌‌‌​​‌​​‌​‌​‌​​‌‌‌​‌‌​​​​​​​​​​​​​​​​‌​​​​​‌​​​​‌​​​​⁠
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

// agentplugins implements the host side of the Agent Plugins 1.0.0 open
// standard (https://agent-plugins.org/): it loads a plugin directory
// (plugin.json manifest + skills/*/SKILL.md + mcp.json) published for any
// compatible client (VS Code, Cursor, Copilot, ChatGPT, Codex, Kiro, ...) and
// installs its components into aflare's own skill registry and MCP config.
//
// Together with marketplace.ExportPlugin (which writes the same format) this
// makes aflare bidirectional in the plugin ecosystem: aflare plugins run in
// other clients, and plugins from other clients run in aflare.
//
// Security model (mirrors the spec's package-level rules):
//   - the manifest must declare a non-empty name; everything else is optional
//   - component paths are validated to stay inside the plugin root
//   - mcp.json entries are limited to stdio transport; cwd must be an
//     explicit "./"-relative path inside the plugin root (no "../" escapes)
//   - ${PLUGIN_ROOT} placeholders expand to the plugin's absolute root
//
// What the spec deliberately leaves out (permissions, sandboxing, signing)
// stays aflare's own responsibility: installing a plugin only materializes
// files under the aflare skills directory and upserts stdio server entries
// into the user's .mcp.json — nothing is executed at install time.

package agentplugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/alib8b8/aflare/internal/mcp"
	"github.com/alib8b8/aflare/internal/skills"
)

const (
	// ManifestName is the manifest file name in both supported layouts.
	ManifestName = "plugin.json"
	// ManifestNestedDir is the open-plugin-spec layout prefix (.plugin/plugin.json).
	ManifestNestedDir = ".plugin"
	// SkillsDirName is the fixed directory holding Agent Skills.
	SkillsDirName = "skills"
	// MCPFileName declares MCP servers shipped with the plugin.
	MCPFileName = "mcp.json"
	// PluginRootPlaceholder expands to the plugin's absolute root directory.
	PluginRootPlaceholder = "${PLUGIN_ROOT}"
	// DefaultCategory is the skill category prefix assigned to plugin skills.
	DefaultCategory = "plugin"
)

// Manifest mirrors the Agent Plugins 1.0.0 plugin.json manifest. Only name is
// required by the spec; the schema is closed (unknown top-level fields are
// rejected by compliant producers and ignored here on read).
type Manifest struct {
	Schema      string   `json:"$schema,omitempty"`
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	Description string   `json:"description,omitempty"`
	Author      string   `json:"author,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Repository  string   `json:"repository,omitempty"`
	License     string   `json:"license,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
}

// LoadManifest discovers and parses the plugin manifest. Both published
// layouts are supported, with precedence matching the spec's
// "manifest location and precedence" section:
//
//  1. <pluginDir>/.plugin/plugin.json  (open-plugin-spec layout)
//  2. <pluginDir>/plugin.json          (agent-plugins.org layout, also what
//     aflare's own marketplace export writes)
func LoadManifest(pluginDir string) (*Manifest, error) {
	absDir, err := filepath.Abs(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve plugin dir: %w", err)
	}

	candidates := []string{
		filepath.Join(absDir, ManifestNestedDir, ManifestName),
		filepath.Join(absDir, ManifestName),
	}
	var manifestPath string
	var data []byte
	for _, c := range candidates {
		if d, err := os.ReadFile(c); err == nil { // #nosec G304 -- candidates are fixed names joined to the resolved plugin dir
			manifestPath, data = c, d
			break
		}
	}
	if data == nil {
		return nil, fmt.Errorf("no %s found in %s (looked at .plugin/%s and %s)",
			ManifestName, pluginDir, ManifestName, ManifestName)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest %s: %w", manifestPath, err)
	}
	// Spec: name is the single required field.
	if strings.TrimSpace(m.Name) == "" {
		return nil, fmt.Errorf("manifest %s: required field \"name\" is empty", manifestPath)
	}
	if !validName(m.Name) {
		return nil, fmt.Errorf("manifest %s: invalid plugin name %q (allowed: letters, digits, '-', '_', '.', max 214 chars)", manifestPath, m.Name)
	}
	return &m, nil
}

// validName enforces npm-package-like plugin names (the spec defers naming to
// package-manager conventions; this matches the strictest common subset).
func validName(name string) bool {
	if len(name) == 0 || len(name) > 214 {
		return false
	}
	if strings.TrimSpace(name) != name {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		case r == '@' || r == '/': // scoped names like @scope/plugin
		default:
			return false
		}
	}
	// Scoped names may contain "/", but never traversal segments.
	for _, seg := range strings.Split(name, "/") {
		if seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// SkillDoc is one parsed skills/<dir>/SKILL.md component. The frontmatter
// follows the Agent Skills open standard: name + description; the body below
// the closing --- is the instruction content.
type SkillDoc struct {
	Name        string // frontmatter name, falls back to the directory name
	Description string // frontmatter description
	Body        string // instruction content below the frontmatter
	SKILLPath   string // absolute path of the source SKILL.md
}

// LoadSkills discovers and parses every skills/*/SKILL.md under the plugin
// root. A missing skills/ directory is not an error (the plugin simply ships
// no skills). Skill directory names must be safe single path segments so the
// materialized copy can never escape the target skills root.
func LoadSkills(pluginDir string) ([]SkillDoc, error) {
	absDir, err := filepath.Abs(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve plugin dir: %w", err)
	}
	skillsRoot := filepath.Join(absDir, SkillsDirName)

	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", skillsRoot, err)
	}

	var docs []SkillDoc
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirName := e.Name()
		if !validSkillSegment(dirName) {
			return nil, fmt.Errorf("unsafe skill directory name %q (must be a simple path segment without traversal)", dirName)
		}
		skillPath := filepath.Join(skillsRoot, dirName, "SKILL.md")
		data, err := os.ReadFile(skillPath) // #nosec G304 -- path built from validated segments
		if err != nil {
			continue // spec: a non-conforming skill is skipped, not fatal
		}
		doc, err := parseSKILLMarkdown(string(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", skillPath, err)
		}
		if doc.Name == "" {
			doc.Name = dirName
		}
		// The frontmatter name flows into the target install path, so it gets
		// the same traversal check as directory names — a compliant directory
		// name must not be able to smuggle an escaping frontmatter name.
		if !validSkillSegment(doc.Name) {
			return nil, fmt.Errorf("%s: unsafe skill name %q (must be a simple path segment without traversal)", skillPath, doc.Name)
		}
		doc.SKILLPath = skillPath
		docs = append(docs, doc)
	}
	return docs, nil
}

// validSkillSegment rejects absolute paths, separators, traversal and hidden
// segments, mirroring the aflare skill-ID segment rules.
func validSkillSegment(seg string) bool {
	if seg == "" || len(seg) > 128 {
		return false
	}
	if strings.ContainsAny(seg, "/\\") || strings.ContainsRune(seg, '\x00') {
		return false
	}
	if seg == "." || seg == ".." || strings.HasPrefix(seg, ".") {
		return false
	}
	if len(seg) >= 2 && seg[1] == ':' { // Windows drive letter
		return false
	}
	return true
}

// parseSKILLMarkdown splits an optional YAML frontmatter block from the
// markdown body. The frontmatter must be the very first line and is closed by
// the second "---" line; anything after that is the skill body.
func parseSKILLMarkdown(content string) (SkillDoc, error) {
	var doc SkillDoc
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		doc.Body = strings.TrimSpace(normalized)
		return doc, nil
	}
	rest := normalized[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return doc, fmt.Errorf("unterminated frontmatter (missing closing ---)")
	}
	frontmatter := rest[:end]
	body := strings.TrimPrefix(rest[end+len("\n---"):], "\n")

	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		return doc, fmt.Errorf("invalid frontmatter YAML: %w", err)
	}
	doc.Name = strings.TrimSpace(fm.Name)
	doc.Description = strings.TrimSpace(fm.Description)
	doc.Body = strings.TrimSpace(body)
	return doc, nil
}

// MCPServer is one sanitized mcp.json entry, ready to be upserted into the
// user's aflare MCP config.
type MCPServer struct {
	Name  string
	Entry mcp.ServerEntry
}

// LoadMCP parses the plugin's mcp.json and returns the stdio servers it
// declares. Rules enforced here:
//
//   - non-stdio transports (http/sse) are skipped with no error: aflare only
//     speaks stdio to MCP servers, and the spec mandates per-component
//     failure isolation (a bad entry disables only itself)
//   - cwd, when present, must start with "./" and resolve inside the plugin
//     root (spec: plugin-relative only, no "../" escapes)
//   - ${PLUGIN_ROOT} in command/args/env/cwd expands to the plugin root
//   - a server without a command is rejected (it could never start)
func LoadMCP(pluginDir string) ([]MCPServer, error) {
	absDir, err := filepath.Abs(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve plugin dir: %w", err)
	}
	mcpPath := filepath.Join(absDir, MCPFileName)

	data, err := os.ReadFile(mcpPath) // #nosec G304 -- fixed name inside plugin dir
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", mcpPath, err)
	}

	var file struct {
		MCPServers map[string]struct {
			Type    string            `json:"type,omitempty"`
			Command string            `json:"command,omitempty"`
			Args    []string          `json:"args,omitempty"`
			Env     map[string]string `json:"env,omitempty"`
			Cwd     string            `json:"cwd,omitempty"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", MCPFileName, err)
	}

	var servers []MCPServer
	for name, raw := range file.MCPServers {
		if raw.Type != "" && raw.Type != "stdio" {
			continue // unsupported transport here; skip per-component isolation
		}
		command := expandPluginRoot(raw.Command, absDir)
		if command == "" {
			return nil, fmt.Errorf("mcp server %q: stdio entry requires a command", name)
		}
		entry := mcp.ServerEntry{
			Type:    "stdio",
			Command: command,
			Env:     make(map[string]string, len(raw.Env)),
		}
		for _, a := range raw.Args {
			entry.Args = append(entry.Args, expandPluginRoot(a, absDir))
		}
		for k, v := range raw.Env {
			entry.Env[k] = expandPluginRoot(v, absDir)
		}
		if raw.Cwd != "" {
			cwd := expandPluginRoot(raw.Cwd, absDir)
			if !strings.HasPrefix(cwd, "./") && cwd != "." {
				return nil, fmt.Errorf("mcp server %q: cwd must be a \"./\"-relative path inside the plugin root, got %q", name, raw.Cwd)
			}
			resolved := filepath.Clean(filepath.Join(absDir, cwd))
			if !withinDir(absDir, resolved) {
				return nil, fmt.Errorf("mcp server %q: cwd escapes the plugin root", name)
			}
			// String containment is not enough: a symlink inside the plugin
			// root can point anywhere on disk. When the path exists, verify
			// the symlink-resolved location is still inside the (resolved)
			// plugin root.
			if realResolved, err := filepath.EvalSymlinks(resolved); err == nil {
				realRoot, rerr := filepath.EvalSymlinks(absDir)
				if rerr == nil && !withinDir(realRoot, realResolved) {
					return nil, fmt.Errorf("mcp server %q: cwd resolves (symlink) outside the plugin root", name)
				}
			}
			entry.Cwd = resolved
		}
		servers = append(servers, MCPServer{Name: name, Entry: entry})
	}
	return servers, nil
}

// expandPluginRoot replaces ${PLUGIN_ROOT} occurrences with the plugin root.
func expandPluginRoot(s, pluginRoot string) string {
	return strings.ReplaceAll(s, PluginRootPlaceholder, pluginRoot)
}

// withinDir reports whether path is root itself or located underneath it.
// Both must already be cleaned/absolute.
func withinDir(root, path string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}

// InstallOptions configures InstallPlugin.
type InstallOptions struct {
	// SkillsBaseDir is the aflare skills root the plugin's skills are
	// materialized into (e.g. ~/.config/aflare/skills).
	SkillsBaseDir string
	// MCPConfigPath is the user's .mcp.json; declared stdio servers are
	// upserted into it. Empty skips MCP registration.
	MCPConfigPath string
	// Category is the skill category prefix; defaults to "plugin". Plugin
	// skills get the ID "<category>/<plugin>-<skill>".
	Category string
}

// InstallResult summarizes what InstallPlugin did.
type InstallResult struct {
	Manifest        *Manifest
	SkillsInstalled []string // skill IDs
	MCPServers      []string // server names upserted
	PluginDir       string   // absolute source plugin dir
}

// InstallPlugin loads an Agent Plugins 1.0.0 directory and installs its
// components into aflare:
//
//   - each skills/<name>/SKILL.md is materialized under
//     <SkillsBaseDir>/<category>/<plugin>-<name>/ as skill.json plus a
//     runnable workflow.yaml wrapper (single openai step whose prompt embeds
//     the SKILL.md instructions), making it immediately usable via
//     `aflare run <id>`
//   - each stdio mcp.json server is upserted into the MCP config (existing
//     user entries are never overwritten, matching UpsertMCPServer semantics)
//
// Nothing from the plugin is executed during installation.
func InstallPlugin(pluginDir string, opts InstallOptions) (*InstallResult, error) {
	manifest, err := LoadManifest(pluginDir)
	if err != nil {
		return nil, err
	}
	absDir, err := filepath.Abs(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve plugin dir: %w", err)
	}

	category := opts.Category
	if category == "" {
		category = DefaultCategory
	}
	if !validSkillSegment(category) {
		return nil, fmt.Errorf("invalid category %q", opts.Category)
	}

	res := &InstallResult{Manifest: manifest, PluginDir: absDir}

	// 1. Skills.
	docs, err := LoadSkills(pluginDir)
	if err != nil {
		return nil, err
	}
	if len(docs) > 0 {
		if opts.SkillsBaseDir == "" {
			return nil, fmt.Errorf("plugin ships %d skills but InstallOptions.SkillsBaseDir is empty", len(docs))
		}
		skillsBase, err := filepath.Abs(opts.SkillsBaseDir)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve skills base dir: %w", err)
		}
		pluginSeg := strings.ReplaceAll(manifest.Name, "/", "-")
		pluginSeg = strings.ReplaceAll(pluginSeg, "@", "")
		var metas []*skills.SkillMeta
		for _, doc := range docs {
			skillID := fmt.Sprintf("%s/%s-%s", category, pluginSeg, doc.Name)
			targetDir := filepath.Join(skillsBase, filepath.FromSlash(skillID))
			// Defense in depth: no matter what slipped through the segment
			// validation, the materialized path must stay under the skills root.
			if !withinDir(skillsBase, targetDir) {
				return nil, fmt.Errorf("install skill %s: target path escapes the skills base dir", skillID)
			}
			meta, err := materializeSkill(targetDir, skillID, manifest, doc)
			if err != nil {
				return nil, fmt.Errorf("install skill %s: %w", skillID, err)
			}
			metas = append(metas, meta)
			res.SkillsInstalled = append(res.SkillsInstalled, skillID)
		}
		// Refresh the registry index so newly installed skills are visible
		// to `aflare list` / `aflare run`. Load() may take the stale-index
		// path, so the new metas are merged explicitly via AddSkill.
		reg := skills.NewSkillRegistry(opts.SkillsBaseDir)
		if err := reg.Load(); err != nil {
			return nil, fmt.Errorf("reload skill registry: %w", err)
		}
		for _, meta := range metas {
			if err := reg.AddSkill(meta); err != nil {
				return nil, fmt.Errorf("register skill: %w", err)
			}
		}
		if err := reg.SaveRegistry(); err != nil {
			return nil, fmt.Errorf("save skill registry: %w", err)
		}
	}

	// 2. MCP servers.
	if opts.MCPConfigPath != "" {
		servers, err := LoadMCP(pluginDir)
		if err != nil {
			return nil, err
		}
		for _, s := range servers {
			if _, err := mcp.UpsertMCPServer(opts.MCPConfigPath, s.Name, s.Entry); err != nil {
				return nil, fmt.Errorf("register mcp server %s: %w", s.Name, err)
			}
			res.MCPServers = append(res.MCPServers, s.Name)
		}
	}

	return res, nil
}

// materializeSkill writes the skill.json + workflow.yaml + SKILL.md copy for
// one plugin skill and returns the resulting registry metadata. The workflow
// is a deterministic single-step wrapper: the SKILL.md instructions become
// the openai node's system prompt and the workflow input becomes the user
// task, so plugin skills run without any codegen.
func materializeSkill(targetDir, skillID string, manifest *Manifest, doc SkillDoc) (*skills.SkillMeta, error) {
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return nil, fmt.Errorf("create skill dir: %w", err)
	}

	description := doc.Description
	if description == "" {
		description = fmt.Sprintf("Agent Plugin skill %q from plugin %s", doc.Name, manifest.Name)
	}
	if len(description) > 500 {
		description = description[:497] + "..."
	}

	meta := skills.SkillMeta{
		ID:          skillID,
		Name:        doc.Name,
		Version:     manifest.Version,
		Description: description,
		Author:      manifest.Author,
		Category:    strings.SplitN(skillID, "/", 2)[0],
		Tags:        append([]string{"agent-plugin"}, manifest.Keywords...),
		Inputs: []skills.SkillIO{
			{Name: "input", Type: "string", Required: true, Description: "Task for the skill"},
		},
		Outputs: []skills.SkillIO{
			{Name: "output", Type: "string", Description: "Skill result"},
		},
		Path: targetDir,
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal skill.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, skills.SkillMetaFile), metaBytes, 0o640); err != nil { // #nosec G306 -- skill metadata is not a secret; 0640 matches the skill dir's group-readable 0750
		return nil, fmt.Errorf("write skill.json: %w", err)
	}

	// The openai node sends the step input as the user message, so the
	// SKILL.md instructions go into the system prompt and the workflow's
	// input (the caller's task) arrives as the user turn — no templating
	// needed.
	system := buildSkillSystemPrompt(manifest, doc)
	wf := fmt.Sprintf(`name: %s
description: %q
# Materialized from Agent Plugins 1.0.0 plugin %q, skill %q.
# The SKILL.md instructions are embedded verbatim in the system prompt;
# the workflow input is passed through as the user task.
steps:
  - name: execute
    node: openai
    params:
      system: |
%s
`, doc.Name, description, manifest.Name, doc.Name, indentBlock(system, 8))

	if err := os.WriteFile(filepath.Join(targetDir, "workflow.yaml"), []byte(wf), 0o640); err != nil { // #nosec G306 -- runnable workflow definition is not a secret; 0640 matches the skill dir's group-readable 0750
		return nil, fmt.Errorf("write workflow.yaml: %w", err)
	}

	// Keep the original SKILL.md next to the wrapper for provenance and
	// re-export by other Agent Plugins compatible clients.
	if err := os.WriteFile(filepath.Join(targetDir, "SKILL.md"), []byte(renderSourceSKILL(doc)), 0o640); err != nil { // #nosec G306 -- provenance copy for re-export is not a secret; 0640 matches the skill dir's group-readable 0750
		return nil, fmt.Errorf("write SKILL.md: %w", err)
	}
	return &meta, nil
}

// buildSkillSystemPrompt assembles the system prompt for the wrapper step:
// the SKILL.md instructions, framed so the model treats them as its
// operating procedure for the incoming task.
func buildSkillSystemPrompt(manifest *Manifest, doc SkillDoc) string {
	var sb strings.Builder
	sb.WriteString("You are executing the following skill. Follow these instructions precisely for the user's task.\n\n")
	if doc.Description != "" {
		fmt.Fprintf(&sb, "# %s\n\n", doc.Description)
	}
	sb.WriteString(doc.Body)
	return sb.String()
}

// renderSourceSKILL regenerates the SKILL.md content for the installed copy.
func renderSourceSKILL(doc SkillDoc) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "name: %s\n", doc.Name)
	if doc.Description != "" {
		fmt.Fprintf(&sb, "description: %s\n", doc.Description)
	}
	sb.WriteString("---\n\n")
	sb.WriteString(doc.Body)
	sb.WriteString("\n")
	return sb.String()
}

// indentBlock prefixes every line of s with n spaces (blank lines stay empty).
func indentBlock(s string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n")
}
