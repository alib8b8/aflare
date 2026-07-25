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

package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	validAppNames = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	validActions  = map[string]bool{
		"click": true, "long_click": true, "type": true,
		"scroll": true, "swipe": true, "pinch": true, "drag": true,
		"wait_element": true, "screenshot": true, "open": true,
		"search": true, "share": true, "edit": true, "view": true,
	}
	validWorkflows = map[string]bool{
		"share_content": true, "save_for_later": true,
		"compare_prices": true, "book_and_add_calendar": true,
	}
	// 鸿蒙 Ability 类型白名单
	validHarmonyAbilityTypes = map[string]bool{
		"page": true, "slice": true, "service": true, "data": true,
	}
	// 鸿蒙原子化服务动作白名单
	validHarmonyAtomicActions = map[string]bool{
		"launch": true, "router": true, "share": true, "notify": true,
	}
)

// Platform 表示目标运行平台
type Platform string

const (
	PlatformAndroid Platform = "android"
	PlatformIOS     Platform = "ios"
	PlatformHarmony Platform = "harmony"
	PlatformDesktop Platform = "desktop"
	PlatformUnknown Platform = "unknown"
)

// DetectPlatform 根据环境变量检测当前运行平台
func DetectPlatform() Platform {
	// 鸿蒙检测：OHOS_ROOT 或 /system/etc/param/ohos.para
	if os.Getenv("OHOS_ROOT") != "" || os.Getenv("HOS_ROOT") != "" {
		return PlatformHarmony
	}
	if _, err := os.Stat("/system/etc/param/ohos.para"); err == nil {
		return PlatformHarmony
	}
	// Android 检测
	if os.Getenv("ANDROID_ROOT") != "" || os.Getenv("TERMUX_VERSION") != "" {
		return PlatformAndroid
	}
	// iOS 检测（越狱环境）
	if os.Getenv("CFFIXED_USER_HOME") != "" && os.Getenv("HOME") == "/var/mobile" {
		return PlatformIOS
	}
	return PlatformDesktop
}

func init() {
	Register(&AppLaunchNode{})
	Register(&UIAutomateNode{})
	Register(&NotificationNode{})
	Register(&IntentRouterNode{})
	Register(&DeviceStateNode{})
	Register(&CrossAppActionNode{})
	Register(&AgentMessageNode{})
	Register(&AgentInboxNode{})
	Register(&HarmonyAbilityNode{})
	Register(&HarmonyAtomicServiceNode{})
	Register(&HarmonyWidgetNode{})
	Register(&HarmonyDeviceAdaptNode{})
}

type AppLaunchNode struct{}

