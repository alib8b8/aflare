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

	"github.com/alib8b8/aflare/internal/ai"
)

// HandleReview handles the "review" command.
func HandleReview(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: aflare review <workflow.yaml>")
		fmt.Println("\nAI-powered workflow review — analyzes your workflow YAML and provides")
		fmt.Println("optimization suggestions, security checks, and a natural-language summary.")
		fmt.Println("\nInspired by agno v2.8.7 AdvisorTools: the advisor model reviews the")
		fmt.Println("generated workflow before execution, catching issues early.")
		os.Exit(1)
	}

	const maxFileSize = 10 * 1024 * 1024 // 10 MB
	fi, err := os.Stat(args[0])
	if err != nil {
		fmt.Printf("❌ Failed to stat workflow file: %v\n", err)
		os.Exit(1)
	}
	if fi.Size() > maxFileSize {
		fmt.Printf("❌ Workflow file too large (%d bytes). Max allowed: %d bytes (10 MB).\n", fi.Size(), maxFileSize)
		os.Exit(1)
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Printf("❌ Failed to read workflow file: %v\n", err)
		os.Exit(1)
	}

	workflowYAML := string(data)

	optimizer := ai.NewWorkflowOptimizer()
	report := optimizer.Analyze(workflowYAML)

	explainer := ai.NewWorkflowExplainer()
	explanation := explainer.Explain(workflowYAML)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║           aflare Workflow Advisor Review                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("📋 What this workflow does:")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println(explanation)
	fmt.Println()

	scoreColor := "🟢"
	if report.Score < 70 {
		scoreColor = "🟡"
	}
	if report.Score < 40 {
		scoreColor = "🔴"
	}
	fmt.Printf("%s Optimization Score: %d/100\n", scoreColor, report.Score)
	fmt.Println(strings.Repeat("─", 60))

	if len(report.Suggestions) == 0 {
		fmt.Println("✅ No issues found — workflow looks great!")
	} else {
		var errors, warnings, infos []ai.Suggestion
		for _, s := range report.Suggestions {
			switch s.Severity {
			case ai.SeverityError:
				errors = append(errors, s)
			case ai.SeverityWarning:
				warnings = append(warnings, s)
			case ai.SeverityInfo:
				infos = append(infos, s)
			}
		}

		if len(errors) > 0 {
			fmt.Printf("\n🔴 Errors (%d):\n", len(errors))
			for _, s := range errors {
				fmt.Printf("  ❌ %s\n", s.Message)
				if s.Fix != "" {
					fmt.Printf("     Fix: %s\n", s.Fix)
				}
			}
		}
		if len(warnings) > 0 {
			fmt.Printf("\n🟡 Warnings (%d):\n", len(warnings))
			for _, s := range warnings {
				fmt.Printf("  ⚠️  %s\n", s.Message)
				if s.Fix != "" {
					fmt.Printf("     Fix: %s\n", s.Fix)
				}
			}
		}
		if len(infos) > 0 {
			fmt.Printf("\n🔵 Suggestions (%d):\n", len(infos))
			for _, s := range infos {
				fmt.Printf("  💡 %s\n", s.Message)
				if s.Fix != "" {
					fmt.Printf("     Tip: %s\n", s.Fix)
				}
			}
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("💡 Tip: Run 'aflare review' before 'aflare run' to catch issues early.")
	fmt.Println("   The advisor model checks for security, performance, and reliability.")
}
