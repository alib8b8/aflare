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

package secrets

import (
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

	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"
)

const (
	SecretTypeNormal = "normal"
	SecretTypeSecret = "secret" // #nosec G101 -- type label constant, not a credential value
	pbkdf2Iterations = 600000
	pbkdf2SaltSize   = 16
	pbkdf2KeySize    = 32
)

var defaultSecretsPath string

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

type SecretManager struct {
	mu     sync.RWMutex
	groups map[string]*SecretGroup
	aead   cipher.AEAD
	salt   []byte
}

func deriveKey(masterPassword string, salt []byte) []byte {
	return pbkdf2.Key([]byte(masterPassword), salt, pbkdf2Iterations, pbkdf2KeySize, sha256.New)
}

func generateSalt() ([]byte, error) {
	salt := make([]byte, pbkdf2SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

func newSecretManagerWithSalt(masterPassword string, salt []byte) (*SecretManager, error) {
	key := deriveKey(masterPassword, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &SecretManager{
		groups: make(map[string]*SecretGroup),
		aead:   aead,
		salt:   salt,
	}, nil
}

func NewSecretManager(masterPassword string) (*SecretManager, error) {
	salt, err := generateSalt()
	if err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return newSecretManagerWithSalt(masterPassword, salt)
}

func LoadFromFile(path, masterPassword string) (*SecretManager, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- internally generated secrets path
	if err != nil {
		if os.IsNotExist(err) {
			return NewSecretManager(masterPassword)
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	if len(data) < pbkdf2SaltSize {
		return nil, fmt.Errorf("file too short: invalid format")
	}

	salt := data[:pbkdf2SaltSize]
	ciphertext := data[pbkdf2SaltSize:]

	sm, err := newSecretManagerWithSalt(masterPassword, salt)
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

func (sm *SecretManager) SaveToFile(path string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stored := struct {
		Groups map[string]*SecretGroup `json:"groups"`
	}{
		Groups: sm.groups,
	}

	plaintext, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	ciphertext, err := sm.encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("failed to encrypt data: %w", err)
	}

	output := make([]byte, 0, len(sm.salt)+len(ciphertext))
	output = append(output, sm.salt...)
	output = append(output, ciphertext...)

	// Atomic write: write to .tmp first, then rename to final path.
	// This prevents file corruption on partial writes (e.g. crash mid-write).
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, output, 0600); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return nil
}

func (sm *SecretManager) encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, sm.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := sm.aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
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
		// Store in keyring for future use (best-effort)
		_ = keyring.Set(keyringService, "master", password)
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
		// Store in keyring for future use (best-effort)
		_ = keyring.Set(keyringService, "master", string(password))
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
