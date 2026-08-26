// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌‌​​‌‌​​​‌​‌​​‌‌‌‌‌​‌​‌​​​‌​‌​​‌‌‌​​​​​​‌​‌​​‌‌​​‌‌‌​‌​‌‌‌​‌​‌​​​‌​​​​​​​​​​​​​​​​​​‌​​​​‌‌​​‌​​​‌​⁠
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

package nodes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestParsePipelineConfig_YAML pins the documented YAML support (schema:
// "YAML or JSON pipeline configuration"): block-style YAML parses, and the
// snake_case yaml tags (depends_on / input_from / timeout_seconds) map
// correctly. Before this was implemented, any non-JSON input died with
// "unsupported format, use JSON" despite the schema's claim.
func TestParsePipelineConfig_YAML(t *testing.T) {
	config, err := parsePipelineConfig(`
steps:
  - name: fetch
    node: http_request
    input: "seed"
    depends_on:
      - earlier
    input_from:
      - earlier
    params:
      url: "https://example.com"
timeout_seconds: 42
`, "auto")
	if err != nil {
		t.Fatalf("YAML parse failed: %v", err)
	}
	if len(config.Steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(config.Steps))
	}
	s := config.Steps[0]
	if s.Name != "fetch" || s.Node != "http_request" || s.Input != "seed" {
		t.Errorf("step fields = %+v", s)
	}
	if len(s.DependsOn) != 1 || s.DependsOn[0] != "earlier" {
		t.Errorf("depends_on = %v, want [earlier]", s.DependsOn)
	}
	if len(s.InputFrom) != 1 || s.InputFrom[0] != "earlier" {
		t.Errorf("input_from = %v, want [earlier]", s.InputFrom)
	}
	if s.Params["url"] != "https://example.com" {
		t.Errorf("params[url] = %q", s.Params["url"])
	}
	if config.TimeoutSeconds != 42 {
		t.Errorf("timeout_seconds = %d, want 42", config.TimeoutSeconds)
	}
}

// TestParsePipelineConfig_AutoSniff verifies format=auto dispatches by
// payload shape: {/[ prefix → JSON, anything else → YAML.
func TestParsePipelineConfig_AutoSniff(t *testing.T) {
	jsonCfg, err := parsePipelineConfig(`{"steps":[{"name":"a","node":"x"}]}`, "auto")
	if err != nil {
		t.Fatalf("auto/JSON parse failed: %v", err)
	}
	if len(jsonCfg.Steps) != 1 || jsonCfg.Steps[0].Name != "a" {
		t.Errorf("auto/JSON steps = %+v", jsonCfg.Steps)
	}

	yamlCfg, err := parsePipelineConfig("steps:\n  - name: a\n    node: x\n", "auto")
	if err != nil {
		t.Fatalf("auto/YAML parse failed: %v", err)
	}
	if len(yamlCfg.Steps) != 1 || yamlCfg.Steps[0].Name != "a" {
		t.Errorf("auto/YAML steps = %+v", yamlCfg.Steps)
	}
}

// TestParsePipelineConfig_ExplicitFormat verifies format=json / format=yaml
// force their parser (precise error messages) and that an invalid format
// value fails fast instead of being silently ignored.
func TestParsePipelineConfig_ExplicitFormat(t *testing.T) {
	yamlInput := "steps:\n  - name: a\n    node: x\n"

	if _, err := parsePipelineConfig(yamlInput, "json"); err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("format=json on YAML input: err = %v, want invalid JSON", err)
	}
	if _, err := parsePipelineConfig(yamlInput, "yaml"); err != nil {
		t.Errorf("format=yaml on YAML input failed: %v", err)
	}
	if _, err := parsePipelineConfig(`{"steps":[]}`, "yaml"); err != nil {
		t.Errorf("format=yaml on JSON input failed (YAML is a superset): %v", err)
	}
	if _, err := parsePipelineConfig(yamlInput, "xml"); err == nil || !strings.Contains(err.Error(), `invalid format "xml"`) {
		t.Errorf("invalid format value: err = %v, want invalid format", err)
	}
}

