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

// Execute implements the Node interface
func (n *FileWriteNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	// Get output path from params
	path, ok := params["path"]
	if !ok || path == "" {
		return "", fmt.Errorf("path parameter is required")
	}

	// Write the file (overwrite if exists)
	err := os.WriteFile(path, []byte(input), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("written to %s", path), nil
}
