// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌​​‌‌​‌‌‌‌‌‌‌​​​​​‌​‌​​​‌​‌​​​​​​​​​​‌‌‌‌‌‌‌‌‌​​​​​​​​​​​​​​​​‌‌‌​​‌​‌‌‌‌‌‌‌​‌⁠
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

package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestFSStore constructs an FSStore backed by per-test temp directories,
// avoiding any I/O to the user's home (~/.aflare). All fields are
// constructed directly because NewFSStore would touch the user's home dir
// via os.UserHomeDir().
func newTestFSStore(t *testing.T) *FSStore {
	t.Helper()
	return &FSStore{
		sessionMgr: NewSessionMemoryManager(t.TempDir(), 10, 100),
		profileMgr: &UserProfileManager{
			profiles:   make(map[string]*UserProfile),
			storageDir: t.TempDir(),
			mu:         sync.RWMutex{},
			maxPerUser: defaultMaxPrefsPerUser,
		},
		skillsDir: filepath.Join(t.TempDir(), "skills"),
		kgPath:    filepath.Join(t.TempDir(), "knowledge_graph.json"),
	}
}

func TestNewFSStore_NonNil(t *testing.T) {
	fs := NewFSStore("")
	if fs == nil {
		t.Fatal("expected non-nil FSStore")
	}
	if fs.skillsDir == "" {
		t.Error("expected non-empty skillsDir")
	}
	if fs.kgPath == "" {
		t.Error("expected non-empty kgPath")
	}
	if fs.sessionMgr == nil {
		t.Error("expected non-nil sessionMgr")
	}
	if fs.profileMgr == nil {
		t.Error("expected non-nil profileMgr")
	}
}

func TestNewFSStore_CustomSkillsDir(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "my-skills")
	fs := NewFSStore(custom)
	if fs.skillsDir != custom {
		t.Errorf("expected skillsDir %q, got %q", custom, fs.skillsDir)
	}
}

func TestValidateFSPath(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"/mem/short/key", false},
		{"/mem", false},
		{"/", false},
		{"/profile/coding_style", false},
		{"/kg/entities/go", false},
		{"/skills/my-skill", false},
		{"", true},
		{"mem/short", true},   // missing leading /
		{"/mem/../etc", true}, // contains ..
		{"/mem//short", true}, // contains //
		{"relative/path", true},
	}
	for _, tt := range tests {
		err := validateFSPath(tt.path)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateFSPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
		}
	}
}

func TestSplitFSPath(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"/mem/short/key", []string{"mem", "short", "key"}},
		{"/mem/", []string{"mem"}},
		{"/", []string{""}},
		{"/mem/short/", []string{"mem", "short"}},
		{"mem/short", []string{"mem", "short"}},
	}
	for _, tt := range tests {
		got := splitFSPath(tt.path)
		if len(got) != len(tt.expected) {
			t.Errorf("splitFSPath(%q) = %v, want %v", tt.path, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("splitFSPath(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.expected[i])
			}
		}
	}
}

// --- memory backend tests ---

func TestFSStore_MemWriteRead(t *testing.T) {
	fs := newTestFSStore(t)

	if err := fs.Write("/mem/short/greeting", "hello world"); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := fs.Read("/mem/short/greeting")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got != "hello world" {
		t.Errorf("expected 'hello world', got %q", got)
	}
}

func TestFSStore_MemWriteReadAcrossLevels(t *testing.T) {
	fs := newTestFSStore(t)

	for _, level := range []string{"short", "medium", "long"} {
		path := "/mem/" + level + "/k"
		if err := fs.Write(path, "value-"+level); err != nil {
			t.Fatalf("Write(%s) failed: %v", path, err)
		}
		got, err := fs.Read(path)
		if err != nil {
			t.Fatalf("Read(%s) failed: %v", path, err)
		}
		if got != "value-"+level {
			t.Errorf("expected 'value-%s', got %q", level, got)
		}
	}
}

func TestFSStore_MemReadLevelMismatch(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/mem/short/k1", "v1"); err != nil {
		t.Fatal(err)
	}
	// Reading the same key at a different level should fail.
	if _, err := fs.Read("/mem/medium/k1"); err == nil {
		t.Error("expected error when reading key at wrong level")
	}
}

