// Copyright (c) 2026 llm-box Contributors
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
	"os"
	"path/filepath"
	"testing"

	"github.com/alib8b8/llm-box/internal/i18n"
	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/workflow"
)

func TestParseArgs_BasicCommand(t *testing.T) {
	cmd, args, safeMode, dryRun, _, _, _, _, _, _ := ParseArgs([]string{"run", "workflow.yaml"})

	if cmd != "run" {
		t.Errorf("expected command 'run', got %q", cmd)
	}
	if len(args) != 1 || args[0] != "workflow.yaml" {
		t.Errorf("expected args ['workflow.yaml'], got %v", args)
	}
	if safeMode {
		t.Error("expected safeMode to be false")
	}
	if dryRun {
		t.Error("expected dryRun to be false")
	}
}

func TestParseArgs_WithSafeMode(t *testing.T) {
	cmd, args, safeMode, _, _, _, _, _, _, _ := ParseArgs([]string{"--safe-mode", "run", "workflow.yaml"})

	if cmd != "run" {
		t.Errorf("expected command 'run', got %q", cmd)
	}
	if len(args) != 1 || args[0] != "workflow.yaml" {
		t.Errorf("expected args ['workflow.yaml'], got %v", args)
	}
	if !safeMode {
		t.Error("expected safeMode to be true")
	}
}

func TestParseArgs_SafeModeInMiddle(t *testing.T) {
	_, args, safeMode, _, _, _, _, _, _, _ := ParseArgs([]string{"run", "--safe-mode", "workflow.yaml"})

	if len(args) != 1 || args[0] != "workflow.yaml" {
		t.Errorf("expected args ['workflow.yaml'], got %v", args)
	}
	if !safeMode {
		t.Error("expected safeMode to be true")
	}
}

func TestParseArgs_Empty(t *testing.T) {
	cmd, args, safeMode, _, _, _, _, _, _, _ := ParseArgs([]string{})

	if cmd != "" {
		t.Errorf("expected empty command, got %q", cmd)
	}
	if args != nil {
		t.Errorf("expected nil args, got %v", args)
	}
	if safeMode {
		t.Error("expected safeMode to be false")
	}
}

func TestParseArgs_OnlySafeMode(t *testing.T) {
	cmd, _, safeMode, _, _, _, _, _, _, _ := ParseArgs([]string{"--safe-mode"})

	if cmd != "" {
		t.Errorf("expected empty command, got %q", cmd)
	}
	if !safeMode {
		t.Error("expected safeMode to be true")
	}
}

func TestParseArgs_WithDryRun(t *testing.T) {
	cmd, args, _, dryRun, _, _, _, _, _, _ := ParseArgs([]string{"--dry-run", "run", "workflow.yaml"})

	if cmd != "run" {
		t.Errorf("expected command 'run', got %q", cmd)
	}
	if len(args) != 1 || args[0] != "workflow.yaml" {
		t.Errorf("expected args ['workflow.yaml'], got %v", args)
	}
	if !dryRun {
		t.Error("expected dryRun to be true")
	}
}

func TestParseArgs_WithLang(t *testing.T) {
	cmd, args, _, _, _, lang, _, _, _, _ := ParseArgs([]string{"--lang", "zh", "run", "workflow.yaml"})

	if cmd != "run" {
		t.Errorf("expected command 'run', got %q", cmd)
	}
	if len(args) != 1 || args[0] != "workflow.yaml" {
		t.Errorf("expected args ['workflow.yaml'], got %v", args)
	}
	if lang != "zh" {
		t.Errorf("expected lang 'zh', got %q", lang)
	}
}

func TestParseArgs_WithLangEquals(t *testing.T) {
	cmd, _, _, _, _, lang, _, _, _, _ := ParseArgs([]string{"--lang=en", "run", "workflow.yaml"})

	if cmd != "run" {
		t.Errorf("expected command 'run', got %q", cmd)
	}
	if lang != "en" {
		t.Errorf("expected lang 'en', got %q", lang)
	}
}

