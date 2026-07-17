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
		ParamSchema{Name: "strategy", Type: "string", Description: "Strategy: sequential, parallel, hierarchical, mindsearch, moe (default: sequential)", Required: false, Default: "sequential"},
		ParamSchema{Name: "output_format", Type: "string", Description: "Output format: json, markdown, summary (default: json)", Required: false, Default: "json"},
		ParamSchema{Name: "domain", Type: "string", Description: "Domain specialization: general,legal,medical,education,finance,creative,tech,business (default: general)", Required: false, Default: "general"},
		ParamSchema{Name: "enable_moe", Type: "string", Description: "Enable Mixture-of-Experts routing (default: false)", Required: false, Default: "false"},
		ParamSchema{Name: "max_depth", Type: "string", Description: "Max decomposition depth for hierarchical/mindsearch (default: 3)", Required: false, Default: "3"},
	)
	return NodeSchema{
		Name:        "supervisor",
		Description: "Advanced supervisor with MoE routing, MindSearch deep research, and 12+ domain specialists",
		Input:       "string - the overall goal or task to supervise",
		Output:      "string - structured task plan with delegation and synthesis",
		Params:      params,
	}
}

var allSpecialists = map[string]string{
	"planner":          "planner — breaks tasks into structured steps and milestones",
	"researcher":       "researcher — gathers, verifies, and synthesizes information",
	"critic":           "critic — reviews quality, finds flaws, suggests improvements",
	"code_review":      "code_review — audits code for bugs, security, performance, style",
	"evaluator":        "evaluator — scores output against rubrics and success criteria",
	"reflector":        "reflector — self-improves output through iterative refinement",
	"legal_expert":     "legal_expert — legal analysis, contract review, compliance guidance",
	"medical_expert":   "medical_expert — health information, medical research, clinical guidance",
	"educational_expert": "educational_expert — curriculum design, learning paths, pedagogical advice",
	"financial_expert": "financial_expert — financial analysis, investment, budgeting, risk assessment",
	"creative_writer":  "creative_writer — content creation, storytelling, copywriting, editing",
	"data_analyst":     "data_analyst — data analysis, statistics, visualization, insights",
	"ux_designer":      "ux_designer — user experience design, wireframing, usability review",
	"product_manager":  "product_manager — product strategy, roadmap, requirements, prioritization",
	"qa_engineer":      "qa_engineer — test planning, quality assurance, edge case analysis",
	"devops_engineer":  "devops_engineer — infrastructure, deployment, CI/CD, monitoring",
}

