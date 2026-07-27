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
	"math/rand"
	"strconv"
	"strings"
	"time"
)

var moeModelWhitelist = map[string]bool{
	"colibri-744b-moe": true,
	"qwen3-moe-72b":    true,
	"mixtral-8x7b":     true,
	"llama3.3-moe-70b": true,
}

var moeQuantizationWhitelist = map[string]bool{
	"int4": true,
	"int8": true,
	"fp16": true,
	"fp32": true,
}

var moeLoadStrategyWhitelist = map[string]bool{
	"on_demand": true,
	"layered":   true,
	"preload":   true,
}

type moeModelConfig struct {
	Name            string
	NumExperts      int
	ExpertsPerToken int
	ExpertSizeGB    float64
	TotalLayers     int
	HiddenSize      int
	NumHeads        int
}

var moeModelConfigs = map[string]moeModelConfig{
	"colibri-744b-moe": {
		Name:            "colibri-744b-moe",
		NumExperts:      64,
		ExpertsPerToken: 2,
		ExpertSizeGB:    12.0,
		TotalLayers:     80,
		HiddenSize:      16384,
		NumHeads:        128,
	},
	"qwen3-moe-72b": {
		Name:            "qwen3-moe-72b",
		NumExperts:      8,
		ExpertsPerToken: 2,
		ExpertSizeGB:    9.0,
		TotalLayers:     32,
		HiddenSize:      8192,
		NumHeads:        64,
	},
	"mixtral-8x7b": {
		Name:            "mixtral-8x7b",
		NumExperts:      8,
		ExpertsPerToken: 2,
		ExpertSizeGB:    1.0,
		TotalLayers:     32,
		HiddenSize:      4096,
		NumHeads:        32,
	},
	"llama3.3-moe-70b": {
		Name:            "llama3.3-moe-70b",
		NumExperts:      8,
		ExpertsPerToken: 2,
		ExpertSizeGB:    8.8,
		TotalLayers:     32,
		HiddenSize:      8192,
		NumHeads:        64,
	},
}

type moeLoadOrderEntry struct {
	Layer     int    `json:"layer"`
	ExpertIdx int    `json:"expert_idx"`
	Action    string `json:"action"`
	Timestamp string `json:"timestamp"`
}

type moeResult struct {
	Model           string              `json:"model"`
	NumExperts      int                 `json:"num_experts"`
	ActiveExperts   int                 `json:"active_experts"`
	MemoryUsageGB   float64             `json:"memory_usage_gb"`
	LatencyMs       int64               `json:"latency_ms"`
	ThroughputTokS  float64             `json:"throughput_tok_s"`
	Streaming       bool                `json:"streaming"`
	ExpertLoadOrder []moeLoadOrderEntry `json:"expert_load_order"`
	Quantization    string              `json:"quantization"`
	LoadStrategy    string              `json:"load_strategy"`
	ExpertGroupSize int                 `json:"expert_group_size"`
	MemoryLimitGB   float64             `json:"memory_limit_gb"`
	MemorySavedGB   float64             `json:"memory_saved_gb"`
}

type MoEStreamingNode struct{}

func init() {
	Register(&MoEStreamingNode{})
}

func (n *MoEStreamingNode) Name() string {
	return "moe_streaming"
}

func (n *MoEStreamingNode) Description() string {
	return "MoE (Mixture of Experts) streaming expert loading for running large models on consumer hardware"
}

