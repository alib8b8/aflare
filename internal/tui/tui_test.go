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

package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	m := NewModel("test workflow", "test.yaml", 3)

	if m.workflowName != "test workflow" {
		t.Errorf("expected workflow name 'test workflow', got '%s'", m.workflowName)
	}

	if m.workflowPath != "test.yaml" {
		t.Errorf("expected path 'test.yaml', got '%s'", m.workflowPath)
	}

	if len(m.steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(m.steps))
	}

	for i, step := range m.steps {
		if step.Status != StatusPending {
			t.Errorf("expected step %d to be pending, got %d", i, step.Status)
		}
	}
}

func TestModel_Init(t *testing.T) {
	m := NewModel("test", "test.yaml", 2)
	cmd := m.Init()
	if cmd != nil {
		t.Error("expected Init to return nil command")
	}
}

func TestModel_WorkflowStartMsg(t *testing.T) {
	m := NewModel("old", "old.yaml", 2)

	msg := WorkflowStartMsg{
		Name:  "new workflow",
		Path:  "new.yaml",
		Steps: 5,
	}

	newM, _ := m.Update(msg)
	model := newM.(*Model)

	if model.workflowName != "new workflow" {
		t.Errorf("expected name 'new workflow', got '%s'", model.workflowName)
	}

	if len(model.steps) != 5 {
		t.Errorf("expected 5 steps, got %d", len(model.steps))
	}
}

func TestModel_StepStartMsg(t *testing.T) {
	m := NewModel("test", "test.yaml", 2)

	msg := StepStartMsg{
		Index: 0,
		Name:  "fetch_url",
	}

	newM, _ := m.Update(msg)
	model := newM.(*Model)

	if model.steps[0].Name != "fetch_url" {
		t.Errorf("expected step name 'fetch_url', got '%s'", model.steps[0].Name)
	}

	if model.steps[0].Status != StatusRunning {
		t.Errorf("expected status running, got %d", model.steps[0].Status)
	}
}

func TestModel_StepEndMsg_Success(t *testing.T) {
	m := NewModel("test", "test.yaml", 2)
	m.steps[0].Status = StatusRunning
	m.steps[0].Name = "fetch_url"

	msg := StepEndMsg{
		Index:    0,
		Name:     "fetch_url",
		Output:   "result data",
		Error:    nil,
		Duration: 100 * time.Millisecond,
	}

	newM, _ := m.Update(msg)
	model := newM.(*Model)

	if model.steps[0].Status != StatusDone {
		t.Errorf("expected status done, got %d", model.steps[0].Status)
	}

	if model.steps[0].Output != "result data" {
		t.Errorf("expected output 'result data', got '%s'", model.steps[0].Output)
	}
}

func TestModel_StepEndMsg_Error(t *testing.T) {
	m := NewModel("test", "test.yaml", 2)
	m.steps[0].Status = StatusRunning
	m.steps[0].Name = "fetch_url"

	msg := StepEndMsg{
		Index:    0,
		Name:     "fetch_url",
		Error:    errors.New("connection failed"),
		Duration: 50 * time.Millisecond,
	}

	newM, _ := m.Update(msg)
	model := newM.(*Model)

	if model.steps[0].Status != StatusError {
		t.Errorf("expected status error, got %d", model.steps[0].Status)
	}

	if model.steps[0].Error != "connection failed" {
		t.Errorf("expected error 'connection failed', got '%s'", model.steps[0].Error)
	}
}

func TestModel_WorkflowEndMsg_Success(t *testing.T) {
	m := NewModel("test", "test.yaml", 2)

	msg := WorkflowEndMsg{Success: true}
	newM, _ := m.Update(msg)
	model := newM.(*Model)

	if !model.finished {
		t.Error("expected workflow to be finished")
	}

	if !model.success {
		t.Error("expected workflow to be successful")
	}
}

func TestModel_WorkflowEndMsg_Failure(t *testing.T) {
	m := NewModel("test", "test.yaml", 2)

	msg := WorkflowEndMsg{Success: false}
	newM, _ := m.Update(msg)
	model := newM.(*Model)

	if !model.finished {
		t.Error("expected workflow to be finished")
	}

	if model.success {
		t.Error("expected workflow to be unsuccessful")
	}
}

func TestModel_WindowSizeMsg(t *testing.T) {
	m := NewModel("test", "test.yaml", 2)

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newM, _ := m.Update(msg)
	model := newM.(*Model)

	if model.width != 100 {
		t.Errorf("expected width 100, got %d", model.width)
	}

	if model.height != 50 {
		t.Errorf("expected height 50, got %d", model.height)
	}
}

func TestModel_View(t *testing.T) {
	m := NewModel("test workflow", "test.yaml", 2)
	m.steps[0] = Step{
		Name:   "fetch_url",
		Status: StatusDone,
		Output: "hello world",
	}
	m.steps[1] = Step{
		Name:   "ollama",
		Status: StatusRunning,
	}
	m.finished = false

	view := m.View()

	if view == "" {
		t.Error("expected non-empty view")
	}

	// Should contain workflow name
	if !contains(view, "test workflow") {
		t.Error("view should contain workflow name")
	}

	// Should contain step names
	if !contains(view, "fetch_url") {
		t.Error("view should contain step name")
	}
}

