// Copyright (c) 2026 aflare Contributors
//
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
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/emmansun/gmsm/sm3"

	"github.com/alib8b8/aflare/internal/logger"
)

// sm3CompatWarnOnce rate-limits the pre-0.9.0 incompatibility warning to
// one line per process, no matter how many records are appended.
var sm3CompatWarnOnce sync.Once

// SaveRecord saves a workflow execution record to the history directory
func SaveRecord(record Record) error {
	dir := getHistoryDir()
	if dir == "" {
		return fmt.Errorf("history directory not available")
	}

	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	if record.ID == "" {
		record.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	// Validate ID even though it is usually internally generated: some
	// callers (e.g. resume/wal) pass an explicit ID, and an unchecked value
	// like "../config/evil" would escape the history directory on write.
	if !isValidRecordID(record.ID) {
		return fmt.Errorf("invalid record ID: %q", record.ID)
	}

	filename := filepath.Join(dir, record.ID+".json")
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	return nil
}

// ClearHistory removes all history records
func ClearHistory() error {
	dir := getHistoryDir()
	if dir == "" {
		return fmt.Errorf("history directory not available")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read history directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name())) // best-effort cleanup
	}

	return nil
}

const (
	// auditLockFileName is the cross-process append lock (sentinel file).
	auditLockFileName = "audit.log.lock"
	// auditLockStale bounds how long an orphaned lock (crashed holder) blocks
	// appends before being reclaimed.
	auditLockStale = 30 * time.Second
	// auditLockWait is how long an append waits to acquire the lock before
	// failing loudly rather than risking a chain fork.
	auditLockWait = 5 * time.Second
)

// Audit MAC algorithm identifiers (AuditLog.MACAlgo). The algorithm for newly
// appended records is selected via AFLARE_AUDIT_HMAC_ALGO; verification always
// follows the mac_algo field stored in each record.
const (
	auditMACSHA256 = "sha256"
	auditMACSM3    = "sm3"
)

// auditEnvHMACAlgo selects the MAC algorithm for new audit records
// ("sha256", the default, or "sm3").
const auditEnvHMACAlgo = "AFLARE_AUDIT_HMAC_ALGO"

// resolveAuditMACAlgo maps an AFLARE_AUDIT_HMAC_ALGO value to a MAC algorithm
// name. Empty and "sha256" map to sha256; "sm3" maps to sm3. Unknown values are
// rejected so a typo cannot silently downgrade the chain to the default.
func resolveAuditMACAlgo(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", auditMACSHA256:
		return auditMACSHA256, nil
	case auditMACSM3:
		return auditMACSM3, nil
	default:
		return "", fmt.Errorf("invalid %s value %q (want %q or %q)",
			auditEnvHMACAlgo, value, auditMACSHA256, auditMACSM3)
	}
}

// computeAuditHash returns curr_hash = hex(HMAC(secret, prev_hash || record_content))
// using the algorithm named by entry.MACAlgo: HMAC-SHA256 by default (also for
// legacy records whose mac_algo field is absent) or HMAC-SM3 for the Chinese
// national cryptography suite. record_content is the JSON encoding of the entry
// with CurrHash cleared. The caller must ensure entry.CurrHash is empty before
// calling.
func computeAuditHash(secret []byte, entry AuditLog) (string, error) {
	if entry.CurrHash != "" {
		return "", fmt.Errorf("entry must not have CurrHash set when computing hash")
	}
	var newHash func() hash.Hash
	switch entry.MACAlgo {
	case "", auditMACSHA256:
		newHash = sha256.New
	case auditMACSM3:
		newHash = sm3.New
	default:
		return "", fmt.Errorf("unknown audit MAC algorithm %q", entry.MACAlgo)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("failed to marshal entry for hashing: %w", err)
	}
	mac := hmac.New(newHash, secret)
	mac.Write([]byte(entry.PrevHash))
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// readLastAuditHash returns the curr_hash of the last non-empty line in the audit
// log file. It seeks near the end of the file rather than reading the whole file.
// Returns auditZeroHash when the file is missing or empty. When the 8 KiB tail
// window fails to parse (a record larger than the window, or a torn final line
// from a crashed writer), it falls back to scanning the full file: oversized
// records must not block future appends, while a genuinely torn line still
// fails loudly instead of silently forking the chain.
func readLastAuditHash(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304 -- internally generated history path
	if err != nil {
		if os.IsNotExist(err) {
			return auditZeroHash, nil
		}
		return "", err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := stat.Size()
	if size == 0 {
		return auditZeroHash, nil
	}

	// Read a trailing chunk large enough to contain the last record. Audit
	// entries are small JSON lines; 8 KiB is ample for typical records.
	bufSize := int64(8192)
	if bufSize > size {
		bufSize = size
	}
	buf := make([]byte, bufSize)
	if _, err := f.ReadAt(buf, size-bufSize); err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read tail of audit log: %w", err)
	}

	// Walk the trailing lines backwards to find the last non-empty record.
	lastHash, tailErr := lastHashFromTail(string(buf))
	if tailErr == nil {
		return lastHash, nil
	}

	// Fallback: rescan the whole file. The first line of the tail window may
	// be a record's middle, so only the full scan can distinguish "oversized
	// record" (recovers, continues the chain) from "torn last line" (error).
	data, err := os.ReadFile(path) // #nosec G304 -- internally generated history path
	if err != nil {
		return "", fmt.Errorf("failed to reread audit log: %w", err)
	}
	fullHash, fullErr := lastHashFromTail(string(data))
	if fullErr != nil {
		return "", fmt.Errorf("failed to parse last audit log line (torn tail? back up the file and truncate to the last complete record): %w", fullErr)
	}
	return fullHash, nil
}

// lastHashFromTail walks lines backwards and returns the curr_hash of the last
// parseable non-empty record, or an error when the last non-empty line cannot
// be parsed.
func lastHashFromTail(content string) (string, error) {
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var entry AuditLog
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return "", fmt.Errorf("failed to parse last audit log line: %w", err)
		}
		if entry.CurrHash == "" {
			// Legacy record without a hash: treat as the start of a new chain.
			return auditZeroHash, nil
		}
		return entry.CurrHash, nil
	}
	return auditZeroHash, nil
}

