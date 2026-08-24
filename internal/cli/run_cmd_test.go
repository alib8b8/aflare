// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​​​​‌​​​​‌‌‌‌​​‌‌​‌‌​‌​‌​​​‌‌​‌‌‌‌​​‌‌‌​​​‌‌​‌‌‌‌​​​​​​​​​​​​​​​​​​​​‌​‌‌‌‌‌‌​‌‌‌⁠
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

// minimalRunWorkflow is a valid workflow with no input_schema, no {{var.*}}
// references and no LLM nodes, so a dry run passes validation and reaches the
// "Dry run completed" branch without needing a provider, network or TTY.
const minimalRunWorkflow = `name: cli-dryrun-demo
description: minimal workflow used by run command tests
steps:
  - name: save
    node: file_write
    params:
      path: out.txt
      content: hello
`

// writeRunFixture writes content to name inside dir and returns its path.
func writeRunFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture %s: %v", path, err)
	}
	return path
}

// runCase captures stdout and the error returned by fn in one step.
type runCase struct {
	out string
	err error
}

// runCaptured runs fn with stdout captured and returns the captured output
// and the returned error. Shared by the CLI command tests.
func runCaptured(fn func() error) runCase {
	var rc runCase
	rc.out = captureOutput(func() {
		rc.err = fn()
	})
	return rc
}

func TestRunCmd_NoArgs(t *testing.T) {
	i18n.Init("en")
	rc := runCaptured(func() error {
		return HandleRun(nil, false, false)
	})
	if code := exitCodeForErr(rc.err); code != 1 {
		t.Errorf("expected exit code 1 for no args, got %d (err=%v)", code, rc.err)
	}
	if !strings.Contains(rc.out, "Usage: aflare run") {
		t.Errorf("expected usage output, got: %s", rc.out)
	}
}

func TestRunCmd_Help(t *testing.T) {
	i18n.Init("en")
	for _, arg := range []string{"--help", "-h"} {
		rc := runCaptured(func() error {
			return HandleRun([]string{arg}, false, false)
		})
		if code := exitCodeForErr(rc.err); code != 0 {
			t.Errorf("%s: expected exit code 0, got %d (err=%v)", arg, code, rc.err)
		}
		if !strings.Contains(rc.out, "Usage: aflare run") {
			t.Errorf("%s: expected usage output, got: %s", arg, rc.out)
		}
	}
}

func TestRunCmd_MissingWorkflowFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	rc := runCaptured(func() error {
		return HandleRun([]string{missing}, false, false)
	})
	if code := exitCodeForErr(rc.err); code != 1 {
		t.Errorf("expected exit code 1 for missing workflow file, got %d (err=%v)", code, rc.err)
	}
	if !strings.Contains(rc.out, "Error preparing workflow") {
		t.Errorf("expected prepare failure output, got: %s", rc.out)
	}
}

func TestRunCmd_ParamsFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	params := writeRunFixture(t, dir, "params.json", "{not valid json")
	wf := writeRunFixture(t, dir, "wf.yaml", minimalRunWorkflow)

	rc := runCaptured(func() error {
		return HandleRun([]string{"--params-file", params, wf}, false, false)
	})
	if code := exitCodeForErr(rc.err); code != 1 {
		t.Errorf("expected exit code 1 for invalid params file, got %d (err=%v)", code, rc.err)
	}
	if !strings.Contains(rc.out, "读取参数文件失败") {
		t.Errorf("expected params-file failure output, got: %s", rc.out)
	}
}

func TestRunCmd_DryRunMinimalWorkflow(t *testing.T) {
	wf := writeRunFixture(t, t.TempDir(), "minimal.yaml", minimalRunWorkflow)

	rc := runCaptured(func() error {
		return HandleRun([]string{wf}, true, false)
	})
	if code := exitCodeForErr(rc.err); code != 0 {
		t.Errorf("expected exit code 0 for dry run, got %d (err=%v)", code, rc.err)
	}
	if !strings.Contains(rc.out, "Dry run completed") {
		t.Errorf("expected dry-run completion output, got: %s", rc.out)
	}
}

