// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌​‌​​​‌‌​​‌​‌‌​‌​‌‌‌‌​​​​‌‌​‌​​‌‌‌‌​‌​​‌‌​‌​‌​​​​​​​​​​​​​​​​​‌‌‌‌‌‌‌​‌‌‌​​​​‌⁠
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

package drone

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestDroneNode_Name(t *testing.T) {
	n := &DroneNode{}
	if n.Name() != "drone" {
		t.Errorf("expected name 'drone', got '%s'", n.Name())
	}
}

func TestDroneNode_Description(t *testing.T) {
	n := &DroneNode{}
	desc := n.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
	if !strings.Contains(desc, "MAVLink") {
		t.Error("description should mention MAVLink")
	}
}

func TestDroneNode_Schema(t *testing.T) {
	n := &DroneNode{}
	s := n.Schema()
	if s.Name != "drone" {
		t.Errorf("schema name = %s", s.Name)
	}
	if len(s.Params) == 0 {
		t.Error("schema should have params")
	}
}

// TestDroneNode_Execute_Simulate tests all actions in simulate mode.
func TestDroneNode_Execute_Simulate(t *testing.T) {
	n := &DroneNode{}
	ctx := context.Background()

	tests := []struct {
		name    string
		action  string
		params  map[string]string
		wantErr bool
	}{
		{name: "arm", action: "arm", params: nil},
		{name: "disarm", action: "disarm", params: nil},
		{name: "takeoff", action: "takeoff", params: map[string]string{"target_altitude_m": "20"}},
		{name: "land", action: "land", params: nil},
		{name: "rtl", action: "rtl", params: nil},
		{name: "hold", action: "hold", params: map[string]string{"target_altitude_m": "15"}},
		{name: "goto", action: "goto", params: map[string]string{"target_latitude": "30.5", "target_longitude": "120.5", "target_altitude_m": "30"}},
		{name: "patrol", action: "patrol", params: map[string]string{"target_altitude_m": "25"}},
		{name: "survey", action: "survey", params: map[string]string{"target_altitude_m": "50"}},
		{name: "orbit", action: "orbit", params: map[string]string{"target_altitude_m": "30"}},
		{name: "follow", action: "follow", params: nil},
		{name: "camera", action: "camera", params: nil},
		{name: "deliver", action: "deliver", params: nil},
		{name: "get_telemetry", action: "get_telemetry", params: nil},
		{name: "get_status", action: "get_status", params: nil},
		{name: "get_gps", action: "get_gps", params: nil},
		{name: "get_battery", action: "get_battery", params: nil},
		{name: "mission_upload", action: "mission_upload", params: map[string]string{"waypoints": `[{"lat": 30.5, "lon": 120.5, "alt": 20}]`}},
		{name: "mission_start", action: "mission_start", params: nil},
		{name: "mission_pause", action: "mission_pause", params: nil},
		{name: "mission_resume", action: "mission_resume", params: nil},
		{name: "mission_clear", action: "mission_clear", params: nil},
		{name: "set_mode", action: "set_mode", params: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]string{"action": tc.action, "mode": "simulate"}
			if tc.params != nil {
				for k, v := range tc.params {
					params[k] = v
				}
			}

			result, err := n.Execute(ctx, "test input", params)
			if tc.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if err == nil && result == "" {
				t.Error("expected non-empty result")
			}

			// Validate JSON output
			if err == nil {
				var dr DroneResult
				if jsonErr := json.Unmarshal([]byte(result), &dr); jsonErr != nil {
					t.Errorf("failed to unmarshal result: %v", jsonErr)
				}
				if dr.Type != "drone" {
					t.Errorf("result type = %s, want drone", dr.Type)
				}
				if dr.Action != tc.action {
					t.Errorf("result action = %s, want %s", dr.Action, tc.action)
				}
				if dr.Mode != "simulate" {
					t.Errorf("result mode = %s, want simulate", dr.Mode)
				}
				if dr.Telemetry == nil {
					t.Error("expected telemetry in simulate mode")
				}
			}
		})
	}
}

// TestDroneNode_Execute_InvalidAction tests error handling.
func TestDroneNode_Execute_InvalidAction(t *testing.T) {
	n := &DroneNode{}
	ctx := context.Background()

	params := map[string]string{
		"action": "crash_into_building",
		"mode":   "simulate",
	}
	_, err := n.Execute(ctx, "crash", params)
	if err == nil {
		t.Error("expected error for invalid action")
	}
}

// TestDroneNode_Execute_InvalidModel tests error handling.
func TestDroneNode_Execute_InvalidModel(t *testing.T) {
	n := &DroneNode{}
	ctx := context.Background()

	params := map[string]string{
		"action":      "takeoff",
		"mode":        "simulate",
		"drone_model": "DJIPhantom",
	}
	_, err := n.Execute(ctx, "takeoff", params)
	if err == nil {
		t.Error("expected error for invalid drone model")
	}
}

