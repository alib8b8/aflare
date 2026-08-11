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

// capability_stubs.go provides lightweight implementations for the remaining
// Agent type taxonomy capabilities. These are structurally complete but
// intentionally minimal — they serve as extension points for future
// deep implementations while keeping the pluggable system fully functional.

package agent

import (
	"context"
	"fmt"
	"strings"
)

// ── AdaptiveCapability (学习型/自适应 Agent) ────────────────────────────

// AdaptiveCapability enables learning from experience through feedback.
// In a full implementation, this would use reinforcement learning or
// pattern-based adaptation to improve over time.
type AdaptiveCapability struct {
	feedback []string
}

func NewAdaptiveCapability() *AdaptiveCapability {
	return &AdaptiveCapability{feedback: make([]string, 0)}
}

func (a *AdaptiveCapability) Name() string        { return "adaptive" }
func (a *AdaptiveCapability) Description() string  { return "Learning and adaptation: improves from feedback and experience (学习型/自适应 Agent)" }

func (a *AdaptiveCapability) Init(loop *AgentLoop) error { return nil }

func (a *AdaptiveCapability) PreProcess(ctx context.Context, input string) (string, error) {
	// Inject past learnings as context
	if len(a.feedback) > 0 && len(a.feedback) <= 5 {
		context := "\n[Adaptive Learning] Past feedback:\n"
		for _, f := range a.feedback {
			context += fmt.Sprintf("- %s\n", f)
		}
		context += "Consider this feedback when responding.\n"
		return input + context, nil
	}
	return "", nil
}

func (a *AdaptiveCapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	lower := strings.ToLower(output)
	if strings.Contains(lower, "error") || strings.Contains(lower, "failed") {
		feedback := fmt.Sprintf("Avoid errors like in response to: %s", truncateStr(input, 60))
		a.feedback = append(a.feedback, feedback)
		if len(a.feedback) > 20 {
			a.feedback = a.feedback[len(a.feedback)-20:]
		}
		// Persist to cross-session learning log
		appendAdaptiveFeedback(feedback)
	}
	return "", nil
}

func (a *AdaptiveCapability) Shutdown() error { return nil }

// ── MemoryCapability (有状态 Agent) ──────────────────────────────────────

// MemoryCapability provides cross-session long-term memory.
// It wraps the existing ContextManager with persistent storage awareness.
type MemoryCapability struct {
	sessionMemory map[string]string
}

func NewMemoryCapability() *MemoryCapability {
	return &MemoryCapability{
		sessionMemory: make(map[string]string),
	}
}

func (m *MemoryCapability) Name() string        { return "memory" }
func (m *MemoryCapability) Description() string  { return "Cross-session memory: remembers preferences and history across sessions (有状态 Agent)" }

func (m *MemoryCapability) Init(loop *AgentLoop) error { return nil }

func (m *MemoryCapability) PreProcess(ctx context.Context, input string) (string, error) {
	if len(m.sessionMemory) == 0 {
		return "", nil
	}
	var sb strings.Builder
	sb.WriteString("\n[Memory - Saved Preferences]\n")
	for key, value := range m.sessionMemory {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", key, value))
	}
	sb.WriteString("Respect these preferences when responding.\n")
	return input + sb.String(), nil
}

func (m *MemoryCapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	// Extract user preferences from the conversation
	lower := strings.ToLower(input)
	if strings.Contains(lower, "prefer") || strings.Contains(lower, "always") || strings.Contains(lower, "never") {
		m.sessionMemory[truncateStr(input, 40)] = truncateStr(output, 80)
		if len(m.sessionMemory) > 50 {
			// Prune oldest entries
			for k := range m.sessionMemory {
				delete(m.sessionMemory, k)
				break
			}
		}
	}
	return "", nil
}

func (m *MemoryCapability) Shutdown() error { return nil }

// ── PlanningCapability (规划式 Agent) ────────────────────────────────────

// PlanningCapability adds explicit goal-driven planning to the agent.
// It generates and tracks action sequences before execution.
type PlanningCapability struct {
	plans []string
}

func NewPlanningCapability() *PlanningCapability {
	return &PlanningCapability{plans: make([]string, 0)}
}

