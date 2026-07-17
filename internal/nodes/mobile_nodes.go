// Package nodes provides built-in workflow nodes for mobile and cross-platform AI systems.
// These nodes enable llm-box to serve as the orchestration engine for AI-powered operating systems.
package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func init() {
	// Mobile & Cross-platform nodes
	Register(&AppLaunchNode{})
	Register(&UIAutomateNode{})
	Register(&NotificationNode{})
	Register(&IntentRouterNode{})
	Register(&DeviceStateNode{})
	Register(&CrossAppActionNode{})
}

// =============================================================================
// App Launch Node - Launch and control mobile apps
// =============================================================================

type AppLaunchNode struct{}

func (n *AppLaunchNode) Name() string { return "app_launch" }
func (n *AppLaunchNode) Description() string {
	return "Launch a mobile/desktop app with optional parameters"
}
func (n *AppLaunchNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "app_launch",
		Description: "Launch a mobile/desktop app with optional parameters. Cross-platform app control for AI systems.",
		Input:       "string - optional input to pass to the app",
		Output:      "string - launch result and app session info",
		Params: []ParamSchema{
			{Name: "app", Type: "string", Description: "App identifier: package name (Android), bundle ID (iOS), app name (desktop)", Required: true},
			{Name: "action", Type: "string", Description: "Deep link action: open, search, share, edit, view (default: open)", Required: false, Default: "open"},
			{Name: "uri", Type: "string", Description: "Deep link URI or URL scheme (e.g., myapp://detail?id=123)", Required: false},
			{Name: "params", Type: "string", Description: "JSON parameters to pass to the app", Required: false},
			{Name: "wait", Type: "string", Description: "Wait for app to fully launch: true/false (default: true)", Required: false, Default: "true"},
			{Name: "timeout", Type: "string", Description: "Launch timeout (default: 10s)", Required: false, Default: "10s"},
		},
	}
}

func (n *AppLaunchNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	app := params["app"]
	if app == "" {
		return "", fmt.Errorf("app parameter is required")
	}

	action := getMobileParam(params, "action", "open")
	uri := params["uri"]
	extraParams := params["params"]
	wait := getMobileParam(params, "wait", "true") == "true"

	intent := map[string]interface{}{
		"type":    "app_launch",
		"app":     app,
		"action":  action,
		"wait":    wait,
		"input":   input,
		"version": "1.0",
	}

	if uri != "" {
		intent["uri"] = uri
	}
	if extraParams != "" {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(extraParams), &parsed); err == nil {
			intent["params"] = parsed
		}
	}

	result, _ := json.MarshalIndent(intent, "", "  ")
	return fmt.Sprintf("App launch intent created:\n%s", string(result)), nil
}

// =============================================================================
// UI Automate Node - Automated UI interactions
// =============================================================================

type UIAutomateNode struct{}

func (n *UIAutomateNode) Name() string { return "ui_automate" }
func (n *UIAutomateNode) Description() string {
	return "Automate UI interactions: click, type, scroll, swipe"
}
func (n *UIAutomateNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "ui_automate",
		Description: "Automate UI interactions: click, type, scroll, swipe. Accessibility-based automation for AI assistants.",
		Input:       "string - text to type or context for interaction",
		Output:      "string - UI automation result and state changes",
		Params: []ParamSchema{
			{Name: "action", Type: "string", Description: "UI action: click, long_click, type, scroll, swipe, pinch, drag, wait_element, screenshot", Required: true},
			{Name: "selector", Type: "string", Description: "Element selector: id, text, content_desc, class (e.g., 'id:button_submit', 'text:登录')", Required: false},
			{Name: "x", Type: "string", Description: "X coordinate for swipe/click (0-100% of screen width)", Required: false},
			{Name: "y", Type: "string", Description: "Y coordinate for swipe/click (0-100% of screen height)", Required: false},
			{Name: "direction", Type: "string", Description: "Scroll/swipe direction: up, down, left, right", Required: false},
			{Name: "text", Type: "string", Description: "Text to type (overrides input)", Required: false},
			{Name: "duration", Type: "string", Description: "Action duration in ms (default: 300)", Required: false, Default: "300"},
		},
	}
}

func (n *UIAutomateNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	action := params["action"]
	if action == "" {
		return "", fmt.Errorf("action parameter is required")
	}

	selector := params["selector"]
	text := getMobileParam(params, "text", input)

	uiCmd := map[string]interface{}{
		"type":     "ui_automate",
		"action":   action,
		"duration": getMobileParam(params, "duration", "300"),
		"version":  "1.0",
	}

	if selector != "" {
		selectorType := "text"
		if strings.HasPrefix(selector, "id:") {
			selectorType = "id"
			selector = strings.TrimPrefix(selector, "id:")
		} else if strings.HasPrefix(selector, "text:") {
			selectorType = "text"
			selector = strings.TrimPrefix(selector, "text:")
		}
		uiCmd["selector"] = map[string]string{"type": selectorType, "value": selector}
	}

	if action == "type" {
		uiCmd["text"] = text
	}
	if params["direction"] != "" {
		uiCmd["direction"] = params["direction"]
	}

	result, _ := json.MarshalIndent(uiCmd, "", "  ")
	return fmt.Sprintf("UI automation command:\n%s", string(result)), nil
}

