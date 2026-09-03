// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​​​‌​‌​​‌​‌​​​‌​​‌​​‌‌‌​​​‌​​​​​​‌‌‌‌​​‌‌​​‌‌‌​​​​​​​​​​​​​​​​​​​​​‌​‌‌​​​‌​‌‌​⁠
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

package secrets

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emmansun/gmsm/sm4"
	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"

	"github.com/alib8b8/aflare/internal/fsutil"
	"github.com/alib8b8/aflare/internal/logger"
)

// Cipher suite identifiers for the at-rest encryption of the secrets store.
// The suite is selected via AFLARE_SECRETS_CIPHER (aes-gcm by default).
const (
	CipherAESGCM = "aes-gcm"
	CipherSM4GCM = "sm4-gcm"
)

// Key-derivation function names for the at-rest encryption of the secrets
// store. New writes default to Argon2id (memory-hard: GPU cracking costs an
// order of magnitude more than PBKDF2). PBKDF2 stays selectable so shared
// fleets can pin the legacy KDF — and with aes-gcm, the byte-compatible
// headerless format — via AFLARE_SECRETS_KDF=pbkdf2.
const (
	KDFArgon2id = "argon2id"
	KDFPBKDF2   = "pbkdf2"
)

// On-disk format. Versioned files start with a header
// (magic "AFLSEC" + version byte + cipher ID byte [+ KDF ID byte for v2])
// followed by the 16-byte KDF salt and the AEAD ciphertext
// (nonce || ciphertext+tag). Legacy files written before the header existed
// start directly with the salt and are always decrypted as
// AES-256-GCM + PBKDF2.
//
//	v1 (8 bytes): magic + 0x01 + cipherID          — PBKDF2 implied
//	v2 (9 bytes): magic + 0x02 + cipherID + kdfID  — KDF from header
const (
	secretsEnvCipher = "AFLARE_SECRETS_CIPHER"
	secretsEnvKDF    = "AFLARE_SECRETS_KDF"
	secretsMagic     = "AFLSEC"
	secretsVersion1  = 0x01
	secretsVersion2  = 0x02
	secretsHdrSizeV1 = 8
	secretsHdrSizeV2 = 9
	cipherIDAESGCM   = 0x01
	cipherIDSM4GCM   = 0x02
	kdfIDPBKDF2      = 0x01
	kdfIDArgon2id    = 0x02
)

const (
	SecretTypeNormal = "normal"
	SecretTypeSecret = "secret" // #nosec G101 -- type label constant, not a credential value
	pbkdf2Iterations = 600000
	pbkdf2SaltSize   = 16
)

// Argon2id work parameters: the OWASP-recommended interactive profile
// (64 MiB, 1 pass, 2 lanes) — ~50ms on a laptop, prohibitive on a GPU rig.
// Package-level variables so tests can dial them down; production code
// must not modify them.
var (
	argon2Time    = 1
	argon2Memory  = 64 * 1024 // KiB
	argon2Threads = 2
)

var defaultSecretsPath string

// sm4CompatWarnOnce rate-limits the pre-0.9.0 incompatibility warning to
// one line per process, no matter how often the store is saved.
var sm4CompatWarnOnce sync.Once

// kdfUpgradeWarnOnce rate-limits the pbkdf2→argon2id upgrade notice to one
// line per process, mirroring sm4CompatWarnOnce.
var kdfUpgradeWarnOnce sync.Once

func init() {
	if home, err := os.UserHomeDir(); err == nil {
		defaultSecretsPath = filepath.Join(home, ".config", "aflare", "secrets.dat")
	}
}

