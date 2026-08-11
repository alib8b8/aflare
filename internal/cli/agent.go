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

// agent.go implements the "aflare agent" command — a unified agent daemon
// that fuses interactive chat, scheduled tasks, and file-watch events into
// a single event loop. All input sources feed into the same AgentLoop core.

package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/alib8b8/aflare/internal/agent"
	"github.com/alib8b8/aflare/internal/i18n"
	"github.com/alib8b8/aflare/internal/meta"
	"github.com/alib8b8/aflare/internal/scheduler"
)

// HandleAgent handles the "agent" command — unified daemon agent.
// It supports stdin (interactive chat), scheduler events, and file-watch
// events, all feeding into the same AgentLoop.
func HandleAgent(args []string) {
	cfg := agent.DefaultConfig()

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider", "-p":
			if i+1 < len(args) {
				cfg.Provider = args[i+1]
				i++
			}
		case "--model", "-m":
			if i+1 < len(args) {
				cfg.Model = args[i+1]
				i++
			}
		case "--api-key", "-k":
			if i+1 < len(args) {
				cfg.APIKey = args[i+1]
				i++
			}
		case "--endpoint", "-e":
			if i+1 < len(args) {
				cfg.Endpoint = args[i+1]
				i++
			}
		case "--tools", "-t":
			if i+1 < len(args) {
				cfg.Tools = parseToolsArg(args[i+1])
				i++
			}
		case "--capabilities", "-c":
			if i+1 < len(args) {
				cfg.Capabilities = agent.ParseCapabilities(args[i+1])
				i++
			}
		case "--max-iterations", "-n":
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil && n > 0 {
					cfg.MaxIterations = n
				}
				i++
			}
		case "--safe-mode", "-s":
			cfg.SafeMode = true
		case "--no-stdin":
			// daemon-only mode: no interactive input
		case "--help", "-h":
			PrintAgentUsage()
			return
		default:
			fmt.Printf("Unknown argument: %s\n", args[i])
			PrintAgentUsage()
			os.Exit(1)
		}
	}

	loop := agent.NewAgentLoop(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create the shared input channel
	inputs := make(chan agent.AgentInput, 10)

	// Start the AgentLoop
	go loop.Run(ctx, inputs, nil)

	// Start the scheduler
	sched := scheduler.New()
	sched.Start()
	defer sched.Stop()

	// Wire scheduler events into the agent loop
	// The scheduler fires events as user messages to the agent
	sched.AddTask("agent-heartbeat", "*/5 * * * *", func(taskCtx context.Context) {
		select {
		case inputs <- agent.AgentInput{
			Source:  agent.SourceScheduler,
			Message: "[system] Scheduled heartbeat. Review recent context and check if any pending tasks need attention.",
		}:
		case <-ctx.Done():
		}
	})

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println(agent.WelcomeMessage(meta.GetVersion()))
	fmt.Println("Agent running in daemon mode (scheduler + stdin).")
	fmt.Println("Type messages to interact, /help for commands, Ctrl-C to interrupt, Ctrl-D to exit.")
	fmt.Println()

	// Stdin scanner
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1*1024*1024)

	running := true
	for running {
		select {
		case sig := <-sigCh:
			fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
			running = false

		default:
			fmt.Print("> ")
			if !scanner.Scan() {
				// Ctrl-D (EOF)
				fmt.Println()
				running = false
				break
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			if strings.HasPrefix(line, "/") {
				handleAgentCommand(line, loop, &running)
				continue
			}

			replyCh := make(chan agent.AgentOutput, 1)
			select {
			case inputs <- agent.AgentInput{
				Source:  agent.SourceStdin,
				Message: line,
				ReplyTo: replyCh,
			}:
				// Wait for response
				out := <-replyCh
				if out.Error != nil {
					fmt.Printf("Error: %v\n", out.Error)
				} else {
					fmt.Println(out.Response)
					fmt.Println()
				}
			case <-ctx.Done():
				running = false
			}
		}
	}

	close(inputs)
	cancel()
	fmt.Println("Goodbye!")
}

// handleAgentCommand processes slash commands in agent mode.
func handleAgentCommand(cmd string, loop *agent.AgentLoop, running *bool) {
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
		fmt.Println("  /exit, /quit   Exit agent")

	case "/skills":
		fmt.Println(agent.ListCategories())

	case "/tools":
		fmt.Println("Available tools:")
		for _, t := range loop.Tools() {
			fmt.Printf("  %-20s %s\n", t.Name, t.Description)
		}

	case "/capabilities":
		caps := loop.Capabilities()
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
		fmt.Println(loop.Context().Summary())

	case "/clear":
		loop.Context().Reset()
		fmt.Println("Conversation cleared.")

	case "/exit", "/quit", "/q":
		*running = false

	default:
		fmt.Printf("Unknown command: %s. Type /help for commands.\n", parts[0])
	}
}

// PrintAgentUsage prints help for the agent command.
func PrintAgentUsage() {
	fmt.Println(i18n.T("usage.agent"))
	fmt.Println()
	fmt.Println("Usage: aflare agent [options]")
	fmt.Println()
	fmt.Println("Starts a unified agent daemon that fuses interactive chat with")
	fmt.Println("scheduled tasks and file-watch events in a single event loop.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --provider, -p <name>     LLM provider (default: ollama)")
	fmt.Println("  --model, -m <name>        Model name (default: llama3)")
	fmt.Println("  --api-key, -k <key>       API key for cloud providers")
	fmt.Println("  --endpoint, -e <url>      Custom API endpoint")
	fmt.Println("  --tools, -t <list>        Comma-separated tool names, or 'all'")
	fmt.Println("  --capabilities, -c <list> Comma-separated capability names, or 'all'")
	fmt.Println("  --max-iterations, -n <n>  Max agent iterations per turn (default: 10)")
	fmt.Println("  --safe-mode, -s            Block execute and destructive tools")
	fmt.Println("  --help, -h                 Show this help")
	fmt.Println()
	fmt.Println("Capabilities (--capabilities):")
	fmt.Println("  reflection     Self-reflection and self-correction (反思/自我批评)")
	fmt.Println("  human-in-loop  Pause at critical decisions for human approval (人机协同)")
	fmt.Println("  bdi            Belief-Desire-Intention goal management (BDI)")
	fmt.Println("  utility        Utility-driven optimization of decisions (效用驱动)")
	fmt.Println("  adaptive       Learning and adaptation from feedback (学习型/自适应)")
	fmt.Println("  memory         Cross-session long-term memory (有状态)")
	fmt.Println("  planning       Goal-driven planning and action sequencing (规划式)")
	fmt.Println("  multi-agent    Multi-agent collaboration (多Agent协作式)")
	fmt.Println("  workflow       Predefined workflow/pipeline execution (工作流/管道式)")
	fmt.Println("  simulation     Simulation and generative behavior modeling (模拟/生成式)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  aflare agent                                    # local ollama (default)")
	fmt.Println("  aflare agent -p deepseek -m deepseek-chat       # DeepSeek")
	fmt.Println("  aflare agent -s                                 # safe mode")
	fmt.Println("  aflare agent -c reflection,bdi                  # with reflection + BDI")
	fmt.Println("  aflare agent -c all                             # all capabilities")
	fmt.Println()
	fmt.Println("Agent commands:")
	fmt.Println("  /help       Show commands")
	fmt.Println("  /tools      List available tools")
	fmt.Println("  /capabilities  List active capabilities")
	fmt.Println("  /history    Show conversation state")
	fmt.Println("  /clear      Clear conversation history")
	fmt.Println("  /exit       Exit agent")
}