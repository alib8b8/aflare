// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​​‌‌‌​‌​​​​‌​​‌‌‌‌​​​‌​​​‌​​​‌‌​​‌​​‌‌​‌​‌​​​​​​​​​​​​​​​​​​​​​‌‌‌​​​​‌​​​‌​​​‌⁠
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

// loop.go implements the unified AgentLoop — the core execution loop that
// powers both chat mode (stdin) and daemon mode (multi-source events).
// The loop reads from an input channel, processes through ReActAgent, and
// routes responses to the appropriate output channel based on the input source.

package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/nodes/core"
	"github.com/alib8b8/aflare/internal/watermark"
)

// InputSource identifies where an agent input came from.
type InputSource string

const (
	SourceStdin     InputSource = "stdin"
	SourceScheduler InputSource = "scheduler"
	SourceFileWatch InputSource = "filewatch"
	SourceMCP       InputSource = "mcp"
	SourceHTTP      InputSource = "http"
)

// AgentInput carries a single input message with its source metadata.
type AgentInput struct {
	Source  InputSource        // where the input came from
	Message string             // the input text
	ReplyTo chan<- AgentOutput // where to send the response (nil = discard)
}

// AgentOutput carries a response from the agent with routing info.
type AgentOutput struct {
	Source   InputSource // original source (for routing)
	Response string
	Error    error
}

// AgentLoop is the unified execution loop. It reads inputs from a channel,
// processes them through the ReActAgent, and routes responses back.
// Multiple input sources (stdin, scheduler, filewatch, MCP, HTTP) feed into
// the same loop — the agent doesn't care where the input came from.
//
// The loop supports pluggable capabilities (reflection, human-in-the-loop,
// utility optimization, etc.) that hook into the execution cycle via
// PreProcess and PostProcess.
type AgentLoop struct {
	config      Config
	reg         *core.Registry
	tools       []core.AgentTool
	systemMsg   string
	ctx         *ContextManager
	interrupt   chan struct{}           // internal interrupt for Ctrl-C
	caps        *CapabilityRegistry     // pluggable capabilities
	onCompress  func(before, after int) // optional compression notification
	llmProvider nodes.LLMProvider       // optional: inject mock LLM provider for testing
}

// NewAgentLoop creates a new AgentLoop with the given configuration.
func NewAgentLoop(cfg Config) *AgentLoop {
	if cfg.Provider == "" {
		cfg.Provider = "ollama"
	}
	if cfg.Model == "" {
		cfg.Model = "llama3"
	}
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 10
	}
	if len(cfg.Tools) == 0 {
		cfg.Tools = DefaultTools
	}

	reg := core.GetGlobalRegistry()
	registerChatNodes(reg)
	nodes.RegisterBuiltins(reg)
	tools := buildToolList(cfg.Tools, cfg.SafeMode)

	cm := NewContextManager()
	cm.SetProvider(cfg.Provider)
	// P1-4: explicit budget override (Config.ContextBudget > 0) wins over
	// the provider default selected by SetProvider.
	if cfg.ContextBudget > 0 {
		cm.SetBudget(cfg.ContextBudget)
	}
	systemMsg := BuildSystemPrompt(tools, "0.0.0")
	cm.SetSystemPrompt(systemMsg)

	loop := &AgentLoop{
		config:    cfg,
		reg:       reg,
		tools:     tools,
		systemMsg: systemMsg,
		ctx:       cm,
		interrupt: make(chan struct{}, 1),
		caps:      NewCapabilityRegistry(),
	}

	// Initialize capabilities from config
	if len(cfg.Capabilities) > 0 {
		for _, name := range cfg.Capabilities {
			cap := CreateCapability(name)
			if cap != nil {
				loop.caps.Register(cap)
			}
		}
		if err := loop.caps.InitAll(loop); err != nil {
			log.Printf("[agent] capability init warning: %v", err)
		}
	}

	return loop
}

