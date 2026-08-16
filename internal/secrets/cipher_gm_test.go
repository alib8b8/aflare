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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeLegacyAESStore writes a secrets file in the legacy pre-header format
// (salt || nonce || ciphertext+tag, AES-256-GCM with a 32-byte PBKDF2 key)
// exactly as older binaries did, containing one secret env/K.
func writeLegacyAESStore(t *testing.T, path, masterPassword, value string) {
	t.Helper()

	salt := make([]byte, pbkdf2SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		t.Fatalf("failed to generate legacy salt: %v", err)
	}

	block, err := aes.NewCipher(deriveKey(masterPassword, salt, 32))
	if err != nil {
		t.Fatalf("failed to create legacy cipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("failed to create legacy GCM: %v", err)
	}

	plaintext := []byte(`{"groups":{"env":{"name":"env","secrets":{"K":{` +
		`"key":"K","value":"` + value + `","type":"secret",` +
		`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}}}}}`)
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		t.Fatalf("failed to generate legacy nonce: %v", err)
	}
	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)

	out := make([]byte, 0, len(salt)+len(ciphertext))
	out = append(out, salt...)
	out = append(out, ciphertext...)
	if err := os.WriteFile(path, out, 0600); err != nil {
		t.Fatalf("failed to write legacy store: %v", err)
	}
}

// TestSaveLoadWithSM4GCM round-trips a store written with SM4-GCM: the file
// carries the versioned header identifying sm4, reloads decrypt with SM4, and
// the secret value survives.
func TestSaveLoadWithSM4GCM(t *testing.T) {
	t.Setenv("AFLARE_SECRETS_CIPHER", "sm4-gcm")
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.dat")

	sm, err := NewSecretManager("master-pw")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}
	if sm.cipherName != CipherSM4GCM {
		t.Fatalf("expected cipher %q, got %q", CipherSM4GCM, sm.cipherName)
	}
	if err := sm.AddGroup("env"); err != nil {
		t.Fatalf("AddGroup failed: %v", err)
	}
	if err := sm.AddSecret("env", "API_KEY", "sk-123456", SecretTypeSecret, ""); err != nil {
		t.Fatalf("AddSecret failed: %v", err)
	}
	if err := sm.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read store: %v", err)
	}
	if !hasSecretsHeader(data) {
		t.Fatal("expected versioned header on saved store")
	}
	if data[len(secretsMagic)] != secretsVersion {
		t.Errorf("expected version %d, got %d", secretsVersion, data[len(secretsMagic)])
	}
	if id := data[len(secretsMagic)+1]; id != cipherIDSM4GCM {
		t.Errorf("expected cipher id %d (sm4), got %d", cipherIDSM4GCM, id)
	}

	// Reload without any env override: the header alone selects SM4.
	t.Setenv("AFLARE_SECRETS_CIPHER", "")
	loaded, err := LoadFromFile(path, "master-pw")
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}
	if loaded.cipherName != CipherSM4GCM {
		t.Errorf("expected loaded cipher %q, got %q", CipherSM4GCM, loaded.cipherName)
	}
	value, err := loaded.GetSecret("env", "API_KEY")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if value != "sk-123456" {
		t.Errorf("secret value = %q, want %q", value, "sk-123456")
	}

	// Wrong master password must fail SM4-GCM authentication.
	if _, err := LoadFromFile(path, "wrong-pw"); err == nil {
		t.Error("LoadFromFile should fail with wrong master password")
	}
}

// TestSaveLoadWithAESHeader verifies the default (aes-gcm) path also writes
// the versioned header and round-trips.
func TestSaveLoadWithAESHeader(t *testing.T) {
	t.Setenv("AFLARE_SECRETS_CIPHER", "")
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.dat")

	sm, err := NewSecretManager("master-pw")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}
	if sm.cipherName != CipherAESGCM {
		t.Fatalf("expected cipher %q, got %q", CipherAESGCM, sm.cipherName)
	}
	if err := sm.AddGroup("env"); err != nil {
		t.Fatalf("AddGroup failed: %v", err)
	}
	if err := sm.AddSecret("env", "KEY", "v", SecretTypeNormal, ""); err != nil {
		t.Fatalf("AddSecret failed: %v", err)
	}
	if err := sm.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read store: %v", err)
	}
	if !hasSecretsHeader(data) {
		t.Fatal("expected versioned header on saved store")
	}
	if id := data[len(secretsMagic)+1]; id != cipherIDAESGCM {
		t.Errorf("expected cipher id %d (aes), got %d", cipherIDAESGCM, id)
	}

	loaded, err := LoadFromFile(path, "master-pw")
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}
	value, err := loaded.GetSecret("env", "KEY")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if value != "v" {
		t.Errorf("secret value = %q, want %q", value, "v")
	}
}

// TestLoadLegacyAESFileWithoutHeader proves backward compatibility: files
// written by pre-header binaries (no magic) are decrypted as AES-256-GCM even
// when the environment selects SM4.
func TestLoadLegacyAESFileWithoutHeader(t *testing.T) {
	t.Setenv("AFLARE_SECRETS_CIPHER", "sm4-gcm")
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.dat")
	writeLegacyAESStore(t, path, "legacy-master", "legacy-value")

	sm, err := LoadFromFile(path, "legacy-master")
	if err != nil {
		t.Fatalf("legacy AES file should decrypt: %v", err)
	}
	if sm.cipherName != CipherAESGCM {
		t.Errorf("legacy file should load as %q, got %q", CipherAESGCM, sm.cipherName)
	}
	value, err := sm.GetSecret("env", "K")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if value != "legacy-value" {
		t.Errorf("secret value = %q, want %q", value, "legacy-value")
	}
}

