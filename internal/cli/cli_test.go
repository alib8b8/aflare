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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/alib8b8/llm-box/internal/i18n"
	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/workflow"
)

func TestParseArgs_BasicCommand(t *testing.T) {
	cmd, args, safeMode, dryRun, _, _, _, _, _, _, _ := ParseArgs([]string{"run", "workflow.yaml"})

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
	cmd, args, safeMode, _, _, _, _, _, _, _, _ := ParseArgs([]string{"--safe-mode", "run", "workflow.yaml"})

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
	_, args, safeMode, _, _, _, _, _, _, _, _ := ParseArgs([]string{"run", "--safe-mode", "workflow.yaml"})

	if len(args) != 1 || args[0] != "workflow.yaml" {
		t.Errorf("expected args ['workflow.yaml'], got %v", args)
	}
	if !safeMode {
		t.Error("expected safeMode to be true")
	}
}

func TestParseArgs_Empty(t *testing.T) {
	cmd, args, safeMode, _, _, _, _, _, _, _, _ := ParseArgs([]string{})

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
	cmd, _, safeMode, _, _, _, _, _, _, _, _ := ParseArgs([]string{"--safe-mode"})

	if cmd != "" {
		t.Errorf("expected empty command, got %q", cmd)
	}
	if !safeMode {
		t.Error("expected safeMode to be true")
	}
}

func TestParseArgs_WithDryRun(t *testing.T) {
	cmd, args, _, dryRun, _, _, _, _, _, _, _ := ParseArgs([]string{"--dry-run", "run", "workflow.yaml"})

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
	cmd, args, _, _, _, lang, _, _, _, _, _ := ParseArgs([]string{"--lang", "zh", "run", "workflow.yaml"})

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
	cmd, _, _, _, _, lang, _, _, _, _, _ := ParseArgs([]string{"--lang=en", "run", "workflow.yaml"})

	if cmd != "run" {
		t.Errorf("expected command 'run', got %q", cmd)
	}
	if lang != "en" {
		t.Errorf("expected lang 'en', got %q", lang)
	}
}

func TestParseArgs_WithMcpServer(t *testing.T) {
	cmd, _, _, _, mcpServer, _, _, _, _, _, _ := ParseArgs([]string{"--mcp-server"})

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
			cmd, _, _, _, _, _, _, _, _, _, _ := ParseArgs(tc.args)
			if cmd != tc.expected {
				t.Errorf("expected command %q, got %q", tc.expected, cmd)
			}
		})
	}
}

