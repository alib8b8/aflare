package nodes

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

type ConditionNode struct{}

func init() {
	Register(&ConditionNode{})
}

func (n *ConditionNode) Name() string {
	return "condition"
}

// Execute evaluates a condition expression against the input.
// Supports simple patterns:
//   - "contains:keyword"  - true if input contains keyword
//   - "equals:value"      - true if input equals value
//   - "starts_with:prefix" - true if input starts with prefix
//   - "ends_with:suffix"   - true if input ends with suffix
//   - "regex:pattern"      - true if input matches regex
//   - "empty"              - true if input is empty
//   - "not_empty"          - true if input is not empty
//   - "true" / "false"     - literal
//
// When condition is true, returns "true"; when false, returns "false".
// Step-level conditional logic uses the skip_if/only_if metadata fields.
func (n *ConditionNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	expr, ok := params["expr"]
	if !ok || expr == "" {
		expr, ok = params["condition"]
		if !ok || expr == "" {
			return "", fmt.Errorf("expr or condition parameter is required")
		}
	}

	result, err := evaluateCondition(expr, input)
	if err != nil {
		return "", fmt.Errorf("condition evaluation failed: %w", err)
	}

	if result {
		return "true", nil
	}
	return "false", nil
}

func evaluateCondition(expr, input string) (bool, error) {
	// Trim whitespace
	expr = strings.TrimSpace(expr)
	input = strings.TrimSpace(input)

	// Check for "not_" prefix
	negate := false
	if strings.HasPrefix(expr, "not ") {
		negate = true
		expr = strings.TrimSpace(expr[4:])
	}

	result, err := evalPositive(expr, input)
	if err != nil {
		return false, err
	}

	if negate {
		return !result, nil
	}
	return result, nil
}

func evalPositive(expr, input string) (bool, error) {
	if expr == "empty" {
		return input == "", nil
	}
	if expr == "not_empty" {
		return input != "", nil
	}
	if expr == "true" {
		return true, nil
	}
	if expr == "false" {
		return false, nil
	}

	colonIdx := strings.Index(expr, ":")
	if colonIdx < 0 {
		return false, fmt.Errorf("invalid condition format: %s", expr)
	}

	op := expr[:colonIdx]
	value := expr[colonIdx+1:]

	switch op {
	case "contains":
		return strings.Contains(input, value), nil
	case "equals":
		return input == value, nil
	case "starts_with":
		return strings.HasPrefix(input, value), nil
	case "ends_with":
		return strings.HasSuffix(input, value), nil
	case "regex":
		matched, err := regexp.MatchString(value, input)
		if err != nil {
			return false, fmt.Errorf("invalid regex %q: %w", value, err)
		}
		return matched, nil
	default:
		return false, fmt.Errorf("unsupported operator: %s", op)
	}
}
