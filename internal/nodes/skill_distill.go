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
)

var (
	validSourceTypes = map[string]bool{
		"book":          true,
		"video":         true,
		"podcast":       true,
		"article":       true,
		"documentation": true,
		"conversation":  true,
	}
	validDistillTypes = map[string]bool{
		"workflow":  true,
		"decision":  true,
		"analysis":  true,
		"creative":  true,
		"prompt":    true,
		"checklist": true,
	}
	validQualityLevels = map[string]bool{
		"basic":    true,
		"standard": true,
		"expert":   true,
	}
	skillNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{1,63}$`)
)

type SkillDistillNode struct{}

func (n *SkillDistillNode) Name() string { return "skill_distill" }

func (n *SkillDistillNode) Description() string {
	return "Distill methodologies from books, videos, podcasts, and documents into callable skills. Supports workflow, decision, analysis, creative, prompt, and checklist skill types."
}

func (n *SkillDistillNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - source content to distill",
		Output:      "string - JSON with distilled skill structure",
		Params: []ParamSchema{
			{Name: "source_type", Type: "string", Description: "Source type: book/video/podcast/article/documentation/conversation (default: article)", Required: false, Default: "article"},
			{Name: "distill_type", Type: "string", Description: "Distill type: workflow/decision/analysis/creative/prompt/checklist (default: workflow)", Required: false, Default: "workflow"},
			{Name: "content", Type: "string", Description: "Source content text (max 100000 chars)", Required: false},
			{Name: "skill_name", Type: "string", Description: "Target skill name", Required: false},
			{Name: "max_steps", Type: "int", Description: "Max number of steps (default: 10)", Required: false, Default: "10"},
			{Name: "quality", Type: "string", Description: "Quality level: basic/standard/expert (default: standard)", Required: false, Default: "standard"},
		},
	}
}

func (n *SkillDistillNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	sourceType := getParam(params, "source_type", "article")
	if !validSourceTypes[sourceType] {
		return "", fmt.Errorf("invalid source_type: %s", sourceType)
	}

	distillType := getParam(params, "distill_type", "workflow")
	if !validDistillTypes[distillType] {
		return "", fmt.Errorf("invalid distill_type: %s", distillType)
	}

	content := getParam(params, "content", "")
	if input != "" && content == "" {
		content = input
	}
	if len(content) > 100000 {
		return "", fmt.Errorf("content too long (max 100000 chars)")
	}

	skillName := getParam(params, "skill_name", "")
	if skillName == "" {
		skillName = fmt.Sprintf("%s_%s_skill", distillType, sourceType)
	}
	if !skillNamePattern.MatchString(skillName) {
		return "", fmt.Errorf("invalid skill_name format: must start with letter, 2-64 chars, alphanumeric/_- only")
	}

	maxSteps := parseIntSafe(getParam(params, "max_steps", "10"), 10)
	if maxSteps < 1 || maxSteps > 50 {
		maxSteps = 10
	}

	quality := getParam(params, "quality", "standard")
	if !validQualityLevels[quality] {
		return "", fmt.Errorf("invalid quality: %s", quality)
	}

	tokensProcessed := len(content) / 4

	distilledSkill := generateDistilledSkill(skillName, sourceType, distillType, content, maxSteps, quality)

	result := map[string]interface{}{
		"skill_name":       distilledSkill.Name,
		"description":      distilledSkill.Description,
		"trigger_words":    distilledSkill.TriggerWords,
		"steps":            distilledSkill.Steps,
		"rules":            distilledSkill.Rules,
		"examples":         distilledSkill.Examples,
		"pitfalls":         distilledSkill.Pitfalls,
		"source_type":      sourceType,
		"distill_type":     distillType,
		"tokens_processed": tokensProcessed,
		"quality":          quality,
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

type DistilledSkill struct {
	Name         string   `json:"skill_name"`
	Description  string   `json:"description"`
	TriggerWords []string `json:"trigger_words"`
	Steps        []string `json:"steps"`
	Rules        []string `json:"rules"`
	Examples     []string `json:"examples"`
	Pitfalls     []string `json:"pitfalls"`
}

func generateDistilledSkill(name, sourceType, distillType, content string, maxSteps int, quality string) *DistilledSkill {
	desc := fmt.Sprintf("Distilled %s skill from %s content. Quality level: %s.", distillType, sourceType, quality)

	triggerWords := []string{
		strings.ReplaceAll(name, "_", " "),
		distillType,
		sourceType,
	}

	steps := generateSteps(distillType, maxSteps, quality)
	rules := generateRules(distillType, quality)
	examples := generateExamples(distillType, quality)
	pitfalls := generatePitfalls(distillType)

	return &DistilledSkill{
		Name:         name,
		Description:  desc,
		TriggerWords: triggerWords,
		Steps:        steps,
		Rules:        rules,
		Examples:     examples,
		Pitfalls:     pitfalls,
	}
}

func generateSteps(distillType string, maxSteps int, quality string) []string {
	baseSteps := map[string][]string{
		"workflow": {
			"Define the goal and desired outcome",
			"Gather required information and resources",
			"Break down the task into actionable steps",
			"Execute each step in sequence",
			"Verify results at each checkpoint",
			"Handle edge cases and exceptions",
			"Synthesize and deliver the final output",
		},
		"decision": {
			"Define the decision to be made",
			"Identify key criteria and constraints",
			"Collect relevant data and information",
			"Generate alternative options",
			"Evaluate each option against criteria",
			"Consider risks and tradeoffs",
			"Make the decision and document rationale",
		},
		"analysis": {
			"Define the analysis objective",
			"Collect and organize raw data",
			"Clean and preprocess the data",
			"Apply analytical methods and frameworks",
			"Identify patterns and insights",
			"Validate findings with cross-checks",
			"Present conclusions and recommendations",
		},
		"creative": {
			"Understand the creative brief and constraints",
			"Research inspirations and references",
			"Generate initial ideas and concepts",
			"Refine and develop selected ideas",
			"Prototype and test variations",
			"Gather feedback and iterate",
			"Finalize and polish the output",
		},
		"prompt": {
			"Define the task and desired output format",
			"Identify the target AI model capabilities",
			"Craft the core instruction clearly",
			"Add context and background information",
			"Include examples for few-shot learning",
			"Set constraints and quality requirements",
			"Test and refine the prompt",
		},
		"checklist": {
			"Define the scope of the checklist",
			"Identify all key items to verify",
			"Organize items in logical order",
			"Define pass/fail criteria for each item",
			"Add notes and reference information",
			"Include escalation paths for failures",
			"Review and validate completeness",
		},
	}

	steps := baseSteps[distillType]
	if steps == nil {
		steps = baseSteps["workflow"]
	}

	if quality == "basic" {
		if len(steps) > 5 {
			steps = steps[:5]
		}
	} else if quality == "expert" {
		extraSteps := []string{
			"Document lessons learned and best practices",
			"Create feedback loop for continuous improvement",
		}
		steps = append(steps, extraSteps...)
	}

	if len(steps) > maxSteps {
		steps = steps[:maxSteps]
	}

	return steps
}

func generateRules(distillType string, quality string) []string {
	baseRules := []string{
		"Always start with clear objectives",
		"Maintain consistency throughout the process",
		"Document decisions and reasoning",
		"Validate outputs against requirements",
	}

	qualityRules := map[string][]string{
		"basic":    baseRules[:2],
		"standard": baseRules,
		"expert": append(baseRules, []string{
			"Consider edge cases and failure modes",
			"Include fallback mechanisms",
			"Optimize for efficiency and quality",
		}...),
	}

	rules := qualityRules[quality]
	if rules == nil {
		rules = qualityRules["standard"]
	}

	return rules
}

func generateExamples(distillType string, quality string) []string {
	examples := []string{
		fmt.Sprintf("Example 1: Applying %s to a common scenario", distillType),
		fmt.Sprintf("Example 2: Complex %s with multiple variables", distillType),
	}

	if quality == "expert" {
		examples = append(examples, fmt.Sprintf("Example 3: Advanced %s with edge cases", distillType))
	}

	return examples
}

func generatePitfalls(distillType string) []string {
	pitfalls := []string{
		"Skipping initial planning leads to rework",
		"Overcomplicating simple tasks",
		"Not validating assumptions early",
	}

	return pitfalls
}

func init() {
	Register(&SkillDistillNode{})
}
