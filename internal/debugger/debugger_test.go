package debugger

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewDebugger(t *testing.T) {
	d := NewDebugger()
	if d == nil {
		t.Fatal("expected non-nil debugger")
	}

	state := d.GetState()
	if state.Paused {
		t.Error("expected initial state not paused")
	}
	if state.ExecutedSteps != 0 {
		t.Errorf("expected 0 executed steps, got %d", state.ExecutedSteps)
	}
	if state.CurrentStepIndex != 0 {
		t.Errorf("expected 0 current step index, got %d", state.CurrentStepIndex)
	}
}

func TestAddAndRemoveBreakpoint_ByIndex(t *testing.T) {
	d := NewDebugger()

	d.AddBreakpoint(2, "")
	d.AddBreakpoint(5, "")

	bps := d.ListBreakpoints()
	if len(bps) != 2 {
		t.Fatalf("expected 2 breakpoints, got %d", len(bps))
	}

	found := make(map[int]bool)
	for _, bp := range bps {
		if bp.Index >= 0 {
			found[bp.Index] = true
		}
	}
	if !found[2] || !found[5] {
		t.Error("expected breakpoints at index 2 and 5")
	}

	d.RemoveBreakpoint(2, "")
	bps = d.ListBreakpoints()
	if len(bps) != 1 {
		t.Errorf("expected 1 breakpoint after removal, got %d", len(bps))
	}
}

func TestAddAndRemoveBreakpoint_ByName(t *testing.T) {
	d := NewDebugger()

	d.AddBreakpoint(-1, "step-a")
	d.AddBreakpoint(-1, "step-b")

	bps := d.ListBreakpoints()
	nameCount := 0
	for _, bp := range bps {
		if bp.Name != "" {
			nameCount++
		}
	}
	if nameCount != 2 {
		t.Fatalf("expected 2 name breakpoints, got %d", nameCount)
	}

	d.RemoveBreakpoint(-1, "step-a")
	bps = d.ListBreakpoints()
	nameCount = 0
	for _, bp := range bps {
		if bp.Name != "" {
			nameCount++
		}
	}
	if nameCount != 1 {
		t.Errorf("expected 1 name breakpoint after removal, got %d", nameCount)
	}
}

func TestListBreakpoints_Empty(t *testing.T) {
	d := NewDebugger()
	bps := d.ListBreakpoints()
	if len(bps) != 0 {
		t.Errorf("expected 0 breakpoints, got %d", len(bps))
	}
}

func TestWaitForStep_NoBreakpoint(t *testing.T) {
	d := NewDebugger()
	ctx := context.Background()

	result := d.WaitForStep(ctx, 0, "step-0", "input-0", nil)
	if !result {
		t.Error("expected WaitForStep to return true when no breakpoint")
	}

	state := d.GetState()
	if state.Paused {
		t.Error("expected not paused when no breakpoint")
	}
	if state.CurrentStepIndex != 0 {
		t.Errorf("expected current step index 0, got %d", state.CurrentStepIndex)
	}
	if state.CurrentStepName != "step-0" {
		t.Errorf("expected current step name 'step-0', got '%s'", state.CurrentStepName)
	}
	if state.CurrentInput != "input-0" {
		t.Errorf("expected current input 'input-0', got '%s'", state.CurrentInput)
	}
}

func TestWaitForStep_BreakpointByIndex(t *testing.T) {
	d := NewDebugger()
	d.AddBreakpoint(1, "")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	var result bool
	go func() {
		defer wg.Done()
		result = d.WaitForStep(ctx, 1, "step-1", "input-1", nil)
	}()

	time.Sleep(20 * time.Millisecond)

	state := d.GetState()
	if !state.Paused {
		t.Error("expected paused at breakpoint")
	}

	d.Step()
	wg.Wait()

	if !result {
		t.Error("expected WaitForStep to return true after Step")
	}

	state = d.GetState()
	if state.Paused {
		t.Error("expected not paused after Step")
	}
	if state.ExecutedSteps != 1 {
		t.Errorf("expected 1 executed step, got %d", state.ExecutedSteps)
	}
}

func TestWaitForStep_BreakpointByName(t *testing.T) {
	d := NewDebugger()
	d.AddBreakpoint(-1, "special-step")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	var result bool
	go func() {
		defer wg.Done()
		result = d.WaitForStep(ctx, 0, "special-step", "test-input", nil)
	}()

	time.Sleep(20 * time.Millisecond)

	state := d.GetState()
	if !state.Paused {
		t.Error("expected paused at name breakpoint")
	}

	d.Continue()
	wg.Wait()

	if !result {
		t.Error("expected WaitForStep to return true after Continue")
	}
}

