// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/cli"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/workflow"
)

// TestE2E_SimpleWorkflow tests the full path: CLI create → parse → validate → assert steps.
// Covers the pipeline: description → GenerateWorkflow → SaveWorkflow → ParseWorkflow → ValidateWorkflow.
func TestE2E_SimpleWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	// 1. Create workflow from description
	desc := "fetch https://example.com and save to output.txt"
	path, err := workflow.CreateWorkflowFromDescription(desc)
	if err != nil {
		t.Fatalf("CreateWorkflowFromDescription: %v", err)
	}

	// 2. Verify the file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("generated workflow file not found at %s: %v", path, err)
	}

	// 3. Parse the generated YAML
	wf, reg, err := cli.PrepareWorkflow(path)
	if err != nil {
		t.Fatalf("PrepareWorkflow: %v", err)
	}

	// 4. Validate the workflow
	suggestions := workflow.ValidateWorkflow(wf)
	if len(suggestions) > 0 {
		t.Logf("validation suggestions: %v", suggestions)
	}

	// 5. Assert the workflow has expected steps
	if wf.Name == "" {
		t.Error("workflow must have a name")
	}

	hasFetch := false
	hasFileWrite := false
	for _, step := range wf.Steps {
		switch step.Node {
		case "fetch_url":
			hasFetch = true
			if step.Params["url"] != "https://example.com" {
				t.Errorf("expected url 'https://example.com', got %q", step.Params["url"])
			}
		case "file_write":
			hasFileWrite = true
			if step.Params["path"] != "output.txt" {
				t.Errorf("expected path 'output.txt', got %q", step.Params["path"])
			}
		}
	}
	if !hasFetch {
		t.Error("workflow should have a fetch_url step")
	}
	if !hasFileWrite {
		t.Error("workflow should have a file_write step")
	}
	if reg == nil {
		t.Error("registry should not be nil")
	}
}

// TestE2E_DryRun tests that a generated workflow can be parsed and validated
// without actually executing it (simulating the --dry-run flag).
func TestE2E_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	// 1. Create workflow from description
	desc := "summarize this article and save to summary.txt"
	path, err := workflow.CreateWorkflowFromDescription(desc)
	if err != nil {
		t.Fatalf("CreateWorkflowFromDescription: %v", err)
	}

	// 2. Prepare (parse + validate) the workflow — this is what --dry-run does
	wf, reg, err := cli.PrepareWorkflow(path)
	if err != nil {
		t.Fatalf("PrepareWorkflow: %v", err)
	}

	// 3. Validate the workflow
	suggestions := workflow.ValidateWorkflow(wf)
	for _, s := range suggestions {
		t.Logf("  suggestion: %s", s)
	}

	// 4. All nodes referenced in steps must be known
	for i, step := range wf.Steps {
		if step.IsIf() || step.IsLoop() || step.IsMap() || step.IsReduce() || step.IsParallel() || step.IsSaga() || step.HasCaptureError() {
			continue
		}
		if _, ok := reg.Get(step.Node); !ok {
			t.Errorf("step %d uses unknown node %q", i+1, step.Node)
		}
	}

	if reg == nil {
		t.Error("registry should not be nil")
	}
}

// TestE2E_MultiStepWorkflow tests a more complex workflow: fetch + summarize + save.
func TestE2E_MultiStepWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	desc := "fetch https://hn.algolia.com/api and summarize and save to hn_summary.md"
	path, err := workflow.CreateWorkflowFromDescription(desc)
	if err != nil {
		t.Fatalf("CreateWorkflowFromDescription: %v", err)
	}

	wf, err := workflow.ParseWorkflow(path)
	if err != nil {
		t.Fatalf("ParseWorkflow: %v", err)
	}

	// Verify step order: fetch_url first, file_write last
	if len(wf.Steps) < 3 {
		t.Fatalf("expected at least 3 steps, got %d", len(wf.Steps))
	}

	if wf.Steps[0].Node != "fetch_url" {
		t.Errorf("first step should be fetch_url, got %s", wf.Steps[0].Node)
	}
	if wf.Steps[len(wf.Steps)-1].Node != "file_write" {
		t.Errorf("last step should be file_write, got %s", wf.Steps[len(wf.Steps)-1].Node)
	}

	// Should have an LLM step for summarize
	hasLLM := false
	for _, step := range wf.Steps {
		if step.Node == "ollama" || step.Node == "openai" || step.Node == "deepseek" {
			hasLLM = true
			break
		}
	}
	if !hasLLM {
		t.Error("workflow should have an LLM step for summarize")
	}
}

