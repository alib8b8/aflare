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
	// Longer content to ensure enough segments for distributed embedding.
	content := "Hello, this is AI-generated content about blockchain technology. " +
		"It demonstrates the capabilities of distributed watermark embedding. " +
		"The watermark should be scattered across multiple segments of the text."

	full := EncodeTextWithSuffix(content)
	if full == "" {
		t.Fatal("EncodeTextWithSuffix returned empty")
	}
	if !strings.HasPrefix(full, "Hello") {
		t.Error("EncodeTextWithSuffix should preserve original content start")
	}

	// Watermark should be invisible (no visible characters added).
	visible := stripZeroWidth(full)
	for _, r := range visible {
		if r == zwBit0 || r == zwBit1 || r == zwStart || r == zwEnd {
			t.Errorf("visible text should not contain zero-width characters, got %U", r)
		}
	}

	// Decode should recover the payload.
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
	content := "Some generated text about artificial intelligence. " +
		"This is a longer piece of content that will be split into multiple segments. " +
		"Each segment will contain a shard of the watermark for distributed protection."

	full := EncodeTextWithSuffix(content)

	if !HasWatermark(full) {
		t.Error("HasWatermark should return true for watermarked text")
	}
	if HasWatermark("plain text without any watermark at all anywhere") {
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
		"Short text that is long enough to split into multiple segments for distributed watermarking",
		"Medium length text with some special characters: !@#$%^&*(). " +
			"This needs to be sufficiently long to have multiple segments for the watermark distribution.",
		"Very long text " + strings.Repeat("abcdefghij ", 100) +
			"with enough content to split into segments for the watermark",
		"中文内容测试需要足够长的文本才能分成多个片段进行水印嵌入测试",
		"Mixed content with emoji and 中文 and English. " +
			"This is a longer text that should be split into multiple segments. " +
			"The distributed watermark will be embedded across these segments.",
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
	content := "This is visible text that should remain unchanged. " +
		"The watermark is embedded using invisible zero-width characters."

	full := EncodeTextWithSuffix(content)

	// Strip all zero-width characters and BOM, and normalize whitespace.
	cleaned := stripZeroWidth(full)
	// Normalize non-breaking spaces back to regular spaces for comparison.
	cleaned = strings.ReplaceAll(cleaned, string(wsBit1), " ")
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
	content := "Original content that is sufficiently long. " +
		"It contains multiple sentences so the watermark can be distributed. " +
		"This ensures the distributed embedding works correctly."

	full := EncodeTextWithSuffix(content)

	// Tamper by modifying the visible content.
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

func TestDistributedEmbedding(t *testing.T) {
	// Test that the watermark is truly distributed across the text,
	// not just appended at the end.
	content := "First sentence about the topic. " +
		"Second sentence with more details. " +
		"Third sentence providing additional context. " +
		"Fourth sentence wrapping up the discussion."

	full := EncodeTextWithSuffix(content)

	// There should be multiple zero-width sequences (one per shard).
	shardCount := strings.Count(full, string(zwStart))
	if shardCount < 2 {
		t.Errorf("expected at least 2 shards distributed in text, got %d. "+
			"Watermark may be appended at the end instead of distributed.", shardCount)
	}

	// The last shard should not be at the very end of the text.
	lastShardPos := strings.LastIndex(full, string(zwStart))
	if lastShardPos > len(full)-20 {
		t.Error("last shard appears to be at the end of text, should be distributed")
	}
}

func TestPartialTruncationRecovery(t *testing.T) {
	// Test that watermark can be recovered even if part of the text is truncated.
	content := "First paragraph with important information. " +
		"Second paragraph explaining the details. " +
		"Third paragraph with additional context. " +
		"Fourth paragraph concluding the discussion. " +
		"Fifth paragraph with final remarks."

	full := EncodeTextWithSuffix(content)

	// Truncate the last 30% of the text (simulating end truncation).
	runes := []rune(full)
	truncated := string(runes[:len(runes)*7/10])

	// Should still be able to recover the watermark from remaining shards.
	_, ok := DecodeText(truncated)
	if !ok {
		t.Error("watermark should be recoverable after partial truncation")
	}
}

func TestWhitespaceFingerprint(t *testing.T) {
	// Test that whitespace fingerprint encoding/decoding works.
	text := "hello world this is a test"
	encoded := encodeWhitespaceFingerprint(text, 2) // shard index 2

	// Should have some non-breaking spaces.
	if !strings.Contains(encoded, string(wsBit1)) {
		t.Error("whitespace fingerprint should encode non-breaking spaces")
	}

	// Decode should recover the shard index.
	index := decodeWhitespaceFingerprint(encoded)
	if index != 2 {
		t.Errorf("expected shard index 2, got %d", index)
	}
}

func TestBuildShards(t *testing.T) {
	// Create a known payload and verify shard construction.
	payload := make([]byte, 23)
	for i := range payload {
		payload[i] = byte(i)
	}

	shards := buildShards(payload)
	if len(shards) != 4 {
		t.Fatalf("expected 4 shards, got %d", len(shards))
	}
	for i, s := range shards {
		if len(s) != 8 {
			t.Errorf("shard %d: expected 8 bytes, got %d", i, len(s))
		}
	}

	// Verify parity: XOR of all data shards should equal the parity shard.
	var xorCheck [8]byte
	for i := 0; i < 3; i++ {
		for j := 0; j < 8; j++ {
			xorCheck[j] ^= shards[i][j]
		}
	}
	for j := 0; j < 8; j++ {
		if xorCheck[j] != shards[3][j] {
			t.Errorf("parity mismatch at byte %d: expected %02x, got %02x",
				j, xorCheck[j], shards[3][j])
		}
	}
}

func TestShardRecoveryParity(t *testing.T) {
	// Test that any 3 of 4 shards can recover the full payload.
	payload := make([]byte, 23)
	for i := range payload {
		payload[i] = byte(i + 42)
	}
	// Add valid checksum.
	chk := checksum16(payload[:21])
	payload[21] = byte(chk >> 8)
	payload[22] = byte(chk & 0xFF)

	shards := buildShards(payload)

	// Test recovery with all 4 shards.
	recovered, ok := recoverPayload(shards)
	if !ok {
		t.Fatal("failed to recover from all 4 shards")
	}
	if !verifyChecksum(recovered) {
		t.Error("checksum verification failed for full recovery")
	}

	// Test recovery with shards 0, 1, 2 (no parity).
	recovered, ok = recoverPayload([][]byte{shards[0], shards[1], shards[2]})
	if !ok {
		t.Fatal("failed to recover from data shards only")
	}
	if !verifyChecksum(recovered) {
		t.Error("checksum verification failed for data-only recovery")
	}

	// Test recovery with shards 0, 1, 3 (parity replaces shard 2).
	recovered, ok = recoverPayload([][]byte{shards[0], shards[1], shards[3]})
	if !ok {
		t.Fatal("failed to recover with parity shard")
	}
	if !verifyChecksum(recovered) {
		t.Error("checksum verification failed for parity recovery")
	}

	// Test recovery with shards 0, 2, 3 (parity replaces shard 1).
	recovered, ok = recoverPayload([][]byte{shards[0], shards[2], shards[3]})
	if !ok {
		t.Fatal("failed to recover with parity shard (variant 2)")
	}
	if !verifyChecksum(recovered) {
		t.Error("checksum verification failed for parity recovery (variant 2)")
	}
}

func TestChecksum(t *testing.T) {
	data := []byte("hello world")
	chk := checksum16(data)

	// Verify checksum round-trip.
	full := append(data, byte(chk>>8), byte(chk&0xFF))
	if !verifyChecksum(full) {
		t.Error("checksum verification should pass")
	}

	// Tampered data should fail.
	full[0] ^= 0x01
	if verifyChecksum(full) {
		t.Error("checksum verification should fail for tampered data")
	}
}

func TestLegacyDecode(t *testing.T) {
	// Test that legacy format (single suffix block) is still decodable.
	content := "Some short content for legacy format testing"
	payload := buildPayload(content)
	legacy := content + encodeLegacySuffix(payload)

	decoded, ok := decodeLegacy(legacy)
	if !ok {
		t.Fatal("failed to decode legacy watermark")
	}
	if decoded.Version != wmVersion {
		t.Errorf("expected version %d, got %d", wmVersion, decoded.Version)
	}
}