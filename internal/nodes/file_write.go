package nodes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

	if err := atomicWriteFile(safePath, []byte(input), 0600); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("written to %s", path), nil
}

// atomicWriteFile writes content to a file atomically by first writing to a
// temporary file in the same directory, then renaming to the target path.
// This ensures the target file is either fully written or unchanged.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}

	tmpFile, err := os.CreateTemp(dir, ".tmp-*-"+filepath.Base(path))
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if tmpFile != nil {
			_ = tmpFile.Close()
		}
		_ = os.Remove(tmpPath)
	}()

	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}

	if _, err := tmpFile.Write(data); err != nil {
		return err
	}

	if err := tmpFile.Sync(); err != nil {
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}
	tmpFile = nil

	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	return nil
}
