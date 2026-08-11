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

// capability.go defines the AgentCapability interface — a pluggable system
// that lets users enable agent intelligence dimensions (reflection, BDI,
// human-in-the-loop, utility optimization) on demand.
//
// Each capability hooks into the AgentLoop lifecycle:
//   Init → PreProcess → [agent execution] → PostProcess → Shutdown
//
// This mirrors the full Agent type taxonomy:
//   4. Decision & Reasoning — BDI, utility-driven, planning
//   5. Internal Architecture — reflection/self-criticism, hybrid
//   9. Human-Machine Collaboration — human-in-the-loop
//   6. Learning & Adaptation — adaptive learning from feedback

package agent

import (
	"context"
	"fmt"
	"strings"
)

// AgentCapability is a pluggable intelligence dimension for an AgentLoop.
// Each capability lives as a middleware around the agent's execution cycle,
// modifying inputs, outputs, or internal state before/after each turn.
type AgentCapability interface {
	// Name returns a short identifier for this capability (e.g. "reflection", "bdi").
	Name() string

	// Description returns a human-readable description of what the capability does.
	Description() string

	// Init is called once when the capability is attached to the AgentLoop.
	// Use this to initialize internal state, register hooks, etc.
	Init(loop *AgentLoop) error

	// PreProcess runs before each turn's agent execution.
	// It can modify the input, inject additional context, or return
	// a nil string to indicate no modification is needed.
	// The original input is passed unchanged if PreProcess returns ("", nil).
	PreProcess(ctx context.Context, input string) (string, error)

	// PostProcess runs after each turn's agent execution.
	// It receives the original input and the agent's output, and can:
	//   - Modify the output (e.g. add reflection notes)
	//   - Trigger self-correction by returning a special signal
	//   - Store learnings for future turns
	// Return ("", nil) to leave the output unchanged.
	PostProcess(ctx context.Context, input, output string) (string, error)

	// Shutdown is called when the AgentLoop is shutting down.
	// Use this to persist state, clean up resources, etc.
	Shutdown() error
}

// CapabilityRegistry manages a set of AgentCapability instances.
type CapabilityRegistry struct {
	capabilities []AgentCapability
	byName       map[string]AgentCapability
}

// NewCapabilityRegistry creates an empty capability registry.
func NewCapabilityRegistry() *CapabilityRegistry {
	return &CapabilityRegistry{
		byName: make(map[string]AgentCapability),
	}
}

// Register adds a capability to the registry.
func (cr *CapabilityRegistry) Register(cap AgentCapability) {
	cr.capabilities = append(cr.capabilities, cap)
	cr.byName[cap.Name()] = cap
}

// Get returns a capability by name, or nil if not found.
func (cr *CapabilityRegistry) Get(name string) AgentCapability {
	return cr.byName[name]
}

// All returns all registered capabilities in order.
func (cr *CapabilityRegistry) All() []AgentCapability {
	return cr.capabilities
}

// InitAll initializes all capabilities with the given AgentLoop.
func (cr *CapabilityRegistry) InitAll(loop *AgentLoop) error {
	for _, cap := range cr.capabilities {
		if err := cap.Init(loop); err != nil {
			return fmt.Errorf("capability %s init failed: %w", cap.Name(), err)
		}
	}
	return nil
}

// PreProcessAll runs PreProcess on all capabilities in order.
// Each capability can modify the input; modifications are chained.
func (cr *CapabilityRegistry) PreProcessAll(ctx context.Context, input string) (string, error) {
	current := input
	for _, cap := range cr.capabilities {
		modified, err := cap.PreProcess(ctx, current)
		if err != nil {
			return current, fmt.Errorf("capability %s pre-process failed: %w", cap.Name(), err)
		}
		if modified != "" {
			current = modified
		}
	}
	return current, nil
}

// PostProcessAll runs PostProcess on all capabilities in order.
// Each capability can modify the output; modifications are chained.
func (cr *CapabilityRegistry) PostProcessAll(ctx context.Context, input, output string) (string, error) {
	current := output
	for _, cap := range cr.capabilities {
		modified, err := cap.PostProcess(ctx, input, current)
		if err != nil {
			return current, fmt.Errorf("capability %s post-process failed: %w", cap.Name(), err)
		}
		if modified != "" {
			current = modified
		}
	}
	return current, nil
}

// ShutdownAll shuts down all capabilities.
func (cr *CapabilityRegistry) ShutdownAll() error {
	var errs []string
	for _, cap := range cr.capabilities {
		if err := cap.Shutdown(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", cap.Name(), err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Count returns the number of registered capabilities.
func (cr *CapabilityRegistry) Count() int {
	return len(cr.capabilities)
}

// Names returns the names of all registered capabilities.
func (cr *CapabilityRegistry) Names() []string {
	names := make([]string, 0, len(cr.capabilities))
	for _, cap := range cr.capabilities {
		names = append(names, cap.Name())
	}
	return names
}

// AvailableCapabilities is the list of all recognized capability names.
// These map to the Agent type taxonomy dimensions.
var AvailableCapabilities = map[string]string{
	"reflection":     "Self-reflection and self-correction after each turn (反思/自我批评 Agent)",
	"human-in-loop":  "Pause at critical decisions for human approval (Human-in-the-loop Agent)",
	"bdi":            "Belief-Desire-Intention goal management and tracking (BDI Agent)",
	"utility":        "Utility-driven optimization of decisions (效用驱动 Agent)",
	"adaptive":       "Learning and adaptation from feedback (学习型/自适应 Agent)",
	"memory":         "Cross-session long-term memory and state (有状态 Agent)",
	"planning":       "Goal-driven planning and action sequencing (规划式 Agent)",
	"multi-agent":    "Multi-agent collaboration and coordination (多 Agent 协作式)",
	"workflow":       "Predefined workflow/pipeline execution (工作流/管道式 Agent)",
	"simulation":     "Simulation and generative behavior modeling (模拟/生成式 Agent)",
}

// ParseCapabilities parses a comma-separated capability string into a list.
func ParseCapabilities(input string) []string {
	if input == "" || input == "all" {
		names := make([]string, 0, len(AvailableCapabilities))
		for name := range AvailableCapabilities {
			names = append(names, name)
		}
		return names
	}
	parts := strings.Split(input, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := AvailableCapabilities[p]; ok {
			result = append(result, p)
		}
	}
	return result
}

// CreateCapability creates a capability instance by name.
func CreateCapability(name string) AgentCapability {
	switch name {
	case "reflection":
		return NewReflectionCapability()
	case "human-in-loop":
		return NewHumanInLoopCapability()
	case "bdi":
		return NewBDICapability()
	case "utility":
		return NewUtilityCapability()
	case "adaptive":
		return NewAdaptiveCapability()
	case "memory":
		return NewMemoryCapability()
	case "planning":
		return NewPlanningCapability()
	case "multi-agent":
		return NewMultiAgentCapability()
	case "workflow":
		return NewWorkflowCapability()
	case "simulation":
		return NewSimulationCapability()
	default:
		return nil
	}
}