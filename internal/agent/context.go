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

package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/nodes/core"
)

const (
	// MaxContextChars is the character budget for the full conversation context.
	// When exceeded, older messages are compressed into a summary.
	MaxContextChars = 8000

	// KeepRecentN is the number of most recent messages preserved during compression.
	KeepRecentN = 4

	// MaxSummaryChars caps the compressed summary to prevent unbounded growth.
	MaxSummaryChars = 2000
)

// ContextManager manages multi-turn conversation history with automatic
// compression when the context exceeds the character budget.
// Safe for concurrent use via the HTTP API.
type ContextManager struct {
	mu           sync.RWMutex
	messages     []core.LLMMessage
	systemPrompt string
	compressNode *nodes.CompressNode
}

// NewContextManager creates a new context manager.
func NewContextManager() *ContextManager {
	return &ContextManager{
		messages:     make([]core.LLMMessage, 0),
		compressNode: &nodes.CompressNode{},
	}
}

// SetSystemPrompt sets the system prompt (shown at the top of the context).
func (cm *ContextManager) SetSystemPrompt(prompt string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.systemPrompt = prompt
}

// AddUser appends a user message to the history.
func (cm *ContextManager) AddUser(content string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.messages = append(cm.messages, core.LLMMessage{Role: "user", Content: content})
}

// AddAssistant appends an assistant message to the history.
func (cm *ContextManager) AddAssistant(content string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.messages = append(cm.messages, core.LLMMessage{Role: "assistant", Content: content})
}

// BuildPrefix returns the conversation history as a formatted string
// suitable for inclusion in the agent's prompt. The system prompt is prepended.
func (cm *ContextManager) BuildPrefix() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var sb strings.Builder
	if cm.systemPrompt != "" {
		sb.WriteString(cm.systemPrompt)
		sb.WriteString("\n\n")
	}
	sb.WriteString("Conversation history:\n")
	for _, m := range cm.messages {
		sb.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
	}
	return sb.String()
}

// CompressIfNeeded checks the character budget and compresses older messages
// if the total exceeds MaxContextChars. Recent messages are always preserved.
func (cm *ContextManager) CompressIfNeeded() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.totalChars() <= MaxContextChars {
		return
	}
	cm.compress()
}

// totalChars returns the total character count of all messages.
// Caller must hold cm.mu.
func (cm *ContextManager) totalChars() int {
	n := 0
	for _, m := range cm.messages {
		n += len(m.Content)
	}
	return n
}

// compress replaces older messages with a summary, keeping the most recent ones.
// Caller must hold cm.mu.
func (cm *ContextManager) compress() {
	if len(cm.messages) <= KeepRecentN+2 {
		return
	}

	keepIdx := len(cm.messages) - KeepRecentN
	if keepIdx <= 0 {
		return
	}

	// Build text from older messages to compress
	var oldText strings.Builder
	for i := 0; i < keepIdx; i++ {
		oldText.WriteString(fmt.Sprintf("[%s] %s\n", cm.messages[i].Role, cm.messages[i].Content))
	}

	compressed, err := cm.compressNode.Execute(
		context.Background(),
		oldText.String(),
		map[string]string{
			"algorithm":        "hybrid",
			"ratio":            "0.15",
			"max_chars":        fmt.Sprintf("%d", MaxSummaryChars),
			"preserve_headers": "false",
			"preserve_numbers": "true",
			"output":           "text",
		},
	)
	if err != nil || compressed == "" {
		// Fallback: generate a simple summary from the truncated messages
		// instead of silently dropping them.
		compressed = cm.buildFallbackSummary(keepIdx)
	}

	// Replace old messages with summary, keep recent ones
	recent := cm.messages[keepIdx:]
	cm.messages = []core.LLMMessage{
		{Role: "system", Content: fmt.Sprintf("[Previous conversation summary]\n%s\n[End summary]", compressed)},
	}
	cm.messages = append(cm.messages, recent...)
}

// buildFallbackSummary generates a simple text summary when the compression
// node fails. It extracts the first 100 chars of each old message to avoid
// complete data loss.
func (cm *ContextManager) buildFallbackSummary(keepIdx int) string {
	var sb strings.Builder
	sb.WriteString("(compressed) ")
	for i := 0; i < keepIdx && i < 10; i++ {
		role := cm.messages[i].Role
		content := cm.messages[i].Content
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		sb.WriteString(fmt.Sprintf("[%s] %s | ", role, content))
	}
	return sb.String()
}

// Summary returns a human-readable summary of the conversation state.
// Used by the /history command.
func (cm *ContextManager) Summary() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	chars := cm.totalChars()
	msgs := len(cm.messages)
	status := "ok"
	if chars > MaxContextChars {
		status = "compressed"
	}
	return fmt.Sprintf("Messages: %d | Characters: %d (limit: %d) | Status: %s",
		msgs, chars, MaxContextChars, status)
}

// Reset clears the conversation history (keeps the system prompt).
func (cm *ContextManager) Reset() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.messages = make([]core.LLMMessage, 0)
}