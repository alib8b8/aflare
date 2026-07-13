package versioning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupTestManager(t *testing.T) (*VersionManager, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "versioning-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})
	return NewVersionManager(tmpDir), tmpDir
}

func TestNewVersionManager(t *testing.T) {
	vm := NewVersionManager("/tmp/test-storage")
	if vm == nil {
		t.Fatal("expected non-nil VersionManager")
	}
	if vm.storageDir != "/tmp/test-storage" {
		t.Errorf("expected storageDir /tmp/test-storage, got %s", vm.storageDir)
	}
}

func TestValidateWorkflowName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid-workflow", false},
		{"workflow123", false},
		{"", true},
		{"../etc/passwd", true},
		{"workflow/name", true},
		{"workflow\\name", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWorkflowName(tt.name)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.name, err)
			}
		})
	}
}

func TestSaveVersion(t *testing.T) {
	vm, _ := setupTestManager(t)

	v, err := vm.SaveVersion("test-workflow", "name: hello", "alice", "initial commit")
	if err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}
	if v.ID == "" {
		t.Error("expected non-empty version ID")
	}
	if v.WorkflowName != "test-workflow" {
		t.Errorf("expected workflow name test-workflow, got %s", v.WorkflowName)
	}
	if v.Content != "name: hello" {
		t.Errorf("expected content 'name: hello', got %s", v.Content)
	}
	if v.Author != "alice" {
		t.Errorf("expected author alice, got %s", v.Author)
	}
	if v.Message != "initial commit" {
		t.Errorf("expected message 'initial commit', got %s", v.Message)
	}
	if v.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if len(v.Tags) != 0 {
		t.Errorf("expected empty tags, got %v", v.Tags)
	}

	// Verify files on disk.
	yamlPath := filepath.Join(vm.workflowDir("test-workflow"), v.ID+".yaml")
	if _, err := os.Stat(yamlPath); err != nil {
		t.Errorf("yaml file not found: %v", err)
	}
	metaPath := filepath.Join(vm.workflowDir("test-workflow"), v.ID+".json")
	if _, err := os.Stat(metaPath); err != nil {
		t.Errorf("json file not found: %v", err)
	}

	// Verify file permissions (owner read/write only).
	info, err := os.Stat(yamlPath)
	if err != nil {
		t.Fatalf("stat yaml file failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected yaml file permission 0600, got %04o", info.Mode().Perm())
	}
}

func TestSaveVersionInvalidWorkflowName(t *testing.T) {
	vm, _ := setupTestManager(t)
	_, err := vm.SaveVersion("../bad", "content", "alice", "msg")
	if err == nil {
		t.Error("expected error for invalid workflow name")
	}
}

func TestListVersions(t *testing.T) {
	vm, _ := setupTestManager(t)

	// Empty list for non-existent workflow.
	versions, err := vm.ListVersions("nonexistent")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(versions))
	}

	// Save multiple versions.
	if _, err := vm.SaveVersion("wf", "v1", "alice", "first"); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := vm.SaveVersion("wf", "v2", "bob", "second"); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	versions, err = vm.ListVersions("wf")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions[0].Content != "v1" {
		t.Errorf("expected first version content v1, got %s", versions[0].Content)
	}
	if versions[1].Content != "v2" {
		t.Errorf("expected second version content v2, got %s", versions[1].Content)
	}
}

func TestGetVersion(t *testing.T) {
	vm, _ := setupTestManager(t)

	saved, err := vm.SaveVersion("wf", "content", "alice", "msg")
	if err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	v, err := vm.GetVersion("wf", saved.ID)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	if v.ID != saved.ID {
		t.Errorf("expected ID %s, got %s", saved.ID, v.ID)
	}
	if v.Content != "content" {
		t.Errorf("expected content 'content', got %s", v.Content)
	}

	// Non-existent version.
	_, err = vm.GetVersion("wf", "99991231000000-deadbeef")
	if err == nil {
		t.Error("expected error for non-existent version")
	}
}

