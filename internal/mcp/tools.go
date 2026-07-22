package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alib8b8/llm-box/internal/history"
	"github.com/alib8b8/llm-box/internal/memory"
	"github.com/alib8b8/llm-box/internal/nodes"
	"github.com/alib8b8/llm-box/internal/templates"
	"github.com/alib8b8/llm-box/internal/workflow"
	"gopkg.in/yaml.v3"
)

// toolCallTimeout is the maximum duration allowed for a single tool call.
const toolCallTimeout = 30 * time.Second

// sanitizeError removes potentially sensitive information from error messages.
func sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	// Remove file paths that may contain home directories
	if home, _ := os.UserHomeDir(); home != "" {
		msg = strings.ReplaceAll(msg, home, "~")
	}
	// Redact tokens/keys if they appear in the message
	sensitivePatterns := []string{"token", "key", "secret", "password", "credential"}
	lowerMsg := strings.ToLower(msg)
	for _, p := range sensitivePatterns {
		if strings.Contains(lowerMsg, p) {
			return fmt.Errorf("tool execution failed (sensitive details redacted)")
		}
	}
	return fmt.Errorf("%s", msg)
}

// requireString validates that args contains a non-empty string for the given key.
func requireString(args map[string]interface{}, key string) (string, error) {
	v, ok := args[key].(string)
	if !ok || strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("parameter %q is required and must be a non-empty string", key)
	}
	return v, nil
}

// optionalString returns the string value for key, or empty string if missing/invalid.
func optionalString(args map[string]interface{}, key string) string {
	v, ok := args[key].(string)
	if !ok {
		return ""
	}
	return v
}

// optionalBool returns the bool value for key, or the default if missing/invalid.
func optionalBool(args map[string]interface{}, key string, def bool) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		if strings.ToLower(v) == "true" {
			return true
		} else if strings.ToLower(v) == "false" {
			return false
		}
	}
	return def
}

// optionalInt returns the int value for key, or the default if missing/invalid.
func optionalInt(args map[string]interface{}, key string, def int) int {
	switch v := args[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return def
}

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
	}
}

// ------------------------------------------------------------------
// Tool call dispatch
// ------------------------------------------------------------------

func (s *Server) callExtendedTool(params *toolCallParams) (*toolCallResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), toolCallTimeout)
	defer cancel()

	// Execute the actual work in a goroutine so the timeout cancels it.
	type result struct {
		res *toolCallResult
		err error
	}
	done := make(chan result, 1)

	go func() {
		var r result
		switch params.Name {
		// Backwards-compatible tools
		case "create_workflow":
			r.res, r.err = s.createWorkflow(params.Arguments)
		case "run_workflow":
			r.res, r.err = s.runWorkflow(params.Arguments)
		case "run_workflow_yaml":
			r.res, r.err = s.runWorkflowYAML(params.Arguments)
		case "list_nodes":
			r.res, r.err = s.listNodes()
		case "validate_workflow":
			r.res, r.err = s.validateWorkflow(params.Arguments)
		// New tools
		case "workflow_run":
			r.res, r.err = s.toolWorkflowRun(params.Arguments)
		case "workflow_create":
			r.res, r.err = s.toolWorkflowCreate(params.Arguments)
		case "workflow_list":
			r.res, r.err = s.toolWorkflowList(params.Arguments)
		case "workflow_validate":
			r.res, r.err = s.toolWorkflowValidate(params.Arguments)
		case "node_list":
			r.res, r.err = s.toolNodeList()
		case "node_info":
			r.res, r.err = s.toolNodeInfo(params.Arguments)
		case "history_list":
			r.res, r.err = s.toolHistoryList(params.Arguments)
		case "template_list":
			r.res, r.err = s.toolTemplateList(params.Arguments)
		case "template_render":
			r.res, r.err = s.toolTemplateRender(params.Arguments)
		// Memory tools
		case "memory_store":
			r.res, r.err = s.toolMemoryStore(params.Arguments)
		case "memory_retrieve":
			r.res, r.err = s.toolMemoryRetrieve(params.Arguments)
		case "memory_search":
			r.res, r.err = s.toolMemorySearch(params.Arguments)
		case "memory_stats":
			r.res, r.err = s.toolMemoryStats(params.Arguments)
		case "memory_list_sessions":
			r.res, r.err = s.toolMemoryListSessions()
		// Code graph tools
		case "code_graph_index":
			r.res, r.err = s.toolCodeGraphIndex(params.Arguments)
		case "code_graph_query":
			r.res, r.err = s.toolCodeGraphQuery(params.Arguments)
		case "code_graph_stats":
			r.res, r.err = s.toolCodeGraphStats()
		default:
			r.err = fmt.Errorf("unknown tool: %s", params.Name)
		}
		done <- r
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("tool call timed out after %v", toolCallTimeout)
	case r := <-done:
		if r.err != nil {
			return nil, sanitizeError(r.err)
		}
		return r.res, nil
	}
}

