// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​‌​​‌​​‌‌‌‌‌‌​‌‌‌‌​‌​‌​​​‌‌‌‌‌‌​​​‌‌‌​​​‌​​‌​‌​​​​​​​​​​​​​​​​‌‌‌​‌​​‌​‌‌​​‌​‌⁠
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

package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	roottemplates "github.com/alib8b8/aflare"
)

// EnsureEmbeddedTemplates materializes the embedded template catalog (323
// workflow templates + skills-registry.json) into dir when dir does not yet
// contain any templates. It is idempotent and safe to call on every CLI
// invocation that needs the template registry — on a bare binary install with
// no source tree, the first call unpacks the catalog; subsequent calls are
// cheap no-ops (hasTemplatesOnDisk short-circuits before walking the embed FS).
//
// This is kept OUT of SkillRegistry.Load() so that Load() remains a pure
// read-from-disk operation (testable with an empty TempDir). CLI callers that
// want the "bare binary just works" behaviour call this before Load().
func EnsureEmbeddedTemplates(dir string) error {
	if dir == "" {
		return nil
	}
	if hasTemplatesOnDisk(dir) {
		return nil
	}
	return releaseEmbeddedTemplates(dir)
}

// hasTemplatesOnDisk reports whether baseDir already contains a usable
// templates tree (i.e. the skills-registry.json index or any workflow.yaml).
// When false, callers should release the embedded templates to baseDir so
// subsequent list/clone/run operations work on a bare binary install.
func hasTemplatesOnDisk(baseDir string) bool {
	if baseDir == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(baseDir, RegistryFileName)); err == nil {
		return true
	}
	// Fall back to checking for at least one workflow.yaml anywhere.
	found := false
	_ = filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // WalkDir callback: skip on error, continue traversal
		}
		if d.Name() == "workflow.yaml" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// releaseEmbeddedTemplates extracts the embedded templates/ tree (323 workflow
// templates + skills-registry.json) from the binary into targetDir. It is a
// one-time materialization: after release, all subsequent operations read from
// disk as before. Existing files are NOT overwritten (idempotent re-runs are
// safe and cheap — only missing files are written).
//
// This is what makes aflare "just work" on a bare binary install with no
// source tree and no network: the full template catalog ships inside the
// binary and is unpacked to the user templates dir on first use.
func releaseEmbeddedTemplates(targetDir string) error {
	if targetDir == "" {
		return fmt.Errorf("target directory is empty")
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create templates dir %s: %w", targetDir, err)
	}

	return fs.WalkDir(roottemplates.Embedded, "templates", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// p is "templates/<category>/<name>/<file>". Strip the leading
		// "templates/" so the on-disk layout matches the ID scheme
		// (<category>/<name>) used by SkillRegistry.
		rel, err := filepath.Rel("templates", p)
		if err != nil {
			return err
		}
		dest := filepath.Join(targetDir, rel)

		// Idempotent: skip files that already exist on disk (e.g. user
		// customized a template, or a previous release already wrote it).
		if _, err := os.Stat(dest); err == nil {
			return nil
		}

		data, err := roottemplates.Embedded.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", p, err)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		// #nosec G306 -- template files are YAML/JSON/Markdown, not
		// executable; 0o644 is the conventional mode.
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		return nil
	})
}
