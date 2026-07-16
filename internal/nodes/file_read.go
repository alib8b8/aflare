package nodes

import (
	"context"
	"fmt"
	"os"
)

const maxFileReadSize = 10 * 1024 * 1024 // 10MB max file read size

type FileReadNode struct{}

func init() {
	Register(&FileReadNode{})
}

func (n *FileReadNode) Name() string {
	return "file_read"
}

func (n *FileReadNode) Description() string {
	return "Read content from a file"
}

func (n *FileReadNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "file_read",
		Description: "Read content from a file",
		Input:       "string - not used",
		Output:      "string - file content",
		Params: []ParamSchema{
			{Name: "path", Type: "string", Description: "File path to read from", Required: true},
		},
	}
}

func (n *FileReadNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	path, ok := params["path"]
	if !ok || path == "" {
		return "", fmt.Errorf("path parameter is required")
	}

	safePath, err := validateReadPath(path)
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	fi, err := os.Stat(safePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}
	if fi.Size() > maxFileReadSize {
		return "", fmt.Errorf("file too large (max %d bytes)", maxFileReadSize)
	}

	data, err := os.ReadFile(safePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(data), nil
}
