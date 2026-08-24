// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​​​‌‌‌‌​‌​‌‌​‌‌‌‌​‌‌‌‌‌‌‌​​​‌‌​‌‌‌‌​‌​‌​​​‌‌​‌‌‌‌​​​​​​​​​​​​​​​​‌​​‌‌‌​​​‌​‌​‌​‌⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
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

// wmTextContent is long enough for the distributed embedding to place all
// shards (mirrors the round-trip fixture in the watermark package tests).
const wmTextContent = "Hello, this is AI-generated content about blockchain technology. " +
	"It demonstrates the capabilities of distributed watermark embedding. " +
	"The watermark should be scattered across multiple segments of the text."

func writeWatermarkFixture(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture %s: %v", path, err)
	}
	return path
}

func TestWatermarkCmd_NoArgsShowsInfo(t *testing.T) {
	var err error
	out := captureOutput(func() {
		err = HandleWatermark(nil)
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for no args, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "aflare Watermark System") {
		t.Errorf("expected watermark info output, got: %s", out)
	}
}

func TestWatermarkCmd_InfoSubcommand(t *testing.T) {
	var err error
	out := captureOutput(func() {
		err = HandleWatermark([]string{"info"})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for info, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "aflare Watermark System") {
		t.Errorf("expected watermark info output, got: %s", out)
	}
}

func TestWatermarkCmd_UnknownSubcommand(t *testing.T) {
	var err error
	out := captureOutput(func() {
		err = HandleWatermark([]string{"frobnicate"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for unknown subcommand, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Unknown watermark subcommand: frobnicate") {
		t.Errorf("expected unknown-subcommand output, got: %s", out)
	}
}

// TestWatermarkCmd_MissingOperandUsage covers the usage/exit-1 path of every
// subcommand that requires a file operand (or --all for check-source).
func TestWatermarkCmd_MissingOperandUsage(t *testing.T) {
	tests := []struct {
		args    []string
		wantUse string
	}{
		{[]string{"decode"}, "Usage: aflare watermark decode"},
		{[]string{"verify"}, "Usage: aflare watermark verify"},
		{[]string{"encode-source"}, "Usage: aflare watermark encode-source"},
		{[]string{"decode-source"}, "Usage: aflare watermark decode-source"},
		{[]string{"strip-source"}, "Usage: aflare watermark strip-source"},
		{[]string{"check-source"}, "Usage: aflare watermark check-source"},
	}
	for _, tc := range tests {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var err error
			out := captureOutput(func() {
				err = HandleWatermark(tc.args)
			})
			if code := exitCodeForErr(err); code != 1 {
				t.Errorf("expected exit code 1, got %d (err=%v)", code, err)
			}
			if !strings.Contains(out, tc.wantUse) {
				t.Errorf("expected %q in output, got: %s", tc.wantUse, out)
			}
		})
	}
}

func TestWatermarkCmd_DecodeTextWatermark(t *testing.T) {
	path := writeWatermarkFixture(t, "marked.txt", watermark.EncodeTextWithSuffix(wmTextContent))

	var err error
	out := captureOutput(func() {
		err = HandleWatermark([]string{"decode", path})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for text watermark, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Watermark found") {
		t.Errorf("expected watermark-found output, got: %s", out)
	}
	if !strings.Contains(out, "text (zero-width)") {
		t.Errorf("expected zero-width type in output, got: %s", out)
	}
}

func TestWatermarkCmd_DecodeYAMLWatermark(t *testing.T) {
	path := writeWatermarkFixture(t, "marked.yaml", watermark.EncodeYAML("workflow content"))

	var err error
	out := captureOutput(func() {
		err = HandleWatermark([]string{"decode", path})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for YAML watermark, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "YAML comment") {
		t.Errorf("expected YAML comment type in output, got: %s", out)
	}
}

// TestWatermarkCmd_DecodeNoWatermark covers the not-found path of decode:
// unlike verify it is not an error — it just reports that nothing was found.
func TestWatermarkCmd_DecodeNoWatermark(t *testing.T) {
	path := writeWatermarkFixture(t, "plain.txt", "plain text without any watermark")

	var err error
	out := captureOutput(func() {
		err = HandleWatermark([]string{"decode", path})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for unmarked file, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "No aflare watermark found") {
		t.Errorf("expected no-watermark output, got: %s", out)
	}
}

func TestWatermarkCmd_DecodeMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.txt")

	var err error
	out := captureOutput(func() {
		err = HandleWatermark([]string{"decode", missing})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for missing file, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Error reading file") {
		t.Errorf("expected read-error output, got: %s", out)
	}
}

func TestWatermarkCmd_VerifyTextWatermark(t *testing.T) {
	path := writeWatermarkFixture(t, "marked.txt", watermark.EncodeTextWithSuffix(wmTextContent))

	var err error
	out := captureOutput(func() {
		err = HandleWatermark([]string{"verify", path})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for text watermark, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "text (zero-width)") {
		t.Errorf("expected zero-width type in output, got: %s", out)
	}
	if !strings.Contains(out, "Status:    valid") {
		t.Errorf("expected valid status in output, got: %s", out)
	}
}

func TestWatermarkCmd_VerifyYAMLWatermark(t *testing.T) {
	path := writeWatermarkFixture(t, "marked.yaml", watermark.EncodeYAML("workflow content"))

	var err error
	out := captureOutput(func() {
		err = HandleWatermark([]string{"verify", path})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for YAML watermark, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "YAML comment") {
		t.Errorf("expected YAML comment type in output, got: %s", out)
	}
}

func TestWatermarkCmd_VerifyNoWatermark(t *testing.T) {
	path := writeWatermarkFixture(t, "plain.txt", "plain text without any watermark")

	var err error
	out := captureOutput(func() {
		err = HandleWatermark([]string{"verify", path})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for unmarked file, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "No aflare watermark found") {
		t.Errorf("expected no-watermark output, got: %s", out)
	}
}

func TestWatermarkCmd_VerifyMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.txt")

	var err error
	_ = captureOutput(func() {
		err = HandleWatermark([]string{"verify", missing})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("expected exit code 1 for missing file, got %d (err=%v)", code, err)
	}
}

// TestWatermarkCmd_EncodeSource covers the source-code watermark embedding:
// happy path, already-watermarked file, and unreadable file.
func TestWatermarkCmd_EncodeSource(t *testing.T) {
	path := writeWatermarkFixture(t, "demo.go", wmLicensedSrc)

	var err error
	out := captureOutput(func() {
		err = HandleWatermark([]string{"encode-source", path})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for encode-source, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Source-code watermark embedded") {
		t.Errorf("expected embed output, got: %s", out)
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("reading watermarked file: %v", readErr)
	}
	if !watermark.HasSourceWatermark(string(data)) {
		t.Error("expected source watermark in file after encode")
	}

	// Encoding again must fail: the file already carries a watermark.
	var err2 error
	out2 := captureOutput(func() {
		err2 = HandleWatermark([]string{"encode-source", path})
	})
	if code := exitCodeForErr(err2); code != 1 {
		t.Errorf("expected exit code 1 for already-watermarked file, got %d (err=%v)", code, err2)
	}
	if !strings.Contains(out2, "already has a watermark") {
		t.Errorf("expected already-watermarked output, got: %s", out2)
	}

	// Missing file.
	missing := filepath.Join(t.TempDir(), "does-not-exist.go")
	var err3 error
	out3 := captureOutput(func() {
		err3 = HandleWatermark([]string{"encode-source", missing})
	})
	if code := exitCodeForErr(err3); code != 1 {
		t.Errorf("expected exit code 1 for missing file, got %d (err=%v)", code, err3)
	}
	if !strings.Contains(out3, "Error reading file") {
		t.Errorf("expected read-error output, got: %s", out3)
	}
}

// TestWatermarkCmd_DecodeSource covers extracting the invisible source
// watermark, including the deploy-ID trace line when
// AFLARE_DEPLOYMENT_ID is set at encode time.
func TestWatermarkCmd_DecodeSource(t *testing.T) {
	t.Run("without deploy id", func(t *testing.T) {
		t.Setenv("AFLARE_DEPLOYMENT_ID", "")
		path := writeWatermarkFixture(t, "demo.go", watermark.EncodeSource(wmLicensedSrc))

		var err error
		out := captureOutput(func() {
			err = HandleWatermark([]string{"decode-source", path})
		})
		if code := exitCodeForErr(err); code != 0 {
			t.Errorf("expected exit code 0, got %d (err=%v)", code, err)
		}
		if !strings.Contains(out, "source-code") {
			t.Errorf("expected source-code type in output, got: %s", out)
		}
		if !strings.Contains(out, "set AFLARE_DEPLOYMENT_ID to enable leak tracing") {
			t.Errorf("expected unset deploy-id hint, got: %s", out)
		}
	})

	t.Run("with deploy id", func(t *testing.T) {
		t.Setenv("AFLARE_DEPLOYMENT_ID", "1a2b")
		path := writeWatermarkFixture(t, "demo.go", watermark.EncodeSource(wmLicensedSrc))

		var err error
		out := captureOutput(func() {
			err = HandleWatermark([]string{"decode-source", path})
		})
		if code := exitCodeForErr(err); code != 0 {
			t.Errorf("expected exit code 0, got %d (err=%v)", code, err)
		}
		if !strings.Contains(out, "Deploy ID: 1a2b") {
			t.Errorf("expected deploy id 1a2b in output, got: %s", out)
		}
	})

	t.Run("no watermark", func(t *testing.T) {
		path := writeWatermarkFixture(t, "plain.go", wmLicensedSrc)

		var err error
		out := captureOutput(func() {
			err = HandleWatermark([]string{"decode-source", path})
		})
		if code := exitCodeForErr(err); code != 1 {
			t.Errorf("expected exit code 1 for unmarked file, got %d (err=%v)", code, err)
		}
		if !strings.Contains(out, "No source-code watermark found") {
			t.Errorf("expected no-watermark output, got: %s", out)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "does-not-exist.go")
		var err error
		_ = captureOutput(func() {
			err = HandleWatermark([]string{"decode-source", missing})
		})
		if code := exitCodeForErr(err); code != 1 {
			t.Errorf("expected exit code 1 for missing file, got %d (err=%v)", code, err)
		}
	})
}

// TestWatermarkCmd_StripSource covers removing a source watermark: happy
// path round-trip, unmarked file, and missing file.
func TestWatermarkCmd_StripSource(t *testing.T) {
	path := writeWatermarkFixture(t, "demo.go", watermark.EncodeSource(wmLicensedSrc))

	var err error
	out := captureOutput(func() {
		err = HandleWatermark([]string{"strip-source", path})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for strip-source, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "watermark removed") {
		t.Errorf("expected strip output, got: %s", out)
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("reading stripped file: %v", readErr)
	}
	if watermark.HasSourceWatermark(string(data)) {
		t.Error("expected source watermark removed after strip")
	}

	// Stripping an unmarked file fails.
	plain := writeWatermarkFixture(t, "plain.go", wmLicensedSrc)
	var err2 error
	out2 := captureOutput(func() {
		err2 = HandleWatermark([]string{"strip-source", plain})
	})
	if code := exitCodeForErr(err2); code != 1 {
		t.Errorf("expected exit code 1 for unmarked file, got %d (err=%v)", code, err2)
	}
	if !strings.Contains(out2, "No source-code watermark found") {
		t.Errorf("expected no-watermark output, got: %s", out2)
	}

	// Missing file.
	missing := filepath.Join(t.TempDir(), "does-not-exist.go")
	var err3 error
	_ = captureOutput(func() {
		err3 = HandleWatermark([]string{"strip-source", missing})
	})
	if code := exitCodeForErr(err3); code != 1 {
		t.Errorf("expected exit code 1 for missing file, got %d (err=%v)", code, err3)
	}
}

// TestWatermarkCmd_CheckSourceAll covers the CI-style coverage check against
// a temp tree: a fully watermarked tree passes, an unmarked file fails with
// the offending path listed.
func TestWatermarkCmd_CheckSourceAll(t *testing.T) {
	dir := t.TempDir()
	marked := filepath.Join(dir, "marked.go")
	if err := os.WriteFile(marked, []byte(watermark.EncodeSource(wmLicensedSrc)), 0o644); err != nil {
		t.Fatalf("writing marked file: %v", err)
	}

	var err error
	out := captureOutput(func() {
		err = handleWatermarkCheckSourceAll(dir)
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for fully watermarked tree, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "All Go files carry a source watermark") {
		t.Errorf("expected success output, got: %s", out)
	}

	unmarked := filepath.Join(dir, "unmarked.go")
	if err := os.WriteFile(unmarked, []byte(wmLicensedSrc), 0o644); err != nil {
		t.Fatalf("writing unmarked file: %v", err)
	}

	var err2 error
	out2 := captureOutput(func() {
		err2 = handleWatermarkCheckSourceAll(dir)
	})
	if code := exitCodeForErr(err2); code != 1 {
		t.Errorf("expected exit code 1 for tree with unmarked file, got %d (err=%v)", code, err2)
	}
	if !strings.Contains(out2, "missing a source watermark") {
		t.Errorf("expected missing-watermark output, got: %s", out2)
	}
	if !strings.Contains(out2, "unmarked.go") {
		t.Errorf("expected offending path in output, got: %s", out2)
	}
}

// TestWatermarkCmd_EncodeSourceAllRoot covers the batch embedding handler
// against a temp tree, including its idempotent second run.
func TestWatermarkCmd_EncodeSourceAllRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(wmLicensedSrc), 0o644); err != nil {
		t.Fatalf("writing source file: %v", err)
	}

	var err error
	out := captureOutput(func() {
		err = handleWatermarkEncodeSourceAll(dir)
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Errorf("expected exit code 0 for batch encode, got %d (err=%v)", code, err)
	}
	if !strings.Contains(out, "Source watermarks embedded in 1 files") {
		t.Errorf("expected batch embed output, got: %s", out)
	}

	// Second run is a no-op: the file is already watermarked.
	var err2 error
	out2 := captureOutput(func() {
		err2 = handleWatermarkEncodeSourceAll(dir)
	})
	if code := exitCodeForErr(err2); code != 0 {
		t.Errorf("expected exit code 0 for idempotent re-run, got %d (err=%v)", code, err2)
	}
	if !strings.Contains(out2, "embedded in 0 files") || !strings.Contains(out2, "1 already watermarked") {
		t.Errorf("expected idempotent re-run output, got: %s", out2)
	}
}
