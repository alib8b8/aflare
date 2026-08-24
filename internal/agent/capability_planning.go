// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​‌‌​‌​​​‌​​‌​‌‌​​‌‌‌‌​​​‌‌​​‌‌‌​‌‌‌‌​‌‌‌​​‌‌‌‌​​​​​​​​​​​​​​​​‌‌‌​​​‌​‌‌‌​​‌​​⁠
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

// capability_planning.go implements PlanningCapability — goal-driven
// multi-step planning with task decomposition, plan tracking, and
// execution monitoring.
//
// This implements the "规划式 Agent" type from the taxonomy:
//   Generates action sequences before execution, tracks progress
//   through each step, and adapts the plan when execution diverges.

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PlanStep represents a single step in an execution plan.
type PlanStep struct {
	ID     int    `json:"id"`
	Goal   string `json:"goal"`
	Status string `json:"status"` // "pending", "in_progress", "done", "failed"
	Tool   string `json:"tool,omitempty"`
}

// Plan represents a multi-step execution plan.
type Plan struct {
	ID        string     `json:"id"`
	Goal      string     `json:"goal"`
	Steps     []PlanStep `json:"steps"`
	CreatedAt string     `json:"created_at"`
	UpdatedAt string     `json:"updated_at"`
}

// PlanningCapability adds explicit goal-driven planning to the agent.
// It decomposes complex goals into steps, tracks progress, and persists
// active plans across sessions for recovery.
type PlanningCapability struct {
	mu         sync.RWMutex
	activePlan *Plan
	completed  []Plan
	storePath  string
}

func NewPlanningCapability() *PlanningCapability {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "aflare")
	_ = os.MkdirAll(dir, 0o755)
	return &PlanningCapability{
		completed: make([]Plan, 0),
		storePath: filepath.Join(dir, "plans.json"),
	}
}

func (p *PlanningCapability) Name() string { return CapabilityPlanning }
func (p *PlanningCapability) Description() string {
	return "Goal-driven planning: generates action sequences and tracks progress (规划式 Agent)"
}

