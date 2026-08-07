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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

// RobotActionNode wraps a Vision-Language-Action (VLA) model for
// embodied intelligence. Inspired by OpenBMB/MiniCPM-Robot: a small
// model turns natural-language instructions + environment observations
// into a structured action sequence executable on a real or simulated
// robot.
//
// The node supports two backends:
//   - "simulate" (default): produces deterministic, reviewable action
//     plans from the instruction without calling any external service.
//     Useful for testing and for agents that reason about plans rather
//     than executing them on hardware.
//   - "api": POSTs the instruction + observation to an external VLA
//     endpoint (e.g. a locally served MiniCPM-Robot / pi0 server) and
//     returns its action sequence.
type RobotActionNode struct{}

func init() {
	core.Register(&RobotActionNode{})
}

func (n *RobotActionNode) Name() string { return "robot_action" }

func (n *RobotActionNode) Description() string {
	return "Plan robot actions from natural-language instructions and environment state via a VLA model (MiniCPM-Robot inspired)"
}

func (n *RobotActionNode) Schema() core.NodeSchema {
	return core.NodeSchema{
		Name:        "robot_action",
		Description: "Embodied-intelligence action planner. Turns a natural-language instruction plus optional visual/proprioceptive state into a structured action sequence. Defaults to a deterministic simulator; set backend=api to call an external VLA server.",
		Input:       "string - natural-language instruction (e.g. \"make a sandwich\")",
		Output:      "string - JSON action plan",
		Params: []core.ParamSchema{
			{Name: "backend", Type: "string", Description: "simulate (default) | api", Default: "simulate"},
			{Name: "instruction", Type: "string", Description: "Instruction text. Overrides input when set."},
			{Name: "observation", Type: "string", Description: "JSON describing the environment (objects, positions, sensor readings). Optional."},
			{Name: "image_base64", Type: "string", Description: "Base64-encoded current camera frame (api backend only). Optional."},
			{Name: "robot_type", Type: "string", Description: "Robot form factor: arm | mobile | humanoid | gripper (default arm)", Default: "arm"},
			{Name: "max_steps", Type: "string", Description: "Maximum action steps to plan (default 20)", Default: "20"},
			{Name: "api_endpoint", Type: "string", Description: "VLA model HTTP endpoint (api backend). Required when backend=api."},
			{Name: "api_key", Type: "string", Description: "Bearer token for the VLA endpoint (optional)."},
			{Name: "timeout", Type: "string", Description: "Per-request timeout in seconds for the api backend (default 30)", Default: "30"},
		},
	}
}

