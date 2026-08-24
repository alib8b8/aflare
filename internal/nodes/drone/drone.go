// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌‌​‌​‌​‌‌‌‌‌‌​‌‌​​​​‌​​​​‌‌‌​‌‌‌‌‌​‌‌‌​​​​​‌​‌​​​​​​​​​​​​​​​​​‌‌​​‌‌‌​‌‌​​‌​​⁠
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

// Package drone provides a workflow node for controlling MAVLink-compatible
// drones (PX4, ArduPilot) via a HTTP bridge. The bridge can be a MAVSDK server
// or the included drone_bridge.py script.
package drone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/httpclient"
	"github.com/alib8b8/aflare/internal/nodes/core"
)

var (
	validDroneActions = map[string]bool{
		"arm": true, "disarm": true, "takeoff": true, "land": true,
		"rtl": true, "hold": true, "goto": true, "mission_start": true,
		"mission_pause": true, "mission_resume": true, "mission_upload": true,
		"mission_clear": true, "set_mode": true, "get_telemetry": true,
		"get_status": true, "get_gps": true, "get_battery": true,
		"camera": true, "deliver": true, "patrol": true, "survey": true,
		"orbit": true, "follow": true,
	}
	validDroneModes = map[string]bool{
		"simulate": true, "mavsdk": true, "http": true,
	}
	validDroneModels = map[string]bool{
		"PX4": true, "ArduPilot": true, "ArduCopter": true,
		"ArduPlane": true, "ArduRover": true, "generic": true,
	}
	validMissionTypes = map[string]bool{
		"waypoint": true, "survey": true, "corridor": true,
		"orbit": true, "patrol": true,
	}
	latPattern = regexp.MustCompile(`^-?([0-8]?\d(\.\d+)?|90(\.0+)?)$`)
	lonPattern = regexp.MustCompile(`^-?((1[0-7]\d|[1-9]?\d)(\.\d+)?|180(\.0+)?)$`)
)

// DroneNode controls MAVLink-compatible drones (PX4, ArduPilot) via HTTP bridge.
// Supports arm/disarm, takeoff, land, RTL, mission upload, telemetry,
// patrol, survey, orbit, and follow-me operations.
// The backend is a MAVSDK server or the included drone_bridge.py script.
type DroneNode struct{}

func (n *DroneNode) Name() string { return "drone" }

func (n *DroneNode) Description() string {
	return "Control MAVLink-compatible drones (PX4/ArduPilot). Supports arm, takeoff, land, RTL, mission upload, telemetry, patrol, survey, orbit, camera, and delivery. Communicate via HTTP bridge (MAVSDK server or drone_bridge.py)."
}

func (n *DroneNode) Schema() core.NodeSchema {
	return core.NodeSchema{
		Name:        "drone",
		Description: "Control MAVLink-compatible drones (PX4/ArduPilot) via HTTP bridge. Supports arm/disarm, takeoff, land, RTL, waypoint mission upload, telemetry polling, patrol, survey, orbit, follow-me, and camera capture. Requires a MAVSDK server (drone_bridge.py) running on the drone's companion computer or ground station.",
		Input:       "string - natural language instruction or command description",
		Output:      "string - JSON drone control result with telemetry and status",
		Params: []core.ParamSchema{
			{Name: "drone_model", Type: "string", Description: "Drone firmware: PX4, ArduPilot, ArduCopter, ArduPlane, ArduRover, generic (default: PX4)", Required: false, Default: "PX4"},
			{Name: "drone_id", Type: "string", Description: "Unique drone identifier (default: auto-generated)", Required: false},
			{Name: "bridge_host", Type: "string", Description: "MAVSDK bridge host IP (default: 127.0.0.1)", Required: false, Default: "127.0.0.1"},
			{Name: "bridge_port", Type: "string", Description: "MAVSDK bridge port (default: 50051 for gRPC, 8080 for HTTP)", Required: false, Default: "8080"},
			{Name: "bridge_token", Type: "string", Description: "Bearer token for bridge authentication (default: none)", Required: false},
			{Name: "action", Type: "string", Description: "Action: arm, disarm, takeoff, land, rtl, hold, goto, mission_start, mission_pause, mission_resume, mission_upload, mission_clear, set_mode, get_telemetry, get_status, get_gps, get_battery, camera, deliver, patrol, survey, orbit, follow", Required: false},
			{Name: "mode", Type: "string", Description: "Backend mode: simulate (default) | mavsdk | http", Required: false, Default: "simulate"},
			{Name: "target_altitude_m", Type: "float", Description: "Target altitude in meters (default: 10)", Required: false, Default: "10"},
			{Name: "target_latitude", Type: "float", Description: "Target latitude for goto/mission waypoints", Required: false},
			{Name: "target_longitude", Type: "float", Description: "Target longitude for goto/mission waypoints", Required: false},
			{Name: "mission_type", Type: "string", Description: "Mission type: waypoint, survey, corridor, orbit, patrol (default: waypoint)", Required: false, Default: "waypoint"},
			{Name: "waypoints", Type: "string", Description: "JSON array of waypoints [{lat, lon, alt}] for mission_upload", Required: false},
			{Name: "flight_speed_ms", Type: "float", Description: "Flight speed in m/s (default: 5)", Required: false, Default: "5"},
			{Name: "safety_altitude_m", Type: "float", Description: "Minimum safety altitude in meters (default: 3)", Required: false, Default: "3"},
			{Name: "max_flight_time_s", Type: "string", Description: "Maximum flight time in seconds (default: 300)", Required: false, Default: "300"},
			{Name: "geofence_radius_m", Type: "float", Description: "Geofence radius in meters (default: 200)", Required: false, Default: "200"},
			{Name: "timeout", Type: "string", Description: "HTTP request timeout in seconds (default: 15)", Required: false, Default: "15"},
		},
	}
}

