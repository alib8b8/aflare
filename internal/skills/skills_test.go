package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSkillRegistry(t *testing.T) {
	dir := t.TempDir()
	sr := NewSkillRegistry(dir)
	if sr == nil {
		t.Fatal("expected non-nil registry")
	}
	if sr.Count() != 0 {
		t.Fatalf("expected 0 skills, got %d", sr.Count())
	}
}

func TestSkillRegistryLoad(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	wfContent := `name: test
nodes: []`
	if err := os.WriteFile(filepath.Join(skillDir, "workflow.yaml"), []byte(wfContent), 0644); err != nil {
		t.Fatal(err)
	}

	sr := NewSkillRegistry(dir)
	if err := sr.Load(); err != nil {
		t.Fatal(err)
	}
	if sr.Count() != 1 {
		t.Fatalf("expected 1 skill, got %d", sr.Count())
	}
}

func TestSkillRegistrySave(t *testing.T) {
	dir := t.TempDir()
	sr := NewSkillRegistry(dir)
	if err := sr.Load(); err != nil {
		t.Fatal(err)
	}
	if err := sr.SaveRegistry(); err != nil {
		t.Fatal(err)
	}
	regPath := filepath.Join(dir, RegistryFileName)
	if _, err := os.Stat(regPath); os.IsNotExist(err) {
		t.Fatal("registry file not created")
	}
}

func TestSkillRegistrySearch(t *testing.T) {
	dir := t.TempDir()
	sr := NewSkillRegistry(dir)
	if err := sr.Load(); err != nil {
		t.Fatal(err)
	}
	results := sr.Search("nonexistent")
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestSkillRegistryCategories(t *testing.T) {
	dir := t.TempDir()
	sr := NewSkillRegistry(dir)
	if err := sr.Load(); err != nil {
		t.Fatal(err)
	}
	cats := sr.Categories()
	if cats == nil {
		t.Fatal("expected non-nil categories")
	}
}

func TestSkillMeta(t *testing.T) {
	meta := &SkillMeta{
		ID:          "test/skill",
		Name:        "Test Skill",
		Version:     "1.0.0",
		Description: "A test skill",
		Category:    "test",
		Tags:        []string{"test", "skill"},
		Keywords:    []string{"testing"},
	}
	if meta.ID != "test/skill" {
		t.Errorf("unexpected ID: %s", meta.ID)
	}
	if !matchKeyword(meta, "test") {
		t.Error("expected to match 'test'")
	}
	if !matchKeyword(meta, "testing") {
		t.Error("expected to match 'testing'")
	}
	if matchKeyword(meta, "nonexistent") {
		t.Error("expected no match for 'nonexistent'")
	}
}