func TestParseArgs_WithMcpServer(t *testing.T) {
	cmd, _, _, _, mcpServer, _, _, _, _, _ := ParseArgs([]string{"--mcp-server"})

	if cmd != "" {
		t.Errorf("expected empty command, got %q", cmd)
	}
	if !mcpServer {
		t.Error("expected mcpServer to be true")
	}
}

func TestParseArgs_HelpFlags(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		expected string
	}{
		{"help", []string{"help"}, "help"},
		{"-h", []string{"-h"}, "-h"},
		{"--help", []string{"--help"}, "--help"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, _, _, _, _, _, _, _, _, _ := ParseArgs(tc.args)
			if cmd != tc.expected {
				t.Errorf("expected command %q, got %q", tc.expected, cmd)
			}
		})
	}
}

func TestParseArgs_DryShort(t *testing.T) {
	cmd, args, _, dryRun, _, _, _, _, _, _ := ParseArgs([]string{"--dry", "run", "workflow.yaml"})

	if cmd != "run" {
		t.Errorf("expected command 'run', got %q", cmd)
	}
	if len(args) != 1 || args[0] != "workflow.yaml" {
		t.Errorf("expected args ['workflow.yaml'], got %v", args)
	}
	if !dryRun {
		t.Error("expected dryRun to be true for --dry")
	}
}

func TestParseArgs_MultipleFlags(t *testing.T) {
	cmd, args, safeMode, dryRun, mcpServer, lang, concise, _, _, _ := ParseArgs([]string{
		"--safe-mode", "--dry-run", "--lang=zh", "--mcp-server",
		"run", "workflow.yaml", "arg1", "arg2",
	})

	if cmd != "run" {
		t.Errorf("expected command 'run', got %q", cmd)
	}
	if len(args) != 3 || args[0] != "workflow.yaml" || args[1] != "arg1" || args[2] != "arg2" {
		t.Errorf("expected args ['workflow.yaml', 'arg1', 'arg2'], got %v", args)
	}
	if !safeMode {
		t.Error("expected safeMode to be true")
	}
	if !dryRun {
		t.Error("expected dryRun to be true")
	}
	if !mcpServer {
		t.Error("expected mcpServer to be true")
	}
	if lang != "zh" {
		t.Errorf("expected lang 'zh', got %q", lang)
	}
	if concise {
		t.Error("expected concise to be false")
	}
}

func TestParseArgs_LangAtEnd(t *testing.T) {
	cmd, args, _, _, _, lang, _, _, _, _ := ParseArgs([]string{"run", "workflow.yaml", "--lang", "en"})

	if cmd != "run" {
		t.Errorf("expected command 'run', got %q", cmd)
	}
	if len(args) != 1 || args[0] != "workflow.yaml" {
		t.Errorf("expected args ['workflow.yaml'], got %v", args)
	}
	if lang != "en" {
		t.Errorf("expected lang 'en', got %q", lang)
	}
}

func TestParseArgs_LangInvalidValue(t *testing.T) {
	_, _, _, _, _, lang, _, _, _, _ := ParseArgs([]string{"--lang", "invalid-lang", "run", "workflow.yaml"})

	if lang != "invalid-lang" {
		t.Errorf("expected lang 'invalid-lang', got %q", lang)
	}
}

func TestParseArgs_LangNoValue(t *testing.T) {
	cmd, args, _, _, _, lang, _, _, _, _ := ParseArgs([]string{"--lang", "run", "workflow.yaml"})

	if cmd != "workflow.yaml" {
		t.Errorf("expected command 'workflow.yaml', got %q", cmd)
	}
	if len(args) != 0 {
		t.Errorf("expected args [], got %v", args)
	}
	if lang != "run" {
		t.Errorf("expected lang 'run', got %q", lang)
	}
}

func TestParseArgs_WithConcise(t *testing.T) {
	cmd, _, _, _, _, _, concise, _, _, _ := ParseArgs([]string{"--concise", "run", "workflow.yaml"})

	if cmd != "run" {
		t.Errorf("expected command 'run', got %q", cmd)
	}
	if !concise {
		t.Error("expected concise to be true")
	}
}

