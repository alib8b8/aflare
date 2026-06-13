package nodes

import (
	"context"
	"fmt"
	"strings"
)

type TransformNode struct{}

func init() {
	Register(&TransformNode{})
}

func (n *TransformNode) Name() string {
	return "transform"
}

func (n *TransformNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	operation, ok := params["operation"]
	if !ok || operation == "" {
		return input, nil
	}

	switch strings.ToLower(operation) {
	case "upper":
		return strings.ToUpper(input), nil
	case "lower":
		return strings.ToLower(input), nil
	case "trim":
		return strings.TrimSpace(input), nil
	case "lines":
		lines := strings.Split(input, "\n")
		return fmt.Sprintf("%d lines", len(lines)), nil
	case "words":
		words := strings.Fields(input)
		return fmt.Sprintf("%d words", len(words)), nil
	case "chars":
		return fmt.Sprintf("%d characters", len(input)), nil
	case "first_line":
		lines := strings.SplitN(input, "\n", 2)
		if len(lines) > 0 {
			return lines[0], nil
		}
		return "", nil
	case "first_500":
		if len(input) > 500 {
			return input[:500] + "...", nil
		}
		return input, nil
	case "first_1000":
		if len(input) > 1000 {
			return input[:1000] + "...", nil
		}
		return input, nil
	case "summary":
		if len(input) > 200 {
			return input[:200] + "...", nil
		}
		return input, nil
	default:
		return input, nil
	}
}
