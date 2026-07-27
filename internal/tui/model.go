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
	"fmt"
	"strings"
	"time"

	"github.com/alib8b8/llm-box/internal/i18n"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StepStatus represents the execution status of a step
type StepStatus int

const (
	StatusPending StepStatus = iota
	StatusRunning
	StatusDone
	StatusError
)

// Step represents a single workflow step in the TUI
type Step struct {
	Name         string
	Params       map[string]string
	Status       StepStatus
	Output       string
	StreamOutput string
	Error        string
	Duration     time.Duration
	Streaming    bool
}

// StepStartMsg is sent when a step starts executing
type StepStartMsg struct {
	Index int
	Name  string
}

// StepStreamMsg is sent when a streaming step produces a chunk of output
type StepStreamMsg struct {
	Index int
	Name  string
	Chunk string
}

// StepEndMsg is sent when a step finishes executing
type StepEndMsg struct {
	Index    int
	Name     string
	Output   string
	Error    error
	Duration time.Duration
}

// WorkflowStartMsg is sent when the workflow starts
type WorkflowStartMsg struct {
	Name  string
	Path  string
	Steps int
}

// WorkflowEndMsg is sent when the workflow ends
type WorkflowEndMsg struct {
	Success bool
}

// Styles using lipgloss
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 2).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1)

	pendingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")).
			Bold(true)

	doneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	previewStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2).
			MarginTop(1)
)

// Model is the bubbletea model
type Model struct {
	workflowName string
	workflowPath string
	steps        []Step
	finished     bool
	success      bool
	width        int
	height       int
}

// NewModel creates a new TUI model
func NewModel(workflowName, workflowPath string, stepCount int) *Model {
	steps := make([]Step, stepCount)
	for i := 0; i < stepCount; i++ {
		steps[i] = Step{
			Status: StatusPending,
		}
	}
	return &Model{
		workflowName: workflowName,
		workflowPath: workflowPath,
		steps:        steps,
	}
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.finished {
			return m, tea.Quit
		}

	case WorkflowStartMsg:
		m.workflowName = msg.Name
		m.workflowPath = msg.Path
		// Reset steps
		m.steps = make([]Step, msg.Steps)
		for i := 0; i < msg.Steps; i++ {
			m.steps[i] = Step{Status: StatusPending}
		}
		return m, nil

	case StepStartMsg:
		if msg.Index >= 0 && msg.Index < len(m.steps) {
			m.steps[msg.Index].Name = msg.Name
			m.steps[msg.Index].Status = StatusRunning
			m.steps[msg.Index].Streaming = false
			m.steps[msg.Index].StreamOutput = ""
		}
		return m, nil

	case StepStreamMsg:
		if msg.Index >= 0 && msg.Index < len(m.steps) {
			m.steps[msg.Index].Name = msg.Name
			m.steps[msg.Index].Streaming = true
			m.steps[msg.Index].StreamOutput += msg.Chunk
		}
		return m, nil

	case StepEndMsg:
		if msg.Index >= 0 && msg.Index < len(m.steps) {
			m.steps[msg.Index].Name = msg.Name
			m.steps[msg.Index].Output = msg.Output
			m.steps[msg.Index].Duration = msg.Duration
			if msg.Error != nil {
				m.steps[msg.Index].Status = StatusError
				m.steps[msg.Index].Error = msg.Error.Error()
			} else {
				m.steps[msg.Index].Status = StatusDone
			}
		}
		return m, nil

	case WorkflowEndMsg:
		m.finished = true
		m.success = msg.Success
		return m, nil
	}

	return m, nil
}

// View renders the TUI
func (m *Model) View() string {
	var sb strings.Builder

	// Title
	title := fmt.Sprintf("🚀 %s", i18n.T("tui.title", m.workflowName))
	sb.WriteString(titleStyle.Render(title))
	sb.WriteString("\n")
	sb.WriteString(headerStyle.Render("📁 " + m.workflowPath))
	sb.WriteString("\n")

	// Steps section
	sb.WriteString(headerStyle.Render("📋 " + i18n.T("tui.steps")))
	sb.WriteString("\n")

	for i, step := range m.steps {
		status := m.renderStepStatus(step)
		line := fmt.Sprintf("  "+i18n.T("tui.step_index"), i+1, step.Name, status)
		if step.Status == StatusDone {
			line += fmt.Sprintf("  "+i18n.T("tui.step_duration"), step.Duration)
		}
		if step.Status == StatusError {
			line += fmt.Sprintf("  %s", step.Error)
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Data preview section
	sb.WriteString("\n")
	sb.WriteString(headerStyle.Render("📄 " + i18n.T("tui.output_preview")))
	sb.WriteString("\n")

	preview := ""
	streaming := false
	for i := len(m.steps) - 1; i >= 0; i-- {
		if m.steps[i].Status == StatusRunning && m.steps[i].Streaming && m.steps[i].StreamOutput != "" {
			preview = m.steps[i].StreamOutput
			streaming = true
			break
		}
		if m.steps[i].Status == StatusDone && m.steps[i].Output != "" {
			preview = m.steps[i].Output
			break
		}
	}

	if preview == "" {
		preview = i18n.T("tui.no_output")
	} else if len(preview) > 1000 {
		preview = preview[len(preview)-1000:] + "\n" + i18n.T("tui.showing_last", 1000)
	}

	if streaming {
		preview += " ▊"
	}

	sb.WriteString(previewStyle.Render(preview))
	sb.WriteString("\n")

	// Footer
	sb.WriteString("\n")
	if m.finished {
		if m.success {
			sb.WriteString(doneStyle.Render("✅ " + i18n.T("tui.finished_quit")))
		} else {
			sb.WriteString(errorStyle.Render("❌ " + i18n.T("tui.failed_quit")))
		}
	} else {
		sb.WriteString(runningStyle.Render("⏳ " + i18n.T("tui.running")))
	}

	return sb.String()
}

// renderStepStatus returns a colored status indicator
func (m *Model) renderStepStatus(step Step) string {
	switch step.Status {
	case StatusPending:
		return pendingStyle.Render("⏳ " + i18n.T("tui.status_pending"))
	case StatusRunning:
		return runningStyle.Render("🔄 " + i18n.T("tui.status_running"))
	case StatusDone:
		return doneStyle.Render("✅ " + i18n.T("tui.status_done"))
	case StatusError:
		return errorStyle.Render("❌ " + i18n.T("tui.status_error"))
	default:
		return "?"
	}
}
