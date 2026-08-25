// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation; either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package agentx

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// writeFakeAgent creates an executable script that echoes its arguments
// prefixed with AGENT-OK, or sleeps forever when mode == "hang".
func writeFakeAgent(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-agent")
	var content string
	switch mode {
	case "hang":
		content = "#!/bin/sh\nsleep 60\n"
	case "fail":
		content = "#!/bin/sh\necho \"boom\" >&2\nexit 3\n"
	default:
		content = "#!/bin/sh\necho \"AGENT-OK $@\"\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil { // #nosec G306 -- test helper must be executable
		t.Fatalf("write fake agent: %v", err)
	}
	return path
}

func testAudit(entries *[]string) func(string) error {
	return func(entry string) error {
		*entries = append(*entries, entry)
		return nil
	}
}

func TestRunCLI_GenericProfile_EchoesPrompt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake agent is POSIX-only")
	}
	bin := writeFakeAgent(t, "ok")
	var audit []string

	def := AgentDef{Name: "fake", Driver: DriverCLI, Profile: "generic", Binary: bin, Args: []string{"run", "--flag"}}
	out, err := RunCLI(context.Background(), def, Task{
		Prompt: "do the thing",
		Audit:  testAudit(&audit),
	})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if !strings.HasPrefix(out, "AGENT-OK") {
		t.Errorf("output = %q, want AGENT-OK prefix", out)
	}
	// The literal args come first, then the prompt as a single argv
	// element behind "--".
	if !strings.Contains(out, "run --flag -- do the thing") {
		t.Errorf("output = %q, want argv containing \"run --flag -- do the thing\"", out)
	}
	if len(audit) != 1 || !strings.Contains(audit[0], "agent=fake") {
		t.Errorf("audit entries = %v, want one entry naming the agent", audit)
	}
}

func TestRunCLI_PromptStartingWithDashIsNotAFlag(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake agent is POSIX-only")
	}
	bin := writeFakeAgent(t, "ok")

	def := AgentDef{Name: "fake", Driver: DriverCLI, Profile: "generic", Binary: bin}
	out, err := RunCLI(context.Background(), def, Task{Prompt: "--dangerous"})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if !strings.Contains(out, "-- --dangerous") {
		t.Errorf("output = %q, prompt must sit behind \"--\" separator", out)
	}
}

func TestRunCLI_ModelInjectionBlocked(t *testing.T) {
	def := AgentDef{Name: "fake", Driver: DriverCLI, Profile: "codex", Binary: "fake"}
	_, err := RunCLI(context.Background(), def, Task{Prompt: "x", Model: "--inject-flag"})
	if err == nil || !strings.Contains(err.Error(), "invalid model") {
		t.Fatalf("err = %v, want invalid model rejection", err)
	}
}

func TestRunCLI_BadSandboxBlocked(t *testing.T) {
	def := AgentDef{Name: "fake", Driver: DriverCLI, Profile: "codex", Binary: "fake"}
	_, err := RunCLI(context.Background(), def, Task{Prompt: "x", Sandbox: "no-sandbox-please"})
	if err == nil || !strings.Contains(err.Error(), "invalid codex sandbox") {
		t.Fatalf("err = %v, want sandbox rejection", err)
	}
}

func TestRunCLI_ClaudeProfile_RejectsInteractiveApproval(t *testing.T) {
	def := AgentDef{Name: "fake", Driver: DriverCLI, Profile: "claude", Binary: "fake"}
	_, err := RunCLI(context.Background(), def, Task{Prompt: "x", Approval: "on-request"})
	if err == nil || !strings.Contains(err.Error(), "approval_policy=never") {
		t.Fatalf("err = %v, want claude approval rejection", err)
	}
}

func TestRunCLI_RelativeBinaryPathBlocked(t *testing.T) {
	def := AgentDef{Name: "fake", Driver: DriverCLI, Profile: "generic", Binary: "./sneaky/agent"}
	_, err := RunCLI(context.Background(), def, Task{Prompt: "x"})
	if err == nil || !strings.Contains(err.Error(), "bare command name or an absolute path") {
		t.Fatalf("err = %v, want relative binary rejection", err)
	}
}

func TestRunCLI_EmptyPromptRejected(t *testing.T) {
	def := AgentDef{Name: "fake", Driver: DriverCLI, Profile: "generic", Binary: "fake"}
	_, err := RunCLI(context.Background(), def, Task{Prompt: "   "})
	if err == nil || !strings.Contains(err.Error(), "non-empty prompt") {
		t.Fatalf("err = %v, want empty prompt rejection", err)
	}
}

