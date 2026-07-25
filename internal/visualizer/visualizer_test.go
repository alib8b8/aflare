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

package visualizer

import (
	"strings"
	"testing"
)

var simpleWorkflow = `
name: simple-test
description: A simple workflow
steps:
  - node: fetch_url
    params:
      url: "https://example.com"
  - node: openai
    params:
      model: "gpt-4"
      prompt: "Summarize"
  - node: file_write
    params:
      path: "output.txt"
`

var parallelWorkflow = `
name: parallel-test
description: A parallel workflow
steps:
  - parallel:
      - node: fetch_url
        params:
          url: "https://a.com"
      - node: fetch_url
        params:
          url: "https://b.com"
  - node: combine
    params:
      separator: "\n"
  - node: file_write
    params:
      path: "result.txt"
`

var ifWorkflow = `
name: if-test
description: A conditional workflow
steps:
  - node: execute
    params:
      command: "echo hello"
  - if:
      condition: "output != ''"
      then:
        - node: openai
          params:
            prompt: "Analyze"
      else:
        - node: notify
          params:
            message: "No output"
  - node: file_write
    params:
      path: "final.txt"
`

var emptyWorkflow = `
name: empty-test
description: An empty workflow
steps: []
`

func TestGenerateMermaid(t *testing.T) {
	mermaid := GenerateMermaid(simpleWorkflow)
	if !strings.Contains(mermaid, "flowchart TD") {
		t.Error("expected Mermaid to contain 'flowchart TD'")
	}
	if !strings.Contains(mermaid, "start([Start])") {
		t.Error("expected Mermaid to contain start node")
	}
	if !strings.Contains(mermaid, "fetch_url") {
		t.Error("expected Mermaid to contain fetch_url node")
	}
	if !strings.Contains(mermaid, "openai") {
		t.Error("expected Mermaid to contain openai node")
	}

	// Test LR direction
	mermaidLR := GenerateMermaidWithDirection(simpleWorkflow, "LR")
	if !strings.Contains(mermaidLR, "flowchart LR") {
		t.Error("expected Mermaid LR to contain 'flowchart LR'")
	}
}

func TestGenerateMermaidParallel(t *testing.T) {
	mermaid := GenerateMermaid(parallelWorkflow)
	if !strings.Contains(mermaid, "subgraph") {
		t.Error("expected Mermaid parallel to contain subgraph")
	}
	if !strings.Contains(mermaid, "Parallel") {
		t.Error("expected Mermaid parallel subgraph to be labeled Parallel")
	}
}

func TestGenerateMermaidIf(t *testing.T) {
	mermaid := GenerateMermaid(ifWorkflow)
	if !strings.Contains(mermaid, "{") {
		t.Error("expected Mermaid if to contain diamond shape for condition")
	}
	if !strings.Contains(mermaid, "true") {
		t.Error("expected Mermaid if to contain true edge label")
	}
	if !strings.Contains(mermaid, "false") {
		t.Error("expected Mermaid if to contain false edge label")
	}
}

func TestGenerateDOT(t *testing.T) {
	dot := GenerateDOT(simpleWorkflow)
	if !strings.Contains(dot, "digraph Workflow") {
		t.Error("expected DOT to contain 'digraph Workflow'")
	}
	if !strings.Contains(dot, "start") {
		t.Error("expected DOT to contain start node")
	}
	if !strings.Contains(dot, "shape=circle") {
		t.Error("expected DOT to contain circle shape for openai")
	}
	if !strings.Contains(dot, "shape=diamond") {
		t.Error("expected DOT to contain diamond shape for fetch_url")
	}
}

func TestGenerateDOTParallel(t *testing.T) {
	dot := GenerateDOT(parallelWorkflow)
	if !strings.Contains(dot, "fetch_url") {
		t.Error("expected DOT parallel to contain fetch_url nodes")
	}
}

func TestGenerateASCII(t *testing.T) {
	ascii := GenerateASCII(simpleWorkflow)
	if !strings.Contains(ascii, "Start") {
		t.Error("expected ASCII to contain Start")
	}
	if !strings.Contains(ascii, "fetch_url") {
		t.Error("expected ASCII to contain fetch_url")
	}
	if !strings.Contains(ascii, "openai") {
		t.Error("expected ASCII to contain openai")
	}
}

func TestGenerateJSON(t *testing.T) {
	data := GenerateJSON(simpleWorkflow)
	if data.Metadata.Name != "simple-test" {
		t.Errorf("expected metadata name 'simple-test', got %q", data.Metadata.Name)
	}
	if data.Metadata.StepCount != 3 {
		t.Errorf("expected step count 3, got %d", data.Metadata.StepCount)
	}

	// Should have start + 3 nodes
	if len(data.Nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(data.Nodes))
	}

	// Check start node
	foundStart := false
	for _, n := range data.Nodes {
		if n.ID == "start" && n.Type == "start" {
			foundStart = true
			break
		}
	}
	if !foundStart {
		t.Error("expected JSON nodes to contain start node")
	}

	// Check edges connect start to first step
	if len(data.Edges) == 0 {
		t.Error("expected at least one edge")
	}
	foundStartEdge := false
	for _, e := range data.Edges {
		if e.From == "start" {
			foundStartEdge = true
			break
		}
	}
	if !foundStartEdge {
		t.Error("expected at least one edge from start")
	}
}

