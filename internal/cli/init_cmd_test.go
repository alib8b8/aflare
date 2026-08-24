// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​‌‌​‌​‌‌‌‌​​​​​‌​​​‌‌‌‌​​‌‌​‌​‌‌‌‌‌​​​​‌​‌​‌​‌​‌​​​​​​​​​​​​​​​​​​‌‌‌‌​​​‌‌​​‌‌​​⁠
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
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/config"
)

// injectLLMConfig seeds the package-level config cache with a configured
// ollama provider so HandleInit's trailing offerLLMConfig short-circuits
// (detectLLMConfig → true) instead of reading stdin.
func injectLLMConfig(t *testing.T) {
	t.Helper()
	config.SetConfig(&config.Config{
		Providers: map[string]config.LLMProviderConfig{
			"ollama": {Model: "llama3", Endpoint: "http://localhost:11434"},
		},
	})
	t.Cleanup(func() { config.SetConfig(nil) })
}

func TestInitCmdMCPQuickUnsupportedAgent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var err error
	output := captureOutput(func() {
		err = HandleInitMCPQuick("unsupported")
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleInitMCPQuick(unsupported) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(output, "MCP setup failed") {
		t.Errorf("expected MCP setup failure message, got: %s", output)
	}
}

func TestInitCmdAgentQuickUnsupportedAgent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var err error
	output := captureOutput(func() {
		err = HandleInitAgentQuick("unsupported")
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleInitAgentQuick(unsupported) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(output, "Skill installation failed") {
		t.Errorf("expected skill installation failure message, got: %s", output)
	}
}

func TestInitCmdChannelQuickInvalid(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var err error
	output := captureOutput(func() {
		err = HandleChannelQuick("bogus")
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleChannelQuick(bogus) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(output, "Failed to set channel") {
		t.Errorf("expected channel failure message, got: %s", output)
	}
}

func TestInitCmdQuickHappyPaths(t *testing.T) {
	t.Run("mcp quick", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		var err error
		output := captureOutput(func() {
			err = HandleInitMCPQuick("claude-code")
		})
		if err != nil {
			t.Fatalf("HandleInitMCPQuick(claude-code) = %v, want nil", err)
		}
		if !strings.Contains(output, "MCP configured for Claude Code") {
			t.Errorf("expected MCP confirmation, got: %s", output)
		}
		if _, serr := os.Stat(filepath.Join(home, ".claude", "claude_desktop_config.json")); serr != nil {
			t.Errorf("claude code MCP config not written: %v", serr)
		}
	})
	t.Run("agent quick", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		var err error
		output := captureOutput(func() {
			err = HandleInitAgentQuick("claude-code")
		})
		if err != nil {
			t.Fatalf("HandleInitAgentQuick(claude-code) = %v, want nil", err)
		}
		if !strings.Contains(output, "Skills installed for Claude Code") {
			t.Errorf("expected skills confirmation, got: %s", output)
		}
		// No skills/templates source in the test cwd → directory-ready status.
		if !strings.Contains(output, "skills directory ready") {
			t.Errorf("expected directory-ready status, got: %s", output)
		}
		if _, serr := os.Stat(filepath.Join(home, ".claude", "skills")); serr != nil {
			t.Errorf("skills directory not created: %v", serr)
		}
	})
}

func TestInitCmdChannelQuickValid(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var err error
	output := captureOutput(func() {
		err = HandleChannelQuick("beta")
	})
	if err != nil {
		t.Fatalf("HandleChannelQuick(beta) = %v, want nil", err)
	}
	if !strings.Contains(output, "Update channel set to: beta") {
		t.Errorf("expected channel confirmation, got: %s", output)
	}

	data, rerr := os.ReadFile(filepath.Join(home, ".config", "aflare", "autoupgrade.yaml"))
	if rerr != nil {
		t.Fatalf("autoupgrade config not written: %v", rerr)
	}
	if !strings.Contains(string(data), "channel: beta") {
		t.Errorf("autoupgrade config = %s, want it to contain %q", data, "channel: beta")
	}
}

func TestInitCmdHelp(t *testing.T) {
	for _, arg := range []string{"--help", "-h"} {
		var err error
		output := captureOutput(func() {
			err = HandleInit([]string{arg})
		})
		if err != nil {
			t.Errorf("HandleInit([%q]) = %v, want nil", arg, err)
		}
		if !strings.Contains(output, "Usage: aflare init") {
			t.Errorf("expected usage output for %q, got: %s", arg, output)
		}
	}
}

func TestInitCmdFlagErrorPaths(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{"mcp equals form", []string{"--mcp=unsupported"}, "MCP setup failed"},
		{"mcp space form", []string{"--mcp", "unsupported"}, "MCP setup failed"},
		{"agent equals form", []string{"--agent=unsupported"}, "Skill installation failed"},
		{"agent space form", []string{"--agent", "unsupported"}, "Skill installation failed"},
		{"channel equals form", []string{"--channel=bogus"}, "Failed to set channel"},
		{"channel space form", []string{"--channel", "bogus"}, "Failed to set channel"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			var err error
			output := captureOutput(func() {
				err = HandleInit(tt.args)
			})
			if code := exitCodeForErr(err); code != 1 {
				t.Errorf("HandleInit(%v) exit code = %d, want 1 (err=%v)", tt.args, code, err)
			}
			if !strings.Contains(output, tt.wantMsg) {
				t.Errorf("HandleInit(%v) output missing %q, got: %s", tt.args, tt.wantMsg, output)
			}
		})
	}
}