// Context returns the context manager for external access (e.g. /history, /clear).
func (a *AgentLoop) Context() *ContextManager {
	return a.ctx
}

// Tools returns the tool list for external access (e.g. /tools).
func (a *AgentLoop) Tools() []core.AgentTool {
	return a.tools
}

// Capabilities returns the capability registry for external access (e.g. /capabilities).
func (a *AgentLoop) Capabilities() *CapabilityRegistry {
	return a.caps
}

// SetCompressCallback sets a callback that is invoked when the context
// manager compresses older messages. The callback receives the before/after
// message counts.
func (a *AgentLoop) SetCompressCallback(fn func(before, after int)) {
	a.onCompress = fn
}

// SetLLMProvider injects a mock LLM provider for testing.
// When set, the agent uses this provider instead of making real HTTP calls.
func (a *AgentLoop) SetLLMProvider(p nodes.LLMProvider) {
	a.llmProvider = p
}

// ProcessOptions configures streaming and visibility callbacks for Process.
type ProcessOptions struct {
	OnChunk      func(chunk string)            // called per token during streaming
	OnToolCall   func(toolName, input string)  // called before tool execution
	OnToolResult func(toolName, result string) // called after tool execution
}

// Process is the unified entry point for all agent input processing.
// Both chat mode (with streaming) and daemon mode (no streaming) use this.
func (a *AgentLoop) Process(ctx context.Context, input string, opts ProcessOptions) (string, error) {
	// Run capability PreProcess hooks
	processedInput, err := a.caps.PreProcessAll(ctx, input)
	if err != nil {
		// If PreProcess returned an error with an empty input, it means
		// "skip this turn" (e.g. human-in-the-loop cancellation or awaiting approval).
		// Propagate the error to the caller so it can be displayed to the user.
		if processedInput == "" {
			return "", err
		}
		log.Printf("[agent] pre-process warning: %v", err)
		processedInput = input // fall back to original
	}
	if processedInput == "" {
		processedInput = input
	}

	a.ctx.AddUser(processedInput)

	agent := nodes.NewReActAgent(
		a.config.Provider,
		a.config.Model,
		a.config.APIKey,
		a.config.Endpoint,
		a.systemMsg,
		a.config.MaxIterations,
		a.tools,
		a.reg,
		false, // enableThinking
		false, // showThinking
	)

	// Set streaming and tool-visibility callbacks
	agent.SetCallbacks(opts.OnChunk, opts.OnToolCall, opts.OnToolResult)

	// Inject mock LLM provider if set (for testing)
	if a.llmProvider != nil {
		agent.SetLLMProvider(a.llmProvider)
	}

	var response string
	if opts.OnChunk != nil {
		response, err = agent.RunStream(ctx, a.ctx.BuildPrefix(), opts.OnChunk)
	} else {
		response, err = agent.Run(ctx, a.ctx.BuildPrefix())
	}
	if err != nil {
		return "", fmt.Errorf("agent execution failed: %w", err)
	}

	// Run capability PostProcess hooks
	processedOutput, capErr := a.caps.PostProcessAll(ctx, processedInput, response)
	if capErr != nil {
		log.Printf("[agent] post-process warning: %v", capErr)
		processedOutput = response // fall back to original
	}
	if processedOutput != "" {
		response = processedOutput
	}

	// Embed invisible watermark for content provenance. Disabled when
	// AFLARE_WATERMARK_DISABLE=1 (e.g. for compliance-sensitive deployments
	// where any extra metadata embedded in agent output — including the
	// generation timestamp carried by the watermark payload — must not
	// leak to downstream consumers). Default is enabled to preserve
	// content-provenance guarantees.
	if os.Getenv("AFLARE_WATERMARK_DISABLE") != "1" {
		response = watermark.EncodeTextWithSuffix(response)
	}

	a.ctx.AddAssistant(response)
	before, after := a.ctx.CompressIfNeeded()
	if before > 0 && after > 0 && before != after {
		log.Printf("[agent] context compressed: %d → %d messages", before, after)
		if a.onCompress != nil {
			a.onCompress(before, after)
		}
	}

	return response, nil
}

