// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌​​​‌​‌‌​​‌‌‌‌​‌​‌​‌‌​​‌​​‌​​​​​‌‌​​‌‌‌‌‌‌‌‌‌​​​​​​​​​​​​​​​​​‌‌​‌​​‌​‌‌‌​​‌‌​⁠
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
	"fmt"
	"strings"

	"github.com/alib8b8/aflare/internal/workflow"
	"gopkg.in/yaml.v3"
)

// NodeVisual represents a node in the workflow visualization.
type NodeVisual struct {
	ID     string            `json:"id"`
	Type   string            `json:"type"`
	Label  string            `json:"label"`
	X      int               `json:"x"`
	Y      int               `json:"y"`
	Color  string            `json:"color"`
	Shape  string            `json:"shape"`
	Params map[string]string `json:"params,omitempty"`
}

// EdgeVisual represents an edge between two nodes.
type EdgeVisual struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
	Style string `json:"style"`
}

// VisualizationMetadata holds metadata about the workflow.
type VisualizationMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	StepCount   int    `json:"step_count"`
}

// VisualizationData is the complete visualization data for frontend rendering.
type VisualizationData struct {
	Nodes    []NodeVisual          `json:"nodes"`
	Edges    []EdgeVisual          `json:"edges"`
	Metadata VisualizationMetadata `json:"metadata"`
}

// internal representation during traversal
type visualNode struct {
	ID       string
	Type     string
	Label    string
	Params   map[string]string
	Subgraph string
}

type visualEdge struct {
	From  string
	To    string
	Label string
}

type workflowVisitor struct {
	nodes       []visualNode
	edges       []visualEdge
	nodeCounter int
}

func (v *workflowVisitor) nextID(prefix string) string {
	v.nodeCounter++
	if prefix == "" {
		prefix = "node"
	}
	return fmt.Sprintf("%s_%d", prefix, v.nodeCounter)
}

func (v *workflowVisitor) addNode(id, nodeType, label, subgraph string, params map[string]string) {
	v.nodes = append(v.nodes, visualNode{
		ID:       id,
		Type:     nodeType,
		Label:    label,
		Params:   params,
		Subgraph: subgraph,
	})
}

func (v *workflowVisitor) addEdge(from, to, label string) {
	v.edges = append(v.edges, visualEdge{From: from, To: to, Label: label})
}

func (v *workflowVisitor) visitSteps(steps []workflow.WorkflowStep, prevIDs []string, subgraph string) []string {
	currentPrev := prevIDs
	for _, step := range steps {
		nextPrev := v.visitWorkflowStep(step, currentPrev, subgraph)
		currentPrev = nextPrev
	}
	return currentPrev
}

func (v *workflowVisitor) visitWorkflowStep(step workflow.WorkflowStep, prevIDs []string, subgraph string) []string {
	switch {
	case step.IsParallel():
		return v.visitParallel(step, prevIDs, subgraph)
	case step.IsIf():
		return v.visitIf(step, prevIDs, subgraph)
	default:
		return v.visitSimpleStep(step, prevIDs, subgraph)
	}
}

func (v *workflowVisitor) visitSimpleStep(step workflow.WorkflowStep, prevIDs []string, subgraph string) []string {
	nodeType := step.Node
	if nodeType == "" {
		nodeType = "step"
	}
	label := step.Name
	if label == "" {
		label = step.Node
	}
	id := v.nextID(sanitizeID(nodeType))
	v.addNode(id, nodeType, label, subgraph, step.Params)
	for _, prev := range prevIDs {
		v.addEdge(prev, id, "")
	}
	return []string{id}
}

func (v *workflowVisitor) visitParallel(step workflow.WorkflowStep, prevIDs []string, subgraph string) []string {
	subgID := v.nextID("parallel")
	startID := subgID + "_start"
	v.addNode(startID, "parallel", "Parallel", subgID, nil)
	for _, prev := range prevIDs {
		v.addEdge(prev, startID, "")
	}

	endID := subgID + "_end"
	v.addNode(endID, "parallel_end", "End Parallel", subgID, nil)

	for _, pStep := range step.Parallel {
		pID := v.nextID(sanitizeID(pStep.Node))
		label := pStep.Node
		v.addNode(pID, pStep.Node, label, subgID, pStep.Params)
		v.addEdge(startID, pID, "")
		v.addEdge(pID, endID, "")
	}

	return []string{endID}
}

