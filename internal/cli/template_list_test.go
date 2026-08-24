// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​‌‌​‌‌‌‌​​‌‌​​​‌​‌‌​‌​‌‌​‌​​‌‌​‌‌‌‌​‌​​​‌‌​​‌‌​​‌​​​​​​​​​​​​​​​​​​​‌‌​​​​​​‌‌‌​‌⁠
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
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/meta"
	skillsPkg "github.com/alib8b8/aflare/internal/skills"
)

// Minimal workflow fixtures for the template tests. Node types are chosen so
// inferDifficultyFromWorkflow classifies them as easy / medium / hard; the
// hard fixture also exercises the parallel / saga / loop step variants.
const (
	seedEasyWorkflow = `name: seed-easy
steps:
  - node: fetch_url
    name: fetch
`
	seedMediumWorkflow = `name: seed-medium
steps:
  - node: llm
    name: summarize
`
	seedHardWorkflow = `name: seed-hard
steps:
  - node: fetch_url
    name: fetch
    parallel:
      - node: code_interpreter
    saga:
      steps:
        - node: sandbox
    loop:
      steps:
        - node: llm
`
	// Unbalanced flow sequence — guaranteed YAML parse error.
	seedBadYAMLWorkflow = `name: [unclosed
steps:
  - node: llm
`
)

// seedTemplate describes one template fixture installed into the resolved
// templates directory for the duration of a test.
type seedTemplate struct {
	ID          string // skill ID ("<category>/<name>"), also the on-disk subdirectory
	Difficulty  string // optional explicit difficulty; empty → inferred from workflow.yaml
	Description string // optional description; defaults to "Seeded template <id>"
	Workflow    string // workflow.yaml content; empty → the file is not written
}

// seedTemplatesRegistry takes over the templates directory that
// meta.ResolveTemplatesPath() resolves to for the duration of the test: it
// writes a skills-registry.json index containing exactly the given entries
// (plus their workflow.yaml files) and restores the previous state via
// t.Cleanup. It returns the templates directory.
//
// SkillRegistry.Load() prefers the index file over directory scanning, so the
// handlers under test observe exactly these entries, and the index's presence
// makes EnsureEmbeddedTemplates a no-op — the 323-template embedded catalog is
// never materialized (or, when an earlier test in the same binary already
// materialized it, temporarily hidden).
//
// The directory cannot be redirected via AFLARE_HOME instead: meta caches its
// paths with sync.Once, and by the time these tests run the process-wide
// cache may already point at the real user home (create_test.go releases the
// embedded catalog there during a full-suite run). Seeding the resolved
// directory directly stays hermetic under both the filtered and full runs.
func seedTemplatesRegistry(t *testing.T, entries ...seedTemplate) string {
	t.Helper()

	tplDir := meta.ResolveTemplatesPath()
	if err := os.MkdirAll(tplDir, 0o755); err != nil {
		t.Fatalf("seedTemplatesRegistry: create templates dir: %v", err)
	}
	registryPath := filepath.Join(tplDir, skillsPkg.RegistryFileName)

	// Snapshot the previous index so it can be restored afterwards.
	prevIndex, hadPrev := func() ([]byte, bool) {
		data, err := os.ReadFile(registryPath)
		if err != nil {
			return nil, false
		}
		return data, true
	}()
	t.Cleanup(func() {
		if hadPrev {
			if err := os.WriteFile(registryPath, prevIndex, 0o644); err != nil {
				t.Logf("restore registry index: %v", err)
			}
			return
		}
		if err := os.Remove(registryPath); err != nil {
			t.Logf("remove seeded registry index: %v", err)
		}
	})

	metas := make([]*skillsPkg.SkillMeta, 0, len(entries))
	for _, e := range entries {
		dir := filepath.Join(tplDir, filepath.FromSlash(e.ID))
		if e.Workflow != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("seedTemplatesRegistry: create %s: %v", dir, err)
			}
			if err := os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(e.Workflow), 0o644); err != nil {
				t.Fatalf("seedTemplatesRegistry: write workflow.yaml in %s: %v", dir, err)
			}
			// Fixture directories use "zz-seed-" names that never collide
			// with the embedded catalog; remove them on cleanup.
			t.Cleanup(func() {
				if err := os.RemoveAll(dir); err != nil {
					t.Logf("remove seed dir %s: %v", dir, err)
				}
			})
		}
		desc := e.Description
		if desc == "" {
			desc = "Seeded template " + e.ID
		}
		metas = append(metas, &skillsPkg.SkillMeta{
			ID:          e.ID,
			Name:        path.Base(e.ID),
			Version:     "1.0.0",
			Description: desc,
			Author:      "seed",
			Category:    seedCategory(e.ID),
			Difficulty:  e.Difficulty,
		})
	}

	index := struct {
		Version string                 `json:"version"`
		Count   int                    `json:"count"`
		Skills  []*skillsPkg.SkillMeta `json:"skills"`
	}{Version: "1.0.0", Count: len(metas), Skills: metas}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		t.Fatalf("seedTemplatesRegistry: marshal index: %v", err)
	}
	if err := os.WriteFile(registryPath, data, 0o644); err != nil {
		t.Fatalf("seedTemplatesRegistry: write index: %v", err)
	}
	return tplDir
}

