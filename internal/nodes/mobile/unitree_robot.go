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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/alib8b8/llm-box/internal/nodes/core"
)

var (
	validUnitreeModels = map[string]bool{
		"Go2": true, "B2": true, "B2-W": true,
		"Go1": true, "A1": true,
		"H1": true, "H1-2": true, "G1": true, "G1-Humanoid": true,
	}
	validUnitreeActions = map[string]bool{
		"stand": true, "sit": true, "walk": true, "run": true,
		"stop": true, "dance": true, "patrol": true,
		"get_status": true, "get_camera": true, "navigate": true,
		"step_up": true, "step_down": true, "wave_hand": true, "shake_hands": true,
	}
	validUnitreeModes = map[string]bool{
		"simulate": true, "api": true,
	}
	unitreeRobotIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
	unitreeIPPattern      = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)
)

// UnitreeRobotNode controls Unitree (宇树) quadruped and humanoid robots
// via the built-in HTTP API. Supports Go2, B2, H1, G1 and other models.
// Two backends: simulate (default) and api (direct hardware control).
type UnitreeRobotNode struct{}

func (n *UnitreeRobotNode) Name() string { return "unitree_robot" }

func (n *UnitreeRobotNode) Description() string {
	return "Control Unitree (宇树) robots (Go2/B2/H1/G1). Supports stand, sit, walk, run, patrol, dance, navigation, camera capture, and status queries. Two backends: simulate (test) and api (direct HTTP control)."
}

func (n *UnitreeRobotNode) Schema() core.NodeSchema {
	return core.NodeSchema{
		Name:        "unitree_robot",
		Description: "Control Unitree (宇树) robots via HTTP API. Execute high-level actions (stand/sit/walk/run/patrol/dance) and query robot status/camera. Defaults to simulation mode; set mode=api for direct hardware control.",
		Input:       "string - natural language instruction or command description",
		Output:      "string - JSON robot control result with status and action details",
		Params: []core.ParamSchema{
			{Name: "robot_model", Type: "string", Description: "Robot model: Go2, B2, B2-W, Go1, A1, H1, H1-2, G1, G1-Humanoid (default: Go2)", Required: false, Default: "Go2"},
			{Name: "robot_id", Type: "string", Description: "Unique robot identifier (default: auto-generated)", Required: false},
			{Name: "robot_ip", Type: "string", Description: "Robot IP address (required when mode=api)", Required: false},
			{Name: "action", Type: "string", Description: "Action: stand, sit, walk, run, stop, dance, patrol, get_status, get_camera, navigate, step_up, step_down, wave_hand, shake_hands", Required: false},
			{Name: "mode", Type: "string", Description: "Backend mode: simulate (default) | api", Required: false, Default: "simulate"},
			{Name: "speed", Type: "float", Description: "Movement speed 0.0-1.0 (default: 0.5)", Required: false, Default: "0.5"},
			{Name: "target_location", Type: "string", Description: "Target location for navigate/patrol (x,y,z or named place)", Required: false},
			{Name: "duration_seconds", Type: "string", Description: "Action duration in seconds (default: 5)", Required: false, Default: "5"},
			{Name: "safety_zone_m", Type: "float", Description: "Safety zone radius in meters (default: 1.0)", Required: false, Default: "1.0"},
			{Name: "api_endpoint", Type: "string", Description: "Custom API endpoint path (default: /api/control)", Required: false},
			{Name: "api_key", Type: "string", Description: "API key for authenticated robot access (optional)", Required: false},
			{Name: "timeout", Type: "string", Description: "HTTP request timeout in seconds (api mode, default: 15)", Required: false, Default: "15"},
		},
	}
}

// UnitreeRobotResult is the JSON output from the node.
type UnitreeRobotResult struct {
	Type         string                 `json:"type"`
	RobotModel   string                 `json:"robot_model"`
	RobotID      string                 `json:"robot_id"`
	Action       string                 `json:"action"`
	Mode         string                 `json:"mode"`
	Success      bool                   `json:"success"`
	Description  string                 `json:"description"`
	Parameters   map[string]interface{} `json:"parameters"`
	SafetyChecks *SafetyCheckResult     `json:"safety_checks,omitempty"`
	Status       map[string]interface{} `json:"status,omitempty"`
	CameraFrame  string                 `json:"camera_frame,omitempty"` // base64 encoded when get_camera
	Error        string                 `json:"error,omitempty"`
	APIRaw       string                 `json:"api_raw,omitempty"` // raw API response for debugging
	Timestamp    string                 `json:"timestamp"`
}

