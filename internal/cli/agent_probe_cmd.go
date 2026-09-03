// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌​​‌‌​​​​​‌‌​​‌​​​‌​‌‌‌​‌‌​​‌​‌​​​​‌‌‌​​‌‌​‌‌​‌​‌‌​​‌​​‌​‌​​‌‌‌‌​​​​​​​​​​​​​​​​​‌​‌‌​‌‌​‌​‌‌‌​​‌⁠
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​‌‌‌‌‌​‌‌​‌​‌​​​‌‌‌​​‌‌‌​​‌‌​‌​‌‌​​‌​​‌​‌‌‌​‌​​‌​​‌​​‌‌​​‌‌​‌​​​‌​​‌‌​​‌​​​‌​‌​​‌‌​‌​​‌​‌​‌​​‌​​​​‌​​‌​​​​​​​‌​​​​‌​​
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
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/agentx"
)

// probeTimeout bounds each single probe (A2A card fetch / config
// resolution). CLI probes are a local LookPath and return instantly.
const probeTimeout = 10 * time.Second

// probeAgents verifies that registered agents are actually usable before
// a workflow delegates to them: CLI binaries must resolve in PATH, A2A
// endpoints must serve an agent card. Invoked as `aflare agent probe
// [name...]` (all agents when no name is given); exits non-zero when any
// probed agent is not ready, so it can gate CI or pre-flight checks.
func probeAgents(names []string) error {
	agents := agentx.List()
	if len(agents) == 0 {
		fmt.Println("No agents registered. Add entries under `agents:` in your aflare config.")
		return nil
	}

	if len(names) > 0 {
		byName := make(map[string]agentx.AgentDef, len(agents))
		for _, def := range agents {
			byName[def.Name] = def
		}
		selected := make([]agentx.AgentDef, 0, len(names))
		for _, name := range names {
			def, ok := byName[strings.TrimSpace(name)]
			if !ok {
				return fmt.Errorf("agent %q is not registered (see `aflare agent list`)", name)
			}
			selected = append(selected, def)
		}
		agents = selected
	}

	fmt.Printf("Probing %d agent(s)...\n\n", len(agents))

	failed := 0
	for _, def := range agents {
		ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
		detail, err := probeOne(ctx, def)
		cancel()
		state := "OK"
		if err != nil {
			state = "FAIL"
			failed++
		}
		fmt.Printf("  %-14s %-5s %-5s %s\n", def.Name, string(def.Driver), state, detail)
	}

	fmt.Println()
	if failed > 0 {
		fmt.Printf("%d/%d agent(s) not ready.\n", failed, len(agents))
		return exitErr(1)
	}
	fmt.Printf("All %d agent(s) ready.\n", len(agents))
	return nil
}

// probeOne checks a single agent and returns a human-readable detail
// line; a non-nil error marks the agent not ready.
func probeOne(ctx context.Context, def agentx.AgentDef) (string, error) {
	resolved, err := def.Resolve()
	if err != nil {
		return err.Error(), err
	}
	switch resolved.Driver {
	case agentx.DriverCLI:
		path, err := exec.LookPath(resolved.Binary)
		if err != nil {
			return fmt.Sprintf("binary %q not found in PATH (install it or fix `binary:` in config)", resolved.Binary), err
		}
		return fmt.Sprintf("%s (profile %s)", path, resolved.Profile), nil
	case agentx.DriverA2A:
		card, err := agentx.FetchAgentCard(ctx, resolved)
		if err != nil {
			return fmt.Sprintf("agent card unreachable: %v", err), err
		}
		name := card.Name
		if name == "" {
			name = "(unnamed)"
		}
		detail := fmt.Sprintf("%q", name)
		if card.Version != "" {
			detail += " v" + card.Version
		}
		if n := len(card.Skills); n > 0 {
			detail += fmt.Sprintf(" (%d skills)", n)
		}
		return detail, nil
	default:
		return fmt.Sprintf("unknown driver %q", resolved.Driver), fmt.Errorf("unknown driver %q", resolved.Driver)
	}
}