func TestParseArgs_DryShort(t *testing.T) {
	cmd, args, _, dryRun, _, _, _, _, _, _, _ := ParseArgs([]string{"--dry", "run", "workflow.yaml"})

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
	cmd, args, safeMode, dryRun, mcpServer, lang, concise, _, _, _, _ := ParseArgs([]string{
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
	cmd, args, _, _, _, lang, _, _, _, _, _ := ParseArgs([]string{"run", "workflow.yaml", "--lang", "en"})

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
	_, _, _, _, _, lang, _, _, _, _, _ := ParseArgs([]string{"--lang", "invalid-lang", "run", "workflow.yaml"})

	if lang != "invalid-lang" {
		t.Errorf("expected lang 'invalid-lang', got %q", lang)
	}
}

func TestParseArgs_LangNoValue(t *testing.T) {
	cmd, args, _, _, _, lang, _, _, _, _, _ := ParseArgs([]string{"--lang", "run", "workflow.yaml"})

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
	cmd, _, _, _, _, _, concise, _, _, _, _ := ParseArgs([]string{"--concise", "run", "workflow.yaml"})

	if cmd != "run" {
		t.Errorf("expected command 'run', got %q", cmd)
	}
	if !concise {
		t.Error("expected concise to be true")
	}
}

func TestParseArgs_WithQuiet(t *testing.T) {
	cmd, _, _, _, _, _, concise, _, _, _, _ := ParseArgs([]string{"-q", "run", "workflow.yaml"})

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
		t.Fatal("expected non-nil workflow")
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
		t.Fatal("expected non-nil workflow")
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
		t.Fatal("expected non-nil workflow")
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

func TestEscapeJSONString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{`hello "world"`, `hello \"world\"`},
		{`path\to\file`, `path\\to\\file`},
		{`mixed "quotes" and \slashes`, `mixed \"quotes\" and \\slashes`},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeJSONString(tt.input)
			if got != tt.want {
				t.Errorf("escapeJSONString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMergeMCPConfigSafe_Empty(t *testing.T) {
	result, err := mergeMCPConfigSafe([]byte("{}"), "/usr/bin/llm-box")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var cfg mcpConfig
	if err := json.Unmarshal([]byte(result), &cfg); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if cfg.MCPServers["llm-box"].Command != "/usr/bin/llm-box" {
		t.Errorf("expected command '/usr/bin/llm-box', got %q", cfg.MCPServers["llm-box"].Command)
	}
	if len(cfg.MCPServers["llm-box"].Args) != 1 || cfg.MCPServers["llm-box"].Args[0] != "--mcp-server" {
		t.Errorf("expected args ['--mcp-server'], got %v", cfg.MCPServers["llm-box"].Args)
	}
}

func TestMergeMCPConfigSafe_NilServers(t *testing.T) {
	result, err := mergeMCPConfigSafe([]byte("{}"), "/usr/bin/llm-box")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestMergeMCPConfigSafe_ExistingServer(t *testing.T) {
	existing := `{"mcpServers":{"llm-box":{"command":"old","args":["--old"]}}}`
	result, err := mergeMCPConfigSafe([]byte(existing), "/usr/bin/llm-box")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var cfg mcpConfig
	if err := json.Unmarshal([]byte(result), &cfg); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if cfg.MCPServers["llm-box"].Command != "old" {
		t.Errorf("expected existing command to be preserved 'old', got %q", cfg.MCPServers["llm-box"].Command)
	}
}

func TestMergeMCPConfigSafe_InvalidJSON(t *testing.T) {
	_, err := mergeMCPConfigSafe([]byte("invalid json"), "/usr/bin/llm-box")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGetBinaryPath(t *testing.T) {
	path, err := GetBinaryPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty binary path")
	}
}

func TestCopyDir(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(srcDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "file2.txt"), []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}

	data, err = os.ReadFile(filepath.Join(dstDir, "sub", "file2.txt"))
	if err != nil {
		t.Fatalf("failed to read copied sub file: %v", err)
	}
	if string(data) != "world" {
		t.Errorf("expected 'world', got %q", string(data))
	}
}

func TestCopyDir_SourceNotExist(t *testing.T) {
	err := copyDir("/nonexistent/source", "/tmp/dst")
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}

func TestInstallSkills_Unsupported(t *testing.T) {
	_, err := InstallSkills("unsupported-agent")
	if err == nil {
		t.Error("expected error for unsupported agent type")
	}
}

func TestInstallSkillsToDir_SourceNotExist(t *testing.T) {
	skillsDir := t.TempDir()
	result, err := installSkillsToDir(skillsDir, "/nonexistent/source", "TestAgent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Installed {
		t.Error("expected Installed to be false when source doesn't exist")
	}
	if result.Agent != "TestAgent" {
		t.Errorf("expected agent 'TestAgent', got %q", result.Agent)
	}
	if result.SkillPath != skillsDir {
		t.Errorf("expected skill path %q, got %q", skillsDir, result.SkillPath)
	}
}

func TestInstallSkillsToDir_WithSkills(t *testing.T) {
	skillsDir := t.TempDir()
	sourceDir := t.TempDir()

	skillDir := filepath.Join(sourceDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "workflow.yaml"), []byte("name: test"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := installSkillsToDir(skillsDir, sourceDir, "TestAgent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Installed {
		t.Error("expected Installed to be true")
	}

	installedPath := filepath.Join(skillsDir, "llm-box-test-skill")
	if _, err := os.Stat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected skill to be installed at %q", installedPath)
	}
}