func TestParseArgs_WithQuiet(t *testing.T) {
	cmd, _, _, _, _, _, concise, _, _, _ := ParseArgs([]string{"-q", "run", "workflow.yaml"})

	if cmd != "run" {
		t.Errorf("expected command 'run', got %q", cmd)
	}
	if !concise {
		t.Error("expected concise to be true for -q")
	}
}

func TestValidateCommand_Valid(t *testing.T) {
	commands := []string{"create", "run", "help", "-h", "--help", "install", "uninstall", "registry", "list", "validate"}
	for _, cmd := range commands {
		if err := ValidateCommand(cmd); err != nil {
			t.Errorf("expected command %q to be valid, got error: %v", cmd, err)
		}
	}
}

func TestValidateCommand_ValidAdditional(t *testing.T) {
	commands := []string{"version", "--version", "-v", "self-update", "update", "autoupgrade", "au"}
	for _, cmd := range commands {
		if err := ValidateCommand(cmd); err != nil {
			t.Errorf("expected command %q to be valid, got error: %v", cmd, err)
		}
	}
}

func TestValidateCommand_Invalid(t *testing.T) {
	commands := []string{"foobar", "workflow.yaml", "unknown"}
	for _, cmd := range commands {
		if err := ValidateCommand(cmd); err == nil {
			t.Errorf("expected command %q to be invalid, got nil error", cmd)
		}
	}
}

