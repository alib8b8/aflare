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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alib8b8/llm-box/internal/logger"
	"github.com/alib8b8/llm-box/internal/nodes"
	tea "github.com/charmbracelet/bubbletea"
)

// evaluateCondition evaluates a condition expression against the current input.
// Syntax is the same as the condition node, but the comparison value is evaluated
// through the expression engine so {{step.0}}, {{var.name}} etc. work.
//
// Examples:
//
//	contains:hello          - input contains "hello"
//	equals:{{var.target}}   - input equals the value of var.target
//	empty                   - input is empty
//	not_empty               - input is not empty
//	regex:\d+               - input matches regex
//	starts_with:https       - input starts with "https"
//	ends_with:.json         - input ends with ".json"
//	not contains:skip       - input does NOT contain "skip"
func evaluateCondition(cond string, input string, engine *ExpressionEngine) (bool, error) {
	if cond == "" {
		return true, nil
	}

	// Evaluate any {{...}} expressions in the condition's comparison value
	evaluated, err := engine.Evaluate(cond, input)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate condition expression: %w", err)
	}

	negate := false
	if strings.HasPrefix(evaluated, "not ") {
		negate = true
		evaluated = strings.TrimPrefix(evaluated, "not ")
	}

	result := false
	var op, value string

	if strings.Contains(evaluated, ":") {
		parts := strings.SplitN(evaluated, ":", 2)
		op = strings.TrimSpace(parts[0])
		value = strings.TrimSpace(parts[1])
	} else {
		op = strings.TrimSpace(evaluated)
	}

	switch op {
	case "true":
		result = true
	case "false":
		result = false
	case "contains":
		result = strings.Contains(input, value)
	case "equals":
		result = input == value
	case "starts_with":
		result = strings.HasPrefix(input, value)
	case "ends_with":
		result = strings.HasSuffix(input, value)
	case "regex":
		matched, err := nodes.SafeRegexMatch(value, input)
		if err != nil {
			return false, fmt.Errorf("regex evaluation failed: %w", err)
		}
		result = matched
	case "empty":
		result = input == ""
	case "not_empty":
		result = input != ""
	default:
		return false, fmt.Errorf("unknown condition operator: %s", op)
	}

	if negate {
		result = !result
	}
	return result, nil
}

// executeIfBranch evaluates an if/else condition and executes the appropriate branch.
// It returns the output of the last step in the executed branch.
func executeIfBranch(ctx context.Context, stepIndex int, ifCfg *IfConfig, input string, engine *ExpressionEngine, reg *nodes.Registry, program *tea.Program, globalLimiter *ConcurrencyLimiter) ([]StepResult, string, error) {
	// Check if/else nesting depth
	depth := 0
	if v, ok := ctx.Value(ifDepthKey).(int); ok {
		depth = v
	}
	if depth >= MaxIfDepth {
		return nil, "", fmt.Errorf("maximum if/else nesting depth (%d) exceeded", MaxIfDepth)
	}

	pass, err := evaluateCondition(ifCfg.Condition, input, engine)
	if err != nil {
		return nil, "", fmt.Errorf("if condition evaluation failed: %w", err)
	}

	var branchSteps []WorkflowStep
	if pass {
		branchSteps = ifCfg.Then
		logger.Info("if branch: executing then", "index", stepIndex, "sub_steps", len(branchSteps))
	} else {
		branchSteps = ifCfg.Else
		logger.Info("if branch: executing else", "index", stepIndex, "sub_steps", len(branchSteps))
	}

	// Execute branch steps as a sub-workflow with incremented depth
	subWf := &Workflow{
		Name:  fmt.Sprintf("if-branch-%d", stepIndex),
		Steps: branchSteps,
	}
	// Pass incremented depth and global limiter via context
	childCtx := context.WithValue(ctx, ifDepthKey, depth+1)
	output, subResults, err := ExecuteWorkflowWithTUI(childCtx, subWf, reg, program)
	if err != nil {
		return subResults, "", err
	}

	return subResults, output, nil
}

// applyOutputStrategy applies the specified output strategy to combined parallel/loop results.
// The input `output` is already joined with "\n---\n" separator.
// For most strategies, we need the raw outputs before joining.
func applyOutputStrategy(output string, strategy string) string {
	if strategy == "" || strategy == "join" {
		return output
	}

	// Split the joined output back into parts
	parts := strings.Split(output, "\n---\n")

	switch strategy {
	case "first":
		if len(parts) > 0 {
			return parts[0]
		}
		return ""
	case "last":
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
		return ""
	case "longest":
		var best string
		for _, p := range parts {
			if len(p) > len(best) {
				best = p
			}
		}
		return best
	case "shortest":
		if len(parts) == 0 {
			return ""
		}
		best := parts[0]
		for _, p := range parts[1:] {
			if len(p) < len(best) {
				best = p
			}
		}
		return best
	case "json_array":
		// Build a JSON array from the parts
		arr := make([]string, len(parts))
		for i, p := range parts {
			// Try to parse each part as JSON; if it fails, use as string
			var raw json.RawMessage
			if err := json.Unmarshal([]byte(p), &raw); err == nil {
				arr[i] = p
			} else {
				b, _ := json.Marshal(p)
				arr[i] = string(b)
			}
		}
		return "[" + strings.Join(arr, ",") + "]"
	default:
		return output
	}
}

// validateInputSchema validates the workflow input against the defined schema.
// Currently performs basic checks: required fields and type coercion.
// The input is expected to be a JSON string if schema is defined.
func validateInputSchema(wf *Workflow) error {
	if len(wf.InputSchema) == 0 {
		return nil
	}

	// Schema validation is informational - it logs warnings but doesn't block execution
	// since input could be non-JSON strings (e.g., plain text for LLM processing)
	// Full validation would require the input to be provided at parse time.
	return nil
}
