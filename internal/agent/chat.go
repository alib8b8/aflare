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
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alib8b8/aflare/internal/meta"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/nodes/core"
)

// DefaultTools is the default tool set for the chat agent.
// Safe by default: no execute, no file_write.
var DefaultTools = []string{
	"template_list", "template_info", "run_workflow", "create_workflow",
	"memory_store", "memory_retrieve", "memory_search",
	"fetch_url", "http_request", "file_read", "json_parse",
	"transform", "combine", "template",
}

// DefaultSendTimeout is the maximum time allowed for a single SendMessage call.
const DefaultSendTimeout = 5 * time.Minute

// Config holds the configuration for a ChatSession.
type Config struct {
	Provider      string   // LLM provider (default: "ollama")
	Model         string   // LLM model (default: "llama3")
	APIKey        string   // API key
	Endpoint      string   // Custom endpoint
	Tools         []string // Tool names to enable (default: DefaultTools)
	MaxIterations int      // Max agent iterations per turn (default: 10)
	SafeMode      bool     // Block execute and destructive tools
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Provider:      "ollama",
		Model:         "llama3",
		MaxIterations: 10,
		Tools:         DefaultTools,
	}
}

// ChatSession manages an interactive REPL chat session with the aflare agent.
type ChatSession struct {
	config     Config
	ctx        *ContextManager
	reg        *core.Registry
	tools      []core.AgentTool
	systemMsg  string
	running    bool
	safeMode   bool
	interrupt  chan os.Signal
}

// NewChatSession creates a new chat session with the given configuration.
func NewChatSession(cfg Config) *ChatSession {
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
	registerChatNodes(reg) // register template/workflow nodes so agent can use project's own functionality
	tools := buildToolList(cfg.Tools, cfg.SafeMode)
	version := meta.GetVersion()

	cm := NewContextManager()
	systemMsg := BuildSystemPrompt(tools, version)
	cm.SetSystemPrompt(systemMsg)

	return &ChatSession{
		config:    cfg,
		ctx:       cm,
		reg:       reg,
		tools:     tools,
		systemMsg: systemMsg,
		safeMode:  cfg.SafeMode,
		interrupt: make(chan os.Signal, 1),
	}
}

// buildToolList creates the agent tool list from tool names.
// Chat-specific tools (template_list, run_workflow, etc.) are always included.
// When safeMode is true, dangerous tools (execute, file_write, code_interpreter) are excluded.
func buildToolList(toolNames []string, safeMode bool) []core.AgentTool {
	// Always include core chat tools
	chatTools := []core.AgentTool{
		{Name: "template_list", Description: "Search available workflow templates by keyword or category", NodeName: "template_list"},
		{Name: "template_info", Description: "Get detailed info about a specific template", NodeName: "template_info"},
		{Name: "run_workflow", Description: "Run a workflow template. Input: JSON with 'template' and 'params'", NodeName: "run_workflow"},
		{Name: "create_workflow", Description: "Compose and run a new workflow from available nodes", NodeName: "create_workflow"},
		{Name: "memory_store", Description: "Store important information for later recall", NodeName: "memory"},
		{Name: "memory_retrieve", Description: "Recall previously stored information", NodeName: "memory"},
		{Name: "memory_search", Description: "Search memory for relevant context", NodeName: "memory"},
		{Name: "context_compress", Description: "Compress conversation history to free context space", NodeName: "compress"},
	}

	// Dangerous tools blocked in safe mode
	dangerousTools := map[string]bool{
		"execute":          true,
		"file_write":       true,
		"code_interpreter": true,
	}

	// Parse user-requested tools
	userTools := core.ParseToolsList(strings.Join(toolNames, ","))

	// Deduplicate: user tools override chat tools with same name
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

// Run starts the interactive REPL loop.
func (s *ChatSession) Run() {
	s.running = true
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println(WelcomeMessage(meta.GetVersion()))
	fmt.Println()

	// Ctrl-C: interrupt current turn, Ctrl-D: exit
	signal.Notify(s.interrupt, syscall.SIGINT)
	defer signal.Stop(s.interrupt)

	for s.running {
		fmt.Print("> ")
		if !scanner.Scan() {
			// Ctrl-D (EOF)
			fmt.Println()
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "/") {
			s.handleCommand(line)
			continue
		}

		s.handleTurn(line)
	}

	fmt.Println("Goodbye!")
}

// handleTurn processes a single user message and prints the response.
// Ctrl-C during execution interrupts the current turn but does not exit.
func (s *ChatSession) handleTurn(input string) {
	// Drain any stale interrupt signals
	select {
	case <-s.interrupt:
	default:
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Goroutine to watch for Ctrl-C during agent execution
	done := make(chan struct{})
	go func() {
		select {
		case <-s.interrupt:
			fmt.Println("\n(interrupted)")
			cancel()
		case <-done:
		}
	}()

	response, err := s.processInput(ctx, input)
	close(done)

	if err != nil {
		if ctx.Err() != nil {
			fmt.Println("(cancelled)")
		} else {
			fmt.Printf("Error: %v\n", err)
		}
		return
	}
	fmt.Println(response)
	fmt.Println()
}

// processInput handles a single user message and returns the agent's response.
func (s *ChatSession) processInput(ctx context.Context, input string) (string, error) {
	s.ctx.AddUser(input)

	agent := nodes.NewReActAgent(
		s.config.Provider,
		s.config.Model,
		s.config.APIKey,
		s.config.Endpoint,
		s.systemMsg,
		s.config.MaxIterations,
		s.tools,
		s.reg,
		false, // enableThinking
		false, // showThinking
	)

	response, err := agent.Run(ctx, s.ctx.BuildPrefix())
	if err != nil {
		return "", fmt.Errorf("agent execution failed: %w", err)
	}

	s.ctx.AddAssistant(response)
	s.ctx.CompressIfNeeded()

	return response, nil
}

// SendMessage processes a single user message and returns the agent's response.
// Public API for programmatic chat integration (HTTP endpoints).
// The call is bounded by DefaultSendTimeout to prevent hanging indefinitely.
func (s *ChatSession) SendMessage(input string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultSendTimeout)
	defer cancel()
	return s.processInput(ctx, input)
}

// ResetSession clears the conversation history.
func (s *ChatSession) ResetSession() {
	s.ctx.Reset()
}

// handleCommand processes slash commands.
func (s *ChatSession) handleCommand(cmd string) {
	parts := strings.SplitN(cmd, " ", 2)
	switch parts[0] {
	case "/help", "/h":
		fmt.Println("Commands:")
		fmt.Println("  /help, /h      Show this help")
		fmt.Println("  /skills        List skill categories (300+ templates)")
		fmt.Println("  /tools         List available tools")
		fmt.Println("  /history       Show conversation state")
		fmt.Println("  /clear         Clear conversation history")
		fmt.Println("  /exit, /quit   Exit chat")

	case "/skills":
		fmt.Println(ListCategories())

	case "/tools":
		fmt.Println("Available tools:")
		for _, t := range s.tools {
			fmt.Printf("  %-20s %s\n", t.Name, t.Description)
		}

	case "/history":
		fmt.Println(s.ctx.Summary())

	case "/clear":
		s.ctx.Reset()
		fmt.Println("Conversation cleared.")

	case "/exit", "/quit", "/q":
		s.running = false

	default:
		fmt.Printf("Unknown command: %s. Type /help for commands.\n", parts[0])
	}
}