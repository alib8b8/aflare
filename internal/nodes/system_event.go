package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	validEventTypes = map[string]bool{
		"notification":        true,
		"incoming_call":       true,
		"sms_received":        true,
		"alarm_triggered":     true,
		"location_changed":    true,
		"battery_low":         true,
		"battery_charging":    true,
		"screen_on":           true,
		"screen_off":          true,
		"app_foreground":      true,
		"bluetooth_connected": true,
		"wifi_connected":      true,
		"headphone_connected": true,
	}
	validTriggerModes = map[string]bool{
		"immediate": true,
		"debounce":  true,
		"throttle":  true,
	}
)

// SystemEventNode listens for mobile system events and triggers workflows
type SystemEventNode struct{}

func (n *SystemEventNode) Name() string { return "system_event" }

func (n *SystemEventNode) Description() string {
	return "Listen for mobile system events (notification, call, SMS, location, battery, etc.) and trigger workflows"
}

func (n *SystemEventNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - filter pattern or event configuration JSON",
		Output:      "string - matched event data in JSON format",
		Params: []ParamSchema{
			{Name: "event_type", Type: "string", Description: "Event type to listen for (notification/incoming_call/sms_received/alarm_triggered/location_changed/battery_low/battery_charging/screen_on/screen_off/app_foreground/bluetooth_connected/wifi_connected/headphone_connected)", Required: true},
			{Name: "trigger_mode", Type: "string", Description: "Trigger mode: immediate/debounce/throttle (default: immediate)", Required: false, Default: "immediate"},
			{Name: "debounce_ms", Type: "int", Description: "Debounce interval in milliseconds (default: 1000)", Required: false, Default: "1000"},
			{Name: "filter_app", Type: "string", Description: "Filter by app package name (for notification/app_foreground events)", Required: false},
			{Name: "filter_keyword", Type: "string", Description: "Filter by keyword in event content", Required: false},
			{Name: "battery_threshold", Type: "int", Description: "Battery level threshold 0-100 (for battery_low event)", Required: false, Default: "20"},
			{Name: "location_radius_m", Type: "int", Description: "Location change radius in meters (default: 100)", Required: false, Default: "100"},
		},
	}
}