func (n *UnitreeRobotNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	// Validate robot model
	robotModel := getMobileParam(params, "robot_model", "Go2")
	if !validUnitreeModels[robotModel] {
		valid := make([]string, 0, len(validUnitreeModels))
		for m := range validUnitreeModels {
			valid = append(valid, m)
		}
		return "", fmt.Errorf("invalid robot_model: %s (supported: %s)", robotModel, strings.Join(valid, ", "))
	}

	// Validate robot ID
	robotID := getMobileParam(params, "robot_id", "")
	if robotID != "" && !unitreeRobotIDPattern.MatchString(robotID) {
		return "", fmt.Errorf("invalid robot_id format")
	}
	if robotID == "" {
		robotID = fmt.Sprintf("unitree_%d", time.Now().Unix())
	}

	// Validate mode
	mode := getMobileParam(params, "mode", "simulate")
	if !validUnitreeModes[mode] {
		return "", fmt.Errorf("invalid mode: %s (supported: simulate, api)", mode)
	}

	// Validate action
	action := getMobileParam(params, "action", "")
	if action != "" && !validUnitreeActions[action] {
		return "", fmt.Errorf("invalid action: %s (supported: stand, sit, walk, run, stop, dance, patrol, get_status, get_camera, navigate, step_up, step_down, wave_hand, shake_hands)", action)
	}

	// If no explicit action, try to infer from input
	if action == "" {
		action = inferUnitreeAction(input, robotModel)
	}

	// Validate and clamp speed
	speed := parseFloatSafe(getMobileParam(params, "speed", "0.5"), 0.5)
	if speed < 0 || speed > 1.0 {
		speed = 0.5
	}

	// Validate target location
	targetLocation := getMobileParam(params, "target_location", "")
	if len(targetLocation) > 200 {
		return "", fmt.Errorf("target_location too long")
	}

	// Validate duration
	durationSec := core.ParamInt(params, "duration_seconds", 5, 1, 300)

	// Validate safety zone
	safetyZone := parseFloatSafe(getMobileParam(params, "safety_zone_m", "1.0"), 1.0)
	if safetyZone < 0.1 || safetyZone > 10.0 {
		safetyZone = 1.0
	}

	// For API mode, validate robot_ip
	robotIP := getMobileParam(params, "robot_ip", "")
	if mode == "api" {
		if robotIP == "" {
			return "", fmt.Errorf("robot_ip is required when mode=api")
		}
		if !unitreeIPPattern.MatchString(robotIP) {
			return "", fmt.Errorf("invalid robot_ip format: %s", robotIP)
		}
	}

	// Build action parameters
	actionParams := map[string]interface{}{
		"speed":            speed,
		"duration_seconds": durationSec,
	}
	if targetLocation != "" {
		actionParams["target_location"] = targetLocation
	}

	// Run safety checks
	safetyResult := runUnitreeSafetyChecks(robotModel, action, speed, safetyZone)

	// Build result
	result := UnitreeRobotResult{
		Type:         "unitree_robot",
		RobotModel:   robotModel,
		RobotID:      robotID,
		Action:       action,
		Mode:         mode,
		Success:      safetyResult.Passed,
		Description:  describeUnitreeAction(robotModel, action, targetLocation),
		Parameters:   actionParams,
		SafetyChecks: safetyResult,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
	}

	// API mode: send HTTP request to robot
	if mode == "api" {
		apiEndpoint := getMobileParam(params, "api_endpoint", "/api/control")
		apiKey := getMobileParam(params, "api_key", "")
		timeoutSec := core.ParamInt(params, "timeout", 15, 1, 60)

		apiResult, err := n.callUnitreeAPI(ctx, robotIP, apiEndpoint, apiKey, robotModel, action, actionParams, timeoutSec)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.APIRaw = apiResult
			// Parse status from API response
			var apiResp map[string]interface{}
			if json.Unmarshal([]byte(apiResult), &apiResp) == nil {
				result.Status = apiResp
			}
		}
	} else {
		// Simulate mode: generate deterministic response
		result.Status = simulateUnitreeStatus(robotModel, action, actionParams)
		if action == "get_camera" {
			result.CameraFrame = "[simulated: camera frame would be captured from " + robotModel + "]"
		}
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal result: %w", err)
	}
	return string(output), nil
}

