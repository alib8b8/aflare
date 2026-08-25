// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌‌‌​​‌‌​​​​‌​​‌‌​‌​‌​​​‌​​‌‌​‌‌‌​​‌‌​​​‌​‌‌​‌​‌​​​​​​​​​​​​​​​​‌‌‌‌‌‌​​​​​‌​​​‌⁠
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
	"time"

	"github.com/alib8b8/aflare/internal/agent"
	"github.com/alib8b8/aflare/internal/filewatch"
	"github.com/alib8b8/aflare/internal/i18n"
	"github.com/alib8b8/aflare/internal/meta"
	"github.com/alib8b8/aflare/internal/scheduler"
	"github.com/alib8b8/aflare/internal/taskqueue"
)

// HandleAgent handles the "agent" command — unified daemon agent.
// It supports stdin (interactive chat), scheduler events, file-watch
// events, and task queue, all feeding into the same AgentLoop.
func HandleAgent(args []string) error {
	// `aflare agent list` inspects the registry of external agents
	// aflare can command — not a daemon mode, so intercept it before
	// daemon flag parsing.
	if len(args) > 0 && args[0] == "list" {
		return listAgents()
	}

	cfg := agent.DefaultConfig()
	var watchDir string
	if err := parseAgentArgs(args, &cfg, &watchDir); err != nil {
		return err
	}

	loop := agent.NewAgentLoop(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create the shared input channel
	inputs := make(chan agent.AgentInput, 100)

	// Start the AgentLoop in a goroutine
	go loop.Run(ctx, inputs, nil)

	// ── Task Queue ──────────────────────────────────────────────────────
	// All scheduler and filewatch tasks go through the task queue for
	// ordered, non-duplicate execution.
	tq := taskqueue.New(100)
	go tq.Run(ctx, func(taskCtx context.Context, task *taskqueue.Task) taskqueue.TaskResult {
		// Create a reply channel for the agent
		replyCh := make(chan agent.AgentOutput, 1)
		select {
		case inputs <- agent.AgentInput{
			Source:  agent.SourceScheduler,
			Message: task.Message,
			ReplyTo: replyCh,
		}:
		case <-taskCtx.Done():
			return taskqueue.TaskResult{TaskID: task.ID, Error: taskCtx.Err()}
		}

		select {
		case out := <-replyCh:
			return taskqueue.TaskResult{TaskID: task.ID, Response: out.Response, Error: out.Error}
		case <-taskCtx.Done():
			return taskqueue.TaskResult{TaskID: task.ID, Error: taskCtx.Err()}
		}
	})

	// ── Scheduler ───────────────────────────────────────────────────────
	sched := scheduler.New()
	sched.Start()
	defer sched.Stop()

	// Load persisted schedule entries and wire them into the task queue
	schedPath := scheduler.DefaultSchedulesPath()
	entries, err := scheduler.LoadSchedules(schedPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load schedules: %v\n", err)
	}
	for _, entry := range entries {
		entry := entry // capture
		if err := sched.AddTask(entry.ID, entry.Cron, func(taskCtx context.Context) {
			if err := tq.Enqueue(&taskqueue.Task{
				ID:        fmt.Sprintf("sched-%s-%d", entry.ID, time.Now().Unix()),
				Source:    "scheduler",
				Message:   entry.Description,
				CreatedAt: time.Now(),
			}); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to enqueue scheduled task: %v\n", err)
			}
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to add scheduled task %s: %v\n", entry.ID, err)
		}
		if entry.Description != "" {
			fmt.Printf("Loaded scheduled task: %s (%s) → %s\n", entry.ID, entry.Cron, entry.Description)
		} else {
			fmt.Printf("Loaded scheduled task: %s (%s)\n", entry.ID, entry.Cron)
		}
	}

	// ── File Watch ──────────────────────────────────────────────────────
	if watchDir != "" {
		watchEvents := make(chan filewatch.Event, 100)
		watcher, err := filewatch.NewWatcher(watchDir, filewatch.DefaultPollInterval, watchEvents)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: filewatch setup failed: %v\n", err)
		} else {
			go watcher.Start(ctx)
			// Feed filewatch events into the task queue as agent inputs
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case event := <-watchEvents:
						if err := tq.Enqueue(&taskqueue.Task{
							ID:        fmt.Sprintf("fw-%s-%d", event.Path, event.Timestamp.Unix()),
							Source:    "filewatch",
							Message:   filewatch.FormatEvent(event),
							CreatedAt: event.Timestamp,
						}); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to enqueue filewatch task: %v\n", err)
						}
					}
				}
			}()
			fmt.Printf("Watching directory: %s\n", watchDir)
		}
	}

	// ── Signal Handling ─────────────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println(agent.WelcomeMessage(meta.GetVersion()))
	fmt.Println("Agent running in daemon mode (scheduler + filewatch + stdin).")
	if watchDir != "" {
		fmt.Printf("  Watch:   %s\n", watchDir)
	}
	fmt.Printf("  Queue:   %d tasks pending\n", tq.Size())
	fmt.Println("Type messages to interact, /help for commands, Ctrl-C to interrupt, Ctrl-D to exit.")
	fmt.Println()

	// ── Stdin Scanner ───────────────────────────────────────────────────
	// stdin reading runs in its own goroutine: the previous inline
	// scanner.Scan() blocked the loop between lines, so a signal arriving
	// while stdin was idle was never observed — the daemon could not be
	// stopped with SIGINT/SIGTERM at all (only SIGKILL worked), hanging
	// production supervisors past their stop timeout.
	stdinDone := make(chan struct{})
	lines := make(chan string, 16)
	go func() {
		defer close(stdinDone)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 0, 64*1024), 1*1024*1024)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
	}()

	running := true
	for running {
		fmt.Print("> ")

		var line string
		var ok bool
		select {
		case sig := <-sigCh:
			fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
			running = false
			continue

		case <-ctx.Done():
			running = false
			continue

		case line, ok = <-lines:
			if !ok {
				// Ctrl-D (EOF)
				fmt.Println()
				running = false
				continue
			}
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "/") {
			handleAgentCommand(line, loop, &running, tq)
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

	close(inputs)
	cancel()
	// Give the stdin goroutine a moment to observe ctx cancellation so a
	// blocked send cannot outlive shutdown; it is abandoned either way at
	// process exit.
	select {
	case <-stdinDone:
	case <-time.After(2 * time.Second):
	}
	fmt.Println("Goodbye!")
	return nil
}

// handleAgentCommand processes slash commands in agent mode.
func handleAgentCommand(cmd string, loop *agent.AgentLoop, running *bool, tq *taskqueue.Queue) {
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
		fmt.Println("  /queue         Show task queue status")
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

	case "/queue":
		fmt.Printf("Task queue: %d pending, %d active\n", tq.Size(), tq.ActiveCount())
		summary := tq.StatusSummary()
		fmt.Printf("  Status: %d done, %d failed\n", summary[taskqueue.StatusDone], summary[taskqueue.StatusFailed])

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
	fmt.Println("scheduled tasks, file-watch events, and a task queue in a single event loop.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --smart                   智能模式预设（reflection + memory）")
	fmt.Println("  --careful                 谨慎模式预设（human-in-loop + planning + reflection）")
	fmt.Println("  --custom -c <list>        自定义 capability 组合（高级用户）")
	fmt.Println("  --provider, -p <name>     LLM provider (default: ollama)")
	fmt.Println("  --model, -m <name>        Model name (default: llama3)")
	fmt.Println("  --api-key, -k <key>       API key for cloud providers")
	fmt.Println("  --endpoint, -e <url>      Custom API endpoint")
	fmt.Println("  --tools, -t <list>        Comma-separated tool names, or 'all'")
	fmt.Println("  --capabilities, -c <list> Comma-separated capability names, or 'all'")
	fmt.Println("  --max-iterations, -n <n>  Max agent iterations per turn (default: 10)")
	fmt.Println("  --context-budget <n>      Context budget in tokens (default: per provider, e.g. 8000 for ollama)")
	fmt.Println("  --safe-mode, -s            Block execute and destructive tools")
	fmt.Println("  --watch <dir>             Watch directory for file changes, feed to agent")
	fmt.Println("  --help, -h                 Show this help")
	fmt.Println()
	fmt.Println("Capabilities (--custom -c):")
	fmt.Println("  reflection     Self-reflection and self-correction")
	fmt.Println("  human-in-loop  Pause at critical decisions for human approval")
	fmt.Println("  utility        Utility-driven optimization of decisions")
	fmt.Println("  memory         Cross-session long-term memory")
	fmt.Println("  planning       Goal-driven planning and action sequencing")
	fmt.Println("  workflow       Predefined workflow/pipeline execution")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  aflare agent                                    # 默认模式（无 capability）")
	fmt.Println("  aflare agent --smart                           # 智能模式（推荐）")
	fmt.Println("  aflare agent --careful                         # 谨慎模式（有风险操作时）")
	fmt.Println("  aflare agent --custom -c reflection,bdi        # 自定义组合")
	fmt.Println("  aflare agent -p deepseek -m deepseek-chat       # DeepSeek")
	fmt.Println("  aflare agent -s                                 # safe mode")
	fmt.Println("  aflare agent --watch ./logs                     # watch directory for changes")
	fmt.Println("  aflare agent -c all                             # all capabilities")
}

// parseAgentArgs parses CLI arguments for the agent command.
func parseAgentArgs(args []string, cfg *agent.Config, watchDir *string) error {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--smart":
			// 断点14: 智能模式预设。
			cfg.Capabilities = agent.ResolvePreset("smart")
		case "--careful":
			// 断点14: 谨慎模式预设。
			cfg.Capabilities = agent.ResolvePreset("careful")
		case "--custom":
			// 断点14: 标记使用自定义组合，-c 紧随其后。
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
				// 断点14: --custom -c xxx 时覆盖预设；否则 -c 单独使用也生效。
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
		case "--context-budget":
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil && n > 0 {
					cfg.ContextBudget = n
				}
				i++
			}
		case "--safe-mode", "-s":
			cfg.SafeMode = true
		case "--watch":
			if i+1 < len(args) {
				*watchDir = args[i+1]
				i++
			}
		case "--no-stdin":
			// daemon-only mode: no interactive input
		case "--help", "-h":
			PrintAgentUsage()
			return nil
		default:
			fmt.Printf("Unknown argument: %s\n", args[i])
			PrintAgentUsage()
			return exitErr(1)
		}
	}
	return nil
}
