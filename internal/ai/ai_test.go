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

package ai

import (
	"strings"
	"testing"
)

func TestWorkflowOptimizer_InvalidYAML(t *testing.T) {
	o := NewWorkflowOptimizer()
	report := o.Analyze("not: yaml: [")
	if report.Score != 0 {
		t.Errorf("expected score 0 for invalid YAML, got %d", report.Score)
	}
	if len(report.Suggestions) == 0 {
		t.Error("expected at least one suggestion for invalid YAML")
	}
}

func TestWorkflowOptimizer_EmptyWorkflow(t *testing.T) {
	o := NewWorkflowOptimizer()
	report := o.Analyze("name: test\nsteps: []")
	if report.Score >= 100 {
		t.Error("expected score below 100 for empty workflow")
	}
	foundNoSteps := false
	for _, s := range report.Suggestions {
		if strings.Contains(s.Message, "no steps") {
			foundNoSteps = true
			break
		}
	}
	if !foundNoSteps {
		t.Error("expected suggestion about missing steps")
	}
}

func TestWorkflowOptimizer_DuplicateNodes(t *testing.T) {
	yaml := `
name: dup-test
steps:
  - node: fetch_url
    params:
      url: https://example.com
  - node: fetch_url
    params:
      url: https://example.com
`
	o := NewWorkflowOptimizer()
	report := o.Analyze(yaml)
	found := false
	for _, s := range report.Suggestions {
		if strings.Contains(s.Message, "duplicate") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected duplicate node suggestion")
	}
}

func TestWorkflowOptimizer_ComplexCondition(t *testing.T) {
	yaml := `
name: cond-test
steps:
  - node: condition
    condition: "a > 1 && b < 2 || c == 3 && d != 4 || e > 5"
`
	o := NewWorkflowOptimizer()
	report := o.Analyze(yaml)
	found := false
	for _, s := range report.Suggestions {
		if strings.Contains(s.Message, "complex condition") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected complex condition suggestion")
	}
}

func TestWorkflowOptimizer_HardcodedSecret(t *testing.T) {
	yaml := `
name: secret-test
steps:
  - node: openai
    params:
      api_key: sk-123456789012345678901234567890
`
	o := NewWorkflowOptimizer()
	report := o.Analyze(yaml)
	found := false
	for _, s := range report.Suggestions {
		if s.Severity == SeverityError && strings.Contains(s.Message, "secret") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected hardcoded secret error")
	}
}

func TestWorkflowOptimizer_Timeout(t *testing.T) {
	yaml := `
name: timeout-test
steps:
  - node: fetch_url
    params:
      url: https://example.com
      _timeout: 500ms
`
	o := NewWorkflowOptimizer()
	report := o.Analyze(yaml)
	found := false
	for _, s := range report.Suggestions {
		if strings.Contains(s.Message, "short") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected short timeout warning")
	}
}

func TestWorkflowOptimizer_RetryPolicy(t *testing.T) {
	yaml := `
name: retry-test
steps:
  - node: openai
    params:
      model: gpt-4
`
	o := NewWorkflowOptimizer()
	report := o.Analyze(yaml)
	found := false
	for _, s := range report.Suggestions {
		if strings.Contains(s.Message, "retry policy") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected retry policy suggestion")
	}
}

func TestWorkflowOptimizer_Parallelization(t *testing.T) {
	yaml := `
name: parallel-test
steps:
  - node: fetch_url
    params:
      url: https://a.com
  - node: fetch_url
    params:
      url: https://b.com
`
	o := NewWorkflowOptimizer()
	report := o.Analyze(yaml)
	found := false
	for _, s := range report.Suggestions {
		if strings.Contains(s.Message, "parallelized") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected parallelization suggestion")
	}
}

func TestWorkflowOptimizer_UnusedOutput(t *testing.T) {
	yaml := `
name: unused-test
steps:
  - node: fetch_url
    params:
      url: https://example.com
  - node: file_write
    params:
      path: out.txt
`
	o := NewWorkflowOptimizer()
	report := o.Analyze(yaml)
	found := false
	for _, s := range report.Suggestions {
		if strings.Contains(s.Message, "unused") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected unused output suggestion")
	}
}

func TestWorkflowExplainer_InvalidYAML(t *testing.T) {
	e := NewWorkflowExplainer()
	out := e.Explain("bad yaml: [")
	if !strings.Contains(out, "Unable to explain") {
		t.Errorf("expected failure message, got: %s", out)
	}
}

func TestWorkflowExplainer_EmptyWorkflow(t *testing.T) {
	e := NewWorkflowExplainer()
	out := e.Explain("name: empty\nsteps: []")
	if !strings.Contains(out, "no steps") {
		t.Errorf("expected no-steps message, got: %s", out)
	}
}

func TestWorkflowExplainer_Normal(t *testing.T) {
	yaml := `
name: summarize-web
steps:
  - node: fetch_url
    params:
      url: https://example.com
  - node: openai
    params:
      model: gpt-4
      system: You are a helpful assistant that summarizes text concisely.
  - node: file_write
    params:
      path: summary.txt
`
	e := NewWorkflowExplainer()
	out := e.Explain(yaml)
	if !strings.Contains(out, "summarize-web") {
		t.Error("expected workflow name in explanation")
	}
	if !strings.Contains(out, "Fetch content from https://example.com") {
		t.Error("expected fetch step description")
	}
	if !strings.Contains(out, "Call openai") {
		t.Error("expected openai step description")
	}
	if !strings.Contains(out, "Write the result to 'summary.txt'") {
		t.Error("expected file_write step description")
	}
}

