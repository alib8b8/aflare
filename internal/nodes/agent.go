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

const defaultMaxAgentIterations = 10

// AgentTool is now an alias for core.AgentTool (defined in
// internal/nodes/core/params.go and re-exported via agent_node.go).

type AgentThought struct {
	Thought     string `json:"thought"`
	Action      string `json:"action"`
	ActionInput string `json:"action_input"`
	FinalAnswer string `json:"final_answer,omitempty"`
	Observation string `json:"observation,omitempty"`
}

type ReActAgent struct {
	provider       string
	model          string
	apiKey         string
	endpoint       string
	systemPrompt   string
	maxIters       int
	tools          []AgentTool
	registry       *Registry
	enableThinking bool
	showThinking   bool
}

func NewReActAgent(provider, model, apiKey, endpoint, systemPrompt string, maxIters int, tools []AgentTool, reg *Registry, enableThinking, showThinking bool) *ReActAgent {
	if maxIters <= 0 {
		maxIters = defaultMaxAgentIterations
	}
	return &ReActAgent{
		provider:       provider,
		model:          model,
		apiKey:         apiKey,
		endpoint:       endpoint,
		systemPrompt:   systemPrompt,
		maxIters:       maxIters,
		tools:          tools,
		registry:       reg,
		enableThinking: enableThinking,
		showThinking:   showThinking,
	}
}

func (a *ReActAgent) Run(ctx context.Context, input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("agent input cannot be empty")
	}

	toolDescs := a.buildToolDescriptions()
	systemPrompt := a.buildSystemPrompt(toolDescs)

	conversation := []LLMMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: input},
	}

	var thoughtChain []string
	var lastAnswer string

	for i := 0; i < a.maxIters; i++ {
		response, err := a.callLLM(ctx, conversation)
		if err != nil {
			return "", fmt.Errorf("agent iteration %d failed: %w", i, err)
		}

		thought, err := parseReActResponse(response)
		if err != nil {
			conversation = append(conversation,
				LLMMessage{Role: "assistant", Content: response},
				LLMMessage{Role: "user", Content: fmt.Sprintf("Error parsing your response: %v\n\nPlease respond with valid JSON in the exact format specified.", err)},
			)
			continue
		}

		if a.enableThinking && thought.Thought != "" {
			thoughtChain = append(thoughtChain, fmt.Sprintf("[Step %d] %s", i+1, thought.Thought))
		}

		if thought.FinalAnswer != "" {
			lastAnswer = thought.FinalAnswer
			break
		}

		conversation = append(conversation, LLMMessage{Role: "assistant", Content: response})

		observation, err := a.executeTool(ctx, thought.Action, thought.ActionInput)
		if err != nil {
			observation = fmt.Sprintf("Error: %v", err)
		}

		if a.enableThinking {
			thoughtChain = append(thoughtChain, fmt.Sprintf("[Step %d Observation] %s", i+1, truncate(observation, 200)))
		}

		conversation = append(conversation, LLMMessage{
			Role:    "user",
			Content: fmt.Sprintf("Observation: %s", observation),
		})

		if i == a.maxIters-1 {
			conversation = append(conversation, LLMMessage{
				Role:    "user",
				Content: "You have reached the maximum number of iterations. Please provide your best final answer now using action: final_answer.",
			})
			finalResp, err := a.callLLM(ctx, conversation)
			if err == nil {
				finalThought, parseErr := parseReActResponse(finalResp)
				if parseErr == nil && finalThought.FinalAnswer != "" {
					lastAnswer = finalThought.FinalAnswer
				} else {
					lastAnswer = finalResp
				}
			} else {
				lastAnswer = observation
			}
		}
	}

	if lastAnswer == "" {
		return "", fmt.Errorf("agent reached max iterations (%d) without producing a final answer", a.maxIters)
	}

	if a.enableThinking && a.showThinking && len(thoughtChain) > 0 {
		fullOutput := fmt.Sprintf("--- Thinking Chain ---\n%s\n--- Final Answer ---\n%s",
			strings.Join(thoughtChain, "\n\n"), lastAnswer)
		return fullOutput, nil
	}

	return lastAnswer, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (a *ReActAgent) buildToolDescriptions() string {
	var descs []string
	for _, t := range a.tools {
		descs = append(descs, fmt.Sprintf("- %s: %s", t.Name, t.Description))
	}
	return strings.Join(descs, "\n")
}

