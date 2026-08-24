// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌​​​‌‌​​​‌‌​​​‌​‌​​‌​​​​​​‌‌‌‌​‌​​‌​‌‌​​‌​​​‌‌​​​​​​​​​​​​​​​​​​‌‌‌​​‌​​‌‌​‌​​​⁠
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
	"fmt"
	"os"
	"strings"
	"time"
)

// AuditZeroHash exposes the chain-start zero hash so callers (e.g. the audit
// bundle exporter) can reference the documented chain-head convention without
// duplicating the constant.
const AuditZeroHash = auditZeroHash

// AuditBundleVersion is the format version of the audit export bundle. The
// version is covered by the bundle signature and must match on verification.
const AuditBundleVersion = 1

// Sentinel errors reported by VerifyAuditBundle. Callers use errors.Is to
// determine which of the three integrity checks failed (signature,
// records digest, hash-chain replay including head_hash).
var (
	// ErrAuditBundleSignature indicates the HMAC signature over
	// (manifest + version + generated_at + head_hash) does not match.
	ErrAuditBundleSignature = errors.New("bundle signature verification failed")
	// ErrAuditBundleRecordsHash indicates the recomputed SHA-256 of the
	// canonical records array does not match manifest.records_sha256.
	ErrAuditBundleRecordsHash = errors.New("bundle records_sha256 mismatch")
	// ErrAuditBundleChain indicates the in-bundle hash-chain replay failed or
	// the last record's curr_hash does not match the bundle head_hash.
	ErrAuditBundleChain = errors.New("bundle hash chain verification failed")
)

// AuditBundleFilter records the --since/--until filter values that produced an
// export bundle. A nil filter (JSON null) means the bundle covers all records.
type AuditBundleFilter struct {
	Since string `json:"since,omitempty"`
	Until string `json:"until,omitempty"`
}

// AuditBundleTimeRange is the actual [min, max] timestamp range of the records
// included in a bundle. It is null when the bundle contains no records.
type AuditBundleTimeRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// AuditBundleManifest carries the integrity metadata of a bundle. It is fully
// covered by the bundle signature.
type AuditBundleManifest struct {
	RecordsSHA256 string             `json:"records_sha256"`
	Count         int                `json:"count"`
	Filter        *AuditBundleFilter `json:"filter"`
}

// AuditBundle is the single-file compliance export produced by
// "aflare audit export". Records are the verbatim audit log entries in chain
// (file) order; HeadHash is the curr_hash of the last record (the chain zero
// hash for an empty bundle).
type AuditBundle struct {
	Version     int                   `json:"version"`
	GeneratedAt string                `json:"generated_at"`
	RecordCount int                   `json:"record_count"`
	TimeRange   *AuditBundleTimeRange `json:"time_range"`
	HeadHash    string                `json:"head_hash"`
	Records     []AuditLog            `json:"records"`
	Manifest    AuditBundleManifest   `json:"manifest"`
	Signature   string                `json:"signature"`
}

// AuditHMACKey returns the audit HMAC key used to SIGN new material (bundle
// signatures), resolved with the same priority as the audit log writer
// itself (AFLARE_AUDIT_HMAC_KEY > PBKDF2 of AFLARE_SECRETS_PASSWORD >
// per-install key file > legacy default). Verification uses
// auditKeyCandidates instead, because a bundle may have been signed when a
// different key configuration was active.
func AuditHMACKey() []byte {
	if key := os.Getenv(auditEnvHMACKey); key != "" {
		return []byte(key)
	}
	if password := os.Getenv(auditEnvSecretsPasswd); password != "" {
		return deriveAuditKeyFromPassword(password)
	}
	if key, err := readAuditKeyFile(); err == nil && key != nil {
		return key
	}
	return []byte(auditDefaultKey)
}

// ReadAuditLogFile reads all audit log entries from path in file (chain)
// order, oldest first. Unlike ReadAuditLogs it does not re-sort by timestamp
// and it fails on malformed lines: callers are expected to have run
// VerifyAuditChain first, after which every line must parse. A missing file is
// reported as an empty slice.
func ReadAuditLogFile(path string) ([]AuditLog, error) {
	safePath, err := safeFilePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid audit log path: %w", err)
	}
	data, err := os.ReadFile(safePath) // #nosec G304 -- path validated by safeFilePath
	if err != nil {
		if os.IsNotExist(err) {
			return []AuditLog{}, nil
		}
		return nil, fmt.Errorf("failed to read audit log: %w", err)
	}

	logs := []AuditLog{}
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var log AuditLog
		if err := json.Unmarshal([]byte(line), &log); err != nil {
			return nil, fmt.Errorf("line %d: failed to parse audit record: %w", i+1, err)
		}
		logs = append(logs, log)
	}
	return logs, nil
}