func TestInitCmdMCPAndAgentFlags(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	injectLLMConfig(t)

	var err error
	output := captureOutput(func() {
		err = HandleInit([]string{"--mcp=claude-code", "--agent=claude-code"})
	})
	if err != nil {
		t.Fatalf("HandleInit([--mcp=claude-code --agent=claude-code]) = %v, want nil", err)
	}
	if !strings.Contains(output, "MCP configured for Claude Code") {
		t.Errorf("expected MCP confirmation, got: %s", output)
	}
	if !strings.Contains(output, "Skills installed for Claude Code") {
		t.Errorf("expected skills confirmation, got: %s", output)
	}

	// MCP config file written under the temp HOME.
	if _, serr := os.Stat(filepath.Join(home, ".claude", "claude_desktop_config.json")); serr != nil {
		t.Errorf("claude code MCP config not written: %v", serr)
	}
	// Skills directory created (no source templates in the test cwd →
	// "directory ready" status, which is still success).
	if _, serr := os.Stat(filepath.Join(home, ".claude", "skills")); serr != nil {
		t.Errorf("skills directory not created: %v", serr)
	}
}

func TestInitCmdChannelFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	injectLLMConfig(t)

	var err error
	output := captureOutput(func() {
		err = HandleInit([]string{"--channel=beta"})
	})
	if err != nil {
		t.Fatalf("HandleInit([--channel=beta]) = %v, want nil", err)
	}
	if !strings.Contains(output, "Update channel set to: beta") {
		t.Errorf("expected channel confirmation, got: %s", output)
	}

	data, rerr := os.ReadFile(filepath.Join(home, ".config", "aflare", "autoupgrade.yaml"))
	if rerr != nil {
		t.Fatalf("autoupgrade config not written: %v", rerr)
	}
	if !strings.Contains(string(data), "channel: beta") {
		t.Errorf("autoupgrade config = %s, want it to contain %q", data, "channel: beta")
	}
}

func TestInitCmdBareFlags(t *testing.T) {
	t.Run("bare --mcp means all", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		injectLLMConfig(t)

		var err error
		captureOutput(func() {
			err = HandleInit([]string{"--mcp"})
		})
		if err != nil {
			t.Fatalf("HandleInit([--mcp]) = %v, want nil", err)
		}
		if _, serr := os.Stat(filepath.Join(home, ".claude", "claude_desktop_config.json")); serr != nil {
			t.Errorf("claude code MCP config not written: %v", serr)
		}
		if _, serr := os.Stat(filepath.Join(home, ".config", "opencode", "User", "mcp.json")); serr != nil {
			t.Errorf("opencode MCP config not written: %v", serr)
		}
	})
	t.Run("bare --agent means all", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		injectLLMConfig(t)

		var err error
		captureOutput(func() {
			err = HandleInit([]string{"--agent"})
		})
		if err != nil {
			t.Fatalf("HandleInit([--agent]) = %v, want nil", err)
		}
		if _, serr := os.Stat(filepath.Join(home, ".claude", "skills")); serr != nil {
			t.Errorf("claude code skills directory not created: %v", serr)
		}
		if _, serr := os.Stat(filepath.Join(home, ".config", "opencode", "User", "skills")); serr != nil {
			t.Errorf("opencode skills directory not created: %v", serr)
		}
	})
}

func TestInitCmdPrintUsage(t *testing.T) {
	output := captureOutput(func() {
		PrintInitUsage()
	})
	if !strings.Contains(output, "Usage: aflare init") {
		t.Errorf("expected usage header, got: %s", output)
	}
	for _, flag := range []string{"--mcp", "--agent", "--channel"} {
		if !strings.Contains(output, flag) {
			t.Errorf("usage output missing %q, got: %s", flag, output)
		}
	}
}
