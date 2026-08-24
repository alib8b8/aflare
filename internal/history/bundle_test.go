// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌​‌​‌​‌​‌​​‌​‌​‌​​​​‌‌‌​‌​​‌​​​​​​​‌​‌​​‌‌‌‌‌‌​​​​​​​​​​​​​​​​​‌‌​​​‌​​​‌​​‌‌‌⁠
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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildAuditChainForBundle appends count chained audit records with explicit
// timestamps and returns them in file (chain) order.
func buildAuditChainForBundle(t *testing.T, count int) []AuditLog {
	t.Helper()
	base := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
	records := make([]AuditLog, 0, count)
	for i := 0; i < count; i++ {
		log := AuditLog{
			ID:        "bundle-record-" + string(rune('a'+i)),
			Timestamp: base.Add(time.Duration(i) * time.Hour),
			Action:    AuditActionConfigChange,
			User:      "alice",
			Detail:    "change-" + string(rune('a'+i)),
			Success:   true,
		}
		if err := AppendAuditLog(log); err != nil {
			t.Fatalf("failed to append audit log %d: %v", i, err)
		}
		records = append(records, log)
	}
	return records
}

func TestReadAuditLogFile_PreservesFileOrder(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	SetHistoryDir(t.TempDir())

	appended := buildAuditChainForBundle(t, 3)

	logs, err := ReadAuditLogFile(GetAuditLogPath())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(logs))
	}
	for i := range logs {
		if logs[i].ID != appended[i].ID {
			t.Errorf("record %d: expected ID %s (file order), got %s", i, appended[i].ID, logs[i].ID)
		}
	}
}

func TestReadAuditLogFile_Empty(t *testing.T) {
	SetHistoryDir(t.TempDir())

	logs, err := ReadAuditLogFile(GetAuditLogPath())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected 0 records, got %d", len(logs))
	}
}