// ------------------------------------------------------------------
// Individual tool implementations
// ------------------------------------------------------------------

func (s *Server) toolWorkflowRun(args map[string]interface{}) (*toolCallResult, error) {
	file, err := requireString(args, "file")
	if err != nil {
		return nil, err
	}

	timeoutSec := optionalInt(args, "timeout_seconds", 30)
	if timeoutSec < 1 {
		timeoutSec = 1
	}
	if timeoutSec > 300 {
		timeoutSec = 300
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	wf, err := workflow.ParseWorkflow(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	reg := nodes.GetGlobalRegistry()
	nodes.RegisterBuiltins(reg)

	result, _, err := workflow.ExecuteWorkflow(ctx, wf, reg)
	if err != nil {
		return nil, fmt.Errorf("workflow execution failed: %w", err)
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: result}},
	}, nil
}

func (s *Server) toolWorkflowCreate(args map[string]interface{}) (*toolCallResult, error) {
	desc, err := requireString(args, "description")
	if err != nil {
		return nil, err
	}

	wf, err := workflow.GenerateWorkflow(desc)
	if err != nil {
		return nil, fmt.Errorf("failed to generate workflow: %w", err)
	}

	if name := optionalString(args, "name"); name != "" {
		wf.Name = name
	}

	yamlBytes, err := yaml.Marshal(wf)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow: %w", err)
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: string(yamlBytes)}},
	}, nil
}

func (s *Server) toolWorkflowList(args map[string]interface{}) (*toolCallResult, error) {
	dir := optionalString(args, "directory")
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("invalid directory: %w", err)
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, name)
		}
	}

	if len(files) == 0 {
		return &toolCallResult{
			Content: []content{{Type: "text", Text: "No workflow files found in " + absDir}},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Workflow files in %s:\n\n", absDir))
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("  - %s\n", f))
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: sb.String()}},
	}, nil
}

func (s *Server) toolWorkflowValidate(args map[string]interface{}) (*toolCallResult, error) {
	file := optionalString(args, "file")
	yamlStr := optionalString(args, "yaml")

	var wf *workflow.Workflow
	var err error

	if file != "" {
		wf, err = workflow.ParseWorkflow(file)
		if err != nil {
			return nil, fmt.Errorf("failed to parse workflow file: %w", err)
		}
	} else if yamlStr != "" {
		if len(yamlStr) > workflow.MaxFileSize {
			return nil, fmt.Errorf("workflow YAML too large (max %d bytes)", workflow.MaxFileSize)
		}
		wf = &workflow.Workflow{}
		if err := yaml.Unmarshal([]byte(yamlStr), wf); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
	} else {
		return nil, fmt.Errorf("either 'file' or 'yaml' parameter is required")
	}

	warnings := workflow.ValidateWorkflow(wf)

	reg := nodes.GetGlobalRegistry()
	nodes.RegisterBuiltins(reg)

	for i, step := range wf.Steps {
		if _, ok := reg.Get(step.Node); !ok {
			warnings = append(warnings, fmt.Sprintf("Step %d: unknown node '%s'", i+1, step.Node))
		}
	}

	var sb strings.Builder
	if len(warnings) == 0 {
		sb.WriteString("Workflow is valid. No issues found.")
	} else {
		sb.WriteString("Validation warnings:\n")
		for _, w := range warnings {
			sb.WriteString(fmt.Sprintf("  - %s\n", w))
		}
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: sb.String()}},
	}, nil
}

