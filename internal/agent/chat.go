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

package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/nodes/core"
	"github.com/alib8b8/aflare/internal/workflow"
)

// Config holds the configuration for a ChatSession.
type Config struct {
	Provider       string        // LLM provider name (e.g., "ollama", "openai")
	Model          string        // LLM model name (e.g., "llama3", "gpt-4")
	APIKey         string        // API key for the LLM provider
	Endpoint       string        // Custom API endpoint (empty for default)
	TemplatesDir   string        // Path to templates directory
	SystemPrompt   string        // Custom system prompt (empty for default)
	MaxIterations  int           // Max agent iterations per turn (default: 10)
	EnableThinking bool          // Show agent thinking chain
	ShowThinking   bool          // Display thinking chain to user
	MaxTokens      int           // Max context tokens (default: 8000)
	SessionID      string        // Session ID for memory persistence
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Provider:       "ollama",
		Model:          "llama3",
		MaxIterations:  10,
		EnableThinking: false,
		ShowThinking:   false,
		MaxTokens:      DefaultMaxContextTokens,
		SessionID:      fmt.Sprintf("chat_%d", time.Now().Unix()),
	}
}

// ChatSession manages an interactive chat session with the aflare agent.
type ChatSession struct {
	config  Config
	ctx     *ContextManager
	reg     *core.Registry
	tools   []core.AgentTool
	running bool
}

// NewChatSession creates a new chat session with the given configuration.
func NewChatSession(cfg Config) *ChatSession {
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 10
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = DefaultMaxContextTokens
	}
	if cfg.SessionID == "" {
		cfg.SessionID = fmt.Sprintf("chat_%d", time.Now().Unix())
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = SystemPrompt
	}
	if cfg.TemplatesDir == "" {
		// Default to the templates directory relative to the working directory
		cfg.TemplatesDir = "templates"
	}

	reg := core.GetGlobalRegistry()

	cm := NewContextManager(cfg.SessionID)
	cm.maxTokens = cfg.MaxTokens
	cm.SetSystemPrompt(cfg.SystemPrompt)

	return &ChatSession{
		config: cfg,
		ctx:    cm,
		reg:    reg,
		tools:  buildTools(cfg.TemplatesDir, reg),
	}
}

// buildTools creates the agent tool list for the chat session.
func buildTools(templatesDir string, reg *core.Registry) []core.AgentTool {
	return []core.AgentTool{
		{
			Name:        "template_list",
			Description: "Search available workflow templates by keyword or category. Returns matching template names and descriptions.",
			NodeName:    "template_list",
		},
		{
			Name:        "template_info",
			Description: "Get detailed information about a specific template, including its description and required parameters.",
			NodeName:    "template_info",
		},
		{
			Name:        "run_workflow",
			Description: "Run a workflow template with the given parameters. Input should be a JSON object with 'template' (template name) and 'params' (key-value parameters).",
			NodeName:    "run_workflow",
		},
		{
			Name:        "create_workflow",
			Description: "Compose a new workflow from available nodes and execute it. Input should be a YAML workflow definition.",
			NodeName:    "create_workflow",
		},
		{
			Name:        "memory_store",
			Description: "Store important information for later recall. Input: the information to remember.",
			NodeName:    "memory",
		},
		{
			Name:        "memory_retrieve",
			Description: "Recall previously stored information by key. Input: the memory key to retrieve.",
			NodeName:    "memory",
		},
		{
			Name:        "memory_search",
			Description: "Search memory for relevant context. Input: search query.",
			NodeName:    "memory",
		},
		{
			Name:        "context_compress",
			Description: "Compress the conversation history to free up context space. No input needed.",
			NodeName:    "compress",
		},
	}
}

// Run starts the interactive chat loop, reading from stdin and writing to stdout.
func (s *ChatSession) Run() {
	s.running = true
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("aflare chat — local-first automation agent")
	fmt.Println("Type /help for commands, /quit to exit")
	fmt.Println()

	for s.running {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Handle slash commands
		if strings.HasPrefix(line, "/") {
			s.handleCommand(line)
			continue
		}

		// Process user input
		response, err := s.processInput(line)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		fmt.Println(response)
		fmt.Println()
	}
}

