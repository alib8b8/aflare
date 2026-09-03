// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​‌​​​‌‌‌​‌​‌​‌​​​​‌​​‌​‌​​​​‌​​​​​‌‌​‌‌‌​​​​​​​​​​​​​​​​​​​​​​‌​‌​​‌​​​‌‌‌‌‌​​⁠
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

	legacyKey, err := deriveKey(masterPassword, salt, 32, KDFPBKDF2)
	if err != nil {
		t.Fatalf("failed to derive legacy key: %v", err)
	}
	block, err := aes.NewCipher(legacyKey)
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
	if data[len(secretsMagic)] != secretsVersion2 {
		t.Errorf("expected version %d (argon2id era), got %d", secretsVersion2, data[len(secretsMagic)])
	}
	if id := data[len(secretsMagic)+1]; id != cipherIDSM4GCM {
		t.Errorf("expected cipher id %d (sm4), got %d", cipherIDSM4GCM, id)
	}
	if id := data[len(secretsMagic)+2]; id != kdfIDArgon2id {
		t.Errorf("expected kdf id %d (argon2id, the default), got %d", kdfIDArgon2id, id)
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

// TestSaveDefaultWritesArgon2idHeader verifies the new default write format:
// aes-gcm + argon2id stores carry the v2 header (cipher + KDF IDs), because
// the KDF must be discoverable on load, and round-trip.
func TestSaveDefaultWritesArgon2idHeader(t *testing.T) {
	t.Setenv("AFLARE_SECRETS_CIPHER", "")
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.dat")

	sm, err := NewSecretManager("master-pw")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}
	if sm.cipherName != CipherAESGCM || sm.kdfName != KDFArgon2id {
		t.Fatalf("expected aes-gcm + argon2id defaults, got %q + %q", sm.cipherName, sm.kdfName)
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
		t.Fatal("default (argon2id) save must carry the versioned header")
	}
	if data[len(secretsMagic)] != secretsVersion2 {
		t.Errorf("expected version %d, got %d", secretsVersion2, data[len(secretsMagic)])
	}
	if id := data[len(secretsMagic)+1]; id != cipherIDAESGCM {
		t.Errorf("expected cipher id %d (aes), got %d", cipherIDAESGCM, id)
	}
	if id := data[len(secretsMagic)+2]; id != kdfIDArgon2id {
		t.Errorf("expected kdf id %d (argon2id), got %d", kdfIDArgon2id, id)
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

// TestSavePinnedPBKDF2WritesLegacyFormat verifies the maximum-compatibility
// rollback path: pinning AFLARE_SECRETS_KDF=pbkdf2 restores the headerless
// pre-0.9.0 byte format for aes-gcm stores, and the v1 format for sm4-gcm.
func TestSavePinnedPBKDF2WritesLegacyFormat(t *testing.T) {
	t.Setenv("AFLARE_SECRETS_KDF", "pbkdf2")
	dir := t.TempDir()
	path := filepath.Join(dir, "aes.dat")

	sm, err := NewSecretManager("master-pw")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
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
	if hasSecretsHeader(data) {
		t.Fatal("pbkdf2+aes-gcm save must be headerless (pre-0.9.0 byte compatibility)")
	}
	if len(data) <= pbkdf2SaltSize {
		t.Fatalf("store too short: %d bytes", len(data))
	}

	// pbkdf2 + sm4-gcm: v1 header, byte-identical to pre-Argon2id sm4 stores.
	sm4Path := filepath.Join(dir, "sm4.dat")
	t.Setenv("AFLARE_SECRETS_CIPHER", "sm4-gcm")
	if err := sm.SaveToFile(sm4Path); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
	data, err = os.ReadFile(sm4Path)
	if err != nil {
		t.Fatalf("failed to read store: %v", err)
	}
	if !hasSecretsHeader(data) {
		t.Fatal("pbkdf2+sm4-gcm save must carry the v1 header")
	}
	if data[len(secretsMagic)] != secretsVersion1 {
		t.Errorf("expected version %d (v1), got %d", secretsVersion1, data[len(secretsMagic)])
	}

	// Both stores still round-trip.
	for _, p := range []string{path, sm4Path} {
		loaded, err := LoadFromFile(p, "master-pw")
		if err != nil {
			t.Fatalf("LoadFromFile(%s) failed: %v", p, err)
		}
		if value, err := loaded.GetSecret("env", "KEY"); err != nil || value != "v" {
			t.Errorf("GetSecret(%s) = %q, %v; want %q, nil", p, value, err, "v")
		}
	}
}

// TestSaveSM4ThenRollbackToLegacy proves the mixed-fleet rollback path: an
// SM4-GCM store is rewritten in the legacy headerless AES format once BOTH
// the cipher and the KDF environment variables switch back to the legacy
// values and the store is saved again.
func TestSaveSM4ThenRollbackToLegacy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.dat")

	// Phase 1: opt into guomi and save — file carries the SM4 v2 header.
	t.Setenv("AFLARE_SECRETS_CIPHER", "sm4-gcm")
	sm, err := NewSecretManager("master-pw")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}
	if err := sm.AddGroup("env"); err != nil {
		t.Fatalf("AddGroup failed: %v", err)
	}
	if err := sm.AddSecret("env", "KEY", "keep-me", SecretTypeNormal, ""); err != nil {
		t.Fatalf("AddSecret failed: %v", err)
	}
	if err := sm.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !hasSecretsHeader(data) || data[len(secretsMagic)+1] != cipherIDSM4GCM {
		t.Fatal("expected SM4 header after guomi save")
	}

	// Phase 2: switch back to aes-gcm + pbkdf2, load, save — legacy
	// headerless format restored.
	t.Setenv("AFLARE_SECRETS_CIPHER", "")
	t.Setenv("AFLARE_SECRETS_KDF", "pbkdf2")
	loaded, err := LoadFromFile(path, "master-pw")
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}
	if err := loaded.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
	data, _ = os.ReadFile(path)
	if hasSecretsHeader(data) {
		t.Fatal("expected legacy headerless format after rollback to aes-gcm + pbkdf2")
	}

	// Data survives the round-trip.
	final, err := LoadFromFile(path, "master-pw")
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}
	value, err := final.GetSecret("env", "KEY")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if value != "keep-me" {
		t.Errorf("secret value = %q, want %q", value, "keep-me")
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

// TestLoadRejectsBadHeader checks that unsupported versions, unknown cipher
// IDs, and unknown KDF IDs in the header are rejected with a clear error.
func TestLoadRejectsBadHeader(t *testing.T) {
	dir := t.TempDir()

	future := append([]byte(secretsMagic), secretsVersion2+1, cipherIDAESGCM)
	path := filepath.Join(dir, "future.dat")
	if err := os.WriteFile(path, future, 0600); err != nil {
		t.Fatalf("failed to write store: %v", err)
	}
	if _, err := LoadFromFile(path, "pw"); err == nil || !strings.Contains(err.Error(), "version") {
		t.Errorf("expected version error, got: %v", err)
	}

	unknownCipher := append([]byte(secretsMagic), secretsVersion1, byte(0x7f))
	path = filepath.Join(dir, "unknown.dat")
	if err := os.WriteFile(path, unknownCipher, 0600); err != nil {
		t.Fatalf("failed to write store: %v", err)
	}
	if _, err := LoadFromFile(path, "pw"); err == nil || !strings.Contains(err.Error(), "cipher id") {
		t.Errorf("expected cipher id error, got: %v", err)
	}

	unknownKDF := append([]byte(secretsMagic), secretsVersion2, cipherIDAESGCM, byte(0x7f))
	path = filepath.Join(dir, "unknown-kdf.dat")
	if err := os.WriteFile(path, unknownKDF, 0600); err != nil {
		t.Fatalf("failed to write store: %v", err)
	}
	if _, err := LoadFromFile(path, "pw"); err == nil || !strings.Contains(err.Error(), "kdf id") {
		t.Errorf("expected kdf id error, got: %v", err)
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

// TestInspectFile covers the doctor-facing probe: legacy files report
// aes-gcm + pbkdf2 + legacy, versioned files report their header cipher and
// KDF, and a missing file reports nothing without an error.
func TestInspectFile(t *testing.T) {
	dir := t.TempDir()

	// Missing file: ("", "", false, nil).
	name, kdf, legacy, err := InspectFile(filepath.Join(dir, "absent.dat"))
	if err != nil || name != "" || kdf != "" || legacy {
		t.Errorf("missing file: got (%q, %q, %v, %v), want (\"\", \"\", false, nil)", name, kdf, legacy, err)
	}

	// Legacy headerless store: aes-gcm + pbkdf2 + legacy=true.
	legacyPath := filepath.Join(dir, "legacy.dat")
	writeLegacyAESStore(t, legacyPath, "pw", "v")
	name, kdf, legacy, err = InspectFile(legacyPath)
	if err != nil || name != CipherAESGCM || kdf != KDFPBKDF2 || !legacy {
		t.Errorf("legacy file: got (%q, %q, %v, %v), want (%q, %q, true, nil)", name, kdf, legacy, err, CipherAESGCM, KDFPBKDF2)
	}

	// SM4 versioned store (default argon2id): sm4-gcm + argon2id + legacy=false.
	t.Setenv("AFLARE_SECRETS_CIPHER", "sm4-gcm")
	sm, err := NewSecretManager("pw")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}
	sm4Path := filepath.Join(dir, "sm4.dat")
	if err := sm.SaveToFile(sm4Path); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
	name, kdf, legacy, err = InspectFile(sm4Path)
	if err != nil || name != CipherSM4GCM || kdf != KDFArgon2id || legacy {
		t.Errorf("sm4 file: got (%q, %q, %v, %v), want (%q, %q, false, nil)", name, kdf, legacy, err, CipherSM4GCM, KDFArgon2id)
	}

	// pbkdf2-pinned store: headerless legacy bytes again, so InspectFile
	// reports the pinned KDF with legacy=true (legacy = headerless
	// format, regardless of which binary wrote it).
	t.Setenv("AFLARE_SECRETS_CIPHER", "")
	t.Setenv("AFLARE_SECRETS_KDF", "pbkdf2")
	pinnedPath := filepath.Join(dir, "pinned.dat")
	if err := sm.SaveToFile(pinnedPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
	name, kdf, legacy, err = InspectFile(pinnedPath)
	if err != nil || name != CipherAESGCM || kdf != KDFPBKDF2 || !legacy {
		t.Errorf("pinned pbkdf2 file: got (%q, %q, %v, %v), want (%q, %q, true, nil)", name, kdf, legacy, err, CipherAESGCM, KDFPBKDF2)
	}
}

// TestSecretsKDFMigration proves the smooth-migration path: a legacy
// PBKDF2 store (any pre-Argon2id binary could have written it) loads via
// PBKDF2, then its next save transparently upgrades it to Argon2id with the
// v2 header — secrets intact, no user action required.
func TestSecretsKDFMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.dat")
	writeLegacyAESStore(t, path, "legacy-master", "legacy-value")

	// Load: headerless file → PBKDF2 + AES-GCM regardless of env defaults.
	sm, err := LoadFromFile(path, "legacy-master")
	if err != nil {
		t.Fatalf("legacy store should load: %v", err)
	}
	if sm.kdfName != KDFPBKDF2 || sm.cipherName != CipherAESGCM {
		t.Fatalf("expected pbkdf2+aes-gcm on load, got %s+%s", sm.kdfName, sm.cipherName)
	}

	// Save with defaults: upgraded to argon2id, v2 header on disk.
	if err := sm.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !hasSecretsHeader(data) || data[len(secretsMagic)] != secretsVersion2 || data[len(secretsMagic)+2] != kdfIDArgon2id {
		t.Fatal("expected v2 argon2id header after upgrade save")
	}

	// Reload: header selects argon2id, value intact.
	loaded, err := LoadFromFile(path, "legacy-master")
	if err != nil {
		t.Fatalf("upgraded store should load: %v", err)
	}
	if loaded.kdfName != KDFArgon2id {
		t.Errorf("expected loaded kdf argon2id, got %s", loaded.kdfName)
	}
	value, err := loaded.GetSecret("env", "K")
	if err != nil || value != "legacy-value" {
		t.Errorf("GetSecret = %q, %v; want %q, nil", value, err, "legacy-value")
	}

	// Doctor's probe sees the upgraded KDF.
	if _, kdf, _, err := InspectFile(path); err != nil || kdf != KDFArgon2id {
		t.Errorf("InspectFile kdf = %q, %v; want %q, nil", kdf, err, KDFArgon2id)
	}
}

// TestResolveSecretsKDF covers the KDF environment value parsing, mirroring
// the cipher test: case/whitespace tolerance, pbkdf2 pinning, argon2id
// default, rejection of unknown values.
func TestResolveSecretsKDF(t *testing.T) {
	cases := []struct {
		env  string
		want string
	}{
		{"", KDFArgon2id},
		{"argon2id", KDFArgon2id},
		{"ARGON2ID", KDFArgon2id},
		{"pbkdf2", KDFPBKDF2},
		{"  PBKDF2  ", KDFPBKDF2},
	}
	for _, tc := range cases {
		got, err := resolveSecretsKDF(tc.env)
		if err != nil {
			t.Errorf("resolveSecretsKDF(%q) unexpected error: %v", tc.env, err)
			continue
		}
		if got != tc.want {
			t.Errorf("resolveSecretsKDF(%q) = %q, want %q", tc.env, got, tc.want)
		}
	}

	if _, err := resolveSecretsKDF("scrypt"); err == nil {
		t.Error("resolveSecretsKDF should reject unknown KDFs")
	}
}

// TestNewSecretManagerRejectsInvalidKDFEnv ensures an invalid
// AFLARE_SECRETS_KDF value surfaces as an error rather than a silent
// downgrade to a weaker KDF.
func TestNewSecretManagerRejectsInvalidKDFEnv(t *testing.T) {
	t.Setenv("AFLARE_SECRETS_KDF", "scrypt")

	_, err := NewSecretManager("pw")
	if err == nil {
		t.Fatal("NewSecretManager should fail for invalid AFLARE_SECRETS_KDF")
	}
	if !strings.Contains(err.Error(), "AFLARE_SECRETS_KDF") {
		t.Errorf("error should mention AFLARE_SECRETS_KDF, got: %v", err)
	}
}

// TestSaveRejectsInvalidKDFEnv ensures saving also fails on an invalid KDF
// environment value instead of silently writing with another KDF.
func TestSaveRejectsInvalidKDFEnv(t *testing.T) {
	sm, err := NewSecretManager("pw")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}

	t.Setenv("AFLARE_SECRETS_KDF", "scrypt")
	err = sm.SaveToFile(filepath.Join(t.TempDir(), "secrets.dat"))
	if err == nil {
		t.Fatal("SaveToFile should fail for invalid AFLARE_SECRETS_KDF")
	}
	if !strings.Contains(err.Error(), "AFLARE_SECRETS_KDF") {
		t.Errorf("error should mention AFLARE_SECRETS_KDF, got: %v", err)
	}
}
