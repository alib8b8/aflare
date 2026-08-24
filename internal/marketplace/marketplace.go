// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌​​​‌​‌​​‌​​​​‌‌‌​​​‌​​‌​‌​‌​​‌‌​​​‌​​​‌​‌‌‌​‌​​​​​​​​​​​​​​​​‌‌‌​‌​​‌​​​​​​‌‌⁠
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

// Package marketplace implements the aflare Workflow Marketplace — a package
// manager that lets users discover, install, and manage workflow packages.
// Workflows are installed from the marketplace registry into the user's
// ~/.aflare/workflows/ directory.
//
// Agent Plugins 1.0.0 compatibility: packages can be exported as Agent Plugins
// (plugin.json + skills/ + mcp.json) and imported from Agent Plugins format.
// This enables aflare workflows to be packaged once and used across compatible
// agent clients (VS Code, Cursor, GitHub Copilot, ChatGPT, etc.).
package marketplace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/alib8b8/aflare/internal/meta"
)

// Package represents a workflow package in the marketplace.
type Package struct {
	Name         string `yaml:"name" json:"name"`
	Version      string `yaml:"version" json:"version"`
	Description  string `yaml:"description" json:"description"`
	Category     string `yaml:"category" json:"category"`
	Author       string `yaml:"author" json:"author"`
	WorkflowYAML string `yaml:"-" json:"-"`
}

// Registry holds a collection of marketplace packages.
type Registry struct {
	packages []Package
}

// NewRegistry creates a new Registry seeded with built-in starter packages.
func NewRegistry() *Registry {
	r := &Registry{}
	r.registerBuiltins()
	return r
}

// registerBuiltins populates the registry with five starter workflow packages.
func (r *Registry) registerBuiltins() {
	r.packages = append(r.packages, r.builtinCorePackages()...)
	r.packages = append(r.packages, r.builtinDomainPackages()...)
	r.packages = append(r.packages, r.builtinUtilityPackages()...)
}

func (r *Registry) builtinCorePackages() []Package {
	return []Package{
		{
			Name:        "btc-monitor",
			Version:     "1.0.0",
			Description: "Monitor Bitcoin price movements and send alerts via Telegram",
			Category:    "finance",
			Author:      "aflare",
			WorkflowYAML: `name: "BTC Monitor"
description: "Fetch Bitcoin price from CoinGecko and send Telegram alert on threshold breach"
steps:
  - node: fetch_url
    id: fetch_price
    params:
      url: "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd"
  - node: json_parse
    id: parse_price
    params:
      path: "input.bitcoin.usd"
  - node: condition
    id: check_threshold
    params:
      condition: "value > 60000"
    then:
      - node: notify
        id: send_alert
        params:
          channel: "telegram"
          message: "BTC price alert: {{value}} USD"
`,
		},
		{
			Name:        "github-alert",
			Version:     "1.0.0",
			Description: "Aggregate GitHub repository activity and send daily digest",
			Category:    "devops",
			Author:      "aflare",
			WorkflowYAML: `name: "GitHub Daily Digest"
description: "Fetch recent issues, PRs, and commits from a GitHub repo and generate a daily summary"
steps:
  - node: fetch_url
    id: fetch_issues
    params:
      url: "https://api.github.com/repos/{{repo}}/issues?state=open&per_page=10"
      headers:
        Accept: "application/vnd.github.v3+json"
  - node: http_request
    id: fetch_prs
    params:
      url: "https://api.github.com/repos/{{repo}}/pulls?state=open&per_page=10"
      method: "GET"
      headers:
        Accept: "application/vnd.github.v3+json"
  - node: llm
    id: summarize
    params:
      prompt: "Summarize the following GitHub activity into a daily digest:\n\nIssues: {{issues}}\n\nPRs: {{prs}}"
  - node: notify
    id: send_digest
    params:
      channel: "email"
      subject: "GitHub Daily Digest - {{repo}}"
      message: "{{summary}}"
`,
		},
	}
}

