// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​​‌​‌‌‌​​‌‌‌‌‌‌‌‌​​‌​‌‌‌‌​‌​‌‌​‌‌‌‌‌​‌‌​‌​‌‌​‌​​​​​​​​​​​​​​​​​​‌​​‌​‌‌‌​​‌​​​⁠
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
	"os"
	"path/filepath"
	"testing"
)

func TestNewSecretManager(t *testing.T) {
	sm, err := NewSecretManager("test-password")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}
	if sm == nil {
		t.Fatal("NewSecretManager returned nil")
	}
}

func TestAddGroup(t *testing.T) {
	sm, err := NewSecretManager("test-password")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}

	err = sm.AddGroup("env")
	if err != nil {
		t.Fatalf("AddGroup failed: %v", err)
	}

	groups := sm.ListGroups()
	if len(groups) != 1 || groups[0] != "env" {
		t.Fatalf("ListGroups returned %v, expected [env]", groups)
	}
}

func TestAddDuplicateGroup(t *testing.T) {
	sm, err := NewSecretManager("test-password")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}

	err = sm.AddGroup("env")
	if err != nil {
		t.Fatalf("AddGroup failed: %v", err)
	}

	err = sm.AddGroup("env")
	if err == nil {
		t.Fatal("AddGroup should fail for duplicate group")
	}
}

func TestRemoveGroup(t *testing.T) {
	sm, err := NewSecretManager("test-password")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}

	sm.AddGroup("env")
	err = sm.RemoveGroup("env")
	if err != nil {
		t.Fatalf("RemoveGroup failed: %v", err)
	}

	groups := sm.ListGroups()
	if len(groups) != 0 {
		t.Fatalf("ListGroups returned %v, expected empty", groups)
	}
}

func TestAddSecret(t *testing.T) {
	sm, err := NewSecretManager("test-password")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}

	sm.AddGroup("env")
	err = sm.AddSecret("env", "API_KEY", "sk-1234567890abcdef", SecretTypeSecret, "API key for service")
	if err != nil {
		t.Fatalf("AddSecret failed: %v", err)
	}

	value, err := sm.GetSecret("env", "API_KEY")
	if err != nil {
		t.Fatalf("GetSecret failed: %v", err)
	}
	if value != "sk-1234567890abcdef" {
		t.Fatalf("GetSecret returned %q, expected %q", value, "sk-1234567890abcdef")
	}
}

func TestGetSecretMasked(t *testing.T) {
	sm, err := NewSecretManager("test-password")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}

	sm.AddGroup("env")
	sm.AddSecret("env", "API_KEY", "sk-1234567890abcdef", SecretTypeSecret, "")
	sm.AddSecret("env", "ENV", "production", SecretTypeNormal, "")

	masked, err := sm.GetSecretMasked("env", "API_KEY")
	if err != nil {
		t.Fatalf("GetSecretMasked failed: %v", err)
	}
	if masked != "sk***************ef" {
		t.Fatalf("GetSecretMasked returned %q, expected %q", masked, "sk***************ef")
	}

	normal, err := sm.GetSecretMasked("env", "ENV")
	if err != nil {
		t.Fatalf("GetSecretMasked failed: %v", err)
	}
	if normal != "production" {
		t.Fatalf("GetSecretMasked returned %q for normal secret, expected %q", normal, "production")
	}
}

func TestUpdateSecret(t *testing.T) {
	sm, err := NewSecretManager("test-password")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}

	sm.AddGroup("env")
	sm.AddSecret("env", "API_KEY", "old-value", SecretTypeSecret, "")

	err = sm.UpdateSecret("env", "API_KEY", "new-value")
	if err != nil {
		t.Fatalf("UpdateSecret failed: %v", err)
	}

	value, _ := sm.GetSecret("env", "API_KEY")
	if value != "new-value" {
		t.Fatalf("GetSecret returned %q, expected %q", value, "new-value")
	}
}

func TestRemoveSecret(t *testing.T) {
	sm, err := NewSecretManager("test-password")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}

	sm.AddGroup("env")
	sm.AddSecret("env", "API_KEY", "value", SecretTypeSecret, "")

	err = sm.RemoveSecret("env", "API_KEY")
	if err != nil {
		t.Fatalf("RemoveSecret failed: %v", err)
	}

	_, err = sm.GetSecret("env", "API_KEY")
	if err == nil {
		t.Fatal("GetSecret should fail for removed secret")
	}
}

