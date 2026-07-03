package tui

import (
	"errors"
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
