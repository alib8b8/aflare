package debugger

import (
	"context"
	"sync"
)

type Breakpoint struct {
	Index int
	Name  string
}

type DebugState struct {
	CurrentStepIndex int
	CurrentStepName  string
	CurrentInput     string
	Variables        map[string]string
	Paused           bool
	ExecutedSteps    int
	StepOutputs      map[int]string
}

type Debugger struct {
	mu sync.RWMutex

	breakpointsByIndex map[int]bool
	breakpointsByName  map[string]bool

	state DebugState

	stepChan  chan struct{}
	pauseFlag bool

	stepOutputs map[int]string
}

func NewDebugger() *Debugger {
	return &Debugger{
		breakpointsByIndex: make(map[int]bool),
		breakpointsByName:  make(map[string]bool),
		state: DebugState{
			Variables:   make(map[string]string),
			StepOutputs: make(map[int]string),
		},
		stepChan:    make(chan struct{}),
		stepOutputs: make(map[int]string),
	}
}

func (d *Debugger) AddBreakpoint(index int, name string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if index >= 0 {
		d.breakpointsByIndex[index] = true
	}
	if name != "" {
		d.breakpointsByName[name] = true
	}
}

func (d *Debugger) RemoveBreakpoint(index int, name string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if index >= 0 {
		delete(d.breakpointsByIndex, index)
	}
	if name != "" {
		delete(d.breakpointsByName, name)
	}
}

func (d *Debugger) ListBreakpoints() []Breakpoint {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var bps []Breakpoint

	for idx := range d.breakpointsByIndex {
		bps = append(bps, Breakpoint{Index: idx})
	}

	for name := range d.breakpointsByName {
		bps = append(bps, Breakpoint{Name: name})
	}

	return bps
}

func (d *Debugger) Step() {
	d.mu.Lock()
	paused := d.state.Paused
	d.mu.Unlock()

	if paused {
		select {
		case d.stepChan <- struct{}{}:
		default:
		}
	}
}

func (d *Debugger) Continue() {
	d.mu.Lock()
	d.pauseFlag = false
	paused := d.state.Paused
	d.mu.Unlock()

	if paused {
		select {
		case d.stepChan <- struct{}{}:
		default:
		}
	}
}

func (d *Debugger) Pause() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.pauseFlag = true
}

func (d *Debugger) WaitForStep(ctx context.Context, stepIndex int, stepName string, input string, variables map[string]string) bool {
	d.mu.Lock()

	d.state.CurrentStepIndex = stepIndex
	d.state.CurrentStepName = stepName
	d.state.CurrentInput = input

	if variables != nil {
		d.state.Variables = make(map[string]string, len(variables))
		for k, v := range variables {
			d.state.Variables[k] = v
		}
	}

	shouldPause := d.pauseFlag || d.breakpointsByIndex[stepIndex] || (stepName != "" && d.breakpointsByName[stepName])

	if !shouldPause {
		d.mu.Unlock()
		return true
	}

	d.state.Paused = true
	d.mu.Unlock()

	select {
	case <-ctx.Done():
		d.mu.Lock()
		d.state.Paused = false
		d.mu.Unlock()
		return false
	case <-d.stepChan:
		d.mu.Lock()
		d.state.Paused = false
		d.state.ExecutedSteps++
		d.mu.Unlock()
		return true
	}
}

func (d *Debugger) RecordStepOutput(stepIndex int, output string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stepOutputs[stepIndex] = output
	d.state.StepOutputs[stepIndex] = output
}

func (d *Debugger) GetState() DebugState {
	d.mu.RLock()
	defer d.mu.RUnlock()

	state := d.state
	state.Variables = make(map[string]string, len(d.state.Variables))
	for k, v := range d.state.Variables {
		state.Variables[k] = v
	}
	state.StepOutputs = make(map[int]string, len(d.state.StepOutputs))
	for k, v := range d.state.StepOutputs {
		state.StepOutputs[k] = v
	}

	return state
}
