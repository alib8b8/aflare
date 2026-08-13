// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/workflow"
)

// TestParseParams covers the --params token parser (断点8). It verifies both
// the space-separated multi-token form and the single-token form, and that
// tokens without "=" are ignored.
func TestParseParams(t *testing.T) {
	tests := []struct {
		name   string
		tokens []string
		want   map[string]string
	}{
		{
			name:   "empty",
			tokens: nil,
			want:   nil,
		},
		{
			name:   "single token without equals ignored",
			tokens: []string{"noequals"},
			want:   nil,
		},
		{
			name:   "single k=v",
			tokens: []string{"domains=example.com"},
			want:   map[string]string{"domains": "example.com"},
		},
		{
			name:   "multiple tokens",
			tokens: []string{"domains=example.com", "notify_url=https://hook.example"},
			want: map[string]string{
				"domains":    "example.com",
				"notify_url": "https://hook.example",
			},
		},
		{
			// A single --params token is split on whitespace into pairs;
			// a trailing bare token without "=" is ignored. To pass a value
			// containing spaces, quote the whole --params value and use a
			// delimiter the value itself does not contain, or pass each
			// domain as its own key.
			name:   "single token with embedded spaces splits pairs",
			tokens: []string{"domains=example.com google.com"},
			want:   map[string]string{"domains": "example.com"},
		},
		{
			name:   "single token with multiple pairs",
			tokens: []string{"a=1 b=2"},
			want:   map[string]string{"a": "1", "b": "2"},
		},
		{
			name:   "value containing equals sign",
			tokens: []string{"expr=a=b"},
			want:   map[string]string{"expr": "a=b"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseParams(tc.tokens)
			if len(got) != len(tc.want) {
				t.Fatalf("parseParams got %d entries, want %d (%v)", len(got), len(tc.want), got)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("parseParams[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

// TestSuggestSkeletonName verifies the kebab-case name derivation for the
// `aflare template new` hint (断点9).
func TestSuggestSkeletonName(t *testing.T) {
	tests := []struct {
		desc string
		want string
	}{
		{"", "my-workflow"},
		{"x", "my-workflow"}, // single short word -> fallback
		{"auto reply email", "auto-reply"},
		{"Help me auto reply email please", "auto-reply"}, // stopwords skipped
		{"fetch example.com and save to file", "fetch-example.com"},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			if got := suggestSkeletonName(tc.desc); got != tc.want {
				t.Errorf("suggestSkeletonName(%q) = %q, want %q", tc.desc, got, tc.want)
			}
		})
	}
}

// TestPrintCreateSuggestions_Output captures stdout and verifies that the
// suggestions include the --ai and template new hints (断点9). This function
// is invoked when keyword matching fails and no LLM is configured; it always
// prints the actionable hints regardless of the template registry state.
func TestPrintCreateSuggestions_Output(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	printCreateSuggestions("auto reply email")

	_ = w.Close()
	out, _ := io.ReadAll(r)

	got := string(out)
	for _, want := range []string{
		"未找到完全匹配的模板",
		"--ai",
		"aflare init",
		"aflare template new",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("suggestions missing %q in output:\n%s", want, got)
		}
	}
}

// TestSearchTemplatesForSuggestion_NoPanic verifies the helper does not panic
// when the template registry cannot be loaded (e.g. no templates directory in
// the test working directory). It returns either nil or a non-nil slice; the
// contract under test is graceful degradation, not a specific result.
func TestSearchTemplatesForSuggestion_NoPanic(t *testing.T) {
	// In the test working directory (internal/cli) there is typically no
	// templates/ subdirectory, so registry.Load() fails and the helper
	// returns nil. If a templates dir happens to exist, results may be
	// non-nil — either way it must not panic.
	got := searchTemplatesForSuggestion("ssl cert checker")
	// No assertion on len: just ensure no panic and no invalid entries.
	for _, s := range got {
		if s == nil {
			t.Error("search returned a nil SkillMeta entry")
		}
	}
}

// TestPrintInputSchemaHelp verifies the parameter help output (断点8) includes
// each field, its required/optional status, default value, and a copy-pasteable
// example command.
func TestPrintInputSchemaHelp(t *testing.T) {
	// Capture stdout.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	wf := &workflow.Workflow{
		Name: "SSL Certificate Checker",
		InputSchema: []workflow.InputField{
			{Name: "domains", Type: "string", Required: true},
			{Name: "notify_url", Type: "string", Required: false, Default: ""},
			{Name: "timeout", Type: "int", Required: false, Default: "30"},
		},
	}
	printInputSchemaHelp("ssl-cert-checker.yaml", wf)

	_ = w.Close()
	out, _ := io.ReadAll(r)
	got := string(out)

	for _, want := range []string{
		"此模板需要以下参数",
		"domains", // required field
		"必填",
		"notify_url", // optional field
		"选填",
		"（默认: 30）", // default value marker for timeout
		"示例：",
		"aflare run ssl-cert-checker.yaml --set", // 断点12: --set 替代 --params
		"--params-file",                          // 断点12: 提示文件参数
	} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("input schema help missing %q in output:\n%s", want, got)
		}
	}
}
