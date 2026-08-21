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
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

// slowEchoNode is a test node that sleeps before echoing its input back,
// tracking how many executions overlap in time.
type slowEchoNode struct {
	delay       time.Duration
	mu          sync.Mutex
	inFlight    int
	maxInFlight int
}

func (n *slowEchoNode) Name() string        { return "slow_echo" }
func (n *slowEchoNode) Description() string { return "echoes input after a delay" }
func (n *slowEchoNode) Schema() core.NodeSchema {
	return core.NodeSchema{
		Name:        "slow_echo",
		Description: "echoes input after a delay",
		Input:       "text",
		Output:      "echoed text",
	}
}

func (n *slowEchoNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	n.mu.Lock()
	n.inFlight++
	if n.inFlight > n.maxInFlight {
		n.maxInFlight = n.inFlight
	}
	n.mu.Unlock()

	select {
	case <-time.After(n.delay):
	case <-ctx.Done():
		return "", ctx.Err()
	}

	n.mu.Lock()
	n.inFlight--
	n.mu.Unlock()
	return "echo:" + input, nil
}

// multiToolProvider is a mock LLMProvider whose first Call returns several
// native tool_calls; the second returns a final answer.
type multiToolProvider struct {
	calls int
	mu    sync.Mutex
}

func (p *multiToolProvider) Call(ctx context.Context, messages []LLMMessage, tools []core.ToolDefinition, onChunk func(chunk string)) (string, []core.LLMToolCall, error) {
	p.mu.Lock()
	p.calls++
	n := p.calls
	p.mu.Unlock()

	if n == 1 {
		return "", []core.LLMToolCall{
			{ID: "c1", Type: "function", Function: core.LLMToolCallFunc{Name: "slow_tool", Arguments: `{"input":"first"}`}},
			{ID: "c2", Type: "function", Function: core.LLMToolCallFunc{Name: "slow_tool", Arguments: `{"input":"second"}`}},
			{ID: "c3", Type: "function", Function: core.LLMToolCallFunc{Name: "slow_tool", Arguments: `{"input":"third"}`}},
		}, nil
	}
	return `{"thought":"done","final_answer":"all tools done"}`, nil, nil
}

// TestReActAgent_ParallelToolCalls verifies that multiple tool calls in one
// native-function-calling round execute concurrently and that their
// observations reach the conversation in call order.
func TestReActAgent_ParallelToolCalls(t *testing.T) {
	node := &slowEchoNode{delay: 120 * time.Millisecond}
	reg := NewRegistry()
	reg.Register(node)

	provider := &multiToolProvider{}
	agent := NewReActAgent(
		"openai", "gpt-4", "key", "https://example.com",
		"test prompt", 5,
		[]AgentTool{{Name: "slow_tool", Description: "slow echo", NodeName: "slow_echo"}},
		reg, false, false,
	)
	agent.SetLLMProvider(provider)

	start := time.Now()
	answer, err := agent.Run(context.Background(), "run three tools")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if answer != "all tools done" {
		t.Errorf("answer = %q, want %q", answer, "all tools done")
	}

	// Concurrency: three 120ms tools running in parallel finish well below
	// the 360ms a serial loop would need.
	if elapsed >= 300*time.Millisecond {
		t.Errorf("elapsed = %v, want < 300ms (tools should run concurrently)", elapsed)
	}
	node.mu.Lock()
	maxConc := node.maxInFlight
	node.mu.Unlock()
	if maxConc < 2 {
		t.Errorf("max concurrent tool executions = %d, want >= 2", maxConc)
	}
}

