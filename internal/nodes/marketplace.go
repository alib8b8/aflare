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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alib8b8/llm-box/internal/nodes/core"
)

// NodeCategory and its constants (CategoryLLM, CategoryAgent, ...) plus the
// Registry.Search / Registry.NodesByCategory methods have been moved to
// internal/nodes/core/registry.go so they can be defined on the local
// *core.Registry type. They are re-exported here via type aliases so that
// existing callers (marketplace node, CLI) keep compiling unchanged.
type NodeCategory = core.NodeCategory

const (
	CategoryLLM       = core.CategoryLLM
	CategoryAgent     = core.CategoryAgent
	CategoryIO        = core.CategoryIO
	CategoryTransform = core.CategoryTransform
	CategoryFlow      = core.CategoryFlow
	CategoryData      = core.CategoryData
	CategorySecurity  = core.CategorySecurity
	CategoryUtility   = core.CategoryUtility
)

type PluginInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Path        string `json:"path"`
	NodeCount   int    `json:"node_count"`
}

type NodeMarketplace struct {
	pluginDir string
}

func NewNodeMarketplace(pluginDir string) *NodeMarketplace {
	return &NodeMarketplace{pluginDir: pluginDir}
}

func (m *NodeMarketplace) ListAvailablePlugins() ([]PluginInfo, error) {
	if _, err := os.Stat(m.pluginDir); os.IsNotExist(err) {
		return nil, nil
	}

	var plugins []PluginInfo
	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			pluginPath := filepath.Join(m.pluginDir, entry.Name())
			info := m.scanPlugin(pluginPath)
			if info != nil {
				plugins = append(plugins, *info)
			}
		}
	}

	return plugins, nil
}

