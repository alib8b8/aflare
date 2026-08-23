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

package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/history"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/templates"
)

// ------------------------------------------------------------------
// Node / history / template tool implementations
// ------------------------------------------------------------------

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
	switch {
	case keyword != "":
		list = tm.Search(keyword)
	case category != "":
		list = tm.ListByCategory(category)
	default:
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

// truncate shortens s to maxLen characters, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