type Secret struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Type        string    `json:"type"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SecretGroup struct {
	Name    string            `json:"name"`
	Secrets map[string]Secret `json:"secrets"`
}

// SecretManager keeps the in-memory secret groups plus the crypto material
// needed to persist them. The master password, cipher name, and KDF name are
// retained so SaveToFile can re-derive a key when the configured cipher or
// KDF differs from the ones the store was loaded with.
type SecretManager struct {
	mu             sync.RWMutex
	groups         map[string]*SecretGroup
	aead           cipher.AEAD
	salt           []byte
	masterPassword string
	cipherName     string
	kdfName        string
}

// cipherKeySize returns the PBKDF2 output length required by the cipher:
// 32 bytes for AES-256, 16 bytes for SM4 (128-bit block/key cipher).
func cipherKeySize(cipherName string) (int, error) {
	switch cipherName {
	case CipherAESGCM:
		return 32, nil
	case CipherSM4GCM:
		return 16, nil
	default:
		return 0, fmt.Errorf("unsupported secrets cipher: %s", cipherName)
	}
}

// resolveSecretsCipher maps an AFLARE_SECRETS_CIPHER value to a cipher name.
// Empty and "aes-gcm" map to aes-gcm; "sm4-gcm" maps to sm4-gcm. Unknown
// values are rejected so a typo cannot silently re-encrypt the store with an
// unintended (or default) cipher.
func resolveSecretsCipher(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", CipherAESGCM:
		return CipherAESGCM, nil
	case CipherSM4GCM:
		return CipherSM4GCM, nil
	default:
		return "", fmt.Errorf("invalid %s value %q (want %q or %q)",
			secretsEnvCipher, value, CipherAESGCM, CipherSM4GCM)
	}
}

// configuredCipher returns the cipher selected by AFLARE_SECRETS_CIPHER.
func configuredCipher() (string, error) {
	return resolveSecretsCipher(os.Getenv(secretsEnvCipher))
}

// resolveSecretsKDF maps an AFLARE_SECRETS_KDF value to a KDF name. Empty
// and "argon2id" map to argon2id (the memory-hard default); "pbkdf2" pins
// the legacy KDF. Unknown values are rejected so a typo cannot silently
// downgrade the store's key derivation.
func resolveSecretsKDF(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", KDFArgon2id:
		return KDFArgon2id, nil
	case KDFPBKDF2:
		return KDFPBKDF2, nil
	default:
		return "", fmt.Errorf("invalid %s value %q (want %q or %q)",
			secretsEnvKDF, value, KDFArgon2id, KDFPBKDF2)
	}
}

// configuredKDF returns the KDF selected by AFLARE_SECRETS_KDF.
func configuredKDF() (string, error) {
	return resolveSecretsKDF(os.Getenv(secretsEnvKDF))
}

// kdfNameByID maps an on-disk KDF identifier byte back to a KDF name.
func kdfNameByID(id byte) (string, error) {
	switch id {
	case kdfIDPBKDF2:
		return KDFPBKDF2, nil
	case kdfIDArgon2id:
		return KDFArgon2id, nil
	default:
		return "", fmt.Errorf("unknown kdf id %d in secrets file header", id)
	}
}

// cipherIDByName returns the on-disk cipher identifier byte for a cipher name.
func cipherIDByName(cipherName string) (byte, error) {
	switch cipherName {
	case CipherAESGCM:
		return cipherIDAESGCM, nil
	case CipherSM4GCM:
		return cipherIDSM4GCM, nil
	default:
		return 0, fmt.Errorf("unsupported secrets cipher: %s", cipherName)
	}
}

// cipherNameByID maps an on-disk cipher identifier byte back to a cipher name.
func cipherNameByID(id byte) (string, error) {
	switch id {
	case cipherIDAESGCM:
		return CipherAESGCM, nil
	case cipherIDSM4GCM:
		return CipherSM4GCM, nil
	default:
		return "", fmt.Errorf("unknown cipher id %d in secrets file header", id)
	}
}

// hasSecretsHeader reports whether data starts with the versioned secrets file
// header. Legacy (pre-header) files begin directly with the 16-byte salt,
// which is uniformly random and collides with the 6-byte magic with
// negligible probability.
func hasSecretsHeader(data []byte) bool {
	return len(data) >= secretsHdrSizeV1 && bytes.Equal(data[:len(secretsMagic)], []byte(secretsMagic))
}

// InspectFile reports the at-rest cipher and KDF of a secrets store without
// decrypting it: the header values for versioned files, AES-GCM + PBKDF2
// for legacy headerless files. A missing file yields ("", "", false, nil)
// so callers can distinguish "no store yet" from a real read error.
func InspectFile(path string) (cipherName, kdfName string, legacy bool, err error) {
	data, err := os.ReadFile(path) // #nosec G304 -- caller-supplied store path
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", false, nil
		}
		return "", "", false, fmt.Errorf("failed to read file: %w", err)
	}
	if !hasSecretsHeader(data) {
		return CipherAESGCM, KDFPBKDF2, true, nil
	}
	// Validate the version byte exactly like LoadFromFile so doctor never
	// misdiagnoses a future-format file by reading its bytes out of context.
	version := data[len(secretsMagic)]
	switch version {
	case secretsVersion1:
		name, err := cipherNameByID(data[len(secretsMagic)+1])
		if err != nil {
			return "", "", false, err
		}
		return name, KDFPBKDF2, false, nil
	case secretsVersion2:
		if len(data) < secretsHdrSizeV2 {
			return "", "", false, fmt.Errorf("secrets file header truncated")
		}
		name, err := cipherNameByID(data[len(secretsMagic)+1])
		if err != nil {
			return "", "", false, err
		}
		kdf, err := kdfNameByID(data[len(secretsMagic)+2])
		if err != nil {
			return "", "", false, err
		}
		return name, kdf, false, nil
	default:
		return "", "", false, fmt.Errorf("unsupported secrets file version %d", version)
	}
}

// deriveKey derives the cipher key from the master password with the
// selected KDF: Argon2id (memory-hard, default) or PBKDF2-SHA256 (legacy).
func deriveKey(masterPassword string, salt []byte, keyLen int, kdfName string) ([]byte, error) {
	switch kdfName {
	case KDFArgon2id:
		return argon2.IDKey([]byte(masterPassword), salt,
			uint32(argon2Time), uint32(argon2Memory), uint8(argon2Threads), uint32(keyLen)), nil
	case KDFPBKDF2:
		return pbkdf2.Key([]byte(masterPassword), salt, pbkdf2Iterations, keyLen, sha256.New), nil
	default:
		return nil, fmt.Errorf("unsupported secrets kdf: %s", kdfName)
	}
}

func generateSalt() ([]byte, error) {
	salt := make([]byte, pbkdf2SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// newAEAD builds the AEAD for a cipher name from an already-derived key.
func newAEAD(cipherName string, key []byte) (cipher.AEAD, error) {
	var block cipher.Block
	var err error
	switch cipherName {
	case CipherAESGCM:
		block, err = aes.NewCipher(key)
	case CipherSM4GCM:
		block, err = sm4.NewCipher(key)
	default:
		return nil, fmt.Errorf("unsupported secrets cipher: %s", cipherName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	return aead, nil
}

func newSecretManagerWithSalt(masterPassword string, salt []byte, cipherName, kdfName string) (*SecretManager, error) {
	keyLen, err := cipherKeySize(cipherName)
	if err != nil {
		return nil, err
	}

	key, err := deriveKey(masterPassword, salt, keyLen, kdfName)
	if err != nil {
		return nil, err
	}
	aead, err := newAEAD(cipherName, key)
	if err != nil {
		return nil, err
	}

	return &SecretManager{
		groups:         make(map[string]*SecretGroup),
		aead:           aead,
		salt:           salt,
		masterPassword: masterPassword,
		cipherName:     cipherName,
		kdfName:        kdfName,
	}, nil
}

// NewSecretManager creates a fresh secret manager. The at-rest cipher and
// KDF follow the AFLARE_SECRETS_CIPHER / AFLARE_SECRETS_KDF selections
// (aes-gcm + argon2id by default).
func NewSecretManager(masterPassword string) (*SecretManager, error) {
	cipherName, err := configuredCipher()
	if err != nil {
		return nil, err
	}
	kdfName, err := configuredKDF()
	if err != nil {
		return nil, err
	}
	salt, err := generateSalt()
	if err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return newSecretManagerWithSalt(masterPassword, salt, cipherName, kdfName)
}

// LoadFromFile loads the encrypted secrets store. The decryption cipher and
// KDF are determined by the file header (v2), the file header with PBKDF2
// implied (v1), or AES-256-GCM + PBKDF2 for legacy headerless files —
// regardless of the current cipher/KDF configuration.
func LoadFromFile(path, masterPassword string) (*SecretManager, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- internally generated secrets path
	if err != nil {
		if os.IsNotExist(err) {
			return NewSecretManager(masterPassword)
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	cipherName := CipherAESGCM
	kdfName := KDFPBKDF2
	payload := data
	if hasSecretsHeader(data) {
		version := data[len(secretsMagic)]
		switch version {
		case secretsVersion1:
			cipherName, err = cipherNameByID(data[len(secretsMagic)+1])
			if err != nil {
				return nil, err
			}
			payload = data[secretsHdrSizeV1:]
		case secretsVersion2:
			if len(data) < secretsHdrSizeV2 {
				return nil, fmt.Errorf("secrets file header truncated")
			}
			cipherName, err = cipherNameByID(data[len(secretsMagic)+1])
			if err != nil {
				return nil, err
			}
			kdfName, err = kdfNameByID(data[len(secretsMagic)+2])
			if err != nil {
				return nil, err
			}
			payload = data[secretsHdrSizeV2:]
		default:
			return nil, fmt.Errorf("unsupported secrets file version %d", version)
		}
	}

	if len(payload) < pbkdf2SaltSize {
		return nil, fmt.Errorf("file too short: invalid format")
	}

	salt := payload[:pbkdf2SaltSize]
	ciphertext := payload[pbkdf2SaltSize:]

	sm, err := newSecretManagerWithSalt(masterPassword, salt, cipherName, kdfName)
	if err != nil {
		return nil, err
	}

	plaintext, err := sm.decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	var stored struct {
		Groups map[string]*SecretGroup `json:"groups"`
	}
	if err := json.Unmarshal(plaintext, &stored); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	if stored.Groups != nil {
		sm.groups = stored.Groups
	}

	return sm, nil
}

// SaveToFile persists the store. The cipher and KDF used for writing follow
// the current AFLARE_SECRETS_CIPHER / AFLARE_SECRETS_KDF selections, so
// switching either environment variable re-encrypts the store on the next
// save (loading, in contrast, always follows the file header).
//
// On-disk format selection:
//   - argon2id (default KDF): v2 header carrying cipher + KDF. A store
//     loaded with pbkdf2 is transparently upgraded on its next save.
//   - pbkdf2 + sm4-gcm: v1 header, byte-identical to pre-Argon2id stores.
//   - pbkdf2 + aes-gcm: headerless legacy bytes, byte-compatible with
//     binaries before 0.9.0 — the maximum-compatibility rollback path
//     (set both env vars to the legacy values and re-save).
func (sm *SecretManager) SaveToFile(path string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	cipherName, err := configuredCipher()
	if err != nil {
		return err
	}
	kdfName, err := configuredKDF()
	if err != nil {
		return err
	}

	aead := sm.aead
	if cipherName != sm.cipherName || kdfName != sm.kdfName {
		// Re-key for the newly selected cipher/KDF, reusing the existing
		// salt (the AEAD nonce is fresh per save, so reuse is safe).
		keyLen, kerr := cipherKeySize(cipherName)
		if kerr != nil {
			return kerr
		}
		key, kerr := deriveKey(sm.masterPassword, sm.salt, keyLen, kdfName)
		if kerr != nil {
			return kerr
		}
		aead, kerr = newAEAD(cipherName, key)
		if kerr != nil {
			return kerr
		}
	}

	stored := struct {
		Groups map[string]*SecretGroup `json:"groups"`
	}{
		Groups: sm.groups,
	}

	plaintext, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)

	// Header selection (see the function comment for the compatibility
	// matrix). The default (argon2id) always tags the file so the KDF is
	// discoverable on load.
	var header []byte
	switch {
	case kdfName == KDFArgon2id:
		cipherID, cerr := cipherIDByName(cipherName)
		if cerr != nil {
			return cerr
		}
		header = make([]byte, 0, secretsHdrSizeV2)
		header = append(header, secretsMagic...)
		header = append(header, secretsVersion2, cipherID, kdfIDArgon2id)
		if sm.kdfName != KDFArgon2id {
			kdfUpgradeWarnOnce.Do(func() {
				logger.Warn("secrets store KDF upgraded to argon2id; older aflare binaries cannot read this file",
					"rollback", "set "+secretsEnvKDF+"=pbkdf2 and re-save")
			})
		}
	case cipherName != CipherAESGCM:
		// pbkdf2 + non-default cipher: v1 header, byte-identical to the
		// pre-Argon2id format for that cipher.
		cipherID, cerr := cipherIDByName(cipherName)
		if cerr != nil {
			return cerr
		}
		header = make([]byte, 0, secretsHdrSizeV1)
		header = append(header, secretsMagic...)
		header = append(header, secretsVersion1, cipherID)
		sm4CompatWarnOnce.Do(func() {
			logger.Warn("secrets store is being written with a non-default cipher; binaries before 0.9.0 cannot read this file",
				"cipher", cipherName,
				"rollback", "set "+secretsEnvCipher+"=aes-gcm and re-save")
		})
	default:
		// pbkdf2 + aes-gcm: headerless legacy bytes (pre-0.9.0 compatible).
	}

	output := make([]byte, 0, len(header)+len(sm.salt)+len(ciphertext))
	output = append(output, header...)
	output = append(output, sm.salt...)
	output = append(output, ciphertext...)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	// Crash-safe atomic write (temp file + fsync + rename, then directory
	// fsync). The secrets store is the one file a mid-write crash must never
	// destroy: without the fsync, the rename can hit the disk before the
	// data does, leaving an empty store — and every API key in it — gone.
	// os.CreateTemp inside refuses to follow planted symlinks (O_EXCL), so
	// the old fixed-name tmp dance is not needed either.
	if err := fsutil.WriteFileAtomic(path, output, 0600); err != nil {
		return err
	}

	return nil
}

func (sm *SecretManager) decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := sm.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := sm.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func (sm *SecretManager) AddGroup(groupName string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if groupName == "" {
		return errors.New("group name cannot be empty")
	}

	if _, exists := sm.groups[groupName]; exists {
		return fmt.Errorf("group %q already exists", groupName)
	}

	sm.groups[groupName] = &SecretGroup{
		Name:    groupName,
		Secrets: make(map[string]Secret),
	}

	return nil
}

func (sm *SecretManager) RemoveGroup(groupName string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.groups[groupName]; !exists {
		return fmt.Errorf("group %q does not exist", groupName)
	}

	delete(sm.groups, groupName)
	return nil
}

func (sm *SecretManager) ListGroups() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	groups := make([]string, 0, len(sm.groups))
	for name := range sm.groups {
		groups = append(groups, name)
	}
	sort.Strings(groups)
	return groups
}

func (sm *SecretManager) AddSecret(group, key, value, secretType, description string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if key == "" {
		return errors.New("secret key cannot be empty")
	}

	if secretType != SecretTypeNormal && secretType != SecretTypeSecret {
		return fmt.Errorf("invalid secret type: %s", secretType)
	}

	g, exists := sm.groups[group]
	if !exists {
		return fmt.Errorf("group %q does not exist", group)
	}

	if _, exists := g.Secrets[key]; exists {
		return fmt.Errorf("secret %q already exists in group %q", key, group)
	}

	now := time.Now()
	g.Secrets[key] = Secret{
		Key:         key,
		Value:       value,
		Type:        secretType,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	return nil
}

func (sm *SecretManager) RemoveSecret(group, key string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	g, exists := sm.groups[group]
	if !exists {
		return fmt.Errorf("group %q does not exist", group)
	}

	if _, exists := g.Secrets[key]; !exists {
		return fmt.Errorf("secret %q does not exist in group %q", key, group)
	}

	delete(g.Secrets, key)
	return nil
}

func (sm *SecretManager) GetSecret(group, key string) (string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	g, exists := sm.groups[group]
	if !exists {
		return "", fmt.Errorf("group %q does not exist", group)
	}

	s, exists := g.Secrets[key]
	if !exists {
		return "", fmt.Errorf("secret %q does not exist in group %q", key, group)
	}

	return s.Value, nil
}

func maskValue(value string) string {
	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}

func (sm *SecretManager) GetSecretMasked(group, key string) (string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	g, exists := sm.groups[group]
	if !exists {
		return "", fmt.Errorf("group %q does not exist", group)
	}

	s, exists := g.Secrets[key]
	if !exists {
		return "", fmt.Errorf("secret %q does not exist in group %q", key, group)
	}

	if s.Type == SecretTypeSecret {
		return maskValue(s.Value), nil
	}

	return s.Value, nil
}

func (sm *SecretManager) UpdateSecret(group, key, newValue string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	g, exists := sm.groups[group]
	if !exists {
		return fmt.Errorf("group %q does not exist", group)
	}

	s, exists := g.Secrets[key]
	if !exists {
		return fmt.Errorf("secret %q does not exist in group %q", key, group)
	}

	s.Value = newValue
	s.UpdatedAt = time.Now()
	g.Secrets[key] = s

	return nil
}

type SecretInfo struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Type        string    `json:"type"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (sm *SecretManager) ListSecrets(group string) ([]SecretInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	g, exists := sm.groups[group]
	if !exists {
		return nil, fmt.Errorf("group %q does not exist", group)
	}

	secrets := make([]SecretInfo, 0, len(g.Secrets))
	for _, s := range g.Secrets {
		value := s.Value
		if s.Type == SecretTypeSecret {
			value = maskValue(s.Value)
		}
		secrets = append(secrets, SecretInfo{
			Key:         s.Key,
			Value:       value,
			Type:        s.Type,
			Description: s.Description,
			CreatedAt:   s.CreatedAt,
			UpdatedAt:   s.UpdatedAt,
		})
	}

	sort.Slice(secrets, func(i, j int) bool {
		return secrets[i].Key < secrets[j].Key
	})

	return secrets, nil
}

