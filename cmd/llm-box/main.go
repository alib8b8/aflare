package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/tui"
	"github.com/alib8b8/llm-box/internal/workflow"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Handle commands
	command := os.Args[1]

	switch command {
	case "create":
		handleCreate()
		return
	case "run":
		handleRun()
		return
	case "-h", "--help", "help":
		printUsage()
		return
	default:
		// If not a known command, assume it's a workflow file
		handleRunFile(command)
	}
}

func handleCreate() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: llm-box create \"your workflow description\"")
		fmt.Println("\nExamples:")
		fmt.Println("  llm-box create \"fetch example.com and save to file\"")
		fmt.Println("  llm-box create \"fetch Hacker News and save to hn.txt\"")
		fmt.Println("  llm-box create \"summarize article and write to summary.md\"")
		os.Exit(1)
	}

	description := strings.Join(os.Args[2:], " ")
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

func handleRun() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: llm-box run <workflow-file.yaml>")
		os.Exit(1)
	}
	handleRunFile(os.Args[2])
}

func handleRunFile(wfPath string) {
	wf, err := workflow.ParseWorkflow(wfPath)
	if err != nil {
		fmt.Printf("Error parsing workflow: %v\n", err)
		os.Exit(1)
	}

	// Get the global registry (nodes register themselves on init)
	reg := nodes.GetGlobalRegistry()

	// Load external nodes from ./nodes directory
	wd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Warning: failed to get working directory: %v\n", err)
	} else {
		nodesDir := filepath.Join(wd, "nodes")
		if err := reg.LoadExternalNodes(nodesDir); err != nil {
			fmt.Printf("Warning: failed to load external nodes: %v\n", err)
		}
	}

	// Check if we're in a TTY (interactive terminal)
	if isatty.IsTerminal(os.Stdout.Fd()) {
		runTUI(wfPath, wf, reg)
	} else {
		runCLI(wf, reg)
	}
}

func printUsage() {
	fmt.Println(`llm-box - Terminal-first AI workflow engine

Usage:
  llm-box create "workflow description"   Create a new workflow from natural language
  llm-box run <workflow-file.yaml>         Run a YAML workflow
  llm-box help                            Show this help message

Examples:
  llm-box create "fetch example.com and save to file"
  llm-box run examples/basic_summary.yaml
  llm-box run examples/multi_step.yaml`)
}

// runTUI runs the workflow with the TUI interface
func runTUI(wfPath string, wf *workflow.Workflow, reg *nodes.Registry) {
	model := tui.NewModel(wf.Name, wfPath, len(wf.Steps))
	program := tea.NewProgram(model, tea.WithAltScreen())

	// Run the workflow in a goroutine
	go func() {
		ctx := context.Background()
		workflow.ExecuteWorkflowWithTUI(ctx, wf, reg, program)
		// Give the TUI a moment to show final state
		// then send quit
		program.Send(tea.QuitMsg{})
	}()

	// Start the TUI
	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

// runCLI runs the workflow in command-line mode
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

	ctx := context.Background()
	finalOutput, stepResults, err := workflow.ExecuteWorkflow(ctx, wf, reg)

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