// seedCategory derives the category from a "<category>/<name>" seed ID.
func seedCategory(id string) string {
	if i := strings.Index(id, "/"); i > 0 {
		return id[:i]
	}
	return "uncategorized"
}

func TestTemplateListEmpty(t *testing.T) {
	seedTemplatesRegistry(t) // no templates at all

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"default shows easy hint", nil, "未找到无需额外配置的模板"},
		{"all shows empty hint", []string{"--all"}, "未找到任何模板"},
		{"unknown flag is reported", []string{"--zz-bogus-flag"}, "Unknown flag: --zz-bogus-flag"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			out := captureOutput(func() {
				err = handleTemplateList(tc.args)
			})
			if code := exitCodeForErr(err); code != 0 {
				t.Errorf("handleTemplateList(%v) exit code = %d, want 0 (err=%v)", tc.args, code, err)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("handleTemplateList(%v) output missing %q:\n%s", tc.args, tc.want, out)
			}
		})
	}
}

func TestTemplateListSeeded(t *testing.T) {
	seedTemplatesRegistry(t,
		seedTemplate{ID: "devops-infra/zz-seed-easy", Workflow: seedEasyWorkflow},
		seedTemplate{ID: "data-ai/zz-seed-llm", Workflow: seedMediumWorkflow},
		seedTemplate{ID: "software-engineering/zz-seed-hard", Workflow: seedHardWorkflow},
		// Explicit difficulty from the skill metadata (no workflow.yaml).
		seedTemplate{
			ID:          "devops-infra/zz-seed-static",
			Difficulty:  "easy",
			Description: "A deliberately long seeded description that exceeds forty characters",
		},
		// Registered but no workflow.yaml → inference falls back to easy.
		seedTemplate{ID: "devops-infra/zz-seed-noyaml"},
		// Invalid workflow.yaml → inference falls back to easy.
		seedTemplate{ID: "devops-infra/zz-seed-badyaml", Workflow: seedBadYAMLWorkflow},
	)

	cases := []struct {
		name    string
		args    []string
		want    []string
		wantNot []string
	}{
		{
			name: "default lists only easy templates",
			want: []string{
				"共 4 个",
				"devops-infra/zz-seed-easy",
				"devops-infra/zz-seed-static",
				"devops-infra/zz-seed-noyaml",
				"devops-infra/zz-seed-badyaml",
				// Long description is truncated to 37 chars + "...".
				"A deliberately long seeded descriptio...",
				"easy=4", "总计=6",
			},
			wantNot: []string{"data-ai/zz-seed-llm", "software-engineering/zz-seed-hard"},
		},
		{
			name: "all lists every difficulty",
			args: []string{"--all"},
			want: []string{
				"全部模板（共 6 个）",
				"data-ai/zz-seed-llm",
				"software-engineering/zz-seed-hard",
				// Short description is not truncated.
				"Seeded template data-ai/zz-seed-llm",
			},
		},
		{
			name:    "category filter",
			args:    []string{"--all", "--category=data-ai"},
			want:    []string{"data-ai/zz-seed-llm"},
			wantNot: []string{"devops-infra/zz-seed-easy"},
		},
		{
			name:    "bare category flag is ignored",
			args:    []string{"--category"},
			want:    []string{"devops-infra/zz-seed-easy"},
			wantNot: []string{"data-ai/zz-seed-llm"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			out := captureOutput(func() {
				err = handleTemplateList(tc.args)
			})
			if code := exitCodeForErr(err); code != 0 {
				t.Errorf("handleTemplateList(%v) exit code = %d, want 0 (err=%v)", tc.args, code, err)
			}
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("handleTemplateList(%v) output missing %q:\n%s", tc.args, w, out)
				}
			}
			for _, w := range tc.wantNot {
				if strings.Contains(out, w) {
					t.Errorf("handleTemplateList(%v) output unexpectedly contains %q:\n%s", tc.args, w, out)
				}
			}
		})
	}
}

func TestTemplateNewArgErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"missing name", nil},
		{"path traversal", []string{"../evil"}},
		{"path separator", []string{"a/b"}},
		{"leading dot", []string{".hidden"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			captureOutput(func() {
				err = handleTemplateNew(tc.args)
			})
			if code := exitCodeForErr(err); code != 1 {
				t.Errorf("handleTemplateNew(%v) exit code = %d, want 1 (err=%v)", tc.args, code, err)
			}
		})
	}
}

func TestTemplateNewCreatesSkeleton(t *testing.T) {
	t.Chdir(t.TempDir())

	var err error
	out := captureOutput(func() {
		err = handleTemplateNew([]string{"zz-my-flow"})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Fatalf("handleTemplateNew exit code = %d, want 0 (err=%v)", code, err)
	}
	if !strings.Contains(out, "已创建模板骨架") {
		t.Errorf("expected creation message, got:\n%s", out)
	}
	data, readErr := os.ReadFile(filepath.Join("zz-my-flow", "workflow.yaml"))
	if readErr != nil {
		t.Fatalf("skeleton workflow.yaml not created: %v", readErr)
	}
	if !strings.Contains(string(data), "name: zz-my-flow") {
		t.Errorf("skeleton does not mention the template name:\n%s", data)
	}

	// Creating the same template again fails: the directory already exists.
	var errAgain error
	captureOutput(func() {
		errAgain = handleTemplateNew([]string{"zz-my-flow"})
	})
	if code := exitCodeForErr(errAgain); code != 1 {
		t.Errorf("second handleTemplateNew exit code = %d, want 1 (err=%v)", code, errAgain)
	}
}

func TestTemplateCloneArgErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no args", nil},
		{"missing dest", []string{"devops-infra/zz-seed-easy"}},
		{"traversal dest", []string{"devops-infra/zz-seed-easy", "../evil"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			captureOutput(func() {
				err = handleTemplateClone(tc.args)
			})
			if code := exitCodeForErr(err); code != 1 {
				t.Errorf("handleTemplateClone(%v) exit code = %d, want 1 (err=%v)", tc.args, code, err)
			}
		})
	}
}

