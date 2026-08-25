// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​‌‌​‌​​‌‌​​‌​​‌‌​‌​​​​​​‌​‌​​​‌‌‌‌​‌‌​​​‌​‌​‌‌‌‌‌​‌‌‌‌​‌​‌​​​‌‌​​​​​​​​​​​​​​​​​​​​‌‌‌‌​​​​​‌​‌​​​⁠
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

package cli

import (
	"fmt"
	"strings"

	"github.com/alib8b8/aflare/internal/agentx"
)

// listAgents prints the external agents aflare can command and
// supervise. Invoked as `aflare agent list` (intercepted before daemon
// flag parsing in HandleAgent).
func listAgents() error {
	agents := agentx.List()
	if len(agents) == 0 {
		fmt.Println("No agents registered. Add entries under `agents:` in your aflare config.")
		return nil
	}

	fmt.Println("External agents aflare can command:")
	fmt.Println(strings.Repeat("-", 78))
	fmt.Printf("  %-14s %-6s %-22s %s\n", "NAME", "DRIVER", "TARGET", "DESCRIPTION")
	fmt.Println(strings.Repeat("-", 78))
	for _, def := range agents {
		target := def.Binary
		if def.Driver == agentx.DriverA2A {
			target = def.URL
		}
		state := ""
		if def.Enabled != nil && !*def.Enabled {
			state = " (disabled)"
		}
		fmt.Printf("  %-14s %-6s %-22s %s%s\n", def.Name, string(def.Driver), truncate(target, 22), truncate(def.Description, 28), state)
	}
	fmt.Println(strings.Repeat("-", 78))
	fmt.Println("Delegate via supervisor specialists: \"@codex,@claude\", or the cli_agent / a2a_agent nodes.")
	return nil
}