func TestPauseAndResume(t *testing.T) {
	d := NewDebugger()
	d.Pause()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		d.WaitForStep(ctx, 0, "step-0", "input", nil)
	}()

	time.Sleep(20 * time.Millisecond)

	state := d.GetState()
	if !state.Paused {
		t.Error("expected paused due to pause flag")
	}

	d.Continue()
	wg.Wait()

	state = d.GetState()
	if state.Paused {
		t.Error("expected not paused after Continue")
	}
}

func TestWaitForStep_ContextCancelled(t *testing.T) {
	d := NewDebugger()
	d.AddBreakpoint(0, "")

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	var result bool
	go func() {
		defer wg.Done()
		result = d.WaitForStep(ctx, 0, "step-0", "input", nil)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	wg.Wait()

	if result {
		t.Error("expected WaitForStep to return false when context cancelled")
	}

	state := d.GetState()
	if state.Paused {
		t.Error("expected not paused after context cancellation")
	}
}

func TestGetState_Variables(t *testing.T) {
	d := NewDebugger()
	vars := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	ctx := context.Background()
	d.WaitForStep(ctx, 0, "step-0", "input", vars)

	state := d.GetState()
	if len(state.Variables) != 2 {
		t.Errorf("expected 2 variables, got %d", len(state.Variables))
	}
	if state.Variables["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %s", state.Variables["key1"])
	}
	if state.Variables["key2"] != "value2" {
		t.Errorf("expected key2=value2, got %s", state.Variables["key2"])
	}
}

func TestGetState_StepOutputs(t *testing.T) {
	d := NewDebugger()

	d.RecordStepOutput(0, "output-0")
	d.RecordStepOutput(1, "output-1")

	state := d.GetState()
	if len(state.StepOutputs) != 2 {
		t.Errorf("expected 2 step outputs, got %d", len(state.StepOutputs))
	}
	if state.StepOutputs[0] != "output-0" {
		t.Errorf("expected step 0 output 'output-0', got '%s'", state.StepOutputs[0])
	}
	if state.StepOutputs[1] != "output-1" {
		t.Errorf("expected step 1 output 'output-1', got '%s'", state.StepOutputs[1])
	}
}

func TestGetState_CopyIsolation(t *testing.T) {
	d := NewDebugger()

	ctx := context.Background()
	vars := map[string]string{"key": "value"}
	d.WaitForStep(ctx, 0, "step-0", "input", vars)
	d.RecordStepOutput(0, "output")

	state1 := d.GetState()
	state1.Variables["key"] = "modified"
	state1.StepOutputs[0] = "modified"

	state2 := d.GetState()
	if state2.Variables["key"] != "value" {
		t.Error("expected variable isolation, but modification affected original")
	}
	if state2.StepOutputs[0] != "output" {
		t.Error("expected step output isolation, but modification affected original")
	}
}

func TestConcurrentAccess(t *testing.T) {
	d := NewDebugger()
	var wg sync.WaitGroup
	numGoroutines := 10
	numOps := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				idx := (id*numOps + j) % 100
				d.AddBreakpoint(idx, "")
				d.ListBreakpoints()
				d.GetState()
				d.RecordStepOutput(idx, "out")
				d.RemoveBreakpoint(idx, "")
			}
		}(i)
	}

	wg.Wait()
}

func TestContinueClearsPauseFlag(t *testing.T) {
	d := NewDebugger()
	d.Pause()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		d.WaitForStep(ctx, 0, "step-0", "input", nil)
	}()

	time.Sleep(20 * time.Millisecond)
	d.Continue()
	wg.Wait()

	result := d.WaitForStep(context.Background(), 1, "step-1", "input2", nil)
	if !result {
		t.Error("expected second step to pass without pause after Continue clears pause flag")
	}
}

func TestStep_WhenNotPaused(t *testing.T) {
	d := NewDebugger()

	done := make(chan struct{})
	go func() {
		d.Step()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Error("Step should not block when not paused")
	}
}

func TestContinue_WhenNotPaused(t *testing.T) {
	d := NewDebugger()

	done := make(chan struct{})
	go func() {
		d.Continue()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Error("Continue should not block when not paused")
	}
}
