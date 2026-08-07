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
)

type ClarificationQuestion struct {
	Question      string   `json:"question"`
	Category      string   `json:"category"`
	Options       []string `json:"options,omitempty"`
	ClarifiedGoal string   `json:"clarified_goal,omitempty"`
}

type ClarificationResult struct {
	NeedsClarification bool                    `json:"needs_clarification"`
	Confidence         float64                 `json:"confidence"`
	Questions          []ClarificationQuestion `json:"questions,omitempty"`
	ClarifiedGoal      string                  `json:"clarified_goal,omitempty"`
	Ambiguities        []string                `json:"ambiguities,omitempty"`
}

type ClarifyNode struct{}

func init() {
	Register(&ClarifyNode{})
}

func (n *ClarifyNode) Name() string {
	return "clarify"
}

func (n *ClarifyNode) Description() string {
	return "Analyze task for ambiguity and ask clarifying questions before execution"
}

func (n *ClarifyNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "clarify",
		Description: "Pre-execution ambiguity checker: identifies unclear requirements and generates clarifying questions (ACQUIRE framework)",
		Input:       "string - the task or goal to analyze for ambiguity",
		Output:      "string - JSON with clarification result: needs_clarification, questions, confidence, clarified_goal",
		Params: []ParamSchema{
			{Name: "provider", Type: "string", Description: "LLM provider (default: ollama)", Required: false, Default: "ollama"},
			{Name: "model", Type: "string", Description: "Model name (default: llama3)", Required: false, Default: "llama3"},
			{Name: "api_key", Type: "string", Description: "API key", Required: false},
			{Name: "endpoint", Type: "string", Description: "API endpoint URL", Required: false},
			{Name: "threshold", Type: "string", Description: "Confidence threshold 0-100, below this trigger clarification (default: 70)", Required: false, Default: "70"},
			{Name: "max_questions", Type: "string", Description: "Max clarification questions to ask (default: 5)", Required: false, Default: "5"},
			{Name: "context", Type: "string", Description: "Additional context about the task", Required: false},
			{Name: "user_answers", Type: "string", Description: "JSON object of user's answers to previous questions (question -> answer)", Required: false},
		},
	}
}

func (n *ClarifyNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	provider := getParam(params, "provider", "ollama")
	model := getParam(params, "model", "llama3")
	apiKey := getParam(params, "api_key", "")
	endpoint := getParam(params, "endpoint", defaultEndpointFor(provider))
	thresholdStr := getParam(params, "threshold", "70")
	maxQuestions := getParam(params, "max_questions", "5")
	contextInfo := getParam(params, "context", "")
	userAnswers := getParam(params, "user_answers", "")

	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("task input cannot be empty")
	}

	answersPrompt := ""
	if userAnswers != "" {
		answersPrompt = fmt.Sprintf("\n\nUser has already provided answers to these clarification questions:\n%s\n\nUse these answers to refine the goal and check if more clarification is needed.", userAnswers)
	}

	contextPrompt := ""
	if contextInfo != "" {
		contextPrompt = fmt.Sprintf("\n\nAdditional context:\n%s", contextInfo)
	}

	systemPrompt := fmt.Sprintf(`You are an expert requirement analyst. Before executing any task, you must determine if the task description is clear enough.

Analyze the given task for:
1. Ambiguous requirements (unclear what exactly needs to be done)
2. Missing constraints (scope, priorities, success criteria not specified)
3. Assumptions that would lead to wrong output
4. Multiple possible interpretations

Clarification categories:
- scope: What exactly is in or out of scope?
- constraints: Are there time, resource, or quality constraints?
- priority: What is most important if trade-offs are needed?
- success: How will success be measured?
- context: What background info is needed?

Output format (MUST be valid JSON only):
{
  "needs_clarification": true/false,
  "confidence": 0-100 (how confident you are the task is clear, 100=totally clear),
  "ambiguities": ["list of specific ambiguities found"],
  "questions": [
    {
      "question": "clear, specific question to ask the user",
      "category": "scope|constraints|priority|success|context",
      "options": ["suggested answer options if applicable"]
    }
  ],
  "clarified_goal": "if no clarification needed, provide the precise restated goal; if user_answers provided, integrate them into a refined goal"
}

Rules:
- Set needs_clarification=true if confidence < %s
- Maximum %s questions
- Questions should be specific and actionable, not vague
- If the task is already clear (confidence >= %s), set needs_clarification=false
- Always identify specific ambiguities, don't just say "unclear"%s%s`, thresholdStr, maxQuestions, thresholdStr, answersPrompt, contextPrompt)

	result, err := runAgentLLM(ctx, provider, model, apiKey, endpoint, systemPrompt, input)
	if err != nil {
		return "", fmt.Errorf("clarify agent failed: %w", err)
	}

	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, "```json")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	if !strings.HasPrefix(result, "{") {
		if idx := strings.Index(result, "{"); idx != -1 {
			if endIdx := strings.LastIndex(result, "}"); endIdx > idx {
				result = result[idx : endIdx+1]
			}
		}
	}

	var clarifyResult ClarificationResult
	if err := json.Unmarshal([]byte(result), &clarifyResult); err != nil {
		return "", fmt.Errorf("failed to parse clarification result: %w", err)
	}

	if clarifyResult.Confidence == 0 && !clarifyResult.NeedsClarification {
		clarifyResult.Confidence = 85
	}

	output, err := json.MarshalIndent(clarifyResult, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal output: %w", err)
	}

	return string(output), nil
}
