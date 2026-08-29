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
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"

	"github.com/alib8b8/aflare/internal/logger"
)

// Cipher suite identifiers for the at-rest encryption of the secrets store.
// The suite is selected via AFLARE_SECRETS_CIPHER (aes-gcm by default).
const (
	CipherAESGCM = "aes-gcm"
	CipherSM4GCM = "sm4-gcm"
)

// On-disk format. Versioned files start with an 8-byte header
// (magic "AFLSEC" + version byte + cipher ID byte) followed by the 16-byte
// PBKDF2 salt and the AEAD ciphertext (nonce || ciphertext+tag). Legacy files
// written before the header existed start directly with the salt and are
// always decrypted as AES-256-GCM.
const (
	secretsEnvCipher = "AFLARE_SECRETS_CIPHER"
	secretsMagic     = "AFLSEC"
	secretsVersion   = 0x01
	secretsHdrSize   = 8
	cipherIDAESGCM   = 0x01
	cipherIDSM4GCM   = 0x02
)

const (
	SecretTypeNormal = "normal"
	SecretTypeSecret = "secret" // #nosec G101 -- type label constant, not a credential value
	pbkdf2Iterations = 600000
	pbkdf2SaltSize   = 16
)

var defaultSecretsPath string

// sm4CompatWarnOnce rate-limits the pre-0.9.0 incompatibility warning to
// one line per process, no matter how often the store is saved.
var sm4CompatWarnOnce sync.Once

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
// needed to persist them. The master password and cipher name are retained so
// SaveToFile can re-derive a key when the configured cipher differs from the
// one the store was loaded with.
type SecretManager struct {
	mu             sync.RWMutex
	groups         map[string]*SecretGroup
	aead           cipher.AEAD
	salt           []byte
	masterPassword string
	cipherName     string
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
	return len(data) >= secretsHdrSize && bytes.Equal(data[:len(secretsMagic)], []byte(secretsMagic))
}

// InspectFile reports the at-rest cipher of a secrets store without
// decrypting it: the header cipher for versioned files, CipherAESGCM for
// legacy headerless files. A missing file yields ("", false, nil) so
// callers can distinguish "no store yet" from a real read error.
func InspectFile(path string) (cipherName string, legacy bool, err error) {
	data, err := os.ReadFile(path) // #nosec G304 -- caller-supplied store path
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to read file: %w", err)
	}
	if !hasSecretsHeader(data) {
		return CipherAESGCM, true, nil
	}
	// Validate the version byte exactly like LoadFromFile so doctor never
	// misdiagnoses a future-format file by reading its bytes out of context.
	if version := data[len(secretsMagic)]; version != secretsVersion {
		return "", false, fmt.Errorf("unsupported secrets file version %d", version)
	}
	name, err := cipherNameByID(data[len(secretsMagic)+1])
	if err != nil {
		return "", false, err
	}
	return name, false, nil
}

