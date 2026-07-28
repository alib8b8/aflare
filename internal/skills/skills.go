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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type SkillIO struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
}

type SkillMeta struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Description  string    `json:"description"`
	Author       string    `json:"author"`
	Category     string    `json:"category"`
	Tags         []string  `json:"tags"`
	Keywords     []string  `json:"keywords"`
	Inputs       []SkillIO `json:"inputs"`
	Outputs      []SkillIO `json:"outputs"`
	Dependencies []string  `json:"dependencies"`
	Path         string    `json:"-"`
}

type SkillRegistry struct {
	mu      sync.RWMutex
	skills  map[string]*SkillMeta
	baseDir string
	indexed bool
}

const (
	RegistryFileName = "skills-registry.json"
	SkillMetaFile    = "skill.json"
)

func NewSkillRegistry(baseDir string) *SkillRegistry {
	return &SkillRegistry{
		skills:  make(map[string]*SkillMeta),
		baseDir: baseDir,
	}
}

func (sr *SkillRegistry) Load() error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	registryPath := filepath.Join(sr.baseDir, RegistryFileName)
	if data, err := os.ReadFile(registryPath); err == nil { // #nosec G304 -- internally generated path
		var registry struct {
			Skills []*SkillMeta `json:"skills"`
		}
		if err := json.Unmarshal(data, &registry); err == nil {
			for _, s := range registry.Skills {
				sr.skills[s.ID] = s
			}
			sr.indexed = true
			return nil
		}
	}

	return sr.scanDirectory()
}

func (sr *SkillRegistry) scanDirectory() error {
	if _, err := os.Stat(sr.baseDir); os.IsNotExist(err) {
		return nil
	}

	return filepath.Walk(sr.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}

		metaPath := filepath.Join(path, SkillMetaFile)
		if data, err := os.ReadFile(metaPath); err == nil { // #nosec G304 -- internally generated path
			var meta SkillMeta
			if err := json.Unmarshal(data, &meta); err == nil {
				if meta.ID != "" {
					meta.Path = path
					sr.skills[meta.ID] = &meta
				}
			}
		}

		if _, statErr := os.Stat(filepath.Join(path, "workflow.yaml")); statErr == nil {
			relPath, relErr := filepath.Rel(sr.baseDir, path)
			if relErr == nil && relPath != "." {
				autoID := filepath.ToSlash(relPath)
				if _, exists := sr.skills[autoID]; !exists {
					meta := autoGenerateMeta(path, autoID, sr.baseDir)
					if meta != nil {
						sr.skills[autoID] = meta
					}
				}
			}
		}
		return nil
	})
}

func autoGenerateMeta(dirPath, id, baseDir string) *SkillMeta {
	name := filepath.Base(dirPath)
	category := "uncategorized"
	if rel, err := filepath.Rel(baseDir, dirPath); err == nil {
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) > 1 {
			category = parts[0]
		}
	}

	readmePath := filepath.Join(dirPath, "README.md")
	description := ""
	if data, err := os.ReadFile(readmePath); err == nil { // #nosec G304 -- internally generated path
		lines := strings.SplitN(string(data), "\n", 10)
		for _, line := range lines {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, ">")
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				description = line
				break
			}
		}
	}
	if description == "" {
		description = fmt.Sprintf("%s workflow template", name)
	}

	return &SkillMeta{
		ID:          id,
		Name:        name,
		Version:     "1.0.0",
		Description: description,
		Author:      "llm-box community",
		Category:    category,
		Tags:        []string{category, "workflow"},
		Keywords:    []string{name, category},
		Path:        dirPath,
	}
}

func (sr *SkillRegistry) SaveRegistry() error {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	skills := make([]*SkillMeta, 0, len(sr.skills))
	for _, s := range sr.skills {
		skills = append(skills, s)
	}
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].ID < skills[j].ID
	})

	registry := struct {
		Version string       `json:"version"`
		Count   int          `json:"count"`
		Skills  []*SkillMeta `json:"skills"`
	}{
		Version: "1.0.0",
		Count:   len(skills),
		Skills:  skills,
	}

	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}

	registryPath := filepath.Join(sr.baseDir, RegistryFileName)
	return os.WriteFile(registryPath, data, 0644)
}

func (sr *SkillRegistry) GenerateMissingMetas() int {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	generated := 0
	for id, meta := range sr.skills {
		if meta.Path == "" {
			continue
		}
		metaPath := filepath.Join(meta.Path, SkillMetaFile)
		if _, err := os.Stat(metaPath); os.IsNotExist(err) {
			data, jsonErr := json.MarshalIndent(meta, "", "  ")
			if jsonErr == nil {
				if writeErr := os.WriteFile(metaPath, data, 0644); writeErr == nil {
					generated++
				}
			}
		}
		_ = id
	}
	return generated
}

func (sr *SkillRegistry) List() []*SkillMeta {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	result := make([]*SkillMeta, 0, len(sr.skills))
	for _, s := range sr.skills {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func (sr *SkillRegistry) Get(id string) (*SkillMeta, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	s, ok := sr.skills[id]
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", id)
	}
	return s, nil
}

func (sr *SkillRegistry) Search(keyword string) []*SkillMeta {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	keyword = strings.ToLower(keyword)
	var result []*SkillMeta
	for _, s := range sr.skills {
		if matchKeyword(s, keyword) {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func matchKeyword(s *SkillMeta, keyword string) bool {
	if strings.Contains(strings.ToLower(s.ID), keyword) {
		return true
	}
	if strings.Contains(strings.ToLower(s.Name), keyword) {
		return true
	}
	if strings.Contains(strings.ToLower(s.Description), keyword) {
		return true
	}
	if strings.Contains(strings.ToLower(s.Category), keyword) {
		return true
	}
	for _, tag := range s.Tags {
		if strings.Contains(strings.ToLower(tag), keyword) {
			return true
		}
	}
	for _, kw := range s.Keywords {
		if strings.Contains(strings.ToLower(kw), keyword) {
			return true
		}
	}
	return false
}

func (sr *SkillRegistry) Categories() []string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	catSet := make(map[string]struct{})
	for _, s := range sr.skills {
		catSet[s.Category] = struct{}{}
	}
	result := make([]string, 0, len(catSet))
	for cat := range catSet {
		result = append(result, cat)
	}
	sort.Strings(result)
	return result
}

func (sr *SkillRegistry) ListByCategory(category string) []*SkillMeta {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	var result []*SkillMeta
	for _, s := range sr.skills {
		if s.Category == category {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func (sr *SkillRegistry) Count() int {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return len(sr.skills)
}
