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
	"os"

	"github.com/alib8b8/aflare/internal/i18n"
)

// HandleInstall handles the "install" command.
func HandleInstall(args []string) {
	if len(args) < 1 {
		fmt.Println(i18n.T("install.usage"))
		fmt.Printf("\n%s\n", i18n.T("install.use_list"))
		os.Exit(1)
	}

	nodeName := args[0]
	if err := InstallNode(nodeName); err != nil {
		fmt.Printf("❌ %s\n", i18n.T("install.failed", nodeName, err))
		os.Exit(1)
	}

	fmt.Printf("✅ %s\n", i18n.T("install.success", nodeName))
	fmt.Printf("\n%s\n", i18n.T("install.usage_hint"))
	fmt.Printf("  steps:\n    - node: %s\n", nodeName)
}

// HandleUninstall handles the "uninstall" command.
func HandleUninstall(args []string) {
	if len(args) < 1 {
		fmt.Println(i18n.T("uninstall.usage"))
		os.Exit(1)
	}

	nodeName := args[0]
	if err := UninstallNode(nodeName); err != nil {
		fmt.Printf("❌ %s\n", i18n.T("uninstall.failed", nodeName, err))
		os.Exit(1)
	}

	fmt.Printf("✅ %s\n", i18n.T("uninstall.success", nodeName))
}
