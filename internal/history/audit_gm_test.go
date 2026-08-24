// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌‌​​‌​​‌‌‌​​​‌‌​​‌‌​‌​​​‌‌‌​​​​‌​‌​‌‌‌‌​​​‌‌​‌​​​​​​​​​​​​​​​​‌​‌​​​​‌​​‌‌​​​​⁠
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
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// TestAppendAuditLog_SM3ChainAndVerify appends a full SM3 chain and verifies it
// end-to-end: every record is tagged mac_algo=sm3, the prev_hash/curr_hash
// linkage uses the same semantics as the SHA-256 chain, and VerifyAuditChain
// accepts it.
func TestAppendAuditLog_SM3ChainAndVerify(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	t.Setenv("AFLARE_AUDIT_HMAC_ALGO", "sm3")
	SetHistoryDir(t.TempDir())

	path := appendAuditLogsForChain(t, 3)

	lines := readAuditLines(t, path)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	prevCurr := auditZeroHash
	for i, line := range lines {
		var entry AuditLog
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("failed to parse line %d: %v", i+1, err)
		}
		if entry.MACAlgo != auditMACSM3 {
			t.Errorf("line %d: expected mac_algo %q, got %q", i+1, auditMACSM3, entry.MACAlgo)
		}
		if entry.PrevHash != prevCurr {
			t.Errorf("line %d: prev_hash mismatch (expected %s, got %s)", i+1, prevCurr, entry.PrevHash)
		}
		if len(entry.CurrHash) != 64 {
			t.Errorf("line %d: expected 64-hex-char curr_hash, got %d chars", i+1, len(entry.CurrHash))
		}
		prevCurr = entry.CurrHash
	}

	valid, brokenAt, err := VerifyAuditChain(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !valid {
		t.Errorf("expected valid SM3 chain, broken at line %d", brokenAt)
	}
}

// TestVerifyAuditChain_MixedAlgorithms verifies a chain that switches between
// sha256 and sm3 mid-chain: each record is verified with its own mac_algo, and
// the linkage semantics stay identical across the switch.
func TestVerifyAuditChain_MixedAlgorithms(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	SetHistoryDir(t.TempDir())

	// Two sha256 records first.
	t.Setenv("AFLARE_AUDIT_HMAC_ALGO", "sha256")
	path := appendAuditLogsForChain(t, 2)

	// Switch to sm3 mid-chain.
	t.Setenv("AFLARE_AUDIT_HMAC_ALGO", "sm3")
	if err := AppendAuditLog(AuditLog{Action: AuditActionConfigChange, User: "alice", Detail: "change-sm3"}); err != nil {
		t.Fatalf("failed to append SM3 record: %v", err)
	}

	// And back to sha256 for the tail.
	t.Setenv("AFLARE_AUDIT_HMAC_ALGO", "sha256")
	if err := AppendAuditLog(AuditLog{Action: AuditActionConfigChange, User: "alice", Detail: "change-back"}); err != nil {
		t.Fatalf("failed to append SHA256 record: %v", err)
	}

	lines := readAuditLines(t, path)
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}

	// sha256 records omit the field entirely (pre-0.9.0 byte compatibility);
	// only the SM3 record carries mac_algo.
	wantAlgos := []string{"", "", auditMACSM3, ""}
	entries := make([]AuditLog, 0, len(lines))
	for i, line := range lines {
		var entry AuditLog
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("failed to parse line %d: %v", i+1, err)
		}
		if entry.MACAlgo != wantAlgos[i] {
			t.Errorf("line %d: expected mac_algo %q, got %q", i+1, wantAlgos[i], entry.MACAlgo)
		}
		entries = append(entries, entry)
	}

	// The linkage semantics must not change across an algorithm switch:
	// each prev_hash is the previous record's curr_hash, whatever the algo.
	for i := 1; i < len(entries); i++ {
		if entries[i].PrevHash != entries[i-1].CurrHash {
			t.Errorf("line %d: prev_hash %s does not link to previous curr_hash %s",
				i+1, entries[i].PrevHash, entries[i-1].CurrHash)
		}
	}

	valid, brokenAt, err := VerifyAuditChain(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !valid {
		t.Errorf("expected mixed-algorithm chain to be valid, broken at line %d", brokenAt)
	}
}