func (n *SystemEventNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	eventType := getMobileParam(params, "event_type", "")
	if eventType == "" {
		return "", fmt.Errorf("event_type parameter is required")
	}
	if !validEventTypes[eventType] {
		return "", fmt.Errorf("invalid event_type: %s", eventType)
	}

	triggerMode := getMobileParam(params, "trigger_mode", "immediate")
	if !validTriggerModes[triggerMode] {
		return "", fmt.Errorf("invalid trigger_mode: %s", triggerMode)
	}

	debounceMs := parseIntSafe(getMobileParam(params, "debounce_ms", "1000"), 1000)
	if debounceMs < 0 || debounceMs > 60000 {
		debounceMs = 1000
	}

	filterApp := getMobileParam(params, "filter_app", "")
	if filterApp != "" && !validAppNames.MatchString(filterApp) {
		return "", fmt.Errorf("invalid filter_app: %s", filterApp)
	}

	filterKeyword := getMobileParam(params, "filter_keyword", "")
	if len(filterKeyword) > 200 {
		return "", fmt.Errorf("filter_keyword too long")
	}

	batteryThreshold := parseIntSafe(getMobileParam(params, "battery_threshold", "20"), 20)
	if batteryThreshold < 0 || batteryThreshold > 100 {
		batteryThreshold = 20
	}

	locationRadius := parseIntSafe(getMobileParam(params, "location_radius_m", "100"), 100)
	if locationRadius < 1 || locationRadius > 10000 {
		locationRadius = 100
	}

	// Simulate event data based on event type
	eventData := simulateEventData(eventType, input, filterApp, filterKeyword, batteryThreshold, locationRadius)

	result := map[string]interface{}{
		"type":           "system_event",
		"event_type":     eventType,
		"trigger_mode":   triggerMode,
		"debounce_ms":    debounceMs,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"platform":       string(DetectPlatform()),
		"event_data":     eventData,
		"matched":        eventData != nil,
		"filter_app":     filterApp,
		"filter_keyword": filterKeyword,
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func simulateEventData(eventType, input, filterApp, filterKeyword string, batteryThreshold, locationRadius int) map[string]interface{} {
	switch eventType {
	case "notification":
		app := "com.tencent.mm"
		title := "New message"
		content := "Hello, are you free tonight?"
		if filterApp != "" && !strings.Contains(app, filterApp) {
			return nil
		}
		if filterKeyword != "" && !strings.Contains(content, filterKeyword) && !strings.Contains(title, filterKeyword) {
			return nil
		}
		return map[string]interface{}{
			"package_name": app,
			"title":        title,
			"content":      content,
			"timestamp":    time.Now().Unix(),
		}
	case "incoming_call":
		return map[string]interface{}{
			"caller_number": "+86 138****8888",
			"caller_name":   "Zhang San",
			"timestamp":     time.Now().Unix(),
		}
	case "sms_received":
		content := "Your verification code is 123456"
		if filterKeyword != "" && !strings.Contains(content, filterKeyword) {
			return nil
		}
		return map[string]interface{}{
			"sender":    "+86 10086",
			"content":   content,
			"timestamp": time.Now().Unix(),
		}
	case "battery_low":
		level := 15
		if level > batteryThreshold {
			return nil
		}
		return map[string]interface{}{
			"level":       level,
			"threshold":   batteryThreshold,
			"is_charging": false,
			"timestamp":   time.Now().Unix(),
		}
	case "location_changed":
		return map[string]interface{}{
			"latitude":   39.9042,
			"longitude":  116.4074,
			"accuracy":   locationRadius,
			"place_name": "Beijing",
			"timestamp":  time.Now().Unix(),
		}
	case "alarm_triggered":
		return map[string]interface{}{
			"alarm_id":  "alarm_001",
			"label":     "Morning workout",
			"timestamp": time.Now().Unix(),
		}
	case "wifi_connected":
		return map[string]interface{}{
			"ssid":      "Home_WiFi_5G",
			"bssid":     "aa:bb:cc:dd:ee:ff",
			"timestamp": time.Now().Unix(),
		}
	default:
		return map[string]interface{}{
			"event":     eventType,
			"timestamp": time.Now().Unix(),
		}
	}
}

const maxEventSubscriptions = 1000
const subscriptionExpiry = 24 * time.Hour

// EventSubscription manages active event listeners
type EventSubscription struct {
	ID            string
	EventType     string
	TriggerMode   string
	DebounceMs    int
	FilterApp     string
	FilterKeyword string
	Callback      func(event map[string]interface{})
	lastTrigger   time.Time
	lastTriggerMu sync.Mutex
	Active        bool
	createdAt     time.Time
}

var (
	eventSubscriptions   = make(map[string]*EventSubscription)
	eventSubscriptionsMu sync.RWMutex
	eventIDCounter       int
	eventIDCounterMu     sync.Mutex
)

func generateEventID() string {
	eventIDCounterMu.Lock()
	defer eventIDCounterMu.Unlock()
	eventIDCounter++
	return fmt.Sprintf("sub_%d_%d", time.Now().Unix(), eventIDCounter)
}

func cleanupExpiredSubscriptionsLocked() {
	now := time.Now()
	for id, sub := range eventSubscriptions {
		if now.Sub(sub.createdAt) > subscriptionExpiry {
			sub.Active = false
			delete(eventSubscriptions, id)
		}
	}
}

// SubscribeEvent registers a new system event listener
func SubscribeEvent(sub *EventSubscription) string {
	eventSubscriptionsMu.Lock()
	defer eventSubscriptionsMu.Unlock()

	cleanupExpiredSubscriptionsLocked()

	if len(eventSubscriptions) >= maxEventSubscriptions {
		return ""
	}

	sub.ID = generateEventID()
	sub.Active = true
	sub.createdAt = time.Now()
	eventSubscriptions[sub.ID] = sub
	return sub.ID
}

// UnsubscribeEvent removes a system event listener
func UnsubscribeEvent(id string) bool {
	eventSubscriptionsMu.Lock()
	defer eventSubscriptionsMu.Unlock()
	if sub, ok := eventSubscriptions[id]; ok {
		sub.Active = false
		delete(eventSubscriptions, id)
		return true
	}
	return false
}

// DispatchEvent dispatches a system event to all matching subscribers
func DispatchEvent(eventType string, eventData map[string]interface{}) {
	eventSubscriptionsMu.RLock()
	var subs []*EventSubscription
	for _, sub := range eventSubscriptions {
		if sub.Active && sub.EventType == eventType {
			subs = append(subs, sub)
		}
	}
	eventSubscriptionsMu.RUnlock()

	for _, sub := range subs {
		// Filter by app
		if sub.FilterApp != "" {
			if app, ok := eventData["package_name"].(string); ok && !strings.Contains(app, sub.FilterApp) {
				continue
			}
		}

		// Filter by keyword
		if sub.FilterKeyword != "" {
			matched := false
			for _, v := range eventData {
				if s, ok := v.(string); ok && strings.Contains(s, sub.FilterKeyword) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Apply debounce/throttle with per-subscription lock
		now := time.Now()
		shouldTrigger := true
		sub.lastTriggerMu.Lock()
		if sub.TriggerMode == "debounce" || sub.TriggerMode == "throttle" {
			if now.Sub(sub.lastTrigger) < time.Duration(sub.DebounceMs)*time.Millisecond {
				if sub.TriggerMode == "throttle" {
					shouldTrigger = false
				}
			}
		}
		if shouldTrigger {
			sub.lastTrigger = now
		}
		sub.lastTriggerMu.Unlock()

		if shouldTrigger {
			go safeCallback(sub.Callback, eventData)
		}
	}
}

func safeCallback(cb func(event map[string]interface{}), event map[string]interface{}) {
	defer func() {
		if r := recover(); r != nil {
			// Log panic but don't crash the event dispatcher
		}
	}()
	if cb != nil {
		cb(event)
	}
}

func init() {
	Register(&SystemEventNode{})
}