func (p *PlanningCapability) Init(loop *AgentLoop) error {
	// Load active plan from previous session if any
	data, err := os.ReadFile(p.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no saved plan is fine
		}
		return fmt.Errorf("read plan store: %w", err)
	}
	var saved struct {
		Active    *Plan  `json:"active"`
		Completed []Plan `json:"completed"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		return fmt.Errorf("unmarshal plan store: %w", err)
	}
	p.activePlan = saved.Active
	if saved.Completed != nil {
		p.completed = saved.Completed
	}
	return nil
}

func (p *PlanningCapability) PreProcess(ctx context.Context, input string) (string, error) {
	p.mu.RLock()
	hasPlan := p.activePlan != nil
	p.mu.RUnlock()

	var sb strings.Builder

	if hasPlan {
		// Continue existing plan — show current state
		p.mu.RLock()
		plan := p.activePlan
		p.mu.RUnlock()

		pending := 0
		done := 0
		for _, s := range plan.Steps {
			switch s.Status {
			case "done":
				done++
			case "pending", "in_progress":
				pending++
			}
		}

		sb.WriteString("\n[Planning Mode — Active Plan]\n")
		sb.WriteString(fmt.Sprintf("Goal: %s\n", plan.Goal))
		sb.WriteString(fmt.Sprintf("Progress: %d/%d steps complete\n", done, len(plan.Steps)))
		sb.WriteString("Steps:\n")
		for _, s := range plan.Steps {
			icon := "○"
			switch s.Status {
			case "done":
				icon = "✓"
			case "in_progress":
				icon = "▶"
			case "failed":
				icon = "✗"
			}
			sb.WriteString(fmt.Sprintf("  %s [%s] %s", icon, s.Status, s.Goal))
			if s.Tool != "" {
				sb.WriteString(fmt.Sprintf(" (use %s)", s.Tool))
			}
			sb.WriteString("\n")
		}
		if pending > 0 {
			sb.WriteString("Continue executing the plan step by step. Mark each step as done when complete.\n")
		} else {
			sb.WriteString("All steps complete! Provide a summary and mark the plan as finished.\n")
		}
	} else if len(input) > 80 || containsActionVerb(input) {
		// No active plan — encourage planning for complex tasks
		sb.WriteString("\n[Planning Mode] This looks like a complex task. Before acting:\n")
		sb.WriteString("1) Identify the goal\n")
		sb.WriteString("2) Break it into concrete steps\n")
		sb.WriteString("3) Choose the right tool for each step\n")
		sb.WriteString("4) Execute step by step, tracking progress\n")
		sb.WriteString("Start your response with 'Plan:' followed by numbered steps, then execute each step.\n")
	}

	if sb.Len() == 0 {
		return "", nil
	}
	return input + sb.String(), nil
}

func (p *PlanningCapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	lowerOutput := strings.ToLower(output)

	// Detect plan creation: "Plan:" followed by numbered steps
	if strings.Contains(lowerOutput, "plan:") && (strings.Contains(lowerOutput, "step 1") || strings.Contains(lowerOutput, "1.")) {
		p.mu.Lock()
		p.activePlan = p.extractPlan(output)
		p.mu.Unlock()
		if p.activePlan != nil {
			if err := p.persist(); err != nil {
				log.Printf("[planning] persist failed: %v", err)
			}
		}
	}

	// Detect step completion
	if p.activePlan != nil {
		p.mu.Lock()
		updated := false
		for i := range p.activePlan.Steps {
			if p.activePlan.Steps[i].Status == "in_progress" {
				// Check if this step's goal is mentioned as done
				if p.isStepCompleted(p.activePlan.Steps[i].Goal, output) {
					p.activePlan.Steps[i].Status = "done"
					updated = true
					// Start next step
					if i+1 < len(p.activePlan.Steps) {
						p.activePlan.Steps[i+1].Status = "in_progress"
					}
				}
				break
			}
			if p.activePlan.Steps[i].Status == "pending" {
				p.activePlan.Steps[i].Status = "in_progress"
				updated = true
				break
			}
		}

		// Check if all steps are done
		allDone := true
		for _, s := range p.activePlan.Steps {
			if s.Status != "done" && s.Status != "failed" {
				allDone = false
				break
			}
		}
		if allDone {
			p.activePlan.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			p.completed = append(p.completed, *p.activePlan)
			if len(p.completed) > 20 {
				p.completed = p.completed[len(p.completed)-20:]
			}
			p.activePlan = nil
			updated = true
		}

		if updated {
			if err := p.persist(); err != nil {
				log.Printf("[planning] persist failed: %v", err)
			}
		}
		p.mu.Unlock()
	}

	return "", nil
}

func (p *PlanningCapability) Shutdown() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.persist()
}

// extractPlan parses a plan from the agent's output.
func (p *PlanningCapability) extractPlan(output string) *Plan {
	steps := make([]PlanStep, 0)
	lines := strings.Split(output, "\n")
	inPlan := false
	stepNum := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		if strings.HasPrefix(lower, "plan:") {
			inPlan = true
			continue
		}

		if !inPlan {
			continue
		}

		// Detect numbered steps: "1. do X", "Step 1: do X", "- [ ] do X"
		if isStepLine(trimmed) {
			stepNum++
			goal := extractStepGoal(trimmed)
			tool := inferToolForStep(goal)
			steps = append(steps, PlanStep{
				ID:     stepNum,
				Goal:   goal,
				Status: "pending",
				Tool:   tool,
			})
		}
	}

	if len(steps) == 0 {
		return nil
	}

	// First step starts as in_progress
	steps[0].Status = "in_progress"

	return &Plan{
		ID:        fmt.Sprintf("plan_%d", time.Now().Unix()),
		Goal:      extractGoal(output),
		Steps:     steps,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// isStepCompleted checks if a step goal is reflected in the output.
func (p *PlanningCapability) isStepCompleted(goal, output string) bool {
	lowerOutput := strings.ToLower(output)
	lowerGoal := strings.ToLower(goal)

	// Check if key words from goal appear with completion indicators
	keywords := tokenize(lowerGoal)
	completionWords := []string{"done", "completed", "finished", "success", "result", "created", "found", "executed", "ran"}

	keywordMatches := 0
	for _, kw := range keywords {
		if len(kw) > 3 && strings.Contains(lowerOutput, kw) {
			keywordMatches++
		}
	}

	completionMatches := 0
	for _, cw := range completionWords {
		if strings.Contains(lowerOutput, cw) {
			completionMatches++
		}
	}

	// Step is considered complete if keywords appear AND completion indicators present
	return keywordMatches >= 2 && completionMatches >= 1
}

// persist saves the current plan state to disk.
func (p *PlanningCapability) persist() error {
	data, err := json.Marshal(struct {
		Active    *Plan  `json:"active"`
		Completed []Plan `json:"completed"`
	}{
		Active:    p.activePlan,
		Completed: p.completed,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(p.storePath, data, 0o600)
}

// Helper functions

func isStepLine(line string) bool {
	// Match patterns like "1. do X", "Step 1: do X", "- [ ] do X", "1) do X"
	if len(line) < 3 {
		return false
	}
	// Numbered: "1.", "1)", "1 -"
	if line[0] >= '0' && line[0] <= '9' {
		if len(line) > 1 && (line[1] == '.' || line[1] == ')' || line[1] == '-') {
			return true
		}
	}
	// "Step N:"
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "step ") && len(line) > 5 && line[5] >= '0' && line[5] <= '9'
}

func extractStepGoal(line string) string {
	// Strip the step number prefix
	// Find first non-prefix character
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '-' || line[i] == '[' || line[i] == ']' ||
		(line[i] >= '0' && line[i] <= '9') || line[i] == '.' || line[i] == ')' || line[i] == ':') {
		i++
	}
	// Skip "step" prefix
	lower := strings.ToLower(line)
	if strings.HasPrefix(lower, "step ") {
		// Find the colon after step number
		colonIdx := strings.Index(line, ":")
		if colonIdx > 0 {
			i = colonIdx + 1
		}
	}
	goal := strings.TrimSpace(line[i:])
	if goal == "" {
		goal = "unknown step"
	}
	return truncateStr(goal, 100)
}

func inferToolForStep(goal string) string {
	lower := strings.ToLower(goal)
	switch {
	case strings.Contains(lower, "search") || strings.Contains(lower, "find") ||
		strings.Contains(lower, "run") || strings.Contains(lower, "execute"):
		return "run_workflow"
	case strings.Contains(lower, "create") || strings.Contains(lower, "build") || strings.Contains(lower, "make") ||
		strings.Contains(lower, "info") || strings.Contains(lower, "check") || strings.Contains(lower, "inspect"):
		return "create_workflow"
	case strings.Contains(lower, "remember") || strings.Contains(lower, "store") || strings.Contains(lower, "save"):
		return "memory_store"
	case strings.Contains(lower, "recall") || strings.Contains(lower, "retrieve") || strings.Contains(lower, "get"):
		return "memory_retrieve"
	default:
		return ""
	}
}

func extractGoal(output string) string {
	lower := strings.ToLower(output)
	// Try to find explicit goal statements
	patterns := []string{
		"goal:", "objective:", "task:", "i need to", "i want to",
	}
	for _, p := range patterns {
		idx := strings.Index(lower, p)
		if idx >= 0 {
			rest := output[idx+len(p):]
			// Take up to newline or 80 chars
			end := strings.Index(rest, "\n")
			if end < 0 {
				end = min(len(rest), 80)
			}
			return strings.TrimSpace(rest[:end])
		}
	}
	return "execute plan"
}

func containsActionVerb(input string) bool {
	lower := strings.ToLower(input)
	verbs := []string{"create", "build", "make", "setup", "configure", "deploy",
		"analyze", "search", "find", "generate", "write", "implement", "fix", "solve"}
	for _, v := range verbs {
		if strings.Contains(lower, v) {
			return true
		}
	}
	return false
}