func TestRunCLI_AuditFailClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake agent is POSIX-only")
	}
	bin := writeFakeAgent(t, "ok")
	def := AgentDef{Name: "fake", Driver: DriverCLI, Profile: "generic", Binary: bin}
	_, err := RunCLI(context.Background(), def, Task{
		Prompt: "x",
		Audit:  func(string) error { return os.ErrPermission },
	})
	if err == nil || !strings.Contains(err.Error(), "audit log") {
		t.Fatalf("err = %v, want fail-closed audit error", err)
	}
}

func TestRunCLI_TimeoutKillsSubprocess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake agent is POSIX-only")
	}
	bin := writeFakeAgent(t, "hang")
	def := AgentDef{Name: "fake", Driver: DriverCLI, Profile: "generic", Binary: bin}
	start := time.Now()
	_, err := RunCLI(context.Background(), def, Task{Prompt: "x", Timeout: 2 * time.Second})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want timeout error", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("timeout took %s, deadline not enforced", elapsed)
	}
}

func TestRunCLI_NonZeroExitSurfacesStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake agent is POSIX-only")
	}
	bin := writeFakeAgent(t, "fail")
	def := AgentDef{Name: "fake", Driver: DriverCLI, Profile: "generic", Binary: bin}
	_, err := RunCLI(context.Background(), def, Task{Prompt: "x"})
	if err == nil {
		t.Fatal("want error on non-zero exit")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v, want stderr content surfaced", err)
	}
}

func TestRunCLI_CwdCanonicalized(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake agent is POSIX-only")
	}
	bin := writeFakeAgent(t, "ok")
	dir := t.TempDir()
	// Sneak in a traversal segment: must still resolve inside the temp
	// dir, never escape it.
	sneaky := filepath.Join(dir, "sub", "..", "inner")
	inner := filepath.Join(dir, "inner")
	if err := os.MkdirAll(inner, 0o750); err != nil {
		t.Fatal(err)
	}

	def := AgentDef{Name: "fake", Driver: DriverCLI, Profile: "generic", Binary: bin}
	_, err := RunCLI(context.Background(), def, Task{Prompt: "x", Cwd: sneaky})
	if err != nil {
		t.Fatalf("RunCLI with sneaky cwd: %v", err)
	}
}

func TestRunCLI_CwdMustExist(t *testing.T) {
	def := AgentDef{Name: "fake", Driver: DriverCLI, Profile: "generic", Binary: "fake"}
	_, err := RunCLI(context.Background(), def, Task{Prompt: "x", Cwd: "/nonexistent/path/for/aflare/test"})
	if err == nil || !strings.Contains(err.Error(), "cwd") {
		t.Fatalf("err = %v, want cwd rejection", err)
	}
}

func TestBuildProfileArgs_CodexShape(t *testing.T) {
	def := AgentDef{Name: "c", Profile: "codex"}
	args, err := buildProfileArgs(def, "do it", "gpt-5.6", "strict", "never", 5, "/tmp")
	if err != nil {
		t.Fatalf("buildProfileArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"exec --sandbox strict --approval-policy never", "--model gpt-5.6", "--max-turns 5", "--cwd /tmp", "-- do it"} {
		if !strings.Contains(joined, want) {
			t.Errorf("codex argv %q missing %q", joined, want)
		}
	}
}

func TestBuildProfileArgs_ClaudeShape(t *testing.T) {
	def := AgentDef{Name: "cl", Profile: "claude"}
	args, err := buildProfileArgs(def, "do it", "sonnet", "", "never", 3, "")
	if err != nil {
		t.Fatalf("buildProfileArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-p do it", "--output-format text", "--model sonnet", "--max-turns 3"} {
		if !strings.Contains(joined, want) {
			t.Errorf("claude argv %q missing %q", joined, want)
		}
	}
}

func TestBuildProfileArgs_GeminiShape(t *testing.T) {
	def := AgentDef{Name: "g", Profile: "gemini"}
	args, err := buildProfileArgs(def, "do it", "gemini-2.5-pro", "", "", 0, "")
	if err != nil {
		t.Fatalf("buildProfileArgs: %v", err)
	}
	joined := strings.Join(args, " ")
	if joined != "-p do it --model gemini-2.5-pro" {
		t.Errorf("gemini argv = %q", joined)
	}
}

func TestIsValidModelName(t *testing.T) {
	valid := []string{"gpt-5.6", "openai/o4-mini", "claude-sonnet-4", "gemini_2.5", "m.x"}
	invalid := []string{"", "-x", "--flag", "a b", "a;b", "model\n", strings.Repeat("m", 129)}
	for _, m := range valid {
		if !isValidModelName(m) {
			t.Errorf("isValidModelName(%q) = false, want true", m)
		}
	}
	for _, m := range invalid {
		if isValidModelName(m) {
			t.Errorf("isValidModelName(%q) = true, want false", m)
		}
	}
}
