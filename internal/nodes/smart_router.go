package nodes

import (
	"context"
	"fmt"
	"strings"
)

type SmartRouterNode struct{}

func init() {
	Register(&SmartRouterNode{})
}

func (n *SmartRouterNode) Name() string {
	return "smart_router"
}

func (n *SmartRouterNode) Description() string {
	return "Auto-route tasks to the most suitable model based on complexity and content type"
}

func (n *SmartRouterNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "smart_router",
		Description: "Smart router that selects the best model/provider based on task analysis",
		Input:       "string - the task or query to analyze and route",
		Output:      "string - the response from the selected model",
		Params: []ParamSchema{
			{Name: "fast_model", Type: "string", Description: "Fast/cheap model for simple tasks (default: ollama:llama3)", Required: false, Default: "ollama:llama3"},
			{Name: "medium_model", Type: "string", Description: "Medium model for average tasks (default: ollama:llama3)", Required: false, Default: "ollama:llama3"},
			{Name: "strong_model", Type: "string", Description: "Strong model for complex tasks (default: openai:gpt-4o)", Required: false, Default: "openai:gpt-4o"},
			{Name: "system_prompt", Type: "string", Description: "System prompt for the selected model", Required: false},
			{Name: "show_routing", Type: "string", Description: "Show routing decision in output (default: false)", Required: false, Default: "false"},
			{Name: "force_tier", Type: "string", Description: "Force a specific tier: fast, medium, strong (optional)", Required: false},
		},
	}
}

type TaskTier string

const (
	TierFast   TaskTier = "fast"
	TierMedium TaskTier = "medium"
	TierStrong TaskTier = "strong"
)

type TaskAnalysis struct {
	Tier         TaskTier
	Reason       string
	Complexity   int
	Category     string
	NeedThinking bool
}

func (n *SmartRouterNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	fastModel := getParam(params, "fast_model", "ollama:llama3")
	mediumModel := getParam(params, "medium_model", "ollama:llama3")
	strongModel := getParam(params, "strong_model", "openai:gpt-4o")
	systemPrompt := getParam(params, "system_prompt", "")
	showRouting := getParam(params, "show_routing", "false") == "true"
	forceTier := getParam(params, "force_tier", "")

	taskInput := strings.TrimSpace(input)
	if taskInput == "" {
		return "", fmt.Errorf("input cannot be empty")
	}

	var analysis TaskAnalysis
	if forceTier != "" {
		analysis = TaskAnalysis{
			Tier:   TaskTier(forceTier),
			Reason: fmt.Sprintf("Forced by user to %s tier", forceTier),
		}
	} else {
		analysis = analyzeTask(taskInput)
	}

	var selectedModel string
	switch analysis.Tier {
	case TierFast:
		selectedModel = fastModel
	case TierMedium:
		selectedModel = mediumModel
	case TierStrong:
		selectedModel = strongModel
	default:
		selectedModel = mediumModel
	}

	provider, model := parseModelRef(selectedModel)

	compatNode := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            provider,
		DefaultModel:    model,
		DefaultEndpoint: defaultEndpointFor(provider),
		EnvAPIKey:       strings.ToUpper(provider) + "_API_KEY",
		ProviderName:    provider,
	})

	llmParams := map[string]string{
		"model":    model,
		"endpoint": defaultEndpointFor(provider),
	}

	var fullPrompt string
	if systemPrompt != "" {
		fullPrompt = fmt.Sprintf("System: %s\n\nUser: %s", systemPrompt, taskInput)
	} else {
		fullPrompt = taskInput
	}

	response, err := compatNode.Execute(ctx, fullPrompt, llmParams)
	if err != nil {
		return "", fmt.Errorf("model %s execution failed: %w", selectedModel, err)
	}

	if showRouting {
		routingInfo := fmt.Sprintf(
			"[Smart Router]\nTier: %s\nModel: %s\nCategory: %s\nComplexity: %d/10\nReason: %s\n\n---\n\n",
			analysis.Tier, selectedModel, analysis.Category, analysis.Complexity, analysis.Reason,
		)
		return routingInfo + response, nil
	}

	return response, nil
}

func analyzeTask(input string) TaskAnalysis {
	inputLower := strings.ToLower(input)
	inputLen := len(input)

	complexity := 3
	category := "general"
	reason := "Default routing"
	needThinking := false

	codeKeywords := []string{"code", "function", "algorithm", "debug", "refactor", "optimize", "implement", "编程", "代码", "算法", "调试", "优化", "实现"}
	for _, kw := range codeKeywords {
		if strings.Contains(inputLower, kw) {
			category = "coding"
			complexity += 2
			reason = "Coding task detected"
			break
		}
	}

	mathKeywords := []string{"math", "calculate", "equation", "proof", "theorem", "数学", "计算", "方程", "证明", "统计", "概率"}
	for _, kw := range mathKeywords {
		if strings.Contains(inputLower, kw) {
			category = "math"
			complexity += 3
			reason = "Mathematical task detected"
			needThinking = true
			break
		}
	}

	writingKeywords := []string{"write", "essay", "article", "report", "story", "novel", "写", "文章", "报告", "论文", "故事", "小说"}
	for _, kw := range writingKeywords {
		if strings.Contains(inputLower, kw) {
			category = "writing"
			complexity += 1
			reason = "Writing task detected"
			break
		}
	}

	researchKeywords := []string{"research", "analyze", "compare", "evaluate", "study", "研究", "分析", "比较", "评估", "调研"}
	for _, kw := range researchKeywords {
		if strings.Contains(inputLower, kw) {
			category = "research"
			complexity += 3
			reason = "Research/analysis task detected"
			needThinking = true
			break
		}
	}

	translationKeywords := []string{"translate", "translation", "翻译", "transcribe"}
	for _, kw := range translationKeywords {
		if strings.Contains(inputLower, kw) {
			category = "translation"
			complexity -= 1
			reason = "Translation task - can use fast model"
			break
		}
	}

	simpleKeywords := []string{"what is", "who is", "definition", "explain", "什么是", "定义", "解释一下", "hello", "hi", "你好"}
	for _, kw := range simpleKeywords {
		if strings.Contains(inputLower, kw) && inputLen < 200 {
			category = "simple_qa"
			complexity -= 2
			reason = "Simple question-answering"
			break
		}
	}

	if inputLen > 2000 {
		complexity += 2
		reason += " (long input)"
	}
	if strings.Count(input, "?") > 3 {
		complexity += 1
		reason += " (multiple questions)"
	}

	if complexity < 1 {
		complexity = 1
	}
	if complexity > 10 {
		complexity = 10
	}

	var tier TaskTier
	switch {
	case complexity <= 3:
		tier = TierFast
	case complexity <= 6:
		tier = TierMedium
	default:
		tier = TierStrong
	}

	return TaskAnalysis{
		Tier:         tier,
		Reason:       reason,
		Complexity:   complexity,
		Category:     category,
		NeedThinking: needThinking,
	}
}

func parseModelRef(ref string) (provider, model string) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "ollama", ref
}
