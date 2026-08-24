// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌‌‌​​‌​​‌​‌​‌​​​​​‌‌​​​‌​​‌‌​‌​​‌‌​‌‌​‌‌​‌‌‌​​‌‌‌‌​​‌‌‌​‌​‌​​​​​​​​​​​​​​​​​​​​​‌‌​‌‌‌‌‌​‌​‌‌‌⁠
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

package history

import (
	"bufio"
	"crypto/hmac"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// safeFilePath validates a file path to prevent path traversal and null-byte
// injection attacks. It resolves symlinks and cleans the path.
func safeFilePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty file path")
	}
	// Reject paths containing null bytes
	if strings.ContainsRune(path, '\x00') {
		return "", fmt.Errorf("file path contains null byte")
	}
	cleaned := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		// If the file doesn't exist yet, use the cleaned absolute path
		if os.IsNotExist(err) {
			absPath, err := filepath.Abs(cleaned)
			if err != nil {
				return "", fmt.Errorf("failed to resolve file path: %w", err)
			}
			return absPath, nil
		}
		return "", fmt.Errorf("failed to resolve file path: %w", err)
	}
	return resolved, nil
}

// VerifyAuditChain validates the HMAC hash chain of the audit log at path.
// It returns valid=true when every record's prev_hash links to the previous
// record's curr_hash and each curr_hash matches the recomputed HMAC. Each
// record is verified with the algorithm named by its own mac_algo field
// (sha256 when absent), so mixed sha256/sm3 chains verify correctly.
// brokenAtLine is the 1-based file line number of the first broken record (0
// when the file is empty or the whole chain is valid). err is non-nil for I/O
// or format errors, including legacy records that lack hash fields.
// The path is validated to prevent path traversal and null-byte injection.
func VerifyAuditChain(path string) (valid bool, brokenAtLine int, err error) {
	safePath, err := safeFilePath(path)
	if err != nil {
		return false, 0, err
	}
	f, err := os.Open(safePath) // #nosec G304 -- path validated by safeFilePath
	if err != nil {
		if os.IsNotExist(err) {
			// An absent audit log is trivially intact.
			return true, 0, nil
		}
		return false, 0, fmt.Errorf("failed to open audit log: %w", err)
	}
	defer f.Close()

	// A chain is signed with exactly one key, but WHICH key depends on how the
	// deployment was configured over time (env, password, per-install key
	// file, or the legacy public default). Replay under every candidate and
	// accept the first fully valid pass; parse errors are key-independent and
	// fail immediately.
	candidates := auditKeyCandidates()
	type verifyResult struct {
		valid     bool
		brokenAt  int
		err       error
		scanError bool
	}
	results := make([]verifyResult, 0, len(candidates))
	for _, secret := range candidates {
		valid, brokenAt, err, scanErr := verifyAuditLines(f, secret)
		results = append(results, verifyResult{valid, brokenAt, err, scanErr})
		if valid || scanErr {
			if valid {
				return true, 0, nil
			}
			return false, brokenAt, err
		}
		// Chain mismatch under this key: rewind and try the next candidate.
		if _, serr := f.Seek(0, io.SeekStart); serr != nil {
			return false, 0, fmt.Errorf("failed to rewind audit log: %w", serr)
		}
	}
	// No candidate verified the whole chain: report the first candidate's
	// mismatch position (the highest-priority key), which is the most useful
	// diagnostic for the operator.
	if len(results) > 0 {
		return false, results[0].brokenAt, results[0].err
	}
	return false, 0, fmt.Errorf("no audit HMAC key candidates available")
}

// verifyAuditLines replays the hash chain in f under one key. scanErr marks
// key-independent failures (parse/IO errors) that must abort the candidate
// loop; a pure mismatch returns scanErr=false so other keys can be tried.
func verifyAuditLines(f *os.File, secret []byte) (valid bool, brokenAt int, err error, scanErr bool) {
	expectedPrev := auditZeroHash
	lineNum := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry AuditLog
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return false, lineNum, fmt.Errorf("line %d: failed to parse record: %w", lineNum, err), true
		}
		// Backwards compatibility: legacy records without hash fields cannot be
		// verified and must be reported explicitly rather than crashing.
		if entry.PrevHash == "" && entry.CurrHash == "" {
			return false, lineNum, fmt.Errorf("line %d: incompatible format (missing prev_hash/curr_hash fields)", lineNum), true
		}
		if entry.PrevHash != expectedPrev {
			return false, lineNum, nil, false
		}
		savedHash := entry.CurrHash
		entry.CurrHash = ""
		computedHash, err := computeAuditHash(secret, entry)
		if err != nil {
			return false, lineNum, fmt.Errorf("line %d: %w", lineNum, err), true
		}
		if !hmac.Equal([]byte(computedHash), []byte(savedHash)) {
			return false, lineNum, nil, false
		}
		expectedPrev = savedHash
	}
	if err := scanner.Err(); err != nil {
		return false, lineNum, fmt.Errorf("failed to read audit log: %w", err), true
	}
	return true, 0, nil, false
}