// =============================================================================
// Notification Node - Send and manage notifications
// =============================================================================

type NotificationNode struct{}

func (n *NotificationNode) Name() string        { return "send_notification" }
func (n *NotificationNode) Description() string { return "Send system notification with actions" }
func (n *NotificationNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "send_notification",
		Description: "Send system notification with actions. Cross-platform notification for AI assistants.",
		Input:       "string - notification body text",
		Output:      "string - notification send result",
		Params: []ParamSchema{
			{Name: "title", Type: "string", Description: "Notification title", Required: true},
			{Name: "body", Type: "string", Description: "Notification body (overrides input)", Required: false},
			{Name: "priority", Type: "string", Description: "Priority: low, default, high, max (default: default)", Required: false, Default: "default"},
			{Name: "actions", Type: "string", Description: "JSON array of action buttons", Required: false},
			{Name: "sound", Type: "string", Description: "Notification sound: default, none, or file path", Required: false, Default: "default"},
		},
	}
}

func (n *NotificationNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	title := params["title"]
	if title == "" {
		return "", fmt.Errorf("title parameter is required")
	}

	body := getMobileParam(params, "body", input)
	if body == "" {
		body = input
	}

	notif := map[string]interface{}{
		"type":     "notification",
		"title":    title,
		"body":     body,
		"priority": getMobileParam(params, "priority", "default"),
		"sound":    getMobileParam(params, "sound", "default"),
		"version":  "1.0",
	}

	if actions := params["actions"]; actions != "" {
		var parsed []map[string]string
		if err := json.Unmarshal([]byte(actions), &parsed); err == nil {
			notif["actions"] = parsed
		}
	}

	result, _ := json.MarshalIndent(notif, "", "  ")
	return fmt.Sprintf("Notification prepared:\n%s", string(result)), nil
}

// =============================================================================
// Intent Router Node - Route intents to handlers
// =============================================================================

type IntentRouterNode struct{}

func (n *IntentRouterNode) Name() string        { return "intent_router" }
func (n *IntentRouterNode) Description() string { return "Route user intents to appropriate handlers" }
func (n *IntentRouterNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "intent_router",
		Description: "Route user intents to appropriate handlers. Central dispatch for AI assistant commands.",
		Input:       "string - user utterance or intent text",
		Output:      "string - routed intent with handler assignment",
		Params: []ParamSchema{
			{Name: "mode", Type: "string", Description: "Routing mode: classify, route, execute (default: classify)", Required: false, Default: "classify"},
			{Name: "domains", Type: "string", Description: "Comma-separated domains: travel,food,shopping,entertainment,work,communication,system", Required: false},
			{Name: "fallback", Type: "string", Description: "Fallback handler when no match (default: general_assistant)", Required: false, Default: "general_assistant"},
		},
	}
}

