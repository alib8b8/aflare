// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
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
	"sort"
	"strings"
)

type SkillQualityScore struct {
	Name          string
	Description   string
	Category      string
	Completeness  float64
	Documentation float64
	Usability     float64
	Overall       float64
	Tags          []string
}

type SkillExplorerNode struct{}

func init() {
	Register(&SkillExplorerNode{})
}

func (n *SkillExplorerNode) Name() string {
	return "skill_explorer"
}

func (n *SkillExplorerNode) Description() string {
	return "Skill ecosystem explorer: discover, evaluate, and recommend skills (awesome-claude-skills inspired)"
}

func (n *SkillExplorerNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "skill_explorer",
		Description: "Discover, evaluate, and recommend skills from the ecosystem. Quality scoring, category browsing, and smart recommendations. Inspired by awesome-claude-skills.",
		Input:       "string - search query for skills or empty to list all",
		Output:      "string - skills listing with quality scores and recommendations",
		Params: []ParamSchema{
			{Name: "action", Type: "string", Description: "Action: list|search|recommend|evaluate|categories (default: list)", Required: false, Default: "list"},
			{Name: "category", Type: "string", Description: "Filter by category: development,productivity,research,creative,business,all (default: all)", Required: false, Default: "all"},
			{Name: "min_quality", Type: "string", Description: "Minimum quality score 0-100 (default: 50)", Required: false, Default: "50"},
			{Name: "limit", Type: "string", Description: "Maximum results (default: 20)", Required: false, Default: "20"},
			{Name: "sort_by", Type: "string", Description: "Sort by: quality|name|category (default: quality)", Required: false, Default: "quality"},
		},
	}
}

var skillRegistry = []SkillQualityScore{
	{Name: "aflare-workflow", Description: "Build and execute AI workflows with multi-step nodes", Category: "development", Tags: []string{"workflow", "automation", "pipeline"}, Completeness: 95, Documentation: 90, Usability: 85, Overall: 90},
	{Name: "code-review", Description: "Hybrid architecture code review with deterministic rules + LLM", Category: "development", Tags: []string{"code-review", "security", "quality"}, Completeness: 90, Documentation: 80, Usability: 85, Overall: 85},
	{Name: "smart-search", Description: "Multi-source search aggregator with 20+ information sources", Category: "research", Tags: []string{"search", "intelligence", "news"}, Completeness: 88, Documentation: 75, Usability: 80, Overall: 81},
	{Name: "agent-browser", Description: "Agent-optimized web browser for autonomous navigation", Category: "productivity", Tags: []string{"browser", "scraping", "automation"}, Completeness: 80, Documentation: 70, Usability: 75, Overall: 75},
	{Name: "self-heal", Description: "Automatic code formatting, dependency management, and build fixing", Category: "development", Tags: []string{"automation", "build", "quality"}, Completeness: 85, Documentation: 75, Usability: 80, Overall: 80},
	{Name: "llm-router", Description: "Multi-model intelligent router with fallback and cost optimization", Category: "development", Tags: []string{"llm", "router", "fallback"}, Completeness: 92, Documentation: 85, Usability: 88, Overall: 88},
	{Name: "knowledge-graph", Description: "Build and query knowledge graphs from unstructured text", Category: "research", Tags: []string{"knowledge", "graph", "rag"}, Completeness: 75, Documentation: 65, Usability: 70, Overall: 70},
	{Name: "multi-role-agent", Description: "Integrated marketing, BD, IR, and research agent roles", Category: "business", Tags: []string{"agent", "marketing", "business"}, Completeness: 70, Documentation: 60, Usability: 65, Overall: 65},
	{Name: "omniroute-gateway", Description: "Unified AI gateway to 290+ providers and 500+ models", Category: "development", Tags: []string{"gateway", "api", "providers"}, Completeness: 85, Documentation: 70, Usability: 75, Overall: 77},
	{Name: "swarm-communication", Description: "Multi-agent swarm communication with Nostr-style protocol", Category: "development", Tags: []string{"agent", "swarm", "communication"}, Completeness: 72, Documentation: 65, Usability: 68, Overall: 68},
}

var skillCategories = []string{"development", "productivity", "research", "creative", "business"}

func (n *SkillExplorerNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	action := getParam(params, "action", "list")
	category := getParam(params, "category", "all")
	minQuality := paramFloat(params, "min_quality", 50.0, 0, 100)
	limit := paramInt(params, "limit", 20, 1, 1000)
	sortBy := getParam(params, "sort_by", "quality")

	switch action {
	case "categories":
		return n.listCategories(), nil
	case "search":
		return n.searchSkills(input, category, minQuality, limit, sortBy), nil
	case "recommend":
		return n.recommendSkills(input, category, limit), nil
	case "evaluate":
		return n.evaluateSkill(input), nil
	case "list":
		fallthrough
	default:
		return n.listSkills(category, minQuality, limit, sortBy), nil
	}
}

func (n *SkillExplorerNode) listCategories() string {
	var sb strings.Builder
	sb.WriteString("## 🏷️ Skill Categories\n\n")
	for _, cat := range skillCategories {
		count := 0
		for _, s := range skillRegistry {
			if s.Category == cat {
				count++
			}
		}
		sb.WriteString(fmt.Sprintf("- **%s** (%d skills)\n", cat, count))
	}
	sb.WriteString(fmt.Sprintf("\nTotal: %d skills in %d categories\n", len(skillRegistry), len(skillCategories)))
	return sb.String()
}

