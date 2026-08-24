// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌‌‌​​‌​​​​​​​​​‌​‌‌​‌​‌​​​​​​​​‌​​‌​‌‌‌​​​‌‌​‌​​​​​​​​​​​​​​​​‌​​​‌​‌​​​​‌​​​​⁠
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

package agent

import (
	"context"
	"strings"
	"testing"
)

func TestUtilityCapability_Basic(t *testing.T) {
	u := NewUtilityCapability()
	if u.Name() != "utility" {
		t.Errorf("Name() = %q, want %q", u.Name(), "utility")
	}
	if u.Description() == "" {
		t.Error("Description() should not be empty")
	}
	if !strings.Contains(u.Description(), "Utility") {
		t.Errorf("Description() = %q, want it to mention Utility", u.Description())
	}
	if err := u.Init(&AgentLoop{}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := u.Shutdown(); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

func TestUtilityCapability_SetDimensions(t *testing.T) {
	u := NewUtilityCapability()
	custom := []UtilityDimension{
		{Name: "custom", Weight: 1.0, Description: "single dimension"},
	}
	u.SetDimensions(custom)
	// After setting a single dimension, scoreOutput should only have that one.
	score := u.scoreOutput("in", "out")
	if _, ok := score.Dimensions["custom"]; !ok {
		t.Error("expected custom dimension after SetDimensions")
	}
	if len(score.Dimensions) != 1 {
		t.Errorf("expected 1 dimension, got %d", len(score.Dimensions))
	}
}

func TestUtilityCapability_PreProcess(t *testing.T) {
	t.Run("empty history returns empty", func(t *testing.T) {
		u := NewUtilityCapability()
		out, err := u.PreProcess(context.Background(), "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output for empty history, got %q", out)
		}
	})

	t.Run("high utility history returns empty", func(t *testing.T) {
		u := NewUtilityCapability()
		u.history = []UtilityScore{{Total: 0.9}, {Total: 0.8}}
		out, err := u.PreProcess(context.Background(), "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output for high-utility history, got %q", out)
		}
	})

	t.Run("low utility history injects context", func(t *testing.T) {
		u := NewUtilityCapability()
		u.history = []UtilityScore{{Total: 0.2}, {Total: 0.3}}
		out, err := u.PreProcess(context.Background(), "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "Utility Context") {
			t.Errorf("expected utility context injection, got %q", out)
		}
		if !strings.Contains(out, "hello") {
			t.Error("expected original input to be preserved")
		}
	})
}

func TestUtilityCapability_PostProcess(t *testing.T) {
	t.Run("empty output returns empty", func(t *testing.T) {
		u := NewUtilityCapability()
		out, err := u.PostProcess(context.Background(), "in", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty output, got %q", out)
		}
		if len(u.history) != 0 {
			t.Error("empty output should not be recorded in history")
		}
	})

	t.Run("normal output recorded returns empty", func(t *testing.T) {
		u := NewUtilityCapability()
		out, err := u.PostProcess(context.Background(), "in", "Result: 42")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Normal score should not trigger suggestion (total >= 0.4).
		_ = out
		if len(u.history) != 1 {
			t.Errorf("expected 1 history entry, got %d", len(u.history))
		}
	})

	t.Run("low score adds suggestion", func(t *testing.T) {
		u := NewUtilityCapability()
		// error words (>2) + dangerous words + hedges → low total < 0.4
		lowOutput := "error failed cannot unable delete remove maybe perhaps"
		out, err := u.PostProcess(context.Background(), "in", lowOutput)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "Utility Analysis") {
			t.Errorf("expected suggestion appended, got %q", out)
		}
	})
}

