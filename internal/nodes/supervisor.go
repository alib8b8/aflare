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

type SupervisorNode struct{}

func init() {
	Register(&SupervisorNode{})
}

func (n *SupervisorNode) Name() string {
	return "supervisor"
}

func (n *SupervisorNode) Description() string {
	return "Supervisor agent with MoE routing, MindSearch mode, and domain specialist delegation"
}

func (n *SupervisorNode) Schema() NodeSchema {
	params := []ParamSchema{
		{Name: "provider", Type: "string", Description: "LLM provider (default: ollama)", Required: false, Default: "ollama"},
		{Name: "model", Type: "string", Description: "Model name (default: llama3)", Required: false, Default: "llama3"},
		{Name: "api_key", Type: "string", Description: "API key for cloud providers", Required: false},
		{Name: "endpoint", Type: "string", Description: "API endpoint URL", Required: false},
	}
	params = append(params,
		ParamSchema{Name: "specialists", Type: "string", Description: "Comma-separated list of specialist agents: planner,researcher,critic,code_review,evaluator,reflector,legal_expert,medical_expert,educational_expert,financial_expert,creative_writer,data_analyst", Required: false, Default: "planner,researcher,critic,evaluator"},
		ParamSchema{Name: "strategy", Type: "string", Description: "Strategy: sequential, parallel, hierarchical, mindsearch, moe, agency, swarm (default: sequential)", Required: false, Default: "sequential"},
		ParamSchema{Name: "output_format", Type: "string", Description: "Output format: json, markdown, summary (default: json)", Required: false, Default: "json"},
		ParamSchema{Name: "domain", Type: "string", Description: "Domain specialization: general,legal,medical,education,finance,creative,tech,business (default: general)", Required: false, Default: "general"},
		ParamSchema{Name: "enable_moe", Type: "string", Description: "Enable Mixture-of-Experts routing (default: false)", Required: false, Default: "false"},
		ParamSchema{Name: "max_depth", Type: "string", Description: "Max decomposition depth for hierarchical/mindsearch (default: 3)", Required: false, Default: "3"},
		ParamSchema{Name: "subagent_prompts", Type: "string", Description: "Inject per-specialist subagent prompt templates into the supervisor context (default: true). Borrows Grok Build's main/subagent prompt hierarchy.", Required: false, Default: "true"},
		ParamSchema{Name: "collaboration_template", Type: "string", Description: "Collaboration template: software_development, product_design, data_science, marketing, research, legal_compliance, healthcare, education, finance, game_development, video_production, security_operations, cloud_infrastructure, content_creation, community_management, startup_acceleration, ai_development, design_system, event_management, translation_localization", Required: false},
		ParamSchema{Name: "template_role", Type: "string", Description: "Template role to use: team, workflow, review_cycle (default: team)", Required: false, Default: "team"},
	)
	return NodeSchema{
		Name:        "supervisor",
		Description: "Advanced supervisor with MoE routing, MindSearch deep research, 232+ domain specialists, and collaboration templates",
		Input:       "string - the overall goal or task to supervise",
		Output:      "string - structured task plan with delegation and synthesis",
		Params:      params,
	}
}

func (n *SupervisorNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	provider := getParam(params, "provider", "ollama")
	model := getParam(params, "model", "llama3")
	apiKey := getParam(params, "api_key", "")
	endpoint := getParam(params, "endpoint", defaultEndpointFor(provider))
	specialists := getParam(params, "specialists", "planner,researcher,critic,evaluator")
	strategy := getParam(params, "strategy", "sequential")
	outputFormat := getParam(params, "output_format", "json")
	domain := getParam(params, "domain", "general")
	enableMoE := getParam(params, "enable_moe", "false") == "true"
	maxDepthStr := getParam(params, "max_depth", "3")
	collaborationTemplate := getParam(params, "collaboration_template", "")
	templateRole := getParam(params, "template_role", "team")

	maxDepth := 3
	if _, err := fmt.Sscanf(maxDepthStr, "%d", &maxDepth); err != nil {
		// keep default value on parse failure
	}
	if maxDepth < 1 {
		maxDepth = 1
	}
	if maxDepth > 10 {
		maxDepth = 10
	}

	if collaborationTemplate != "" {
		if template, ok := collaborationTemplates[collaborationTemplate]; ok {
			if roleSpecialists, ok := template[templateRole]; ok {
				specialists = strings.Join(roleSpecialists, ",")
			}
		}
	} else if preset, ok := domainSpecialistPresets[domain]; ok && specialists == "planner,researcher,critic,evaluator" {
		specialists = strings.Join(preset, ",")
	}

	specialistList := strings.Split(specialists, ",")
	for i := range specialistList {
		specialistList[i] = strings.TrimSpace(specialistList[i])
	}

	specDescs := buildSpecialistDescriptions(specialistList)

	var systemPrompt string
	switch strategy {
	case "mindsearch":
		systemPrompt = buildMindSearchPrompt(specDescs, maxDepth)
	case "moe":
		systemPrompt = buildMoEPrompt(specDescs)
	case "parallel":
		systemPrompt = buildParallelPrompt(specDescs)
	case "hierarchical":
		systemPrompt = buildHierarchicalPrompt(specDescs, maxDepth)
	case "agency":
		systemPrompt = buildAgencyPrompt(specDescs)
	case "swarm":
		systemPrompt = buildSwarmPrompt(specDescs)
	default:
		systemPrompt = buildSequentialPrompt(specDescs)
	}

	if enableMoE && strategy != "moe" {
		systemPrompt += "\n\nAdditionally, use Mixture-of-Experts routing: when a subtask requires multiple expertise areas, assign it to multiple specialists and merge their results."
	}

	// 子智能体提示词分层：注入各 specialist 的行为边界模板（借鉴 Grok Build 主/子 Agent 架构）
	if getParam(params, "subagent_prompts", "true") != "false" {
		systemPrompt += RenderSubagentPromptForSpecialists(specialistList)
	}

	result, err := runAgentLLM(ctx, provider, model, apiKey, endpoint, systemPrompt, input)
	if err != nil {
		return "", fmt.Errorf("supervisor agent failed: %w", err)
	}

	if outputFormat == "json" {
		return cleanJSONResp(result), nil
	}

	return result, nil
}

func cleanJSONResp(response string) string {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)
	return response
}
