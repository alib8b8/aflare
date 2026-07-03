package nodes

import (
	"context"
	"fmt"
	"os"
)

// FileWriteNode writes content to a file
type FileWriteNode struct{}

func init() {
	Register(&FileWriteNode{})
}

// Name returns the node name
func (n *FileWriteNode) Name() string {
	return "file_write"
}

func (n *FileWriteNode) Description() string {
	return "Write content to a file"
}

func (n *FileWriteNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "file_write",
		Description: "Write content to a file",
		Input:       "string - content to write to the file",
		Output:      "string - confirmation message",
		Params: []ParamSchema{
			{Name: "path", Type: "string", Description: "File path to write to", Required: true},
		},
	}
}

// Execute implements the Node interface
func (n *FileWriteNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	path, ok := params["path"]
	if !ok || path == "" {
		return "", fmt.Errorf("path parameter is required")
	}

	safePath, err := validateWritePath(path)
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	err = os.WriteFile(safePath, []byte(input), 0600)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("written to %s", path), nil
}
