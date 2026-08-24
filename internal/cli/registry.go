// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌​​‌​​‌​‌​‌‌​‌‌​​​​​‌‌​‌‌‌​‌​​​‌‌‌‌‌​​‌​‌‌‌‌‌‌​​​​​​​​​​​​​​​​‌‌​​‌‌​‌​‌‌​‌‌‌​⁠
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
)

// HandleRegistry handles the "registry" command.
func HandleRegistry(args []string) error {
	if len(args) < 1 {
		fmt.Println(i18n.T("registry.usage"))
		fmt.Println("\nCommands:")
		fmt.Println("  sync     - aflare registry sync")
		fmt.Println("  list     - aflare registry list")
		fmt.Println("  search   - aflare registry search <query>")
		return exitErr(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "sync":
		if err := SyncRegistry(); err != nil {
			fmt.Printf("❌ %s\n", i18n.T("registry.sync_failed", err))
			return exitErr(1)
		}
		fmt.Printf("✅ %s\n", i18n.T("registry.sync_success"))

	case "list":
		nodes, err := ListRegistryNodes()
		if err != nil {
			fmt.Printf("❌ %s\n", i18n.T("registry.list_failed", err))
			fmt.Printf("\n%s\n", i18n.T("registry.sync_hint"))
			return exitErr(1)
		}

		if len(nodes) == 0 {
			fmt.Println(i18n.T("registry.empty"))
			return nil
		}

		fmt.Println(i18n.T("registry.list_title"))
		fmt.Println("-" + strings.Repeat("-", 78))
		fmt.Printf("  %-15s %-40s %-10s %s\n", i18n.T("table.name"), i18n.T("table.description"), i18n.T("table.version"), i18n.T("table.category"))
		fmt.Println("-" + strings.Repeat("-", 78))
		for _, node := range nodes {
			fmt.Printf("  %-15s %-40s %-10s %s\n", node.Name, truncate(node.Description, 38), node.Version, node.Category)
		}

	case "search":
		if len(args) < 2 {
			fmt.Println(i18n.T("registry.search_usage"))
			return exitErr(1)
		}

		query := strings.Join(args[1:], " ")
		nodes, err := SearchRegistryNodes(query)
		if err != nil {
			fmt.Printf("❌ %s\n", i18n.T("registry.list_failed", err))
			return exitErr(1)
		}

		if len(nodes) == 0 {
			fmt.Printf("%s\n", i18n.T("registry.no_match", query))
			return nil
		}

		fmt.Println(i18n.T("registry.search_result", len(nodes), query))
		fmt.Println("-" + strings.Repeat("-", 78))
		fmt.Printf("  %-15s %-40s %-10s %s\n", i18n.T("table.name"), i18n.T("table.description"), i18n.T("table.version"), i18n.T("table.category"))
		fmt.Println("-" + strings.Repeat("-", 78))
		for _, node := range nodes {
			fmt.Printf("  %-15s %-40s %-10s %s\n", node.Name, truncate(node.Description, 38), node.Version, node.Category)
		}

	default:
		fmt.Printf("%s\n", i18n.T("registry.unknown_cmd", subCmd))
		return exitErr(1)
	}
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 2 {
		return s[:maxLen]
	}
	return s[:maxLen-2] + ".."
}