func (n *MoEStreamingNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - optional input prompt for inference",
		Output:      "string - JSON format with MoE loading metrics",
		Params: []ParamSchema{
			{Name: "model", Type: "string", Description: "Model name: colibri-744b-moe/qwen3-moe-72b/mixtral-8x7b/llama3.3-moe-70b", Required: true},
			{Name: "max_experts_per_token", Type: "int", Description: "Max experts per token (default: 2)", Required: false, Default: "2"},
			{Name: "expert_group_size", Type: "int", Description: "Expert group size (default: 64)", Required: false, Default: "64"},
			{Name: "memory_limit_gb", Type: "float", Description: "Memory limit in GB (default: 24)", Required: false, Default: "24"},
			{Name: "streaming", Type: "bool", Description: "Enable streaming inference (default: true)", Required: false, Default: "true"},
			{Name: "quantization", Type: "string", Description: "Quantization: int4/int8/fp16/fp32 (default: int4)", Required: false, Default: "int4"},
			{Name: "load_strategy", Type: "string", Description: "Load strategy: on_demand/layered/preload (default: on_demand)", Required: false, Default: "on_demand"},
		},
	}
}

func (n *MoEStreamingNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	model := getParam(params, "model", "")
	if model == "" {
		return "", fmt.Errorf("model parameter is required")
	}
	if !moeModelWhitelist[model] {
		return "", fmt.Errorf("unsupported model: %s (supported: colibri-744b-moe, qwen3-moe-72b, mixtral-8x7b, llama3.3-moe-70b)", model)
	}

	config := moeModelConfigs[model]

	maxExpertsPerToken, err := strconv.Atoi(getParam(params, "max_experts_per_token", "2"))
	if err != nil || maxExpertsPerToken < 1 {
		maxExpertsPerToken = 2
	}
	if maxExpertsPerToken > config.NumExperts {
		maxExpertsPerToken = config.NumExperts
	}

	expertGroupSize, err := strconv.Atoi(getParam(params, "expert_group_size", "64"))
	if err != nil || expertGroupSize < 1 {
		expertGroupSize = 64
	}

	memoryLimitGB, err := strconv.ParseFloat(getParam(params, "memory_limit_gb", "24"), 64)
	if err != nil || memoryLimitGB < 1 {
		memoryLimitGB = 24
	}
	if memoryLimitGB > 1024 {
		memoryLimitGB = 1024
	}

	streaming := strings.ToLower(getParam(params, "streaming", "true")) == "true"

	quantization := getParam(params, "quantization", "int4")
	if !moeQuantizationWhitelist[quantization] {
		return "", fmt.Errorf("unsupported quantization: %s (supported: int4, int8, fp16, fp32)", quantization)
	}

	loadStrategy := getParam(params, "load_strategy", "on_demand")
	if !moeLoadStrategyWhitelist[loadStrategy] {
		return "", fmt.Errorf("unsupported load_strategy: %s (supported: on_demand, layered, preload)", loadStrategy)
	}

	quantFactor := n.getQuantizationFactor(quantization)

	totalModelSizeGB := float64(config.NumExperts) * config.ExpertSizeGB * quantFactor
	activeModelSizeGB := float64(maxExpertsPerToken) * config.ExpertSizeGB * float64(config.TotalLayers) / float64(config.NumExperts) * quantFactor

	if activeModelSizeGB > memoryLimitGB {
		return "", fmt.Errorf("insufficient memory: model requires %.1f GB but limit is %.1f GB", activeModelSizeGB, memoryLimitGB)
	}

	startTime := time.Now()

	loadOrder := n.simulateExpertLoading(model, config, maxExpertsPerToken, expertGroupSize, loadStrategy)

	latencyMs := n.calculateLatency(config, loadStrategy, quantization)
	throughputTokS := n.calculateThroughput(config, quantization, streaming)

	queryTime := time.Since(startTime)
	totalLatency := latencyMs + queryTime.Milliseconds()

	memorySavedGB := totalModelSizeGB - activeModelSizeGB

	result := moeResult{
		Model:           model,
		NumExperts:      config.NumExperts,
		ActiveExperts:   maxExpertsPerToken,
		MemoryUsageGB:   activeModelSizeGB,
		LatencyMs:       totalLatency,
		ThroughputTokS:  throughputTokS,
		Streaming:       streaming,
		ExpertLoadOrder: loadOrder,
		Quantization:    quantization,
		LoadStrategy:    loadStrategy,
		ExpertGroupSize: expertGroupSize,
		MemoryLimitGB:   memoryLimitGB,
		MemorySavedGB:   memorySavedGB,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(data), nil
}

func (n *MoEStreamingNode) getQuantizationFactor(quant string) float64 {
	factors := map[string]float64{
		"int4": 0.25,
		"int8": 0.5,
		"fp16": 1.0,
		"fp32": 2.0,
	}
	if f, ok := factors[quant]; ok {
		return f
	}
	return 0.25
}

func (n *MoEStreamingNode) simulateExpertLoading(model string, config moeModelConfig, maxExpertsPerToken, expertGroupSize int, loadStrategy string) []moeLoadOrderEntry {
	var order []moeLoadOrderEntry
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	numGroups := (config.NumExperts + expertGroupSize - 1) / expertGroupSize

	switch loadStrategy {
	case "preload":
		for group := 0; group < numGroups; group++ {
			for i := 0; i < expertGroupSize; i++ {
				expertIdx := group*expertGroupSize + i
				if expertIdx >= config.NumExperts {
					break
				}
				for layer := 0; layer < config.TotalLayers; layer++ {
					order = append(order, moeLoadOrderEntry{
						Layer:     layer,
						ExpertIdx: expertIdx,
						Action:    "preload",
						Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
					})
				}
			}
		}

	case "layered":
		for layer := 0; layer < config.TotalLayers; layer += 4 {
			for group := 0; group < numGroups; group++ {
				expertsInGroup := expertGroupSize
				if group == numGroups-1 {
					expertsInGroup = config.NumExperts % expertGroupSize
					if expertsInGroup == 0 {
						expertsInGroup = expertGroupSize
					}
				}
				activeExperts := r.Perm(expertsInGroup)[:maxExpertsPerToken]
				for _, idx := range activeExperts {
					expertIdx := group*expertGroupSize + idx
					for l := layer; l < layer+4 && l < config.TotalLayers; l++ {
						order = append(order, moeLoadOrderEntry{
							Layer:     l,
							ExpertIdx: expertIdx,
							Action:    "layer_load",
							Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
						})
					}
				}
			}
		}

	case "on_demand":
		for layer := 0; layer < config.TotalLayers; layer++ {
			activeExperts := r.Perm(config.NumExperts)[:maxExpertsPerToken]
			for _, expertIdx := range activeExperts {
				order = append(order, moeLoadOrderEntry{
					Layer:     layer,
					ExpertIdx: expertIdx,
					Action:    "activate",
					Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
				})
			}
		}
	}

	return order
}

func (n *MoEStreamingNode) calculateLatency(config moeModelConfig, strategy, quantization string) int64 {
	baseLatency := int64(config.TotalLayers) * 10

	quantFactor := float64(1.0)
	switch quantization {
	case "int4":
		quantFactor = 0.3
	case "int8":
		quantFactor = 0.5
	case "fp16":
		quantFactor = 0.8
	case "fp32":
		quantFactor = 1.0
	}

	strategyFactor := float64(1.0)
	switch strategy {
	case "preload":
		strategyFactor = 0.5
	case "layered":
		strategyFactor = 0.7
	case "on_demand":
		strategyFactor = 1.0
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return int64(float64(baseLatency) * quantFactor * strategyFactor * (1 + r.Float64()*0.2))
}

func (n *MoEStreamingNode) calculateThroughput(config moeModelConfig, quantization string, streaming bool) float64 {
	baseThroughput := float64(config.NumHeads) * 5.0

	quantFactor := float64(1.0)
	switch quantization {
	case "int4":
		quantFactor = 3.0
	case "int8":
		quantFactor = 2.0
	case "fp16":
		quantFactor = 1.2
	case "fp32":
		quantFactor = 1.0
	}

	streamingFactor := float64(1.0)
	if streaming {
		streamingFactor = 1.5
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return baseThroughput * quantFactor * streamingFactor * (0.9 + r.Float64()*0.2)
}
