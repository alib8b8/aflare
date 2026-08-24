// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​​​‌​​‌‌‌‌​‌‌​‌​​​​​​​‌‌​​‌‌‌‌​​‌‌​‌‌​​​‌‌​​​​​​‌​​​​​​​​​​​​​​​​​​‌‌​‌‌​‌‌‌‌​‌​‌⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/i18n"
)

// validValidateWorkflow passes ValidateWorkflow with no suggestions (named,
// has steps, has a file_write step) and only references registered builtin
// nodes, so `aflare validate` reports it as fully valid.
const validValidateWorkflow = `name: validate-demo
description: workflow used by validate command tests
steps:
  - name: save
    node: file_write
    params:
      path: out.txt
      content: hello
`

func writeValidateFixture(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture %s: %v", path, err)
	}
	return path
}

func TestValidateCmd_NoArgs(t *testing.T) {
	i18n.Init("en")
	var err error
	out := captureOutput(func() {
		err = HandleValidate(nil)
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for no args, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Usage: aflare validate") {
		t.Errorf("expected usage output, got: %s", out)
	}
}

func TestValidateCmd_ValidWorkflow(t *testing.T) {
	i18n.Init("en")
	wf := writeValidateFixture(t, "valid.yaml", validValidateWorkflow)

	var err error
	out := captureOutput(func() {
		err = HandleValidate([]string{wf})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for valid workflow, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Workflow is valid!") {
		t.Errorf("expected valid output, got: %s", out)
	}
}

func TestValidateCmd_InvalidYAML(t *testing.T) {
	i18n.Init("en")
	wf := writeValidateFixture(t, "broken.yaml", "name: broken\nsteps:\n  - node: [unterminated\n")

	var err error
	out := captureOutput(func() {
		err = HandleValidate([]string{wf})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for invalid YAML, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Error loading workflow") {
		t.Errorf("expected load-error output, got: %s", out)
	}
}

// TestValidateCmd_UnknownNodeWarning covers the registry cross-check: a step
// whose node is not registered produces a warning and a non-zero exit.
func TestValidateCmd_UnknownNodeWarning(t *testing.T) {
	i18n.Init("en")
	wf := writeValidateFixture(t, "unknown-node.yaml", `name: unknown-node-demo
steps:
  - node: no_such_node
    params:
      foo: bar
`)

	var err error
	out := captureOutput(func() {
		err = HandleValidate([]string{wf})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for unknown node, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "unknown node 'no_such_node'") {
		t.Errorf("expected unknown-node warning, got: %s", out)
	}
}

func TestValidateCmd_MissingFile(t *testing.T) {
	i18n.Init("en")
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	var err error
	out := captureOutput(func() {
		err = HandleValidate([]string{missing})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for missing file, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Error loading workflow") {
		t.Errorf("expected load-error output, got: %s", out)
	}
}
