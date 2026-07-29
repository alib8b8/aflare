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

package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alib8b8/llm-box/internal/nodes"
	tea "github.com/charmbracelet/bubbletea"
)

// ── evaluateCondition tests ──

func TestEvaluateCondition_EmptyCond(t *testing.T) {
	engine := NewExpressionEngine()
	pass, err := evaluateCondition("", "input", engine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Error("expected true for empty condition")
	}
}

func TestEvaluateCondition_TrueOp(t *testing.T) {
	engine := NewExpressionEngine()
	pass, err := evaluateCondition("true", "anything", engine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Error("expected true")
	}
}

func TestEvaluateCondition_FalseOp(t *testing.T) {
	engine := NewExpressionEngine()
	pass, err := evaluateCondition("false", "anything", engine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pass {
		t.Error("expected false")
	}
}

func TestEvaluateCondition_Contains(t *testing.T) {
	engine := NewExpressionEngine()
	pass, err := evaluateCondition("contains:world", "hello world", engine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Error("expected true for contains")
	}
}

func TestEvaluateCondition_NotContains(t *testing.T) {
	engine := NewExpressionEngine()
	pass, err := evaluateCondition("not contains:xyz", "hello world", engine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Error("expected true for not contains")
	}
}

func TestEvaluateCondition_Equals(t *testing.T) {
	engine := NewExpressionEngine()
	pass, err := evaluateCondition("equals:hello", "hello", engine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Error("expected true for equals")
	}
}

func TestEvaluateCondition_StartsWith(t *testing.T) {
	engine := NewExpressionEngine()
	pass, err := evaluateCondition("starts_with:hel", "hello", engine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Error("expected true for starts_with")
	}
}

func TestEvaluateCondition_EndsWith(t *testing.T) {
	engine := NewExpressionEngine()
	pass, err := evaluateCondition("ends_with:llo", "hello", engine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Error("expected true for ends_with")
	}
}

func TestEvaluateCondition_Regex(t *testing.T) {
	engine := NewExpressionEngine()
	pass, err := evaluateCondition(`regex:\d+`, "abc123", engine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Error("expected true for regex")
	}
}

func TestEvaluateCondition_RegexInvalid(t *testing.T) {
	engine := NewExpressionEngine()
	_, err := evaluateCondition("regex:[invalid", "input", engine)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestEvaluateCondition_EmptyOp(t *testing.T) {
	engine := NewExpressionEngine()
	pass, err := evaluateCondition("empty", "", engine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Error("expected true for empty")
	}
}

func TestEvaluateCondition_NotEmptyOp(t *testing.T) {
	engine := NewExpressionEngine()
	pass, err := evaluateCondition("not_empty", "hello", engine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Error("expected true for not_empty")
	}
}

func TestEvaluateCondition_UnknownOp(t *testing.T) {
	engine := NewExpressionEngine()
	_, err := evaluateCondition("unknown:val", "input", engine)
	if err == nil {
		t.Error("expected error for unknown operator")
	}
}

func TestEvaluateCondition_WithExpression(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetVariable("target", "hello")
	pass, err := evaluateCondition("equals:{{var.target}}", "hello", engine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Error("expected true when expression matches")
	}
}

func TestEvaluateCondition_NotPrefix(t *testing.T) {
	engine := NewExpressionEngine()
	pass, err := evaluateCondition("not equals:hello", "world", engine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pass {
		t.Error("expected true for not equals")
	}
}

// ── executeIfBranch tests ──

func TestExecuteIfBranch_ThenBranch(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wfStep := WorkflowStep{
		If: &IfConfig{
			Condition: "equals:yes",
			Then: []WorkflowStep{
				{Node: "test", Params: map[string]string{"prefix": "then"}},
			},
			Else: []WorkflowStep{
				{Node: "test", Params: map[string]string{"prefix": "else"}},
			},
		},
	}

	engine := NewExpressionEngine()
	results, output, err := executeIfBranch(context.Background(), 0, wfStep.If, "yes", engine, reg, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The if-step's input propagates into the branch sub-workflow, so the
	// test node receives "yes": output = prefix + " " + input = "then yes".
	if output != "then yes" {
		t.Errorf("expected 'then yes', got '%s'", output)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestExecuteIfBranch_ElseBranch(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wfStep := WorkflowStep{
		If: &IfConfig{
			Condition: "equals:yes",
			Then: []WorkflowStep{
				{Node: "test", Params: map[string]string{"prefix": "then"}},
			},
			Else: []WorkflowStep{
				{Node: "test", Params: map[string]string{"prefix": "else"}},
			},
		},
	}

	engine := NewExpressionEngine()
	results, output, err := executeIfBranch(context.Background(), 0, wfStep.If, "no", engine, reg, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Input "no" propagates into the else branch: "else" + " " + "no" = "else no".
	if output != "else no" {
		t.Errorf("expected 'else no', got '%s'", output)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestExecuteIfBranch_NestedDepthLimit(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	var buildNested func(depth int) *IfConfig
	buildNested = func(depth int) *IfConfig {
		if depth == 0 {
			return &IfConfig{
				Condition: "true",
				Then: []WorkflowStep{
					{Node: "test"},
				},
			}
		}
		return &IfConfig{
			Condition: "true",
			Then: []WorkflowStep{
				{If: buildNested(depth - 1)},
			},
		}
	}

	wfStep := WorkflowStep{
		If: buildNested(MaxIfDepth),
	}

	engine := NewExpressionEngine()
	ctx := context.WithValue(context.Background(), ifDepthKey, 0)
	_, _, err := executeIfBranch(ctx, 0, wfStep.If, "input", engine, reg, nil, nil)
	if err == nil {
		t.Error("expected error for exceeding max if depth")
	}
}

// ── applyOutputStrategy tests ──

func TestApplyOutputStrategy_First(t *testing.T) {
	output := "a\n---\nb\n---\nc"
	result := applyOutputStrategy(output, "first")
	if result != "a" {
		t.Errorf("expected 'a', got '%s'", result)
	}
}

func TestApplyOutputStrategy_Last(t *testing.T) {
	output := "a\n---\nb\n---\nc"
	result := applyOutputStrategy(output, "last")
	if result != "c" {
		t.Errorf("expected 'c', got '%s'", result)
	}
}

func TestApplyOutputStrategy_Longest(t *testing.T) {
	output := "a\n---\nbbb\n---\ncc"
	result := applyOutputStrategy(output, "longest")
	if result != "bbb" {
		t.Errorf("expected 'bbb', got '%s'", result)
	}
}

func TestApplyOutputStrategy_Shortest(t *testing.T) {
	output := "aaa\n---\nb\n---\ncc"
	result := applyOutputStrategy(output, "shortest")
	if result != "b" {
		t.Errorf("expected 'b', got '%s'", result)
	}
}

func TestApplyOutputStrategy_JSONArray(t *testing.T) {
	output := "hello\n---\n{\"key\":\"val\"}"
	result := applyOutputStrategy(output, "json_array")
	if !strings.Contains(result, "[") || !strings.Contains(result, "]") {
		t.Errorf("expected JSON array, got '%s'", result)
	}
}

func TestApplyOutputStrategy_JSONArrayWithJSON(t *testing.T) {
	output := "{\"a\":1}\n---\n{\"b\":2}"
	result := applyOutputStrategy(output, "json_array")
	expected := `[{"a":1},{"b":2}]`
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestApplyOutputStrategy_Default(t *testing.T) {
	output := "a\n---\nb"
	result := applyOutputStrategy(output, "")
	if result != output {
		t.Errorf("expected unchanged output, got '%s'", result)
	}
}

func TestApplyOutputStrategy_Join(t *testing.T) {
	output := "a\n---\nb"
	result := applyOutputStrategy(output, "join")
	if result != output {
		t.Errorf("expected unchanged output, got '%s'", result)
	}
}

func TestApplyOutputStrategy_Empty(t *testing.T) {
	if applyOutputStrategy("", "first") != "" {
		t.Error("expected empty for first on empty input")
	}
	if applyOutputStrategy("", "last") != "" {
		t.Error("expected empty for last on empty input")
	}
	if applyOutputStrategy("", "longest") != "" {
		t.Error("expected empty for longest on empty input")
	}
	if applyOutputStrategy("", "shortest") != "" {
		t.Error("expected empty for shortest on empty input")
	}
}

// ── State management tests ──

func TestSaveStateAndLoadState(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	// validateStatePath requires the file to exist, so create it first
	os.WriteFile("state.json", []byte{}, 0600)

	state := &WorkflowState{
		WorkflowName: "test",
		StepIndex:    2,
		Data:         "output",
		StepOutputs:  map[int]string{0: "a", 1: "b"},
		Variables:    map[string]string{"x": "1"},
		SavedAt:      time.Now(),
	}

	err := SaveState("state.json", state)
	if err != nil {
		t.Fatalf("unexpected error saving state: %v", err)
	}

	loaded, err := LoadState("state.json")
	if err != nil {
		t.Fatalf("unexpected error loading state: %v", err)
	}
	if loaded.WorkflowName != state.WorkflowName {
		t.Errorf("expected name %s, got %s", state.WorkflowName, loaded.WorkflowName)
	}
	if loaded.StepIndex != state.StepIndex {
		t.Errorf("expected step index %d, got %d", state.StepIndex, loaded.StepIndex)
	}
}

func TestSaveState_EmptyPath(t *testing.T) {
	err := SaveState("", &WorkflowState{})
	if err != nil {
		t.Error("expected nil error for empty path")
	}
}

func TestLoadState_EmptyPath(t *testing.T) {
	loaded, err := LoadState("")
	if err != nil {
		t.Error("expected nil error for empty path")
	}
	if loaded != nil {
		t.Error("expected nil state for empty path")
	}
}

func TestSaveState_InvalidPath(t *testing.T) {
	err := SaveState("/absolute/path.json", &WorkflowState{})
	if err == nil {
		t.Error("expected error for absolute path")
	}
}

func TestLoadState_InvalidPath(t *testing.T) {
	_, err := LoadState("/absolute/path.json")
	if err == nil {
		t.Error("expected error for absolute path")
	}
}

func TestLoadState_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	_, err := LoadState("missing.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadState_TooLarge(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	huge := make([]byte, MaxFileSize+1)
	os.WriteFile("huge.json", huge, 0600)

	_, err := LoadState("huge.json")
	if err == nil {
		t.Error("expected error for too large file")
	}
}

func TestLoadState_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	os.WriteFile("bad.json", []byte("not json"), 0600)
	_, err := LoadState("bad.json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestValidateStatePath_Empty(t *testing.T) {
	_, err := validateStatePath("")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestValidateStatePath_Absolute(t *testing.T) {
	_, err := validateStatePath("/etc/passwd")
	if err == nil {
		t.Error("expected error for absolute path")
	}
}

func TestValidateStatePath_Traversal(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	_, err := validateStatePath("../escape")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestValidateStatePath_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	// validateStatePath requires the file to exist
	os.WriteFile("state.json", []byte{}, 0600)

	path, err := validateStatePath("state.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(path, "state.json") {
		t.Errorf("expected path to contain state.json, got %s", path)
	}
}

func TestSaveCurrentStateAndRestoreState(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch", "fetched")
	engine.SetVariable("api_key", "secret")

	wf := &Workflow{Name: "test"}
	state := SaveCurrentState(wf, 1, "data", engine)
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if state.StepOutputs[0] != "fetched" {
		t.Errorf("expected step output 'fetched', got %s", state.StepOutputs[0])
	}
	if state.Variables["api_key"] != "secret" {
		t.Errorf("expected variable 'secret', got %s", state.Variables["api_key"])
	}

	newEngine := NewExpressionEngine()
	data := RestoreState(state, newEngine)
	if data != "data" {
		t.Errorf("expected data 'data', got %s", data)
	}
	val, ok := newEngine.GetVariable("api_key")
	if !ok || val != "secret" {
		t.Error("expected restored variable")
	}
}

// ── ConcurrencyLimiter tests ──

func TestConcurrencyLimiter_AcquireRelease(t *testing.T) {
	limiter := NewConcurrencyLimiter(1)
	ctx := context.Background()

	err := limiter.Acquire(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	limiter.Release()
}

func TestConcurrencyLimiter_Nil(t *testing.T) {
	var limiter *ConcurrencyLimiter
	err := limiter.Acquire(context.Background())
	if err != nil {
		t.Error("expected nil limiter to not error")
	}
	limiter.Release() // should not panic
}

func TestConcurrencyLimiter_ContextCancel(t *testing.T) {
	limiter := NewConcurrencyLimiter(1)
	ctx, cancel := context.WithCancel(context.Background())

	limiter.Acquire(ctx)
	cancel()

	err := limiter.Acquire(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	limiter.Release()
}

func TestConcurrencyLimiter_ZeroMax(t *testing.T) {
	limiter := NewConcurrencyLimiter(0)
	if limiter != nil {
		t.Error("expected nil limiter for max <= 0")
	}
}

// ── WorkflowTimeout test ──

type slowNode struct{}

func (n *slowNode) Name() string        { return "slow" }
func (n *slowNode) Description() string { return "slow node" }
func (n *slowNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: "slow", Input: "string", Output: "string"}
}
func (n *slowNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	select {
	case <-time.After(500 * time.Millisecond):
		return "done", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func TestWorkflowTimeout(t *testing.T) {
	// Configure the timeout per-Executor instead of mutating the package-level
	// WorkflowTimeout global, which would race with parallel tests.
	exec := NewExecutor().WithTimeout(50 * time.Millisecond)

	reg := nodes.NewRegistry()
	reg.Register(&slowNode{})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{Node: "slow"},
		},
	}

	_, _, err := exec.Execute(context.Background(), wf, reg)
	if err == nil {
		t.Error("expected timeout error")
	}
}

// ── MaxSteps test ──

func TestMaxStepsExceeded(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	steps := make([]WorkflowStep, MaxSteps+1)
	for i := range steps {
		steps[i] = WorkflowStep{Node: "test"}
	}
	wf := &Workflow{Steps: steps}

	_, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Error("expected error for max steps exceeded")
	}
}

// ── validateInputSchema test ──

func TestValidateInputSchema(t *testing.T) {
	wf := &Workflow{
		InputSchema: []InputField{
			{Name: "query", Type: "string", Required: true},
		},
	}
	err := validateInputSchema(wf)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	wf2 := &Workflow{}
	err = validateInputSchema(wf2)
	if err != nil {
		t.Errorf("expected no error for empty schema, got %v", err)
	}
}

// ── Parser tests ──

func TestParseWorkflow_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "workflow.yaml")
	content := `name: test workflow
steps:
  - node: test
    params:
      prefix: hello
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	wf, err := ParseWorkflow(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Name != "test workflow" {
		t.Errorf("expected name 'test workflow', got %s", wf.Name)
	}
	if len(wf.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(wf.Steps))
	}
}

func TestParseWorkflow_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.yaml")
	if err := os.WriteFile(path, []byte("not: [ valid yaml :::"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := ParseWorkflow(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParseWorkflow_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "dir.yaml")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	_, err := ParseWorkflow(path)
	if err == nil {
		t.Error("expected error for directory")
	}
}

func TestParseWorkflow_TooLarge(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "huge.yaml")
	huge := make([]byte, MaxFileSize+1)
	for i := range huge {
		huge[i] = ' '
	}
	content := append([]byte("name: x\nsteps:\n"), huge...)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := ParseWorkflow(path)
	if err == nil {
		t.Error("expected error for too large file")
	}
}

func TestSafeWorkflowPath_Empty(t *testing.T) {
	_, err := safeWorkflowPath("")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestSafeWorkflowPath_InvalidExt(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "workflow.txt")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	_, err := safeWorkflowPath(path)
	if err == nil {
		t.Error("expected error for non-yaml extension")
	}
}

func TestSafeWorkflowPath_Symlink(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "real.yaml")
	link := filepath.Join(tmpDir, "link.yaml")
	os.WriteFile(target, []byte("x"), 0644)
	os.Symlink(target, link)

	result, err := safeWorkflowPath(link)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "real.yaml") {
		t.Errorf("expected resolved path, got %s", result)
	}
}

func TestSafeWorkflowPath_SymlinkToNonYaml(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "real.txt")
	link := filepath.Join(tmpDir, "link.yaml")
	os.WriteFile(target, []byte("x"), 0644)
	os.Symlink(target, link)

	_, err := safeWorkflowPath(link)
	if err == nil {
		t.Error("expected error for symlink to non-yaml")
	}
}

// ── util.go test ──

func TestPseudoRand(t *testing.T) {
	r1 := pseudoRand()
	r2 := pseudoRand()
	if r1 < 0 || r1 >= 1 {
		t.Errorf("expected r1 in [0,1), got %f", r1)
	}
	if r2 < 0 || r2 >= 1 {
		t.Errorf("expected r2 in [0,1), got %f", r2)
	}
}

// ── types.go tests ──

func TestGetBackoffDelay(t *testing.T) {
	step := &WorkflowStep{
		Delay: "1s",
		Backoff: &BackoffConfig{
			Exponential: true,
			Base:        "100ms",
			MaxDelay:    "500ms",
		},
	}

	// attempt 1 returns base retry delay (1s), custom base only used for attempt > 1
	d1 := step.GetBackoffDelay(1)
	if d1 != 1*time.Second {
		t.Errorf("expected 1s for attempt 1, got %v", d1)
	}

	d2 := step.GetBackoffDelay(2)
	if d2 != 200*time.Millisecond {
		t.Errorf("expected 200ms for attempt 2, got %v", d2)
	}

	d3 := step.GetBackoffDelay(3)
	if d3 != 400*time.Millisecond {
		t.Errorf("expected 400ms for attempt 3, got %v", d3)
	}
}

func TestGetBackoffDelay_Jitter(t *testing.T) {
	step := &WorkflowStep{
		Delay: "1s",
		Backoff: &BackoffConfig{
			Exponential: true,
			Base:        "100ms",
			Jitter:      true,
		},
	}

	// attempt 2 with jitter: base 100ms * 2 = 200ms, then * (0.75 .. 1.0)
	d := step.GetBackoffDelay(2)
	if d < 150*time.Millisecond || d > 200*time.Millisecond {
		t.Errorf("expected jitter in (150ms, 200ms], got %v", d)
	}
}

func TestGetBackoffDelay_NoExponential(t *testing.T) {
	step := &WorkflowStep{
		Delay: "2s",
	}
	d := step.GetBackoffDelay(2)
	if d != 2*time.Second {
		t.Errorf("expected 2s, got %v", d)
	}
}

func TestGetBackoffDelay_OverflowCap(t *testing.T) {
	step := &WorkflowStep{
		Delay: "1s",
		Backoff: &BackoffConfig{
			Exponential: true,
			MaxDelay:    "2s",
		},
	}
	d := step.GetBackoffDelay(10)
	if d != 2*time.Second {
		t.Errorf("expected 2s cap, got %v", d)
	}
}

func TestGetRetryDelay_Invalid(t *testing.T) {
	step := &WorkflowStep{Delay: "invalid"}
	d := step.GetRetryDelay()
	if d != 1*time.Second {
		t.Errorf("expected 1s default, got %v", d)
	}
}

func TestGetRetryDelay_StepInvalid(t *testing.T) {
	s := &Step{Delay: "invalid"}
	d := s.GetRetryDelay()
	if d != 1*time.Second {
		t.Errorf("expected 1s default, got %v", d)
	}
}

func TestGetTimeout_Invalid(t *testing.T) {
	step := &WorkflowStep{
		Params: map[string]string{"_timeout": "invalid"},
	}
	d := step.GetTimeout()
	if d != 0 {
		t.Errorf("expected 0 for invalid timeout, got %v", d)
	}
}

func TestStepGetTimeout(t *testing.T) {
	s := &Step{
		Params: map[string]string{"_timeout": "5s"},
	}
	if s.GetTimeout() != 5*time.Second {
		t.Errorf("expected 5s, got %v", s.GetTimeout())
	}
}

func TestStepGetTimeout_Invalid(t *testing.T) {
	s := &Step{
		Params: map[string]string{"_timeout": "bad"},
	}
	if s.GetTimeout() != 0 {
		t.Errorf("expected 0, got %v", s.GetTimeout())
	}
}

func TestStepGetRetryCount_Negative(t *testing.T) {
	s := &Step{Retry: -1}
	if s.GetRetryCount() != 0 {
		t.Errorf("expected 0, got %d", s.GetRetryCount())
	}
}

func TestWorkflowStepGetRetryCount_Negative(t *testing.T) {
	step := &WorkflowStep{Retry: -1}
	if step.GetRetryCount() != 0 {
		t.Errorf("expected 0, got %d", step.GetRetryCount())
	}
}

// ── init() / ExecuteWorkflowFunc tests ──

func TestExecuteWorkflowFunc_WithWorkflow(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	wf := &Workflow{
		Steps: []WorkflowStep{{Node: "test"}},
	}

	result, stepResults, err := nodes.ExecuteWorkflowFunc(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "processed: " {
		t.Errorf("unexpected result: %s", result)
	}
	if len(stepResults) != 1 {
		t.Errorf("expected 1 result, got %d", len(stepResults))
	}
}

func TestExecuteWorkflowFunc_WithString(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "test"})

	yamlStr := "name: test\nsteps:\n  - node: test\n"
	result, _, err := nodes.ExecuteWorkflowFunc(context.Background(), yamlStr, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "processed: " {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestExecuteWorkflowFunc_StringInvalidYAML(t *testing.T) {
	reg := nodes.NewRegistry()
	_, _, err := nodes.ExecuteWorkflowFunc(context.Background(), "not: [ valid", reg)
	if err == nil {
		t.Error("expected error for invalid YAML string")
	}
}

func TestExecuteWorkflowFunc_StringTooLarge(t *testing.T) {
	reg := nodes.NewRegistry()
	huge := make([]byte, MaxFileSize+1)
	_, _, err := nodes.ExecuteWorkflowFunc(context.Background(), string(huge), reg)
	if err == nil {
		t.Error("expected error for too large string")
	}
}

func TestExecuteWorkflowFunc_UnsupportedType(t *testing.T) {
	reg := nodes.NewRegistry()
	_, _, err := nodes.ExecuteWorkflowFunc(context.Background(), 123, reg)
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}

// ── executeWithRetry limit tests (via types) ──

func TestBackoffConfig_MaxDelayCap(t *testing.T) {
	step := &WorkflowStep{
		Delay: "1s",
		Backoff: &BackoffConfig{
			Exponential: true,
			Base:        "1s",
			MaxDelay:    "3s",
		},
	}
	// attempt 3: 1s -> 2s -> 4s, but capped at 3s
	d := step.GetBackoffDelay(3)
	if d != 3*time.Second {
		t.Errorf("expected 3s cap, got %v", d)
	}
}

func TestBackoffConfig_InvalidBase(t *testing.T) {
	step := &WorkflowStep{
		Delay: "1s",
		Backoff: &BackoffConfig{
			Exponential: true,
			Base:        "invalid",
		},
	}
	d := step.GetBackoffDelay(2)
	if d != 2*time.Second {
		t.Errorf("expected 2s (from delay), got %v", d)
	}
}

func TestBackoffConfig_InvalidMaxDelay(t *testing.T) {
	step := &WorkflowStep{
		Delay: "1s",
		Backoff: &BackoffConfig{
			Exponential: true,
			MaxDelay:    "invalid",
		},
	}
	d := step.GetBackoffDelay(100)
	if d != MaxRetryDelay {
		t.Errorf("expected MaxRetryDelay, got %v", d)
	}
}

// ── streamSink tests ──

// TestStreamSinkOnChunkFillsBuffer verifies onChunk writes succeed without
// drops while the buffer has capacity. The forwarding goroutine is NOT started
// so the channel is not drained, isolating the write/drop logic from any
// program.Send interaction.
func TestStreamSinkOnChunkFillsBuffer(t *testing.T) {
	s := &streamSink{
		idx:      7,
		nodeName: "test-node",
		ch:       make(chan string, streamChunkBufferSize),
		done:     make(chan struct{}),
	}
	for i := 0; i < streamChunkBufferSize; i++ {
		s.onChunk(fmt.Sprintf("chunk-%d", i))
	}
	if dropped := s.dropped.Load(); dropped != 0 {
		t.Errorf("expected 0 drops when buffer has capacity, got %d", dropped)
	}
	if len(s.ch) != streamChunkBufferSize {
		t.Errorf("expected buffer to be full (%d items), got %d", streamChunkBufferSize, len(s.ch))
	}
	close(s.ch)
}

// TestStreamSinkOnChunkDropsOldest verifies that writing past the buffer
// capacity drops the oldest pending chunk and increments the drop counter.
func TestStreamSinkOnChunkDropsOldest(t *testing.T) {
	s := &streamSink{
		idx:      3,
		nodeName: "slow-node",
		ch:       make(chan string, streamChunkBufferSize),
		done:     make(chan struct{}),
	}
	// Fill the buffer exactly.
	for i := 0; i < streamChunkBufferSize; i++ {
		s.onChunk(fmt.Sprintf("chunk-%d", i))
	}
	// Write K more; each should drop the oldest and append the new one.
	extra := 5
	for i := 0; i < extra; i++ {
		s.onChunk(fmt.Sprintf("extra-%d", i))
	}
	if dropped := s.dropped.Load(); dropped != int64(extra) {
		t.Errorf("expected %d drops, got %d", extra, dropped)
	}
	if len(s.ch) != streamChunkBufferSize {
		t.Errorf("expected buffer to still be full (%d items), got %d", streamChunkBufferSize, len(s.ch))
	}
	// The oldest `extra` chunks (chunk-0 .. chunk-4) should have been dropped;
	// the first item in the channel should now be chunk-5.
	first := <-s.ch
	want := "chunk-5"
	if first != want {
		t.Errorf("expected first item to be %s (oldest %d dropped), got %s", want, extra, first)
	}
	// The last item should be the most recent extra chunk.
	remaining := streamChunkBufferSize - 1
	for i := 0; i < remaining-1; i++ {
		<-s.ch
	}
	last := <-s.ch
	if last != "extra-4" {
		t.Errorf("expected last item to be extra-4, got %s", last)
	}
	close(s.ch)
}

// streamProbeModel is a minimal bubbletea model used to verify a tea.Program
// is actively consuming messages. It signals receipt of the first message via
// the ready channel so tests can safely proceed to call program.Send-backed
// code without deadlocking.
type streamProbeModel struct {
	ready chan struct{}
}

func (m streamProbeModel) Init() tea.Cmd { return nil }
func (m streamProbeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	select {
	case m.ready <- struct{}{}:
	default:
	}
	return m, nil
}
func (streamProbeModel) View() string { return "" }

// streamProbeMsg is an arbitrary message type used to probe whether Run() has
// entered its event loop.
type streamProbeMsg struct{}

// TestStreamSinkFlushDrainsAndExits verifies that flush closes the chunk
// channel, drains queued chunks through program.Send, and lets the forwarding
// goroutine exit cleanly (the done channel is closed).
func TestStreamSinkFlushDrainsAndExits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan struct{}, 1)
	program := tea.NewProgram(
		streamProbeModel{ready: ready},
		tea.WithContext(ctx),
		tea.WithoutRenderer(),
		tea.WithInput(nil),
		tea.WithoutSignalHandler(),
	)
	go func() {
		_, _ = program.Run()
	}()

	// Probe: wait until Run() is actively consuming messages so program.Send
	// inside the sink's forwarding goroutine does not block forever.
	program.Send(streamProbeMsg{})
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("program.Run() did not start consuming within 2s")
	}

	sink := newStreamSink(program, 0, "test-node")
	for i := 0; i < 10; i++ {
		sink.onChunk(fmt.Sprintf("chunk-%d", i))
	}
	sink.flush()

	select {
	case <-sink.done:
		// good: forwarding goroutine exited
	default:
		t.Error("expected done channel to be closed after flush")
	}
	if dropped := sink.dropped.Load(); dropped != 0 {
		t.Errorf("expected 0 drops with a consuming program, got %d", dropped)
	}
}
