// Copyright (c) 2026 llm-box Contributors
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

func TestSkillRegistryGet(t *testing.T) {
	dir := t.TempDir()
	sr := NewSkillRegistry(dir)
	if err := sr.Load(); err != nil {
		t.Fatal(err)
	}

	_, err := sr.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestSkillRegistryListByCategory(t *testing.T) {
	dir := t.TempDir()
	sr := NewSkillRegistry(dir)
	if err := sr.Load(); err != nil {
		t.Fatal(err)
	}

	results := sr.ListByCategory("nonexistent")
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent category, got %d", len(results))
	}
}

func TestSkillRegistryGenerateMissingMetas(t *testing.T) {
	dir := t.TempDir()
	sr := NewSkillRegistry(dir)
	if err := sr.Load(); err != nil {
		t.Fatal(err)
	}

	generated := sr.GenerateMissingMetas()
	if generated != 0 {
		t.Errorf("expected 0 generated metas for empty registry, got %d", generated)
	}
}

func TestSkillRegistryList(t *testing.T) {
	dir := t.TempDir()
	sr := NewSkillRegistry(dir)
	if err := sr.Load(); err != nil {
		t.Fatal(err)
	}

	list := sr.List()
	if list == nil {
		t.Fatal("expected non-nil list")
	}
	if len(list) != 0 {
		t.Errorf("expected 0 items in list, got %d", len(list))
	}
}

func TestAutoGenerateMeta(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "coding", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "workflow.yaml"), []byte("name: test"), 0644); err != nil {
		t.Fatal(err)
	}

	meta := autoGenerateMeta(skillDir, "coding/my-skill", dir)
	if meta == nil {
		t.Fatal("expected non-nil meta")
	}
	if meta.ID != "coding/my-skill" {
		t.Errorf("expected ID 'coding/my-skill', got %q", meta.ID)
	}
	if meta.Category != "coding" {
		t.Errorf("expected category 'coding', got %q", meta.Category)
	}
	if meta.Name != "my-skill" {
		t.Errorf("expected name 'my-skill', got %q", meta.Name)
	}
}

func TestAutoGenerateMetaWithReadme(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "workflow.yaml"), []byte("name: test"), 0644); err != nil {
		t.Fatal(err)
	}
	readme := "# My Skill\n\nThis is a custom description for testing.\n\nMore text here."
	if err := os.WriteFile(filepath.Join(skillDir, "README.md"), []byte(readme), 0644); err != nil {
		t.Fatal(err)
	}

	meta := autoGenerateMeta(skillDir, "my-skill", dir)
	if meta == nil {
		t.Fatal("expected non-nil meta")
	}
	if meta.Description != "This is a custom description for testing." {
		t.Errorf("expected description from readme, got %q", meta.Description)
	}
}