// handleCommand processes slash commands like /quit, /help, /clear, /templates.
func (s *ChatSession) handleCommand(cmd string) {
	parts := strings.SplitN(cmd, " ", 2)
	switch parts[0] {
	case "/quit", "/exit", "/q":
		s.running = false
		fmt.Println("Goodbye!")

	case "/help", "/h":
		fmt.Println("Available commands:")
		fmt.Println("  /help, /h          Show this help")
		fmt.Println("  /quit, /exit, /q   Exit chat")
		fmt.Println("  /clear             Clear conversation history")
		fmt.Println("  /templates [kw]    List available templates (optional keyword filter)")
		fmt.Println("  /nodes             List available workflow nodes")
		fmt.Println("  /memory            Show memory stats for this session")

	case "/clear":
		s.ctx.Reset()
		fmt.Println("Conversation history cleared.")

	case "/templates":
		keyword := ""
		if len(parts) > 1 {
			keyword = parts[1]
		}
		list := s.listTemplates(keyword)
		if len(list) == 0 {
			fmt.Println("No templates found.")
		} else {
			fmt.Printf("Found %d templates:\n", len(list))
			for _, t := range list {
				fmt.Printf("  %s — %s\n", t.Name, t.Description)
			}
		}

	case "/nodes":
		nodeList := s.reg.ListNodes()
		fmt.Printf("Available nodes (%d):\n", len(nodeList))
		for _, info := range nodeList {
			fmt.Printf("  %-25s %s\n", info.Name, info.Description)
		}

	case "/memory":
		memNode := &nodes.MemoryNode{}
		output, err := memNode.Execute(
			context.Background(),
			"",
			map[string]string{
				"operation":  "session_stats",
				"session_id": s.config.SessionID,
			},
		)
		if err != nil {
			fmt.Printf("Memory error: %v\n", err)
		} else {
			fmt.Println(output)
		}

	default:
		fmt.Printf("Unknown command: %s. Type /help for available commands.\n", parts[0])
	}
}

// SendMessage processes a single user message and returns the agent's response.
// This is the public API for programmatic chat integration (e.g., HTTP endpoints).
func (s *ChatSession) SendMessage(input string) (string, error) {
	return s.processInput(input)
}

// ResetSession clears the conversation history while keeping the system prompt.
func (s *ChatSession) ResetSession() {
	s.ctx.Reset()
}

// processInput handles a single user message and returns the agent's response.
func (s *ChatSession) processInput(input string) (string, error) {
	// Add user message to context
	s.ctx.Add("user", input)

	// Build the agent
	agent := nodes.NewReActAgent(
		s.config.Provider,
		s.config.Model,
		s.config.APIKey,
		s.config.Endpoint,
		s.config.SystemPrompt,
		s.config.MaxIterations,
		s.tools,
		s.reg,
		s.config.EnableThinking,
		s.config.ShowThinking,
	)

	// Run the agent with full conversation context
	ctx := context.Background()
	response, err := agent.Run(ctx, s.ctx.BuildPrompt())
	if err != nil {
		return "", fmt.Errorf("agent execution failed: %w", err)
	}

	// Add assistant response to context
	s.ctx.Add("assistant", response)

	return response, nil
}

// TemplateInfo holds basic template metadata extracted from the YAML header.
type TemplateInfo struct {
	Name        string
	Description string
	Category    string
	Path        string
}

// listTemplates scans the templates directory and returns matching templates.
func (s *ChatSession) listTemplates(keyword string) []TemplateInfo {
	var results []TemplateInfo

	keyword = strings.ToLower(keyword)

	_ = filepath.Walk(s.config.TemplatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Name() != "workflow.yaml" {
			return nil
		}

		// Parse the workflow to extract metadata
		wf, parseErr := workflow.ParseWorkflow(path)
		if parseErr != nil {
			// Try to extract just name/description from raw YAML
			return nil
		}

		// Extract category from path
		relPath, _ := filepath.Rel(s.config.TemplatesDir, path)
		category := filepath.Dir(relPath)

		ti := TemplateInfo{
			Name:        wf.Name,
			Description: wf.Description,
			Category:    category,
			Path:        relPath,
		}

		if keyword == "" ||
			strings.Contains(strings.ToLower(ti.Name), keyword) ||
			strings.Contains(strings.ToLower(ti.Description), keyword) ||
			strings.Contains(strings.ToLower(ti.Category), keyword) {
			results = append(results, ti)
		}

		return nil
	})

	return results
}