// TestRunCmd_FlagParsingDryRun exercises the flag-parsing branches of
// HandleRun (--set/--resume/--params/--params-file in both spaced and
// single-token forms). Every complete row reaches the dry-run branch, so the
// assertions double as a check that flags never swallow the workflow path.
func TestRunCmd_FlagParsingDryRun(t *testing.T) {
	dir := t.TempDir()
	wf := writeRunFixture(t, dir, "minimal.yaml", minimalRunWorkflow)
	params := writeRunFixture(t, dir, "params.json", `{"city": "Paris"}`)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		wantOut string
	}{
		{
			// The workflow path must precede --set: the flag consumes
			// every following non-flag token as a key=value pair.
			name:    "set consumes following pair",
			args:    []string{wf, "--set", "city=Paris"},
			wantOut: "Dry run completed",
		},
		{
			name:    "set single-token form",
			args:    []string{"--set=city=Paris", wf},
			wantOut: "Dry run completed",
		},
		{
			name:    "resume flag without explicit path",
			args:    []string{"--resume", wf},
			wantOut: "Dry run completed",
		},
		{
			name:    "resume single-token form",
			args:    []string{"--resume=/tmp/checkpoint.json", wf},
			wantOut: "Dry run completed",
		},
		{
			name:    "params-file single-token form",
			args:    []string{"--params-file=" + params, wf},
			wantOut: "Dry run completed",
		},
		{
			name:    "deprecated params still merge",
			args:    []string{wf, "--params", "city=Paris"},
			wantOut: "Dry run completed",
		},
		{
			name:    "flags but no workflow file",
			args:    []string{"--set", "city=Paris"},
			wantErr: true,
			wantOut: "Usage: aflare run",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i18n.Init("en")
			rc := runCaptured(func() error {
				return HandleRun(tc.args, true, false)
			})
			code := exitCodeForErr(rc.err)
			if tc.wantErr && code != 1 {
				t.Errorf("expected exit code 1, got %d (err=%v)", code, rc.err)
			}
			if !tc.wantErr && code != 0 {
				t.Errorf("expected exit code 0, got %d (err=%v)", code, rc.err)
			}
			if !strings.Contains(rc.out, tc.wantOut) {
				t.Errorf("expected output to contain %q, got: %s", tc.wantOut, rc.out)
			}
		})
	}
}

// TestRunCmd_InputSchemaHint covers 断点8: a workflow that declares an
// input_schema but is run without parameters prints the schema help and
// exits 1 instead of failing with an empty-parameter error mid-run.
func TestRunCmd_InputSchemaHint(t *testing.T) {
	wf := writeRunFixture(t, t.TempDir(), "schema.yaml", `name: paramed-demo
steps:
  - node: file_write
    params:
      path: out.txt
input_schema:
  - name: city
    type: string
    required: true
`)

	rc := runCaptured(func() error {
		return HandleRun([]string{wf}, true, false)
	})
	if code := exitCodeForErr(rc.err); code != 1 {
		t.Errorf("expected exit code 1 for missing schema params, got %d (err=%v)", code, rc.err)
	}
	if !strings.Contains(rc.out, "此模板需要以下参数") {
		t.Errorf("expected input-schema help output, got: %s", rc.out)
	}
	if !strings.Contains(rc.out, "city") {
		t.Errorf("expected schema field name in help output, got: %s", rc.out)
	}
}

