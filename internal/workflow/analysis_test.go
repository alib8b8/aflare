// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any version.
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
	"reflect"
	"testing"
)

func TestIsLLMNode(t *testing.T) {
	cases := map[string]bool{
		"ollama":     true,
		"openai":     true,
		"deepseek":   true,
		"glm":        true,
		"qwen":       true,
		"anthropic":  true,
		"llm_router": true,
		// case-insensitive / whitespace tolerant
		"Ollama":   true,
		" OpenAI ": true, //nolint:gocritic // intentional: key with surrounding whitespace
		// non-LLM nodes
		"http_request": false,
		"execute":      false,
		"combine":      false,
		"notify":       false,
		"":             false,
		"unknown":      false,
	}
	for name, want := range cases {
		if got := IsLLMNode(name); got != want {
			t.Errorf("IsLLMNode(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestRequiresLLM(t *testing.T) {
	t.Run("nil and empty", func(t *testing.T) {
		if RequiresLLM(nil) {
			t.Error("RequiresLLM(nil) should be false")
		}
		if RequiresLLM(&Workflow{}) {
			t.Error("RequiresLLM(empty workflow) should be false")
		}
	})

	t.Run("non-LLM top-level step", func(t *testing.T) {
		wf := &Workflow{Steps: []WorkflowStep{{Node: "http_request"}}}
		if RequiresLLM(wf) {
			t.Error("http_request-only workflow should not require LLM")
		}
	})

	t.Run("LLM top-level step", func(t *testing.T) {
		wf := &Workflow{Steps: []WorkflowStep{{Node: "ollama"}}}
		if !RequiresLLM(wf) {
			t.Error("workflow with ollama step should require LLM")
		}
	})

	t.Run("LLM inside parallel", func(t *testing.T) {
		wf := &Workflow{Steps: []WorkflowStep{{
			Node:     "combine",
			Parallel: []Step{{Node: "openai"}},
		}}}
		if !RequiresLLM(wf) {
			t.Error("LLM inside parallel should require LLM")
		}
	})

	t.Run("LLM inside on_error", func(t *testing.T) {
		wf := &Workflow{Steps: []WorkflowStep{{
			Node:    "http_request",
			OnError: &Step{Node: "deepseek"},
		}}}
		if !RequiresLLM(wf) {
			t.Error("LLM inside on_error should require LLM")
		}
	})

	t.Run("looped LLM node", func(t *testing.T) {
		wf := &Workflow{Steps: []WorkflowStep{{
			Node: "qwen",
			Loop: &LoopConfig{Items: "{{var.items}}"},
		}}}
		if !RequiresLLM(wf) {
			t.Error("looped LLM node should require LLM")
		}
	})

	t.Run("LLM inside if/then", func(t *testing.T) {
		wf := &Workflow{Steps: []WorkflowStep{{
			Node: "combine",
			If: &IfConfig{
				Condition: "{{step.x}} != ''",
				Then:      []WorkflowStep{{Node: "glm"}},
				Else:      []WorkflowStep{{Node: "http_request"}},
			},
		}}}
		if !RequiresLLM(wf) {
			t.Error("LLM inside if/then should require LLM")
		}
	})

	t.Run("LLM inside map", func(t *testing.T) {
		wf := &Workflow{Steps: []WorkflowStep{{
			Node: "combine",
			Map: &MapConfig{
				Over:  "{{var.urls}}",
				Steps: []WorkflowStep{{Node: "ollama"}},
			},
		}}}
		if !RequiresLLM(wf) {
			t.Error("LLM inside map should require LLM")
		}
	})

	t.Run("LLM inside reduce", func(t *testing.T) {
		wf := &Workflow{Steps: []WorkflowStep{{
			Node: "combine",
			Reduce: &ReduceConfig{
				Over:    "{{var.items}}",
				Initial: "0",
				Steps:   []WorkflowStep{{Node: "kimi"}},
			},
		}}}
		if !RequiresLLM(wf) {
			t.Error("LLM inside reduce should require LLM")
		}
	})

	t.Run("LLM inside saga forward", func(t *testing.T) {
		wf := &Workflow{Steps: []WorkflowStep{{
			Node: "combine",
			Saga: &SagaConfig{Steps: []SagaStep{
				{Forward: WorkflowStep{Node: "openai"}},
			}},
		}}}
		if !RequiresLLM(wf) {
			t.Error("LLM inside saga forward should require LLM")
		}
	})

	t.Run("LLM inside saga compensate", func(t *testing.T) {
		comp := WorkflowStep{Node: "deepseek"}
		wf := &Workflow{Steps: []WorkflowStep{{
			Node: "combine",
			Saga: &SagaConfig{Steps: []SagaStep{
				{Forward: WorkflowStep{Node: "http_request"}, Compensate: &comp},
			}},
		}}}
		if !RequiresLLM(wf) {
			t.Error("LLM inside saga compensate should require LLM")
		}
	})

	t.Run("LLM inside capture_error", func(t *testing.T) {
		wf := &Workflow{Steps: []WorkflowStep{{
			Node:         "http_request",
			CaptureError: []WorkflowStep{{Node: "ollama"}},
		}}}
		if !RequiresLLM(wf) {
			t.Error("LLM inside capture_error should require LLM")
		}
	})
}

func TestExtractReferencedVars(t *testing.T) {
	t.Run("nil workflow", func(t *testing.T) {
		if got := ExtractReferencedVars(nil); got != nil {
			t.Errorf("ExtractReferencedVars(nil) = %v, want nil", got)
		}
	})

	t.Run("no references", func(t *testing.T) {
		wf := &Workflow{Steps: []WorkflowStep{{
			Node:   "http_request",
			Params: map[string]string{"url": "https://example.com"},
		}}}
		if got := ExtractReferencedVars(wf); got != nil {
			t.Errorf("ExtractReferencedVars with no refs = %v, want nil", got)
		}
	})

	t.Run("var syntax", func(t *testing.T) {
		wf := &Workflow{Steps: []WorkflowStep{{
			Node: "http_request",
			Params: map[string]string{
				"url":   "{{var.api_url}}",
				"token": "{{var.api_token}}",
			},
		}}}
		got := ExtractReferencedVars(wf)
		want := []string{"api_token", "api_url"} // sorted
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ExtractReferencedVars var = %v, want %v", got, want)
		}
	})

	t.Run("params syntax with spaces and dot", func(t *testing.T) {
		wf := &Workflow{Steps: []WorkflowStep{{
			Node: "execute",
			Params: map[string]string{
				"cmd": "{{ .params.timeout }}",
			},
		}}}
		got := ExtractReferencedVars(wf)
		want := []string{"timeout"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ExtractReferencedVars .params = %v, want %v", got, want)
		}
	})

	t.Run("dedupe across steps and nested", func(t *testing.T) {
		wf := &Workflow{Steps: []WorkflowStep{
			{Node: "http_request", Params: map[string]string{"url": "{{var.base}}"}},
			{Node: "combine", Condition: "{{var.threshold}} > 0"},
			{Node: "combine", Map: &MapConfig{
				Over:  "{{var.items}}",
				Steps: []WorkflowStep{{Node: "execute", Params: map[string]string{"cmd": "{{var.base}}"}}},
			}},
		}}
		got := ExtractReferencedVars(wf)
		want := []string{"base", "items", "threshold"} // deduped + sorted
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ExtractReferencedVars dedupe = %v, want %v", got, want)
		}
	})

	t.Run("ignores non-param references", func(t *testing.T) {
		wf := &Workflow{Steps: []WorkflowStep{{
			Node: "combine",
			Params: map[string]string{
				"prompt": "{{step.prev_output}}", // step.* not a param
			},
		}}}
		if got := ExtractReferencedVars(wf); got != nil {
			t.Errorf("ExtractReferencedVars should ignore step.* refs, got %v", got)
		}
	})
}
