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

package nodes

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// writeMockCodex writes a mock `codex` executable that echoes its argv (one
// per line, prefixed with "arg:") to stdout, so tests can assert exactly how
// the node constructed the command line. When sleep is non-zero it sleeps
// first, letting tests exercise the timeout path.
func writeMockCodex(t *testing.T, sleep time.Duration, stdout, stderr string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("mock codex shell script not supported on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "codex")
	script := "#!/bin/sh\n"
	if sleep > 0 {
		script += "sleep " + sleep.String() + "\n"
	}
	if stdout != "" {
		script += "printf '%s\\n' '" + stdout + "'\n"
	}
	if stderr != "" {
		script += "printf '%s\\n' '" + stderr + "' >&2\n"
	}
	script += "for a in \"$@\"; do printf 'arg:%s\\n' \"$a\"; done\n"
	if err := os.WriteFile(path, []byte(script), 0750); err != nil {
		t.Fatalf("write mock codex: %v", err)
	}
	return path
}

func TestCodexAgentNode_FlagConstruction(t *testing.T) {
	bin := writeMockCodex(t, 0, "", "")
	node := &CodexAgentNode{}

	out, err := node.Execute(context.Background(), "fix the failing tests", map[string]string{
		"binary":          bin,
		"model":           "gpt-5.6",
		"sandbox":         "permissive",
		"approval_policy": "on-failure",
		"max_turns":       "20",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	want := []string{
		"arg:exec",
		"arg:--sandbox",
		"arg:permissive",
		"arg:--approval-policy",
		"arg:on-failure",
		"arg:--model",
		"arg:gpt-5.6",
		"arg:--max-turns",
		"arg:20",
		"arg:--",
		"arg:fix the failing tests",
	}
	got := strings.Split(strings.TrimSpace(out), "\n")
	if len(got) != len(want) {
		t.Fatalf("output = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("argv[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCodexAgentNode_Defaults(t *testing.T) {
	bin := writeMockCodex(t, 0, "final answer here", "")
	node := &CodexAgentNode{}

	out, err := node.Execute(context.Background(), "do a thing", map[string]string{"binary": bin})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "final answer here") {
		t.Errorf("output missing mock stdout: %q", out)
	}
	// Defaults: strict sandbox, never approval, no model/max-turns/cwd flags.
	for _, want := range []string{"arg:--sandbox", "arg:strict", "arg:--approval-policy", "arg:never"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing default flag %q in %q", want, out)
		}
	}
	for _, unwanted := range []string{"arg:--model", "arg:--max-turns", "arg:--cwd"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("output should not contain %q: %q", unwanted, out)
		}
	}
}

func TestCodexAgentNode_PromptNotShellInterpreted(t *testing.T) {
	// The prompt contains shell metacharacters; because it is passed as a
	// single argv element (no sh -c), they must arrive verbatim — an
	// injected `; touch pwned` must NOT execute.
	dir := t.TempDir()
	bin := writeMockCodex(t, 0, "", "")
	node := &CodexAgentNode{}

	out, err := node.Execute(context.Background(), "hello; touch "+filepath.Join(dir, "pwned"), map[string]string{"binary": bin})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "pwned")); statErr == nil {
		t.Fatal("injected command was executed — prompt must not go through a shell")
	}
	if !strings.Contains(out, "arg:hello; touch ") {
		t.Errorf("prompt not passed verbatim: %q", out)
	}
}

func TestCodexAgentNode_Timeout(t *testing.T) {
	bin := writeMockCodex(t, 5*time.Second, "", "")
	node := &CodexAgentNode{}

	start := time.Now()
	_, err := node.Execute(context.Background(), "slow task", map[string]string{
		"binary":  bin,
		"timeout": "1s",
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %v, want timeout", err)
	}
	if elapsed := time.Since(start); elapsed >= 4*time.Second {
		t.Errorf("timeout took %v, want ~1s", elapsed)
	}
}

func TestCodexAgentNode_MissingBinary(t *testing.T) {
	node := &CodexAgentNode{}
	_, err := node.Execute(context.Background(), "task", map[string]string{
		"binary": filepath.Join(t.TempDir(), "does-not-exist"),
	})
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	// LookPath resolves the binary before spawning, so a missing binary is
	// reported directly instead of as a spawn-time exec failure.
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want lookup failure", err)
	}
}

func TestCodexAgentNode_StderrSurfacedOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codex-fail")
	if runtime.GOOS == "windows" {
		t.Skip("mock codex shell script not supported on windows")
	}
	script := "#!/bin/sh\necho 'auth required: run codex login' >&2\nexit 3\n"
	if err := os.WriteFile(path, []byte(script), 0750); err != nil {
		t.Fatalf("write mock: %v", err)
	}

	node := &CodexAgentNode{}
	_, err := node.Execute(context.Background(), "task", map[string]string{"binary": path})
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(err.Error(), "codex login") {
		t.Errorf("error should surface codex stderr diagnostics: %v", err)
	}
}

func TestCodexAgentNode_StdoutEmptyFallsBackToStderr(t *testing.T) {
	// A mock that writes ONLY to stderr (no arg echo on stdout), so the
	// stdout-empty fallback path is exercised.
	if runtime.GOOS == "windows" {
		t.Skip("mock codex shell script not supported on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "codex-stderr-only")
	script := "#!/bin/sh\necho 'answer on stderr' >&2\n"
	if err := os.WriteFile(path, []byte(script), 0750); err != nil {
		t.Fatalf("write mock: %v", err)
	}

	node := &CodexAgentNode{}
	out, err := node.Execute(context.Background(), "task", map[string]string{"binary": path})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "answer on stderr") {
		t.Errorf("fallback to stderr failed: %q", out)
	}
}

func TestCodexAgentNode_Validation(t *testing.T) {
	node := &CodexAgentNode{}
	ctx := context.Background()

	cases := []struct {
		name   string
		input  string
		params map[string]string
		want   string
	}{
		{"empty prompt", "  ", map[string]string{}, "non-empty prompt"},
		{"bad sandbox", "x", map[string]string{"sandbox": "loose"}, "invalid sandbox"},
		{"bad approval", "x", map[string]string{"approval_policy": "always"}, "invalid approval_policy"},
		{"max_turns negative", "x", map[string]string{"max_turns": "-1"}, "max_turns out of range"},
		{"max_turns huge", "x", map[string]string{"max_turns": "100000"}, "max_turns out of range"},
		{"bad model chars", "x", map[string]string{"model": "gpt; rm -rf /"}, "invalid model"},
		{"model flag smuggling", "x", map[string]string{"model": "--dangerous-flag"}, "invalid model"},
		{"binary relative path", "x", map[string]string{"binary": "./codex"}, "bare command name or an absolute path"},
		{"cwd missing", "x", map[string]string{"cwd": "/nonexistent-dir-xyz"}, "not accessible"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := node.Execute(ctx, tc.input, tc.params)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Execute(%s) error = %v, want containing %q", tc.name, err, tc.want)
			}
		})
	}
}

