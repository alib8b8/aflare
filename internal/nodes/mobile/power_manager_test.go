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

package mobile

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alib8b8/llm-box/internal/nodes/core"
)

func TestPowerManagerNode_Metadata(t *testing.T) {
	node := &PowerManagerNode{}
	if node.Name() != "power_manager" {
		t.Errorf("name: %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("empty description")
	}
	schema := node.Schema()
	if schema.Name != "power_manager" {
		t.Errorf("schema name: %s", schema.Name)
	}
}

func TestPowerManagerNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &PowerManagerNode{}

	_, err := node.Execute(ctx, "", map[string]string{"profile": "turbo"})
	if err == nil {
		t.Fatal("expected error for invalid profile")
	}
	if !strings.Contains(err.Error(), "invalid profile") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPowerManagerNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &PowerManagerNode{}

	tests := []struct {
		name    string
		params  map[string]string
		wantSub string
	}{
		{
			"eco profile",
			map[string]string{"profile": "eco"},
			"\"configured_profile\": \"eco\"",
		},
		{
			"balanced profile",
			map[string]string{"profile": "balanced"},
			"\"configured_profile\": \"balanced\"",
		},
		{
			"high profile",
			map[string]string{"profile": "high"},
			"\"configured_profile\": \"high\"",
		},
		{
			"param clamping - max_hz too small",
			map[string]string{"profile": "balanced", "max_inference_hz": "0.01"},
			"\"max_inference_hz\": 2",
		},
		{
			"param clamping - max_hz too large",
			map[string]string{"profile": "balanced", "max_inference_hz": "1000"},
			"\"max_inference_hz\": 2",
		},
		{
			"param clamping - thermal_limit too low",
			map[string]string{"profile": "balanced", "thermal_limit_c": "10"},
			"\"thermal_limit_c\": 75",
		},
		{
			"param clamping - thermal_limit too high",
			map[string]string{"profile": "balanced", "thermal_limit_c": "200"},
			"\"thermal_limit_c\": 75",
		},
		{
			"param clamping - min_battery_pct out of range",
			map[string]string{"profile": "balanced", "min_battery_pct": "200"},
			"\"min_battery_pct\": 20",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := node.Execute(ctx, "", tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, tt.wantSub) {
				t.Errorf("expected %q in output: %s", tt.wantSub, out)
			}
		})
	}
}

