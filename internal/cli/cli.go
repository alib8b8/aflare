package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/registry"
	"github.com/alib8b8/llm-box/internal/workflow"
)

// ParseArgs parses the command-line arguments and returns the command and its arguments.
// It also detects the --safe-mode and --dry-run flags.
func ParseArgs(args []string) (command string, commandArgs []string, safeMode bool, dryRun bool) {
	safeMode = false
	dryRun = false
	var filtered []string
	for _, arg := range args {
		if arg == "--safe-mode" {
			safeMode = true
		} else if arg == "--dry-run" || arg == "--dry" {
			dryRun = true
		} else {
			filtered = append(filtered, arg)
		}
	}

	if len(filtered) == 0 {
		return "", nil, safeMode, dryRun
	}

	command = filtered[0]
	commandArgs = filtered[1:]
	return
}

// ValidateCommand checks if the command is recognized.
func ValidateCommand(command string) error {
	switch command {
	case "create", "run", "help", "-h", "--help", "install", "uninstall", "registry":
		return nil
	}
	if command == "" {
		return fmt.Errorf("no command provided")
	}
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
  llm-box install <node-name>              Install a community node
  llm-box uninstall <node-name>            Uninstall a community node
  llm-box registry sync                    Sync node registry from server
  llm-box registry list                    List available nodes in registry
  llm-box registry search <query>          Search for nodes in registry
  llm-box help                            Show this help message

Options:
  --safe-mode   Run in safe mode (disables execute and external nodes)

Examples:
  llm-box create "fetch example.com and save to file"
  llm-box run examples/basic_summary.yaml
  llm-box --safe-mode run examples/multi_step.yaml
  llm-box registry sync
  llm-box registry search weather
  llm-box install weather_api`
}

// RunWorkflow executes a workflow and returns the final output and results.
// This is the non-TUI entry point suitable for testing.
func RunWorkflow(wf *workflow.Workflow, reg *nodes.Registry) (string, []workflow.StepResult, error) {
	ctx := context.Background()
	return workflow.ExecuteWorkflow(ctx, wf, reg)
}

func InstallNode(name string) error {
	return registry.InstallNode(name)
}

func UninstallNode(name string) error {
	return registry.UninstallNode(name)
}

func SyncRegistry() error {
	return registry.SyncRegistry()
}

func ListRegistryNodes() ([]registry.NodeInfo, error) {
	return registry.ListNodes()
}

func SearchRegistryNodes(query string) ([]registry.NodeInfo, error) {
	return registry.SearchNodes(query)
}

func ListInstalledNodes() ([]string, error) {
	return registry.ListInstalledNodes()
}

// SummarizeCommand returns a summary of the command, useful for error messages.
func SummarizeCommand(command string, args []string) string {
	if len(args) == 0 {
		return command
	}
	return fmt.Sprintf("%s %s", command, strings.Join(args, " "))
}
