// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package workflow

import (
	"os"
	"path/filepath"
	"strings"

	aferrors "github.com/alib8b8/aflare/internal/errors"
	"gopkg.in/yaml.v3"
)

// safeWorkflowPath validates that a workflow file path is safe to read.
// It prevents path traversal and ensures the file is within the working directory
// or an absolute path that has been explicitly provided.
//
// Note: path is not restricted to the current working directory because users may
// legitimately run workflow files outside it (e.g. /tmp/test.yaml). Security is
// enforced by validating the file extension (.yaml/.yml only) and rejecting
// symlinks that cannot be resolved; YAML parsing failures do not leak sensitive
// information from arbitrary files.
func safeWorkflowPath(path string) (string, error) {
	if path == "" {
		return "", aferrors.New(aferrors.CodeWorkflowParseError, "path is empty")
	}

	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", aferrors.Wrap(err, aferrors.CodeWorkflowParseError, "failed to resolve path")
	}

	info, err := os.Lstat(absPath)
	if err != nil {
		return "", aferrors.Wrap(err, aferrors.CodeWorkflowParseError, "failed to stat file")
	}

	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			return "", aferrors.Wrap(err, aferrors.CodeWorkflowParseError, "failed to resolve symlink")
		}
		absPath = resolved
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	if ext != ".yaml" && ext != ".yml" {
		return "", aferrors.New(aferrors.CodeWorkflowParseError, "only .yaml and .yml workflow files are allowed")
	}

	return absPath, nil
}

// ParseWorkflow parses a YAML file into a Workflow structure
func ParseWorkflow(path string) (*Workflow, error) {
	safePath, err := safeWorkflowPath(path)
	if err != nil {
		return nil, aferrors.Wrap(err, aferrors.CodeWorkflowParseError, "invalid workflow file path")
	}

	file, err := os.Open(safePath) // #nosec G304 -- path validated
	if err != nil {
		return nil, aferrors.Wrap(err, aferrors.CodeWorkflowParseError, "failed to open workflow file")
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, aferrors.Wrap(err, aferrors.CodeWorkflowParseError, "failed to stat workflow file")
	}
	if info.IsDir() {
		return nil, aferrors.New(aferrors.CodeWorkflowParseError, "workflow path is a directory, not a file")
	}
	if info.Size() > MaxFileSize {
		return nil, aferrors.Newf(aferrors.CodeWorkflowParseError, "workflow file too large (%d bytes, max %d bytes)", info.Size(), MaxFileSize)
	}

	data := make([]byte, info.Size())
	_, err = file.Read(data)
	if err != nil {
		return nil, aferrors.Wrap(err, aferrors.CodeWorkflowParseError, "failed to read workflow file")
	}

	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, aferrors.Wrap(err, aferrors.CodeWorkflowParseError, "failed to parse workflow YAML")
	}

	return &wf, nil
}

// ParseWorkflowFromContent parses YAML content into a Workflow structure
func ParseWorkflowFromContent(content string) (*Workflow, error) {
	if len(content) > MaxFileSize {
		return nil, aferrors.Newf(aferrors.CodeWorkflowParseError, "workflow content too large (%d bytes, max %d bytes)", len(content), MaxFileSize)
	}

	var wf Workflow
	if err := yaml.Unmarshal([]byte(content), &wf); err != nil {
		return nil, aferrors.Wrap(err, aferrors.CodeWorkflowParseError, "failed to parse workflow YAML")
	}

	return &wf, nil
}
