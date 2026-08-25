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
	"strings"
	"testing"
)

// withTestLoader swaps the registry's config loader and forces a reload.
func withTestLoader(t *testing.T, defs map[string]AgentDef) {
	t.Helper()
	prev := globalRegistry.loader
	SetLoader(func() map[string]AgentDef { return defs })
	t.Cleanup(func() {
		SetLoader(prev)
	})
}

func TestRegistry_BuiltinPresets(t *testing.T) {
	withTestLoader(t, nil)

	for _, name := range []string{"codex", "claude", "gemini"} {
		def, ok := Get(name)
		if !ok {
			t.Fatalf("builtin agent %q missing from registry", name)
		}
		if def.Driver != DriverCLI {
			t.Errorf("agent %q: driver = %q, want cli", name, def.Driver)
		}
		if def.Binary == "" {
			t.Errorf("agent %q: builtin CLI agent has empty binary", name)
		}
		if def.Description == "" {
			t.Errorf("agent %q: builtin agent has empty description", name)
		}
	}
}

func TestRegistry_ListSorted(t *testing.T) {
	withTestLoader(t, nil)

	agents := List()
	if len(agents) < 3 {
		t.Fatalf("List returned %d agents, want >= 3 builtins", len(agents))
	}
	for i := 1; i < len(agents); i++ {
		if agents[i-1].Name > agents[i].Name {
			t.Errorf("List not sorted: %q before %q", agents[i-1].Name, agents[i].Name)
		}
	}
}

func TestRegistry_ConfigOverridesBuiltin(t *testing.T) {
	withTestLoader(t, map[string]AgentDef{
		"codex": {Binary: "/opt/codex/bin/codex", Model: "gpt-test"},
	})

	def, ok := Get("codex")
	if !ok {
		t.Fatal("codex missing after config override")
	}
	// Explicit override wins.
	if def.Binary != "/opt/codex/bin/codex" {
		t.Errorf("Binary = %q, want /opt/codex/bin/codex", def.Binary)
	}
	if def.Model != "gpt-test" {
		t.Errorf("Model = %q, want gpt-test", def.Model)
	}
	// Unset fields inherit from the preset.
	if def.Profile != "codex" {
		t.Errorf("Profile = %q, want inherited codex", def.Profile)
	}
	if def.Sandbox != "strict" {
		t.Errorf("Sandbox = %q, want inherited strict", def.Sandbox)
	}
}

func TestRegistry_CustomA2AAgent(t *testing.T) {
	withTestLoader(t, map[string]AgentDef{
		"researcher": {Driver: DriverA2A, URL: "http://127.0.0.1:9999/"},
	})

	def, ok := Get("researcher")
	if !ok {
		t.Fatal("custom a2a agent missing")
	}
	if def.Driver != DriverA2A || def.URL != "http://127.0.0.1:9999/" {
		t.Errorf("unexpected def: %+v", def)
	}
}

func TestAgentDef_Resolve(t *testing.T) {
	enabled := true
	disabled := false

	tests := []struct {
		name    string
		def     AgentDef
		wantErr string
	}{
		{name: "valid cli", def: AgentDef{Name: "a", Driver: DriverCLI, Binary: "codex", Profile: "codex", Enabled: &enabled}},
		{name: "cli defaults to generic profile", def: AgentDef{Name: "a", Driver: DriverCLI, Binary: "tool"}},
		{name: "cli without binary", def: AgentDef{Name: "a", Driver: DriverCLI}, wantErr: "requires binary"},
		{name: "cli unknown profile", def: AgentDef{Name: "a", Driver: DriverCLI, Binary: "x", Profile: "warp"}, wantErr: "unknown cli profile"},
		{name: "cli args with prompt placeholder", def: AgentDef{Name: "a", Driver: DriverCLI, Binary: "x", Args: []string{"{prompt}"}, Profile: "generic"}, wantErr: "{prompt} placeholder"},
		{name: "valid a2a", def: AgentDef{Name: "r", Driver: DriverA2A, URL: "http://example.com/"}},
		{name: "a2a without url", def: AgentDef{Name: "r", Driver: DriverA2A}, wantErr: "requires url"},
		{name: "unknown driver", def: AgentDef{Name: "x", Driver: "carrier-pigeon"}, wantErr: "unknown driver"},
		{name: "disabled", def: AgentDef{Name: "a", Driver: DriverCLI, Binary: "x", Enabled: &disabled}, wantErr: "disabled"},
		{name: "no name", def: AgentDef{Driver: DriverCLI, Binary: "x"}, wantErr: "no name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.def.Resolve()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Resolve() unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Resolve() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestTask_ResolveTimeout(t *testing.T) {
	tests := []struct {
		in   Task
		want string
	}{
		{Task{}, DefaultTimeout.String()},
		{Task{Timeout: -1}, DefaultTimeout.String()},
		{Task{Timeout: MaxTimeout * 2}, MaxTimeout.String()},
		{Task{Timeout: 30_000_000_000}, "30s"},
	}
	for _, tt := range tests {
		if got := tt.in.resolveTimeout().String(); got != tt.want {
			t.Errorf("resolveTimeout(%v) = %s, want %s", tt.in.Timeout, got, tt.want)
		}
	}
}
