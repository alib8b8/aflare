// Copyright (c) 2026 aflare Contributors
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

package core

import (
	"testing"
)

// --- GetParam ---

func TestGetParam(t *testing.T) {
	params := map[string]string{"present": "value", "empty": ""}

	cases := []struct {
		name       string
		key        string
		defaultVal string
		want       string
	}{
		{"present non-empty", "present", "default", "value"},
		{"present but empty", "empty", "default", "default"},
		{"missing key", "missing", "default", "default"},
		{"missing key empty default", "missing", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := GetParam(params, c.key, c.defaultVal); got != c.want {
				t.Errorf("GetParam(..., %q, %q) = %q, want %q", c.key, c.defaultVal, got, c.want)
			}
		})
	}
}

func TestGetParam_NilMap(t *testing.T) {
	if got := GetParam(nil, "key", "fallback"); got != "fallback" {
		t.Errorf("GetParam(nil, ...) = %q, want fallback", got)
	}
}

// --- ParamInt ---
func TestParamInt(t *testing.T) {
	params := map[string]string{
		"valid":   "42",
		"invalid": "not-a-number",
		"float":   "3.14",
		"neg":     "-5",
		"empty":   "",
	}

	cases := []struct {
		name       string
		key        string
		defaultVal int
		min, max   int
		want       int
	}{
		{"valid in range", "valid", 0, 0, 100, 42},
		{"invalid format returns default", "invalid", 7, 0, 100, 7},
		{"float parses leading integer", "float", 9, 0, 100, 3},
		{"below min clamps to min", "neg", 0, 0, 100, 0},
		{"above max clamps to max", "valid", 0, 0, 10, 10},
		{"empty returns default", "empty", 5, 0, 100, 5},
		{"missing returns default", "missing", 3, 0, 100, 3},
		{"boundary min", "valid", 0, 42, 100, 42},
		{"boundary max", "valid", 0, 0, 42, 42},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ParamInt(params, c.key, c.defaultVal, c.min, c.max); got != c.want {
				t.Errorf("ParamInt(..., %q, %d, %d, %d) = %d, want %d",
					c.key, c.defaultVal, c.min, c.max, got, c.want)
			}
		})
	}
}

func TestParamInt_NoClampingWhenMinGreaterThanMax(t *testing.T) {
	// When min > max, clamping is disabled and the parsed value is returned.
	params := map[string]string{"v": "999"}
	if got := ParamInt(params, "v", 0, 100, 0); got != 999 {
		t.Errorf("expected 999 with clamping disabled, got %d", got)
	}
}

// --- ParamFloat ---

func TestParamFloat(t *testing.T) {
	params := map[string]string{
		"valid":   "3.14",
		"invalid": "nope",
		"empty":   "",
		"int":     "5",
	}

	cases := []struct {
		name       string
		key        string
		defaultVal float64
		min, max   float64
		want       float64
	}{
		{"valid in range", "valid", 0, 0, 10, 3.14},
		{"invalid format returns default", "invalid", 1.5, 0, 10, 1.5},
		{"empty returns default", "empty", 2.0, 0, 10, 2.0},
		{"below min clamps to min", "valid", 0, 5, 10, 5},
		{"above max clamps to max", "valid", 0, 0, 1, 1},
		{"integer string parses", "int", 0, 0, 10, 5},
		{"missing returns default", "missing", 7.7, 0, 10, 7.7},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ParamFloat(params, c.key, c.defaultVal, c.min, c.max); got != c.want {
				t.Errorf("ParamFloat(..., %q, %v, %v, %v) = %v, want %v",
					c.key, c.defaultVal, c.min, c.max, got, c.want)
			}
		})
	}
}

func TestParamFloat_NoClampingWhenMinGreaterThanMax(t *testing.T) {
	params := map[string]string{"v": "999.9"}
	if got := ParamFloat(params, "v", 0, 100, 0); got != 999.9 {
		t.Errorf("expected 999.9 with clamping disabled, got %v", got)
	}
}

// --- DefaultEndpointFor ---

