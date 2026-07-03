package nodes

import (
	"context"
	"fmt"
	"strings"
)

type CombineNode struct{}

func init() {
	Register(&CombineNode{})
}

func (n *CombineNode) Name() string {
	return "combine"
}

func (n *CombineNode) Description() string {
	return "Combine multiple inputs into one"
}

func (n *CombineNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "combine",
		Description: "Combine multiple inputs into one",
		Input:       "string - input text to format",
		Output:      "string - formatted output",
		Params: []ParamSchema{
			{Name: "format", Type: "string", Description: "Output format: text, markdown, csv, json (default: text)", Required: false, Default: "text"},
		},
	}
}

func (n *CombineNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	format, ok := params["format"]
	if !ok || format == "" {
		format = "text"
	}

	switch strings.ToLower(format) {
	case "markdown":
		lines := strings.Split(input, "\n")
		var result string
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				result += "- " + line + "\n"
			}
		}
		return result, nil
	case "csv":
		lines := strings.Split(strings.TrimSpace(input), "\n")
		var rows [][]string
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				rows = append(rows, strings.Fields(line))
			}
		}
		var csv string
		for _, row := range rows {
			csv += strings.Join(row, ",") + "\n"
		}
		return csv, nil
	case "json":
		escaped := strings.ReplaceAll(input, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		escaped = strings.ReplaceAll(escaped, "\n", "\\n")
		escaped = strings.ReplaceAll(escaped, "\r", "\\r")
		escaped = strings.ReplaceAll(escaped, "\t", "\\t")
		return fmt.Sprintf("{\n  \"data\": \"%s\"\n}", escaped), nil
	case "text":
		return input, nil
	default:
		return input, nil
	}
}
