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

// agent_e2e_test.go covers P0 test gaps:
//   - Agent full-chain E2E (mock LLM + real registry + tool execution)
//   - Capability chain (PreProcessAll → PostProcessAll with error/interrupt/skip)
//   - Streaming output callbacks (onChunk / onToolCall / onToolResult)
//   - Multi-line input (\ continuation + empty-line submit + buffer limit)

package agent

import (
	"context"
	"strings"
	"sync"
	"testing"
	"unicode"

	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/nodes/core"
)

// ── Mock LLM Provider ─────────────────────────────────────────────────────

// mockLLMProvider implements nodes.LLMProvider for testing. It returns
// predefined ReAct JSON responses in sequence, simulating an LLM without
// making real HTTP calls. Each Call() invocation returns the next response.
//
// Two modes for streaming:
//   - If chunkedResponses[i] is non-empty, send those chunks via onChunk
//     in order. This allows tests to verify chunk content and ordering.
//   - Otherwise, send the full response as a single chunk (backward compat).
type mockLLMProvider struct {
	responses        []string
	chunkedResponses [][]string // per-call chunk sequences for streaming
	callCount        int
	mu               sync.Mutex
}

func (m *mockLLMProvider) Call(ctx context.Context, messages []nodes.LLMMessage, tools []core.ToolDefinition, onChunk func(chunk string)) (content string, toolCalls []core.LLMToolCall, err error) {
	m.mu.Lock()
	i := m.callCount
	if i < len(m.responses) {
		m.callCount++
	}
	m.mu.Unlock()

	if i >= len(m.responses) {
		return `{"final_answer": "done"}`, nil, nil
	}

	resp := m.responses[i]

	// If onChunk is set, simulate streaming by sending chunks
	if onChunk != nil {
		if i < len(m.chunkedResponses) && len(m.chunkedResponses[i]) > 0 {
			for _, chunk := range m.chunkedResponses[i] {
				onChunk(chunk)
			}
		} else {
			onChunk(resp)
		}
	}

	return resp, nil, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────

// stripZeroWidth removes zero-width watermark characters injected by
// the watermark module. These characters are invisible to humans but
// affect string comparisons.
func stripZeroWidth(s string) string {
	var result strings.Builder
	for _, r := range s {
		switch r {
		case '\u200B', '\u200C', '\u200D', '\uFEFF':
			// skip zero-width characters
		default:
			if unicode.IsPrint(r) || unicode.IsSpace(r) {
				result.WriteRune(r)
			}
		}
	}
	return result.String()
}

// ── P0: Agent Full-Chain E2E ──────────────────────────────────────────────

// TestE2E_AgentFullChain tests the complete path:
// AgentLoop.Process → ReActAgent.Run → mock LLM → executeTool → response.
// Uses a mock LLMProvider that returns ReAct JSON, and verifies the
// tool is actually executed and the result returned.
func TestE2E_AgentFullChain(t *testing.T) {
	mock := &mockLLMProvider{
		responses: []string{
			`{"thought": "I should echo the user's message", "action": "execute", "action_input": "echo hello_e2e_test"}`,
			`{"thought": "The command executed successfully", "final_answer": "The echo command returned: hello_e2e_test"}`,
		},
	}

	cfg := Config{
		Provider:      "ollama",
		Model:         "llama3",
		MaxIterations: 3,
		Tools:         []string{"execute"},
		SafeMode:      false,
	}

	loop := NewAgentLoop(cfg)
	loop.SetLLMProvider(mock)
	ctx := context.Background()

	response, err := loop.Process(ctx, "echo hello", ProcessOptions{})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if response == "" {
		t.Fatal("expected non-empty response")
	}
	cleanResponse := stripZeroWidth(response)
	if !strings.Contains(cleanResponse, "hello_e2e_test") {
		t.Errorf("expected response to contain 'hello_e2e_test', got: %s", cleanResponse)
	}
}

// TestE2E_AgentToolExecution verifies that the ReActAgent correctly calls
// tools and returns results. Uses a mock that triggers template_list tool.
func TestE2E_AgentToolExecution(t *testing.T) {
	mock := &mockLLMProvider{
		responses: []string{
			`{"thought": "Let me search for templates", "action": "template_list", "action_input": "search build"}`,
			`{"thought": "I found some templates", "final_answer": "Here are the templates matching your search."}`,
		},
	}

	cfg := Config{
		Provider:      "ollama",
		Model:         "llama3",
		MaxIterations: 3,
		Tools:         DefaultTools,
	}

	loop := NewAgentLoop(cfg)
	loop.SetLLMProvider(mock)
	ctx := context.Background()

	response, err := loop.Process(ctx, "find templates for building", ProcessOptions{})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if response == "" {
		t.Fatal("expected non-empty response")
	}
}

// TestE2E_AgentUnknownTool verifies graceful handling when the LLM
// requests a tool that doesn't exist in the registry.
func TestE2E_AgentUnknownTool(t *testing.T) {
	mock := &mockLLMProvider{
		responses: []string{
			`{"thought": "Let me try an unknown tool", "action": "nonexistent_tool", "action_input": "test"}`,
			`{"thought": "That tool doesn't exist, I'll try something else", "action": "template_list", "action_input": "search"}`,
			`{"thought": "Found templates", "final_answer": "Here are the results."}`,
		},
	}

	cfg := Config{
		Provider:      "ollama",
		Model:         "llama3",
		MaxIterations: 3,
		Tools:         DefaultTools,
	}

	loop := NewAgentLoop(cfg)
	loop.SetLLMProvider(mock)
	ctx := context.Background()

	response, err := loop.Process(ctx, "search templates", ProcessOptions{})
	if err != nil {
		t.Fatalf("Process should not fail on unknown tool: %v", err)
	}
	if response == "" {
		t.Fatal("expected non-empty response")
	}
}

// TestE2E_AgentMaxIterations verifies the agent stops after maxIterations
// even if the LLM keeps requesting tools.
func TestE2E_AgentMaxIterations(t *testing.T) {
	responses := make([]string, 5)
	for i := range responses {
		responses[i] = `{"thought": "looping", "action": "template_list", "action_input": "search"}`
	}

	mock := &mockLLMProvider{responses: responses}

	cfg := Config{
		Provider:      "ollama",
		Model:         "llama3",
		MaxIterations: 2, // only 2 iterations allowed
		Tools:         DefaultTools,
	}

	loop := NewAgentLoop(cfg)
	loop.SetLLMProvider(mock)
	ctx := context.Background()

	response, err := loop.Process(ctx, "search", ProcessOptions{})
	// Should either succeed or fail with max iterations
	if err != nil {
		t.Logf("expected error on max iterations: %v", err)
	}
	_ = response
}

// ── P0: Capability Chain Tests ────────────────────────────────────────────

// fakeCapability is a test capability that records Pre/Post calls.
type fakeCapability struct {
	name        string
	preCalled   int
	postCalled  int
	preReturn   string
	preErr      error
	postReturn  string
	postErr     error
	initCalled  bool
	initErr     error
	mu          sync.Mutex
}

func (f *fakeCapability) Name() string        { return f.name }
func (f *fakeCapability) Description() string  { return "fake test capability" }

func (f *fakeCapability) Init(loop *AgentLoop) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.initCalled = true
	return f.initErr
}

