package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/alib8b8/llm-box/internal/cli"
	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/tui"
	"github.com/alib8b8/llm-box/internal/workflow"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println(cli.PrintUsage())
		os.Exit(1)
	}

	command, args, safeMode, dryRun := cli.ParseArgs(os.Args[1:])

	if safeMode {
		nodes.SetSafeMode(true)
		fmt.Println("🔒 Safe mode enabled - execute node and external nodes are disabled")
	}

	if dryRun {
		fmt.Println("📋 Dry run mode - workflow will be validated but not executed")
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
	case "-h", "--help", "help":
		fmt.Println(cli.PrintUsage())
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
		fmt.Println("No nodes registered")
		return
	}

	fmt.Println("Available nodes:")
	fmt.Println("-" + strings.Repeat("-", 78))
	fmt.Printf("  %-20s %s\n", "NAME", "DESCRIPTION")
	fmt.Println("-" + strings.Repeat("-", 78))
	for _, info := range nodeList {
		fmt.Printf("  %-20s %s\n", info.Name, info.Description)
	}

	installed, err := cli.ListInstalledNodes()
	if err == nil && len(installed) > 0 {
		fmt.Println("\nInstalled community nodes:")
		for _, name := range installed {
			fmt.Printf("  - %s\n", name)
		}
	}
}

func handleValidate(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: llm-box validate <workflow-file.yaml>")
		os.Exit(1)
	}

	wf, reg, err := cli.PrepareWorkflow(args[0])
	if err != nil {
		fmt.Printf("❌ Error loading workflow: %v\n", err)
		os.Exit(1)
	}

	warnings := workflow.ValidateWorkflow(wf)

	for i, step := range wf.Steps {
		if _, ok := reg.Get(step.Node); !ok {
			warnings = append(warnings, fmt.Sprintf("Step %d: unknown node '%s'", i+1, step.Node))
		}
	}

	if len(warnings) == 0 {
		fmt.Println("✅ Workflow is valid!")
	} else {
		fmt.Println("⚠️ Validation warnings:")
		for _, warning := range warnings {
			fmt.Printf("  - %s\n", warning)
		}
		os.Exit(1)
	}
}

func handleInstall(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: llm-box install <node-name>")
		fmt.Println("\nUse 'llm-box registry list' to see available nodes")
		os.Exit(1)
	}

	nodeName := args[0]
	if err := cli.InstallNode(nodeName); err != nil {
		fmt.Printf("❌ Failed to install node '%s': %v\n", nodeName, err)
		os.Exit(1)
	}

	fmt.Printf("✅ Node '%s' installed successfully!\n", nodeName)
	fmt.Println("\nTo use this node, add it to your workflow:")
	fmt.Printf("  steps:\n    - node: %s\n", nodeName)
}

func handleUninstall(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: llm-box uninstall <node-name>")
		os.Exit(1)
	}

	nodeName := args[0]
	if err := cli.UninstallNode(nodeName); err != nil {
		fmt.Printf("❌ Failed to uninstall node '%s': %v\n", nodeName, err)
		os.Exit(1)
	}

	fmt.Printf("✅ Node '%s' uninstalled successfully!\n", nodeName)
}

