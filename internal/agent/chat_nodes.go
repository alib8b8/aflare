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

// chat_nodes.go registers nodes that bridge the chat agent's tools to the
// project's existing template and workflow functionality. These nodes are
// registered in the global registry during chat session initialization
// so they are discoverable by the ReActAgent.
//
// The 300+ templates in the templates/ directory are treated as the agent's
// skills — the agent can discover them via template_list, inspect them via
// template_info, and execute them via run_workflow.

package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/alib8b8/aflare/internal/meta"
	"github.com/alib8b8/aflare/internal/metrics"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/nodes/core"
	"github.com/alib8b8/aflare/internal/templates"
	workflow "github.com/alib8b8/aflare/internal/workflow"
)

// TemplateCategories lists all available template categories (skills domains).
// Each directory under templates/ corresponds to a domain the agent can help with.
var TemplateCategories = []string{
	"business", "content-creative", "data-ai", "devops-infra",
	"ecommerce", "education", "finance", "healthcare",
	"hr", "integrations", "iot", "legal",
	"lifestyle", "marketing", "software-engineering", "supply-chain",
}

// extTemplate holds metadata for an external template file.
type extTemplate struct {
	Name        string // e.g. "stock-screener"
	Category    string // e.g. "finance"
	Description string // extracted from comments
	FilePath    string // relative path: "templates/finance/stock-screener/workflow.yaml"
}

// scanExternalTemplates scans the templates/ directory for external workflow files.
// Returns templates sorted by category then name.
// Errors are logged but not fatal — the agent can still operate with built-in templates.
func scanExternalTemplates() []extTemplate {
	// Try common locations for the templates directory
	templatesDir := findTemplatesDir()
	if templatesDir == "" {
		return nil
	}

	var results []extTemplate
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		log.Printf("[chat_nodes] failed to read templates directory %s: %v", templatesDir, err)
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		category := entry.Name()

		// Skip hidden directories and non-category dirs
		if strings.HasPrefix(category, ".") {
			continue
		}

		catPath := filepath.Join(templatesDir, category)
		templates, err := os.ReadDir(catPath)
		if err != nil {
			log.Printf("[chat_nodes] failed to read category %s: %v", category, err)
			continue
		}

		for _, t := range templates {
			if !t.IsDir() {
				continue
			}
			templateName := t.Name()
			wfPath := filepath.Join(catPath, templateName, "workflow.yaml")
			if _, err := os.Stat(wfPath); err != nil {
				continue
			}

			desc := extractDescription(wfPath)
			results = append(results, extTemplate{
				Name:        templateName,
				Category:    category,
				Description: desc,
				FilePath:    wfPath,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Category != results[j].Category {
			return results[i].Category < results[j].Category
		}
		return results[i].Name < results[j].Name
	})

	return results
}

// findTemplatesDir locates the templates directory relative to cwd or executable.
func findTemplatesDir() string {
	// Try relative to cwd first
	if _, err := os.Stat("templates"); err == nil {
		return "templates"
	}
	// Try relative to executable
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), "templates")
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}
	return ""
}

// extractDescription reads the first comment lines from a workflow YAML
// to use as a description. Falls back to the template name.
func extractDescription(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	var desc []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "name:") || strings.HasPrefix(line, "steps:") {
			continue
		}
		if strings.HasPrefix(line, "#") {
			text := strings.TrimPrefix(line, "#")
			text = strings.TrimSpace(text)
			// Skip usage hints
			if strings.HasPrefix(text, "Usage:") || strings.HasPrefix(text, "Config:") {
				continue
			}
			desc = append(desc, text)
			if len(desc) >= 3 {
				break
			}
		} else {
			// First non-comment, non-name line stops description extraction
			break
		}
	}
	return strings.Join(desc, " ")
}

// ── TemplateListNode ────────────────────────────────────────────────────

type templateListNode struct{}

func (n *templateListNode) Name() string { return "template_list" }
func (n *templateListNode) Description() string {
	return "Search available workflow templates (skills) by keyword or category. Returns 300+ pre-built skills across 17 domains."
}

