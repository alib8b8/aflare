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
	"regexp"
	"strings"
)

// llmNodeNames is the set of node names that invoke an LLM provider and thus
// require a configured LLM (API key for cloud providers, a running Ollama for
// the local provider). Used by RequiresLLM to pre-flight `aflare run` so a
// missing LLM config is reported up front with an actionable hint instead of
// failing mid-execution at the LLM node.
var llmNodeNames = map[string]bool{
	"ollama": true, "openai": true, "deepseek": true, "glm": true,
	"kimi": true, "qwen": true, "mistral": true, "yi": true,
	"anthropic": true, "gemini": true, "ascend": true, "cambricon": true,
	"hygon": true, "baichuan": true, "internlm": true, "xverse": true,
	"minimax": true, "coze": true, "ima": true, "mimo": true,
	"llm_router": true,
}

// IsLLMNode reports whether the given node name invokes an LLM provider.
func IsLLMNode(name string) bool {
	return llmNodeNames[strings.ToLower(strings.TrimSpace(name))]
}

// RequiresLLM reports whether the workflow contains any step that invokes an
// LLM node (including nested compound steps: parallel / loop / map / reduce /
// saga / if / capture_error / on_error). This lets the CLI warn the user
// before running a workflow whose LLM dependency is unmet (断点A).
func RequiresLLM(wf *Workflow) bool {
	if wf == nil {
		return false
	}
	for i := range wf.Steps {
		if stepRequiresLLM(&wf.Steps[i]) {
			return true
		}
	}
	return false
}

// stepRequiresLLM checks a single WorkflowStep and its nested sub-steps.
func stepRequiresLLM(s *WorkflowStep) bool {
	if s == nil {
		return false
	}
	if IsLLMNode(s.Node) {
		return true
	}
	for i := range s.Parallel {
		if IsLLMNode(s.Parallel[i].Node) {
			return true
		}
	}
	if s.OnError != nil && IsLLMNode(s.OnError.Node) {
		return true
	}
	if s.Loop != nil && IsLLMNode(s.Node) {
		return true
	}
	if s.If != nil {
		for i := range s.If.Then {
			if stepRequiresLLM(&s.If.Then[i]) {
				return true
			}
		}
		for i := range s.If.Else {
			if stepRequiresLLM(&s.If.Else[i]) {
				return true
			}
		}
	}
	if s.Map != nil {
		for i := range s.Map.Steps {
			if stepRequiresLLM(&s.Map.Steps[i]) {
				return true
			}
		}
	}
	if s.Reduce != nil {
		for i := range s.Reduce.Steps {
			if stepRequiresLLM(&s.Reduce.Steps[i]) {
				return true
			}
		}
	}
	if s.Saga != nil {
		for i := range s.Saga.Steps {
			if stepRequiresLLM(&s.Saga.Steps[i].Forward) {
				return true
			}
			if s.Saga.Steps[i].Compensate != nil && stepRequiresLLM(s.Saga.Steps[i].Compensate) {
				return true
			}
		}
	}
	for i := range s.CaptureError {
		if stepRequiresLLM(&s.CaptureError[i]) {
			return true
		}
	}
	return false
}

// paramRefRegex matches template variable references that resolve to
// workflow parameters supplied via `--set` / `--params` at run time. It covers
// both the runtime syntax `{{var.NAME}}` and the Go-template-style
// `{{ .params.NAME }}` / `{{params.NAME}}` that some hand-written YAMLs use.
var paramRefRegex = regexp.MustCompile(`\{\{\s*\.?\s*(?:var|params)\s*\.\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)

// ExtractReferencedVars returns the deduplicated, sorted list of parameter
// variable names referenced anywhere in the workflow's steps (params values,
// conditions, and nested compound steps). This is used to surface a parameter
// help hint for templates that lack an explicit input_schema but reference
// `{{var.X}}` / `{{ .params.X }}` in their YAML (断点E), so the user is told
// which --set values to provide instead of hitting an opaque "variable not
// found" error mid-run.
func ExtractReferencedVars(wf *Workflow) []string {
	if wf == nil {
		return nil
	}
	seen := make(map[string]struct{})
	collect := func(values ...string) {
		for _, v := range values {
			for _, m := range paramRefRegex.FindAllStringSubmatch(v, -1) {
				if len(m) > 1 && m[1] != "" {
					seen[m[1]] = struct{}{}
				}
			}
		}
	}
	for i := range wf.Steps {
		collectStepRefs(&wf.Steps[i], collect)
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	// Deterministic order for stable help output.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// collectStepRefs walks a step's string fields and nested sub-steps, calling
// collect for each string that may contain a parameter reference.
func collectStepRefs(s *WorkflowStep, collect func(...string)) {
	if s == nil {
		return
	}
	for _, v := range s.Params {
		collect(v)
	}
	collect(s.Condition)
	for i := range s.Parallel {
		for _, v := range s.Parallel[i].Params {
			collect(v)
		}
		collect(s.Parallel[i].Condition)
	}
	if s.OnError != nil {
		for _, v := range s.OnError.Params {
			collect(v)
		}
		collect(s.OnError.Condition)
	}
	if s.If != nil {
		collect(s.If.Condition)
		for i := range s.If.Then {
			collectStepRefs(&s.If.Then[i], collect)
		}
		for i := range s.If.Else {
			collectStepRefs(&s.If.Else[i], collect)
		}
	}
	if s.Map != nil {
		collect(s.Map.Over)
		for i := range s.Map.Steps {
			collectStepRefs(&s.Map.Steps[i], collect)
		}
	}
	if s.Reduce != nil {
		collect(s.Reduce.Over, s.Reduce.Initial)
		for i := range s.Reduce.Steps {
			collectStepRefs(&s.Reduce.Steps[i], collect)
		}
	}
	if s.Loop != nil {
		collect(s.Loop.Items)
	}
	if s.Saga != nil {
		for i := range s.Saga.Steps {
			collectStepRefs(&s.Saga.Steps[i].Forward, collect)
			if s.Saga.Steps[i].Compensate != nil {
				collectStepRefs(s.Saga.Steps[i].Compensate, collect)
			}
		}
	}
	for i := range s.CaptureError {
		collectStepRefs(&s.CaptureError[i], collect)
	}
}
