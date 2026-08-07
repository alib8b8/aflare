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

// -----------------------------------------------------------------
// Helper function tests
// -----------------------------------------------------------------

func TestGetMobileParam(t *testing.T) {
	tests := []struct {
		name       string
		params     map[string]string
		key        string
		defaultVal string
		want       string
	}{
		{"missing key", map[string]string{}, "foo", "def", "def"},
		{"empty value", map[string]string{"foo": ""}, "foo", "def", "def"},
		{"non-empty value", map[string]string{"foo": "bar"}, "foo", "def", "bar"},
		{"nil params", nil, "foo", "def", "def"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMobileParam(tt.params, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateInput(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short ascii", "hello", 10, "hello"},
		{"exact length ascii", "hello", 5, "hello"},
		{"truncate ascii rune-wise", "abcdefgh", 4, "abcd..."},
		{"empty", "", 5, ""},
		{"truncate by rune", "你好世界你好世界", 4, "你好世界..."},
		{"ascii over rune limit", "abcdef", 3, "abc..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateInput(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeEcho(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"plain ascii", "hello", "hello"},
		{"control chars filtered", "a\x00b\x07c", "abc"},
		{"delete char filtered", "ab\x7fc", "abc"},
		{"long input truncated", strings.Repeat("a", 200), strings.Repeat("a", 100) + "..."},
		{"chinese preserved", "你好", "你好"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeEcho(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateAppURI(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{"empty allowed", "", false},
		{"valid https", "https://example.com", false},
		{"valid deep link", "myapp://path", false},
		{"harmony scheme", "ohos://path", false},
		{"file scheme blocked", "file:///etc/passwd", true},
		{"data scheme blocked", "data://text", true},
		{"javascript scheme blocked", "javascript://alert", true},
		{"ftp scheme blocked", "ftp://host", true},
		{"too long", "https://" + strings.Repeat("a", 4100), true},
		{"uppercase file blocked", "FILE://test", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAppURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAppURI(%q) err = %v, wantErr %v", tt.uri, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAppParams(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{"empty", map[string]interface{}{}, false},
		{"valid simple", map[string]interface{}{"k": "v"}, false},
		{"key too long", map[string]interface{}{strings.Repeat("a", 101): "v"}, true},
		{"string value too long", map[string]interface{}{"k": strings.Repeat("a", 1001)}, true},
		{"array too long", map[string]interface{}{"k": make([]interface{}, 101)}, true},
		{"array ok", map[string]interface{}{"k": []interface{}{1, 2, 3}}, false},
		{"nested map valid", map[string]interface{}{"outer": map[string]interface{}{"inner": "v"}}, false},
		{"nested map invalid", map[string]interface{}{"outer": map[string]interface{}{strings.Repeat("a", 101): "v"}}, true},
		{"non-string scalar ok", map[string]interface{}{"n": 123, "b": true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAppParams(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

func TestParseIntSafe(t *testing.T) {
	tests := []struct {
		s    string
		def  int
		want int
	}{
		{"123", 0, 123},
		{"", 7, 7},
		{"abc", 9, 9},
		{"-5", 0, -5},
	}
	for _, tt := range tests {
		got := parseIntSafe(tt.s, tt.def)
		if got != tt.want {
			t.Errorf("parseIntSafe(%q, %d) = %d, want %d", tt.s, tt.def, got, tt.want)
		}
	}
}

func TestParseFloatSafe(t *testing.T) {
	tests := []struct {
		s    string
		def  float64
		want float64
	}{
		{"3.14", 0, 3.14},
		{"", 1.5, 1.5},
		{"abc", 2.0, 2.0},
		{"NaN", 1.0, 1.0},
		{"Inf", 1.0, 1.0},
		{"+Inf", 1.0, 1.0},
		{"-Inf", 1.0, 1.0},
	}
	for _, tt := range tests {
		got := parseFloatSafe(tt.s, tt.def)
		if got != tt.want {
			t.Errorf("parseFloatSafe(%q, %v) = %v, want %v", tt.s, tt.def, got, tt.want)
		}
	}
}

func TestDetectPlatform(t *testing.T) {
	t.Setenv("OHOS_ROOT", "/ohos")
	if got := DetectPlatform(); got != PlatformHarmony {
		t.Errorf("with OHOS_ROOT set, expected harmony, got %v", got)
	}
}

func TestDetectPlatformDesktop(t *testing.T) {
	// Clear all detection env vars to force desktop fallback
	t.Setenv("OHOS_ROOT", "")
	t.Setenv("HOS_ROOT", "")
	t.Setenv("ANDROID_ROOT", "")
	t.Setenv("TERMUX_VERSION", "")
	t.Setenv("CFFIXED_USER_HOME", "")
	t.Setenv("HOME", "/tmp")
	got := DetectPlatform()
	if got != PlatformDesktop {
		t.Errorf("expected desktop, got %v", got)
	}
}

func TestDetectPlatformAndroid(t *testing.T) {
	t.Setenv("OHOS_ROOT", "")
	t.Setenv("HOS_ROOT", "")
	t.Setenv("ANDROID_ROOT", "/system")
	t.Setenv("TERMUX_VERSION", "")
	t.Setenv("CFFIXED_USER_HOME", "")
	t.Setenv("HOME", "/tmp")
	if got := DetectPlatform(); got != PlatformAndroid {
		t.Errorf("expected android, got %v", got)
	}
}

func TestClassifyIntent(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		domains     []string
		wantDomain  string
		wantHandler string
	}{
		{"book flight zh", "帮我订机票", []string{"travel"}, "travel", ""},
		{"book hotel en", "I want to book a hotel", []string{"travel"}, "travel", ""},
		{"order food", "我要订餐", []string{"food"}, "food", ""},
		{"play music en", "play music", []string{"entertainment"}, "entertainment", ""},
		{"send message", "发消息给张三", []string{"communication"}, "communication", ""},
		{"make call", "请打电话给李四", []string{"communication"}, "communication", ""},
		{"set reminder", "提醒我开会", []string{"work"}, "work", ""},
		{"control device", "打开灯", []string{"system"}, "system", ""},
		{"no match", "what's the meaning of life", []string{"travel"}, "", ""},
		{"empty domains", "订机票", []string{}, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyIntent(tt.input, tt.domains)
			domain, _ := result["domain"].(string)
			handler, _ := result["handler"].(string)
			if domain != tt.wantDomain {
				t.Errorf("domain: got %q, want %q", domain, tt.wantDomain)
			}
			if handler != tt.wantHandler {
				t.Errorf("handler: got %q, want %q", handler, tt.wantHandler)
			}
		})
	}
}

func TestGenerateCrossAppSteps(t *testing.T) {
	shareSteps := generateCrossAppSteps("share_content", []string{"com.app"}, "input")
	if len(shareSteps) != 2 {
		t.Errorf("share_content: expected 2 steps, got %d", len(shareSteps))
	}

	saveSteps := generateCrossAppSteps("save_for_later", []string{}, "input")
	if len(saveSteps) != 2 {
		t.Errorf("save_for_later: expected 2 steps, got %d", len(saveSteps))
	}

	defaultSteps := generateCrossAppSteps("unknown_workflow", []string{}, "input")
	if len(defaultSteps) != 1 {
		t.Errorf("default: expected 1 step, got %d", len(defaultSteps))
	}
}

// -----------------------------------------------------------------
// Harmony device adaptation helpers
// -----------------------------------------------------------------

func TestGetDeviceCapabilities(t *testing.T) {
	tests := []struct {
		dt       HarmonyDeviceType
		minItems int
		hasTouch bool
	}{
		{DevicePhoneStandard, 5, true},
		{DevicePhoneDualFold, 7, true},
		{DevicePhoneTripleFold, 7, true},
		{DeviceTablet, 5, true},
		{DeviceSmartScreen, 4, true},
		{DeviceCar, 4, true},
		{DeviceWearable, 4, true},
		{DeviceUnknown, 1, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.dt), func(t *testing.T) {
			caps := getDeviceCapabilities(tt.dt)
			if len(caps) < tt.minItems {
				t.Errorf("got %d caps, want >= %d", len(caps), tt.minItems)
			}
			found := false
			for _, c := range caps {
				if c == "touch" {
					found = true
					break
				}
			}
			if tt.hasTouch && !found {
				t.Error("expected touch capability")
			}
		})
	}
}

func TestGetLayoutStrategy(t *testing.T) {
	tests := []struct {
		info HarmonyDeviceInfo
		want string
	}{
		{HarmonyDeviceInfo{Type: DevicePhoneStandard}, "single_column"},
		{HarmonyDeviceInfo{Type: DevicePhoneDualFold, FoldState: "half_folded"}, "dual_column_split"},
		{HarmonyDeviceInfo{Type: DevicePhoneDualFold, FoldState: "unfolded"}, "adaptive_column"},
		{HarmonyDeviceInfo{Type: DevicePhoneTripleFold}, "adaptive_column"},
		{HarmonyDeviceInfo{Type: DeviceTablet}, "dual_column_or_grid"},
		{HarmonyDeviceInfo{Type: DeviceSmartScreen}, "large_card_grid"},
		{HarmonyDeviceInfo{Type: DeviceCar}, "simplified_single_column"},
		{HarmonyDeviceInfo{Type: DeviceWearable}, "minimal_single_column"},
		{HarmonyDeviceInfo{Type: DeviceUnknown}, "single_column"},
	}
	for _, tt := range tests {
		got := getLayoutStrategy(tt.info)
		if got != tt.want {
			t.Errorf("getLayoutStrategy(%v) = %q, want %q", tt.info.Type, got, tt.want)
		}
	}
}

func TestGetBreakpoints(t *testing.T) {
	types := []HarmonyDeviceType{
		DevicePhoneStandard, DevicePhoneDualFold, DevicePhoneTripleFold,
		DeviceTablet, DeviceSmartScreen, DeviceCar, DeviceWearable, DeviceUnknown,
	}
	for _, dt := range types {
		bp := getBreakpoints(HarmonyDeviceInfo{Type: dt})
		if bp["sm"] == 0 || bp["md"] == 0 || bp["lg"] == 0 {
			t.Errorf("device %v: missing basic breakpoints: %v", dt, bp)
		}
	}
}

func TestGetUIComponents(t *testing.T) {
	types := []HarmonyDeviceType{
		DevicePhoneStandard, DevicePhoneDualFold, DeviceTablet,
		DeviceSmartScreen, DeviceCar, DeviceWearable, DeviceUnknown,
	}
	for _, dt := range types {
		comps := getUIComponents(HarmonyDeviceInfo{Type: dt})
		if len(comps) == 0 {
			t.Errorf("device %v: empty components", dt)
		}
	}
}

func TestGetInteractionHints(t *testing.T) {
	tests := []HarmonyDeviceType{
		DeviceCar, DeviceWearable, DeviceSmartScreen,
		DevicePhoneDualFold, DevicePhoneStandard, DeviceUnknown,
	}
	for _, dt := range tests {
		hints := getInteractionHints(HarmonyDeviceInfo{Type: dt})
		if len(hints) == 0 {
			t.Errorf("device %v: empty hints", dt)
		}
	}
}

func TestGetFoldAdaptation(t *testing.T) {
	info := HarmonyDeviceInfo{Type: DevicePhoneDualFold, FoldState: "unfolded"}
	fa := getFoldAdaptation(info)
	if fa["fold_type"] != "phone_dual_fold" {
		t.Errorf("unexpected fold_type: %v", fa["fold_type"])
	}
	if fa["current_state"] != "unfolded" {
		t.Errorf("unexpected current_state: %v", fa["current_state"])
	}
	if _, ok := fa["recommended"]; !ok {
		t.Error("expected recommended field")
	}
}

func TestGenerateAdaptationPlan(t *testing.T) {
	info := HarmonyDeviceInfo{Type: DevicePhoneDualFold, IsFoldable: true, FoldState: "unfolded"}
	plan := generateAdaptationPlan(info, "custom requirement")
	if _, ok := plan["layout_strategy"]; !ok {
		t.Error("expected layout_strategy")
	}
	if _, ok := plan["fold_adaptation"]; !ok {
		t.Error("expected fold_adaptation for foldable device")
	}
	if _, ok := plan["custom_requirements"]; !ok {
		t.Error("expected custom_requirements field")
	}

	// Non-foldable device: no fold_adaptation key
	info2 := HarmonyDeviceInfo{Type: DevicePhoneStandard}
	plan2 := generateAdaptationPlan(info2, "")
	if _, ok := plan2["fold_adaptation"]; ok {
		t.Error("non-foldable device should not have fold_adaptation")
	}
}

// -----------------------------------------------------------------
// AppLaunchNode
// -----------------------------------------------------------------

func TestAppLaunchNode_Metadata(t *testing.T) {
	node := &AppLaunchNode{}
	if node.Name() != "app_launch" {
		t.Errorf("name: %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("empty description")
	}
	schema := node.Schema()
	if schema.Name != "app_launch" {
		t.Errorf("schema name: %s", schema.Name)
	}
	if len(schema.Params) == 0 {
		t.Error("expected params")
	}
}

func TestAppLaunchNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &AppLaunchNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"missing app", map[string]string{}, "app parameter is required"},
		{"invalid app name", map[string]string{"app": "bad app!"}, "invalid app name"},
		{"invalid action", map[string]string{"app": "com.app", "action": "fly"}, "invalid action"},
		{"invalid platform", map[string]string{"app": "com.app", "platform": "windows"}, "invalid platform"},
		{"disallowed uri scheme", map[string]string{"app": "com.app", "uri": "file:///etc/passwd"}, "URI scheme not allowed"},
		{"invalid params json", map[string]string{"app": "com.app", "params": "{bad"}, "invalid params JSON"},
		{"param value too long", map[string]string{"app": "com.app", "params": `{"k":"` + strings.Repeat("a", 1001) + `"}`}, "parameter value too long"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "", tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestAppLaunchNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &AppLaunchNode{}

	out, err := node.Execute(ctx, "hello", map[string]string{"app": "com.example.app"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "App launch intent created") {
		t.Errorf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "com.example.app") {
		t.Error("expected app name in output")
	}

	// Harmony platform adds harmony-specific field
	out, err = node.Execute(ctx, "", map[string]string{
		"app":      "com.example.app",
		"platform": "harmony",
		"uri":      "ohos://page",
		"params":   `{"key":"value"}`,
	})
	if err != nil {
		t.Fatalf("harmony path error: %v", err)
	}
	if !strings.Contains(out, "harmony") {
		t.Error("expected harmony field")
	}
	if !strings.Contains(out, "ohos://page") {
		t.Error("expected uri")
	}
}

// -----------------------------------------------------------------
// UIAutomateNode
// -----------------------------------------------------------------

func TestUIAutomateNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &UIAutomateNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"missing action", map[string]string{}, "action parameter is required"},
		{"invalid action", map[string]string{"action": "fly"}, "invalid action"},
		{"selector too long", map[string]string{"action": "click", "selector": strings.Repeat("a", 501)}, "selector too long"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "", tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestUIAutomateNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &UIAutomateNode{}

	// click with id selector
	out, err := node.Execute(ctx, "", map[string]string{
		"action":   "click",
		"selector": "id:login_button",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "UI automation command") {
		t.Errorf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "login_button") {
		t.Error("expected selector value in output")
	}

	// type action with text prefix
	out, err = node.Execute(ctx, "default text", map[string]string{
		"action": "type",
		"text":   "id:user_field",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "type") {
		t.Error("expected type action")
	}

	// text prefix selector
	out, err = node.Execute(ctx, "", map[string]string{
		"action":    "click",
		"selector":  "text:Submit",
		"direction": "up",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Submit") {
		t.Error("expected Submit in output")
	}
}

// -----------------------------------------------------------------
// NotificationNode
// -----------------------------------------------------------------

func TestNotificationNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &NotificationNode{}

	tests := []struct {
		name   string
		input  string
		params map[string]string
		errSub string
	}{
		{"missing title", "", map[string]string{}, "title parameter is required"},
		{"invalid actions json", "", map[string]string{"title": "T", "actions": "bad"}, "invalid actions JSON"},
		{"too many actions", "", map[string]string{"title": "T", "actions": `[{"id":"a","title":"A"},{"id":"b","title":"B"},{"id":"c","title":"C"},{"id":"d","title":"D"}]`}, "too many action buttons"},
		{"missing action id", "", map[string]string{"title": "T", "actions": `[{"title":"A"}]`}, "requires 'id'"},
		{"missing action title", "", map[string]string{"title": "T", "actions": `[{"id":"a"}]`}, "requires 'title'"},
		{"action id too long", "", map[string]string{"title": "T", "actions": `[{"id":"` + strings.Repeat("a", 51) + `","title":"A"}]`}, "id too long"},
		{"action title too long", "", map[string]string{"title": "T", "actions": `[{"id":"a","title":"` + strings.Repeat("a", 21) + `"}]`}, "title too long"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, tt.input, tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestNotificationNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &NotificationNode{}

	out, err := node.Execute(ctx, "body text", map[string]string{
		"title":    "Hello",
		"priority": "high",
		"actions":  `[{"id":"ok","title":"OK"},{"id":"cancel","title":"Cancel"}]`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Notification prepared") {
		t.Errorf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "Hello") {
		t.Error("expected title in output")
	}

	// Truncation paths
	longTitle := strings.Repeat("t", 200)
	longBody := strings.Repeat("b", 2000)
	out, err = node.Execute(ctx, longBody, map[string]string{"title": longTitle})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Notification prepared") {
		t.Error("expected notification prepared output")
	}
}

// -----------------------------------------------------------------
// IntentRouterNode
// -----------------------------------------------------------------

func TestIntentRouterNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &IntentRouterNode{}

	tests := []struct {
		name   string
		input  string
		params map[string]string
		errSub string
	}{
		{"empty input", "  ", map[string]string{}, "input cannot be empty"},
		{"invalid mode", "hello", map[string]string{"mode": "invalid"}, "invalid mode"},
		{"invalid domain", "订机票", map[string]string{"domains": "travel,invalid"}, "invalid domain"},
		{"fallback too long", "hello", map[string]string{"fallback": strings.Repeat("a", 101)}, "fallback handler name too long"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, tt.input, tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestIntentRouterNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &IntentRouterNode{}

	out, err := node.Execute(ctx, "帮我订机票去北京", map[string]string{"mode": "route"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "intent_router") {
		t.Errorf("unexpected output: %s", out)
	}
	// The travel domain keyword "机票" should be matched by classifyIntent
	if !strings.Contains(out, "travel") {
		t.Error("expected travel domain in output")
	}
}

// -----------------------------------------------------------------
// DeviceStateNode
// -----------------------------------------------------------------

func TestDeviceStateNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &DeviceStateNode{}

	_, err := node.Execute(ctx, "", map[string]string{"query": "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid query")
	}
	if !strings.Contains(err.Error(), "invalid query") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeviceStateNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &DeviceStateNode{}

	for _, q := range []string{"battery", "network", "location", "apps", "storage", "all"} {
		out, err := node.Execute(ctx, "", map[string]string{"query": q})
		if err != nil {
			t.Errorf("query %s: unexpected error: %v", q, err)
		}
		if !strings.Contains(out, "Device state query") {
			t.Errorf("query %s: unexpected output: %s", q, out)
		}
	}
}

// -----------------------------------------------------------------
// CrossAppActionNode
// -----------------------------------------------------------------

func TestCrossAppActionNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &CrossAppActionNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"missing workflow", map[string]string{}, "workflow parameter is required"},
		{"invalid workflow", map[string]string{"workflow": "unknown"}, "invalid workflow"},
		{"invalid app name", map[string]string{"workflow": "share_content", "apps": "bad app!"}, "invalid app name"},
		{"invalid data json", map[string]string{"workflow": "share_content", "data": "{bad"}, "invalid data JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "", tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestCrossAppActionNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &CrossAppActionNode{}

	out, err := node.Execute(ctx, "share this", map[string]string{
		"workflow": "share_content",
		"apps":     "com.app1,com.app2",
		"data":     `{"url":"https://example.com"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Cross-app action plan") {
		t.Errorf("unexpected output: %s", out)
	}
}

// -----------------------------------------------------------------
// AgentMessageNode
// -----------------------------------------------------------------

func TestAgentMessageNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &AgentMessageNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"missing to_did", map[string]string{}, "to_did parameter is required"},
		{"invalid to_did format", map[string]string{"to_did": "notadid"}, "invalid to_did format"},
		{"to_did too long", map[string]string{"to_did": "did:m:" + strings.Repeat("a", 260)}, "to_did too long"},
		{"invalid from_did", map[string]string{"to_did": "did:m:abc", "from_did": "bad"}, "invalid from_did format"},
		{"invalid priority", map[string]string{"to_did": "did:m:abc", "priority": "urgentest"}, "invalid priority"},
		{"endpoint too long", map[string]string{"to_did": "did:m:abc", "endpoint": "https://" + strings.Repeat("a", 2050)}, "endpoint too long"},
		{"endpoint invalid scheme", map[string]string{"to_did": "did:m:abc", "endpoint": "ftp://example.com"}, "only http and https URLs are allowed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "body", tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestAgentMessageNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &AgentMessageNode{}

	out, err := node.Execute(ctx, "Hello there", map[string]string{
		"to_did":   "did:example:abc",
		"from_did": "did:example:sender",
		"subject":  "Greeting",
		"priority": "high",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Agent message prepared") {
		t.Errorf("unexpected output: %s", out)
	}

	// Long body and subject truncation
	longBody := strings.Repeat("b", 6000)
	longSubject := strings.Repeat("s", 300)
	out, err = node.Execute(ctx, longBody, map[string]string{
		"to_did":  "did:example:abc",
		"subject": longSubject,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "truncated") {
		t.Error("expected truncation marker")
	}
}

// -----------------------------------------------------------------
// AgentInboxNode
// -----------------------------------------------------------------

func TestAgentInboxNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &AgentInboxNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"invalid action", map[string]string{"action": "fly"}, "invalid action"},
		{"missing message_id for read", map[string]string{"action": "read"}, "message_id is required"},
		{"missing message_id for delete", map[string]string{"action": "delete"}, "message_id is required"},
		{"message_id too long", map[string]string{"action": "read", "message_id": strings.Repeat("a", 129)}, "message_id too long"},
		{"invalid from_did", map[string]string{"action": "list", "from_did": "bad"}, "invalid from_did format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "", tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestAgentInboxNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &AgentInboxNode{}

	// list action
	out, err := node.Execute(ctx, "", map[string]string{"action": "list", "limit": "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Agent inbox query") {
		t.Errorf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "\"limit\": 5") {
		t.Error("expected limit 5 in output")
	}

	// invalid limit falls back to 10
	out, err = node.Execute(ctx, "", map[string]string{"action": "list", "limit": "notanumber"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "\"limit\": 10") {
		t.Error("expected limit fallback to 10")
	}

	// read action with from_did filter
	out, err = node.Execute(ctx, "", map[string]string{
		"action":     "read",
		"message_id": "msg1",
		"from_did":   "did:example:abc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "msg1") {
		t.Error("expected message_id in output")
	}
}

// -----------------------------------------------------------------
// HarmonyAbilityNode
// -----------------------------------------------------------------

func TestHarmonyAbilityNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &HarmonyAbilityNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"missing bundle_name", map[string]string{}, "bundle_name parameter is required"},
		{"invalid bundle_name", map[string]string{"bundle_name": "bad name!"}, "invalid bundle_name"},
		{"bundle_name too long", map[string]string{"bundle_name": strings.Repeat("a", 257)}, "bundle_name too long"},
		{"missing ability_name", map[string]string{"bundle_name": "com.app"}, "ability_name parameter is required"},
		{"invalid ability_name", map[string]string{"bundle_name": "com.app", "ability_name": "bad name!"}, "invalid ability_name"},
		{"ability_name too long", map[string]string{"bundle_name": "com.app", "ability_name": strings.Repeat("a", 129)}, "ability_name too long"},
		{"invalid ability_type", map[string]string{"bundle_name": "com.app", "ability_name": "Main", "ability_type": "invalid"}, "invalid ability_type"},
		{"invalid uri", map[string]string{"bundle_name": "com.app", "ability_name": "Main", "uri": "file:///etc/passwd"}, "URI scheme not allowed"},
		{"invalid params json", map[string]string{"bundle_name": "com.app", "ability_name": "Main", "params": "{bad"}, "invalid params JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "", tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestHarmonyAbilityNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &HarmonyAbilityNode{}

	out, err := node.Execute(ctx, "data", map[string]string{
		"bundle_name":  "com.example.app",
		"ability_name": "MainAbility",
		"ability_type": "page",
		"uri":          "ohos://page",
		"params":       `{"key":"value"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "HarmonyOS Ability launch intent") {
		t.Errorf("unexpected output: %s", out)
	}
}

// -----------------------------------------------------------------
// HarmonyAtomicServiceNode
// -----------------------------------------------------------------

func TestHarmonyAtomicServiceNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &HarmonyAtomicServiceNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"missing service_id", map[string]string{}, "service_id parameter is required"},
		{"invalid service_id", map[string]string{"service_id": "bad id!"}, "invalid service_id"},
		{"service_id too long", map[string]string{"service_id": strings.Repeat("a", 257)}, "service_id too long"},
		{"invalid action", map[string]string{"service_id": "com.svc", "action": "fly"}, "invalid action"},
		{"invalid card_id", map[string]string{"service_id": "com.svc", "card_id": "bad card!"}, "invalid card_id"},
		{"card_id too long", map[string]string{"service_id": "com.svc", "card_id": strings.Repeat("a", 129)}, "card_id too long"},
		{"invalid params json", map[string]string{"service_id": "com.svc", "params": "{bad"}, "invalid params JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "", tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestHarmonyAtomicServiceNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &HarmonyAtomicServiceNode{}

	out, err := node.Execute(ctx, "data", map[string]string{
		"service_id": "com.example.svc",
		"action":     "launch",
		"card_id":    "card1",
		"params":     `{"k":"v"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "HarmonyOS Atomic Service intent") {
		t.Errorf("unexpected output: %s", out)
	}
}

// -----------------------------------------------------------------
// HarmonyWidgetNode
// -----------------------------------------------------------------

func TestHarmonyWidgetNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &HarmonyWidgetNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"invalid action", map[string]string{"action": "fly"}, "invalid action"},
		{"missing widget_id for update", map[string]string{"action": "update"}, "widget_id is required"},
		{"invalid widget_id", map[string]string{"action": "update", "widget_id": "bad id!"}, "invalid widget_id"},
		{"widget_id too long", map[string]string{"action": "update", "widget_id": strings.Repeat("a", 129)}, "widget_id too long"},
		{"add missing provider_bundle", map[string]string{"action": "add"}, "provider_bundle is required"},
		{"add invalid provider_bundle", map[string]string{"action": "add", "provider_bundle": "bad bundle!"}, "invalid provider_bundle"},
		{"add missing widget_name", map[string]string{"action": "add", "provider_bundle": "com.p"}, "widget_name is required"},
		{"add invalid widget_name", map[string]string{"action": "add", "provider_bundle": "com.p", "widget_name": "bad name!"}, "invalid widget_name"},
		{"invalid data json", map[string]string{"action": "query", "widget_id": "w1", "data": "{bad"}, "invalid data JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "", tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestHarmonyWidgetNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &HarmonyWidgetNode{}

	out, err := node.Execute(ctx, "content", map[string]string{
		"action":          "add",
		"provider_bundle": "com.example.provider",
		"widget_name":     "WeatherWidget",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "HarmonyOS Widget operation") {
		t.Errorf("unexpected output: %s", out)
	}

	out, err = node.Execute(ctx, "", map[string]string{
		"action":    "query",
		"widget_id": "widget1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "widget1") {
		t.Error("expected widget_id in output")
	}
}

// -----------------------------------------------------------------
// HarmonyDeviceAdaptNode
// -----------------------------------------------------------------

func TestHarmonyDeviceAdaptNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &HarmonyDeviceAdaptNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"invalid device_type", map[string]string{"device_type": "invalid"}, "invalid device_type"},
		{"invalid fold_state", map[string]string{"fold_state": "invalid"}, "invalid fold_state"},
		{"invalid orientation", map[string]string{"orientation": "sideways"}, "invalid orientation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "", tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestHarmonyDeviceAdaptNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &HarmonyDeviceAdaptNode{}

	out, err := node.Execute(ctx, "adaptive reqs", map[string]string{
		"device_type":  "phone_dual_fold",
		"fold_state":   "half_folded",
		"orientation":  "portrait",
		"screen_width": "1080",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "HarmonyOS device adaptation") {
		t.Errorf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "dual_column_split") {
		t.Error("expected dual_column_split layout for half_folded")
	}
}

func TestHarmonyDeviceAdaptNode_ScreenParamClamping(t *testing.T) {
	ctx := context.Background()
	node := &HarmonyDeviceAdaptNode{}

	// Out-of-range values get clamped to defaults
	out, err := node.Execute(ctx, "", map[string]string{
		"device_type":     "phone_standard",
		"screen_width":    "10",    // too small, falls back to 1080
		"screen_height":   "99999", // too large, falls back to 2400
		"screen_density":  "0.1",   // too small, falls back to 3.0
		"screen_density2": "abc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "1080") {
		t.Error("expected clamped screen_width 1080")
	}
}

func TestHarmonyDeviceAdaptNode_NonFoldableIgnoresFoldState(t *testing.T) {
	ctx := context.Background()
	node := &HarmonyDeviceAdaptNode{}

	// phone_standard is not foldable, so fold_state should be ignored
	out, err := node.Execute(ctx, "", map[string]string{
		"device_type": "phone_standard",
		"fold_state":  "unfolded",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// fold_state should not appear in output (empty string is omitempty)
	if strings.Contains(out, "\"fold_state\"") {
		t.Error("non-foldable device should not include fold_state")
	}
}

// Sanity: ensure all nodes were registered with the core registry.
func TestMobileNodes_Registered(t *testing.T) {
	names := []string{
		"app_launch", "ui_automate", "send_notification", "intent_router",
		"device_state", "cross_app_action", "agent_message", "agent_inbox",
		"harmony_ability", "harmony_atomic_service", "harmony_widget",
		"harmony_device_adapt",
	}
	for _, n := range names {
		if _, ok := core.Get(n); !ok {
			t.Errorf("node %q not registered", n)
		}
	}
}
