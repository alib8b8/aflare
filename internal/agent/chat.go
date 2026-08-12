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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/alib8b8/aflare/internal/meta"
	"github.com/alib8b8/aflare/internal/metrics"
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
	loop        *AgentLoop
	running     bool
	interrupt   chan os.Signal
	sessionPath string // path to persisted session file
	provider    string // LLM provider (for ollama-specific streaming filter)

	// Analytics
	firstSession bool  // true if this is a new session (no restored session)
	turnCount    int64 // number of user turns completed
	startTime    time.Time
}

// NewChatSession creates a new chat session with the given configuration.
func NewChatSession(cfg Config) *ChatSession {
	loop := NewAgentLoop(cfg)
	// Set up compression notification so the user sees context compression
	loop.SetCompressCallback(func(before, after int) {
		fmt.Printf("(context compressed: %d → %d messages)\n", before, after)
	})
	return &ChatSession{
		loop:        loop,
		interrupt:   make(chan os.Signal, 1),
		sessionPath: DefaultSessionPath(),
		provider:    cfg.Provider,
	}
}

// SetSessionPath overrides the default session persistence path.
func (s *ChatSession) SetSessionPath(path string) {
	s.sessionPath = path
}

// Run starts the interactive REPL loop.
// Stdin input is fed into the AgentLoop, responses are printed to stdout.
// Supports multi-line input: lines ending with \ are continued on the next line.
// On exit, the session is persisted to disk. On next start, the user can /resume.
func (s *ChatSession) Run() {
	s.startTime = time.Now()
	s.running = true
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1*1024*1024)

	fmt.Println(WelcomeMessage(meta.GetVersion()))

	// Check for saved session
	session, hasSession := LoadSession(s.sessionPath)
	if hasSession && len(session.Messages) > 0 {
		fmt.Printf("\nPrevious session found (%d messages, saved %s).\n",
			session.MessageCnt,
			session.SavedAt.Format("2006-01-02 15:04"))
		fmt.Println("Type /resume to restore the conversation, or just start chatting to begin fresh.")
	}
	s.firstSession = !hasSession || len(session.Messages) == 0
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
		s.printPrompt()
		if inMultiLine {
			fmt.Print("... (empty line to submit, \\ to continue) ")
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
			if !inMultiLine {
				fmt.Println("(Multi-line mode: type your lines, press Enter twice to submit)")
			}
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

	// Record session turns before exit
	metrics.RecordSessionTurns(int(atomic.LoadInt64(&s.turnCount)))

	// Persist session on exit
	s.saveSession()
	DeleteSession(s.sessionPath + ".lock")
	fmt.Println("Goodbye!")
}

// printPrompt prints the prompt line with context window indicator.
// Shows usage percentage and a warning when compression is active or near limit.
func (s *ChatSession) printPrompt() {
	chars, limit, compressed := s.loop.Context().ContextUsage()
	pct := chars * 100 / limit
	switch {
	case compressed:
		fmt.Printf("[ctx: compressed] ❯ ")
	case pct >= 80:
		fmt.Printf("[ctx: %d%% ⚠] ❯ ", pct)
	default:
		fmt.Printf("[ctx: %d%%] ❯ ", pct)
	}
}

// saveSession persists the current conversation to disk.
func (s *ChatSession) saveSession() {
	ctx := s.loop.Context()
	if ctx.MessageCount() == 0 {
		return
	}
	if err := ctx.SaveSession(s.sessionPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save session: %v\n", err)
	}
}

