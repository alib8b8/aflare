// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌​​‌‌‌‌‌‌​​​‌​‌‌​‌‌‌​​‌‌​‌‌‌​‌‌‌​​‌​‌‌​‌​​​‌‌‌​​​​​​​​​​​​​​​​​​‌​‌‌‌​​‌‌‌‌‌​‌‌⁠
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
	"unicode"

	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/nodes/core"
)

const (
	// MaxContextChars is the default token budget for the full conversation
	// context. When exceeded, older messages are compressed into a summary.
	// It is the fallback for providers without an entry in
	// providerContextBudgets and the historical default (P1-4 made the
	// budget provider-aware; behaviour for unknown providers is unchanged).
	MaxContextChars = 8000

	// KeepRecentN is the number of most recent messages preserved during compression.
	KeepRecentN = 4

	// MaxSummaryChars caps the compressed summary to prevent unbounded growth.
	MaxSummaryChars = 2000

	// minContextBudget is the floor applied to explicit budget overrides
	// (SetBudget). Below this, every turn would trigger compression into a
	// summary that itself approaches the budget — churn without benefit.
	minContextBudget = 1000
)

// providerContextBudgets maps each supported provider to its default
// conversation-history budget in estimated tokens (P1-4). Values are
// deliberately conservative fractions (~25%) of the provider's typical
// context window: the budget covers only the conversation history, while
// the model's window must also hold the system prompt, tool schemas, the
// current user message, and the response.
//
// Providers not listed here (including custom OpenAI-compatible endpoints
// routed through generic names) fall back to MaxContextChars, preserving
// the pre-P1-4 behaviour. An explicit SetBudget call overrides the table.
var providerContextBudgets = map[string]int{
	// ollama keeps the legacy budget: local models range from 2K to 128K
	// and the default must fit the smallest common window (llama3: 8K).
	"ollama":   MaxContextChars,
	"openai":   32000, // gpt-4o/gpt-5 class: 128K window
	"deepseek": 32000, // V3/R1: 128K window
	"glm":      32000, // glm-4.x: 128K window
	"kimi":     48000, // moonshot long-context: 200K window
	"qwen":     32000, // qwen-max/plus: 128K window
	"mistral":  16000, // 32K (small) to 128K (large): conservative
	"yi":       16000, // Yi-34B: 32K window
}

// estimateTokens estimates the number of tokens in text using a provider-aware
// heuristic. For ollama (English-first), it uses 4 chars ≈ 1 token. For other
// providers, it counts CJK characters at 1.5 tokens each and Latin characters
// at 0.25 tokens each. This is intentionally approximate — better than raw
// character count for mixed-language contexts.
func estimateTokens(text string, provider string) int {
	if provider == "ollama" {
		return len(text) / 4
	}
	cjk := countCJK(text)
	other := len([]rune(text)) - cjk
	return int(float64(cjk)*1.5 + float64(other)*0.25)
}

// countCJK counts the number of CJK (Chinese/Japanese/Korean) characters.
func countCJK(text string) int {
	n := 0
	for _, r := range text {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) ||
			unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r) {
			n++
		}
	}
	return n
}

// ContextManager manages multi-turn conversation history with automatic
// compression when the context exceeds the token budget.
// Safe for concurrent use via the HTTP API.
type ContextManager struct {
	mu             sync.RWMutex
	messages       []core.LLMMessage
	systemPrompt   string
	compressNode   *nodes.CompressNode
	provider       string // LLM provider for token estimation + default budget
	budgetOverride int    // explicit budget (P1-4); 0 = use provider default
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

// SetProvider sets the LLM provider, which selects both the token
// estimation heuristic and the default context budget from
// providerContextBudgets.
func (cm *ContextManager) SetProvider(provider string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.provider = provider
}

// SetBudget overrides the context budget with an explicit value (P1-4).
// It takes precedence over the provider default, regardless of the order
// in which SetProvider / SetBudget are called. n <= 0 clears the override
// (falling back to the provider default); values below minContextBudget
// are clamped up to it.
func (cm *ContextManager) SetBudget(n int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if n <= 0 {
		cm.budgetOverride = 0
		return
	}
	if n < minContextBudget {
		n = minContextBudget
	}
	cm.budgetOverride = n
}

// Budget returns the effective context budget: the explicit override if
// set, otherwise the provider default, otherwise MaxContextChars.
func (cm *ContextManager) Budget() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.effectiveBudget()
}

// effectiveBudget computes the effective budget. Caller must hold cm.mu.
func (cm *ContextManager) effectiveBudget() int {
	if cm.budgetOverride > 0 {
		return cm.budgetOverride
	}
	if b, ok := providerContextBudgets[cm.provider]; ok {
		return b
	}
	return MaxContextChars
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

// CompressIfNeeded checks the token budget and compresses older messages
// if the total exceeds it. Recent messages are always preserved.
// Returns (before, after) message counts when compression occurred, or (0,0) otherwise.
func (cm *ContextManager) CompressIfNeeded() (int, int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.totalTokens() <= cm.effectiveBudget() {
		return 0, 0
	}
	before := len(cm.messages)
	cm.compress()
	after := len(cm.messages)
	return before, after
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

// totalTokens returns the estimated token count of all messages using
// provider-aware heuristics. Caller must hold cm.mu.
func (cm *ContextManager) totalTokens() int {
	n := 0
	for _, m := range cm.messages {
		n += estimateTokens(m.Content, cm.provider)
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
	tokens := cm.totalTokens()
	budget := cm.effectiveBudget()
	msgs := len(cm.messages)
	status := "ok"
	if tokens > budget {
		status = "compressed"
	}
	return fmt.Sprintf("Messages: %d | Characters: %d | Tokens: %d (limit: %d) | Status: %s",
		msgs, chars, tokens, budget, status)
}

// ContextUsage returns the current context usage as a fraction of the limit,
// and whether compression is active. Suitable for prompt display.
func (cm *ContextManager) ContextUsage() (chars int, limit int, compressed bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	chars = cm.totalTokens()
	limit = cm.effectiveBudget()
	// Check if any message is a compression summary
	for _, m := range cm.messages {
		if m.Role == "system" && strings.Contains(m.Content, "[Previous conversation summary]") {
			compressed = true
			break
		}
	}
	return
}

// Reset clears the conversation history (keeps the system prompt).
func (cm *ContextManager) Reset() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.messages = make([]core.LLMMessage, 0)
}

// TotalChars returns the current character count of all messages.
func (cm *ContextManager) TotalChars() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.totalChars()
}

// MessageCount returns the number of messages in the context.
func (cm *ContextManager) MessageCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.messages)
}

// Messages returns a copy of all messages in the context.
func (cm *ContextManager) Messages() []core.LLMMessage {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	cp := make([]core.LLMMessage, len(cm.messages))
	copy(cp, cm.messages)
	return cp
}
