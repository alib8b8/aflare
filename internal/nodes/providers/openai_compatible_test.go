// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​​​​‌‌​‌‌​‌​​​​​​​​‌​‌‌​‌‌‌​‌​​​​​​‌‌​​‌‌​​‌‌‌​​​​​​​​​​​​​​​​‌​‌​​‌‌‌‌​‌‌‌‌​‌⁠
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

package providers

import (
	"testing"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

// TestOpenAICompatibleConfigs verifies the exported config table: every
// entry must be registered in the core registry and the returned slice must
// be a defensive copy.
func TestOpenAICompatibleConfigs(t *testing.T) {
	configs := OpenAICompatibleConfigs()
	if len(configs) == 0 {
		t.Fatal("OpenAICompatibleConfigs() is empty")
	}

	for _, cfg := range configs {
		t.Run(cfg.Name, func(t *testing.T) {
			if cfg.Name == "" {
				t.Error("config has empty Name")
			}
			if cfg.ProviderName == "" {
				t.Error("config has empty ProviderName")
			}
			// coze/ima deliberately leave DefaultModel empty (model must
			// be supplied per call); ima also requires IMA_API_BASE.
			if cfg.DefaultModel == "" && cfg.Name != "coze" && cfg.Name != "ima" {
				t.Error("config has empty DefaultModel")
			}
			if cfg.DefaultEndpoint == "" && cfg.Name != "ima" {
				t.Error("config has empty DefaultEndpoint")
			}
			// Every config must correspond to a registered node.
			node, ok := core.Get(cfg.Name)
			if !ok {
				t.Fatalf("node %q from config table not found in registry", cfg.Name)
			}
			if node.Name() != cfg.Name {
				t.Errorf("node Name() = %q, want %q", node.Name(), cfg.Name)
			}
		})
	}

	// Mutating the returned slice must not affect the package table.
	configs[0].DefaultModel = "mutated"
	fresh := OpenAICompatibleConfigs()
	if fresh[0].DefaultModel == "mutated" {
		t.Error("OpenAICompatibleConfigs must return a defensive copy")
	}
}