func (a *ReActAgent) buildSystemPrompt(toolDescs string) string {
	thinkingInstruction := ""
	if a.enableThinking {
		thinkingInstruction = `
Thinking mode is ENABLED. For the "thought" field:
- Think deeply and step by step
- Break down complex problems into smaller parts
- Consider multiple approaches before deciding
- Reflect on potential mistakes or edge cases
- Be thorough and explicit about your reasoning
- The thought field is for your internal reasoning — make it detailed and comprehensive`
	}

	basePrompt := `You are a helpful AI agent that uses tools to answer questions.
Follow the ReAct (Reason + Act) pattern strictly.
%s

Available tools:
%s

Response format (MUST be valid JSON):
{
  "thought": "your reasoning about what to do next",
  "action": "tool_name to use, or 'final_answer' when done",
  "action_input": "input to pass to the tool",
  "final_answer": "your final answer to the user (only when action is final_answer)"
}

Rules:
1. Always respond with valid JSON only — no extra text, no markdown code blocks
2. Use tools to gather information before giving a final answer
3. If you have enough information, use action: "final_answer" and put the answer in final_answer
4. Be concise and focused on the task`

	if a.systemPrompt != "" {
		basePrompt = a.systemPrompt + "\n\n" + basePrompt
	}

	return fmt.Sprintf(basePrompt, thinkingInstruction, toolDescs)
}

func parseReActResponse(response string) (AgentThought, error) {
	response = strings.TrimSpace(response)
	// Strip markdown code fences: ```json ... ``` or ``` ... ```
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Some LLMs wrap JSON in markdown even when told not to; try to extract
	if !strings.HasPrefix(response, "{") {
		if idx := strings.Index(response, "{"); idx != -1 {
			if endIdx := strings.LastIndex(response, "}"); endIdx > idx {
				response = response[idx : endIdx+1]
			}
		}
	}

	var thought AgentThought
	if err := json.Unmarshal([]byte(response), &thought); err != nil {
		return AgentThought{}, fmt.Errorf("invalid JSON response: %w", err)
	}

	if thought.Action == "" && thought.FinalAnswer == "" {
		return AgentThought{}, fmt.Errorf("response must have either action or final_answer")
	}

	return thought, nil
}

func (a *ReActAgent) executeTool(ctx context.Context, toolName, toolInput string) (string, error) {
	var targetTool *AgentTool
	for i := range a.tools {
		if a.tools[i].Name == toolName {
			targetTool = &a.tools[i]
			break
		}
	}
	if targetTool == nil {
		return fmt.Sprintf("Unknown tool: %s. Available tools: %s", toolName, a.toolNames()), nil
	}

	if a.registry == nil {
		return "", fmt.Errorf("no registry available for tool execution")
	}

	node, ok := a.registry.Get(targetTool.NodeName)
	if !ok {
		return "", fmt.Errorf("tool node %q not found in registry", targetTool.NodeName)
	}

	result, err := node.Execute(ctx, toolInput, map[string]string{})
	if err != nil {
		return "", fmt.Errorf("tool %s execution failed: %w", toolName, err)
	}

	if len(result) > 4000 {
		result = result[:4000] + "\n... (truncated)"
	}

	return result, nil
}

func (a *ReActAgent) toolNames() string {
	var names []string
	for _, t := range a.tools {
		names = append(names, t.Name)
	}
	return strings.Join(names, ", ")
}

func (a *ReActAgent) callLLM(ctx context.Context, messages []LLMMessage) (string, error) {
	switch a.provider {
	case "ollama":
		return a.callOllama(ctx, messages)
	default:
		return a.callOpenAICompatible(ctx, messages)
	}
}

func (a *ReActAgent) callOllama(ctx context.Context, messages []LLMMessage) (string, error) {
	node := &OllamaNode{}
	params := map[string]string{
		"model":    a.model,
		"endpoint": a.endpoint,
	}
	fullPrompt := buildConversationPrompt(messages)
	return node.Execute(ctx, fullPrompt, params)
}

func (a *ReActAgent) callOpenAICompatible(ctx context.Context, messages []LLMMessage) (string, error) {
	compatNode := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            a.provider,
		DefaultModel:    a.model,
		DefaultEndpoint: a.endpoint,
		EnvAPIKey:       strings.ToUpper(a.provider) + "_API_KEY",
		ProviderName:    a.provider,
	})
	params := map[string]string{
		"model":    a.model,
		"api_key":  a.apiKey,
		"endpoint": a.endpoint,
	}
	fullPrompt := buildConversationPrompt(messages)
	return compatNode.Execute(ctx, fullPrompt, params)
}

func buildConversationPrompt(messages []LLMMessage) string {
	var parts []string
	for _, m := range messages {
		parts = append(parts, fmt.Sprintf("%s: %s", m.Role, m.Content))
	}
	return strings.Join(parts, "\n\n")
}
