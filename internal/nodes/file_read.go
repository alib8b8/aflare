// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​​​​‌​​​‌‌​‌​​‌‌​‌‌​​​‌‌‌​​‌​‌​​‌​​​‌‌​​‌​​‌​​‌​​​​​​​​​​​​​​​​‌​​‌‌​​​​‌‌‌‌‌‌​⁠
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

	"github.com/alib8b8/aflare/internal/connector"
	"github.com/alib8b8/aflare/internal/nodes/core"
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
		Description: "Read content from a file. Automatically redacts secrets (API keys, tokens, .env files) by default for privacy — set redact=false to disable. With a `connector` (files/notes) the path is resolved inside the connector's root and its include/max_bytes ceilings apply.",
		Input:       "string - not used",
		Output:      "string - file content (with secrets redacted by default)",
		Params: []ParamSchema{
			{Name: "path", Type: "string", Description: "File path to read from. Relative to the working directory, or relative to the connector root when `connector` is set.", Required: true},
			{Name: "connector", Type: "string", Description: "Named files/notes connector (aflare connector add --type files|notes). Paths resolve inside its root and cannot escape; the connector's include allowlist and max_bytes ceiling apply. Ignored database connectors are rejected.", Required: false},
			{Name: "redact", Type: "string", Description: "Redact secrets in output: true (default) / false. When true, .env and credential files are fully masked; other files have known secret patterns masked.", Required: false, Default: "true"},
		},
	}
}

// resolveFileConnector loads a files/notes connector spec. Database
// connectors are rejected with a hint toward sql_query.
func resolveFileConnector(name string) (connector.Spec, error) {
	spec, err := resolveConnectorSpec(name)
	if err != nil {
		return connector.Spec{}, err
	}
	if !spec.IsFileConnector() {
		return connector.Spec{}, fmt.Errorf("connector %q is a %s connector; file nodes expect files/notes connectors (use sql_query for databases)", name, spec.Type)
	}
	return spec, nil
}

func (n *FileReadNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	path, ok := params["path"]
	if !ok || path == "" {
		return "", fmt.Errorf("path parameter is required")
	}

	var safePath string
	var err error
	maxReadSize := int64(maxFileReadSize)
	if connName := getParam(params, "connector", ""); connName != "" {
		spec, cerr := resolveFileConnector(connName)
		if cerr != nil {
			return "", cerr
		}
		// Paths resolve inside the connector root with the same
		// containment rules as workdir mode: no absolute paths, no
		// traversal, L2+ symlink-escape checks.
		safePath, err = core.ValidateReadPathIn(spec.Root, path)
		if err != nil {
			return "", fmt.Errorf("path validation failed: %w", err)
		}
		if !spec.MatchInclude(filepath.Base(safePath)) {
			return "", fmt.Errorf("connector %q does not allow reading %q (include allowlist: %s)",
				connName, filepath.Base(safePath), strings.Join(spec.EffectiveInclude(), ", "))
		}
		// The connector ceiling can only tighten the node's hard cap.
		maxReadSize = int64(spec.EffectiveMaxBytes())
		if maxReadSize > int64(maxFileReadSize) {
			maxReadSize = int64(maxFileReadSize)
		}
	} else {
		safePath, err = validateReadPath(path)
		if err != nil {
			return "", fmt.Errorf("path validation failed: %w", err)
		}
	}

	fi, err := os.Stat(safePath) // codeql[go/path-injection] -- safePath is the validateReadPath/connector-root result; SafeJoinPath blocks absolute/traversal/symlink escape
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}
	if fi.Size() > maxReadSize {
		return "", fmt.Errorf("file too large (max %d bytes)", maxReadSize)
	}

	data, err := os.ReadFile(safePath) // #nosec G304 -- path validated by validateReadPath // codeql[go/path-injection] -- same safePath from validateReadPath above
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)

	// 隐私优先：默认对读取内容进行密钥脱敏（借鉴 Grok Build 隐私丑闻教训）
	redact := getParam(params, "redact", "true")
	if redact != "false" {
		// 敏感文件（.env / 私钥 / credentials）整文件脱敏
		wholeFile := IsSensitiveFile(filepath.Base(safePath))
		masked, hits := RedactSecrets(content, wholeFile)
		if hits > 0 {
			// 记录脱敏行为（不泄露文件内容，仅记录命中数）
			return masked + fmt.Sprintf("\n\n[security: %d secret(s) redacted]", hits), nil
		}
	}

	return content, nil
}
