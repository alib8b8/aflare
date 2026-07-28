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

package nodes

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func (n *CodeKnowledgeGraphNode) executeMCPTool(input string, params map[string]string) (string, error) {
	mcpTool := getParam(params, "mcp_tool", "")
	if !ckgMCPToolWhitelist[mcpTool] {
		return "", fmt.Errorf("invalid mcp_tool: %s (supported: list_entities, search_graph, analyze_dependencies, get_entity_details, list_relations, generate_summary)", mcpTool)
	}

	safePath, err := validateReadPath(params["path"])
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	var files []string
	if info, err := os.Stat(safePath); err == nil && info.IsDir() {
		files, err = n.collectFiles(safePath)
		if err != nil {
			return "", fmt.Errorf("failed to collect files: %w", err)
		}
	} else {
		files = []string{safePath}
	}

	var entities []ckgEntity
	var relations []ckgRelation
	for _, f := range files {
		e, r := n.extractFromFile(f)
		entities = append(entities, e...)
		relations = append(relations, r...)
	}

	var result map[string]interface{}
	switch mcpTool {
	case "list_entities":
		result = map[string]interface{}{
			"tool":      mcpTool,
			"count":     len(entities),
			"entities":  entities,
			"path":      params["path"],
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
	case "search_graph":
		query := getParam(params, "query", input)
		topK := parseIntSafe(getParam(params, "top_k", "10"), 10)
		threshold := parseFloatSafe(getParam(params, "threshold", "0.7"), 0.7)
		results := n.performTokenEfficientSearch(query, "semantic", entities, topK, threshold)
		result = map[string]interface{}{
			"tool":      mcpTool,
			"query":     query,
			"count":     len(results),
			"results":   results,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
	case "analyze_dependencies":
		deps := n.analyzeDependencies(entities, relations)
		result = map[string]interface{}{
			"tool":            mcpTool,
			"total_entities":  len(entities),
			"total_relations": len(relations),
			"dependencies":    deps,
			"timestamp":       time.Now().UTC().Format(time.RFC3339),
		}
	case "get_entity_details":
		entityName := getParam(params, "entity_name", input)
		details := n.getEntityDetails(entityName, entities, relations)
		result = map[string]interface{}{
			"tool":      mcpTool,
			"entity":    entityName,
			"details":   details,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
	case "list_relations":
		result = map[string]interface{}{
			"tool":      mcpTool,
			"count":     len(relations),
			"relations": relations,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
	case "generate_summary":
		summary := n.generateGraphSummary(entities, relations)
		result = map[string]interface{}{
			"tool":      mcpTool,
			"summary":   summary,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data), nil
}
