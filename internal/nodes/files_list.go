// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​‌‌‌​‌​‌‌​​​‌​​‌‌​‌​​​​‌‌‌​​​‌‌‌​‌‌‌​‌‌​‌​​​​​​​​​‌​​​​‌​​​​‌​‌​​​​​​​​​​​​​​​​​​‌‌‌‌​​‌‌​‌​‌​​‌⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation; either version 3 of the License, or
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
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

const (
	defaultMaxListEntries = 200
	maxListEntriesHard    = 1000
)

// FilesListNode lists files under a files/notes connector root. It is the
// discovery primitive for personal data connectors: an AI agent can
// enumerate the vault (obsidian-style note folders, document trees) before
// reading individual files through file_read.
//
// Security posture:
//   - Listing is connector-scoped: it never walks outside the connector
//     root, and the connector's include allowlist applies.
//   - Dotfiles and dot-directories (.git, .obsidian, .trash) are skipped
//     — they are config/metadata, not user content.
//   - Symlinks are skipped (WalkDir does not follow them): no symlink
//     escape via listing.
//   - The result is capped at max_entries (hard cap 1000).
type FilesListNode struct{}

func init() {
	Register(&FilesListNode{})
}

func (n *FilesListNode) Name() string { return "files_list" }

func (n *FilesListNode) Description() string {
	return "List files under a files/notes connector root"
}

func (n *FilesListNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "files_list",
		Description: "List files under a files/notes connector root (relative paths + sizes). Skips dotfiles/dot-directories and symlinks. The connector's include allowlist applies. Use it to discover paths before file_read.",
		Input:       "string - not used",
		Output:      "string - JSON {files: [{path, bytes}], count, truncated}",
		Params: []ParamSchema{
			{Name: "connector", Type: "string", Description: "Named files/notes connector whose root is listed.", Required: true},
			{Name: "pattern", Type: "string", Description: "Glob matched against paths relative to the connector root, e.g. \"notes/*.md\" (single level) or \"**/*.md\" (any depth). Default: all files.", Required: false, Default: "**/*"},
			{Name: "max_entries", Type: "string", Description: "Max entries to return (default 200, hard cap 1000).", Required: false, Default: "200"},
		},
	}
}

// matchRel reports whether a connector-root-relative path matches a glob
// that may start with "**/" (any number of directory levels). The rest is
// matched with filepath.Match, whose "*" does not cross "/".
func matchRel(pattern, rel string) bool {
	pattern = filepath.ToSlash(pattern)
	rel = filepath.ToSlash(rel)
	if pattern == "**" || pattern == "**/*" || pattern == "" {
		return true
	}
	if strings.HasPrefix(pattern, "**/") {
		suffix := pattern[len("**/"):]
		parts := strings.Split(rel, "/")
		for i := range parts {
			if ok, err := filepath.Match(suffix, strings.Join(parts[i:], "/")); err == nil && ok {
				return true
			}
		}
		return false
	}
	ok, err := filepath.Match(pattern, rel)
	return err == nil && ok
}

type fileEntry struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

func (n *FilesListNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	connName := getParam(params, "connector", "")
	if connName == "" {
		return "", fmt.Errorf("connector parameter is required (aflare connector add --type files|notes)")
	}
	spec, err := resolveFileConnector(connName)
	if err != nil {
		return "", err
	}

	pattern := getParam(params, "pattern", "**/*")
	cleanPattern := filepath.ToSlash(filepath.Clean(strings.ReplaceAll(pattern, "\\", "/")))
	if strings.HasPrefix(cleanPattern, "/") || strings.HasPrefix(cleanPattern, "../") || cleanPattern == ".." {
		return "", fmt.Errorf("pattern must be relative to the connector root (got %q)", pattern)
	}
	maxEntries := core.ParamInt(params, "max_entries", defaultMaxListEntries, 1, maxListEntriesHard)

	type result struct {
		Files     []fileEntry `json:"files"`
		Count     int         `json:"count"`
		Truncated bool        `json:"truncated"`
	}
	res := result{Files: []fileEntry{}}

	walkErr := filepath.WalkDir(spec.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip symlinks entirely (files and directories): a listing must
		// never disclose or follow anything outside the connector root.
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !spec.MatchInclude(name) {
			return nil
		}
		rel, rerr := filepath.Rel(spec.Root, path)
		if rerr != nil {
			// WalkDir always yields paths under root, so Rel failing here
			// means something is deeply wrong — surface it instead of
			// silently skipping the entry.
			return fmt.Errorf("make %q relative to connector root: %w", path, rerr)
		}
		if !matchRel(pattern, rel) {
			return nil
		}
		if len(res.Files) >= maxEntries {
			res.Truncated = true
			return filepath.SkipAll
		}
		if info, ierr := d.Info(); ierr == nil {
			res.Files = append(res.Files, fileEntry{Path: filepath.ToSlash(rel), Bytes: info.Size()})
		}
		return nil
	})
	if walkErr != nil {
		return "", fmt.Errorf("failed to walk connector root: %w", walkErr)
	}

	sort.Slice(res.Files, func(i, j int) bool { return res.Files[i].Path < res.Files[j].Path })
	res.Count = len(res.Files)

	out, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal result: %w", err)
	}
	return string(out), nil
}