func (m *NodeMarketplace) scanPlugin(path string) *PluginInfo {
	info := &PluginInfo{
		Name: filepath.Base(path),
		Path: path,
	}

	manifestPath := filepath.Join(path, "plugin.json")
	if data, err := os.ReadFile(manifestPath); err == nil {
		var manifest struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"description"`
			Author      string `json:"author"`
		}
		if err := json.Unmarshal(data, &manifest); err == nil {
			if manifest.Name != "" {
				info.Name = manifest.Name
			}
			info.Version = manifest.Version
			info.Description = manifest.Description
			info.Author = manifest.Author
		}
	}

	if entries, err := os.ReadDir(path); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".so") || strings.HasSuffix(e.Name(), ".go") {
				info.NodeCount++
			}
		}
	}

	return info
}

type MarketplaceNode struct{}

func init() {
	Register(&MarketplaceNode{})
}

func (n *MarketplaceNode) Name() string {
	return "node_marketplace"
}

func (n *MarketplaceNode) Description() string {
	return "Node marketplace: list, search, and manage workflow nodes"
}

func (n *MarketplaceNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "node_marketplace",
		Description: "Node marketplace - list, search, and categorize available workflow nodes",
		Input:       "string - search query (for search action)",
		Output:      "string - node listing or search results",
		Params: []ParamSchema{
			{Name: "action", Type: "string", Description: "Action: list, search, categories, count, details (default: list)", Required: false, Default: "list"},
			{Name: "category", Type: "string", Description: "Filter by category: llm, agent, io, transform, flow, data, utility", Required: false},
			{Name: "format", Type: "string", Description: "Output format: text, markdown, json (default: markdown)", Required: false, Default: "markdown"},
			{Name: "node_name", Type: "string", Description: "Node name for details action", Required: false},
		},
	}
}

func (n *MarketplaceNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	action := getParam(params, "action", "list")
	category := params["category"]
	format := getParam(params, "format", "markdown")
	nodeName := params["node_name"]

	reg := GetGlobalRegistry()

	switch action {
	case "list":
		var nodes []NodeInfo
		if category != "" {
			nodes = reg.NodesByCategory(NodeCategory(category))
		} else {
			nodes = reg.ListNodes()
		}
		return formatNodeInfoList(nodes, format, category), nil

	case "search":
		query := input
		if query == "" {
			query = params["query"]
		}
		if query == "" {
			return "", fmt.Errorf("search query is required (use input or query param)")
		}
		results := reg.Search(query)
		return formatNodeInfoList(results, format, fmt.Sprintf("search: %s", query)), nil

	case "categories":
		return formatCategories(reg, format), nil

	case "count":
		return fmt.Sprintf("Total nodes: %d", len(reg.ListNodes())), nil

	case "details":
		if nodeName == "" {
			nodeName = input
		}
		if nodeName == "" {
			return "", fmt.Errorf("node_name is required for details action")
		}
		node, ok := reg.Get(nodeName)
		if !ok {
			return "", fmt.Errorf("node not found: %s", nodeName)
		}
		return formatNodeDetails(node, format), nil

	default:
		return "", fmt.Errorf("unknown action: %s (supported: list, search, categories, count, details)", action)
	}
}

func formatNodeInfoList(nodes []NodeInfo, format, filter string) string {
	switch format {
	case "json":
		result := map[string]interface{}{
			"filter": filter,
			"count":  len(nodes),
			"nodes":  nodes,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return string(data)

	default:
		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("## Available Nodes (%d)", len(nodes)))
		if filter != "" {
			builder.WriteString(fmt.Sprintf(" - %s", filter))
		}
		builder.WriteString("\n\n")

		for _, node := range nodes {
			builder.WriteString(fmt.Sprintf("### `%s`\n", node.Name))
			builder.WriteString(fmt.Sprintf("%s\n\n", node.Description))
		}

		return builder.String()
	}
}

func formatNodeDetails(node Node, format string) string {
	schema := node.Schema()

	switch format {
	case "json":
		data, _ := json.MarshalIndent(schema, "", "  ")
		return string(data)

	default:
		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("## Node: `%s`\n\n", node.Name()))
		builder.WriteString(fmt.Sprintf("**Description:** %s\n\n", node.Description()))
		builder.WriteString(fmt.Sprintf("**Input:** %s\n\n", schema.Input))
		builder.WriteString(fmt.Sprintf("**Output:** %s\n\n", schema.Output))

		if len(schema.Params) > 0 {
			builder.WriteString("### Parameters\n\n")
			builder.WriteString("| Name | Type | Required | Default | Description |\n")
			builder.WriteString("|------|------|----------|---------|-------------|\n")
			for _, p := range schema.Params {
				required := "No"
				if p.Required {
					required = "Yes"
				}
				builder.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s |\n",
					p.Name, p.Type, required, p.Default, p.Description))
			}
		}

		return builder.String()
	}
}

func formatCategories(reg *Registry, format string) string {
	categories := []struct {
		name NodeCategory
		desc string
	}{
		{CategoryLLM, "LLM 模型节点"},
		{CategoryAgent, "Agent 智能体节点"},
		{CategoryIO, "输入输出节点"},
		{CategoryTransform, "转换节点"},
		{CategoryFlow, "流程控制节点"},
		{CategoryData, "数据处理节点"},
		{CategoryUtility, "工具节点"},
	}

	switch format {
	case "json":
		result := make(map[string]int)
		total := 0
		for _, cat := range categories {
			nodes := reg.NodesByCategory(cat.name)
			result[string(cat.name)] = len(nodes)
			total += len(nodes)
		}
		result["total_registered"] = len(reg.ListNodes())
		data, _ := json.MarshalIndent(result, "", "  ")
		return string(data)

	default:
		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("## Node Categories (Total registered: %d nodes)\n\n", len(reg.ListNodes())))

		for _, cat := range categories {
			nodes := reg.NodesByCategory(cat.name)
			builder.WriteString(fmt.Sprintf("### %s (%d nodes)\n\n", cat.desc, len(nodes)))
			for _, node := range nodes {
				builder.WriteString(fmt.Sprintf("- `%s` - %s\n", node.Name, node.Description))
			}
			builder.WriteString("\n")
		}

		return builder.String()
	}
}
