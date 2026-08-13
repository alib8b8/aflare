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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type KnowledgeGraphNode struct{}

func (n *KnowledgeGraphNode) Name() string {
	return "knowledge_graph"
}

func (n *KnowledgeGraphNode) Description() string {
	return "Build, query, and explore knowledge graphs from text or structured data"
}

func (n *KnowledgeGraphNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "knowledge_graph",
		Description: "Knowledge graph node - extract entities/relations, build graph, query and traverse",
		Input:       "string - text to extract knowledge from, or a query for an existing graph",
		Output:      "string - knowledge graph data or query results",
		Params: []ParamSchema{
			{Name: "action", Type: "string", Description: "Action: extract, extract_llm, query, traverse, stats, visualize (default: extract). extract_llm calls an LLM for higher-quality entity/relation extraction.", Required: false, Default: "extract"},
			{Name: "graph_path", Type: "string", Description: "Path to save/load the graph JSON file", Required: false},
			{Name: "query", Type: "string", Description: "Query for search/traverse (entity name or relation type)", Required: false},
			{Name: "max_depth", Type: "string", Description: "Max traversal depth (default: 2)", Required: false, Default: "2"},
			{Name: "top_k", Type: "string", Description: "Max results to return (default: 10)", Required: false, Default: "10"},
			{Name: "format", Type: "string", Description: "Output format: json, markdown, mermaid (default: markdown)", Required: false, Default: "markdown"},
			{Name: "provider", Type: "string", Description: "LLM provider for extract_llm (default: openai)", Required: false, Default: "openai"},
			{Name: "model", Type: "string", Description: "LLM model name for extract_llm", Required: false},
			{Name: "api_key", Type: "string", Description: "LLM API key for extract_llm (or set <PROVIDER>_API_KEY env var)", Required: false},
			{Name: "endpoint", Type: "string", Description: "LLM API base URL for extract_llm", Required: false},
			{Name: "language", Type: "string", Description: "Prompt language hint for extract_llm: en or zh (default: en)", Required: false, Default: "en"},
			{Name: "session_id", Type: "string", Description: "C-3: when set with memory_key, links extracted entities to that memory entry", Required: false},
			{Name: "memory_key", Type: "string", Description: "C-3: memory entry key to link extracted entities to", Required: false},
		},
	}
}

type KGEntity struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties,omitempty"`
}

type KGRelation struct {
	From       string  `json:"from"`
	To         string  `json:"to"`
	Relation   string  `json:"relation"`
	Confidence float64 `json:"confidence,omitempty"`
}

type KnowledgeGraph struct {
	Entities  map[string]KGEntity `json:"entities"`
	Relations []KGRelation        `json:"relations"`
}

func NewKnowledgeGraph() *KnowledgeGraph {
	return &KnowledgeGraph{
		Entities:  make(map[string]KGEntity),
		Relations: make([]KGRelation, 0),
	}
}

func (kg *KnowledgeGraph) AddEntity(name, entityType string, props map[string]string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	if existing, ok := kg.Entities[name]; ok {
		if entityType != "" && existing.Type == "" {
			existing.Type = entityType
		}
		if props != nil {
			if existing.Properties == nil {
				existing.Properties = make(map[string]string)
			}
			for k, v := range props {
				existing.Properties[k] = v
			}
		}
		kg.Entities[name] = existing
	} else {
		kg.Entities[name] = KGEntity{
			Name:       name,
			Type:       entityType,
			Properties: props,
		}
	}
}

func (kg *KnowledgeGraph) AddRelation(from, to, relation string, confidence float64) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" || to == "" || relation == "" {
		return
	}
	kg.Relations = append(kg.Relations, KGRelation{
		From:       from,
		To:         to,
		Relation:   relation,
		Confidence: confidence,
	})
}

func (kg *KnowledgeGraph) Save(path string) error {
	safePath, err := validateWritePath(path)
	if err != nil {
		return fmt.Errorf("invalid save path: %w", err)
	}
	data, err := json.MarshalIndent(kg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(safePath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}
	return os.WriteFile(safePath, data, 0644)
}

func LoadKnowledgeGraph(path string) (*KnowledgeGraph, error) {
	safePath, err := validateReadPath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid load path: %w", err)
	}
	data, err := os.ReadFile(safePath)
	if err != nil {
		return nil, err
	}
	var kg KnowledgeGraph
	if err := json.Unmarshal(data, &kg); err != nil {
		return nil, err
	}
	if kg.Entities == nil {
		kg.Entities = make(map[string]KGEntity)
	}
	return &kg, nil
}

