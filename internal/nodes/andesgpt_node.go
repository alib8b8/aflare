// Copyright (c) 2026 llm-box Contributors
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
	"regexp"
	"strings"
	"time"
)

var (
	validAndesSizes = map[string]bool{
		"tiny":  true, // ~1B, 端侧
		"turbo": true, // ~7B, 端云协同
		"titan": true, // ~100B+, 云端
	}
	validAndesScenes = map[string]bool{
		"life":         true,
		"imaging":      true,
		"productivity": true,
		"creative":     true,
		"knowledge":    true,
	}
	personaIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)
)

// AndesGPTNode integrates OPPO's AndesGPT large model (Tiny/Turbo/Titan)
type AndesGPTNode struct{}

func (n *AndesGPTNode) Name() string { return "andesgpt" }

func (n *AndesGPTNode) Description() string {
	return "OPPO AndesGPT large model integration. Supports Tiny (端侧1B), Turbo (端云协同7B), Titan (云端100B+) sizes with PersonaX personalization and end-cloud collaboration."
}

func (n *AndesGPTNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - user prompt or conversation message",
		Output:      "string - model response with persona and collaboration metadata",
		Params: []ParamSchema{
			{Name: "model_size", Type: "string", Description: "Model size: tiny (端侧1B) / turbo (端云协同7B) / titan (云端100B+) (default: turbo)", Required: false, Default: "turbo"},
			{Name: "scene", Type: "string", Description: "Application scene: life/imaging/productivity/creative/knowledge (default: life)", Required: false, Default: "life"},
			{Name: "persona_id", Type: "string", Description: "PersonaX persona ID for personalized responses (千人千面)", Required: false},
			{Name: "use_memory", Type: "bool", Description: "Use PersonaX long-term memory (default: true)", Required: false, Default: "true"},
			{Name: "max_tokens", Type: "int", Description: "Max output tokens (default: 1024)", Required: false, Default: "1024"},
			{Name: "temperature", Type: "float", Description: "Sampling temperature 0.0-1.0 (default: 0.7)", Required: false, Default: "0.7"},
			{Name: "system_prompt", Type: "string", Description: "System prompt", Required: false},
			{Name: "stream", Type: "bool", Description: "Enable streaming response (default: false)", Required: false, Default: "false"},
			{Name: "end_cloud_mode", Type: "string", Description: "End-cloud collaboration mode: auto/force_end/force_cloud (default: auto)", Required: false, Default: "auto"},
		},
	}
}

func (n *AndesGPTNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	modelSize := getMobileParam(params, "model_size", "turbo")
	if !validAndesSizes[modelSize] {
		return "", fmt.Errorf("invalid model_size: %s (tiny/turbo/titan)", modelSize)
	}

	scene := getMobileParam(params, "scene", "life")
	if !validAndesScenes[scene] {
		return "", fmt.Errorf("invalid scene: %s", scene)
	}

	personaID := getMobileParam(params, "persona_id", "")
	if personaID != "" && !personaIDPattern.MatchString(personaID) {
		return "", fmt.Errorf("invalid persona_id format")
	}

	useMemory := strings.ToLower(getMobileParam(params, "use_memory", "true")) == "true"

	maxTokens := parseIntSafe(getMobileParam(params, "max_tokens", "1024"), 1024)
	if maxTokens < 1 || maxTokens > 8192 {
		maxTokens = 1024
	}

	systemPrompt := getMobileParam(params, "system_prompt", "")
	if len(systemPrompt) > 4000 {
		return "", fmt.Errorf("system_prompt too long")
	}

	stream := strings.ToLower(getMobileParam(params, "stream", "false")) == "true"
	endCloudMode := getMobileParam(params, "end_cloud_mode", "auto")
	if endCloudMode != "auto" && endCloudMode != "force_end" && endCloudMode != "force_cloud" {
		return "", fmt.Errorf("invalid end_cloud_mode: %s", endCloudMode)
	}

	// Determine execution location based on model size and end-cloud mode
	execLocation := determineExecLocation(modelSize, endCloudMode, input)

	// Simulate AndesGPT response
	startTime := time.Now()
	response := simulateAndesResponse(input, modelSize, scene, personaID, useMemory, systemPrompt, maxTokens)
	latency := time.Since(startTime)

	// Estimate token usage
	inputTokens := len(input) / 4
	outputTokens := len(response) / 4

	result := map[string]interface{}{
		"type":           "andesgpt",
		"model_size":     modelSize,
		"scene":          scene,
		"persona_id":     personaID,
		"use_memory":     useMemory,
		"exec_location":  execLocation,
		"end_cloud_mode": endCloudMode,
		"response":       response,
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

func determineExecLocation(modelSize, endCloudMode, input string) string {
	if endCloudMode == "force_end" {
		return "on_device"
	}
	if endCloudMode == "force_cloud" {
		return "cloud"
	}

	// Auto mode: decide based on model size and input complexity
	switch modelSize {
	case "tiny":
		return "on_device"
	case "turbo":
		// Simple queries on device, complex ones on cloud
		if len(input) < 100 {
			return "on_device"
		}
		return "end_cloud_collaboration"
	case "titan":
		return "cloud"
	default:
		return "cloud"
	}
}

func simulateAndesResponse(input, modelSize, scene, personaID string, useMemory bool, systemPrompt string, maxTokens int) string {
	// Scene-specific responses
	switch scene {
	case "life":
		if useMemory && personaID != "" {
			return fmt.Sprintf("基于您的个人偏好，为您推荐：%s。小布已记住您的喜好，下次会提供更精准的建议。", input)
		}
		return fmt.Sprintf("生活助手：已为您处理「%s」的相关请求。", input)
	case "imaging":
		return fmt.Sprintf("影像AI：已优化处理「%s」的图像相关任务，支持AI消除、AI虚化、AI增强。", input)
	case "productivity":
		return fmt.Sprintf("效率助手：已为您完成「%s」的办公任务，支持文档摘要、邮件润色、日程管理。", input)
	case "creative":
		return fmt.Sprintf("创作助手：基于「%s」已生成创意内容，支持文生图、音乐生成、写真生成。", input)
	case "knowledge":
		return fmt.Sprintf("知识问答：关于「%s」的问题，AndesGPT结合增强检索提供准确回答，降低幻觉。", input)
	default:
		return fmt.Sprintf("AndesGPT (%s): %s", modelSize, input)
	}
}

func init() {
	Register(&AndesGPTNode{})
}
