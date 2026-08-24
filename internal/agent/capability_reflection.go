// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌‌‌​​​‌‌​​‌‌​​‌​​‌‌‌​‌​​​​​‌‌‌‌​​‌‌​​‌‌​​​‌‌​‌​​​​​​​​​​​​​​​​​‌‌‌‌‌​​‌‌​​​​​‌⁠
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

// capability_reflection.go implements the ReflectionCapability —
// a self-reflection/self-criticism layer that evaluates the agent's output
// and triggers self-correction when quality is insufficient.
//
// This implements the "反思 / 自我批评 Agent" type from the taxonomy:
//   After each execution, the capability checks the result against
//   quality criteria. If the output is incomplete, inconsistent, or
//   low-quality, it appends a reflection prompt to trigger the agent
//   to self-correct in the next turn.
//
// Key behaviors:
//   - Maintains a reflection log of past corrections for learning
//   - Detects stale responses (same output repeated)
//   - Detects empty/truncated outputs
//   - Detects error-like patterns
//   - Generates constructive self-critique prompts

package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// ReflectionCapability implements self-reflection and self-correction
// after each agent turn. It evaluates output quality and injects
// reflection prompts to trigger iterative improvement.
type ReflectionCapability struct {
	mu             sync.RWMutex
	lastOutput     string // last output for staleness detection
	reflectionLog  []string
	maxReflections int // max reflection rounds per turn
}

// NewReflectionCapability creates a new reflection capability.
func NewReflectionCapability() *ReflectionCapability {
	return &ReflectionCapability{
		reflectionLog:  make([]string, 0),
		maxReflections: 3,
	}
}

func (r *ReflectionCapability) Name() string { return CapabilityReflection }
func (r *ReflectionCapability) Description() string {
	return "Self-reflection and self-correction: evaluates output quality and triggers improvement when needed"
}

func (r *ReflectionCapability) Init(loop *AgentLoop) error {
	// Load past reflection issues from cross-session learning journal.
	// These are used to improve the quality checker's awareness of
	// recurring problems.
	pastIssues := loadRecentReflectionIssues(20)
	if len(pastIssues) > 0 {
		r.reflectionLog = append(r.reflectionLog, pastIssues...)
	}
	return nil
}

func (r *ReflectionCapability) PreProcess(ctx context.Context, input string) (string, error) {
	// No pre-processing needed — reflection happens after execution.
	return "", nil
}

func (r *ReflectionCapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	if output == "" {
		return "", nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	issues := r.evaluateQuality(input, output)

	if len(issues) == 0 {
		r.lastOutput = output
		return "", nil // output is good, no modification needed
	}

	// Persist to cross-session learning log
	appendReflection(input, issues)

	// Build reflection prompt
	reflection := r.buildReflectionPrompt(input, output, issues)
	r.reflectionLog = append(r.reflectionLog, reflection)

	if len(r.reflectionLog) > 100 {
		r.reflectionLog = r.reflectionLog[len(r.reflectionLog)-100:]
	}

	// Append reflection note to the output
	augmented := output + "\n\n" + reflection
	r.lastOutput = augmented
	return augmented, nil
}

func (r *ReflectionCapability) Shutdown() error {
	return nil
}

// evaluateQuality checks the output against quality criteria.
// Returns a list of issues found (empty = good quality).
func (r *ReflectionCapability) evaluateQuality(input, output string) []string {
	var issues []string

	// Check for empty/truncated output (only flag if < 10 chars — very short)
	trimmed := strings.TrimSpace(output)
	if len(trimmed) < 10 {
		issues = append(issues, "output is too short or empty")
	}

	// Check for error patterns
	lowerOutput := strings.ToLower(output)
	if strings.Contains(lowerOutput, "i don't know") ||
		strings.Contains(lowerOutput, "i cannot") ||
		strings.Contains(lowerOutput, "i'm not able") {
		issues = append(issues, "agent is refusing or unable to help")
	}

	// Check for staleness (same output as last time)
	if r.lastOutput != "" && output == r.lastOutput {
		issues = append(issues, "output is identical to the previous response (stale/repeated)")
	}

	// Check for missing action — only flag when the input clearly asks for action
	// but the output contains no concrete steps or results. Don't flag when the
	// output is a legitimate final answer (e.g. presenting results of a tool call).
	hasAction := strings.Contains(output, "run_workflow") ||
		strings.Contains(output, "create_workflow") ||
		strings.Contains(output, "I'll") ||
		strings.Contains(output, "Let me") ||
		strings.Contains(output, "Here") ||
		strings.Contains(output, "Result") ||
		strings.Contains(lowerOutput, "step") ||
		strings.Contains(lowerOutput, "done") ||
		strings.Contains(lowerOutput, "completed") ||
		strings.Contains(lowerOutput, "here is")
	inputIsActionable := strings.Contains(strings.ToLower(input), "do") ||
		strings.Contains(strings.ToLower(input), "run") ||
		strings.Contains(strings.ToLower(input), "create") ||
		strings.Contains(strings.ToLower(input), "make") ||
		strings.Contains(strings.ToLower(input), "build") ||
		strings.Contains(strings.ToLower(input), "帮我") ||
		strings.Contains(strings.ToLower(input), "做")
	if !hasAction && inputIsActionable && len(trimmed) > 30 {
		issues = append(issues, "response lacks concrete actions or tool calls for an actionable request")
	}

	// Check for excessive hedging
	hedgeCount := 0
	hedges := []string{"maybe", "perhaps", "might", "could try", "not sure", "possibly"}
	for _, h := range hedges {
		hedgeCount += strings.Count(lowerOutput, h)
	}
	if hedgeCount > 3 {
		issues = append(issues, "response is overly hesitant with excessive hedging")
	}

	return issues
}

// buildReflectionPrompt creates a self-critique prompt that encourages
// the agent to improve its response.
func (r *ReflectionCapability) buildReflectionPrompt(input, output string, issues []string) string {
	var sb strings.Builder
	sb.WriteString("--- [Self-Reflection] ---\n")
	sb.WriteString(fmt.Sprintf("Quality issues detected (%d):\n", len(issues)))
	for i, issue := range issues {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, issue))
	}

	sb.WriteString("\nReflection prompt for next turn:\n")
	sb.WriteString(fmt.Sprintf("Review your last response to '%s'. ", truncateStr(input, 100)))
	sb.WriteString("The response had quality issues: ")

	for i, issue := range issues {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(issue)
	}
	sb.WriteString(". Please provide a better response with concrete actions, clear steps, and confident tone. Use your tools (run_workflow, create_workflow) to take action rather than just describing what could be done.")
	sb.WriteString("\n--- [End Self-Reflection] ---")

	return sb.String()
}
