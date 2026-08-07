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

package workflow

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func FuzzExpressionEvaluate(f *testing.F) {
	f.Add("{{input}}", "hello world")
	f.Add("{{step.0}}", "")
	f.Add("{{var.name}}", "")
	f.Add("static text", "anything")
	f.Add("{{env.PATH}}", "")
	f.Add("{{loop.item}}", "")
	f.Add("{{step.0.jsonpath:$.a}}", `{"a":1}`)
	f.Add("{{", "")
	f.Add("}}", "")
	f.Add("{{{{}}}}", "")
	f.Add("{{unknown.value}}", "test")
	f.Add("{{step.0.jsonpath:$..a}}", `{"a":{"a":1}}`)

	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch", `{"users":[{"name":"Alice"}]}`)
	engine.SetVariable("name", "test")
	engine.SetLoopVars("item1", 0, 3)

	f.Fuzz(func(t *testing.T, expr string, input string) {
		done := make(chan struct{})
		var panicErr interface{}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicErr = r
				}
				close(done)
			}()
			_, _ = engine.Evaluate(expr, input)
		}()

		select {
		case <-done:
			if panicErr != nil {
				t.Fatalf("Evaluate panicked: %v\nexpr=%q input=%q", panicErr, expr, input)
			}
		case <-time.After(5 * time.Second):
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			t.Fatalf("Evaluate timed out\nexpr=%q input=%q\n%s", expr, input, buf[:n])
		}
	})
}

// FuzzExpressionEval fuzzes the expression evaluation function with a
// pre-configured engine that has step outputs, variables, and loop
// variables set. It covers the full dispatch path (resolveInstr) and
// ensures no panics or hangs across all expression variants.
func FuzzExpressionEval(f *testing.F) {
	// Seed corpus: cover all known expression prefixes and edge cases.
	seeds := []struct {
		expr  string
		input string
	}{
		// Basic expressions
		{"{{input}}", "hello world"},
		{"{{step.0}}", ""},
		{"{{step.fetch}}", ""},
		{"{{var.name}}", ""},
		{"{{env.PATH}}", ""},
		{"{{env.HOME}}", ""},
		{"{{loop.item}}", ""},
		{"{{loop.index}}", ""},
		{"{{loop.count}}", ""},
		{"{{loop.acc}}", ""},
		// Static text
		{"static text without expressions", "anything"},
		{"", ""},
		// JSONPath expressions
		{"{{step.0.jsonpath:$.a}}", `{"a":1}`},
		{"{{step.0.jsonpath:$..name}}", `{"users":[{"name":"Alice"}]}`},
		{"{{step.0.jsonpath:$.users[0].name}}", `{"users":[{"name":"Alice"}]}`},
		// Mixed expressions
		{"prefix {{input}} middle {{var.name}} suffix", "hello"},
		{"{{step.0}} {{step.1}}", ""},
		// Edge cases: braces
		{"{{", ""},
		{"}}", ""},
		{"{{{{}}}}", ""},
		{"{{x}}", ""},
		{"{not an expression}", "value"},
		// Unknown prefixes (should be left verbatim)
		{"{{unknown.prefix}}", "test"},
		{"{{.GoTemplate}}", ""},
		// Unicode and special characters
		{"{{input}} — 你好世界", "你好"},
		{"{{var.name}} \x00", ""},
		// Whitespace variations
		{"{{ input }}", "data"},
		{"{{  var.name  }}", ""},
		// Long expressions
		{strings.Repeat("{{input}}", 100), "x"},
		// Nested-like patterns
		{"{{input}} {{input}} {{input}}", "abc"},
	}

	for _, s := range seeds {
		f.Add(s.expr, s.input)
	}

	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch", `{"users":[{"name":"Alice"}],"count":1}`)
	engine.SetStepOutput(1, "process", "processed data")
	engine.SetVariable("name", "test-variable")
	engine.SetVariable("count", "42")
	engine.SetLoopVars("item1", 0, 3)
	engine.SetReduceVars("accumulated", "current", 1, 5)

	f.Fuzz(func(t *testing.T, expr string, input string) {
		done := make(chan struct{})
		var panicErr interface{}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicErr = r
				}
				close(done)
			}()
			_, _ = engine.Evaluate(expr, input)
		}()

		select {
		case <-done:
			if panicErr != nil {
				t.Fatalf("Evaluate panicked: %v\nexpr=%q input=%q", panicErr, expr, input)
			}
		case <-time.After(5 * time.Second):
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			t.Fatalf("Evaluate timed out\nexpr=%q input=%q\n%s", expr, input, buf[:n])
		}
	})
}
