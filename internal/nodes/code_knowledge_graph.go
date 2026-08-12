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
	"strconv"
	"strings"
	"time"
)

type CodeKnowledgeGraphNode struct{}

func init() {
	Register(&CodeKnowledgeGraphNode{})
}

func (n *CodeKnowledgeGraphNode) Name() string {
	return "code_knowledge_graph"
}

func (n *CodeKnowledgeGraphNode) Description() string {
	return "Semantic code knowledge graph with vector retrieval, 158 language support, MCP tool exposure, and token-efficient review. Supports incremental updates and persistent indexing."
}

func (n *CodeKnowledgeGraphNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - optional query context or MCP tool call",
		Output:      "string - JSON format with entities, relations, concepts, query results, or MCP tool response",
		Params: []ParamSchema{
			{Name: "path", Type: "string", Description: "Code path to analyze", Required: true},
			{Name: "mode", Type: "string", Description: "Mode: build/build_and_query/query_only/incremental/mcp_tool (default: build_and_query)", Required: false, Default: "build_and_query"},
			{Name: "query", Type: "string", Description: "Query statement (text or vector)", Required: false},
			{Name: "query_type", Type: "string", Description: "Query type: semantic/symbol/path/relation (default: semantic)", Required: false, Default: "semantic"},
			{Name: "top_k", Type: "int", Description: "Number of results to return (1-100, default: 10)", Required: false, Default: "10"},
			{Name: "threshold", Type: "float", Description: "Similarity threshold 0.0-1.0 (default: 0.7)", Required: false, Default: "0.7"},
			{Name: "vector_dim", Type: "int", Description: "Vector dimension (default: 384)", Required: false, Default: "384"},
			{Name: "mcp_tool", Type: "string", Description: "MCP tool to call: list_entities/search_graph/analyze_dependencies/get_entity_details/list_relations/generate_summary", Required: false},
			{Name: "entity_name", Type: "string", Description: "Entity name for get_entity_details tool", Required: false},
			{Name: "token_efficient", Type: "bool", Description: "Enable token-efficient review mode (default: true)", Required: false, Default: "true"},
			{Name: "incremental_update", Type: "bool", Description: "Use incremental update (only process changed files)", Required: false, Default: "false"},
			{Name: "use_cache", Type: "bool", Description: "Use persistent cache index (default: true)", Required: false, Default: "true"},
			{Name: "force_rebuild", Type: "bool", Description: "Force rebuild index from scratch (default: false)", Required: false, Default: "false"},
		},
	}
}

func (n *CodeKnowledgeGraphNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	rawPath := params["path"]
	if rawPath == "" {
		return "", fmt.Errorf("path parameter is required")
	}

	mode := getParam(params, "mode", "build_and_query")
	if !ckgModeWhitelist[mode] {
		return "", fmt.Errorf("invalid mode: %s (supported: build, build_and_query, query_only, incremental, mcp_tool, pr_analysis, code_review)", mode)
	}

	if mode == "mcp_tool" {
		return n.executeMCPTool(input, params)
	}

	if mode == "pr_analysis" {
		return n.executePRAnalysis(ctx, input, params)
	}

	if mode == "code_review" {
		return n.executeCodeReview(ctx, input, params)
	}

	queryType := getParam(params, "query_type", "semantic")
	if !ckgQueryTypeWhitelist[queryType] {
		return "", fmt.Errorf("invalid query_type: %s (supported: semantic, symbol, path, relation)", queryType)
	}

	topK, err := strconv.Atoi(getParam(params, "top_k", "10"))
	if err != nil || topK < 1 {
		topK = 10
	}
	if topK > 100 {
		topK = 100
	}

	threshold, err := strconv.ParseFloat(getParam(params, "threshold", "0.7"), 64)
	if err != nil || threshold < 0.0 {
		threshold = 0.7
	}
	if threshold > 1.0 {
		threshold = 1.0
	}

	tokenEfficient := strings.ToLower(getParam(params, "token_efficient", "true")) == "true"
	incrementalUpdate := strings.ToLower(getParam(params, "incremental_update", "false")) == "true"
	useCache := strings.ToLower(getParam(params, "use_cache", "true")) == "true"
	forceRebuild := strings.ToLower(getParam(params, "force_rebuild", "false")) == "true"

	query := getParam(params, "query", "")
	if mode != "build" && query == "" && input == "" {
		return "", fmt.Errorf("query or input is required for query mode")
	}

	if query == "" {
		query = input
	}

	if len(query) > 4096 {
		return "", fmt.Errorf("query too long (max 4096 characters)")
	}

	safePath, err := validateReadPath(rawPath)
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	info, err := os.Stat(safePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat path: %w", err)
	}

	files, idx, err := n.collectFilesForKG(useCache, safePath, info, forceRebuild, incrementalUpdate)
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no files found at path: %s", rawPath)
	}

	result, queryTime, err := n.buildGraph(ctx, mode, query, queryType, topK, threshold, tokenEfficient, useCache, idx, files)
	if err != nil {
		return "", err
	}

	// 计算 FilesAnalyzed：如果使用缓存，统计文件总数；否则统计实际分析的文件数
	filesAnalyzed := len(files)
	if useCache && idx != nil {
		// 使用缓存时，FilesAnalyzed 表示索引中的文件总数
		filesAnalyzed = len(idx.FileHashes)
	}

	result.Stats = ckgStats{
		FilesAnalyzed:      filesAnalyzed,
		EntitiesExtracted:  len(result.Entities),
		RelationsExtracted: len(result.Relations),
		ConceptsExtracted:  len(result.Concepts),
		QueryTimeMs:        queryTime.Milliseconds(),
		CacheUsed:          useCache && idx != nil,
		IndexAge:           "",
	}

	// 如果使用缓存，计算索引年龄
	if idx != nil {
		age := time.Since(idx.CreatedAt)
		result.Stats.IndexAge = age.Round(time.Second).String()
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(data), nil
}

