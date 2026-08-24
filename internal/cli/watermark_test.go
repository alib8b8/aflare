// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌‌‌‌‌‌‌​‌‌‌​​​​‌‌‌‌‌‌‌‌‌‌​‌‌‌‌‌‌‌‌​‌‌‌​‌​‌​​‌​​​​​​​​​​​​​​​​​‌​‌‌‌​‌‌​​​‌​​‌​⁠

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

	"github.com/alib8b8/aflare/internal/watermark"
)

const wmLicensedSrc = `// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify.

package demo

func Hello() string { return "hi" }
`

// TestEncodeSourceAll watermarks a fake tree and asserts that licensed Go
// files get a watermark, already-marked files are skipped, non-Go files are
// untouched, and excluded directories (vendor, killer-demos) are left alone.
func TestEncodeSourceAll(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("main.go", wmLicensedSrc)
	write("internal/agent/agent.go", wmLicensedSrc)
	write("internal/agent/bench_test.go", "package agent\n") // no copyright header
	write("docs/readme.md", "# doc")
	write("vendor/lib/lib.go", wmLicensedSrc)
	write("examples/killer-demos/demo.go", wmLicensedSrc)
	write("already.go", watermark.EncodeSource(wmLicensedSrc))

	res, err := encodeSourceAll(dir)
	if err != nil {
		t.Fatalf("encodeSourceAll: %v", err)
	}
	if res.Watermarked != 3 {
		t.Errorf("Watermarked = %d, want 3 (main.go, agent.go, bench_test.go)", res.Watermarked)
	}
	if res.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1 (already.go)", res.Skipped)
	}

	for _, rel := range []string{"main.go", "internal/agent/agent.go", "internal/agent/bench_test.go", "already.go"} {
		data, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Fatal(err)
		}
		if !watermark.HasSourceWatermark(string(data)) {
			t.Errorf("%s: no watermark after batch run", rel)
		}
	}

	for _, rel := range []string{"vendor/lib/lib.go", "examples/killer-demos/demo.go"} {
		data, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Fatal(err)
		}
		if watermark.HasSourceWatermark(string(data)) {
			t.Errorf("%s: watermarked despite directory exclusion", rel)
		}
	}

	data, err := os.ReadFile(filepath.Join(dir, "docs/readme.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != len("# doc") {
		t.Errorf("non-Go file was modified: %q", string(data))
	}
}

// TestEncodeSourceAllIdempotent re-runs the batch operation and expects every
// file to be reported as already watermarked with no rewrites.
func TestEncodeSourceAllIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(wmLicensedSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := encodeSourceAll(dir); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	res, err := encodeSourceAll(dir)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if res.Watermarked != 0 || res.Skipped != 1 {
		t.Errorf("second run = %+v, want {Watermarked:0 Skipped:1}", res)
	}

	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Error("second run rewrote an already-watermarked file")
	}
}

// TestCheckSourceAll verifies the read-only coverage check: watermarked trees
// pass, a missing watermark is reported by path, and excluded directories are
// ignored exactly like in encodeSourceAll.
func TestCheckSourceAll(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("marked.go", watermark.EncodeSource(wmLicensedSrc))
	write("vendor/lib.go", wmLicensedSrc) // excluded dir — must be ignored

	missing, err := checkSourceAll(dir)
	if err != nil {
		t.Fatalf("checkSourceAll: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty for fully watermarked tree", missing)
	}

	write("unmarked.go", wmLicensedSrc)
	missing, err = checkSourceAll(dir)
	if err != nil {
		t.Fatalf("checkSourceAll after adding file: %v", err)
	}
	if len(missing) != 1 || !strings.HasSuffix(missing[0], "unmarked.go") {
		t.Errorf("missing = %v, want exactly [unmarked.go]", missing)
	}

	// The check must never modify files.
	data, err := os.ReadFile(filepath.Join(dir, "unmarked.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != wmLicensedSrc {
		t.Error("checkSourceAll modified a file — it must be read-only")
	}
}