// TestReActAgent_ParallelToolCallsOrdering verifies observations are appended
// to the conversation in call order even when tools finish out of order.
func TestReActAgent_ParallelToolCallsOrdering(t *testing.T) {
	var mu sync.Mutex
	var toolMessages []string

	node := &orderedEchoNode{delays: map[string]time.Duration{
		"first":  150 * time.Millisecond, // slowest, finishes last
		"second": 10 * time.Millisecond,  // fastest, finishes first
	}}
	reg := NewRegistry()
	reg.Register(node)

	provider := &orderProvider{conversations: &mu, toolMessages: &toolMessages}
	agent := NewReActAgent(
		"openai", "gpt-4", "key", "https://example.com",
		"test prompt", 5,
		[]AgentTool{{Name: "ordered_tool", Description: "ordered echo", NodeName: "ordered_echo"}},
		reg, false, false,
	)
	agent.SetLLMProvider(provider)

	if _, err := agent.Run(context.Background(), "run tools"); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// The final LLM call received tool-role messages in the ORIGINAL call
	// order (first, second), not completion order.
	want := []string{"echo:first", "echo:second"}
	if len(toolMessages) != len(want) {
		t.Fatalf("tool messages = %v, want %v", toolMessages, want)
	}
	for i, w := range want {
		if toolMessages[i] != w {
			t.Errorf("tool message[%d] = %q, want %q (observations must stay in call order)", i, toolMessages[i], w)
		}
	}
}

// orderedEchoNode echoes its input with a per-input delay so later-listed
// tools finish before earlier-listed ones.
type orderedEchoNode struct {
	delays map[string]time.Duration
}

func (n *orderedEchoNode) Name() string        { return "ordered_echo" }
func (n *orderedEchoNode) Description() string { return "echoes input after a per-input delay" }
func (n *orderedEchoNode) Schema() core.NodeSchema {
	return core.NodeSchema{Name: "ordered_echo", Description: "echo", Input: "text", Output: "echoed"}
}

func (n *orderedEchoNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	input = strings.TrimSpace(input)
	d := n.delays[input]
	if d == 0 {
		d = 20 * time.Millisecond
	}
	select {
	case <-time.After(d):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return "echo:" + input, nil
}

// orderProvider returns two tool calls (slow "first", fast "second") and then
// records every tool-role message it receives on the final call.
type orderProvider struct {
	calls         int
	conversations *sync.Mutex
	toolMessages  *[]string
}

func (p *orderProvider) Call(ctx context.Context, messages []LLMMessage, tools []core.ToolDefinition, onChunk func(chunk string)) (string, []core.LLMToolCall, error) {
	p.calls++
	if p.calls == 1 {
		return "", []core.LLMToolCall{
			{ID: "c1", Type: "function", Function: core.LLMToolCallFunc{Name: "ordered_tool", Arguments: `{"input":"first"}`}},
			{ID: "c2", Type: "function", Function: core.LLMToolCallFunc{Name: "ordered_tool", Arguments: `{"input":"second"}`}},
		}, nil
	}
	p.conversations.Lock()
	for _, m := range messages {
		if m.Role == "tool" {
			*p.toolMessages = append(*p.toolMessages, m.Content)
		}
	}
	p.conversations.Unlock()
	return `{"thought":"done","final_answer":"done"}`, nil, nil
}

// TestReActAgent_SingleToolCallUnchanged verifies a single tool call still
// executes inline via the legacy path (no goroutine overhead, immediate
// callbacks) and the observation reaches the conversation.
func TestReActAgent_SingleToolCallUnchanged(t *testing.T) {
	node := &slowEchoNode{delay: 10 * time.Millisecond}
	reg := NewRegistry()
	reg.Register(node)

	var callbacks int32
	provider := &singleToolProvider{}
	agent := NewReActAgent(
		"openai", "gpt-4", "key", "https://example.com",
		"test prompt", 5,
		[]AgentTool{{Name: "slow_tool", Description: "slow echo", NodeName: "slow_echo"}},
		reg, false, false,
	)
	agent.SetLLMProvider(provider)
	agent.SetCallbacks(nil,
		func(toolName, input string) { atomic.AddInt32(&callbacks, 1) },
		func(toolName, result string) { atomic.AddInt32(&callbacks, 1) },
	)

	answer, err := agent.Run(context.Background(), "run one tool")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if answer != "single done" {
		t.Errorf("answer = %q, want %q", answer, "single done")
	}
	if got := atomic.LoadInt32(&callbacks); got != 2 {
		t.Errorf("callback invocations = %d, want 2 (one call + one result)", got)
	}
}

type singleToolProvider struct {
	calls int
}

func (p *singleToolProvider) Call(ctx context.Context, messages []LLMMessage, tools []core.ToolDefinition, onChunk func(chunk string)) (string, []core.LLMToolCall, error) {
	p.calls++
	if p.calls == 1 {
		return "", []core.LLMToolCall{
			{ID: "c1", Type: "function", Function: core.LLMToolCallFunc{Name: "slow_tool", Arguments: `{"input":"only"}`}},
		}, nil
	}
	return `{"thought":"done","final_answer":"single done"}`, nil, nil
}

// TestReActAgent_ParallelToolCallsError verifies a failing tool in a parallel
// round becomes an "Error: ..." observation without failing the whole run.
func TestReActAgent_ParallelToolCallsError(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&okEchoNode{})
	reg.Register(&failEchoNode{})

	provider := &mixedProvider{}
	agent := NewReActAgent(
		"openai", "gpt-4", "key", "https://example.com",
		"test prompt", 5,
		[]AgentTool{
			{Name: "ok_tool", Description: "ok", NodeName: "ok_echo"},
			{Name: "bad_tool", Description: "bad", NodeName: "fail_echo"},
		},
		reg, false, false,
	)
	agent.SetLLMProvider(provider)

	answer, err := agent.Run(context.Background(), "run mixed tools")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if answer != "mixed done" {
		t.Errorf("answer = %q, want %q", answer, "mixed done")
	}
	for _, obs := range provider.finalToolMessages {
		if strings.HasPrefix(obs, "Error:") {
			continue // expected for bad_tool
		}
		if obs != "echo:ok" {
			t.Errorf("observation = %q, want %q", obs, "echo:ok")
		}
	}
}