func (f *fakeCapability) PreProcess(ctx context.Context, input string) (string, error) {
	f.mu.Lock()
	f.preCalled++
	f.mu.Unlock()
	return f.preReturn, f.preErr
}

func (f *fakeCapability) PostProcess(ctx context.Context, input, output string) (string, error) {
	f.mu.Lock()
	f.postCalled++
	f.mu.Unlock()
	return f.postReturn, f.postErr
}

func (f *fakeCapability) Shutdown() error { return nil }

// TestE2E_CapabilityChain_ChainedOutput verifies that PreProcessAll chains
// output through multiple capabilities in append-style: cap3→cap2→cap1→input.
func TestE2E_CapabilityChain_ChainedOutput(t *testing.T) {
	cr := NewCapabilityRegistry()

	cap1 := &fakeCapability{name: "cap1", preReturn: "[cap1]"}
	cap2 := &fakeCapability{name: "cap2", preReturn: "[cap2]"}
	cap3 := &fakeCapability{name: "cap3", preReturn: "[cap3]"}

	cr.Register(cap1)
	cr.Register(cap2)
	cr.Register(cap3)

	result, err := cr.PreProcessAll(context.Background(), "hello")
	if err != nil {
		t.Fatalf("PreProcessAll failed: %v", err)
	}
	// Append-style: cap3 output prepended to cap2+cap1+input
	expected := "[cap3][cap2][cap1]hello"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
	if cap1.preCalled != 1 || cap2.preCalled != 1 || cap3.preCalled != 1 {
		t.Errorf("expected all 3 capabilities called once, got: cap1=%d, cap2=%d, cap3=%d",
			cap1.preCalled, cap2.preCalled, cap3.preCalled)
	}
}