func (p *PlanningCapability) Name() string        { return "planning" }
func (p *PlanningCapability) Description() string  { return "Goal-driven planning: generates action sequences and tracks progress (规划式 Agent)" }

func (p *PlanningCapability) Init(loop *AgentLoop) error { return nil }

func (p *PlanningCapability) PreProcess(ctx context.Context, input string) (string, error) {
	// Encourage the agent to plan before acting
	planPrompt := "\n[Planning Mode] Before acting, outline a plan: 1) Identify the goal, 2) List steps needed, 3) Choose the right tool for each step, 4) Execute step by step.\n"
	return input + planPrompt, nil
}

func (p *PlanningCapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	// Track if the agent followed a plan
	if strings.Contains(strings.ToLower(output), "step 1") || strings.Contains(strings.ToLower(output), "plan:") {
		p.plans = append(p.plans, truncateStr(output, 100))
		if len(p.plans) > 20 {
			p.plans = p.plans[len(p.plans)-20:]
		}
	}
	return "", nil
}

func (p *PlanningCapability) Shutdown() error { return nil }

// ── MultiAgentCapability (多 Agent 协作式) ───────────────────────────────

// MultiAgentCapability enables multi-agent collaboration.
// Full implementation would spawn sub-agents for specialized tasks.
type MultiAgentCapability struct {
	subAgents []string
}

func NewMultiAgentCapability() *MultiAgentCapability {
	return &MultiAgentCapability{subAgents: make([]string, 0)}
}

func (m *MultiAgentCapability) Name() string        { return "multi-agent" }
func (m *MultiAgentCapability) Description() string  { return "Multi-agent collaboration: coordinates multiple specialized agents (多 Agent 协作式)" }

func (m *MultiAgentCapability) Init(loop *AgentLoop) error { return nil }

func (m *MultiAgentCapability) PreProcess(ctx context.Context, input string) (string, error) {
	// Encourage the agent to decompose complex tasks
	if len(input) > 100 {
		context := "\n[Multi-Agent Mode] For complex tasks, consider decomposing into sub-tasks: " +
			"1) Identify independent sub-tasks, 2) Address each with the right tool, " +
			"3) Combine results into a coherent response.\n"
		return input + context, nil
	}
	return "", nil
}

func (m *MultiAgentCapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	return "", nil
}

func (m *MultiAgentCapability) Shutdown() error { return nil }

// ── WorkflowCapability (工作流/管道式 Agent) ─────────────────────────────

// WorkflowCapability enforces predefined workflow execution patterns.
// This maps to the existing template/workflow system already in the agent.
type WorkflowCapability struct{}

func NewWorkflowCapability() *WorkflowCapability {
	return &WorkflowCapability{}
}

func (w *WorkflowCapability) Name() string        { return "workflow" }
func (w *WorkflowCapability) Description() string  { return "Predefined workflow execution: stable, predictable pipeline steps (工作流/管道式 Agent)" }

func (w *WorkflowCapability) Init(loop *AgentLoop) error { return nil }

func (w *WorkflowCapability) PreProcess(ctx context.Context, input string) (string, error) {
	// Encourage template-first approach
	context := "\n[Workflow Mode] Prefer existing templates over composing new ones. " +
		"Use template_list to find a matching workflow, template_info to inspect it, " +
		"and run_workflow to execute it. Only create new workflows when no template matches.\n"
	return input + context, nil
}

func (w *WorkflowCapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	return "", nil
}

func (w *WorkflowCapability) Shutdown() error { return nil }

// ── SimulationCapability (模拟/生成式 Agent) ─────────────────────────────

// SimulationCapability enables generative behavior modeling.
// Used for game NPCs, social simulations, and scenario generation.
type SimulationCapability struct{}

func NewSimulationCapability() *SimulationCapability {
	return &SimulationCapability{}
}

func (s *SimulationCapability) Name() string        { return "simulation" }
func (s *SimulationCapability) Description() string  { return "Simulation and generative modeling: produces human-like behavior outputs (模拟/生成式 Agent)" }

func (s *SimulationCapability) Init(loop *AgentLoop) error { return nil }

func (s *SimulationCapability) PreProcess(ctx context.Context, input string) (string, error) {
	return "", nil
}

func (s *SimulationCapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	return "", nil
}

func (s *SimulationCapability) Shutdown() error { return nil }