func TestGetLatestVersion(t *testing.T) {
	vm, _ := setupTestManager(t)

	// No versions.
	_, err := vm.GetLatestVersion("wf")
	if err == nil {
		t.Error("expected error when no versions exist")
	}

	if _, err := vm.SaveVersion("wf", "first", "alice", "msg1"); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := vm.SaveVersion("wf", "second", "bob", "msg2"); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	latest, err := vm.GetLatestVersion("wf")
	if err != nil {
		t.Fatalf("GetLatestVersion failed: %v", err)
	}
	if latest.Content != "second" {
		t.Errorf("expected latest content 'second', got %s", latest.Content)
	}
}

func TestCompareVersions(t *testing.T) {
	vm, _ := setupTestManager(t)

	v1, err := vm.SaveVersion("wf", "line1\nline2\nline3", "alice", "v1")
	if err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}
	v2, err := vm.SaveVersion("wf", "line1\nline2 modified\nline3\nline4", "bob", "v2")
	if err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	diff, err := vm.CompareVersions("wf", v1.ID, v2.ID)
	if err != nil {
		t.Fatalf("CompareVersions failed: %v", err)
	}

	if !strings.Contains(diff, "---") {
		t.Error("expected diff to contain '---'")
	}
	if !strings.Contains(diff, "+++") {
		t.Error("expected diff to contain '+++'")
	}
	if !strings.Contains(diff, "-") {
		t.Error("expected diff to contain deletions")
	}
	if !strings.Contains(diff, "+") {
		t.Error("expected diff to contain additions")
	}
}

func TestRollback(t *testing.T) {
	vm, _ := setupTestManager(t)

	v1, err := vm.SaveVersion("wf", "original", "alice", "first")
	if err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}
	_, err = vm.SaveVersion("wf", "modified", "bob", "second")
	if err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	rolled, err := vm.Rollback("wf", v1.ID)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}
	if rolled.Content != "original" {
		t.Errorf("expected rolled back content 'original', got %s", rolled.Content)
	}
	if !strings.Contains(rolled.Message, "Rollback") {
		t.Errorf("expected rollback message to contain 'Rollback', got %s", rolled.Message)
	}

	// Verify a new version was created.
	versions, err := vm.ListVersions("wf")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 3 {
		t.Errorf("expected 3 versions after rollback, got %d", len(versions))
	}
}

func TestTagVersion(t *testing.T) {
	vm, _ := setupTestManager(t)

	v, err := vm.SaveVersion("wf", "content", "alice", "msg")
	if err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	if err := vm.TagVersion("wf", v.ID, "v1.0.0"); err != nil {
		t.Fatalf("TagVersion failed: %v", err)
	}

	// Tag non-existent version.
	err = vm.TagVersion("wf", "99991231000000-deadbeef", "v2.0.0")
	if err == nil {
		t.Error("expected error when tagging non-existent version")
	}

	// Empty tag.
	err = vm.TagVersion("wf", v.ID, "")
	if err == nil {
		t.Error("expected error for empty tag")
	}
}

func TestGetVersionByTag(t *testing.T) {
	vm, _ := setupTestManager(t)

	v, err := vm.SaveVersion("wf", "content", "alice", "msg")
	if err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}
	if err := vm.TagVersion("wf", v.ID, "stable"); err != nil {
		t.Fatalf("TagVersion failed: %v", err)
	}

	found, err := vm.GetVersionByTag("wf", "stable")
	if err != nil {
		t.Fatalf("GetVersionByTag failed: %v", err)
	}
	if found.ID != v.ID {
		t.Errorf("expected version ID %s, got %s", v.ID, found.ID)
	}

	// Non-existent tag.
	_, err = vm.GetVersionByTag("wf", "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent tag")
	}
}