var domainSpecialistPresets = map[string][]string{
	"general":  {"planner", "researcher", "critic", "evaluator"},
	"legal":    {"planner", "legal_expert", "researcher", "critic", "evaluator"},
	"medical":  {"planner", "medical_expert", "researcher", "critic", "evaluator"},
	"education": {"planner", "educational_expert", "researcher", "creative_writer", "evaluator"},
	"finance":  {"planner", "financial_expert", "data_analyst", "researcher", "evaluator"},
	"creative": {"planner", "creative_writer", "critic", "reflector", "evaluator"},
	"tech":     {"planner", "code_review", "qa_engineer", "devops_engineer", "architect"},
	"business": {"planner", "product_manager", "financial_expert", "data_analyst", "evaluator"},
	"research": {"planner", "researcher", "data_analyst", "critic", "evaluator"},
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

	maxDepth := 3
	fmt.Sscanf(maxDepthStr, "%d", &maxDepth)
	if maxDepth < 1 {
		maxDepth = 1
	}
	if maxDepth > 10 {
		maxDepth = 10
	}

	if preset, ok := domainSpecialistPresets[domain]; ok && specialists == "planner,researcher,critic,evaluator" {
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
	default:
		systemPrompt = buildSequentialPrompt(specDescs)
	}

	if enableMoE && strategy != "moe" {
		systemPrompt += "\n\nAdditionally, use Mixture-of-Experts routing: when a subtask requires multiple expertise areas, assign it to multiple specialists and merge their results."
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

func buildSpecialistDescriptions(specialistList []string) string {
	var descs []string
	for _, s := range specialistList {
		if desc, ok := allSpecialists[s]; ok {
			descs = append(descs, fmt.Sprintf("- %s", desc))
		}
	}
	return strings.Join(descs, "\n")
}

func buildSequentialPrompt(specDescs string) string {
	return fmt.Sprintf(`You are a supervisor agent. Your job is to:
1. Analyze the given task thoroughly
2. Break it into ordered subtasks
3. Assign each subtask to the most appropriate specialist
4. Define dependencies between subtasks
5. Specify how results should be synthesized

Available specialists:
%s

Strategy: sequential — subtasks execute one after another in order.

Output format (MUST be valid JSON):
{
  "task": "the original task",
  "analysis": "brief analysis of the task requirements",
  "subtasks": [
    {
      "id": 1,
      "description": "what this subtask does",
      "assigned_to": "specialist_name",
      "depends_on": [],
      "input": "what input to pass to this specialist",
      "expected_output": "what output to expect"
    }
  ],
  "synthesis_plan": "how to combine the results from all subtasks",
  "success_criteria": ["list of criteria to determine if the task is complete"],
  "total_subtasks": N
}

Respond with ONLY the JSON object, no extra text, no markdown code blocks.`, specDescs)
}

func buildParallelPrompt(specDescs string) string {
	return fmt.Sprintf(`You are a supervisor agent. Your job is to:
1. Analyze the given task thoroughly
2. Break it into independent subtasks that can run in parallel
3. Assign each subtask to the most appropriate specialist
4. Group subtasks by dependency (parallel groups)
5. Specify how results should be synthesized

Available specialists:
%s

Strategy: parallel — independent subtasks execute simultaneously.

Output format (MUST be valid JSON):
{
  "task": "the original task",
  "analysis": "brief analysis of the task requirements",
  "parallel_groups": [
    {
      "group_id": 1,
      "description": "what this group does",
      "depends_on_groups": [],
      "subtasks": [
        {
          "id": 1,
          "description": "subtask description",
          "assigned_to": "specialist_name",
          "input": "input for this subtask",
          "expected_output": "expected output"
        }
      ]
    }
  ],
  "synthesis_plan": "how to combine the results from all parallel groups",
  "success_criteria": ["list of criteria"]
}

Respond with ONLY the JSON object, no extra text, no markdown code blocks.`, specDescs)
}

func buildHierarchicalPrompt(specDescs string, maxDepth int) string {
	return fmt.Sprintf(`You are a supervisor agent. Your job is to:
1. Analyze the given task at a high level
2. Break it into top-level subtasks
3. For each complex subtask, further decompose it into smaller subtasks
4. Continue decomposition until tasks are simple enough to execute directly
5. Assign each leaf task to the most appropriate specialist
6. Specify how results should be aggregated up the hierarchy

Available specialists:
%s

Strategy: hierarchical — tree-based task decomposition.
Max decomposition depth: %d levels.

Output format (MUST be valid JSON):
{
  "task": "the original task",
  "analysis": "brief analysis of the task requirements",
  "root_tasks": [
    {
      "id": 1,
      "description": "high-level task description",
      "assigned_to": "specialist_name or 'decompose'",
      "subtasks": [
        {
          "id": 1.1,
          "description": "subtask description",
          "assigned_to": "specialist_name",
          "input": "input for this task",
          "expected_output": "expected output",
          "subtasks": []
        }
      ]
    }
  ],
  "aggregation_strategy": "how results flow up the hierarchy",
  "max_depth": %d,
  "total_leaf_tasks": N
}

Respond with ONLY the JSON object, no extra text, no markdown code blocks.`, specDescs, maxDepth, maxDepth)
}

func buildMindSearchPrompt(specDescs string, maxDepth int) string {
	return fmt.Sprintf(`You are a MindSearch-style supervisor agent. Your job is to:
1. Analyze the task and formulate a search/research plan
2. Define the root question and key sub-questions
3. Create a graph of exploration paths (not just linear steps)
4. Assign exploration tasks to researchers and analysts
5. Define convergence criteria for when enough information is gathered
6. Specify how to synthesize the final answer from multiple exploration paths

Available specialists:
%s

Strategy: mindsearch — graph-based exploration with iterative refinement.
Max exploration depth: %d levels.
Inspired by the MindSearch framework with Planner-Searcher architecture.

Output format (MUST be valid JSON):
{
  "task": "the original task/question",
  "root_question": "the core question to answer",
  "initial_hypotheses": ["list of initial hypotheses or angles to explore"],
  "exploration_graph": {
    "nodes": [
      {
        "id": "n1",
        "question": "question to explore",
        "assigned_to": "researcher or data_analyst",
        "status": "pending",
        "parent": null,
        "depth": 1
      }
    ],
    "edges": [
      {"from": "n1", "to": "n2", "relationship": "leads_to"}
    ]
  },
  "exploration_phases": [
    {
      "phase": 1,
      "goal": "what to accomplish in this phase",
      "tasks": ["list of tasks for this phase"],
      "assigned_specialists": ["specialist names"]
    }
  ],
  "convergence_criteria": ["how to know when exploration is complete"],
  "synthesis_method": "how to synthesize the final answer from all exploration paths",
  "max_depth": %d
}

Respond with ONLY the JSON object, no extra text, no markdown code blocks.`, specDescs, maxDepth, maxDepth)
}

func buildMoEPrompt(specDescs string) string {
	return fmt.Sprintf(`You are a Mixture-of-Experts (MoE) supervisor agent. Your job is to:
1. Analyze the task and identify all expertise domains needed
2. Route the task (or sub-tasks) to the most relevant specialists
3. Where multiple specialties are needed, split the task and route to multiple experts
4. Define how expert outputs will be combined (weighted voting, concatenation, synthesis)
5. Handle conflicts between experts with a critic/arbitrator

Available specialists (experts):
%s

Strategy: moe — Mixture-of-Experts routing with dynamic expert selection.
Inspired by ChatLaw's multi-role legal agent and MoE architectures.

Output format (MUST be valid JSON):
{
  "task": "the original task",
  "domain_analysis": {
    "primary_domain": "main expertise area",
    "secondary_domains": ["other relevant domains"],
    "confidence": 0.85
  },
  "expert_routing": [
    {
      "expert": "specialist_name",
      "task_portion": "what this expert handles",
      "weight": 0.6,
      "input": "input to pass to this expert",
      "expected_output": "expected output from this expert"
    }
  ],
  "conflict_resolution": {
    "method": "critic_review or voting or synthesis",
    "arbiter": "critic or evaluator",
    "rules": ["how to resolve disagreements between experts"]
  },
  "synthesis_method": "how to combine outputs from all experts",
  "selected_experts": ["list of all selected expert names"],
  "total_experts": N
}

Respond with ONLY the JSON object, no extra text, no markdown code blocks.`, specDescs)
}

func cleanJSONResp(response string) string {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)
	return response
}
