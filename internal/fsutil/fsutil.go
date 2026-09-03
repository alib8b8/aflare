// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌‌​​​‌‌​​​​​​‌​​​​‌‌​​‌​‌​​​​​‌​‌‌‌‌‌​‌​‌​​‌​‌‌​‌‌‌‌​‌​‌​‌‌​‌​​‌‌​​​​​​​​​​​​​​​​‌​​​​‌‌​​​​​​‌​‌⁠
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

// Package fsutil provides crash-safe filesystem primitives for aflare's
// persistence layer (workflow checkpoints, scheduler stores). The engine
// sells auditable, deterministic execution — the files that make a crashed
// run resumable must not themselves be the thing a crash destroys.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WriteFileAtomic writes data to path atomically: the payload goes to a
// temp file in the same directory, which is fsynced and renamed over the
// target, then the directory is fsynced so the rename itself is durable.
// A crash mid-write therefore leaves the previous file intact instead of a
// truncated JSON document. perm is applied to the final file (the temp
// file starts at 0600 and is chmod'ed before the rename, so the target is
// never briefly world-readable). The parent directory must already exist.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op after a successful rename

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write temp file %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to sync temp file %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("failed to chmod temp file %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("failed to rename %s to %s: %w", tmpName, path, err)
	}
	// Best-effort directory fsync: on some filesystems the rename is not
	// durable until the containing directory is synced. Failure here does
	// not corrupt anything — the rename already happened.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// PreserveCorrupt moves an unreadable persistence file (e.g. a checkpoint
// whose JSON was truncated by a crash) aside to <path>.corrupt-<unix-nano>
// so the caller can start fresh without silently destroying the user's
// last recoverable state. It returns the preserved path and never fails
// harder than the situation already is: if the rename fails, the original
// error is what matters, so the return value is only informational.
func PreserveCorrupt(path string) string {
	preserved := fmt.Sprintf("%s.corrupt-%d", path, time.Now().UnixNano())
	if err := os.Rename(path, preserved); err != nil {
		return ""
	}
	return preserved
}