// VerifyAuditRecordChain replays the HMAC hash chain over an in-memory record
// slice using the same rules as VerifyAuditChain: the first record's prev_hash
// must be the 64-hex zero hash, every record's prev_hash must equal the
// previous record's curr_hash, and every curr_hash must match the recomputed
// HMAC. brokenIndex is the 1-based position of the first broken record (0 when
// the slice is empty or the whole chain is valid); err is non-nil for format
// errors such as legacy records lacking hash fields.
func VerifyAuditRecordChain(records []AuditLog) (valid bool, brokenIndex int, err error) {
	return verifyAuditRecordChainFrom(records, auditZeroHash)
}

// verifyAuditRecordChainFrom replays the hash chain with a caller-supplied
// expected prev_hash for the first record; every later link uses the standard
// rules. Bundle verification uses it to replay contiguous chain slices whose
// first prev_hash legitimately points at a record outside the bundle. The
// replay tries every audit key candidate, so bundles signed under a since-
// rotated key configuration still verify.
func verifyAuditRecordChainFrom(records []AuditLog, firstExpectedPrev string) (valid bool, brokenIndex int, err error) {
	var firstMismatch struct {
		brokenIndex int
		hasResult   bool
	}
	for _, secret := range auditKeyCandidates() {
		expectedPrev := firstExpectedPrev
		broken := 0
		for i, entry := range records {
			if entry.PrevHash == "" && entry.CurrHash == "" {
				return false, i + 1, fmt.Errorf("record %d: incompatible format (missing prev_hash/curr_hash fields)", i+1)
			}
			if entry.PrevHash != expectedPrev {
				broken = i + 1
				break
			}
			savedHash := entry.CurrHash
			entry.CurrHash = ""
			computedHash, cerr := computeAuditHash(secret, entry)
			if cerr != nil {
				return false, i + 1, fmt.Errorf("record %d: %w", i+1, cerr)
			}
			if !hmac.Equal([]byte(computedHash), []byte(savedHash)) {
				broken = i + 1
				break
			}
			expectedPrev = savedHash
		}
		if broken == 0 {
			return true, 0, nil
		}
		if !firstMismatch.hasResult {
			firstMismatch.brokenIndex, firstMismatch.hasResult = broken, true
		}
	}
	if firstMismatch.hasResult {
		return false, firstMismatch.brokenIndex, nil
	}
	return true, 0, nil
}

// BuildAuditBundle assembles a signed export bundle from records already
// filtered by the caller. filter records the raw --since/--until values (nil
// when unfiltered) and is stored in the manifest verbatim. The records slice
// is copied, so later mutation of the caller's slice cannot alter the bundle.
func BuildAuditBundle(records []AuditLog, filter *AuditBundleFilter, generatedAt time.Time) (*AuditBundle, error) {
	// Copy to a non-nil slice so the JSON encoding is "[]" rather than null
	// and to decouple the bundle from caller mutations.
	out := make([]AuditLog, len(records))
	copy(out, records)

	canonical, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("failed to canonicalize records: %w", err)
	}
	digest := sha256.Sum256(canonical)

	bundle := &AuditBundle{
		Version:     AuditBundleVersion,
		GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		RecordCount: len(out),
		HeadHash:    AuditZeroHash,
		Records:     out,
		Manifest: AuditBundleManifest{
			RecordsSHA256: hex.EncodeToString(digest[:]),
			Count:         len(out),
			Filter:        filter,
		},
	}
	if len(out) > 0 {
		bundle.HeadHash = out[len(out)-1].CurrHash
		from, to := out[0].Timestamp, out[0].Timestamp
		for _, r := range out {
			if r.Timestamp.Before(from) {
				from = r.Timestamp
			}
			if r.Timestamp.After(to) {
				to = r.Timestamp
			}
		}
		bundle.TimeRange = &AuditBundleTimeRange{
			From: from.Format(time.RFC3339Nano),
			To:   to.Format(time.RFC3339Nano),
		}
	}

	if err := signAuditBundle(bundle); err != nil {
		return nil, err
	}
	return bundle, nil
}

// auditBundleSigningPayload builds the canonical JSON bytes covered by the
// bundle signature: {"generated_at", "head_hash", "manifest", "version"}.
// json.Marshal of a map emits keys in sorted order, which makes the encoding
// deterministic (canonical) for both signing and verification.
func auditBundleSigningPayload(version int, generatedAt, headHash string, manifest AuditBundleManifest) ([]byte, error) {
	payload := map[string]interface{}{
		"version":      version,
		"generated_at": generatedAt,
		"head_hash":    headHash,
		"manifest":     manifest,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signing payload: %w", err)
	}
	return data, nil
}

// signAuditBundle sets bundle.Signature to the HMAC-SHA256 (audit key) over
// the canonical signing payload, hex encoded.
func signAuditBundle(bundle *AuditBundle) error {
	payload, err := auditBundleSigningPayload(bundle.Version, bundle.GeneratedAt, bundle.HeadHash, bundle.Manifest)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, AuditHMACKey())
	mac.Write(payload)
	bundle.Signature = hex.EncodeToString(mac.Sum(nil))
	return nil
}

