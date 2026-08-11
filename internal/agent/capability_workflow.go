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

// capability_workflow.go implements WorkflowCapability —
// predefined workflow/pipeline execution with template discovery,
// validation, and execution tracking.
//
// This implements the "工作流/管道式 Agent" type from the taxonomy:
//   Enforces predefined execution patterns, encouraging template-first
//   approaches. When no template matches, guides the agent through
//   composing a new workflow from available nodes.

package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// WorkflowExecution represents a single workflow execution record.
type WorkflowExecution struct {
	Template  string `json:"template"`
	StartedAt string `json:"started_at"`
	Status    string `json:"status"` // "running", "success", "failed"
	Steps     int    `json:"steps"`
	Error     string `json:"error,omitempty"`
}

// WorkflowCapability enforces predefined workflow execution patterns.
// It tracks execution history, validates templates, and provides
// guidance on template selection and composition.
type WorkflowCapability struct {
	mu       sync.RWMutex
	history  []WorkflowExecution
	templates map[string]TemplateMeta // cached template metadata
}

// TemplateMeta holds minimal template metadata for quick lookup.
type TemplateMeta struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Category    string   `yaml:"category"`
	Steps       []string `yaml:"steps"`
}

func NewWorkflowCapability() *WorkflowCapability {
	return &WorkflowCapability{
		history:   make([]WorkflowExecution, 0),
		templates: make(map[string]TemplateMeta),
	}
}

func (w *WorkflowCapability) Name() string       { return "workflow" }
func (w *WorkflowCapability) Description() string { return "Predefined workflow execution: stable, predictable pipeline steps (工作流/管道式 Agent)" }

func (w *WorkflowCapability) Init(loop *AgentLoop) error {
	// Scan templates directory for available workflows
	w.scanTemplates()
	return nil
}

func (w *WorkflowCapability) PreProcess(ctx context.Context, input string) (string, error) {
	w.mu.RLock()
	templateCount := len(w.templates)
	recentHistory := make([]WorkflowExecution, len(w.history))
	copy(recentHistory, w.history)
	w.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("\n[Workflow Mode — Pipeline Execution]\n")

	// Show recent execution history if any
	if len(recentHistory) > 0 {
		sb.WriteString("Recent workflow executions:\n")
		start := 0
		if len(recentHistory) > 3 {
			start = len(recentHistory) - 3
		}
		for _, h := range recentHistory[start:] {
			icon := "✓"
			if h.Status == "failed" {
				icon = "✗"
			}
			sb.WriteString(fmt.Sprintf("  %s %s (%s)\n", icon, h.Template, h.Status))
		}
		sb.WriteString("\n")
	}

	// Guidance for template selection
	sb.WriteString(fmt.Sprintf("%d templates available. ", templateCount))
	sb.WriteString("Prefer existing templates: use template_list to find a matching workflow, ")
	sb.WriteString("template_info to inspect parameters, and run_workflow to execute. ")
	sb.WriteString("Only compose new workflows with create_workflow when no template matches.\n")

	// If input suggests a new task, suggest matching templates
	if suggestsTask(input) {
		sb.WriteString("Hint: use template_list with keywords from your request to find related workflows.\n")
	}

	return input + sb.String(), nil
}

func (w *WorkflowCapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	lowerOutput := strings.ToLower(output)

	// Detect workflow execution
	if strings.Contains(lowerOutput, "run_workflow") || strings.Contains(lowerOutput, "running workflow") {
		w.mu.Lock()
		exec := WorkflowExecution{
			Template:  w.extractTemplateName(output),
			StartedAt: time.Now().UTC().Format(time.RFC3339),
			Status:    "running",
		}
		w.history = append(w.history, exec)
		if len(w.history) > 50 {
			w.history = w.history[len(w.history)-50:]
		}
		w.mu.Unlock()
	}

	// Detect workflow completion
	if strings.Contains(lowerOutput, "completed") || strings.Contains(lowerOutput, "finished") ||
		strings.Contains(lowerOutput, "workflow result") {
		w.mu.Lock()
		for i := len(w.history) - 1; i >= 0; i-- {
			if w.history[i].Status == "running" {
				if strings.Contains(lowerOutput, "error") || strings.Contains(lowerOutput, "failed") {
					w.history[i].Status = "failed"
					w.history[i].Error = truncateStr(output, 200)
				} else {
					w.history[i].Status = "success"
				}
				break
			}
		}
		w.mu.Unlock()
	}

	// Detect template creation
	if strings.Contains(lowerOutput, "create_workflow") || strings.Contains(lowerOutput, "creating workflow") {
		w.mu.Lock()
		// Refresh template cache after creation
		w.scanTemplates()
		w.mu.Unlock()
	}

	return "", nil
}

