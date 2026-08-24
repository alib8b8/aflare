// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​​‌​‌‌‌‌​‌​​‌‌​​​​‌‌​‌‌​​‌‌‌​​‌​‌​​‌​​‌‌‌‌​‌‌​​​​​​​​​​​​​​​​​​​​‌‌‌​​‌​​​​‌‌​⁠
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
	"fmt"
	"os"
	"strings"

	"github.com/alib8b8/aflare/internal/secrets"
	"golang.org/x/term"
)

// HandleSecrets handles the "secrets" command.
//
//	Usage: aflare secrets <command> [options]
//
// Commands:
//
//	set <group> <key> [value]   Store a secret (prompts for value if omitted)
//	get <group> <key>           Print a secret value (masked unless --raw)
//	list [group]                List groups, or secrets in a group (masked)
//	-h, --help                  Show this help message
//
// Secrets are encrypted at rest in ~/.config/aflare/secrets.dat with AES-256-GCM
// (default) or SM4-GCM (Chinese national standard, selected via
// AFLARE_SECRETS_CIPHER=aes-gcm|sm4-gcm), keyed by a master password from the
// OS keyring (or AFLARE_SECRETS_PASSWORD).
// This is the local-first alternative to storing API keys in plaintext config.
func HandleSecrets(args []string) error {
	if len(args) == 0 {
		PrintSecretsUsage()
		return exitErr(1)
	}

	subCmd := args[0]
	rest := args[1:]
	switch subCmd {
	case "-h", "--help", "help":
		PrintSecretsUsage()
	case "set":
		if err := handleSecretSet(rest); err != nil {
			return err
		}
	case "get":
		if err := handleSecretGet(rest); err != nil {
			return err
		}
	case "list":
		if err := handleSecretList(rest); err != nil {
			return err
		}
	default:
		fmt.Printf("Unknown secrets subcommand: %s\n\n", subCmd)
		PrintSecretsUsage()
		return exitErr(1)
	}
	return nil
}

// loadSecretsOrFail opens the secret manager, exiting with a friendly hint on
// failure. The most common failure on headless Linux / CI / containers is that
// no OS keyring (D-Bus secret service) is available and no master password is
// configured — surface that case explicitly instead of a raw English error.
func loadSecretsOrFail() (*secrets.SecretManager, error) {
	sm, err := secrets.GetSecretManager()
	if err != nil {
		fmt.Printf("❌ 无法打开密钥库：%v\n", err)
		// GetMasterPassword returns this exact phrase when neither keyring nor
		// env var nor interactive terminal can supply a password.
		if strings.Contains(err.Error(), "AFLARE_SECRETS_PASSWORD") || strings.Contains(err.Error(), "password not set") {
			fmt.Println()
			fmt.Println("当前环境似乎没有可用的系统密钥环（headless 服务器 / 容器 / CI 环境常见）。")
			fmt.Println("请设置主密码环境变量后重试：")
			fmt.Println("  export AFLARE_SECRETS_PASSWORD='你的主密码'")
		}
		return nil, exitErr(1)
	}
	return sm, nil
}

// handleSecretSet stores a secret. Usage: aflare secrets set <group> <key> [value]
// If value is omitted, prompts on stderr without echo.
func handleSecretSet(args []string) error {
	if len(args) < 2 {
		fmt.Println("Usage: aflare secrets set <group> <key> [value]")
		fmt.Println("  (value omitted → secure prompt without echo)")
		return exitErr(1)
	}
	group, key := args[0], args[1]
	value := ""
	if len(args) >= 3 {
		value = strings.Join(args[2:], " ")
	} else {
		// Prompt without echo when stdin is a terminal.
		v, err := readSecretValueFromTerminal("Enter value for " + key + ": ")
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			return exitErr(1)
		}
		value = v
	}
	if value == "" {
		fmt.Println("❌ value cannot be empty")
		return exitErr(1)
	}

	sm, err := loadSecretsOrFail()
	if err != nil {
		return err
	}

	if err := secretSet(sm, group, key, value); err != nil {
		fmt.Printf("❌ %v\n", err)
		return exitErr(1)
	}
	if err := sm.SaveToFile(secrets.DefaultPath()); err != nil {
		fmt.Printf("❌ save: %v\n", err)
		return exitErr(1)
	}
	fmt.Printf("✅ saved %s/%s (encrypted, masked below)\n", group, key)
	return nil
}

// secretSet upserts a secret into the in-memory manager (caller persists via
// SaveToFile). Ensures the group exists and updates an existing key rather
// than erroring, so the command is idempotent. Extracted for testability.
func secretSet(sm *secrets.SecretManager, group, key, value string) error {
	if err := sm.AddGroup(group); err != nil {
		// "already exists" is fine; other errors are not.
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("create group: %w", err)
		}
	}
	if err := sm.UpdateSecret(group, key, value); err != nil {
		// Not present yet → add.
		if err := sm.AddSecret(group, key, value, secrets.SecretTypeSecret, ""); err != nil {
			return fmt.Errorf("add secret: %w", err)
		}
	}
	return nil
}

