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

// capability_utility.go implements the UtilityCapability —
// a utility-driven optimization layer that evaluates trade-offs between
// different options and helps the agent choose the optimal path.
//
// This implements the "效用驱动 Agent" type from the taxonomy:
//   Instead of just finding a valid plan, the agent evaluates multiple
//   possible actions using utility functions that weigh time, cost,
//   quality, and risk. The agent selects the action with the highest
//   expected utility.
//
// Key behaviors:
//   - Defines utility dimensions: time, cost, quality, risk, completeness
//   - Scores agent outputs on these dimensions
//   - Suggests alternative approaches when utility is low
//   - Tracks utility history for learning which strategies work best
//   - Weights can be customized per use case

package agent

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// UtilityDimension represents a dimension along which actions are evaluated.
type UtilityDimension struct {
	Name        string
	Weight      float64 // relative importance (0.0 - 1.0)
	Description string
}

// UtilityScore is a scored evaluation of a single option.
type UtilityScore struct {
	Option     string
	Dimensions map[string]float64 // score per dimension (0.0 - 1.0)
	Total      float64            // weighted total utility
	Timestamp  time.Time
}

// DefaultUtilityDimensions defines the standard evaluation axes.
var DefaultUtilityDimensions = []UtilityDimension{
	{Name: "correctness", Weight: 0.30, Description: "How correct and accurate is the response?"},
	{Name: "completeness", Weight: 0.20, Description: "How complete and thorough is the response?"},
	{Name: "efficiency", Weight: 0.15, Description: "How efficiently does it solve the problem (steps, time)?"},
	{Name: "safety", Weight: 0.15, Description: "How safe is the proposed action?"},
	{Name: "clarity", Weight: 0.10, Description: "How clear and well-structured is the response?"},
	{Name: "actionability", Weight: 0.10, Description: "How actionable is the response (concrete steps)?"},
}

// UtilityCapability implements utility-driven decision optimization.
type UtilityCapability struct {
	mu         sync.RWMutex
	dimensions []UtilityDimension
	history    []UtilityScore
	maxHistory int
}

// NewUtilityCapability creates a new utility capability with default dimensions.
func NewUtilityCapability() *UtilityCapability {
	return &UtilityCapability{
		dimensions: DefaultUtilityDimensions,
		history:    make([]UtilityScore, 0),
		maxHistory: 50,
	}
}

// SetDimensions allows customizing the utility dimensions and weights.
func (u *UtilityCapability) SetDimensions(dims []UtilityDimension) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.dimensions = dims
}

func (u *UtilityCapability) Name() string        { return "utility" }
func (u *UtilityCapability) Description() string  { return "Utility-driven optimization: evaluates trade-offs to find optimal solutions (效用驱动 Agent)" }

func (u *UtilityCapability) Init(loop *AgentLoop) error {
	return nil
}

func (u *UtilityCapability) PreProcess(ctx context.Context, input string) (string, error) {
	// Inject utility context: remind the agent to consider trade-offs
	u.mu.RLock()
	defer u.mu.RUnlock()

	if len(u.history) > 0 {
		avgUtility := u.averageUtility()
		if avgUtility < 0.5 {
			// Low historical utility — suggest improvement
			context := fmt.Sprintf(
				"\n[Utility Context] Recent responses have averaged %.1f%% utility. "+
					"Consider trade-offs: prioritize correctness and completeness. "+
					"Use tools to verify before responding.",
				avgUtility*100,
			)
			return input + context, nil
		}
	}
	return "", nil
}

