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

package agent

// CapabilityPresets defines named capability bundles for common scenarios
// (断点14: Agent 模式概念过重). Instead of requiring users to understand and
// manually combine individual capabilities (reflection/utility/...), we
// offer curated presets that match real-world usage patterns.
//
// Users can still use --custom -c reflection,utility for full control.
var CapabilityPresets = map[string]CapabilityPreset{
	"smart": {
		Name:         "smart",
		Description:  "智能模式：反思 + 长期记忆，适合复杂推理任务",
		Capabilities: []string{"reflection", "memory"},
	},
	"careful": {
		Name:         "careful",
		Description:  "谨慎模式：人工介入 + 规划 + 反思，适合有风险的操作",
		Capabilities: []string{"human-in-loop", "planning", "reflection"},
	},
}

// CapabilityPreset is a named bundle of capabilities.
type CapabilityPreset struct {
	Name         string
	Description  string
	Capabilities []string
}

// ResolvePreset returns the capability list for a preset name, or nil if the
// name is not a known preset.
func ResolvePreset(name string) []string {
	preset, ok := CapabilityPresets[name]
	if !ok {
		return nil
	}
	// Return a copy to prevent callers from mutating the package-level slice.
	out := make([]string, len(preset.Capabilities))
	copy(out, preset.Capabilities)
	return out
}

// IsPreset reports whether name is a known capability preset.
func IsPreset(name string) bool {
	_, ok := CapabilityPresets[name]
	return ok
}