func (n *SkillExplorerNode) listSkills(category string, minQuality float64, limit int, sortBy string) string {
	filtered := n.filterSkills(category, minQuality)
	sorted := n.sortSkills(filtered, sortBy)

	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 🛠️ Skills (%d shown, %d total)\n\n", len(sorted), len(filtered)))

	for i, s := range sorted {
		sb.WriteString(fmt.Sprintf("%d. **%s** - %s\n", i+1, s.Name, s.Description))
		sb.WriteString(fmt.Sprintf("   Category: `%s` | Quality: ⭐ %.0f/100\n", s.Category, s.Overall))
		if len(s.Tags) > 0 {
			tagStrs := make([]string, len(s.Tags))
			for j, t := range s.Tags {
				tagStrs[j] = "`" + t + "`"
			}
			sb.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(tagStrs, " ")))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (n *SkillExplorerNode) searchSkills(query, category string, minQuality float64, limit int, sortBy string) string {
	query = strings.ToLower(strings.TrimSpace(query))
	filtered := n.filterSkills(category, minQuality)

	var results []SkillQualityScore
	for _, s := range filtered {
		if query == "" ||
			strings.Contains(strings.ToLower(s.Name), query) ||
			strings.Contains(strings.ToLower(s.Description), query) {
			hasTag := false
			for _, t := range s.Tags {
				if strings.Contains(strings.ToLower(t), query) {
					hasTag = true
					break
				}
			}
			if !hasTag && query != "" {
				continue
			}
			results = append(results, s)
		}
	}

	sorted := n.sortSkills(results, sortBy)
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 🔍 Search Results for \"%s\" (%d found)\n\n", query, len(sorted)))

	for i, s := range sorted {
		sb.WriteString(fmt.Sprintf("%d. **%s** ⭐%.0f\n", i+1, s.Name, s.Overall))
		sb.WriteString(fmt.Sprintf("   %s\n\n", s.Description))
	}
	return sb.String()
}

func (n *SkillExplorerNode) recommendSkills(query, category string, limit int) string {
	filtered := n.filterSkills(category, 60)
	sorted := n.sortSkills(filtered, "quality")

	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	var sb strings.Builder
	sb.WriteString("## 🎯 Recommended Skills\n\n")

	if query != "" {
		sb.WriteString(fmt.Sprintf("Based on your interest in: **%s**\n\n", query))
	}

	for i, s := range sorted {
		sb.WriteString(fmt.Sprintf("%d. ⭐ **%s** (%.0f/100)\n", i+1, s.Name, s.Overall))
		sb.WriteString(fmt.Sprintf("   _%s_\n\n", s.Description))
	}
	return sb.String()
}

func (n *SkillExplorerNode) evaluateSkill(skillName string) string {
	skillName = strings.ToLower(strings.TrimSpace(skillName))
	var found *SkillQualityScore
	for i := range skillRegistry {
		if strings.ToLower(skillRegistry[i].Name) == skillName {
			found = &skillRegistry[i]
			break
		}
	}

	if found == nil {
		return fmt.Sprintf("## ❌ Skill Not Found\n\nNo skill named \"%s\" in registry. Use `action=list` to see all skills.", skillName)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 📊 Quality Evaluation: %s\n\n", found.Name))
	sb.WriteString(fmt.Sprintf("**Description**: %s\n\n", found.Description))
	sb.WriteString(fmt.Sprintf("**Category**: `%s`\n\n", found.Category))
	sb.WriteString("### Scores\n\n")
	sb.WriteString(fmt.Sprintf("- **Completeness**: %.0f/100 %s\n", found.Completeness, renderBar(found.Completeness)))
	sb.WriteString(fmt.Sprintf("- **Documentation**: %.0f/100 %s\n", found.Documentation, renderBar(found.Documentation)))
	sb.WriteString(fmt.Sprintf("- **Usability**: %.0f/100 %s\n", found.Usability, renderBar(found.Usability)))
	sb.WriteString(fmt.Sprintf("\n### **Overall: %.0f/100** %s\n", found.Overall, renderBar(found.Overall)))

	if len(found.Tags) > 0 {
		sb.WriteString("\n**Tags**: ")
		for _, t := range found.Tags {
			sb.WriteString("`" + t + "` ")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (n *SkillExplorerNode) filterSkills(category string, minQuality float64) []SkillQualityScore {
	var result []SkillQualityScore
	for _, s := range skillRegistry {
		if category != "all" && s.Category != category {
			continue
		}
		if s.Overall < minQuality {
			continue
		}
		result = append(result, s)
	}
	return result
}

func (n *SkillExplorerNode) sortSkills(skills []SkillQualityScore, sortBy string) []SkillQualityScore {
	result := make([]SkillQualityScore, len(skills))
	copy(result, skills)

	switch sortBy {
	case "name":
		sort.Slice(result, func(i, j int) bool {
			return result[i].Name < result[j].Name
		})
	case "category":
		sort.Slice(result, func(i, j int) bool {
			if result[i].Category != result[j].Category {
				return result[i].Category < result[j].Category
			}
			return result[i].Overall > result[j].Overall
		})
	case "quality":
		fallthrough
	default:
		sort.Slice(result, func(i, j int) bool {
			return result[i].Overall > result[j].Overall
		})
	}
	return result
}

func renderBar(score float64) string {
	filled := int(score / 10)
	bar := ""
	for i := 0; i < 10; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	return bar
}