// DroneResult is the JSON output from the node.
type DroneResult struct {
	Type         string              `json:"type"`
	DroneModel   string              `json:"drone_model"`
	DroneID      string              `json:"drone_id"`
	Action       string              `json:"action"`
	Mode         string              `json:"mode"`
	Success      bool                `json:"success"`
	Description  string              `json:"description"`
	Telemetry    *DroneTelemetry     `json:"telemetry,omitempty"`
	Mission      *DroneMissionStatus `json:"mission,omitempty"`
	SafetyChecks *DroneSafetyResult  `json:"safety_checks,omitempty"`
	Error        string              `json:"error,omitempty"`
	Timestamp    string              `json:"timestamp"`
}

// DroneTelemetry holds telemetry data returned by the drone.
type DroneTelemetry struct {
	Armed         bool    `json:"armed"`
	FlightMode    string  `json:"flight_mode"`
	BatteryPct    float64 `json:"battery_pct"`
	AltitudeM     float64 `json:"altitude_m"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	GroundSpeedMS float64 `json:"ground_speed_ms"`
	HeadingDeg    float64 `json:"heading_deg"`
	GPSSatellites int     `json:"gps_satellites"`
	GPSFixType    int     `json:"gps_fix_type"`
	InAir         bool    `json:"in_air"`
	HomeLat       float64 `json:"home_lat,omitempty"`
	HomeLon       float64 `json:"home_lon,omitempty"`
}

// DroneMissionStatus holds mission execution status.
type DroneMissionStatus struct {
	TotalItems   int     `json:"total_items"`
	CurrentIndex int     `json:"current_index"`
	ProgressPct  float64 `json:"progress_pct"`
	State        string  `json:"state"` // idle, uploading, executing, paused, complete, cancelled
}

// DroneSafetyResult holds pre-flight safety check results.
type DroneSafetyResult struct {
	Passed          bool     `json:"passed"`
	GPSOK           bool     `json:"gps_ok"`
	BatteryOK       bool     `json:"battery_ok"`
	GeofenceOK      bool     `json:"geofence_ok"`
	AltitudeOK      bool     `json:"altitude_ok"`
	PreArmChecks    bool     `json:"pre_arm_checks"`
	Warnings        []string `json:"warnings"`
	Recommendations []string `json:"recommendations"`
}

func (n *DroneNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	droneModel := getDroneParam(params, "drone_model", "PX4")
	if !validDroneModels[droneModel] {
		return "", fmt.Errorf("invalid drone_model: %s", droneModel)
	}

	droneID := getDroneParam(params, "drone_id", "")
	if droneID == "" {
		droneID = fmt.Sprintf("drone_%d", time.Now().Unix())
	}

	mode := getDroneParam(params, "mode", "simulate")
	if !validDroneModes[mode] {
		return "", fmt.Errorf("invalid mode: %s (supported: simulate, mavsdk, http)", mode)
	}

	action := getDroneParam(params, "action", "")
	if action == "" {
		action = inferDroneAction(input)
	}
	if !validDroneActions[action] {
		return "", fmt.Errorf("invalid action: %s", action)
	}

	bridgeHost := getDroneParam(params, "bridge_host", "127.0.0.1")
	bridgePort := getDroneParam(params, "bridge_port", "8080")
	bridgeToken := getDroneParam(params, "bridge_token", "")

	targetAlt := parseFloatSafe(getDroneParam(params, "target_altitude_m", "10"), 10)
	if targetAlt < 0 || targetAlt > 500 {
		return "", fmt.Errorf("target_altitude_m out of range (0-500)")
	}

	flightSpeed := parseFloatSafe(getDroneParam(params, "flight_speed_ms", "5"), 5)
	if flightSpeed < 0.5 || flightSpeed > 30 {
		return "", fmt.Errorf("flight_speed_ms out of range (0.5-30)")
	}

	safetyAlt := parseFloatSafe(getDroneParam(params, "safety_altitude_m", "3"), 3)
	geofenceRadius := parseFloatSafe(getDroneParam(params, "geofence_radius_m", "200"), 200)
	maxFlightTime := core.ParamInt(params, "max_flight_time_s", 300, 10, 3600)

	// Validate GPS coordinates if provided
	targetLat := getDroneParam(params, "target_latitude", "")
	targetLon := getDroneParam(params, "target_longitude", "")
	if targetLat != "" && !latPattern.MatchString(targetLat) {
		return "", fmt.Errorf("invalid target_latitude")
	}
	if targetLon != "" && !lonPattern.MatchString(targetLon) {
		return "", fmt.Errorf("invalid target_longitude")
	}

	missionType := getDroneParam(params, "mission_type", "waypoint")
	if !validMissionTypes[missionType] {
		return "", fmt.Errorf("invalid mission_type: %s", missionType)
	}

	waypointsJSON := getDroneParam(params, "waypoints", "")

	// Build action parameters
	actionParams := map[string]interface{}{
		"target_altitude_m": targetAlt,
		"flight_speed_ms":   flightSpeed,
		"safety_altitude_m": safetyAlt,
	}
	if targetLat != "" {
		actionParams["target_latitude"] = targetLat
	}
	if targetLon != "" {
		actionParams["target_longitude"] = targetLon
	}

	// Run pre-flight safety checks
	safety := runDroneSafetyChecks(action, droneModel, safetyAlt, geofenceRadius, maxFlightTime)

	// Build result
	result := DroneResult{
		Type:         "drone",
		DroneModel:   droneModel,
		DroneID:      droneID,
		Action:       action,
		Mode:         mode,
		Success:      safety.Passed,
		Description:  describeDroneAction(action, droneModel, targetAlt, targetLat, targetLon),
		SafetyChecks: safety,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
	}

	// Connected mode: send HTTP request to MAVSDK bridge
	if mode == "mavsdk" || mode == "http" {
		timeoutSec := core.ParamInt(params, "timeout", 15, 1, 120)
		telemetry, mission, err := n.callBridge(ctx, bridgeHost, bridgePort, bridgeToken, action, actionParams, waypointsJSON, timeoutSec, mode)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Telemetry = telemetry
			result.Mission = mission
		}
	} else {
		// Simulate mode
		result.Telemetry = simulateDroneTelemetry(action, droneModel, targetAlt)
		if isMissionAction(action) {
			result.Mission = simulateDroneMission(action)
		}
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal result: %w", err)
	}
	return string(output), nil
}

// callBridge sends a command to the MAVSDK HTTP bridge.
func (n *DroneNode) callBridge(ctx context.Context, host, port, token, action string, params map[string]interface{}, waypointsJSON string, timeoutSec int, mode string) (*DroneTelemetry, *DroneMissionStatus, error) {
	rawURL := fmt.Sprintf("http://%s:%s/api/v1/drone/%s", host, port, action)

	// SSRF protection: validate the bridge URL before dialing.
	if err := validateDroneBridgeURL(rawURL); err != nil {
		return nil, nil, err
	}

	reqBody := map[string]interface{}{
		"action":     action,
		"parameters": params,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	if waypointsJSON != "" {
		var wpts []interface{}
		if err := json.Unmarshal([]byte(waypointsJSON), &wpts); err != nil {
			return nil, nil, fmt.Errorf("invalid waypoints JSON: %w", err)
		}
		reqBody["waypoints"] = wpts
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, rawURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := droneBridgeClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("drone bridge call failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, droneMaxResponseSize))
	if err != nil {
		return nil, nil, fmt.Errorf("read bridge response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("drone bridge returned status %d: %s", resp.StatusCode, string(raw))
	}

	var bridgeResp droneBridgeResponse
	if err := json.Unmarshal(raw, &bridgeResp); err != nil {
		return nil, nil, fmt.Errorf("parse bridge response: %w", err)
	}

	if !bridgeResp.Success {
		return nil, nil, fmt.Errorf("drone bridge error: %s", bridgeResp.Error)
	}

	return bridgeResp.Telemetry, bridgeResp.Mission, nil
}

const droneMaxResponseSize = 1 * 1024 * 1024 // 1MB

// droneBridgeClient is the shared HTTP client for drone bridge communication.
// It allows loopback and private addresses (drone bridges are typically on
// localhost or LAN) but blocks link-local (cloud metadata), multicast,
// unspecified, and reserved IP ranges.
var droneBridgeClient = httpclient.NewClient(httpclient.Options{
	Timeout:   120 * time.Second,
	Validator: validateDroneBridgeIP,
})

// validateDroneBridgeIP allows loopback and private IPs (drone bridges are
// typically on the companion computer or local network) but blocks link-local
// (169.254.x.x cloud metadata), multicast, unspecified, and reserved ranges.
func validateDroneBridgeIP(ip net.IP, displayHost string) error {
	if ip.IsLoopback() || ip.IsPrivate() {
		return nil
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("drone bridge: link-local address %s is not allowed", displayHost)
	}
	if ip.IsUnspecified() {
		return fmt.Errorf("drone bridge: unspecified address %s is not allowed", displayHost)
	}
	if ip.IsMulticast() {
		return fmt.Errorf("drone bridge: multicast address %s is not allowed", displayHost)
	}
	if httpclient.IsReservedIP(ip) {
		return fmt.Errorf("drone bridge: reserved address %s is not allowed", displayHost)
	}
	return nil
}

// validateDroneBridgeURL validates the bridge URL before making a request.
// It blocks non-http schemes, URLs with userinfo, and validates the host's
// resolved IPs through validateDroneBridgeIP.
func validateDroneBridgeURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid drone bridge URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("drone bridge: only http and https are allowed, got %s", u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("drone bridge: URLs with credentials are not allowed")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("drone bridge: URL has no host")
	}
	// Resolve and validate every IP.
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("drone bridge: failed to resolve %s: %w", host, err)
	}
	for _, ip := range ips {
		if err := validateDroneBridgeIP(ip, host); err != nil {
			return err
		}
	}
	return nil
}

type droneBridgeResponse struct {
	Success   bool                `json:"success"`
	Error     string              `json:"error,omitempty"`
	Telemetry *DroneTelemetry     `json:"telemetry,omitempty"`
	Mission   *DroneMissionStatus `json:"mission,omitempty"`
}

// inferDroneAction infers a drone action from natural language input.
func inferDroneAction(input string) string {
	lower := strings.ToLower(input)

	switch {
	// Check disarm before arm (disarm contains "arm" as substring).
	case strings.Contains(lower, "disarm") || strings.Contains(lower, "上锁") || strings.Contains(lower, "锁定"):
		return "disarm"
	case strings.Contains(lower, "arm") || strings.Contains(lower, "解锁"):
		return "arm"
	case strings.Contains(lower, "起飞") || strings.Contains(lower, "takeoff") || strings.Contains(lower, "take off"):
		return "takeoff"
	case strings.Contains(lower, "降落") || strings.Contains(lower, "land") || strings.Contains(lower, "着陆"):
		return "land"
	case strings.Contains(lower, "返航") || strings.Contains(lower, "rtl") || strings.Contains(lower, "return") || strings.Contains(lower, "回家"):
		return "rtl"
	case strings.Contains(lower, "悬停") || strings.Contains(lower, "hold") || strings.Contains(lower, "hover"):
		return "hold"
	case strings.Contains(lower, "去") || strings.Contains(lower, "goto") || strings.Contains(lower, "飞往") || strings.Contains(lower, "飞到"):
		return "goto"
	// Mission actions checked before patrol (巡逻任务 contains 巡逻).
	case strings.Contains(lower, "任务") && (strings.Contains(lower, "开始") || strings.Contains(lower, "start") || strings.Contains(lower, "执行")):
		return "mission_start"
	case strings.Contains(lower, "任务") && (strings.Contains(lower, "暂停") || strings.Contains(lower, "pause")):
		return "mission_pause"
	case strings.Contains(lower, "任务") && (strings.Contains(lower, "继续") || strings.Contains(lower, "resume")):
		return "mission_resume"
	case strings.Contains(lower, "任务") && (strings.Contains(lower, "上传") || strings.Contains(lower, "upload")):
		return "mission_upload"
	case strings.Contains(lower, "航线") || strings.Contains(lower, "waypoint"):
		return "mission_upload"
	case strings.Contains(lower, "巡逻") || strings.Contains(lower, "巡检") || strings.Contains(lower, "patrol"):
		return "patrol"
	case strings.Contains(lower, "测绘") || strings.Contains(lower, "survey") || strings.Contains(lower, "测量"):
		return "survey"
	case strings.Contains(lower, "环绕") || strings.Contains(lower, "orbit") || strings.Contains(lower, "盘旋"):
		return "orbit"
	case strings.Contains(lower, "跟随") || strings.Contains(lower, "follow"):
		return "follow"
	case strings.Contains(lower, "拍照") || strings.Contains(lower, "camera") || strings.Contains(lower, "拍摄"):
		return "camera"
	case strings.Contains(lower, "投递") || strings.Contains(lower, "deliver") || strings.Contains(lower, "送货"):
		return "deliver"
	case strings.Contains(lower, "遥测") || strings.Contains(lower, "telemetry") || strings.Contains(lower, "数据"):
		return "get_telemetry"
	case strings.Contains(lower, "状态") || strings.Contains(lower, "status"):
		return "get_status"
	case strings.Contains(lower, "gps") || strings.Contains(lower, "位置"):
		return "get_gps"
	case strings.Contains(lower, "电池") || strings.Contains(lower, "battery") || strings.Contains(lower, "电量"):
		return "get_battery"
	default:
		return "get_telemetry"
	}
}

// describeDroneAction returns a human-readable description.
func describeDroneAction(action, model string, alt float64, lat, lon string) string {
	switch action {
	case "arm":
		return fmt.Sprintf("Arm %s motors", model)
	case "disarm":
		return fmt.Sprintf("Disarm %s motors", model)
	case "takeoff":
		return fmt.Sprintf("%s takes off to %.0fm altitude", model, alt)
	case "land":
		return fmt.Sprintf("%s lands at current position", model)
	case "rtl":
		return fmt.Sprintf("%s returns to launch point", model)
	case "hold":
		return fmt.Sprintf("%s holds position at %.0fm", model, alt)
	case "goto":
		if lat != "" && lon != "" {
			return fmt.Sprintf("%s flies to (%s, %s) at %.0fm", model, lon, lat, alt)
		}
		return fmt.Sprintf("%s navigates to target at %.0fm", model, alt)
	case "patrol":
		return fmt.Sprintf("%s patrols at %.0fm altitude", model, alt)
	case "survey":
		return fmt.Sprintf("%s surveys area at %.0fm altitude", model, alt)
	case "orbit":
		return fmt.Sprintf("%s orbits at %.0fm altitude", model, alt)
	case "follow":
		return fmt.Sprintf("%s follows target at %.0fm altitude", model, alt)
	case "camera":
		return fmt.Sprintf("%s captures camera image", model)
	case "deliver":
		return fmt.Sprintf("%s executes delivery mission", model)
	case "mission_start":
		return fmt.Sprintf("%s starts mission execution", model)
	case "mission_upload":
		return fmt.Sprintf("%s uploads mission", model)
	case "get_telemetry":
		return fmt.Sprintf("Query %s telemetry", model)
	case "get_status":
		return fmt.Sprintf("Query %s system status", model)
	case "get_gps":
		return fmt.Sprintf("Query %s GPS position", model)
	case "get_battery":
		return fmt.Sprintf("Query %s battery level", model)
	default:
		return fmt.Sprintf("%s executes %s", model, action)
	}
}

// isMissionAction checks if the action is mission-related.
func isMissionAction(action string) bool {
	return action == "mission_start" || action == "mission_upload" ||
		action == "patrol" || action == "survey" || action == "orbit"
}

// runDroneSafetyChecks performs pre-flight safety validation.
func runDroneSafetyChecks(action, model string, safetyAlt, geofenceRadius float64, maxFlightTime int) *DroneSafetyResult {
	result := &DroneSafetyResult{
		Passed:       true,
		GPSOK:        true,
		BatteryOK:    true,
		GeofenceOK:   true,
		AltitudeOK:   true,
		PreArmChecks: true,
	}

	// Flight actions require safety checks
	if isFlightAction(action) {
		// Altitude check
		if safetyAlt < 2 {
			result.AltitudeOK = false
			result.Warnings = append(result.Warnings,
				"Safety altitude too low (< 2m): risk of ground collision")
			result.Recommendations = append(result.Recommendations,
				"Increase safety_altitude_m to at least 3m")
		}

		// Geofence check
		if geofenceRadius > 500 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Large geofence radius %.0fm: ensure regulatory compliance", geofenceRadius))
		}

		// Flight time check
		if maxFlightTime > 600 {
			result.Warnings = append(result.Warnings,
				"Long flight time (> 10min): ensure battery capacity is sufficient")
			result.Recommendations = append(result.Recommendations,
				"Monitor battery level and set RTL trigger at 25%")
		}

		// Arm/takeoff safety
		if action == "arm" || action == "takeoff" {
			result.Warnings = append(result.Warnings,
				"Ensure clear takeoff area and no people nearby")
			result.Recommendations = append(result.Recommendations,
				"Perform visual inspection before arming")
		}

		// Delivery/patrol special checks
		if action == "deliver" {
			result.Warnings = append(result.Warnings,
				"Verify delivery zone is clear of obstacles and people")
		}
	}

	// Geofence violated
	if geofenceRadius < 10 {
		result.GeofenceOK = false
		result.Warnings = append(result.Warnings,
			"Geofence radius too small (< 10m): drone cannot navigate safely")
	}

	return result
}

// isFlightAction checks if the action involves actual flight.
func isFlightAction(action string) bool {
	return action == "takeoff" || action == "land" || action == "goto" ||
		action == "rtl" || action == "patrol" || action == "survey" ||
		action == "orbit" || action == "follow" || action == "deliver" ||
		action == "mission_start" || action == "arm"
}

// simulateDroneTelemetry generates simulated telemetry for testing.
func simulateDroneTelemetry(action, model string, alt float64) *DroneTelemetry {
	t := &DroneTelemetry{
		Armed:         false,
		FlightMode:    "GUIDED",
		BatteryPct:    92.0,
		AltitudeM:     0,
		Latitude:      30.2741,
		Longitude:     120.1551,
		GroundSpeedMS: 0,
		HeadingDeg:    0,
		GPSSatellites: 16,
		GPSFixType:    3,
		InAir:         false,
		HomeLat:       30.2741,
		HomeLon:       120.1551,
	}

	switch action {
	case "arm":
		t.Armed = true
	case "takeoff":
		t.Armed = true
		t.InAir = true
		t.AltitudeM = alt
		t.FlightMode = "TAKEOFF"
	case "goto", "patrol", "survey", "orbit", "follow":
		t.Armed = true
		t.InAir = true
		t.AltitudeM = alt
		t.GroundSpeedMS = 5
		t.FlightMode = "AUTO"
	case "land":
		t.Armed = false
		t.InAir = false
		t.AltitudeM = 0
		t.FlightMode = "LAND"
	case "rtl":
		t.FlightMode = "RTL"
		t.AltitudeM = alt
	case "hold":
		t.Armed = true
		t.InAir = true
		t.AltitudeM = alt
		t.FlightMode = "LOITER"
	}

	return t
}

// simulateDroneMission generates simulated mission status.
func simulateDroneMission(action string) *DroneMissionStatus {
	m := &DroneMissionStatus{
		TotalItems:   5,
		CurrentIndex: 0,
		ProgressPct:  0,
		State:        "idle",
	}

	switch action {
	case "mission_upload":
		m.State = "uploading"
		m.ProgressPct = 100
	case "mission_start", "patrol", "survey", "orbit":
		m.State = "executing"
		m.CurrentIndex = 1
		m.ProgressPct = 20
	}

	return m
}

// getDroneParam returns a parameter value with a default fallback.
func getDroneParam(params map[string]string, key, defaultVal string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultVal
}

func parseFloatSafe(s string, defaultVal float64) float64 {
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return defaultVal
	}
	return f
}

func init() {
	core.Register(&DroneNode{})
}