type okEchoNode struct{}

func (n *okEchoNode) Name() string        { return "ok_echo" }
func (n *okEchoNode) Description() string { return "ok echo" }
func (n *okEchoNode) Schema() core.NodeSchema {
	return core.NodeSchema{Name: "ok_echo", Description: "ok", Input: "text", Output: "echoed"}
}
func (n *okEchoNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	return "echo:" + strings.TrimSpace(input), nil
}

type failEchoNode struct{}

func (n *failEchoNode) Name() string        { return "fail_echo" }
func (n *failEchoNode) Description() string { return "always fails" }
func (n *failEchoNode) Schema() core.NodeSchema {
	return core.NodeSchema{Name: "fail_echo", Description: "fails", Input: "text", Output: "none"}
}
func (n *failEchoNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	return "", fmt.Errorf("boom")
}

type mixedProvider struct {
	calls             int
	finalToolMessages []string
}

func (p *mixedProvider) Call(ctx context.Context, messages []LLMMessage, tools []core.ToolDefinition, onChunk func(chunk string)) (string, []core.LLMToolCall, error) {
	p.calls++
	if p.calls == 1 {
		return "", []core.LLMToolCall{
			{ID: "c1", Type: "function", Function: core.LLMToolCallFunc{Name: "ok_tool", Arguments: `{"input":"ok"}`}},
			{ID: "c2", Type: "function", Function: core.LLMToolCallFunc{Name: "bad_tool", Arguments: `{"input":"bad"}`}},
		}, nil
	}
	for _, m := range messages {
		if m.Role == "tool" {
			p.finalToolMessages = append(p.finalToolMessages, m.Content)
		}
	}
	return `{"thought":"done","final_answer":"mixed done"}`, nil, nil
}