func TestFSStore_MemListByLevel(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/mem/short/k1", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Write("/mem/short/k2", "v2"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Write("/mem/long/k3", "v3"); err != nil {
		t.Fatal(err)
	}

	entries, err := fs.List("/mem/short")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 short-term entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Category != "mem" {
			t.Errorf("expected category mem, got %s", e.Category)
		}
		if e.Type != "file" {
			t.Errorf("expected type file, got %s", e.Type)
		}
	}

	entries, err = fs.List("/mem/long")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 long-term entry, got %d", len(entries))
	}
}

func TestFSStore_MemListTopLevel(t *testing.T) {
	fs := newTestFSStore(t)
	entries, err := fs.List("/mem")
	if err != nil {
		t.Fatalf("List(/mem) failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 level dirs, got %d", len(entries))
	}
	expected := map[string]bool{"/mem/short": true, "/mem/medium": true, "/mem/long": true}
	for _, e := range entries {
		if !expected[e.Path] {
			t.Errorf("unexpected path: %s", e.Path)
		}
		if e.Type != "dir" {
			t.Errorf("expected dir, got %s for %s", e.Type, e.Path)
		}
	}
}

func TestFSStore_MemReadLevelList(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/mem/short/k1", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Write("/mem/short/k2", "v2"); err != nil {
		t.Fatal(err)
	}
	// Reading /mem/short (without key) returns newline-separated keys.
	got, err := fs.Read("/mem/short")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 keys, got %d: %q", len(lines), got)
	}
}

func TestFSStore_MemDelete(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/mem/short/k1", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Delete("/mem/short/k1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := fs.Read("/mem/short/k1"); err == nil {
		t.Error("expected error after delete")
	}
}

func TestFSStore_MemDeleteNotFound(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Delete("/mem/short/nonexistent"); err == nil {
		t.Error("expected error when deleting nonexistent memory")
	}
}

// --- profile backend tests ---

func TestFSStore_ProfileWriteRead(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/profile/coding_style/language", "go"); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	got, err := fs.Read("/profile/coding_style/language")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got != "go" {
		t.Errorf("expected 'go', got %q", got)
	}
}

func TestFSStore_ProfileListCategories(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/profile/coding_style/language", "go"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Write("/profile/output_format/format", "markdown"); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.List("/profile")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) < 2 {
		t.Errorf("expected at least 2 category dirs, got %d", len(entries))
	}
}

func TestFSStore_ProfileListByCategory(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/profile/coding_style/language", "go"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Write("/profile/coding_style/indent", "4"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Write("/profile/output_format/format", "markdown"); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.List("/profile/coding_style")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 coding_style prefs, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Category != "profile" {
			t.Errorf("expected category profile, got %s", e.Category)
		}
	}
}

func TestFSStore_ProfileReadNotFound(t *testing.T) {
	fs := newTestFSStore(t)
	if _, err := fs.Read("/profile/coding_style/nonexistent"); err == nil {
		t.Error("expected error when reading nonexistent preference")
	}
}

func TestFSStore_ProfileDeleteNotSupported(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/profile/coding_style/language", "go"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Delete("/profile/coding_style/language"); err == nil {
		t.Error("expected error when deleting profile (delete not supported)")
	}
}

// --- knowledge graph backend tests ---

func TestFSStore_KGEntityWriteReadJSON(t *testing.T) {
	fs := newTestFSStore(t)
	entityJSON := `{"name":"go","type":"language","properties":{"paradigm":"imperative"}}`
	if err := fs.Write("/kg/entities/go", entityJSON); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	got, err := fs.Read("/kg/entities/go")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if !strings.Contains(got, "go") || !strings.Contains(got, "language") {
		t.Errorf("expected entity JSON with 'go' and 'language', got %q", got)
	}
}

func TestFSStore_KGEntityWriteReadString(t *testing.T) {
	fs := newTestFSStore(t)
	// Non-JSON content is treated as the entity type.
	if err := fs.Write("/kg/entities/python", "language"); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	got, err := fs.Read("/kg/entities/python")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if !strings.Contains(got, "python") || !strings.Contains(got, "language") {
		t.Errorf("expected entity with name=python and type=language, got %q", got)
	}
}

func TestFSStore_KGEntityList(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/kg/entities/go", "language"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Write("/kg/entities/python", "language"); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.List("/kg/entities")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entities, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Category != "kg" {
			t.Errorf("expected category kg, got %s", e.Category)
		}
	}
}

