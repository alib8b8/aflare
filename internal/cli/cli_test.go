package cli

import (
	"testing"
)

func TestParseArgs_BasicCommand(t *testing.T) {
	cmd, args, safeMode := ParseArgs([]string{"run", "workflow.yaml"})

	if cmd != "run" {
		t.Errorf("expected command 'run', got %q", cmd)
	}
	if len(args) != 1 || args[0] != "workflow.yaml" {
		t.Errorf("expected args ['workflow.yaml'], got %v", args)
	}
	if safeMode {
		t.Error("expected safeMode to be false")
	}
}

func TestParseArgs_WithSafeMode(t *testing.T) {
	cmd, args, safeMode := ParseArgs([]string{"--safe-mode", "run", "workflow.yaml"})

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
	_, args, safeMode := ParseArgs([]string{"run", "--safe-mode", "workflow.yaml"})

	if len(args) != 1 || args[0] != "workflow.yaml" {
		t.Errorf("expected args ['workflow.yaml'], got %v", args)
	}
	if !safeMode {
		t.Error("expected safeMode to be true")
	}
}

func TestParseArgs_Empty(t *testing.T) {
	cmd, args, safeMode := ParseArgs([]string{})

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
	cmd, _, safeMode := ParseArgs([]string{"--safe-mode"})

	if cmd != "" {
		t.Errorf("expected empty command, got %q", cmd)
	}
	if !safeMode {
		t.Error("expected safeMode to be true")
	}
}

func TestValidateCommand_Valid(t *testing.T) {
	commands := []string{"create", "run", "help", "-h", "--help", "workflow.yaml"}
	for _, cmd := range commands {
		if err := ValidateCommand(cmd); err != nil {
			t.Errorf("expected command %q to be valid, got error: %v", cmd, err)
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

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
