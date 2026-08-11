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

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alib8b8/aflare/internal/i18n"
	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/meta"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/registry"
	"github.com/alib8b8/aflare/internal/workflow"
)

// ParseArgs parses the command-line arguments and returns the command and its arguments.
// It also detects the --safe-mode, --dry-run, --mcp-server, --lang, --concise, and --ai flags.
func ParseArgs(args []string) (command string, commandArgs []string, safeMode bool, dryRun bool, mcpServer bool, lang string, concise bool, initMCP string, initAgent string, updateChannel string, serveMode bool, aiMode bool, otelEndpoint string, otelServiceName string) {
	safeMode = false
	dryRun = false
	mcpServer = false
	lang = ""
	concise = false
	initMCP = ""
	initAgent = ""
	updateChannel = ""
	serveMode = false
	aiMode = false
	otelEndpoint = ""
	otelServiceName = ""
	var filtered []string
	skipNext := false
	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == "--safe-mode" {
			safeMode = true
		} else if arg == "--dry-run" || arg == "--dry" {
			dryRun = true
		} else if arg == "--mcp-server" {
			mcpServer = true
		} else if arg == "--concise" || arg == "-q" || arg == "--quiet" {
			concise = true
		} else if arg == "--lang" {
			if i+1 < len(args) {
				lang = args[i+1]
				skipNext = true
			}
		} else if strings.HasPrefix(arg, "--lang=") {
			lang = strings.TrimPrefix(arg, "--lang=")
		} else if arg == "--mcp" {
			if i+1 < len(args) {
				initMCP = args[i+1]
				skipNext = true
			} else {
				initMCP = "all"
			}
		} else if strings.HasPrefix(arg, "--mcp=") {
			initMCP = strings.TrimPrefix(arg, "--mcp=")
		} else if arg == "--agent" {
			if i+1 < len(args) {
				initAgent = args[i+1]
				skipNext = true
			} else {
				initAgent = "all"
			}
		} else if strings.HasPrefix(arg, "--agent=") {
			initAgent = strings.TrimPrefix(arg, "--agent=")
		} else if arg == "--channel" {
			if i+1 < len(args) {
				updateChannel = args[i+1]
				skipNext = true
			}
		} else if strings.HasPrefix(arg, "--channel=") {
			updateChannel = strings.TrimPrefix(arg, "--channel=")
		} else if arg == "--serve" {
			serveMode = true
		} else if arg == "--ai" {
			aiMode = true
		} else if arg == "--otel-endpoint" {
			if i+1 < len(args) {
				otelEndpoint = args[i+1]
				skipNext = true
			}
		} else if strings.HasPrefix(arg, "--otel-endpoint=") {
			otelEndpoint = strings.TrimPrefix(arg, "--otel-endpoint=")
		} else if arg == "--otel-service-name" {
			if i+1 < len(args) {
				otelServiceName = args[i+1]
				skipNext = true
			}
		} else if strings.HasPrefix(arg, "--otel-service-name=") {
			otelServiceName = strings.TrimPrefix(arg, "--otel-service-name=")
		} else {
			filtered = append(filtered, arg)
		}
	}

	// Validate --lang against supported languages (warning only)
	if lang != "" {
		supported := i18n.AvailableLanguages()
		valid := false
		for _, l := range supported {
			if l == lang {
				valid = true
				break
			}
		}
		if !valid {
			fmt.Fprintf(os.Stderr, "Warning: unsupported language '%s', supported: %s\n", lang, strings.Join(supported, ", "))
		}
	}

	if len(filtered) == 0 {
		return "", nil, safeMode, dryRun, mcpServer, lang, concise, initMCP, initAgent, updateChannel, serveMode, aiMode, otelEndpoint, otelServiceName
	}

	command = filtered[0]
	commandArgs = filtered[1:]
	return
}

// ValidateCommand checks if the command is recognized.
func ValidateCommand(command string) error {
	if command == "" {
		return fmt.Errorf("no command provided")
	}
	switch command {
	case "create", "run", "help", "-h", "--help", "install", "uninstall", "registry", "list", "validate", "review", "version", "--version", "-v", "self-update", "update", "autoupgrade", "au", "init", "webui", "skills", "schedule", "audit", "serve", "marketplace", "secrets", "resume", "chat", "agent":
		return nil
	}
	return fmt.Errorf("unknown command: %s", command)
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
			// Non-fatal: log the error and continue
			logger.Warn("failed to load external nodes", "dir", nodesDir, "error", loadErr)
		}
	}

	return wf, reg, nil
}

// PrintUsage prints the program usage information
func PrintUsage() string {
	return fmt.Sprintf(`%s

%s:
  aflare create "workflow description"   %s
  aflare run <workflow-file.yaml>         %s
  aflare install <node-name>              %s
  aflare uninstall <node-name>            %s
  aflare registry sync                    %s
  aflare registry list                    %s
  aflare registry search <query>          %s
  aflare version                         %s
  aflare self-update                     %s
  aflare autoupgrade <cmd>               %s
  aflare resume <run-id>                  %s
  aflare chat                            %s
  aflare agent                           %s
  aflare help                            %s

%s:
  --safe-mode   %s
  --lang <lang>  %s (en, zh)

%s:
  aflare create "fetch example.com and save to file"
  aflare run examples/basic_summary.yaml
  aflare --safe-mode run examples/multi_step.yaml
  aflare --lang zh run examples/basic_summary.yaml
  aflare registry sync
  aflare registry search weather
  aflare install weather_api`,
		i18n.T("usage.title"),
		i18n.T("usage.help"),
		i18n.T("usage.create"),
		i18n.T("usage.run"),
		i18n.T("usage.install"),
		i18n.T("usage.uninstall"),
		i18n.T("usage.registry_sync"),
		i18n.T("usage.registry_list"),
		i18n.T("usage.registry_search"),
		i18n.T("usage.version"),
		i18n.T("usage.self_update"),
		i18n.T("usage.autoupgrade"),
		i18n.T("usage.resume"),
		i18n.T("usage.chat"),
		i18n.T("usage.agent"),
		i18n.T("usage.help"),
		i18n.T("options"),
		i18n.T("usage.safe_mode"),
		i18n.T("options.lang"),
		i18n.T("examples"),
	)
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

func PrintVersion() string {
	return meta.GetBuildInfo()
}

func SelfUpdate(repo string) (string, error) {
	return meta.SelfUpdate(repo)
}

func CheckUpdate(repo string) (string, bool, error) {
	release, err := meta.CheckLatestRelease(repo)
	if err != nil {
		return "", false, err
	}
	hasUpdate := meta.HasUpdate(meta.GetVersion(), release)
	return release.TagName, hasUpdate, nil
}

// SummarizeCommand returns a summary of the command, useful for error messages.
func SummarizeCommand(command string, args []string) string {
	if len(args) == 0 {
		return command
	}
	return fmt.Sprintf("%s %s", command, strings.Join(args, " "))
}