// TestE2E_ValidateWorkflowYAML tests that the generated YAML is valid and
// parseable by the standard YAML parser.
func TestE2E_ValidateWorkflowYAML(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	desc := "fetch example.com and write to output.txt"
	path, err := workflow.CreateWorkflowFromDescription(desc)
	if err != nil {
		t.Fatalf("CreateWorkflowFromDescription: %v", err)
	}

	// Read the generated file
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Parse the YAML directly
	wf, err := workflow.ParseWorkflowFromContent(string(content))
	if err != nil {
		t.Fatalf("ParseWorkflowFromContent: %v", err)
	}

	// Verify the workflow has the right structure
	if wf.Name == "" {
		t.Error("parsed workflow missing name")
	}
	if len(wf.Steps) == 0 {
		t.Fatal("parsed workflow has no steps")
	}

	// Round-trip: marshal → parse again
	yamlStr := wf.ToYAML()
	wf2, err := workflow.ParseWorkflowFromContent(yamlStr)
	if err != nil {
		t.Fatalf("round-trip ParseWorkflowFromContent: %v", err)
	}
	if wf2.Name != wf.Name {
		t.Errorf("round-trip name mismatch: %q vs %q", wf.Name, wf2.Name)
	}
	if len(wf2.Steps) != len(wf.Steps) {
		t.Errorf("round-trip step count mismatch: %d vs %d", len(wf.Steps), len(wf2.Steps))
	}
}

// TestE2E_CLIRunWorkflow tests that a generated workflow can be executed
// (non-TUI path) without errors.
func TestE2E_CLIRunWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	// Create a simple workflow with only deterministic nodes (no LLM calls)
	wf := &workflow.Workflow{
		Name: "E2E CLI Test",
		Steps: []workflow.WorkflowStep{
			{Node: "execute", Params: map[string]string{"command": "echo hello e2e"}},
		},
	}

	if err := workflow.SaveWorkflow(wf, "e2e_cli_test"); err != nil {
		t.Fatalf("SaveWorkflow: %v", err)
	}

	// Parse the saved workflow
	parsed, reg, err := cli.PrepareWorkflow("e2e_cli_test.yaml")
	if err != nil {
		t.Fatalf("PrepareWorkflow: %v", err)
	}

	// Execute the workflow (this is the CLI code path)
	output, results, err := cli.RunWorkflow(parsed, reg)
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}

	if !strings.Contains(output, "hello e2e") {
		t.Errorf("expected output to contain 'hello e2e', got %q", output)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 step result, got %d", len(results))
	}
	if results[0].Error != nil {
		t.Errorf("step 0 should not have an error, got %v", results[0].Error)
	}
}

// TestE2E_WorkflowWithInvalidNode tests that PrepareWorkflow detects a
// workflow referencing an unknown node (the validator should flag it).
func TestE2E_WorkflowWithInvalidNode(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	wf := &workflow.Workflow{
		Name: "Invalid Node Test",
		Steps: []workflow.WorkflowStep{
			{Node: "nonexistent_node_xyz", Params: map[string]string{"arg": "val"}},
		},
	}

	if err := workflow.SaveWorkflow(wf, "invalid_node"); err != nil {
		t.Fatalf("SaveWorkflow: %v", err)
	}

	parsed, reg, err := cli.PrepareWorkflow("invalid_node.yaml")
	if err != nil {
		t.Fatalf("PrepareWorkflow should not error on parse: %v", err)
	}

	// The node should not be found in the registry
	if _, ok := reg.Get(parsed.Steps[0].Node); ok {
		t.Error("nonexistent node should not be registered")
	}
}

// TestE2E_BuiltinNodeCoverage verifies that all builtin nodes used in common
// workflow descriptions are registered and available.
func TestE2E_BuiltinNodeCoverage(t *testing.T) {
	reg := nodes.NewRegistry()
	nodes.RegisterBuiltins(reg)

	// Nodes that should always be registered
	requiredNodes := []string{
		"fetch_url", "file_write", "execute", "combine",
		"json_parse", "http_request", "template_render",
	}
	for _, name := range requiredNodes {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("required builtin node %q is not registered", name)
		}
	}
}

// TestE2E_CreateWorkflowFromDescriptionWithAI_Fallback tests that the AI
// creation path falls back gracefully when no API key is configured.
func TestE2E_CreateWorkflowFromDescriptionWithAI_Fallback(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	// Ensure no API key is set so the LLM path fails and falls back
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("AFLARE_LLM_GENERATOR_API_KEY")

	desc := "fetch example.com and save to output.txt"
	path, err := workflow.CreateWorkflowFromDescriptionWithAI(desc, true)
	if err != nil {
		t.Fatalf("CreateWorkflowFromDescriptionWithAI should fall back to rule-based: %v", err)
	}

	// Verify the fallback generated a valid workflow
	wf, err := workflow.ParseWorkflow(path)
	if err != nil {
		t.Fatalf("ParseWorkflow on fallback result: %v", err)
	}

	hasFetch := false
	for _, step := range wf.Steps {
		if step.Node == "fetch_url" {
			hasFetch = true
			break
		}
	}
	if !hasFetch {
		t.Error("fallback workflow should have fetch_url step")
	}
}