func (n *KnowledgeGraphNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	action := getParam(params, "action", "extract")
	graphPath := params["graph_path"]
	query := getParam(params, "query", input)
	maxDepthStr := getParam(params, "max_depth", "2")
	topKStr := getParam(params, "top_k", "10")
	format := getParam(params, "format", "markdown")

	maxDepth := 2
	if _, err := fmt.Sscanf(maxDepthStr, "%d", &maxDepth); err != nil {
		// keep default value on parse failure
	}
	if maxDepth < 1 {
		maxDepth = 1
	}
	if maxDepth > 10 {
		maxDepth = 10
	}

	topK := 10
	if _, err := fmt.Sscanf(topKStr, "%d", &topK); err != nil {
		// keep default value on parse failure
	}
	if topK < 1 {
		topK = 1
	}
	if topK > 1000 {
		topK = 1000
	}

	switch action {
	case "extract":
		return n.extractFromText(input, graphPath, format)
	case "extract_llm":
		return n.extractLLMFromText(ctx, input, graphPath, format, params)
	case "query":
		return n.queryGraph(graphPath, query, topK, format)
	case "traverse":
		return n.traverseGraph(graphPath, query, maxDepth, topK, format)
	case "stats":
		return n.graphStats(graphPath, format)
	case "visualize":
		return n.visualizeGraph(graphPath, format)
	default:
		return "", fmt.Errorf("unknown action: %s (supported: extract, extract_llm, query, traverse, stats, visualize)", action)
	}
}

func (n *KnowledgeGraphNode) extractFromText(text, graphPath, format string) (string, error) {
	kg := NewKnowledgeGraph()

	extractEntitiesSimple(text, kg)
	extractRelationsSimple(text, kg)

	if graphPath != "" {
		if existing, err := LoadKnowledgeGraph(graphPath); err == nil {
			for name, entity := range kg.Entities {
				existing.AddEntity(name, entity.Type, entity.Properties)
			}
			for _, rel := range kg.Relations {
				existing.AddRelation(rel.From, rel.To, rel.Relation, rel.Confidence)
			}
			kg = existing
		}
		if err := kg.Save(graphPath); err != nil {
			return "", fmt.Errorf("failed to save graph: %w", err)
		}
	}

	return formatGraphOutput(kg, format), nil
}

func extractEntitiesSimple(text string, kg *KnowledgeGraph) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		patterns := []struct {
			prefix string
			etype  string
		}{
			{"Person: ", "Person"},
			{"Organization: ", "Organization"},
			{"Company: ", "Company"},
			{"Location: ", "Location"},
			{"Product: ", "Product"},
			{"Concept: ", "Concept"},
			{"Event: ", "Event"},
			{"Technology: ", "Technology"},
		}

		for _, p := range patterns {
			if strings.HasPrefix(line, p.prefix) {
				name := strings.TrimPrefix(line, p.prefix)
				names := strings.Split(name, ",")
				for _, n := range names {
					n = strings.TrimSpace(n)
					if n != "" {
						kg.AddEntity(n, p.etype, nil)
					}
				}
			}
		}
	}

	words := strings.Fields(text)
	for _, word := range words {
		if len(word) > 3 && word[0] >= 'A' && word[0] <= 'Z' {
			cleanWord := strings.Trim(word, ".,;:!?()[]{}\"'")
			if len(cleanWord) > 3 {
				if _, exists := kg.Entities[cleanWord]; !exists {
					kg.AddEntity(cleanWord, "Unknown", nil)
				}
			}
		}
	}
}

func extractRelationsSimple(text string, kg *KnowledgeGraph) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, " -> ") || strings.Contains(line, " → ") {
			parts := strings.SplitN(line, " -> ", 2)
			if len(parts) == 2 {
				fromPart := strings.TrimSpace(parts[0])
				rest := strings.TrimSpace(parts[1])
				relation := "related_to"
				toPart := rest
				if strings.Contains(rest, ":") {
					relParts := strings.SplitN(rest, ":", 2)
					relation = strings.TrimSpace(relParts[0])
					toPart = strings.TrimSpace(relParts[1])
				}
				kg.AddRelation(fromPart, toPart, relation, 0.7)
			}
		}

		if strings.HasPrefix(line, "Relation: ") {
			relLine := strings.TrimPrefix(line, "Relation: ")
			if parts := strings.SplitN(relLine, " -- ", 3); len(parts) == 3 {
				kg.AddRelation(
					strings.TrimSpace(parts[0]),
					strings.TrimSpace(parts[2]),
					strings.TrimSpace(parts[1]),
					0.8,
				)
			}
		}
	}
}

