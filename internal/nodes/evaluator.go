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
	"fmt"
	"strings"
)

type EvaluatorNode struct{}

func init() {
	Register(&EvaluatorNode{})
}

func (n *EvaluatorNode) Name() string {
	return "evaluator"
}

func (n *EvaluatorNode) Description() string {
	return "Evaluate and score output against criteria"
}

func (n *EvaluatorNode) Schema() NodeSchema {
	params := baseAgentParams()
	params = append(params,
		ParamSchema{Name: "rubric", Type: "string", Description: "Evaluation rubric: quality, correctness, completeness, clarity, custom (default: quality)", Required: false, Default: "quality"},
		ParamSchema{Name: "criteria", Type: "string", Description: "Custom criteria for evaluation (comma-separated dimensions)", Required: false},
		ParamSchema{Name: "scale", Type: "string", Description: "Score scale: 1-5, 1-10, percentage (default: 1-10)", Required: false, Default: "1-10"},
		ParamSchema{Name: "threshold", Type: "string", Description: "Pass/fail threshold score (e.g. 7)", Required: false},
		ParamSchema{Name: "output_format", Type: "string", Description: "Output format: json, markdown, score_only (default: json)", Required: false, Default: "json"},
	)
	return NodeSchema{
		Name:        "evaluator",
		Description: "Evaluator agent that scores output against criteria with structured rubrics",
		Input:       "string - the content to evaluate",
		Output:      "string - evaluation scores and justification",
		Params:      params,
	}
}

func (n *EvaluatorNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	provider := getParam(params, "provider", "ollama")
	model := getParam(params, "model", "llama3")
	apiKey := getParam(params, "api_key", "")
	endpoint := getParam(params, "endpoint", defaultEndpointFor(provider))
	rubric := getParam(params, "rubric", "quality")
	criteria := getParam(params, "criteria", "")
	scale := getParam(params, "scale", "1-10")
	threshold := getParam(params, "threshold", "")
	outputFormat := getParam(params, "output_format", "json")

	rubricDimensions := map[string][]string{
		"quality":      {"Accuracy", "Completeness", "Clarity", "Relevance", "Overall quality"},
		"correctness":  {"Factual accuracy", "Logical consistency", "Edge cases covered", "Error handling", "Correctness"},
		"completeness": {"Requirements coverage", "Depth of analysis", "Actionable insights", "Completeness"},
		"clarity":      {"Structure", "Readability", "Conciseness", "Flow", "Clarity"},
	}

	dimensions, ok := rubricDimensions[rubric]
	if !ok {
		dimensions = rubricDimensions["quality"]
	}
	if criteria != "" {
		dims := []string{}
		for _, c := range strings.Split(criteria, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				dims = append(dims, c)
			}
		}
		if len(dims) > 0 {
			dimensions = dims
		}
	}

	dimensionJSON := ""
	for i, d := range dimensions {
		if i > 0 {
			dimensionJSON += ",\n"
		}
		dimensionJSON += fmt.Sprintf("    \"%s\": {\"score\": N, \"justification\": \"brief reason\"}", d)
	}

	systemPrompt := fmt.Sprintf(`You are an objective evaluator. Score the given content on multiple dimensions.

Score scale: %s
%s

Output format (MUST be valid JSON):
{
  "overall_score": N,
  "dimensions": {
%s
  },
  "summary": "one sentence overall assessment",
  "passed": true/false
}

%s

Be fair and consistent. No extra text — just the JSON.`, scale,
		func() string {
			if threshold != "" {
				return fmt.Sprintf("Pass threshold: score >= %s", threshold)
			}
			return ""
		}(),
		dimensionJSON,
		func() string {
			if threshold != "" {
				return fmt.Sprintf("Set 'passed' to true if overall_score >= %s, otherwise false.", threshold)
			}
			return "Set 'passed' to true by default."
		}())

	result, err := runAgentLLM(ctx, provider, model, apiKey, endpoint, systemPrompt, input)
	if err != nil {
		return "", fmt.Errorf("evaluator agent failed: %w", err)
	}

	if outputFormat == "json" {
		return cleanJSONResponse(result), nil
	}

	return result, nil
}
