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
// Secrets are AES-256-GCM encrypted at rest in ~/.config/aflare/secrets.dat,
// keyed by a master password from the OS keyring (or AFLARE_SECRETS_PASSWORD).
// This is the local-first alternative to storing API keys in plaintext config.
func HandleSecrets(args []string) {
	if len(args) == 0 {
		PrintSecretsUsage()
		os.Exit(1)
	}

	subCmd := args[0]
	rest := args[1:]
	switch subCmd {
	case "-h", "--help", "help":
		PrintSecretsUsage()
	case "set":
		handleSecretSet(rest)
	case "get":
		handleSecretGet(rest)
	case "list":
		handleSecretList(rest)
	default:
		fmt.Printf("Unknown secrets subcommand: %s\n\n", subCmd)
		PrintSecretsUsage()
		os.Exit(1)
	}
}

// handleSecretSet stores a secret. Usage: aflare secrets set <group> <key> [value]
// If value is omitted, prompts on stderr without echo.
func handleSecretSet(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: aflare secrets set <group> <key> [value]")
		fmt.Println("  (value omitted → secure prompt without echo)")
		os.Exit(1)
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
			os.Exit(1)
		}
		value = v
	}
	if value == "" {
		fmt.Println("❌ value cannot be empty")
		os.Exit(1)
	}

	sm, err := secrets.GetSecretManager()
	if err != nil {
		fmt.Printf("❌ failed to open secrets store: %v\n", err)
		os.Exit(1)
	}

	if err := secretSet(sm, group, key, value); err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
	if err := sm.SaveToFile(secrets.DefaultPath()); err != nil {
		fmt.Printf("❌ save: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ saved %s/%s (encrypted, masked below)\n", group, key)
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
func handleSecretGet(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: aflare secrets get <group> <key> [--raw]")
		os.Exit(1)
	}
	group, key := args[0], args[1]
	raw := false
	for _, a := range args[2:] {
		if a == "--raw" {
			raw = true
		}
	}

	sm, err := secrets.GetSecretManager()
	if err != nil {
		fmt.Printf("❌ failed to open secrets store: %v\n", err)
		os.Exit(1)
	}

	out, err := secretGet(sm, group, key, raw)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
	fmt.Println(out)
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
func handleSecretList(args []string) {
	sm, err := secrets.GetSecretManager()
	if err != nil {
		fmt.Printf("❌ failed to open secrets store: %v\n", err)
		os.Exit(1)
	}

	out, err := secretList(sm, args)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
	fmt.Print(out)
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
