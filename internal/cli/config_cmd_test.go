// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​​‌‌‌‌‌‌‌‌​​‌​‌‌‌​​‌​​‌‌​‌​​‌‌‌‌​‌​‌​‌​​‌​​​‌‌​​‌​​​​​​​​​​​​​​​​​‌‌‌‌‌​‌‌​​‌​‌‌​⁠
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
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/config"
)

// TestConfigCmdShowMissingConfig must be the first test in the package binary
// that reaches config.LoadConfig(): the config package caches the first load
// in a sync.Once, so the "load failed" notice branch of handleConfigShow is
// only reachable while the singleton is still uninitialized. A malformed
// AFLARE_CONFIG file makes that first load fail deterministically.
func TestConfigCmdShowMissingConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("providers: [unclosed"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AFLARE_CONFIG", cfgPath)
	config.SetConfig(nil)
	t.Cleanup(func() { config.SetConfig(nil) })

	// Guard against test-order changes: if the singleton was already
	// initialized successfully by an earlier test, the notice branch is
	// unreachable in this binary run.
	if _, err := config.LoadConfig(); err == nil {
		t.Skip("config singleton already initialized; notice path unreachable")
	}

	var err error
	output := captureOutput(func() {
		err = HandleConfig([]string{"show"})
	})
	if err != nil {
		t.Fatalf("HandleConfig([show]) = %v, want nil", err)
	}
	if !strings.Contains(output, "状态：未配置或加载失败") {
		t.Errorf("expected load-failure notice, got: %s", output)
	}
	if !strings.Contains(output, "运行 aflare init") {
		t.Errorf("expected init hint, got: %s", output)
	}
	if !strings.Contains(output, cfgPath) {
		t.Errorf("expected config path %q in output, got: %s", cfgPath, output)
	}
}

func TestConfigCmdShowWithProviders(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config.SetConfig(&config.Config{
		Providers: map[string]config.LLMProviderConfig{
			"ollama": {Model: "llama3", Endpoint: "http://localhost:11434"},
			"openai": {Model: "gpt-4o-mini", APIKey: "sk-abcd1234efgh5678ijkl"},
		},
		SecurityLevel: config.SecurityLevelL2,
	})
	t.Cleanup(func() { config.SetConfig(nil) })

	var err error
	output := captureOutput(func() {
		err = HandleConfig([]string{"show"})
	})
	if err != nil {
		t.Fatalf("HandleConfig([show]) = %v, want nil", err)
	}
	for _, want := range []string{
		"配置文件：",
		"ollama:",
		"openai:",
		"model:    gpt-4o-mini",
		"api_key:  sk-a***************ijkl", // redacted
		"endpoint: http://localhost:11434",
		"安全等级：L2",
		"Safe Mode：false",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("show output missing %q, got: %s", want, output)
		}
	}
	if strings.Contains(output, "sk-abcd1234efgh5678ijkl") {
		t.Error("raw API key leaked in show output")
	}
	// Providers are printed in sorted order: ollama before openai.
	if strings.Index(output, "ollama:") > strings.Index(output, "openai:") {
		t.Errorf("expected ollama listed before openai, got: %s", output)
	}
}

func TestConfigCmdShowNoProviders(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := &config.Config{Providers: map[string]config.LLMProviderConfig{}}
	config.SetConfig(cfg)
	t.Cleanup(func() { config.SetConfig(nil) })

	var err error
	output := captureOutput(func() {
		err = HandleConfig([]string{"show"})
	})
	if err != nil {
		t.Fatalf("HandleConfig([show]) = %v, want nil", err)
	}
	if !strings.Contains(output, "LLM Providers：（未配置）") {
		t.Errorf("expected empty-providers notice, got: %s", output)
	}
	if !strings.Contains(output, "安全等级：L1 (default)") {
		t.Errorf("expected default security level, got: %s", output)
	}

	// safe_mode on upgrades the displayed default level to L3.
	cfg.SafeMode = true
	output = captureOutput(func() {
		err = HandleConfig([]string{"show"})
	})
	if err != nil {
		t.Fatalf("HandleConfig([show]) = %v, want nil", err)
	}
	if !strings.Contains(output, "安全等级：L3 (safe_mode)") {
		t.Errorf("expected safe-mode security level, got: %s", output)
	}
	if !strings.Contains(output, "Safe Mode：true") {
		t.Errorf("expected Safe Mode true, got: %s", output)
	}
}

