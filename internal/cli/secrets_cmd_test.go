// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation; either version 3 of the License, or
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
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/secrets"
)

// newManagerAt loads (or creates) a SecretManager at the given path using the
// test master password, bypassing the global default path.
func newManagerAt(t *testing.T, path string) *secrets.SecretManager {
	t.Helper()
	sm, err := secrets.LoadFromFile(path, "test-master-pw")
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	return sm
}

func TestSecretsSetGetList_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "secrets.dat")
	t.Setenv("AFLARE_SECRETS_PASSWORD", "test-master-pw")

	// Use the package's own GetSecretManager path by writing to the real
	// default path is not safe in tests; instead exercise the SecretManager
	// API directly through the exported surface that the CLI uses.
	sm := newManagerAt(t, storePath)
	if err := sm.AddGroup("openai"); err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("AddGroup: %v", err)
	}
	if err := sm.AddSecret("openai", "api_key", "sk-test-12345", secrets.SecretTypeSecret, ""); err != nil {
		t.Fatalf("AddSecret: %v", err)
	}
	if err := sm.SaveToFile(storePath); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}

	// Reload from disk and verify the value survives the encrypt/decrypt cycle.
	sm2 := newManagerAt(t, storePath)
	got, err := sm2.GetSecret("openai", "api_key")
	if err != nil {
		t.Fatalf("GetSecret: %v", err)
	}
	if got != "sk-test-12345" {
		t.Errorf("GetSecret = %q, want %q", got, "sk-test-12345")
	}

	masked, err := sm2.GetSecretMasked("openai", "api_key")
	if err != nil {
		t.Fatalf("GetSecretMasked: %v", err)
	}
	if masked == "sk-test-12345" {
		t.Errorf("masked value should not equal plaintext, got %q", masked)
	}
	if !strings.Contains(masked, "*") {
		t.Errorf("masked value should contain '*', got %q", masked)
	}

	infos, err := sm2.ListSecrets("openai")
	if err != nil {
		t.Fatalf("ListSecrets: %v", err)
	}
	if len(infos) != 1 || infos[0].Key != "api_key" {
		t.Errorf("ListSecrets = %+v, want one entry api_key", infos)
	}
}

func TestSecretsSet_IdempotentUpdate(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "secrets.dat")
	t.Setenv("AFLARE_SECRETS_PASSWORD", "test-master-pw")

	sm := newManagerAt(t, storePath)
	_ = sm.AddGroup("g")
	_ = sm.AddSecret("g", "k", "v1", secrets.SecretTypeSecret, "")

	// Update path (the CLI tries UpdateSecret first, then AddSecret).
	if err := sm.UpdateSecret("g", "k", "v2"); err != nil {
		t.Fatalf("UpdateSecret: %v", err)
	}
	got, _ := sm.GetSecret("g", "k")
	if got != "v2" {
		t.Errorf("after update = %q, want %q", got, "v2")
	}
}

func TestSecretsGet_MissingErrors(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "secrets.dat")
	t.Setenv("AFLARE_SECRETS_PASSWORD", "test-master-pw")

	sm := newManagerAt(t, storePath)
	_ = sm.AddGroup("g")
	if _, err := sm.GetSecret("g", "nope"); err == nil {
		t.Error("GetSecret(missing) should error")
	}
	if _, err := sm.GetSecret("missing-group", "k"); err == nil {
		t.Error("GetSecret(missing group) should error")
	}
}

// TestSecretSetCore_NewAndUpsert exercises the extracted secretSet helper
// (the CLI handler's core logic) for both the add and update paths.
func TestSecretSetCore_NewAndUpsert(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "secrets.dat")
	t.Setenv("AFLARE_SECRETS_PASSWORD", "test-master-pw")

	sm := newManagerAt(t, storePath)

	// Add path: group doesn't exist yet, secretSet creates it.
	if err := secretSet(sm, "svc", "token", "t1"); err != nil {
		t.Fatalf("secretSet(new): %v", err)
	}
	got, _ := sm.GetSecret("svc", "token")
	if got != "t1" {
		t.Errorf("after add = %q, want %q", got, "t1")
	}

	// Upsert path: same key, new value.
	if err := secretSet(sm, "svc", "token", "t2"); err != nil {
		t.Fatalf("secretSet(upsert): %v", err)
	}
	got, _ = sm.GetSecret("svc", "token")
	if got != "t2" {
		t.Errorf("after upsert = %q, want %q", got, "t2")
	}
}