// buildGraph builds the knowledge graph from indexed data or by extracting
// from files, performs token-efficient or vector search when a query is
// provided, and handles inkling review mode. It returns the populated result,
// query duration, and any error.
func (n *CodeKnowledgeGraphNode) buildGraph(
	ctx context.Context,
	mode string,
	query string,
	queryType string,
	topK int,
	threshold float64,
	tokenEfficient bool,
	useCache bool,
	idx *ckgIndex,
	files []string,
) (ckgResult, time.Duration, error) {
	result := ckgResult{
		Entities:   []ckgEntity{},
		Relations:  []ckgRelation{},
		Concepts:   []ckgConcept{},
		Results:    []ckgQueryResult{},
		TokenSaved: 0,
	}

	startTime := time.Now()

	// 从索引获取数据（如果使用缓存）
	if useCache && idx != nil {
		result.Entities = idx.Entities
		result.Relations = idx.Relations
		result.Concepts = idx.Concepts
	} else if mode != "query_only" {
		for _, f := range files {
			if err := ctx.Err(); err != nil {
				return ckgResult{}, 0, fmt.Errorf("context cancelled: %w", err)
			}
			entities, relations := n.extractFromFile(f)
			result.Entities = append(result.Entities, entities...)
			result.Relations = append(result.Relations, relations...)
		}

		result.Concepts = n.extractConcepts(result.Entities)
	}

	// 计算 Token 节省
	if tokenEfficient && len(result.Entities) > 0 {
		result.TokenSaved = len(result.Entities) * 200
	} else if len(result.Entities) > 0 {
		result.TokenSaved = len(result.Entities) * 100
	}

	if mode != "build" && query != "" {
		if tokenEfficient {
			result.Results = n.performTokenEfficientSearch(query, queryType, result.Entities, topK, threshold)
		} else {
			result.Results = n.performVectorSearch(query, queryType, result.Entities, topK, threshold)
		}
	}

	if mode == "inkling_review" {
		reviewResult := n.executeInklingReview(files)
		result.ReviewResult = &reviewResult
	}

	queryTime := time.Since(startTime)
	return result, queryTime, nil
}

// collectFilesForKG collects files and builds a persistent index when
// useCache is enabled. It returns the list of files, the optional index,
// and any error encountered.
func (n *CodeKnowledgeGraphNode) collectFilesForKG(useCache bool, safePath string, info os.FileInfo, forceRebuild bool, incrementalUpdate bool) ([]string, *ckgIndex, error) {
	var files []string
	var idx *ckgIndex
	var err error

	if useCache && info.IsDir() {
		indexPath := n.getIndexFilePath(safePath)
		idx, err = n.loadIndex(indexPath)
		if err != nil {
			idx = nil
		}

		idx, err = n.buildIndexIncremental(safePath, idx, forceRebuild)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to build index: %w", err)
		}

		if err := n.saveIndex(idx, indexPath); err != nil {
			fmt.Fprintf(os.Stderr, "[ckg] warning: failed to save index: %v\n", err)
		}

		files, err = n.collectFiles(safePath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to collect files: %w", err)
		}
	} else {
		if info.IsDir() {
			if incrementalUpdate {
				files, err = n.collectChangedFiles(safePath)
			} else {
				files, err = n.collectFiles(safePath)
			}
			if err != nil {
				return nil, nil, fmt.Errorf("failed to collect files: %w", err)
			}
		} else {
			files = []string{safePath}
		}
	}

	return files, idx, nil
}
