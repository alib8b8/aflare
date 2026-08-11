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
	Capabilities  []string // Capability names to enable (e.g. "reflection", "bdi")
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
// Under the hood it wraps an AgentLoop — the same core that powers daemon mode.
type ChatSession struct {
	loop      *AgentLoop
	running   bool
	interrupt chan os.Signal
}

// NewChatSession creates a new chat session with the given configuration.
func NewChatSession(cfg Config) *ChatSession {
	return &ChatSession{
		loop:      NewAgentLoop(cfg),
		interrupt: make(chan os.Signal, 1),
	}
}

// Run starts the interactive REPL loop.
// Stdin input is fed into the AgentLoop, responses are printed to stdout.
// Supports multi-line input: lines ending with \ are continued on the next line.
func (s *ChatSession) Run() {
	s.running = true
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1*1024*1024)

	fmt.Println(WelcomeMessage(meta.GetVersion()))
	fmt.Println()

	// Ctrl-C: interrupt current turn, Ctrl-D: exit
	signal.Notify(s.interrupt, syscall.SIGINT)
	defer signal.Stop(s.interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var multiLineBuf strings.Builder
	inMultiLine := false
	const maxMultiLineBytes = 1 * 1024 * 1024 // 1MB limit for multi-line input

	for s.running {
		if inMultiLine {
			fmt.Print("... ")
		} else {
			fmt.Print("> ")
		}

		if !scanner.Scan() {
			// Ctrl-D (EOF)
			fmt.Println()
			break
		}

		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		// Multi-line continuation: lines ending with \ or empty lines in continuation mode
		if strings.HasSuffix(line, "\\") {
			inMultiLine = true
			// Append the line without the trailing backslash
			multiLineBuf.WriteString(strings.TrimSuffix(line, "\\"))
			multiLineBuf.WriteString("\n")
			// Force submit if buffer exceeds 1MB
			if multiLineBuf.Len() > maxMultiLineBytes {
				input := strings.TrimSpace(multiLineBuf.String())
				multiLineBuf.Reset()
				inMultiLine = false
				if input != "" {
					s.handleTurn(ctx, input)
				}
			}
			continue
		}

		if inMultiLine {
			if trimmedLine == "" {
				// Empty line in continuation mode: submit the accumulated input
				input := strings.TrimSpace(multiLineBuf.String())
				multiLineBuf.Reset()
				inMultiLine = false
				if input == "" {
					continue
				}
				s.handleTurn(ctx, input)
				continue
			}
			// Non-empty line in continuation mode: append and continue
			multiLineBuf.WriteString(line)
			multiLineBuf.WriteString("\n")
			continue
		}

		// Single-line mode
		if trimmedLine == "" {
			continue
		}

		if strings.HasPrefix(trimmedLine, "/") {
			s.handleCommand(trimmedLine)
			continue
		}

		s.handleTurn(ctx, trimmedLine)
	}

	cancel()
	fmt.Println("Goodbye!")
}

// handleTurn processes a single user message and prints the response.
// Ctrl-C during execution interrupts the current turn but does not exit.
// Streaming output is printed token-by-token in real time.
func (s *ChatSession) handleTurn(parentCtx context.Context, input string) {
	// Drain any stale interrupt signals
	select {
	case <-s.interrupt:
	default:
	}

	ctx, cancel := context.WithCancel(parentCtx)
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

	// Streaming output: print tokens as they arrive
	onChunk := func(chunk string) {
		fmt.Print(chunk)
	}

	// Tool visibility: show tool calls and results
	onToolCall := func(toolName, input string) {
		fmt.Printf("  %s(\"%s\") → ", toolName, truncateStr(input, 50))
	}
	onToolResult := func(toolName, result string) {
		fmt.Printf("%s\n", truncateStr(result, 100))
	}

	_, err := s.loop.ProcessInputStream(ctx, input, onChunk, onToolCall, onToolResult)
	close(done)

	if err != nil {
		if ctx.Err() != nil {
			fmt.Println("\n(cancelled)")
		} else {
			fmt.Printf("\nError: %v\n", err)
		}
	}
	fmt.Println()
}

// processInput handles a single user message and returns the agent's response.
func (s *ChatSession) processInput(ctx context.Context, input string) (string, error) {
	return s.loop.ProcessInput(ctx, input)
}

// SendMessage processes a single user message and returns the agent's response.
// Public API for programmatic chat integration (HTTP endpoints).
// The call is bounded by DefaultSendTimeout to prevent hanging indefinitely.
func (s *ChatSession) SendMessage(input string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultSendTimeout)
	defer cancel()
	return s.loop.ProcessInput(ctx, input)
}

// ResetSession clears the conversation history.
func (s *ChatSession) ResetSession() {
	s.loop.Context().Reset()
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
		fmt.Println("  /capabilities  List active capabilities")
		fmt.Println("  /history       Show conversation state")
		fmt.Println("  /clear         Clear conversation history")
		fmt.Println("  /exit, /quit   Exit chat")

	case "/skills":
		fmt.Println(ListCategories())

	case "/tools":
		fmt.Println("Available tools:")
		for _, t := range s.loop.Tools() {
			fmt.Printf("  %-20s %s\n", t.Name, t.Description)
		}

	case "/capabilities":
		caps := s.loop.Capabilities()
		if caps.Count() == 0 {
			fmt.Println("No capabilities active. Use --capabilities to enable (e.g. -c reflection,bdi,utility).")
		} else {
			fmt.Printf("Active capabilities (%d):\n", caps.Count())
			for _, name := range caps.Names() {
				cap := caps.Get(name)
				if cap != nil {
					fmt.Printf("  %-16s %s\n", name, cap.Description())
				}
			}
		}

	case "/history":
		fmt.Println(s.loop.Context().Summary())

	case "/clear":
		s.loop.Context().Reset()
		fmt.Println("Conversation cleared.")

	case "/exit", "/quit", "/q":
		s.running = false

	default:
		fmt.Printf("Unknown command: %s. Type /help for commands.\n", parts[0])
	}
}