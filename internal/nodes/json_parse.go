package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type JSONParseNode struct{}

func init() {
	Register(&JSONParseNode{})
}

func (n *JSONParseNode) Name() string {
	return "json_parse"
}

func (n *JSONParseNode) Description() string {
	return "Parse and extract JSON data"
}

func (n *JSONParseNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "json_parse",
		Description: "Parse and extract JSON data",
		Input:       "string - JSON string to parse",
		Output:      "string - extracted value or pretty-printed JSON",
		Params: []ParamSchema{
			{Name: "path", Type: "string", Description: "Dot-notation path to extract (e.g. data.items[0].name). If omitted, pretty-prints entire JSON.", Required: false},
		},
	}
}

func (n *JSONParseNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	path, ok := params["path"]
	if !ok || path == "" {
		pretty, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal JSON: %w", err)
		}
		return string(pretty), nil
	}

	value, err := getJSONValue(data, path)
	if err != nil {
		return "", err
	}

	switch v := value.(type) {
	case string:
		return v, nil
	default:
		pretty, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal value: %w", err)
		}
		return string(pretty), nil
	}
}

func getJSONValue(data interface{}, path string) (interface{}, error) {
	keys := strings.Split(path, ".")
	current := data

	for _, key := range keys {
		if key == "" {
			continue
		}

		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[key]
			if !ok {
				return nil, fmt.Errorf("key '%s' not found in JSON", key)
			}
			current = val
		case []interface{}:
			var idx int
			if _, err := fmt.Sscanf(key, "[%d]", &idx); err == nil {
				if idx < 0 || idx >= len(v) {
					return nil, fmt.Errorf("index %d out of range (length %d)", idx, len(v))
				}
				current = v[idx]
			} else {
				return nil, fmt.Errorf("expected array index like [0], got '%s'", key)
			}
		default:
			return nil, fmt.Errorf("cannot access key '%s' on non-object/array type", key)
		}
	}

	return current, nil
}