func (n *AppLaunchNode) Name() string { return "app_launch" }
func (n *AppLaunchNode) Description() string {
	return "Launch a mobile/desktop app with optional parameters"
}
func (n *AppLaunchNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "app_launch",
		Description: "Launch a mobile/desktop app with optional parameters. Cross-platform app control for AI systems. Supports Android, iOS, and HarmonyOS.",
		Input:       "string - optional input to pass to the app",
		Output:      "string - launch result and app session info",
		Params: []ParamSchema{
			{Name: "app", Type: "string", Description: "App identifier: package name (Android), bundle ID (iOS), bundleName (HarmonyOS), app name (desktop)", Required: true},
			{Name: "action", Type: "string", Description: "Deep link action: open, search, share, edit, view (default: open)", Required: false, Default: "open"},
			{Name: "uri", Type: "string", Description: "Deep link URI or URL scheme. HarmonyOS uses ohos:// or host:// scheme", Required: false},
			{Name: "platform", Type: "string", Description: "Target platform: android, ios, harmony, desktop (auto-detected if omitted)", Required: false},
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

	if !validAppNames.MatchString(app) {
		return "", fmt.Errorf("invalid app name: contains invalid characters")
	}

	action := getMobileParam(params, "action", "open")
	if !validActions[action] {
		return "", fmt.Errorf("invalid action: %s (allowed: open, search, share, edit, view)", action)
	}

	// 检测目标平台
	platform := getMobileParam(params, "platform", string(DetectPlatform()))
	validPlatforms := map[string]bool{"android": true, "ios": true, "harmony": true, "desktop": true}
	if !validPlatforms[platform] {
		return "", fmt.Errorf("invalid platform: %s (allowed: android, ios, harmony, desktop)", platform)
	}

	uri := params["uri"]
	if uri != "" {
		if err := validateAppURI(uri); err != nil {
			return "", err
		}
	}

	extraParams := params["params"]
	var parsedParams map[string]interface{}
	if extraParams != "" {
		if err := json.Unmarshal([]byte(extraParams), &parsedParams); err != nil {
			return "", fmt.Errorf("invalid params JSON: %w", err)
		}
		if err := validateAppParams(parsedParams); err != nil {
			return "", err
		}
	}

	intent := map[string]interface{}{
		"type":     "app_launch",
		"app":      app,
		"action":   action,
		"platform": platform,
		"wait":     getMobileParam(params, "wait", "true") == "true",
		"input":    truncateInput(input, 1000),
		"version":  "1.0",
	}

	if uri != "" {
		intent["uri"] = uri
	}
	if parsedParams != nil {
		intent["params"] = parsedParams
	}

	// 鸿蒙特有字段
	if platform == "harmony" {
		intent["harmony"] = map[string]interface{}{
			"bundle_name":  app,
			"ability_type": "page",
			"uri_scheme":   "ohos://",
		}
	}

	result, _ := json.MarshalIndent(intent, "", "  ")
	return fmt.Sprintf("App launch intent created:\n%s", string(result)), nil
}

func validateAppURI(uri string) error {
	if len(uri) > 4096 {
		return fmt.Errorf("URI too long")
	}
	// 允许的 URI scheme：标准 scheme + 鸿蒙 ohos://
	disallowedSchemes := []string{"file://", "data://", "javascript://", "ftp://"}
	for _, scheme := range disallowedSchemes {
		if strings.HasPrefix(strings.ToLower(uri), scheme) {
			return fmt.Errorf("URI scheme not allowed: %s", scheme)
		}
	}
	return nil
}

func validateAppParams(params map[string]interface{}) error {
	for k, v := range params {
		if len(k) > 100 {
			return fmt.Errorf("parameter key too long")
		}
		switch val := v.(type) {
		case string:
			if len(val) > 1000 {
				return fmt.Errorf("parameter value too long for key: %s", k)
			}
		case map[string]interface{}:
			if err := validateAppParams(val); err != nil {
				return err
			}
		case []interface{}:
			if len(val) > 100 {
				return fmt.Errorf("array too long for key: %s", k)
			}
		}
	}
	return nil
}

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
			{Name: "selector", Type: "string", Description: "Element selector: id, text, content_desc, class", Required: false},
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

	if !validActions[action] {
		return "", fmt.Errorf("invalid action: %s (allowed: click, long_click, type, scroll, swipe, pinch, drag, wait_element, screenshot)", action)
	}

	selector := params["selector"]
	if selector != "" && len(selector) > 500 {
		return "", fmt.Errorf("selector too long")
	}

	text := getMobileParam(params, "text", input)
	if len(text) > 2000 {
		text = text[:2000]
	}

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

	if len(title) > 100 {
		title = title[:100]
	}

	body := getMobileParam(params, "body", input)
	if body == "" {
		body = input
	}
	if len(body) > 1000 {
		body = body[:1000]
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
		if err := json.Unmarshal([]byte(actions), &parsed); err != nil {
			return "", fmt.Errorf("invalid actions JSON: %w", err)
		}
		if len(parsed) > 3 {
			return "", fmt.Errorf("too many action buttons (max 3)")
		}
		for _, action := range parsed {
			if _, ok := action["id"]; !ok || action["id"] == "" {
				return "", fmt.Errorf("action button requires 'id' field")
			}
			if _, ok := action["title"]; !ok || action["title"] == "" {
				return "", fmt.Errorf("action button requires 'title' field")
			}
			if len(action["id"]) > 50 {
				return "", fmt.Errorf("action button id too long")
			}
			if len(action["title"]) > 20 {
				return "", fmt.Errorf("action button title too long")
			}
		}
		notif["actions"] = parsed
	}

	result, _ := json.MarshalIndent(notif, "", "  ")
	return fmt.Sprintf("Notification prepared:\n%s", string(result)), nil
}

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
	validModes := map[string]bool{"classify": true, "route": true, "execute": true}
	if !validModes[mode] {
		return "", fmt.Errorf("invalid mode: %s (allowed: classify, route, execute)", mode)
	}

	domains := strings.Split(getMobileParam(params, "domains", "travel,food,shopping,entertainment,work,communication,system"), ",")
	validDomains := map[string]bool{
		"travel": true, "food": true, "shopping": true,
		"entertainment": true, "work": true, "communication": true, "system": true,
	}
	for _, d := range domains {
		if d != "" && !validDomains[d] {
			return "", fmt.Errorf("invalid domain: %s", d)
		}
	}

	fallback := getMobileParam(params, "fallback", "general_assistant")
	if len(fallback) > 100 {
		return "", fmt.Errorf("fallback handler name too long")
	}

	intent := classifyIntent(input, domains)

	result := map[string]interface{}{
		"type":           "intent_router",
		"mode":           mode,
		"input":          truncateInput(input, 500),
		"classification": intent,
		"handler":        intent["handler"],
		"fallback":       fallback,
		"version":        "1.0",
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

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

	validQueries := map[string]bool{"battery": true, "network": true, "location": true, "apps": true, "storage": true, "all": true}
	if !validQueries[query] {
		return "", fmt.Errorf("invalid query: %s (allowed: battery, network, location, apps, storage, all)", query)
	}

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
			{Name: "apps", Type: "string", Description: "Comma-separated apps involved", Required: false},
			{Name: "data", Type: "string", Description: "JSON data to pass between apps", Required: false},
		},
	}
}

