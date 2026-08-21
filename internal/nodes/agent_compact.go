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

package nodes

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/alib8b8/aflare/internal/compress"
	"github.com/alib8b8/aflare/internal/metrics"
)

// Conversation compaction for the ReAct agent, following the harness
// pattern that measurably improved Codex ARC-AGI-3 scores (13.3% → 38.3%
// with context compaction + retained reasoning): within one agent Run the
// tool observations are usually the bulk of the context while the model's
// own reasoning (assistant messages) is small and disproportionately
// valuable. When the running conversation exceeds a character budget we
// therefore compress the bulky middle (observations) via the algorithmic
// compress package — no extra LLM call — while KEEPING assistant
// ("thought") messages verbatim: retained reasoning.

const (
	// defaultAgentContextBudget is the conversation char budget below its
	// normal operating size; 0 disables compaction.
	defaultAgentContextBudget = 65536
	// agentKeepRecentMessages is how many trailing messages are always
	// kept verbatim (recent context is the most decision-relevant).
	agentKeepRecentMessages = 6
)

var (
	agentBudgetOnce sync.Once
	agentBudget     int
)

// defaultAgentContextBudgetResolved reads AFLARE_AGENT_CONTEXT_BUDGET once
// (chars; 0 or negative disables compaction) with a sane default.
func defaultAgentContextBudgetResolved() int {
	agentBudgetOnce.Do(func() {
		agentBudget = defaultAgentContextBudget
		if v := strings.TrimSpace(os.Getenv("AFLARE_AGENT_CONTEXT_BUDGET")); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				agentBudget = n
			}
		}
	})
	return agentBudget
}

// SetContextBudget sets the conversation character budget for this agent
// run. <= 0 disables compaction entirely.
func (a *ReActAgent) SetContextBudget(chars int) {
	a.contextBudget = chars
}

// conversationChars sums the content length of all messages.
func conversationChars(msgs []LLMMessage) int {
	total := 0
	for _, m := range msgs {
		total += len(m.Content)
	}
	return total
}

// compactConversation returns msgs unchanged while the total content is
// within budget. Once over budget it rebuilds the conversation as:
//
//	[system, initial user,
//	 user: "[Earlier steps compacted]" summary of middle observations,
//	 assistant messages from the middle, verbatim (retained reasoning),
//	 ...last agentKeepRecentMessages messages]
//
// so the model keeps its own reasoning and the freshest exchanges while
// stale tool output collapses into a compressed summary. The function is
// deterministic and side-effect free apart from the metrics counter.
func compactConversation(msgs []LLMMessage, budget int) []LLMMessage {
	if budget <= 0 || len(msgs) <= 2+agentKeepRecentMessages {
		return msgs
	}
	if conversationChars(msgs) <= budget {
		return msgs
	}

	head := msgs[:2] // system + initial user
	tail := msgs[len(msgs)-agentKeepRecentMessages:]
	middle := msgs[2 : len(msgs)-agentKeepRecentMessages]

	// Split the middle: assistant messages are retained verbatim; every
	// other message (tool results / observations) is compressed.
	var retained []LLMMessage
	var bulky strings.Builder
	for _, m := range middle {
		if m.Role == "assistant" {
			retained = append(retained, m)
			continue
		}
		bulky.WriteString(m.Role)
		bulky.WriteString(": ")
		bulky.WriteString(m.Content)
		bulky.WriteString("\n")
	}

	summaryMsg := LLMMessage{}
	if s := bulky.String(); s != "" {
		res := compress.Compress(s, compress.Config{
			Algorithm:       compress.AlgoHybrid,
			TargetRatio:     0.2,
			MaxOutputChars:  4000,
			PreserveHeaders: true,
			PreserveNumbers: true,
		})
		summaryMsg = LLMMessage{
			Role: "user",
			Content: "[Earlier steps compacted — summary of previous tool " +
				"observations]\n" + res.Text,
		}
	}

	out := make([]LLMMessage, 0, 2+1+len(retained)+len(tail))
	out = append(out, head...)
	if summaryMsg.Content != "" {
		out = append(out, summaryMsg)
	}
	out = append(out, retained...)
	out = append(out, tail...)

	metrics.IncAgentContextCompactions()
	return out
}