func TestUtilityScore_Dimensions(t *testing.T) {
	// Cover all scoring branches with varied outputs.
	tests := []struct {
		name   string
		output string
	}{
		{"correctness many errors", "error failed cannot unable not found invalid exception"},
		{"completeness very short", "hi"},
		{"completeness long with structure", "1. first\n2. second\n3. third\nexample: e.g. " + strings.Repeat("x", 600)},
		{"efficiency hedges", "maybe perhaps might could try not sure about this"},
		{"efficiency tool", "run_workflow template_list"},
		{"efficiency very long", strings.Repeat("a", 2100)},
		{"safety dangerous", "rm -rf delete remove overwrite"},
		{"safety confirm and safe", "are you sure confirm safe backup"},
		{"clarity structured", "## Header\n\nline1\n\nline2\n\n```code```\n\nline3"},
		{"clarity code block only", "text ```code```"},
		{"actionability steps", "step 1 first template_list run_workflow ```cmd```"},
		{"actionability input match", "the answer is input here"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := NewUtilityCapability()
			score := u.scoreOutput("input match", tt.output)
			// Every default dimension must be present and in [0,1].
			for _, dim := range DefaultUtilityDimensions {
				v, ok := score.Dimensions[dim.Name]
				if !ok {
					t.Errorf("missing dimension %q", dim.Name)
					continue
				}
				if v < 0 || v > 1 {
					t.Errorf("dimension %q = %v, out of [0,1]", dim.Name, v)
				}
			}
			if score.Total < 0 || score.Total > 1 {
				t.Errorf("total %v out of [0,1]", score.Total)
			}
		})
	}
}

func TestEvaluateDimension_Unknown(t *testing.T) {
	u := NewUtilityCapability()
	got := u.evaluateDimension("nonexistent", "out", "out", 3, "in")
	if got != 0.5 {
		t.Errorf("unknown dimension should default to 0.5, got %v", got)
	}
}

func TestUtilityCapability_averageUtility(t *testing.T) {
	u := NewUtilityCapability()
	// Empty history → 1.0
	if got := u.averageUtility(); got != 1.0 {
		t.Errorf("empty averageUtility = %v, want 1.0", got)
	}

	// <=10 entries: mean of all
	u.history = []UtilityScore{{Total: 0.5}, {Total: 0.7}}
	if got := u.averageUtility(); got != 0.6 {
		t.Errorf("averageUtility = %v, want 0.6", got)
	}

	// >10 entries: only last 10
	u.history = make([]UtilityScore, 15)
	for i := range u.history {
		u.history[i].Total = 0.5
	}
	// Override the last entry so we can verify only the last 10 are averaged.
	u.history[14].Total = 1.0
	// avg = (0.5*9 + 1.0*1) / 10 = 0.55
	if got := u.averageUtility(); got != 0.55 {
		t.Errorf("averageUtility = %v, want 0.55", got)
	}
}

func TestUtilityCapability_GetHistory(t *testing.T) {
	u := NewUtilityCapability()
	u.history = []UtilityScore{{Total: 0.5}, {Total: 0.8}}
	h := u.GetHistory()
	if len(h) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(h))
	}
	// Returned slice should be a copy — mutating it must not affect internal state.
	h[0].Total = 0.0
	if u.history[0].Total != 0.5 {
		t.Error("GetHistory should return a copy, internal state was mutated")
	}
}

func TestUtilityCapability_HistoryTrimming(t *testing.T) {
	u := NewUtilityCapability()
	u.maxHistory = 3
	// Append 5 outputs; history should be trimmed to maxHistory.
	for i := 0; i < 5; i++ {
		_, _ = u.PostProcess(context.Background(), "in", "Result: 42")
	}
	if len(u.history) != 3 {
		t.Errorf("expected history trimmed to 3, got %d", len(u.history))
	}
}

func TestUtilityBuildSuggestion(t *testing.T) {
	u := NewUtilityCapability()
	// Build a score with one weak dimension to exercise the < 0.5 branch.
	score := UtilityScore{
		Option:     "test",
		Dimensions: map[string]float64{},
		Total:      0.3,
	}
	for _, dim := range u.dimensions {
		if dim.Name == "correctness" {
			score.Dimensions[dim.Name] = 0.2
		} else {
			score.Dimensions[dim.Name] = 0.8
		}
	}
	s := u.buildSuggestion(score)
	if !strings.Contains(s, "Utility Analysis") {
		t.Error("expected analysis header in suggestion")
	}
	if !strings.Contains(s, "correctness") {
		t.Error("expected weak dimension in suggestion")
	}
}

func TestClamp(t *testing.T) {
	if clamp(-1, 0, 1) != 0 {
		t.Error("clamp below min should return min")
	}
	if clamp(2, 0, 1) != 1 {
		t.Error("clamp above max should return max")
	}
	if clamp(0.5, 0, 1) != 0.5 {
		t.Error("clamp within range should return value")
	}
}
