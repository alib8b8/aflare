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

package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

// ── LoadDir Tests ──────────────────────────────────────────────────────────
// Note: LoadPlugin requires compiled .so files (plugin.Open) and cannot be
// unit tested in standard Go test environments. LoadDir's directory scanning
// and filtering logic is tested below.

func TestLoadDir_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	pm := NewPluginManager()

	loaded, err := LoadDir(dir, pm)
	if err != nil {
		t.Fatalf("LoadDir failed: %v", err)
	}
	if loaded != 0 {
		t.Errorf("expected 0 loaded, got %d", loaded)
	}
}

func TestLoadDir_NonExistentDirectory(t *testing.T) {
	pm := NewPluginManager()
	loaded, err := LoadDir("/nonexistent/plugin/dir/12345", pm)
	if err != nil {
		t.Errorf("LoadDir should not error on non-existent dir, got: %v", err)
	}
	if loaded != 0 {
		t.Errorf("expected 0 loaded from non-existent dir, got %d", loaded)
	}
}

func TestLoadDir_SkipsNonSOFiles(t *testing.T) {
	dir := t.TempDir()
	pm := NewPluginManager()

	// Create non-.so files
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("docs"), 0644)
	os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	loaded, err := LoadDir(dir, pm)
	if err != nil {
		t.Fatalf("LoadDir failed: %v", err)
	}
	if loaded != 0 {
		t.Errorf("expected 0 loaded (no .so files), got %d", loaded)
	}
}

func TestLoadDir_SOFileLoadAttempt(t *testing.T) {
	dir := t.TempDir()
	pm := NewPluginManager()

	// Create a file with .so extension — LoadDir will attempt plugin.Open
	// which will fail because it's not a real plugin, but the error is logged
	// and loading continues. This tests the error handling path.
	soPath := filepath.Join(dir, "fake_plugin.so")
	os.WriteFile(soPath, []byte("not a real plugin"), 0644)

	loaded, err := LoadDir(dir, pm)
	if loaded != 0 {
		t.Errorf("expected 0 loaded (invalid .so), got %d", loaded)
	}
	if err == nil {
		t.Error("expected error from LoadDir when .so fails to load, got nil")
	}
}

func TestLoadDir_MultipleSOFiles(t *testing.T) {
	dir := t.TempDir()
	pm := NewPluginManager()

	os.WriteFile(filepath.Join(dir, "a.so"), []byte("invalid"), 0644)
	os.WriteFile(filepath.Join(dir, "b.so"), []byte("invalid"), 0644)
	os.WriteFile(filepath.Join(dir, "c.txt"), []byte("skip"), 0644)

	loaded, err := LoadDir(dir, pm)
	if loaded != 0 {
		t.Errorf("expected 0 loaded (invalid .so files), got %d", loaded)
	}
	if err == nil {
		t.Error("expected error from LoadDir when .so files fail to load, got nil")
	}
}

// ── DefaultPluginDir ────────────────────────────────────────────────────────

func TestDefaultPluginDir(t *testing.T) {
	dir := DefaultPluginDir()
	if dir == "" {
		t.Error("DefaultPluginDir returned empty string")
	}
	// Should end with plugins directory
	if filepath.Base(dir) != "plugins" {
		t.Errorf("expected base name 'plugins', got %q", filepath.Base(dir))
	}
}