// TestSaveReencryptsWithSelectedCipher verifies the cipher-switch migration
// path: loading a legacy AES store and saving with AFLARE_SECRETS_CIPHER=sm4-gcm
// rewrites the file in the versioned SM4 format, which then loads correctly
// with no environment override.
func TestSaveReencryptsWithSelectedCipher(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.dat")
	writeLegacyAESStore(t, path, "legacy-master", "legacy-value")

	t.Setenv("AFLARE_SECRETS_CIPHER", "")
	sm, err := LoadFromFile(path, "legacy-master")
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	t.Setenv("AFLARE_SECRETS_CIPHER", "sm4-gcm")
	if err := sm.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read store: %v", err)
	}
	if !hasSecretsHeader(data) {
		t.Fatal("expected versioned header after re-encryption")
	}
	if id := data[len(secretsMagic)+1]; id != cipherIDSM4GCM {
		t.Errorf("expected cipher id %d (sm4), got %d", cipherIDSM4GCM, id)
	}

	t.Setenv("AFLARE_SECRETS_CIPHER", "")
	loaded, err := LoadFromFile(path, "legacy-master")
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}
	value, err := loaded.GetSecret("env", "K")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if value != "legacy-value" {
		t.Errorf("secret value = %q, want %q", value, "legacy-value")
	}
}

// TestResolveSecretsCipher covers the environment value parsing, including
// case/whitespace tolerance and rejection of unknown ciphers.
func TestResolveSecretsCipher(t *testing.T) {
	cases := []struct {
		env  string
		want string
	}{
		{"", CipherAESGCM},
		{"aes-gcm", CipherAESGCM},
		{"AES-GCM", CipherAESGCM},
		{"sm4-gcm", CipherSM4GCM},
		{"SM4-GCM", CipherSM4GCM},
		{"  sm4-gcm  ", CipherSM4GCM},
	}
	for _, tc := range cases {
		got, err := resolveSecretsCipher(tc.env)
		if err != nil {
			t.Errorf("resolveSecretsCipher(%q) unexpected error: %v", tc.env, err)
			continue
		}
		if got != tc.want {
			t.Errorf("resolveSecretsCipher(%q) = %q, want %q", tc.env, got, tc.want)
		}
	}

	if _, err := resolveSecretsCipher("rot13"); err == nil {
		t.Error("resolveSecretsCipher should reject unknown ciphers")
	}
}

// TestNewSecretManagerRejectsInvalidCipherEnv ensures an invalid
// AFLARE_SECRETS_CIPHER value surfaces as an error rather than a silent default.
func TestNewSecretManagerRejectsInvalidCipherEnv(t *testing.T) {
	t.Setenv("AFLARE_SECRETS_CIPHER", "rot13")

	_, err := NewSecretManager("pw")
	if err == nil {
		t.Fatal("NewSecretManager should fail for invalid AFLARE_SECRETS_CIPHER")
	}
	if !strings.Contains(err.Error(), "AFLARE_SECRETS_CIPHER") {
		t.Errorf("error should mention AFLARE_SECRETS_CIPHER, got: %v", err)
	}
}

// TestSaveRejectsInvalidCipherEnv ensures saving also fails on an invalid
// environment value instead of silently writing with another cipher.
func TestSaveRejectsInvalidCipherEnv(t *testing.T) {
	sm, err := NewSecretManager("pw")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}

	t.Setenv("AFLARE_SECRETS_CIPHER", "rot13")
	err = sm.SaveToFile(filepath.Join(t.TempDir(), "secrets.dat"))
	if err == nil {
		t.Fatal("SaveToFile should fail for invalid AFLARE_SECRETS_CIPHER")
	}
	if !strings.Contains(err.Error(), "AFLARE_SECRETS_CIPHER") {
		t.Errorf("error should mention AFLARE_SECRETS_CIPHER, got: %v", err)
	}
}

// TestLoadRejectsBadHeader checks that unsupported versions and unknown cipher
// IDs in the header are rejected with a clear error.
func TestLoadRejectsBadHeader(t *testing.T) {
	dir := t.TempDir()

	future := append([]byte(secretsMagic), secretsVersion+1, cipherIDAESGCM)
	path := filepath.Join(dir, "future.dat")
	if err := os.WriteFile(path, future, 0600); err != nil {
		t.Fatalf("failed to write store: %v", err)
	}
	if _, err := LoadFromFile(path, "pw"); err == nil || !strings.Contains(err.Error(), "version") {
		t.Errorf("expected version error, got: %v", err)
	}

	unknownCipher := append([]byte(secretsMagic), secretsVersion, byte(0x7f))
	path = filepath.Join(dir, "unknown.dat")
	if err := os.WriteFile(path, unknownCipher, 0600); err != nil {
		t.Fatalf("failed to write store: %v", err)
	}
	if _, err := LoadFromFile(path, "pw"); err == nil || !strings.Contains(err.Error(), "cipher id") {
		t.Errorf("expected cipher id error, got: %v", err)
	}
}

// TestCipherKeySize pins the PBKDF2 output length per cipher: 32 bytes for
// AES-256, 16 bytes for SM4.
func TestCipherKeySize(t *testing.T) {
	if n, err := cipherKeySize(CipherAESGCM); err != nil || n != 32 {
		t.Errorf("cipherKeySize(aes-gcm) = %d, %v; want 32, nil", n, err)
	}
	if n, err := cipherKeySize(CipherSM4GCM); err != nil || n != 16 {
		t.Errorf("cipherKeySize(sm4-gcm) = %d, %v; want 16, nil", n, err)
	}
	if _, err := cipherKeySize("rot13"); err == nil {
		t.Error("cipherKeySize should reject unknown ciphers")
	}
}
