package nodes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
			{Name: "mode", Type: "string", Description: "Write mode: write (default) or append", Required: false, Default: "write"},
		},
	}
}

// Execute implements the Node interface
func (n *FileWriteNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	path, ok := params["path"]
	if !ok || path == "" {
		return "", fmt.Errorf("path parameter is required")
	}

	mode, ok := params["mode"]
	if !ok || mode == "" {
		mode = "write"
	}

	safePath, err := validateWritePath(path)
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	switch strings.ToLower(mode) {
	case "append":
		if err := appendToFile(safePath, []byte(input)); err != nil {
			return "", fmt.Errorf("failed to append to file: %w", err)
		}
		return fmt.Sprintf("appended to %s", path), nil
	case "write", "":
		if err := atomicWriteFile(safePath, []byte(input), 0600); err != nil {
			return "", fmt.Errorf("failed to write file: %w", err)
		}
		return fmt.Sprintf("written to %s", path), nil
	default:
		return "", fmt.Errorf("invalid mode: %s (supported: write, append)", mode)
	}
}

func appendToFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	// Ensure data ends with newline for clean appending
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}

	_, err = f.Write(data)
	return err
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

	cleanup := func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := tmpFile.Write(data); err != nil {
		cleanup()
		return err
	}

	if err := tmpFile.Sync(); err != nil {
		cleanup()
		return err
	}

	if err := tmpFile.Chmod(perm); err != nil {
		cleanup()
		return err
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return nil
}