// TestPipelineNode_DownstreamSkippedOnUpstreamFailure pins standard DAG
// failure semantics inside the pipeline node: when an upstream step fails,
// every transitive downstream step is cascade-skipped (not executed with
// missing input), independent branches still run, and the overall pipeline
// reports failure with the root cause. Previously a failed step was marked
// completed and its dependents were scheduled anyway.
func TestPipelineNode_DownstreamSkippedOnUpstreamFailure(t *testing.T) {
	output, err := (&PipelineNode{}).Execute(context.Background(), `
steps:
  - name: a_bad
    node: template_render
    params:
      template: "{{ broken"
  - name: b_dependent
    node: template_render
    params:
      template: "should not run"
    depends_on: [a_bad]
  - name: c_cascade
    node: template_render
    params:
      template: "should not run either"
    depends_on: [b_dependent]
  - name: d_independent
    node: template_render
    params:
      template: "still runs"
`, map[string]string{})
	if err != nil {
		t.Fatalf("pipeline execution failed: %v", err)
	}

	var pr PipelineResult
	if err := json.Unmarshal([]byte(output), &pr); err != nil {
		t.Fatalf("failed to unmarshal pipeline result: %v\n%s", err, output)
	}

	if pr.Success {
		t.Errorf("pipeline should report failure")
	}
	byName := map[string]PipelineStepResult{}
	for _, r := range pr.Results {
		byName[r.Name] = r
	}

	// Root failure: real error, not a skip.
	if r := byName["a_bad"]; r.Error == "" || r.Skipped {
		t.Errorf("a_bad should carry a real error, got %+v", r)
	}
	// Direct dependent skipped, blaming the root.
	if r := byName["b_dependent"]; !r.Skipped || r.Output != "" || r.Error != `skipped: upstream step "a_bad" failed` {
		t.Errorf("b_dependent should be skipped blaming a_bad, got %+v", r)
	}
	// Cascade: skipped transitively, blaming its direct dependency.
	if r := byName["c_cascade"]; !r.Skipped || r.Output != "" || r.Error != `skipped: upstream step "b_dependent" failed` {
		t.Errorf("c_cascade should be cascade-skipped blaming b_dependent, got %+v", r)
	}
	// Independent branch unaffected.
	if r := byName["d_independent"]; r.Error != "" || r.Skipped || r.Output != "still runs" {
		t.Errorf("d_independent should still run successfully, got %+v", r)
	}
}

// TestPipelineNode_ExecuteYAML is an end-to-end run: a YAML pipeline with a
// dependency chain (b depends_on a, c input_from a+b) executes through the
// real node and reports success. This is the exact scenario the schema
// advertised but previously rejected.
func TestPipelineNode_ExecuteYAML(t *testing.T) {
	output, err := (&PipelineNode{}).Execute(context.Background(), `
steps:
  - name: a
    node: template_render
    params:
      template: "A"
  - name: b
    node: template_render
    params:
      template: "B"
    depends_on: [a]
  - name: c
    node: template_render
    params:
      template: "C+{{ .input }}"
    depends_on: [a, b]
    input_from: [a, b]
`, map[string]string{})
	if err != nil {
		t.Fatalf("YAML pipeline execution failed: %v", err)
	}
	if !strings.Contains(output, `"success": true`) {
		t.Errorf("pipeline not successful:\n%s", output)
	}
	for _, name := range []string{"\"a\"", "\"b\"", "\"c\""} {
		if !strings.Contains(output, name) {
			t.Errorf("result missing step %s:\n%s", name, output)
		}
	}
	if !strings.Contains(output, "C+A") {
		t.Errorf("input_from join missing in output:\n%s", output)
	}
}