func (r *Registry) builtinDomainPackages() []Package {
	return []Package{
		{
			Name:        "arxiv-daily",
			Version:     "1.0.0",
			Description: "Fetch latest arXiv papers by category and generate research summaries",
			Category:    "research",
			Author:      "aflare",
			WorkflowYAML: `name: "arXiv Daily Paper Digest"
description: "Query arXiv API for latest papers in specified categories and generate AI-powered summaries"
steps:
  - node: fetch_url
    id: fetch_papers
    params:
      url: "http://export.arxiv.org/api/query?search_query=cat:{{category}}&sortBy=submittedDate&sortOrder=descending&max_results=5"
  - node: doc_parse
    id: parse_feed
    params:
      format: "xml"
  - node: llm
    id: summarize_papers
    params:
      model: "gpt-4"
      prompt: "Summarize the following arXiv papers in 2-3 bullet points each:\n\n{{papers}}"
  - node: file_write
    id: save_digest
    params:
      path: "arxiv-daily-{{date}}.md"
      content: "{{summary}}"
`,
		},
		{
			Name:        "financial-aml",
			Version:     "1.0.0",
			Description: "Anti-Money Laundering transaction screening and risk scoring workflow",
			Category:    "finance",
			Author:      "aflare",
			WorkflowYAML: `name: "AML Transaction Screening"
description: "Screen transactions against sanctions lists, perform risk scoring, and flag suspicious activity"
steps:
  - node: fetch_url
    id: fetch_transactions
    params:
      url: "{{api_endpoint}}/transactions?date={{date}}"
  - node: json_parse
    id: parse_tx
    params:
      path: "input.transactions"
  - node: llm
    id: risk_assessment
    params:
      model: "gpt-4"
      prompt: "Analyze the following transactions for AML risk. Flag any transactions that match patterns: large round amounts, high-risk jurisdictions, rapid movement between accounts. Return risk score (0-100) and reasoning.\n\nTransactions: {{transactions}}"
  - node: condition
    id: check_high_risk
    params:
      condition: "risk_score > 70"
    then:
      - node: notify
        id: alert_compliance
        params:
          channel: "email"
          subject: "AML Alert: High Risk Transaction Detected"
          message: "Risk Score: {{risk_score}}\n\nDetails: {{risk_details}}"
  - node: file_write
    id: save_report
    params:
      path: "aml-report-{{date}}.json"
      content: "{{report}}"
`,
		},
	}
}

func (r *Registry) builtinUtilityPackages() []Package {
	return []Package{
		{
			Name:        "habit-tracker",
			Version:     "1.0.0",
			Description: "Track personal habits and send motivational reminders — inspired by Pace",
			Category:    "health",
			Author:      "aflare",
			WorkflowYAML: `name: "Habit Tracker"
description: "Track daily habits, count progress, and receive motivational reminders at configurable intervals"
schedule:
  cron: "0 */2 * * *"
steps:
  - node: file_read
    id: load_state
    params:
      path: "~/.aflare/habit-tracker.json"
    on_error:
      - node: file_write
        id: init_state
        params:
          path: "~/.aflare/habit-tracker.json"
          content: '{"habit": "{{habit_name}}", "streak": 0, "total": 0, "last_check": "{{now}}"}'
  - node: json_parse
    id: parse_state
    params:
      path: "input"
  - node: counter
    id: update_counter
    params:
      value: "{{streak}}"
      increment: 1
  - node: condition
    id: check_streak
    params:
      condition: "{{streak}} > 0"
    then:
      - node: notify
        id: send_motivation
        params:
          channel: "telegram"
          message: "{{habit_name}} streak: {{streak}} days! You've tracked {{total}} times total. Keep going!"
  - node: condition
    id: check_milestone
    params:
      condition: "{{streak}} % 7 == 0"
    then:
      - node: notify
        id: milestone_alert
        params:
          channel: "telegram"
          message: "Milestone: {{habit_name}} streak reached {{streak}} days! Consider rewarding yourself."
  - node: file_write
    id: save_state
    params:
      path: "~/.aflare/habit-tracker.json"
      content: "{{state}}"
`,
		},
	}
}

// List returns all packages in the registry, sorted by name.
func (r *Registry) List() []Package {
	sorted := make([]Package, len(r.packages))
	copy(sorted, r.packages)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}