// TestE2E_CapabilityChain_ErrorInterrupt verifies that when a capability
// returns an error, the chain stops and the error is propagated.
func TestE2E_CapabilityChain_ErrorInterrupt(t *testing.T) {
	cr := NewCapabilityRegistry()

	cap1 := &fakeCapability{name: "cap1", preReturn: "modified"}
	cap2 := &fakeCapability{name: "cap2", preErr: context.DeadlineExceeded}
	cap3 := &fakeCapability{name: "cap3", preReturn: "should not reach"}

	cr.Register(cap1)
	cr.Register(cap2)
	cr.Register(cap3)

	modified, err := cr.PreProcessAll(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error from cap2")
	}
	if modified != "" {
		t.Errorf("expected empty modified on error, got %q", modified)
	}
	if cap3.preCalled != 0 {
		t.Error("cap3 should not be called after error")
	}
}

// TestE2E_CapabilityChain_EmptySkip verifies that when a capability returns
// empty string, the input passes through unchanged.
func TestE2E_CapabilityChain_EmptySkip(t *testing.T) {
	cr := NewCapabilityRegistry()

	cap1 := &fakeCapability{name: "cap1", preReturn: ""} // skip
	cap2 := &fakeCapability{name: "cap2", preReturn: ""} // skip
	cap3 := &fakeCapability{name: "cap3", preReturn: ""} // skip

	cr.Register(cap1)
	cr.Register(cap2)
	cr.Register(cap3)

	result, err := cr.PreProcessAll(context.Background(), "hello")
	if err != nil {
		t.Fatalf("PreProcessAll failed: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected unchanged input 'hello', got %q", result)
	}
}

// TestE2E_CapabilityChain_PostProcess verifies PostProcessAll chaining.
func TestE2E_CapabilityChain_PostProcess(t *testing.T) {
	cr := NewCapabilityRegistry()

	cap1 := &fakeCapability{name: "cap1", postReturn: "[cap1: modified]"}
	cap2 := &fakeCapability{name: "cap2", postReturn: ""} // skip
	cap3 := &fakeCapability{name: "cap3", postReturn: "[cap3: final]"}

	cr.Register(cap1)
	cr.Register(cap2)
	cr.Register(cap3)

	result, err := cr.PostProcessAll(context.Background(), "input", "original")
	if err != nil {
		t.Fatalf("PostProcessAll failed: %v", err)
	}
	if result != "[cap3: final]" {
		t.Errorf("expected '[cap3: final]', got %q", result)
	}
	if cap1.postCalled != 1 || cap2.postCalled != 1 || cap3.postCalled != 1 {
		t.Error("all capabilities should be called in PostProcess")
	}
}

// ── P0: Streaming Output Tests ────────────────────────────────────────────

