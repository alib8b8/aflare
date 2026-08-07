// Copyright (c) 2026 aflare Contributors
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

	"github.com/alib8b8/aflare/internal/nodes/core"
)

func TestScreenUnderstandingNode_Metadata(t *testing.T) {
	node := &ScreenUnderstandingNode{}
	if node.Name() != "screen_understanding" {
		t.Errorf("name: %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("empty description")
	}
	schema := node.Schema()
	if schema.Name != "screen_understanding" {
		t.Errorf("schema name: %s", schema.Name)
	}
}

func TestScreenUnderstandingNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &ScreenUnderstandingNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"invalid platform", map[string]string{"platform": "windows"}, "invalid platform"},
		{"invalid action", map[string]string{"action": "destroy"}, "invalid action"},
		{"invalid target_element", map[string]string{"target_element": strings.Repeat("a", 101)}, "invalid target_element"},
		{"invalid target_app", map[string]string{"target_app": "bad app!"}, "invalid target_app"},
		{"ocr_text too large", map[string]string{"ocr_text": strings.Repeat("a", 10001)}, "ocr_text too large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "screen content", tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestScreenUnderstandingNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &ScreenUnderstandingNode{}

	out, err := node.Execute(ctx, "login screen with username input and submit button", map[string]string{
		"platform": "android",
		"action":   "analyze",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "screen_understanding") {
		t.Errorf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "login_screen") {
		t.Error("expected login_screen type")
	}

	// Interact action
	out, err = node.Execute(ctx, "click the submit button", map[string]string{
		"platform":       "android",
		"action":         "interact",
		"target_element": "submit",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "interaction_plan") {
		t.Error("expected interaction_plan in output")
	}

	// Navigate action with target app
	out, err = node.Execute(ctx, "", map[string]string{
		"platform":   "android",
		"action":     "navigate",
		"target_app": "com.example.app",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "com.example.app") {
		t.Error("expected target_app in output")
	}
}

func TestAnalyzeScreen(t *testing.T) {
	analysis := analyzeScreen("android", "Home screen with tabs", "")
	if analysis == nil {
		t.Fatal("expected non-nil analysis")
	}
	if analysis.ScreenType != "home_screen" {
		t.Errorf("ScreenType: got %q, want home_screen", analysis.ScreenType)
	}
	if len(analysis.TopBar) != 0 && analysis.TopBar[0].Bounds.Y > 200 {
		t.Error("TopBar elements should have y < 200")
	}
}

func TestDetectScreenType(t *testing.T) {
	tests := []struct {
		content string
		want    string
	}{
		{"please login", "login_screen"},
		{"用户登录", "login_screen"},
		{"welcome home", "home_screen"},
		{"回到首页", "home_screen"},
		{"item list", "list_screen"},
		{"查看列表", "list_screen"},
		{"product detail", "detail_screen"},
		{"商品详情", "detail_screen"},
		{"chat with me", "chat_screen"},
		{"开始聊天", "chat_screen"},
		{"open settings", "settings_screen"},
		{"系统设置", "settings_screen"},
		{"random content", "generic_screen"},
		{"", "generic_screen"},
	}
	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			got := detectScreenType(tt.content)
			if got != tt.want {
				t.Errorf("detectScreenType(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestExtractAppName(t *testing.T) {
	tests := []struct {
		content string
		want    string
	}{
		{"MyApp\nrest of content", "MyApp"},
		{"short", "short"},
		{strings.Repeat("a", 60), "unknown"}, // first line too long
		{"", ""},
		{"  trim me  ", "trim me"},
	}
	for _, tt := range tests {
		got := extractAppName(tt.content)
		if got != tt.want {
			t.Errorf("extractAppName(%q) = %q, want %q", tt.content, got, tt.want)
		}
	}
}

func TestGenerateElementsFromContent(t *testing.T) {
	// button content
	elements := generateElementsFromContent("has button", "android")
	if len(elements) == 0 {
		t.Error("expected at least one button element")
	}

	// input content
	elements = generateElementsFromContent("has input field", "android")
	if len(elements) == 0 {
		t.Error("expected at least one text_field element")
	}

	// list content (3 items)
	elements = generateElementsFromContent("list view", "android")
	if len(elements) != 3 {
		t.Errorf("expected 3 list items, got %d", len(elements))
	}

	// tabs content (3 tabs)
	elements = generateElementsFromContent("tabs view", "android")
	if len(elements) != 3 {
		t.Errorf("expected 3 tab elements, got %d", len(elements))
	}

	// 输入 chinese
	elements = generateElementsFromContent("输入框", "android")
	if len(elements) == 0 {
		t.Error("expected text_field for 输入")
	}

	// 按钮 chinese
	elements = generateElementsFromContent("按钮", "android")
	if len(elements) == 0 {
		t.Error("expected button for 按钮")
	}

	// 列表 chinese
	elements = generateElementsFromContent("列表", "android")
	if len(elements) != 3 {
		t.Errorf("expected 3 list items for 列表, got %d", len(elements))
	}

	// no patterns
	elements = generateElementsFromContent("nothing matches", "android")
	if len(elements) != 0 {
		t.Errorf("expected 0 elements, got %d", len(elements))
	}
}

func TestFilterElementsByRegion(t *testing.T) {
	elements := []UIElement{
		{Type: "tab", Text: "TopTab", Bounds: Bounds{X: 0, Y: 100}},
		{Type: "button", Text: "MidButton", Bounds: Bounds{X: 0, Y: 500}},
		{Type: "button", Text: "BotButton", Bounds: Bounds{X: 0, Y: 1200}},
		{Type: "fab", Text: "FAB", Bounds: Bounds{X: 0, Y: 600}},
		{Type: "popup", Text: "Popup", Bounds: Bounds{X: 0, Y: 600}},
	}

	top := filterElementsByRegion(elements, "top")
	if len(top) != 1 || top[0].Text != "TopTab" {
		t.Errorf("top filter: got %v", top)
	}

	content := filterElementsByRegion(elements, "content")
	if len(content) != 3 {
		t.Errorf("content filter: got %d, want 3 (button, fab, popup)", len(content))
	}

	bottom := filterElementsByRegion(elements, "bottom")
	if len(bottom) != 1 || bottom[0].Text != "BotButton" {
		t.Errorf("bottom filter: got %v", bottom)
	}

	floating := filterElementsByRegion(elements, "floating")
	if len(floating) != 2 {
		t.Errorf("floating filter: got %d, want 2 (fab, popup)", len(floating))
	}
}

func TestExtractActionableItems(t *testing.T) {
	elements := []UIElement{
		{Type: "button", Text: "Submit", Interactable: true},
		{Type: "button", Text: "Cancel", Interactable: true},
		{Type: "button", Text: "Submit", Interactable: true}, // duplicate
		{Type: "label", Text: "Title", Interactable: false},  // not interactable
		{Type: "button", Text: "", Interactable: true},       // empty text
	}
	items := extractActionableItems(elements)
	if len(items) != 2 {
		t.Errorf("expected 2 unique items, got %d: %v", len(items), items)
	}
}

func TestGenerateInteractionPlan(t *testing.T) {
	analysis := &ScreenAnalysis{
		ContentArea: []UIElement{
			{Type: "button", Text: "Submit Button", Bounds: Bounds{X: 10, Y: 500}},
		},
	}

	// Target found
	plan := generateInteractionPlan(analysis, "submit")
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if len(plan.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Action != "tap" {
		t.Errorf("expected tap action, got %s", plan.Steps[0].Action)
	}

	// Target not found - fallback scroll
	plan = generateInteractionPlan(analysis, "nonexistent")
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if len(plan.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Action != "scroll" {
		t.Errorf("expected scroll action, got %s", plan.Steps[0].Action)
	}
	if plan.Steps[0].Direction != "down" {
		t.Errorf("expected down direction, got %s", plan.Steps[0].Direction)
	}
}

func TestGenerateInteractionPlan_TopBarFallback(t *testing.T) {
	analysis := &ScreenAnalysis{
		ContentArea: []UIElement{},
		TopBar: []UIElement{
			{Type: "button", Text: "Settings", Bounds: Bounds{X: 10, Y: 50}},
		},
	}
	plan := generateInteractionPlan(analysis, "settings")
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Action != "tap" {
		t.Errorf("expected tap on topbar element, got %v", plan.Steps)
	}
}

func TestGenerateNavigationPlan(t *testing.T) {
	// With target app
	plan := generateNavigationPlan("android", "com.example.app")
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if len(plan.Steps) != 3 {
		t.Errorf("expected 3 steps (home, swipe, tap), got %d", len(plan.Steps))
	}
	if !strings.Contains(plan.Goal, "com.example.app") {
		t.Errorf("expected goal to contain app name: %s", plan.Goal)
	}

	// Without target app
	plan = generateNavigationPlan("android", "")
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if len(plan.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(plan.Steps))
	}
}

func TestValidPlatform(t *testing.T) {
	tests := []struct {
		p    string
		want bool
	}{
		{"android", true},
		{"ios", true},
		{"harmony", true},
		{"web", true},
		{"windows", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := validPlatform(tt.p); got != tt.want {
			t.Errorf("validPlatform(%q) = %v, want %v", tt.p, got, tt.want)
		}
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"你好世界你好", 4, "你好世界..."},
	}
	for _, tt := range tests {
		got := truncateString(tt.s, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
		}
	}
}

// Ensure screen_understanding node was registered.
func TestScreenUnderstandingNode_Registered(t *testing.T) {
	if _, ok := core.Get("screen_understanding"); !ok {
		t.Error("screen_understanding not registered")
	}
}