// callUnitreeAPI sends an HTTP request to the Unitree robot's built-in API.
func (n *UnitreeRobotNode) callUnitreeAPI(ctx context.Context, robotIP, endpoint, apiKey, robotModel, action string, params map[string]interface{}, timeoutSec int) (string, error) {
	// Build URL: http://<robot-ip>/api/control or custom endpoint
	url := fmt.Sprintf("http://%s%s", robotIP, endpoint)

	reqBody := map[string]interface{}{
		"robot_model": robotModel,
		"action":      action,
		"parameters":  params,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := unitreeHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Unitree API call to %s failed: %w", robotIP, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("Unitree API returned status %d: %s", resp.StatusCode, string(body))
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, unitreeMaxResponseSize))
	if err != nil {
		return "", fmt.Errorf("read API response: %w", err)
	}
	return string(raw), nil
}

// unitreeMaxResponseSize caps the API response body.
const unitreeMaxResponseSize = 8 * 1024 * 1024 // 8MB

// unitreeHTTPClient uses the safe LLM HTTP client from core.
var unitreeHTTPClient = core.SafeLLMHTTPClient

// inferUnitreeAction infers a robot action from natural language input.
func inferUnitreeAction(input, robotModel string) string {
	lower := strings.ToLower(input)

	// Check if it's a humanoid model (H1, G1, G1-Humanoid)
	isHumanoid := strings.HasPrefix(strings.ToLower(robotModel), "h1") ||
		strings.HasPrefix(strings.ToLower(robotModel), "g1")

	switch {
	case strings.Contains(lower, "站") || strings.Contains(lower, "stand"):
		return "stand"
	case strings.Contains(lower, "坐") || strings.Contains(lower, "sit") || strings.Contains(lower, "蹲"):
		return "sit"
	case strings.Contains(lower, "跑") || strings.Contains(lower, "run"):
		return "run"
	case strings.Contains(lower, "停") || strings.Contains(lower, "stop"):
		return "stop"
	case strings.Contains(lower, "走") || strings.Contains(lower, "walk"):
		return "walk"
	case strings.Contains(lower, "巡逻") || strings.Contains(lower, "巡检") || strings.Contains(lower, "patrol"):
		return "patrol"
	case strings.Contains(lower, "舞") || strings.Contains(lower, "dance"):
		return "dance"
	case strings.Contains(lower, "拍照") || strings.Contains(lower, "camera") || strings.Contains(lower, "照片"):
		return "get_camera"
	case strings.Contains(lower, "状态") || strings.Contains(lower, "status") || strings.Contains(lower, "电池") || strings.Contains(lower, "battery"):
		return "get_status"
	case strings.Contains(lower, "去") || strings.Contains(lower, "navigate") || strings.Contains(lower, "到"):
		return "navigate"
	case strings.Contains(lower, "上去") || strings.Contains(lower, "上楼") || strings.Contains(lower, "step up"):
		return "step_up"
	case strings.Contains(lower, "下去") || strings.Contains(lower, "下楼") || strings.Contains(lower, "step down"):
		return "step_down"
	case isHumanoid && (strings.Contains(lower, "挥手") || strings.Contains(lower, "wave")):
		return "wave_hand"
	case isHumanoid && (strings.Contains(lower, "握手") || strings.Contains(lower, "shake")):
		return "shake_hands"
	default:
		return "stand"
	}
}

// describeUnitreeAction returns a human-readable description of the action.
func describeUnitreeAction(robotModel, action, location string) string {
	desc := fmt.Sprintf("%s performs %s", robotModel, action)
	switch action {
	case "patrol":
		desc += " patrol"
		if location != "" {
			desc += " around " + location
		}
	case "navigate":
		if location != "" {
			desc += " to " + location
		}
	case "get_status":
		desc = fmt.Sprintf("Query %s system status (battery, temperature, IMU, GPS)", robotModel)
	case "get_camera":
		desc = fmt.Sprintf("Capture camera frame from %s", robotModel)
	case "step_up":
		desc = fmt.Sprintf("%s climbs up stairs/obstacle", robotModel)
	case "step_down":
		desc = fmt.Sprintf("%s descends stairs/obstacle", robotModel)
	case "wave_hand":
		desc = fmt.Sprintf("%s waves hand", robotModel)
	case "shake_hands":
		desc = fmt.Sprintf("%s shakes hands", robotModel)
	}
	return desc
}

