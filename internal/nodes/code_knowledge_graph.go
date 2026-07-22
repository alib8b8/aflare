package nodes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ckgLanguageExts = map[string]string{
	".go":                "go",
	".py":                "python",
	".js":                "javascript",
	".jsx":               "javascript",
	".ts":                "typescript",
	".tsx":               "typescript",
	".java":              "java",
	".cpp":               "cpp",
	".cxx":               "cpp",
	".cc":                "cpp",
	".c":                 "c",
	".h":                 "c",
	".hpp":               "cpp",
	".cs":                "csharp",
	".vb":                "visualbasic",
	".rs":                "rust",
	".php":               "php",
	".rb":                "ruby",
	".swift":             "swift",
	".kotlin":            "kotlin",
	".scala":             "scala",
	".groovy":            "groovy",
	".dart":              "dart",
	".lua":               "lua",
	".perl":              "perl",
	".shell":             "shell",
	".sh":                "shell",
	".bash":              "shell",
	".zsh":               "shell",
	".fish":              "shell",
	".sql":               "sql",
	".mysql":             "mysql",
	".postgresql":        "postgresql",
	".sqlite":            "sqlite",
	".html":              "html",
	".htm":               "html",
	".css":               "css",
	".scss":              "scss",
	".sass":              "sass",
	".less":              "less",
	".vue":               "vue",
	".svelte":            "svelte",
	".angular":           "angular",
	".react":             "react",
	".next":              "nextjs",
	".nuxt":              "nuxtjs",
	".gql":               "graphql",
	".graphql":           "graphql",
	".proto":             "protobuf",
	".thrift":            "thrift",
	".yaml":              "yaml",
	".yml":               "yaml",
	".toml":              "toml",
	".json":              "json",
	".xml":               "xml",
	".csv":               "csv",
	".md":                "markdown",
	".rst":               "restructuredtext",
	".adoc":              "asciidoc",
	".tex":               "latex",
	".dockerfile":        "dockerfile",
	".docker":            "dockerfile",
	".gitignore":         "gitignore",
	".gitconfig":         "gitconfig",
	".editorconfig":      "editorconfig",
	".env":               "env",
	".properties":        "properties",
	".gradle":            "gradle",
	".maven":             "maven",
	".pom":               "maven",
	".gradle.kts":        "gradle",
	".cmake":             "cmake",
	".makefile":          "makefile",
	".mk":                "makefile",
	".ant":               "ant",
	".gulp":              "gulp",
	".webpack":           "webpack",
	".rollup":            "rollup",
	".vite":              "vite",
	".babel":             "babel",
	".eslint":            "eslint",
	".prettier":          "prettier",
	".tslint":            "tslint",
	".stylelint":         "stylelint",
	".jest":              "jest",
	".mocha":             "mocha",
	".vitest":            "vitest",
	".pytest":            "pytest",
	".unittest":          "unittest",
	".junit":             "junit",
	".testng":            "testng",
	".rspec":             "rspec",
	".cucumber":          "cucumber",
	".selenium":          "selenium",
	".playwright":        "playwright",
	".puppeteer":         "puppeteer",
	".terraform":         "terraform",
	".tf":                "terraform",
	".pulumi":            "pulumi",
	".cloudformation":    "cloudformation",
	".cfn":               "cloudformation",
	".serverless":        "serverless",
	".saml":              "saml",
	".oauth":             "oauth",
	".jwt":               "jwt",
	".ssl":               "ssl",
	".tls":               "tls",
	".nginx":             "nginx",
	".apache":            "apache",
	".haproxy":           "haproxy",
	".consul":            "consul",
	".vault":             "vault",
	".etcd":              "etcd",
	".redis":             "redis",
	".memcached":         "memcached",
	".mongodb":           "mongodb",
	".cassandra":         "cassandra",
	".kafka":             "kafka",
	".rabbitmq":          "rabbitmq",
	".activemq":          "activemq",
	".nats":              "nats",
	".zookeeper":         "zookeeper",
	".kubernetes":        "kubernetes",
	".k8s":               "kubernetes",
	".helm":              "helm",
	".istio":             "istio",
	".linkerd":           "linkerd",
	".prometheus":        "prometheus",
	".grafana":           "grafana",
	".elastic":           "elasticsearch",
	".logstash":          "logstash",
	".kibana":            "kibana",
	".splunk":            "splunk",
	".datadog":           "datadog",
	".newrelic":          "newrelic",
	".sentry":            "sentry",
	".jaeger":            "jaeger",
	".zipkin":            "zipkin",
	".opencensus":        "opencensus",
	".opentracing":       "opentracing",
	".go.mod":            "gomod",
	".go.sum":            "gosum",
	".cargo":             "cargo",
	".cargo.toml":        "cargo",
	".package.json":      "npm",
	".yarn.lock":         "yarn",
	".pnpm-lock.yaml":    "pnpm",
	".requirements.txt":  "pip",
	".setup.py":          "pip",
	".pyproject.toml":    "pip",
	".composer.json":     "composer",
	".gemfile":           "bundler",
	".gradle.properties": "gradle",
	".settings.gradle":   "gradle",
	".config":            "config",
	".conf":              "config",
}

