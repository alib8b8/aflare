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

	"github.com/alib8b8/aflare/internal/api"
)

// HandleServe handles the "serve" command (HTTP API server mode).
func HandleServe(args []string) {
	host := ""
	port := "8080"
	apiKey := ""
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
		case "--api-key", "-k":
			if i+1 < len(args) {
				apiKey = args[i+1]
				i++
			}
		case "--dir", "-d":
			if i+1 < len(args) {
				workflowsDir = args[i+1]
				i++
			}
		case "--help", "-h":
			PrintServeUsage()
			return
		default:
			fmt.Printf("Unknown argument: %s\n", args[i])
			PrintServeUsage()
			os.Exit(1)
		}
	}

	server := api.NewServer(host, port, apiKey)
	if workflowsDir != "" {
		server.SetWorkflowsDir(workflowsDir)
	}

	fmt.Printf("Starting API server on http://localhost:%s\n", port)
	fmt.Println("Endpoints:")
	fmt.Println("  GET  /health               - Health check")
	fmt.Println("  GET  /api/v1/metrics        - Prometheus metrics")
	fmt.Println("  POST /api/v1/workflows/run  - Run a workflow")
	fmt.Println("  GET  /api/v1/workflows      - List available workflows")
	fmt.Println("  GET  /api/v1/workflows/{name} - Get workflow details")
	fmt.Println("Press Ctrl+C to stop")

	if err := server.Start(); err != nil {
		fmt.Printf("API server error: %v\n", err)
		os.Exit(1)
	}
}

// PrintServeUsage prints usage information for the serve command.
func PrintServeUsage() {
	fmt.Println("Usage: aflare serve [options]")
	fmt.Println("\nOptions:")
	fmt.Println("  --port, -p <port>      - API server port (default: 8080)")
	fmt.Println("  --host, -H <host>      - API server host (default: 0.0.0.0)")
	fmt.Println("  --api-key, -k <key>    - API key for authentication")
	fmt.Println("  --dir, -d <dir>        - Workflows directory")
	fmt.Println("  --help, -h             - Show this help")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare serve")
	fmt.Println("  aflare serve --port 9090")
	fmt.Println("  aflare serve --api-key my-secret-key")
	fmt.Println("  aflare serve --dir /path/to/workflows")
	fmt.Println("  aflare --serve --port 8080")
}
