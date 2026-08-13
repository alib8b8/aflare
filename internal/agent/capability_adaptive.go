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

// capability_stubs.go provides the AdaptiveCapability implementation.
// Other capabilities have been promoted to their own fully-implemented files:
//   - capability_memory.go     (MemoryCapability)
//   - capability_planning.go   (PlanningCapability)
//   - capability_workflow.go   (WorkflowCapability)

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

func (a *AdaptiveCapability) Name() string { return "adaptive" }
func (a *AdaptiveCapability) Description() string {
	return "Learning and adaptation: improves from feedback and experience (学习型/自适应 Agent)"
}

func (a *AdaptiveCapability) Init(loop *AgentLoop) error {
	// Load past adaptive feedback from cross-session learning journal.
	past := loadRecentAdaptiveFeedback(10)
	if len(past) > 0 {
		a.feedback = append(a.feedback, past...)
	}
	return nil
}

func (a *AdaptiveCapability) PreProcess(ctx context.Context, input string) (string, error) {
	// Inject past learnings as context
	if len(a.feedback) > 0 && len(a.feedback) <= 10 {
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