func TestCodexAgentNode_SafeMode(t *testing.T) {
	prev := IsSafeMode()
	SetSafeMode(true)
	defer SetSafeMode(prev)

	node := &CodexAgentNode{}
	_, err := node.Execute(context.Background(), "task", nil)
	if err == nil || !strings.Contains(err.Error(), "safe mode") {
		t.Errorf("safe mode should block codex_agent, got: %v", err)
	}
}

func TestCodexBinaryPath_Override(t *testing.T) {
	if got := CodexBinaryPath("/custom/path/codex"); got != "/custom/path/codex" {
		t.Errorf("CodexBinaryPath override = %q", got)
	}
	// No override: either a resolved PATH hit or the bare name fallback.
	got := CodexBinaryPath("")
	if got == "" {
		t.Error("CodexBinaryPath default should not be empty")
	}
}

func TestCodexAgentNode_PromptStartingWithDashIsPositional(t *testing.T) {
	// A prompt that starts with "-" must arrive after the "--" separator
	// so the CLI treats it as the task text, not as an unknown flag.
	bin := writeMockCodex(t, 0, "", "")
	node := &CodexAgentNode{}

	out, err := node.Execute(context.Background(), "--version", map[string]string{"binary": bin})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	last := lines[len(lines)-1]
	if last != "arg:--version" {
		t.Errorf("last argv = %q, want prompt verbatim after --", last)
	}
	foundSep := false
	for _, l := range lines {
		if l == "arg:--" {
			foundSep = true
		}
	}
	if !foundSep {
		t.Errorf("argv missing the -- separator: %q", lines)
	}
}

func TestCodexAgentNode_CwdNotADirectory(t *testing.T) {
	bin := writeMockCodex(t, 0, "", "")
	filePath := filepath.Join(t.TempDir(), "plain-file")
	if err := os.WriteFile(filePath, []byte("x"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	node := &CodexAgentNode{}
	_, err := node.Execute(context.Background(), "task", map[string]string{
		"binary": bin,
		"cwd":    filePath,
	})
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error = %v, want 'not a directory'", err)
	}
}

func TestCodexAgentNode_Registered(t *testing.T) {
	reg := NewRegistry()
	RegisterBuiltins(reg)
	if _, ok := reg.Get("codex_agent"); !ok {
		t.Error("codex_agent not registered via RegisterBuiltins")
	}
	if _, ok := Get("codex_agent"); !ok {
		t.Error("codex_agent not registered in global registry")
	}
}