// TestSecretGetCore_MaskedAndRaw exercises the extracted secretGet helper.
func TestSecretGetCore_MaskedAndRaw(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "secrets.dat")
	t.Setenv("AFLARE_SECRETS_PASSWORD", "test-master-pw")

	sm := newManagerAt(t, storePath)
	_ = sm.AddGroup("g")
	_ = sm.AddSecret("g", "k", "sk-1234567890", secrets.SecretTypeSecret, "")

	masked, err := secretGet(sm, "g", "k", false)
	if err != nil {
		t.Fatalf("secretGet(masked): %v", err)
	}
	if masked == "sk-1234567890" {
		t.Error("masked should not equal plaintext")
	}

	raw, err := secretGet(sm, "g", "k", true)
	if err != nil {
		t.Fatalf("secretGet(raw): %v", err)
	}
	if raw != "sk-1234567890" {
		t.Errorf("raw = %q, want plaintext", raw)
	}

	if _, err := secretGet(sm, "g", "missing", false); err == nil {
		t.Error("secretGet(missing) should error")
	}
}

// TestSecretListCore_GroupsAndSecrets exercises the extracted secretList helper
// for both the no-arg (list groups) and group-arg (list secrets) paths.
func TestSecretListCore_GroupsAndSecrets(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "secrets.dat")
	t.Setenv("AFLARE_SECRETS_PASSWORD", "test-master-pw")

	sm := newManagerAt(t, storePath)

	// Empty store → hint text.
	out, err := secretList(sm, nil)
	if err != nil {
		t.Fatalf("secretList(empty): %v", err)
	}
	if !strings.Contains(out, "no secret groups") {
		t.Errorf("empty list = %q, want hint", out)
	}

	// Populate.
	_ = sm.AddGroup("alpha")
	_ = sm.AddGroup("beta")
	_ = sm.AddSecret("alpha", "k1", "v1", secrets.SecretTypeSecret, "")
	_ = sm.AddSecret("alpha", "k2", "v2", secrets.SecretTypeSecret, "")

	// List groups.
	out, err = secretList(sm, nil)
	if err != nil {
		t.Fatalf("secretList(groups): %v", err)
	}
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Errorf("groups list = %q, want alpha+beta", out)
	}

	// List secrets in a group (masked).
	out, err = secretList(sm, []string{"alpha"})
	if err != nil {
		t.Fatalf("secretList(group): %v", err)
	}
	if !strings.Contains(out, "k1") || !strings.Contains(out, "k2") {
		t.Errorf("secrets list = %q, want k1+k2", out)
	}

	// Empty group.
	_ = sm.AddGroup("empty")
	out, err = secretList(sm, []string{"empty"})
	if err != nil {
		t.Fatalf("secretList(empty group): %v", err)
	}
	if !strings.Contains(out, "no secrets") {
		t.Errorf("empty group list = %q, want no-secrets hint", out)
	}

	// Missing group errors.
	if _, err := secretList(sm, []string{"nope"}); err == nil {
		t.Error("secretList(missing group) should error")
	}
}

func TestEnvVarForProvider(t *testing.T) {
	cases := map[string]string{
		"openai":   "OPENAI_API_KEY",
		"deepseek": "DEEPSEEK_API_KEY",
		"qwen":     "QWEN_API_KEY",
		"glm":      "GLM_API_KEY",
		"kimi":     "KIMI_API_KEY",
	}
	for provider, want := range cases {
		if got := envVarForProvider(provider); got != want {
			t.Errorf("envVarForProvider(%q) = %q, want %q", provider, got, want)
		}
	}
}

