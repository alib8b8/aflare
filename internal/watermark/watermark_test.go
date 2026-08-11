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

package watermark

import (
	"strings"
	"testing"
)

func TestEncodeDecodeText(t *testing.T) {
	content := "Hello, this is AI-generated content about blockchain technology."
	wm := EncodeText(content)
	if wm == "" {
		t.Fatal("EncodeText returned empty")
	}

	// Watermark should be invisible (no visible characters)
	visible := stripZeroWidth(wm)
	if visible != "" {
		t.Errorf("expected invisible watermark, got visible chars: %q", visible)
	}

	// Full text with watermark
	full := EncodeTextWithSuffix(content)
	if !strings.HasPrefix(full, content) {
		t.Error("EncodeTextWithSuffix should preserve original content")
	}

	// Decode should recover the payload
	payload, ok := DecodeText(full)
	if !ok {
		t.Fatal("DecodeText failed to find watermark")
	}
	if payload.Version != wmVersion {
		t.Errorf("expected version %d, got %d", wmVersion, payload.Version)
	}
	if payload.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
	if len(payload.Hash) != 8 {
		t.Errorf("expected 8-byte hash, got %d bytes", len(payload.Hash))
	}
}

func TestHasWatermark(t *testing.T) {
	content := "Some generated text"
	full := EncodeTextWithSuffix(content)

	if !HasWatermark(full) {
		t.Error("HasWatermark should return true for watermarked text")
	}
	if HasWatermark("plain text without watermark") {
		t.Error("HasWatermark should return false for plain text")
	}
}

func TestDecodeTextNoWatermark(t *testing.T) {
	_, ok := DecodeText("plain text without any zero-width characters")
	if ok {
		t.Error("DecodeText should return false for plain text")
	}
}

func TestEmptyContent(t *testing.T) {
	if EncodeText("") != "" {
		t.Error("EncodeText should return empty for empty content")
	}
	if EncodeTextWithSuffix("") != "" {
		t.Error("EncodeTextWithSuffix should return empty for empty content")
	}
	if EncodeYAML("") != "" {
		t.Error("EncodeYAML should return empty for empty content")
	}
}

func TestYAMLWatermark(t *testing.T) {
	content := "name: test-workflow\nsteps:\n  - node: fetch_url\n"
	wm := EncodeYAML(content)
	if wm == "" {
		t.Fatal("EncodeYAML returned empty")
	}
	if !strings.HasPrefix(wm, "# aflare-watermark: ") {
		t.Errorf("expected YAML watermark prefix, got: %s", wm)
	}

	payload, ok := DecodeYAML(wm)
	if !ok {
		t.Fatal("DecodeYAML failed")
	}
	if payload.Version != wmVersion {
		t.Errorf("expected version %d, got %d", wmVersion, payload.Version)
	}
}

func TestDecodeYAMLInvalid(t *testing.T) {
	_, ok := DecodeYAML("steps:\n  - node: fetch_url\n")
	if ok {
		t.Error("DecodeYAML should return false for non-watermark line")
	}
	_, ok = DecodeYAML("# aflare-watermark: invalid-base64!!!")
	if ok {
		t.Error("DecodeYAML should return false for invalid base64")
	}
}

func TestRoundTripMultipleContent(t *testing.T) {
	contents := []string{
		"Short text",
		"Medium length text with some special characters: !@#$%^&*()",
		"Very long text " + strings.Repeat("abcdefghij", 100),
		"中文内容测试",
		"Mixed content with emoji ✨ and 中文 and English",
	}

	for _, content := range contents {
		full := EncodeTextWithSuffix(content)
		payload, ok := DecodeText(full)
		if !ok {
			t.Errorf("failed to decode watermark for content: %q", content)
			continue
		}
		if payload.Version != wmVersion {
			t.Errorf("version mismatch for content: %q", content)
		}
	}
}

func TestWatermarkDoesNotChangeVisibleContent(t *testing.T) {
	content := "This is visible text."
	full := EncodeTextWithSuffix(content)

	// Strip all zero-width characters and BOM
	cleaned := stripZeroWidth(full)
	if cleaned != content {
		t.Errorf("visible content changed:\n  original: %q\n  cleaned:  %q", content, cleaned)
	}
}

// stripZeroWidth removes all zero-width and BOM characters from text.
func stripZeroWidth(s string) string {
	var result strings.Builder
	for _, r := range s {
		switch r {
		case zwStart, zwEnd, zwBit0, zwBit1:
			// skip zero-width characters
		default:
			result.WriteRune(r)
		}
	}
	return result.String()
}

func TestTamperedWatermark(t *testing.T) {
	content := "Original content"
	full := EncodeTextWithSuffix(content)

	// Tamper by modifying the visible content
	tampered := strings.Replace(full, "Original", "Modified", 1)

	payload, ok := DecodeText(tampered)
	if !ok {
		t.Fatal("watermark should still be extractable even if content is tampered")
	}

	// The hash won't match the modified content, but that's expected
	// — the watermark proves the content was generated by aflare,
	// and the hash mismatch proves tampering.
	if payload.Version != wmVersion {
		t.Error("version should still be correct")
	}
}

func TestInfo(t *testing.T) {
	info := Info()
	if info == "" {
		t.Error("Info should not be empty")
	}
	if !strings.Contains(info, "aflare") {
		t.Error("Info should mention aflare")
	}
}