func TestFSStore_KGEntityReadNameList(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/kg/entities/go", "language"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Write("/kg/entities/python", "language"); err != nil {
		t.Fatal(err)
	}
	got, err := fs.Read("/kg/entities")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 entity names, got %d: %q", len(lines), got)
	}
}

func TestFSStore_KGEntityDelete(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/kg/entities/go", "language"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Delete("/kg/entities/go"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := fs.Read("/kg/entities/go"); err == nil {
		t.Error("expected error after delete")
	}
}

func TestFSStore_KGEntityDeleteNotFound(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Delete("/kg/entities/nonexistent"); err == nil {
		t.Error("expected error when deleting nonexistent entity")
	}
}

func TestFSStore_KGEntityReadNotFound(t *testing.T) {
	fs := newTestFSStore(t)
	if _, err := fs.Read("/kg/entities/nonexistent"); err == nil {
		t.Error("expected error when reading nonexistent entity")
	}
}

func TestFSStore_KGRelationWriteRead(t *testing.T) {
	fs := newTestFSStore(t)
	relJSON := `{"from":"go","to":"python","relation":"related_to","confidence":0.8}`
	if err := fs.Write("/kg/relations", relJSON); err != nil {
		t.Fatalf("Write relation failed: %v", err)
	}
	got, err := fs.Read("/kg/relations")
	if err != nil {
		t.Fatalf("Read relations failed: %v", err)
	}
	if !strings.Contains(got, "related_to") {
		t.Errorf("expected relation JSON with 'related_to', got %q", got)
	}
}

func TestFSStore_KGRelationInvalidJSON(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/kg/relations", "not-json"); err == nil {
		t.Error("expected error for invalid relation JSON")
	}
}

func TestFSStore_KGListTopLevel(t *testing.T) {
	fs := newTestFSStore(t)
	entries, err := fs.List("/kg")
	if err != nil {
		t.Fatalf("List(/kg) failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (entities, relations), got %d", len(entries))
	}
	paths := map[string]bool{}
	for _, e := range entries {
		paths[e.Path] = true
	}
	if !paths["/kg/entities"] || !paths["/kg/relations"] {
		t.Errorf("expected /kg/entities and /kg/relations, got %v", paths)
	}
}

func TestFSStore_KGInvalidSubpath(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/kg/unknown/x", "v"); err == nil {
		t.Error("expected error for invalid kg subpath")
	}
	if _, err := fs.Read("/kg/unknown/x"); err == nil {
		t.Error("expected error for invalid kg subpath")
	}
	if _, err := fs.List("/kg/unknown"); err == nil {
		t.Error("expected error for invalid kg subpath")
	}
}

func TestFSStore_KGDeleteRelationsNotSupported(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Delete("/kg/relations"); err == nil {
		t.Error("expected error when deleting kg/relations")
	}
}

// --- skills backend tests ---

func TestFSStore_SkillsWriteRead(t *testing.T) {
	fs := newTestFSStore(t)
	content := "# My Skill\n\nThis is a test skill."
	if err := fs.Write("/skills/test-skill", content); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	got, err := fs.Read("/skills/test-skill")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got != content {
		t.Errorf("expected %q, got %q", content, got)
	}
}

func TestFSStore_SkillsList(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/skills/skill1", "content1"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Write("/skills/skill2", "content2"); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.List("/skills")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 skills, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Category != "skills" {
			t.Errorf("expected category skills, got %s", e.Category)
		}
		if e.Type != "file" {
			t.Errorf("expected type file, got %s", e.Type)
		}
	}
}

func TestFSStore_SkillsDelete(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/skills/test", "content"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Delete("/skills/test"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := fs.Read("/skills/test"); err == nil {
		t.Error("expected error after delete")
	}
}

func TestFSStore_SkillsReadNotFound(t *testing.T) {
	fs := newTestFSStore(t)
	if _, err := fs.Read("/skills/nonexistent"); err == nil {
		t.Error("expected error when reading nonexistent skill")
	}
}

func TestFSStore_SkillsListEmptyDir(t *testing.T) {
	fs := newTestFSStore(t)
	// skillsDir does not exist yet; List should return nil/nil.
	entries, err := fs.List("/skills")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected nil or empty entries, got %d", len(entries))
	}
}

// --- search tests ---

func TestFSStore_SearchMemScope(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/mem/long/programming", "I love Go programming language"); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.Search("programming", "/mem", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one mem search result")
	}
	for _, e := range entries {
		if e.Category != "mem" {
			t.Errorf("expected only mem entries, got category %s", e.Category)
		}
	}
}

