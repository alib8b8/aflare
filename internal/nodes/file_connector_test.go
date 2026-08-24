// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​‌‌‌​‌​‌‌​​​‌​​‌‌‌​‌‌‌​​‌​​‌​‌​​‌‌‌​​​‌​​‌‌​‌‌​‌​​‌‌‌‌​​​​‌‌‌‌​‌​​​​​​​​​​​​​​​​‌​‌​​‌‌​​​‌‌​​‌​⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation; either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package nodes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/connector"
)

// fileConnectorFixture creates a directory tree under a temp root and
// registers a connector pointing at it.
//
//	root/
//	  a.md        "alpha"
//	  b.txt       "bravo"
//	  sub/c.md    "charlie"
//	  .hidden.md  (must never be listed)
func fileConnectorFixture(t *testing.T, name, connType string, mutate func(*connector.Spec)) string {
	t.Helper()
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "sub"))
	writeFile(t, filepath.Join(root, "a.md"), "alpha")
	writeFile(t, filepath.Join(root, "b.txt"), "bravo")
	writeFile(t, filepath.Join(root, "sub", "c.md"), "charlie")
	writeFile(t, filepath.Join(root, ".hidden.md"), "secret")

	spec := connector.Spec{Name: name, Type: connType, Root: root}
	if mutate != nil {
		mutate(&spec)
	}
	setupConnectorRegistry(t, spec)
	return root
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func boolPtr(b bool) *bool { return &b }

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// --- file_read ---

func TestFileRead_ConnectorReadWithinRoot(t *testing.T) {
	fileConnectorFixture(t, "docs", connector.TypeFiles, nil)

	n := &FileReadNode{}
	out, err := n.Execute(t.Context(), "", map[string]string{
		"connector": "docs",
		"path":      "a.md",
		"redact":    "false",
	})
	if err != nil {
		t.Fatalf("read via connector: %v", err)
	}
	if out != "alpha" {
		t.Errorf("content = %q, want alpha", out)
	}

	// Nested path inside the root works too.
	out, err = n.Execute(t.Context(), "", map[string]string{
		"connector": "docs",
		"path":      "sub/c.md",
		"redact":    "false",
	})
	if err != nil {
		t.Fatalf("nested read via connector: %v", err)
	}
	if out != "charlie" {
		t.Errorf("nested content = %q, want charlie", out)
	}
}

func TestFileRead_ConnectorRejectsEscape(t *testing.T) {
	fileConnectorFixture(t, "docs", connector.TypeFiles, nil)
	n := &FileReadNode{}

	for _, path := range []string{"../escape.md", "/etc/passwd"} {
		_, err := n.Execute(t.Context(), "", map[string]string{"connector": "docs", "path": path})
		if err == nil {
			t.Errorf("path %q should be rejected", path)
			continue
		}
		if !strings.Contains(err.Error(), "path validation failed") {
			t.Errorf("path %q: error %q should be a path validation failure", path, err.Error())
		}
	}
}

func TestFileRead_ConnectorIncludeAllowlist(t *testing.T) {
	fileConnectorFixture(t, "docs", connector.TypeFiles, func(s *connector.Spec) {
		s.Include = []string{"*.md"}
	})
	n := &FileReadNode{}

	if _, err := n.Execute(t.Context(), "", map[string]string{"connector": "docs", "path": "a.md", "redact": "false"}); err != nil {
		t.Errorf("allowlisted .md read failed: %v", err)
	}
	_, err := n.Execute(t.Context(), "", map[string]string{"connector": "docs", "path": "b.txt"})
	if err == nil || !strings.Contains(err.Error(), "does not allow reading") {
		t.Errorf("non-allowlisted .txt read should fail with allowlist error, got %v", err)
	}
}

func TestFileRead_NotesConnectorDefaultsToMarkdown(t *testing.T) {
	fileConnectorFixture(t, "vault", connector.TypeNotes, nil)
	n := &FileReadNode{}

	if _, err := n.Execute(t.Context(), "", map[string]string{"connector": "vault", "path": "a.md", "redact": "false"}); err != nil {
		t.Errorf("notes connector should read .md: %v", err)
	}
	_, err := n.Execute(t.Context(), "", map[string]string{"connector": "vault", "path": "b.txt"})
	if err == nil || !strings.Contains(err.Error(), "does not allow reading") {
		t.Errorf("notes connector should reject .txt by default, got %v", err)
	}
}

