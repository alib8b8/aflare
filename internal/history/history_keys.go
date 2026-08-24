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
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/pbkdf2"

	"github.com/alib8b8/aflare/internal/logger"
)

// Audit HMAC key configuration. Resolution order:
//  1. AFLARE_AUDIT_HMAC_KEY environment variable (raw bytes)
//  2. PBKDF2 derivation from AFLARE_SECRETS_PASSWORD
//  3. Auto-generated random key file <historyDir>/audit-hmac.key (0600),
//     created lazily for NEW chains only (see auditKeyForAppend)
//  4. Insecure built-in default — only to keep pre-0.9.0 default-key chains
//     appending/verifying; flagged by doctor and warned once per process
const (
	auditEnvHMACKey       = "AFLARE_AUDIT_HMAC_KEY"
	auditEnvSecretsPasswd = "AFLARE_SECRETS_PASSWORD"
	auditPBKDF2Salt       = "aflare-audit-hmac-salt-v1"
	auditPBKDF2Iterations = 100000
	auditPBKDF2KeyLen     = 32
	// auditDefaultKey is an insecure fallback used only when no key is configured.
	auditDefaultKey = "aflare-default-audit-hmac-key-v1"
	// auditKeyFileName holds the auto-generated per-install random key that
	// new chains sign with when no explicit key is configured. Its value is
	// never the public default, so fresh deployments are secure by default.
	auditKeyFileName = "audit-hmac.key"
)

var (
	// auditKeyCache caches the PBKDF2 result keyed by the input password so the
	// expensive derivation runs at most once per password value.
	auditKeyCacheMu   sync.Mutex
	auditKeyCachePass string
	auditKeyCacheKey  []byte
	// warnDefaultKeyOnce ensures the default-key warning is logged only once.
	warnDefaultKeyOnce sync.Once
)

// auditKeyFilePath returns the path of the auto-generated random key file in
// the history directory, or "" when no history directory is configured.
func auditKeyFilePath() string {
	dir := getHistoryDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, auditKeyFileName)
}

// readAuditKeyFile loads the auto-generated key file. A missing file yields
// (nil, nil); anything else (unreadable, wrong size) is an error because
// silently regenerating the key would fork the chain's verifiability.
func readAuditKeyFile() ([]byte, error) {
	path := auditKeyFilePath()
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path) // #nosec G304 -- internally generated history path
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read audit key file: %w", err)
	}
	key := bytes.TrimSpace(data)
	if len(key) == 0 {
		return nil, fmt.Errorf("audit key file %s is empty", path)
	}
	return key, nil
}

// ensureAuditKeyFile returns the auto-generated key, creating the key file on
// first use with a fresh 32-byte random key (hex encoded, 0600, atomic write).
func ensureAuditKeyFile() ([]byte, error) {
	if key, err := readAuditKeyFile(); err != nil || key != nil {
		return key, err
	}
	path := auditKeyFilePath()
	if path == "" {
		return nil, fmt.Errorf("history directory not available")
	}
	raw := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return nil, fmt.Errorf("failed to generate audit key: %w", err)
	}
	key := []byte(hex.EncodeToString(raw))
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, key, 0600); err != nil {
		return nil, fmt.Errorf("failed to write audit key file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, fmt.Errorf("failed to finalize audit key file: %w", err)
	}
	logger.Info("generated per-install audit HMAC key", "path", path)
	return key, nil
}

// auditKeyCandidates returns the HMAC keys a chain may have been signed with,
// in priority order: explicit env key, password-derived key, the per-install
// key file, and finally the public default (legacy pre-0.9.0 chains only).
// Duplicates are removed so verification never tries the same key twice.
func auditKeyCandidates() [][]byte {
	var candidates [][]byte
	seen := map[string]bool{}
	add := func(key []byte) {
		if len(key) == 0 {
			return
		}
		s := string(key)
		if seen[s] {
			return
		}
		seen[s] = true
		candidates = append(candidates, key)
	}
	if key := os.Getenv(auditEnvHMACKey); key != "" {
		add([]byte(key))
	}
	if password := os.Getenv(auditEnvSecretsPasswd); password != "" {
		add(deriveAuditKeyFromPassword(password))
	}
	if key, err := readAuditKeyFile(); err == nil {
		add(key)
	} else {
		logger.Warn("failed to read audit key file; continuing with other key candidates", "error", err.Error())
	}
	add([]byte(auditDefaultKey))
	return candidates
}

// auditKeyForAppend resolves the signing key for a new record appended to the
// audit log at auditPath:
//   - env key / password-derived key win when configured (explicit operator
//     intent, same as every version before 0.9.0);
//   - otherwise, if the log is missing or empty (a NEW chain), the per-install
//     random key file is generated and used — fresh deployments never sign
//     with the public default;
//   - otherwise (an existing legacy chain with records and no key file), the
//     chain must continue under the key it was started with, so the public
//     default is used with a warning; doctor explains how to migrate.
func auditKeyForAppend(auditPath string) ([]byte, error) {
	if key := os.Getenv(auditEnvHMACKey); key != "" {
		return []byte(key), nil
	}
	if password := os.Getenv(auditEnvSecretsPasswd); password != "" {
		return deriveAuditKeyFromPassword(password), nil
	}
	if key, err := readAuditKeyFile(); err != nil {
		return nil, err
	} else if key != nil {
		return key, nil
	}
	if fi, err := os.Stat(auditPath); err != nil || fi.Size() == 0 {
		// Missing or empty log: a new chain — generate and use a random key.
		return ensureAuditKeyFile()
	}
	warnDefaultKeyOnce.Do(func() {
		logger.Warn("audit chain was started with the public default HMAC key (pre-0.9.0 behavior); continuing with it to keep the chain verifiable. Set AFLARE_AUDIT_HMAC_KEY and export+rotate the chain to migrate.",
			"migration", "AFLARE_AUDIT_HMAC_KEY=$(openssl rand -hex 32) aflare audit export --out archive.json && <move audit.log aside>")
	})
	return []byte(auditDefaultKey), nil
}

// AuditKeyStatus reports how the audit HMAC key is (or will be) resolved, for
// doctor. usingDefaultKey is true when the ACTIVE chain key is the public
// default (forgeable by anyone reading the source).
func AuditKeyStatus() (configured bool, keyFileExists bool, usingDefaultKey bool) {
	if os.Getenv(auditEnvHMACKey) != "" || os.Getenv(auditEnvSecretsPasswd) != "" {
		return true, false, false
	}
	key, err := readAuditKeyFile()
	keyFileExists = err == nil && key != nil
	if keyFileExists {
		return true, true, false
	}
	// No explicit key and no key file: a new chain would generate one, but an
	// existing chain continues on the public default.
	if dir := getHistoryDir(); dir != "" {
		if fi, err := os.Stat(filepath.Join(dir, auditLogFileName)); err == nil && fi.Size() > 0 {
			return false, false, true
		}
	}
	return false, false, false
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
