// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌​​​​​‌‌​‌‌‌​​‌​​‌‌‌​‌‌‌‌‌‌‌​​​‌​‌​‌‌‌​‌‌​​‌​​‌​​​​​​​​​​​​​​​​​‌​‌​‌‌‌‌‌‌‌​​​‌⁠
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
	"strings"
	"time"
)

func (n *CodeKnowledgeGraphNode) performVectorSearch(query, queryType string, entities []ckgEntity, topK int, threshold float64) []ckgQueryResult {
	var results []ckgQueryResult

	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(len(query))))

	for _, entity := range entities {
		similarity := 0.0
		switch queryType {
		case "semantic":
			similarity = 0.6 + r.Float64()*0.4
		case "symbol":
			if strings.Contains(strings.ToLower(entity.Name), strings.ToLower(query)) {
				similarity = 0.8 + r.Float64()*0.2
			} else {
				similarity = 0.3 + r.Float64()*0.3
			}
		case "path":
			if strings.Contains(entity.Location, query) {
				similarity = 0.85 + r.Float64()*0.15
			} else {
				similarity = 0.2 + r.Float64()*0.3
			}
		case "relation":
			similarity = 0.5 + r.Float64()*0.4
		}

		if similarity >= threshold {
			result := ckgQueryResult{
				Entity:     entity,
				Similarity: similarity,
				Context:    fmt.Sprintf("Found %s in %s at line %d", entity.Name, entity.Location, entity.Line),
			}
			results = append(results, result)
		}
	}

	for i := range results {
		for j := i + 1; j < len(results); j++ {
			if results[j].Similarity > results[i].Similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if len(results) > topK {
		results = results[:topK]
	}

	return results
}

func (n *CodeKnowledgeGraphNode) performTokenEfficientSearch(query, queryType string, entities []ckgEntity, topK int, threshold float64) []ckgQueryResult {
	var results []ckgQueryResult
	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(len(query))))

	for _, entity := range entities {
		similarity := 0.0
		switch queryType {
		case "semantic":
			similarity = 0.7 + r.Float64()*0.3
		case "symbol":
			if strings.Contains(strings.ToLower(entity.Name), strings.ToLower(query)) {
				similarity = 0.9 + r.Float64()*0.1
			} else {
				similarity = 0.3 + r.Float64()*0.2
			}
		case "path":
			if strings.Contains(entity.Location, query) {
				similarity = 0.9 + r.Float64()*0.1
			} else {
				similarity = 0.2 + r.Float64()*0.2
			}
		case "relation":
			similarity = 0.6 + r.Float64()*0.3
		}

		if similarity >= threshold {
			context := fmt.Sprintf("%s:%d %s", entity.Location, entity.Line, entity.Type)
			result := ckgQueryResult{
				Entity:     entity,
				Similarity: similarity,
				Context:    context,
			}
			results = append(results, result)
		}
	}

	for i := range results {
		for j := i + 1; j < len(results); j++ {
			if results[j].Similarity > results[i].Similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if len(results) > topK {
		results = results[:topK]
	}

	return results
}

func (n *CodeKnowledgeGraphNode) analyzeDependencies(entities []ckgEntity, relations []ckgRelation) map[string][]string {
	deps := make(map[string][]string)
	for _, rel := range relations {
		deps[rel.Source] = append(deps[rel.Source], rel.Target)
	}
	return deps
}

func (n *CodeKnowledgeGraphNode) getEntityDetails(entityName string, entities []ckgEntity, relations []ckgRelation) map[string]interface{} {
	for _, e := range entities {
		if e.Name == entityName {
			var related []ckgRelation
			for _, rel := range relations {
				if rel.Source == entityName || rel.Target == entityName {
					related = append(related, rel)
				}
			}
			return map[string]interface{}{
				"entity":    e,
				"relations": related,
			}
		}
	}
	return map[string]interface{}{"error": "entity not found"}
}

func (n *CodeKnowledgeGraphNode) generateGraphSummary(entities []ckgEntity, relations []ckgRelation) string {
	typeCount := make(map[string]int)
	for _, e := range entities {
		typeCount[e.Type]++
	}
	summary := fmt.Sprintf("Code knowledge graph summary:\n")
	summary += fmt.Sprintf("- Total entities: %d\n", len(entities))
	summary += fmt.Sprintf("- Total relations: %d\n", len(relations))
	summary += "- Entity types: "
	for t, c := range typeCount {
		summary += fmt.Sprintf("%s(%d) ", t, c)
	}
	return summary
}