// TestVerifyAuditChain_SM3Tampered tampers with a record body in an SM3 chain
// and expects verification to report the chain broken at that line.
func TestVerifyAuditChain_SM3Tampered(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	t.Setenv("AFLARE_AUDIT_HMAC_ALGO", "sm3")
	SetHistoryDir(t.TempDir())

	path := appendAuditLogsForChain(t, 4)
	lines := readAuditLines(t, path)
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}

	// Tamper with line 3 (index 2): change the detail but leave curr_hash
	// unchanged so the recomputed HMAC-SM3 no longer matches.
	tampered := strings.Replace(lines[2], "change-2", "change-EVIL", 1)
	if tampered == lines[2] {
		t.Fatal("tampering did not modify the line")
	}
	lines[2] = tampered
	writeAuditLines(t, path, lines)

	valid, brokenAt, err := VerifyAuditChain(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid {
		t.Error("expected SM3 chain to detect tampering")
	}
	if brokenAt != 3 {
		t.Errorf("expected broken at line 3, got %d", brokenAt)
	}
}

// TestVerifyAuditChain_LegacyRecordWithoutMACAlgo builds a record exactly as
// pre-SM3 binaries wrote it (prev_hash/curr_hash present, mac_algo absent) and
// verifies it still validates as sha256 even when the environment selects sm3.
func TestVerifyAuditChain_LegacyRecordWithoutMACAlgo(t *testing.T) {
	key := "legacy-key"
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", key)
	t.Setenv("AFLARE_AUDIT_HMAC_ALGO", "sm3")
	SetHistoryDir(t.TempDir())

	entry := AuditLog{
		ID:        "legacy-1",
		Timestamp: time.Now(),
		Action:    AuditActionLogin,
		Success:   true,
		PrevHash:  auditZeroHash,
	}
	currHash, err := computeAuditHash([]byte(key), entry)
	if err != nil {
		t.Fatalf("failed to compute legacy hash: %v", err)
	}
	entry.CurrHash = currHash
	line, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal legacy entry: %v", err)
	}
	if strings.Contains(string(line), "mac_algo") {
		t.Fatalf("legacy entry unexpectedly contains mac_algo: %s", line)
	}

	path := GetAuditLogPath()
	if err := os.WriteFile(path, append(line, '\n'), 0600); err != nil {
		t.Fatalf("failed to write legacy audit log: %v", err)
	}

	valid, brokenAt, err := VerifyAuditChain(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !valid {
		t.Errorf("expected legacy record without mac_algo to verify as sha256, broken at line %d", brokenAt)
	}
}

// TestAppendAuditLog_DefaultMACAlgoIsSHA256 checks that with the environment
// variable unset, the stored mac_algo field is empty — which both computeAuditHash
// and VerifyAuditChain treat as sha256, and which keeps records byte-identical
// to pre-0.9.0 output (the field is omitted from the JSON entirely).
func TestAppendAuditLog_DefaultMACAlgoIsSHA256(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	t.Setenv("AFLARE_AUDIT_HMAC_ALGO", "")
	SetHistoryDir(t.TempDir())

	path := appendAuditLogsForChain(t, 1)
	line := readAuditLines(t, path)[0]

	var entry AuditLog
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("failed to parse line: %v", err)
	}
	if entry.MACAlgo != "" {
		t.Errorf("expected mac_algo to stay empty by default (sha256 implied), got %q", entry.MACAlgo)
	}
}

