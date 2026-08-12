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

package filewatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ── NewWatcher Validation ──────────────────────────────────────────────────

func TestNewWatcher_ValidDirectory(t *testing.T) {
	dir := t.TempDir()
	events := make(chan Event, 10)
	w, err := NewWatcher(dir, 100*time.Millisecond, events)
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	if w == nil {
		t.Fatal("NewWatcher returned nil")
	}
	if w.rootPath != dir {
		t.Errorf("rootPath mismatch: expected %q, got %q", dir, w.rootPath)
	}
}

func TestNewWatcher_NonExistentPath(t *testing.T) {
	events := make(chan Event, 10)
	_, err := NewWatcher("/nonexistent/path/12345", 100*time.Millisecond, events)
	if err == nil {
		t.Error("expected error for non-existent path, got nil")
	}
}

func TestNewWatcher_NotADirectory(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	events := make(chan Event, 10)
	_, err := NewWatcher(filePath, 100*time.Millisecond, events)
	if err == nil {
		t.Error("expected error for non-directory path, got nil")
	}
}

func TestNewWatcher_DefaultInterval(t *testing.T) {
	dir := t.TempDir()
	events := make(chan Event, 10)
	w, err := NewWatcher(dir, 0, events) // zero interval → default
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	if w.interval != DefaultPollInterval {
		t.Errorf("expected default interval %v, got %v", DefaultPollInterval, w.interval)
	}
}

// ── Diff Logic ─────────────────────────────────────────────────────────────

func TestDiff_Create(t *testing.T) {
	w := &Watcher{}
	old := map[string]fileMeta{}
	cur := map[string]fileMeta{
		"newfile.txt": {ModTime: time.Now(), Size: 100},
	}

	events := w.diff(old, cur)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "create" {
		t.Errorf("expected 'create' event, got %q", events[0].Type)
	}
	if events[0].Path != "newfile.txt" {
		t.Errorf("expected path 'newfile.txt', got %q", events[0].Path)
	}
	if events[0].Size != 100 {
		t.Errorf("expected size 100, got %d", events[0].Size)
	}
}

func TestDiff_Modify(t *testing.T) {
	w := &Watcher{}
	now := time.Now()
	old := map[string]fileMeta{
		"file.txt": {ModTime: now.Add(-1 * time.Hour), Size: 50},
	}
	cur := map[string]fileMeta{
		"file.txt": {ModTime: now, Size: 100},
	}

	events := w.diff(old, cur)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "modify" {
		t.Errorf("expected 'modify' event, got %q", events[0].Type)
	}
	if events[0].Path != "file.txt" {
		t.Errorf("expected path 'file.txt', got %q", events[0].Path)
	}
	if events[0].Size != 100 {
		t.Errorf("expected size 100, got %d", events[0].Size)
	}
}

func TestDiff_Modify_SizeOnly(t *testing.T) {
	w := &Watcher{}
	now := time.Now()
	old := map[string]fileMeta{
		"file.txt": {ModTime: now, Size: 50},
	}
	cur := map[string]fileMeta{
		"file.txt": {ModTime: now, Size: 100},
	}

	events := w.diff(old, cur)
	if len(events) != 1 {
		t.Fatalf("expected 1 event for size change, got %d", len(events))
	}
	if events[0].Type != "modify" {
		t.Errorf("expected 'modify' event for size change, got %q", events[0].Type)
	}
}

func TestDiff_Delete(t *testing.T) {
	w := &Watcher{}
	old := map[string]fileMeta{
		"deleted.txt": {ModTime: time.Now(), Size: 100},
	}
	cur := map[string]fileMeta{}

	events := w.diff(old, cur)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "delete" {
		t.Errorf("expected 'delete' event, got %q", events[0].Type)
	}
	if events[0].Path != "deleted.txt" {
		t.Errorf("expected path 'deleted.txt', got %q", events[0].Path)
	}
}

func TestDiff_NoChange(t *testing.T) {
	w := &Watcher{}
	now := time.Now()
	state := map[string]fileMeta{
		"file.txt": {ModTime: now, Size: 100},
	}

	events := w.diff(state, state)
	if len(events) != 0 {
		t.Errorf("expected 0 events for no change, got %d", len(events))
	}
}

