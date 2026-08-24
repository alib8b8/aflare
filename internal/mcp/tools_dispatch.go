// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌​​‌‌​‌​‌‌‌‌​‌​​‌‌‌‌​‌​‌​‌‌​​‌‌‌‌‌​​​‌‌​​​​​‌​​​​​​​​​​​​​​​​​​‌‌‌​​​‌‌​​‌​‌‌​⁠
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
	"context"
	"fmt"
	"runtime/debug"

	"github.com/alib8b8/aflare/internal/logger"
)

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
		defer func() {
			if r := recover(); r != nil {
				logger.Error("tool call panicked",
					"tool", params.Name,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				done <- result{err: fmt.Errorf("tool %s panicked: %v", params.Name, r)}
			}
		}()
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
		// Vertical domain tools
		case "context_compress":
			r.res, r.err = s.toolContextCompress(params.Arguments)
		case "search_aggregated":
			r.res, r.err = s.toolSearchAggregated(params.Arguments)
		case "geospatial_query":
			r.res, r.err = s.toolGeospatialQuery(params.Arguments)
		case "preference_get":
			r.res, r.err = s.toolPreferenceGet(params.Arguments)
		case "preference_set":
			r.res, r.err = s.toolPreferenceSet(params.Arguments)
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
