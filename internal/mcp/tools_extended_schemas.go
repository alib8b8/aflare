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

package mcp

// ------------------------------------------------------------------
// Extended tool schemas
// ------------------------------------------------------------------

func (s *Server) getExtendedTools() []tool {
	return []tool{
		// Backwards-compatible aliases
		{
			Name:        "create_workflow",
			Description: "Generate a YAML workflow from a plain English description. Returns the workflow YAML content.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Plain English description of the workflow to generate (e.g., 'fetch Hacker News and save to file')",
					},
				},
				Required: []string{"description"},
			},
		},
		{
			Name:        "run_workflow",
			Description: "Execute a llm-box workflow from a YAML file path. Returns the final output of the workflow.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"file": map[string]interface{}{
						"type":        "string",
						"description": "Path to the YAML workflow file to execute",
					},
				},
				Required: []string{"file"},
			},
		},
		{
			Name:        "run_workflow_yaml",
			Description: "Execute a llm-box workflow from raw YAML content. Returns the final output of the workflow.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"yaml": map[string]interface{}{
						"type":        "string",
						"description": "Raw YAML content of the workflow to execute",
					},
				},
				Required: []string{"yaml"},
			},
		},
		{
			Name:        "list_nodes",
			Description: "List all available llm-box nodes with their descriptions. Call this to discover what nodes can be used in workflows.",
			InputSchema: inputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
		{
			Name:        "validate_workflow",
			Description: "Validate a llm-box workflow YAML file without executing it. Returns validation warnings if any.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"file": map[string]interface{}{
						"type":        "string",
						"description": "Path to the YAML workflow file to validate",
					},
				},
				Required: []string{"file"},
			},
		},
		// New tools (requested names)
		{
			Name:        "workflow_run",
			Description: "Run a workflow from a YAML file path with optional timeout override.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"file": map[string]interface{}{
						"type":        "string",
						"description": "Path to the YAML workflow file to execute",
					},
					"timeout_seconds": map[string]interface{}{
						"type":        "integer",
						"description": "Optional timeout in seconds (default 30, max 300)",
						"default":     30,
					},
				},
				Required: []string{"file"},
			},
		},
		{
			Name:        "workflow_create",
			Description: "Create a new workflow from a plain English description.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Plain English description of the workflow to generate",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Optional workflow name",
					},
				},
				Required: []string{"description"},
			},
		},
		{
			Name:        "workflow_list",
			Description: "List available workflow files in a directory.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"directory": map[string]interface{}{
						"type":        "string",
						"description": "Directory to scan for workflow files (default: current working directory)",
					},
				},
			},
		},
		{
			Name:        "workflow_validate",
			Description: "Validate a workflow YAML file or raw YAML content without executing it.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"file": map[string]interface{}{
						"type":        "string",
						"description": "Path to the YAML workflow file to validate",
					},
					"yaml": map[string]interface{}{
						"type":        "string",
						"description": "Raw YAML content to validate (used if file is not provided)",
					},
				},
			},
		},
		{
			Name:        "node_list",
			Description: "List all available nodes with name and description.",
			InputSchema: inputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
		{
			Name:        "node_info",
			Description: "Get detailed information about a specific node including its parameter schema.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the node to query",
					},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "history_list",
			Description: "List workflow execution history with optional filtering.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of records to return (default 50, max 200)",
						"default":     50,
					},
					"success_only": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, only return successful executions",
						"default":     false,
					},
					"workflow": map[string]interface{}{
						"type":        "string",
						"description": "Filter by workflow name (partial match)",
					},
				},
			},
		},
		{
			Name:        "template_list",
			Description: "List available workflow templates.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Filter by category",
					},
					"keyword": map[string]interface{}{
						"type":        "string",
						"description": "Search keyword for template name or description",
					},
				},
			},
		},
		{
			Name:        "template_render",
			Description: "Render a workflow template with variables.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the template to render",
					},
					"vars": map[string]interface{}{
						"type":        "object",
						"description": "Variables to pass to the template (key-value map)",
					},
				},
				Required: []string{"name"},
			},
		},
		// Memory tools (session-isolated)
		{
			Name:        "memory_store",
			Description: "Store a memory entry in the session-isolated memory system.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session ID for isolated memory (default: 'default')",
						"default":     "default",
					},
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Memory key (optional, auto-generated if empty)",
					},
					"value": map[string]interface{}{
						"type":        "string",
						"description": "Memory content to store",
					},
					"level": map[string]interface{}{
						"type":        "string",
						"description": "Memory level: short/medium/long (default: medium)",
						"default":     "medium",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Memory type: fact/concept/experience/preference/relationship/task/context (default: fact)",
						"default":     "fact",
					},
					"tags": map[string]interface{}{
						"type":        "string",
						"description": "Comma-separated tags for categorization",
					},
					"confidence": map[string]interface{}{
						"type":        "number",
						"description": "Confidence level 0.0-1.0 (default: 0.8)",
						"default":     0.8,
					},
				},
				Required: []string{"value"},
			},
		},
		{
			Name:        "memory_retrieve",
			Description: "Retrieve a memory entry by key from session-isolated memory.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session ID (default: 'default')",
						"default":     "default",
					},
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Memory key to retrieve",
					},
				},
				Required: []string{"key"},
			},
		},
		{
			Name:        "memory_search",
			Description: "Search memory entries matching a query in session-isolated memory.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session ID (default: 'default')",
						"default":     "default",
					},
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query",
					},
					"level": map[string]interface{}{
						"type":        "string",
						"description": "Filter by memory level: short/medium/long (optional)",
					},
					"top_k": map[string]interface{}{
						"type":        "integer",
						"description": "Number of results (1-100, default: 10)",
						"default":     10,
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "memory_stats",
			Description: "Get memory statistics for a session or global stats.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session ID (default: 'default', use 'global' for all sessions)",
						"default":     "default",
					},
				},
			},
		},
		{
			Name:        "memory_list_sessions",
			Description: "List all active memory sessions.",
			InputSchema: inputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
		// Code knowledge graph tools
		{
			Name:        "code_graph_index",
			Description: "Build or update the code knowledge graph index for a directory. Enables incremental updates and reduces token usage for large repos.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the code directory to index (default: current working directory)",
					},
				},
			},
		},
		{
			Name:        "code_graph_query",
			Description: "Query the code knowledge graph to find entities, relations, and concepts related to your query.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query for the code graph (e.g., 'authentication', 'database', 'error handling')",
					},
					"top_k": map[string]interface{}{
						"type":        "integer",
						"description": "Number of results (default: 10)",
						"default":     10,
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "code_graph_stats",
			Description: "Get code knowledge graph statistics including total files, tokens saved, and index size.",
			InputSchema: inputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
		// Vertical domain tools (geeflow/headroom/last30days inspired)
		{
			Name:        "context_compress",
			Description: "Intelligent context compression with 6 algorithms (extract/keyword/cluster/sliding_window/hybrid). Reduces tokens by 60-95%.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Text content to compress",
					},
					"algorithm": map[string]interface{}{
						"type":        "string",
						"description": "Compression algorithm: extract|keyword|cluster|sliding_window|hybrid (default: hybrid)",
						"default":     "hybrid",
					},
					"ratio": map[string]interface{}{
						"type":        "number",
						"description": "Target compression ratio 0.01-1.0, lower=more aggressive (default: 0.2)",
						"default":     0.2,
					},
					"max_chars": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum output characters (default: 4000)",
						"default":     4000,
					},
				},
				Required: []string{"text"},
			},
		},
		{
			Name:        "search_aggregated",
			Description: "Multi-platform search with real-signal ranking (Hacker News, GitHub, Reddit). Sorted by votes/comments instead of SEO.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query",
					},
					"sources": map[string]interface{}{
						"type":        "string",
						"description": "Comma-separated sources: reddit,hn,github,twitter,youtube,google (default: hn,github,reddit)",
						"default":     "hn,github,reddit",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Max results per source (default: 10)",
						"default":     10,
					},
					"time_range": map[string]interface{}{
						"type":        "string",
						"description": "Time range: day|week|month|year|all (default: week)",
						"default":     "week",
					},
					"sort_by": map[string]interface{}{
						"type":        "string",
						"description": "Sort by: signal|relevance|time (default: signal)",
						"default":     "signal",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "geospatial_query",
			Description: "Natural language geospatial analysis (geeflow-inspired). Translate queries into GIS/remote sensing operations.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Natural language geospatial query (e.g., 'Show NDVI trend in Amazon basin 2020-2024')",
					},
					"dataset": map[string]interface{}{
						"type":        "string",
						"description": "Dataset: sentinel2|landsat8|modis|elevation|population|nightlights (default: sentinel2)",
						"default":     "sentinel2",
					},
					"region": map[string]interface{}{
						"type":        "string",
						"description": "Region name or WGS84 bounding box (minLon,minLat,maxLon,maxLat)",
					},
					"time_start": map[string]interface{}{
						"type":        "string",
						"description": "Start date YYYY-MM-DD",
					},
					"time_end": map[string]interface{}{
						"type":        "string",
						"description": "End date YYYY-MM-DD",
					},
					"output_format": map[string]interface{}{
						"type":        "string",
						"description": "Output format: geojson|csv|png|summary (default: summary)",
						"default":     "summary",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "preference_get",
			Description: "Get a learned user preference from the user profile. Preferences are learned across sessions (MemSlides-inspired).",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"user_id": map[string]interface{}{
						"type":        "string",
						"description": "User ID (default: 'default')",
						"default":     "default",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Preference category: coding_style|output_format|model_choice|verbosity|language|safety|workflow|custom",
						"default":     "custom",
					},
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Preference key name",
					},
				},
				Required: []string{"key"},
			},
		},
		{
			Name:        "preference_set",
			Description: "Set or learn a user preference. Stored persistently across sessions (MemSlides-inspired user profiling).",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"user_id": map[string]interface{}{
						"type":        "string",
						"description": "User ID (default: 'default')",
						"default":     "default",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Preference category: coding_style|output_format|model_choice|verbosity|language|safety|workflow|custom",
						"default":     "custom",
					},
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Preference key name",
					},
					"value": map[string]interface{}{
						"type":        "string",
						"description": "Preference value",
					},
					"learn": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, lower confidence and count as observation (learned from interaction). If false, explicit preference.",
						"default":     false,
					},
				},
				Required: []string{"key", "value"},
			},
		},
	}
}