// ExecuteTool is called by the ReActAgent to execute chat-specific tools
// (template_list, template_info, run_workflow, create_workflow).
// It implements the tool execution interface expected by the agent.
func (s *ChatSession) ExecuteTool(ctx context.Context, toolName, input string) (string, error) {
	switch toolName {
	case "template_list":
		return s.executeTemplateList(input)
	case "template_info":
		return s.executeTemplateInfo(input)
	case "run_workflow":
		return s.executeRunWorkflow(input)
	case "create_workflow":
		return s.executeCreateWorkflow(input)
	default:
		// Delegate to the registry for standard node tools
		node, ok := s.reg.Get(toolName)
		if !ok {
			return "", fmt.Errorf("unknown tool: %s", toolName)
		}

		params := s.buildToolParams(toolName, input)
		return node.Execute(ctx, input, params)
	}
}

func (s *ChatSession) buildToolParams(toolName, input string) map[string]string {
	params := map[string]string{
		"session_id": s.config.SessionID,
	}

	switch toolName {
	case "memory":
		params["operation"] = "store"
		params["level"] = "medium"
		params["type"] = "context"
	case "compress":
		params["algorithm"] = "hybrid"
		params["ratio"] = "0.2"
		params["max_chars"] = "4000"
	}

	return params
}

func (s *ChatSession) executeTemplateList(input string) (string, error) {
	keyword := strings.TrimSpace(input)
	list := s.listTemplates(keyword)
	if len(list) == 0 {
		return "No templates found matching: " + keyword, nil
	}

	data, _ := json.MarshalIndent(list, "", "  ")
	return string(data), nil
}

func (s *ChatSession) executeTemplateInfo(input string) (string, error) {
	name := strings.TrimSpace(input)
	if name == "" {
		return "", fmt.Errorf("template name is required")
	}

	templates := s.listTemplates(name)
	for _, t := range templates {
		if strings.EqualFold(t.Name, name) || strings.Contains(strings.ToLower(t.Path), strings.ToLower(name)) {
			// Parse full workflow to get input_schema
			fullPath := filepath.Join(s.config.TemplatesDir, t.Path)
			wf, err := workflow.ParseWorkflow(fullPath)
			if err != nil {
				return "", fmt.Errorf("failed to parse template: %w", err)
			}

			info := map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"category":    t.Category,
				"path":        t.Path,
				"params":      wf.InputSchema,
			}
			data, _ := json.MarshalIndent(info, "", "  ")
			return string(data), nil
		}
	}

	return "", fmt.Errorf("template not found: %s", name)
}

func (s *ChatSession) executeRunWorkflow(input string) (string, error) {
	// Parse JSON input: {"template": "...", "params": {...}}
	var req struct {
		Template string            `json:"template"`
		Params   map[string]string `json:"params"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", fmt.Errorf("invalid input format: expected JSON with 'template' and 'params' fields: %w", err)
	}

	if req.Template == "" {
		return "", fmt.Errorf("template name is required")
	}

	// Find the template file
	templates := s.listTemplates(req.Template)
	if len(templates) == 0 {
		return "", fmt.Errorf("template not found: %s", req.Template)
	}

	fullPath := filepath.Join(s.config.TemplatesDir, templates[0].Path)
	wf, err := workflow.ParseWorkflow(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to parse workflow: %w", err)
	}

	// Apply user-provided parameters as vars
	for k, v := range req.Params {
		if wf.Vars == nil {
			wf.Vars = make(map[string]string)
		}
		wf.Vars[k] = v
	}

	exec := workflow.NewExecutor()
	result, _, err := exec.Execute(context.Background(), wf, s.reg)
	if err != nil {
		return "", fmt.Errorf("workflow execution failed: %w", err)
	}

	return result, nil
}

func (s *ChatSession) executeCreateWorkflow(input string) (string, error) {
	// Parse the workflow definition from YAML
	wf, err := workflow.ParseWorkflowFromContent(input)
	if err != nil {
		return "", fmt.Errorf("failed to parse workflow: %w", err)
	}

	exec := workflow.NewExecutor()
	result, _, err := exec.Execute(context.Background(), wf, s.reg)
	if err != nil {
		return "", fmt.Errorf("workflow execution failed: %w", err)
	}

	return result, nil
}