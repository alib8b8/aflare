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
)

// HandleSecrets handles the "secrets" command.
func HandleSecrets(args []string) {
	if len(args) == 0 {
		PrintSecretsUsage()
		os.Exit(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "-h", "--help", "help":
		PrintSecretsUsage()
	default:
		fmt.Printf("Unknown secrets subcommand: %s\n\n", subCmd)
		PrintSecretsUsage()
		os.Exit(1)
	}
}

// PrintSecretsUsage prints usage information for the secrets command.
func PrintSecretsUsage() {
	fmt.Println("Usage: aflare secrets <command> [options]")
	fmt.Println("\nManage secrets and credentials for workflows.")
	fmt.Println("\nCommands:")
	fmt.Println("  -h, --help   Show this help message")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare secrets --help")
}