func TestConfigCmdNoArgs(t *testing.T) {
	var err error
	output := captureOutput(func() {
		err = HandleConfig(nil)
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleConfig(nil) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(output, "Usage: aflare config") {
		t.Errorf("expected usage output, got: %s", output)
	}
}

func TestConfigCmdHelp(t *testing.T) {
	for _, arg := range []string{"--help", "-h", "help"} {
		var err error
		output := captureOutput(func() {
			err = HandleConfig([]string{arg})
		})
		if err != nil {
			t.Errorf("HandleConfig([%q]) = %v, want nil", arg, err)
		}
		if !strings.Contains(output, "Usage: aflare config") {
			t.Errorf("expected usage output for %q, got: %s", arg, output)
		}
	}
}

func TestConfigCmdUnknownSubcommand(t *testing.T) {
	var err error
	output := captureOutput(func() {
		err = HandleConfig([]string{"frobnicate"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleConfig([frobnicate]) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(output, "Unknown config subcommand") {
		t.Errorf("expected unknown-subcommand message, got: %s", output)
	}
}

func TestConfigCmdSetUsageError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var err error
	output := captureOutput(func() {
		err = HandleConfig([]string{"set", "llm.provider"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleConfig([set llm.provider]) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(output, "config set requires <key> <value>") {
		t.Errorf("expected set usage message, got: %s", output)
	}
}

func TestConfigCmdSetInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{"unknown key", []string{"set", "nope", "x"}, "未知的配置项"},
		{"invalid security level", []string{"set", "security_level", "L9"}, "无效的安全等级"},
		{"invalid safe_mode", []string{"set", "safe_mode", "maybe"}, "无效的布尔值"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			var err error
			output := captureOutput(func() {
				err = HandleConfig(tt.args)
			})
			if code := exitCodeForErr(err); code != 1 {
				t.Errorf("HandleConfig(%v) exit code = %d, want 1 (err=%v)", tt.args, code, err)
			}
			if !strings.Contains(output, tt.wantMsg) {
				t.Errorf("HandleConfig(%v) output missing %q, got: %s", tt.args, tt.wantMsg, output)
			}
		})
	}
}

func TestConfigCmdSetProvider(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AFLARE_PROVIDER", "")

	var err error
	output := captureOutput(func() {
		err = HandleConfig([]string{"set", "llm.provider", "openai"})
	})
	if err != nil {
		t.Fatalf("HandleConfig([set llm.provider openai]) = %v, want nil", err)
	}
	if !strings.Contains(output, "已设置 llm.provider = openai") {
		t.Errorf("expected set confirmation, got: %s", output)
	}

	data, rerr := os.ReadFile(filepath.Join(home, ".config", "aflare", "config.yaml"))
	if rerr != nil {
		t.Fatalf("config file not written: %v", rerr)
	}
	body := string(data)
	if !strings.Contains(body, "openai") {
		t.Errorf("config missing openai provider entry:\n%s", body)
	}
	// Provider defaults from providerDefaultsMap are applied.
	if !strings.Contains(body, "gpt-4o-mini") {
		t.Errorf("config missing default model for openai:\n%s", body)
	}
}

func TestConfigCmdSetProviderFields(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		value  string
		wantIn string
	}{
		{"model", "llm.model", "qwen3-max", "model: qwen3-max"},
		{"api_key", "llm.api_key", "sk-test-12345678", "api_key: sk-test-12345678"},
		{"endpoint", "llm.endpoint", "https://api.example.com/v1", "endpoint: https://api.example.com/v1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("AFLARE_PROVIDER", "")

			var err error
			captureOutput(func() {
				err = HandleConfig([]string{"set", tt.key, tt.value})
			})
			if err != nil {
				t.Fatalf("HandleConfig([set %s %s]) = %v, want nil", tt.key, tt.value, err)
			}

			// With no prior config the values land on the default
			// "ollama" provider entry.
			data, rerr := os.ReadFile(filepath.Join(home, ".config", "aflare", "config.yaml"))
			if rerr != nil {
				t.Fatalf("config file not written: %v", rerr)
			}
			if !strings.Contains(string(data), tt.wantIn) {
				t.Errorf("config = %s, want it to contain %q", data, tt.wantIn)
			}
			if !strings.Contains(string(data), "ollama") {
				t.Errorf("config missing default ollama provider entry:\n%s", data)
			}
		})
	}
}

func TestConfigCmdSetAPIKeyRedactedEcho(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AFLARE_PROVIDER", "")

	var err error
	output := captureOutput(func() {
		err = HandleConfig([]string{"set", "llm.api_key", "sk-abcd1234efgh5678ijkl"})
	})
	if err != nil {
		t.Fatalf("HandleConfig([set llm.api_key ...]) = %v, want nil", err)
	}
	if strings.Contains(output, "sk-abcd1234efgh5678ijkl") {
		t.Error("raw API key echoed to stdout")
	}
	if !strings.Contains(output, "sk-a***************ijkl") {
		t.Errorf("expected redacted key echo, got: %s", output)
	}
}

func TestConfigCmdSetModelTargetsCurrentProvider(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AFLARE_PROVIDER", "")

	// First set llm.provider so the on-disk config has a single openai
	// entry; a subsequent llm.model set must apply to that provider.
	var first, second error
	captureOutput(func() {
		first = HandleConfig([]string{"set", "llm.provider", "openai"})
	})
	if first != nil {
		t.Fatalf("set llm.provider = %v, want nil", first)
	}
	captureOutput(func() {
		second = HandleConfig([]string{"set", "llm.model", "my-custom-model"})
	})
	if second != nil {
		t.Fatalf("set llm.model = %v, want nil", second)
	}

	data, rerr := os.ReadFile(filepath.Join(home, ".config", "aflare", "config.yaml"))
	if rerr != nil {
		t.Fatalf("config file not written: %v", rerr)
	}
	body := string(data)
	if !strings.Contains(body, "model: my-custom-model") {
		t.Errorf("custom model not persisted:\n%s", body)
	}
	if strings.Contains(body, "ollama") {
		t.Errorf("model applied to default ollama provider instead of current openai:\n%s", body)
	}
}

func TestConfigCmdSetSecurityLevel(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		wantIn string
	}{
		{"uppercase", "L2", "security_level: L2"},
		{"lowercase is normalized", "l3", "security_level: L3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			var err error
			captureOutput(func() {
				err = HandleConfig([]string{"set", "security_level", tt.value})
			})
			if err != nil {
				t.Fatalf("set security_level %s = %v, want nil", tt.value, err)
			}
			data, rerr := os.ReadFile(filepath.Join(home, ".config", "aflare", "config.yaml"))
			if rerr != nil {
				t.Fatalf("config file not written: %v", rerr)
			}
			if !strings.Contains(string(data), tt.wantIn) {
				t.Errorf("config = %s, want it to contain %q", data, tt.wantIn)
			}
		})
	}
}

func TestConfigCmdSetSafeMode(t *testing.T) {
	for _, val := range []string{"true", "1", "yes", "on"} {
		t.Run("true alias "+val, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			var err error
			captureOutput(func() {
				err = HandleConfig([]string{"set", "safe_mode", val})
			})
			if err != nil {
				t.Fatalf("set safe_mode %s = %v, want nil", val, err)
			}
			data, rerr := os.ReadFile(filepath.Join(home, ".config", "aflare", "config.yaml"))
			if rerr != nil {
				t.Fatalf("config file not written: %v", rerr)
			}
			if !strings.Contains(string(data), "safe_mode: true") {
				t.Errorf("config = %s, want it to contain %q", data, "safe_mode: true")
			}
		})
	}
	for _, val := range []string{"false", "0", "no", "off"} {
		t.Run("false alias "+val, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			var err error
			captureOutput(func() {
				err = HandleConfig([]string{"set", "safe_mode", val})
			})
			if err != nil {
				t.Fatalf("set safe_mode %s = %v, want nil", val, err)
			}
			// false is dropped by omitempty, so the key must be absent.
			data, rerr := os.ReadFile(filepath.Join(home, ".config", "aflare", "config.yaml"))
			if rerr != nil {
				t.Fatalf("config file not written: %v", rerr)
			}
			if strings.Contains(string(data), "safe_mode: true") {
				t.Errorf("config = %s, want safe_mode unset for %q", data, val)
			}
		})
	}
}