func TestFileRead_ConnectorMaxBytesCeiling(t *testing.T) {
	fileConnectorFixture(t, "docs", connector.TypeFiles, func(s *connector.Spec) {
		s.MaxBytes = 3 // "alpha" is 5 bytes
	})
	n := &FileReadNode{}

	_, err := n.Execute(t.Context(), "", map[string]string{"connector": "docs", "path": "a.md"})
	if err == nil || !strings.Contains(err.Error(), "file too large") {
		t.Errorf("max_bytes ceiling should reject the 5-byte file, got %v", err)
	}
}

func TestFileRead_ConnectorWrongType(t *testing.T) {
	setupConnectorRegistry(t, sqliteConnectorSpec("db"))
	n := &FileReadNode{}

	_, err := n.Execute(t.Context(), "", map[string]string{"connector": "db", "path": "a.md"})
	if err == nil || !strings.Contains(err.Error(), "file nodes expect files/notes connectors") {
		t.Errorf("database connector should be rejected by file_read with a hint, got %v", err)
	}
}

// --- file_write ---

func TestFileWrite_ReadOnlyConnectorRejected(t *testing.T) {
	fileConnectorFixture(t, "docs", connector.TypeFiles, nil)
	n := &FileWriteNode{}

	_, err := n.Execute(t.Context(), "new content", map[string]string{"connector": "docs", "path": "new.md"})
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Errorf("read-only connector write should be rejected, got %v", err)
	}
}

func TestFileWrite_WritableConnector(t *testing.T) {
	root := fileConnectorFixture(t, "docs", connector.TypeFiles, func(s *connector.Spec) {
		s.ReadOnly = boolPtr(false)
	})
	n := &FileWriteNode{}

	if _, err := n.Execute(t.Context(), "hello world", map[string]string{"connector": "docs", "path": "new.md"}); err != nil {
		t.Fatalf("writable connector write failed: %v", err)
	}
	if got := readFile(t, filepath.Join(root, "new.md")); got != "hello world" {
		t.Errorf("written content = %q, want hello world", got)
	}

	// Nested write into an existing subdirectory.
	if _, err := n.Execute(t.Context(), "nested", map[string]string{"connector": "docs", "path": "sub/new.md"}); err != nil {
		t.Fatalf("nested write failed: %v", err)
	}
	if got := readFile(t, filepath.Join(root, "sub", "new.md")); got != "nested" {
		t.Errorf("nested content = %q, want nested", got)
	}
}

func TestFileWrite_ConnectorRejectsEscapeAndBlacklist(t *testing.T) {
	fileConnectorFixture(t, "docs", connector.TypeFiles, func(s *connector.Spec) {
		s.ReadOnly = boolPtr(false)
	})
	n := &FileWriteNode{}

	_, err := n.Execute(t.Context(), "x", map[string]string{"connector": "docs", "path": "../escape.md"})
	if err == nil || !strings.Contains(err.Error(), "path validation failed") {
		t.Errorf("traversal write should be rejected, got %v", err)
	}
	// The node's sensitive-extension blacklist still applies inside
	// connector roots: .env stays unwritable.
	_, err = n.Execute(t.Context(), "x", map[string]string{"connector": "docs", "path": "creds.env"})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Errorf(".env write via connector should be rejected, got %v", err)
	}
}

func TestFileWrite_ConnectorIncludeAllowlist(t *testing.T) {
	fileConnectorFixture(t, "docs", connector.TypeFiles, func(s *connector.Spec) {
		s.ReadOnly = boolPtr(false)
		s.Include = []string{"*.md"}
	})
	n := &FileWriteNode{}

	if _, err := n.Execute(t.Context(), "ok", map[string]string{"connector": "docs", "path": "note.md"}); err != nil {
		t.Errorf("allowlisted write failed: %v", err)
	}
	_, err := n.Execute(t.Context(), "x", map[string]string{"connector": "docs", "path": "data.txt"})
	if err == nil || !strings.Contains(err.Error(), "does not allow writing") {
		t.Errorf("non-allowlisted write should be rejected, got %v", err)
	}
}

// --- files_list ---

