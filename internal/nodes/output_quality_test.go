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

func TestOutputQuality_Analyze(t *testing.T) {
	node, ok := Get("output_quality")
	if !ok {
		t.Fatal("output_quality not found in registry")
	}

	ctx := context.Background()

	tests := []struct {
		name     string
		input    string
		params   map[string]string
		wantErr  bool
		contains string
	}{
		{
			name:     "AI-flavor Chinese text",
			input:    "首先，我需要说明。其次，做法如下。最后，希望对你有帮助！🚀✨🎯✅",
			params:   map[string]string{"action": "analyze"},
			contains: "Detected AI Traces",
		},
		{
			name:     "natural human English text",
			input:    "I went to the store today. Bought milk and eggs. The cat was sleeping when I returned home.",
			params:   map[string]string{"action": "analyze", "lang": "en"},
			contains: "No significant AI traces detected",
		},
		{
			name:     "analyze with brief detail",
			input:    "首先，我需要说明。希望对你有帮助！",
			params:   map[string]string{"action": "analyze", "detail": "brief"},
			contains: "Quality Report",
		},
		{
			name:     "default action is analyze",
			input:    "I went to the store today. Bought milk and eggs.",
			params:   map[string]string{"lang": "en"},
			contains: "Quality Report",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := node.Execute(ctx, tt.input, tt.params)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(output, tt.contains) {
				t.Errorf("expected output to contain %q, got: %s", tt.contains, output)
			}
		})
	}
}

func TestOutputQuality_Gate(t *testing.T) {
	node, ok := Get("output_quality")
	if !ok {
		t.Fatal("output_quality not found in registry")
	}

	ctx := context.Background()

	humanText := "I went to the store today. Bought milk and eggs. The cat was sleeping when I returned home."
	aiText := "首先，我需要说明。其次，做法如下。最后，希望对你有帮助！🚀✨🎯✅"

	tests := []struct {
		name     string
		input    string
		params   map[string]string
		contains string
	}{
		{
			name:     "pass with low min_score",
			input:    humanText,
			params:   map[string]string{"action": "gate", "min_score": "10", "lang": "en"},
			contains: "PASSED",
		},
		{
			name:     "fail with high min_score",
			input:    aiText,
			params:   map[string]string{"action": "gate", "min_score": "95"},
			contains: "FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := node.Execute(ctx, tt.input, tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(output, tt.contains) {
				t.Errorf("expected output to contain %q, got: %s", tt.contains, output)
			}
		})
	}
}

func TestOutputQuality_Suggest(t *testing.T) {
	node, ok := Get("output_quality")
	if !ok {
		t.Fatal("output_quality not found in registry")
	}

	ctx := context.Background()

	t.Run("AI text produces suggestions", func(t *testing.T) {
		input := "首先，我需要说明。希望对你有帮助！"
		output, err := node.Execute(ctx, input, map[string]string{"action": "suggest"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "Rewrite Suggestions") {
			t.Errorf("expected 'Rewrite Suggestions' header, got: %s", output)
		}
	})

	t.Run("natural text produces no suggestions", func(t *testing.T) {
		input := "I went to the store today. Bought milk and eggs."
		output, err := node.Execute(ctx, input, map[string]string{"action": "suggest", "lang": "en"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "No rewrite suggestions needed") {
			t.Errorf("expected no-suggestions message, got: %s", output)
		}
	})
}

func TestOutputQuality_Checklist(t *testing.T) {
	node, ok := Get("output_quality")
	if !ok {
		t.Fatal("output_quality not found in registry")
	}

	ctx := context.Background()

	output, err := node.Execute(ctx, "any input", map[string]string{"action": "checklist"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Anti-AI-Flavor Writing Checklist") {
		t.Errorf("expected checklist header, got: %s", output)
	}
	for _, category := range []string{"Template Phrases", "Emoji Usage", "Structure Variety", "Concreteness"} {
		if !strings.Contains(output, category) {
			t.Errorf("expected category %q in checklist, got: %s", category, output)
		}
	}
}

func TestOutputQuality_EmptyInput(t *testing.T) {
	node, ok := Get("output_quality")
	if !ok {
		t.Fatal("output_quality not found in registry")
	}

	ctx := context.Background()

	t.Run("empty input returns error", func(t *testing.T) {
		_, err := node.Execute(ctx, "", map[string]string{"action": "analyze"})
		if err == nil {
			t.Error("expected error for empty input")
		}
	})

	t.Run("whitespace-only input returns error", func(t *testing.T) {
		_, err := node.Execute(ctx, "   \n\t  ", map[string]string{"action": "analyze"})
		if err == nil {
			t.Error("expected error for whitespace-only input")
		}
	})

	t.Run("empty input fails for every action", func(t *testing.T) {
		for _, action := range []string{"analyze", "gate", "suggest"} {
			_, err := node.Execute(ctx, "", map[string]string{"action": action})
			if err == nil {
				t.Errorf("expected error for action %q with empty input", action)
			}
		}
	})

	t.Run("checklist also rejects empty input", func(t *testing.T) {
		// The empty-input guard runs before the action switch, so every
		// action (including checklist) rejects empty input.
		_, err := node.Execute(ctx, "", map[string]string{"action": "checklist"})
		if err == nil {
			t.Error("expected error for checklist with empty input")
		}
	})
}

func TestOutputQuality_InvalidMinScore(t *testing.T) {
	node, ok := Get("output_quality")
	if !ok {
		t.Fatal("output_quality not found in registry")
	}

	ctx := context.Background()

	// Invalid min_score should fall back to the default of 60.0
	output, err := node.Execute(ctx, "some text input here", map[string]string{
		"action":    "gate",
		"min_score": "not-a-number",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The gate output prints the min_score threshold, so 60.0 must appear.
	if !strings.Contains(output, "60.0") {
		t.Errorf("expected default min_score 60.0 in output, got: %s", output)
	}
}

func TestOutputQuality_MinScoreClamping(t *testing.T) {
	node, ok := Get("output_quality")
	if !ok {
		t.Fatal("output_quality not found in registry")
	}

	ctx := context.Background()

	text := "I went to the store today. Bought milk and eggs."

	t.Run("negative clamped to 0", func(t *testing.T) {
		output, err := node.Execute(ctx, text, map[string]string{
			"action":    "gate",
			"min_score": "-50",
			"lang":      "en",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Clamped to 0; overall is always >= 0 so should pass.
		if !strings.Contains(output, "PASSED") {
			t.Errorf("expected PASSED with min_score clamped to 0, got: %s", output)
		}
		if !strings.Contains(output, "0.0") {
			t.Errorf("expected clamped min_score 0.0 in output, got: %s", output)
		}
	})

	t.Run("value over 100 clamped to 100", func(t *testing.T) {
		output, err := node.Execute(ctx, text, map[string]string{
			"action":    "gate",
			"min_score": "200",
			"lang":      "en",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Clamped to 100; the threshold displayed in the output must be 100.0.
		if !strings.Contains(output, "100.0") {
			t.Errorf("expected clamped min_score 100.0 in output, got: %s", output)
		}
	})
}
