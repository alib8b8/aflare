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

// capability_simulation.go implements SimulationCapability —
// generative behavior modeling for scenario generation, role-playing,
// and human-like behavior simulation.
//
// This implements the "模拟/生成式 Agent" type from the taxonomy:
//   Produces human-like behaviors, maintains character consistency,
//   and generates realistic scenarios for testing, training, or
//   entertainment purposes.

package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// SimulationPersona defines a character or role for simulation.
type SimulationPersona struct {
	Name       string   `json:"name"`
	Role       string   `json:"role"`
	Traits     []string `json:"traits"`
	Background string   `json:"background"`
	Goals      []string `json:"goals"`
}

// SimulationState tracks the current state of a simulation.
type SimulationState struct {
	Persona    SimulationPersona `json:"persona"`
	Turn       int               `json:"turn"`
	Context    string            `json:"context"`
	Memory     []string          `json:"memory"`      // key events in the simulation
	WorldState map[string]string `json:"world_state"` // shared world state
}

// SimulationCapability enables generative behavior modeling for
// scenario simulation, role-playing, and NPC behavior generation.
type SimulationCapability struct {
	mu     sync.RWMutex
	state  *SimulationState
	active bool
}

func NewSimulationCapability() *SimulationCapability {
	return &SimulationCapability{}
}

func (s *SimulationCapability) Name() string { return "simulation" }
func (s *SimulationCapability) Description() string {
	return "Simulation and generative modeling: produces human-like behavior outputs (模拟/生成式 Agent)"
}

func (s *SimulationCapability) Init(loop *AgentLoop) error { return nil }

