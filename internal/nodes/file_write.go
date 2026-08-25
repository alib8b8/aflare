// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​‌‌‌‌‌​‌‌‌‌​‌‌‌​​​​‌‌​‌​​​‌‌​‌​‌‌​​​‌​‌‌​​‌‌‌‌​​​​​​​​​​​​​​​​‌‌‌‌‌​​‌​‌‌‌‌‌‌‌⁠
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

package nodes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/nodes/core"
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
			{Name: "path", Type: "string", Description: "File path to write to. Relative to the working directory, or relative to the connector root when `connector` is set.", Required: true},
			{Name: "content", Type: "string", Description: "Content to write; defaults to the step input (previous step output). Expressions like {{var.x}} are evaluated before the node runs.", Required: false},
			{Name: "connector", Type: "string", Description: "Named files/notes connector. Must be registered with --writable; paths resolve inside its root and its include allowlist applies.", Required: false},
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

	// content param (documented in skills/aflare/nodes-reference.md) overrides
	// the step input; when absent, keep the legacy behaviour of writing the
	// previous step's output.
	content := input
	if c, has := params["content"]; has {
		content = c
	}

	var safePath string
	var err error
	if connName := getParam(params, "connector", ""); connName != "" {
		spec, cerr := resolveFileConnector(connName)
		if cerr != nil {
			return "", cerr
		}
		// Read-only connectors (the default) cannot be written through
		// at all — node params can only tighten, never loosen.
		if spec.IsReadOnly() {
			return "", fmt.Errorf("connector %q is read-only; re-register it with --writable to allow writes", connName)
		}
		// Keep the same containment + dotfile/extension rules as
		// workdir mode (.env/.sh etc. stay unwritable), plus the
		// unconditional connector-root symlink containment check.
		safePath, err = core.ValidateWritePathIn(spec.Root, path)
		if err != nil {
			return "", fmt.Errorf("path validation failed: %w", err)
		}
		if cerr := validateConnectorPathContainment(spec.Root, safePath); cerr != nil {
			return "", fmt.Errorf("path validation failed: %w", cerr)
		}
		if !spec.MatchInclude(filepath.Base(safePath)) {
			return "", fmt.Errorf("connector %q does not allow writing %q (include allowlist: %s)",
				connName, filepath.Base(safePath), strings.Join(spec.EffectiveInclude(), ", "))
		}
	} else {
		safePath, err = validateWritePath(path)
		if err != nil {
			return "", fmt.Errorf("path validation failed: %w", err)
		}
	}

	switch strings.ToLower(mode) {
	case "append":
		if err := appendToFile(safePath, []byte(content)); err != nil {
			return "", fmt.Errorf("failed to append to file: %w", err)
		}
		return fmt.Sprintf("appended to %s", path), nil
	case "write", "":
		if err := atomicWriteFile(safePath, []byte(content), 0600); err != nil {
			return "", fmt.Errorf("failed to write file: %w", err)
		}
		return fmt.Sprintf("written to %s", path), nil
	default:
		return "", fmt.Errorf("invalid mode: %s (supported: write, append)", mode)
	}
}

func appendToFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) // #nosec G304 -- path validated by validateWritePath // codeql[go/path-injection] -- path is the safePath from validateWritePath in Execute; only called with that value
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

	tmpFile, err := os.CreateTemp(dir, ".tmp-*-"+filepath.Base(path)) // codeql[go/path-injection] -- dir = filepath.Dir(path); path is the validateWritePath-sanitized safePath passed by Execute
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	cleanup := func() {
		if err := tmpFile.Close(); err != nil {
			logger.Error("temp file close failed", "err", err)
		}
		if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
			logger.Warn("temp file cleanup failed", "path", tmpPath, "err", err)
		}
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
		if rerr := os.Remove(tmpPath); rerr != nil && !os.IsNotExist(rerr) {
			logger.Warn("temp file cleanup failed", "path", tmpPath, "err", rerr)
		}
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil { // codeql[go/path-injection] -- path is the validateWritePath-sanitized safePath; tmpPath is the CreateTemp file created in its own directory
		if rerr := os.Remove(tmpPath); rerr != nil && !os.IsNotExist(rerr) {
			logger.Warn("temp file cleanup failed", "path", tmpPath, "err", rerr)
		}
		return err
	}

	return nil
}
