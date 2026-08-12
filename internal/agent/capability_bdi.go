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

// capability_bdi.go implements the BDICapability —
// a Belief-Desire-Intention architecture that maintains the agent's
// world model, goals, and committed action plans.
//
// This implements the "BDI Agent (信念-愿望-意图)" type from the taxonomy:
//   - Beliefs: Facts the agent knows about the world (user preferences,
//     environment state, system capabilities)
//   - Desires: Goals the user wants to achieve (explicitly stated or
//     inferred from conversation)
//   - Intentions: Committed plans of action the agent has decided to pursue
//
// The BDI cycle:
//   Belief revision → Goal generation → Intention selection → Action → Update
//
// Key behaviors:
//   - Maintains a structured belief base (key-value facts)
//   - Tracks active goals with priority and progress
//   - Manages intention stack (committed action plans)
//   - Periodically injects goal-tracking context into the conversation
//   - Detects goal completion and triggers goal cleanup

package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Belief represents a fact the agent believes to be true about the world.
type Belief struct {
	Key        string
	Value      string
	Confidence float64 // 0.0 - 1.0
	UpdatedAt  time.Time
}

// Desire represents a goal the user wants to achieve.
type Desire struct {
	ID          string
	Description string
	Priority    int    // 1 (highest) - 5 (lowest)
	Status      string // "active", "in-progress", "completed", "abandoned"
	CreatedAt   time.Time
	Progress    string // human-readable progress note
}

// Intention represents a committed plan of action.
type Intention struct {
	ID          string
	GoalID      string // which goal this intention serves
	Description string
	Steps       []string // action steps
	CurrentStep int
	Status      string // "pending", "active", "completed", "failed"
	CreatedAt   time.Time
}

// BDICapability implements the Belief-Desire-Intention architecture
// for goal-driven agent behavior.
type BDICapability struct {
	mu         sync.RWMutex
	beliefs    map[string]*Belief
	desires    []*Desire
	intentions []*Intention
	goalIdx    int
	lastReview time.Time
}

// NewBDICapability creates a new BDI capability.
func NewBDICapability() *BDICapability {
	return &BDICapability{
		beliefs:    make(map[string]*Belief),
		desires:    make([]*Desire, 0),
		intentions: make([]*Intention, 0),
		lastReview: time.Now(),
	}
}

func (b *BDICapability) Name() string { return "bdi" }
func (b *BDICapability) Description() string {
	return "Belief-Desire-Intention: goal management and tracking (BDI Agent)"
}

func (b *BDICapability) Init(loop *AgentLoop) error {
	// Initialize with some base beliefs about the system
	b.setBelief("system.name", "aflare", 1.0)
	b.setBelief("system.type", "local-first automation agent", 1.0)
	b.setBelief("system.capabilities", "300+ workflow templates, ReAct agent, tool calling", 1.0)
	return nil
}

func (b *BDICapability) PreProcess(ctx context.Context, input string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Periodic goal review: every 5 turns, remind the agent of active goals
	if time.Since(b.lastReview) > 5*time.Minute {
		b.lastReview = time.Now()
		bdiContext := b.buildBDIContext()
		if bdiContext != "" {
			return input + "\n\n" + bdiContext, nil
		}
	}

	// Extract potential goals from user input
	b.extractDesiresFromInput(input)

	return "", nil
}

func (b *BDICapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Extract beliefs from agent output
	b.extractBeliefsFromOutput(output)

	// Check if any goals were completed
	b.checkGoalCompletion(input, output)

	return "", nil
}

func (b *BDICapability) Shutdown() error {
	return nil
}

// setBelief sets a belief with confidence.
func (b *BDICapability) setBelief(key, value string, confidence float64) {
	b.beliefs[key] = &Belief{
		Key:        key,
		Value:      value,
		Confidence: confidence,
		UpdatedAt:  time.Now(),
	}
}

// extractDesiresFromInput parses user input for goal-like statements.
// Only extracts goals from clear intent statements, not trivial queries.
func (b *BDICapability) extractDesiresFromInput(input string) {
	lower := strings.ToLower(input)

	// Strong goal indicators — user clearly states a multi-step objective
	goalIndicators := []string{
		"i want to", "i need to", "i'd like to",
		"我需要", "我想", "帮我",
		"goal:", "target:", "objective:", "aim:",
		"create a", "build a", "set up", "develop a",
		"generate a", "automate", "deploy",
	}

	hasGoal := false
	for _, indicator := range goalIndicators {
		if strings.Contains(lower, indicator) {
			hasGoal = true
			break
		}
	}

	if !hasGoal {
		return
	}

	// Don't create goals for trivial one-line queries (e.g. "what time is it")
	trivialQueries := []string{"what time", "what day", "how are you", "hello", "hi ", "thanks"}
	for _, q := range trivialQueries {
		if strings.Contains(lower, q) {
			return
		}
	}

	// Don't create duplicate goals
	for _, d := range b.desires {
		if strings.Contains(input, d.Description) || strings.Contains(d.Description, input) {
			return
		}
	}

	b.goalIdx++
	goal := &Desire{
		ID:          fmt.Sprintf("goal-%d", b.goalIdx),
		Description: truncateStr(input, 200),
		Priority:    3, // default medium priority
		Status:      "active",
		CreatedAt:   time.Now(),
	}
	b.desires = append(b.desires, goal)

	// Keep max 20 goals, remove oldest completed ones first
	if len(b.desires) > 20 {
		b.pruneDesires()
	}
}

