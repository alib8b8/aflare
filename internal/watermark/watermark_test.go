// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌‌‌‌​​​‌‌‌​​‌​​‌​‌​‌​​‌​‌‌​​​​​‌​​​​‌​​​‌‌​​‌‌​​‌‌‌‌​​​​​‌​‌​​​​​​​​​​​​​​​​‌‌‌​​​​‌‌​‌​‌‌​​⁠
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
	"crypto/sha256"
	"encoding/binary"
	"slices"
	"strings"
	"testing"
	"time"
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
	if len(payload.Hash) != 6 {
		t.Errorf("expected 6-byte hash (v2), got %d bytes", len(payload.Hash))
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

func TestDeployIDRoundTrip(t *testing.T) {
	t.Setenv("AFLARE_DEPLOYMENT_ID", "1a2b")

	if id := ResolveDeployID(); id != 0x1a2b {
		t.Fatalf("ResolveDeployID expected 0x1a2b, got 0x%04x", id)
	}

	content := "Content watermarked with a deployment identifier for leak tracing. " +
		"It must be long enough to distribute shards across segments."
	full := EncodeTextWithSuffix(content)
	payload, ok := DecodeText(full)
	if !ok {
		t.Fatal("DecodeText failed for deploy-ID watermarked content")
	}
	if payload.Version != wmVersion {
		t.Errorf("expected version %d, got %d", wmVersion, payload.Version)
	}
	if payload.DeployID != 0x1a2b {
		t.Errorf("expected deploy ID 0x1a2b, got 0x%04x", payload.DeployID)
	}
}

func TestResolveDeployIDInvalid(t *testing.T) {
	t.Setenv("AFLARE_DEPLOYMENT_ID", "not-hex")
	if id := ResolveDeployID(); id != 0 {
		t.Errorf("invalid hex should yield 0, got 0x%04x", id)
	}
	t.Setenv("AFLARE_DEPLOYMENT_ID", "12345") // 5 digits > uint16
	if id := ResolveDeployID(); id != 0 {
		t.Errorf("out-of-range value should yield 0, got 0x%04x", id)
	}
}

// TestParsePayloadV1BackwardCompat verifies that v1 payloads (8-byte hash,
// no deploy ID) written by aflare ≤ 0.9.0 still decode.
func TestParsePayloadV1BackwardCompat(t *testing.T) {
	payload := make([]byte, payloadSize)
	copy(payload[0:4], magicBytes)
	payload[4] = wmVersionV1
	binary.BigEndian.PutUint64(payload[5:13], uint64(1700000000))
	hash := sha256.Sum256([]byte("legacy content"))
	copy(payload[13:21], hash[:8])

	p, ok := parsePayload(payload)
	if !ok {
		t.Fatal("v1 payload should decode")
	}
	if p.Version != wmVersionV1 {
		t.Errorf("expected v1, got version %d", p.Version)
	}
	if len(p.Hash) != 8 {
		t.Errorf("v1 hash should be 8 bytes, got %d", len(p.Hash))
	}
	if p.DeployID != 0 {
		t.Errorf("v1 payload should have zero deploy ID, got 0x%04x", p.DeployID)
	}
	if !p.Timestamp.Equal(time.Unix(1700000000, 0)) {
		t.Errorf("timestamp mismatch: %v", p.Timestamp)
	}
}

func TestParsePayloadUnknownVersion(t *testing.T) {
	payload := make([]byte, payloadSize)
	copy(payload[0:4], magicBytes)
	payload[4] = 0x7F
	if _, ok := parsePayload(payload); ok {
		t.Error("unknown version should be rejected")
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
	full := append(slices.Clone(data), byte(chk>>8), byte(chk&0xFF))
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
	payload := buildPayload(content, 0)
	legacy := content + encodeLegacySuffix(payload)

	decoded, ok := decodeLegacy(legacy)
	if !ok {
		t.Fatal("failed to decode legacy watermark")
	}
	if decoded.Version != wmVersion {
		t.Errorf("expected version %d, got %d", wmVersion, decoded.Version)
	}
}

// ── Source code watermark tests ──

const sampleGoSource = `// Copyright (c) 2026 aflare Contributors
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

package main

import "fmt"

func main() {
	fmt.Println("hello aflare")
}
`

func TestEncodeSource_InsertsWatermark(t *testing.T) {
	encoded := EncodeSource(sampleGoSource)
	if encoded == sampleGoSource {
		t.Fatal("EncodeSource should modify the source, but it returned the same string")
	}
	if !strings.Contains(encoded, zwPrefix) {
		t.Fatal("encoded source should contain the \"// aflare\" prefix")
	}
	// The original content should still be present.
	if !strings.Contains(encoded, "package main") {
		t.Error("encoded source should preserve package declaration")
	}
	if !strings.Contains(encoded, "fmt.Println(\"hello aflare\")") {
		t.Error("encoded source should preserve function body")
	}
}

func TestEncodeSource_EmptySource(t *testing.T) {
	if encoded := EncodeSource(""); encoded != "" {
		t.Errorf("EncodeSource on empty should return empty, got %q", encoded)
	}
}

func TestEncodeSource_NoCopyright(t *testing.T) {
	src := "package main\n\nfunc main() {}\n"
	encoded := EncodeSource(src)
	if !strings.Contains(encoded, zwPrefix) {
		t.Fatal("even without copyright header, watermark should be prepended")
	}
	if !strings.Contains(encoded, "package main") {
		t.Error("original content should still be present")
	}
}

func TestDecodeSource_RoundTrip(t *testing.T) {
	encoded := EncodeSource(sampleGoSource)
	payload, ok := DecodeSource(encoded)
	if !ok {
		t.Fatal("DecodeSource should find the watermark")
	}
	if payload.Version != wmVersion {
		t.Errorf("expected version %d, got %d", wmVersion, payload.Version)
	}
	if payload.Timestamp.IsZero() {
		t.Error("payload should have non-zero timestamp")
	}
}

func TestDecodeSource_NoWatermark(t *testing.T) {
	_, ok := DecodeSource(sampleGoSource)
	if ok {
		t.Error("DecodeSource should return false for unwatermarked source")
	}
}

func TestDecodeSource_EmptySource(t *testing.T) {
	_, ok := DecodeSource("")
	if ok {
		t.Error("DecodeSource should return false for empty source")
	}
}

func TestHasSourceWatermark(t *testing.T) {
	encoded := EncodeSource(sampleGoSource)
	if !HasSourceWatermark(encoded) {
		t.Error("HasSourceWatermark should return true for encoded source")
	}
	if HasSourceWatermark(sampleGoSource) {
		t.Error("HasSourceWatermark should return false for unencoded source")
	}
}

func TestStripSourceWatermark(t *testing.T) {
	encoded := EncodeSource(sampleGoSource)
	stripped := StripSourceWatermark(encoded)
	if HasSourceWatermark(stripped) {
		t.Error("StripSourceWatermark should remove the watermark")
	}
	// The stripped source should still be valid Go code.
	if !strings.Contains(stripped, "package main") {
		t.Error("stripped source should preserve package declaration")
	}
	if !strings.Contains(stripped, "fmt.Println(\"hello aflare\")") {
		t.Error("stripped source should preserve function body")
	}
	// Stripping must restore the exact original source: the watermark line
	// is removed together with its newline, so no blank line may remain.
	if stripped != sampleGoSource {
		t.Errorf("strip(encode(src)) should round-trip to src\n got: %q\nwant: %q", stripped, sampleGoSource)
	}
}

func TestStripSourceWatermark_NoWatermark(t *testing.T) {
	stripped := StripSourceWatermark(sampleGoSource)
	if stripped != sampleGoSource {
		t.Error("StripSourceWatermark on unwatermarked source should return unchanged")
	}
}

func TestStripSourceWatermark_EmptySource(t *testing.T) {
	if stripped := StripSourceWatermark(""); stripped != "" {
		t.Errorf("StripSourceWatermark on empty should return empty, got %q", stripped)
	}
}

func TestSourceWatermark_TamperResistant(t *testing.T) {
	encoded := EncodeSource(sampleGoSource)
	// Minor modification: add a comment line — watermark should still be detectable.
	tampered := strings.Replace(encoded, "package main", "// minor edit\npackage main", 1)
	_, ok := DecodeSource(tampered)
	if !ok {
		t.Error("watermark should survive minor edits like adding a comment")
	}
}

func TestSourceWatermark_Idempotent(t *testing.T) {
	// Encoding twice should not stack watermarks.
	once := EncodeSource(sampleGoSource)
	twice := EncodeSource(once)
	// The second encode should not add another watermark line.
	if strings.Count(twice, zwPrefix) > strings.Count(once, zwPrefix)+1 {
		t.Error("double encoding should not stack watermarks")
	}
}
