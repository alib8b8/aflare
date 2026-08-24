// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌‌​​‌​‌‌​‌‌​‌​​‌‌​‌​‌​‌​​‌‌​​‌‌​​‌​‌​​​​‌‌‌‌​​​​​​​​​​​​​​​​​​‌​​‌‌​​‌‌​​​​‌​​⁠
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
	"testing"
)

func BenchmarkGenerateWorkflow_Simple(b *testing.B) {
	desc := "fetch example.com and save to file"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateWorkflow(desc)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateWorkflow_Complex(b *testing.B) {
	desc := "summarize AI news from hackernews and save to report.md with deepseek"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateWorkflow(desc)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseWorkflow(b *testing.B) {
	yamlContent := `name: Test Workflow
steps:
  - node: fetch_url
    params:
      url: https://example.com
  - node: transform
    params:
      operation: extract_text
  - node: file_write
    params:
      path: output.txt
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseWorkflowFromContent(yamlContent)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateWorkflow(b *testing.B) {
	wf := &Workflow{
		Name: "Test",
		Steps: []WorkflowStep{
			{Node: "echo", Params: map[string]string{"message": "hello"}},
			{Node: "transform", Params: map[string]string{"operation": "uppercase"}},
			{Node: "echo", Params: map[string]string{"message": "done"}},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateWorkflow(wf)
	}
}

func BenchmarkExpressionEvaluate_Simple(b *testing.B) {
	engine := NewExpressionEngine()
	engine.SetVariable("name", "world")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			_, _ = engine.Evaluate("Hello, {{var.name}}!", "")
		}
	}
}

func BenchmarkExpressionEvaluate_Multiple(b *testing.B) {
	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch", "step zero output")
	engine.SetStepOutput(1, "process", "step one output")
	engine.SetVariable("api_key", "secret123")
	engine.SetVariable("user", "alice")
	expr := "User: {{var.user}}, Key: {{var.api_key}}, Step0: {{step.0}}, Step1: {{step.1}}"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			_, _ = engine.Evaluate(expr, "input")
		}
	}
}