func TestFSStore_SearchSkillsScope(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/skills/programming", "# Programming Guide"); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.Search("programming", "/skills", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one skills search result")
	}
	for _, e := range entries {
		if e.Category != "skills" {
			t.Errorf("expected only skills entries, got category %s", e.Category)
		}
	}
}

func TestFSStore_SearchKGScope(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/kg/entities/programming", "skill"); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.Search("programming", "/kg", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one kg search result")
	}
}

func TestFSStore_SearchAllScopes(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/mem/long/programming", "Go programming"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Write("/skills/programming", "# Programming"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Write("/kg/entities/programming", "skill"); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.Search("programming", "", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(entries) < 2 {
		t.Errorf("expected at least 2 results across all scopes, got %d", len(entries))
	}
}

func TestFSStore_SearchTopKLimit(t *testing.T) {
	fs := newTestFSStore(t)
	for i := 0; i < 5; i++ {
		key := "skill" + string(rune('a'+i))
		if err := fs.Write("/skills/"+key, "content about "+key); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := fs.Search("skill", "/skills", 2)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(entries) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(entries))
	}
}

func TestFSStore_SearchDefaultTopK(t *testing.T) {
	fs := newTestFSStore(t)
	// topK <= 0 should default to 10.
	entries, err := fs.Search("nonexistent", "", 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 results, got %d", len(entries))
	}
}

func TestFSStore_SearchNoMatch(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/mem/short/k1", "hello world"); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.Search("completely-different-query", "", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 results for non-matching query, got %d", len(entries))
	}
}

// --- root and error handling tests ---

func TestFSStore_ListRoot(t *testing.T) {
	fs := newTestFSStore(t)
	entries, err := fs.List("/")
	if err != nil {
		t.Fatalf("List(/) failed: %v", err)
	}
	if len(entries) != 4 {
		t.Errorf("expected 4 top-level dirs, got %d", len(entries))
	}
	expectedPaths := map[string]bool{
		"/mem": true, "/profile": true, "/kg": true, "/skills": true,
	}
	for _, e := range entries {
		if !expectedPaths[e.Path] {
			t.Errorf("unexpected path: %s", e.Path)
		}
		if e.Type != "dir" {
			t.Errorf("expected type dir, got %s for %s", e.Type, e.Path)
		}
	}
}

func TestFSStore_ReadRoot(t *testing.T) {
	fs := newTestFSStore(t)
	if _, err := fs.Read("/"); err == nil {
		t.Error("expected error when reading root")
	}
}

func TestFSStore_DeleteRoot(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Delete("/"); err == nil {
		t.Error("expected error when deleting root")
	}
}

func TestFSStore_WriteRoot(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/", "v"); err == nil {
		t.Error("expected error when writing root")
	}
}

func TestFSStore_InvalidPath(t *testing.T) {
	fs := newTestFSStore(t)
	// Path without leading slash
	if err := fs.Write("mem/short/k", "v"); err == nil {
		t.Error("expected error for path without leading /")
	}
	// Path with ..
	if err := fs.Write("/mem/../etc/passwd", "v"); err == nil {
		t.Error("expected error for path with ..")
	}
	// Path with //
	if err := fs.Write("/mem//short", "v"); err == nil {
		t.Error("expected error for path with //")
	}
}

func TestFSStore_ContentTooLarge(t *testing.T) {
	fs := newTestFSStore(t)
	big := strings.Repeat("a", maxFSContentLength+1)
	if err := fs.Write("/mem/short/big", big); err == nil {
		t.Error("expected error for oversized content")
	}
}

func TestFSStore_KeyTooLong(t *testing.T) {
	fs := newTestFSStore(t)
	longKey := strings.Repeat("a", maxFSKeyLength+1)
	path := "/mem/short/" + longKey
	if err := fs.Write(path, "v"); err == nil {
		t.Error("expected error for too-long key")
	}
}

func TestFSStore_UnknownCategory(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/unknown/foo", "v"); err == nil {
		t.Error("expected error for unknown category on Write")
	}
	if _, err := fs.Read("/unknown/foo"); err == nil {
		t.Error("expected error for unknown category on Read")
	}
	if _, err := fs.List("/unknown"); err == nil {
		t.Error("expected error for unknown category on List")
	}
	if err := fs.Delete("/unknown/foo"); err == nil {
		t.Error("expected error for unknown category on Delete")
	}
}

