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

// Package marketplace implements the aflare Workflow Marketplace — a package
// manager that lets users discover, install, and manage workflow packages.
// Workflows are installed from the marketplace registry into the user's
// ~/.aflare/workflows/ directory.
package marketplace

import (
	"fmt"
	"os"
	"path/filepath"
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
	r.packages = []Package{
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
		{
			Name:        "unitree-patrol",
			Version:     "1.0.0",
			Description: "Autonomous robot patrol routine using Unitree robot with vision-based inspection",
			Category:    "robot",
			Author:      "aflare",
			WorkflowYAML: `name: "Unitree Patrol Routine"
description: "Execute an autonomous patrol route with the Unitree robot, capturing images and detecting anomalies"
steps:
  - node: robot_control
    id: initialize
    params:
      action: "stand"
      robot_model: "unitree_go2"
  - node: robot_action
    id: patrol_waypoint_1
    params:
      action: "navigate"
      x: "0"
      y: "5"
      speed: "0.5"
  - node: robot_control
    id: capture_image_1
    params:
      action: "capture_image"
      camera: "front"
      save_path: "patrol/waypoint_1.jpg"
  - node: robot_action
    id: patrol_waypoint_2
    params:
      action: "navigate"
      x: "5"
      y: "5"
      speed: "0.5"
  - node: robot_control
    id: capture_image_2
    params:
      action: "capture_image"
      camera: "front"
      save_path: "patrol/waypoint_2.jpg"
  - node: llm
    id: analyze_images
    params:
      model: "gpt-4-vision"
      prompt: "Analyze the patrol images for anomalies: unauthorized personnel, equipment damage, or environmental hazards."
  - node: robot_control
    id: return_home
    params:
      action: "navigate"
      x: "0"
      y: "0"
      speed: "0.5"
  - node: notify
    id: patrol_report
    params:
      channel: "telegram"
      message: "Patrol completed. Anomalies: {{anomalies}}"
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

	workflowsDir := filepath.Join(meta.DataDir(), "workflows")
	if err := os.MkdirAll(workflowsDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create workflows directory: %w", err)
	}

	destPath := filepath.Join(workflowsDir, pkg.Name+".yaml")
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
	files := append(yamlFiles, ymlFiles...)

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