// handleTurn processes a single user message and prints the response.
// Ctrl-C during execution interrupts the current turn but does not exit.
// Streaming output is printed token-by-token in real time.
func (s *ChatSession) handleTurn(parentCtx context.Context, input string) {
	// Increment turn count atomically
	atomic.AddInt64(&s.turnCount, 1)

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

	// Streaming output: print tokens as they arrive.
	// For ollama, wrap the chunk callback with a JSON ReAct filter
	// so users see only the thought/final_answer text, not the JSON structure.
	streamed := false
	onChunk := func(chunk string) {
		streamed = true
		fmt.Print(chunk)
	}

	// For ollama provider, onChunk is passed directly — the callOllama
	// function in nodes/agent.go handles ReAct JSON filtering internally.
	onToolCall := func(toolName, input string) {
		fmt.Printf("  %s(\"%s\") → ", toolName, truncateStr(input, 50))
	}
	onToolResult := func(toolName, result string) {
		fmt.Printf("%s\n", truncateStr(result, 100))
	}

	response, err := s.loop.Process(ctx, input, ProcessOptions{
		OnChunk:      onChunk,
		OnToolCall:   onToolCall,
		OnToolResult: onToolResult,
	})
	close(done)

	// Always persist session after each turn, even on error.
	// The user message was already added to context before the LLM call.
	defer s.saveSession()

	// Record first session success metric on the first turn
	if s.firstSession && atomic.LoadInt64(&s.turnCount) == 1 {
		outcome := "success"
		if err != nil {
			if ctx.Err() != nil {
				outcome = "timeout"
			} else {
				outcome = "error"
			}
		} else if time.Since(s.startTime) > 2*time.Minute {
			outcome = "timeout"
		}
		metrics.RecordFirstSession(s.provider, outcome)
	}

	if err != nil {
		if ctx.Err() != nil {
			fmt.Println("\n(cancelled)")
		} else {
			fmt.Printf("\nError: %v\n", err)
		}
		return
	}
	// If nothing was streamed (e.g. ollama suppressed JSON), print the response.
	// If content was streamed, just add a trailing newline.
	if streamed {
		fmt.Println()
	} else if response != "" {
		fmt.Println(response)
	}
}

// processInput handles a single user message and returns the agent's response.
func (s *ChatSession) processInput(ctx context.Context, input string) (string, error) {
	return s.loop.Process(ctx, input, ProcessOptions{})
}

// SendMessage processes a single user message and returns the agent's response.
// Public API for programmatic chat integration (HTTP endpoints).
// The call is bounded by DefaultSendTimeout to prevent hanging indefinitely.
func (s *ChatSession) SendMessage(input string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultSendTimeout)
	defer cancel()
	return s.loop.Process(ctx, input, ProcessOptions{})
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
		fmt.Println("  /resume        Restore previous conversation")
		fmt.Println("  /export        Export conversation to markdown file")
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

	case "/resume":
		s.handleResume()

	case "/export":
		s.handleExport()

	case "/exit", "/quit", "/q":
		s.running = false

	default:
		fmt.Printf("Unknown command: %s. Type /help for commands.\n", parts[0])
	}
}

// handleResume restores the previous conversation from the session file.
func (s *ChatSession) handleResume() {
	session, ok := LoadSession(s.sessionPath)
	if !ok || len(session.Messages) == 0 {
		fmt.Println("No saved session found.")
		return
	}

	restored := s.loop.Context().RestoreSession(s.sessionPath)
	if restored == 0 {
		fmt.Println("No messages to restore.")
		return
	}

	fmt.Printf("Restored %d messages from %s.\n", restored, session.SavedAt.Format("2006-01-02 15:04"))
	fmt.Printf("Context: %d/%d chars\n", s.loop.Context().TotalChars(), MaxContextChars)
}

// handleExport exports the conversation to a markdown file.
func (s *ChatSession) handleExport() {
	messages := s.loop.Context().Messages()
	if len(messages) == 0 {
		fmt.Println("No conversation to export.")
		return
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("export-%s.md", timestamp)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# aflare Chat Export\n\n"))
	sb.WriteString(fmt.Sprintf("Exported: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString("---\n\n")

	for _, m := range messages {
		switch m.Role {
		case "user":
			sb.WriteString(fmt.Sprintf("**You:** %s\n\n", m.Content))
		case "assistant":
			sb.WriteString(fmt.Sprintf("**aflare:** %s\n\n", m.Content))
		case "system":
			// Skip system messages (internal summaries, etc.)
			if !strings.HasPrefix(m.Content, "[Previous conversation summary]") {
				sb.WriteString(fmt.Sprintf("*[System]* %s\n\n", m.Content))
			}
		}
	}

	if err := os.WriteFile(filename, []byte(sb.String()), 0644); err != nil {
		fmt.Printf("Failed to export: %v\n", err)
		return
	}

	fmt.Printf("Conversation exported to %s (%d messages)\n", filename, len(messages))
}