package nodes

import (
	"context"
	"fmt"
	"os"
)

type FileReadNode struct{}

func init() {
	Register(&FileReadNode{})
}

func (n *FileReadNode) Name() string {
	return "file_read"
}

func (n *FileReadNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	path, ok := params["path"]
	if !ok || path == "" {
		return "", fmt.Errorf("path parameter is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(data), nil
}