func (s *Server) toolNodeList() (*toolCallResult, error) {
	reg := nodes.GetGlobalRegistry()
	nodes.RegisterBuiltins(reg)

	nodeList := reg.ListNodes()

	var sb strings.Builder
	sb.WriteString("Available nodes:\n\n")
	sb.WriteString(fmt.Sprintf("%-20s %s\n", "NAME", "DESCRIPTION"))
	sb.WriteString(strings.Repeat("-", 70))
	sb.WriteString("\n")
	for _, info := range nodeList {
		sb.WriteString(fmt.Sprintf("%-20s %s\n", info.Name, info.Description))
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: sb.String()}},
	}, nil
}

func (s *Server) toolNodeInfo(args map[string]interface{}) (*toolCallResult, error) {
	name, err := requireString(args, "name")
	if err != nil {
		return nil, err
	}

	reg := nodes.GetGlobalRegistry()
	nodes.RegisterBuiltins(reg)

	node, ok := reg.Get(name)
	if !ok {
		return nil, fmt.Errorf("node not found: %s", name)
	}

	schema := node.Schema()
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Node: %s\n", node.Name()))
	sb.WriteString(fmt.Sprintf("Description: %s\n\n", node.Description()))
	sb.WriteString("Schema:\n")
	sb.WriteString(string(data))

	return &toolCallResult{
		Content: []content{{Type: "text", Text: sb.String()}},
	}, nil
}

func (s *Server) toolHistoryList(args map[string]interface{}) (*toolCallResult, error) {
	limit := optionalInt(args, "limit", 50)
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}

	successOnly := optionalBool(args, "success_only", false)
	workflowFilter := optionalString(args, "workflow")

	filter := history.RecordFilter{}
	if successOnly {
		v := true
		filter.Success = &v
	}
	if workflowFilter != "" {
		filter.Workflow = workflowFilter
	}

	records, err := history.ListRecordsWithFilter(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list history: %w", err)
	}

	if len(records) > limit {
		records = records[:limit]
	}

	if len(records) == 0 {
		return &toolCallResult{
			Content: []content{{Type: "text", Text: "No history records found."}},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("History records (%d shown):\n\n", len(records)))
	sb.WriteString(fmt.Sprintf("%-26s %-20s %-10s %-8s %s\n", "STARTED", "NAME", "TRIGGER", "STATUS", "DURATION"))
	sb.WriteString(strings.Repeat("-", 90))
	sb.WriteString("\n")
	for _, r := range records {
		status := "success"
		if !r.Success {
			status = "failed"
		}
		sb.WriteString(fmt.Sprintf("%-26s %-20s %-10s %-8s %v\n",
			r.StartedAt.Format(time.RFC3339),
			truncate(r.Name, 20),
			r.Trigger,
			status,
			r.Duration,
		))
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: sb.String()}},
	}, nil
}