func TestGenerateJSONParallel(t *testing.T) {
	data := GenerateJSON(parallelWorkflow)
	if data.Metadata.StepCount != 3 {
		t.Errorf("expected step count 3, got %d", data.Metadata.StepCount)
	}
	// start + parallel_start + 2 parallel steps + parallel_end + combine + file_write = 7
	if len(data.Nodes) != 7 {
		t.Errorf("expected 7 nodes for parallel workflow, got %d", len(data.Nodes))
	}
}

func TestGenerateJSONIf(t *testing.T) {
	data := GenerateJSON(ifWorkflow)
	if data.Metadata.StepCount != 3 {
		t.Errorf("expected step count 3, got %d", data.Metadata.StepCount)
	}

	// Verify condition node exists
	foundCondition := false
	for _, n := range data.Nodes {
		if n.Type == "condition" {
			foundCondition = true
			if n.Color != "#f1c40f" {
				t.Errorf("expected condition node color #f1c40f, got %s", n.Color)
			}
			if n.Shape != "diamond" {
				t.Errorf("expected condition node shape diamond, got %s", n.Shape)
			}
			break
		}
	}
	if !foundCondition {
		t.Error("expected JSON nodes to contain condition node")
	}

	// Verify true/false edges
	foundTrue := false
	foundFalse := false
	for _, e := range data.Edges {
		if e.Label == "true" {
			foundTrue = true
		}
		if e.Label == "false" {
			foundFalse = true
		}
	}
	if !foundTrue {
		t.Error("expected true edge label")
	}
	if !foundFalse {
		t.Error("expected false edge label")
	}
}

func TestEmptyWorkflow(t *testing.T) {
	mermaid := GenerateMermaid(emptyWorkflow)
	if !strings.Contains(mermaid, "start([Start])") {
		t.Error("expected empty workflow Mermaid to still have start node")
	}

	dot := GenerateDOT(emptyWorkflow)
	if !strings.Contains(dot, "start") {
		t.Error("expected empty workflow DOT to still have start node")
	}

	ascii := GenerateASCII(emptyWorkflow)
	if !strings.Contains(ascii, "Start") {
		t.Error("expected empty workflow ASCII to still have Start")
	}

	data := GenerateJSON(emptyWorkflow)
	if len(data.Nodes) != 1 {
		t.Errorf("expected 1 node for empty workflow, got %d", len(data.Nodes))
	}
	if len(data.Edges) != 0 {
		t.Errorf("expected 0 edges for empty workflow, got %d", len(data.Edges))
	}
}

func TestInvalidYAML(t *testing.T) {
	invalid := "this is not: valid: yaml: ["
	mermaid := GenerateMermaid(invalid)
	if !strings.Contains(mermaid, "Error") {
		t.Error("expected Mermaid error for invalid YAML")
	}

	dot := GenerateDOT(invalid)
	if !strings.Contains(dot, "Error") {
		t.Error("expected DOT error for invalid YAML")
	}

	ascii := GenerateASCII(invalid)
	if !strings.Contains(ascii, "Error") {
		t.Error("expected ASCII error for invalid YAML")
	}

	data := GenerateJSON(invalid)
	if len(data.Nodes) != 0 {
		t.Error("expected empty JSON data for invalid YAML")
	}
}

func TestNodeStyles(t *testing.T) {
	tests := []struct {
		nodeType      string
		expectedColor string
		expectedShape string
	}{
		{"openai", "#3498db", "circle"},
		{"ollama", "#3498db", "circle"},
		{"execute", "#e74c3c", "rect"},
		{"fetch_url", "#2ecc71", "diamond"},
		{"http_request", "#2ecc71", "diamond"},
		{"condition", "#f1c40f", "diamond"},
		{"file_read", "#95a5a6", "cylinder"},
		{"file_write", "#95a5a6", "cylinder"},
		{"unknown", "#ffffff", "rect"},
	}

	for _, tt := range tests {
		color, shape := getNodeStyle(tt.nodeType)
		if color != tt.expectedColor {
			t.Errorf("nodeType %s: expected color %s, got %s", tt.nodeType, tt.expectedColor, color)
		}
		if shape != tt.expectedShape {
			t.Errorf("nodeType %s: expected shape %s, got %s", tt.nodeType, tt.expectedShape, shape)
		}
	}
}

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "node"},
		{"fetch_url", "fetch_url"},
		{"123abc", "n123abc"},
		{"a-b-c", "a_b_c"},
		{"___", "n___"},
	}

	for _, tt := range tests {
		result := sanitizeID(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeID(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGenerateJSONLayout(t *testing.T) {
	data := GenerateJSON(simpleWorkflow)
	for i, n := range data.Nodes {
		if n.X < 0 || n.Y < 0 {
			t.Errorf("node %s has negative coordinates (%d, %d)", n.ID, n.X, n.Y)
		}
		if i > 0 {
			// Check grid layout: nodes should be spaced by 150 horizontally and 100 vertically
			col := (i - 1) % 3
			row := (i - 1) / 3
			expectedX := (col + 1) * 150
			expectedY := (row + 1) * 100
			if n.X != expectedX || n.Y != expectedY {
				t.Errorf("node %s expected (%d,%d), got (%d,%d)", n.ID, expectedX, expectedY, n.X, n.Y)
			}
		}
	}
}
