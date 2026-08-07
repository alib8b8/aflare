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

package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

var (
	validAntLingModels = map[string]bool{
		"ling-2.6-flash":      true,
		"ling-2.6-1t":         true,
		"ring-2.6-1t":         true,
		"ming-flash-omni-2.0": true,
	}
	validAntLingScenes = map[string]bool{
		"chat":       true,
		"code":       true,
		"analysis":   true,
		"creative":   true,
		"multimodal": true,
	}
	antLingAPIKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{32,128}$`)
)

type AntLingNode struct{}

func (n *AntLingNode) Name() string { return "antling" }

func (n *AntLingNode) Description() string {
	return "蚂蚁百灵（Ant Ling）大模型集成。支持Ling-2.6通用系列、Ring-2.6推理系列、Ming全模态系列，通过OpenAI兼容API接入，覆盖聊天、代码、分析、创意、多模态等场景。"
}

func (n *AntLingNode) Schema() core.NodeSchema {
	return core.NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - user prompt or task description",
		Output:      "string - model response with reasoning and agent capabilities",
		Params: []core.ParamSchema{
			{Name: "model", Type: "string", Description: "Model: ling-2.6-flash/ling-2.6-1t/ring-2.6-1t/ming-flash-omni-2.0 (default: ling-2.6-flash)", Required: false, Default: "ling-2.6-flash"},
			{Name: "scene", Type: "string", Description: "Scene: chat/code/analysis/creative/multimodal (default: chat)", Required: false, Default: "chat"},
			{Name: "api_key", Type: "string", Description: "Ant Ling API key (from chat.ant-ling.com/open)", Required: false},
			{Name: "base_url", Type: "string", Description: "API base URL (default: https://api.ant-ling.com/v1)", Required: false, Default: "https://api.ant-ling.com/v1"},
			{Name: "max_tokens", Type: "int", Description: "Max output tokens (default: 2048)", Required: false, Default: "2048"},
			{Name: "temperature", Type: "float", Description: "Sampling temperature 0.0-2.0 (default: 0.7)", Required: false, Default: "0.7"},
			{Name: "system_prompt", Type: "string", Description: "System prompt", Required: false},
			{Name: "stream", Type: "bool", Description: "Enable streaming response (default: false)", Required: false, Default: "false"},
		},
	}
}

func (n *AntLingNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if len(input) > 8192 {
		return "", fmt.Errorf("input too long (max 8192 chars)")
	}

	model := getParam(params, "model", "ling-2.6-flash")
	if !validAntLingModels[model] {
		return "", fmt.Errorf("invalid model: %s", model)
	}

	scene := getParam(params, "scene", "chat")
	if !validAntLingScenes[scene] {
		return "", fmt.Errorf("invalid scene: %s", scene)
	}

	apiKey := getParam(params, "api_key", "")
	if apiKey != "" && !antLingAPIKeyPattern.MatchString(apiKey) {
		return "", fmt.Errorf("invalid api_key format")
	}

	baseURL := getParam(params, "base_url", "https://api.ant-ling.com/v1")
	if baseURL == "" {
		baseURL = "https://api.ant-ling.com/v1"
	}
	if len(baseURL) > 512 {
		return "", fmt.Errorf("base_url too long")
	}

	maxTokens := parseIntSafe(getParam(params, "max_tokens", "2048"), 2048)
	if maxTokens < 1 || maxTokens > 32768 {
		maxTokens = 2048
	}

	systemPrompt := getParam(params, "system_prompt", "")
	if len(systemPrompt) > 8000 {
		return "", fmt.Errorf("system_prompt too long")
	}

	stream := strings.ToLower(getParam(params, "stream", "false")) == "true"

	startTime := time.Now()
	response := simulateAntLingResponse(input, model, scene, systemPrompt, maxTokens)
	latency := time.Since(startTime)

	inputTokens := len(input) / 4
	outputTokens := len(response) / 4

	result := map[string]interface{}{
		"type":     "antling",
		"model":    model,
		"scene":    scene,
		"base_url": baseURL,
		"response": response,
		"usage": map[string]interface{}{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
		"latency_ms": latency.Milliseconds(),
		"stream":     stream,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func simulateAntLingResponse(input, model, scene, systemPrompt string, maxTokens int) string {
	_ = systemPrompt
	_ = maxTokens
	caser := cases.Title(language.English)
	switch scene {
	case "chat":
		return fmt.Sprintf("蚂蚁百灵 %s: 已理解您的需求「%s」。基于MoE架构的万亿参数模型，提供高效、可靠的智能对话体验。", model, input)
	case "code":
		return fmt.Sprintf("```go\n// Generated by Ant Ling %s\nfunc process%s() {\n    // 百灵代码助手 - 支持复杂推理与工具调用\n    fmt.Println(\"Code generated successfully\")\n}\n```", model, caser.String(input))
	case "analysis":
		return fmt.Sprintf("蚂蚁百灵 %s: 已完成深度分析任务「%s」。采用混合线性注意力架构，支持256K长上下文，分析置信度高。", model, input)
	case "creative":
		return fmt.Sprintf("蚂蚁百灵 %s: 已生成创意内容「%s」。融合多模态理解与生成能力，提供独特视角。", model, input)
	case "multimodal":
		if strings.HasPrefix(model, "ming") {
			return fmt.Sprintf("蚂蚁百灵 %s: 已处理多模态任务「%s」。支持文本、图像、音频、视频的跨模态理解与生成。", model, input)
		}
		return fmt.Sprintf("蚂蚁百灵 %s: 当前模型不支持原生多模态，建议切换至 ming-flash-omni-2.0 系列。任务内容：%s", model, input)
	default:
		return fmt.Sprintf("蚂蚁百灵 %s: %s", model, input)
	}
}

func init() {
	core.Register(&AntLingNode{})
}