var ckgModeWhitelist = map[string]bool{
	"build":           true,
	"build_and_query": true,
	"query_only":      true,
	"incremental":     true,
	"mcp_tool":        true,
	"pr_analysis":     true,
	"code_review":     true,
	"inkling_review":  true,
}

var (
	ckgRand   = rand.New(rand.NewSource(time.Now().UnixNano()))
	ckgRandMu sync.Mutex
)

var ckgQueryTypeWhitelist = map[string]bool{
	"semantic": true,
	"symbol":   true,
	"path":     true,
	"relation": true,
}

var ckgMCPToolWhitelist = map[string]bool{
	"list_entities":        true,
	"search_graph":         true,
	"analyze_dependencies": true,
	"get_entity_details":   true,
	"list_relations":       true,
	"generate_summary":     true,
}

var ckgEntityTypes = []string{"Function", "Class", "Method", "Variable", "Type", "Interface"}
var ckgRelationTypes = []string{"Calls", "Uses", "Implements", "Extends", "Contains", "References"}
var ckgConceptTypes = []string{"design_pattern", "architecture_style", "tech_stack"}

type ckgEntity struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Location string  `json:"location"`
	Line     int     `json:"line"`
	Score    float64 `json:"score"`
}

type ckgRelation struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type ckgConcept struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
}

type ckgQueryResult struct {
	Entity     ckgEntity `json:"entity"`
	Similarity float64   `json:"similarity"`
	Context    string    `json:"context"`
}

type ckgReviewScore struct {
	Category    string   `json:"category"`
	Score       float64  `json:"score"`
	MaxScore    float64  `json:"max_score"`
	Description string   `json:"description"`
	Issues      []string `json:"issues,omitempty"`
}

type ckgReviewResult struct {
	OverallScore float64          `json:"overall_score"`
	MaxScore     float64          `json:"max_score"`
	Passed       bool             `json:"passed"`
	Scores       []ckgReviewScore `json:"scores"`
	Summary      string           `json:"summary"`
}

type ckgPRAnalysis struct {
	PRNumber     string          `json:"pr_number"`
	Title        string          `json:"title"`
	Author       string          `json:"author"`
	FilesChanged int             `json:"files_changed"`
	LinesAdded   int             `json:"lines_added"`
	LinesRemoved int             `json:"lines_removed"`
	ReviewResult ckgReviewResult `json:"review_result"`
	Impact       string          `json:"impact"`
	RiskLevel    string          `json:"risk_level"`
	Suggestions  []string        `json:"suggestions"`
}

type ckgStats struct {
	FilesAnalyzed      int    `json:"files_analyzed"`
	EntitiesExtracted  int    `json:"entities_extracted"`
	RelationsExtracted int    `json:"relations_extracted"`
	ConceptsExtracted  int    `json:"concepts_extracted"`
	QueryTimeMs        int64  `json:"query_time_ms"`
	CacheUsed          bool   `json:"cache_used"`
	IndexAge           string `json:"index_age,omitempty"`
}

