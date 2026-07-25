// Copyright (c) 2026 llm-box Contributors
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

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetAPIKey_EnvVar(t *testing.T) {
	os.Setenv("TEST_API_KEY", "test-key-123")
	defer os.Unsetenv("TEST_API_KEY")

	apiKey := GetAPIKey("test", "TEST_API_KEY")
	if apiKey != "test-key-123" {
		t.Errorf("expected 'test-key-123', got '%s'", apiKey)
	}
}

func TestGetAPIKey_ConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "llm-box.yaml")

	configContent := `
providers:
  testprovider:
    api_key: config-key-456
    model: custom-model
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	os.Setenv("LLM_BOX_CONFIG", configPath)
	defer os.Unsetenv("LLM_BOX_CONFIG")

	resetForTesting()

	apiKey := GetAPIKey("testprovider", "TESTPROVIDER_API_KEY")
	if apiKey != "config-key-456" {
		t.Errorf("expected 'config-key-456', got '%s'", apiKey)
	}
}

func TestGetDefaultModel_ConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "llm-box.yaml")

	configContent := `
providers:
  myprovider:
    model: my-custom-model
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	os.Setenv("LLM_BOX_CONFIG", configPath)
	defer os.Unsetenv("LLM_BOX_CONFIG")

	resetForTesting()

	model := GetDefaultModel("myprovider", "MYPROVIDER_MODEL", "default-model")
	if model != "my-custom-model" {
		t.Errorf("expected 'my-custom-model', got '%s'", model)
	}
}

func TestGetDefaultModel_Default(t *testing.T) {
	resetForTesting()

	model := GetDefaultModel("nonexistent", "NONEXISTENT_MODEL", "default-model")
	if model != "default-model" {
		t.Errorf("expected 'default-model', got '%s'", model)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "llm-box.yaml")

	err := os.WriteFile(configPath, []byte("invalid: yaml: :"), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	os.Setenv("LLM_BOX_CONFIG", configPath)
	defer os.Unsetenv("LLM_BOX_CONFIG")

	resetForTesting()

	_, err = LoadConfig()
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestIsSafeMode(t *testing.T) {
	os.Setenv("LLM_BOX_SAFE_MODE", "1")
	defer os.Unsetenv("LLM_BOX_SAFE_MODE")

	resetForTesting()

	if !IsSafeMode() {
		t.Error("expected safe mode to be enabled")
	}
}

func TestSetConfig(t *testing.T) {
	resetForTesting()

	cfg := &Config{
		SafeMode: true,
		Providers: map[string]LLMProviderConfig{
			"test": {APIKey: "set-key"},
		},
	}

	SetConfig(cfg)

	if !IsSafeMode() {
		t.Error("expected safe mode to be enabled")
	}

	apiKey := GetAPIKey("test", "TEST_API_KEY")
	if apiKey != "set-key" {
		t.Errorf("expected 'set-key', got '%s'", apiKey)
	}
}
