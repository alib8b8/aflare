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

package cli

import (
	"fmt"
	"strings"

	"github.com/alib8b8/aflare/internal/i18n"
	"github.com/alib8b8/aflare/internal/nodes"
)

// HandleList handles the "list" command.
func HandleList() {
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	nodeList := reg.ListNodes()
	if len(nodeList) == 0 {
		fmt.Println(i18n.T("list.empty"))
		return
	}

	fmt.Println(i18n.T("list.title"))
	fmt.Println("-" + strings.Repeat("-", 78))
	fmt.Printf("  %-20s %s\n", i18n.T("table.name"), i18n.T("table.description"))
	fmt.Println("-" + strings.Repeat("-", 78))
	for _, info := range nodeList {
		fmt.Printf("  %-20s %s\n", info.Name, info.Description)
	}

	installed, err := ListInstalledNodes()
	if err == nil && len(installed) > 0 {
		fmt.Printf("\n%s\n", i18n.T("list.installed"))
		for _, name := range installed {
			fmt.Printf("  - %s\n", name)
		}
	}
}
