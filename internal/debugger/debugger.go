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

// NewDebugger 创建并返回一个空的调试器实例。
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

// AddBreakpoint 添加断点：index 为步骤序号（>=0 有效），name 为步骤名称（非空有效）。
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

// RemoveBreakpoint 移除断点：按 index 与 name 分别删除对应条目。
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

// ListBreakpoints 返回当前已设置的全部断点列表。
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

// Step 在暂停状态下放行一个步骤；未暂停时为空操作。
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

// Continue 清除暂停标记并放行当前阻塞步骤，使后续步骤连续执行直到下一个断点。
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

// Pause 设置暂停标记，使下一个步骤进入 WaitForStep 时阻塞。
func (d *Debugger) Pause() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.pauseFlag = true
}

// WaitForStep 在执行某步骤前更新调试状态，并根据断点/暂停标记决定是否阻塞。
// 返回 true 表示可继续执行；ctx 取消时返回 false 并清除暂停状态。
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

// RecordStepOutput 记录指定步骤的输出，用于调试时回放。
func (d *Debugger) RecordStepOutput(stepIndex int, output string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stepOutputs[stepIndex] = output
	d.state.StepOutputs[stepIndex] = output
}

// GetState 返回当前调试状态的深拷贝，调用方可安全修改。
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
