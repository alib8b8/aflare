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
	"encoding/json"
	"strings"
	"testing"

	"github.com/alib8b8/llm-box/internal/nodes/core"
)

func TestRobotActionNode_Metadata(t *testing.T) {
	n := &RobotActionNode{}
	if n.Name() != "robot_action" {
		t.Errorf("Name = %q, want robot_action", n.Name())
	}
	s := n.Schema()
	if s.Name != "robot_action" {
		t.Errorf("Schema Name = %q", s.Name)
	}
}

func TestRobotActionNode_Registered(t *testing.T) {
	// mobile package init() registers the node; verify via core registry.
	if _, ok := core.Get("robot_action"); !ok {
		t.Fatal("robot_action node not registered")
	}
}

func TestRobotActionNode_SimulateSandwich(t *testing.T) {
	n := &RobotActionNode{}
	out, err := n.Execute(t.Context(), "make a sandwich", map[string]string{
		"backend":    "simulate",
		"robot_type": "arm",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var plan VLAActionPlan
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !plan.Simulated {
		t.Error("plan should be simulated")
	}
	if plan.StepCount < 5 {
		t.Errorf("sandwich plan too short: %d steps", plan.StepCount)
	}
	// Should include a grasp and a place.
	hasGrasp, hasPlace := false, false
	for _, s := range plan.Steps {
		if s.Action == "grasp" {
			hasGrasp = true
		}
		if s.Action == "place" {
			hasPlace = true
		}
	}
	if !hasGrasp || !hasPlace {
		t.Errorf("sandwich plan missing grasp/place: %+v", plan.Steps)
	}
	// Step indices should be sequential 1..N.
	for i, s := range plan.Steps {
		if s.Step != i+1 {
			t.Errorf("step %d has Step=%d", i, s.Step)
		}
	}
}

func TestRobotActionNode_SimulatePick(t *testing.T) {
	n := &RobotActionNode{}
	out, err := n.Execute(t.Context(), "pick up the cup", map[string]string{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var plan VLAActionPlan
	_ = json.Unmarshal([]byte(out), &plan)
	// extractTarget should find "cup".
	foundCup := false
	for _, s := range plan.Steps {
		if s.Target == "cup" {
			foundCup = true
		}
	}
	if !foundCup {
		t.Errorf("pick plan did not target cup: %+v", plan.Steps)
	}
}

func TestRobotActionNode_SimulateGenericFallback(t *testing.T) {
	n := &RobotActionNode{}
	out, err := n.Execute(t.Context(), "organize the desk", map[string]string{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var plan VLAActionPlan
	_ = json.Unmarshal([]byte(out), &plan)
	// Generic skeleton should include perceive + verify.
	hasPerceive, hasVerify := false, false
	for _, s := range plan.Steps {
		if s.Action == "perceive" {
			hasPerceive = true
		}
		if s.Action == "verify" {
			hasVerify = true
		}
	}
	if !hasPerceive || !hasVerify {
		t.Errorf("generic plan missing perceive/verify: %+v", plan.Steps)
	}
}

func TestRobotActionNode_MaxStepsCap(t *testing.T) {
	n := &RobotActionNode{}
	out, err := n.Execute(t.Context(), "make a sandwich", map[string]string{
		"max_steps": "3",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var plan VLAActionPlan
	_ = json.Unmarshal([]byte(out), &plan)
	if plan.StepCount > 3 {
		t.Errorf("plan exceeded max_steps: %d steps", plan.StepCount)
	}
}

func TestRobotActionNode_InvalidRobotType(t *testing.T) {
	n := &RobotActionNode{}
	_, err := n.Execute(t.Context(), "do something", map[string]string{
		"robot_type": "dragon",
	})
	if err == nil {
		t.Error("expected error for invalid robot_type")
	}
}

func TestRobotActionNode_EmptyInstruction(t *testing.T) {
	n := &RobotActionNode{}
	_, err := n.Execute(t.Context(), "   ", map[string]string{})
	if err == nil {
		t.Error("expected error for empty instruction")
	}
}

func TestRobotActionNode_InstructionTooLong(t *testing.T) {
	n := &RobotActionNode{}
	long := strings.Repeat("a", 4001)
	_, err := n.Execute(t.Context(), long, map[string]string{})
	if err == nil {
		t.Error("expected error for too-long instruction")
	}
}

func TestRobotActionNode_UnknownBackend(t *testing.T) {
	n := &RobotActionNode{}
	_, err := n.Execute(t.Context(), "do something", map[string]string{
		"backend": "magic",
	})
	if err == nil {
		t.Error("expected error for unknown backend")
	}
}

func TestRobotActionNode_APIBackendMissingEndpoint(t *testing.T) {
	n := &RobotActionNode{}
	_, err := n.Execute(t.Context(), "do something", map[string]string{
		"backend": "api",
	})
	if err == nil {
		t.Error("expected error when backend=api without api_endpoint")
	}
}

func TestRobotActionNode_APIBackendInvalidImage(t *testing.T) {
	n := &RobotActionNode{}
	_, err := n.Execute(t.Context(), "do something", map[string]string{
		"backend":      "api",
		"api_endpoint": "http://127.0.0.1:8080/vla",
		"image_base64": "!!!not base64!!!",
	})
	if err == nil {
		t.Error("expected error for invalid image_base64")
	}
}

func TestRobotActionNode_InvalidObservation(t *testing.T) {
	n := &RobotActionNode{}
	_, err := n.Execute(t.Context(), "do something", map[string]string{
		"observation": "{not json",
	})
	if err == nil {
		t.Error("expected error for invalid observation JSON")
	}
}

func TestExtractTarget(t *testing.T) {
	cases := []struct {
		in       string
		fallback string
		want     string
	}{
		{"pick up the cup", "obj", "cup"},
		{"grab apple", "obj", "apple"},
		{"open the door", "obj", "door"},
		{"do something", "obj", "obj"},
	}
	for _, c := range cases {
		got := extractTarget(c.in, c.fallback)
		if got != c.want {
			t.Errorf("extractTarget(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsValidVLARobotType(t *testing.T) {
	for _, valid := range []string{"arm", "mobile", "humanoid", "gripper"} {
		if !isValidVLARobotType(valid) {
			t.Errorf("%q should be valid", valid)
		}
	}
	for _, invalid := range []string{"", "dragon", "ARM"} {
		if isValidVLARobotType(invalid) {
			t.Errorf("%q should be invalid", invalid)
		}
	}
}