func deriveKey(masterPassword string, salt []byte, keyLen int) []byte {
	return pbkdf2.Key([]byte(masterPassword), salt, pbkdf2Iterations, keyLen, sha256.New)
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

func newSecretManagerWithSalt(masterPassword string, salt []byte, cipherName string) (*SecretManager, error) {
	keyLen, err := cipherKeySize(cipherName)
	if err != nil {
		return nil, err
	}

	aead, err := newAEAD(cipherName, deriveKey(masterPassword, salt, keyLen))
	if err != nil {
		return nil, err
	}

	return &SecretManager{
		groups:         make(map[string]*SecretGroup),
		aead:           aead,
		salt:           salt,
		masterPassword: masterPassword,
		cipherName:     cipherName,
	}, nil
}

// NewSecretManager creates a fresh secret manager. The at-rest cipher follows
// the AFLARE_SECRETS_CIPHER selection (aes-gcm by default).
func NewSecretManager(masterPassword string) (*SecretManager, error) {
	cipherName, err := configuredCipher()
	if err != nil {
		return nil, err
	}
	salt, err := generateSalt()
	if err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return newSecretManagerWithSalt(masterPassword, salt, cipherName)
}

// LoadFromFile loads the encrypted secrets store. The decryption cipher is
// determined by the file header; files without a header (legacy format) are
// decrypted as AES-256-GCM regardless of the current cipher configuration.
func LoadFromFile(path, masterPassword string) (*SecretManager, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- internally generated secrets path
	if err != nil {
		if os.IsNotExist(err) {
			return NewSecretManager(masterPassword)
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	cipherName := CipherAESGCM
	payload := data
	if hasSecretsHeader(data) {
		if version := data[len(secretsMagic)]; version != secretsVersion {
			return nil, fmt.Errorf("unsupported secrets file version %d", version)
		}
		cipherName, err = cipherNameByID(data[len(secretsMagic)+1])
		if err != nil {
			return nil, err
		}
		payload = data[secretsHdrSize:]
	}

	if len(payload) < pbkdf2SaltSize {
		return nil, fmt.Errorf("file too short: invalid format")
	}

	salt := payload[:pbkdf2SaltSize]
	ciphertext := payload[pbkdf2SaltSize:]

	sm, err := newSecretManagerWithSalt(masterPassword, salt, cipherName)
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

// SaveToFile persists the store. The cipher used for writing follows the
// current AFLARE_SECRETS_CIPHER selection, so switching the environment
// variable re-encrypts the store on the next save (loading, in contrast,
// always follows the file header).
//
// Backward compatibility: with the default aes-gcm selection the file is
// written in the legacy headerless format, byte-compatible with binaries
// before 0.9.0. The versioned header is written only for non-default
// ciphers (sm4-gcm), which old binaries cannot read anyway — so mixed
// fleets stay compatible until guomi is explicitly opted into. Switching
// the environment variable back to aes-gcm and saving rewrites the file
// in the legacy format (a rollback path for shared-home fleets).
func (sm *SecretManager) SaveToFile(path string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	cipherName, err := configuredCipher()
	if err != nil {
		return err
	}

	aead := sm.aead
	if cipherName != sm.cipherName {
		// Re-key for the newly selected cipher, reusing the existing salt.
		keyLen, kerr := cipherKeySize(cipherName)
		if kerr != nil {
			return kerr
		}
		aead, kerr = newAEAD(cipherName, deriveKey(sm.masterPassword, sm.salt, keyLen))
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

	output := make([]byte, 0, secretsHdrSize+len(sm.salt)+len(ciphertext))
	if cipherName != CipherAESGCM {
		// Non-default cipher: tag the file so LoadFromFile (and humans)
		// can tell which suite encrypted it. The default AES path stays
		// headerless for pre-0.9.0 byte compatibility.
		cipherID, cerr := cipherIDByName(cipherName)
		if cerr != nil {
			return cerr
		}
		output = append(output, secretsMagic...)
		output = append(output, secretsVersion, cipherID)
		sm4CompatWarnOnce.Do(func() {
			logger.Warn("secrets store is being written with a non-default cipher; binaries before 0.9.0 cannot read this file",
				"cipher", cipherName,
				"rollback", "set "+secretsEnvCipher+"=aes-gcm and re-save")
		})
	}
	output = append(output, sm.salt...)
	output = append(output, ciphertext...)

	// Atomic write: write to .tmp first, then rename to final path.
	// This prevents file corruption on partial writes (e.g. crash mid-write).
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	tmpPath := path + ".tmp"
	// Clear any pre-existing tmp path before writing: in a writable shared
	// directory an attacker could plant secrets.dat.tmp -> /etc/... and have
	// the atomic write clobber the target. os.Remove never follows symlinks,
	// so removing it keeps the write inside the directory we control.
	if fi, err := os.Lstat(tmpPath); err == nil {
		if fi.IsDir() {
			return fmt.Errorf("refusing to write temporary file %s: a directory already exists there", tmpPath)
		}
		if err := os.Remove(tmpPath); err != nil {
			return fmt.Errorf("failed to clear stale temporary file: %w", err)
		}
	}
	if err := os.WriteFile(tmpPath, output, 0600); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
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