// handleSecretGet prints a secret value. Default masked; --raw prints cleartext.
// Usage: aflare secrets get <group> <key> [--raw]
func handleSecretGet(args []string) error {
	if len(args) < 2 {
		fmt.Println("Usage: aflare secrets get <group> <key> [--raw]")
		return exitErr(1)
	}
	group, key := args[0], args[1]
	raw := false
	for _, a := range args[2:] {
		if a == "--raw" {
			raw = true
		}
	}

	sm, err := loadSecretsOrFail()
	if err != nil {
		return err
	}

	out, err := secretGet(sm, group, key, raw)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return exitErr(1)
	}
	fmt.Println(out)
	return nil
}

// secretGet resolves a secret value (masked unless raw) from the manager.
// Extracted from handleSecretGet for testability (no os.Exit, no store open).
func secretGet(sm *secrets.SecretManager, group, key string, raw bool) (string, error) {
	if raw {
		return sm.GetSecret(group, key)
	}
	return sm.GetSecretMasked(group, key)
}

// handleSecretList lists groups, or secrets within a group (masked).
// Usage: aflare secrets list [group]
func handleSecretList(args []string) error {
	sm, err := loadSecretsOrFail()
	if err != nil {
		return err
	}

	out, err := secretList(sm, args)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return exitErr(1)
	}
	fmt.Print(out)
	return nil
}

// secretList builds the listing text (groups, or secrets in a group, masked).
// Extracted from handleSecretList for testability.
func secretList(sm *secrets.SecretManager, args []string) (string, error) {
	if len(args) == 0 {
		groups := sm.ListGroups()
		if len(groups) == 0 {
			return "(no secret groups yet — use `aflare secrets set <group> <key> [value]`)\n", nil
		}
		var b strings.Builder
		b.WriteString("Groups:\n")
		for _, g := range groups {
			b.WriteString("  " + g + "\n")
		}
		return b.String(), nil
	}

	group := args[0]
	infos, err := sm.ListSecrets(group)
	if err != nil {
		return "", err
	}
	if len(infos) == 0 {
		return fmt.Sprintf("(group %q has no secrets)\n", group), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Secrets in %s (values masked):\n", group)
	for _, info := range infos {
		fmt.Fprintf(&b, "  %-32s  %s\n", info.Key, info.Value)
	}
	return b.String(), nil
}

// PrintSecretsUsage prints usage information for the secrets command.
func PrintSecretsUsage() {
	fmt.Println("Usage: aflare secrets <command> [options]")
	fmt.Println("\nManage secrets and credentials for workflows.")
	fmt.Println("Secrets are AES-256-GCM encrypted at rest in ~/.config/aflare/secrets.dat.")
	fmt.Println("Master password comes from the OS keyring or AFLARE_SECRETS_PASSWORD.")
	fmt.Println("\nCipher suite (env AFLARE_SECRETS_CIPHER):")
	fmt.Println("  aes-gcm   AES-256-GCM (default)")
	fmt.Println("  sm4-gcm   SM4-GCM (Chinese national cryptography standard)")
	fmt.Println("  Existing AES files are read transparently and re-encrypted")
	fmt.Println("  with the selected cipher on the next save.")
	fmt.Println("\nCommands:")
	fmt.Println("  set <group> <key> [value]   Store a secret (prompts for value if omitted)")
	fmt.Println("  get <group> <key> [--raw]   Print a secret (masked, or cleartext with --raw)")
	fmt.Println("  list [group]                List groups, or secrets in a group (masked)")
	fmt.Println("  -h, --help                  Show this help message")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare secrets set openai api_key sk-...")
	fmt.Println("  aflare secrets set openai api_key        # secure prompt, no echo")
	fmt.Println("  aflare secrets get openai api_key")
	fmt.Println("  aflare secrets list openai")
	fmt.Println("  AFLARE_SECRETS_CIPHER=sm4-gcm aflare secrets set openai api_key")
	fmt.Println("\nIn workflows, reference as: {{secret.openai.api_key}}")
}

// readSecretValueFromTerminal prompts on stderr and reads a line from stdin
// without echoing (terminal only). Falls back to an error when stdin is not
// a TTY, so callers don't silently accept piped/empty input as a secret.
func readSecretValueFromTerminal(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("stdin is not a terminal; pass the value as an argument: aflare secrets set <group> <key> <value>")
	}
	fmt.Fprint(os.Stderr, prompt)
	bytes, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr) // newline after the hidden input
	if err != nil {
		return "", fmt.Errorf("failed to read value: %w", err)
	}
	return string(bytes), nil
}