func (n *CrossAppActionNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	workflow := params["workflow"]
	if workflow == "" {
		return "", fmt.Errorf("workflow parameter is required")
	}

	if !validWorkflows[workflow] {
		return "", fmt.Errorf("invalid workflow: %s (allowed: share_content, save_for_later, compare_prices, book_and_add_calendar)", workflow)
	}

	apps := strings.Split(params["apps"], ",")
	for _, app := range apps {
		if app != "" && !validAppNames.MatchString(app) {
			return "", fmt.Errorf("invalid app name: %s", app)
		}
	}

	var parsedData map[string]interface{}
	if data := params["data"]; data != "" {
		if err := json.Unmarshal([]byte(data), &parsedData); err != nil {
			return "", fmt.Errorf("invalid data JSON: %w", err)
		}
		if err := validateAppParams(parsedData); err != nil {
			return "", err
		}
	}

	action := map[string]interface{}{
		"type":     "cross_app_action",
		"workflow": workflow,
		"input":    truncateInput(input, 500),
		"version":  "1.0",
	}

	if len(apps) > 0 && apps[0] != "" {
		action["apps"] = apps
	}
	if parsedData != nil {
		action["data"] = parsedData
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

func getMobileParam(params map[string]string, key, defaultVal string) string {
	if v, ok := params[key]; ok && v != "" {
		return v
	}
	return defaultVal
}

func truncateInput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s[:maxLen]
	}
	return string(r[:maxLen]) + "..."
}

// sanitizeEcho 过滤控制字符并截断用户输入，防止错误消息泄露或注入控制序列。
func sanitizeEcho(s string) string {
	r := []rune(s)
	out := make([]rune, 0, len(r))
	for _, c := range r {
		if c < 0x20 || c == 0x7f {
			continue
		}
		out = append(out, c)
		if len(out) >= 100 {
			out = append(out, '.', '.', '.')
			break
		}
	}
	return string(out)
}

// AgentMessageNode sends cross-domain messages to other agents,
// inspired by awiki.ai's agent-native messaging.
type AgentMessageNode struct{}

func (n *AgentMessageNode) Name() string        { return "agent_message" }
func (n *AgentMessageNode) Description() string { return "Send message to another agent by DID" }
func (n *AgentMessageNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "agent_message",
		Description: "Send cross-domain message to another agent by DID. Inspired by awiki.ai agent-native messaging.",
		Input:       "string - message body or payload",
		Output:      "string - send result",
		Params: []ParamSchema{
			{Name: "to_did", Type: "string", Description: "Receiver agent DID (e.g. did:awiki:agent123)", Required: true},
			{Name: "from_did", Type: "string", Description: "Sender agent DID", Required: false},
			{Name: "subject", Type: "string", Description: "Message subject/type", Required: false},
			{Name: "priority", Type: "string", Description: "Priority: low, normal, high, urgent (default: normal)", Required: false, Default: "normal"},
			{Name: "endpoint", Type: "string", Description: "Target agent endpoint URL (optional, for direct send)", Required: false},
		},
	}
}

func (n *AgentMessageNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	toDID := params["to_did"]
	if toDID == "" {
		return "", fmt.Errorf("to_did parameter is required")
	}
	if !strings.HasPrefix(toDID, "did:") {
		return "", fmt.Errorf("invalid to_did format: must start with 'did:'")
	}
	if len(toDID) > 256 {
		return "", fmt.Errorf("to_did too long")
	}

	fromDID := getMobileParam(params, "from_did", "")
	if fromDID != "" && !strings.HasPrefix(fromDID, "did:") {
		return "", fmt.Errorf("invalid from_did format")
	}

	priority := getMobileParam(params, "priority", "normal")
	validPriorities := map[string]bool{"low": true, "normal": true, "high": true, "urgent": true}
	if !validPriorities[priority] {
		return "", fmt.Errorf("invalid priority: %s", priority)
	}

	body := input
	if len(body) > 5000 {
		body = body[:5000] + "... (truncated)"
	}

	subject := getMobileParam(params, "subject", "")
	if len(subject) > 200 {
		subject = subject[:200] + "... (truncated)"
	}

	msg := map[string]interface{}{
		"type":      "agent_message",
		"from":      fromDID,
		"to":        toDID,
		"subject":   subject,
		"body":      body,
		"priority":  priority,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0",
	}

	endpoint := params["endpoint"]
	if endpoint != "" {
		if err := validateAgentEndpoint(endpoint); err != nil {
			return "", err
		}
		msg["endpoint"] = endpoint
	}

	result, _ := json.MarshalIndent(msg, "", "  ")
	return fmt.Sprintf("Agent message prepared:\n%s", string(result)), nil
}

