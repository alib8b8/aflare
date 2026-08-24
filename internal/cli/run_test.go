// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​​‌‌‌​‌‌‌‌‌​​‌‌‌​‌​​‌​​‌‌‌​‌​​​‌‌‌​​​‌​​‌‌​​​‌​​​​​​​​​​​​​​​​​‌‌‌‌‌‌​​​‌​​​​​⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseSetParams verifies the --set token parser: each token is exactly
// one key=value pair and values keep their spaces (unlike the legacy
// --params form, which splits a token on whitespace into multiple pairs).
// This is what makes `--set test_command=go test ./...` work instead of
// silently truncating the value to "go".
func TestParseSetParams(t *testing.T) {
	tests := []struct {
		name   string
		tokens []string
		want   map[string]string
	}{
		{
			name:   "empty",
			tokens: nil,
			want:   nil,
		},
		{
			name:   "token without equals ignored",
			tokens: []string{"noequals"},
			want:   nil,
		},
		{
			name:   "value with spaces kept verbatim",
			tokens: []string{"test_command=go test ./... -short"},
			want:   map[string]string{"test_command": "go test ./... -short"},
		},
		{
			name:   "multiple tokens are separate pairs",
			tokens: []string{"cmd=echo hello world", "path=out dir/file.md"},
			want: map[string]string{
				"cmd":  "echo hello world",
				"path": "out dir/file.md",
			},
		},
		{
			name:   "value containing equals sign",
			tokens: []string{"expr=a=b c"},
			want:   map[string]string{"expr": "a=b c"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseSetParams(tc.tokens)
			if len(got) != len(tc.want) {
				t.Fatalf("parseSetParams got %d entries, want %d (%v)", len(got), len(tc.want), got)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("parseSetParams[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

// TestAuditLock_SecondProcessRejected verifies the H-6 cross-process audit
// lock: once a lock is held on an audit directory, a second acquireAuditLock
// (simulating a concurrent aflare process) must fail. Without this lock, two
// processes sharing one audit directory would interleave HMAC hash-chain
// appends and fork the chain, breaking tamper-evidence.
func TestAuditLock_SecondProcessRejected(t *testing.T) {
	dir := t.TempDir()
	release, err := acquireAuditLock(dir)
	if err != nil {
		t.Fatalf("first acquireAuditLock: %v", err)
	}
	defer release()

	// A second acquire on the same directory must fail because the lock
	// file already exists (O_CREATE|O_EXCL).
	if _, err := acquireAuditLock(dir); err == nil {
		t.Errorf("second acquireAuditLock should have failed (lock held), got nil")
	}
}

// TestAuditLock_ReleaseAllowsNext verifies that releasing the audit lock lets
// a subsequent acquire succeed. This confirms the lock is not permanently
// held after a clean process exit, so back-to-back runs are not blocked
// (only concurrent runs are).
func TestAuditLock_ReleaseAllowsNext(t *testing.T) {
	dir := t.TempDir()
	release, err := acquireAuditLock(dir)
	if err != nil {
		t.Fatalf("first acquireAuditLock: %v", err)
	}
	release()

	// After release, the lock file is removed and a second acquire must
	// succeed.
	release2, err := acquireAuditLock(dir)
	if err != nil {
		t.Fatalf("second acquireAuditLock after release: %v", err)
	}
	release2()
}

// TestAuditLock_CreatesLockFile verifies the lock file is materialized at the
// expected path so an operator can identify and remove a stale lock left by a
// crashed process.
func TestAuditLock_CreatesLockFile(t *testing.T) {
	dir := t.TempDir()
	release, err := acquireAuditLock(dir)
	if err != nil {
		t.Fatalf("acquireAuditLock: %v", err)
	}
	defer release()

	lockPath := filepath.Join(dir, ".audit.lock")
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lock file at %s: %v", lockPath, err)
	}

	// On release the lock file is removed.
	release()
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Errorf("expected lock file removed after release; stat err = %v", err)
	}
}

// TestAuditLock_EmptyDirNoop verifies that passing an empty directory skips
// locking entirely (returns a no-op release and no error). This covers the
// case where no audit directory is available (e.g. HOME unset), where audit
// logging no-ops and a lock is unnecessary.
func TestAuditLock_EmptyDirNoop(t *testing.T) {
	release, err := acquireAuditLock("")
	if err != nil {
		t.Fatalf("acquireAuditLock(\"\") should not error: %v", err)
	}
	release()
}