func TestFSStore_InvalidMemoryLevel(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/mem/invalid/key", "v"); err == nil {
		t.Error("expected error for invalid memory level on Write")
	}
	if _, err := fs.Read("/mem/invalid/key"); err == nil {
		t.Error("expected error for invalid memory level on Read")
	}
	if _, err := fs.List("/mem/invalid"); err == nil {
		t.Error("expected error for invalid memory level on List")
	}
}

func TestFSStore_MemReadMissingLevel(t *testing.T) {
	fs := newTestFSStore(t)
	if _, err := fs.Read("/mem"); err == nil {
		t.Error("expected error when reading /mem without level")
	}
}

func TestFSStore_ProfileReadMissingCategory(t *testing.T) {
	fs := newTestFSStore(t)
	if _, err := fs.Read("/profile"); err == nil {
		t.Error("expected error when reading /profile without category")
	}
}

func TestFSStore_KGReadMissingSubpath(t *testing.T) {
	fs := newTestFSStore(t)
	if _, err := fs.Read("/kg"); err == nil {
		t.Error("expected error when reading /kg without subpath")
	}
}

func TestFSStore_SkillsReadMissingName(t *testing.T) {
	fs := newTestFSStore(t)
	if _, err := fs.Read("/skills"); err == nil {
		t.Error("expected error when reading /skills without name")
	}
}

// --- persistence tests ---

func TestFSStore_KGPersistenceAcrossInstances(t *testing.T) {
	kgPath := filepath.Join(t.TempDir(), "knowledge_graph.json")
	skillsDir := filepath.Join(t.TempDir(), "skills")

	fs1 := &FSStore{
		sessionMgr: NewSessionMemoryManager(t.TempDir(), 10, 100),
		profileMgr: &UserProfileManager{
			profiles:   make(map[string]*UserProfile),
			storageDir: t.TempDir(),
			mu:         sync.RWMutex{},
			maxPerUser: defaultMaxPrefsPerUser,
		},
		skillsDir: skillsDir,
		kgPath:    kgPath,
	}
	if err := fs1.Write("/kg/entities/go", "language"); err != nil {
		t.Fatal(err)
	}

	// New FSStore instance pointing at the same kgPath should see the entity.
	fs2 := &FSStore{
		sessionMgr: NewSessionMemoryManager(t.TempDir(), 10, 100),
		profileMgr: &UserProfileManager{
			profiles:   make(map[string]*UserProfile),
			storageDir: t.TempDir(),
			mu:         sync.RWMutex{},
			maxPerUser: defaultMaxPrefsPerUser,
		},
		skillsDir: skillsDir,
		kgPath:    kgPath,
	}
	got, err := fs2.Read("/kg/entities/go")
	if err != nil {
		t.Fatalf("Read from second instance failed: %v", err)
	}
	if !strings.Contains(got, "go") {
		t.Errorf("expected persisted entity, got %q", got)
	}
}

func TestFSStore_SkillsPersistenceOnDisk(t *testing.T) {
	fs := newTestFSStore(t)
	if err := fs.Write("/skills/persisted", "# persisted skill"); err != nil {
		t.Fatal(err)
	}
	// Verify the file exists on disk with .md extension.
	data, err := os.ReadFile(filepath.Join(fs.skillsDir, "persisted.md"))
	if err != nil {
		t.Fatalf("expected skill file on disk: %v", err)
	}
	if !strings.Contains(string(data), "persisted skill") {
		t.Errorf("expected persisted content, got %q", string(data))
	}
}

// --- FSEntry serialization ---

func TestFSEntry_JSONRoundTrip(t *testing.T) {
	now := time.Now()
	entry := FSEntry{
		Path:       "/mem/short/test",
		Type:       "file",
		Size:       42,
		ModifiedAt: now,
		Category:   "mem",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if !strings.Contains(string(data), "/mem/short/test") {
		t.Errorf("expected JSON to contain path, got %s", string(data))
	}

	var unmarshaled FSEntry
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if unmarshaled.Path != entry.Path {
		t.Errorf("expected path %q, got %q", entry.Path, unmarshaled.Path)
	}
	if unmarshaled.Size != entry.Size {
		t.Errorf("expected size %d, got %d", entry.Size, unmarshaled.Size)
	}
}