func validateAgentEndpoint(endpoint string) error {
	if len(endpoint) > 2048 {
		return fmt.Errorf("endpoint too long")
	}
	// Use the existing SSRF-safe URL validator from security.go
	return validateURL(endpoint)
}

// AgentInboxNode queries or manages the local agent's message inbox.
type AgentInboxNode struct{}

func (n *AgentInboxNode) Name() string        { return "agent_inbox" }
func (n *AgentInboxNode) Description() string { return "Query agent message inbox" }
func (n *AgentInboxNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "agent_inbox",
		Description: "Query agent message inbox. Retrieve and manage cross-domain messages.",
		Input:       "string - optional filter query",
		Output:      "string - inbox messages in JSON",
		Params: []ParamSchema{
			{Name: "action", Type: "string", Description: "Action: list, read, delete, mark_read (default: list)", Required: false, Default: "list"},
			{Name: "message_id", Type: "string", Description: "Message ID for read/delete/mark_read", Required: false},
			{Name: "from_did", Type: "string", Description: "Filter by sender DID", Required: false},
			{Name: "limit", Type: "string", Description: "Max messages to return (default: 10)", Required: false, Default: "10"},
		},
	}
}

func (n *AgentInboxNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	action := getMobileParam(params, "action", "list")
	validActions := map[string]bool{"list": true, "read": true, "delete": true, "mark_read": true}
	if !validActions[action] {
		return "", fmt.Errorf("invalid action: %s (allowed: list, read, delete, mark_read)", action)
	}

	if action != "list" {
		msgID := params["message_id"]
		if msgID == "" {
			return "", fmt.Errorf("message_id is required for %s action", action)
		}
		if len(msgID) > 128 {
			return "", fmt.Errorf("message_id too long")
		}
	}

	fromDID := params["from_did"]
	if fromDID != "" && !strings.HasPrefix(fromDID, "did:") {
		return "", fmt.Errorf("invalid from_did format")
	}

	limit := 10
	if v := params["limit"]; v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	inbox := map[string]interface{}{
		"type":    "agent_inbox",
		"action":  action,
		"limit":   limit,
		"version": "1.0",
	}

	if fromDID != "" {
		inbox["filter_from"] = fromDID
	}
	if action != "list" {
		inbox["message_id"] = params["message_id"]
	}

	// Placeholder: real implementation would query a message store
	inbox["messages"] = []map[string]interface{}{}
	inbox["total"] = 0

	result, _ := json.MarshalIndent(inbox, "", "  ")
	return fmt.Sprintf("Agent inbox query:\n%s", string(result)), nil
}

// HarmonyAbilityNode 启动鸿蒙 Ability，支持 page/slice/service/data 四种类型
type HarmonyAbilityNode struct{}

func (n *HarmonyAbilityNode) Name() string { return "harmony_ability" }
func (n *HarmonyAbilityNode) Description() string {
	return "Launch HarmonyOS Ability (page, slice, service, data)"
}
func (n *HarmonyAbilityNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "harmony_ability",
		Description: "Launch HarmonyOS Ability with specified type. Supports page (UI), slice (partial UI), service (background), data (data provider).",
		Input:       "string - optional input data for the ability",
		Output:      "string - ability launch result",
		Params: []ParamSchema{
			{Name: "bundle_name", Type: "string", Description: "HarmonyOS bundle name (e.g. com.example.myapplication)", Required: true},
			{Name: "ability_name", Type: "string", Description: "Ability class name (e.g. MainAbility)", Required: true},
			{Name: "ability_type", Type: "string", Description: "Ability type: page, slice, service, data (default: page)", Required: false, Default: "page"},
			{Name: "uri", Type: "string", Description: "Deep link URI using ohos:// scheme", Required: false},
			{Name: "params", Type: "string", Description: "JSON parameters for the ability", Required: false},
		},
	}
}

