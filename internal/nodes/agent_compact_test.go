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
	"strings"
	"testing"
)

// buildBulkyConversation returns a conversation of the shape the ReAct loop
// produces: [system, initial user] followed by alternating assistant
// ("thought") and user ("tool observation") messages, then a final tail.
// Observations are large and repetitive so compression has something to chew
// on; thoughts are short and unique so retention is observable.
func buildBulkyConversation(pairs, tailMsgs int, obs string) []LLMMessage {
	msgs := []LLMMessage{
		{Role: "system", Content: "You are a test agent."},
		{Role: "user", Content: "do the task"},
	}
	for i := 0; i < pairs; i++ {
		msgs = append(msgs,
			LLMMessage{Role: "assistant", Content: "thought-" + string(rune('a'+i)) + ": reasoning step"},
			LLMMessage{Role: "user", Content: obs},
		)
	}
	for i := 0; i < tailMsgs; i++ {
		role := "assistant"
		if i%2 == 1 {
			role = "user"
		}
		msgs = append(msgs, LLMMessage{Role: role, Content: "tail-message-" + string(rune('a'+i))})
	}
	return msgs
}

func TestCompactConversation_WithinBudgetUnchanged(t *testing.T) {
	msgs := buildBulkyConversation(4, 4, "small observation")
	out := compactConversation(msgs, 1<<20)
	if len(out) != len(msgs) {
		t.Fatalf("len = %d, want %d (no compaction within budget)", len(out), len(msgs))
	}
	for i := range msgs {
		if out[i] != msgs[i] {
			t.Errorf("msg[%d] changed: %+v vs %+v", i, out[i], msgs[i])
		}
	}
}

func TestCompactConversation_DisabledWithZeroBudget(t *testing.T) {
	msgs := buildBulkyConversation(4, 4, strings.Repeat("x", 5000))
	out := compactConversation(msgs, 0)
	if len(out) != len(msgs) {
		t.Fatalf("len = %d, want %d (budget 0 disables compaction)", len(out), len(msgs))
	}
}

func TestCompactConversation_TooShortNeverCompacted(t *testing.T) {
	// 2 + agentKeepRecentMessages messages: never compacted regardless of size.
	msgs := []LLMMessage{
		{Role: "system", Content: strings.Repeat("s", 10000)},
		{Role: "user", Content: strings.Repeat("u", 10000)},
	}
	for i := 0; i < agentKeepRecentMessages; i++ {
		msgs = append(msgs, LLMMessage{Role: "assistant", Content: strings.Repeat("a", 10000)})
	}
	out := compactConversation(msgs, 100)
	if len(out) != len(msgs) {
		t.Fatalf("len = %d, want %d (short conversation never compacted)", len(out), len(msgs))
	}
}

func TestCompactConversation_CompactsAndRetainsReasoning(t *testing.T) {
	obs := strings.Repeat("observation data line with plenty of detail 1234567890\n", 300) // ~19KB
	msgs := buildBulkyConversation(6, agentKeepRecentMessages, obs)
	if len(msgs) != 2+12+agentKeepRecentMessages {
		t.Fatalf("setup: len = %d", len(msgs))
	}

	budget := 8000 // conversation is far over budget
	out := compactConversation(msgs, budget)

	// 1. Head is preserved verbatim.
	if out[0].Role != "system" || out[0].Content != "You are a test agent." {
		t.Errorf("system message not preserved: %+v", out[0])
	}
	if out[1].Role != "user" || out[1].Content != "do the task" {
		t.Errorf("initial user message not preserved: %+v", out[1])
	}

	// 2. A compaction summary is inserted after the head.
	hasSummary := false
	for _, m := range out[2 : len(out)-agentKeepRecentMessages] {
		if strings.Contains(m.Content, "[Earlier steps compacted") {
			hasSummary = true
		}
	}
	if !hasSummary {
		t.Error("no compaction summary message found")
	}

	// 3. Retained reasoning: every middle assistant thought survives verbatim.
	for i := 0; i < 6; i++ {
		want := "thought-" + string(rune('a'+i)) + ": reasoning step"
		found := false
		for _, m := range out {
			if m.Role == "assistant" && m.Content == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("retained reasoning missing: %q", want)
		}
	}

	// 4. Tail is preserved verbatim and in order.
	tail := msgs[len(msgs)-agentKeepRecentMessages:]
	gotTail := out[len(out)-agentKeepRecentMessages:]
	for i := range tail {
		if gotTail[i] != tail[i] {
			t.Errorf("tail[%d] = %+v, want %+v", i, gotTail[i], tail[i])
		}
	}

	// 5. The bulky observations are gone as verbatim messages.
	for _, m := range out {
		if m.Content == obs {
			t.Error("verbatim bulky observation still present after compaction")
		}
	}

	// 6. Net size reduction.
	if got, want := conversationChars(out), conversationChars(msgs); got >= want {
		t.Errorf("conversationChars = %d, want < %d (compaction must shrink)", got, want)
	}
}

func TestCompactConversation_AllAssistantMiddleIsNoop(t *testing.T) {
	// Middle consisting purely of assistant messages: nothing bulky to
	// compress, so the conversation passes through message-for-message.
	msgs := []LLMMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "go"},
	}
	for i := 0; i < 8; i++ {
		msgs = append(msgs, LLMMessage{Role: "assistant", Content: "step " + strings.Repeat("r", 200)})
	}
	out := compactConversation(msgs, 100)
	if len(out) != len(msgs) {
		t.Fatalf("len = %d, want %d", len(out), len(msgs))
	}
	for i := range msgs {
		if out[i] != msgs[i] {
			t.Errorf("msg[%d] changed: %+v vs %+v", i, out[i], msgs[i])
		}
	}
}

func TestReActAgent_SetContextBudget(t *testing.T) {
	a := &ReActAgent{}
	a.SetContextBudget(4321)
	if a.contextBudget != 4321 {
		t.Errorf("contextBudget = %d, want 4321", a.contextBudget)
	}
	a.SetContextBudget(0) // disables compaction
	if a.contextBudget != 0 {
		t.Errorf("contextBudget = %d, want 0 (disabled)", a.contextBudget)
	}
}

func TestDefaultAgentContextBudgetResolvedPositive(t *testing.T) {
	if got := defaultAgentContextBudgetResolved(); got <= 0 {
		t.Errorf("default budget = %d, want > 0", got)
	}
}
