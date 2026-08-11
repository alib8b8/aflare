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
	"unicode/utf8"

	"github.com/alib8b8/aflare/internal/compress"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/nodes/core"
)

const (
	// DefaultMaxContextTokens is the default token budget for conversation history.
	// Rough estimate: 1 token ≈ 4 characters for English text.
	DefaultMaxContextTokens = 8000

	// DefaultMaxContextChars is the character-level fallback when token counting
	// is unavailable (roughly 4 chars per token for English).
	DefaultMaxContextChars = DefaultMaxContextTokens * 4

	// KeepLastN is the number of recent messages to always preserve during compression.
	KeepLastN = 6

	// MaxMemoryKeyLength is the maximum length for memory keys.
	MaxMemoryKeyLength = 128
)

// ContextManager manages conversation history with automatic compression
// and key fact persistence via the memory system.
type ContextManager struct {
	messages       []core.LLMMessage
	sessionID      string
	compressNode   *nodes.CompressNode
	memoryNode     *nodes.MemoryNode
	maxTokens      int
	compressConfig compress.Config
}

// NewContextManager creates a new context manager for the given session.
func NewContextManager(sessionID string) *ContextManager {
	cfg := compress.DefaultConfig()
	cfg.Algorithm = compress.AlgoHybrid
	cfg.TargetRatio = 0.2
	cfg.MaxOutputChars = 4000
	cfg.PreserveHeaders = true
	cfg.PreserveNumbers = true

	return &ContextManager{
		messages:       make([]core.LLMMessage, 0),
		sessionID:      sessionID,
		compressNode:   &nodes.CompressNode{},
		memoryNode:     &nodes.MemoryNode{},
		maxTokens:      DefaultMaxContextTokens,
		compressConfig: cfg,
	}
}

// Add appends a message to the conversation history and triggers compression
// if the context window exceeds the token budget.
func (cm *ContextManager) Add(role, content string) {
	cm.messages = append(cm.messages, core.LLMMessage{
		Role:    role,
		Content: content,
	})
	if cm.estimateTokens() > cm.maxTokens {
		cm.autoCompress()
	}
}

// Messages returns the current conversation history.
func (cm *ContextManager) Messages() []core.LLMMessage {
	return cm.messages
}

// SetSystemPrompt sets or replaces the system message at the beginning of the conversation.
func (cm *ContextManager) SetSystemPrompt(prompt string) {
	if len(cm.messages) > 0 && cm.messages[0].Role == "system" {
		cm.messages[0].Content = prompt
	} else {
		cm.messages = append([]core.LLMMessage{{Role: "system", Content: prompt}}, cm.messages...)
	}
}

// BuildPrompt constructs a single prompt string from the conversation history
// suitable for agents that don't support multi-message chat formats.
func (cm *ContextManager) BuildPrompt() string {
	var parts []string
	for _, m := range cm.messages {
		parts = append(parts, fmt.Sprintf("%s: %s", m.Role, m.Content))
	}
	return strings.Join(parts, "\n\n")
}

// estimateTokens provides a rough token count estimate (4 chars ≈ 1 token).
func (cm *ContextManager) estimateTokens() int {
	total := 0
	for _, m := range cm.messages {
		total += utf8.RuneCountInString(m.Content)
	}
	return total / 4
}

// autoCompress compresses older messages while preserving the most recent ones.
// Key facts from compressed messages are persisted to memory before compression.
func (cm *ContextManager) autoCompress() {
	if len(cm.messages) <= KeepLastN+2 {
		return // Not enough messages to compress
	}

	keepIdx := len(cm.messages) - KeepLastN
	if keepIdx <= 1 {
		keepIdx = 2 // Always keep system message + at least 1 more
	}

	// Extract key facts from older messages and store in memory
	oldMessages := cm.messages[1:keepIdx]
	_ = cm.maybeRemember(oldMessages)

	// Build text to compress from older messages
	var oldText strings.Builder
	for _, m := range oldMessages {
		oldText.WriteString(fmt.Sprintf("[%s] %s\n", m.Role, m.Content))
	}

	// Compress older messages
	compressed, err := cm.compressNode.Execute(
		context.Background(),
		oldText.String(),
		map[string]string{
			"algorithm":        "hybrid",
			"ratio":            fmt.Sprintf("%.1f", cm.compressConfig.TargetRatio),
			"max_chars":        fmt.Sprintf("%d", cm.compressConfig.MaxOutputChars),
			"preserve_headers": "true",
			"preserve_numbers": "true",
			"output":           "text",
		},
	)
	if err != nil || compressed == "" {
		return // Keep original messages if compression fails
	}

	// Replace older messages with a compressed summary
	systemMsg := cm.messages[0]
	recent := cm.messages[keepIdx:]
	cm.messages = []core.LLMMessage{
		systemMsg,
		{
			Role:    "system",
			Content: fmt.Sprintf("[Compressed conversation history]\n%s\n[End compressed history]", compressed),
		},
	}
	cm.messages = append(cm.messages, recent...)
}

// maybeRemember extracts key facts from messages and stores them in memory.
func (cm *ContextManager) maybeRemember(messages []core.LLMMessage) error {
	var combined strings.Builder
	for _, m := range messages {
		combined.WriteString(m.Content)
		combined.WriteString("\n")
	}

	text := combined.String()
	if len(text) < 100 {
		return nil
	}

	// Store as a summary of key facts from this conversation segment
	_, err := cm.memoryNode.Execute(
		context.Background(),
		text,
		map[string]string{
			"operation":  "store",
			"session_id": cm.sessionID,
			"key":        fmt.Sprintf("chat_context_%d", len(cm.messages)),
			"level":      "medium",
			"type":       "context",
			"tags":       "chat,conversation,auto",
			"confidence": "0.7",
		},
	)
	return err
}

// Retrieve searches memory for relevant context.
func (cm *ContextManager) Retrieve(query string) (string, error) {
	return cm.memoryNode.Execute(
		context.Background(),
		query,
		map[string]string{
			"operation":  "search",
			"session_id": cm.sessionID,
			"query":      query,
			"top_k":      "5",
			"threshold":  "0.4",
		},
	)
}

// Reset clears the conversation history (keeps the system prompt).
func (cm *ContextManager) Reset() {
	var systemMsg core.LLMMessage
	if len(cm.messages) > 0 && cm.messages[0].Role == "system" {
		systemMsg = cm.messages[0]
	}
	cm.messages = []core.LLMMessage{}
	if systemMsg.Role != "" {
		cm.messages = append(cm.messages, systemMsg)
	}
}