func (n *KnowledgeGraphNode) queryGraph(graphPath, query string, topK int, format string) (string, error) {
	if graphPath == "" {
		return "", fmt.Errorf("graph_path is required for query action")
	}

	kg, err := LoadKnowledgeGraph(graphPath)
	if err != nil {
		return "", fmt.Errorf("failed to load graph: %w", err)
	}

	queryLower := strings.ToLower(query)
	var matchedEntities []KGEntity
	for name, entity := range kg.Entities {
		if strings.Contains(strings.ToLower(name), queryLower) ||
			strings.Contains(strings.ToLower(entity.Type), queryLower) {
			matchedEntities = append(matchedEntities, entity)
		}
	}

	var matchedRelations []KGRelation
	for _, rel := range kg.Relations {
		if strings.Contains(strings.ToLower(rel.From), queryLower) ||
			strings.Contains(strings.ToLower(rel.To), queryLower) ||
			strings.Contains(strings.ToLower(rel.Relation), queryLower) {
			matchedRelations = append(matchedRelations, rel)
		}
	}

	if len(matchedEntities) > topK {
		matchedEntities = matchedEntities[:topK]
	}
	if len(matchedRelations) > topK {
		matchedRelations = matchedRelations[:topK]
	}

	switch format {
	case "json":
		result := map[string]interface{}{
			"query":           query,
			"entities":        matchedEntities,
			"relations":       matchedRelations,
			"total_entities":  len(matchedEntities),
			"total_relations": len(matchedRelations),
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return string(data), nil
	default:
		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("## Knowledge Graph Query: %s\n\n", query))
		builder.WriteString(fmt.Sprintf("**Matched Entities:** %d\n", len(matchedEntities)))
		for _, e := range matchedEntities {
			builder.WriteString(fmt.Sprintf("- %s (%s)\n", e.Name, e.Type))
		}
		builder.WriteString(fmt.Sprintf("\n**Matched Relations:** %d\n", len(matchedRelations)))
		for _, r := range matchedRelations {
			builder.WriteString(fmt.Sprintf("- %s --[%s]--> %s\n", r.From, r.Relation, r.To))
		}
		return builder.String(), nil
	}
}

func (n *KnowledgeGraphNode) traverseGraph(graphPath, startEntity string, maxDepth, topK int, format string) (string, error) {
	if graphPath == "" {
		return "", fmt.Errorf("graph_path is required for traverse action")
	}

	kg, err := LoadKnowledgeGraph(graphPath)
	if err != nil {
		return "", fmt.Errorf("failed to load graph: %w", err)
	}

	type TraversalNode struct {
		Name  string
		Depth int
		Path  []string
	}

	visited := make(map[string]bool)
	var result []TraversalNode
	queue := []TraversalNode{{Name: startEntity, Depth: 0, Path: []string{startEntity}}}

	for len(queue) > 0 && len(result) < topK {
		current := queue[0]
		queue = queue[1:]

		if visited[current.Name] {
			continue
		}
		visited[current.Name] = true
		result = append(result, current)

		if current.Depth >= maxDepth {
			continue
		}

		for _, rel := range kg.Relations {
			next := ""
			if rel.From == current.Name {
				next = rel.To
			} else if rel.To == current.Name {
				next = rel.From
			}
			if next != "" && !visited[next] {
				newPath := make([]string, len(current.Path)+1)
				copy(newPath, current.Path)
				newPath[len(newPath)-1] = next
				queue = append(queue, TraversalNode{
					Name:  next,
					Depth: current.Depth + 1,
					Path:  newPath,
				})
			}
		}
	}

	switch format {
	case "json":
		data, _ := json.MarshalIndent(map[string]interface{}{
			"start":   startEntity,
			"depth":   maxDepth,
			"visited": result,
			"total":   len(result),
		}, "", "  ")
		return string(data), nil
	default:
		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("## Graph Traversal from: %s (depth: %d)\n\n", startEntity, maxDepth))
		for _, node := range result {
			builder.WriteString(fmt.Sprintf("- [Depth %d] %s  \n", node.Depth, strings.Join(node.Path, " → ")))
		}
		builder.WriteString(fmt.Sprintf("\n**Total nodes visited:** %d\n", len(result)))
		return builder.String(), nil
	}
}