func (s *Server) toolTemplateList(args map[string]interface{}) (*toolCallResult, error) {
	tm := templates.NewTemplateManager()

	category := optionalString(args, "category")
	keyword := optionalString(args, "keyword")

	var list []*templates.Template
	if keyword != "" {
		list = tm.Search(keyword)
	} else if category != "" {
		list = tm.ListByCategory(category)
	} else {
		list = tm.List()
	}

	if len(list) == 0 {
		return &toolCallResult{
			Content: []content{{Type: "text", Text: "No templates found."}},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Templates (%d found):\n\n", len(list)))
	sb.WriteString(fmt.Sprintf("%-20s %-15s %-30s %s\n", "NAME", "CATEGORY", "DESCRIPTION", "VERSION"))
	sb.WriteString(strings.Repeat("-", 100))
	sb.WriteString("\n")
	for _, t := range list {
		sb.WriteString(fmt.Sprintf("%-20s %-15s %-30s %s\n",
			truncate(t.Name, 20),
			truncate(t.Category, 15),
			truncate(t.Description, 30),
			t.Version,
		))
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: sb.String()}},
	}, nil
}

func (s *Server) toolTemplateRender(args map[string]interface{}) (*toolCallResult, error) {
	name, err := requireString(args, "name")
	if err != nil {
		return nil, err
	}

	vars := make(map[string]string)
	if rawVars, ok := args["vars"].(map[string]interface{}); ok {
		for k, v := range rawVars {
			if str, ok := v.(string); ok {
				vars[k] = str
			} else {
				vars[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	tm := templates.NewTemplateManager()
	rendered, err := tm.Render(name, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: rendered}},
	}, nil
}

// ------------------------------------------------------------------
// Memory tool implementations
// ------------------------------------------------------------------

func (s *Server) toolMemoryStore(args map[string]interface{}) (*toolCallResult, error) {
	sessionID := optionalString(args, "session_id")
	if sessionID == "" {
		sessionID = "default"
	}
	value, err := requireString(args, "value")
	if err != nil {
		return nil, err
	}
	key := optionalString(args, "key")
	level := optionalString(args, "level")
	if level == "" {
		level = "medium"
	}
	memType := optionalString(args, "type")
	if memType == "" {
		memType = "fact"
	}
	tagsStr := optionalString(args, "tags")
	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}
	confidence := 0.8
	switch v := args["confidence"].(type) {
	case float64:
		confidence = v
	case int:
		confidence = float64(v)
	}

	sess := memory.GetSession(sessionID)
	id, expiresAt, err := sess.Store(key, value, level, memType, tags, 72, confidence, "mcp")
	if err != nil {
		return nil, fmt.Errorf("failed to store memory: %w", err)
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: fmt.Sprintf("Stored: key=%s id=%s expires=%s", key, id, expiresAt.Format(time.RFC3339))}},
	}, nil
}

func (s *Server) toolMemoryRetrieve(args map[string]interface{}) (*toolCallResult, error) {
	sessionID := optionalString(args, "session_id")
	if sessionID == "" {
		sessionID = "default"
	}
	key, err := requireString(args, "key")
	if err != nil {
		return nil, err
	}

	sess := memory.GetSession(sessionID)
	entry, err := sess.Retrieve(key)
	if err != nil {
		return nil, err
	}

	data, _ := json.MarshalIndent(entry, "", "  ")
	return &toolCallResult{
		Content: []content{{Type: "text", Text: string(data)}},
	}, nil
}

func (s *Server) toolMemorySearch(args map[string]interface{}) (*toolCallResult, error) {
	sessionID := optionalString(args, "session_id")
	if sessionID == "" {
		sessionID = "default"
	}
	query, err := requireString(args, "query")
	if err != nil {
		return nil, err
	}
	level := optionalString(args, "level")
	topK := optionalInt(args, "top_k", 10)

	sess := memory.GetSession(sessionID)
	results := sess.Search(query, level, topK, 0.3)

	if len(results) == 0 {
		return &toolCallResult{
			Content: []content{{Type: "text", Text: "No matching memory entries found."}},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d memory entries:\n\n", len(results)))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s (key: %s)\n", i+1, r.Type, r.Value, r.Key))
		if len(r.Value) > 150 {
			sb.WriteString(fmt.Sprintf("   Preview: %s...\n", r.Value[:147]))
		}
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: sb.String()}},
	}, nil
}

