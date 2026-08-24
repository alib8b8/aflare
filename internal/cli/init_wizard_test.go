// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌‌‌​​​‌‌‌​‌​​‌‌‌​‌‌‌‌‌​​​‌‌‌​‌​‌‌​​‌‌‌​​​‌‌​​​​​​​​​​​​​​​​​​​‌‌​​‌‌​​‌​‌‌​‌‌​⁠
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

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alib8b8/aflare/internal/config"
	"gopkg.in/yaml.v3"
)

func TestWriteWizardConfig_CreatesNewConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AFLARE_CONFIG", filepath.Join(dir, "config.yaml"))

	cfgPath, err := writeWizardConfig("ollama", "llama3", "", "http://localhost:11434")
	if err != nil {
		t.Fatalf("writeWizardConfig failed: %v", err)
	}

	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	pcfg, ok := cfg.Providers["ollama"]
	if !ok {
		t.Fatal("ollama provider not found in config")
	}
	if pcfg.Model != "llama3" {
		t.Errorf("model = %q, want llama3", pcfg.Model)
	}
	if pcfg.Endpoint != "http://localhost:11434" {
		t.Errorf("endpoint = %q, want http://localhost:11434", pcfg.Endpoint)
	}
}

func TestWriteWizardConfig_SkipsProvider(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AFLARE_CONFIG", filepath.Join(dir, "config.yaml"))

	// Write an initial config with openai.
	_, err := writeWizardConfig("openai", "gpt-4o-mini", "sk-test", "")
	if err != nil {
		t.Fatalf("first writeWizardConfig failed: %v", err)
	}

	// Reset config singleton to reload.
	config.SetConfig(nil)
	t.Cleanup(func() { config.SetConfig(nil) })

	// Write again with provider="" (skip) — should preserve openai.
	_, err = writeWizardConfig("", "", "", "")
	if err != nil {
		t.Fatalf("second writeWizardConfig failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	// openai entry should still be present.
	cfg := &config.Config{Providers: map[string]config.LLMProviderConfig{}}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := cfg.Providers["openai"]; !ok {
		t.Error("openai provider was clobbered by skip write")
	}
}

func TestWriteWizardConfig_PreservesExistingProviders(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	// Pre-write a config with deepseek.
	existing := &config.Config{
		Providers: map[string]config.LLMProviderConfig{
			"deepseek": {APIKey: "sk-existing", Model: "deepseek-chat"},
		},
		SecurityLevel: config.SecurityLevelL2,
	}
	data, _ := yaml.Marshal(existing)
	_ = os.WriteFile(cfgPath, data, 0600)

	t.Setenv("AFLARE_CONFIG", cfgPath)

	// Add ollama via wizard — deepseek should be preserved.
	_, err := writeWizardConfig("ollama", "llama3", "", "http://localhost:11434")
	if err != nil {
		t.Fatalf("writeWizardConfig failed: %v", err)
	}

	loaded := &config.Config{Providers: map[string]config.LLMProviderConfig{}}
	raw, _ := os.ReadFile(cfgPath)
	if err := yaml.Unmarshal(raw, loaded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := loaded.Providers["deepseek"]; !ok {
		t.Error("deepseek provider was clobbered")
	}
	if _, ok := loaded.Providers["ollama"]; !ok {
		t.Error("ollama provider was not added")
	}
	if loaded.SecurityLevel != config.SecurityLevelL2 {
		t.Errorf("security_level = %q, want L2", loaded.SecurityLevel)
	}
}

func TestDetectLLMConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AFLARE_CONFIG", filepath.Join(dir, "config.yaml"))
	config.SetConfig(nil)
	t.Cleanup(func() { config.SetConfig(nil) })

	// No config file → false.
	if detectLLMConfig() {
		t.Error("expected false with no config")
	}

	// Write config with ollama (no key needed). writeWizardConfig injects the
	// new config into the cache via config.SetConfig, so detectLLMConfig can
	// read it without a restart.
	_, err := writeWizardConfig("ollama", "llama3", "", "http://localhost:11434")
	if err != nil {
		t.Fatalf("writeWizardConfig failed: %v", err)
	}
	if !detectLLMConfig() {
		t.Error("expected true with ollama configured")
	}
}

func TestConfigFilePath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AFLARE_CONFIG", filepath.Join(dir, "custom.yaml"))
	if got := configFilePath(); got != filepath.Join(dir, "custom.yaml") {
		t.Errorf("configFilePath = %q, want custom.yaml", got)
	}
}

func TestRedactAPIKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sk-abcd1234efgh5678ijkl", "sk-a***************ijkl"}, // 23 chars: 4 + 15 stars + 4
		{"short", "*****"},         // len <= 8 → all masked (5 stars)
		{"12345678", "********"},   // len == 8 → all masked
		{"123456789", "1234*6789"}, // 9 chars: 4 + 1 star + 4
	}
	for _, tt := range tests {
		got := redactAPIKey(tt.input)
		if got != tt.want {
			t.Errorf("redactAPIKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
