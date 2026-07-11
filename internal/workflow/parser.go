package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxWorkflowFileSize = 10 * 1024 * 1024 // 10MB max workflow file size

// safeWorkflowPath validates that a workflow file path is safe to read.
// It prevents path traversal and ensures the file is within the working directory
// or an absolute path that has been explicitly provided.
func safeWorkflowPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}

	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	info, err := os.Lstat(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve symlink: %w", err)
		}
		absPath = resolved
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	if ext != ".yaml" && ext != ".yml" {
		return "", fmt.Errorf("only .yaml and .yml workflow files are allowed")
	}

	return absPath, nil
}

// ParseWorkflow parses a YAML file into a Workflow structure
func ParseWorkflow(path string) (*Workflow, error) {
	safePath, err := safeWorkflowPath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid workflow file path: %w", err)
	}

	file, err := os.Open(safePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open workflow file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat workflow file: %w", err)
	}
	if info.Size() > maxWorkflowFileSize {
		return nil, fmt.Errorf("workflow file too large (%d bytes, max %d bytes)", info.Size(), maxWorkflowFileSize)
	}

	data := make([]byte, info.Size())
	_, err = file.Read(data)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}

	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

	return &wf, nil
}
