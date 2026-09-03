// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌​​​​​​​​‌‌​​‌​​​​​​​‌​​‌​‌‌‌‌​​​​​‌‌​‌‌​​‌‌​‌​‌‌‌​​​‌​​‌​‌​‌‌​‌​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌‌​​​‌‌‌‌⁠
// aflare​‌​​​​​‌​​‌‌​​‌‌‌​​​​​‌​​​​‌‌​​​​‌​‌​​‌‌​​‌​​​‌​‌​​​​​​‌‌​​​​​‌‌​‌‌​‌‌‌​‌​​​‌‌​‌‌​​​​‌​​​‌​​​‌​​‌‌‌​​​‌​​​‌​​‌‌‌​​​​​‌​​‌​‌​​​‌‌​‌​​‌‌‌‌‌‌​​‌​​​​​‌​​‌‌​‌​‌‌‌​‌‌​​‌​​‌‌‌​​‌​‌‌​​​​​‌‌​​‌​​‌​‌​‌‌​‌​‌‌‌​‌​‌​‌​‌‌‌‌‌​
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

package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomicRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := WriteFileAtomic(path, []byte(`{"a":1}`), 0600); err != nil {
		t.Fatalf("first write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != `{"a":1}` {
		t.Fatalf("content = %q, want %q", got, `{"a":1}`)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("perm = %o, want 600", info.Mode().Perm())
	}
}

// Overwrite must leave exactly one file: the new contents, no leftover
// temp files from either write.
func TestWriteFileAtomicOverwriteNoTempLeftover(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	for _, content := range []string{`{"v":1}`, `{"v":2}`, `{"v":3}`} {
		if err := WriteFileAtomic(path, []byte(content), 0600); err != nil {
			t.Fatalf("write %q: %v", content, err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != `{"v":3}` {
		t.Fatalf("content = %q, want the last write", got)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "state.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory contains leftover temp files: %v", names)
	}
}

// The temp file is chmod'ed to perm BEFORE the rename, so the target never
// briefly appears with looser permissions than requested.
func TestWriteFileAtomicAppliesPerm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mode-check")

	if err := WriteFileAtomic(path, []byte("x"), 0640); err != nil {
		t.Fatalf("write: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0640 {
		t.Errorf("perm = %o, want 640", info.Mode().Perm())
	}
}

func TestPreserveCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")
	if err := os.WriteFile(path, []byte("truncated {"), 0600); err != nil {
		t.Fatal(err)
	}

	preserved := PreserveCorrupt(path)
	if preserved == "" {
		t.Fatal("PreserveCorrupt returned empty (rename failed)")
	}
	if !strings.HasPrefix(filepath.Base(preserved), "checkpoint.json.corrupt-") {
		t.Errorf("preserved name = %q, want checkpoint.json.corrupt-<ts>", filepath.Base(preserved))
	}
	got, err := os.ReadFile(preserved)
	if err != nil {
		t.Fatalf("preserved file unreadable: %v", err)
	}
	if string(got) != "truncated {" {
		t.Errorf("preserved content = %q, want the original bytes", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("original path still exists after preservation: %v", err)
	}
}

// Preserving a missing file reports failure (empty string) instead of
// panicking — callers use it on a best-effort basis.
func TestPreserveCorruptMissing(t *testing.T) {
	dir := t.TempDir()
	if got := PreserveCorrupt(filepath.Join(dir, "nope.json")); got != "" {
		t.Errorf("PreserveCorrupt on missing file = %q, want empty", got)
	}
}
