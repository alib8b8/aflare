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
	"strings"
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

func TestGetEndpoint_EnvVar(t *testing.T) {
	os.Setenv("TEST_ENDPOINT", "https://custom.endpoint.com")
	defer os.Unsetenv("TEST_ENDPOINT")

	resetForTesting()

	endpoint := GetEndpoint("testprovider", "TEST_ENDPOINT", "https://default.endpoint")
	if endpoint != "https://custom.endpoint.com" {
		t.Errorf("expected 'https://custom.endpoint.com', got '%s'", endpoint)
	}
}

func TestGetEndpoint_ConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "llm-box.yaml")

	configContent := `
providers:
  testprovider:
    endpoint: https://config.endpoint.com
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	os.Setenv("LLM_BOX_CONFIG", configPath)
	defer os.Unsetenv("LLM_BOX_CONFIG")

	resetForTesting()

	endpoint := GetEndpoint("testprovider", "NONEXISTENT_ENDPOINT", "https://default.endpoint")
	if endpoint != "https://config.endpoint.com" {
		t.Errorf("expected 'https://config.endpoint.com', got '%s'", endpoint)
	}
}

func TestGetEndpoint_Default(t *testing.T) {
	resetForTesting()

	endpoint := GetEndpoint("nonexistent", "NONEXISTENT_ENDPOINT", "https://default.endpoint")
	if endpoint != "https://default.endpoint" {
		t.Errorf("expected 'https://default.endpoint', got '%s'", endpoint)
	}
}

func TestIsSafeMode_FalseValues(t *testing.T) {
	falseValues := []string{"false", "0", "no", "off", "disable", "disabled", "FALSE", "NO"}
	for _, val := range falseValues {
		t.Run(val, func(t *testing.T) {
			os.Setenv("LLM_BOX_SAFE_MODE", val)
			defer os.Unsetenv("LLM_BOX_SAFE_MODE")

			resetForTesting()

			if IsSafeMode() {
				t.Errorf("expected safe mode to be disabled for value %q", val)
			}
		})
	}
}

func TestIsSafeMode_TrueValues(t *testing.T) {
	trueValues := []string{"true", "1", "yes", "on", "enable", "enabled", "TRUE", "YES"}
	for _, val := range trueValues {
		t.Run(val, func(t *testing.T) {
			os.Setenv("LLM_BOX_SAFE_MODE", val)
			defer os.Unsetenv("LLM_BOX_SAFE_MODE")

			resetForTesting()

			if !IsSafeMode() {
				t.Errorf("expected safe mode to be enabled for value %q", val)
			}
		})
	}
}

func TestGetSecurityLevel_EnvVar(t *testing.T) {
	levels := []string{"L0", "L1", "L2", "L3", "l0", "l1", "l2", "l3"}
	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			os.Setenv("LLM_BOX_SECURITY_LEVEL", level)
			defer os.Unsetenv("LLM_BOX_SECURITY_LEVEL")

			resetForTesting()

			result := GetSecurityLevel()
			expected := strings.ToUpper(level)
			if result != expected {
				t.Errorf("expected %q, got %q", expected, result)
			}
		})
	}
}

func TestGetSecurityLevel_InvalidEnv(t *testing.T) {
	os.Setenv("LLM_BOX_SECURITY_LEVEL", "INVALID")
	defer os.Unsetenv("LLM_BOX_SECURITY_LEVEL")

	resetForTesting()

	result := GetSecurityLevel()
	if result != "L1" {
		t.Errorf("expected L1 as default for invalid env, got %q", result)
	}
}

func TestGetSecurityLevel_ConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "llm-box.yaml")

	configContent := `
security_level: L2
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	os.Setenv("LLM_BOX_CONFIG", configPath)
	defer os.Unsetenv("LLM_BOX_CONFIG")

	resetForTesting()

	result := GetSecurityLevel()
	if result != "L2" {
		t.Errorf("expected 'L2', got '%s'", result)
	}
}

func TestGetSecurityLevel_SafeModeDefaultsToL3(t *testing.T) {
	os.Setenv("LLM_BOX_SAFE_MODE", "1")
	defer os.Unsetenv("LLM_BOX_SAFE_MODE")

	resetForTesting()

	result := GetSecurityLevel()
	if result != "L3" {
		t.Errorf("expected 'L3' when safe mode is on, got '%s'", result)
	}
}

