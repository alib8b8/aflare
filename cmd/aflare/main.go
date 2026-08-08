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

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/alib8b8/aflare/internal/ai"
	"github.com/alib8b8/aflare/internal/api"
	"github.com/alib8b8/aflare/internal/autoupgrade"
	"github.com/alib8b8/aflare/internal/cli"
	"github.com/alib8b8/aflare/internal/history"
	"github.com/alib8b8/aflare/internal/i18n"
	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/mcp"
	"github.com/alib8b8/aflare/internal/meta"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/output"
	"github.com/alib8b8/aflare/internal/scheduler"
	"github.com/alib8b8/aflare/internal/skills"
	"github.com/alib8b8/aflare/internal/tui"
	"github.com/alib8b8/aflare/internal/webui"
	"github.com/alib8b8/aflare/internal/workflow"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

var sensitiveKeyPrefixes = []string{"api", "token", "bearer", "password", "passwd", "secret", "auth"}

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, prefix := range sensitiveKeyPrefixes {
		if strings.HasPrefix(lower, prefix) || strings.Contains(lower, "_"+prefix) || strings.Contains(lower, "-"+prefix) {
			return true
		}
	}
	return false
}

func redactParams(params map[string]string) map[string]string {
	redacted := make(map[string]string)
	for k, v := range params {
		if isSensitiveKey(k) {
			redacted[k] = "***"
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

func main() {
	if len(os.Args) < 2 {
		i18n.Init("")
		fmt.Println(cli.PrintUsage())
		os.Exit(1)
	}

	command, args, safeMode, dryRun, mcpServer, lang, concise, initMCP, initAgent, updateChannel, serveMode := cli.ParseArgs(os.Args[1:])

	// Set output mode based on --concise flag
	if concise {
		output.SetMode(output.ModeConcise)
	}

	// Initialize i18n with detected or specified language
	i18n.Init(lang)

	// MCP server mode: start JSON-RPC server over stdio
	if mcpServer {
		server := mcp.NewServer()
		if err := server.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// API server mode: start HTTP API server (--serve flag)
	if serveMode {
		handleServe(args)
		return
	}

	if initMCP != "" {
		result, err := cli.SetupMCP(initMCP)
		if err != nil {
			fmt.Printf("❌ MCP setup failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ MCP configured for %s\n", result.Agent)
		fmt.Printf("   Config: %s\n", result.ConfigPath)
		fmt.Printf("   Command: %s\n", result.Command)
		return
	}

	if initAgent != "" {
		result, err := cli.InstallSkills(initAgent)
		if err != nil {
			fmt.Printf("❌ Skill installation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Skills installed for %s\n", result.Agent)
		fmt.Printf("   Path: %s\n", result.SkillPath)
		if result.Installed {
			fmt.Println("   Status: skills copied successfully")
		} else {
			fmt.Println("   Status: skills directory ready (no source templates found)")
		}
		return
	}

	if updateChannel != "" {
		config, err := autoupgrade.LoadConfig()
		if err != nil {
			fmt.Printf("❌ Failed to load config: %v\n", err)
			os.Exit(1)
		}
		if err := autoupgrade.SetChannel(config, updateChannel); err != nil {
			fmt.Printf("❌ Failed to set channel: %v\n", err)
			os.Exit(1)
		}
		if err := autoupgrade.SaveConfig(config); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Update channel set to: %s\n", updateChannel)
		return
	}

	if safeMode {
		nodes.SetSafeMode(true)
		fmt.Println(i18n.T("safe_mode.enabled"))
	}

	if dryRun {
		fmt.Println(i18n.T("dry_run.enabled"))
	}

	if command == "" {
		fmt.Println(cli.PrintUsage())
		os.Exit(1)
	}

	if err := cli.ValidateCommand(command); err != nil {
		fmt.Println(cli.PrintUsage())
		os.Exit(1)
	}

	switch command {
	case "create":
		handleCreate(args)
		return
	case "run":
		handleRun(args, dryRun)
		return
	case "install":
		handleInstall(args)
		return
	case "uninstall":
		handleUninstall(args)
		return
	case "registry":
		handleRegistry(args)
		return
	case "list":
		handleList()
		return
	case "validate":
		handleValidate(args)
		return
	case "review":
		handleReview(args)
		return
	case "version", "--version", "-v":
		fmt.Println(cli.PrintVersion())
		return
	case "self-update", "update":
		handleSelfUpdate()
		return
	case "autoupgrade", "au":
		handleAutoUpgrade(args)
		return
	case "init":
		handleInit(args)
		return
	case "skills":
		handleSkills(args)
		return
	case "-h", "--help", "help":
		fmt.Println(cli.PrintUsage())
		return
	case "webui":
		handleWebUI(args)
		return
	case "schedule":
		handleSchedule(args)
		return
	case "audit":
		handleAudit(args)
		return
	case "serve":
		handleServe(args)
		return
	default:
		// Known subcommands are handled above; anything else is treated as a
		// workflow file path. "schedule" is handled by its own case above and
		// must never reach handleRunFile.
		handleRunFile(command, dryRun, false, "")
	}
}

func handleList() {
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	nodeList := reg.ListNodes()
	if len(nodeList) == 0 {
		fmt.Println(i18n.T("list.empty"))
		return
	}

	fmt.Println(i18n.T("list.title"))
	fmt.Println("-" + strings.Repeat("-", 78))
	fmt.Printf("  %-20s %s\n", i18n.T("table.name"), i18n.T("table.description"))
	fmt.Println("-" + strings.Repeat("-", 78))
	for _, info := range nodeList {
		fmt.Printf("  %-20s %s\n", info.Name, info.Description)
	}

	installed, err := cli.ListInstalledNodes()
	if err == nil && len(installed) > 0 {
		fmt.Printf("\n%s\n", i18n.T("list.installed"))
		for _, name := range installed {
			fmt.Printf("  - %s\n", name)
		}
	}
}

func handleValidate(args []string) {
	if len(args) < 1 {
		fmt.Println(i18n.T("validate.usage"))
		os.Exit(1)
	}

	wf, reg, err := cli.PrepareWorkflow(args[0])
	if err != nil {
		fmt.Printf("❌ %s\n", i18n.T("validate.error", err))
		os.Exit(1)
	}

	warnings := workflow.ValidateWorkflow(wf)

	for i, step := range wf.Steps {
		// Compound steps (if/loop/map/reduce/parallel/saga/capture_error) have no
		// node of their own; their sub-steps are resolved when executed, so
		// skip the node-existence check for them.
		if step.IsIf() || step.IsLoop() || step.IsMap() || step.IsReduce() || step.IsParallel() || step.IsSaga() || step.HasCaptureError() {
			continue
		}
		if _, ok := reg.Get(step.Node); !ok {
			warnings = append(warnings, fmt.Sprintf("Step %d: unknown node '%s'", i+1, step.Node))
		}
	}

	if len(warnings) == 0 {
		fmt.Printf("✅ %s\n", i18n.T("validate.valid"))
	} else {
		fmt.Printf("⚠️ %s\n", i18n.T("validate.warnings"))
		for _, warning := range warnings {
			fmt.Printf("  - %s\n", warning)
		}
		os.Exit(1)
	}
}

func handleReview(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: aflare review <workflow.yaml>")
		fmt.Println("\nAI-powered workflow review — analyzes your workflow YAML and provides")
		fmt.Println("optimization suggestions, security checks, and a natural-language summary.")
		fmt.Println("\nInspired by agno v2.8.7 AdvisorTools: the advisor model reviews the")
		fmt.Println("generated workflow before execution, catching issues early.")
		os.Exit(1)
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Printf("❌ Failed to read workflow file: %v\n", err)
		os.Exit(1)
	}

	workflowYAML := string(data)

	// Run the AI optimizer (rule-based analysis)
	optimizer := ai.NewWorkflowOptimizer()
	report := optimizer.Analyze(workflowYAML)

	// Generate natural-language explanation
	explainer := ai.NewWorkflowExplainer()
	explanation := explainer.Explain(workflowYAML)

	// Print header
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║           aflare Workflow Advisor Review                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Print the natural-language summary
	fmt.Println("📋 What this workflow does:")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println(explanation)
	fmt.Println()

	// Print optimization score
	scoreColor := "🟢"
	if report.Score < 70 {
		scoreColor = "🟡"
	}
	if report.Score < 40 {
		scoreColor = "🔴"
	}
	fmt.Printf("%s Optimization Score: %d/100\n", scoreColor, report.Score)
	fmt.Println(strings.Repeat("─", 60))

	if len(report.Suggestions) == 0 {
		fmt.Println("✅ No issues found — workflow looks great!")
	} else {
		// Group by severity
		var errors, warnings, infos []ai.Suggestion
		for _, s := range report.Suggestions {
			switch s.Severity {
			case ai.SeverityError:
				errors = append(errors, s)
			case ai.SeverityWarning:
				warnings = append(warnings, s)
			case ai.SeverityInfo:
				infos = append(infos, s)
			}
		}

		if len(errors) > 0 {
			fmt.Printf("\n🔴 Errors (%d):\n", len(errors))
			for _, s := range errors {
				fmt.Printf("  ❌ %s\n", s.Message)
				if s.Fix != "" {
					fmt.Printf("     Fix: %s\n", s.Fix)
				}
			}
		}
		if len(warnings) > 0 {
			fmt.Printf("\n🟡 Warnings (%d):\n", len(warnings))
			for _, s := range warnings {
				fmt.Printf("  ⚠️  %s\n", s.Message)
				if s.Fix != "" {
					fmt.Printf("     Fix: %s\n", s.Fix)
				}
			}
		}
		if len(infos) > 0 {
			fmt.Printf("\n🔵 Suggestions (%d):\n", len(infos))
			for _, s := range infos {
				fmt.Printf("  💡 %s\n", s.Message)
				if s.Fix != "" {
					fmt.Printf("     Tip: %s\n", s.Fix)
				}
			}
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("💡 Tip: Run 'aflare review' before 'aflare run' to catch issues early.")
	fmt.Println("   The advisor model checks for security, performance, and reliability.")
}

func handleInstall(args []string) {
	if len(args) < 1 {
		fmt.Println(i18n.T("install.usage"))
		fmt.Printf("\n%s\n", i18n.T("install.use_list"))
		os.Exit(1)
	}

	nodeName := args[0]
	if err := cli.InstallNode(nodeName); err != nil {
		fmt.Printf("❌ %s\n", i18n.T("install.failed", nodeName, err))
		os.Exit(1)
	}

	fmt.Printf("✅ %s\n", i18n.T("install.success", nodeName))
	fmt.Printf("\n%s\n", i18n.T("install.usage_hint"))
	fmt.Printf("  steps:\n    - node: %s\n", nodeName)
}

func handleUninstall(args []string) {
	if len(args) < 1 {
		fmt.Println(i18n.T("uninstall.usage"))
		os.Exit(1)
	}

	nodeName := args[0]
	if err := cli.UninstallNode(nodeName); err != nil {
		fmt.Printf("❌ %s\n", i18n.T("uninstall.failed", nodeName, err))
		os.Exit(1)
	}

	fmt.Printf("✅ %s\n", i18n.T("uninstall.success", nodeName))
}

func handleRegistry(args []string) {
	if len(args) < 1 {
		fmt.Println(i18n.T("registry.usage"))
		fmt.Println("\nCommands:")
		fmt.Println("  sync     - aflare registry sync")
		fmt.Println("  list     - aflare registry list")
		fmt.Println("  search   - aflare registry search <query>")
		os.Exit(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "sync":
		if err := cli.SyncRegistry(); err != nil {
			fmt.Printf("❌ %s\n", i18n.T("registry.sync_failed", err))
			os.Exit(1)
		}
		fmt.Printf("✅ %s\n", i18n.T("registry.sync_success"))

	case "list":
		nodes, err := cli.ListRegistryNodes()
		if err != nil {
			fmt.Printf("❌ %s\n", i18n.T("registry.list_failed", err))
			fmt.Printf("\n%s\n", i18n.T("registry.sync_hint"))
			os.Exit(1)
		}

		if len(nodes) == 0 {
			fmt.Println(i18n.T("registry.empty"))
			return
		}

		fmt.Println(i18n.T("registry.list_title"))
		fmt.Println("-" + strings.Repeat("-", 78))
		fmt.Printf("  %-15s %-40s %-10s %s\n", i18n.T("table.name"), i18n.T("table.description"), i18n.T("table.version"), i18n.T("table.category"))
		fmt.Println("-" + strings.Repeat("-", 78))
		for _, node := range nodes {
			fmt.Printf("  %-15s %-40s %-10s %s\n", node.Name, truncate(node.Description, 38), node.Version, node.Category)
		}

	case "search":
		if len(args) < 2 {
			fmt.Println(i18n.T("registry.search_usage"))
			os.Exit(1)
		}

		query := strings.Join(args[1:], " ")
		nodes, err := cli.SearchRegistryNodes(query)
		if err != nil {
			fmt.Printf("❌ %s\n", i18n.T("registry.list_failed", err))
			os.Exit(1)
		}

		if len(nodes) == 0 {
			fmt.Printf("%s\n", i18n.T("registry.no_match", query))
			return
		}

		fmt.Println(i18n.T("registry.search_result", len(nodes), query))
		fmt.Println("-" + strings.Repeat("-", 78))
		fmt.Printf("  %-15s %-40s %-10s %s\n", i18n.T("table.name"), i18n.T("table.description"), i18n.T("table.version"), i18n.T("table.category"))
		fmt.Println("-" + strings.Repeat("-", 78))
		for _, node := range nodes {
			fmt.Printf("  %-15s %-40s %-10s %s\n", node.Name, truncate(node.Description, 38), node.Version, node.Category)
		}

	default:
		fmt.Printf("%s\n", i18n.T("registry.unknown_cmd", subCmd))
		os.Exit(1)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 2 {
		return s[:maxLen]
	}
	return s[:maxLen-2] + ".."
}

func handleCreate(args []string) {
	if len(args) < 1 {
		fmt.Println(i18n.T("create.usage"))
		fmt.Println("\nExamples:")
		fmt.Println("  aflare create \"fetch example.com and save to file\"")
		fmt.Println("  aflare create \"fetch Hacker News and save to hn.txt\"")
		fmt.Println("  aflare create \"summarize article and write to summary.md\"")
		os.Exit(1)
	}

	description := cli.SummarizeCommand("", args)
	fmt.Printf("%s\n", i18n.T("create.start", description))

	filename, err := workflow.CreateWorkflowFromDescription(description)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ %s\n", i18n.T("create.success", filename))
	fmt.Printf("\n%s\n", i18n.T("create.run_hint"))
	fmt.Printf("  aflare run %s\n", filename)
}

func handleRun(args []string, dryRun bool) {
	// Parse --resume flag. Two forms are supported:
	//   aflare run --resume my-workflow.yaml
	//     → boolean flag; checkpoint defaults to ~/.aflare/checkpoints/<name>.json
	//   aflare run --resume /path/to/state.json my-workflow.yaml
	//   aflare run --resume=/path/to/state.json my-workflow.yaml
	//     → explicit checkpoint path
	resumeEnabled := false
	resumePath := ""
	var filtered []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--resume" {
			resumeEnabled = true
			// If at least two args remain after --resume, the first is the
			// checkpoint path and the second is the workflow file.
			remaining := len(args) - i - 1
			if remaining >= 2 {
				resumePath = args[i+1]
				i++
			}
		} else if strings.HasPrefix(arg, "--resume=") {
			resumeEnabled = true
			resumePath = strings.TrimPrefix(arg, "--resume=")
		} else {
			filtered = append(filtered, arg)
		}
	}
	if len(filtered) < 1 {
		fmt.Println(i18n.T("run.usage"))
		os.Exit(1)
	}
	handleRunFile(filtered[0], dryRun, resumeEnabled, resumePath)
}

func handleRunFile(wfPath string, dryRun bool, resumeEnabled bool, resumePath string) {
	wf, reg, err := cli.PrepareWorkflow(wfPath)
	if err != nil {
		fmt.Printf("Error preparing workflow: %v\n", err)
		os.Exit(1)
	}

	if suggestions := workflow.ValidateWorkflow(wf); len(suggestions) > 0 {
		fmt.Println("⚠️ Workflow validation warnings:")
		for _, suggestion := range suggestions {
			fmt.Printf("  - %s\n", suggestion)
		}
	}

	if dryRun {
		fmt.Println("\n✅ Dry run completed - workflow is valid")
		return
	}

	// Compute the checkpoint state path.
	statePath := ""
	if resumeEnabled {
		if resumePath != "" {
			statePath = resumePath
		} else {
			// Default: ~/.aflare/checkpoints/<workflow-name>.json
			name := wf.Name
			if name == "" {
				name = strings.TrimSuffix(filepath.Base(wfPath), filepath.Ext(wfPath))
			}
			statePath = filepath.Join(meta.DataDir(), "checkpoints", name+".json")
		}
	}

	if isatty.IsTerminal(os.Stdout.Fd()) {
		runTUI(wfPath, wf, reg, statePath)
	} else {
		runCLI(wf, reg, statePath)
	}
}

// resolveAuditDir returns the directory the audit log will land in for a given
// configured dir. When dir is non-empty it is used as-is; otherwise the history
// package's default audit directory is derived from GetAuditLogPath. Returns
// "" when no audit directory is available (e.g. HOME unset), in which case
// audit logging no-ops inside history.AppendAuditLog and no lock is needed.
func resolveAuditDir(dir string) string {
	if dir != "" {
		return dir
	}
	p := history.GetAuditLogPath()
	if p == "" {
		return ""
	}
	return filepath.Dir(p)
}

// acquireAuditLock takes an exclusive lock on the audit directory to prevent
// two aflare processes from concurrently appending to the same HMAC
// hash-chained audit log (H-6). The history package's auditWriteMu only
// serializes appends within a single process; without this lock, two
// `aflare run` invocations sharing one audit directory would interleave
// appends and fork the hash chain, making VerifyAuditChain fail and breaking
// tamper-evidence — the core guarantee for the financial audit scenario.
//
// The lock is a .audit.lock file created atomically with O_CREATE|O_EXCL. On
// success a release function is returned that closes and removes the lock;
// the caller MUST defer it. A stale lock left by a crashed process blocks
// subsequent runs — in that case the caller disables audit for the new
// process (with a warning) rather than corrupting the chain; the operator
// removes the stale lock manually. Pass dir="" to skip locking entirely
// (audit no-ops anyway when no directory is configured).
func acquireAuditLock(dir string) (func(), error) {
	if dir == "" {
		return func() {}, nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("audit: create dir %s: %w", dir, err)
	}
	lockPath := filepath.Join(dir, ".audit.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("another aflare process is writing audit logs to %s; only one process may run with audit enabled at a time (set AFLARE_AUDIT_DIR to isolate, or remove a stale %s)", dir, lockPath)
		}
		return nil, err
	}
	fmt.Fprintf(f, "pid=%d started=%s\n", os.Getpid(), time.Now().Format(time.RFC3339))
	return func() {
		_ = f.Close()
		_ = os.Remove(lockPath)
	}, nil
}

// newAuditEnabledExecutor builds an Executor with tamper-evident audit
// logging, first acquiring a process-exclusive lock on the audit directory
// (H-6). If the lock cannot be acquired (another process holds it, or a stale
// lock remains), audit is disabled for this process — with a warning — so the
// hash chain is never forked by concurrent writers. The returned release
// function must be deferred by the caller to release the lock on exit. When
// no audit directory is available, audit no-ops and the release is a no-op.
//
// auditDir is passed through to WithAuditLog unchanged ("" means "use the
// history default"); the lock is taken on the resolved directory so the
// default directory is also protected.
func newAuditEnabledExecutor(auditDir string) (*workflow.Executor, func()) {
	resolved := resolveAuditDir(auditDir)
	if resolved == "" {
		return workflow.NewExecutor().WithAuditLog(true, ""), func() {}
	}
	release, err := acquireAuditLock(resolved)
	if err != nil {
		logger.Warn("audit lock failed; disabling audit for this process to avoid cross-process hash-chain corruption",
			"dir", resolved, "error", err)
		return workflow.NewExecutor().WithAuditLog(false, ""), func() {}
	}
	return workflow.NewExecutor().WithAuditLog(true, auditDir), release
}

func runTUI(wfPath string, wf *workflow.Workflow, reg *nodes.Registry, statePath string) {
	model := tui.NewModel(wf.Name, wfPath, len(wf.Steps))
	program := tea.NewProgram(model, tea.WithAltScreen())

	// Derive a cancellable context so that when the TUI exits (program.Run
	// returns), the in-flight workflow goroutine receives a cancellation
	// signal instead of running on under context.Background() after the
	// user has already left. The Executor derives its step timeout from
	// this ctx, so nodes that honor ctx will abort promptly.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		// Route through the Executor so tamper-evident audit logging is
		// applied to every TUI workflow run. Audit degrades gracefully (and
		// silently after one warning) when the HMAC key env vars are unset.
		// newAuditEnabledExecutor acquires a per-process lock on the audit
		// directory (H-6) so concurrent aflare processes cannot fork the
		// HMAC hash chain; if the lock is held, audit is disabled for this
		// process rather than corrupting the chain.
		exec, releaseAudit := newAuditEnabledExecutor("")
		defer releaseAudit()
		if statePath != "" {
			exec = exec.WithCheckpoint(statePath)
		}
		if _, _, _, err := exec.ExecuteWithTrace(ctx, wf, reg, program); err != nil {
			log.Printf("Workflow execution error: %v", err)
		}
	}()

	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func runCLI(wf *workflow.Workflow, reg *nodes.Registry, statePath string) {
	if wf.Name != "" {
		fmt.Printf("%s\n", i18n.T("workflow.name", wf.Name))
	}
	if wf.Description != "" {
		fmt.Printf("%s\n", i18n.T("workflow.description", wf.Description))
	}
	fmt.Printf("\n%s\n", i18n.T("workflow.steps", len(wf.Steps)))
	for i, step := range wf.Steps {
		fmt.Printf("  %d. Node: %s\n", i+1, step.Node)
		if len(step.Params) > 0 {
			fmt.Printf("     Params: %v\n", redactParams(step.Params))
		}
	}

	fmt.Printf("\n=== %s ===\n", i18n.T("workflow.executing"))

	var finalOutput string
	var stepResults []workflow.StepResult
	var execErr error
	// Route through the Executor so tamper-evident audit logging is applied
	// to every CLI workflow run. Audit is a no-op when the HMAC key env vars
	// are not set (graceful degradation), so this never blocks execution.
	// newAuditEnabledExecutor acquires a per-process lock on the audit
	// directory (H-6) so concurrent aflare processes cannot fork the HMAC
	// hash chain; if the lock is held, audit is disabled for this process.
	exec, releaseAudit := newAuditEnabledExecutor("")
	defer releaseAudit()
	if statePath != "" {
		exec = exec.WithCheckpoint(statePath)
	}
	finalOutput, stepResults, execErr = exec.Execute(context.Background(), wf, reg)

	for _, result := range stepResults {
		status := "✅"
		if result.Error != nil {
			status = "❌"
			fmt.Printf("\n%s Step %d (%s): %v\n", status, result.StepIndex+1, result.NodeName, result.Error)
		} else {
			fmt.Printf("%s Step %d (%s): %s\n", status, result.StepIndex+1, result.NodeName, i18n.T("step.duration", result.Duration))
		}
	}

	if execErr != nil {
		fmt.Printf("\n%s\n", i18n.T("workflow.failed", execErr))
		os.Exit(1)
	}

	fmt.Printf("\n=== %s ===\n", i18n.T("workflow.final_output"))
	fmt.Println(finalOutput)
	fmt.Println("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("✅ "+i18n.T("workflow.completed")))
}

func handleSelfUpdate() {
	repo := "alib8b8/aflare"
	fmt.Println("Checking for updates...")
	result, err := cli.SelfUpdate(repo)
	if err != nil {
		fmt.Printf("❌ Update failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ %s\n", result)
}

func handleAutoUpgrade(args []string) {
	if len(args) == 0 {
		printAutoUpgradeUsage()
		os.Exit(1)
	}

	config, err := autoupgrade.LoadConfig()
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	engine := autoupgrade.NewUpgradeEngine(config)

	subCmd := args[0]
	switch subCmd {
	case "status":
		state := engine.GetState()
		fmt.Println("Auto-upgrade Status:")
		fmt.Printf("  Mode: %s\n", config.Mode)
		fmt.Printf("  Current Version: %s\n", state.CurrentVersion)
		fmt.Printf("  Latest Version: %s\n", state.LatestVersion)
		fmt.Printf("  Auto-update Enabled: %t\n", config.AutoUpdateEnabled)
		fmt.Printf("  Auto-merge Enabled: %t\n", config.AutoMergeEnabled)
		fmt.Printf("  Check Interval: %s\n", config.CheckInterval)
		fmt.Printf("  Backup Before Upgrade: %t\n", config.BackupBeforeUpgrade)
		fmt.Printf("  Rollback On Failure: %t\n", config.RollbackOnFailure)
		fmt.Printf("  Last Check: %s\n", state.LastCheck.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Last Upgrade: %s\n", state.LastUpgrade.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Upgrade In Progress: %t\n", state.UpgradeInProgress)
		fmt.Printf("  Upgrade Status: %s\n", state.UpgradeStatus)

	case "enable":
		config.Mode = autoupgrade.ModeAuto
		config.AutoUpdateEnabled = true
		if err := autoupgrade.SaveConfig(config); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Auto-upgrade enabled")
		fmt.Println("   Mode: auto (will automatically download and install updates)")

	case "disable":
		config.Mode = autoupgrade.ModeManual
		config.AutoUpdateEnabled = false
		if err := autoupgrade.SaveConfig(config); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Auto-upgrade disabled")
		fmt.Println("   Mode: manual (you need to run 'aflare self-update' manually)")

	case "monitor":
		config.Mode = autoupgrade.ModeMonitor
		if err := autoupgrade.SaveConfig(config); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Auto-upgrade set to monitor mode")
		fmt.Println("   Mode: monitor (checks for updates but does not install)")

	case "run":
		fmt.Println("Running manual upgrade check...")
		result, err := engine.RunSelfUpdate()
		if err != nil {
			fmt.Printf("❌ Upgrade failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ %s\n", result)

	case "config":
		if len(args) < 2 {
			fmt.Println("Usage: aflare autoupgrade config <key>=<value>")
			fmt.Println("\nAvailable keys:")
			fmt.Println("  mode           - auto, monitor, manual")
			fmt.Println("  auto_update    - true, false")
			fmt.Println("  auto_merge     - true, false")
			fmt.Println("  interval       - 1h, 6h, 24h, etc.")
			fmt.Println("  backup         - true, false")
			fmt.Println("  rollback       - true, false")
			os.Exit(1)
		}

		for _, arg := range args[1:] {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				fmt.Printf("⚠️ Invalid config format: %s\n", arg)
				continue
			}
			key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			updateConfigKey(config, key, value)
		}

		if err := autoupgrade.SaveConfig(config); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Config updated")

	case "auto-merge":
		result, err := engine.RunAutoMerge()
		if err != nil {
			fmt.Printf("❌ Auto-merge failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ %s\n", result)

	default:
		fmt.Printf("❌ Unknown command: %s\n", subCmd)
		printAutoUpgradeUsage()
		os.Exit(1)
	}
}

func printAutoUpgradeUsage() {
	fmt.Println("Usage: aflare autoupgrade <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  status       - Show current auto-upgrade status")
	fmt.Println("  enable       - Enable automatic updates")
	fmt.Println("  disable      - Disable automatic updates")
	fmt.Println("  monitor      - Enable monitor mode (notify only)")
	fmt.Println("  run          - Manually trigger upgrade check")
	fmt.Println("  config       - Configure auto-upgrade settings")
	fmt.Println("  auto-merge   - Run automatic branch merge")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare autoupgrade enable")
	fmt.Println("  aflare autoupgrade config mode=auto interval=6h")
	fmt.Println("  aflare autoupgrade run")
}

func handleWebUI(args []string) {
	host := ""
	port := ""
	workflowsDir := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--host", "-H":
			if i+1 < len(args) {
				host = args[i+1]
				i++
			}
		case "--port", "-p":
			if i+1 < len(args) {
				port = args[i+1]
				i++
			}
		case "--dir", "-d":
			if i+1 < len(args) {
				workflowsDir = args[i+1]
				i++
			}
		case "--help", "-h":
			printWebUIUsage()
			return
		default:
			fmt.Printf("Unknown argument: %s\n", args[i])
			printWebUIUsage()
			os.Exit(1)
		}
	}

	server := webui.NewWebUIServer(host, port)
	if workflowsDir != "" {
		server.SetWorkflowsDir(workflowsDir)
	}

	fmt.Printf("Starting WebUI server on http://localhost:%s\n", port)
	fmt.Println("Press Ctrl+C to stop")

	if err := server.Start(); err != nil {
		fmt.Printf("WebUI server error: %v\n", err)
		os.Exit(1)
	}
}

func printWebUIUsage() {
	fmt.Println("Usage: aflare webui [options]")
	fmt.Println("\nOptions:")
	fmt.Println("  --port, -p <port>    - WebUI server port (default: 8081)")
	fmt.Println("  --dir, -d <dir>      - Workflows directory")
	fmt.Println("  --help, -h           - Show this help")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare webui")
	fmt.Println("  aflare webui --port 8080")
	fmt.Println("  aflare webui --dir /path/to/workflows")
}

func handleServe(args []string) {
	host := ""
	port := "8080"
	apiKey := ""
	workflowsDir := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--host", "-H":
			if i+1 < len(args) {
				host = args[i+1]
				i++
			}
		case "--port", "-p":
			if i+1 < len(args) {
				port = args[i+1]
				i++
			}
		case "--api-key", "-k":
			if i+1 < len(args) {
				apiKey = args[i+1]
				i++
			}
		case "--dir", "-d":
			if i+1 < len(args) {
				workflowsDir = args[i+1]
				i++
			}
		case "--help", "-h":
			printServeUsage()
			return
		default:
			fmt.Printf("Unknown argument: %s\n", args[i])
			printServeUsage()
			os.Exit(1)
		}
	}

	server := api.NewServer(host, port, apiKey)
	if workflowsDir != "" {
		server.SetWorkflowsDir(workflowsDir)
	}

	fmt.Printf("Starting API server on http://localhost:%s\n", port)
	fmt.Println("Endpoints:")
	fmt.Println("  GET  /health               - Health check")
	fmt.Println("  GET  /api/v1/metrics        - Prometheus metrics")
	fmt.Println("  POST /api/v1/workflows/run  - Run a workflow")
	fmt.Println("  GET  /api/v1/workflows      - List available workflows")
	fmt.Println("  GET  /api/v1/workflows/{name} - Get workflow details")
	fmt.Println("Press Ctrl+C to stop")

	if err := server.Start(); err != nil {
		fmt.Printf("API server error: %v\n", err)
		os.Exit(1)
	}
}

func printServeUsage() {
	fmt.Println("Usage: aflare serve [options]")
	fmt.Println("\nOptions:")
	fmt.Println("  --port, -p <port>      - API server port (default: 8080)")
	fmt.Println("  --host, -H <host>      - API server host (default: 0.0.0.0)")
	fmt.Println("  --api-key, -k <key>    - API key for authentication")
	fmt.Println("  --dir, -d <dir>        - Workflows directory")
	fmt.Println("  --help, -h             - Show this help")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare serve")
	fmt.Println("  aflare serve --port 9090")
	fmt.Println("  aflare serve --api-key my-secret-key")
	fmt.Println("  aflare serve --dir /path/to/workflows")
	fmt.Println("  aflare --serve --port 8080")
}

func updateConfigKey(config *autoupgrade.UpgradeConfig, key, value string) {
	switch strings.ToLower(key) {
	case "mode":
		switch strings.ToLower(value) {
		case "auto":
			config.Mode = autoupgrade.ModeAuto
		case "monitor":
			config.Mode = autoupgrade.ModeMonitor
		case "manual":
			config.Mode = autoupgrade.ModeManual
		}
	case "auto_update", "auto-update", "autoupdate":
		config.AutoUpdateEnabled = strings.ToLower(value) == "true"
	case "auto_merge", "auto-merge", "automerge":
		config.AutoMergeEnabled = strings.ToLower(value) == "true"
	case "interval", "check_interval":
		config.CheckInterval = value
	case "backup", "backup_before_upgrade":
		config.BackupBeforeUpgrade = strings.ToLower(value) == "true"
	case "rollback", "rollback_on_failure":
		config.RollbackOnFailure = strings.ToLower(value) == "true"
	}
}

func handleInit(args []string) {
	mcpTarget := ""
	agentTarget := ""
	channel := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--mcp":
			if i+1 < len(args) {
				mcpTarget = args[i+1]
				i++
			} else {
				mcpTarget = "all"
			}
		case "--agent":
			if i+1 < len(args) {
				agentTarget = args[i+1]
				i++
			} else {
				agentTarget = "all"
			}
		case "--channel":
			if i+1 < len(args) {
				channel = args[i+1]
				i++
			}
		case "--help", "-h":
			printInitUsage()
			return
		default:
			if strings.HasPrefix(args[i], "--mcp=") {
				mcpTarget = strings.TrimPrefix(args[i], "--mcp=")
			} else if strings.HasPrefix(args[i], "--agent=") {
				agentTarget = strings.TrimPrefix(args[i], "--agent=")
			} else if strings.HasPrefix(args[i], "--channel=") {
				channel = strings.TrimPrefix(args[i], "--channel=")
			}
		}
	}

	if mcpTarget == "" && agentTarget == "" && channel == "" {
		printInitUsage()
		os.Exit(1)
	}

	if mcpTarget != "" {
		result, err := cli.SetupMCP(mcpTarget)
		if err != nil {
			fmt.Printf("❌ MCP setup failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ MCP configured for %s\n", result.Agent)
		fmt.Printf("   Config: %s\n", result.ConfigPath)
		fmt.Printf("   Command: %s\n", result.Command)
	}

	if agentTarget != "" {
		result, err := cli.InstallSkills(agentTarget)
		if err != nil {
			fmt.Printf("❌ Skill installation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Skills installed for %s\n", result.Agent)
		fmt.Printf("   Path: %s\n", result.SkillPath)
		if result.Installed {
			fmt.Println("   Status: skills copied successfully")
		} else {
			fmt.Println("   Status: skills directory ready (no source templates found)")
		}
	}

	if channel != "" {
		config, err := autoupgrade.LoadConfig()
		if err != nil {
			fmt.Printf("❌ Failed to load config: %v\n", err)
			os.Exit(1)
		}
		if err := autoupgrade.SetChannel(config, channel); err != nil {
			fmt.Printf("❌ Failed to set channel: %v\n", err)
			os.Exit(1)
		}
		if err := autoupgrade.SaveConfig(config); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Update channel set to: %s\n", channel)
	}
}

func printInitUsage() {
	fmt.Println("Usage: aflare init [options]")
	fmt.Println("\nInitialize aflare integration with AI agents and configure settings.")
	fmt.Println("\nOptions:")
	fmt.Println("  --mcp <agent>       Setup MCP server configuration (claude-code, opencode, all)")
	fmt.Println("  --agent <agent>     Install aflare skills to agent (claude-code, opencode, all)")
	fmt.Println("  --channel <channel> Set update channel (stable, beta, nightly)")
	fmt.Println("  -h, --help          Show this help message")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare init --mcp all")
	fmt.Println("  aflare init --mcp claude-code --agent all")
	fmt.Println("  aflare init --channel beta")
}

func handleSkills(args []string) {
	if len(args) == 0 {
		printSkillsUsage()
		return
	}

	templatesDir := meta.ResolveTemplatesPath()
	registry := skills.NewSkillRegistry(templatesDir)
	if err := registry.Load(); err != nil {
		fmt.Printf("❌ Failed to load skills: %v\n", err)
		os.Exit(1)
	}

	switch args[0] {
	case "list", "ls":
		skillList := registry.List()
		fmt.Printf("📦 Total skills: %d\n\n", len(skillList))
		for _, s := range skillList {
			fmt.Printf("  %-45s v%-8s %s\n", s.ID, s.Version, s.Description)
		}
	case "scan", "index":
		count := registry.Count()
		fmt.Printf("🔍 Scanned %d skills\n", count)
		if err := registry.SaveRegistry(); err != nil {
			fmt.Printf("❌ Failed to save registry: %v\n", err)
			os.Exit(1)
		}
		generated := registry.GenerateMissingMetas()
		fmt.Printf("✅ Registry saved, %d missing skill.json generated\n", generated)
	case "generate", "gen":
		generated := registry.GenerateMissingMetas()
		fmt.Printf("✅ Generated %d skill.json files\n", generated)
	case "save", "export":
		if err := registry.SaveRegistry(); err != nil {
			fmt.Printf("❌ Failed to save registry: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Registry saved to %s/%s\n", templatesDir, skills.RegistryFileName)
	case "search":
		if len(args) < 2 {
			fmt.Println("Usage: aflare skills search <keyword>")
			os.Exit(1)
		}
		results := registry.Search(args[1])
		fmt.Printf("🔍 Found %d skills matching \"%s\":\n\n", len(results), args[1])
		for _, s := range results {
			fmt.Printf("  %-45s %s\n", s.ID, s.Description)
		}
	case "categories", "cats":
		cats := registry.Categories()
		fmt.Printf("📂 Categories (%d):\n\n", len(cats))
		for _, cat := range cats {
			catSkills := registry.ListByCategory(cat)
			fmt.Printf("  %-30s %d skills\n", cat, len(catSkills))
		}
	case "category", "cat":
		if len(args) < 2 {
			fmt.Println("Usage: aflare skills category <name>")
			os.Exit(1)
		}
		catSkills := registry.ListByCategory(args[1])
		fmt.Printf("📂 Category: %s (%d skills)\n\n", args[1], len(catSkills))
		for _, s := range catSkills {
			fmt.Printf("  %-45s %s\n", s.ID, s.Description)
		}
	case "show", "get":
		if len(args) < 2 {
			fmt.Println("Usage: aflare skills show <id>")
			os.Exit(1)
		}
		s, err := registry.Get(args[1])
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("📦 Skill: %s\n", s.ID)
		fmt.Printf("   Name: %s\n", s.Name)
		fmt.Printf("   Version: %s\n", s.Version)
		fmt.Printf("   Author: %s\n", s.Author)
		fmt.Printf("   Category: %s\n", s.Category)
		fmt.Printf("   Description: %s\n", s.Description)
		fmt.Printf("   Tags: %v\n", s.Tags)
		fmt.Printf("   Keywords: %v\n", s.Keywords)
		if s.Path != "" {
			fmt.Printf("   Path: %s\n", s.Path)
		}
	case "-h", "--help", "help":
		printSkillsUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", args[0])
		printSkillsUsage()
		os.Exit(1)
	}
}

func printSkillsUsage() {
	fmt.Println("Usage: aflare skills <command> [options]")
	fmt.Println("\nManage and discover aflare skills/templates.")
	fmt.Println("\nCommands:")
	fmt.Println("  list, ls               List all available skills")
	fmt.Println("  scan, index            Scan templates and save registry")
	fmt.Println("  generate, gen          Generate missing skill.json files")
	fmt.Println("  save, export           Export skills registry to JSON")
	fmt.Println("  search <keyword>       Search skills by keyword")
	fmt.Println("  categories, cats       List all skill categories")
	fmt.Println("  category <name>        List skills in a category")
	fmt.Println("  show <id>              Show skill details")
	fmt.Println("  -h, --help             Show this help message")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare skills scan")
	fmt.Println("  aflare skills list")
	fmt.Println("  aflare skills search security")
	fmt.Println("  aflare skills category development")
}

func handleSchedule(args []string) {
	if len(args) == 0 {
		printScheduleUsage()
		os.Exit(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "add":
		handleScheduleAdd(args[1:])
	case "list":
		handleScheduleList()
	case "remove":
		handleScheduleRemove(args[1:])
	case "start":
		handleScheduleStart()
	case "-h", "--help", "help":
		printScheduleUsage()
	default:
		fmt.Printf("Unknown schedule subcommand: %s\n\n", subCmd)
		printScheduleUsage()
		os.Exit(1)
	}
}

func handleScheduleAdd(args []string) {
	var cronExpr, taskID, wfPath string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--cron":
			if i+1 >= len(args) {
				fmt.Println("❌ --cron requires a value")
				os.Exit(1)
			}
			cronExpr = args[i+1]
			i++
		case "--id":
			if i+1 >= len(args) {
				fmt.Println("❌ --id requires a value")
				os.Exit(1)
			}
			taskID = args[i+1]
			i++
		case "--help", "-h":
			printScheduleUsage()
			return
		default:
			if strings.HasPrefix(args[i], "--cron=") {
				cronExpr = strings.TrimPrefix(args[i], "--cron=")
			} else if strings.HasPrefix(args[i], "--id=") {
				taskID = strings.TrimPrefix(args[i], "--id=")
			} else if !strings.HasPrefix(args[i], "-") && wfPath == "" {
				wfPath = args[i]
			} else {
				fmt.Printf("❌ Unknown argument: %s\n", args[i])
				os.Exit(1)
			}
		}
	}

	if cronExpr == "" {
		fmt.Println("❌ --cron is required")
		printScheduleUsage()
		os.Exit(1)
	}
	if wfPath == "" {
		fmt.Println("❌ workflow file path is required")
		printScheduleUsage()
		os.Exit(1)
	}

	// Resolve to an absolute path so the schedule survives directory changes.
	absPath, err := filepath.Abs(wfPath)
	if err != nil {
		fmt.Printf("❌ Failed to resolve workflow path: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(absPath); err != nil {
		fmt.Printf("❌ Workflow file not found: %s\n", absPath)
		os.Exit(1)
	}
	wfPath = absPath

	// Default task ID: workflow filename stem.
	if taskID == "" {
		base := filepath.Base(wfPath)
		taskID = strings.TrimSuffix(base, filepath.Ext(base))
	}

	// Validate the cron expression using a throwaway scheduler.
	validateSched := scheduler.New()
	if err := validateSched.AddTask(taskID, cronExpr, func(context.Context) {}); err != nil {
		fmt.Printf("❌ Invalid cron expression: %v\n", err)
		os.Exit(1)
	}

	// Load existing schedules and check for duplicate ID.
	path := scheduler.DefaultSchedulesPath()
	entries, err := scheduler.LoadSchedules(path)
	if err != nil {
		fmt.Printf("❌ Failed to load schedules: %v\n", err)
		os.Exit(1)
	}
	for _, e := range entries {
		if e.ID == taskID {
			fmt.Printf("❌ Task with id %q already exists (use --id to specify a different id)\n", taskID)
			os.Exit(1)
		}
	}

	entries = append(entries, scheduler.ScheduleEntry{
		ID:           taskID,
		Cron:         cronExpr,
		WorkflowPath: wfPath,
	})
	if err := scheduler.SaveSchedules(path, entries); err != nil {
		fmt.Printf("❌ Failed to save schedule: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Scheduled task %q added\n", taskID)
	fmt.Printf("   Cron:     %s\n", cronExpr)
	fmt.Printf("   Workflow: %s\n", wfPath)
	fmt.Printf("   Run 'aflare schedule start' to begin executing on schedule.\n")
}

func handleScheduleList() {
	path := scheduler.DefaultSchedulesPath()
	entries, err := scheduler.LoadSchedules(path)
	if err != nil {
		fmt.Printf("❌ Failed to load schedules: %v\n", err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		fmt.Println("No scheduled tasks. Use 'aflare schedule add' to add one.")
		return
	}

	fmt.Printf("Scheduled tasks (%d):\n", len(entries))
	fmt.Println("-" + strings.Repeat("-", 78))
	fmt.Printf("  %-20s %-20s %s\n", "ID", "CRON", "WORKFLOW")
	fmt.Println("-" + strings.Repeat("-", 78))
	for _, e := range entries {
		fmt.Printf("  %-20s %-20s %s\n", e.ID, e.Cron, e.WorkflowPath)
	}
}

func handleScheduleRemove(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: aflare schedule remove <id>")
		os.Exit(1)
	}
	id := args[0]

	path := scheduler.DefaultSchedulesPath()
	entries, err := scheduler.LoadSchedules(path)
	if err != nil {
		fmt.Printf("❌ Failed to load schedules: %v\n", err)
		os.Exit(1)
	}

	found := false
	updated := make([]scheduler.ScheduleEntry, 0, len(entries))
	for _, e := range entries {
		if e.ID == id {
			found = true
			continue
		}
		updated = append(updated, e)
	}
	if !found {
		fmt.Printf("❌ Task with id %q not found\n", id)
		os.Exit(1)
	}

	if err := scheduler.SaveSchedules(path, updated); err != nil {
		fmt.Printf("❌ Failed to save schedules: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Removed task %q\n", id)
}

func handleScheduleStart() {
	path := scheduler.DefaultSchedulesPath()
	entries, err := scheduler.LoadSchedules(path)
	if err != nil {
		fmt.Printf("❌ Failed to load schedules: %v\n", err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		fmt.Println("No scheduled tasks. Use 'aflare schedule add' to add one.")
		os.Exit(1)
	}

	sched := scheduler.New()
	for _, e := range entries {
		entry := e // capture for closure
		wf, reg, err := cli.PrepareWorkflow(entry.WorkflowPath)
		if err != nil {
			fmt.Printf("❌ Failed to prepare workflow %q: %v\n", entry.WorkflowPath, err)
			os.Exit(1)
		}
		taskFunc := func(ctx context.Context) {
			// ctx is cancelled when the scheduler stops, so a scheduled
			// workflow in flight at shutdown can abort via its normal
			// context-propagation path instead of running to completion.
			if _, _, err := workflow.ExecuteWorkflow(ctx, wf, reg); err != nil {
				log.Printf("scheduled workflow %q execution failed: %v", entry.ID, err)
			}
		}
		if err := sched.AddTask(entry.ID, entry.Cron, taskFunc); err != nil {
			fmt.Printf("❌ Failed to add task %q: %v\n", entry.ID, err)
			os.Exit(1)
		}
		fmt.Printf("📋 Loaded task %q (%s -> %s)\n", entry.ID, entry.Cron, entry.WorkflowPath)
	}

	sched.Start()
	fmt.Printf("\n🚀 Scheduler started with %d task(s). Press Ctrl+C to stop.\n", len(entries))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\n⏹  Stopping scheduler...")
	sched.Stop()
	fmt.Println("✅ Scheduler stopped.")
}

func printScheduleUsage() {
	fmt.Println("Usage: aflare schedule <command> [options]")
	fmt.Println("\nSchedule workflows to run at specified times using cron expressions.")
	fmt.Println("\nCommands:")
	fmt.Println("  add --cron \"<expr>\" [--id <id>] <workflow.yaml>  Add a scheduled task")
	fmt.Println("  list                                            List all scheduled tasks")
	fmt.Println("  remove <id>                                     Remove a scheduled task")
	fmt.Println("  start                                           Start the scheduler (foreground)")
	fmt.Println("  -h, --help                                      Show this help message")
	fmt.Println("\nCron expression (5 fields): minute hour day-of-month month day-of-week")
	fmt.Println("  e.g. \"0 9 * * *\"      - daily at 09:00")
	fmt.Println("       \"*/15 * * * *\"   - every 15 minutes")
	fmt.Println("       \"0 9 * * 1-5\"    - weekdays at 09:00")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare schedule add --cron \"0 9 * * *\" my-workflow.yaml")
	fmt.Println("  aflare schedule add --id daily-report --cron \"0 9 * * *\" report.yaml")
	fmt.Println("  aflare schedule list")
	fmt.Println("  aflare schedule remove daily-report")
	fmt.Println("  aflare schedule start")
}

func handleAudit(args []string) {
	if len(args) == 0 {
		printAuditUsage()
		os.Exit(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "verify":
		handleAuditVerify(args[1:])
	case "-h", "--help", "help":
		printAuditUsage()
	default:
		fmt.Printf("Unknown audit subcommand: %s\n\n", subCmd)
		printAuditUsage()
		os.Exit(1)
	}
}

func handleAuditVerify(args []string) {
	auditPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file", "-f":
			if i+1 < len(args) {
				auditPath = args[i+1]
				i++
			} else {
				fmt.Println("❌ --file requires a value")
				os.Exit(1)
			}
		case "--help", "-h":
			fmt.Println("Usage: aflare audit verify [--file <path>]")
			return
		default:
			if strings.HasPrefix(args[i], "--file=") {
				auditPath = strings.TrimPrefix(args[i], "--file=")
			} else {
				fmt.Printf("❌ Unknown argument: %s\n", args[i])
				os.Exit(1)
			}
		}
	}

	if auditPath == "" {
		auditPath = history.GetAuditLogPath()
		if auditPath == "" {
			fmt.Println("❌ Could not resolve audit log path. Specify one with --file.")
			os.Exit(1)
		}
	}

	valid, brokenAt, err := history.VerifyAuditChain(auditPath)
	if err != nil {
		fmt.Printf("❌ Audit log verification error in %s: %v\n", auditPath, err)
		os.Exit(1)
	}
	if valid {
		fmt.Printf("✅ Audit log chain is valid: %s\n", auditPath)
		return
	}
	fmt.Printf("❌ Audit log chain is BROKEN at line %d: %s\n", brokenAt, auditPath)
	os.Exit(1)
}

func printAuditUsage() {
	fmt.Println("Usage: aflare audit <command> [options]")
	fmt.Println("\nVerify the integrity of the tamper-evident audit log hash chain.")
	fmt.Println("\nCommands:")
	fmt.Println("  verify [--file <path>]   Verify the audit log HMAC hash chain")
	fmt.Println("  -h, --help               Show this help message")
	fmt.Println("\nOptions:")
	fmt.Println("  --file, -f <path>   Path to the audit log file (defaults to the standard location)")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare audit verify")
	fmt.Println("  aflare audit verify --file /path/to/audit.log.jsonl")
}