func TestDefaultEndpointFor(t *testing.T) {
	cases := []struct {
		provider string
		want     string
	}{
		{"ollama", "http://localhost:11434"},
		{"openai", "https://api.openai.com/v1"},
		{"deepseek", "https://api.deepseek.com/v1"},
		{"glm", "https://open.bigmodel.cn/api/paas/v4"},
		{"kimi", "https://api.moonshot.cn/v1"},
		{"qwen", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
		{"mistral", "https://api.mistral.ai/v1"},
		{"yi", "https://api.lingyiwanwu.com/v1"},
		{"anthropic", "https://api.anthropic.com/v1"},
		{"gemini", "https://generativelanguage.googleapis.com/v1beta/openai"},
		{"unknown_provider", "http://localhost:11434"},
		{"", "http://localhost:11434"},
	}
	for _, c := range cases {
		t.Run(c.provider, func(t *testing.T) {
			if got := DefaultEndpointFor(c.provider); got != c.want {
				t.Errorf("DefaultEndpointFor(%q) = %q, want %q", c.provider, got, c.want)
			}
		})
	}
}

// --- ParseToolsList ---

func TestParseToolsList_EmptyReturnsDefaults(t *testing.T) {
	tools := ParseToolsList("")
	if len(tools) != 2 {
		t.Fatalf("expected 2 default tools, got %d", len(tools))
	}
	names := []string{tools[0].Name, tools[1].Name}
	// Defaults are fetch_url and json_parse.
	if names[0] != "fetch_url" || names[1] != "json_parse" {
		t.Errorf("default tools = %v, want [fetch_url json_parse]", names)
	}
}

func TestParseToolsList_SingleTool(t *testing.T) {
	tools := ParseToolsList("file_read")
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "file_read" || tools[0].NodeName != "file_read" {
		t.Errorf("tool = %+v", tools[0])
	}
	if tools[0].Description == "" {
		t.Error("expected non-empty description")
	}
}

func TestParseToolsList_MultipleTools(t *testing.T) {
	tools := ParseToolsList("fetch_url, json_parse, transform")
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}
	wantNames := []string{"fetch_url", "json_parse", "transform"}
	for i, want := range wantNames {
		if tools[i].Name != want {
			t.Errorf("tool[%d].Name = %q, want %q", i, tools[i].Name, want)
		}
	}
}

func TestParseToolsList_UnknownToolsIgnored(t *testing.T) {
	tools := ParseToolsList("fetch_url, bogus_tool, also_fake, json_parse")
	if len(tools) != 2 {
		t.Fatalf("expected 2 known tools (unknown ignored), got %d: %+v", len(tools), tools)
	}
}

func TestParseToolsList_AllUnknownReturnsDefaults(t *testing.T) {
	tools := ParseToolsList("bogus1,fake2")
	if len(tools) != 2 {
		t.Fatalf("expected 2 default tools when all unknown, got %d", len(tools))
	}
	if tools[0].Name != "fetch_url" || tools[1].Name != "json_parse" {
		t.Errorf("expected defaults, got %v", tools)
	}
}

func TestParseToolsList_AllKnownTools(t *testing.T) {
	input := "fetch_url,http_request,file_read,file_write,json_parse,transform,combine,template,ollama,code_interpreter,execute"
	tools := ParseToolsList(input)
	if len(tools) != 11 {
		t.Fatalf("expected 11 tools, got %d", len(tools))
	}
}

func TestParseToolsList_TemplateMapsToRender(t *testing.T) {
	tools := ParseToolsList("template")
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].NodeName != "template_render" {
		t.Errorf("template tool NodeName = %q, want template_render", tools[0].NodeName)
	}
}

func TestParseToolsList_TrimsWhitespace(t *testing.T) {
	tools := ParseToolsList("   fetch_url   ,   json_parse   ")
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
}

// --- BaseAgentParams ---

func TestBaseAgentParams(t *testing.T) {
	params := BaseAgentParams()
	if len(params) != 4 {
		t.Fatalf("expected 4 params, got %d", len(params))
	}
	byName := map[string]ParamSchema{}
	for _, p := range params {
		byName[p.Name] = p
	}
	expectedNames := []string{"provider", "model", "api_key", "endpoint"}
	for _, name := range expectedNames {
		if _, ok := byName[name]; !ok {
			t.Errorf("expected param %q in BaseAgentParams", name)
		}
	}
	if byName["provider"].Default != "ollama" {
		t.Errorf("provider default = %q, want ollama", byName["provider"].Default)
	}
	if byName["model"].Default != "llama3" {
		t.Errorf("model default = %q, want llama3", byName["model"].Default)
	}
	if byName["api_key"].Required {
		t.Error("api_key should not be required")
	}
}

// --- TitleCase ---

func TestTitleCase(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"hello", "Hello"},
		{"Hello", "Hello"},
		{"HELLO", "HELLO"}, // only first rune uppercased, rest unchanged
		{"h", "H"},
		{"ünicode", "Ünicode"},
	}
	for _, c := range cases {
		if got := TitleCase(c.in); got != c.want {
			t.Errorf("TitleCase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- AgentTool ---

func TestAgentTool(t *testing.T) {
	tool := AgentTool{Name: "n", Description: "d", NodeName: "nn"}
	if tool.Name != "n" || tool.Description != "d" || tool.NodeName != "nn" {
		t.Errorf("AgentTool fields = %+v", tool)
	}
}