func TestListSecrets(t *testing.T) {
	sm, err := NewSecretManager("test-password")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}

	sm.AddGroup("env")
	sm.AddSecret("env", "B_KEY", "b-value", SecretTypeNormal, "")
	sm.AddSecret("env", "A_KEY", "a-secret-value", SecretTypeSecret, "")

	secrets, err := sm.ListSecrets("env")
	if err != nil {
		t.Fatalf("ListSecrets failed: %v", err)
	}

	if len(secrets) != 2 {
		t.Fatalf("ListSecrets returned %d secrets, expected 2", len(secrets))
	}

	if secrets[0].Key != "A_KEY" {
		t.Fatalf("First secret key is %q, expected %q", secrets[0].Key, "A_KEY")
	}

	if secrets[0].Type == SecretTypeSecret {
		if secrets[0].Value == "a-secret-value" {
			t.Fatal("Secret value should be masked in ListSecrets")
		}
	}
}

func TestGetAllVars(t *testing.T) {
	sm, err := NewSecretManager("test-password")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}

	sm.AddGroup("env")
	sm.AddSecret("env", "API_KEY", "secret-value", SecretTypeSecret, "")
	sm.AddGroup("config")
	sm.AddSecret("config", "TIMEOUT", "30", SecretTypeNormal, "")

	vars := sm.GetAllVars()
	if len(vars) != 2 {
		t.Fatalf("GetAllVars returned %d vars, expected 2", len(vars))
	}

	if vars["env_API_KEY"] != "secret-value" {
		t.Fatalf("env_API_KEY = %q, expected %q", vars["env_API_KEY"], "secret-value")
	}

	if vars["config_TIMEOUT"] != "30" {
		t.Fatalf("config_TIMEOUT = %q, expected %q", vars["config_TIMEOUT"], "30")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir, err := os.MkdirTemp("", "secrets-test")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "secrets.enc")

	sm, err := NewSecretManager("master-password")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}

	sm.AddGroup("env")
	sm.AddSecret("env", "API_KEY", "sk-123456", SecretTypeSecret, "test key")
	sm.AddSecret("env", "ENV", "prod", SecretTypeNormal, "")

	err = sm.SaveToFile(path)
	if err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("File permissions = %o, expected 0600", info.Mode().Perm())
	}

	loaded, err := LoadFromFile(path, "master-password")
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	value, _ := loaded.GetSecret("env", "API_KEY")
	if value != "sk-123456" {
		t.Fatalf("Loaded secret value = %q, expected %q", value, "sk-123456")
	}
}

func TestLoadWithWrongPassword(t *testing.T) {
	dir, err := os.MkdirTemp("", "secrets-test")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "secrets.enc")

	sm, _ := NewSecretManager("correct-password")
	sm.AddGroup("env")
	sm.AddSecret("env", "KEY", "value", SecretTypeNormal, "")
	sm.SaveToFile(path)

	_, err = LoadFromFile(path, "wrong-password")
	if err == nil {
		t.Fatal("LoadFromFile should fail with wrong password")
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	sm, err := LoadFromFile("/nonexistent/path/secrets.enc", "password")
	if err != nil {
		t.Fatalf("LoadFromFile with non-existent file should not error: %v", err)
	}
	if sm == nil {
		t.Fatal("LoadFromFile should return a manager for non-existent file")
	}
}

func TestMaskValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ab", "**"},
		{"abcd", "****"},
		{"abcde", "ab*de"},
		{"abcdef", "ab**ef"},
		{"sk-1234567890", "sk*********90"},
	}

	for _, tt := range tests {
		result := maskValue(tt.input)
		if result != tt.expected {
			t.Errorf("maskValue(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestInvalidSecretType(t *testing.T) {
	sm, _ := NewSecretManager("password")
	sm.AddGroup("env")

	err := sm.AddSecret("env", "KEY", "value", "invalid-type", "")
	if err == nil {
		t.Fatal("AddSecret should fail with invalid type")
	}
}

func TestEmptyGroupName(t *testing.T) {
	sm, _ := NewSecretManager("password")

	err := sm.AddGroup("")
	if err == nil {
		t.Fatal("AddGroup should fail with empty name")
	}
}

func TestEmptySecretKey(t *testing.T) {
	sm, _ := NewSecretManager("password")
	sm.AddGroup("env")

	err := sm.AddSecret("env", "", "value", SecretTypeNormal, "")
	if err == nil {
		t.Fatal("AddSecret should fail with empty key")
	}
}

func TestNonExistentGroup(t *testing.T) {
	sm, _ := NewSecretManager("password")

	_, err := sm.GetSecret("nonexistent", "KEY")
	if err == nil {
		t.Fatal("GetSecret should fail with non-existent group")
	}

	err = sm.AddSecret("nonexistent", "KEY", "value", SecretTypeNormal, "")
	if err == nil {
		t.Fatal("AddSecret should fail with non-existent group")
	}

	_, err = sm.ListSecrets("nonexistent")
	if err == nil {
		t.Fatal("ListSecrets should fail with non-existent group")
	}

	err = sm.RemoveGroup("nonexistent")
	if err == nil {
		t.Fatal("RemoveGroup should fail with non-existent group")
	}
}