func (n *KnowledgeGraphNode) graphStats(graphPath, format string) (string, error) {
	if graphPath == "" {
		return "", fmt.Errorf("graph_path is required for stats action")
	}

	kg, err := LoadKnowledgeGraph(graphPath)
	if err != nil {
		return "", fmt.Errorf("failed to load graph: %w", err)
	}

	typeCounts := make(map[string]int)
	for _, entity := range kg.Entities {
		t := entity.Type
		if t == "" {
			t = "Unknown"
		}
		typeCounts[t]++
	}

	relationCounts := make(map[string]int)
	for _, rel := range kg.Relations {
		relationCounts[rel.Relation]++
	}

	switch format {
	case "json":
		data, _ := json.MarshalIndent(map[string]interface{}{
			"total_entities":  len(kg.Entities),
			"total_relations": len(kg.Relations),
			"entity_types":    typeCounts,
			"relation_types":  relationCounts,
		}, "", "  ")
		return string(data), nil
	default:
		var builder strings.Builder
		builder.WriteString("## Knowledge Graph Statistics\n\n")
		builder.WriteString(fmt.Sprintf("- **Total Entities:** %d\n", len(kg.Entities)))
		builder.WriteString(fmt.Sprintf("- **Total Relations:** %d\n\n", len(kg.Relations)))
		builder.WriteString("### Entity Types\n")
		for t, count := range typeCounts {
			builder.WriteString(fmt.Sprintf("- %s: %d\n", t, count))
		}
		builder.WriteString("\n### Relation Types\n")
		for r, count := range relationCounts {
			builder.WriteString(fmt.Sprintf("- %s: %d\n", r, count))
		}
		return builder.String(), nil
	}
}

func (n *KnowledgeGraphNode) visualizeGraph(graphPath, format string) (string, error) {
	if graphPath == "" {
		return "", fmt.Errorf("graph_path is required for visualize action")
	}

	kg, err := LoadKnowledgeGraph(graphPath)
	if err != nil {
		return "", fmt.Errorf("failed to load graph: %w", err)
	}

	switch format {
	case "mermaid":
		var builder strings.Builder
		builder.WriteString("```mermaid\ngraph TD\n")
		for name, entity := range kg.Entities {
			label := name
			if entity.Type != "" && entity.Type != "Unknown" {
				label = fmt.Sprintf("%s\\n(%s)", name, entity.Type)
			}
			safeName := strings.ReplaceAll(name, " ", "_")
			safeName = strings.ReplaceAll(safeName, "-", "_")
			builder.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", safeName, label))
		}
		for _, rel := range kg.Relations {
			fromSafe := strings.ReplaceAll(rel.From, " ", "_")
			fromSafe = strings.ReplaceAll(fromSafe, "-", "_")
			toSafe := strings.ReplaceAll(rel.To, " ", "_")
			toSafe = strings.ReplaceAll(toSafe, "-", "_")
			builder.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", fromSafe, rel.Relation, toSafe))
		}
		builder.WriteString("```\n")
		return builder.String(), nil
	default:
		return formatGraphOutput(kg, format), nil
	}
}

func formatGraphOutput(kg *KnowledgeGraph, format string) string {
	switch format {
	case "json":
		data, _ := json.MarshalIndent(kg, "", "  ")
		return string(data)
	default:
		var builder strings.Builder
		builder.WriteString("## Knowledge Graph\n\n")
		builder.WriteString(fmt.Sprintf("**Entities:** %d\n", len(kg.Entities)))
		builder.WriteString(fmt.Sprintf("**Relations:** %d\n\n", len(kg.Relations)))

		builder.WriteString("### Entities\n")
		for name, entity := range kg.Entities {
			etype := entity.Type
			if etype == "" || etype == "Unknown" {
				builder.WriteString(fmt.Sprintf("- %s\n", name))
			} else {
				builder.WriteString(fmt.Sprintf("- %s (%s)\n", name, etype))
			}
		}

		builder.WriteString("\n### Relations\n")
		for _, rel := range kg.Relations {
			builder.WriteString(fmt.Sprintf("- %s --[%s]--> %s\n", rel.From, rel.Relation, rel.To))
		}

		return builder.String()
	}
}
