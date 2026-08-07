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
	"strings"

	"github.com/alib8b8/aflare/internal/compress"
	"github.com/alib8b8/aflare/internal/nodes"
)

// ------------------------------------------------------------------
// Vertical domain tool implementations
// (geeflow / headroom / last30days inspired)
// ------------------------------------------------------------------

func (s *Server) toolContextCompress(args map[string]interface{}) (*toolCallResult, error) {
	text, err := requireString(args, "text")
	if err != nil {
		return nil, err
	}

	algo := optionalString(args, "algorithm")
	if algo == "" {
		algo = "hybrid"
	}
	ratio := 0.2
	if r, ok := args["ratio"].(float64); ok {
		ratio = r
	}
	maxChars := 4000
	if m, ok := args["max_chars"].(float64); ok {
		maxChars = int(m)
	}

	cfg := compress.DefaultConfig()
	algoMap := map[string]compress.Algorithm{
		"extract":        compress.AlgoExtract,
		"keyword":        compress.AlgoKeyword,
		"cluster":        compress.AlgoCluster,
		"sliding_window": compress.AlgoSlidingWindow,
		"hybrid":         compress.AlgoHybrid,
	}
	if a, ok := algoMap[algo]; ok {
		cfg.Algorithm = a
	}
	cfg.TargetRatio = ratio
	cfg.MaxOutputChars = maxChars

	result := compress.Compress(text, cfg)

	output := fmt.Sprintf(
		"Compressed: %d → %d chars (saved %.1f%%)\nAlgorithm: %s\n\n%s",
		result.OriginalChars, result.CompressedChars,
		(1-result.Ratio)*100, result.Algorithm,
		result.Text,
	)
	if len(result.Keywords) > 0 {
		output += fmt.Sprintf("\n\nKeywords: %s", strings.Join(result.Keywords, ", "))
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: output}},
	}, nil
}

func (s *Server) toolSearchAggregated(args map[string]interface{}) (*toolCallResult, error) {
	query, err := requireString(args, "query")
	if err != nil {
		return nil, err
	}
	sources := optionalString(args, "sources")
	if sources == "" {
		sources = "hn,github,reddit"
	}
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	timeRange := optionalString(args, "time_range")
	if timeRange == "" {
		timeRange = "week"
	}
	sortBy := optionalString(args, "sort_by")
	if sortBy == "" {
		sortBy = "signal"
	}

	reg := nodes.GetGlobalRegistry()
	nodes.RegisterBuiltins(reg)
	searchNode, ok := reg.Get("search_aggregate")
	if !ok {
		return nil, fmt.Errorf("search_aggregate node not available")
	}

	params := map[string]string{
		"sources":    sources,
		"limit":      fmt.Sprintf("%d", limit),
		"time_range": timeRange,
		"sort_by":    sortBy,
	}
	result, err := searchNode.Execute(context.Background(), query, params)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: result}},
	}, nil
}

func (s *Server) toolGeospatialQuery(args map[string]interface{}) (*toolCallResult, error) {
	query, err := requireString(args, "query")
	if err != nil {
		return nil, err
	}
	dataset := optionalString(args, "dataset")
	if dataset == "" {
		dataset = "sentinel2"
	}
	region := optionalString(args, "region")
	timeStart := optionalString(args, "time_start")
	timeEnd := optionalString(args, "time_end")
	outputFormat := optionalString(args, "output_format")
	if outputFormat == "" {
		outputFormat = "summary"
	}

	result := fmt.Sprintf("🌍 Geospatial Analysis Query\n\n")
	result += fmt.Sprintf("Query: %s\n", query)
	result += fmt.Sprintf("Dataset: %s\n", dataset)
	if region != "" {
		result += fmt.Sprintf("Region: %s\n", region)
	}
	if timeStart != "" || timeEnd != "" {
		result += fmt.Sprintf("Time range: %s → %s\n", timeStart, timeEnd)
	}
	result += fmt.Sprintf("Output format: %s\n\n", outputFormat)

	result += fmt.Sprintf("Note: This is a geospatial query template (geeflow-inspired).\n")
	result += fmt.Sprintf("For actual GIS processing, connect Google Earth Engine or a local GIS backend via:\n")
	result += fmt.Sprintf("  1. MCP server wrapping GEE Python API\n")
	result += fmt.Sprintf("  2. Direct HTTP calls to EO data providers (Sentinel Hub, NASA Earthdata)\n")
	result += fmt.Sprintf("  3. Local GDAL/Rasterio processing via code_interpreter\n\n")

	result += fmt.Sprintf("Query would be translated to:\n")
	result += fmt.Sprintf("  - Image collection: %s\n", dataset)
	if region != "" {
		result += fmt.Sprintf("  - Filter bounds: %s\n", region)
	}
	if timeStart != "" {
		result += fmt.Sprintf("  - Filter date: %s to %s\n", timeStart, timeEnd)
	}
	result += fmt.Sprintf("  - Natural language: %s\n", query)

	return &toolCallResult{
		Content: []content{{Type: "text", Text: result}},
	}, nil
}