func (w *WorkflowCapability) Shutdown() error { return nil }

// scanTemplates scans the templates directory for available workflows.
func (w *WorkflowCapability) scanTemplates() {
	// Look for templates in common locations
	searchPaths := []string{
		"templates",
		filepath.Join("..", "templates"),
	}

	// Try to find the templates directory relative to the working directory
	cwd, err := os.Getwd()
	if err == nil {
		searchPaths = append(searchPaths,
			filepath.Join(cwd, "templates"),
			filepath.Join(cwd, "..", "templates"),
		)
	}

	for _, basePath := range searchPaths {
		entries, err := os.ReadDir(basePath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			// Skip special directories
			if entry.Name() == "custom" || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			// Look for workflow.yaml in each template directory
			wfPath := filepath.Join(basePath, entry.Name(), "workflow.yaml")
			meta := w.parseTemplateMeta(wfPath)
			if meta != nil {
				meta.Category = entry.Name()
				w.templates[meta.Name] = *meta
			}
		}
		// Also scan custom templates
		customPath := filepath.Join(basePath, "custom")
		customEntries, err := os.ReadDir(customPath)
		if err == nil {
			for _, entry := range customEntries {
				if !entry.IsDir() {
					continue
				}
				wfPath := filepath.Join(customPath, entry.Name(), "workflow.yaml")
				meta := w.parseTemplateMeta(wfPath)
				if meta != nil {
					meta.Category = "custom"
					w.templates[meta.Name] = *meta
				}
			}
		}
		break // Use the first valid templates directory
	}
}

// parseTemplateMeta extracts metadata from a workflow YAML file.
func (w *WorkflowCapability) parseTemplateMeta(path string) *TemplateMeta {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var meta TemplateMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil
	}

	if meta.Name == "" {
		return nil
	}

	// Extract step names from the workflow
	var workflow struct {
		Steps []struct {
			Name string `yaml:"name"`
		} `yaml:"steps"`
	}
	if err := yaml.Unmarshal(data, &workflow); err == nil {
		for _, s := range workflow.Steps {
			if s.Name != "" {
				meta.Steps = append(meta.Steps, s.Name)
			}
		}
	}

	return &meta
}

// extractTemplateName tries to extract the template name from the output.
func (w *WorkflowCapability) extractTemplateName(output string) string {
	lowerOutput := strings.ToLower(output)

	// Check for template names from the cache first (longest match wins)
	bestName := ""
	bestLen := 0
	for name := range w.templates {
		lowerName := strings.ToLower(name)
		if strings.Contains(lowerOutput, lowerName) && len(name) > bestLen {
			bestName = name
			bestLen = len(name)
		}
	}
	if bestName != "" {
		return bestName
	}

	// Try to find template name patterns
	patterns := []string{
		"template:", "workflow:", "executing",
	}
	for _, p := range patterns {
		idx := strings.Index(lowerOutput, p)
		if idx >= 0 {
			rest := strings.TrimSpace(output[idx+len(p):])
			// Take first word or quoted string
			if strings.HasPrefix(rest, "\"") || strings.HasPrefix(rest, "'") {
				rest = rest[1:]
				end := strings.IndexAny(rest, "\"'")
				if end > 0 {
					return rest[:end]
				}
			}
			parts := strings.Fields(rest)
			if len(parts) > 0 {
				return parts[0]
			}
		}
	}

	// For "running <template>" pattern, skip the verb
	verbs := []string{"running", "run", "executing", "execute"}
	for _, verb := range verbs {
		idx := strings.Index(lowerOutput, verb+" ")
		if idx >= 0 {
			rest := strings.TrimSpace(output[idx+len(verb)+1:])
			// Take two words to capture multi-word template names
			parts := strings.Fields(rest)
			if len(parts) >= 2 {
				return parts[0] + "-" + parts[1]
			}
			if len(parts) == 1 {
				return parts[0]
			}
		}
	}

	return "unknown"
}

// suggestsTask checks if the input suggests a new task that could use a template.
func suggestsTask(input string) bool {
	lower := strings.ToLower(input)
	taskWords := []string{"create", "build", "make", "setup", "configure", "deploy",
		"generate", "analyze", "scan", "check", "test", "run"}
	for _, w := range taskWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}