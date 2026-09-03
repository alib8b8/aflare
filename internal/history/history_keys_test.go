// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌‌​​​‌‌​​​​​​‌​​​​​​‌‌​​‌‌‌‌​​‌‌​​‌​​​​‌​‌​​‌​‌​​​‌​​‌‌​​‌​​‌‌‌‌​​​​​​​​​​​​​​​​​​​​​‌‌​‌‌​‌‌​‌​‌⁠
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
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestAuditChainKeyContinuity_AcrossPasswordEnvSwitch verifies that a chain
// started under the per-install key file keeps being signed with that key
// even when AFLARE_SECRETS_PASSWORD later becomes (or changes) — the
// password-derived key must not silently fork the chain, because
// VerifyAuditChain only replays a whole chain under a single key.
func TestAuditChainKeyContinuity_AcrossPasswordEnvSwitch(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)
	t.Cleanup(func() { SetHistoryDir("") })

	path := filepath.Join(tmpDir, auditLogFileName)

	// Phase 1: start the chain with no env key and no password → the
	// per-install key file is generated.
	if err := AppendAuditLog(AuditLog{Action: AuditActionLogin, User: "alice", Detail: "first"}); err != nil {
		t.Fatalf("phase 1 append failed: %v", err)
	}

	// Phase 2: export a password — the append must CONTINUE the key-file
	// chain, not switch to the password-derived key.
	t.Setenv(auditEnvSecretsPasswd, "pw-1")
	if err := AppendAuditLog(AuditLog{Action: AuditActionLogin, User: "alice", Detail: "second"}); err != nil {
		t.Fatalf("phase 2 append failed: %v", err)
	}

	// Phase 3: the chain verifies while the password is exported (the
	// key-file candidate still replays the whole chain).
	if valid, brokenAt, err := VerifyAuditChain(path); err != nil || !valid {
		t.Fatalf("phase 3: expected valid chain, got valid=%v brokenAt=%d err=%v", valid, brokenAt, err)
	}

	// Phase 4: a DIFFERENT password (rotated) must not fork the chain either.
	t.Setenv(auditEnvSecretsPasswd, "pw-rotated")
	if err := AppendAuditLog(AuditLog{Action: AuditActionLogin, User: "alice", Detail: "third"}); err != nil {
		t.Fatalf("phase 4 append failed: %v", err)
	}
	if valid, brokenAt, err := VerifyAuditChain(path); err != nil || !valid {
		t.Fatalf("phase 4: expected valid chain after rotated password append, got valid=%v brokenAt=%d err=%v", valid, brokenAt, err)
	}
}

// TestAuditAppendRefusedWhenChainKeyUnavailable verifies the refusal path: a
// chain that was STARTED under a password-derived key (no key file exists)
// must reject appends once the password is no longer exported, instead of
// silently signing new records with a mismatched key and permanently breaking
// verification.
func TestAuditAppendRefusedWhenChainKeyUnavailable(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)
	t.Cleanup(func() { SetHistoryDir("") })

	// Start the chain under the password-derived key (fresh dir: no key
	// file, and the first append finds an empty log).
	t.Setenv(auditEnvSecretsPasswd, "pw-only")
	path := filepath.Join(tmpDir, auditLogFileName)
	if err := AppendAuditLog(AuditLog{Action: AuditActionLogin, User: "bob", Detail: "first"}); err != nil {
		t.Fatalf("first append failed: %v", err)
	}
	// No key file may have been generated for a password-started chain.
	if _, err := os.Stat(filepath.Join(tmpDir, auditKeyFileName)); err == nil {
		t.Fatal("password-started chain must not generate a key file")
	}

	// Drop the password: the chain's signing key is no longer derivable.
	os.Unsetenv(auditEnvSecretsPasswd)
	err := AppendAuditLog(AuditLog{Action: AuditActionLogin, User: "bob", Detail: "second"})
	if !errors.Is(err, ErrAuditKeyUnavailable) {
		t.Fatalf("expected ErrAuditKeyUnavailable, got %v", err)
	}

	// The chain must be unchanged (one record) and still verify under the
	// original password.
	lines := readAuditLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 record after refused append, got %d", len(lines))
	}
	t.Setenv(auditEnvSecretsPasswd, "pw-only")
	if valid, brokenAt, verr := VerifyAuditChain(path); verr != nil || !valid {
		t.Fatalf("expected valid chain, got valid=%v brokenAt=%d err=%v", valid, brokenAt, verr)
	}
}

// TestAuditChainContinuity_EnvKeyOverrides verifies that the explicit
// AFLARE_AUDIT_HMAC_KEY still wins over continuity — it represents operator
// intent after an archive-and-rotate migration.
func TestAuditChainContinuity_EnvKeyOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	SetHistoryDir(tmpDir)
	t.Cleanup(func() { SetHistoryDir("") })

	t.Setenv(auditEnvHMACKey, "explicit-key")
	path := filepath.Join(tmpDir, auditLogFileName)
	if err := AppendAuditLog(AuditLog{Action: AuditActionLogin, User: "carol", Detail: "first"}); err != nil {
		t.Fatalf("first append failed: %v", err)
	}
	if valid, _, err := VerifyAuditChain(path); err != nil || !valid {
		t.Fatalf("expected valid chain under env key, got %v", err)
	}
}
