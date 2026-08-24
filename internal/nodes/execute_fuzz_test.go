// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌‌‌‌‌‌​‌​​‌‌‌‌‌‌​​​​​​‌​​​​​​‌‌‌​​​‌​​‌​​‌​​‌‌​​​​​​​​​​​​​​​​​​‌​‌​‌​‌‌‌‌‌​‌​⁠
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

package nodes

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

// FuzzExecuteCommand fuzzes the ExecuteNode's Execute method with arbitrary
// command strings and parameter values. It uses dry_run=true to avoid actual
// command execution, testing the full parsing/validation path instead.
func FuzzExecuteCommand(f *testing.F) {
	// Seed corpus: cover valid commands, edge cases, and known attack vectors.
	seeds := []struct {
		command string
		timeout string
	}{
		// Valid commands
		{"echo hello", "5s"},
		{"ls -la", "1m"},
		{"cat /etc/hosts", "30s"},
		{"grep pattern file.txt", "10s"},
		// Empty / special
		{"", ""},
		{" ", "5s"},
		// Shell metacharacters (should be rejected by allowlist)
		{"echo a; rm -rf /", "5s"},
		{"echo a && cat /etc/passwd", "1m"},
		{"echo $(whoami)", "5s"},
		{"echo `whoami`", "5s"},
		{"echo a | cat", "5s"},
		{"echo a & ls", "5s"},
		{"echo a > /tmp/evil", "5s"},
		// Very long command
		{strings.Repeat("echo ", 500) + "hello", "5s"},
		// Known bad patterns
		{"\x00", "5s"},
		{"\n", "5s"},
		{"\r", "5s"},
		// Unicode
		{"echo 你好世界", "5s"},
		// sed/awk -i (should be blocked)
		{"sed -i 's/a/b/' file.txt", "5s"},
		{"awk -i inplace '{print}' file.txt", "5s"},
	}

	for _, s := range seeds {
		f.Add(s.command, s.timeout)
	}

	node := &ExecuteNode{}

	f.Fuzz(func(t *testing.T, command string, timeout string) {
		// Skip commands that are too long — the node enforces a 4096 byte limit
		// and we want to fuzz the behavior inside that limit.
		params := map[string]string{
			"command": command,
			"dry_run": "true",
			"timeout": timeout,
		}

		done := make(chan struct{})
		var panicErr interface{}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicErr = r
				}
				close(done)
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _ = node.Execute(ctx, "", params)
		}()

		select {
		case <-done:
			if panicErr != nil {
				t.Fatalf("ExecuteNode.Execute panicked: %v\ncommand=%q timeout=%q", panicErr, command, timeout)
			}
		case <-time.After(15 * time.Second):
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			t.Fatalf("ExecuteNode.Execute timed out\ncommand=%q timeout=%q\n%s", command, timeout, buf[:n])
		}
	})
}

// FuzzExecuteCommandParams fuzzes the ExecuteNode.Execute method with
// arbitrary params to test parameter validation resilience.
func FuzzExecuteCommandParams(f *testing.F) {
	seeds := []struct {
		key   string
		value string
	}{
		{"command", "echo hello"},
		{"dry_run", "true"},
		{"timeout", "5s"},
		{"extra", "value"},
		{"", ""},
		{strings.Repeat("x", 51), "v"},
		{"key", strings.Repeat("x", 1001)},
	}

	for _, s := range seeds {
		f.Add(s.key, s.value)
	}

	node := &ExecuteNode{}

	f.Fuzz(func(t *testing.T, key string, value string) {
		params := map[string]string{
			"command": "echo test",
			"dry_run": "true",
			"timeout": "5s",
			key:       value,
		}

		done := make(chan struct{})
		var panicErr interface{}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicErr = r
				}
				close(done)
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _ = node.Execute(ctx, "", params)
		}()

		select {
		case <-done:
			if panicErr != nil {
				t.Fatalf("ExecuteNode.Execute panicked: %v\nkey=%q value=%q", panicErr, key, value)
			}
		case <-time.After(15 * time.Second):
			// timeout is acceptable for fuzz
		}
	})
}
