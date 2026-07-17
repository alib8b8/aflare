package nodes

import (
	"context"
	"encoding/json"
	"fmt"
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
)

func init() {
	Register(&AppLaunchNode{})
	Register(&UIAutomateNode{})
	Register(&NotificationNode{})
	Register(&IntentRouterNode{})
	Register(&DeviceStateNode{})
	Register(&CrossAppActionNode{})
	Register(&AgentMessageNode{})
	Register(&AgentInboxNode{})
}

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
			{Name: "uri", Type: "string", Description: "Deep link URI or URL scheme", Required: false},
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
		"type":    "app_launch",
		"app":     app,
		"action":  action,
		"wait":    getMobileParam(params, "wait", "true") == "true",
		"input":   truncateInput(input, 1000),
		"version": "1.0",
	}

	if uri != "" {
		intent["uri"] = uri
	}
	if parsedParams != nil {
		intent["params"] = parsedParams
	}

	result, _ := json.MarshalIndent(intent, "", "  ")
	return fmt.Sprintf("App launch intent created:\n%s", string(result)), nil
}

func validateAppURI(uri string) error {
	if len(uri) > 4096 {
		return fmt.Errorf("URI too long")
	}
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
	return s[:maxLen] + "... (truncated)"
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

	msg := map[string]interface{}{
		"type":      "agent_message",
		"from":      fromDID,
		"to":        toDID,
		"subject":   getMobileParam(params, "subject", ""),
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
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		return fmt.Errorf("endpoint must use http or https")
	}
	return nil
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
