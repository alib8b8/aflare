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

package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// MaxCallDepth limits recursive workflow calls to prevent stack overflow
const MaxCallDepth = 10

// callDepthKey is used to propagate the recursion depth through the context
// chain. Unlike a per-ctx map, this survives context derivation
// (WithTimeout/WithCancel) because derived contexts inherit parent values.
type callDepthKeyType struct{}

var callDepthKey = callDepthKeyType{}

var ExecuteWorkflowFunc func(ctx context.Context, wf interface{}, reg *Registry) (string, []interface{}, error)

type CallNode struct{}

func init() {
	Register(&CallNode{})
}

func (n *CallNode) Name() string {
	return "call"
}

func (n *CallNode) Description() string {
	return "Call another workflow"
}

func (n *CallNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "call",
		Description: "Call another workflow file",
		Input:       "string - input data to pass to the called workflow",
		Output:      "string - output from the called workflow",
		Params: []ParamSchema{
			{Name: "workflow", Type: "string", Description: "Path to the workflow file to call", Required: true},
			{Name: "vars", Type: "string", Description: "JSON or key=value pairs to pass as workflow variables (e.g. topic=AI,model=gpt-4)"},
		},
	}
}

func (n *CallNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if ExecuteWorkflowFunc == nil {
		return "", fmt.Errorf("workflow execution function not registered")
	}

	workflowPath, ok := params["workflow"]
	if !ok || workflowPath == "" {
		return "", fmt.Errorf("workflow parameter is required")
	}

	// Validate path to prevent arbitrary file read
	safePath, err := validateReadPath(workflowPath)
	if err != nil {
		if filepath.IsAbs(workflowPath) {
			return "", fmt.Errorf("absolute paths not allowed for workflow calls, use relative paths: %w", err)
		}
		return "", fmt.Errorf("invalid workflow path: %w", err)
	}
	workflowPath = safePath

	// Check recursion depth to prevent stack overflow from circular calls.
	depth := 0
	if v, ok := ctx.Value(callDepthKey).(int); ok {
		depth = v
	}
	if depth >= MaxCallDepth {
		return "", fmt.Errorf("maximum workflow call depth (%d) exceeded - possible circular call detected", MaxCallDepth)
	}
	childCtx := context.WithValue(ctx, callDepthKey, depth+1)

	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return "", fmt.Errorf("failed to read workflow file %q: %w", workflowPath, err)
	}

	// Limit workflow file size to prevent DoS
	if len(data) > 10*1024*1024 {
		return "", fmt.Errorf("workflow file too large (max 10MB)")
	}

	// If vars parameter is provided, inject them into the workflow YAML
	workflowContent := string(data)
	if varsParam, hasVars := params["vars"]; hasVars && varsParam != "" {
		injectedVars := parseVarsParam(varsParam)
		if len(injectedVars) > 0 {
			workflowContent, err = injectVarsIntoWorkflow(workflowContent, injectedVars)
			if err != nil {
				return "", fmt.Errorf("failed to inject vars into workflow: %w", err)
			}
		}
	}

	result, _, err := ExecuteWorkflowFunc(childCtx, workflowContent, GetGlobalRegistry())
	if err != nil {
		return "", fmt.Errorf("failed to execute workflow %q: %w", workflowPath, err)
	}

	return result, nil
}

// parseVarsParam parses the vars parameter string.
// Supports two formats:
// 1. JSON: {"key":"value","key2":"value2"}
// 2. Key=value pairs: key=value,key2=value2
func parseVarsParam(s string) map[string]string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Security: limit input size
	if len(s) > 1*1024*1024 {
		return nil
	}

	// Try JSON first (safe - no YAML anchor/billion laughs risk)
	if strings.HasPrefix(s, "{") {
		var m map[string]string
		if err := json.Unmarshal([]byte(s), &m); err == nil && len(m) > 0 {
			return m
		}
	}

	// Fall back to key=value pairs
	result := make(map[string]string)
	parts := strings.Split(s, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])
			if key != "" {
				result[key] = val
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// injectVarsIntoWorkflow merges the provided vars into the workflow's existing vars.
func injectVarsIntoWorkflow(content string, newVars map[string]string) (string, error) {
	// Parse the workflow YAML
	var wf map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &wf); err != nil {
		return "", fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

	// Get or create vars section
	existingVars, _ := wf["vars"].(map[string]interface{})
	if existingVars == nil {
		existingVars = make(map[string]interface{})
	}

	// Merge new vars (new vars override existing ones with the same key)
	for k, v := range newVars {
		existingVars[k] = v
	}
	wf["vars"] = existingVars

	// Re-marshal
	out, err := yaml.Marshal(wf)
	if err != nil {
		return "", fmt.Errorf("failed to re-marshal workflow: %w", err)
	}
	return string(out), nil
}
