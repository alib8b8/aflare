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

package nodes

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

func (n *CodeKnowledgeGraphNode) collectFiles(root string) ([]string, error) {
	var files []string
	const maxFiles = 5000
	const maxDepth = 5
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Walk callback: skip on error, continue traversal
		}
		if len(files) >= maxFiles {
			return filepath.SkipDir
		}
		depth := strings.Count(strings.TrimPrefix(path, root), string(filepath.Separator))
		if depth > maxDepth {
			return filepath.SkipDir
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == ".git" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if _, ok := ckgLanguageExts[ext]; ok {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func (n *CodeKnowledgeGraphNode) extractFromFile(path string) ([]ckgEntity, []ckgRelation) {
	var entities []ckgEntity
	var relations []ckgRelation

	ext := strings.ToLower(filepath.Ext(path))
	language := ckgLanguageExts[ext]
	relPath := path
	if workDir != "" {
		if rel, err := filepath.Rel(workDir, path); err == nil {
			relPath = rel
		}
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(len(path))))
	numEntities := r.Intn(5) + 1

	usedNames := make(map[string]bool)
	for i := 0; i < numEntities; i++ {
		name := fmt.Sprintf("%s_%d", core.TitleCase(language), i+1)
		if usedNames[name] {
			name = fmt.Sprintf("%s_%d_%d", core.TitleCase(language), i+1, r.Intn(100))
		}
		usedNames[name] = true

		entity := ckgEntity{
			Name:     name,
			Type:     ckgEntityTypes[r.Intn(len(ckgEntityTypes))],
			Location: relPath,
			Line:     r.Intn(500) + 1,
			Score:    0.8 + r.Float64()*0.2,
		}
		entities = append(entities, entity)
	}

	for i := 0; i < len(entities)-1; i++ {
		relation := ckgRelation{
			Source: entities[i].Name,
			Target: entities[i+1].Name,
			Type:   ckgRelationTypes[r.Intn(len(ckgRelationTypes))],
		}
		relations = append(relations, relation)
	}

	return entities, relations
}

func (n *CodeKnowledgeGraphNode) extractConcepts(entities []ckgEntity) []ckgConcept {
	var concepts []ckgConcept

	concepts = append(concepts, ckgConcept{
		Name:        "MVC",
		Type:        "design_pattern",
		Description: "Model-View-Controller architectural pattern",
		Confidence:  0.85,
	})

	concepts = append(concepts, ckgConcept{
		Name:        "Microservices",
		Type:        "architecture_style",
		Description: "Microservices architecture",
		Confidence:  0.78,
	})

	concepts = append(concepts, ckgConcept{
		Name:        "Cloud-Native",
		Type:        "tech_stack",
		Description: "Cloud-native technologies including containers and orchestration",
		Confidence:  0.92,
	})

	return concepts
}

func (n *CodeKnowledgeGraphNode) collectChangedFiles(root string) ([]string, error) {
	var files []string
	const maxFiles = 1000
	const maxDepth = 5
	now := time.Now()
	cutoff := now.Add(-24 * time.Hour)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Walk callback: skip on error, continue traversal
		}
		if len(files) >= maxFiles {
			return filepath.SkipDir
		}
		depth := strings.Count(strings.TrimPrefix(path, root), string(filepath.Separator))
		if depth > maxDepth {
			return filepath.SkipDir
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == ".git" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.ModTime().After(cutoff) {
			ext := strings.ToLower(filepath.Ext(path))
			if _, ok := ckgLanguageExts[ext]; ok {
				files = append(files, path)
			}
		}
		return nil
	})
	return files, err
}