// VLAAction describes a single atomic action in a VLA plan.
type VLAAction struct {
	Step        int                    `json:"step"`
	Action      string                 `json:"action"`
	Target      string                 `json:"target,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Description string                 `json:"description,omitempty"`
}

// VLAActionPlan is the full plan returned by the node.
type VLAActionPlan struct {
	Instruction string      `json:"instruction"`
	RobotType   string      `json:"robot_type"`
	Backend     string      `json:"backend"`
	Steps       []VLAAction `json:"steps"`
	StepCount   int         `json:"step_count"`
	Timestamp   string      `json:"timestamp"`
	Simulated   bool        `json:"simulated"`
}

func (n *RobotActionNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	instruction := core.GetParam(params, "instruction", strings.TrimSpace(input))
	if instruction == "" {
		return "", fmt.Errorf("instruction is required (param or input)")
	}
	if len(instruction) > 4000 {
		return "", fmt.Errorf("instruction too long (max 4000 chars)")
	}

	robotType := core.GetParam(params, "robot_type", "arm")
	if !isValidVLARobotType(robotType) {
		return "", fmt.Errorf("invalid robot_type: %s (supported: arm, mobile, humanoid, gripper)", robotType)
	}

	maxSteps := core.ParamInt(params, "max_steps", 20, 1, 200)
	backend := core.GetParam(params, "backend", "simulate")

	var observation map[string]interface{}
	if obsJSON := core.GetParam(params, "observation", ""); obsJSON != "" {
		if err := json.Unmarshal([]byte(obsJSON), &observation); err != nil {
			return "", fmt.Errorf("invalid observation JSON: %w", err)
		}
	}

	var steps []VLAAction
	var err error
	switch backend {
	case "simulate":
		steps = simulateActionPlan(instruction, robotType, observation, maxSteps)
	case "api":
		endpoint := core.GetParam(params, "api_endpoint", "")
		if endpoint == "" {
			return "", fmt.Errorf("api_endpoint is required when backend=api")
		}
		apiKey := core.GetParam(params, "api_key", "")
		imageB64 := core.GetParam(params, "image_base64", "")
		if imageB64 != "" {
			if _, err := base64.StdEncoding.DecodeString(imageB64); err != nil {
				return "", fmt.Errorf("invalid image_base64: %w", err)
			}
		}
		timeoutSec := core.ParamInt(params, "timeout", 30, 1, 300)
		steps, err = n.callVAAPI(ctx, endpoint, apiKey, instruction, robotType, observation, imageB64, maxSteps, timeoutSec)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unknown backend: %s (supported: simulate, api)", backend)
	}

	plan := VLAActionPlan{
		Instruction: instruction,
		RobotType:   robotType,
		Backend:     backend,
		Steps:       steps,
		StepCount:   len(steps),
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Simulated:   backend == "simulate",
	}
	out, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal plan: %w", err)
	}
	return string(out), nil
}

var vlaRobotTypes = map[string]bool{
	"arm":      true,
	"mobile":   true,
	"humanoid": true,
	"gripper":  true,
}

func isValidVLARobotType(t string) bool { return vlaRobotTypes[t] }

// simulateActionPlan produces a deterministic, reviewable action plan
// for common embodied tasks. It is intentionally conservative: when an
// instruction does not match a known template, it falls back to a
// generic "perceive -> approach -> act -> verify" skeleton.
func simulateActionPlan(instruction, robotType string, obs map[string]interface{}, maxSteps int) []VLAAction {
	instr := strings.ToLower(instruction)
	var steps []VLAAction

	// Helper to append a step with an incrementing index.
	add := func(action, target, desc string, params map[string]interface{}) {
		if len(steps) >= maxSteps {
			return
		}
		steps = append(steps, VLAAction{
			Step:        len(steps) + 1,
			Action:      action,
			Target:      target,
			Parameters:  params,
			Description: desc,
		})
	}

	// Common preamble: perceive the scene.
	add("perceive", "scene", "capture current environment state",
		map[string]interface{}{"sensors": []string{"camera", "proprioception"}})

	switch {
	case strings.Contains(instr, "sandwich"):
		add("move_to", "counter", "navigate to kitchen counter", nil)
		add("grasp", "bread", "pick up bread slice", map[string]interface{}{"gripper_force": "gentle"})
		add("place", "plate", "place bread on plate", nil)
		add("grasp", "filling", "pick up filling", map[string]interface{}{"gripper_force": "medium"})
		add("place", "bread", "place filling on bread", nil)
		add("grasp", "bread_top", "pick up second bread slice", nil)
		add("place", "filling", "place top bread", nil)
		add("release", "gripper", "open gripper", nil)
		add("move_to", "serve", "deliver sandwich", nil)
	case strings.Contains(instr, "pick") || strings.Contains(instr, "grab") || strings.Contains(instr, "take"):
		target := extractTarget(instr, "object")
		add("move_to", target, "approach "+target, nil)
		add("grasp", target, "grasp "+target, map[string]interface{}{"gripper_force": "adaptive"})
		add("lift", target, "lift object 10cm", map[string]interface{}{"height_m": 0.1})
		add("move_to", "destination", "transport to destination", nil)
		add("release", target, "release object", nil)
	case strings.Contains(instr, "open"):
		target := extractTarget(instr, "door")
		add("move_to", target, "approach "+target, nil)
		add("grasp", target+"_handle", "grasp handle", nil)
		add("pull", target, "pull/push to open", map[string]interface{}{"force_n": 15})
		add("release", target+"_handle", "release handle", nil)
	case strings.Contains(instr, "clean") || strings.Contains(instr, "wipe"):
		add("move_to", "surface", "approach surface to clean", nil)
		add("grasp", "cloth", "pick up cleaning cloth", nil)
		add("wipe", "surface", "wipe in zigzag pattern", map[string]interface{}{"pattern": "zigzag", "passes": 3})
		add("release", "cloth", "release cloth", nil)
	default:
		// Generic skeleton: perceive -> approach -> act -> verify.
		add("move_to", "target", "approach target described in instruction", nil)
		add("act", "target", "perform the requested manipulation", map[string]interface{}{
			"strategy": "adaptive",
		})
		add("verify", "scene", "confirm task completion via visual check", nil)
	}

	if len(steps) == 0 {
		add("noop", "", "no action planned", nil)
	}
	return steps
}

// extractTarget naively extracts the noun following a verb like
// "pick up the X". It skips common filler words (up/the/a/an) after
// the verb. Falls back to fallback when no noun is found.
func extractTarget(instr, fallback string) string {
	words := strings.Fields(instr)
	stopWords := map[string]bool{"up": true, "the": true, "a": true, "an": true, "that": true, "this": true}
	for i, w := range words {
		if w == "pick" || w == "grab" || w == "take" || w == "open" || w == "clean" || w == "wipe" {
			// Scan forward skipping filler words, return first real noun.
			for j := i + 1; j < len(words); j++ {
				if !stopWords[words[j]] {
					return words[j]
				}
			}
		}
	}
	return fallback
}

// callVAAPI POSTs a VLA request to an external endpoint and parses the
// returned action sequence. The wire format is intentionally simple so
// it can adapt to MiniCPM-Robot, pi0, or a custom server.
func (n *RobotActionNode) callVAAPI(ctx context.Context, endpoint, apiKey, instruction, robotType string, obs map[string]interface{}, imageB64 string, maxSteps, timeoutSec int) ([]VLAAction, error) {
	reqBody := map[string]interface{}{
		"instruction": instruction,
		"robot_type":  robotType,
		"max_steps":   maxSteps,
	}
	if obs != nil {
		reqBody["observation"] = obs
	}
	if imageB64 != "" {
		reqBody["image_base64"] = imageB64
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	req, err := newVLARequest(reqCtx, endpoint, apiKey, bodyBytes)
	if err != nil {
		return nil, err
	}
	resp, err := vlaHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VLA API call failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("VLA API returned status %d", resp.StatusCode)
	}

	var apiResp struct {
		Steps []VLAAction `json:"steps"`
		// Some servers return "actions" instead.
		Actions []VLAAction `json:"actions"`
	}
	if err := json.NewDecoder(limitReader(resp.Body, vlaMaxResponseSize)).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode VLA response: %w", err)
	}
	steps := apiResp.Steps
	if len(steps) == 0 {
		steps = apiResp.Actions
	}
	// Normalize step indices.
	for i := range steps {
		steps[i].Step = i + 1
	}
	if len(steps) > maxSteps {
		steps = steps[:maxSteps]
	}
	return steps, nil
}

// vlaMaxResponseSize caps the VLA API response body to prevent OOM
// when a misbehaving endpoint streams an excessively large payload.
const vlaMaxResponseSize = 8 * 1024 * 1024 // 8MB

// vlaHTTPClient reuses the safe LLM HTTP client from the core package.
// It blocks private non-loopback ranges at dial time to prevent SSRF,
// while allowing loopback so locally served VLA models (e.g. a
// MiniCPM-Robot server on 127.0.0.1) remain reachable.
var vlaHTTPClient = core.SafeLLMHTTPClient

// limitReader wraps r to limit reading to n bytes.
func limitReader(r io.Reader, n int64) io.Reader { return io.LimitReader(r, n) }

// newVLARequest builds the POST request to a VLA endpoint. The endpoint
// is validated with core.ValidateLMLEndpoint so local model servers
// (loopback) are allowed but private non-loopback addresses are still
// blocked.
func newVLARequest(ctx context.Context, endpoint, apiKey string, body []byte) (*http.Request, error) {
	if err := core.ValidateLMLEndpoint(endpoint); err != nil {
		return nil, fmt.Errorf("invalid api_endpoint: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return req, nil
}