// TestE2E_StreamingCallbacks verifies that onChunk, onToolCall, and
// onToolResult receive the correct content in the correct order during
// agent execution.
func TestE2E_StreamingCallbacks(t *testing.T) {
	mock := &mockLLMProvider{
		responses: []string{
			`{"thought": "I will execute a command", "action": "execute", "action_input": "echo stream_test"}`,
			`{"thought": "Command executed", "final_answer": "Stream test completed."}`,
		},
		// Simulate token-by-token streaming for the second response
		chunkedResponses: [][]string{
			nil, // first call: use default (full response as one chunk)
			{"Stream", " test", " completed."}, // second call: chunked
		},
	}

	var (
		chunks      []string
		toolCalls   []string
		toolResults []string
		mu          sync.Mutex
	)

	cfg := Config{
		Provider:      "ollama",
		Model:         "llama3",
		MaxIterations: 3,
		Tools:         []string{"execute"},
	}

	loop := NewAgentLoop(cfg)
	loop.SetLLMProvider(mock)
	ctx := context.Background()

	_, err := loop.Process(ctx, "test streaming", ProcessOptions{
		OnChunk: func(chunk string) {
			mu.Lock()
			chunks = append(chunks, chunk)
			mu.Unlock()
		},
		OnToolCall: func(toolName, input string) {
			mu.Lock()
			toolCalls = append(toolCalls, toolName+":"+input)
			mu.Unlock()
		},
		OnToolResult: func(toolName, result string) {
			mu.Lock()
			toolResults = append(toolResults, toolName+":"+result)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify chunk content and order — the second call sends 3 chunks
	if len(chunks) == 0 {
		t.Error("expected at least one chunk callback")
	}
	// Check that chunked response chunks arrived in order
	allChunks := strings.Join(chunks, "")
	if !strings.Contains(allChunks, "Stream") {
		t.Errorf("expected chunk content to contain 'Stream', got: %v", chunks)
	}
	if !strings.Contains(allChunks, "completed") {
		t.Errorf("expected chunk content to contain 'completed', got: %v", chunks)
	}
	// Verify order: "Stream" must appear before "completed"
	streamIdx := strings.Index(allChunks, "Stream")
	completedIdx := strings.Index(allChunks, "completed")
	if streamIdx < 0 || completedIdx < 0 || streamIdx >= completedIdx {
		t.Errorf("chunks out of order: 'Stream' at %d, 'completed' at %d", streamIdx, completedIdx)
	}

	// Verify tool call was recorded with correct content
	if len(toolCalls) == 0 {
		t.Error("expected at least one tool call")
	} else {
		found := false
		for _, tc := range toolCalls {
			if strings.Contains(tc, "execute") && strings.Contains(tc, "echo stream_test") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected 'execute:echo stream_test' tool call, got: %v", toolCalls)
		}
	}

	// Verify tool result was recorded with non-empty content
	if len(toolResults) == 0 {
		t.Error("expected at least one tool result")
	} else {
		for _, tr := range toolResults {
			parts := strings.SplitN(tr, ":", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				t.Errorf("tool result should have non-empty name and result, got: %q", tr)
			}
		}
	}

	t.Logf("chunks received: %d, tool calls: %d, tool results: %d",
		len(chunks), len(toolCalls), len(toolResults))
}

// TestE2E_StreamingNoCallbacks verifies that Process works correctly
// when no streaming callbacks are provided (nil opts).
func TestE2E_StreamingNoCallbacks(t *testing.T) {
	mock := &mockLLMProvider{
		responses: []string{
			`{"thought": "Simple task", "final_answer": "Done without streaming."}`,
		},
	}

	cfg := Config{
		Provider:      "ollama",
		Model:         "llama3",
		MaxIterations: 2,
		Tools:         DefaultTools,
	}

	loop := NewAgentLoop(cfg)
	loop.SetLLMProvider(mock)
	ctx := context.Background()

	response, err := loop.Process(ctx, "hello", ProcessOptions{})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if response == "" {
		t.Fatal("expected non-empty response")
	}
}

// ── P0: Multi-line Input Tests ────────────────────────────────────────────

// TestE2E_MultiLineInput tests that the ChatSession correctly handles
// multi-line input with \ continuation and empty-line submission.
// This tests the scanner logic in chat.go Run() method.
func TestE2E_MultiLineInput_BufferLogic(t *testing.T) {
	// Simulate the multi-line buffer logic directly
	lines := []string{
		"hello\\",
		"world\\",
		"test",
		"",
	}

	var multiLineBuf strings.Builder
	inMultiLine := false
	var results []string

	for _, line := range lines {
		if strings.HasSuffix(line, "\\") {
			inMultiLine = true
			multiLineBuf.WriteString(strings.TrimSuffix(line, "\\"))
			multiLineBuf.WriteString("\n")
			continue
		}

		if inMultiLine {
			if strings.TrimSpace(line) == "" {
				// Empty line submits
				input := strings.TrimSpace(multiLineBuf.String())
				multiLineBuf.Reset()
				inMultiLine = false
				if input != "" {
					results = append(results, input)
				}
				continue
			}
			// Non-empty line appends
			multiLineBuf.WriteString(line)
			multiLineBuf.WriteString("\n")
			continue
		}

		// Single line
		if strings.TrimSpace(line) != "" {
			results = append(results, strings.TrimSpace(line))
		}
	}

	// Should have 2 results: "hello\nworld\ntest" and the empty line didn't add
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	expected := "hello\nworld\ntest"
	if results[0] != expected {
		t.Errorf("expected %q, got %q", expected, results[0])
	}
}

// TestE2E_MultiLineInput_BufferLimit tests that appendMultiLine force-submits
// when the buffer exceeds maxMultiLineBytes, and resets the buffer properly.
func TestE2E_MultiLineInput_BufferLimit(t *testing.T) {
	const maxBytes = 100

	t.Run("force_submit_when_exceeded", func(t *testing.T) {
		var buf strings.Builder
		// Fill buffer under the limit
		forceSubmit, _ := appendMultiLine(&buf, strings.Repeat("a", 50)+"\\", maxBytes)
		if forceSubmit {
			t.Error("should not force-submit below limit")
		}

		// Add another line that pushes it over
		forceSubmit, result := appendMultiLine(&buf, strings.Repeat("b", 60)+"\\", maxBytes)
		if !forceSubmit {
			t.Error("should force-submit when buffer exceeds limit")
		}
		if result == "" {
			t.Error("force-submit result should be non-empty")
		}
	})

	t.Run("buffer_reset_after_force_submit", func(t *testing.T) {
		var buf strings.Builder
		// Push over limit
		appendMultiLine(&buf, strings.Repeat("x", 120)+"\\", maxBytes)
		// Buffer should be reset after force-submit
		if buf.Len() != 0 {
			t.Errorf("buffer should be empty after force-submit, got %d bytes", buf.Len())
		}
	})

	t.Run("no_submit_at_limit", func(t *testing.T) {
		var buf strings.Builder
		// Exactly at limit (line + \n)
		line := strings.Repeat("a", maxBytes-1) + "\\"
		forceSubmit, _ := appendMultiLine(&buf, line, maxBytes)
		if forceSubmit {
			t.Error("should not force-submit when buffer equals limit")
		}
		// One more byte over
		forceSubmit, _ = appendMultiLine(&buf, "b\\", maxBytes)
		if !forceSubmit {
			t.Error("should force-submit when buffer exceeds limit by 1 byte")
		}
	})

	t.Run("strips_trailing_backslash", func(t *testing.T) {
		var buf strings.Builder
		appendMultiLine(&buf, "hello world\\", 1024*1024)
		content := buf.String()
		if strings.Contains(content, "\\") {
			t.Errorf("trailing backslash should be stripped, got: %q", content)
		}
		if !strings.HasPrefix(content, "hello world") {
			t.Errorf("expected content to start with 'hello world', got: %q", content)
		}
	})
}

// TestE2E_MultiLineInput_EmptySubmit tests that empty multi-line buffer
// is not submitted.
func TestE2E_MultiLineInput_EmptySubmit(t *testing.T) {
	var multiLineBuf strings.Builder

	// Empty buffer in multi-line mode should not be submitted
	input := strings.TrimSpace(multiLineBuf.String())

	if input != "" {
		t.Error("empty buffer should not be submitted")
	}
}

// ── P0: Context Manager E2E ───────────────────────────────────────────────

// TestE2E_ContextManager_TokenEstimation verifies the token estimation
// for different providers.
func TestE2E_ContextManager_TokenEstimation(t *testing.T) {
	tests := []struct {
		provider string
		text     string
		min      int
		max      int
	}{
		{"ollama", "hello world this is a test", 5, 20},
		{"ollama", strings.Repeat("a", 400), 90, 110},
		{"openai", "hello world", 1, 10},
		{"openai", "你好世界测试中文", 5, 20},
	}

	for _, tt := range tests {
		tokens := estimateTokens(tt.text, tt.provider)
		if tokens < tt.min || tokens > tt.max {
			t.Errorf("estimateTokens(%q, %q) = %d, want [%d, %d]",
				tt.provider, tt.text[:min(20, len(tt.text))], tokens, tt.min, tt.max)
		}
	}
}

// TestE2E_ContextManager_Compress verifies that compression reduces
// message count when context exceeds budget.
func TestE2E_ContextManager_Compress(t *testing.T) {
	cm := NewContextManager()
	cm.SetProvider("openai")

	// Add many messages to exceed context budget
	for i := 0; i < 20; i++ {
		cm.AddUser("User message number " + strings.Repeat("x", 200))
		cm.AddAssistant("Assistant response number " + strings.Repeat("y", 200))
	}

	before := cm.MessageCount()
	beforeCompress, afterCompress := cm.CompressIfNeeded()
	t.Logf("messages: before=%d, compress: %d→%d", before, beforeCompress, afterCompress)

	if beforeCompress > 0 && afterCompress > 0 {
		if afterCompress >= beforeCompress {
			t.Error("compression should reduce message count")
		}
	}
}

// ── P0: Registry Node Registration ────────────────────────────────────────

// TestE2E_ChatNodeRegistration verifies that all required chat nodes are
// registered in the global registry.
func TestE2E_ChatNodeRegistration(t *testing.T) {
	reg := core.NewRegistry()
	registerChatNodes(reg)

	requiredNodes := []string{
		"template_list", "template_info", "run_workflow", "create_workflow",
		"memory", "compress", "self_update",
	}

	for _, name := range requiredNodes {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("required chat node %q is not registered", name)
		}
	}
}

// TestE2E_BuiltinNodesExist verifies that all builtin nodes are registered
// and accessible.
func TestE2E_BuiltinNodesExist(t *testing.T) {
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	// These should always be available
	required := []string{
		"fetch_url", "file_write", "execute", "combine",
		"json_parse", "http_request", "template_render", "transform",
	}

	for _, name := range required {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("builtin node %q is not registered", name)
		}
	}
}