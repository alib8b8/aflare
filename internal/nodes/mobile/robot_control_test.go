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

package mobile

import (
	"context"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

func TestRobotControlNode_Metadata(t *testing.T) {
	node := &RobotControlNode{}
	if node.Name() != "robot_control" {
		t.Errorf("name: %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("empty description")
	}
	schema := node.Schema()
	if schema.Name != "robot_control" {
		t.Errorf("schema name: %s", schema.Name)
	}
	if len(schema.Params) == 0 {
		t.Error("expected params")
	}
}

func TestRobotControlNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &RobotControlNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"invalid robot_type", map[string]string{"robot_type": "spaceship"}, "invalid robot_type"},
		{"invalid robot_id format", map[string]string{"robot_id": "bad id!"}, "invalid robot_id format"},
		{"invalid action", map[string]string{"action": "fly"}, "invalid action"},
		{"target_object too long", map[string]string{"target_object": strings.Repeat("a", 201)}, "target_object too long"},
		{"target_location too long", map[string]string{"target_location": strings.Repeat("a", 201)}, "target_location too long"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "", tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestRobotControlNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &RobotControlNode{}

	// Direct action
	out, err := node.Execute(ctx, "pick up the cup", map[string]string{
		"robot_type":       "humanoid",
		"robot_id":         "robot_001",
		"action":           "pick",
		"target_object":    "cup",
		"speed":            "0.7",
		"force_limit":      "5.0",
		"safety_zone_m":    "0.5",
		"visual_feedback":  "true",
		"tactile_feedback": "true",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "robot_control") {
		t.Errorf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "robot_001") {
		t.Error("expected robot_id in output")
	}
}

func TestRobotControlNode_ExecuteTaskPlan(t *testing.T) {
	ctx := context.Background()
	node := &RobotControlNode{}

	tests := []struct {
		name      string
		input     string
		wantSteps int
	}{
		{"pick task", "pick up the cup", 4},
		{"place task", "place the cup on the table", 4},
		{"water task", "get me some water", 6},
		{"default task", "do something else", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := node.Execute(ctx, tt.input, map[string]string{"robot_type": "humanoid"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, "robot_control") {
				t.Errorf("unexpected output: %s", out)
			}
		})
	}
}

func TestRobotControlNode_ExecuteParamClamping(t *testing.T) {
	ctx := context.Background()
	node := &RobotControlNode{}

	// Out-of-range values get clamped to defaults, not errors
	out, err := node.Execute(ctx, "do task", map[string]string{
		"robot_type":    "humanoid",
		"speed":         "5.0", // > 1.0, falls back to 0.5
		"force_limit":   "200", // > 100, falls back to 10.0
		"safety_zone_m": "10",  // > 5.0, falls back to 0.5
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "robot_control") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestRobotControlNode_ExecuteDefaultsRobotID(t *testing.T) {
	ctx := context.Background()
	node := &RobotControlNode{}

	out, err := node.Execute(ctx, "do task", map[string]string{"robot_type": "humanoid"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "robot_") {
		t.Error("expected default robot_id starting with robot_")
	}
}

func TestGenerateTaskPlan(t *testing.T) {
	tests := []struct {
		name      string
		task      string
		wantSteps int
	}{
		{"pick zh", "拿杯子", 4},
		{"pick en", "pick up the cup", 4},
		{"取 zh", "取杯子", 4},
		{"place zh", "放下东西", 4},
		{"place en", "place the object", 4},
		{"water zh", "倒水", 6},
		{"water en", "water the plant", 6},
		{"default", "do something", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := generateTaskPlan(tt.task, "humanoid", 0.5, 10.0, 0.5)
			if plan == nil {
				t.Fatal("expected non-nil plan")
			}
			if len(plan.Actions) != tt.wantSteps {
				t.Errorf("got %d steps, want %d", len(plan.Actions), tt.wantSteps)
			}
			if plan.TotalSteps != tt.wantSteps {
				t.Errorf("TotalSteps: got %d, want %d", plan.TotalSteps, tt.wantSteps)
			}
			if plan.EstimatedMs <= 0 {
				t.Error("expected positive EstimatedMs")
			}
			if plan.FallbackPlan == "" {
				t.Error("expected non-empty fallback plan")
			}
		})
	}
}

func TestGenerateDirectActionPlan(t *testing.T) {
	plan := generateDirectActionPlan("move", "box", "shelf", 0.5, 10.0)
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if len(plan.Actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(plan.Actions))
	}
	if plan.Actions[0].Type != "move" {
		t.Errorf("action type: got %q, want move", plan.Actions[0].Type)
	}
	if !plan.Actions[0].RequiresVision {
		t.Error("expected RequiresVision=true")
	}
	if plan.TotalSteps != 1 {
		t.Errorf("TotalSteps: got %d, want 1", plan.TotalSteps)
	}
	if plan.EstimatedMs != 2000 {
		t.Errorf("EstimatedMs: got %d, want 2000", plan.EstimatedMs)
	}
}

func TestRunSafetyChecks(t *testing.T) {
	tests := []struct {
		name           string
		plan           *RobotActionPlan
		safetyZone     float64
		visual         bool
		tactile        bool
		wantPassed     bool
		wantViolations int
		wantWarnings   int
	}{
		{
			"all clear",
			&RobotActionPlan{Actions: []RobotAction{{ID: "1", Parameters: map[string]interface{}{"force_limit": 10.0}, RequiresVision: true, RequiresTactile: true}}},
			0.5, true, true,
			true, 0, 0,
		},
		{
			"force violation",
			&RobotActionPlan{Actions: []RobotAction{{ID: "1", Parameters: map[string]interface{}{"force_limit": 60.0}}}},
			0.5, true, true,
			false, 1, 0,
		},
		{
			"force warning",
			&RobotActionPlan{Actions: []RobotAction{{ID: "1", Parameters: map[string]interface{}{"force_limit": 25.0}}}},
			0.5, true, true,
			true, 0, 1,
		},
		{
			"vision warning",
			&RobotActionPlan{Actions: []RobotAction{{ID: "1", Parameters: map[string]interface{}{}, RequiresVision: true}}},
			0.5, false, true,
			true, 0, 1,
		},
		{
			"tactile warning",
			&RobotActionPlan{Actions: []RobotAction{{ID: "1", Parameters: map[string]interface{}{}, RequiresTactile: true}}},
			0.5, true, false,
			true, 0, 1,
		},
		{
			"small safety zone warning",
			&RobotActionPlan{Actions: []RobotAction{{ID: "1", Parameters: map[string]interface{}{}}}},
			0.2, true, true,
			true, 0, 1,
		},
		{
			"mitigations added when warnings present",
			&RobotActionPlan{Actions: []RobotAction{{ID: "1", Parameters: map[string]interface{}{}, RequiresVision: true}}},
			0.5, false, true,
			true, 0, 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runSafetyChecks(tt.plan, tt.safetyZone, tt.visual, tt.tactile)
			if result.Passed != tt.wantPassed {
				t.Errorf("Passed: got %v, want %v", result.Passed, tt.wantPassed)
			}
			if len(result.Violations) != tt.wantViolations {
				t.Errorf("Violations: got %d, want %d", len(result.Violations), tt.wantViolations)
			}
			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("Warnings: got %d, want %d", len(result.Warnings), tt.wantWarnings)
			}
			// If there are warnings and passed, mitigations should be present
			if len(result.Warnings) > 0 && result.Passed {
				if len(result.Mitigations) == 0 {
					t.Error("expected mitigations when warnings exist and plan passed")
				}
			}
		})
	}
}

// Ensure robot_control node was registered.
func TestRobotControlNode_Registered(t *testing.T) {
	if _, ok := core.Get("robot_control"); !ok {
		t.Error("robot_control not registered")
	}
}
