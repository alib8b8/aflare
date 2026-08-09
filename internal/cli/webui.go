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

	"github.com/alib8b8/aflare/internal/webui"
)

// HandleWebUI handles the "webui" command.
func HandleWebUI(args []string) {
	host := ""
	port := ""
	workflowsDir := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--host", "-H":
			if i+1 < len(args) {
				host = args[i+1]
				i++
			}
		case "--port", "-p":
			if i+1 < len(args) {
				port = args[i+1]
				i++
			}
		case "--dir", "-d":
			if i+1 < len(args) {
				workflowsDir = args[i+1]
				i++
			}
		case "--help", "-h":
			PrintWebUIUsage()
			return
		default:
			fmt.Printf("Unknown argument: %s\n", args[i])
			PrintWebUIUsage()
			os.Exit(1)
		}
	}

	server := webui.NewWebUIServer(host, port)
	if workflowsDir != "" {
		server.SetWorkflowsDir(workflowsDir)
	}

	fmt.Printf("Starting WebUI server on http://localhost:%s\n", port)
	fmt.Println("Press Ctrl+C to stop")

	if err := server.Start(); err != nil {
		fmt.Printf("WebUI server error: %v\n", err)
		os.Exit(1)
	}
}

// PrintWebUIUsage prints usage information for the webui command.
func PrintWebUIUsage() {
	fmt.Println("Usage: aflare webui [options]")
	fmt.Println("\nOptions:")
	fmt.Println("  --port, -p <port>    - WebUI server port (default: 8081)")
	fmt.Println("  --dir, -d <dir>      - Workflows directory")
	fmt.Println("  --help, -h           - Show this help")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare webui")
	fmt.Println("  aflare webui --port 8080")
	fmt.Println("  aflare webui --dir /path/to/workflows")
}