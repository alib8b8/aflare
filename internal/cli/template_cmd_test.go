// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​​‌‌‌‌‌​​‌​​‌‌​‌​​‌‌‌‌‌‌‌​​‌‌​​​‌‌‌​‌‌​​‌​​​‌​‌​​​​​​​​​​​​​​​​​​‌‌​‌​‌​​‌‌​​​​​​⁠
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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateCmdDispatch(t *testing.T) {
	// "list" needs a loadable registry; seed an empty one.
	seedTemplatesRegistry(t)

	cases := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{"no args", nil, 1},
		{"help", []string{"help"}, 0},
		{"help long", []string{"--help"}, 0},
		{"help short", []string{"-h"}, 0},
		{"list", []string{"list"}, 0},
		{"unknown subcommand", []string{"zz-bogus"}, 1},
		{"new without name", []string{"new"}, 1},
		{"clone without args", []string{"clone"}, 1},
		{"run without id", []string{"run"}, 1},
		{"submit without file", []string{"submit"}, 1},
		{"submit missing file", []string{"submit", "zz-missing-workflow.yaml"}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			out := captureOutput(func() {
				err = HandleTemplateSubmit(tc.args, false, false)
			})
			if code := exitCodeForErr(err); code != tc.wantCode {
				t.Errorf("HandleTemplateSubmit(%v) exit code = %d, want %d (err=%v)\noutput:\n%s",
					tc.args, code, tc.wantCode, err, out)
			}
		})
	}
}

func TestTemplateCmdRunErrors(t *testing.T) {
	seedTemplatesRegistry(t,
		seedTemplate{ID: "devops-infra/zz-seed-easy", Workflow: seedEasyWorkflow},
		// Registered, but its directory has no workflow.yaml.
		seedTemplate{ID: "devops-infra/zz-seed-noyaml"},
	)

	cases := []struct {
		name string
		args []string
	}{
		{"no template id", nil},
		{"flags only", []string{"--set", "k=v"}},
		// A single flag argument leaves the template id empty ("--set k=v"
		// would swallow "k=v" as the id, so use a lone flag here).
		{"flag only", []string{"--resume"}},
		{"unknown template", []string{"zz-missing-tpl"}},
		{"template without workflow.yaml", []string{"devops-infra/zz-seed-noyaml"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			captureOutput(func() {
				err = handleTemplateRun(tc.args, false, false)
			})
			if code := exitCodeForErr(err); code != 1 {
				t.Errorf("handleTemplateRun(%v) exit code = %d, want 1 (err=%v)", tc.args, code, err)
			}
		})
	}
}

func TestTemplateCmdParseSubmitArgs(t *testing.T) {
	cases := []struct {
		name         string
		args         []string
		wantCode     int
		wantYAML     string
		wantCategory string
		wantAuthor   string
	}{
		{name: "no args"},
		{name: "positional path", args: []string{"flow.yaml"}, wantYAML: "flow.yaml"},
		{
			name:         "long options",
			args:         []string{"flow.yaml", "--category", "devops-infra", "--author", "Bob"},
			wantYAML:     "flow.yaml",
			wantCategory: "devops-infra",
			wantAuthor:   "Bob",
		},
		{
			name:         "short options",
			args:         []string{"-c", "security", "-a", "Alice", "flow.yaml"},
			wantYAML:     "flow.yaml",
			wantCategory: "security",
			wantAuthor:   "Alice",
		},
		{name: "help", args: []string{"--help"}},
		{name: "help short", args: []string{"-h"}},
		{name: "dangling category flag", args: []string{"flow.yaml", "--category"}, wantYAML: "flow.yaml"},
		{name: "dangling author flag", args: []string{"flow.yaml", "-a"}, wantYAML: "flow.yaml"},
		{name: "unknown flag", args: []string{"flow.yaml", "--zzz"}, wantCode: 1},
		{name: "second positional", args: []string{"a.yaml", "b.yaml"}, wantCode: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var yamlPath, category, author string
			var err error
			captureOutput(func() {
				err = parseTemplateSubmitArgs(tc.args, &yamlPath, &category, &author)
			})
			if code := exitCodeForErr(err); code != tc.wantCode {
				t.Errorf("parseTemplateSubmitArgs(%v) exit code = %d, want %d (err=%v)",
					tc.args, code, tc.wantCode, err)
			}
			if tc.wantCode != 0 {
				return
			}
			if yamlPath != tc.wantYAML || category != tc.wantCategory || author != tc.wantAuthor {
				t.Errorf("parseTemplateSubmitArgs(%v) = (%q, %q, %q), want (%q, %q, %q)",
					tc.args, yamlPath, category, author, tc.wantYAML, tc.wantCategory, tc.wantAuthor)
			}
		})
	}
}

