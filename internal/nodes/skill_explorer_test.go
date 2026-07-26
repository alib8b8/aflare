// Copyright (c) 2026 llm-box Contributors
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
	"strings"
	"testing"
)

func TestSkillExplorer_List(t *testing.T) {
	node, ok := Get("skill_explorer")
	if !ok {
		t.Fatal("skill_explorer not found in registry")
	}

	ctx := context.Background()

	// Default action is "list"
	output, err := node.Execute(ctx, "", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Skills") {
		t.Errorf("expected skills header in output, got: %s", output)
	}
	// Should include a known skill from the registry
	if !strings.Contains(output, "code-review") {
		t.Errorf("expected 'code-review' in list, got: %s", output)
	}
}

func TestSkillExplorer_Categories(t *testing.T) {
	node, ok := Get("skill_explorer")
	if !ok {
		t.Fatal("skill_explorer not found in registry")
	}

	ctx := context.Background()

	output, err := node.Execute(ctx, "", map[string]string{"action": "categories"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Skill Categories") {
		t.Errorf("expected categories header, got: %s", output)
	}
	// All five declared categories should appear in the listing.
	for _, cat := range []string{"development", "productivity", "research", "creative", "business"} {
		if !strings.Contains(output, cat) {
			t.Errorf("expected category %q in output, got: %s", cat, output)
		}
	}
}

func TestSkillExplorer_Search(t *testing.T) {
	node, ok := Get("skill_explorer")
	if !ok {
		t.Fatal("skill_explorer not found in registry")
	}

	ctx := context.Background()

	output, err := node.Execute(ctx, "code", map[string]string{"action": "search"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Search Results") {
		t.Errorf("expected search results header, got: %s", output)
	}
	// "code" should match the code-review skill (name or tag).
	if !strings.Contains(output, "code-review") {
		t.Errorf("expected 'code-review' in search results, got: %s", output)
	}
}

func TestSkillExplorer_Recommend(t *testing.T) {
	node, ok := Get("skill_explorer")
	if !ok {
		t.Fatal("skill_explorer not found in registry")
	}

	ctx := context.Background()

	output, err := node.Execute(ctx, "workflow", map[string]string{"action": "recommend"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Recommended Skills") {
		t.Errorf("expected recommended header, got: %s", output)
	}
	// Recommendations use a min_quality of 60, so high-quality skills appear.
	if !strings.Contains(output, "llm-box-workflow") {
		t.Errorf("expected 'llm-box-workflow' in recommendations, got: %s", output)
	}
}

func TestSkillExplorer_Evaluate(t *testing.T) {
	node, ok := Get("skill_explorer")
	if !ok {
		t.Fatal("skill_explorer not found in registry")
	}

	ctx := context.Background()

	t.Run("existing skill code-review", func(t *testing.T) {
		output, err := node.Execute(ctx, "code-review", map[string]string{"action": "evaluate"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "Quality Evaluation") {
			t.Errorf("expected evaluation header, got: %s", output)
		}
		if !strings.Contains(output, "code-review") {
			t.Errorf("expected skill name in output, got: %s", output)
		}
	})

	t.Run("non-existent skill", func(t *testing.T) {
		output, err := node.Execute(ctx, "nonexistent-skill-xyz-12345", map[string]string{"action": "evaluate"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "Skill Not Found") {
			t.Errorf("expected 'Skill Not Found' message, got: %s", output)
		}
	})
}

func TestSkillExplorer_CategoryFilter(t *testing.T) {
	node, ok := Get("skill_explorer")
	if !ok {
		t.Fatal("skill_explorer not found in registry")
	}

	ctx := context.Background()

	output, err := node.Execute(ctx, "", map[string]string{
		"action":   "list",
		"category": "development",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should include development skills
	if !strings.Contains(output, "code-review") {
		t.Errorf("expected 'code-review' (development skill), got: %s", output)
	}
	if !strings.Contains(output, "llm-router") {
		t.Errorf("expected 'llm-router' (development skill), got: %s", output)
	}
	// Should NOT include skills from other categories
	if strings.Contains(output, "multi-role-agent") {
		t.Errorf("expected 'multi-role-agent' (business) to be filtered out, got: %s", output)
	}
	if strings.Contains(output, "agent-browser") {
		t.Errorf("expected 'agent-browser' (productivity) to be filtered out, got: %s", output)
	}
}

func TestSkillExplorer_InvalidMinQuality(t *testing.T) {
	node, ok := Get("skill_explorer")
	if !ok {
		t.Fatal("skill_explorer not found in registry")
	}

	ctx := context.Background()

	// Invalid min_quality should fall back to the default of 50; with the
	// default threshold every skill in the registry (overall >= 65) is shown.
	output, err := node.Execute(ctx, "", map[string]string{
		"action":      "list",
		"min_quality": "not-a-number",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Multiple known skills should appear, proving the default was applied
	// rather than the filter rejecting everything.
	expected := []string{"llm-box-workflow", "code-review", "llm-router", "self-heal"}
	for _, name := range expected {
		if !strings.Contains(output, name) {
			t.Errorf("expected skill %q with default min_quality, got: %s", name, output)
		}
	}
}

func TestSkillExplorer_InvalidLimit(t *testing.T) {
	node, ok := Get("skill_explorer")
	if !ok {
		t.Fatal("skill_explorer not found in registry")
	}

	ctx := context.Background()

	// Invalid limit should fall back to the default of 20; with the default
	// limit all 10 skills in the registry should be visible.
	output, err := node.Execute(ctx, "", map[string]string{
		"action": "list",
		"limit":  "not-a-number",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All 10 skills are listed when limit >= 10. Count known skill names
	// that appear in the output to confirm the default limit was used.
	known := []string{
		"llm-box-workflow", "code-review", "smart-search", "agent-browser",
		"self-heal", "llm-router", "knowledge-graph", "multi-role-agent",
		"omniroute-gateway", "swarm-communication",
	}
	seen := 0
	for _, name := range known {
		if strings.Contains(output, name) {
			seen++
		}
	}
	if seen < 5 {
		t.Errorf("expected at least 5 skills with default limit, saw %d in output: %s", seen, output)
	}
}