func (n *HarmonyAbilityNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	bundleName := params["bundle_name"]
	if bundleName == "" {
		return "", fmt.Errorf("bundle_name parameter is required")
	}
	if !validAppNames.MatchString(bundleName) {
		return "", fmt.Errorf("invalid bundle_name: contains invalid characters")
	}
	if len(bundleName) > 256 {
		return "", fmt.Errorf("bundle_name too long (max 256)")
	}

	abilityName := params["ability_name"]
	if abilityName == "" {
		return "", fmt.Errorf("ability_name parameter is required")
	}
	if !validAppNames.MatchString(abilityName) {
		return "", fmt.Errorf("invalid ability_name: contains invalid characters")
	}
	if len(abilityName) > 128 {
		return "", fmt.Errorf("ability_name too long (max 128)")
	}

	abilityType := getMobileParam(params, "ability_type", "page")
	if !validHarmonyAbilityTypes[abilityType] {
		return "", fmt.Errorf("invalid ability_type: %s (allowed: page, slice, service, data)", abilityType)
	}

	uri := params["uri"]
	if uri != "" {
		if err := validateAppURI(uri); err != nil {
			return "", err
		}
	}

	var parsedParams map[string]interface{}
	if p := params["params"]; p != "" {
		if err := json.Unmarshal([]byte(p), &parsedParams); err != nil {
			return "", fmt.Errorf("invalid params JSON: %w", err)
		}
		if err := validateAppParams(parsedParams); err != nil {
			return "", err
		}
	}

	intent := map[string]interface{}{
		"type":         "harmony_ability",
		"bundle_name":  bundleName,
		"ability_name": abilityName,
		"ability_type": abilityType,
		"uri_scheme":   "ohos://",
		"input":        truncateInput(input, 1000),
		"version":      "1.0",
	}
	if uri != "" {
		intent["uri"] = uri
	}
	if parsedParams != nil {
		intent["params"] = parsedParams
	}

	result, _ := json.MarshalIndent(intent, "", "  ")
	return fmt.Sprintf("HarmonyOS Ability launch intent:\n%s", string(result)), nil
}

// HarmonyAtomicServiceNode 启动鸿蒙原子化服务
type HarmonyAtomicServiceNode struct{}

func (n *HarmonyAtomicServiceNode) Name() string { return "harmony_atomic_service" }
func (n *HarmonyAtomicServiceNode) Description() string {
	return "Launch HarmonyOS Atomic Service (card-based lightweight app)"
}
func (n *HarmonyAtomicServiceNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "harmony_atomic_service",
		Description: "Launch HarmonyOS Atomic Service. Lightweight, card-based services that run without installation. Supports launch, router, share, notify actions.",
		Input:       "string - optional input data",
		Output:      "string - atomic service launch result",
		Params: []ParamSchema{
			{Name: "service_id", Type: "string", Description: "Atomic service ID (e.g. com.example.service)", Required: true},
			{Name: "action", Type: "string", Description: "Action: launch, router, share, notify (default: launch)", Required: false, Default: "launch"},
			{Name: "card_id", Type: "string", Description: "Service card ID for widget-style display", Required: false},
			{Name: "params", Type: "string", Description: "JSON parameters for the service", Required: false},
		},
	}
}

func (n *HarmonyAtomicServiceNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	serviceID := params["service_id"]
	if serviceID == "" {
		return "", fmt.Errorf("service_id parameter is required")
	}
	if !validAppNames.MatchString(serviceID) {
		return "", fmt.Errorf("invalid service_id: contains invalid characters")
	}
	if len(serviceID) > 256 {
		return "", fmt.Errorf("service_id too long (max 256)")
	}

	action := getMobileParam(params, "action", "launch")
	if !validHarmonyAtomicActions[action] {
		return "", fmt.Errorf("invalid action: %s (allowed: launch, router, share, notify)", action)
	}

	cardID := params["card_id"]
	if cardID != "" {
		if !validAppNames.MatchString(cardID) {
			return "", fmt.Errorf("invalid card_id: contains invalid characters")
		}
		if len(cardID) > 128 {
			return "", fmt.Errorf("card_id too long (max 128)")
		}
	}

	var parsedParams map[string]interface{}
	if p := params["params"]; p != "" {
		if err := json.Unmarshal([]byte(p), &parsedParams); err != nil {
			return "", fmt.Errorf("invalid params JSON: %w", err)
		}
		if err := validateAppParams(parsedParams); err != nil {
			return "", err
		}
	}

	intent := map[string]interface{}{
		"type":       "harmony_atomic_service",
		"service_id": serviceID,
		"action":     action,
		"input":      truncateInput(input, 1000),
		"version":    "1.0",
	}
	if cardID != "" {
		intent["card_id"] = cardID
	}
	if parsedParams != nil {
		intent["params"] = parsedParams
	}

	result, _ := json.MarshalIndent(intent, "", "  ")
	return fmt.Sprintf("HarmonyOS Atomic Service intent:\n%s", string(result)), nil
}

// HarmonyWidgetNode 管理鸿蒙桌面卡片（Widget）
type HarmonyWidgetNode struct{}

func (n *HarmonyWidgetNode) Name() string { return "harmony_widget" }
func (n *HarmonyWidgetNode) Description() string {
	return "Manage HarmonyOS desktop widgets (cards)"
}
func (n *HarmonyWidgetNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "harmony_widget",
		Description: "Manage HarmonyOS desktop widgets (service cards). Add, update, remove, or query widget state on the home screen.",
		Input:       "string - optional widget content or query",
		Output:      "string - widget operation result",
		Params: []ParamSchema{
			{Name: "action", Type: "string", Description: "Action: add, update, remove, query (default: query)", Required: false, Default: "query"},
			{Name: "widget_id", Type: "string", Description: "Widget identifier (required for update, remove, query)", Required: false},
			{Name: "provider_bundle", Type: "string", Description: "Provider bundle name for add action", Required: false},
			{Name: "widget_name", Type: "string", Description: "Widget ability name for add action", Required: false},
			{Name: "data", Type: "string", Description: "JSON data to update widget content", Required: false},
		},
	}
}