func TestPowerManagerNode_ExecuteBooleans(t *testing.T) {
	ctx := context.Background()
	node := &PowerManagerNode{}

	out, err := node.Execute(ctx, "", map[string]string{
		"profile":       "eco",
		"adaptive_mode": "false",
		"battery_aware": "false",
		"thermal_aware": "false",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "\"adaptive_mode\": false") {
		t.Errorf("expected adaptive_mode false: %s", out)
	}
	if !strings.Contains(out, "\"battery_aware\": false") {
		t.Errorf("expected battery_aware false: %s", out)
	}
}

// PowerManager method tests use fresh instances to avoid singleton contention.

func TestPowerManager_CanRunInferenceRateLimited(t *testing.T) {
	pm := &PowerManager{
		profile:        PowerProfileBalanced,
		maxInferenceHz: 100.0, // very high - minInterval = 10ms
	}
	// First call: no last inference, allowed
	ok, reason := pm.CanRunInference()
	if !ok {
		t.Errorf("first call: expected allowed, got %q", reason)
	}

	// Record an inference to set lastInference to now
	pm.RecordInference(10 * time.Millisecond)

	// Immediate second call should be rate limited
	ok, reason = pm.CanRunInference()
	if ok {
		t.Error("expected rate limited after recent inference")
	}
	if !strings.Contains(reason, "rate limited") {
		t.Errorf("expected rate limited reason, got %q", reason)
	}
}

func TestPowerManager_CanRunInferenceThrottled(t *testing.T) {
	pm := &PowerManager{
		profile:        PowerProfileBalanced,
		maxInferenceHz: 2.0,
	}
	pm.throttled.Store(true)
	ok, reason := pm.CanRunInference()
	if ok {
		t.Error("expected blocked by throttle")
	}
	if !strings.Contains(reason, "thermal throttling") {
		t.Errorf("expected throttle reason, got %q", reason)
	}
}

func TestPowerManager_CanRunInferenceEcoMode(t *testing.T) {
	pm := &PowerManager{
		profile:        PowerProfileEco,
		maxInferenceHz: 2.0,
	}
	ok, reason := pm.CanRunInference()
	if !ok {
		t.Errorf("eco mode should allow, got %q", reason)
	}
	if !strings.Contains(reason, "eco mode") {
		t.Errorf("expected eco mode warning, got %q", reason)
	}
}

func TestPowerManager_CanRunInferenceUnlimited(t *testing.T) {
	pm := &PowerManager{
		profile:        PowerProfileHigh,
		maxInferenceHz: 0, // no rate limit
	}
	ok, reason := pm.CanRunInference()
	if !ok {
		t.Errorf("expected allowed, got %q", reason)
	}
	if reason != "" {
		t.Errorf("expected empty reason, got %q", reason)
	}
}

func TestPowerManager_RecordInferenceAndGetStats(t *testing.T) {
	pm := &PowerManager{
		profile: PowerProfileBalanced,
	}

	// Initially no inferences
	stats := pm.GetStats()
	count, _ := stats["inference_count"].(int64)
	if count != 0 {
		t.Errorf("expected 0 inferences, got %d", count)
	}
	avgLatency, _ := stats["avg_latency_ms"].(float64)
	if avgLatency != 0 {
		t.Errorf("expected 0 avg latency, got %v", avgLatency)
	}

	// Record a few inferences
	pm.RecordInference(100 * time.Millisecond)
	pm.RecordInference(200 * time.Millisecond)

	stats = pm.GetStats()
	count, _ = stats["inference_count"].(int64)
	if count != 2 {
		t.Errorf("expected 2 inferences, got %d", count)
	}
	avgLatency, _ = stats["avg_latency_ms"].(float64)
	if avgLatency != 150.0 {
		t.Errorf("expected avg latency 150, got %v", avgLatency)
	}
	if stats["profile"] != "balanced" {
		t.Errorf("expected balanced profile, got %v", stats["profile"])
	}
	if _, ok := stats["last_inference"]; !ok {
		t.Error("expected last_inference field")
	}
}

func TestPowerAwareWrapper(t *testing.T) {
	// Use a fresh PowerManager via direct construction would not work since
	// PowerAwareWrapper calls GetDefaultPowerManager. The default singleton
	// may have stale state from PowerManagerNode_Execute tests. To make this
	// test reliable, we reset the singleton's rate-limit state by waiting
	// for the minInterval (0.5s at 2.0 Hz) to elapse since the last inference.
	time.Sleep(600 * time.Millisecond)

	wrapped := PowerAwareWrapper(&OnDeviceLLMNode{})
	if wrapped.Name() != "ondevice_llm" {
		t.Errorf("expected ondevice_llm, got %s", wrapped.Name())
	}
	if !strings.Contains(wrapped.Description(), "power-aware") {
		t.Errorf("expected power-aware suffix, got %s", wrapped.Description())
	}
	// Schema should pass through
	schema := wrapped.Schema()
	if schema.Name != "ondevice_llm" {
		t.Errorf("schema name: %s", schema.Name)
	}

	ctx := context.Background()
	out, err := wrapped.Execute(ctx, "translate this", map[string]string{"model": "qwen2-1.5b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "ondevice_llm") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestPowerAwareWrapper_BlockedByThrottle(t *testing.T) {
	pm := GetDefaultPowerManager()
	// Force throttle on
	pm.throttled.Store(true)
	t.Cleanup(func() { pm.throttled.Store(false) })

	wrapped := PowerAwareWrapper(&OnDeviceLLMNode{})
	_, err := wrapped.Execute(context.Background(), "hello", map[string]string{"model": "qwen2-1.5b"})
	if err == nil {
		t.Fatal("expected error from throttled power manager")
	}
	if !strings.Contains(err.Error(), "power management blocked") {
		t.Errorf("unexpected error: %v", err)
	}
}

// Ensure power_manager node was registered.
func TestPowerManagerNode_Registered(t *testing.T) {
	if _, ok := core.Get("power_manager"); !ok {
		t.Error("power_manager not registered")
	}
}
