package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// PowerProfile defines power consumption characteristics
type PowerProfile string

const (
	PowerProfileEco      PowerProfile = "eco"      // Max battery saving
	PowerProfileBalanced PowerProfile = "balanced" // Default
	PowerProfileHigh     PowerProfile = "high"     // Max performance
)

var validPowerProfiles = map[string]bool{
	"eco":      true,
	"balanced": true,
	"high":     true,
}

// PowerManager controls energy consumption for on-device AI inference
type PowerManager struct {
	mu sync.RWMutex

	profile        PowerProfile
	maxInferenceHz float64 // Max inference calls per second
	minBatteryPct  int     // Auto-switch to eco below this
	thermalLimitC  float64 // Throttle if CPU temp exceeds this

	// Runtime stats
	inferenceCount   atomic.Int64
	inferenceLatency atomic.Int64 // cumulative ms
	lastInference    time.Time
	throttled        atomic.Bool

	// Adaptive controls
	adaptiveMode bool
	batteryAware bool
	thermalAware bool
}

var (
	defaultPowerManager *PowerManager
	powerManagerOnce    sync.Once
)

// GetDefaultPowerManager returns the singleton power manager
func GetDefaultPowerManager() *PowerManager {
	powerManagerOnce.Do(func() {
		defaultPowerManager = &PowerManager{
			profile:        PowerProfileBalanced,
			maxInferenceHz: 2.0,
			minBatteryPct:  20,
			thermalLimitC:  75.0,
			adaptiveMode:   true,
			batteryAware:   true,
			thermalAware:   true,
		}
	})
	return defaultPowerManager
}

// PowerManagerNode allows workflow-level power control
type PowerManagerNode struct{}

func (n *PowerManagerNode) Name() string { return "power_manager" }

func (n *PowerManagerNode) Description() string {
	return "Control power consumption for on-device AI. Supports eco/balanced/high profiles with adaptive battery and thermal management"
}

func (n *PowerManagerNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - command or query",
		Output:      "string - power status and recommendations",
		Params: []ParamSchema{
			{Name: "profile", Type: "string", Description: "Power profile: eco/balanced/high (default: balanced)", Required: false, Default: "balanced"},
			{Name: "max_inference_hz", Type: "float", Description: "Max inference calls per second (default: 2.0)", Required: false, Default: "2.0"},
			{Name: "min_battery_pct", Type: "int", Description: "Auto-switch to eco when battery below this % (default: 20)", Required: false, Default: "20"},
			{Name: "thermal_limit_c", Type: "float", Description: "Thermal throttle threshold in Celsius (default: 75.0)", Required: false, Default: "75.0"},
			{Name: "adaptive_mode", Type: "bool", Description: "Enable adaptive power management (default: true)", Required: false, Default: "true"},
			{Name: "battery_aware", Type: "bool", Description: "Monitor battery level (default: true)", Required: false, Default: "true"},
			{Name: "thermal_aware", Type: "bool", Description: "Monitor CPU temperature (default: true)", Required: false, Default: "true"},
		},
	}
}

