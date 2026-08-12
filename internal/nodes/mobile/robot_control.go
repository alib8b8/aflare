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

package mobile

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

var (
	validRobotTypes = map[string]bool{
		"humanoid":    true,
		"mobile_base": true,
		"arm":         true,
		"drone":       true,
		"dog":         true,
		"wheelchair":  true,
	}
	validActionTypes = map[string]bool{
		"move":     true,
		"pick":     true,
		"place":    true,
		"rotate":   true,
		"grasp":    true,
		"release":  true,
		"navigate": true,
		"scan":     true,
		"speak":    true,
		"wait":     true,
	}
	robotIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
)

// RobotControlNode plans and executes robot action sequences for embodied AI
type RobotControlNode struct{}

func (n *RobotControlNode) Name() string { return "robot_control" }

func (n *RobotControlNode) Description() string {
	return "Plan and execute robot action sequences for embodied AI. Supports humanoid robots, mobile bases, robotic arms, drones. Decomposes natural language tasks into low-level robot commands."
}

func (n *RobotControlNode) Schema() core.NodeSchema {
	return core.NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - natural language task description",
		Output:      "string - robot action plan with safety checks",
		Params: []core.ParamSchema{
			{Name: "robot_type", Type: "string", Description: "Robot type: humanoid/mobile_base/arm/drone/dog/wheelchair (default: humanoid)", Required: false, Default: "humanoid"},
			{Name: "robot_id", Type: "string", Description: "Unique robot identifier", Required: false},
			{Name: "action", Type: "string", Description: "Specific action: move/pick/place/rotate/grasp/release/navigate/scan/speak/wait", Required: false},
			{Name: "target_object", Type: "string", Description: "Target object to interact with", Required: false},
			{Name: "target_location", Type: "string", Description: "Target location (x,y,z or named place)", Required: false},
			{Name: "speed", Type: "float", Description: "Movement speed 0.0-1.0 (default: 0.5)", Required: false, Default: "0.5"},
			{Name: "force_limit", Type: "float", Description: "Max force in Newtons (default: 10.0)", Required: false, Default: "10.0"},
			{Name: "safety_zone_m", Type: "float", Description: "Safety zone radius in meters (default: 0.5)", Required: false, Default: "0.5"},
			{Name: "visual_feedback", Type: "bool", Description: "Use visual feedback for verification (default: true)", Required: false, Default: "true"},
			{Name: "tactile_feedback", Type: "bool", Description: "Use tactile feedback for grasping (default: true)", Required: false, Default: "true"},
		},
	}
}