// Run starts the multi-source event loop. It reads from inputs and writes
// responses to the output channel. Returns when inputs is closed.
func (a *AgentLoop) Run(ctx context.Context, inputs <-chan AgentInput, outputs chan<- AgentOutput) {
	for {
		select {
		case <-ctx.Done():
			return
		case input, ok := <-inputs:
			if !ok {
				return
			}
			a.handleInput(ctx, input, outputs)
		}
	}
}

// handleInput processes a single input and routes the response.
func (a *AgentLoop) handleInput(parentCtx context.Context, input AgentInput, outputs chan<- AgentOutput) {
	processCtx, cancel := context.WithTimeout(parentCtx, DefaultSendTimeout)
	defer cancel()

	response, err := a.Process(processCtx, input.Message, ProcessOptions{})

	out := AgentOutput{
		Source:   input.Source,
		Response: response,
		Error:    err,
	}

	// Route to reply channel if provided
	if input.ReplyTo != nil {
		select {
		case input.ReplyTo <- out:
		case <-parentCtx.Done():
		default:
			// Don't block if reply channel is full
		}
	}

	// Also send to the shared output channel if available
	if outputs != nil {
		select {
		case outputs <- out:
		case <-parentCtx.Done():
		default:
		}
	}

	if err != nil {
		log.Printf("[agent] %s input failed: %v", input.Source, err)
	} else {
		log.Printf("[agent] %s input processed (%d chars)", input.Source, len(response))
	}
}

// NotifyEvent sends an event from a non-interactive source (scheduler, filewatch)
// and optionally waits for a response. The event is logged but not printed to stdout.
func (a *AgentLoop) NotifyEvent(ctx context.Context, source InputSource, message string) (string, error) {
	replyCh := make(chan AgentOutput, 1)
	input := AgentInput{
		Source:  source,
		Message: message,
		ReplyTo: replyCh,
	}

	a.handleInput(ctx, input, nil)

	select {
	case out := <-replyCh:
		return out.Response, out.Error
	case <-time.After(DefaultSendTimeout):
		return "", fmt.Errorf("event processing timed out")
	}
}

// buildToolList creates the agent tool list from tool names.
// Chat-specific tools (run_workflow, create_workflow, etc.) are always included.
// When safeMode is true, dangerous tools (execute, file_write, code_interpreter) are excluded.
func buildToolList(toolNames []string, safeMode bool) []core.AgentTool {
	chatTools := []core.AgentTool{
		{Name: "run_workflow", Description: "Run a workflow template. Input: JSON with 'template' and 'params'", NodeName: "run_workflow"},
		{Name: "create_workflow", Description: "Compose and run a new workflow from available nodes", NodeName: "create_workflow"},
		{Name: "memory_store", Description: "Store important information for later recall", NodeName: "memory"},
		{Name: "memory_retrieve", Description: "Recall previously stored information", NodeName: "memory"},
		{Name: "memory_search", Description: "Search memory for relevant context", NodeName: "memory"},
		{Name: "context_compress", Description: "Compress conversation history to free context space", NodeName: "compress"},
		{Name: "self_update", Description: "Check and install aflare updates from GitHub releases", NodeName: "self_update"},
	}

	dangerousTools := map[string]bool{
		"execute":          true,
		"file_write":       true,
		"code_interpreter": true,
	}

	userTools := core.ParseToolsList(strings.Join(toolNames, ","))

	seen := make(map[string]bool)
	var result []core.AgentTool
	for _, t := range chatTools {
		seen[t.Name] = true
		result = append(result, t)
	}
	for _, t := range userTools {
		if seen[t.Name] {
			continue
		}
		if safeMode && dangerousTools[t.Name] {
			continue
		}
		result = append(result, t)
	}
	return result
}