func TestConfigCmdSetOnConfigWithoutProviders(t *testing.T) {
	// An existing config with no providers key unmarshals to a nil map;
	// handleConfigSet must re-initialize it before adding the default
	// ollama entry.
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgPath := filepath.Join(home, ".config", "aflare", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("security_level: L2\n"), 0600); err != nil {
		t.Fatal(err)
	}

	var err error
	captureOutput(func() {
		err = HandleConfig([]string{"set", "llm.model", "llama3"})
	})
	if err != nil {
		t.Fatalf("set llm.model = %v, want nil", err)
	}
	data, rerr := os.ReadFile(cfgPath)
	if rerr != nil {
		t.Fatalf("config file not re-written: %v", rerr)
	}
	body := string(data)
	if !strings.Contains(body, "ollama") {
		t.Errorf("default ollama provider entry missing:\n%s", body)
	}
	if !strings.Contains(body, "security_level: L2") {
		t.Errorf("existing security_level not preserved:\n%s", body)
	}
}

func TestConfigCmdSetEnvProvider(t *testing.T) {
	// AFLARE_PROVIDER pins the "current" provider for llm.model and friends.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AFLARE_PROVIDER", "deepseek")

	var err error
	captureOutput(func() {
		err = HandleConfig([]string{"set", "llm.model", "deepseek-chat"})
	})
	if err != nil {
		t.Fatalf("set llm.model = %v, want nil", err)
	}
	data, rerr := os.ReadFile(filepath.Join(home, ".config", "aflare", "config.yaml"))
	if rerr != nil {
		t.Fatalf("config file not written: %v", rerr)
	}
	body := string(data)
	if !strings.Contains(body, "deepseek") {
		t.Errorf("model not applied to deepseek provider:\n%s", body)
	}
	if strings.Contains(body, "ollama") {
		t.Errorf("model applied to default ollama provider instead of env-pinned deepseek:\n%s", body)
	}
}

func TestConfigCmdSetMalformedConfigFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgPath := filepath.Join(home, ".config", "aflare", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("providers: [unclosed"), 0600); err != nil {
		t.Fatal(err)
	}

	var err error
	output := captureOutput(func() {
		err = HandleConfig([]string{"set", "llm.model", "x"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleConfig([set llm.model x]) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(output, "解析配置文件失败") {
		t.Errorf("expected parse-failure message, got: %s", output)
	}
}

func TestConfigCmdPrintUsage(t *testing.T) {
	output := captureOutput(func() {
		PrintConfigUsage()
	})
	if !strings.Contains(output, "Usage: aflare config <show|set>") {
		t.Errorf("expected usage header, got: %s", output)
	}
	for _, key := range []string{"llm.provider", "llm.model", "llm.api_key", "llm.endpoint", "security_level", "safe_mode"} {
		if !strings.Contains(output, key) {
			t.Errorf("usage output missing %q, got: %s", key, output)
		}
	}
}

func TestConfigCmdRedactConfigValue(t *testing.T) {
	tests := []struct {
		key   string
		value string
		want  string
	}{
		{"llm.api_key", "sk-abcd1234efgh5678ijkl", "sk-a***************ijkl"},
		{"llm.api_key", "short", "*****"},
		{"llm.model", "gpt-4o-mini", "gpt-4o-mini"},
		{"security_level", "L2", "L2"},
	}
	for _, tt := range tests {
		if got := redactConfigValue(tt.key, tt.value); got != tt.want {
			t.Errorf("redactConfigValue(%q, %q) = %q, want %q", tt.key, tt.value, got, tt.want)
		}
	}
}