func TestDiff_MultipleEvents(t *testing.T) {
	w := &Watcher{}
	now := time.Now()
	old := map[string]fileMeta{
		"a.txt": {ModTime: now, Size: 100},
		"b.txt": {ModTime: now, Size: 200},
	}
	cur := map[string]fileMeta{
		"a.txt": {ModTime: now.Add(1 * time.Hour), Size: 150}, // modified
		"c.txt": {ModTime: now, Size: 300},                    // created
		// b.txt deleted
	}

	events := w.diff(old, cur)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Events should be sorted by type then path
	expected := []struct {
		Type string
		Path string
	}{
		{"create", "c.txt"},
		{"delete", "b.txt"},
		{"modify", "a.txt"},
	}
	for i, e := range expected {
		if events[i].Type != e.Type || events[i].Path != e.Path {
			t.Errorf("event[%d]: expected %s:%s, got %s:%s", i, e.Type, e.Path, events[i].Type, events[i].Path)
		}
	}
}

func TestDiff_EmptySnapshots(t *testing.T) {
	w := &Watcher{}
	events := w.diff(map[string]fileMeta{}, map[string]fileMeta{})
	if len(events) != 0 {
		t.Errorf("expected 0 events for empty snapshots, got %d", len(events))
	}
}

// ── Snapshot ───────────────────────────────────────────────────────────────

func TestSnapshot_Basic(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world"), 0644)

	w := &Watcher{rootPath: dir}
	snap, err := w.snapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if len(snap) != 2 {
		t.Errorf("expected 2 files in snapshot, got %d", len(snap))
	}
	if _, ok := snap["a.txt"]; !ok {
		t.Error("expected a.txt in snapshot")
	}
	if _, ok := snap["b.txt"]; !ok {
		t.Error("expected b.txt in snapshot")
	}
}

func TestSnapshot_Subdirectory(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	os.Mkdir(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "c.txt"), []byte("nested"), 0644)

	w := &Watcher{rootPath: dir}
	snap, err := w.snapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if len(snap) != 1 {
		t.Errorf("expected 1 file in snapshot, got %d", len(snap))
	}
	if _, ok := snap["sub/c.txt"]; !ok {
		t.Errorf("expected 'sub/c.txt' in snapshot, got keys: %v", mapKeys(snap))
	}
}

func TestSnapshot_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)

	w := &Watcher{rootPath: dir}
	snap, err := w.snapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if len(snap) != 1 {
		t.Errorf("expected 1 file (directory skipped), got %d", len(snap))
	}
	if _, ok := snap["file.txt"]; !ok {
		t.Error("expected file.txt in snapshot")
	}
}

func TestSnapshot_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	w := &Watcher{rootPath: dir}
	snap, err := w.snapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if len(snap) != 0 {
		t.Errorf("expected 0 files in empty dir, got %d", len(snap))
	}
}

// ── FormatEvent ────────────────────────────────────────────────────────────

func TestFormatEvent(t *testing.T) {
	e := Event{
		Type:      "create",
		Path:      "test/file.txt",
		Timestamp: time.Date(2026, 1, 15, 14, 30, 0, 0, time.UTC),
		Size:      1024,
	}
	got := FormatEvent(e)
	expected := "[filewatch] create: test/file.txt (size: 1024, at: 14:30:00)"
	if got != expected {
		t.Errorf("FormatEvent: expected %q, got %q", expected, got)
	}
}

func TestFormatEvent_Delete(t *testing.T) {
	e := Event{
		Type:      "delete",
		Path:      "removed.txt",
		Timestamp: time.Date(2026, 6, 1, 9, 5, 30, 0, time.UTC),
		Size:      0,
	}
	got := FormatEvent(e)
	expected := "[filewatch] delete: removed.txt (size: 0, at: 09:05:30)"
	if got != expected {
		t.Errorf("FormatEvent: expected %q, got %q", expected, got)
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

func mapKeys(m map[string]fileMeta) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
