// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​​‌‌​​‌​​‌​‌​​‌‌​‌​‌​‌​‌‌‌​​‌‌‌‌​‌​​​​‌‌​‌​​‌​‌​​​​​​​​​​​​​​​​​​‌​​​‌‌‌‌‌​​​‌​⁠
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

package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/workflow"
	"gopkg.in/yaml.v3"
)

// ------------------------------------------------------------------
// Workflow tool implementations
// ------------------------------------------------------------------

func (s *Server) toolWorkflowRun(args map[string]interface{}) (*toolCallResult, error) {
	file, err := requireString(args, "file")
	if err != nil {
		return nil, err
	}

	timeoutSec := optionalInt(args, "timeout_seconds", 30)
	if timeoutSec < 1 {
		timeoutSec = 1
	}
	if timeoutSec > 300 {
		timeoutSec = 300
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	wf, err := workflow.ParseWorkflow(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	reg := nodes.GetGlobalRegistry()
	nodes.RegisterBuiltins(reg)

	result, _, err := workflow.ExecuteWorkflow(ctx, wf, reg)
	if err != nil {
		return nil, fmt.Errorf("workflow execution failed: %w", err)
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: result}},
	}, nil
}

func (s *Server) toolWorkflowCreate(args map[string]interface{}) (*toolCallResult, error) {
	desc, err := requireString(args, "description")
	if err != nil {
		return nil, err
	}

	wf, err := workflow.GenerateWorkflow(desc)
	if err != nil {
		return nil, fmt.Errorf("failed to generate workflow: %w", err)
	}

	if name := optionalString(args, "name"); name != "" {
		wf.Name = name
	}

	yamlBytes, err := yaml.Marshal(wf)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow: %w", err)
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: string(yamlBytes)}},
	}, nil
}

func (s *Server) toolWorkflowList(args map[string]interface{}) (*toolCallResult, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	dir := optionalString(args, "directory")
	if dir == "" {
		dir = cwd
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("invalid directory: %w", err)
	}

	// Restrict directory listing to the current working directory tree.
	// Without this, an MCP client (or a compromised LLM tool call) could
	// enumerate arbitrary directories (e.g. "/etc", "/home/user/.ssh")
	// via the "directory" argument — a directory-listing / information
	// disclosure issue. Reject null bytes and any path that resolves
	// outside cwd.
	if strings.ContainsRune(dir, '\x00') {
		return nil, fmt.Errorf("invalid directory")
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve working directory: %w", err)
	}
	rel, err := filepath.Rel(absCwd, absDir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("directory must be within the current working directory")
	}

	entries, err := os.ReadDir(absDir) // #nosec G304 -- absDir validated to be within cwd
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, name)
		}
	}

	if len(files) == 0 {
		return &toolCallResult{
			Content: []content{{Type: "text", Text: "No workflow files found in " + absDir}},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Workflow files in %s:\n\n", absDir))
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("  - %s\n", f))
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: sb.String()}},
	}, nil
}

func (s *Server) toolWorkflowValidate(args map[string]interface{}) (*toolCallResult, error) {
	file := optionalString(args, "file")
	yamlStr := optionalString(args, "yaml")

	var wf *workflow.Workflow
	var err error

	switch {
	case file != "":
		wf, err = workflow.ParseWorkflow(file)
		if err != nil {
			return nil, fmt.Errorf("failed to parse workflow file: %w", err)
		}
	case yamlStr != "":
		if len(yamlStr) > workflow.MaxFileSize {
			return nil, fmt.Errorf("workflow YAML too large (max %d bytes)", workflow.MaxFileSize)
		}
		wf = &workflow.Workflow{}
		if err := yaml.Unmarshal([]byte(yamlStr), wf); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
	default:
		return nil, fmt.Errorf("either 'file' or 'yaml' parameter is required")
	}

	warnings := workflow.ValidateWorkflow(wf)

	reg := nodes.GetGlobalRegistry()
	nodes.RegisterBuiltins(reg)

	for i, step := range wf.Steps {
		// Compound steps (if/loop/map/reduce/parallel/saga/capture_error) have no
		// node of their own; skip the node-existence check for them.
		if step.IsIf() || step.IsLoop() || step.IsMap() || step.IsReduce() || step.IsParallel() || step.IsSaga() || step.HasCaptureError() {
			continue
		}
		if _, ok := reg.Get(step.Node); !ok {
			warnings = append(warnings, fmt.Sprintf("Step %d: unknown node '%s'", i+1, step.Node))
		}
	}

	var sb strings.Builder
	if len(warnings) == 0 {
		sb.WriteString("Workflow is valid. No issues found.")
	} else {
		sb.WriteString("Validation warnings:\n")
		for _, w := range warnings {
			sb.WriteString(fmt.Sprintf("  - %s\n", w))
		}
	}

	return &toolCallResult{
		Content: []content{{Type: "text", Text: sb.String()}},
	}, nil
}
