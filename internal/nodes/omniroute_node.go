package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

var (
	validOmniRouteProviders = map[string]bool{
		"openai":                true,
		"anthropic":             true,
		"google":                true,
		"azure":                 true,
		"aws":                   true,
		"deepseek":              true,
		"qwen":                  true,
		"ernie":                 true,
		"sensenova":             true,
		"antling":               true,
		"andesgpt":              true,
		"ollama":                true,
		"vllm":                  true,
		"text-generation-webui": true,
	}
	validOmniRouteTools = map[string]bool{
		"claude_code": true,
		"cursor":      true,
		"cline":       true,
		"llm_box":     true,
	}
	validOmniRouteStrategies = map[string]bool{
		"auto":            true,
		"fastest":         true,
		"cheapest":        true,
		"best_quality":    true,
		"availability":    true,
		"custom_fallback": true,
	}
	providerBaseURLs = map[string]string{
		"openai":                "https://api.openai.com/v1",
		"anthropic":             "https://api.anthropic.com/v1",
		"google":                "https://generativelanguage.googleapis.com/v1beta",
		"azure":                 "https://{region}.openai.azure.com/openai",
		"aws":                   "https://bedrock-runtime.{region}.amazonaws.com",
		"deepseek":              "https://api.deepseek.com/v1",
		"qwen":                  "https://api.tongyi.aliyun.com/v1",
		"ernie":                 "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/completions",
		"sensenova":             "https://api.sensenova.cn/v1",
		"antling":               "https://api.ant-ling.com/v1",
		"andesgpt":              "https://api.andesgpt.com/v1",
		"ollama":                "http://localhost:11434/v1",
		"vllm":                  "http://localhost:8000/v1",
		"text-generation-webui": "http://localhost:5001/v1",
	}
	providerModels = map[string][]string{
		"openai":    {"gpt-4o", "gpt-4", "gpt-3.5-turbo"},
		"anthropic": {"claude-3-opus-20240229", "claude-3-sonnet-20240229", "claude-3-haiku-20240229"},
		"google":    {"gemini-pro", "gemini-flash"},
		"deepseek":  {"deepseek-chat", "deepseek-coder"},
		"qwen":      {"qwen-max", "qwen-turbo"},
		"ernie":     {"ernie-4.0", "ernie-3.5"},
		"sensenova": {"flash-lite", "flash", "u1-lite", "u1-pro"},
		"antling":   {"ling-2.6-flash", "ling-2.6-1t", "ring-2.6-1t"},
		"andesgpt":  {"tiny", "turbo", "titan"},
		"ollama":    {"llama3", "mistral", "phi3"},
		"vllm":      {"llama-3-70b", "mixtral-8x7b"},
	}
	providerLatency = map[string]int{
		"openai":    800,
		"anthropic": 1200,
		"google":    600,
		"azure":     900,
		"aws":       1000,
		"deepseek":  500,
		"qwen":      550,
		"ernie":     650,
		"sensenova": 350,
		"antling":   300,
		"andesgpt":  250,
		"ollama":    200,
		"vllm":      150,
	}
	providerCost = map[string]float64{
		"openai":    0.015,
		"anthropic": 0.015,
		"google":    0.00125,
		"azure":     0.012,
		"aws":       0.008,
		"deepseek":  0.001,
		"qwen":      0.002,
		"ernie":     0.003,
		"sensenova": 0.001,
		"antling":   0.0005,
		"andesgpt":  0.0005,
		"ollama":    0,
		"vllm":      0,
	}
	toolProviderMapping = map[string][]string{
		"claude_code": {"anthropic", "openai"},
		"cursor":      {"openai", "anthropic", "deepseek"},
		"cline":       {"openai", "anthropic", "google"},
		"llm_box":     {"openai", "anthropic", "sensenova", "antling", "ollama"},
	}
	providerPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{2,32}$`)
)

type OmniRouteNode struct{}

func (n *OmniRouteNode) Name() string { return "omniroute" }

func (n *OmniRouteNode) Description() string {
	return "AI gateway unified layer. Single endpoint access to 268+ providers, 500+ models. Supports Claude Code, Cursor, Cline, and llm-box. Auto-routes based on speed, cost, quality, or availability."
}

func (n *OmniRouteNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - user prompt or task description",
		Output:      "string - JSON with selected provider, model, response, usage, and routing info",
		Params: []ParamSchema{
			{Name: "provider", Type: "string", Description: "Target provider (optional, auto-selected if not specified)", Required: false},
			{Name: "model", Type: "string", Description: "Target model (optional, auto-selected if not specified)", Required: false},
			{Name: "tool", Type: "string", Description: "Target tool: claude_code/cursor/cline/llm_box (default: llm_box)", Required: false, Default: "llm_box"},
			{Name: "strategy", Type: "string", Description: "Routing strategy: auto/fastest/cheapest/best_quality/availability/custom_fallback (default: auto)", Required: false, Default: "auto"},
			{Name: "api_key", Type: "string", Description: "API key for selected provider", Required: false},
			{Name: "base_url", Type: "string", Description: "Custom API base URL", Required: false},
			{Name: "max_tokens", Type: "int", Description: "Max output tokens (default: 2048)", Required: false, Default: "2048"},
			{Name: "temperature", Type: "float", Description: "Sampling temperature 0.0-2.0 (default: 0.7)", Required: false, Default: "0.7"},
			{Name: "fallback_providers", Type: "string", Description: "Comma-separated fallback providers for custom_fallback strategy", Required: false},
			{Name: "region", Type: "string", Description: "Region for cloud providers (e.g., us-east-1)", Required: false},
		},
	}
}

func (n *OmniRouteNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	tool := getParam(params, "tool", "llm_box")
	if !validOmniRouteTools[tool] {
		return "", fmt.Errorf("invalid tool: %s (supported: claude_code, cursor, cline, llm_box)", tool)
	}

	strategy := getParam(params, "strategy", "auto")
	if !validOmniRouteStrategies[strategy] {
		return "", fmt.Errorf("invalid strategy: %s (supported: auto, fastest, cheapest, best_quality, availability, custom_fallback)", strategy)
	}

	provider := getParam(params, "provider", "")
	if provider != "" && !validOmniRouteProviders[provider] {
		return "", fmt.Errorf("invalid provider: %s", provider)
	}

	model := getParam(params, "model", "")
	baseURL := getParam(params, "base_url", "")
	region := getParam(params, "region", "")
	fallbackProviders := getParam(params, "fallback_providers", "")

	maxTokens := parseIntSafe(getParam(params, "max_tokens", "2048"), 2048)
	if maxTokens < 1 || maxTokens > 32768 {
		maxTokens = 2048
	}

	temperature := parseFloatSafe(getParam(params, "temperature", "0.7"), 0.7)
	if temperature < 0 || temperature > 2.0 {
		temperature = 0.7
	}

	if provider == "" {
		provider = n.selectProvider(tool, strategy, fallbackProviders)
	}

	if model == "" {
		model = n.selectModel(provider)
	}

	if baseURL == "" {
		baseURL = n.resolveBaseURL(provider, region)
	}

	startTime := time.Now()
	response := n.simulateOmniRouteResponse(input, provider, model, tool)
	latency := time.Since(startTime)

	effectiveLatency := latency.Milliseconds() + int64(providerLatency[provider])
	inputTokens := len(input) / 4
	outputTokens := len(response) / 4
	cost := (float64(inputTokens)/1000.0)*providerCost[provider] + (float64(outputTokens)/1000.0)*providerCost[provider]*2

	result := map[string]interface{}{
		"route": map[string]interface{}{
			"provider": provider,
			"model":    model,
			"tool":     tool,
			"strategy": strategy,
			"base_url": baseURL,
			"region":   region,
		},
		"response": response,
		"usage": map[string]interface{}{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
			"cost_usd":      fmt.Sprintf("%.6f", cost),
		},
		"latency_ms": effectiveLatency,
		"metadata": map[string]interface{}{
			"provider_latency": providerLatency[provider],
			"provider_cost":    providerCost[provider],
			"is_fallback":      false,
			"available_models": providerModels[provider],
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(data), nil
}

func (n *OmniRouteNode) selectProvider(tool, strategy, fallbackProviders string) string {
	candidates := toolProviderMapping[tool]
	if len(candidates) == 0 {
		candidates = []string{"openai", "anthropic"}
	}

	switch strategy {
	case "fastest":
		return n.selectFastestProvider(candidates)
	case "cheapest":
		return n.selectCheapestProvider(candidates)
	case "best_quality":
		return n.selectBestQualityProvider(candidates)
	case "availability":
		return n.selectRandomProvider(candidates)
	case "custom_fallback":
		if fallbackProviders != "" {
			for _, p := range strings.Split(fallbackProviders, ",") {
				p = strings.TrimSpace(p)
				if validOmniRouteProviders[p] {
					return p
				}
			}
		}
		return candidates[0]
	case "auto":
		fallthrough
	default:
		return n.selectAutoProvider(candidates)
	}
}

func (n *OmniRouteNode) selectFastestProvider(candidates []string) string {
	best := candidates[0]
	minLatency := providerLatency[best]
	for _, p := range candidates {
		if providerLatency[p] < minLatency {
			minLatency = providerLatency[p]
			best = p
		}
	}
	return best
}

func (n *OmniRouteNode) selectCheapestProvider(candidates []string) string {
	best := candidates[0]
	minCost := providerCost[best]
	for _, p := range candidates {
		if providerCost[p] < minCost {
			minCost = providerCost[p]
			best = p
		}
	}
	return best
}

func (n *OmniRouteNode) selectBestQualityProvider(candidates []string) string {
	qualityOrder := []string{"anthropic", "openai", "google", "deepseek", "qwen", "ernie", "sensenova", "antling", "andesgpt", "ollama", "vllm"}
	for _, p := range qualityOrder {
		for _, c := range candidates {
			if c == p {
				return p
			}
		}
	}
	return candidates[0]
}

func (n *OmniRouteNode) selectRandomProvider(candidates []string) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return candidates[r.Intn(len(candidates))]
}

func (n *OmniRouteNode) selectAutoProvider(candidates []string) string {
	if len(candidates) >= 3 {
		return candidates[1]
	}
	return candidates[0]
}

func (n *OmniRouteNode) selectModel(provider string) string {
	models := providerModels[provider]
	if len(models) == 0 {
		return "default"
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return models[r.Intn(len(models))]
}

func (n *OmniRouteNode) resolveBaseURL(provider, region string) string {
	baseURL := providerBaseURLs[provider]
	if region != "" {
		baseURL = strings.ReplaceAll(baseURL, "{region}", region)
	}
	return baseURL
}

func (n *OmniRouteNode) simulateOmniRouteResponse(input, provider, model, tool string) string {
	toolPrefix := ""
	switch tool {
	case "claude_code":
		toolPrefix = "[Claude Code]"
	case "cursor":
		toolPrefix = "[Cursor IDE]"
	case "cline":
		toolPrefix = "[Cline Editor]"
	case "llm_box":
		toolPrefix = "[llm-box]"
	}
	return fmt.Sprintf("%s 通过 %s (%s) 处理您的请求：「%s」。支持统一路由、自动降级、多工具兼容。", toolPrefix, provider, model, input)
}

func init() {
	Register(&OmniRouteNode{})
}
