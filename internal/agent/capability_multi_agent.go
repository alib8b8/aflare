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

// capability_multi_agent.go implements MultiAgentCapability —
// multi-agent collaboration with task decomposition and role-based
// sub-agent coordination.
//
// This implements the "多 Agent 协作式" type from the taxonomy:
//   Coordinates multiple specialized agents, decomposing complex tasks
//   into sub-tasks handled by different roles, then combining results.

package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// SubAgentRole represents a specialized agent role for task decomposition.
type SubAgentRole struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tools       []string `json:"tools"`
}

// SubTask represents a task delegated to a sub-agent.
type SubTask struct {
	ID     string `json:"id"`
	Role   string `json:"role"`
	Task   string `json:"task"`
	Result string `json:"result,omitempty"`
	Status string `json:"status"` // "pending", "in_progress", "done", "failed"
}

// MultiAgentCapability enables multi-agent collaboration through
// role-based task decomposition and coordination.
type MultiAgentCapability struct {
	mu       sync.RWMutex
	roles    []SubAgentRole
	subTasks []SubTask
	active   bool
}

// Predefined roles for task decomposition.
var defaultRoles = []SubAgentRole{
	{
		Name:        "researcher",
		Description: "Searches for information, templates, and documentation",
		Tools:       []string{"template_list", "template_info", "memory_search"},
	},
	{
		Name:        "executor",
		Description: "Runs workflows and executes concrete actions",
		Tools:       []string{"run_workflow", "create_workflow"},
	},
	{
		Name:        "analyst",
		Description: "Analyzes results, validates output, and provides insights",
		Tools:       []string{"template_info", "memory_retrieve"},
	},
	{
		Name:        "coordinator",
		Description: "Orchestrates sub-tasks, combines results, and communicates findings",
		Tools:       []string{"template_list", "memory_store", "context_compress"},
	},
}

func NewMultiAgentCapability() *MultiAgentCapability {
	return &MultiAgentCapability{
		roles:    defaultRoles,
		subTasks: make([]SubTask, 0),
	}
}

func (m *MultiAgentCapability) Name() string { return "multi-agent" }
func (m *MultiAgentCapability) Description() string {
	return "Multi-agent collaboration: coordinates multiple specialized agents (多 Agent 协作式)"
}

func (m *MultiAgentCapability) Init(loop *AgentLoop) error { return nil }

func (m *MultiAgentCapability) PreProcess(ctx context.Context, input string) (string, error) {
	// Only activate for complex tasks
	if len(input) < 100 {
		return "", nil
	}

	m.mu.Lock()
	m.active = true
	m.subTasks = make([]SubTask, 0)
	m.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("\n[Multi-Agent Mode — Task Decomposition]\n")
	sb.WriteString("This is a complex task. Decompose it into sub-tasks and assign roles:\n\n")
	sb.WriteString("Available roles:\n")
	for _, r := range m.roles {
		sb.WriteString(fmt.Sprintf("  - %s: %s\n", r.Name, r.Description))
	}
	sb.WriteString("\nInstructions:\n")
	sb.WriteString("1) Break the task into independent sub-tasks\n")
	sb.WriteString("2) Assign each sub-task to the most suitable role\n")
	sb.WriteString("3) Execute sub-tasks one at a time using the role's tools\n")
	sb.WriteString("4) Combine results into a coherent response\n")
	sb.WriteString("Start with 'Task decomposition:' followed by numbered sub-tasks with role assignments.\n")

	return input + sb.String(), nil
}