func (n *HarmonyWidgetNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	action := getMobileParam(params, "action", "query")
	validWidgetActions := map[string]bool{"add": true, "update": true, "remove": true, "query": true}
	if !validWidgetActions[action] {
		return "", fmt.Errorf("invalid action: %s (allowed: add, update, remove, query)", action)
	}

	widgetID := params["widget_id"]
	if action != "add" {
		if widgetID == "" {
			return "", fmt.Errorf("widget_id is required for %s action", action)
		}
		if !validAppNames.MatchString(widgetID) {
			return "", fmt.Errorf("invalid widget_id: contains invalid characters")
		}
		if len(widgetID) > 128 {
			return "", fmt.Errorf("widget_id too long (max 128)")
		}
	}

	providerBundle := params["provider_bundle"]
	widgetName := params["widget_name"]
	if action == "add" {
		if providerBundle == "" {
			return "", fmt.Errorf("provider_bundle is required for add action")
		}
		if !validAppNames.MatchString(providerBundle) {
			return "", fmt.Errorf("invalid provider_bundle")
		}
		if widgetName == "" {
			return "", fmt.Errorf("widget_name is required for add action")
		}
		if !validAppNames.MatchString(widgetName) {
			return "", fmt.Errorf("invalid widget_name")
		}
	}

	var parsedData map[string]interface{}
	if d := params["data"]; d != "" {
		if err := json.Unmarshal([]byte(d), &parsedData); err != nil {
			return "", fmt.Errorf("invalid data JSON: %w", err)
		}
		if err := validateAppParams(parsedData); err != nil {
			return "", err
		}
	}

	intent := map[string]interface{}{
		"type":    "harmony_widget",
		"action":  action,
		"version": "1.0",
	}
	if widgetID != "" {
		intent["widget_id"] = widgetID
	}
	if providerBundle != "" {
		intent["provider_bundle"] = providerBundle
	}
	if widgetName != "" {
		intent["widget_name"] = widgetName
	}
	if parsedData != nil {
		intent["data"] = parsedData
	}
	if input != "" {
		intent["input"] = truncateInput(input, 500)
	}

	result, _ := json.MarshalIndent(intent, "", "  ")
	return fmt.Sprintf("HarmonyOS Widget operation:\n%s", string(result)), nil
}

// ============================================================
// 鸿蒙多设备适配检测（借鉴 HarmonyOS Agent Skills 的多设备适配能力）
// 支持：直板机、双折叠、三折叠、平板、智慧屏、车机、穿戴
// ============================================================

// HarmonyDeviceType 鸿蒙设备类型
type HarmonyDeviceType string

const (
	DevicePhoneStandard   HarmonyDeviceType = "phone_standard"    // 直板机
	DevicePhoneDualFold   HarmonyDeviceType = "phone_dual_fold"   // 双折叠
	DevicePhoneTripleFold HarmonyDeviceType = "phone_triple_fold" // 三折叠
	DeviceTablet          HarmonyDeviceType = "tablet"            // 平板
	DeviceSmartScreen     HarmonyDeviceType = "smart_screen"      // 智慧屏
	DeviceCar             HarmonyDeviceType = "car"               // 车机
	DeviceWearable        HarmonyDeviceType = "wearable"          // 穿戴设备
	DeviceUnknown         HarmonyDeviceType = "unknown"
)

// HarmonyDeviceInfo 鸿蒙设备信息
type HarmonyDeviceInfo struct {
	Type          HarmonyDeviceType `json:"type"`
	ScreenWidth   int               `json:"screen_width"`
	ScreenHeight  int               `json:"screen_height"`
	ScreenDensity float64           `json:"screen_density"`
	IsFoldable    bool              `json:"is_foldable"`
	FoldState     string            `json:"fold_state,omitempty"`
	Orientation   string            `json:"orientation,omitempty"`
	Capabilities  []string          `json:"capabilities,omitempty"`
}

// HarmonyDeviceAdaptNode 鸿蒙多设备适配检测节点
type HarmonyDeviceAdaptNode struct{}

