// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​​‌​​‌​‌​​​‌‌​‌​‌‌‌​​​​‌​‌‌‌​‌​​‌‌‌‌‌​‌​​‌‌‌‌​​​​​​​​​​​​​​​​​​‌‌​​‌‌‌​​‌‌‌‌​‌⁠
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
	"context"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes"
)

// jsonNode emits a configurable JSON payload, optionally fenced like an LLM
// would produce.
type jsonNode struct {
	name   string
	out    string
	fenced bool
}

func (n *jsonNode) Name() string        { return n.name }
func (n *jsonNode) Description() string { return "json emitting test node" }
func (n *jsonNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Input: "string", Output: "json"}
}

func (n *jsonNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if n.fenced {
		return "```json\n" + n.out + "\n```", nil
	}
	return n.out, nil
}

const personSchema = `{"type":"object","required":["name","score"],"properties":{"name":{"type":"string"},"score":{"type":"number","minimum":0,"maximum":100}},"additionalProperties":false}`

func TestExecuteWorkflow_OutputSchemaContract(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&jsonNode{name: "good", out: `{"name":"alice","score":42}`})
	reg.Register(&jsonNode{name: "fenced", out: `{"name":"bob","score":7}`, fenced: true})

	wf := &Workflow{
		Name: "contract-ok",
		Steps: []WorkflowStep{
			{Node: "good", OutputSchema: personSchema},
			{Node: "fenced", OutputSchema: personSchema},
		},
	}
	out, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("contract-compliant workflow failed: %v", err)
	}
	if !strings.Contains(out, `"bob"`) {
		t.Fatalf("unexpected final output %q", out)
	}
}

func TestExecuteWorkflow_OutputSchemaViolationFailsStep(t *testing.T) {
	reg := nodes.NewRegistry()
	// score out of range: contract violation on every attempt.
	reg.Register(&jsonNode{name: "bad", out: `{"name":"alice","score":142}`})

	wf := &Workflow{
		Name: "contract-violation",
		Steps: []WorkflowStep{
			{Node: "bad", OutputSchema: personSchema, Retry: 1},
		},
	}
	_, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Fatal("schema-violating output must fail the workflow")
	}
	if !strings.Contains(err.Error(), "output contract violated") {
		t.Fatalf("error must mention contract violation, got: %v", err)
	}
	if !strings.Contains(err.Error(), "/score") {
		t.Fatalf("error must point at the violated field, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 step result, got %d", len(results))
	}
}

func TestExecuteWorkflow_OutputSchemaNonObjectFails(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&jsonNode{name: "notjson", out: "plain text answer"})

	wf := &Workflow{
		Name: "contract-notjson",
		Steps: []WorkflowStep{
			{Node: "notjson", OutputSchema: personSchema},
		},
	}
	if _, _, err := ExecuteWorkflow(context.Background(), wf, reg); err == nil {
		t.Fatal("non-JSON output must violate the contract")
	}
}

func TestExecuteWorkflow_PreviewInputBoundsLargePayload(t *testing.T) {
	reg := nodes.NewRegistry()

	// Producer: emits a payload well above PreviewMaxBytes.
	producerOut := strings.Repeat("line of data 0123456789\n", 2000) // ~40 KB
	reg.Register(&jsonNode{name: "producer", out: producerOut})

	// Consumer: a test node that echoes the length of what it received.
	reg.Register(&lengthEchoNode{name: "consumer"})

	wf := &Workflow{
		Name: "preview-wf",
		Steps: []WorkflowStep{
			{Node: "producer"},
			{Node: "consumer", PreviewInput: true},
		},
	}
	_, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	got := results[1].Output
	if !strings.Contains(got, "bounded preview") {
		t.Fatalf("consumer must receive the preview marker, got: %.200s", got)
	}
	if l := len(got); l > 2*1024 {
		t.Fatalf("preview must stay small, consumer saw %d bytes", l)
	}
	if !strings.Contains(got, "bytes omitted") {
		t.Fatalf("preview must report elided size, got: %.200s", got)
	}
}

func TestExecuteWorkflow_PreviewInputPassthroughSmallPayload(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&jsonNode{name: "producer", out: "small payload"})
	reg.Register(&lengthEchoNode{name: "consumer"})

	wf := &Workflow{
		Name: "preview-small",
		Steps: []WorkflowStep{
			{Node: "producer"},
			{Node: "consumer", PreviewInput: true},
		},
	}
	_, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	if results[1].Output != "small payload" {
		t.Fatalf("small payloads must pass through unchanged, got %q", results[1].Output)
	}
}

func TestExecuteWorkflow_NoPreviewWithoutOptIn(t *testing.T) {
	reg := nodes.NewRegistry()
	big := strings.Repeat("x", 64*1024)
	reg.Register(&jsonNode{name: "producer", out: big})
	reg.Register(&lengthEchoNode{name: "consumer"})

	wf := &Workflow{
		Name: "no-preview",
		Steps: []WorkflowStep{
			{Node: "producer"},
			{Node: "consumer"}, // no preview_input
		},
	}
	_, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	if results[1].Output != big {
		t.Fatalf("without opt-in the full payload must flow, got %d bytes", len(results[1].Output))
	}
}

// lengthEchoNode returns exactly what it received, so tests can assert on the
// step input the executor handed over.
type lengthEchoNode struct {
	name string
}

func (n *lengthEchoNode) Name() string        { return n.name }
func (n *lengthEchoNode) Description() string { return "echo test node" }
func (n *lengthEchoNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: n.name, Input: "string", Output: "string"}
}

func (n *lengthEchoNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	return input, nil
}

func TestBoundedPreview_UnicodeSafety(t *testing.T) {
	// Multibyte payload: cuts must land on rune boundaries.
	s := strings.Repeat("你好世界\n", 5000) // 50k bytes of UTF-8
	p := BoundedPreview(s, PreviewMaxBytes)
	if !utf8ValidString(p) {
		t.Fatal("preview contains invalid UTF-8")
	}
	if !strings.Contains(p, "你好世界") {
		t.Fatal("head sample lost content")
	}
	if !strings.Contains(p, "bytes omitted") {
		t.Fatal("elision marker missing")
	}
}

func TestBoundedPreview_SingleHugeLine(t *testing.T) {
	// One giant line (base64-like): preview must still show head/tail
	// samples instead of an empty body.
	s := strings.Repeat("AQIDBAUG", 20000) // 160 KB single line
	p := BoundedPreview(s, PreviewMaxBytes)
	if !strings.Contains(p, "AQIDBAUG") {
		t.Fatal("single-line payload preview must keep sample content")
	}
	if l := len(p); l > 4*1024 {
		t.Fatalf("single-line preview must stay small, got %d bytes", l)
	}
}

func utf8ValidString(s string) bool {
	for _, r := range s {
		if r == 0xFFFD {
			return false
		}
	}
	return true
}