func (m *MultiAgentCapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.active {
		return "", nil
	}

	lowerOutput := strings.ToLower(output)

	// Detect task decomposition
	if strings.Contains(lowerOutput, "task decomposition") || strings.Contains(lowerOutput, "sub-task") {
		m.subTasks = m.parseSubTasks(output)
		if len(m.subTasks) > 0 {
			// Mark first as in_progress
			m.subTasks[0].Status = "in_progress"
		}
		return "", nil
	}

	// Track sub-task completion
	if len(m.subTasks) > 0 {
		for i := range m.subTasks {
			if m.subTasks[i].Status == "in_progress" {
				// Check if the current sub-task is done
				if m.isSubTaskDone(m.subTasks[i].Task, output) {
					m.subTasks[i].Status = "done"
					m.subTasks[i].Result = truncateStr(output, 200)
					// Start next sub-task
					if i+1 < len(m.subTasks) {
						m.subTasks[i+1].Status = "in_progress"
					}
				}
				break
			}
		}

		// Check if all sub-tasks are done
		allDone := true
		for _, st := range m.subTasks {
			if st.Status != "done" && st.Status != "failed" {
				allDone = false
				break
			}
		}
		if allDone {
			// Add a summary section to the output
			summary := m.buildSummary()
			m.active = false
			m.subTasks = nil
			return output + summary, nil
		}
	}

	return "", nil
}

func (m *MultiAgentCapability) Shutdown() error { return nil }

// parseSubTasks extracts sub-tasks from the agent's output.
func (m *MultiAgentCapability) parseSubTasks(output string) []SubTask {
	var tasks []SubTask
	lines := strings.Split(output, "\n")
	inDecomp := false
	id := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		if strings.Contains(lower, "task decomposition") || strings.Contains(lower, "sub-task") {
			inDecomp = true
			continue
		}

		if !inDecomp {
			continue
		}

		// Detect numbered sub-tasks
		if isStepLine(trimmed) {
			id++
			goal := extractStepGoal(trimmed)
			role := m.inferRole(goal)
			tasks = append(tasks, SubTask{
				ID:     fmt.Sprintf("sub_%d", id),
				Role:   role,
				Task:   goal,
				Status: "pending",
			})
		}

		// Stop when we hit non-task content
		if len(tasks) > 0 && !isStepLine(trimmed) && len(trimmed) > 0 &&
			!strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "*") {
			break
		}
	}

	return tasks
}

// inferRole guesses the best role for a sub-task based on keywords.
func (m *MultiAgentCapability) inferRole(task string) string {
	lower := strings.ToLower(task)
	// Check coordinator first (combine, summarize, etc. may contain sub-words)
	switch {
	case strings.Contains(lower, "combine") || strings.Contains(lower, "summarize") || strings.Contains(lower, "report") || strings.Contains(lower, "present"):
		return "coordinator"
	case strings.Contains(lower, "analyze") || strings.Contains(lower, "check") || strings.Contains(lower, "validate") || strings.Contains(lower, "review"):
		return "analyst"
	case strings.Contains(lower, "run") || strings.Contains(lower, "execute") || strings.Contains(lower, "create") || strings.Contains(lower, "build"):
		return "executor"
	case strings.Contains(lower, "search") || strings.Contains(lower, "find") || strings.Contains(lower, "look"):
		return "researcher"
	default:
		return "executor"
	}
}

// isSubTaskDone checks if a sub-task is reflected as complete in the output.
func (m *MultiAgentCapability) isSubTaskDone(task, output string) bool {
	lowerOutput := strings.ToLower(output)
	keywords := tokenize(strings.ToLower(task))

	keywordMatches := 0
	for _, kw := range keywords {
		if len(kw) > 3 && strings.Contains(lowerOutput, kw) {
			keywordMatches++
		}
	}

	completionWords := []string{"done", "completed", "finished", "result", "success", "found", "created", "executed", "ran"}
	completionMatches := 0
	for _, cw := range completionWords {
		if strings.Contains(lowerOutput, cw) {
			completionMatches++
		}
	}

	return keywordMatches >= 2 && completionMatches >= 1
}

// buildSummary creates a summary of completed sub-tasks.
func (m *MultiAgentCapability) buildSummary() string {
	var sb strings.Builder
	sb.WriteString("\n\n--- [Multi-Agent Summary] ---\n")
	for _, st := range m.subTasks {
		icon := "✓"
		if st.Status == "failed" {
			icon = "✗"
		}
		sb.WriteString(fmt.Sprintf("  %s [%s] %s", icon, st.Role, st.Task))
		if st.Result != "" {
			sb.WriteString(fmt.Sprintf(" → %s", truncateStr(st.Result, 80)))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("--- [End Summary] ---")
	return sb.String()
}