func (u *UtilityCapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	if output == "" {
		return "", nil
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	// Score the output
	score := u.scoreOutput(input, output)
	u.history = append(u.history, score)

	if len(u.history) > u.maxHistory {
		u.history = u.history[len(u.history)-u.maxHistory:]
	}

	// If utility is low, append a suggestion
	if score.Total < 0.4 {
		suggestion := u.buildSuggestion(score)
		return output + "\n\n" + suggestion, nil
	}

	return "", nil
}

func (u *UtilityCapability) Shutdown() error {
	return nil
}

// scoreOutput evaluates an agent output against all utility dimensions.
func (u *UtilityCapability) scoreOutput(input, output string) UtilityScore {
	score := UtilityScore{
		Option:     truncateStr(input, 80),
		Dimensions: make(map[string]float64),
		Timestamp:  time.Now(),
	}

	lower := strings.ToLower(output)
	length := len(output)

	for _, dim := range u.dimensions {
		score.Dimensions[dim.Name] = u.evaluateDimension(dim.Name, output, lower, length, input)
	}

	// Calculate weighted total
	score.Total = 0
	for _, dim := range u.dimensions {
		score.Total += score.Dimensions[dim.Name] * dim.Weight
	}
	score.Total = math.Round(score.Total*100) / 100

	return score
}

// evaluateDimension scores a single utility dimension.
func (u *UtilityCapability) evaluateDimension(name, output, lower string, length int, input string) float64 {
	switch name {
	case "correctness":
		return u.scoreCorrectness(output, lower, length)
	case "completeness":
		return u.scoreCompleteness(output, lower, length)
	case "efficiency":
		return u.scoreEfficiency(output, lower, length)
	case "safety":
		return u.scoreSafety(output, lower)
	case "clarity":
		return u.scoreClarity(output, lower, length)
	case "actionability":
		return u.scoreActionability(output, lower, input)
	default:
		return 0.5
	}
}

func (u *UtilityCapability) scoreCorrectness(output, lower string, length int) float64 {
	score := 0.5 // baseline

	// Penalize error patterns
	errors := []string{"error", "failed", "cannot", "unable", "not found", "invalid", "exception"}
	errCount := 0
	for _, e := range errors {
		errCount += strings.Count(lower, e)
	}
	if errCount > 2 {
		score -= 0.3
	} else if errCount > 0 {
		score -= 0.15
	}

	// Reward for concrete results
	if strings.Contains(lower, "result:") || strings.Contains(lower, "output:") {
		score += 0.2
	}

	// Reward for data/facts
	numbers := 0
	for _, ch := range output {
		if ch >= '0' && ch <= '9' {
			numbers++
		}
	}
	if numbers > 20 {
		score += 0.15
	}

	return clamp(score, 0, 1)
}

func (u *UtilityCapability) scoreCompleteness(output, lower string, length int) float64 {
	score := 0.5

	// Longer responses tend to be more complete (to a point)
	if length > 200 {
		score += 0.2
	}
	if length > 500 {
		score += 0.1
	}
	if length < 50 {
		score -= 0.3
	}

	// Check for structured content
	sections := 0
	if strings.Contains(output, "1.") {
		sections++
	}
	if strings.Contains(output, "2.") {
		sections++
	}
	if strings.Contains(output, "3.") {
		sections++
	}
	score += float64(sections) * 0.1

	// Check for examples
	if strings.Contains(lower, "example") || strings.Contains(lower, "e.g.") {
		score += 0.1
	}

	return clamp(score, 0, 1)
}

func (u *UtilityCapability) scoreEfficiency(output, lower string, length int) float64 {
	score := 0.6

	// Concise is good (balances with completeness)
	if length > 2000 {
		score -= 0.2
	} else if length < 100 {
		score += 0.2
	}

	// Direct action is efficient
	if strings.Contains(lower, "template_list") || strings.Contains(lower, "run_workflow") {
		score += 0.2
	}

	// Hedging is inefficient
	hedges := []string{"maybe", "perhaps", "might", "could try", "not sure"}
	hedgeCount := 0
	for _, h := range hedges {
		hedgeCount += strings.Count(lower, h)
	}
	score -= float64(hedgeCount) * 0.1

	return clamp(score, 0, 1)
}

func (u *UtilityCapability) scoreSafety(output, lower string) float64 {
	score := 0.7

	// Check for dangerous operations
	dangerous := []string{"rm -rf", "delete", "remove", "overwrite", "truncate"}
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			score -= 0.2
		}
	}

	// Check for safety confirmations
	if strings.Contains(lower, "confirm") || strings.Contains(lower, "are you sure") {
		score += 0.2
	}

	// Check for safe defaults
	if strings.Contains(lower, "safe") || strings.Contains(lower, "backup") {
		score += 0.1
	}

	return clamp(score, 0, 1)
}

func (u *UtilityCapability) scoreClarity(output, lower string, length int) float64 {
	score := 0.5

	// Well-structured output
	lines := strings.Split(output, "\n")
	emptyLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			emptyLines++
		}
	}
	// Good ratio of structure
	if len(lines) > 5 && emptyLines > 0 && float64(emptyLines)/float64(len(lines)) < 0.3 {
		score += 0.2
	}

	// Headers or sections
	if strings.Contains(output, "##") || strings.Contains(output, "---") {
		score += 0.15
	}

	// Code blocks (technical clarity)
	if strings.Contains(output, "```") {
		score += 0.1
	}

	return clamp(score, 0, 1)
}

func (u *UtilityCapability) scoreActionability(output, lower string, input string) float64 {
	score := 0.4

	// Has concrete steps
	if strings.Contains(lower, "step 1") || strings.Contains(lower, "first") {
		score += 0.2
	}

	// Has tool calls
	if strings.Contains(lower, "template_list") || strings.Contains(lower, "run_workflow") {
		score += 0.2
	}

	// Has commands or code
	if strings.Contains(output, "```") {
		score += 0.1
	}

	// Directly addresses the input
	if strings.Contains(lower, strings.ToLower(truncateStr(input, 30))) {
		score += 0.1
	}

	return clamp(score, 0, 1)
}

// buildSuggestion creates a utility improvement suggestion.
func (u *UtilityCapability) buildSuggestion(score UtilityScore) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- [Utility Analysis: %.0f%%] ---\n", score.Total*100))
	sb.WriteString("This response scored below the quality threshold. Consider improving:\n")

	// Find the weakest dimensions
	for _, dim := range u.dimensions {
		if score.Dimensions[dim.Name] < 0.5 {
			sb.WriteString(fmt.Sprintf("- %s (%.0f%%): %s\n", dim.Name, score.Dimensions[dim.Name]*100, dim.Description))
		}
	}

	sb.WriteString("\nSuggestion: Review the response and try a more complete, actionable approach. Use your tools to verify facts and provide concrete steps.")
	sb.WriteString("\n--- [End Utility Analysis] ---")
	return sb.String()
}

// averageUtility returns the mean utility score from recent history.
func (u *UtilityCapability) averageUtility() float64 {
	if len(u.history) == 0 {
		return 1.0
	}
	total := 0.0
	start := 0
	if len(u.history) > 10 {
		start = len(u.history) - 10
	}
	for _, s := range u.history[start:] {
		total += s.Total
	}
	return total / float64(len(u.history)-start)
}

// GetHistory returns the utility score history.
func (u *UtilityCapability) GetHistory() []UtilityScore {
	u.mu.RLock()
	defer u.mu.RUnlock()
	result := make([]UtilityScore, len(u.history))
	copy(result, u.history)
	return result
}

// clamp constrains a value to [min, max].
func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}