// TestDroneNode_Execute_InvalidMode tests error handling.
func TestDroneNode_Execute_InvalidMode(t *testing.T) {
	n := &DroneNode{}
	ctx := context.Background()

	params := map[string]string{
		"action": "takeoff",
		"mode":   "bluetooth",
	}
	_, err := n.Execute(ctx, "takeoff", params)
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

// TestDroneNode_Execute_InvalidAltitude tests out-of-range altitude.
func TestDroneNode_Execute_InvalidAltitude(t *testing.T) {
	n := &DroneNode{}
	ctx := context.Background()

	params := map[string]string{
		"action":            "takeoff",
		"mode":              "simulate",
		"target_altitude_m": "1000", // exceed 500m max
	}
	_, err := n.Execute(ctx, "takeoff", params)
	if err == nil {
		t.Error("expected error for altitude out of range")
	}
}

// TestDroneNode_Execute_InvalidSpeed tests out-of-range speed.
func TestDroneNode_Execute_InvalidSpeed(t *testing.T) {
	n := &DroneNode{}
	ctx := context.Background()

	params := map[string]string{
		"action":          "goto",
		"mode":            "simulate",
		"flight_speed_ms": "50", // exceed 30 m/s max
	}
	_, err := n.Execute(ctx, "goto", params)
	if err == nil {
		t.Error("expected error for speed out of range")
	}
}

// TestDroneNode_Execute_InvalidWaypoints tests invalid waypoints JSON.
func TestDroneNode_Execute_InvalidWaypoints(t *testing.T) {
	n := &DroneNode{}
	ctx := context.Background()

	params := map[string]string{
		"action":    "mission_upload",
		"mode":      "mavsdk",
		"waypoints": "{not valid json}",
	}
	result, err := n.Execute(ctx, "upload mission", params)
	// Execute may not return an error (it captures runtime errors in result),
	// but the result should indicate failure.
	if err == nil {
		var dr DroneResult
		if json.Unmarshal([]byte(result), &dr) != nil {
			t.Fatal("failed to unmarshal result")
		}
		if dr.Success {
			t.Error("expected failure for invalid waypoints JSON")
		}
		if dr.Error == "" {
			t.Error("expected error message in result")
		}
	}
}

// TestDroneNode_Execute_InferAction tests natural language action inference.
func TestDroneNode_Execute_InferAction(t *testing.T) {
	n := &DroneNode{}
	ctx := context.Background()

	tests := []struct {
		input          string
		expectedAction string
	}{
		{"起飞到 20 米", "takeoff"},
		{"Take off to 50 meters", "takeoff"},
		{"降落", "land"},
		{"Land now", "land"},
		{"返航", "rtl"},
		{"Return to launch", "rtl"},
		{"悬停", "hold"},
		{"Hover at current position", "hold"},
		{"去 30.5 120.5", "goto"},
		{"巡逻工业区", "patrol"},
		{"Patrol the area", "patrol"},
		{"测绘农田", "survey"},
		{"Survey the field", "survey"},
		{"拍照", "camera"},
		{"解锁", "arm"},
		{"Arm the drone", "arm"},
		{"锁定", "disarm"},
		{"Disarm motors", "disarm"},
		{"查询遥测数据", "get_telemetry"},
		{"查看电池电量", "get_battery"},
		{"GPS 位置", "get_gps"},
		{"开始巡逻任务", "mission_start"},
		{"暂停任务", "mission_pause"},
		{"继续任务", "mission_resume"},
		{"上传航线", "mission_upload"},
		{"环绕飞行", "orbit"},
		{"跟随目标", "follow"},
		{"投递货物", "deliver"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			params := map[string]string{"mode": "simulate"}
			result, err := n.Execute(ctx, tc.input, params)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			var dr DroneResult
			if json.Unmarshal([]byte(result), &dr) != nil {
				t.Errorf("failed to unmarshal: %s", result)
				return
			}
			if dr.Action != tc.expectedAction {
				t.Errorf("inferred action = %s, want %s", dr.Action, tc.expectedAction)
			}
		})
	}
}

// TestDroneNode_Execute_DifferentModels tests all supported drone models.
func TestDroneNode_Execute_DifferentModels(t *testing.T) {
	n := &DroneNode{}
	ctx := context.Background()

	models := []string{"PX4", "ArduPilot", "ArduCopter", "ArduPlane", "ArduRover", "generic"}
	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			params := map[string]string{
				"action":      "get_telemetry",
				"mode":        "simulate",
				"drone_model": model,
			}
			result, err := n.Execute(ctx, "status", params)
			if err != nil {
				t.Errorf("model %s: unexpected error: %v", model, err)
			}
			var dr DroneResult
			if json.Unmarshal([]byte(result), &dr) != nil {
				t.Errorf("model %s: failed to unmarshal", model)
			}
			if dr.DroneModel != model {
				t.Errorf("model %s: result model = %s", model, dr.DroneModel)
			}
		})
	}
}