func TestWorkflowCompleter_InvalidYAML(t *testing.T) {
	c := NewWorkflowCompleter()
	out := c.Complete("bad yaml: [")
	if out != "bad yaml: [" {
		t.Error("expected original input for invalid YAML")
	}
}

func TestWorkflowCompleter_EmptySteps(t *testing.T) {
	c := NewWorkflowCompleter()
	out := c.Complete("name: empty\nsteps: []")
	if !strings.Contains(out, "steps: []") {
		t.Error("expected empty steps to remain unchanged")
	}
}

func TestWorkflowCompleter_FetchPattern(t *testing.T) {
	yaml := `
name: fetch-test
steps:
  - node: fetch_url
    params:
      url: https://example.com
`
	c := NewWorkflowCompleter()
	out := c.Complete(yaml)
	if !strings.Contains(out, "openai") {
		t.Error("expected openai step to be added after fetch_url")
	}
	if !strings.Contains(out, "file_write") {
		t.Error("expected file_write step to be added after openai")
	}
}

func TestWorkflowCompleter_ReadPattern(t *testing.T) {
	yaml := `
name: read-test
steps:
  - node: file_read
    params:
      path: input.txt
`
	c := NewWorkflowCompleter()
	out := c.Complete(yaml)
	if !strings.Contains(out, "openai") {
		t.Error("expected openai step to be added after file_read")
	}
	if !strings.Contains(out, "file_write") {
		t.Error("expected file_write step to be added after openai")
	}
}

func TestWorkflowCompleter_HTTPPattern(t *testing.T) {
	yaml := `
name: http-test
steps:
  - node: http_request
    params:
      method: GET
      url: https://api.example.com/data
`
	c := NewWorkflowCompleter()
	out := c.Complete(yaml)
	if !strings.Contains(out, "json_parse") {
		t.Error("expected json_parse step to be added after http_request")
	}
	if !strings.Contains(out, "file_write") {
		t.Error("expected file_write step to be added after json_parse")
	}
}

func TestNaturalLanguageQuery_InvalidYAML(t *testing.T) {
	q := NewNaturalLanguageQuery()
	out := q.Query("bad yaml", "how many steps?")
	if !strings.Contains(out, "invalid") {
		t.Error("expected invalid yaml response")
	}
}

func TestNaturalLanguageQuery_StepCount(t *testing.T) {
	yaml := `
name: query-test
steps:
  - node: fetch_url
  - node: openai
  - node: file_write
`
	q := NewNaturalLanguageQuery()
	out := q.Query(yaml, "how many steps?")
	if !strings.Contains(out, "3") {
		t.Errorf("expected 3 steps, got: %s", out)
	}
}

func TestNaturalLanguageQuery_NodeUsage(t *testing.T) {
	yaml := `
name: query-test
steps:
  - node: openai
  - node: openai
  - node: file_write
`
	q := NewNaturalLanguageQuery()
	out := q.Query(yaml, "how many steps use openai?")
	if !strings.Contains(out, "2") {
		t.Errorf("expected 2 openai steps, got: %s", out)
	}
}

func TestNaturalLanguageQuery_NodeTypes(t *testing.T) {
	yaml := `
name: query-test
steps:
  - node: fetch_url
  - node: openai
`
	q := NewNaturalLanguageQuery()
	out := q.Query(yaml, "what nodes are used?")
	if !strings.Contains(out, "fetch_url") || !strings.Contains(out, "openai") {
		t.Errorf("expected node list, got: %s", out)
	}
}

func TestNaturalLanguageQuery_Timeout(t *testing.T) {
	yaml := `
name: query-test
steps:
  - node: fetch_url
    params:
      _timeout: 30s
  - node: openai
`
	q := NewNaturalLanguageQuery()
	out := q.Query(yaml, "timeout configuration?")
	if !strings.Contains(out, "fetch_url") {
		t.Errorf("expected timeout info for fetch_url, got: %s", out)
	}
}

func TestNaturalLanguageQuery_Retry(t *testing.T) {
	yaml := `
name: query-test
steps:
  - node: openai
    retry: 3
  - node: file_write
`
	q := NewNaturalLanguageQuery()
	out := q.Query(yaml, "retry configuration?")
	if !strings.Contains(out, "retry=3") {
		t.Errorf("expected retry info, got: %s", out)
	}
}

func TestNaturalLanguageQuery_Parallel(t *testing.T) {
	yaml := `
name: query-test
steps:
  - node: combine
    parallel:
      - node: fetch_url
      - node: fetch_url
`
	q := NewNaturalLanguageQuery()
	out := q.Query(yaml, "parallel steps?")
	if !strings.Contains(out, "1 step(s)") {
		t.Errorf("expected parallel step count, got: %s", out)
	}
}

func TestNaturalLanguageQuery_Name(t *testing.T) {
	yaml := `name: my-workflow
steps:
  - node: fetch_url
`
	q := NewNaturalLanguageQuery()
	out := q.Query(yaml, "what is the name?")
	if !strings.Contains(out, "my-workflow") {
		t.Errorf("expected workflow name, got: %s", out)
	}
}

func TestNaturalLanguageQuery_Description(t *testing.T) {
	yaml := `
name: my-workflow
description: A test workflow
steps:
  - node: fetch_url
`
	q := NewNaturalLanguageQuery()
	out := q.Query(yaml, "what is the description?")
	if !strings.Contains(out, "A test workflow") {
		t.Errorf("expected description, got: %s", out)
	}
}