// Search finds packages whose name, description, or category matches the
// given keyword (case-insensitive).
func (r *Registry) Search(keyword string) []Package {
	if keyword == "" {
		return nil
	}
	kw := strings.ToLower(keyword)
	var results []Package
	for _, pkg := range r.packages {
		if strings.Contains(strings.ToLower(pkg.Name), kw) ||
			strings.Contains(strings.ToLower(pkg.Description), kw) ||
			strings.Contains(strings.ToLower(pkg.Category), kw) {
			results = append(results, pkg)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	return results
}

// ListByCategory returns packages filtered by category (case-insensitive).
func (r *Registry) ListByCategory(category string) []Package {
	if category == "" {
		return nil
	}
	cat := strings.ToLower(category)
	var results []Package
	for _, pkg := range r.packages {
		if strings.ToLower(pkg.Category) == cat {
			results = append(results, pkg)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	return results
}

// Get retrieves a package by name (case-insensitive match).
func (r *Registry) Get(name string) (*Package, error) {
	if name == "" {
		return nil, fmt.Errorf("package name is required")
	}
	nm := strings.ToLower(name)
	for i := range r.packages {
		if strings.ToLower(r.packages[i].Name) == nm {
			pkg := r.packages[i]
			return &pkg, nil
		}
	}
	return nil, fmt.Errorf("package %q not found in marketplace", name)
}

// Install installs a workflow package by name into the user's workflows
// directory (~/.aflare/workflows/<name>.yaml). Returns the path where the
// workflow YAML was written.
func (r *Registry) Install(name string) (string, error) {
	return r.InstallTo(name, filepath.Join(meta.DataDir(), "workflows"))
}

// InstallTo installs a workflow package by name into the specified target
// directory. Returns the path where the workflow YAML was written.
func (r *Registry) InstallTo(name, targetDir string) (string, error) {
	pkg, err := r.Get(name)
	if err != nil {
		return "", err
	}

	if pkg.WorkflowYAML == "" {
		return "", fmt.Errorf("package %q has no workflow YAML to install", name)
	}

	if err := isValidPackageName(pkg.Name); err != nil {
		return "", err
	}

	if err := os.MkdirAll(targetDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create target directory: %w", err)
	}

	destPath := filepath.Join(targetDir, pkg.Name+".yaml")
	if err := os.WriteFile(destPath, []byte(pkg.WorkflowYAML), 0600); err != nil {
		return "", fmt.Errorf("failed to write workflow file: %w", err)
	}

	return destPath, nil
}

// Uninstall removes an installed workflow package by name.
func (r *Registry) Uninstall(name string) error {
	if err := isValidPackageName(name); err != nil {
		return err
	}

	workflowsDir := filepath.Join(meta.DataDir(), "workflows")
	destPath := filepath.Join(workflowsDir, name+".yaml")

	if err := os.Remove(destPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("workflow %q is not installed", name)
		}
		return fmt.Errorf("failed to uninstall workflow: %w", err)
	}

	return nil
}

// ListInstalled returns a sorted list of installed workflow names.
func (r *Registry) ListInstalled() ([]string, error) {
	workflowsDir := filepath.Join(meta.DataDir(), "workflows")

	yamlFiles, err := filepath.Glob(filepath.Join(workflowsDir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}
	ymlFiles, err := filepath.Glob(filepath.Join(workflowsDir, "*.yml"))
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}
	files := slices.Concat(yamlFiles, ymlFiles)

	var names []string
	for _, f := range files {
		names = append(names, strings.TrimSuffix(filepath.Base(f), filepath.Ext(f)))
	}
	sort.Strings(names)

	return names, nil
}

// Categories returns a sorted list of unique categories in the registry.
func (r *Registry) Categories() []string {
	seen := make(map[string]struct{})
	for _, pkg := range r.packages {
		seen[pkg.Category] = struct{}{}
	}
	cats := make([]string, 0, len(seen))
	for c := range seen {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	return cats
}

// ── Agent Plugins 1.0.0 Compatibility ──────────────────────────────────────
//
// Agent Plugins is an open, vendor-neutral specification (agent-plugins.org)
// backed by OpenAI, Google, Amazon, Microsoft, Cursor, and Vercel. It defines
// a common package format for Agent Skills and MCP servers so they can be
// packaged once and used across compatible clients.
//
// aflare workflows can be exported as Agent Plugins, making them usable in
// VS Code, Cursor, GitHub Copilot, ChatGPT, and other compatible clients.

// PluginManifest represents the plugin.json manifest defined by the
// Agent Plugins 1.0.0 specification.
type PluginManifest struct {
	Schema      string   `json:"$schema"`
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	Description string   `json:"description,omitempty"`
	Author      string   `json:"author,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Repository  string   `json:"repository,omitempty"`
	License     string   `json:"license,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
}

// ToPluginManifest converts a marketplace Package into an Agent Plugins
// plugin.json manifest.
func (p *Package) ToPluginManifest() PluginManifest {
	return PluginManifest{
		Schema:      "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		Name:        p.Name,
		Version:     p.Version,
		Description: p.Description,
		Author:      p.Author,
		License:     "AGPL-3.0",
		Keywords:    []string{"aflare", "workflow", p.Category},
	}
}

// ExportPlugin exports a marketplace package as an Agent Plugins-compatible
// directory. The directory structure follows the Agent Plugins 1.0.0 spec:
//
//	<name>/
//	├── plugin.json
//	├── skills/
//	│   └── <name>/
//	│       └── SKILL.md
//	└── mcp.json
//
// The workflow YAML is embedded as a Skill so compatible agents can discover
// and execute it.
func (r *Registry) ExportPlugin(name, targetDir string) (string, error) {
	pkg, err := r.Get(name)
	if err != nil {
		return "", err
	}

	pluginDir := filepath.Join(targetDir, pkg.Name)
	if err := os.MkdirAll(pluginDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Write plugin.json
	manifest := pkg.ToPluginManifest()
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal plugin.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), manifestBytes, 0644); err != nil {
		return "", fmt.Errorf("failed to write plugin.json: %w", err)
	}

	// Write skills/<name>/SKILL.md with the workflow YAML as instructions
	skillsDir := filepath.Join(pluginDir, "skills", pkg.Name)
	if err := os.MkdirAll(skillsDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create skills directory: %w", err)
	}
	skillContent := fmt.Sprintf(`---
name: %s
description: %s
---

# %s

%s

## Workflow YAML

This skill contains an aflare workflow. Install aflare and run:

`+"```bash\n"+`aflare install %s
aflare run %s
`+"```\n"+`
`, pkg.Name, pkg.Description, pkg.Name, pkg.Description, pkg.Name, pkg.Name)
	if err := os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write SKILL.md: %w", err)
	}

	// Write mcp.json with aflare MCP server config
	mcpConfig := map[string]interface{}{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
		"mcpServers": map[string]interface{}{
			"aflare": map[string]interface{}{
				"type":    "stdio",
				"command": "aflare",
				"args":    []string{"--mcp"},
			},
		},
	}
	mcpBytes, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal mcp.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "mcp.json"), mcpBytes, 0644); err != nil {
		return "", fmt.Errorf("failed to write mcp.json: %w", err)
	}

	return pluginDir, nil
}

// ImportPlugin scans a directory for an Agent Plugins-compatible plugin.json
// and returns the extracted metadata. Returns an error if the directory does
// not contain a valid plugin.json.
func ImportPlugin(pluginDir string) (*PluginManifest, error) {
	manifestPath := filepath.Join(pluginDir, "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("plugin.json not found in %s: %w", pluginDir, err)
	}

	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("invalid plugin.json: %w", err)
	}

	if manifest.Schema == "" || manifest.Name == "" {
		return nil, fmt.Errorf("plugin.json missing required fields ($schema, name)")
	}

	return &manifest, nil
}

// isValidPackageName checks that a package name is safe for filesystem use.
func isValidPackageName(name string) error {
	if name == "" || len(name) > 100 {
		return fmt.Errorf("invalid package name: %q (must be 1-100 characters)", name)
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return fmt.Errorf("invalid package name %q (only alphanumeric, hyphens, underscores allowed)", name)
		}
	}
	return nil
}
