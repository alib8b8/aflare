package main

import (
	"context"
	"fmt"
	"os"

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

	command, args, safeMode := cli.ParseArgs(os.Args[1:])

	if safeMode {
		nodes.SetSafeMode(true)
		fmt.Println("🔒 Safe mode enabled - execute node and external nodes are disabled")
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
		handleRun(args)
		return
	case "-h", "--help", "help":
		fmt.Println(cli.PrintUsage())
		return
	default:
		handleRunFile(command)
	}
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

func handleRun(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: llm-box run <workflow-file.yaml>")
		os.Exit(1)
	}
	handleRunFile(args[0])
}

func handleRunFile(wfPath string) {
	wf, reg, err := cli.PrepareWorkflow(wfPath)
	if err != nil {
		fmt.Printf("Error preparing workflow: %v\n", err)
		os.Exit(1)
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
		program.Send(tea.QuitMsg{})
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