// TestDroneNode_Execute_SafetyChecks tests safety check generation.
func TestDroneNode_Execute_SafetyChecks(t *testing.T) {
	n := &DroneNode{}
	ctx := context.Background()

	// Test takeoff with safety checks
	params := map[string]string{
		"action":            "takeoff",
		"mode":              "simulate",
		"target_altitude_m": "50",
		"safety_altitude_m": "1.5", // low safety altitude
		"geofence_radius_m": "5",   // too small
		"max_flight_time_s": "900", // long flight
	}
	result, err := n.Execute(ctx, "takeoff", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var dr DroneResult
	if json.Unmarshal([]byte(result), &dr) != nil {
		t.Fatalf("failed to unmarshal")
	}

	if dr.SafetyChecks == nil {
		t.Fatal("expected safety checks")
	}
	if dr.SafetyChecks.AltitudeOK {
		t.Error("altitude should be flagged with 1.5m safety altitude")
	}
	if dr.SafetyChecks.GeofenceOK {
		t.Error("geofence should be flagged with 5m radius")
	}
	if len(dr.SafetyChecks.Warnings) == 0 {
		t.Error("expected warnings for safety violations")
	}
}

// TestDroneNode_Execute_TelemetryFields tests telemetry output completeness.
func TestDroneNode_Execute_TelemetryFields(t *testing.T) {
	n := &DroneNode{}
	ctx := context.Background()

	params := map[string]string{
		"action": "takeoff",
		"mode":   "simulate",
	}
	result, err := n.Execute(ctx, "takeoff", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var dr DroneResult
	if json.Unmarshal([]byte(result), &dr) != nil {
		t.Fatalf("failed to unmarshal")
	}

	tel := dr.Telemetry
	if tel == nil {
		t.Fatal("expected telemetry")
	}
	if !tel.Armed {
		t.Error("expected armed after takeoff")
	}
	if !tel.InAir {
		t.Error("expected in air after takeoff")
	}
	if tel.AltitudeM <= 0 {
		t.Error("expected positive altitude after takeoff")
	}
	if tel.BatteryPct <= 0 {
		t.Error("expected positive battery percentage")
	}
	if tel.GPSSatellites < 4 {
		t.Error("expected at least 4 GPS satellites")
	}
	if tel.GPSFixType != 3 {
		t.Error("expected 3D GPS fix")
	}
}

// TestParseFloatSafe tests float parsing helper.
func TestParseFloatSafe(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"10", 10},
		{"3.14", 3.14},
		{"", 5},
		{"invalid", 5},
		{"-5", -5},
	}
	for _, tc := range tests {
		result := parseFloatSafe(tc.input, 5)
		if result != tc.expected {
			t.Errorf("parseFloatSafe(%q, 5) = %f, want %f", tc.input, result, tc.expected)
		}
	}
}

// TestGetDroneParam tests parameter retrieval helper.
func TestGetDroneParam(t *testing.T) {
	params := map[string]string{"key1": "val1", "key2": ""}
	if v := getDroneParam(params, "key1", "default"); v != "val1" {
		t.Errorf("expected val1, got %s", v)
	}
	if v := getDroneParam(params, "key2", "default"); v != "default" {
		t.Errorf("expected default, got %s", v)
	}
	if v := getDroneParam(params, "key3", "default"); v != "default" {
		t.Errorf("expected default, got %s", v)
	}
}

// TestDroneNode_Execute_MissionSimulation tests mission simulation.
func TestDroneNode_Execute_MissionSimulation(t *testing.T) {
	n := &DroneNode{}
	ctx := context.Background()

	actions := []string{"mission_upload", "mission_start", "patrol", "survey", "orbit"}
	for _, action := range actions {
		params := map[string]string{
			"action": action,
			"mode":   "simulate",
		}
		result, err := n.Execute(ctx, action, params)
		if err != nil {
			t.Errorf("action %s: unexpected error: %v", action, err)
			continue
		}
		var dr DroneResult
		if json.Unmarshal([]byte(result), &dr) != nil {
			t.Errorf("action %s: failed to unmarshal", action)
			continue
		}
		if dr.Mission == nil {
			t.Errorf("action %s: expected mission status", action)
		}
	}
}

// TestDroneNode_Execute_NonMissionActions tests that non-mission actions don't have mission status.
func TestDroneNode_Execute_NonMissionActions(t *testing.T) {
	n := &DroneNode{}
	ctx := context.Background()

	params := map[string]string{
		"action": "get_telemetry",
		"mode":   "simulate",
	}
	result, err := n.Execute(ctx, "telemetry", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var dr DroneResult
	if json.Unmarshal([]byte(result), &dr) != nil {
		t.Fatalf("failed to unmarshal")
	}
	if dr.Mission != nil {
		t.Error("non-mission action should not have mission status")
	}
}
