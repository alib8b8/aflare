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
	"strings"

	"github.com/alib8b8/aflare/internal/compress"
)

type CompressNode struct{}

func init() {
	Register(&CompressNode{})
}

func (n *CompressNode) Name() string {
	return "compress"
}

func (n *CompressNode) Description() string {
	return "Context compression layer: 6 algorithms reduce tokens by 60-95% (headroom-inspired)"
}

func (n *CompressNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "compress",
		Description: "Intelligent context compression with 6 algorithms: extractive, keyword, cluster, sliding_window, hybrid (headroom-inspired, 60-95% token reduction)",
		Input:       "string - text to compress",
		Output:      "string - compressed text with metadata",
		Params: []ParamSchema{
			{Name: "algorithm", Type: "string", Description: "extract|keyword|cluster|sliding_window|hybrid (default: hybrid)", Required: false, Default: "hybrid"},
			{Name: "ratio", Type: "string", Description: "Target compression ratio 0.01-1.0, lower=more aggressive (default: 0.2)", Required: false, Default: "0.2"},
			{Name: "max_chars", Type: "string", Description: "Maximum output characters (default: 4000)", Required: false, Default: "4000"},
			{Name: "preserve_headers", Type: "string", Description: "Preserve section headers (default: true)", Required: false, Default: "true"},
			{Name: "preserve_numbers", Type: "string", Description: "Preserve sentences with numbers/stats (default: true)", Required: false, Default: "true"},
			{Name: "output", Type: "string", Description: "text|json|stats (default: text)", Required: false, Default: "text"},
			{Name: "keywords", Type: "string", Description: "Also extract top N keywords (default: 0)", Required: false, Default: "0"},
		},
	}
}

func (n *CompressNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", nil
	}

	algoStr := getParam(params, "algorithm", "hybrid")
	ratioStr := getParam(params, "ratio", "0.2")
	maxCharsStr := getParam(params, "max_chars", "4000")
	preserveHeaders := getParam(params, "preserve_headers", "true")
	preserveNumbers := getParam(params, "preserve_numbers", "true")
	outputFmt := getParam(params, "output", "text")
	keywordsN := getParam(params, "keywords", "0")

	algoMap := map[string]compress.Algorithm{
		"extract":        compress.AlgoExtract,
		"abstract":       compress.AlgoAbstract,
		"keyword":        compress.AlgoKeyword,
		"cluster":        compress.AlgoCluster,
		"sliding_window": compress.AlgoSlidingWindow,
		"hybrid":         compress.AlgoHybrid,
	}

	cfg := compress.DefaultConfig()
	if a, ok := algoMap[algoStr]; ok {
		cfg.Algorithm = a
	}

	var ratio float64
	if _, err := fmt.Sscanf(ratioStr, "%f", &ratio); err == nil && ratio > 0 && ratio <= 1 {
		cfg.TargetRatio = ratio
	}

	var maxChars int
	if _, err := fmt.Sscanf(maxCharsStr, "%d", &maxChars); err == nil && maxChars > 0 {
		cfg.MaxOutputChars = maxChars
	}

	cfg.PreserveHeaders = strings.ToLower(preserveHeaders) == "true" || preserveHeaders == "1"
	cfg.PreserveNumbers = strings.ToLower(preserveNumbers) == "true" || preserveNumbers == "1"

	result := compress.Compress(input, cfg)

	var kwCount int
	if _, err := fmt.Sscanf(keywordsN, "%d", &kwCount); err != nil {
		// keep default value on parse failure
	}
	if kwCount > 0 && len(result.Keywords) == 0 {
		result.Keywords = compress.ExtractKeywords(input, kwCount)
	}

	switch outputFmt {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal: %w", err)
		}
		return string(data), nil

	case "stats":
		return fmt.Sprintf(
			"Compression: %d → %d chars (%.1f%%, ratio %.2f)\nAlgorithm: %s\nKeywords: %s",
			result.OriginalChars, result.CompressedChars,
			(1-result.Ratio)*100, result.Ratio,
			result.Algorithm,
			strings.Join(result.Keywords, ", "),
		), nil

	default:
		if len(result.Keywords) > 0 {
			return fmt.Sprintf("[Keywords: %s]\n\n%s",
				strings.Join(result.Keywords, ", "),
				result.Text,
			), nil
		}
		return result.Text, nil
	}
}
