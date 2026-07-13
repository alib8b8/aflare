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
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	SecretTypeNormal = "normal"
	SecretTypeSecret = "secret"
)

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
}

type storedData struct {
	Groups map[string]*SecretGroup `json:"groups"`
}

func deriveKey(masterPassword string) []byte {
	hash := sha256.Sum256([]byte(masterPassword))
	return hash[:]
}

func NewSecretManager(masterPassword string) (*SecretManager, error) {
	key := deriveKey(masterPassword)

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
	}, nil
}

func LoadFromFile(path, masterPassword string) (*SecretManager, error) {
	sm, err := NewSecretManager(masterPassword)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return sm, nil
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	plaintext, err := sm.decrypt(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	var stored storedData
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

	stored := storedData{
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

	if err := os.WriteFile(path, ciphertext, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
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