func handleRegistry(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: llm-box registry <command>")
		fmt.Println("\nCommands:")
		fmt.Println("  sync     - Sync node registry from server")
		fmt.Println("  list     - List available nodes")
		fmt.Println("  search   - Search for nodes by name, description, or tag")
		os.Exit(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "sync":
		if err := cli.SyncRegistry(); err != nil {
			fmt.Printf("❌ Failed to sync registry: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Registry synced successfully!")

	case "list":
		nodes, err := cli.ListRegistryNodes()
		if err != nil {
			fmt.Printf("❌ Failed to list nodes: %v\n", err)
			fmt.Println("\nTry running 'llm-box registry sync' first")
			os.Exit(1)
		}

		if len(nodes) == 0 {
			fmt.Println("No nodes available in registry")
			return
		}

		fmt.Println("Available nodes in registry:")
		fmt.Println("-" + strings.Repeat("-", 78))
		fmt.Printf("  %-15s %-40s %-10s %s\n", "NAME", "DESCRIPTION", "VERSION", "CATEGORY")
		fmt.Println("-" + strings.Repeat("-", 78))
		for _, node := range nodes {
			fmt.Printf("  %-15s %-40s %-10s %s\n", node.Name, truncate(node.Description, 38), node.Version, node.Category)
		}

	case "search":
		if len(args) < 2 {
			fmt.Println("Usage: llm-box registry search <query>")
			os.Exit(1)
		}

		query := strings.Join(args[1:], " ")
		nodes, err := cli.SearchRegistryNodes(query)
		if err != nil {
			fmt.Printf("❌ Failed to search nodes: %v\n", err)
			os.Exit(1)
		}

		if len(nodes) == 0 {
			fmt.Printf("No nodes found matching '%s'\n", query)
			return
		}

		fmt.Printf("Found %d node(s) matching '%s':\n", len(nodes), query)
		fmt.Println("-" + strings.Repeat("-", 78))
		fmt.Printf("  %-15s %-40s %-10s %s\n", "NAME", "DESCRIPTION", "VERSION", "CATEGORY")
		fmt.Println("-" + strings.Repeat("-", 78))
		for _, node := range nodes {
			fmt.Printf("  %-15s %-40s %-10s %s\n", node.Name, truncate(node.Description, 38), node.Version, node.Category)
		}

	default:
		fmt.Printf("Unknown registry command: %s\n", subCmd)
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
		fmt.Println("Usage: llm-box create \"your workflow description\"")
		fmt.Println("\nExamples:")
		fmt.Println("  llm-box create \"fetch example.com and save to file\"")
		fmt.Println("  llm-box create \"fetch Hacker News and save to hn.txt\"")
		fmt.Println("  llm-box create \"summarize article and write to summary.md\"")
		os.Exit(1)
	}

	description := cli.SummarizeCommand("", args)
	fmt.Printf("Creating workflow from: \"%s\"\n", description)

	filename, err := workflow.CreateWorkflowFromDescription(description)
	if err != nil {
		fmt.Printf("Error creating workflow: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ Workflow created: %s\n", filename)
	fmt.Println("\nTo run it:")
	fmt.Printf("  llm-box run %s\n", filename)
}

func handleRun(args []string, dryRun bool) {
	if len(args) < 1 {
		fmt.Println("Usage: llm-box run <workflow-file.yaml>")
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
		workflow.ExecuteWorkflowWithTUI(ctx, wf, reg, program)
	}()

	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func runCLI(wf *workflow.Workflow, reg *nodes.Registry) {
	if wf.Name != "" {
		fmt.Printf("Workflow: %s\n", wf.Name)
	}
	if wf.Description != "" {
		fmt.Printf("Description: %s\n", wf.Description)
	}
	fmt.Printf("\nSteps (%d):\n", len(wf.Steps))
	for i, step := range wf.Steps {
		fmt.Printf("  %d. Node: %s\n", i+1, step.Node)
		if len(step.Params) > 0 {
			fmt.Printf("     Params: %v\n", step.Params)
		}
	}

	fmt.Println("\n=== Executing workflow ===")

	finalOutput, stepResults, err := cli.RunWorkflow(wf, reg)

	for _, result := range stepResults {
		status := "✅"
		if result.Error != nil {
			status = "❌"
			fmt.Printf("\n%s Step %d (%s): %v\n", status, result.StepIndex+1, result.NodeName, result.Error)
		} else {
			fmt.Printf("%s Step %d (%s): took %v\n", status, result.StepIndex+1, result.NodeName, result.Duration)
		}
	}

	if err != nil {
		fmt.Printf("\nWorkflow failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== Final Output ===")
	fmt.Println(finalOutput)
	fmt.Println("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("✅ Workflow completed!"))
}