func (n *IntentRouterNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("input cannot be empty")
	}

	mode := getMobileParam(params, "mode", "classify")
	domains := strings.Split(getMobileParam(params, "domains", "travel,food,shopping,entertainment,work,communication,system"), ",")
	fallback := getMobileParam(params, "fallback", "general_assistant")

	intent := classifyIntent(input, domains)

	result := map[string]interface{}{
		"type":           "intent_router",
		"mode":           mode,
		"input":          input,
		"classification": intent,
		"handler":        intent["handler"],
		"fallback":       fallback,
		"version":        "1.0",
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

// classifyIntent performs keyword-based intent classification
func classifyIntent(input string, domains []string) map[string]interface{} {
	inputLower := strings.ToLower(input)

	domainKeywords := map[string][]string{
		"travel":        {"机票", "酒店", "火车", "航班", "旅游", "flight", "hotel"},
		"food":          {"订餐", "外卖", "餐厅", "美食", "order food", "restaurant"},
		"shopping":      {"购物", "买", "下单", "商品", "shop", "buy"},
		"entertainment": {"音乐", "视频", "电影", "游戏", "music", "video"},
		"work":          {"会议", "日程", "提醒", "邮件", "meeting", "email"},
		"communication": {"发消息", "打电话", "微信", "message", "call"},
		"system":        {"设置", "打开", "关闭", "settings", "open"},
	}

	handlerPatterns := map[string]string{
		"book_flight":    "机票|航班|flight",
		"book_hotel":     "酒店|住宿|hotel",
		"order_food":     "订餐|外卖|order food",
		"play_music":     "播放音乐|听歌|play music",
		"send_message":   "发消息|发送|send message",
		"make_call":      "打电话|拨打|call",
		"set_reminder":   "提醒|remind",
		"control_device": "打开|关闭|open|close",
	}

	bestDomain := ""
	bestHandler := ""
	bestConfidence := 0.0

	for _, domain := range domains {
		if keywords, ok := domainKeywords[domain]; ok {
			for _, kw := range keywords {
				if strings.Contains(inputLower, strings.ToLower(kw)) {
					bestDomain = domain
					bestConfidence = 0.8
					break
				}
			}
		}
	}

	for handler, pattern := range handlerPatterns {
		if strings.Contains(inputLower, pattern) {
			bestHandler = handler
			if bestConfidence < 0.9 {
				bestConfidence = 0.9
			}
			break
		}
	}

	return map[string]interface{}{
		"domain":     bestDomain,
		"handler":    bestHandler,
		"confidence": bestConfidence,
	}
}

// =============================================================================
// Device State Node - Query device state
// =============================================================================

type DeviceStateNode struct{}

func (n *DeviceStateNode) Name() string { return "device_state" }
func (n *DeviceStateNode) Description() string {
	return "Query device state: battery, network, location"
}
func (n *DeviceStateNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "device_state",
		Description: "Query device state: battery, network, location, apps, storage. Context awareness for AI assistants.",
		Input:       "string - optional query filter",
		Output:      "string - device state information in JSON",
		Params: []ParamSchema{
			{Name: "query", Type: "string", Description: "State query: battery, network, location, apps, storage, all (default: all)", Required: false, Default: "all"},
		},
	}
}

func (n *DeviceStateNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	query := getMobileParam(params, "query", "all")

	state := map[string]interface{}{
		"type":    "device_state",
		"query":   query,
		"version": "1.0",
	}

	state["battery"] = map[string]interface{}{"level": "<filled_by_platform>"}
	state["network"] = map[string]interface{}{"type": "<filled_by_platform>"}
	state["location"] = map[string]interface{}{"latitude": "<filled_by_platform>"}

	result, _ := json.MarshalIndent(state, "", "  ")
	return fmt.Sprintf("Device state query:\n%s", string(result)), nil
}

// =============================================================================
// Cross-App Action Node - Actions that span multiple apps
// =============================================================================

type CrossAppActionNode struct{}

func (n *CrossAppActionNode) Name() string        { return "cross_app_action" }
func (n *CrossAppActionNode) Description() string { return "Execute actions across multiple apps" }
func (n *CrossAppActionNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "cross_app_action",
		Description: "Execute actions across multiple apps. Multi-app workflows for AI assistants.",
		Input:       "string - action description or context",
		Output:      "string - cross-app action result",
		Params: []ParamSchema{
			{Name: "workflow", Type: "string", Description: "Workflow name: share_content, save_for_later, compare_prices", Required: true},
			{Name: "apps", Type: "string", Description: "Comma-separated apps involved (e.g., 'wechat,alipay,camera')", Required: false},
			{Name: "data", Type: "string", Description: "JSON data to pass between apps", Required: false},
		},
	}
}

func (n *CrossAppActionNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	workflow := params["workflow"]
	if workflow == "" {
		return "", fmt.Errorf("workflow parameter is required")
	}

	apps := strings.Split(params["apps"], ",")

	action := map[string]interface{}{
		"type":     "cross_app_action",
		"workflow": workflow,
		"input":    input,
		"version":  "1.0",
	}

	if len(apps) > 0 && apps[0] != "" {
		action["apps"] = apps
	}

	steps := generateCrossAppSteps(workflow, apps, input)
	action["steps"] = steps

	result, _ := json.MarshalIndent(action, "", "  ")
	return fmt.Sprintf("Cross-app action plan:\n%s", string(result)), nil
}

func generateCrossAppSteps(workflow string, apps []string, input string) []map[string]interface{} {
	switch workflow {
	case "share_content":
		return []map[string]interface{}{
			{"step": 1, "node": "ui_automate", "params": map[string]string{"action": "screenshot"}},
			{"step": 2, "node": "app_launch", "params": map[string]string{"app": apps[0], "action": "share"}},
		}
	case "save_for_later":
		return []map[string]interface{}{
			{"step": 1, "node": "ui_automate", "params": map[string]string{"action": "screenshot"}},
			{"step": 2, "node": "file_write", "params": map[string]string{"path": "saved_content.json"}},
		}
	default:
		return []map[string]interface{}{
			{"step": 1, "node": "agent", "params": map[string]string{"system": "Plan cross-app workflow"}},
		}
	}
}

// Helper function to avoid name collision with existing getParam
func getMobileParam(params map[string]string, key, defaultVal string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultVal
}