func (s *Server) toolMemoryStats(args map[string]interface{}) (*toolCallResult, error) {
	sessionID := optionalString(args, "session_id")
	if sessionID == "" {
		sessionID = "default"
	}

	if sessionID == "global" {
		gs := memory.GetGlobalStats()
		return &toolCallResult{
			Content: []content{{Type: "text", Text: fmt.Sprintf(
				"Global Memory: %d sessions, %d entries, %.2f MB total",
				gs.ActiveSessions, gs.TotalEntries, gs.TotalEstimatedMB,
			)}},
		}, nil
	}

	sess := memory.GetSession(sessionID)
	stats := sess.GetStats()
	return &toolCallResult{
		Content: []content{{Type: "text", Text: fmt.Sprintf(
			"Session '%s': %d entries (S:%d M:%d L:%d), %.2f KB, %d accesses",
			sessionID, stats.TotalEntries,
			stats.ShortTermCount, stats.MediumTermCount, stats.LongTermCount,
			float64(stats.EstimatedBytes)/1024, stats.TotalAccesses,
		)}},
	}, nil
}

func (s *Server) toolMemoryListSessions() (*toolCallResult, error) {
	sessions := memory.ListSessions()
	gs := memory.GetGlobalStats()

	if len(sessions) == 0 {
		return &toolCallResult{
			Content: []content{{Type: "text", Text: "No active memory sessions."}},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Active memory sessions (%d):\n\n", len(sessions)))
	for _, id := range sessions {
		ss := gs.PerSession[id]
		sb.WriteString(fmt.Sprintf("  • %s: %d entries (%.2f KB)\n",
			id, ss.TotalEntries, float64(ss.EstimatedBytes)/1024))
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: sb.String()}},
	}, nil
}

// ------------------------------------------------------------------
// Code graph tool implementations
// ------------------------------------------------------------------

func (s *Server) toolCodeGraphIndex(args map[string]interface{}) (*toolCallResult, error) {
	path := optionalString(args, "path")
	if path == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	reg := nodes.GetGlobalRegistry()
	nodes.RegisterBuiltins(reg)
	ckg, ok := reg.Get("code_knowledge_graph")
	if !ok {
		return nil, fmt.Errorf("code_knowledge_graph node not available")
	}

	result, err := ckg.Execute(context.Background(), path, map[string]string{
		"operation": "index",
	})
	if err != nil {
		return nil, fmt.Errorf("code graph indexing failed: %w", err)
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: result}},
	}, nil
}

func (s *Server) toolCodeGraphQuery(args map[string]interface{}) (*toolCallResult, error) {
	query, err := requireString(args, "query")
	if err != nil {
		return nil, err
	}
	topK := optionalInt(args, "top_k", 10)

	reg := nodes.GetGlobalRegistry()
	nodes.RegisterBuiltins(reg)
	ckg, ok := reg.Get("code_knowledge_graph")
	if !ok {
		return nil, fmt.Errorf("code_knowledge_graph node not available")
	}

	result, err := ckg.Execute(context.Background(), query, map[string]string{
		"operation": "query",
		"top_k":     fmt.Sprintf("%d", topK),
	})
	if err != nil {
		return nil, fmt.Errorf("code graph query failed: %w", err)
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: result}},
	}, nil
}

func (s *Server) toolCodeGraphStats() (*toolCallResult, error) {
	reg := nodes.GetGlobalRegistry()
	nodes.RegisterBuiltins(reg)
	ckg, ok := reg.Get("code_knowledge_graph")
	if !ok {
		return nil, fmt.Errorf("code_knowledge_graph node not available")
	}

	result, err := ckg.Execute(context.Background(), "", map[string]string{
		"operation": "stats",
	})
	if err != nil {
		return nil, fmt.Errorf("code graph stats failed: %w", err)
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: result}},
	}, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