// TestResolveAuditMACAlgo covers the environment value parsing, including
// case/whitespace tolerance and rejection of unknown algorithms.
func TestResolveAuditMACAlgo(t *testing.T) {
	cases := []struct {
		env  string
		want string
	}{
		{"", auditMACSHA256},
		{"sha256", auditMACSHA256},
		{"SHA256", auditMACSHA256},
		{"sm3", auditMACSM3},
		{"SM3", auditMACSM3},
		{"  sm3  ", auditMACSM3},
	}
	for _, tc := range cases {
		got, err := resolveAuditMACAlgo(tc.env)
		if err != nil {
			t.Errorf("resolveAuditMACAlgo(%q) unexpected error: %v", tc.env, err)
			continue
		}
		if got != tc.want {
			t.Errorf("resolveAuditMACAlgo(%q) = %q, want %q", tc.env, got, tc.want)
		}
	}

	if _, err := resolveAuditMACAlgo("md5"); err == nil {
		t.Error("resolveAuditMACAlgo should reject unknown algorithms")
	}
}

// TestAppendAuditLog_InvalidMACAlgoEnv ensures an invalid AFLARE_AUDIT_HMAC_ALGO
// value fails the append instead of silently falling back to the default.
func TestAppendAuditLog_InvalidMACAlgoEnv(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	t.Setenv("AFLARE_AUDIT_HMAC_ALGO", "md5")
	SetHistoryDir(t.TempDir())

	err := AppendAuditLog(AuditLog{Action: AuditActionLogin})
	if err == nil {
		t.Fatal("expected error for invalid AFLARE_AUDIT_HMAC_ALGO")
	}
	if !strings.Contains(err.Error(), "AFLARE_AUDIT_HMAC_ALGO") {
		t.Errorf("error should mention AFLARE_AUDIT_HMAC_ALGO, got: %v", err)
	}
}

// TestComputeAuditHash_AlgorithmSelection verifies the hash dispatch: sm3 and
// sha256 produce different digests for the same record, and unknown algorithms
// are rejected.
func TestComputeAuditHash_AlgorithmSelection(t *testing.T) {
	entry := AuditLog{Action: AuditActionLogin, PrevHash: auditZeroHash}

	entry.MACAlgo = auditMACSHA256
	shaHash, err := computeAuditHash([]byte("k"), entry)
	if err != nil {
		t.Fatalf("sha256 computeAuditHash failed: %v", err)
	}

	entry.MACAlgo = auditMACSM3
	sm3Hash, err := computeAuditHash([]byte("k"), entry)
	if err != nil {
		t.Fatalf("sm3 computeAuditHash failed: %v", err)
	}

	if shaHash == sm3Hash {
		t.Error("expected sha256 and sm3 digests to differ")
	}

	entry.MACAlgo = "md5"
	if _, err := computeAuditHash([]byte("k"), entry); err == nil {
		t.Error("computeAuditHash should reject unknown algorithms")
	}
}

// TestAppendAuditLog_DefaultOmitsMACAlgoField pins the mixed-fleet
// compatibility contract: with the default sha256 selection (no env var),
// appended records carry no mac_algo field at all, so the JSON lines are
// byte-identical to what pre-0.9.0 binaries wrote and their readers (which
// recompute HMAC-SHA256 and ignore unknown fields) verify them unchanged.
func TestAppendAuditLog_DefaultOmitsMACAlgoField(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	t.Setenv("AFLARE_AUDIT_HMAC_ALGO", "")
	SetHistoryDir(t.TempDir())

	path := appendAuditLogsForChain(t, 2)

	for i, line := range readAuditLines(t, path) {
		if strings.Contains(line, "mac_algo") {
			t.Errorf("line %d: default sha256 record must not carry mac_algo (got %q)", i+1, line)
		}
	}

	// The chain still verifies, and the exported path helper resolves it.
	valid, _, err := VerifyAuditChain(path)
	if err != nil || !valid {
		t.Fatalf("default chain should verify: valid=%v err=%v", valid, err)
	}
	if AuditLogPath() != path {
		t.Errorf("AuditLogPath() = %q, want %q", AuditLogPath(), path)
	}
}
