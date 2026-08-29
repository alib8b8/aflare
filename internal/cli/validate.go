// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌‌‌​‌​​‌‌​‌​‌​​‌‌​‌​‌‌​​​‌‌‌‌‌​‌​‌​​​​‌‌​‌​​​‌‌​​​​​​​​​​​​​​​​​‌​‌​‌‌‌​​‌​‌​‌‌⁠
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

	"github.com/alib8b8/aflare/internal/i18n"
	"github.com/alib8b8/aflare/internal/workflow"
)

// HandleValidate handles the "validate" command.
func HandleValidate(args []string) error {
	if len(args) < 1 {
		fmt.Println(i18n.T("validate.usage"))
		return exitErr(1)
	}

	wf, reg, err := PrepareWorkflow(args[0])
	if err != nil {
		fmt.Printf("❌ %s\n", i18n.T("validate.error", err))
		return exitErr(1)
	}

	warnings := workflow.ValidateWorkflow(wf)

	// Hard failures make the workflow unrunnable and must fail CI: a
	// workflow with no steps is a no-op, and an unknown node aborts
	// execution at that step. Pure suggestions (missing name, missing
	// file_write) are advisory — the workflow still runs, so exit 0 and
	// let `aflare validate && ...` pipelines proceed.
	hardFailure := len(wf.Steps) == 0

	for i, step := range wf.Steps {
		if step.IsIf() || step.IsLoop() || step.IsMap() || step.IsReduce() || step.IsParallel() || step.IsSaga() || step.HasCaptureError() {
			continue
		}
		if _, ok := reg.Get(step.Node); !ok {
			warnings = append(warnings, workflow.NodeWarning(i+1, step.Node))
			hardFailure = true
		}
	}

	if len(warnings) == 0 {
		fmt.Printf("✅ %s\n", i18n.T("validate.valid"))
	} else {
		fmt.Printf("⚠️ %s\n", i18n.T("validate.warnings"))
		for _, warning := range warnings {
			fmt.Printf("  - %s\n", warning)
		}
		if hardFailure {
			return exitErr(1)
		}
	}
	return nil
}