func TestVerifyAuditRecordChain_ValidAndBroken(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	SetHistoryDir(t.TempDir())

	buildAuditChainForBundle(t, 4)
	logs, err := ReadAuditLogFile(GetAuditLogPath())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valid, brokenAt, err := VerifyAuditRecordChain(logs)
	if err != nil || !valid {
		t.Fatalf("expected valid chain, got valid=%v brokenAt=%d err=%v", valid, brokenAt, err)
	}

	// Tamper with record 2 content: the recomputed HMAC must not match.
	logs[1].Detail = "tampered"
	valid, brokenAt, err = VerifyAuditRecordChain(logs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid || brokenAt != 2 {
		t.Errorf("expected broken chain at record 2, got valid=%v brokenAt=%d", valid, brokenAt)
	}

	// A middle record removal breaks the link at the following record.
	logs, _ = ReadAuditLogFile(GetAuditLogPath())
	removed := logs[:1:1]
	removed = append(removed, logs[2:]...)
	valid, brokenAt, err = VerifyAuditRecordChain(removed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid || brokenAt != 2 {
		t.Errorf("expected broken chain at record 2 after deletion, got valid=%v brokenAt=%d", valid, brokenAt)
	}
}

func TestBuildAuditBundle_Fields(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	SetHistoryDir(t.TempDir())

	buildAuditChainForBundle(t, 3)
	logs, err := ReadAuditLogFile(GetAuditLogPath())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	generated := time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)
	filter := &AuditBundleFilter{Since: "2026-08-15", Until: ""}
	bundle, err := BuildAuditBundle(logs, filter, generated)
	if err != nil {
		t.Fatalf("failed to build bundle: %v", err)
	}

	if bundle.Version != AuditBundleVersion {
		t.Errorf("expected version %d, got %d", AuditBundleVersion, bundle.Version)
	}
	if bundle.GeneratedAt != "2026-08-16T10:00:00Z" {
		t.Errorf("unexpected generated_at: %s", bundle.GeneratedAt)
	}
	if bundle.RecordCount != 3 || bundle.Manifest.Count != 3 {
		t.Errorf("expected count 3, got record_count=%d manifest.count=%d", bundle.RecordCount, bundle.Manifest.Count)
	}
	if bundle.TimeRange == nil {
		t.Fatal("expected non-nil time_range")
	}
	if bundle.TimeRange.From != logs[0].Timestamp.Format(time.RFC3339Nano) {
		t.Errorf("unexpected time_range.from: %s", bundle.TimeRange.From)
	}
	if bundle.TimeRange.To != logs[2].Timestamp.Format(time.RFC3339Nano) {
		t.Errorf("unexpected time_range.to: %s", bundle.TimeRange.To)
	}
	if bundle.HeadHash != logs[2].CurrHash {
		t.Errorf("head_hash should be the last record's curr_hash")
	}
	if bundle.Manifest.Filter == nil || bundle.Manifest.Filter.Since != "2026-08-15" {
		t.Errorf("manifest filter not preserved: %+v", bundle.Manifest.Filter)
	}

	// records_sha256 must be the SHA-256 of the canonical records JSON.
	canonical, err := json.Marshal(bundle.Records)
	if err != nil {
		t.Fatalf("failed to marshal records: %v", err)
	}
	digest := sha256.Sum256(canonical)
	if bundle.Manifest.RecordsSHA256 != hex.EncodeToString(digest[:]) {
		t.Errorf("records_sha256 mismatch")
	}

	// Signature must be the HMAC over the signing payload.
	payload, err := auditBundleSigningPayload(bundle.Version, bundle.GeneratedAt, bundle.HeadHash, bundle.Manifest)
	if err != nil {
		t.Fatalf("failed to build payload: %v", err)
	}
	mac := hmac.New(sha256.New, AuditHMACKey())
	mac.Write(payload)
	if bundle.Signature != hex.EncodeToString(mac.Sum(nil)) {
		t.Errorf("signature mismatch")
	}
}

func TestBuildAuditBundle_EmptyRecords(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")

	bundle, err := BuildAuditBundle(nil, nil, time.Now())
	if err != nil {
		t.Fatalf("failed to build bundle: %v", err)
	}
	if bundle.RecordCount != 0 {
		t.Errorf("expected 0 records, got %d", bundle.RecordCount)
	}
	if bundle.HeadHash != AuditZeroHash {
		t.Errorf("expected zero head_hash, got %s", bundle.HeadHash)
	}
	if bundle.TimeRange != nil {
		t.Errorf("expected nil time_range, got %+v", bundle.TimeRange)
	}
	// records must serialize as [] rather than null, and filter as null.
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("failed to marshal bundle: %v", err)
	}
	if !strings.Contains(string(data), `"records":[]`) {
		t.Errorf("expected records to marshal as [], got: %s", string(data))
	}
	if !strings.Contains(string(data), `"filter":null`) {
		t.Errorf("expected filter to marshal as null, got: %s", string(data))
	}
}

func TestWriteAndLoadAuditBundle_Roundtrip(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	SetHistoryDir(t.TempDir())

	buildAuditChainForBundle(t, 2)
	logs, err := ReadAuditLogFile(GetAuditLogPath())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bundle, err := BuildAuditBundle(logs, nil, time.Now())
	if err != nil {
		t.Fatalf("failed to build bundle: %v", err)
	}
	path := filepath.Join(t.TempDir(), "bundle.json")
	if err := WriteAuditBundle(bundle, path); err != nil {
		t.Fatalf("failed to write bundle: %v", err)
	}

	loaded, err := LoadAuditBundle(path)
	if err != nil {
		t.Fatalf("failed to load bundle: %v", err)
	}
	if loaded.RecordCount != bundle.RecordCount || loaded.HeadHash != bundle.HeadHash {
		t.Errorf("roundtrip mismatch: count=%d head=%s", loaded.RecordCount, loaded.HeadHash)
	}
	if loaded.Signature != bundle.Signature {
		t.Errorf("signature not preserved on roundtrip")
	}
	if err := VerifyAuditBundle(loaded); err != nil {
		t.Errorf("loaded bundle should verify: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat bundle: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected bundle file mode 0600, got %o", perm)
	}
}

