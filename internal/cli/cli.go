// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​​‌‌​‌​​‌​​​​‌​‌‌‌‌‌​​‌‌‌‌​‌‌​‌‌​​​‌‌‌​​‌​​‌​‌​​​​​​​​​​​​​​​​​​‌‌‌‌‌​​‌​‌​​​​​⁠
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
		switch {
		case arg == "--safe-mode":
			safeMode = true
		case arg == "--dry-run", arg == "--dry":
			dryRun = true
		case arg == "--mcp-server":
			mcpServer = true
		case arg == "--concise", arg == "-q", arg == "--quiet":
			concise = true
		case arg == "--lang":
			if i+1 < len(args) {
				lang = args[i+1]
				skipNext = true
			}
		case strings.HasPrefix(arg, "--lang="):
			lang = strings.TrimPrefix(arg, "--lang=")
		case arg == "--mcp":
			if i+1 < len(args) {
				initMCP = args[i+1]
				skipNext = true
			} else {
				initMCP = "all"
			}
		case strings.HasPrefix(arg, "--mcp="):
			initMCP = strings.TrimPrefix(arg, "--mcp=")
		case arg == "--agent":
			if i+1 < len(args) {
				initAgent = args[i+1]
				skipNext = true
			} else {
				initAgent = "all"
			}
		case strings.HasPrefix(arg, "--agent="):
			initAgent = strings.TrimPrefix(arg, "--agent=")
		case arg == "--channel":
			if i+1 < len(args) {
				updateChannel = args[i+1]
				skipNext = true
			}
		case strings.HasPrefix(arg, "--channel="):
			updateChannel = strings.TrimPrefix(arg, "--channel=")
		case arg == "--serve":
			serveMode = true
		case arg == "--ai":
			aiMode = true
		case arg == "--otel-endpoint":
			if i+1 < len(args) {
				otelEndpoint = args[i+1]
				skipNext = true
			}
		case strings.HasPrefix(arg, "--otel-endpoint="):
			otelEndpoint = strings.TrimPrefix(arg, "--otel-endpoint=")
		case arg == "--otel-service-name":
			if i+1 < len(args) {
				otelServiceName = args[i+1]
				skipNext = true
			}
		case strings.HasPrefix(arg, "--otel-service-name="):
			otelServiceName = strings.TrimPrefix(arg, "--otel-service-name=")
		default:
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

// knownCommands is the canonical list of top-level aflare commands. Shared by
// ValidateCommand and SuggestCommand so the two never drift apart.
var knownCommands = []string{
	"create", "run", "help", "install", "uninstall", "registry", "list",
	"validate", "review", "version", "self-update", "update", "upgrade",
	"autoupgrade", "init", "config", "webui", "schedule", "audit",
	"serve", "webhook", "secrets", "connector", "resume", "chat", "agent",
	"watermark", "doctor", "mcp",
}

// ValidateCommand checks if the command is recognized.
func ValidateCommand(command string) error {
	if command == "" {
		return fmt.Errorf("no command provided")
	}
	switch command {
	case "help", "-h", "--help", "--version", "-v", "au":
		return nil
	}
	for _, c := range knownCommands {
		if c == command {
			return nil
		}
	}
	return fmt.Errorf("unknown command: %s", command)
}

// SuggestCommand returns "did-you-mean" candidates for an unknown command.
// It matches by prefix first, then by a small edit distance, so common typos
// like "node" → "note" / "hisotry" → "history" surface a helpful hint instead
// of a bare usage dump. Returns at most 3 suggestions.
func SuggestCommand(command string) []string {
	if command == "" {
		return nil
	}
	lower := strings.ToLower(command)

	// 1. Exact prefix matches (e.g. "aut" → "autoupgrade").
	var prefixMatches []string
	for _, c := range knownCommands {
		if strings.HasPrefix(c, lower) {
			prefixMatches = append(prefixMatches, c)
		}
	}
	if len(prefixMatches) > 0 {
		return trimSuggestions(prefixMatches, 3)
	}

	// 2. Edit-distance <= 2 (typo correction).
	var typoMatches []string
	for _, c := range knownCommands {
		if levenshtein(lower, c) <= 2 {
			typoMatches = append(typoMatches, c)
		}
	}
	return trimSuggestions(typoMatches, 3)
}

func trimSuggestions(in []string, max int) []string {
	if len(in) > max {
		return in[:max]
	}
	return in
}

// levenshtein computes the edit distance between two strings. Used only for
// command suggestion, so it favours clarity over allocation perf.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
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
  aflare upgrade                         %s
  aflare doctor                          %s
  aflare autoupgrade <cmd>               %s
  aflare resume <run-id>                  %s
  aflare chat                            %s
  aflare agent                           %s
  aflare watermark <decode|verify|info>  %s
  aflare connector <cmd>                 %s
  aflare mcp                             %s
  aflare validate <workflow.yaml>        %s
  aflare list                            %s
  aflare init                            %s
  aflare config <cmd>                    %s
  aflare secrets <cmd>                   %s
  aflare schedule <cmd>                  %s
  aflare audit <cmd>                     %s
  aflare review <workflow.yaml>          %s
  aflare serve                           %s
  aflare webhook                         %s
  aflare webui                           %s
  aflare help                            %s

%s:
  --safe-mode   %s
  --lang <lang>  %s (en, zh)

%s:
  aflare init                            # interactive first-run wizard
  aflare config show                     # view current config
  aflare create "fetch example.com and save to file"
  aflare run examples/data-collector.yaml
  aflare --safe-mode run examples/data-collector.yaml
  aflare --lang zh run examples/data-collector.yaml
  aflare registry sync
  aflare registry search weather
  aflare doctor`,
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
		i18n.T("usage.upgrade"),
		i18n.T("usage.doctor"),
		i18n.T("usage.autoupgrade"),
		i18n.T("usage.resume"),
		i18n.T("usage.chat"),
		i18n.T("usage.agent"),
		i18n.T("usage.watermark"),
		i18n.T("usage.connector"),
		i18n.T("usage.mcp"),
		i18n.T("usage.validate"),
		i18n.T("usage.list"),
		i18n.T("usage.init"),
		i18n.T("usage.config"),
		i18n.T("usage.secrets"),
		i18n.T("usage.schedule"),
		i18n.T("usage.audit"),
		i18n.T("usage.review"),
		i18n.T("usage.serve"),
		i18n.T("usage.webhook"),
		i18n.T("usage.webui"),
		i18n.T("usage.help"),
		i18n.T("options"),
		i18n.T("usage.safe_mode"),
		i18n.T("options.lang"),
		i18n.T("examples"),
	)
}

// FirstRunHint returns a short, localized pointer shown after the usage text
// when aflare is invoked with no arguments, guiding new users to `aflare init`.
func FirstRunHint() string {
	return "\n" + i18n.T("usage.first_run_hint")
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