func TestTemplateCloneSeeded(t *testing.T) {
	seedTemplatesRegistry(t,
		seedTemplate{ID: "devops-infra/zz-seed-easy", Workflow: seedEasyWorkflow},
		// Registered, but its directory has no workflow.yaml.
		seedTemplate{ID: "devops-infra/zz-seed-noyaml"},
	)
	t.Chdir(t.TempDir())

	t.Run("by full id", func(t *testing.T) {
		var err error
		captureOutput(func() {
			err = handleTemplateClone([]string{"devops-infra/zz-seed-easy", "zz-clone-dest"})
		})
		if code := exitCodeForErr(err); code != 0 {
			t.Fatalf("handleTemplateClone exit code = %d, want 0 (err=%v)", code, err)
		}
		data, readErr := os.ReadFile(filepath.Join("zz-clone-dest", "workflow.yaml"))
		if readErr != nil {
			t.Fatalf("cloned workflow.yaml not created: %v", readErr)
		}
		if string(data) != seedEasyWorkflow {
			t.Errorf("cloned workflow.yaml content mismatch:\n%s", data)
		}
	})

	t.Run("fuzzy by name suffix", func(t *testing.T) {
		var err error
		captureOutput(func() {
			err = handleTemplateClone([]string{"zz-seed-easy", "zz-fuzzy-dest"})
		})
		if code := exitCodeForErr(err); code != 0 {
			t.Fatalf("handleTemplateClone (fuzzy) exit code = %d, want 0 (err=%v)", code, err)
		}
		if _, statErr := os.Stat(filepath.Join("zz-fuzzy-dest", "workflow.yaml")); statErr != nil {
			t.Errorf("fuzzy clone did not create workflow.yaml: %v", statErr)
		}
	})

	t.Run("unknown source", func(t *testing.T) {
		var err error
		out := captureOutput(func() {
			err = handleTemplateClone([]string{"zz-no-such-template", "zz-dest-a"})
		})
		if code := exitCodeForErr(err); code != 1 {
			t.Errorf("handleTemplateClone (unknown) exit code = %d, want 1 (err=%v)", code, err)
		}
		if !strings.Contains(out, "未找到模板：zz-no-such-template") {
			t.Errorf("expected 'template not found' message, got:\n%s", out)
		}
	})

	t.Run("source without workflow.yaml", func(t *testing.T) {
		var err error
		captureOutput(func() {
			err = handleTemplateClone([]string{"devops-infra/zz-seed-noyaml", "zz-dest-b"})
		})
		if code := exitCodeForErr(err); code != 1 {
			t.Errorf("handleTemplateClone (no workflow.yaml) exit code = %d, want 1 (err=%v)", code, err)
		}
	})

	t.Run("dest already exists", func(t *testing.T) {
		if err := os.MkdirAll("zz-exists", 0o755); err != nil {
			t.Fatal(err)
		}
		var err error
		captureOutput(func() {
			err = handleTemplateClone([]string{"devops-infra/zz-seed-easy", "zz-exists"})
		})
		if code := exitCodeForErr(err); code != 1 {
			t.Errorf("handleTemplateClone (existing dest) exit code = %d, want 1 (err=%v)", code, err)
		}
	})
}

// chdirIntoDeletedDir moves the test into a directory that is then removed:
// every relative-path mkdir/write under it fails with ENOENT, which reaches
// the MkdirAll error branches without depending on filesystem permissions
// (the test binary may run as root, where chmod-based tricks are no-ops).
func chdirIntoDeletedDir(t *testing.T) {
	t.Helper()
	parent := t.TempDir()
	child := filepath.Join(parent, "gone")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatalf("create doomed cwd: %v", err)
	}
	t.Chdir(child)
	if err := os.RemoveAll(parent); err != nil {
		t.Fatalf("remove doomed cwd: %v", err)
	}
}

func TestTemplateNewMkdirAllFails(t *testing.T) {
	chdirIntoDeletedDir(t)

	var err error
	out := captureOutput(func() {
		err = handleTemplateNew([]string{"zz-flow"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("handleTemplateNew(deleted cwd) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(out, "创建目录失败") {
		t.Errorf("expected directory-creation failure, got:\n%s", out)
	}
}

func TestTemplateCloneMkdirAllFails(t *testing.T) {
	seedTemplatesRegistry(t,
		seedTemplate{ID: "devops-infra/zz-seed-easy", Workflow: seedEasyWorkflow},
	)
	chdirIntoDeletedDir(t)

	var err error
	out := captureOutput(func() {
		err = handleTemplateClone([]string{"devops-infra/zz-seed-easy", "zz-dest"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("handleTemplateClone(deleted cwd) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(out, "创建目录失败") {
		t.Errorf("expected directory-creation failure, got:\n%s", out)
	}
}
