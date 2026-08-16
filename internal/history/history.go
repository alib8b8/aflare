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
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emmansun/gmsm/sm3"
	"golang.org/x/crypto/pbkdf2"

	"github.com/alib8b8/aflare/internal/logger"
)

// TriggerType represents the type of trigger that started the workflow
type TriggerType string

const (
	TriggerManual   TriggerType = "manual"
	TriggerCLI      TriggerType = "cli"
	TriggerAPI      TriggerType = "api"
	TriggerSchedule TriggerType = "schedule"
)

// Record represents a single workflow execution record
type Record struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Path        string        `json:"path,omitempty"`
	Trigger     TriggerType   `json:"trigger,omitempty"`
	User        string        `json:"user,omitempty"`
	Version     string        `json:"version,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
	StartedAt   time.Time     `json:"started_at"`
	EndedAt     time.Time     `json:"ended_at,omitempty"`
	Success     bool          `json:"success"`
	Steps       []StepRecord  `json:"steps,omitempty"`
	FinalOutput string        `json:"final_output,omitempty"`
	Error       string        `json:"error,omitempty"`
}

// StepRecord represents a single step execution record
type StepRecord struct {
	Index      int           `json:"index"`
	Node       string        `json:"node"`
	Params     string        `json:"params,omitempty"`
	RetryCount int           `json:"retry_count,omitempty"`
	InputSize  int           `json:"input_size,omitempty"`
	OutputSize int           `json:"output_size,omitempty"`
	Duration   time.Duration `json:"duration"`
	Success    bool          `json:"success"`
	Error      string        `json:"error,omitempty"`
}

var (
	historyDir   string
	historyDirMu sync.RWMutex
)

func init() {
	home, err := os.UserHomeDir()
	if err == nil {
		historyDir = filepath.Join(home, ".config", "aflare", "history")
	}
}

// SetHistoryDir sets a custom history directory (useful for tests)
func SetHistoryDir(dir string) {
	historyDirMu.Lock()
	defer historyDirMu.Unlock()
	historyDir = dir
}

// getHistoryDir returns the current history directory under a read lock.
func getHistoryDir() string {
	historyDirMu.RLock()
	defer historyDirMu.RUnlock()
	return historyDir
}

// AuditLogPath returns the path of the append-only audit hash-chain log in
// the history directory, or "" when no history directory is configured.
// Read-only consumers (e.g. doctor) use this plus ReadAuditLogFile.
func AuditLogPath() string {
	dir := getHistoryDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, auditLogFileName)
}

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

// ListRecords returns all history records, sorted by time (newest first)
func ListRecords() ([]Record, error) {
	dir := getHistoryDir()
	if dir == "" {
		return nil, fmt.Errorf("history directory not available")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Record{}, nil
		}
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}

	var records []Record
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path) // #nosec G304 -- internally generated history path
		if err != nil {
			continue
		}

		var record Record
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}
		records = append(records, record)
	}

	// Sort by started_at descending (newest first)
	for i := 0; i < len(records)-1; i++ {
		for j := i + 1; j < len(records); j++ {
			if records[j].StartedAt.After(records[i].StartedAt) {
				records[i], records[j] = records[j], records[i]
			}
		}
	}

	return records, nil
}

// GetRecord retrieves a single history record by ID
func GetRecord(id string) (*Record, error) {
	dir := getHistoryDir()
	if dir == "" {
		return nil, fmt.Errorf("history directory not available")
	}

	// Validate ID to prevent path traversal (e.g. id="../config")
	if !isValidRecordID(id) {
		return nil, fmt.Errorf("invalid record ID: %q", id)
	}

	path := filepath.Join(dir, id+".json")
	data, err := os.ReadFile(path) // #nosec G304 -- internally generated history path
	if err != nil {
		return nil, fmt.Errorf("failed to read record: %w", err)
	}

	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("failed to parse record: %w", err)
	}

	return &record, nil
}

// isValidRecordID ensures the ID is safe to use as a filename component.
// It rejects empty IDs, path separators, and dot-segments like "..".
func isValidRecordID(id string) bool {
	if id == "" || len(id) > 100 {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
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

var sensitiveKeywords = []string{"key", "token", "secret", "password"}

// SanitizeParams sanitizes parameter values to remove sensitive information.
// It replaces values of fields containing key/token/secret/password with ***.
func SanitizeParams(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return nil
	}
	result := make(map[string]interface{}, len(params))
	for k, v := range params {
		if isSensitiveKey(k) {
			result[k] = "***"
		} else {
			switch val := v.(type) {
			case map[string]interface{}:
				result[k] = SanitizeParams(val)
			default:
				result[k] = v
			}
		}
	}
	return result
}

func isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	for _, kw := range sensitiveKeywords {
		if strings.Contains(lowerKey, kw) {
			return true
		}
	}
	return false
}

// SummarizeParams creates a sanitized, truncated string summary of parameters.
// The summary is truncated to maxLen characters (200 by default if maxLen <= 0).
func SummarizeParams(params map[string]interface{}, maxLen int) string {
	if params == nil {
		return ""
	}
	if maxLen <= 0 {
		maxLen = 200
	}
	sanitized := SanitizeParams(params)
	data, err := json.Marshal(sanitized)
	if err != nil {
		return ""
	}
	summary := string(data)
	if len(summary) > maxLen {
		summary = summary[:maxLen] + "..."
	}
	return summary
}

// AuditAction represents the type of action being audited
type AuditAction string

const (
	AuditActionLogin         AuditAction = "login"
	AuditActionLogout        AuditAction = "logout"
	AuditActionConfigChange  AuditAction = "config_change"
	AuditActionSensitiveOp   AuditAction = "sensitive_operation"
	AuditActionWorkflowStart AuditAction = "workflow_start"
	AuditActionWorkflowEnd   AuditAction = "workflow_end"
)

// AuditLog represents a single audit log entry
type AuditLog struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	Action    AuditAction `json:"action"`
	User      string      `json:"user,omitempty"`
	Resource  string      `json:"resource,omitempty"`
	Detail    string      `json:"detail,omitempty"`
	Success   bool        `json:"success"`
	IP        string      `json:"ip,omitempty"`
	UserAgent string      `json:"user_agent,omitempty"`
	// PrevHash is the curr_hash of the previous record in the hash chain.
	// It is the 64-char hex zero hash for the first record.
	PrevHash string `json:"prev_hash,omitempty"`
	// CurrHash is the HMAC of (prev_hash || record_content) for this record.
	CurrHash string `json:"curr_hash,omitempty"`
	// MACAlgo names the HMAC algorithm used to compute CurrHash:
	// "sha256" (default; legacy records omit the field) or "sm3" for the
	// Chinese national cryptography suite. Verification recomputes each
	// record with its own algorithm, so mixed-algorithm chains stay valid.
	MACAlgo string `json:"mac_algo,omitempty"`
}

const auditLogFileName = "audit.log.jsonl"

// auditZeroHash is the 32-byte zero value encoded as 64 hex chars, used as the
// prev_hash of the first record in the chain.
const auditZeroHash = "0000000000000000000000000000000000000000000000000000000000000000"

// Audit HMAC key configuration. Priority:
//  1. AFLARE_AUDIT_HMAC_KEY environment variable (raw bytes)
//  2. PBKDF2 derivation from AFLARE_SECRETS_PASSWORD
//  3. Insecure built-in default (logged once at warn level)
const (
	auditEnvHMACKey       = "AFLARE_AUDIT_HMAC_KEY"
	auditEnvSecretsPasswd = "AFLARE_SECRETS_PASSWORD"
	auditPBKDF2Salt       = "aflare-audit-hmac-salt-v1"
	auditPBKDF2Iterations = 100000
	auditPBKDF2KeyLen     = 32
	// auditDefaultKey is an insecure fallback used only when no key is configured.
	auditDefaultKey = "aflare-default-audit-hmac-key-v1"
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

var (
	auditWriteMu sync.Mutex
	// auditKeyCache caches the PBKDF2 result keyed by the input password so the
	// expensive derivation runs at most once per password value.
	auditKeyCacheMu   sync.Mutex
	auditKeyCachePass string
	auditKeyCacheKey  []byte
	// warnDefaultKeyOnce ensures the default-key warning is logged only once.
	warnDefaultKeyOnce sync.Once
)

// getAuditHMACKey returns the HMAC key used to bind audit records into a chain.
// The key is resolved on every call so environment changes are picked up; the
// PBKDF2 derivation result is cached per password.
func getAuditHMACKey() []byte {
	if key := os.Getenv(auditEnvHMACKey); key != "" {
		return []byte(key)
	}
	if password := os.Getenv(auditEnvSecretsPasswd); password != "" {
		return deriveAuditKeyFromPassword(password)
	}
	warnDefaultKeyOnce.Do(func() {
		logger.Warn("audit HMAC key not configured; using insecure default. Set AFLARE_AUDIT_HMAC_KEY for production use.")
	})
	return []byte(auditDefaultKey)
}

// deriveAuditKeyFromPassword derives a 32-byte HMAC key from the secrets master
// password using PBKDF2-SHA256. The result is cached per password.
func deriveAuditKeyFromPassword(password string) []byte {
	auditKeyCacheMu.Lock()
	defer auditKeyCacheMu.Unlock()
	if auditKeyCachePass == password && len(auditKeyCacheKey) > 0 {
		return auditKeyCacheKey
	}
	key := pbkdf2.Key([]byte(password), []byte(auditPBKDF2Salt), auditPBKDF2Iterations, auditPBKDF2KeyLen, sha256.New)
	auditKeyCachePass = password
	auditKeyCacheKey = key
	return key
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
// Returns auditZeroHash when the file is missing or empty.
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
	lines := strings.Split(string(buf), "\n")
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
	// within this process extend the chain rather than corrupting it.
	auditWriteMu.Lock()
	defer auditWriteMu.Unlock()

	prevHash, err := readLastAuditHash(auditPath)
	if err != nil {
		return fmt.Errorf("failed to read previous audit hash: %w", err)
	}
	log.PrevHash = prevHash

	// CurrHash must be empty while computing the hash; it is set afterwards.
	log.CurrHash = ""
	currHash, err := computeAuditHash(getAuditHMACKey(), log)
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

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write audit log: %w", err)
	}

	return nil
}

// GetAuditLogPath returns the path to the audit log file in the current history
// directory. The directory may be empty if history is unavailable.
func GetAuditLogPath() string {
	dir := getHistoryDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, auditLogFileName)
}

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

	secret := getAuditHMACKey()
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
			return false, lineNum, fmt.Errorf("line %d: failed to parse record: %w", lineNum, err)
		}
		// Backwards compatibility: legacy records without hash fields cannot be
		// verified and must be reported explicitly rather than crashing.
		if entry.PrevHash == "" && entry.CurrHash == "" {
			return false, lineNum, fmt.Errorf("line %d: incompatible format (missing prev_hash/curr_hash fields)", lineNum)
		}
		if entry.PrevHash != expectedPrev {
			return false, lineNum, nil
		}
		savedHash := entry.CurrHash
		entry.CurrHash = ""
		computedHash, err := computeAuditHash(secret, entry)
		if err != nil {
			return false, lineNum, fmt.Errorf("line %d: %w", lineNum, err)
		}
		if !hmac.Equal([]byte(computedHash), []byte(savedHash)) {
			return false, lineNum, nil
		}
		expectedPrev = savedHash
	}
	if err := scanner.Err(); err != nil {
		return false, lineNum, fmt.Errorf("failed to read audit log: %w", err)
	}
	return true, 0, nil
}

// RecordFilter defines filters for listing records
type RecordFilter struct {
	StartTime *time.Time
	EndTime   *time.Time
	Success   *bool
	Workflow  string
}

// ListRecordsWithFilter returns history records filtered by the given criteria
func ListRecordsWithFilter(filter RecordFilter) ([]Record, error) {
	records, err := ListRecords()
	if err != nil {
		return nil, err
	}

	var filtered []Record
	for _, r := range records {
		if filter.StartTime != nil && r.StartedAt.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && r.StartedAt.After(*filter.EndTime) {
			continue
		}
		if filter.Success != nil && r.Success != *filter.Success {
			continue
		}
		if filter.Workflow != "" && !strings.Contains(strings.ToLower(r.Name), strings.ToLower(filter.Workflow)) {
			continue
		}
		filtered = append(filtered, r)
	}

	return filtered, nil
}

// Stats contains execution statistics
type Stats struct {
	TotalCount      int           `json:"total_count"`
	SuccessCount    int           `json:"success_count"`
	FailureCount    int           `json:"failure_count"`
	SuccessRate     float64       `json:"success_rate"`
	AverageDuration time.Duration `json:"average_duration"`
	Last24hCount    int           `json:"last_24h_count"`
}

// GetStats returns execution statistics based on all records
func GetStats() (Stats, error) {
	records, err := ListRecords()
	if err != nil {
		return Stats{}, err
	}

	stats := Stats{
		TotalCount: len(records),
	}

	if len(records) == 0 {
		return stats, nil
	}

	var totalDuration time.Duration
	now := time.Now()
	cutoff24h := now.Add(-24 * time.Hour)

	for _, r := range records {
		if r.Success {
			stats.SuccessCount++
		} else {
			stats.FailureCount++
		}
		if r.Duration > 0 {
			totalDuration += r.Duration
		} else if !r.EndedAt.IsZero() && !r.StartedAt.IsZero() {
			totalDuration += r.EndedAt.Sub(r.StartedAt)
		}
		if r.StartedAt.After(cutoff24h) {
			stats.Last24hCount++
		}
	}

	stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalCount)

	var countWithDuration int
	for _, r := range records {
		if r.Duration > 0 || (!r.EndedAt.IsZero() && !r.StartedAt.IsZero()) {
			countWithDuration++
		}
	}
	if countWithDuration > 0 {
		stats.AverageDuration = totalDuration / time.Duration(countWithDuration)
	}

	return stats, nil
}

// ReadAuditLogs reads all audit log entries
func ReadAuditLogs() ([]AuditLog, error) {
	dir := getHistoryDir()
	if dir == "" {
		return nil, fmt.Errorf("history directory not available")
	}

	auditPath := filepath.Join(dir, auditLogFileName)
	data, err := os.ReadFile(auditPath) // #nosec G304 -- internally generated history path
	if err != nil {
		if os.IsNotExist(err) {
			return []AuditLog{}, nil
		}
		return nil, fmt.Errorf("failed to read audit log: %w", err)
	}

	var logs []AuditLog
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var log AuditLog
		if err := json.Unmarshal([]byte(line), &log); err != nil {
			continue
		}
		logs = append(logs, log)
	}

	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Timestamp.After(logs[j].Timestamp)
	})

	return logs, nil
}