func TestLoadAuditBundle_UnsupportedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	data := `{"version": 99, "records": []}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := LoadAuditBundle(path); err == nil || !strings.Contains(err.Error(), "unsupported bundle version") {
		t.Errorf("expected unsupported version error, got: %v", err)
	}
}

// reSignBundle recomputes manifest.records_sha256 and the signature after the
// test mutated bundle.Records, so the remaining failing check (chain replay)
// can be exercised in isolation.
func reSignBundle(t *testing.T, bundle *AuditBundle) {
	t.Helper()
	canonical, err := json.Marshal(bundle.Records)
	if err != nil {
		t.Fatalf("failed to marshal records: %v", err)
	}
	digest := sha256.Sum256(canonical)
	bundle.Manifest.RecordsSHA256 = hex.EncodeToString(digest[:])
	if err := signAuditBundle(bundle); err != nil {
		t.Fatalf("failed to re-sign: %v", err)
	}
}

func TestVerifyAuditBundle_OK(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	SetHistoryDir(t.TempDir())

	buildAuditChainForBundle(t, 3)
	logs, err := ReadAuditLogFile(GetAuditLogPath())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bundle, err := BuildAuditBundle(logs, nil, time.Now())
	if err != nil {
		t.Fatalf("failed to build bundle: %v", err)
	}
	if err := VerifyAuditBundle(bundle); err != nil {
		t.Errorf("expected bundle to verify, got: %v", err)
	}
}

func TestVerifyAuditBundle_TamperedRecord(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	SetHistoryDir(t.TempDir())

	buildAuditChainForBundle(t, 3)
	logs, err := ReadAuditLogFile(GetAuditLogPath())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bundle, err := BuildAuditBundle(logs, nil, time.Now())
	if err != nil {
		t.Fatalf("failed to build bundle: %v", err)
	}

	// Tamper with one record but keep the manifest digest: the recomputed
	// records_sha256 must mismatch.
	bundle.Records[1].Detail = "tampered"
	err = VerifyAuditBundle(bundle)
	if !errors.Is(err, ErrAuditBundleRecordsHash) {
		t.Errorf("expected ErrAuditBundleRecordsHash, got: %v", err)
	}
}

func TestVerifyAuditBundle_TamperedSignature(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	SetHistoryDir(t.TempDir())

	buildAuditChainForBundle(t, 2)
	logs, err := ReadAuditLogFile(GetAuditLogPath())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bundle, err := BuildAuditBundle(logs, nil, time.Now())
	if err != nil {
		t.Fatalf("failed to build bundle: %v", err)
	}

	bundle.Signature = strings.Repeat("f", 64)
	err = VerifyAuditBundle(bundle)
	if !errors.Is(err, ErrAuditBundleSignature) {
		t.Errorf("expected ErrAuditBundleSignature, got: %v", err)
	}
}

func TestVerifyAuditBundle_ChainMismatch(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	SetHistoryDir(t.TempDir())

	buildAuditChainForBundle(t, 3)
	logs, err := ReadAuditLogFile(GetAuditLogPath())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bundle, err := BuildAuditBundle(logs, nil, time.Now())
	if err != nil {
		t.Fatalf("failed to build bundle: %v", err)
	}

	// Swap two middle records, then fix digest + signature so ONLY the
	// chain replay check fails.
	bundle.Records[0], bundle.Records[1] = bundle.Records[1], bundle.Records[0]
	reSignBundle(t, bundle)
	err = VerifyAuditBundle(bundle)
	if !errors.Is(err, ErrAuditBundleChain) {
		t.Errorf("expected ErrAuditBundleChain after record swap, got: %v", err)
	}

	// Tamper with a record's content, then fix digest + signature: the
	// recomputed per-record HMAC must fail the replay.
	logs2, _ := ReadAuditLogFile(GetAuditLogPath())
	bundle2, err := BuildAuditBundle(logs2, nil, time.Now())
	if err != nil {
		t.Fatalf("failed to build bundle: %v", err)
	}
	bundle2.Records[1].Detail = "tampered"
	reSignBundle(t, bundle2)
	err = VerifyAuditBundle(bundle2)
	if !errors.Is(err, ErrAuditBundleChain) {
		t.Errorf("expected ErrAuditBundleChain after content tamper, got: %v", err)
	}
}

func TestVerifyAuditBundle_FilteredSlice(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	SetHistoryDir(t.TempDir())

	buildAuditChainForBundle(t, 4)
	logs, err := ReadAuditLogFile(GetAuditLogPath())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A time-filtered export is a contiguous chain slice: its first record's
	// prev_hash points at an excluded predecessor. The slice must verify.
	slice := logs[1:]
	bundle, err := BuildAuditBundle(slice, &AuditBundleFilter{Since: "2026-08-15"}, time.Now())
	if err != nil {
		t.Fatalf("failed to build bundle: %v", err)
	}
	if bundle.HeadHash != slice[len(slice)-1].CurrHash {
		t.Errorf("head_hash should be the slice tail's curr_hash")
	}
	if err := VerifyAuditBundle(bundle); err != nil {
		t.Errorf("filtered slice bundle should verify, got: %v", err)
	}

	// Tampering with a slice record and re-signing must still be caught by
	// the per-record HMAC replay.
	bundle.Records[0].Detail = "tampered"
	reSignBundle(t, bundle)
	err = VerifyAuditBundle(bundle)
	if !errors.Is(err, ErrAuditBundleChain) {
		t.Errorf("expected ErrAuditBundleChain for tampered slice, got: %v", err)
	}
}

func TestVerifyAuditBundle_HeadHashMismatch(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	SetHistoryDir(t.TempDir())

	buildAuditChainForBundle(t, 2)
	logs, err := ReadAuditLogFile(GetAuditLogPath())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bundle, err := BuildAuditBundle(logs, nil, time.Now())
	if err != nil {
		t.Fatalf("failed to build bundle: %v", err)
	}

	// Change head_hash to the first record's hash and re-sign: the chain is
	// intact but no longer agrees with the declared head.
	bundle.HeadHash = bundle.Records[0].CurrHash
	reSignBundle(t, bundle)
	err = VerifyAuditBundle(bundle)
	if !errors.Is(err, ErrAuditBundleChain) {
		t.Errorf("expected ErrAuditBundleChain for head_hash mismatch, got: %v", err)
	}
}

func TestVerifyAuditBundle_WrongKey(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	SetHistoryDir(t.TempDir())

	buildAuditChainForBundle(t, 2)
	logs, err := ReadAuditLogFile(GetAuditLogPath())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bundle, err := BuildAuditBundle(logs, nil, time.Now())
	if err != nil {
		t.Fatalf("failed to build bundle: %v", err)
	}

	// A different key cannot reproduce the signature.
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "other-key")
	err = VerifyAuditBundle(bundle)
	if !errors.Is(err, ErrAuditBundleSignature) {
		t.Errorf("expected ErrAuditBundleSignature with wrong key, got: %v", err)
	}
}

func TestVerifyAuditBundle_Empty(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")

	bundle, err := BuildAuditBundle(nil, nil, time.Now())
	if err != nil {
		t.Fatalf("failed to build bundle: %v", err)
	}
	if err := VerifyAuditBundle(bundle); err != nil {
		t.Errorf("empty bundle should verify, got: %v", err)
	}
}
