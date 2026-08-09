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
	"strings"

	"github.com/alib8b8/aflare/internal/history"
)

// HandleAudit handles the "audit" command.
func HandleAudit(args []string) {
	if len(args) == 0 {
		PrintAuditUsage()
		os.Exit(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "verify":
		HandleAuditVerify(args[1:])
	case "-h", "--help", "help":
		PrintAuditUsage()
	default:
		fmt.Printf("Unknown audit subcommand: %s\n\n", subCmd)
		PrintAuditUsage()
		os.Exit(1)
	}
}

// HandleAuditVerify handles the "audit verify" subcommand.
func HandleAuditVerify(args []string) {
	auditPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file", "-f":
			if i+1 < len(args) {
				auditPath = args[i+1]
				i++
			} else {
				fmt.Println("❌ --file requires a value")
				os.Exit(1)
			}
		case "--help", "-h":
			fmt.Println("Usage: aflare audit verify [--file <path>]")
			return
		default:
			if strings.HasPrefix(args[i], "--file=") {
				auditPath = strings.TrimPrefix(args[i], "--file=")
			} else {
				fmt.Printf("❌ Unknown argument: %s\n", args[i])
				os.Exit(1)
			}
		}
	}

	if auditPath == "" {
		auditPath = history.GetAuditLogPath()
		if auditPath == "" {
			fmt.Println("❌ Could not resolve audit log path. Specify one with --file.")
			os.Exit(1)
		}
	}

	valid, brokenAt, err := history.VerifyAuditChain(auditPath)
	if err != nil {
		fmt.Printf("❌ Audit log verification error in %s: %v\n", auditPath, err)
		os.Exit(1)
	}
	if valid {
		fmt.Printf("✅ Audit log chain is valid: %s\n", auditPath)
		return
	}
	fmt.Printf("❌ Audit log chain is BROKEN at line %d: %s\n", brokenAt, auditPath)
	os.Exit(1)
}

// PrintAuditUsage prints usage information for the audit command.
func PrintAuditUsage() {
	fmt.Println("Usage: aflare audit <command> [options]")
	fmt.Println("\nVerify the integrity of the tamper-evident audit log hash chain.")
	fmt.Println("\nCommands:")
	fmt.Println("  verify [--file <path>]   Verify the audit log HMAC hash chain")
	fmt.Println("  -h, --help               Show this help message")
	fmt.Println("\nOptions:")
	fmt.Println("  --file, -f <path>   Path to the audit log file (defaults to the standard location)")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare audit verify")
	fmt.Println("  aflare audit verify --file /path/to/audit.log.jsonl")
}