// TestPrintAPIKeyHint verifies the env-var guidance is printed for cloud
// providers and skipped for ollama (no key) / empty key.
func TestPrintAPIKeyHint(t *testing.T) {
	capture := func(fn func()) string {
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		fn()
		w.Close()
		os.Stdout = old
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		return buf.String()
	}

	// Cloud provider with a key → prints export instruction.
	out := capture(func() { printAPIKeyHint("openai", "sk-abc") })
	if !strings.Contains(out, "OPENAI_API_KEY=sk-abc") {
		t.Errorf("hint for openai = %q, want export OPENAI_API_KEY=sk-abc", out)
	}

	// Empty key → no output.
	out = capture(func() { printAPIKeyHint("openai", "") })
	if out != "" {
		t.Errorf("empty key hint = %q, want empty", out)
	}
}

func TestCloudProviderByChoice(t *testing.T) {
	cases := map[string]string{
		"2": "openai",
		"3": "deepseek",
		"4": "qwen",
		"5": "glm",
		"6": "kimi",
		"1": "",
		"7": "",
		"":  "",
	}
	for choice, want := range cases {
		if got := cloudProviderByChoice(choice); got != want {
			t.Errorf("cloudProviderByChoice(%q) = %q, want %q", choice, got, want)
		}
	}
}

// TestPrintSecretsUsage verifies the usage text covers the new subcommands so
// help output stays in sync with the implemented commands.
func TestPrintSecretsUsage(t *testing.T) {
	var buf bytes.Buffer
	// PrintSecretsUsage writes to stdout; capture it.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	PrintSecretsUsage()
	w.Close()
	os.Stdout = old
	_, _ = buf.ReadFrom(r)

	out := buf.String()
	for _, want := range []string{"set", "get", "list", "{{secret.openai.api_key}}", "AES-256-GCM"} {
		if !strings.Contains(out, want) {
			t.Errorf("usage missing %q in output:\n%s", want, out)
		}
	}
}

// TestHandleSecrets_HelpDispatch verifies the -h/--help/help dispatch paths
// print usage without calling os.Exit. These are the only HandleSecrets
// branches safe to test in-process (others os.Exit on error).
func TestHandleSecrets_HelpDispatch(t *testing.T) {
	capture := func(args []string) string {
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		HandleSecrets(args)
		w.Close()
		os.Stdout = old
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		return buf.String()
	}

	for _, flag := range []string{"-h", "--help", "help"} {
		out := capture([]string{flag})
		if !strings.Contains(out, "Usage: aflare secrets") {
			t.Errorf("HandleSecrets(%q) = %q, want usage text", flag, out)
		}
	}
}

// TestReadSecretValueFromTerminal_NonTTY verifies the non-terminal path
// returns an error (rather than silently accepting piped input as a secret).
func TestReadSecretValueFromTerminal_NonTTY(t *testing.T) {
	// In `go test`, stdin is not a terminal, so this exercises the error path.
	_, err := readSecretValueFromTerminal("prompt: ")
	if err == nil {
		t.Error("readSecretValueFromTerminal should error when stdin is not a TTY")
	}
	if !strings.Contains(err.Error(), "not a terminal") {
		t.Errorf("error = %q, want 'not a terminal' hint", err)
	}
}

// TestWriteWizardConfig_NoPlaintextAPIKey verifies that the wizard does not
// persist the api_key to config.yaml — the core of the data-sensitive fix.
func TestWriteWizardConfig_NoPlaintextAPIKey(t *testing.T) {
	dir := t.TempDir()
	// Point the wizard at a temp config path by overriding the configFilePath
	// indirectly: writeWizardConfig reads configFilePath(), which consults
	// AFLARE_CONFIG. Set it to a temp file.
	cfgPath := filepath.Join(dir, "config.yaml")
	t.Setenv("AFLARE_CONFIG", cfgPath)

	_, err := writeWizardConfig("openai", "gpt-4", "sk-secret-12345", "https://api.openai.com/v1")
	if err != nil {
		t.Fatalf("writeWizardConfig: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	body := string(data)
	if strings.Contains(body, "sk-secret-12345") {
		t.Errorf("config.yaml contains plaintext API key:\n%s", body)
	}
	if !strings.Contains(body, "openai") {
		t.Errorf("config.yaml missing provider entry:\n%s", body)
	}
}