func (n *HarmonyDeviceAdaptNode) Name() string { return "harmony_device_adapt" }
func (n *HarmonyDeviceAdaptNode) Description() string {
	return "Detect HarmonyOS device type and provide adaptation guidance"
}
func (n *HarmonyDeviceAdaptNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "harmony_device_adapt",
		Description: "Detect HarmonyOS device type (phone/tablet/foldable/TV/car/wearable) and generate UI adaptation guidance. Inspired by HarmonyOS Agent Skills multi-device adaptation.",
		Input:       "string - optional adaptation requirements",
		Output:      "string - device info and adaptation plan in JSON",
		Params: []ParamSchema{
			{Name: "screen_width", Type: "string", Description: "Screen width in pixels (optional, auto-detected if omitted)", Required: false},
			{Name: "screen_height", Type: "string", Description: "Screen height in pixels (optional)", Required: false},
			{Name: "screen_density", Type: "string", Description: "Screen density (dpi, optional)", Required: false},
			{Name: "device_type", Type: "string", Description: "Device type hint: phone_standard, phone_dual_fold, phone_triple_fold, tablet, smart_screen, car, wearable (auto-detected if omitted)", Required: false},
			{Name: "fold_state", Type: "string", Description: "Fold state for foldable devices: unfolded, half_folded, fully_folded (optional)", Required: false},
			{Name: "orientation", Type: "string", Description: "Orientation: portrait, landscape, auto (default: auto)", Required: false, Default: "auto"},
		},
	}
}

func (n *HarmonyDeviceAdaptNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	// 早期截断，避免过长输入流向下游
	input = truncateInput(input, 500)
	// 解析设备类型
	deviceType := HarmonyDeviceType(getMobileParam(params, "device_type", string(DevicePhoneStandard)))
	validDeviceTypes := map[HarmonyDeviceType]bool{
		DevicePhoneStandard: true, DevicePhoneDualFold: true, DevicePhoneTripleFold: true,
		DeviceTablet: true, DeviceSmartScreen: true, DeviceCar: true, DeviceWearable: true,
	}
	if !validDeviceTypes[deviceType] {
		return "", fmt.Errorf("invalid device_type: %s", sanitizeEcho(string(deviceType)))
	}

	// 解析屏幕参数
	screenWidth := parseIntSafe(getMobileParam(params, "screen_width", "1080"), 1080)
	screenHeight := parseIntSafe(getMobileParam(params, "screen_height", "2400"), 2400)
	screenDensity := parseFloatSafe(getMobileParam(params, "screen_density", "3.0"), 3.0)

	// 限制参数范围
	if screenWidth < 100 || screenWidth > 10000 {
		screenWidth = 1080
	}
	if screenHeight < 100 || screenHeight > 10000 {
		screenHeight = 2400
	}
	if screenDensity < 0.5 || screenDensity > 10.0 {
		screenDensity = 3.0
	}

	// 折叠状态验证
	foldState := params["fold_state"]
	if foldState != "" {
		validFoldStates := map[string]bool{"unfolded": true, "half_folded": true, "fully_folded": true}
		if !validFoldStates[foldState] {
			return "", fmt.Errorf("invalid fold_state: %s (allowed: unfolded, half_folded, fully_folded)", sanitizeEcho(foldState))
		}
	}
	// 一致性校验：仅折叠设备允许 fold_state
	if deviceType != DevicePhoneDualFold && deviceType != DevicePhoneTripleFold {
		foldState = ""
	}

	// 方向验证
	orientation := getMobileParam(params, "orientation", "auto")
	validOrientations := map[string]bool{"portrait": true, "landscape": true, "auto": true}
	if !validOrientations[orientation] {
		return "", fmt.Errorf("invalid orientation: %s (allowed: portrait, landscape, auto)", sanitizeEcho(orientation))
	}

	// 构建设备信息
	info := HarmonyDeviceInfo{
		Type:          deviceType,
		ScreenWidth:   screenWidth,
		ScreenHeight:  screenHeight,
		ScreenDensity: screenDensity,
		IsFoldable:    deviceType == DevicePhoneDualFold || deviceType == DevicePhoneTripleFold,
		FoldState:     foldState,
		Orientation:   orientation,
		Capabilities:  getDeviceCapabilities(deviceType),
	}

	// 生成适配建议
	adaptation := generateAdaptationPlan(info, input)

	result := map[string]interface{}{
		"type":        "harmony_device_adapt",
		"device_info": info,
		"adaptation":  adaptation,
		"version":     "1.0",
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal failed: %w", err)
	}
	return fmt.Sprintf("HarmonyOS device adaptation:\n%s", string(output)), nil
}

func getDeviceCapabilities(dt HarmonyDeviceType) []string {
	switch dt {
	case DevicePhoneStandard:
		return []string{"touch", "camera", "gps", "bluetooth", "nfc", "biometrics"}
	case DevicePhoneDualFold, DevicePhoneTripleFold:
		return []string{"touch", "camera", "gps", "bluetooth", "nfc", "biometrics", "foldable_screen", "multi_window"}
	case DeviceTablet:
		return []string{"touch", "camera", "gps", "bluetooth", "stylus", "multi_window", "split_screen"}
	case DeviceSmartScreen:
		return []string{"touch", "camera", "bluetooth", "voice", "gesture", "remote_control"}
	case DeviceCar:
		return []string{"touch", "voice", "gps", "bluetooth", "steering_wheel_control", "hud"}
	case DeviceWearable:
		return []string{"touch", "bluetooth", "heart_rate", "accelerometer", "gyroscope", "voice"}
	default:
		return []string{"touch"}
	}
}