var auditWriteMu sync.Mutex

// AppendAuditLog appends an audit log entry to the audit log file. Each entry is
// bound to the previous one via an HMAC hash chain (SHA-256 by default, or SM3
// when AFLARE_AUDIT_HMAC_ALGO=sm3) so that tampering or deletion can be detected
// by VerifyAuditChain. The chaining semantics are identical for both algorithms:
// regardless of which algorithm a record uses, its prev_hash is the previous
// record's curr_hash, so algorithm switches never break the chain.
func AppendAuditLog(log AuditLog) error {
	dir := getHistoryDir()
	if dir == "" {
		return fmt.Errorf("history directory not available")
	}

	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	if log.ID == "" {
		log.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	// The MAC algorithm for new records follows AFLARE_AUDIT_HMAC_ALGO and is
	// stored per record; verification reads it back from each record. The
	// default sha256 clears the field so records stay byte-identical to
	// pre-0.9.0 output (whose readers recompute with HMAC-SHA256 anyway).
	algo, err := resolveAuditMACAlgo(os.Getenv(auditEnvHMACAlgo))
	if err != nil {
		return fmt.Errorf("failed to select audit MAC algorithm: %w", err)
	}
	if algo == auditMACSM3 {
		log.MACAlgo = algo
		sm3CompatWarnOnce.Do(func() {
			logger.Warn("audit records are being signed with SM3; binaries before 0.9.0 cannot verify this chain",
				"env", auditEnvHMACAlgo,
				"note", "upgrade all binaries before enabling guomi")
		})
	} else {
		log.MACAlgo = ""
	}

	auditPath := filepath.Join(dir, auditLogFileName)

	// Serialize the read-then-write append under a mutex so concurrent callers
	// within this process extend the chain rather than corrupting it, and
	// under a cross-process sentinel lock so parallel aflare processes do too.
	auditWriteMu.Lock()
	defer auditWriteMu.Unlock()

	unlock, err := lockAuditLog(dir)
	if err != nil {
		return err
	}
	defer unlock()

	prevHash, err := readLastAuditHash(auditPath)
	if err != nil {
		return fmt.Errorf("failed to read previous audit hash: %w", err)
	}
	log.PrevHash = prevHash

	key, err := auditKeyForAppend(auditPath)
	if err != nil {
		return fmt.Errorf("failed to resolve audit HMAC key: %w", err)
	}

	// CurrHash must be empty while computing the hash; it is set afterwards.
	log.CurrHash = ""
	currHash, err := computeAuditHash(key, log)
	if err != nil {
		return fmt.Errorf("failed to compute audit hash: %w", err)
	}
	log.CurrHash = currHash

	data, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal audit log: %w", err)
	}

	f, err := os.OpenFile(auditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) // #nosec G304 -- internally generated history path
	if err != nil {
		return fmt.Errorf("failed to open audit log file: %w", err)
	}
	defer f.Close()

	// The 0600 mode only applies to newly created files; tighten pre-existing
	// files too (audit records carry user/action detail and must stay
	// owner-only). Best effort: a failure to chmod is not worth failing the
	// audit write for.
	if err := f.Chmod(0600); err != nil {
		logger.Warn("failed to tighten audit log permissions", "error", err.Error())
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write audit log: %w", err)
	}

	return nil
}

// lockAuditLog acquires a cross-process sentinel lock around the audit
// read-hash-append critical section, using the same O_CREATE|O_EXCL pattern
// as the idempotency store. A lock older than auditLockStale (crashed holder)
// is reclaimed. The returned func releases the lock and must be called.
func lockAuditLog(dir string) (func(), error) {
	lockPath := filepath.Join(dir, auditLockFileName)
	deadline := time.Now().Add(auditLockWait)
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600) // #nosec G304 -- internally generated history path
		if err == nil {
			_, werr := f.WriteString(time.Now().UTC().Format(time.RFC3339))
			cerr := f.Close()
			if werr != nil {
				return nil, fmt.Errorf("failed to write audit lock: %w", werr)
			}
			if cerr != nil {
				return nil, fmt.Errorf("failed to write audit lock: %w", cerr)
			}
			return func() {
				if rerr := os.Remove(lockPath); rerr != nil && !os.IsNotExist(rerr) {
					logger.Warn("failed to release audit lock", "error", rerr.Error())
				}
			}, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("failed to acquire audit lock: %w", err)
		}
		// Lock exists: reclaim it if stale, otherwise retry until the deadline.
		if fi, serr := os.Stat(lockPath); serr == nil && time.Since(fi.ModTime()) > auditLockStale {
			if rerr := os.Remove(lockPath); rerr == nil || os.IsNotExist(rerr) {
				continue
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("audit log is locked by another process (lock %s); retry shortly", lockPath)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
