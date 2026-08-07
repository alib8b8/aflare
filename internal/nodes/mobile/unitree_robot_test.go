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

package mobile

import (
	"context"
	"strings"
	"testing"

	"github.com/alib8b8/llm-box/internal/nodes/core"
)

func TestUnitreeRobotNode_Metadata(t *testing.T) {
	node := &UnitreeRobotNode{}
	if node.Name() != "unitree_robot" {
		t.Errorf("name: %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("empty description")
	}
	schema := node.Schema()
	if schema.Name != "unitree_robot" {
		t.Errorf("schema name: %s", schema.Name)
	}
	if len(schema.Params) == 0 {
		t.Error("expected params")
	}
}

func TestUnitreeRobotNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &UnitreeRobotNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"invalid robot_model", map[string]string{"robot_model": "unknown"}, "invalid robot_model"},
		{"invalid robot_id format", map[string]string{"robot_id": "bad id!"}, "invalid robot_id format"},
		{"invalid mode", map[string]string{"mode": "direct"}, "invalid mode"},
		{"invalid action", map[string]string{"action": "fly"}, "invalid action"},
		{"target_location too long", map[string]string{"target_location": strings.Repeat("a", 201)}, "target_location too long"},
		{"api mode without robot_ip", map[string]string{"mode": "api"}, "robot_ip is required when mode=api"},
		{"api mode invalid robot_ip", map[string]string{"mode": "api", "robot_ip": "not-an-ip"}, "invalid robot_ip format"},
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

func TestUnitreeRobotNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &UnitreeRobotNode{}

	tests := []struct {
		name   string
		params map[string]string
		check  func(t *testing.T, out string)
	}{
		{
			"default simulate",
			map[string]string{},
			func(t *testing.T, out string) {
				if !strings.Contains(out, "unitree_robot") {
					t.Error("expected unitree_robot in output")
				}
				if !strings.Contains(out, "Go2") {
					t.Error("expected default Go2 model")
				}
			},
		},
		{
			"Go2 stand",
			map[string]string{
				"robot_model": "Go2",
				"action":      "stand",
				"robot_id":    "go2_001",
				"speed":       "0.5",
			},
			func(t *testing.T, out string) {
				if !strings.Contains(out, "go2_001") {
					t.Error("expected robot_id in output")
				}
				if !strings.Contains(out, "stand") {
					t.Error("expected stand action")
				}
			},
		},
		{
			"B2 patrol",
			map[string]string{
				"robot_model":     "B2",
				"action":          "patrol",
				"target_location": "warehouse_zone_a",
				"speed":           "0.3",
				"safety_zone_m":   "2.0",
			},
			func(t *testing.T, out string) {
				if !strings.Contains(out, "B2") {
					t.Error("expected B2 model")
				}
				if !strings.Contains(out, "patrol") {
					t.Error("expected patrol action")
				}
				if !strings.Contains(out, "warehouse_zone_a") {
					t.Error("expected target_location")
				}
			},
		},
		{
			"H1 humanoid",
			map[string]string{
				"robot_model": "H1",
				"action":      "wave_hand",
				"robot_id":    "h1_001",
			},
			func(t *testing.T, out string) {
				if !strings.Contains(out, "H1") {
					t.Error("expected H1 model")
				}
				if !strings.Contains(out, "wave_hand") {
					t.Error("expected wave_hand action")
				}
			},
		},
		{
			"G1 humanoid",
			map[string]string{
				"robot_model": "G1-Humanoid",
				"action":      "shake_hands",
			},
			func(t *testing.T, out string) {
				if !strings.Contains(out, "G1-Humanoid") {
					t.Error("expected G1-Humanoid model")
				}
			},
		},
		{
			"simulate get_status",
			map[string]string{
				"robot_model": "Go2",
				"action":      "get_status",
			},
			func(t *testing.T, out string) {
				if !strings.Contains(out, "battery") {
					t.Error("expected battery in status")
				}
				if !strings.Contains(out, "simulate") {
					t.Error("expected simulate mode")
				}
			},
		},
		{
			"simulate get_camera",
			map[string]string{
				"robot_model": "Go2",
				"action":      "get_camera",
			},
			func(t *testing.T, out string) {
				if !strings.Contains(out, "simulated") {
					t.Error("expected simulated camera")
				}
			},
		},
		{
			"param clamping speed",
			map[string]string{
				"robot_model": "Go2",
				"action":      "walk",
				"speed":       "5.0",
			},
			func(t *testing.T, out string) {
				if !strings.Contains(out, "unitree_robot") {
					t.Error("expected unitree_robot in output")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := node.Execute(ctx, "", tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, out)
		})
	}
}

