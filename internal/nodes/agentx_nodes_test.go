// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​‌‌​‌​​‌‌​​‌​​‌‌​‌‌‌‌‌​‌​‌​‌‌​‌​​‌‌‌‌‌​‌​‌​​‌​‌‌‌‌‌‌‌‌‌‌‌‌‌‌‌​‌​‌‌​​​​​​​​​​​​​​​​​‌​​​​​‌​​‌​‌‌​​⁠
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

package nodes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/agentx"
)

func TestCLIAgentNode_SafeModeDisabled(t *testing.T) {
	SetSafeMode(true)
	t.Cleanup(func() { SetSafeMode(false) })

	node := &CLIAgentNode{}
	_, err := node.Execute(context.Background(), "do things", map[string]string{
		"agent": "codex",
	})
	if err == nil || !strings.Contains(err.Error(), "safe mode") {
		t.Fatalf("err = %v, want safe mode rejection", err)
	}
}

func TestCLIAgentNode_UnknownAgent(t *testing.T) {
	registerTestAgents(t, nil)

	node := &CLIAgentNode{}
	_, err := node.Execute(context.Background(), "do things", map[string]string{
		"agent": "ghost",
	})
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("err = %v, want unknown agent rejection", err)
	}
}

func TestCLIAgentNode_MissingBinaryAndAgent(t *testing.T) {
	registerTestAgents(t, nil)

	node := &CLIAgentNode{}
	_, err := node.Execute(context.Background(), "do things", nil)
	if err == nil || !strings.Contains(err.Error(), "binary") {
		t.Fatalf("err = %v, want binary requirement", err)
	}
}

func TestCLIAgentNode_InlineExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake agent is POSIX-only")
	}
	registerTestAgents(t, nil)

	node := &CLIAgentNode{}
	out, err := node.Execute(context.Background(), "hello there", map[string]string{
		"binary":  fakeCLIAgent(t, "INLINE"),
		"profile": "generic",
	})
	if err != nil {
		t.Fatalf("cli_agent inline: %v", err)
	}
	if !strings.Contains(out, "INLINE") || !strings.Contains(out, "hello there") {
		t.Errorf("out = %q, want inline agent echo", out)
	}
}

func TestCLIAgentNode_RegistryAgentWithOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake agent is POSIX-only")
	}
	registerTestAgents(t, map[string]agentx.AgentDef{
		// Registry binary does not exist; the per-step override replaces
		// it with the fake agent.
		"stubborn": {Driver: agentx.DriverCLI, Profile: "generic", Binary: "/nonexistent/aflare-stubborn"},
	})

	node := &CLIAgentNode{}
	out, err := node.Execute(context.Background(), "via registry", map[string]string{
		"agent":  "stubborn",
		"binary": fakeCLIAgent(t, "OVERRIDE"),
	})
	if err != nil {
		t.Fatalf("cli_agent override: %v", err)
	}
	if !strings.Contains(out, "OVERRIDE") || !strings.Contains(out, "via registry") {
		t.Errorf("out = %q, want overridden agent echo", out)
	}
}

func TestA2AAgentNode_SafeModeDisabled(t *testing.T) {
	SetSafeMode(true)
	t.Cleanup(func() { SetSafeMode(false) })

	node := &A2AAgentNode{}
	_, err := node.Execute(context.Background(), "do things", map[string]string{"url": "http://127.0.0.1:1/"})
	if err == nil || !strings.Contains(err.Error(), "safe mode") {
		t.Fatalf("err = %v, want safe mode rejection", err)
	}
}

func TestA2AAgentNode_MissingURLAndAgent(t *testing.T) {
	registerTestAgents(t, nil)

	node := &A2AAgentNode{}
	_, err := node.Execute(context.Background(), "do things", nil)
	if err == nil || !strings.Contains(err.Error(), "url") {
		t.Fatalf("err = %v, want url requirement", err)
	}
}

func TestA2AAgentNode_InlineExecution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"id":"t1","status":{"state":"completed","message":{"role":"agent","parts":[{"kind":"text","text":"a2a node ok"}]}}}}`))
	}))
	t.Cleanup(srv.Close)
	registerTestAgents(t, nil)

	node := &A2AAgentNode{}
	out, err := node.Execute(context.Background(), "hello a2a", map[string]string{
		"url": srv.URL + "/",
	})
	if err != nil {
		t.Fatalf("a2a_agent inline: %v", err)
	}
	if out != "a2a node ok" {
		t.Errorf("out = %q", out)
	}
}

func TestA2AAgentNode_RegistryAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"id":"t1","status":{"state":"completed"},"artifacts":[{"parts":[{"kind":"text","text":"registry a2a ok"}]}]}}`))
	}))
	t.Cleanup(srv.Close)
	registerTestAgents(t, map[string]agentx.AgentDef{
		"remote": {Driver: agentx.DriverA2A, URL: srv.URL + "/"},
	})

	node := &A2AAgentNode{}
	out, err := node.Execute(context.Background(), "hello", map[string]string{
		"agent": "remote",
	})
	if err != nil {
		t.Fatalf("a2a_agent registry: %v", err)
	}
	if !strings.Contains(out, "registry a2a ok") {
		t.Errorf("out = %q", out)
	}
}

func TestResolveAgentDef_OverridesRegistryFields(t *testing.T) {
	registerTestAgents(t, map[string]agentx.AgentDef{
		"mixed": {Driver: agentx.DriverCLI, Profile: "codex", Binary: "codex", Sandbox: "strict"},
	})

	def, err := resolveAgentDef(agentx.DriverCLI, map[string]string{
		"agent":   "mixed",
		"profile": "generic",
		"binary":  "/usr/local/bin/other",
		"model":   "custom-model",
	})
	if err != nil {
		t.Fatalf("resolveAgentDef: %v", err)
	}
	if def.Profile != "generic" || def.Binary != "/usr/local/bin/other" || def.Model != "custom-model" {
		t.Errorf("def = %+v, param overrides not applied", def)
	}
	if def.Sandbox != "strict" {
		t.Errorf("def.Sandbox = %q, want registry default preserved", def.Sandbox)
	}
}

func TestBuildAgentTask_Params(t *testing.T) {
	task := buildAgentTask("the input", map[string]string{
		"model":           "m-1",
		"sandbox":         "permissive",
		"approval_policy": "never",
		"cwd":             "/tmp",
		"max_turns":       "7",
		"timeout":         "45s",
	})
	if task.Prompt != "the input" || task.Model != "m-1" || task.Sandbox != "permissive" || task.Approval != "never" {
		t.Errorf("task = %+v", task)
	}
	if task.MaxTurns != 7 || task.Timeout.String() != "45s" {
		t.Errorf("task = %+v", task)
	}
	// cwd "/tmp" resolves via symlink on some platforms; only require a
	// non-empty canonical value here.
	if task.Cwd == "" {
		t.Error("cwd not resolved")
	}
}

func TestBuildAgentTask_GarbageIgnored(t *testing.T) {
	task := buildAgentTask("in", map[string]string{
		"max_turns": "banana",
		"timeout":   "not-a-duration",
	})
	if task.MaxTurns != 0 || task.Timeout != 0 {
		t.Errorf("task = %+v, garbage should fall back to defaults", task)
	}
}
