// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​‌‌​‌​​‌‌​​‌​​‌‌​‌​‌​‌​‌​​‌‌‌‌​‌‌‌​‌​​‌‌​​‌‌​​‌​‌​​‌‌‌‌​‌​‌‌​​​‌‌​​​​​​​​​​​​​​​​​​​​​‌​‌‌​​‌‌​​‌​⁠
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
	"fmt"
	"sort"
	"strings"
	"sync"
)

// AgentDef describes one commandable external agent. Definitions come from
// built-in presets (overridable) and from the `agents:` section of the
// aflare config file.
type AgentDef struct {
	// Name is the registry key used from workflows (`@name` in the
	// supervisor, `agent` param on cli_agent/a2a_agent nodes).
	Name string `yaml:"-"`

	// Driver selects the interop channel: "cli" or "a2a".
	Driver DriverKind `yaml:"driver"`

	// Description is surfaced in `aflare agent list` and in supervisor
	// delegation prompts so the planning LLM knows what each agent does.
	Description string `yaml:"description,omitempty"`

	// Profile selects a built-in CLI profile (codex / claude / gemini /
	// generic). CLI channel only.
	Profile string `yaml:"profile,omitempty"`

	// Binary is the CLI executable (bare name or absolute path).
	Binary string `yaml:"binary,omitempty"`

	// Args are extra literal argv elements for the generic CLI profile.
	// They never contain the prompt; the prompt is appended as a single
	// final argv element.
	Args []string `yaml:"args,omitempty"`

	// URL is the A2A service endpoint (A2A channel only).
	URL string `yaml:"url,omitempty"`

	// APIKeyEnv names an environment variable holding the A2A bearer
	// token. The key itself is never stored in the registry.
	APIKeyEnv string `yaml:"api_key_env,omitempty"`

	// Model, Sandbox and Approval are defaults applied when a Task omits
	// them.
	Model    string `yaml:"model,omitempty"`
	Sandbox  string `yaml:"sandbox,omitempty"`
	Approval string `yaml:"approval,omitempty"`

	// Enabled toggles the agent without deleting its definition.
	Enabled *bool `yaml:"enabled,omitempty"`
}

// builtinPresets are the agents aflare can command out of the box, as
// long as the corresponding CLI is installed. Users override any of them
// via the `agents:` config section.
func builtinPresets() map[string]AgentDef {
	enabled := true
	return map[string]AgentDef{
		"codex": {
			Name:        "codex",
			Driver:      DriverCLI,
			Profile:     "codex",
			Binary:      "codex",
			Description: "OpenAI Codex CLI agent (codex exec, non-interactive)",
			Sandbox:     "strict",
			Approval:    "never",
			Enabled:     &enabled,
		},
		"claude": {
			Name:        "claude",
			Driver:      DriverCLI,
			Profile:     "claude",
			Binary:      "claude",
			Description: "Anthropic Claude Code CLI agent (headless -p mode)",
			Sandbox:     "default",
			Approval:    "never",
			Enabled:     &enabled,
		},
		"gemini": {
			Name:        "gemini",
			Driver:      DriverCLI,
			Profile:     "gemini",
			Binary:      "gemini",
			Description: "Google Gemini CLI agent (non-interactive -p mode)",
			Enabled:     &enabled,
		},
	}
}

// Registry resolves agent definitions from built-in presets overlaid with
// user config. It is safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	agents  map[string]AgentDef
	loaded  bool
	loader  func() map[string]AgentDef
	loadErr error
}

// globalRegistry is the process-wide registry. The loader hook is set by
// the config wiring (see LoadFrom) so this package never imports config
// (avoids an import cycle through nodes).
var globalRegistry = &Registry{
	loader: func() map[string]AgentDef { return nil },
}

// SetLoader installs the config-backed agent-definition source. It is
// called once during CLI/workflow bootstrap; passing nil resets to the
// no-config default. Safe to call before first use.
func SetLoader(loader func() map[string]AgentDef) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.loader = loader
	// Force a reload on next access so a late SetLoader is honored.
	globalRegistry.loaded = false
	globalRegistry.loadErr = nil
}

func (r *Registry) ensureLoaded() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loaded {
		return
	}
	agents := builtinPresets()
	if r.loader != nil {
		for name, def := range r.loader() {
			if name == "" {
				continue
			}
			// User config wins over presets; inherit sensible unset
			// fields from the preset being overridden.
			if preset, ok := agents[name]; ok {
				if def.Driver == "" {
					def.Driver = preset.Driver
				}
				if def.Profile == "" {
					def.Profile = preset.Profile
				}
				if def.Binary == "" {
					def.Binary = preset.Binary
				}
				if def.Description == "" {
					def.Description = preset.Description
				}
				if def.Sandbox == "" {
					def.Sandbox = preset.Sandbox
				}
				if def.Approval == "" {
					def.Approval = preset.Approval
				}
			}
			def.Name = name
			if def.Driver == "" {
				def.Driver = DriverCLI
			}
			agents[name] = def
		}
	}
	r.agents = agents
	r.loaded = true
}

// Get returns the definition for name.
func Get(name string) (AgentDef, bool) {
	globalRegistry.ensureLoaded()
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	def, ok := globalRegistry.agents[name]
	return def, ok
}

// List returns all registered agent definitions sorted by name.
func List() []AgentDef {
	globalRegistry.ensureLoaded()
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	out := make([]AgentDef, 0, len(globalRegistry.agents))
	for _, def := range globalRegistry.agents {
		out = append(out, def)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Resolve validates that def is usable for its driver kind and applies
// per-kind defaults.
func (def AgentDef) Resolve() (AgentDef, error) {
	if def.Name == "" {
		return def, fmt.Errorf("agent definition has no name")
	}
	if def.Enabled != nil && !*def.Enabled {
		return def, fmt.Errorf("agent %q is disabled", def.Name)
	}
	switch def.Driver {
	case DriverCLI:
		if def.Binary == "" {
			return def, fmt.Errorf("agent %q: cli driver requires binary", def.Name)
		}
		if def.Profile == "" {
			def.Profile = "generic"
		}
		if !validProfiles[def.Profile] {
			return def, fmt.Errorf("agent %q: unknown cli profile %q", def.Name, def.Profile)
		}
		for _, arg := range def.Args {
			// Literal argv elements only; reject flag-looking or
			// prompt-placeholder entries that could confuse the
			// generic profile's argv contract.
			if strings.Contains(arg, "{prompt}") {
				return def, fmt.Errorf("agent %q: args must not contain the {prompt} placeholder; the prompt is appended automatically", def.Name)
			}
		}
	case DriverA2A:
		if def.URL == "" {
			return def, fmt.Errorf("agent %q: a2a driver requires url", def.Name)
		}
	default:
		return def, fmt.Errorf("agent %q: unknown driver %q (want cli or a2a)", def.Name, def.Driver)
	}
	return def, nil
}