func (sm *SecretManager) GetAllVars() map[string]string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	vars := make(map[string]string)
	for groupName, g := range sm.groups {
		for key, s := range g.Secrets {
			fullKey := groupName + "_" + key
			vars[fullKey] = s.Value
		}
	}
	return vars
}

const keyringService = "aflare-secrets"

// GetMasterPassword retrieves the master password using the following priority:
//  1. System keyring (Linux secret-service, macOS Keychain, Windows Credential Manager)
//  2. AFLARE_SECRETS_PASSWORD environment variable
//  3. Interactive terminal prompt (input is not echoed)
//
// On success with keyring or env, the password is stored in keyring for future use.
func GetMasterPassword() (string, error) {
	// 1. Try system keyring
	if password, err := keyring.Get(keyringService, "master"); err == nil {
		return password, nil
	}

	// 2. Try environment variable
	if password := os.Getenv("AFLARE_SECRETS_PASSWORD"); password != "" {
		// Store in keyring for future use (best-effort). Headless
		// environments (Docker/CI) have no keyring — that is an
		// expected condition, so only surface it at debug level
		// instead of scaring the user on every secrets command.
		if err := keyring.Set(keyringService, "master", password); err != nil {
			logger.Debug("secrets: keyring cache unavailable (headless env?)", "error", err)
		}
		return password, nil
	}

	// 3. Interactive prompt (only if stdin is a terminal)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, "Enter master password: ")
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("failed to read password: %w", err)
		}
		if len(password) == 0 {
			return "", errors.New("master password cannot be empty")
		}
		// Store in keyring for future use (best-effort; see env branch
		// above for why failures stay at debug level).
		if err := keyring.Set(keyringService, "master", string(password)); err != nil {
			logger.Debug("secrets: keyring cache unavailable (headless env?)", "error", err)
		}
		return string(password), nil
	}

	return "", fmt.Errorf("secrets password not set - set AFLARE_SECRETS_PASSWORD environment variable or run in a terminal for interactive input")
}

// GetSecretManager returns the global secret manager instance.
// It reads the master password via GetMasterPassword().
// If the file doesn't exist, it creates a new empty secret manager.
func GetSecretManager() (*SecretManager, error) {
	password, err := GetMasterPassword()
	if err != nil {
		return nil, err
	}
	return LoadFromFile(defaultSecretsPath, password)
}

// DefaultPath returns the on-disk path of the encrypted secrets store
// (~/.config/aflare/secrets.dat). Exported so the CLI can call SaveToFile
// after mutating the manager returned by GetSecretManager.
func DefaultPath() string {
	return defaultSecretsPath
}
