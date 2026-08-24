// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​‌​‌​‌​​‌​​‌​‌​​​​​‌​​‌​​‌​‌‌‌‌‌‌​‌‌​‌​‌‌‌​‌‌‌​​​​​​​​​​​​​​​​​​​​​‌‌​​‌‌​​‌‌​⁠
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

package workflow

import (
	"os"
	"testing"
)

func FuzzParseWorkflowString(f *testing.F) {
	f.Add("name: test\nsteps:\n  - node: test\n")
	f.Add("not: [ valid yaml :::")
	f.Add("")
	f.Add("name: \"test\"\ndescription: \"desc\"\nsteps:\n  - node: test\n    params:\n      key: value\n")
	f.Add("steps:\n  - node: test\n")
	f.Add("name: test\n")

	tmpDir, err := os.MkdirTemp("", "fuzz-parser-*")
	if err != nil {
		f.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	f.Fuzz(func(t *testing.T, input string) {
		tmpFile, err := os.CreateTemp(tmpDir, "fuzz-*.yaml")
		if err != nil {
			t.Skipf("failed to create temp file: %v", err)
			return
		}
		path := tmpFile.Name()

		if _, err := tmpFile.WriteString(input); err != nil {
			tmpFile.Close()
			t.Skipf("failed to write temp file: %v", err)
			return
		}
		tmpFile.Close()
		defer os.Remove(path)

		wf, err := ParseWorkflow(path)
		if err != nil {
			return
		}
		if wf == nil {
			t.Errorf("ParseWorkflow returned nil workflow with no error")
		}
	})
}
