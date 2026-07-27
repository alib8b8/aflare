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

package core

import (
	"testing"
)

// BenchmarkGetParam measures the parameter lookup helper that every node
// calls to resolve its params map. Covers the hit, miss, and empty-value
// branches in a single iteration to reflect a realistic node invocation.
func BenchmarkGetParam(b *testing.B) {
	params := map[string]string{
		"provider": "ollama",
		"model":    "llama3",
		"endpoint": "http://localhost:11434",
		"empty":    "",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetParam(params, "provider", "default")
		_ = GetParam(params, "missing", "default")
		_ = GetParam(params, "empty", "default")
	}
}

// BenchmarkParamInt measures integer parameter parsing with clamping,
// covering the parse-success, parse-failure, and empty/missing branches.
func BenchmarkParamInt(b *testing.B) {
	params := map[string]string{
		"count":   "42",
		"clamped": "99999",
		"bad":     "not-a-number",
		"empty":   "",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ParamInt(params, "count", 10, 0, 100)
		_ = ParamInt(params, "clamped", 10, 0, 100)
		_ = ParamInt(params, "bad", 10, 0, 100)
		_ = ParamInt(params, "empty", 10, 0, 100)
	}
}

// BenchmarkParseToolsList measures the tool-list parser used by ReAct agent
// nodes, at varying list sizes including unknown tool names that get dropped.
func BenchmarkParseToolsList(b *testing.B) {
	cases := []struct {
		name string
		list string
	}{
		{"single", "fetch_url"},
		{"few", "fetch_url,json_parse,transform"},
		{"all_known", "fetch_url,http_request,file_read,file_write,json_parse,transform,combine,template,ollama,code_interpreter,execute"},
		{"with_unknown", "fetch_url,unknown_tool,json_parse,another_fake"},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = ParseToolsList(c.list)
			}
		})
	}
}