func (s *SimulationCapability) PreProcess(ctx context.Context, input string) (string, error) {
	s.mu.RLock()
	active := s.active
	state := s.state
	s.mu.RUnlock()

	if !active {
		// Check if the input requests simulation mode
		if s.isSimulationRequest(input) {
			s.mu.Lock()
			s.active = true
			s.state = s.initializeSimulation(input)
			state = s.state
			s.mu.Unlock()

			var sb strings.Builder
			sb.WriteString("\n[Simulation Mode — Scenario Generation]\n")
			sb.WriteString(fmt.Sprintf("Persona: %s (%s)\n", state.Persona.Name, state.Persona.Role))
			sb.WriteString(fmt.Sprintf("Traits: %s\n", strings.Join(state.Persona.Traits, ", ")))
			sb.WriteString(fmt.Sprintf("Background: %s\n", state.Persona.Background))
			sb.WriteString("\nInstructions:\n")
			sb.WriteString("- Respond as this persona, staying in character\n")
			sb.WriteString("- Maintain consistency with the persona's traits and background\n")
			sb.WriteString("- Track world state changes and key events\n")
			sb.WriteString("- Generate realistic, human-like responses and actions\n")
			sb.WriteString("Start your response with the persona's name and role.\n")
			return input + sb.String(), nil
		}
		return "", nil
	}

	// Active simulation — provide context
	var sb strings.Builder
	sb.WriteString("\n[Simulation — Continuing]\n")
	sb.WriteString(fmt.Sprintf("Turn: %d | Persona: %s\n", state.Turn, state.Persona.Name))
	if len(state.Memory) > 0 {
		sb.WriteString("Recent events:\n")
		start := 0
		if len(state.Memory) > 5 {
			start = len(state.Memory) - 5
		}
		for _, m := range state.Memory[start:] {
			sb.WriteString(fmt.Sprintf("  - %s\n", m))
		}
	}
	if len(state.WorldState) > 0 {
		sb.WriteString("World state:\n")
		for k, v := range state.WorldState {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}
	sb.WriteString("Continue the simulation. Stay in character. Track any changes to world state.\n")
	return input + sb.String(), nil
}

func (s *SimulationCapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return "", nil
	}

	s.state.Turn++

	// Extract key events from the output
	events := s.extractEvents(output)
	s.state.Memory = append(s.state.Memory, events...)
	if len(s.state.Memory) > 50 {
		s.state.Memory = s.state.Memory[len(s.state.Memory)-50:]
	}

	// Track world state changes
	s.trackWorldState(output)

	// Check for simulation end
	if s.isSimulationEnd(output) || strings.Contains(strings.ToLower(input), "end simulation") {
		var sb strings.Builder
		sb.WriteString("\n\n--- [Simulation Complete] ---\n")
		sb.WriteString(fmt.Sprintf("Persona: %s (%s)\n", s.state.Persona.Name, s.state.Persona.Role))
		sb.WriteString(fmt.Sprintf("Total turns: %d\n", s.state.Turn))
		sb.WriteString("Key events:\n")
		for _, e := range s.state.Memory {
			sb.WriteString(fmt.Sprintf("  - %s\n", e))
		}
		sb.WriteString("Final world state:\n")
		for k, v := range s.state.WorldState {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
		sb.WriteString("--- [End Simulation] ---")
		s.active = false
		s.state = nil
		return output + sb.String(), nil
	}

	return "", nil
}

func (s *SimulationCapability) Shutdown() error { return nil }

// isSimulationRequest checks if the input requests simulation mode.
func (s *SimulationCapability) isSimulationRequest(input string) bool {
	lower := strings.ToLower(input)

	// Only trigger for explicit simulation requests, not casual "you are a" usage
	explicitTriggers := []string{
		"simulate", "simulation", "role-play", "role play", "roleplay",
		"pretend you are", "扮演", "模拟", "character:", "persona:", "scenario:",
	}

	for _, t := range explicitTriggers {
		if strings.Contains(lower, t) {
			return true
		}
	}

	// "act as" + specific character
	if strings.Contains(lower, "act as") && len(input) > 50 {
		return true
	}

	return false
}

// initializeSimulation creates a new simulation state from the input.
func (s *SimulationCapability) initializeSimulation(input string) *SimulationState {
	persona := s.extractPersona(input)

	return &SimulationState{
		Persona:    persona,
		Turn:       0,
		Context:    input,
		Memory:     make([]string, 0),
		WorldState: make(map[string]string),
	}
}

// extractPersona extracts persona information from the input.
func (s *SimulationCapability) extractPersona(input string) SimulationPersona {
	persona := SimulationPersona{
		Name:   "Character",
		Role:   "participant",
		Traits: []string{"adaptive"},
	}

	lower := strings.ToLower(input)

	// Extract name
	namePatterns := []string{"name:", "character:", "persona:", "you are", "named "}
	for _, p := range namePatterns {
		idx := strings.Index(lower, p)
		if idx >= 0 {
			rest := strings.TrimSpace(input[idx+len(p):])
			// Take first word or up to comma/newline
			end := strings.IndexAny(rest, ",\n.")
			if end < 0 {
				end = min(len(rest), 30)
			}
			name := strings.TrimSpace(rest[:end])
			if name != "" && len(name) > 1 {
				persona.Name = name
			}
			break
		}
	}

	// Extract role
	rolePatterns := []string{"role:", "as a", "as an", "扮演", "job:"}
	for _, p := range rolePatterns {
		idx := strings.Index(lower, p)
		if idx >= 0 {
			rest := strings.TrimSpace(input[idx+len(p):])
			end := strings.IndexAny(rest, ",\n.")
			if end < 0 {
				end = min(len(rest), 30)
			}
			role := strings.TrimSpace(rest[:end])
			if role != "" && len(role) > 1 {
				persona.Role = role
			}
			break
		}
	}

	// Extract traits
	traitPatterns := []string{"traits:", "personality:", "性格:", "特征:"}
	for _, p := range traitPatterns {
		idx := strings.Index(lower, p)
		if idx >= 0 {
			rest := strings.TrimSpace(input[idx+len(p):])
			end := strings.Index(rest, "\n")
			if end < 0 {
				end = min(len(rest), 100)
			}
			traitsStr := strings.TrimSpace(rest[:end])
			parts := strings.Split(traitsStr, ",")
			var traits []string
			for _, part := range parts {
				t := strings.TrimSpace(part)
				if t != "" {
					traits = append(traits, t)
				}
			}
			if len(traits) > 0 {
				persona.Traits = traits
			}
			break
		}
	}

	// Extract background
	bgPatterns := []string{"background:", "context:", "setting:", "背景:", "场景:"}
	for _, p := range bgPatterns {
		idx := strings.Index(lower, p)
		if idx >= 0 {
			rest := strings.TrimSpace(input[idx+len(p):])
			end := strings.Index(rest, "\n")
			if end < 0 {
				end = min(len(rest), 200)
			}
			persona.Background = strings.TrimSpace(rest[:end])
			break
		}
	}

	return persona
}

// extractEvents identifies key events from the output.
func (s *SimulationCapability) extractEvents(output string) []string {
	var events []string
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		// Detect action/event lines
		eventPatterns := []string{
			"*", "- ", "action:", "event:", "happens:", "does:", "says:",
		}
		for _, p := range eventPatterns {
			if strings.HasPrefix(lower, p) {
				event := strings.TrimPrefix(lower, p)
				event = strings.TrimSpace(event)
				if len(event) > 10 {
					events = append(events, truncateStr(event, 100))
				}
				break
			}
		}

		// Cap at 3 events per turn
		if len(events) >= 3 {
			break
		}
	}

	return events
}

// trackWorldState updates the world state from the output.
func (s *SimulationCapability) trackWorldState(output string) {
	lower := strings.ToLower(output)

	// Track state changes signaled by keywords
	statePatterns := map[string][]string{
		"location":     {"arrive at", "go to", "move to", "enter", "leave"},
		"mood":         {"feel", "happy", "sad", "angry", "excited", "nervous", "calm"},
		"relationship": {"trust", "friend", "enemy", "ally", "rival"},
		"status":       {"success", "fail", "complete", "progress"},
	}

	for key, patterns := range statePatterns {
		for _, p := range patterns {
			idx := strings.Index(lower, p)
			if idx >= 0 {
				// Extract the relevant state value
				rest := output[idx:]
				end := strings.IndexAny(rest, ".,;!\n")
				if end < 0 {
					end = min(len(rest), 60)
				}
				value := strings.TrimSpace(rest[:end])
				if value != "" {
					s.state.WorldState[key] = value
				}
				break
			}
		}
	}
}

// isSimulationEnd checks if the output signals the end of simulation.
func (s *SimulationCapability) isSimulationEnd(output string) bool {
	lower := strings.ToLower(output)
	endPhrases := []string{
		"end simulation", "simulation complete", "simulation end",
		"simulation is complete", "simulation is over",
		"end of simulation", "the end", "fin.",
	}
	for _, p := range endPhrases {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
