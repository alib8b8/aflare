package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/workflow"
)

// ParseArgs parses the command-line arguments and returns the command and its arguments.
// It also detects the --safe-mode flag.
func ParseArgs(args []string) (command string, commandArgs []string, safeMode bool) {
	safeMode = false
	var filtered []string
	for _, arg := range args {
		if arg == "--safe-mode" {
			safeMode = true
		} else {
			filtered = append(filtered, arg)
		}
	}

	if len(filtered) == 0 {
		return "", nil, safeMode
	}

	command = filtered[0]
	commandArgs = filtered[1:]
	return
}

// ValidateCommand checks if the command is recognized.
func ValidateCommand(command string) error {
	switch command {
	case "create", "run", "help", "-h", "--help":
		return nil
	}
	if command == "" {
		return fmt.Errorf("no command provided")
	}
	// Allow file paths as default
	return nil
}

// PrepareWorkflow loads a workflow file and returns the parsed workflow with the registry.
// External nodes are loaded from ./nodes directory if available.
func PrepareWorkflow(wfPath string) (*workflow.Workflow, *nodes.Registry, error) {
	wf, err := workflow.ParseWorkflow(wfPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	reg := nodes.GetGlobalRegistry()

	wd, err := os.Getwd()
	if err == nil {
		nodesDir := filepath.Join(wd, "nodes")
		if loadErr := reg.LoadExternalNodes(nodesDir); loadErr != nil {
			// Non-fatal: just log via error chain
			return wf, reg, nil
		}
	}

	return wf, reg, nil
}

// PrintUsage prints the program usage information
func PrintUsage() string {
	return `llm-box - Terminal-first AI workflow engine

Usage:
  llm-box create "workflow description"   Create a new workflow from natural language
  llm-box run <workflow-file.yaml>         Run a YAML workflow
  llm-box help                            Show this help message

Options:
  --safe-mode   Run in safe mode (disables execute and external nodes)

Examples:
  llm-box create "fetch example.com and save to file"
  llm-box run examples/basic_summary.yaml
  llm-box --safe-mode run examples/multi_step.yaml`
}

// RunWorkflow executes a workflow and returns the final output and results.
// This is the non-TUI entry point suitable for testing.
func RunWorkflow(wf *workflow.Workflow, reg *nodes.Registry) (string, []workflow.StepResult, error) {
	ctx := context.Background()
	return workflow.ExecuteWorkflow(ctx, wf, reg)
}

// SummarizeCommand returns a summary of the command, useful for error messages.
func SummarizeCommand(command string, args []string) string {
	if len(args) == 0 {
		return command
	}
	return fmt.Sprintf("%s %s", command, strings.Join(args, " "))
}