func TestSecurityLevelAtLeast(t *testing.T) {
	tests := []struct {
		current  string
		required string
		want     bool
	}{
		{"L0", "L0", true},
		{"L1", "L0", true},
		{"L2", "L1", true},
		{"L3", "L2", true},
		{"L3", "L3", true},
		{"L0", "L1", false},
		{"L1", "L2", false},
		{"L0", "L3", false},
		{"L1", "INVALID", false},
	}

	for _, tt := range tests {
		t.Run(tt.current+"_vs_"+tt.required, func(t *testing.T) {
			resetForTesting()
			os.Setenv("LLM_BOX_SECURITY_LEVEL", tt.current)
			defer os.Unsetenv("LLM_BOX_SECURITY_LEVEL")

			got := SecurityLevelAtLeast(tt.required)
			if got != tt.want {
				t.Errorf("SecurityLevelAtLeast(%q) with current %q = %v, want %v",
					tt.required, tt.current, got, tt.want)
			}
		})
	}
}

func TestGetRouterConfig_Default(t *testing.T) {
	resetForTesting()

	cfg := GetRouterConfig()
	if cfg.Strategy != "" {
		t.Errorf("expected empty strategy by default, got %q", cfg.Strategy)
	}
}

func TestGetRouterConfig_ConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "llm-box.yaml")

	configContent := `
router:
  strategy: cost
  max_retries: 3
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	os.Setenv("LLM_BOX_CONFIG", configPath)
	defer os.Unsetenv("LLM_BOX_CONFIG")

	resetForTesting()

	cfg := GetRouterConfig()
	if cfg.Strategy != "cost" {
		t.Errorf("expected strategy 'cost', got '%s'", cfg.Strategy)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected max_retries 3, got %d", cfg.MaxRetries)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "nil providers",
			cfg:     &Config{},
			wantErr: false,
		},
		{
			name:    "default config",
			cfg:     &Config{Providers: make(map[string]LLMProviderConfig)},
			wantErr: false,
		},
		{
			name:    "valid security levels",
			cfg:     &Config{SecurityLevel: SecurityLevelL0},
			wantErr: false,
		},
		{
			name:    "invalid security level",
			cfg:     &Config{SecurityLevel: "invalid"},
			wantErr: true,
		},
		{
			name:    "valid router strategy priority",
			cfg:     &Config{Router: RouterConfig{Strategy: RouterStrategyPriority}},
			wantErr: false,
		},
		{
			name:    "valid router strategy cost",
			cfg:     &Config{Router: RouterConfig{Strategy: RouterStrategyCost}},
			wantErr: false,
		},
		{
			name:    "valid router strategy latency",
			cfg:     &Config{Router: RouterConfig{Strategy: RouterStrategyLatency}},
			wantErr: false,
		},
		{
			name:    "valid router strategy round_robin",
			cfg:     &Config{Router: RouterConfig{Strategy: RouterStrategyRoundRobin}},
			wantErr: false,
		},
		{
			name:    "valid router strategy random",
			cfg:     &Config{Router: RouterConfig{Strategy: RouterStrategyRandom}},
			wantErr: false,
		},
		{
			name:    "invalid router strategy",
			cfg:     &Config{Router: RouterConfig{Strategy: "invalid"}},
			wantErr: true,
		},
		{
			name:    "negative max_retries",
			cfg:     &Config{Router: RouterConfig{MaxRetries: -1}},
			wantErr: true,
		},
		{
			name:    "zero max_retries",
			cfg:     &Config{Router: RouterConfig{MaxRetries: 0}},
			wantErr: false,
		},
		{
			name: "provider missing name",
			cfg: &Config{Router: RouterConfig{FallbackOrder: []RouterProviderEntry{
				{Name: "", Priority: 1},
			}}},
			wantErr: true,
		},
		{
			name: "provider negative priority",
			cfg: &Config{Router: RouterConfig{FallbackOrder: []RouterProviderEntry{
				{Name: "test", Priority: -1},
			}}},
			wantErr: true,
		},
		{
			name: "provider negative cost",
			cfg: &Config{Router: RouterConfig{FallbackOrder: []RouterProviderEntry{
				{Name: "test", CostPer1K: -0.5},
			}}},
			wantErr: true,
		},
		{
			name: "provider negative quota",
			cfg: &Config{Router: RouterConfig{FallbackOrder: []RouterProviderEntry{
				{Name: "test", Quota: -100},
			}}},
			wantErr: true,
		},
		{
			name: "valid provider",
			cfg: &Config{Router: RouterConfig{FallbackOrder: []RouterProviderEntry{
				{Name: "test", Priority: 1, CostPer1K: 0.5, Quota: 1000},
			}}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