// runUnitreeSafetyChecks validates the action plan against safety constraints.
func runUnitreeSafetyChecks(robotModel, action string, speed, safetyZone float64) *SafetyCheckResult {
	result := &SafetyCheckResult{
		Passed:      true,
		Warnings:    []string{},
		Violations:  []string{},
		Mitigations: []string{},
	}

	// Speed safety: high-speed actions on quadrupeds
	isQuadruped := strings.HasPrefix(strings.ToLower(robotModel), "go") ||
		strings.HasPrefix(strings.ToLower(robotModel), "b") ||
		strings.HasPrefix(strings.ToLower(robotModel), "a")

	if isQuadruped && action == "run" && speed > 0.8 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("High speed %.1f on %s: ensure open area and no obstacles", speed, robotModel))
		result.Mitigations = append(result.Mitigations, "Reduce speed to 0.5 if environment is uncertain")
	}

	// Humanoid safety: arm actions require proper clearance
	isHumanoid := strings.HasPrefix(strings.ToLower(robotModel), "h1") ||
		strings.HasPrefix(strings.ToLower(robotModel), "g1")

	if isHumanoid && (action == "wave_hand" || action == "shake_hands") {
		if safetyZone < 0.5 {
			result.Warnings = append(result.Warnings,
				"Arm action requires safety zone >= 0.5m to avoid collisions")
		}
	}

	// Patrol duration safety
	if action == "patrol" && safetyZone < 0.5 {
		result.Warnings = append(result.Warnings,
			"Patrol with small safety zone (< 0.5m): ensure path is clear")
	}

	// Safety zone general check
	if safetyZone < 0.3 {
		result.Warnings = append(result.Warnings,
			"Safety zone is very small (< 0.3m), ensure human clearance")
		result.Mitigations = append(result.Mitigations, "Reduce speed by 50%", "Enable obstacle avoidance")
	}

	// API mode warning (informational)
	if len(result.Warnings) > 0 && result.Passed {
		if len(result.Mitigations) == 0 {
			result.Mitigations = append(result.Mitigations, "Monitor robot status during execution")
		}
	}

	return result
}

// simulateUnitreeStatus generates a simulated status response for testing.
func simulateUnitreeStatus(robotModel, action string, params map[string]interface{}) map[string]interface{} {
	status := map[string]interface{}{
		"state":       "active",
		"battery":     85.0,
		"temperature": 38.5,
		"mode":        "simulate",
	}

	// Model-specific status
	isQuadruped := strings.HasPrefix(strings.ToLower(robotModel), "go") ||
		strings.HasPrefix(strings.ToLower(robotModel), "b") ||
		strings.HasPrefix(strings.ToLower(robotModel), "a")

	if isQuadruped {
		status["joint_states"] = map[string]interface{}{
			"front_left":  "ok",
			"front_right": "ok",
			"rear_left":   "ok",
			"rear_right":  "ok",
		}
		status["imu"] = map[string]interface{}{
			"roll":  0.01,
			"pitch": 0.02,
			"yaw":   0.0,
		}
		status["gps"] = map[string]interface{}{
			"latitude":  30.2741,
			"longitude": 120.1551,
			"accuracy":  1.5,
		}
	} else {
		// Humanoid
		status["joint_states"] = map[string]interface{}{
			"head":      "ok",
			"left_arm":  "ok",
			"right_arm": "ok",
			"left_leg":  "ok",
			"right_leg": "ok",
		}
	}

	// Action-specific status
	switch action {
	case "stand":
		status["posture"] = "standing"
		status["balance"] = "stable"
	case "sit":
		status["posture"] = "sitting"
	case "walk", "run", "patrol", "navigate":
		status["posture"] = "moving"
		status["speed"] = params["speed"]
	case "stop":
		status["posture"] = "stopped"
		status["speed"] = 0.0
	case "dance":
		status["posture"] = "dancing"
		status["dance_sequence"] = "predefined_routine_1"
	case "get_status":
		status["posture"] = "idle"
	case "get_camera":
		status["camera"] = "simulated_frame"
	}

	return status
}

func init() {
	core.Register(&UnitreeRobotNode{})
}