func generateAdaptationPlan(info HarmonyDeviceInfo, requirements string) map[string]interface{} {
	plan := map[string]interface{}{
		"layout_strategy":   getLayoutStrategy(info),
		"breakpoints":       getBreakpoints(info),
		"ui_components":     getUIComponents(info),
		"interaction_hints": getInteractionHints(info),
	}
	if info.IsFoldable {
		plan["fold_adaptation"] = getFoldAdaptation(info)
	}
	if requirements != "" {
		plan["custom_requirements"] = truncateInput(requirements, 500)
	}
	return plan
}

func getLayoutStrategy(info HarmonyDeviceInfo) string {
	switch info.Type {
	case DevicePhoneStandard:
		return "single_column"
	case DevicePhoneDualFold, DevicePhoneTripleFold:
		if info.FoldState == "half_folded" {
			return "dual_column_split"
		}
		return "adaptive_column"
	case DeviceTablet:
		return "dual_column_or_grid"
	case DeviceSmartScreen:
		return "large_card_grid"
	case DeviceCar:
		return "simplified_single_column"
	case DeviceWearable:
		return "minimal_single_column"
	default:
		return "single_column"
	}
}

func getBreakpoints(info HarmonyDeviceInfo) map[string]int {
	switch info.Type {
	case DevicePhoneStandard:
		return map[string]int{"sm": 320, "md": 600, "lg": 840}
	case DevicePhoneDualFold:
		return map[string]int{"sm": 320, "md": 600, "lg": 1200, "xl": 1800}
	case DevicePhoneTripleFold:
		return map[string]int{"sm": 320, "md": 600, "lg": 1200, "xl": 2400}
	case DeviceTablet:
		return map[string]int{"sm": 600, "md": 840, "lg": 1200}
	case DeviceSmartScreen:
		return map[string]int{"sm": 960, "md": 1280, "lg": 1920}
	case DeviceCar:
		return map[string]int{"sm": 800, "md": 1200, "lg": 1920}
	case DeviceWearable:
		return map[string]int{"sm": 200, "md": 300, "lg": 400}
	default:
		return map[string]int{"sm": 320, "md": 600, "lg": 840}
	}
}

func getUIComponents(info HarmonyDeviceInfo) []string {
	switch info.Type {
	case DevicePhoneStandard:
		return []string{"navigation_bar", "tab_bar", "list", "card", "dialog"}
	case DevicePhoneDualFold, DevicePhoneTripleFold:
		return []string{"navigation_rail", "tab_bar", "list", "card", "dialog", "side_bar", "drag_to_split"}
	case DeviceTablet:
		return []string{"navigation_rail", "tab_bar", "grid", "card", "dialog", "side_bar", "split_view"}
	case DeviceSmartScreen:
		return []string{"navigation_rail", "card_grid", "voice_search", "gesture_control"}
	case DeviceCar:
		return []string{"simplified_list", "voice_command", "large_touch_target", "quick_action"}
	case DeviceWearable:
		return []string{"crown_scroll", "minimal_list", "voice_input", "notification_card"}
	default:
		return []string{"navigation_bar", "list", "card"}
	}
}

func getInteractionHints(info HarmonyDeviceInfo) []string {
	switch info.Type {
	case DeviceCar:
		return []string{"minimize_text input", "use voice first", "large touch targets (min 48dp)", "no complex gestures"}
	case DeviceWearable:
		return []string{"use crown for scroll", "minimal text input", "voice input preferred", "short interactions (<5s)"}
	case DeviceSmartScreen:
		return []string{"support remote control", "voice search", "gesture navigation", "viewing distance >2m"}
	case DevicePhoneDualFold, DevicePhoneTripleFold:
		return []string{"support drag-to-split", "adaptive layout on fold/unfold", "multi-window awareness", "continuity across fold states"}
	default:
		return []string{"standard touch interaction"}
	}
}

func getFoldAdaptation(info HarmonyDeviceInfo) map[string]interface{} {
	return map[string]interface{}{
		"fold_type":     string(info.Type),
		"current_state": info.FoldState,
		"recommended":   "Use FlexContainer with adaptive breakpoints; listen to fold state changes via @ohos.display",
		"layout_switch": "single_column -> dual_column when unfolding",
		"animation":     "smooth transition (300ms) on fold state change",
	}
}

func parseIntSafe(s string, defaultVal int) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

func parseFloatSafe(s string, defaultVal float64) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return defaultVal
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return defaultVal
	}
	return v
}