func TestModel_View_Finished(t *testing.T) {
	m := NewModel("test", "test.yaml", 1)
	m.steps[0] = Step{
		Name:   "done",
		Status: StatusDone,
	}
	m.finished = true
	m.success = true

	view := m.View()

	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestRenderStepStatus(t *testing.T) {
	m := NewModel("test", "test.yaml", 1)

	tests := []struct {
		status   StepStatus
		expected string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusDone, "done"},
		{StatusError, "error"},
	}

	for _, tt := range tests {
		step := Step{Status: tt.status}
		result := m.renderStepStatus(step)
		if !contains(result, tt.expected) {
			t.Errorf("renderStepStatus(%d) = %q, expected to contain %q", tt.status, result, tt.expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============================================================================
// mermaid_helpers.go tests
// ============================================================================

func TestSanitizeMermaidLabel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"plain text", "hello", "hello"},
		{"trim spaces", "  hello  ", "hello"},
		{"strip double quotes", `"hello"`, "hello"},
		{"strip quotes with spaces", ` "hello" `, "hello"},
		{"newline removed", "a\nb", "ab"},
		{"carriage return removed", "a\rb", "ab"},
		{"tab removed", "a\tb", "ab"},
		{"remove control chars", "a\x00b", "ab"},
		{"remove DEL", "a\x7fb", "ab"},
		{"truncate long label", strings.Repeat("x", 100), strings.Repeat("x", maxMermaidLabelRunes)},
		{"mixed special chars", "\n\thello\x00world\r", "helloworld"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeMermaidLabel(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeMermaidLabel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWriteIgnored(t *testing.T) {
	t.Run("empty ignored", func(t *testing.T) {
		var sb strings.Builder
		writeIgnored(&sb, nil)
		if sb.Len() != 0 {
			t.Errorf("expected empty output, got %q", sb.String())
		}
	})

	t.Run("single ignored line", func(t *testing.T) {
		var sb strings.Builder
		writeIgnored(&sb, []string{"# ignored line"})
		got := sb.String()
		if !strings.Contains(got, "# ignored line") {
			t.Errorf("expected output to contain ignored line, got %q", got)
		}
		if !strings.HasPrefix(got, "\n") {
			t.Errorf("expected output to start with newline, got %q", got)
		}
	})

	t.Run("multiple ignored lines", func(t *testing.T) {
		var sb strings.Builder
		writeIgnored(&sb, []string{"# line1", "# line2"})
		got := sb.String()
		if !strings.Contains(got, "# line1") || !strings.Contains(got, "# line2") {
			t.Errorf("expected output to contain both lines, got %q", got)
		}
	})
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{42, "42"},
		{100, "100"},
		{999, "999"},
		{-1, "-1"},
		{-42, "-42"},
		{-100, "-100"},
	}
	for _, tt := range tests {
		t.Run("itoa_"+tt.want, func(t *testing.T) {
			got := itoa(tt.input)
			if got != tt.want {
				t.Errorf("itoa(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ============================================================================
// mermaid_ascii.go tests
// ============================================================================

func TestRenderMermaidASCII_GraphTD(t *testing.T) {
	input := `graph TD
    A[Start] --> B[Process]
    B --> C[End]`
	result := RenderMermaidASCII(input, 80)
	if result == "" {
		t.Error("expected non-empty result for graph TD")
	}
	if !strings.Contains(result, "Start") || !strings.Contains(result, "Process") || !strings.Contains(result, "End") {
		t.Errorf("result should contain node labels, got: %s", result)
	}
}

func TestRenderMermaidASCII_GraphLR(t *testing.T) {
	input := `graph LR
    A[Input] --> B[Output]`
	result := RenderMermaidASCII(input, 80)
	if result == "" {
		t.Error("expected non-empty result for graph LR")
	}
	if !strings.Contains(result, "Input") || !strings.Contains(result, "Output") {
		t.Errorf("result should contain node labels, got: %s", result)
	}
}

func TestRenderMermaidASCII_SequenceDiagram(t *testing.T) {
	input := `sequenceDiagram
    participant Alice
    participant Bob
    Alice->>Bob: Hello
    Bob-->>Alice: Hi`
	result := RenderMermaidASCII(input, 80)
	if result == "" {
		t.Error("expected non-empty result for sequenceDiagram")
	}
	if !strings.Contains(result, "Alice") || !strings.Contains(result, "Bob") {
		t.Errorf("result should contain participant names, got: %s", result)
	}
}

func TestRenderMermaidASCII_UnsupportedType(t *testing.T) {
	input := `classDiagram
    class Animal`
	result := RenderMermaidASCII(input, 80)
	if !strings.Contains(result, "不支持") {
		t.Errorf("expected unsupported type message, got: %s", result)
	}
}

func TestRenderMermaidASCII_FencedCodeBlock(t *testing.T) {
	input := "```mermaid\ngraph TD\n    A --> B\n```"
	result := RenderMermaidASCII(input, 80)
	if result == "" {
		t.Error("expected non-empty result for fenced code block")
	}
	if !strings.Contains(result, "A") || !strings.Contains(result, "B") {
		t.Errorf("result should contain node IDs, got: %s", result)
	}
}

func TestRenderMermaidASCII_EmptyInput(t *testing.T) {
	// Empty input splits into [""] which falls through to unsupported type message
	result := RenderMermaidASCII("", 80)
	if result == "" {
		t.Error("expected non-empty result for empty input (unsupported type message)")
	}
}

func TestRenderMermaidASCII_WidthLimits(t *testing.T) {
	input := `graph TD
    A --> B`

	// Width <= 0 should default to 80
	result := RenderMermaidASCII(input, 0)
	if result == "" {
		t.Error("expected non-empty result with width 0")
	}

	// Width > maxMermaidWidth should be capped
	result = RenderMermaidASCII(input, 2000)
	if result == "" {
		t.Error("expected non-empty result with capped width")
	}
}

func TestRenderMermaidASCII_InputTruncation(t *testing.T) {
	// Create input larger than maxMermaidInputBytes
	bigInput := strings.Repeat("#", maxMermaidInputBytes+100)
	result := RenderMermaidASCII(bigInput, 80)
	if result == "" {
		t.Error("expected non-empty result for truncated input")
	}
}

func TestRenderMermaidASCII_Flowchart(t *testing.T) {
	input := `flowchart TD
    A --> B`
	result := RenderMermaidASCII(input, 80)
	if result == "" {
		t.Error("expected non-empty result for flowchart")
	}
}

func TestRenderMermaidASCII_OnlyFence(t *testing.T) {
	input := "```mermaid\n```"
	result := RenderMermaidASCII(input, 80)
	// Should return empty since no content between fences
	if result != "" {
		t.Errorf("expected empty result for fences only, got: %s", result)
	}
}

// ============================================================================
// mermaid_graph_parse.go tests
// ============================================================================

func TestParseGraphEdge(t *testing.T) {
	t.Run("solid arrow -->", func(t *testing.T) {
		edge := parseGraphEdge("A --> B")
		if edge == nil {
			t.Fatal("expected non-nil edge")
		}
		if edge.from != "A" || edge.to != "B" || edge.style != "solid" {
			t.Errorf("got from=%q to=%q style=%q", edge.from, edge.to, edge.style)
		}
	})

	t.Run("dotted arrow -.->", func(t *testing.T) {
		edge := parseGraphEdge("A -.-> B")
		if edge == nil {
			t.Fatal("expected non-nil edge")
		}
		if edge.style != "dotted" {
			t.Errorf("expected dotted style, got %q", edge.style)
		}
	})

	t.Run("arrow with label", func(t *testing.T) {
		edge := parseGraphEdge("A -->|label text| B")
		if edge == nil {
			t.Fatal("expected non-nil edge")
		}
		if edge.label != "label text" {
			t.Errorf("expected label 'label text', got %q", edge.label)
		}
	})

	t.Run("standard arrow ---", func(t *testing.T) {
		edge := parseGraphEdge("A --- B")
		if edge == nil {
			t.Fatal("expected non-nil edge")
		}
		if edge.style != "solid" {
			t.Errorf("expected solid style, got %q", edge.style)
		}
	})

	t.Run("arrow ->", func(t *testing.T) {
		edge := parseGraphEdge("A -> B")
		if edge == nil {
			t.Fatal("expected non-nil edge")
		}
		if edge.style != "solid" {
			t.Errorf("expected solid style, got %q", edge.style)
		}
	})

	t.Run("arrow -", func(t *testing.T) {
		edge := parseGraphEdge("A - B")
		if edge == nil {
			t.Fatal("expected non-nil edge")
		}
		if edge.style != "solid" {
			t.Errorf("expected solid style, got %q", edge.style)
		}
	})

	t.Run("not an edge", func(t *testing.T) {
		edge := parseGraphEdge("just some text")
		if edge != nil {
			t.Errorf("expected nil edge, got %+v", edge)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		edge := parseGraphEdge("")
		if edge != nil {
			t.Errorf("expected nil edge for empty input")
		}
	})

	t.Run("node with bracket on right", func(t *testing.T) {
		edge := parseGraphEdge("A --> B[Label]")
		if edge == nil {
			t.Fatal("expected non-nil edge")
		}
		if edge.to != "B" {
			t.Errorf("expected to='B', got %q", edge.to)
		}
	})

	t.Run("dotted with label", func(t *testing.T) {
		edge := parseGraphEdge("A -.->|msg| B")
		if edge == nil {
			t.Fatal("expected non-nil edge")
		}
		if edge.label != "msg" || edge.style != "dotted" {
			t.Errorf("got label=%q style=%q", edge.label, edge.style)
		}
	})
}

func TestParseStandaloneNode(t *testing.T) {
	t.Run("rect node", func(t *testing.T) {
		id := parseStandaloneNode("A[Label]")
		if id != "A" {
			t.Errorf("expected 'A', got %q", id)
		}
	})

	t.Run("round node", func(t *testing.T) {
		id := parseStandaloneNode("B(Rounded)")
		if id != "B" {
			t.Errorf("expected 'B', got %q", id)
		}
	})

	t.Run("diamond node", func(t *testing.T) {
		id := parseStandaloneNode("C{Diamond}")
		if id != "C" {
			t.Errorf("expected 'C', got %q", id)
		}
	})

	t.Run("not a node", func(t *testing.T) {
		id := parseStandaloneNode("just_text")
		if id != "" {
			t.Errorf("expected empty, got %q", id)
		}
	})

	t.Run("empty", func(t *testing.T) {
		id := parseStandaloneNode("")
		if id != "" {
			t.Errorf("expected empty, got %q", id)
		}
	})
}

func TestParseNodeSpec(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantID    string
		wantLabel string
		wantShape string
	}{
		{"plain id", "A", "A", "", ""},
		{"rect", "A[Rectangle]", "A", "Rectangle", "rect"},
		{"rect double bracket", "A[[Double]]", "A", "Double", "rect"},
		{"round", "A(Rounded)", "A", "Rounded", "round"},
		{"circle", "A((Circle))", "A", "Circle", "circle"},
		{"diamond", "A{Diamond}", "A", "Diamond", "diamond"},
		{"empty", "", "", "", ""},
		{"id with underscore", "my_node[Label]", "my_node", "Label", "rect"},
		{"id with hyphen", "my-node[Label]", "my-node", "Label", "rect"},
		{"id with number", "A1[Label]", "A1", "Label", "rect"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, label, shape := parseNodeSpec(tt.input)
			if id != tt.wantID || label != tt.wantLabel || shape != tt.wantShape {
				t.Errorf("parseNodeSpec(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tt.input, id, label, shape, tt.wantID, tt.wantLabel, tt.wantShape)
			}
		})
	}
}

func TestExtractNodeID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"A", "A"},
		{"A[Label]", "A"},
		{"my_node_1", "my_node_1"},
		{"node-id[Text]", "node-id"},
		{"", ""},
		{"123", "123"},
		{"  A  ", "A"},
	}
	for _, tt := range tests {
		t.Run("extract_"+tt.input, func(t *testing.T) {
			got := extractNodeID(tt.input)
			if got != tt.want {
				t.Errorf("extractNodeID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractBracket(t *testing.T) {
	t.Run("simple brackets", func(t *testing.T) {
		inner, ok := extractBracket("[hello]", '[', ']')
		if !ok || inner != "hello" {
			t.Errorf("expected ('hello', true), got (%q, %v)", inner, ok)
		}
	})

	t.Run("nested brackets", func(t *testing.T) {
		inner, ok := extractBracket("[a[b]c]", '[', ']')
		if !ok || inner != "a[b]c" {
			t.Errorf("expected ('a[b]c', true), got (%q, %v)", inner, ok)
		}
	})

	t.Run("mismatched brackets", func(t *testing.T) {
		_, ok := extractBracket("[hello", '[', ']')
		if ok {
			t.Error("expected false for mismatched brackets")
		}
	})

	t.Run("not starting with bracket", func(t *testing.T) {
		_, ok := extractBracket("hello]", '[', ']')
		if ok {
			t.Error("expected false for text not starting with bracket")
		}
	})

	t.Run("empty brackets", func(t *testing.T) {
		inner, ok := extractBracket("[]", '[', ']')
		if !ok || inner != "" {
			t.Errorf("expected ('', true), got (%q, %v)", inner, ok)
		}
	})

	t.Run("parentheses", func(t *testing.T) {
		inner, ok := extractBracket("(hello)", '(', ')')
		if !ok || inner != "hello" {
			t.Errorf("expected ('hello', true), got (%q, %v)", inner, ok)
		}
	})

	t.Run("braces", func(t *testing.T) {
		inner, ok := extractBracket("{hello}", '{', '}')
		if !ok || inner != "hello" {
			t.Errorf("expected ('hello', true), got (%q, %v)", inner, ok)
		}
	})
}

func TestRegisterNodeFromText(t *testing.T) {
	t.Run("new node", func(t *testing.T) {
		nodeMap := make(map[string]*mermaidNode)
		var order []string
		registerNodeFromText(nodeMap, &order, "A[Hello]")
		if len(order) != 1 || order[0] != "A" {
			t.Errorf("expected order [A], got %v", order)
		}
		if nodeMap["A"].label != "Hello" || nodeMap["A"].shape != "rect" {
			t.Errorf("unexpected node: %+v", nodeMap["A"])
		}
	})

	t.Run("update existing node with label", func(t *testing.T) {
		nodeMap := map[string]*mermaidNode{"A": {id: "A", label: "Old", shape: "rect"}}
		var order []string
		registerNodeFromText(nodeMap, &order, "A[New]")
		if nodeMap["A"].label != "New" {
			t.Errorf("expected label 'New', got %q", nodeMap["A"].label)
		}
		if len(order) != 0 {
			t.Errorf("expected no new order entry, got %v", order)
		}
	})

	t.Run("invalid text", func(t *testing.T) {
		nodeMap := make(map[string]*mermaidNode)
		var order []string
		registerNodeFromText(nodeMap, &order, "")
		if len(order) != 0 || len(nodeMap) != 0 {
			t.Error("expected no nodes registered for empty text")
		}
	})
}

func TestFirstSpaceBeforeBracket(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"A B", 1},
		{"A[B] C", 4},
		{"A[B] C[D]", 4},
		{"no_space", -1},
		{"A[B]", -1},
		{"", -1},
		{"A[B] C[D] E", 4},
	}
	for _, tt := range tests {
		t.Run("firstSpace_"+tt.input, func(t *testing.T) {
			got := firstSpaceBeforeBracket(tt.input)
			if got != tt.want {
				t.Errorf("firstSpaceBeforeBracket(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// ============================================================================
// mermaid_graph_render.go tests
// ============================================================================

func TestRenderGraphTD(t *testing.T) {
	nodeMap := map[string]*mermaidNode{
		"A": {id: "A", label: "Start", shape: "rect"},
		"B": {id: "B", label: "Process", shape: "rect"},
	}
	order := []string{"A", "B"}
	edges := []*mermaidEdge{
		{from: "A", to: "B", label: "go", style: "solid"},
	}
	result := renderGraphTD(nodeMap, order, edges, 80, nil)
	if !strings.Contains(result, "Start") || !strings.Contains(result, "Process") {
		t.Errorf("expected nodes in output, got: %s", result)
	}
}

func TestRenderGraphTD_EmptyNodes(t *testing.T) {
	result := renderGraphTD(nil, nil, nil, 80, nil)
	if !strings.Contains(result, "无可渲染节点") {
		t.Errorf("expected empty nodes message, got: %s", result)
	}
}

func TestRenderGraphTD_WithIgnored(t *testing.T) {
	nodeMap := map[string]*mermaidNode{
		"A": {id: "A", label: "X", shape: "rect"},
	}
	order := []string{"A"}
	ignored := []string{"# test ignore"}
	result := renderGraphTD(nodeMap, order, nil, 80, ignored)
	if !strings.Contains(result, "test ignore") {
		t.Errorf("expected ignored line in output, got: %s", result)
	}
}

func TestRenderGraphTD_MultipleEdges(t *testing.T) {
	nodeMap := map[string]*mermaidNode{
		"A": {id: "A", label: "A", shape: "rect"},
		"B": {id: "B", label: "B", shape: "rect"},
		"C": {id: "C", label: "C", shape: "rect"},
	}
	order := []string{"A", "B", "C"}
	edges := []*mermaidEdge{
		{from: "A", to: "B", style: "solid"},
		{from: "A", to: "C", style: "solid"},
	}
	result := renderGraphTD(nodeMap, order, edges, 80, nil)
	if !strings.Contains(result, "A") {
		t.Errorf("expected nodes in output, got: %s", result)
	}
}

func TestRenderGraphLR(t *testing.T) {
	nodeMap := map[string]*mermaidNode{
		"A": {id: "A", label: "Input", shape: "rect"},
		"B": {id: "B", label: "Output", shape: "rect"},
	}
	order := []string{"A", "B"}
	edges := []*mermaidEdge{
		{from: "A", to: "B", label: "flow", style: "solid"},
	}
	result := renderGraphLR(nodeMap, order, edges, 80, nil)
	if !strings.Contains(result, "Input") || !strings.Contains(result, "Output") {
		t.Errorf("expected nodes in output, got: %s", result)
	}
	if !strings.Contains(result, "flow") {
		t.Errorf("expected edge label 'flow' in output, got: %s", result)
	}
}

func TestRenderGraphLR_EmptyNodes(t *testing.T) {
	result := renderGraphLR(nil, nil, nil, 80, nil)
	if !strings.Contains(result, "无可渲染节点") {
		t.Errorf("expected empty nodes message, got: %s", result)
	}
}

func TestRenderGraphLR_WithIgnored(t *testing.T) {
	nodeMap := map[string]*mermaidNode{
		"A": {id: "A", label: "X", shape: "rect"},
	}
	order := []string{"A"}
	ignored := []string{"# test ignore"}
	result := renderGraphLR(nodeMap, order, nil, 80, ignored)
	if !strings.Contains(result, "test ignore") {
		t.Errorf("expected ignored line in output, got: %s", result)
	}
}

func TestRenderNodeBox(t *testing.T) {
	t.Run("rect shape", func(t *testing.T) {
		node := &mermaidNode{id: "A", label: "Test", shape: "rect"}
		box := renderNodeBox(node)
		if len(box) != 3 {
			t.Errorf("expected 3 lines for rect, got %d", len(box))
		}
		if !strings.Contains(box[1], "Test") {
			t.Errorf("expected label in box, got: %v", box)
		}
	})

	t.Run("round shape", func(t *testing.T) {
		node := &mermaidNode{id: "A", label: "Test", shape: "round"}
		box := renderNodeBox(node)
		if len(box) != 3 {
			t.Errorf("expected 3 lines for round, got %d", len(box))
		}
	})

	t.Run("circle shape", func(t *testing.T) {
		node := &mermaidNode{id: "A", label: "Test", shape: "circle"}
		box := renderNodeBox(node)
		if len(box) != 3 {
			t.Errorf("expected 3 lines for circle, got %d", len(box))
		}
	})

	t.Run("diamond shape", func(t *testing.T) {
		node := &mermaidNode{id: "A", label: "Test", shape: "diamond"}
		box := renderNodeBox(node)
		if len(box) != 5 {
			t.Errorf("expected 5 lines for diamond, got %d", len(box))
		}
	})

	t.Run("empty label uses id", func(t *testing.T) {
		node := &mermaidNode{id: "MyID", label: "", shape: "rect"}
		box := renderNodeBox(node)
		if !strings.Contains(box[1], "MyID") {
			t.Errorf("expected id as fallback label, got: %v", box)
		}
	})
}

func TestCenterText(t *testing.T) {
	tests := []struct {
		text   string
		width  int
		expect string
	}{
		{"a", 3, " a "},
		{"ab", 4, " ab "},
		{"abc", 5, " abc "},
		{"abcd", 4, "abcd"},
		{"a", 5, "  a  "},
	}
	for _, tt := range tests {
		t.Run("center_"+tt.text, func(t *testing.T) {
			got := centerText(tt.text, tt.width)
			if got != tt.expect {
				t.Errorf("centerText(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.expect)
			}
		})
	}
}

func TestBoxWidth(t *testing.T) {
	t.Run("normal box", func(t *testing.T) {
		box := []string{"abc", "abcdef", "ab"}
		w := boxWidth(box)
		if w != 6 {
			t.Errorf("expected width 6, got %d", w)
		}
	})

	t.Run("empty box", func(t *testing.T) {
		w := boxWidth(nil)
		if w != 0 {
			t.Errorf("expected width 0, got %d", w)
		}
	})
}

func TestCenterBox(t *testing.T) {
	box := []string{"a", "abc"}
	result := centerBox(box, 5)
	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}
	// centerBox only left-pads (not right-pads)
	// "a" with maxW=5: pad=(5-1)/2=2 → "  a"
	if result[0] != "  a" {
		t.Errorf("expected left-padded 'a', got %q", result[0])
	}
	// "abc" with maxW=5: pad=(5-3)/2=1 → " abc"
	if result[1] != " abc" {
		t.Errorf("expected left-padded 'abc', got %q", result[1])
	}
}

func TestPadLabel(t *testing.T) {
	got := padLabel("test")
	if got != " test " {
		t.Errorf("expected ' test ', got %q", got)
	}
}

// ============================================================================
// mermaid_sequence.go tests
// ============================================================================

func TestParseSeqMessage(t *testing.T) {
	t.Run("->> solid arrow", func(t *testing.T) {
		m := parseSeqMessage("Alice->>Bob: Hello")
		if m == nil {
			t.Fatal("expected non-nil message")
		}
		if m.from != "Alice" || m.to != "Bob" || m.text != "Hello" || m.dashed {
			t.Errorf("got from=%q to=%q text=%q dashed=%v", m.from, m.to, m.text, m.dashed)
		}
	})

	t.Run("-->> dashed arrow", func(t *testing.T) {
		m := parseSeqMessage("Alice-->>Bob: Reply")
		if m == nil {
			t.Fatal("expected non-nil message")
		}
		if !m.dashed {
			t.Error("expected dashed=true")
		}
	})

	t.Run("--> solid arrow", func(t *testing.T) {
		m := parseSeqMessage("Alice-->Bob: Msg")
		if m == nil {
			t.Fatal("expected non-nil message")
		}
		if !m.dashed {
			t.Error("expected dashed=true for -->")
		}
	})

	t.Run("-> basic arrow", func(t *testing.T) {
		m := parseSeqMessage("Alice->Bob: Hi")
		if m == nil {
			t.Fatal("expected non-nil message")
		}
		if m.dashed {
			t.Error("expected dashed=false for ->")
		}
	})

	t.Run("no colon", func(t *testing.T) {
		m := parseSeqMessage("Alice->>Bob")
		if m != nil {
			t.Errorf("expected nil for missing colon, got %+v", m)
		}
	})

	t.Run("empty left", func(t *testing.T) {
		m := parseSeqMessage(": message")
		if m != nil {
			t.Errorf("expected nil for empty left, got %+v", m)
		}
	})

	t.Run("no arrow", func(t *testing.T) {
		m := parseSeqMessage("Alice: hello")
		if m != nil {
			t.Errorf("expected nil for no arrow, got %+v", m)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		m := parseSeqMessage("")
		if m != nil {
			t.Errorf("expected nil for empty input, got %+v", m)
		}
	})
}

func TestSplitActors(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"A", []string{"A"}},
		{"A,B", []string{"A", "B"}},
		{"A, B, C", []string{"A", "B", "C"}},
		{"", []string{}},
		{"  A  ,  B  ", []string{"A", "B"}},
	}
	for _, tt := range tests {
		t.Run("split_"+tt.input, func(t *testing.T) {
			got := splitActors(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d actors, got %d: %v", len(tt.want), len(got), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("actor[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRenderSeqArrow(t *testing.T) {
	participants := []string{"Alice", "Bob"}
	colW := 10

	result := renderSeqArrow(participants, colW, 0, 1, "Hello", false)
	if result == "" {
		t.Error("expected non-empty result")
	}
	if !strings.Contains(result, "Hello") {
		t.Errorf("expected text in arrow, got: %s", result)
	}

	// dashed arrow
	resultDashed := renderSeqArrow(participants, colW, 0, 1, "Hi", true)
	if resultDashed == "" {
		t.Error("expected non-empty dashed result")
	}

	// right-to-left arrow
	resultRL := renderSeqArrow(participants, colW, 1, 0, "Back", false)
	if resultRL == "" {
		t.Error("expected non-empty result for right-to-left")
	}
}

func TestRenderSeqNote(t *testing.T) {
	participants := []string{"Alice", "Bob"}
	colW := 10

	result := renderSeqNote(participants, colW, "Alice", "Note text")
	if result == "" {
		t.Error("expected non-empty result")
	}
	// Note text may be truncated due to column width constraints
	if !strings.Contains(result, "Note") {
		t.Errorf("expected note text prefix in output, got: %s", result)
	}

	// Test with unknown actor
	result2 := renderSeqNote(participants, colW, "Unknown", "text")
	if !strings.Contains(result2, "# Note over") {
		t.Errorf("expected fallback text for unknown actor, got: %s", result2)
	}
}

// ============================================================================
// markdown.go tests
// ============================================================================

func TestRenderMarkdown_Headings(t *testing.T) {
	input := "# H1\n## H2\n### H3\n#### H4\n##### H5\n###### H6"
	result := RenderMarkdown(input, 80)
	for _, h := range []string{"H1", "H2", "H3", "H4", "H5", "H6"} {
		if !strings.Contains(result, h) {
			t.Errorf("expected heading %q in output", h)
		}
	}
}

func TestRenderMarkdown_CodeBlocks(t *testing.T) {
	input := "```go\nfunc main() {}\n```"
	result := RenderMarkdown(input, 80)
	if !strings.Contains(result, "func main()") {
		t.Errorf("expected code in output, got: %s", result)
	}
}

func TestRenderMarkdown_Table(t *testing.T) {
	input := "| Col1 | Col2 |\n|------|------|\n| A    | B    |"
	result := RenderMarkdown(input, 80)
	if !strings.Contains(result, "Col1") || !strings.Contains(result, "A") {
		t.Errorf("expected table content in output, got: %s", result)
	}
}

func TestRenderMarkdown_Lists(t *testing.T) {
	input := "- item1\n- item2\n* item3\n1. ordered1\n2. ordered2"
	result := RenderMarkdown(input, 80)
	if !strings.Contains(result, "item1") || !strings.Contains(result, "ordered1") {
		t.Errorf("expected list items in output, got: %s", result)
	}
}

func TestRenderMarkdown_Links(t *testing.T) {
	input := "[example](https://example.com)"
	result := RenderMarkdown(input, 80)
	if !strings.Contains(result, "example") {
		t.Errorf("expected link text in output, got: %s", result)
	}
}

func TestRenderMarkdown_BoldItalic(t *testing.T) {
	input := "**bold** and *italic*"
	result := RenderMarkdown(input, 80)
	if !strings.Contains(result, "bold") || !strings.Contains(result, "italic") {
		t.Errorf("expected bold/italic in output, got: %s", result)
	}
}

func TestRenderMarkdown_InlineCode(t *testing.T) {
	input := "use `fmt.Println()` for output"
	result := RenderMarkdown(input, 80)
	if !strings.Contains(result, "fmt.Println()") {
		t.Errorf("expected inline code in output, got: %s", result)
	}
}

func TestRenderMarkdown_Blockquote(t *testing.T) {
	input := "> quoted text"
	result := RenderMarkdown(input, 80)
	if !strings.Contains(result, "quoted text") {
		t.Errorf("expected quoted text in output, got: %s", result)
	}
}

func TestRenderMarkdown_HorizontalRule(t *testing.T) {
	input := "---"
	result := RenderMarkdown(input, 80)
	if result == "" {
		t.Error("expected non-empty result for horizontal rule")
	}
}

func TestRenderMarkdown_WidthLimits(t *testing.T) {
	// Width <= 0 should default to 80
	result := RenderMarkdown("hello", 0)
	if result == "" {
		t.Error("expected non-empty result with width 0")
	}

	// Width > maxMarkdownWidth should be capped
	result = RenderMarkdown("hello", 2000)
	if result == "" {
		t.Error("expected non-empty result with capped width")
	}
}

func TestRenderMarkdown_Empty(t *testing.T) {
	result := RenderMarkdown("", 80)
	if result != "\n" {
		t.Errorf("expected just newline for empty input, got: %q", result)
	}
}

func TestRenderMarkdown_UnclosedCodeBlock(t *testing.T) {
	input := "```go\nfunc main() {}"
	result := RenderMarkdown(input, 80)
	if !strings.Contains(result, "func main()") {
		t.Errorf("expected code in output for unclosed block, got: %s", result)
	}
}

func TestParseHeading(t *testing.T) {
	tests := []struct {
		input      string
		wantLevel  int
		wantContent string
		wantOk     bool
	}{
		{"# H1", 1, "H1", true},
		{"## H2", 2, "H2", true},
		{"### H3", 3, "H3", true},
		{"#### H4", 4, "H4", true},
		{"##### H5", 5, "H5", true},
		{"###### H6", 6, "H6", true},
		{"####### Not a heading", 0, "", false},
		{"#NoSpace", 0, "", false},
		{"not a heading", 0, "", false},
		{"# ", 0, "", false},
		{"", 0, "", false},
		{"  # H1 with spaces", 1, "H1 with spaces", true},
	}
	for _, tt := range tests {
		t.Run("heading_"+tt.input, func(t *testing.T) {
			level, content, ok := parseHeading(tt.input)
			if level != tt.wantLevel || content != tt.wantContent || ok != tt.wantOk {
				t.Errorf("parseHeading(%q) = (%d, %q, %v), want (%d, %q, %v)",
					tt.input, level, content, ok, tt.wantLevel, tt.wantContent, tt.wantOk)
			}
		})
	}
}

func TestIsHorizontalRule(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"---", true},
		{"***", true},
		{"___", true},
		{"----", true},
		{"*****", true},
		{"- - -", true},
		{"--", false},
		{"abc", false},
		{"", false},
		{"- -", false},
	}
	for _, tt := range tests {
		t.Run("hr_"+tt.input, func(t *testing.T) {
			got := isHorizontalRule(tt.input)
			if got != tt.want {
				t.Errorf("isHorizontalRule(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsListLine(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"- item", true},
		{"* item", true},
		{"+ item", true},
		{"1. item", true},
		{"10. item", true},
		{"  - indented", true},
		{"not a list", false},
		{"-no space", false},
		{"1.no space", false},
		{"", false},
		{"a. item", false},
	}
	for _, tt := range tests {
		t.Run("list_"+tt.input, func(t *testing.T) {
			got := isListLine(tt.input)
			if got != tt.want {
				t.Errorf("isListLine(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsTableRow(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"| A | B |", true},
		{"| A |", true},
		{"|------|------|", true},
		{"not a table", false},
		{"", false},
		{"A | B", false},
	}
	for _, tt := range tests {
		t.Run("table_"+tt.input, func(t *testing.T) {
			got := isTableRow(tt.input)
			if got != tt.want {
				t.Errorf("isTableRow(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitTableRow(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"| A | B |", []string{"A", "B"}},
		{"| A |", []string{"A"}},
		{"| A | B | C |", []string{"A", "B", "C"}},
		{"|   A   |   B   |", []string{"A", "B"}},
		{"A | B", []string{"A", "B"}},
	}
	for _, tt := range tests {
		t.Run("split_"+tt.input, func(t *testing.T) {
			got := splitTableRow(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d cells, got %d: %v", len(tt.want), len(got), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("cell[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestIsTableSeparator(t *testing.T) {
	tests := []struct {
		cells []string
		want  bool
	}{
		{[]string{"---", "---"}, true},
		{[]string{":---", "---:"}, true},
		{[]string{"---", "abc"}, false},
		{[]string{}, false},
		{[]string{"", "---"}, true},
	}
	for _, tt := range tests {
		t.Run("sep", func(t *testing.T) {
			got := isTableSeparator(tt.cells)
			if got != tt.want {
				t.Errorf("isTableSeparator(%v) = %v, want %v", tt.cells, got, tt.want)
			}
		})
	}
}

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 3, "hel"},
		{"hello", 10, "hello"},
		{"hello", 0, ""},
		{"hello", -1, ""},
		{"", 5, ""},
		{"你好世界", 2, "你好"},
		{"a", 1, "a"},
	}
	for _, tt := range tests {
		t.Run("truncate_"+tt.input, func(t *testing.T) {
			got := truncateRunes(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncateRunes(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestLimitLineRunes(t *testing.T) {
	t.Run("within limit", func(t *testing.T) {
		got := limitLineRunes("hello", 10)
		if got != "hello" {
			t.Errorf("expected 'hello', got %q", got)
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		got := limitLineRunes("hello world", 5)
		if got != "hello" {
			t.Errorf("expected 'hello', got %q", got)
		}
	})

	t.Run("exact limit", func(t *testing.T) {
		got := limitLineRunes("hello", 5)
		if got != "hello" {
			t.Errorf("expected 'hello', got %q", got)
		}
	})
}

func TestSanitizeLabel(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello"},
		{"hello\x00world", 10, "helloworld"},
		{"hello\x7fworld", 10, "helloworld"},
		{"", 10, ""},
	}
	for _, tt := range tests {
		t.Run("sanitize_"+tt.input, func(t *testing.T) {
			got := sanitizeLabel(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("sanitizeLabel(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}

	t.Run("default max", func(t *testing.T) {
		got := sanitizeLabel("hello", 0)
		if got != "hello" {
			t.Errorf("expected 'hello' with default max, got %q", got)
		}
	})
}

func TestRenderInline(t *testing.T) {
	t.Run("plain text", func(t *testing.T) {
		got := renderInline("hello")
		if got != "hello" {
			t.Errorf("expected 'hello', got %q", got)
		}
	})

	t.Run("mixed formatting", func(t *testing.T) {
		got := renderInline("**bold** and *italic* and `code`")
		if !strings.Contains(got, "bold") && !strings.Contains(got, "italic") && !strings.Contains(got, "code") {
			t.Errorf("expected formatted output, got: %s", got)
		}
	})
}

func TestRenderInlineCode(t *testing.T) {
	tests := []struct {
		input string
		check string
	}{
		{"`code`", "code"},
		{"text `code` more", "code"},
		{"no backticks", "no backticks"},
		{"", ""},
		{"`unclosed", "`unclosed"},
	}
	for _, tt := range tests {
		t.Run("inlinecode", func(t *testing.T) {
			got := renderInlineCode(tt.input)
			if !strings.Contains(got, tt.check) {
				t.Errorf("renderInlineCode(%q) = %q, expected to contain %q", tt.input, got, tt.check)
			}
		})
	}
}

func TestRenderLinks(t *testing.T) {
	t.Run("link", func(t *testing.T) {
		got := renderLinks("[text](https://example.com)")
		if !strings.Contains(got, "text") {
			t.Errorf("expected link text in output, got: %s", got)
		}
	})

	t.Run("no link", func(t *testing.T) {
		got := renderLinks("plain text")
		if got != "plain text" {
			t.Errorf("expected 'plain text', got %q", got)
		}
	})

	t.Run("empty", func(t *testing.T) {
		got := renderLinks("")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestRenderBold(t *testing.T) {
	t.Run("bold", func(t *testing.T) {
		got := renderBold("**bold**")
		if !strings.Contains(got, "bold") {
			t.Errorf("expected bold text in output, got: %s", got)
		}
	})

	t.Run("no bold", func(t *testing.T) {
		got := renderBold("plain")
		if got != "plain" {
			t.Errorf("expected 'plain', got %q", got)
		}
	})

	t.Run("empty content", func(t *testing.T) {
		got := renderBold("**")
		if got != "**" {
			t.Errorf("expected '**', got %q", got)
		}
	})
}

func TestRenderItalic(t *testing.T) {
	t.Run("italic", func(t *testing.T) {
		got := renderItalic("*italic*")
		if !strings.Contains(got, "italic") {
			t.Errorf("expected italic text in output, got: %s", got)
		}
	})

	t.Run("no italic", func(t *testing.T) {
		got := renderItalic("plain")
		if got != "plain" {
			t.Errorf("expected 'plain', got %q", got)
		}
	})

	t.Run("does not consume bold", func(t *testing.T) {
		got := renderItalic("**not italic**")
		if !strings.Contains(got, "**") {
			t.Errorf("expected ** preserved, got: %s", got)
		}
	})
}

func TestStripTerminalControl(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check string
	}{
		{"plain text", "hello", "hello"},
		{"with newline", "hello\nworld", "hello\nworld"},
		{"with tab", "hello\tworld", "hello\tworld"},
		{"null byte removed", "he\x00llo", "hello"},
		{"DEL removed", "he\x7fllo", "hello"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripTerminalControl(tt.input)
			if got != tt.check {
				t.Errorf("stripTerminalControl(%q) = %q, want %q", tt.input, got, tt.check)
			}
		})
	}

	t.Run("ANSI CSI removed", func(t *testing.T) {
		got := stripTerminalControl("\x1b[31mred\x1b[0m")
		if got != "red" {
			t.Errorf("expected 'red', got %q", got)
		}
	})

	t.Run("ANSI OSC removed", func(t *testing.T) {
		got := stripTerminalControl("\x1b]0;title\x07text")
		if got != "text" {
			t.Errorf("expected 'text', got %q", got)
		}
	})

	t.Run("ANSI two-char removed", func(t *testing.T) {
		got := stripTerminalControl("\x1bctext")
		if got != "text" {
			t.Errorf("expected 'text', got %q", got)
		}
	})
}

func TestAnsiEscSeqLen(t *testing.T) {
	t.Run("not starting with ESC", func(t *testing.T) {
		got := ansiEscSeqLen("hello")
		if got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
	})

	t.Run("CSI sequence", func(t *testing.T) {
		got := ansiEscSeqLen("\x1b[31m")
		if got != 5 {
			t.Errorf("expected 5, got %d", got)
		}
	})

	t.Run("two-char sequence", func(t *testing.T) {
		got := ansiEscSeqLen("\x1bc")
		if got != 2 {
			t.Errorf("expected 2, got %d", got)
		}
	})

	t.Run("unterminated CSI", func(t *testing.T) {
		got := ansiEscSeqLen("\x1b[31")
		if got != 4 {
			t.Errorf("expected 4, got %d", got)
		}
	})

	t.Run("OSC with BEL", func(t *testing.T) {
		got := ansiEscSeqLen("\x1b]0;title\x07")
		if got != 10 {
			t.Errorf("expected 10, got %d", got)
		}
	})

	t.Run("OSC with ST", func(t *testing.T) {
		got := ansiEscSeqLen("\x1b]0;title\x1b\\")
		if got != 11 {
			t.Errorf("expected 11, got %d", got)
		}
	})

	t.Run("DCS with ST", func(t *testing.T) {
		got := ansiEscSeqLen("\x1bPdata\x1b\\")
		if got != 8 {
			t.Errorf("expected 8, got %d", got)
		}
	})
}

// ============================================================================
// Additional model tests
// ============================================================================

func TestModel_StepStreamMsg(t *testing.T) {
	m := NewModel("test", "test.yaml", 2)

	// First stream chunk
	msg1 := StepStreamMsg{
		Index: 0,
		Name:  "stream_step",
		Chunk: "chunk1",
	}
	newM, _ := m.Update(msg1)
	model := newM.(*Model)

	if model.steps[0].Name != "stream_step" {
		t.Errorf("expected step name 'stream_step', got '%s'", model.steps[0].Name)
	}
	if !model.steps[0].Streaming {
		t.Error("expected streaming=true")
	}
	if model.steps[0].StreamOutput != "chunk1" {
		t.Errorf("expected StreamOutput 'chunk1', got '%s'", model.steps[0].StreamOutput)
	}

	// Second stream chunk
	msg2 := StepStreamMsg{
		Index: 0,
		Name:  "stream_step",
		Chunk: "chunk2",
	}
	newM2, _ := model.Update(msg2)
	model2 := newM2.(*Model)

	if model2.steps[0].StreamOutput != "chunk1chunk2" {
		t.Errorf("expected accumulated StreamOutput 'chunk1chunk2', got '%s'", model2.steps[0].StreamOutput)
	}
}

func TestModel_KeyMsg(t *testing.T) {
	t.Run("q key quits", func(t *testing.T) {
		m := NewModel("test", "test.yaml", 2)
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		if cmd == nil {
			t.Error("expected quit command for 'q' key")
		}
	})

	t.Run("ctrl+c quits", func(t *testing.T) {
		m := NewModel("test", "test.yaml", 2)
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		if cmd == nil {
			t.Error("expected quit command for ctrl+c")
		}
	})

	t.Run("any key quits when finished", func(t *testing.T) {
		m := NewModel("test", "test.yaml", 2)
		m.finished = true
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd == nil {
			t.Error("expected quit command when finished")
		}
	})

	t.Run("other key does not quit when running", func(t *testing.T) {
		m := NewModel("test", "test.yaml", 2)
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil {
			t.Error("expected nil command for non-quit key when running")
		}
	})
}

func TestModel_View_EmptyOutput(t *testing.T) {
	m := NewModel("empty workflow", "empty.yaml", 0)
	m.finished = false

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view for empty steps")
	}
	if !strings.Contains(view, "empty workflow") {
		t.Error("view should contain workflow name")
	}
}

func TestModel_View_Streaming(t *testing.T) {
	m := NewModel("stream", "stream.yaml", 2)
	m.steps[0] = Step{
		Name:      "fetch",
		Status:    StatusRunning,
		Streaming: true,
		StreamOutput: "streaming data",
	}

	view := m.View()
	if !strings.Contains(view, "streaming data") {
		t.Error("view should contain streaming output")
	}
	if !strings.Contains(view, "▊") {
		t.Error("view should contain streaming cursor")
	}
}

func TestModel_View_ErrorState(t *testing.T) {
	m := NewModel("error", "error.yaml", 1)
	m.steps[0] = Step{
		Name:   "bad_step",
		Status: StatusError,
		Error:  "something went wrong",
	}

	view := m.View()
	if !strings.Contains(view, "something went wrong") {
		t.Error("view should contain error message")
	}
}

func TestModel_View_FinishedSuccess(t *testing.T) {
	m := NewModel("success", "success.yaml", 1)
	m.steps[0] = Step{
		Name:   "good_step",
		Status: StatusDone,
		Output: "all good",
	}
	m.finished = true
	m.success = true

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
	if !strings.Contains(view, "all good") {
		t.Error("view should contain output")
	}
}

func TestModel_View_FinishedFailure(t *testing.T) {
	m := NewModel("fail", "fail.yaml", 1)
	m.finished = true
	m.success = false

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestModel_StepStreamMsg_OutOfBounds(t *testing.T) {
	m := NewModel("test", "test.yaml", 1)

	msg := StepStreamMsg{
		Index: 99,
		Name:  "bad",
		Chunk: "data",
	}
	newM, _ := m.Update(msg)
	model := newM.(*Model)

	// Should not panic, no effect
	if len(model.steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(model.steps))
	}
}

func TestModel_StepStartMsg_OutOfBounds(t *testing.T) {
	m := NewModel("test", "test.yaml", 1)

	msg := StepStartMsg{
		Index: -1,
		Name:  "bad",
	}
	newM, _ := m.Update(msg)
	model := newM.(*Model)

	// Should not panic
	if model.steps[0].Status != StatusPending {
		t.Errorf("expected pending status, got %d", model.steps[0].Status)
	}
}

func TestModel_StepEndMsg_OutOfBounds(t *testing.T) {
	m := NewModel("test", "test.yaml", 1)

	msg := StepEndMsg{
		Index: 99,
		Name:  "bad",
	}
	newM, _ := m.Update(msg)
	model := newM.(*Model)

	// Should not panic
	if len(model.steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(model.steps))
	}
}

func TestModel_View_StreamingFirstStep(t *testing.T) {
	// Test that streaming output from the first step is shown
	m := NewModel("stream", "stream.yaml", 2)
	m.steps[0] = Step{
		Name:      "first",
		Status:    StatusRunning,
		Streaming: true,
		StreamOutput: "first output",
	}

	view := m.View()
	if !strings.Contains(view, "first output") {
		t.Error("view should contain streaming output from first step")
	}
}

func TestModel_View_OutputTruncation(t *testing.T) {
	// Output over 1000 chars should be truncated
	m := NewModel("trunc", "trunc.yaml", 1)
	longOutput := strings.Repeat("x", 2000)
	m.steps[0] = Step{
		Name:   "step",
		Status: StatusDone,
		Output: longOutput,
	}

	view := m.View()
	// Should not contain all 2000 chars
	if strings.Contains(view, longOutput) {
		t.Error("view should truncate output over 1000 chars")
	}
}

func TestModel_UnknownMessage(t *testing.T) {
	m := NewModel("test", "test.yaml", 2)
	// Send an unknown message type
	newM, cmd := m.Update("unknown")
	if cmd != nil {
		t.Error("expected nil cmd for unknown message")
	}
	if newM != m {
		t.Error("model should be unchanged for unknown message")
	}
}
