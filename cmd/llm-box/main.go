package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/alib8b8/llm-box/internal/autoupgrade"
	"github.com/alib8b8/llm-box/internal/cli"
	"github.com/alib8b8/llm-box/internal/i18n"
	"github.com/alib8b8/llm-box/internal/mcp"
	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/tui"
	"github.com/alib8b8/llm-box/internal/webui"
	"github.com/alib8b8/llm-box/internal/workflow"
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

	command, args, safeMode, dryRun, mcpServer, lang := cli.ParseArgs(os.Args[1:])

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
	case "version", "--version", "-v":
		fmt.Println(cli.PrintVersion())
		return
	case "self-update", "update":
		handleSelfUpdate()
		return
	case "autoupgrade", "au":
		handleAutoUpgrade(args)
		return
	case "-h", "--help", "help":
		fmt.Println(cli.PrintUsage())
		return
	case "webui":
		handleWebUI(args)
		return
	default:
		handleRunFile(command, dryRun)
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
		fmt.Println("  sync     - llm-box registry sync")
		fmt.Println("  list     - llm-box registry list")
		fmt.Println("  search   - llm-box registry search <query>")
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
	return s[:maxLen-2] + ".."
}

func handleCreate(args []string) {
	if len(args) < 1 {
		fmt.Println(i18n.T("create.usage"))
		fmt.Println("\nExamples:")
		fmt.Println("  llm-box create \"fetch example.com and save to file\"")
		fmt.Println("  llm-box create \"fetch Hacker News and save to hn.txt\"")
		fmt.Println("  llm-box create \"summarize article and write to summary.md\"")
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
	fmt.Printf("  llm-box run %s\n", filename)
}

func handleRun(args []string, dryRun bool) {
	if len(args) < 1 {
		fmt.Println(i18n.T("run.usage"))
		os.Exit(1)
	}
	handleRunFile(args[0], dryRun)
}

func handleRunFile(wfPath string, dryRun bool) {
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

	if isatty.IsTerminal(os.Stdout.Fd()) {
		runTUI(wfPath, wf, reg)
	} else {
		runCLI(wf, reg)
	}
}

func runTUI(wfPath string, wf *workflow.Workflow, reg *nodes.Registry) {
	model := tui.NewModel(wf.Name, wfPath, len(wf.Steps))
	program := tea.NewProgram(model, tea.WithAltScreen())

	go func() {
		ctx := context.Background()
		if _, _, err := workflow.ExecuteWorkflowWithTUI(ctx, wf, reg, program); err != nil {
			log.Printf("Workflow execution error: %v", err)
		}
	}()

	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func runCLI(wf *workflow.Workflow, reg *nodes.Registry) {
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

	finalOutput, stepResults, err := cli.RunWorkflow(wf, reg)

	for _, result := range stepResults {
		status := "✅"
		if result.Error != nil {
			status = "❌"
			fmt.Printf("\n%s Step %d (%s): %v\n", status, result.StepIndex+1, result.NodeName, result.Error)
		} else {
			fmt.Printf("%s Step %d (%s): %s\n", status, result.StepIndex+1, result.NodeName, i18n.T("step.duration", result.Duration))
		}
	}

	if err != nil {
		fmt.Printf("\n%s\n", i18n.T("workflow.failed", err))
		os.Exit(1)
	}

	fmt.Printf("\n=== %s ===\n", i18n.T("workflow.final_output"))
	fmt.Println(finalOutput)
	fmt.Println("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("✅ "+i18n.T("workflow.completed")))
}

func handleSelfUpdate() {
	repo := "alib8b8/llm-box"
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
		fmt.Println("   Mode: manual (you need to run 'llm-box self-update' manually)")

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
			fmt.Println("Usage: llm-box autoupgrade config <key>=<value>")
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
	fmt.Println("Usage: llm-box autoupgrade <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  status       - Show current auto-upgrade status")
	fmt.Println("  enable       - Enable automatic updates")
	fmt.Println("  disable      - Disable automatic updates")
	fmt.Println("  monitor      - Enable monitor mode (notify only)")
	fmt.Println("  run          - Manually trigger upgrade check")
	fmt.Println("  config       - Configure auto-upgrade settings")
	fmt.Println("  auto-merge   - Run automatic branch merge")
	fmt.Println("\nExamples:")
	fmt.Println("  llm-box autoupgrade enable")
	fmt.Println("  llm-box autoupgrade config mode=auto interval=6h")
	fmt.Println("  llm-box autoupgrade run")
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
	fmt.Println("Usage: llm-box webui [options]")
	fmt.Println("\nOptions:")
	fmt.Println("  --port, -p <port>    - WebUI server port (default: 8081)")
	fmt.Println("  --dir, -d <dir>      - Workflows directory")
	fmt.Println("  --help, -h           - Show this help")
	fmt.Println("\nExamples:")
	fmt.Println("  llm-box webui")
	fmt.Println("  llm-box webui --port 8080")
	fmt.Println("  llm-box webui --dir /path/to/workflows")
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
