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

package templates

import (
	"context"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/workflow"
)

// TestStockAlertTemplateContract guards the finance/stock-alert template
// against the field-name mistakes found in the code review that made it
// silently dysfunctional:
//
//   - an `input:` block (not a Workflow field) instead of `vars:` defaults —
//     yaml.Unmarshal dropped it, so symbol/threshold had no defaults;
//   - step-level `id:` / `input:` keys (not WorkflowStep fields) instead of
//     `name:` — dropped, and misleading;
//   - `timeout: 30s` — the http_request node takes integer seconds;
//   - a `price: "{{ .input }}"` template param — passed the LITERAL string
//     "{{ .input }}" into the Go template, rendering a broken alert message.
//
// The test parses the embedded template and asserts the contract end-to-end:
// vars defaults exist, steps are addressable by name, the engine substitutes
// {{var.*}} in the template param while leaving {{ .input }} verbatim for the
// template_render node, and the rendered alert contains the live price.
func TestStockAlertTemplateContract(t *testing.T) {
	raw, err := Embedded.ReadFile("templates/finance/stock-alert/workflow.yaml")
	if err != nil {
		t.Fatalf("read embedded stock-alert workflow: %v", err)
	}
	wf, err := workflow.ParseWorkflowFromContent(string(raw))
	if err != nil {
		t.Fatalf("parse stock-alert workflow: %v", err)
	}

	// vars defaults: symbol/threshold must be real workflow vars so the
	// template runs out of the box and --set can override them.
	if got := wf.Vars["symbol"]; got != "sh600519" {
		t.Errorf("vars.symbol = %q, want sh600519", got)
	}
	if got := wf.Vars["threshold"]; got != "1400" {
		t.Errorf("vars.threshold = %q, want 1400", got)
	}
	if len(wf.Steps) != 3 {
		t.Fatalf("len(steps) = %d, want 3 (http_request, json_parse, if)", len(wf.Steps))
	}

	// Steps are addressed by name: (not id: — the WorkflowStep field is name).
	names := map[string]bool{}
	for _, s := range wf.Steps {
		if s.Name != "" {
			names[s.Name] = true
		}
	}
	for _, want := range []string{"fetch_quote", "current_price"} {
		if !names[want] {
			t.Errorf("step name %q not found; got %v", want, names)
		}
	}

	// http_request timeout must be integer seconds ("30"), not "30s".
	if wf.Steps[0].Params["timeout"] != "30" {
		t.Errorf("http_request timeout = %q, want \"30\" (integer seconds)", wf.Steps[0].Params["timeout"])
	}
	if !strings.Contains(wf.Steps[0].Params["url"], "{{var.symbol}}") {
		t.Errorf("http_request url should reference {{var.symbol}}, got %q", wf.Steps[0].Params["url"])
	}

	// The if-step wraps the alert branch; its condition references the var.
	ifStep := wf.Steps[2]
	if !ifStep.IsIf() {
		t.Fatalf("third step is not an if-step: %+v", ifStep)
	}
	if ifStep.If.Condition != "gt:{{var.threshold}}" {
		t.Errorf("if condition = %q, want gt:{{var.threshold}}", ifStep.If.Condition)
	}
	if len(ifStep.If.Then) != 3 {
		t.Fatalf("then-branch has %d steps, want 3 (template_render, notify, file_write)", len(ifStep.If.Then))
	}

	// The alert message template must render the live price via {{ .input }}
	// and must NOT rely on a price param carrying the literal "{{ .input }}".
	tplStep := ifStep.If.Then[0]
	if tplStep.Node != "template_render" {
		t.Fatalf("first then-step is %q, want template_render", tplStep.Node)
	}
	if _, hasPriceParam := tplStep.Params["price"]; hasPriceParam {
		t.Error("template_render must not carry a price param — {{ .input }} renders directly in the template")
	}
	tpl := tplStep.Params["template"]
	if !strings.Contains(tpl, "{{ .input }}") {
		t.Errorf("template should render the price via {{ .input }}, got %q", tpl)
	}

	// End-to-end: engine substitutes vars in the template param and leaves
	// {{ .input }} verbatim for the Go template, so the rendered alert
	// contains the price in place of {{ .input }}.
	engine := workflow.NewExpressionEngine()
	engine.SetVariable("symbol", wf.Vars["symbol"])
	engine.SetVariable("threshold", wf.Vars["threshold"])
	evaluated, err := engine.Evaluate(tpl, "")
	if err != nil {
		t.Fatalf("evaluate template param: %v", err)
	}
	if !strings.Contains(evaluated, "sh600519") {
		t.Errorf("evaluated template should contain substituted symbol, got %q", evaluated)
	}
	if !strings.Contains(evaluated, "1400") {
		t.Errorf("evaluated template should contain substituted threshold, got %q", evaluated)
	}
	if !strings.Contains(evaluated, "{{ .input }}") {
		t.Errorf("evaluated template must keep {{ .input }} verbatim for template_render, got %q", evaluated)
	}
}

// TestStockAlertTemplateRegistryEntry asserts the template is indexed in the
// embedded skills-registry.json with the id the install command uses.
func TestStockAlertTemplateRegistryEntry(t *testing.T) {
	raw, err := Embedded.ReadFile("templates/skills-registry.json")
	if err != nil {
		t.Fatalf("read embedded skills-registry.json: %v", err)
	}
	if !strings.Contains(string(raw), `"finance/stock-alert"`) {
		t.Error("skills-registry.json does not index finance/stock-alert")
	}
}

// TestStockAlertAlertRendering executes the alert branch offline: the engine
// pre-evaluates the template param (vars substituted, {{ .input }} verbatim),
// then the template_render node renders the message with the live price as
// its input. This is the full alert-message path minus the network fetch.
func TestStockAlertAlertRendering(t *testing.T) {
	raw, err := Embedded.ReadFile("templates/finance/stock-alert/workflow.yaml")
	if err != nil {
		t.Fatalf("read embedded stock-alert workflow: %v", err)
	}
	wf, err := workflow.ParseWorkflowFromContent(string(raw))
	if err != nil {
		t.Fatalf("parse stock-alert workflow: %v", err)
	}
	tplStep := wf.Steps[2].If.Then[0]

	engine := workflow.NewExpressionEngine()
	engine.SetVariable("symbol", wf.Vars["symbol"])
	engine.SetVariable("threshold", wf.Vars["threshold"])
	params := map[string]string{}
	for k, v := range tplStep.Params {
		evaluated, err := engine.Evaluate(v, "1307.88")
		if err != nil {
			t.Fatalf("evaluate param %q: %v", k, err)
		}
		params[k] = evaluated
	}

	node := &nodes.TemplateRenderNode{}
	out, err := node.Execute(context.Background(), "1307.88", params)
	if err != nil {
		t.Fatalf("template_render execute: %v", err)
	}
	for _, want := range []string{"sh600519", "1307.88", "1400"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered alert missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "{{ .input }}") || strings.Contains(out, "{{var.") {
		t.Errorf("rendered alert contains unresolved expressions:\n%s", out)
	}
}