func (n *RobotControlNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	robotType := getMobileParam(params, "robot_type", "humanoid")
	if !validRobotTypes[robotType] {
		return "", fmt.Errorf("invalid robot_type: %s", robotType)
	}

	robotID := getMobileParam(params, "robot_id", "")
	if robotID != "" && !robotIDPattern.MatchString(robotID) {
		return "", fmt.Errorf("invalid robot_id format")
	}
	if robotID == "" {
		robotID = fmt.Sprintf("robot_%d", time.Now().Unix())
	}

	action := getMobileParam(params, "action", "")
	if action != "" && !validActionTypes[action] {
		return "", fmt.Errorf("invalid action: %s", action)
	}

	targetObject := getMobileParam(params, "target_object", "")
	if len(targetObject) > 200 {
		return "", fmt.Errorf("target_object too long")
	}

	targetLocation := getMobileParam(params, "target_location", "")
	if len(targetLocation) > 200 {
		return "", fmt.Errorf("target_location too long")
	}

	speed := parseFloatSafe(getMobileParam(params, "speed", "0.5"), 0.5)
	if speed < 0 || speed > 1.0 {
		speed = 0.5
	}

	forceLimit := parseFloatSafe(getMobileParam(params, "force_limit", "10.0"), 10.0)
	if forceLimit < 0 || forceLimit > 100 {
		forceLimit = 10.0
	}

	safetyZone := parseFloatSafe(getMobileParam(params, "safety_zone_m", "0.5"), 0.5)
	if safetyZone < 0.1 || safetyZone > 5.0 {
		safetyZone = 0.5
	}

	visualFeedback := strings.ToLower(getMobileParam(params, "visual_feedback", "true")) == "true"
	tactileFeedback := strings.ToLower(getMobileParam(params, "tactile_feedback", "true")) == "true"

	// Generate action plan from natural language or direct action
	var plan *RobotActionPlan
	if action != "" {
		plan = generateDirectActionPlan(action, targetObject, targetLocation, speed, forceLimit)
	} else {
		plan = generateTaskPlan(input, robotType, speed, forceLimit, safetyZone)
	}

	// Run safety checks
	safetyResult := runSafetyChecks(plan, safetyZone, visualFeedback, tactileFeedback)

	result := map[string]interface{}{
		"type":             "robot_control",
		"robot_type":       robotType,
		"robot_id":         robotID,
		"input_task":       input,
		"action_plan":      plan,
		"safety_checks":    safetyResult,
		"safe_to_execute":  safetyResult.Passed,
		"visual_feedback":  visualFeedback,
		"tactile_feedback": tactileFeedback,
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

// RobotAction represents a single robot action
type RobotAction struct {
	ID              string                 `json:"id"`
	Type            string                 `json:"type"`
	Description     string                 `json:"description"`
	Parameters      map[string]interface{} `json:"parameters"`
	EstimatedMs     int                    `json:"estimated_ms"`
	RequiresVision  bool                   `json:"requires_vision"`
	RequiresTactile bool                   `json:"requires_tactile"`
}

// RobotActionPlan represents a sequence of robot actions
type RobotActionPlan struct {
	Task         string        `json:"task"`
	RobotType    string        `json:"robot_type"`
	Actions      []RobotAction `json:"actions"`
	TotalSteps   int           `json:"total_steps"`
	EstimatedMs  int           `json:"estimated_ms"`
	FallbackPlan string        `json:"fallback_plan,omitempty"`
}

// SafetyCheckResult represents safety validation result
type SafetyCheckResult struct {
	Passed      bool     `json:"passed"`
	Warnings    []string `json:"warnings,omitempty"`
	Violations  []string `json:"violations,omitempty"`
	Mitigations []string `json:"mitigations,omitempty"`
}

func generateTaskPlan(task, robotType string, speed, forceLimit, safetyZone float64) *RobotActionPlan {
	actions := generateTaskActions(task, speed, forceLimit)

	totalMs := 0
	for _, a := range actions {
		totalMs += a.EstimatedMs
	}

	return &RobotActionPlan{
		Task:         task,
		RobotType:    robotType,
		Actions:      actions,
		TotalSteps:   len(actions),
		EstimatedMs:  totalMs,
		FallbackPlan: "Ask human operator for assistance",
	}
}

func generateTaskActions(task string, speed, forceLimit float64) []RobotAction {
	actions := []RobotAction{}
	lowerTask := strings.ToLower(task)

	// Parse task and generate appropriate action sequence
	switch {
	case strings.Contains(lowerTask, "pick") || strings.Contains(lowerTask, "拿") || strings.Contains(lowerTask, "取"):
		actions = append(actions, RobotAction{
			ID:             "1",
			Type:           "navigate",
			Description:    "Move to object location",
			Parameters:     map[string]interface{}{"speed": speed},
			EstimatedMs:    2000,
			RequiresVision: true,
		})
		actions = append(actions, RobotAction{
			ID:             "2",
			Type:           "scan",
			Description:    "Locate target object with vision",
			Parameters:     map[string]interface{}{"mode": "object_detection"},
			EstimatedMs:    500,
			RequiresVision: true,
		})
		actions = append(actions, RobotAction{
			ID:              "3",
			Type:            "grasp",
			Description:     "Grasp object with force feedback",
			Parameters:      map[string]interface{}{"force_limit": forceLimit},
			EstimatedMs:     1500,
			RequiresVision:  true,
			RequiresTactile: true,
		})
		actions = append(actions, RobotAction{
			ID:             "4",
			Type:           "pick",
			Description:    "Lift object to transport height",
			Parameters:     map[string]interface{}{"height_m": 0.3},
			EstimatedMs:    1000,
			RequiresVision: true,
		})

	case strings.Contains(lowerTask, "place") || strings.Contains(lowerTask, "放"):
		actions = append(actions, RobotAction{
			ID:             "1",
			Type:           "navigate",
			Description:    "Move to placement location",
			Parameters:     map[string]interface{}{"speed": speed},
			EstimatedMs:    2000,
			RequiresVision: true,
		})
		actions = append(actions, RobotAction{
			ID:             "2",
			Type:           "scan",
			Description:    "Verify placement area is clear",
			Parameters:     map[string]interface{}{"mode": "surface_detection"},
			EstimatedMs:    500,
			RequiresVision: true,
		})
		actions = append(actions, RobotAction{
			ID:              "3",
			Type:            "place",
			Description:     "Place object gently",
			Parameters:      map[string]interface{}{"force_limit": forceLimit * 0.3},
			EstimatedMs:     1500,
			RequiresTactile: true,
		})
		actions = append(actions, RobotAction{
			ID:          "4",
			Type:        "release",
			Description: "Release gripper",
			Parameters:  map[string]interface{}{},
			EstimatedMs: 500,
		})

	case strings.Contains(lowerTask, "water") || strings.Contains(lowerTask, "水"):
		actions = append(actions, RobotAction{
			ID:             "1",
			Type:           "navigate",
			Description:    "Move to water dispenser",
			Parameters:     map[string]interface{}{"speed": speed},
			EstimatedMs:    3000,
			RequiresVision: true,
		})
		actions = append(actions, RobotAction{
			ID:             "2",
			Type:           "scan",
			Description:    "Locate cup and dispenser",
			Parameters:     map[string]interface{}{"mode": "object_detection"},
			EstimatedMs:    500,
			RequiresVision: true,
		})
		actions = append(actions, RobotAction{
			ID:              "3",
			Type:            "pick",
			Description:     "Pick up cup",
			Parameters:      map[string]interface{}{},
			EstimatedMs:     1500,
			RequiresVision:  true,
			RequiresTactile: true,
		})
		actions = append(actions, RobotAction{
			ID:             "4",
			Type:           "move",
			Description:    "Position cup under dispenser",
			Parameters:     map[string]interface{}{"precision": "high"},
			EstimatedMs:    1000,
			RequiresVision: true,
		})
		actions = append(actions, RobotAction{
			ID:          "5",
			Type:        "wait",
			Description: "Wait for water dispensing",
			Parameters:  map[string]interface{}{"duration_ms": 3000},
			EstimatedMs: 3000,
		})
		actions = append(actions, RobotAction{
			ID:              "6",
			Type:            "place",
			Description:     "Place filled cup",
			Parameters:      map[string]interface{}{},
			EstimatedMs:     1500,
			RequiresTactile: true,
		})

	default:
		// Generic task decomposition
		actions = append(actions, RobotAction{
			ID:             "1",
			Type:           "scan",
			Description:    "Understand environment",
			Parameters:     map[string]interface{}{"mode": "scene_understanding"},
			EstimatedMs:    1000,
			RequiresVision: true,
		})
		actions = append(actions, RobotAction{
			ID:          "2",
			Type:        "speak",
			Description: "Confirm task understanding",
			Parameters:  map[string]interface{}{"text": "I understand, let me help you with that"},
			EstimatedMs: 2000,
		})
	}

	return actions
}

func generateDirectActionPlan(action, targetObject, targetLocation string, speed, forceLimit float64) *RobotActionPlan {
	actions := []RobotAction{{
		ID:          "1",
		Type:        action,
		Description: fmt.Sprintf("Execute %s on %s", action, targetObject),
		Parameters: map[string]interface{}{
			"target":      targetObject,
			"location":    targetLocation,
			"speed":       speed,
			"force_limit": forceLimit,
		},
		EstimatedMs:    2000,
		RequiresVision: true,
	}}

	return &RobotActionPlan{
		Task:        fmt.Sprintf("Direct action: %s", action),
		RobotType:   "humanoid",
		Actions:     actions,
		TotalSteps:  1,
		EstimatedMs: 2000,
	}
}

func runSafetyChecks(plan *RobotActionPlan, safetyZone float64, visual, tactile bool) *SafetyCheckResult {
	result := &SafetyCheckResult{
		Passed:      true,
		Warnings:    []string{},
		Violations:  []string{},
		Mitigations: []string{},
	}

	// Check for force violations
	for _, action := range plan.Actions {
		if force, ok := action.Parameters["force_limit"].(float64); ok {
			if force > 50 {
				result.Violations = append(result.Violations,
					fmt.Sprintf("Action %s: force %.1fN exceeds safe limit", action.ID, force))
				result.Passed = false
			} else if force > 20 {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Action %s: force %.1fN is high, use caution", action.ID, force))
			}
		}
	}

	// Check vision/tactile requirements
	for _, action := range plan.Actions {
		if action.RequiresVision && !visual {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Action %s requires vision but visual_feedback is disabled", action.ID))
		}
		if action.RequiresTactile && !tactile {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Action %s requires tactile but tactile_feedback is disabled", action.ID))
		}
	}

	// Check safety zone
	if safetyZone < 0.3 {
		result.Warnings = append(result.Warnings, "Safety zone is small (< 0.3m), ensure human clearance")
	}

	// Add mitigations if there are warnings
	if len(result.Warnings) > 0 && result.Passed {
		result.Mitigations = append(result.Mitigations, "Reduce speed by 50%", "Enable all feedback sensors")
	}

	return result
}

func init() {
	core.Register(&RobotControlNode{})
}