func (v *workflowVisitor) visitIf(step workflow.WorkflowStep, prevIDs []string, subgraph string) []string {
	condID := v.nextID("condition")
	condLabel := "condition"
	if step.If.Condition != "" {
		condLabel = step.If.Condition
	}
	v.addNode(condID, "condition", condLabel, subgraph, nil)
	for _, prev := range prevIDs {
		v.addEdge(prev, condID, "")
	}

	thenEndIDs := v.visitSteps(step.If.Then, []string{condID}, subgraph)

	var elseEndIDs []string
	if len(step.If.Else) > 0 {
		elseEndIDs = v.visitSteps(step.If.Else, []string{condID}, subgraph)
	} else {
		elseEndIDs = []string{condID}
	}

	mergeID := v.nextID("merge")
	v.addNode(mergeID, "merge", "End If", subgraph, nil)
	for _, id := range thenEndIDs {
		v.addEdge(id, mergeID, "true")
	}
	for _, id := range elseEndIDs {
		v.addEdge(id, mergeID, "false")
	}

	return []string{mergeID}
}

func sanitizeID(s string) string {
	if s == "" {
		return "node"
	}
	var sb strings.Builder
	for i, r := range s {
		if i == 0 && !isLetter(r) {
			sb.WriteByte('n')
		}
		if isLetter(r) || isDigit(r) {
			sb.WriteRune(r)
		} else {
			sb.WriteByte('_')
		}
	}
	result := sb.String()
	if result == "" {
		return "node"
	}
	return result
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func getNodeStyle(nodeType string) (color, shape string) {
	switch nodeType {
	case "openai", "ollama", "deepseek", "glm", "qwen", "kimi", "baichuan", "yi", "xverse", "mistral", "minimax", "mimo", "ima", "internlm", "fastgpt", "coze", "llm":
		return "#3498db", "circle"
	case "execute":
		return "#e74c3c", "rect"
	case "fetch_url", "http_request":
		return "#2ecc71", "diamond"
	case "condition", "if":
		return "#f1c40f", "diamond"
	case "file_read", "file_write":
		return "#95a5a6", "cylinder"
	default:
		return "#ffffff", "rect"
	}
}

func getMermaidShape(nodeType, label string) string {
	switch nodeType {
	case "condition", "if":
		return fmt.Sprintf("{%s}", label)
	case "openai", "ollama", "deepseek", "glm", "qwen", "kimi", "baichuan", "yi", "xverse", "mistral", "minimax", "mimo", "ima", "internlm", "fastgpt", "coze", "llm":
		return fmt.Sprintf("((%s))", label)
	case "file_read", "file_write":
		return fmt.Sprintf("[(%s)]", label)
	default:
		return fmt.Sprintf("[%s]", label)
	}
}

func getDOTShapeColor(nodeType string) (shape, color string) {
	switch nodeType {
	case "openai", "ollama", "deepseek", "glm", "qwen", "kimi", "baichuan", "yi", "xverse", "mistral", "minimax", "mimo", "ima", "internlm", "fastgpt", "coze", "llm":
		return "circle", "#3498db"
	case "execute":
		return "box", "#e74c3c"
	case "fetch_url", "http_request":
		return "diamond", "#2ecc71"
	case "condition", "if":
		return "diamond", "#f1c40f"
	case "file_read", "file_write":
		return "cylinder", "#95a5a6"
	default:
		return "box", "#ffffff"
	}
}

func parseWorkflowYAML(yamlStr string) (*workflow.Workflow, error) {
	var wf workflow.Workflow
	if err := yaml.Unmarshal([]byte(yamlStr), &wf); err != nil {
		return nil, err
	}
	return &wf, nil
}

// GenerateMermaid generates a Mermaid flowchart with top-down direction.
func GenerateMermaid(workflowYAML string) string {
	return GenerateMermaidWithDirection(workflowYAML, "TD")
}

// GenerateMermaidWithDirection generates a Mermaid flowchart with the specified direction.
func GenerateMermaidWithDirection(workflowYAML string, direction string) string {
	wf, err := parseWorkflowYAML(workflowYAML)
	if err != nil {
		return fmt.Sprintf("%% Error: %v", err)
	}

	visitor := &workflowVisitor{}
	if len(wf.Steps) > 0 {
		visitor.visitSteps(wf.Steps, []string{"start"}, "")
	}

	var sb strings.Builder
	sb.WriteString("flowchart ")
	sb.WriteString(direction)
	sb.WriteString("\n")

	sb.WriteString("    start([Start])\n")

	// Nodes not in a subgraph
	for _, node := range visitor.nodes {
		if node.Subgraph == "" {
			shape := getMermaidShape(node.Type, node.Label)
			sb.WriteString(fmt.Sprintf("    %s%s\n", node.ID, shape))
		}
	}

	// Subgraphs
	subgraphMap := make(map[string][]visualNode)
	for _, node := range visitor.nodes {
		if node.Subgraph != "" {
			subgraphMap[node.Subgraph] = append(subgraphMap[node.Subgraph], node)
		}
	}

	for subgID, nodes := range subgraphMap {
		sb.WriteString(fmt.Sprintf("    subgraph %s [Parallel]\n", subgID))
		for _, node := range nodes {
			shape := getMermaidShape(node.Type, node.Label)
			sb.WriteString(fmt.Sprintf("        %s%s\n", node.ID, shape))
		}
		sb.WriteString("    end\n")
	}

	// Edges
	for _, edge := range visitor.edges {
		if edge.Label != "" {
			sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", edge.From, edge.Label, edge.To))
		} else {
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", edge.From, edge.To))
		}
	}

	return sb.String()
}

// GenerateDOT generates a Graphviz DOT representation.
func GenerateDOT(workflowYAML string) string {
	wf, err := parseWorkflowYAML(workflowYAML)
	if err != nil {
		return fmt.Sprintf("// Error: %v", err)
	}

	visitor := &workflowVisitor{}
	if len(wf.Steps) > 0 {
		visitor.visitSteps(wf.Steps, []string{"start"}, "")
	}

	var sb strings.Builder
	sb.WriteString("digraph Workflow {\n")
	sb.WriteString("    rankdir=TB;\n")
	sb.WriteString("    node [fontname=\"Helvetica\"];\n")
	sb.WriteString("    edge [fontname=\"Helvetica\"];\n")

	sb.WriteString("    start [label=\"Start\", shape=ellipse, style=filled, fillcolor=\"#ecf0f1\"];\n")

	for _, node := range visitor.nodes {
		shape, color := getDOTShapeColor(node.Type)
		label := strings.ReplaceAll(node.Label, "\"", "\\\"")
		sb.WriteString(fmt.Sprintf("    %s [label=\"%s\", shape=%s, style=filled, fillcolor=\"%s\"];\n",
			node.ID, label, shape, color))
	}

	for _, edge := range visitor.edges {
		if edge.Label != "" {
			sb.WriteString(fmt.Sprintf("    %s -> %s [label=\"%s\"];\n", edge.From, edge.To, edge.Label))
		} else {
			sb.WriteString(fmt.Sprintf("    %s -> %s;\n", edge.From, edge.To))
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}

// GenerateASCII generates a simple ASCII text diagram.
func GenerateASCII(workflowYAML string) string {
	wf, err := parseWorkflowYAML(workflowYAML)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	visitor := &workflowVisitor{}
	if len(wf.Steps) > 0 {
		visitor.visitSteps(wf.Steps, []string{"start"}, "")
	}

	var sb strings.Builder
	sb.WriteString("+--------+\n")
	sb.WriteString("| Start  |\n")
	sb.WriteString("+--------+\n")

	for i, node := range visitor.nodes {
		label := node.Label
		if len(label) > 14 {
			label = label[:14]
		}
		width := len(label) + 4
		border := strings.Repeat("-", width)
		sb.WriteString(fmt.Sprintf("+%s+\n", border))
		sb.WriteString(fmt.Sprintf("| %s |\n", centerString(label, width-2)))
		sb.WriteString(fmt.Sprintf("+%s+\n", border))
		if i < len(visitor.nodes)-1 {
			sb.WriteString("    |\n")
			sb.WriteString("    v\n")
		}
	}

	return sb.String()
}

func centerString(s string, width int) string {
	if len(s) >= width {
		return s
	}
	left := (width - len(s)) / 2
	right := width - len(s) - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// GenerateJSON generates visualization data for frontend rendering.
func GenerateJSON(workflowYAML string) VisualizationData {
	wf, err := parseWorkflowYAML(workflowYAML)
	if err != nil {
		return VisualizationData{}
	}

	visitor := &workflowVisitor{}
	if len(wf.Steps) > 0 {
		visitor.visitSteps(wf.Steps, []string{"start"}, "")
	}

	nodes := make([]NodeVisual, 0, len(visitor.nodes)+1)
	nodes = append(nodes, NodeVisual{
		ID:    "start",
		Type:  "start",
		Label: "Start",
		X:     0,
		Y:     0,
		Color: "#ecf0f1",
		Shape: "ellipse",
	})

	cols := 3
	spacingX := 150
	spacingY := 100

	for i, vn := range visitor.nodes {
		color, shape := getNodeStyle(vn.Type)
		row := i / cols
		col := i % cols
		nodes = append(nodes, NodeVisual{
			ID:     vn.ID,
			Type:   vn.Type,
			Label:  vn.Label,
			X:      (col + 1) * spacingX,
			Y:      (row + 1) * spacingY,
			Color:  color,
			Shape:  shape,
			Params: vn.Params,
		})
	}

	edges := make([]EdgeVisual, 0, len(visitor.edges))
	for _, ve := range visitor.edges {
		edges = append(edges, EdgeVisual{
			From:  ve.From,
			To:    ve.To,
			Label: ve.Label,
			Style: "solid",
		})
	}

	return VisualizationData{
		Nodes: nodes,
		Edges: edges,
		Metadata: VisualizationMetadata{
			Name:        wf.Name,
			Description: wf.Description,
			StepCount:   len(wf.Steps),
		},
	}
}
