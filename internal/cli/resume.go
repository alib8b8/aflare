// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌‌‌​‌‌‌‌‌‌‌‌‌​‌​​‌‌​‌‌‌‌​​​‌​‌​​​​​‌‌​​‌‌‌​‌​‌‌​​​​​​​​​​​​​​​​‌​‌‌‌‌‌​​​​‌​​‌​⁠
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
	"os"
	"strings"

	"github.com/alib8b8/aflare/internal/i18n"
	"github.com/alib8b8/aflare/internal/workflow"
)

// HandleResume handles the "resume" command. It resumes a paused workflow
// from a run-id.
//
// Usage:
//
//	aflare resume <run-id>        Resume a paused workflow
//	aflare resume list            List all paused workflows
func HandleResume(args []string) {
	if len(args) == 0 {
		fmt.Println(i18n.T("resume.usage"))
		os.Exit(1)
	}

	subCmd := args[0]
	switch subCmd {
	case "list":
		HandleResumeList()
	case "-h", "--help", "help":
		fmt.Println(i18n.T("resume.usage"))
	default:
		runID := subCmd
		HandleResumeRun(runID)
	}
}

// HandleResumeRun resumes a specific paused workflow by run-id.
func HandleResumeRun(runID string) {
	// Validate run-id: reject path traversal attempts
	if strings.Contains(runID, "/") || strings.Contains(runID, "\\") || strings.Contains(runID, "..") {
		fmt.Fprintf(os.Stderr, "invalid run-id: %s\n", runID)
		os.Exit(1)
	}

	// Check that the run exists and is paused
	meta, err := workflow.LoadRunMeta(runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load run %q: %v\n", runID, err)
		os.Exit(1)
	}
	if meta.Status != "paused" {
		fmt.Fprintf(os.Stderr, "run %q is not paused (status: %s)\n", runID, meta.Status)
		os.Exit(1)
	}

	fmt.Printf("Resuming workflow %q (run-id: %s, paused at step %d: %s)...\n",
		meta.WorkflowName, runID, meta.PausedStep+1, meta.StepName)

	ctx := context.Background()
	output, results, err := workflow.ResumeWorkflow(ctx, runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resume failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nWorkflow resumed and completed successfully.\n")
	fmt.Printf("Output: %s\n", output)
	fmt.Printf("Steps executed: %d\n", len(results))
}

// HandleResumeList lists all paused workflow runs.
func HandleResumeList() {
	runs, err := workflow.ListPausedRuns()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to list paused runs: %v\n", err)
		os.Exit(1)
	}

	if len(runs) == 0 {
		fmt.Println("No paused workflows.")
		return
	}

	fmt.Printf("Paused workflows (%d):\n\n", len(runs))
	for _, r := range runs {
		fmt.Printf("  Run ID:    %s\n", r.RunID)
		fmt.Printf("  Workflow:  %s\n", r.WorkflowName)
		fmt.Printf("  Paused at: step %d (%s)\n", r.PausedStep+1, r.StepName)
		fmt.Printf("  Paused on: %s\n", r.PausedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Resume on: %s\n", r.ResumeOn)
		fmt.Println()
	}
}