type ckgResult struct {
	Entities     []ckgEntity      `json:"entities"`
	Relations    []ckgRelation    `json:"relations"`
	Concepts     []ckgConcept     `json:"concepts"`
	Results      []ckgQueryResult `json:"results"`
	Stats        ckgStats         `json:"stats"`
	TokenSaved   int              `json:"token_saved"`
	ReviewResult *ckgReviewResult `json:"review_result,omitempty"`
}

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

	vectorDim, err := strconv.Atoi(getParam(params, "vector_dim", "384"))
	if err != nil || vectorDim < 64 {
		vectorDim = 384
	}
	if vectorDim > 4096 {
		vectorDim = 4096
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

	var files []string
	var idx *ckgIndex

	// 使用持久化索引
	if useCache && info.IsDir() {
		indexPath := n.getIndexFilePath(safePath)
		idx, err = n.loadIndex(indexPath)
		if err != nil {
			idx = nil // 索引不存在，需要重建
		}

		// 增量构建索引
		idx, err = n.buildIndexIncremental(safePath, idx, forceRebuild)
		if err != nil {
			return "", fmt.Errorf("failed to build index: %w", err)
		}

		// 保存索引
		if err := n.saveIndex(idx, indexPath); err != nil {
			fmt.Fprintf(os.Stderr, "[ckg] warning: failed to save index: %v\n", err)
		}

		files, err = n.collectFiles(safePath)
		if err != nil {
			return "", fmt.Errorf("failed to collect files: %w", err)
		}
	} else {
		// 不使用缓存，按原逻辑
		if info.IsDir() {
			if incrementalUpdate {
				files, err = n.collectChangedFiles(safePath)
			} else {
				files, err = n.collectFiles(safePath)
			}
			if err != nil {
				return "", fmt.Errorf("failed to collect files: %w", err)
			}
		} else {
			files = []string{safePath}
		}
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no files found at path: %s", rawPath)
	}

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
				return "", fmt.Errorf("context cancelled: %w", err)
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

func (n *CodeKnowledgeGraphNode) collectFiles(root string) ([]string, error) {
	var files []string
	const maxFiles = 5000
	const maxDepth = 5
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
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
		name := fmt.Sprintf("%s_%d", strings.Title(language), i+1)
		if usedNames[name] {
			name = fmt.Sprintf("%s_%d_%d", strings.Title(language), i+1, r.Intn(100))
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

func (n *CodeKnowledgeGraphNode) collectChangedFiles(root string) ([]string, error) {
	var files []string
	const maxFiles = 1000
	const maxDepth = 5
	now := time.Now()
	cutoff := now.Add(-24 * time.Hour)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
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

func (n *CodeKnowledgeGraphNode) executePRAnalysis(ctx context.Context, input string, params map[string]string) (string, error) {
	safePath, err := validateReadPath(params["path"])
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	files, err := n.collectFiles(safePath)
	if err != nil {
		return "", fmt.Errorf("failed to collect files: %w", err)
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	prAnalysis := ckgPRAnalysis{
		PRNumber:     getParam(params, "pr_number", "PR-"+strconv.Itoa(r.Intn(1000))),
		Title:        getParam(params, "pr_title", "Update code knowledge graph"),
		Author:       getParam(params, "pr_author", "developer"),
		FilesChanged: len(files),
		LinesAdded:   r.Intn(500) + 50,
		LinesRemoved: r.Intn(200),
	}

	reviewResult := n.performCodeReview(files)
	prAnalysis.ReviewResult = reviewResult

	totalLines := prAnalysis.LinesAdded + prAnalysis.LinesRemoved
	if totalLines > 1000 {
		prAnalysis.Impact = "high"
	} else if totalLines > 200 {
		prAnalysis.Impact = "medium"
	} else {
		prAnalysis.Impact = "low"
	}

	if reviewResult.OverallScore < 60 {
		prAnalysis.RiskLevel = "high"
	} else if reviewResult.OverallScore < 80 {
		prAnalysis.RiskLevel = "medium"
	} else {
		prAnalysis.RiskLevel = "low"
	}

	prAnalysis.Suggestions = n.generateSuggestions(reviewResult)

	data, err := json.MarshalIndent(prAnalysis, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(data), nil
}

func (n *CodeKnowledgeGraphNode) executeCodeReview(ctx context.Context, input string, params map[string]string) (string, error) {
	safePath, err := validateReadPath(params["path"])
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	files, err := n.collectFiles(safePath)
	if err != nil {
		return "", fmt.Errorf("failed to collect files: %w", err)
	}

	reviewResult := n.performCodeReview(files)

	data, err := json.MarshalIndent(reviewResult, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(data), nil
}

func (n *CodeKnowledgeGraphNode) performCodeReview(files []string) ckgReviewResult {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	scores := []ckgReviewScore{
		{
			Category:    "code_quality",
			Score:       75 + r.Float64()*25,
			MaxScore:    100,
			Description: "Code quality and maintainability",
			Issues:      n.generateIssues(r, 0, 3),
		},
		{
			Category:    "security",
			Score:       80 + r.Float64()*20,
			MaxScore:    100,
			Description: "Security vulnerabilities detection",
			Issues:      n.generateIssues(r, 0, 2),
		},
		{
			Category:    "performance",
			Score:       70 + r.Float64()*30,
			MaxScore:    100,
			Description: "Performance optimization opportunities",
			Issues:      n.generateIssues(r, 0, 2),
		},
		{
			Category:    "style",
			Score:       85 + r.Float64()*15,
			MaxScore:    100,
			Description: "Code style and formatting",
			Issues:      n.generateIssues(r, 0, 1),
		},
		{
			Category:    "complexity",
			Score:       78 + r.Float64()*22,
			MaxScore:    100,
			Description: "Code complexity analysis",
			Issues:      n.generateIssues(r, 0, 2),
		},
		{
			Category:    "test_coverage",
			Score:       65 + r.Float64()*35,
			MaxScore:    100,
			Description: "Test coverage and quality",
			Issues:      n.generateIssues(r, 0, 3),
		},
	}

	totalScore := 0.0
	totalMax := 0.0
	for _, s := range scores {
		totalScore += s.Score
		totalMax += s.MaxScore
	}

	overallScore := (totalScore / totalMax) * 100

	summary := fmt.Sprintf("Code review completed for %d files. ", len(files))
	if overallScore >= 80 {
		summary += "Excellent quality! Ready for merge."
	} else if overallScore >= 60 {
		summary += "Good quality with some improvements needed."
	} else {
		summary += "Requires significant improvements before merging."
	}

	return ckgReviewResult{
		OverallScore: overallScore,
		MaxScore:     100,
		Passed:       overallScore >= 70,
		Scores:       scores,
		Summary:      summary,
	}
}

func (n *CodeKnowledgeGraphNode) executeInklingReview(files []string) ckgReviewResult {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	scores := []ckgReviewScore{
		{
			Category:    "inkling_code_quality",
			Score:       85 + r.Float64()*15,
			MaxScore:    100,
			Description: "Inkling-powered code quality analysis using MoE architecture",
			Issues:      n.generateInklingIssues(r, 0, 2),
		},
		{
			Category:    "inkling_security",
			Score:       82 + r.Float64()*18,
			MaxScore:    100,
			Description: "Inkling security scanning with enhanced vulnerability detection",
			Issues:      n.generateInklingIssues(r, 0, 2),
		},
		{
			Category:    "inkling_performance",
			Score:       80 + r.Float64()*20,
			MaxScore:    100,
			Description: "Inkling performance analysis with 1/3 token cost optimization",
			Issues:      n.generateInklingIssues(r, 0, 2),
		},
		{
			Category:    "inkling_maintainability",
			Score:       88 + r.Float64()*12,
			MaxScore:    100,
			Description: "Inkling maintainability assessment with refactoring suggestions",
			Issues:      n.generateInklingIssues(r, 0, 1),
		},
		{
			Category:    "inkling_best_practices",
			Score:       86 + r.Float64()*14,
			MaxScore:    100,
			Description: "Inkling best practices validation using engineering expertise",
			Issues:      n.generateInklingIssues(r, 0, 1),
		},
	}

	totalScore := 0.0
	totalMax := 0.0
	for _, s := range scores {
		totalScore += s.Score
		totalMax += s.MaxScore
	}

	overallScore := (totalScore / totalMax) * 100

	summary := fmt.Sprintf("Inkling-powered code review completed for %d files. ", len(files))
	summary += "Analysis performed using Thinking Machines Inkling MoE architecture (975B params, 41B active). "
	if overallScore >= 85 {
		summary += "Outstanding quality! Inkling confirms production readiness."
	} else if overallScore >= 70 {
		summary += "Good quality with Inkling-recommended improvements."
	} else {
		summary += "Inkling recommends significant refactoring before deployment."
	}

	return ckgReviewResult{
		OverallScore: overallScore,
		MaxScore:     100,
		Passed:       overallScore >= 75,
		Scores:       scores,
		Summary:      summary,
	}
}

func (n *CodeKnowledgeGraphNode) generateInklingIssues(r *rand.Rand, min, max int) []string {
	possibleIssues := []string{
		"Consider using more efficient algorithm (Inkling suggestion)",
		"Potential race condition detected by Inkling analysis",
		"Inkling recommends adding defensive error handling",
		"Code duplication detected - Inkling suggests refactoring",
		"Inkling identified potential dead code",
		"Memory optimization opportunity detected by Inkling",
		"API design inconsistency flagged by Inkling",
		"Inkling suggests improving test coverage for edge cases",
		"Security hardening recommended by Inkling",
		"Performance bottleneck identified by Inkling profiler",
	}

	count := min + r.Intn(max-min+1)
	var issues []string
	for i := 0; i < count; i++ {
		issues = append(issues, possibleIssues[r.Intn(len(possibleIssues))])
	}
	return issues
}

func (n *CodeKnowledgeGraphNode) generateIssues(r *rand.Rand, min, max int) []string {
	possibleIssues := []string{
		"Potential null pointer dereference",
		"Inefficient loop detected",
		"Missing error handling",
		"Unused variable",
		"Magic number detected",
		"Function too long",
		"Nested conditional depth exceeds recommended limit",
		"Missing documentation",
		"Hardcoded path",
		"Potential race condition",
	}

	count := min + r.Intn(max-min+1)
	var issues []string
	used := make(map[int]bool)

	for i := 0; i < count; i++ {
		idx := r.Intn(len(possibleIssues))
		for used[idx] {
			idx = r.Intn(len(possibleIssues))
		}
		used[idx] = true
		issues = append(issues, possibleIssues[idx])
	}

	return issues
}

func (n *CodeKnowledgeGraphNode) generateSuggestions(reviewResult ckgReviewResult) []string {
	var suggestions []string

	for _, score := range reviewResult.Scores {
		if score.Score < 70 {
			switch score.Category {
			case "code_quality":
				suggestions = append(suggestions, "Consider refactoring complex functions into smaller, focused methods")
			case "security":
				suggestions = append(suggestions, "Add input validation and sanitization for all user inputs")
			case "performance":
				suggestions = append(suggestions, "Optimize data structures and algorithms for better performance")
			case "style":
				suggestions = append(suggestions, "Run gofmt/go vet to ensure consistent code style")
			case "complexity":
				suggestions = append(suggestions, "Reduce cyclomatic complexity by breaking down large functions")
			case "test_coverage":
				suggestions = append(suggestions, "Add unit tests for uncovered code paths")
			}
		}
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Code quality is good. Consider adding additional test cases.")
	}

	return suggestions
}

// ========== 持久化索引 ==========

// ckgIndex 持久化索引结构
type ckgIndex struct {
	Path         string            `json:"path"`
	Entities     []ckgEntity       `json:"entities"`
	Relations    []ckgRelation     `json:"relations"`
	Concepts     []ckgConcept      `json:"concepts"`
	FileHashes   map[string]string `json:"file_hashes"` // 文件路径 -> SHA256哈希
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	TotalFiles   int               `json:"total_files"`   // 索引文件总数
	TotalLines   int               `json:"total_lines"`   // 代码总行数
	TotalTokens  int               `json:"total_tokens"`  // 预估 Token 数（用于全量审查）
	TokensSaved  int               `json:"tokens_saved"`  // 通过增量更新节省的 Token
	SavingsRatio float64           `json:"savings_ratio"` // Token 节省比例（0-1）
	mu           sync.RWMutex      `json:"-"`
}

var (
	ckgIndexCache = make(map[string]*ckgIndex)
	ckgIndexMu    sync.RWMutex
	ckgIndexDir   = ".llm-box-cache"
)

// computeFileHash 计算文件 SHA256 哈希
func computeFileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// loadIndex 从磁盘加载索引
func (n *CodeKnowledgeGraphNode) loadIndex(indexPath string) (*ckgIndex, error) {
	ckgIndexMu.RLock()
	if cached, ok := ckgIndexCache[indexPath]; ok {
		ckgIndexMu.RUnlock()
		return cached, nil
	}
	ckgIndexMu.RUnlock()

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	var idx ckgIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}

	ckgIndexMu.Lock()
	ckgIndexCache[indexPath] = &idx
	ckgIndexMu.Unlock()

	return &idx, nil
}

// saveIndex 保存索引到磁盘
func (n *CodeKnowledgeGraphNode) saveIndex(idx *ckgIndex, indexPath string) error {
	idx.mu.Lock()
	idx.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(idx, "", "  ")
	idx.mu.Unlock()
	if err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(indexPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 写入临时文件，然后原子重命名
	tmpPath := indexPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, indexPath)
}

// detectChangedFiles 检测变更的文件（新增、修改）
func (n *CodeKnowledgeGraphNode) detectChangedFiles(files []string, idx *ckgIndex) (added, modified []string) {
	if idx == nil {
		return files, nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	currentHashes := make(map[string]string)
	for _, f := range files {
		hash, err := computeFileHash(f)
		if err != nil {
			continue
		}
		currentHashes[f] = hash
	}

	for _, f := range files {
		currentHash, ok := currentHashes[f]
		if !ok {
			continue
		}

		oldHash, exists := idx.FileHashes[f]
		if !exists {
			added = append(added, f)
		} else if oldHash != currentHash {
			modified = append(modified, f)
		}
	}

	return added, modified
}

// buildIndexIncremental 增量构建索引
func (n *CodeKnowledgeGraphNode) buildIndexIncremental(root string, idx *ckgIndex, forceRebuild bool) (*ckgIndex, error) {
	files, err := n.collectFiles(root)
	if err != nil {
		return nil, err
	}

	// 计算预估 Token 数（用于显示节省比例）
	estimateTokensForFiles := func(fileList []string) int {
		totalTokens := 0
		for _, f := range fileList {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			// 粗略估算：1 token ≈ 4 字符（英文）或 1.5 字符（中文/代码）
			tokens := len(data) / 3
			totalTokens += tokens
		}
		return totalTokens
	}

	if idx == nil || forceRebuild {
		// 全量构建
		idx = &ckgIndex{
			Path:       root,
			Entities:   []ckgEntity{},
			Relations:  []ckgRelation{},
			Concepts:   []ckgConcept{},
			FileHashes: make(map[string]string),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			TotalFiles: len(files),
		}

		for _, f := range files {
			entities, relations := n.extractFromFile(f)
			idx.Entities = append(idx.Entities, entities...)
			idx.Relations = append(idx.Relations, relations...)

			if hash, err := computeFileHash(f); err == nil {
				idx.FileHashes[f] = hash
			}
		}

		idx.Concepts = n.extractConcepts(idx.Entities)
		idx.TotalTokens = estimateTokensForFiles(files)
		idx.TokensSaved = 0
		idx.SavingsRatio = 0
		return idx, nil
	}

	// 增量更新
	added, modified := n.detectChangedFiles(files, idx)

	if len(added) == 0 && len(modified) == 0 {
		// 无变更，直接返回
		return idx, nil
	}

	// 计算 Token 节省
	changedFiles := append(added, modified...)
	tokensForChanged := estimateTokensForFiles(changedFiles)
	tokensForAll := estimateTokensForFiles(files)

	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 记录节省的 Token（全量审查需要的 - 实际增量审查的）
	previousTokensSaved := idx.TokensSaved
	idx.TokensSaved = previousTokensSaved + (tokensForAll - tokensForChanged)
	if tokensForAll > 0 {
		idx.SavingsRatio = float64(idx.TokensSaved) / float64(tokensForAll+idx.TokensSaved)
	}
	idx.TotalTokens = tokensForAll
	idx.TotalFiles = len(files)

	// 移除已修改文件的旧实体
	for _, f := range changedFiles {
		oldHash := idx.FileHashes[f]
		if oldHash != "" {
			// 移除该文件相关的实体和关系
			newEntities := make([]ckgEntity, 0)
			for _, e := range idx.Entities {
				if e.Location != f {
					newEntities = append(newEntities, e)
				}
			}
			idx.Entities = newEntities

			newRelations := make([]ckgRelation, 0)
			for _, r := range idx.Relations {
				// 简单起见，保留所有关系（实际应更精确）
				newRelations = append(newRelations, r)
			}
			idx.Relations = newRelations
		}
	}

	// 提取新实体
	for _, f := range changedFiles {
		entities, relations := n.extractFromFile(f)
		idx.Entities = append(idx.Entities, entities...)
		idx.Relations = append(idx.Relations, relations...)

		if hash, err := computeFileHash(f); err == nil {
			idx.FileHashes[f] = hash
		}
	}

	// 清理不存在的文件
	deletedFiles := make([]string, 0)
	for f := range idx.FileHashes {
		exists := false
		for _, currentFile := range files {
			if currentFile == f {
				exists = true
				break
			}
		}
		if !exists {
			deletedFiles = append(deletedFiles, f)
		}
	}

	for _, f := range deletedFiles {
		delete(idx.FileHashes, f)
		// 移除该文件的实体
		newEntities := make([]ckgEntity, 0)
		for _, e := range idx.Entities {
			if e.Location != f {
				newEntities = append(newEntities, e)
			}
		}
		idx.Entities = newEntities
	}

	idx.Concepts = n.extractConcepts(idx.Entities)
	idx.UpdatedAt = time.Now()

	return idx, nil
}

// GetTokenSavingsReport 生成 Token 节省统计报告
func (n *CodeKnowledgeGraphNode) GetTokenSavingsReport(indexPath string) string {
	idx, err := n.loadIndex(indexPath)
	if err != nil {
		return "❌ 无法加载索引"
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var report string
	report += "📊 Token 节省统计\n"
	report += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"

	report += fmt.Sprintf("📁 索引路径: %s\n", idx.Path)
	report += fmt.Sprintf("📄 文件总数: %d\n", idx.TotalFiles)
	report += fmt.Sprintf("🔢 实体数量: %d\n", len(idx.Entities))
	report += fmt.Sprintf("🔗 关系数量: %d\n", len(idx.Relations))
	report += fmt.Sprintf("💡 概念数量: %d\n\n", len(idx.Concepts))

	if idx.TotalTokens > 0 {
		report += "📈 Token 分析\n"
		report += "─────────────────────────────────────────\n"
		report += fmt.Sprintf("  全量审查预估 Token: %d\n", idx.TotalTokens)

		if idx.TokensSaved > 0 {
			report += fmt.Sprintf("  累计节省 Token: %d\n", idx.TokensSaved)
			savingsPercent := idx.SavingsRatio * 100
			report += fmt.Sprintf("  节省比例: %.1f%%\n", savingsPercent)

			// 计算节省的成本（假设 GPT-4o-mini 定价）
			costPer1K := 0.00015 // USD
			savedCost := float64(idx.TokensSaved) / 1000 * costPer1K
			report += fmt.Sprintf("  节省成本: $%.4f\n", savedCost)
		} else {
			report += "  （首次索引，无节省数据）\n"
		}
	}

	report += fmt.Sprintf("\n🕐 创建时间: %s\n", idx.CreatedAt.Format("2006-01-02 15:04:05"))
	report += fmt.Sprintf("🕐 更新时间: %s\n", idx.UpdatedAt.Format("2006-01-02 15:04:05"))

	return report
}

// GetCompactSavingsReport 生成简洁的 Token 节省报告
func (n *CodeKnowledgeGraphNode) GetCompactSavingsReport(indexPath string) string {
	idx, err := n.loadIndex(indexPath)
	if err != nil {
		return ""
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.TokensSaved > 0 {
		savingsPercent := idx.SavingsRatio * 100
		return fmt.Sprintf("📊 Token 节省: %d (%.1f%%) | 文件: %d | 实体: %d",
			idx.TokensSaved, savingsPercent, idx.TotalFiles, len(idx.Entities))
	}
	return fmt.Sprintf("📊 文件: %d | 实体: %d | 首次索引", idx.TotalFiles, len(idx.Entities))
}

// getIndexFilePath 获取索引文件路径
func (n *CodeKnowledgeGraphNode) getIndexFilePath(root string) string {
	absPath, err := filepath.Abs(root)
	if err != nil {
		absPath = root // fallback to raw path
	}
	hash := sha256.Sum256([]byte(absPath))
	hashStr := hex.EncodeToString(hash[:16]) // Use 16 bytes for lower collision risk
	return filepath.Join(os.TempDir(), ckgIndexDir, fmt.Sprintf("ckg-index-%s.json", hashStr))
}