func TestValidateCommand_Empty(t *testing.T) {
	err := ValidateCommand("")
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestPrintUsage(t *testing.T) {
	i18n.Init("en")
	usage := PrintUsage()
	if usage == "" {
		t.Error("expected non-empty usage")
	}

	expectedSubstrings := []string{
		"llm-box",
		"create",
		"run",
		"--safe-mode",
	}
	for _, substr := range expectedSubstrings {
		if !contains(usage, substr) {
			t.Errorf("expected usage to contain %q", substr)
		}
	}
}

func TestPrintUsage_Chinese(t *testing.T) {
	i18n.Init("zh")
	usage := PrintUsage()
	if usage == "" {
		t.Error("expected non-empty usage")
	}
	if !contains(usage, "llm-box") {
		t.Error("expected usage to contain 'llm-box'")
	}
}

func TestSummarizeCommand(t *testing.T) {
	result := SummarizeCommand("create", []string{"fetch", "example.com", "and", "summarize"})
	expected := "create fetch example.com and summarize"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSummarizeCommand_EmptyArgs(t *testing.T) {
	result := SummarizeCommand("run", []string{})
	if result != "run" {
		t.Errorf("expected 'run', got %q", result)
	}
}

func TestSummarizeCommand_NoCommand(t *testing.T) {
	result := SummarizeCommand("", []string{"hello", "world"})
	if result != " hello world" {
		t.Errorf("expected ' hello world', got %q", result)
	}
}

func TestPrepareWorkflow_NotFound(t *testing.T) {
	_, _, err := PrepareWorkflow("/nonexistent/file.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestPrepareWorkflow_ValidFile(t *testing.T) {
	tempDir := t.TempDir()
	workflowPath := filepath.Join(tempDir, "workflow.yaml")

	workflowContent := `
name: "Test Workflow"
description: "A test workflow"
steps:
  - node: execute
    params:
      command: "echo hello"
`
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("failed to create workflow file: %v", err)
	}

	wf, reg, err := PrepareWorkflow(workflowPath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if wf == nil {
		t.Error("expected non-nil workflow")
	}
	if wf.Name != "Test Workflow" {
		t.Errorf("expected workflow name 'Test Workflow', got %q", wf.Name)
	}
	if wf.Description != "A test workflow" {
		t.Errorf("expected workflow description 'A test workflow', got %q", wf.Description)
	}
	if len(wf.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(wf.Steps))
	}
	if reg == nil {
		t.Error("expected non-nil registry")
	}
}

func TestPrepareWorkflow_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	workflowPath := filepath.Join(tempDir, "invalid.yaml")

	if err := os.WriteFile(workflowPath, []byte("invalid: [yaml"), 0644); err != nil {
		t.Fatalf("failed to create invalid workflow file: %v", err)
	}

	_, _, err := PrepareWorkflow(workflowPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestPrepareWorkflow_EmptySteps(t *testing.T) {
	tempDir := t.TempDir()
	workflowPath := filepath.Join(tempDir, "empty_steps.yaml")

	workflowContent := `
name: "Empty Steps"
steps: []
`
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("failed to create workflow file: %v", err)
	}

	wf, reg, err := PrepareWorkflow(workflowPath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if wf == nil {
		t.Error("expected non-nil workflow")
	}
	if len(wf.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(wf.Steps))
	}
	if reg == nil {
		t.Error("expected non-nil registry")
	}
}

func TestPrepareWorkflow_WithVars(t *testing.T) {
	tempDir := t.TempDir()
	workflowPath := filepath.Join(tempDir, "with_vars.yaml")

	workflowContent := `
name: "Workflow with Vars"
vars:
  url: "https://example.com"
steps:
  - node: execute
    params:
      command: "echo {{url}}"
`
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("failed to create workflow file: %v", err)
	}

	wf, reg, err := PrepareWorkflow(workflowPath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if wf == nil {
		t.Error("expected non-nil workflow")
	}
	if wf.Vars == nil {
		t.Error("expected non-nil vars")
	}
	if wf.Vars["url"] != "https://example.com" {
		t.Errorf("expected var 'url' to be 'https://example.com', got %q", wf.Vars["url"])
	}
	if reg == nil {
		t.Error("expected non-nil registry")
	}
}

func TestRunWorkflow_Success(t *testing.T) {
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	wf := &workflow.Workflow{
		Name: "Test Workflow",
		Steps: []workflow.WorkflowStep{
			{Node: "execute", Params: map[string]string{"command": "echo 'hello world'"}},
		},
	}

	output, results, err := RunWorkflow(wf, reg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if output == "" {
		t.Error("expected non-empty output")
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestRunWorkflow_InvalidNode(t *testing.T) {
	reg := nodes.NewRegistry()

	wf := &workflow.Workflow{
		Name: "Test Workflow",
		Steps: []workflow.WorkflowStep{
			{Node: "nonexistent_node", Params: map[string]string{"command": "echo hello"}},
		},
	}

	_, _, err := RunWorkflow(wf, reg)
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestInstallNode(t *testing.T) {
	err := InstallNode("test-node")
	if err == nil {
		t.Log("InstallNode succeeded (this is expected for valid nodes or mock implementations)")
	}
}

func TestUninstallNode(t *testing.T) {
	err := UninstallNode("test-node")
	if err == nil {
		t.Log("UninstallNode succeeded (this is expected for valid nodes or mock implementations)")
	}
}

func TestSyncRegistry(t *testing.T) {
	err := SyncRegistry()
	if err == nil {
		t.Log("SyncRegistry succeeded")
	}
}

func TestListRegistryNodes(t *testing.T) {
	nodes, err := ListRegistryNodes()
	if err != nil {
		t.Logf("ListRegistryNodes returned error: %v", err)
		return
	}
	t.Logf("Found %d nodes in registry", len(nodes))
}

func TestSearchRegistryNodes(t *testing.T) {
	nodes, err := SearchRegistryNodes("test")
	if err != nil {
		t.Logf("SearchRegistryNodes returned error: %v", err)
		return
	}
	t.Logf("Found %d nodes matching 'test'", len(nodes))
}

func TestListInstalledNodes(t *testing.T) {
	nodes, err := ListInstalledNodes()
	if err != nil {
		t.Logf("ListInstalledNodes returned error: %v", err)
		return
	}
	t.Logf("Found %d installed nodes", len(nodes))
}

func TestPrintVersion(t *testing.T) {
	version := PrintVersion()
	if version == "" {
		t.Error("expected non-empty version string")
	}
}

func TestCheckUpdate(t *testing.T) {
	tag, hasUpdate, err := CheckUpdate("test/repo")
	if err != nil {
		t.Logf("CheckUpdate returned error: %v", err)
		return
	}
	t.Logf("Latest tag: %s, has update: %v", tag, hasUpdate)
}

func TestSelfUpdate(t *testing.T) {
	result, err := SelfUpdate("test/repo")
	if err != nil {
		t.Logf("SelfUpdate returned error: %v", err)
		return
	}
	t.Logf("SelfUpdate result: %s", result)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