// WriteAuditBundle serializes the bundle as pretty-printed JSON to path with
// owner-only permissions (0600), suitable for regulatory submission archives.
func WriteAuditBundle(bundle *AuditBundle, path string) error {
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal bundle: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write bundle: %w", err)
	}
	return nil
}

// LoadAuditBundle reads and parses a bundle file, rejecting unsupported
// format versions.
func LoadAuditBundle(path string) (*AuditBundle, error) {
	safePath, err := safeFilePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid bundle path: %w", err)
	}
	data, err := os.ReadFile(safePath) // #nosec G304 -- path validated by safeFilePath
	if err != nil {
		return nil, fmt.Errorf("failed to read bundle: %w", err)
	}
	var bundle AuditBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("failed to parse bundle: %w", err)
	}
	if bundle.Version != AuditBundleVersion {
		return nil, fmt.Errorf("unsupported bundle version %d (expected %d)", bundle.Version, AuditBundleVersion)
	}
	if bundle.Records == nil {
		bundle.Records = []AuditLog{}
	}
	return &bundle, nil
}

// VerifyAuditBundle checks the three bundle integrity properties required for
// compliance submission, in order:
//  1. the HMAC signature over (manifest + version + generated_at + head_hash),
//  2. the SHA-256 of the canonical records array vs manifest.records_sha256,
//  3. the hash-chain replay over the records, including that the last
//     record's curr_hash equals the bundle head_hash.
//
// Chain-replay convention: an unfiltered bundle starts at the chain head, so
// its first record's prev_hash must be the zero hash (the live-chain rule).
// A time-filtered bundle is a contiguous slice of the chain: its first
// record's prev_hash points at the predecessor record that the filter
// excluded. That prev_hash value is itself authenticated by the record's
// curr_hash HMAC, so the replay links start from it and every internal link,
// every per-record HMAC and the head_hash are still fully verified.
//
// A non-nil error wraps one of ErrAuditBundleSignature,
// ErrAuditBundleRecordsHash or ErrAuditBundleChain so callers can report
// exactly which check failed.
func VerifyAuditBundle(bundle *AuditBundle) error {
	// 1. Signature over the signed container fields. Tried under every audit
	// key candidate: a bundle may have been signed while a different key
	// configuration (env/password/key-file rotation) was active.
	payload, err := auditBundleSigningPayload(bundle.Version, bundle.GeneratedAt, bundle.HeadHash, bundle.Manifest)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAuditBundleSignature, err)
	}
	sigOK := false
	for _, secret := range auditKeyCandidates() {
		mac := hmac.New(sha256.New, secret)
		mac.Write(payload)
		expectedSig := hex.EncodeToString(mac.Sum(nil))
		if hmac.Equal([]byte(expectedSig), []byte(bundle.Signature)) {
			sigOK = true
			break
		}
	}
	if !sigOK {
		return fmt.Errorf("%w: recomputed HMAC over (manifest+version+generated_at+head_hash) does not match the stored signature under any configured audit key", ErrAuditBundleSignature)
	}

	// 2. Records digest.
	canonical, err := json.Marshal(bundle.Records)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAuditBundleRecordsHash, err)
	}
	digest := sha256.Sum256(canonical)
	actualSHA := hex.EncodeToString(digest[:])
	if actualSHA != bundle.Manifest.RecordsSHA256 {
		return fmt.Errorf("%w: expected %s, got %s", ErrAuditBundleRecordsHash, bundle.Manifest.RecordsSHA256, actualSHA)
	}

	// 3. Chain replay plus head_hash agreement. Unfiltered bundles MUST start
	// at the chain head: a non-zero first prev_hash on an unfiltered bundle
	// means someone dropped leading records while claiming completeness
	// (truncation forgery) and must be rejected outright rather than replayed.
	// Filtered bundles are contiguous slices: their first record's prev_hash
	// points at the excluded predecessor and is itself HMAC-authenticated.
	firstExpected := auditZeroHash
	if len(bundle.Records) > 0 {
		if bundle.Manifest.Filter == nil {
			if bundle.Records[0].PrevHash != auditZeroHash {
				return fmt.Errorf("%w: unfiltered bundle does not start at the chain head (first prev_hash %s != zero hash); leading records may have been removed",
					ErrAuditBundleChain, bundle.Records[0].PrevHash)
			}
		} else if bundle.Records[0].PrevHash != "" {
			firstExpected = bundle.Records[0].PrevHash
		}
	}
	valid, brokenAt, err := verifyAuditRecordChainFrom(bundle.Records, firstExpected)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAuditBundleChain, err)
	}
	if !valid {
		return fmt.Errorf("%w: chain broken at record %d", ErrAuditBundleChain, brokenAt)
	}
	expectedHead := AuditZeroHash
	if len(bundle.Records) > 0 {
		expectedHead = bundle.Records[len(bundle.Records)-1].CurrHash
	}
	if expectedHead != bundle.HeadHash {
		return fmt.Errorf("%w: head_hash %s does not match last record curr_hash %s", ErrAuditBundleChain, bundle.HeadHash, expectedHead)
	}
	return nil
}