func (n *PowerManagerNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	pm := GetDefaultPowerManager()

	profileStr := getMobileParam(params, "profile", "balanced")
	if !validPowerProfiles[profileStr] {
		return "", fmt.Errorf("invalid profile: %s", profileStr)
	}

	maxHz := parseFloatSafe(getMobileParam(params, "max_inference_hz", "2.0"), 2.0)
	if maxHz < 0.1 || maxHz > 100.0 {
		maxHz = 2.0
	}

	minBattery := parseIntSafe(getMobileParam(params, "min_battery_pct", "20"), 20)
	if minBattery < 0 || minBattery > 100 {
		minBattery = 20
	}

	thermalLimit := parseFloatSafe(getMobileParam(params, "thermal_limit_c", "75.0"), 75.0)
	if thermalLimit < 40.0 || thermalLimit > 100.0 {
		thermalLimit = 75.0
	}

	adaptive := strings.ToLower(getMobileParam(params, "adaptive_mode", "true")) == "true"
	batteryAware := strings.ToLower(getMobileParam(params, "battery_aware", "true")) == "true"
	thermalAware := strings.ToLower(getMobileParam(params, "thermal_aware", "true")) == "true"

	// Apply settings
	pm.mu.Lock()
	pm.profile = PowerProfile(profileStr)
	pm.maxInferenceHz = maxHz
	pm.minBatteryPct = minBattery
	pm.thermalLimitC = thermalLimit
	pm.adaptiveMode = adaptive
	pm.batteryAware = batteryAware
	pm.thermalAware = thermalAware
	pm.mu.Unlock()

	// Get current system status
	batteryLevel := simulateBatteryLevel()
	cpuTemp := simulateCPUTemperature()
	isCharging := simulateChargingStatus()

	// Determine effective profile
	effectiveProfile := pm.profile
	if pm.adaptiveMode {
		if pm.batteryAware && batteryLevel < pm.minBatteryPct && !isCharging {
			effectiveProfile = PowerProfileEco
		}
		if pm.thermalAware && cpuTemp > pm.thermalLimitC {
			effectiveProfile = PowerProfileEco
			pm.throttled.Store(true)
		} else {
			pm.throttled.Store(false)
		}
	}

	// Get quantization recommendation based on profile
	quantRec := quantizationForProfile(effectiveProfile)

	// Get thread recommendation
	threadRec := threadsForProfile(effectiveProfile)

	result := map[string]interface{}{
		"type":               "power_manager",
		"configured_profile": string(pm.profile),
		"effective_profile":  string(effectiveProfile),
		"max_inference_hz":   pm.maxInferenceHz,
		"min_battery_pct":    pm.minBatteryPct,
		"thermal_limit_c":    pm.thermalLimitC,
		"adaptive_mode":      pm.adaptiveMode,
		"battery_aware":      pm.batteryAware,
		"thermal_aware":      pm.thermalAware,
		"system_status": map[string]interface{}{
			"battery_level": batteryLevel,
			"is_charging":   isCharging,
			"cpu_temp_c":    cpuTemp,
		},
		"recommendations": map[string]interface{}{
			"quantization": quantRec,
			"threads":      threadRec,
			"use_gpu":      effectiveProfile != PowerProfileEco,
			"context_size": contextSizeForProfile(effectiveProfile),
		},
		"throttled": pm.throttled.Load(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

// CanRunInference checks if an inference should be allowed given power constraints
func (pm *PowerManager) CanRunInference() (bool, string) {
	pm.mu.RLock()
	profile := pm.profile
	maxHz := pm.maxInferenceHz
	pm.mu.RUnlock()

	// Check rate limiting
	if maxHz > 0 {
		minInterval := time.Duration(float64(time.Second) / maxHz)
		if time.Since(pm.lastInference) < minInterval {
			return false, fmt.Sprintf("rate limited: max %.1f Hz", maxHz)
		}
	}

	// Check throttling
	if pm.throttled.Load() {
		return false, "thermal throttling active"
	}

	// Eco mode restrictions
	if profile == PowerProfileEco {
		// In eco mode, allow but warn
		return true, "eco mode: reduced performance"
	}

	return true, ""
}

// RecordInference records an inference completion for stats
func (pm *PowerManager) RecordInference(latency time.Duration) {
	pm.inferenceCount.Add(1)
	pm.inferenceLatency.Add(latency.Milliseconds())
	pm.lastInference = time.Now()
}

// GetStats returns current power/inference statistics
func (pm *PowerManager) GetStats() map[string]interface{} {
	pm.mu.RLock()
	profile := pm.profile
	pm.mu.RUnlock()

	count := pm.inferenceCount.Load()
	var avgLatency float64
	if count > 0 {
		avgLatency = float64(pm.inferenceLatency.Load()) / float64(count)
	}

	return map[string]interface{}{
		"profile":         string(profile),
		"inference_count": count,
		"avg_latency_ms":  avgLatency,
		"throttled":       pm.throttled.Load(),
		"last_inference":  pm.lastInference.Format(time.RFC3339),
	}
}

func quantizationForProfile(p PowerProfile) string {
	switch p {
	case PowerProfileEco:
		return "int4"
	case PowerProfileBalanced:
		return "int8"
	case PowerProfileHigh:
		return "fp16"
	default:
		return "int8"
	}
}

func threadsForProfile(p PowerProfile) int {
	cpuCount := runtime.NumCPU()
	switch p {
	case PowerProfileEco:
		return max(1, cpuCount/4)
	case PowerProfileBalanced:
		return max(1, cpuCount/2)
	case PowerProfileHigh:
		return cpuCount
	default:
		return max(1, cpuCount/2)
	}
}

func contextSizeForProfile(p PowerProfile) int {
	switch p {
	case PowerProfileEco:
		return 2048
	case PowerProfileBalanced:
		return 4096
	case PowerProfileHigh:
		return 8192
	default:
		return 4096
	}
}

func simulateBatteryLevel() int {
	// In real implementation, read from /sys/class/power_supply/BAT0/capacity
	return 65
}

func simulateCPUTemperature() float64 {
	// In real implementation, read from thermal zone
	return 42.5
}

func simulateChargingStatus() bool {
	// In real implementation, read from power supply status
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// PowerAwareWrapper wraps any node with power management
func PowerAwareWrapper(node Node) Node {
	return &powerAwareNode{base: node}
}

type powerAwareNode struct {
	base Node
}

func (n *powerAwareNode) Name() string        { return n.base.Name() }
func (n *powerAwareNode) Description() string { return n.base.Description() + " (power-aware)" }
func (n *powerAwareNode) Schema() NodeSchema  { return n.base.Schema() }

func (n *powerAwareNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	pm := GetDefaultPowerManager()
	canRun, reason := pm.CanRunInference()
	if !canRun {
		return "", fmt.Errorf("power management blocked: %s", reason)
	}

	start := time.Now()
	result, err := n.base.Execute(ctx, input, params)
	pm.RecordInference(time.Since(start))
	return result, err
}

func init() {
	Register(&PowerManagerNode{})
}