func TestUnitreeRobotNode_InferAction(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		model   string
		wantAct string
	}{
		{"stand zh", "站起来", "Go2", "stand"},
		{"stand en", "stand up", "Go2", "stand"},
		{"sit zh", "坐下", "Go2", "sit"},
		{"walk zh", "往前走", "Go2", "walk"},
		{"run zh", "跑起来", "Go2", "run"},
		{"stop zh", "停下", "Go2", "stop"},
		{"patrol zh", "巡逻A区", "B2", "patrol"},
		{"patrol en", "patrol area", "B2", "patrol"},
		{"dance zh", "跳舞", "Go2", "dance"},
		{"camera zh", "拍照", "Go2", "get_camera"},
		{"status zh", "检查状态", "Go2", "get_status"},
		{"navigate zh", "去仓库", "Go2", "navigate"},
		{"step_up zh", "上楼梯", "Go2", "step_up"},
		{"step_down zh", "下楼", "Go2", "step_down"},
		{"wave humanoid", "挥手", "H1", "wave_hand"},
		{"shake hands g1", "握手", "G1-Humanoid", "shake_hands"},
		{"default", "hello", "Go2", "stand"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			act := inferUnitreeAction(tt.input, tt.model)
			if act != tt.wantAct {
				t.Errorf("inferUnitreeAction(%q, %q) = %q, want %q", tt.input, tt.model, act, tt.wantAct)
			}
		})
	}
}

func TestUnitreeRobotNode_ActionInferenceViaExecute(t *testing.T) {
	ctx := context.Background()
	node := &UnitreeRobotNode{}

	out, err := node.Execute(ctx, "巡逻仓库A区", map[string]string{
		"robot_model": "B2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "patrol") {
		t.Errorf("expected inferred patrol action, got: %s", out)
	}
}

func TestUnitreeRobotNode_APIModeRequiresIP(t *testing.T) {
	ctx := context.Background()
	node := &UnitreeRobotNode{}

	_, err := node.Execute(ctx, "", map[string]string{
		"mode": "api",
	})
	if err == nil {
		t.Fatal("expected error for api mode without robot_ip")
	}
	if !strings.Contains(err.Error(), "robot_ip is required") {
		t.Errorf("err = %q, want 'robot_ip is required'", err.Error())
	}
}

func TestRunUnitreeSafetyChecks(t *testing.T) {
	tests := []struct {
		name       string
		robotModel string
		action     string
		speed      float64
		safetyZone float64
		wantPassed bool
	}{
		{"all clear", "Go2", "stand", 0.5, 1.0, true},
		{"high speed run warning", "Go2", "run", 0.9, 1.0, true},
		{"humanoid arm safe", "H1", "wave_hand", 0.5, 0.5, true},
		{"humanoid arm small zone", "G1", "wave_hand", 0.5, 0.3, true},
		{"patrol small zone", "B2", "patrol", 0.3, 0.3, true},
		{"very small zone", "Go2", "walk", 0.5, 0.2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runUnitreeSafetyChecks(tt.robotModel, tt.action, tt.speed, tt.safetyZone)
			if result.Passed != tt.wantPassed {
				t.Errorf("Passed: got %v, want %v", result.Passed, tt.wantPassed)
			}
		})
	}
}

func TestDescribeUnitreeAction(t *testing.T) {
	tests := []struct {
		model    string
		action   string
		location string
		want     string
	}{
		{"Go2", "stand", "", "Go2 performs stand"},
		{"B2", "patrol", "warehouse", "B2 performs patrol patrol around warehouse"},
		{"Go2", "get_status", "", "Query Go2 system status (battery, temperature, IMU, GPS)"},
		{"Go2", "get_camera", "", "Capture camera frame from Go2"},
		{"H1", "wave_hand", "", "H1 waves hand"},
		{"G1-Humanoid", "shake_hands", "", "G1-Humanoid shakes hands"},
		{"Go2", "step_up", "", "Go2 climbs up stairs/obstacle"},
		{"Go2", "step_down", "", "Go2 descends stairs/obstacle"},
	}
	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			got := describeUnitreeAction(tt.model, tt.action, tt.location)
			if got != tt.want {
				t.Errorf("describeUnitreeAction(%q, %q, %q) = %q, want %q",
					tt.model, tt.action, tt.location, got, tt.want)
			}
		})
	}
}

func TestUnitreeRobotNode_AllModels(t *testing.T) {
	ctx := context.Background()
	node := &UnitreeRobotNode{}

	models := []string{"Go2", "B2", "B2-W", "Go1", "A1", "H1", "H1-2", "G1", "G1-Humanoid"}
	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			out, err := node.Execute(ctx, "", map[string]string{
				"robot_model": model,
				"action":      "get_status",
			})
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", model, err)
			}
			if !strings.Contains(out, model) {
				t.Errorf("expected %s in output", model)
			}
		})
	}
}

func TestUnitreeRobotNode_AllActions(t *testing.T) {
	ctx := context.Background()
	node := &UnitreeRobotNode{}

	actions := []string{"stand", "sit", "walk", "run", "stop", "dance", "patrol", "get_status", "get_camera", "navigate", "step_up", "step_down"}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			out, err := node.Execute(ctx, "", map[string]string{
				"robot_model": "Go2",
				"action":      action,
			})
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", action, err)
			}
			if !strings.Contains(out, action) {
				t.Errorf("expected %s in output", action)
			}
		})
	}
}

// Ensure unitree_robot node was registered.
func TestUnitreeRobotNode_Registered(t *testing.T) {
	if _, ok := core.Get("unitree_robot"); !ok {
		t.Error("unitree_robot not registered")
	}
}