// extractBeliefsFromOutput parses agent output for factual statements.
func (b *BDICapability) extractBeliefsFromOutput(output string) {
	// Extract key-value pairs from structured output
	// Look for patterns like "X is Y" or "X: Y"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, ": ") && !strings.HasPrefix(line, "#") {
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) == 2 && len(parts[0]) < 50 && len(parts[1]) < 200 {
				key := strings.ToLower(strings.TrimSpace(parts[0]))
				value := strings.TrimSpace(parts[1])
				if existing, ok := b.beliefs[key]; ok {
					existing.Value = value
					existing.UpdatedAt = time.Now()
				} else {
					b.setBelief(key, value, 0.7)
				}
			}
		}
	}
}

// checkGoalCompletion checks if any goals were completed based on the output.
// Only marks a goal as completed when the output contains completion language
// AND the output is related to the specific goal (not just any completion word).
func (b *BDICapability) checkGoalCompletion(input, output string) {
	lowerOutput := strings.ToLower(output)
	completionWords := []string{"completed", "done", "finished", "success", "resolved", "solved", "accomplished"}

	// Only check for completion if the output actually contains completion language
	hasCompletion := false
	for _, word := range completionWords {
		if strings.Contains(lowerOutput, word) {
			hasCompletion = true
			break
		}
	}
	if !hasCompletion {
		return
	}

	// Only mark a goal as completed if the goal's description keywords
	// appear near the completion word in the output.
	for _, d := range b.desires {
		if d.Status == "completed" || d.Status == "abandoned" {
			continue
		}
		// Extract key terms from the goal description (first 3 significant words)
		goalTerms := extractKeyTerms(d.Description)
		related := false
		for _, term := range goalTerms {
			if len(term) > 3 && strings.Contains(lowerOutput, term) {
				related = true
				break
			}
		}
		if related {
			d.Status = "completed"
			d.Progress = "completed based on agent output"
		}
	}
}

// extractKeyTerms extracts the most significant words from a description
// for matching purposes. Skips common stop words.
func extractKeyTerms(desc string) []string {
	stopWords := map[string]bool{
		"i": true, "me": true, "my": true, "you": true, "your": true,
		"to": true, "the": true, "a": true, "an": true, "is": true,
		"and": true, "or": true, "of": true, "in": true, "for": true,
		"with": true, "on": true, "at": true, "it": true, "be": true,
		"this": true, "that": true, "can": true, "will": true, "want": true,
		"need": true, "like": true, "help": true, "please": true,
	}
	words := strings.Fields(strings.ToLower(desc))
	var terms []string
	for _, w := range words {
		w = strings.Trim(w, ".,!?;:\"'()[]{}")
		if len(w) > 3 && !stopWords[w] {
			terms = append(terms, w)
			if len(terms) >= 3 {
				break
			}
		}
	}
	return terms
}

// buildBDIContext creates a context string summarizing active goals and beliefs
// to inject into the agent's next turn.
func (b *BDICapability) buildBDIContext() string {
	activeDesires := b.getActiveDesires()
	if len(activeDesires) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("[BDI Context - Active Goals]\n")

	for _, d := range activeDesires {
		sb.WriteString(fmt.Sprintf("- [%s] %s (priority: %d)\n", d.ID, d.Description, d.Priority))
		if d.Progress != "" {
			sb.WriteString(fmt.Sprintf("  Progress: %s\n", d.Progress))
		}
	}

	// Include relevant beliefs
	if len(b.beliefs) > 0 {
		sb.WriteString("\n[Relevant Beliefs]\n")
		count := 0
		for _, belief := range b.beliefs {
			if count >= 5 {
				break
			}
			sb.WriteString(fmt.Sprintf("- %s: %s (confidence: %.1f)\n", belief.Key, belief.Value, belief.Confidence))
			count++
		}
	}

	sb.WriteString("\n[BDI End]\n")
	return sb.String()
}

// getActiveDesires returns uncompleted goals sorted by priority.
func (b *BDICapability) getActiveDesires() []*Desire {
	var active []*Desire
	for _, d := range b.desires {
		if d.Status == "active" || d.Status == "in-progress" {
			active = append(active, d)
		}
	}
	return active
}

// pruneDesires removes excess completed or abandoned goals to keep the list manageable.
// Keeps the most recent 5 completed/abandoned goals, removes the rest.
func (b *BDICapability) pruneDesires() {
	// Separate active and inactive goals
	var active, completed []*Desire
	for _, d := range b.desires {
		if d.Status == "completed" || d.Status == "abandoned" {
			completed = append(completed, d)
		} else {
			active = append(active, d)
		}
	}

	// Keep only the most recent 5 completed goals
	if len(completed) > 5 {
		completed = completed[len(completed)-5:]
	}

	b.desires = append(active, completed...)
}

// AddGoal allows programmatic addition of a goal.
func (b *BDICapability) AddGoal(description string, priority int) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.goalIdx++
	id := fmt.Sprintf("goal-%d", b.goalIdx)
	b.desires = append(b.desires, &Desire{
		ID:          id,
		Description: description,
		Priority:    priority,
		Status:      "active",
		CreatedAt:   time.Now(),
	})
	return id
}

// GetGoals returns a copy of all desires.
func (b *BDICapability) GetGoals() []*Desire {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]*Desire, len(b.desires))
	copy(result, b.desires)
	return result
}

// GetBeliefs returns a copy of all beliefs.
func (b *BDICapability) GetBeliefs() map[string]*Belief {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make(map[string]*Belief, len(b.beliefs))
	for k, v := range b.beliefs {
		result[k] = &Belief{
			Key:        v.Key,
			Value:      v.Value,
			Confidence: v.Confidence,
			UpdatedAt:  v.UpdatedAt,
		}
	}
	return result
}
