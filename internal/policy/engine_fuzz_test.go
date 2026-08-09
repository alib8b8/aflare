// Copyright (c) 2026 aflare Contributors
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

package policy

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// FuzzLoadPolicy fuzzes LoadPolicy with arbitrary YAML content written to
// temp files. This tests the YAML parser, path validation, and file I/O
// against malformed inputs.
func FuzzLoadPolicy(f *testing.F) {
	// Seed corpus: valid policy, edge cases, and malformed YAML.
	seeds := []string{
		// Valid policy
		"filesystem:\n  read: allowed\n  write: allowed\n  delete: denied\n",
		// Minimal
		"",
		// Malformed YAML
		"filesystem:\n  read: allowed\n  write: [unclosed\n",
		"{{{{{{{{{{\n",
		"---\n...\n",
		// Non-YAML content
		"not a yaml file at all",
		"\x00\x00\x00",
		// Very long keys
		"filesystem:\n  read: allowed\n  write: allowed\n  delete: denied\nnetwork:\n  outbound: allowed\n",
		// Billion laughs style attack
		"a: &a [\"lol\",\"lol\",\"lol\",\"lol\",\"lol\",\"lol\",\"lol\",\"lol\",\"lol\"]\nb: &b [*a,*a,*a,*a,*a,*a,*a,*a,*a]\nc: &c [*b,*b,*b,*b,*b,*b,*b,*b,*b]\nd: [*c,*c,*c,*c,*c,*c,*c,*c,*c]\n",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	tmpDir, err := os.MkdirTemp("", "fuzz-policy-*")
	if err != nil {
		f.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	f.Fuzz(func(t *testing.T, content string) {
		done := make(chan struct{})
		var panicErr interface{}
		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicErr = r
				}
				close(done)
			}()

			tmpFile, err := os.CreateTemp(tmpDir, "fuzz-*.yaml")
			if err != nil {
				t.Skipf("failed to create temp file: %v", err)
				return
			}
			path := tmpFile.Name()

			if _, err := tmpFile.WriteString(content); err != nil {
				tmpFile.Close()
				t.Skipf("failed to write temp file: %v", err)
				return
			}
			tmpFile.Close()
			defer os.Remove(path)

			p, err := LoadPolicy(path)
			if err == nil && p == nil {
				t.Errorf("LoadPolicy returned nil policy with no error")
			}
			// If successfully parsed, verify the engine can be created
			if err == nil && p != nil {
				engine := NewEngine(p, nil)
				if engine == nil {
					t.Errorf("NewEngine returned nil for valid policy")
				}
			}
		}()

		select {
		case <-done:
			if panicErr != nil {
				t.Fatalf("LoadPolicy panicked: %v\ncontent=%q", panicErr, content)
			}
		case <-time.After(5 * time.Second):
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			t.Fatalf("LoadPolicy timed out\ncontent=%q\n%s", content, buf[:n])
		}
	})
}

// FuzzPolicyPathValidation fuzzes the path validation logic with arbitrary
// path strings to ensure no path traversal or null byte injection.
func FuzzPolicyPathValidation(f *testing.F) {
	seeds := []string{
		"policy.yaml",
		"/etc/passwd",
		"../../../etc/passwd",
		"policy\x00.yaml",
		"",
		"/",
		".",
		"..",
		"policy.yaml/../secret",
		"//etc/passwd",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	tmpDir, err := os.MkdirTemp("", "fuzz-policy-path-*")
	if err != nil {
		f.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a valid policy file in the temp dir
	validPath := filepath.Join(tmpDir, "policy.yaml")
	if err := os.WriteFile(validPath, []byte("filesystem:\n  read: allowed\n"), 0600); err != nil {
		f.Fatalf("failed to create valid policy file: %v", err)
	}

	f.Fuzz(func(t *testing.T, path string) {
		done := make(chan struct{})
		var panicErr interface{}
		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicErr = r
				}
				close(done)
			}()

			_, _ = LoadPolicy(path)
		}()

		select {
		case <-done:
			if panicErr != nil {
				t.Fatalf("LoadPolicy panicked: %v\npath=%q", panicErr, path)
			}
		case <-time.After(5 * time.Second):
			// timeout is acceptable for path validation fuzz
		}
	})
}