// TestRunCmd_InputSchemaSatisfiedBySet covers the complementary branch: when
// the input_schema is satisfied via --set, the run proceeds to dry run.
func TestRunCmd_InputSchemaSatisfiedBySet(t *testing.T) {
	wf := writeRunFixture(t, t.TempDir(), "schema.yaml", `name: paramed-demo
steps:
  - node: file_write
    params:
      path: out.txt
input_schema:
  - name: city
    type: string
    required: true
`)

	rc := runCaptured(func() error {
		return HandleRun([]string{wf, "--set", "city=Paris"}, true, false)
	})
	if code := exitCodeForErr(rc.err); code != 0 {
		t.Errorf("expected exit code 0 when schema params supplied, got %d (err=%v)", code, rc.err)
	}
	if !strings.Contains(rc.out, "Dry run completed") {
		t.Errorf("expected dry-run completion output, got: %s", rc.out)
	}
}

// TestRunCmd_MissingVarHint covers 断点E: a template without input_schema
// that references {{var.xxx}} with no default in vars: gets the extracted
// parameter hint instead of an opaque "variable not found" error later.
func TestRunCmd_MissingVarHint(t *testing.T) {
	wf := writeRunFixture(t, t.TempDir(), "refs.yaml", `name: weather-demo
steps:
  - node: http_request
    params:
      url: "https://example.com/weather?city={{var.city}}"
`)

	rc := runCaptured(func() error {
		return HandleRun([]string{wf}, true, false)
	})
	if code := exitCodeForErr(rc.err); code != 1 {
		t.Errorf("expected exit code 1 for missing {{var}} reference, got %d (err=%v)", code, rc.err)
	}
	if !strings.Contains(rc.out, "此模板引用了以下参数") {
		t.Errorf("expected extracted-params help output, got: %s", rc.out)
	}
	if !strings.Contains(rc.out, "city") {
		t.Errorf("expected referenced var name in help output, got: %s", rc.out)
	}
}

// TestRunCmd_DeclaredVarDefaultRuns covers the other side of 断点E: variables
// declared with a default in vars: do not trigger the parameter hint.
func TestRunCmd_DeclaredVarDefaultRuns(t *testing.T) {
	wf := writeRunFixture(t, t.TempDir(), "defaults.yaml", `name: weather-defaults
vars:
  city: Paris
steps:
  - node: http_request
    params:
      url: "https://example.com/weather?city={{var.city}}"
`)

	rc := runCaptured(func() error {
		return HandleRun([]string{wf}, true, false)
	})
	if code := exitCodeForErr(rc.err); code != 0 {
		t.Errorf("expected exit code 0 when vars declare defaults, got %d (err=%v)", code, rc.err)
	}
	if !strings.Contains(rc.out, "Dry run completed") {
		t.Errorf("expected dry-run completion output, got: %s", rc.out)
	}
}

func TestRunCmd_IsSensitiveKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"api_key", true},
		{"API_KEY", true},
		{"password", true},
		{"Authorization", true},
		{"my_api_token", true},
		{"auth-header", true},
		{"bearer_token", true},
		{"client_secret", true},
		{"city", false},
		{"description", false},
		{"output_path", false},
	}
	for _, tc := range tests {
		if got := isSensitiveKey(tc.key); got != tc.want {
			t.Errorf("isSensitiveKey(%q) = %v, want %v", tc.key, got, tc.want)
		}
	}
}

func TestRunCmd_RedactParams(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]string
		want map[string]string
	}{
		{
			name: "nil map yields empty map",
			in:   nil,
			want: map[string]string{},
		},
		{
			name: "sensitive keys are masked",
			in:   map[string]string{"api_key": "sk-123", "password": "hunter2"},
			want: map[string]string{"api_key": "***", "password": "***"},
		},
		{
			name: "plain keys are preserved",
			in:   map[string]string{"city": "Paris", "path": "out.txt"},
			want: map[string]string{"city": "Paris", "path": "out.txt"},
		},
		{
			name: "mixed keys are partially masked",
			in:   map[string]string{"city": "Paris", "token": "abc"},
			want: map[string]string{"city": "Paris", "token": "***"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := redactParams(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("redactParams returned %d entries, want %d (%v)", len(got), len(tc.want), got)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("redactParams[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