func TestFilesList_Basic(t *testing.T) {
	fileConnectorFixture(t, "docs", connector.TypeFiles, nil)
	n := &FilesListNode{}

	out, err := n.Execute(t.Context(), "", map[string]string{"connector": "docs"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	// Dotfiles/dot-dirs excluded; files sorted by path.
	want := `"files": [
    {
      "path": "a.md",
      "bytes": 5
    },
    {
      "path": "b.txt",
      "bytes": 5
    },
    {
      "path": "sub/c.md",
      "bytes": 7
    }
  ]`
	if !strings.Contains(out, want) {
		t.Errorf("list output missing expected entries:\ngot:\n%s\nwant fragment:\n%s", out, want)
	}
	if !strings.Contains(out, `"count": 3`) {
		t.Errorf("count should be 3, output: %s", out)
	}
}

func TestFilesList_PatternFilter(t *testing.T) {
	fileConnectorFixture(t, "docs", connector.TypeFiles, nil)
	n := &FilesListNode{}

	out, err := n.Execute(t.Context(), "", map[string]string{"connector": "docs", "pattern": "**/*.md"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(out, "a.md") || !strings.Contains(out, "sub/c.md") {
		t.Errorf("*.md at any depth should list a.md and sub/c.md, got: %s", out)
	}
	if strings.Contains(out, "b.txt") {
		t.Errorf("pattern should exclude b.txt, got: %s", out)
	}

	// Single-level glob must not cross directories.
	out, err = n.Execute(t.Context(), "", map[string]string{"connector": "docs", "pattern": "*.md"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(out, "a.md") || strings.Contains(out, "sub/c.md") {
		t.Errorf("single-level *.md should list only a.md, got: %s", out)
	}
}

func TestFilesList_NotesDefaultMarkdownOnly(t *testing.T) {
	fileConnectorFixture(t, "vault", connector.TypeNotes, nil)
	n := &FilesListNode{}

	out, err := n.Execute(t.Context(), "", map[string]string{"connector": "vault"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(out, "a.md") || !strings.Contains(out, "sub/c.md") {
		t.Errorf("notes list should include markdown files, got: %s", out)
	}
	if strings.Contains(out, "b.txt") {
		t.Errorf("notes list should exclude b.txt, got: %s", out)
	}
}

func TestFilesList_RejectsBadPatternAndWrongType(t *testing.T) {
	fileConnectorFixture(t, "docs", connector.TypeFiles, nil)
	n := &FilesListNode{}

	for _, pattern := range []string{"../**", "/etc/*"} {
		_, err := n.Execute(t.Context(), "", map[string]string{"connector": "docs", "pattern": pattern})
		if err == nil || !strings.Contains(err.Error(), "relative to the connector root") {
			t.Errorf("pattern %q should be rejected, got %v", pattern, err)
		}
	}

	setupConnectorRegistry(t, sqliteConnectorSpec("db"))
	_, err := n.Execute(t.Context(), "", map[string]string{"connector": "db"})
	if err == nil || !strings.Contains(err.Error(), "file nodes expect files/notes connectors") {
		t.Errorf("database connector should be rejected by files_list, got %v", err)
	}

	_, err = n.Execute(t.Context(), "", map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "connector parameter is required") {
		t.Errorf("missing connector param should error, got %v", err)
	}
}

func TestFilesList_MaxEntries(t *testing.T) {
	fileConnectorFixture(t, "docs", connector.TypeFiles, nil)
	n := &FilesListNode{}

	out, err := n.Execute(t.Context(), "", map[string]string{"connector": "docs", "max_entries": "2"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(out, `"count": 2`) {
		t.Errorf("count should be capped at 2, got: %s", out)
	}
	if !strings.Contains(out, `"truncated": true`) {
		t.Errorf("truncated should be true, got: %s", out)
	}
}

func TestFilesList_SkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.md"), "alpha")

	// A symlink inside the root pointing outside must never be listed.
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.md"), "secret")
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(root, "link.md")); err != nil {
		t.Skipf("symlink creation failed: %v", err)
	}

	setupConnectorRegistry(t, connector.Spec{Name: "docs", Type: connector.TypeFiles, Root: root})
	n := &FilesListNode{}

	out, err := n.Execute(t.Context(), "", map[string]string{"connector": "docs"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if strings.Contains(out, "link.md") {
		t.Errorf("symlink must not be listed, got: %s", out)
	}
}

func TestMatchRel(t *testing.T) {
	cases := []struct {
		pattern, rel string
		want         bool
	}{
		{"**/*", "a.md", true},
		{"**/*", "sub/c.md", true},
		{"**", "sub/c.md", true},
		{"", "a.md", true},
		{"*.md", "a.md", true},
		{"*.md", "sub/a.md", false},
		{"**/*.md", "sub/a.md", true},
		{"**/*.md", "a.md", true},
		{"**/*.md", "sub/deep/a.md", true},
		{"**/*.md", "a.txt", false},
		{"sub/*.md", "sub/a.md", true},
		{"sub/*.md", "other/a.md", false},
		{"a?c.md", "abc.md", true},
	}
	for _, c := range cases {
		if got := matchRel(c.pattern, c.rel); got != c.want {
			t.Errorf("matchRel(%q, %q) = %v, want %v", c.pattern, c.rel, got, c.want)
		}
	}
}