func TestTemplateCmdRunHappyPath(t *testing.T) {
	// The seeded workflow mirrors the minimal dry-run fixture used by the
	// run command tests: a single file_write node dry-runs without any
	// external dependency.
	seedTemplatesRegistry(t,
		seedTemplate{ID: "devops-infra/zz-seed-run", Workflow: `name: seed-run
steps:
  - name: save
    node: file_write
    params:
      path: out.txt
      content: hello
`},
	)

	for _, tc := range []struct {
		name string
		id   string
	}{
		{"by full id", "devops-infra/zz-seed-run"},
		{"fuzzy by name suffix", "zz-seed-run"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			out := captureOutput(func() {
				err = handleTemplateRun([]string{tc.id}, true, false)
			})
			if code := exitCodeForErr(err); code != 0 {
				t.Fatalf("handleTemplateRun(%s) exit code = %d, want 0 (err=%v)", tc.id, code, err)
			}
			if !strings.Contains(out, "▶ 运行模板 devops-infra/zz-seed-run") {
				t.Errorf("expected run header for %s, got:\n%s", tc.id, out)
			}
		})
	}
}

func TestTemplateCmdSubmitUnknownFlag(t *testing.T) {
	// A parse error from parseTemplateSubmitArgs propagates out of
	// handleTemplateSubmit unchanged.
	var err error
	out := captureOutput(func() {
		err = handleTemplateSubmit([]string{"flow.yaml", "--zzz"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("handleTemplateSubmit(unknown flag) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(out, "Unknown argument: --zzz") {
		t.Errorf("expected unknown-argument message, got:\n%s", out)
	}
}

func TestTemplateCmdSubmitDefaults(t *testing.T) {
	tplDir := seedTemplatesRegistry(t)
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(tplDir, "devops-infra", "docker-deploy"))
	})

	// No name, no description in the YAML and no --author: all three
	// defaults kick in, and the file name contains a "docker" keyword so
	// the category is guessed as devops-infra.
	src := filepath.Join(t.TempDir(), "docker-deploy.yaml")
	if err := os.WriteFile(src, []byte("steps:\n  - node: fetch_url\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var err error
	out := captureOutput(func() {
		err = handleTemplateSubmit([]string{src})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Fatalf("handleTemplateSubmit(defaults) exit code = %d, want 0 (err=%v)", code, err)
	}
	if !strings.Contains(out, "Template:  devops-infra/docker-deploy") {
		t.Errorf("expected guessed category in output, got:\n%s", out)
	}

	metaData, readErr := os.ReadFile(filepath.Join(tplDir, "devops-infra", "docker-deploy", "skill.json"))
	if readErr != nil {
		t.Fatalf("read skill.json: %v", readErr)
	}
	for _, want := range []string{
		`"author": "community contributor"`,
		`"description": "docker-deploy workflow template"`,
		`"name": "docker-deploy"`,
	} {
		if !strings.Contains(string(metaData), want) {
			t.Errorf("expected %s in skill.json, got:\n%s", want, metaData)
		}
	}
}

func TestTemplateCmdSubmitCategoryDirIsFile(t *testing.T) {
	tplDir := seedTemplatesRegistry(t)
	// A file where the category directory should go makes MkdirAll fail
	// with ENOTDIR.
	collide := filepath.Join(tplDir, "zz-collide")
	if err := os.WriteFile(collide, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(collide) })

	src := filepath.Join(t.TempDir(), "zz-collide-flow.yaml")
	if err := os.WriteFile(src, []byte("name: x\nsteps:\n  - node: fetch_url\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var err error
	out := captureOutput(func() {
		err = handleTemplateSubmit([]string{src, "--category", "zz-collide"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("handleTemplateSubmit(file as category) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(out, "failed to create template directory") {
		t.Errorf("expected directory-creation failure, got:\n%s", out)
	}
}

func TestTemplateCmdSubmitBadgeStoreCorrupt(t *testing.T) {
	tplDir := seedTemplatesRegistry(t)
	// A corrupt badge store must not block the submission; the badge step
	// fails silently. DefaultStorePath uses os.UserHomeDir (uncached), so
	// HOME redirects it hermetically.
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".aflare"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".aflare", "badges.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(tplDir, "uncategorized", "zz-plain"))
		_ = os.Remove(filepath.Join(tplDir, "uncategorized"))
	})

	src := filepath.Join(t.TempDir(), "zz-plain.yaml")
	if err := os.WriteFile(src, []byte("name: plain\nsteps:\n  - node: fetch_url\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var err error
	out := captureOutput(func() {
		err = handleTemplateSubmit([]string{src, "--author", "Zed"})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Fatalf("handleTemplateSubmit(corrupt badge store) exit code = %d, want 0 (err=%v)", code, err)
	}
	if !strings.Contains(out, "Template prepared for submission!") {
		t.Errorf("expected submission confirmation, got:\n%s", out)
	}
}

func TestTemplateCmdSubmitFileErrors(t *testing.T) {
	tmp := t.TempDir()
	valid := filepath.Join(tmp, "zz-good.yaml")
	if err := os.WriteFile(valid, []byte("name: Good\nsteps:\n  - node: fetch_url\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	emptyFile := filepath.Join(tmp, "zz-empty.yaml")
	if err := os.WriteFile(emptyFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	notYAML := filepath.Join(tmp, "zz-not.yaml.txt")
	if err := os.WriteFile(notYAML, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A file literally named "..yaml" yields the base name "." after the
	// extension is stripped, which must be rejected as path traversal.
	traversalBase := filepath.Join(tmp, "..yaml")
	if err := os.WriteFile(traversalBase, []byte("name: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		args []string
	}{
		{"no file", nil},
		{"missing file", []string{filepath.Join(tmp, "zz-nope.yaml")}},
		{"directory instead of file", []string{tmp}},
		{"not a yaml file", []string{notYAML}},
		{"empty workflow", []string{emptyFile}},
		{"traversal base name", []string{traversalBase}},
		{"traversal category", []string{valid, "--category", "../evil"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			captureOutput(func() {
				err = handleTemplateSubmit(tc.args)
			})
			if code := exitCodeForErr(err); code != 1 {
				t.Errorf("handleTemplateSubmit(%v) exit code = %d, want 1 (err=%v)", tc.args, code, err)
			}
		})
	}
}

func TestTemplateCmdSubmitHappyPath(t *testing.T) {
	tplDir := seedTemplatesRegistry(t)
	// awardBadgeForTemplate persists the badge store under $HOME/.aflare —
	// point HOME at a throwaway directory so the real one is untouched.
	t.Setenv("HOME", t.TempDir())
	// The submission writes template trees into the resolved templates dir;
	// remove them again on cleanup (the registry index is restored by the
	// seed helper).
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(tplDir, "uncategorized", "zz-seed-flow"))
		_ = os.RemoveAll(filepath.Join(tplDir, "devops-infra", "zz-seed-flow"))
		// Drop the guessed-category directory too, but only when this test
		// left it empty — os.Remove refuses to touch non-empty dirs, so a
		// pre-existing real catalog under "uncategorized" is never harmed.
		_ = os.Remove(filepath.Join(tplDir, "uncategorized"))
	})

	src := filepath.Join(t.TempDir(), "zz-seed-flow.yaml")
	workflow := "name: Seed Flow\ndescription: A seeded workflow\nsteps:\n  - node: fetch_url\n"
	if err := os.WriteFile(src, []byte(workflow), 0o644); err != nil {
		t.Fatal(err)
	}

	// Without --category the category is guessed from the file name; no
	// keyword matches, so it lands on the non-standard "uncategorized"
	// category, which prints a warning but still succeeds.
	var err error
	out := captureOutput(func() {
		err = handleTemplateSubmit([]string{src, "--author", "Zed"})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Fatalf("handleTemplateSubmit exit code = %d, want 0 (err=%v)", code, err)
	}
	if !strings.Contains(out, "Template prepared for submission!") {
		t.Errorf("expected submission confirmation, got:\n%s", out)
	}
	if !strings.Contains(out, "uncategorized/zz-seed-flow") {
		t.Errorf("expected guessed category in output, got:\n%s", out)
	}
	if !strings.Contains(out, `category "uncategorized" is not in the standard list`) {
		t.Errorf("expected non-standard-category warning, got:\n%s", out)
	}
	for _, f := range []string{"workflow.yaml", "skill.json", "README.md"} {
		if _, statErr := os.Stat(filepath.Join(tplDir, "uncategorized", "zz-seed-flow", f)); statErr != nil {
			t.Errorf("expected %s in the submitted template dir: %v", f, statErr)
		}
	}

	// With an explicit valid category no warning is printed.
	var errSecond error
	outSecond := captureOutput(func() {
		errSecond = handleTemplateSubmit([]string{src, "--category", "devops-infra", "--author", "Zed"})
	})
	if code := exitCodeForErr(errSecond); code != 0 {
		t.Fatalf("handleTemplateSubmit (explicit category) exit code = %d, want 0 (err=%v)", code, errSecond)
	}
	if !strings.Contains(outSecond, "devops-infra/zz-seed-flow") {
		t.Errorf("expected explicit category in output, got:\n%s", outSecond)
	}
	if strings.Contains(outSecond, "is not in the standard list") {
		t.Errorf("unexpected non-standard-category warning, got:\n%s", outSecond)
	}
	if _, statErr := os.Stat(filepath.Join(tplDir, "devops-infra", "zz-seed-flow", "workflow.yaml")); statErr != nil {
		t.Errorf("expected workflow.yaml under the explicit category: %v", statErr)
	}
}
