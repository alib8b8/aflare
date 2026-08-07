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
	"context"
	"fmt"
	"os"

	"github.com/alib8b8/aflare/internal/nodes"
)

// ------------------------------------------------------------------
// Code knowledge graph tool implementations
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
