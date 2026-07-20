package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	validScreenActions = map[string]bool{
		"tap":        true,
		"double_tap": true,
		"long_press": true,
		"swipe":      true,
		"scroll":     true,
		"type":       true,
		"back":       true,
		"home":       true,
		"recent":     true,
	}
	validSwipeDirections = map[string]bool{
		"up":    true,
		"down":  true,
		"left":  true,
		"right": true,
	}
	uiElementPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\s]{1,100}$`)
)

// ScreenUnderstandingNode simulates L3-level screen content understanding for agent phones
type ScreenUnderstandingNode struct{}

func (n *ScreenUnderstandingNode) Name() string { return "screen_understanding" }

func (n *ScreenUnderstandingNode) Description() string {
	return "Understand screen content like an L3 agent: parse UI elements, identify actionable items, and generate interaction plans. Supports mobile app screens, web pages, and system UIs."
}

func (n *ScreenUnderstandingNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - screen description or OCR text dump",
		Output:      "string - structured screen analysis with interaction plan",
		Params: []ParamSchema{
			{Name: "platform", Type: "string", Description: "Screen platform: android/ios/harmony/web (default: android)", Required: false, Default: "android"},
			{Name: "action", Type: "string", Description: "Goal action: analyze/interact/navigate (default: analyze)", Required: false, Default: "analyze"},
			{Name: "target_element", Type: "string", Description: "Target UI element to interact with", Required: false},
			{Name: "target_app", Type: "string", Description: "Target app package name for navigation", Required: false},
			{Name: "ocr_text", Type: "string", Description: "OCR-extracted text from screen", Required: false},
		},
	}
}

func (n *ScreenUnderstandingNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	platform := getMobileParam(params, "platform", "android")
	if !validPlatform(platform) {
		return "", fmt.Errorf("invalid platform: %s", platform)
	}

	action := getMobileParam(params, "action", "analyze")
	if action != "analyze" && action != "interact" && action != "navigate" {
		return "", fmt.Errorf("invalid action: %s", action)
	}

	targetElement := getMobileParam(params, "target_element", "")
	if targetElement != "" && !uiElementPattern.MatchString(targetElement) {
		return "", fmt.Errorf("invalid target_element")
	}

	targetApp := getMobileParam(params, "target_app", "")
	if targetApp != "" && !validAppNames.MatchString(targetApp) {
		return "", fmt.Errorf("invalid target_app")
	}

	ocrText := getMobileParam(params, "ocr_text", "")
	if len(ocrText) > 10000 {
		return "", fmt.Errorf("ocr_text too large")
	}

	// Simulate screen analysis
	analysis := analyzeScreen(platform, input, ocrText)

	// Generate interaction plan based on action
	var plan *InteractionPlan
	switch action {
	case "interact":
		plan = generateInteractionPlan(analysis, targetElement)
	case "navigate":
		plan = generateNavigationPlan(platform, targetApp)
	default:
		plan = nil
	}

	result := map[string]interface{}{
		"type":             "screen_understanding",
		"platform":         platform,
		"action":           action,
		"screen_analysis":  analysis,
		"interaction_plan": plan,
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

// UIElement represents a detected UI element
type UIElement struct {
	Type         string  `json:"type"`
	Text         string  `json:"text,omitempty"`
	Bounds       Bounds  `json:"bounds"`
	Clickable    bool    `json:"clickable"`
	Interactable bool    `json:"interactable"`
	Confidence   float64 `json:"confidence"`
}

// Bounds represents screen coordinates
type Bounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ScreenAnalysis holds parsed screen information
type ScreenAnalysis struct {
	ScreenType       string      `json:"screen_type"`
	AppName          string      `json:"app_name,omitempty"`
	TopBar           []UIElement `json:"top_bar,omitempty"`
	ContentArea      []UIElement `json:"content_area,omitempty"`
	BottomBar        []UIElement `json:"bottom_bar,omitempty"`
	FloatingElements []UIElement `json:"floating_elements,omitempty"`
	TextContent      string      `json:"text_content,omitempty"`
	ActionableItems  []string    `json:"actionable_items,omitempty"`
}

// InteractionStep represents a single UI interaction
type InteractionStep struct {
	Action      string `json:"action"`
	Target      string `json:"target,omitempty"`
	Coordinates Bounds `json:"coordinates,omitempty"`
	TextInput   string `json:"text_input,omitempty"`
	Direction   string `json:"direction,omitempty"`
	DurationMs  int    `json:"duration_ms,omitempty"`
}

// InteractionPlan represents a sequence of interactions
type InteractionPlan struct {
	Goal        string            `json:"goal"`
	Steps       []InteractionStep `json:"steps"`
	Fallback    string            `json:"fallback,omitempty"`
	EstimatedMs int               `json:"estimated_ms"`
}

func analyzeScreen(platform, input, ocrText string) *ScreenAnalysis {
	// Combine input and OCR text for analysis
	content := input
	if ocrText != "" {
		content += " " + ocrText
	}

	// Detect screen type from content
	screenType := detectScreenType(content)
	appName := extractAppName(content)

	// Generate simulated UI elements based on content
	elements := generateElementsFromContent(content, platform)

	// Extract actionable items
	actionable := extractActionableItems(elements)

	return &ScreenAnalysis{
		ScreenType:       screenType,
		AppName:          appName,
		TopBar:           filterElementsByRegion(elements, "top"),
		ContentArea:      filterElementsByRegion(elements, "content"),
		BottomBar:        filterElementsByRegion(elements, "bottom"),
		FloatingElements: filterElementsByRegion(elements, "floating"),
		TextContent:      truncateString(content, 500),
		ActionableItems:  actionable,
	}
}

func detectScreenType(content string) string {
	lower := strings.ToLower(content)
	switch {
	case strings.Contains(lower, "login") || strings.Contains(lower, "登录"):
		return "login_screen"
	case strings.Contains(lower, "home") || strings.Contains(lower, "首页"):
		return "home_screen"
	case strings.Contains(lower, "list") || strings.Contains(lower, "列表"):
		return "list_screen"
	case strings.Contains(lower, "detail") || strings.Contains(lower, "详情"):
		return "detail_screen"
	case strings.Contains(lower, "chat") || strings.Contains(lower, "聊天"):
		return "chat_screen"
	case strings.Contains(lower, "settings") || strings.Contains(lower, "设置"):
		return "settings_screen"
	default:
		return "generic_screen"
	}
}

func extractAppName(content string) string {
	// Simple heuristic: look for app name patterns
	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		first := strings.TrimSpace(lines[0])
		if len(first) < 50 {
			return first
		}
	}
	return "unknown"
}

func generateElementsFromContent(content, platform string) []UIElement {
	var elements []UIElement

	// Generate elements based on common UI patterns
	if strings.Contains(content, "button") || strings.Contains(content, "按钮") {
		elements = append(elements, UIElement{
			Type:         "button",
			Text:         "Confirm",
			Bounds:       Bounds{X: 100, Y: 800, Width: 200, Height: 60},
			Clickable:    true,
			Interactable: true,
			Confidence:   0.95,
		})
	}
	if strings.Contains(content, "input") || strings.Contains(content, "输入") {
		elements = append(elements, UIElement{
			Type:         "text_field",
			Text:         "",
			Bounds:       Bounds{X: 50, Y: 400, Width: 300, Height: 50},
			Clickable:    true,
			Interactable: true,
			Confidence:   0.92,
		})
	}
	if strings.Contains(content, "list") || strings.Contains(content, "列表") {
		for i := 0; i < 3; i++ {
			elements = append(elements, UIElement{
				Type:         "list_item",
				Text:         fmt.Sprintf("Item %d", i+1),
				Bounds:       Bounds{X: 20, Y: 200 + i*80, Width: 360, Height: 70},
				Clickable:    true,
				Interactable: true,
				Confidence:   0.88,
			})
		}
	}
	if strings.Contains(content, "tab") || strings.Contains(content, "tab") {
		tabs := []string{"Home", "Discover", "Me"}
		for i, t := range tabs {
			elements = append(elements, UIElement{
				Type:         "tab",
				Text:         t,
				Bounds:       Bounds{X: i * 133, Y: 1200, Width: 133, Height: 60},
				Clickable:    true,
				Interactable: true,
				Confidence:   0.96,
			})
		}
	}

	return elements
}

func filterElementsByRegion(elements []UIElement, region string) []UIElement {
	var filtered []UIElement
	for _, e := range elements {
		y := e.Bounds.Y
		switch region {
		case "top":
			if y < 200 {
				filtered = append(filtered, e)
			}
		case "bottom":
			if y > 1100 {
				filtered = append(filtered, e)
			}
		case "floating":
			if e.Type == "fab" || e.Type == "popup" {
				filtered = append(filtered, e)
			}
		case "content":
			if y >= 200 && y <= 1100 {
				filtered = append(filtered, e)
			}
		}
	}
	return filtered
}

func extractActionableItems(elements []UIElement) []string {
	var items []string
	seen := make(map[string]bool)
	for _, e := range elements {
		if e.Interactable && e.Text != "" && !seen[e.Text] {
			items = append(items, e.Text)
			seen[e.Text] = true
		}
	}
	return items
}

func generateInteractionPlan(analysis *ScreenAnalysis, targetElement string) *InteractionPlan {
	steps := []InteractionStep{}

	// Find target element
	var target *UIElement
	for _, e := range analysis.ContentArea {
		if strings.Contains(strings.ToLower(e.Text), strings.ToLower(targetElement)) {
			target = &e
			break
		}
	}
	if target == nil {
		for _, e := range analysis.TopBar {
			if strings.Contains(strings.ToLower(e.Text), strings.ToLower(targetElement)) {
				target = &e
				break
			}
		}
	}

	if target != nil {
		steps = append(steps, InteractionStep{
			Action:      "tap",
			Target:      target.Text,
			Coordinates: target.Bounds,
		})
	} else {
		// Fallback: scroll to find
		steps = append(steps, InteractionStep{
			Action:    "scroll",
			Direction: "down",
		})
	}

	return &InteractionPlan{
		Goal:        fmt.Sprintf("Interact with %s", targetElement),
		Steps:       steps,
		Fallback:    "Ask user for clarification",
		EstimatedMs: len(steps) * 500,
	}
}

func generateNavigationPlan(platform, targetApp string) *InteractionPlan {
	steps := []InteractionStep{
		{Action: "home"},
		{Action: "swipe", Direction: "up", DurationMs: 300},
	}

	if targetApp != "" {
		steps = append(steps, InteractionStep{
			Action:    "tap",
			Target:    targetApp,
			TextInput: targetApp,
		})
	}

	return &InteractionPlan{
		Goal:        fmt.Sprintf("Navigate to %s", targetApp),
		Steps:       steps,
		Fallback:    "Open app drawer and search",
		EstimatedMs: len(steps) * 400,
	}
}

func validPlatform(p string) bool {
	return p == "android" || p == "ios" || p == "harmony" || p == "web"
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func init() {
	Register(&ScreenUnderstandingNode{})
}