func (n *templateListNode) Schema() core.NodeSchema {
	return core.NodeSchema{
		Name:        "template_list",
		Description: "Search available workflow templates (skills) by keyword or category. These are pre-built workflows the agent can run. Use this to find the right skill for the user's request.",
		Input:       "string - keyword to search for (optional)",
		Output:      "string - formatted list of matching templates with category, name, and description",
		Params: []core.ParamSchema{
			{Name: "keyword", Type: "string", Description: "Keyword to search in template name, description, or category", Required: false},
			{Name: "category", Type: "string", Description: "Filter by category name (e.g. finance, healthcare, software-engineering)", Required: false},
		},
	}
}

func (n *templateListNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	category := core.GetParam(params, "category", "")
	keyword := core.GetParam(params, "keyword", "")
	if keyword == "" && input != "" {
		keyword = input
	}

	keyword = strings.ToLower(keyword)

	// Scan external templates (the 300+ skills)
	extTemplates := scanExternalTemplates()

	// Also get built-in templates
	tm := templates.NewTemplateManager()
	builtins := tm.List()

	// Merge and filter
	type entry struct {
		Name        string
		Category    string
		Description string
		Version     string
	}
	var results []entry

	// Add built-in templates
	for _, t := range builtins {
		if category != "" && t.Category != category {
			continue
		}
		if keyword != "" && !matchKeyword(t.Name, t.Description, t.Category, keyword) {
			continue
		}
		results = append(results, entry{
			Name:        t.Name,
			Category:    t.Category,
			Description: t.Description,
			Version:     t.Version,
		})
	}

	// Add external templates
	for _, t := range extTemplates {
		if category != "" && t.Category != category {
			continue
		}
		if keyword != "" && !matchKeyword(t.Name, t.Description, t.Category, keyword) {
			continue
		}
		results = append(results, entry{
			Name:        t.Name,
			Category:    t.Category,
			Description: t.Description,
			Version:     "1.0.0",
		})
	}

	if len(results) == 0 {
		if keyword != "" {
			return fmt.Sprintf("No templates found matching '%s'. Try a different keyword or use /skills to browse categories.", keyword), nil
		}
		return "No templates found.", nil
	}

	// Limit results
	if len(results) > 50 {
		results = results[:50]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Templates (%d found", len(results)))
	if keyword != "" {
		sb.WriteString(fmt.Sprintf(" for '%s'", keyword))
	}
	sb.WriteString("):\n\n")
	sb.WriteString(fmt.Sprintf("%-22s %-22s %s\n", "NAME", "CATEGORY", "DESCRIPTION"))
	sb.WriteString(strings.Repeat("-", 100))
	sb.WriteString("\n")
	for _, t := range results {
		sb.WriteString(fmt.Sprintf("%-22s %-22s %s\n",
			truncateStr(t.Name, 22),
			truncateStr(t.Category, 22),
			t.Description,
		))
	}
	if len(results) >= 50 {
		sb.WriteString("\n... (results truncated, use keyword to narrow search)")
	}
	sb.WriteString("\n\nUse template_info <name> to see details, run_workflow to execute.")

	return sb.String(), nil
}

func matchKeyword(name, description, category, keyword string) bool {
	return strings.Contains(strings.ToLower(name), keyword) ||
		strings.Contains(strings.ToLower(description), keyword) ||
		strings.Contains(strings.ToLower(category), keyword)
}

// ── TemplateInfoNode ────────────────────────────────────────────────────

type templateInfoNode struct{}

func (n *templateInfoNode) Name() string { return "template_info" }
func (n *templateInfoNode) Description() string {
	return "Get detailed info about a template including its YAML content and required parameters"
}

func (n *templateInfoNode) Schema() core.NodeSchema {
	return core.NodeSchema{
		Name:        "template_info",
		Description: "Get detailed info about a specific template. Shows the template's YAML content, parameters, and usage instructions.",
		Input:       "string - template name (e.g. 'stock-screener' or 'finance/stock-screener')",
		Output:      "string - template details with YAML content",
		Params: []core.ParamSchema{
			{Name: "name", Type: "string", Description: "Template name. Can include category prefix: 'finance/stock-screener'", Required: true},
		},
	}
}

func (n *templateInfoNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	name := core.GetParam(params, "name", "")
	if name == "" {
		name = input
	}
	if name == "" {
		return "", fmt.Errorf("template name is required")
	}

	// Handle "category/name" format
	category := ""
	if cat, rest, ok := strings.Cut(name, "/"); ok {
		category = cat
		name = rest
	}

	// Try external templates first
	if info := lookupExternalTemplate(name, category); info != "" {
		return info, nil
	}

	// Fall back to built-in templates
	tm := templates.NewTemplateManager()
	t, err := tm.Get(name)
	if err != nil {
		return "", fmt.Errorf("template not found: %s. Use template_list to see available templates.", name)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Template: %s\n", t.Name))
	sb.WriteString(fmt.Sprintf("Category: %s\n", t.Category))
	sb.WriteString(fmt.Sprintf("Version:  %s\n", t.Version))
	if len(t.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("Tags:     %s\n", strings.Join(t.Tags, ", ")))
	}
	sb.WriteString(fmt.Sprintf("Description: %s\n\n", t.Description))

	if len(t.Variables) > 0 {
		sb.WriteString("Variables:\n")
		for _, v := range t.Variables {
			req := ""
			if v.Required {
				req = " (required)"
			}
			sb.WriteString(fmt.Sprintf("  - %s: %s%s", v.Name, v.Description, req))
			if v.Default != "" {
				sb.WriteString(fmt.Sprintf(" [default: %s]", v.Default))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n--- Template Content ---\n")
	sb.WriteString(t.Content)

	return sb.String(), nil
}

// lookupExternalTemplate finds and reads an external template by name/category.
func lookupExternalTemplate(name, category string) string {
	extTemplates := scanExternalTemplates()
	for _, t := range extTemplates {
		if !strings.EqualFold(t.Name, name) {
			continue
		}
		if category != "" && t.Category != category {
			continue
		}
		data, err := os.ReadFile(t.FilePath)
		if err != nil {
			return ""
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Template: %s\n", t.Name))
		sb.WriteString(fmt.Sprintf("Category: %s\n", t.Category))
		if t.Description != "" {
			sb.WriteString(fmt.Sprintf("Description: %s\n", t.Description))
		}
		sb.WriteString(fmt.Sprintf("File:     %s\n\n", t.FilePath))
		sb.WriteString("--- Template Content ---\n")
		sb.WriteString(string(data))
		return sb.String()
	}
	return ""
}

// ── RunWorkflowNode ─────────────────────────────────────────────────────

type runWorkflowNode struct{}

func (n *runWorkflowNode) Name() string { return "run_workflow" }
func (n *runWorkflowNode) Description() string {
	return "Run a workflow template by name (e.g. 'stock-screener'), file path, or YAML content"
}

func (n *runWorkflowNode) Schema() core.NodeSchema {
	return core.NodeSchema{
		Name:        "run_workflow",
		Description: "Run a workflow template. You can specify a template name (like 'stock-screener' or 'finance/stock-screener'), a file path, or raw YAML content.",
		Input:       "string - template name, YAML content, or file path",
		Output:      "string - workflow execution result",
		Params: []core.ParamSchema{
			{Name: "template", Type: "string", Description: "Template name to run (e.g. 'stock-screener' or 'finance/stock-screener'). Auto-resolves to templates/<category>/<name>/workflow.yaml", Required: false},
			{Name: "file", Type: "string", Description: "Path to workflow YAML file", Required: false},
			{Name: "yaml", Type: "string", Description: "Raw YAML workflow content", Required: false},
		},
	}
}

func (n *runWorkflowNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	reg := core.GetGlobalRegistry()

	var wf *workflow.Workflow
	var err error

	templateName := core.GetParam(params, "template", "")
	file := core.GetParam(params, "file", "")
	yamlStr := core.GetParam(params, "yaml", "")

	// Resolve template name to file path
	if templateName != "" {
		file = resolveTemplatePath(templateName)
		if file == "" {
			return "", fmt.Errorf("template not found: %s. Use template_list to see available templates.", templateName)
		}
	}

	switch {
	case file != "":
		wf, err = workflow.ParseWorkflow(file)
		if err != nil {
			return "", fmt.Errorf("failed to parse workflow file: %w", err)
		}
	case yamlStr != "":
		wf, err = workflow.ParseWorkflowFromContent(yamlStr)
		if err != nil {
			return "", fmt.Errorf("failed to parse workflow YAML: %w", err)
		}
	case input != "":
		// Try resolving input as template name first
		if resolved := resolveTemplatePath(input); resolved != "" {
			wf, err = workflow.ParseWorkflow(resolved)
		} else {
			// Try as YAML content
			wf, err = workflow.ParseWorkflowFromContent(input)
			if err != nil {
				// Fallback: try as file path
				wf, err = workflow.ParseWorkflow(input)
				if err != nil {
					return "", fmt.Errorf("failed to parse workflow: %w", err)
				}
			}
		}
		if err != nil {
			return "", fmt.Errorf("failed to parse workflow: %w", err)
		}
	default:
		return "", fmt.Errorf("template name, file, yaml, or input is required to run a workflow")
	}

	result, _, err := workflow.ExecuteWorkflow(ctx, wf, reg)
	if err != nil {
		return "", fmt.Errorf("workflow execution failed: %w", err)
	}

	// Record template usage metric
	if templateName != "" {
		source := "external"
		// Check if it's a built-in template
		tm := templates.NewTemplateManager()
		if _, err := tm.Get(templateName); err == nil {
			source = "builtin"
		}
		metrics.RecordTemplateUsage(templateName, source)
	}

	return result, nil
}

// resolveTemplatePath converts a template name like "stock-screener" or
// "finance/stock-screener" to the full file path.
func resolveTemplatePath(name string) string {
	extTemplates := scanExternalTemplates()
	for _, t := range extTemplates {
		if strings.EqualFold(t.Name, name) {
			return t.FilePath
		}
		// Match "category/name" format
		full := t.Category + "/" + t.Name
		if strings.EqualFold(full, name) {
			return t.FilePath
		}
	}
	return ""
}

// ── CreateWorkflowNode ──────────────────────────────────────────────────

type createWorkflowNode struct{}

func (n *createWorkflowNode) Name() string { return "create_workflow" }
func (n *createWorkflowNode) Description() string {
	return "Generate a new workflow from a natural language description. Use when no existing template matches."
}

func (n *createWorkflowNode) Schema() core.NodeSchema {
	return core.NodeSchema{
		Name:        "create_workflow",
		Description: "Generate a new workflow YAML from a natural language description. Use this when no existing template matches the user's request. Set save=true to persist as a reusable skill under templates/custom/.",
		Input:       "string - natural language description of the desired workflow",
		Output:      "string - generated workflow YAML content",
		Params: []core.ParamSchema{
			{Name: "name", Type: "string", Description: "Optional name for the workflow", Required: false},
			{Name: "save", Type: "string", Description: "Set to 'true' to save the workflow as a reusable skill under templates/custom/", Required: false},
		},
	}
}

func (n *createWorkflowNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("workflow description is required")
	}

	wf, err := workflow.GenerateWorkflow(input)
	if err != nil {
		return "", fmt.Errorf("failed to generate workflow: %w", err)
	}

	name := core.GetParam(params, "name", "")
	if name != "" {
		wf.Name = name
	}

	yamlBytes, err := yaml.Marshal(wf)
	if err != nil {
		return "", fmt.Errorf("failed to marshal workflow: %w", err)
	}

	save := core.GetParam(params, "save", "")
	if strings.ToLower(save) == "true" {
		saveDir := filepath.Join("templates", "custom")
		if err := os.MkdirAll(saveDir, 0o755); err != nil {
			return "", fmt.Errorf("failed to create custom templates directory: %w", err)
		}

		skillName := strings.ToLower(wf.Name)
		skillName = strings.ReplaceAll(skillName, " ", "_")
		skillName = strings.ReplaceAll(skillName, "-", "_")

		// Strip any path components to prevent directory traversal.
		// LLM-controlled wf.Name could contain "../" or "/" to escape templates/custom/.
		skillName = filepath.Base(skillName)
		if skillName == "." || skillName == ".." || skillName == "" {
			return "", fmt.Errorf("invalid skill name: %q", wf.Name)
		}
		skillDir := filepath.Join(saveDir, skillName)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			return "", fmt.Errorf("failed to create skill directory: %w", err)
		}

		skillPath := filepath.Join(skillDir, "workflow.yaml")
		if err := os.WriteFile(skillPath, yamlBytes, 0o644); err != nil {
			return "", fmt.Errorf("failed to save skill: %w", err)
		}

		return fmt.Sprintf("Workflow saved as reusable skill: templates/custom/%s/workflow.yaml\n\n%s", skillName, string(yamlBytes)), nil
	}

	return string(yamlBytes), nil
}

// ── SelfUpdateNode ──────────────────────────────────────────────────────

type selfUpdateNode struct{}

func (n *selfUpdateNode) Name() string { return "self_update" }
func (n *selfUpdateNode) Description() string {
	return "Check for aflare updates and install the latest version. Use when the user asks to upgrade or if you detect the version is outdated."
}

func (n *selfUpdateNode) Schema() core.NodeSchema {
	return core.NodeSchema{
		Name:        "self_update",
		Description: "Check for aflare updates and install the latest version from GitHub releases. Downloads and verifies the binary with checksum validation.",
		Input:       "string - optional, ignored",
		Output:      "string - update result (already up to date, or updated to version X)",
		Params: []core.ParamSchema{
			{Name: "check_only", Type: "string", Description: "Set to 'true' to only check for updates without installing", Required: false},
		},
	}
}

func (n *selfUpdateNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	checkOnly := core.GetParam(params, "check_only", "")
	if strings.ToLower(checkOnly) == "true" {
		release, err := meta.CheckLatestRelease("alib8b8/aflare")
		if err != nil {
			return "", fmt.Errorf("failed to check for updates: %w", err)
		}
		if meta.HasUpdate(meta.Version, release) {
			return fmt.Sprintf("Update available: %s (current: %s)\nRelease notes: %s", release.TagName, meta.Version, release.Body), nil
		}
		return fmt.Sprintf("Already up to date (current: %s, latest: %s)", meta.Version, release.TagName), nil
	}

	// In safe mode, refuse to install updates — the LLM should not be able
	// to unilaterally replace the binary. Use check_only=true instead.
	if nodes.IsSafeMode() {
		return "", fmt.Errorf("self-update installation is disabled in safe mode; use check_only=true to check for updates")
	}

	result, err := meta.SelfUpdate("alib8b8/aflare")
	if err != nil {
		return "", fmt.Errorf("self-update failed: %w", err)
	}
	return result, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// registerChatNodes registers the chat-specific nodes in the global registry.
func registerChatNodes(reg *core.Registry) {
	reg.Register(&templateListNode{})
	reg.Register(&templateInfoNode{})
	reg.Register(&runWorkflowNode{})
	reg.Register(&createWorkflowNode{})
	reg.Register(&selfUpdateNode{})
	reg.Register(&nodes.MemoryNode{})
	reg.Register(&nodes.CompressNode{})
}

// ListCategories returns a summary of all template categories with counts.
func ListCategories() string {
	extTemplates := scanExternalTemplates()
	catCount := make(map[string]int)
	for _, t := range extTemplates {
		catCount[t.Category]++
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Template categories (%d total):\n\n", len(extTemplates)))
	for _, cat := range TemplateCategories {
		count := catCount[cat]
		if count > 0 {
			sb.WriteString(fmt.Sprintf("  %-22s %d skills\n", cat, count))
		}
	}
	return sb.String()
}