func TestDeleteVersion(t *testing.T) {
	vm, _ := setupTestManager(t)

	v1, err := vm.SaveVersion("wf", "first", "alice", "msg1")
	if err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}
	v2, err := vm.SaveVersion("wf", "second", "bob", "msg2")
	if err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	// Tag v1 then delete it.
	if err := vm.TagVersion("wf", v1.ID, "old"); err != nil {
		t.Fatalf("TagVersion failed: %v", err)
	}

	if err := vm.DeleteVersion("wf", v1.ID); err != nil {
		t.Fatalf("DeleteVersion failed: %v", err)
	}

	// Verify version removed.
	_, err = vm.GetVersion("wf", v1.ID)
	if err == nil {
		t.Error("expected error after deleting version")
	}

	// Verify tag removed.
	_, err = vm.GetVersionByTag("wf", "old")
	if err == nil {
		t.Error("expected error for deleted tag")
	}

	// Cannot delete last version.
	err = vm.DeleteVersion("wf", v2.ID)
	if err == nil {
		t.Error("expected error when deleting last version")
	}
}

func TestDeleteVersionNonExistent(t *testing.T) {
	vm, _ := setupTestManager(t)
	if _, err := vm.SaveVersion("wf", "only", "alice", "msg"); err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	err := vm.DeleteVersion("wf", "99991231000000-deadbeef")
	if err == nil {
		t.Error("expected error when deleting non-existent version")
	}
}

func TestComputeDiff(t *testing.T) {
	a := "hello\nworld"
	b := "hello\nworld\nextra"
	diff := computeDiff(a, b, "a", "b")
	if !strings.Contains(diff, "+") {
		t.Error("expected diff to contain additions")
	}

	// Identical.
	diff = computeDiff("same", "same", "a", "b")
	if strings.Contains(diff, "+a\t") || strings.Contains(diff, "-a\t") {
		t.Error("expected no changes for identical content")
	}
}

func TestGenerateVersionID(t *testing.T) {
	id1 := generateVersionID("hello")
	_ = generateVersionID("hello")
	// Same content may have same hash but timestamp could differ in rare cases.
	// We mainly check format.
	parts := strings.Split(id1, "-")
	if len(parts) != 2 {
		t.Fatalf("expected version ID format timestamp-hash, got %s", id1)
	}
	if len(parts[0]) != 14 {
		t.Errorf("expected timestamp length 14, got %d", len(parts[0]))
	}
	if len(parts[1]) != 8 {
		t.Errorf("expected hash length 8, got %d", len(parts[1]))
	}

	// Different content should generally produce different IDs (hash differs).
	id3 := generateVersionID("world")
	if id1 == id3 {
		// Highly unlikely but possible if timestamps align; accept but log.
		t.Logf("different content produced same ID (timestamps aligned): %s", id1)
	}
}

func TestValidateVersionID(t *testing.T) {
	tests := []struct {
		id      string
		wantErr bool
	}{
		{"20250101120000-abcdef12", false},
		{"", true},
		{"bad", true},
		{"20250101120000", true},
		{"20250101120000-abc", true},
		{"2025010112000a-abcdef12", true},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			err := ValidateVersionID(tt.id)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q", tt.id)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.id, err)
			}
		})
	}
}

func TestGetVersionContentFromYaml(t *testing.T) {
	vm, _ := setupTestManager(t)

	content := "steps:\n  - name: test\n    run: echo hello"
	v, err := vm.SaveVersion("wf", content, "alice", "msg")
	if err != nil {
		t.Fatalf("SaveVersion failed: %v", err)
	}

	// Modify json metadata content to simulate out-of-sync scenario.
	metaPath := filepath.Join(vm.workflowDir("wf"), v.ID+".json")
	data, _ := os.ReadFile(metaPath)
	modified := strings.Replace(string(data), content, "tampered", 1)
	os.WriteFile(metaPath, []byte(modified), 0600)

	// GetVersion should still return the yaml content.
	retrieved, err := vm.GetVersion("wf", v.ID)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	if retrieved.Content != content {
		t.Errorf("expected yaml content, got %s", retrieved.Content)
	}
}
