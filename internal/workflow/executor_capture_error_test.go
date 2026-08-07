// Copyright (c) 2026 aflare Contributors
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
	"encoding/json"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes"
)

// TestCaptureError_BasicRecovery verifies that a failing step with a
// capture_error branch continues the workflow with the branch's output
// instead of aborting. The error message flows in as the branch input.
func TestCaptureError_BasicRecovery(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&errorNode{name: "fail"})
	reg.Register(&testNode{name: "handler"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Node: "fail",
				CaptureError: []WorkflowStep{
					// testNode returns prefix + " " + input; input is the
					// error message ("test error").
					{Node: "handler", Params: map[string]string{"prefix": "handled"}},
				},
			},
			// A second step proves the workflow continued past the failure.
			{Node: "handler", Params: map[string]string{"prefix": "next"}},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("capture_error should have prevented workflow failure: %v", err)
	}
	// Step 1 recovered via capture_error: output = "handled test error".
	// Step 2 takes that as input: "next handled test error".
	if output != "next handled test error" {
		t.Errorf("expected 'next handled test error', got %q", output)
	}
	if len(results) < 2 {
		t.Fatalf("expected >=2 results, got %d", len(results))
	}
	// The failed step's result must be recovered (no propagated error).
	if results[0].Error != nil {
		t.Errorf("step 0 should be recovered, got error: %v", results[0].Error)
	}
	if results[0].Output != "handled test error" {
		t.Errorf("step 0 output should be branch output, got %q", results[0].Output)
	}
}

// TestCaptureError_BranchesOnErrorType verifies the key "error as a branch
// condition" behavior: the capture_error branch inspects the error message
// via a condition and routes to different handlers. This is the contrast to
// continue_on_error (which swallows the error with an empty output and
// cannot branch on it).
func TestCaptureError_BranchesOnErrorType(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&errorNode{name: "fail"})
	reg.Register(&testNode{name: "match"})
	reg.Register(&testNode{name: "nomatch"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Node: "fail",
				CaptureError: []WorkflowStep{
					// Branch on whether the error text contains "test".
					{
						If: &IfConfig{
							Condition: "contains:test",
							Then: []WorkflowStep{
								{Node: "match", Params: map[string]string{"prefix": "MATCH"}},
							},
							Else: []WorkflowStep{
								{Node: "nomatch", Params: map[string]string{"prefix": "NOMATCH"}},
							},
						},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("capture_error should have prevented failure: %v", err)
	}
	// errorNode returns error "test error", which contains "test" → MATCH branch.
	// testNode: "MATCH test error".
	if output != "MATCH test error" {
		t.Errorf("expected 'MATCH test error', got %q", output)
	}
}

// TestCaptureError_NoBranchErrors verifies that without capture_error (or any
// recovery), a step failure still aborts the workflow — i.e. capture_error is
// opt-in and the default fail-fast behavior is preserved.
func TestCaptureError_NoBranchErrors(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&errorNode{name: "fail"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{Node: "fail"},
		},
	}

	_, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err == nil {
		t.Fatal("expected error without capture_error, got nil")
	}
}

// TestCaptureError_InDAG verifies capture_error works under DAG scheduling
// (triggered by declaring depends_on). The branch runs via applyErrorRecovery.
func TestCaptureError_InDAG(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&errorNode{name: "fail"})
	reg.Register(&testNode{name: "handler"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "s1",
				Node: "fail",
				CaptureError: []WorkflowStep{
					{Node: "handler", Params: map[string]string{"prefix": "dag-handled"}},
				},
			},
			{
				Name:      "s2",
				Node:      "handler",
				DependsOn: []string{"s1"},
				Params:    map[string]string{"prefix": "after"},
			},
		},
	}

	output, results, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("capture_error should have prevented DAG failure: %v", err)
	}
	// s1 recovered: "dag-handled test error"; s2 takes it as input.
	if output != "after dag-handled test error" {
		t.Errorf("expected 'after dag-handled test error', got %q", output)
	}
	if len(results) < 2 {
		t.Fatalf("expected >=2 results, got %d", len(results))
	}
	if results[0].Error != nil {
		t.Errorf("s1 should be recovered in DAG, got error: %v", results[0].Error)
	}
}

// TestCaptureError_InsideMap verifies capture_error works for a sub-step
// inside a map iteration: the failed item is recovered via its branch and
// contributes the branch output to the map's array, while non-failing items
// pass through untouched.
func TestCaptureError_InsideMap(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&failOnNode{name: "conditional"})
	reg.Register(&testNode{name: "handler"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Name: "batch",
				Map: &MapConfig{
					// failOnNode fails when input contains "fail"; otherwise
					// returns "ok: <item>". This routes the item directly into
					// the node (no if-branch) so the failing item alone triggers
					// capture_error recovery.
					Over: "good\nbadfail\nalso-good",
					Steps: []WorkflowStep{
						{
							Node: "conditional",
							CaptureError: []WorkflowStep{
								{Node: "handler", Params: map[string]string{"prefix": "recovered"}},
							},
						},
					},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("map with capture_error should not fail: %v", err)
	}
	// Items:
	//   "good"      → ok → "ok: good"
	//   "badfail"   → fail → capture_error → "recovered intentional failure for input: badfail"
	//   "also-good" → ok → "ok: also-good"
	var arr []string
	if err := json.Unmarshal([]byte(output), &arr); err != nil {
		t.Fatalf("map output not json array: %v (output=%s)", err, output)
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 outputs, got %d (%v)", len(arr), arr)
	}
	if arr[0] != "ok: good" {
		t.Errorf("arr[0]=%q, want 'ok: good'", arr[0])
	}
	if arr[1] != "recovered intentional failure for input: badfail" {
		t.Errorf("arr[1]=%q, want 'recovered intentional failure for input: badfail'", arr[1])
	}
	if arr[2] != "ok: also-good" {
		t.Errorf("arr[2]=%q, want 'ok: also-good'", arr[2])
	}
}

// TestCaptureError_TakesPrecedenceOverFallback verifies capture_error is
// checked before fallback in the recovery chain.
func TestCaptureError_TakesPrecedenceOverFallback(t *testing.T) {
	reg := nodes.NewRegistry()
	reg.Register(&errorNode{name: "fail"})
	reg.Register(&testNode{name: "handler"})

	wf := &Workflow{
		Steps: []WorkflowStep{
			{
				Node:     "fail",
				Fallback: "FALLBACK_USED",
				CaptureError: []WorkflowStep{
					{Node: "handler", Params: map[string]string{"prefix": "CAPTURE_USED"}},
				},
			},
		},
	}

	output, _, err := ExecuteWorkflow(context.Background(), wf, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// capture_error wins → "CAPTURE_USED test error", not "FALLBACK_USED".
	if !strings.HasPrefix(output, "CAPTURE_USED") {
		t.Errorf("capture_error should take precedence over fallback, got %q", output)
	}
}
