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
func (s *ChatSession) Run() {
	s.running = true
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1*1024*1024)

	fmt.Println(WelcomeMessage(meta.GetVersion()))
	fmt.Println()

	// Ctrl-C: interrupt current turn, Ctrl-D: exit
	signal.Notify(s.interrupt, syscall.SIGINT)
	defer signal.Stop(s.interrupt)

	// Create an input channel and feed stdin into it
	inputs := make(chan AgentInput, 1)
	outputs := make(chan AgentOutput, 1)

	// Background goroutine: handle stdin → feed into AgentLoop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the AgentLoop in background
	go s.loop.Run(ctx, inputs, outputs)

	for s.running {
		// Show output from the agent loop first
		select {
		case out := <-outputs:
			if out.Error != nil {
				fmt.Printf("Error: %v\n", out.Error)
			} else {
				fmt.Println(out.Response)
				fmt.Println()
			}
			continue
		default:
		}

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

		s.handleTurn(ctx, inputs, outputs, line)
	}

	close(inputs)
	cancel()
	fmt.Println("Goodbye!")
}

// handleTurn processes a single user message and prints the response.
// Ctrl-C during execution interrupts the current turn but does not exit.
func (s *ChatSession) handleTurn(parentCtx context.Context, inputs chan<- AgentInput, outputs <-chan AgentOutput, input string) {
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

	replyCh := make(chan AgentOutput, 1)
	select {
	case inputs <- AgentInput{Source: SourceStdin, Message: input, ReplyTo: replyCh}:
	case <-ctx.Done():
		close(done)
		return
	}

	// Wait for response
	select {
	case out := <-replyCh:
		close(done)
		if out.Error != nil {
			if ctx.Err() != nil {
				fmt.Println("(cancelled)")
			} else {
				fmt.Printf("Error: %v\n", out.Error)
			}
		} else {
			fmt.Println(out.Response)
			fmt.Println()
		}
	case <-ctx.Done():
		close(done)
		fmt.Println("(cancelled)")
	}
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