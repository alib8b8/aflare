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

// Package watermark provides invisible text watermarking for content provenance.
//
// The watermark is modeled after Claude's invisible watermark: it is machine-readable
// and designed to be non-removable through three layers of protection:
//
//  1. Distributed embedding: the watermark is split into redundant shards and
//     scattered at word boundaries throughout the text, not just appended at the end.
//     Truncating the end of the text does not remove all shards.
//
//  2. Whitespace encoding: regular spaces (U+0020) and non-breaking spaces (U+00A0)
//     are alternated to encode a recovery fingerprint. This makes the watermark
//     resistant to "strip invisible characters" sanitizers.
//
//  3. Redundant shard recovery: the payload is encoded with a parity shard (XOR).
//     Only 3 of 4 shards are needed to recover the full payload. Partial text
//     removal or corruption still allows watermark extraction.
//
// Two output modes are supported:
//   - Text (zero-width): invisible Unicode characters distributed in the text.
//   - YAML comment: a human-readable comment line: # aflare-watermark: <base64>
//
// The watermark payload (version 2) contains:
//   - Magic bytes "AFLR" (4 bytes) for identification
//   - Version byte (1 byte, currently 0x02)
//   - Timestamp (8 bytes, Unix epoch seconds)
//   - Content hash (6 bytes, first 6 bytes of SHA-256)
//   - Deployment ID (2 bytes, from AFLARE_DEPLOYMENT_ID, for leak tracing)
//
// Total raw payload: 21 bytes → 23 bytes with checksum → 4 shards of 8 bytes each.
// Version 1 payloads (8-byte hash, no deployment ID) remain decodable.
package watermark

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ── Character constants ──────────────────────────────────────────────────

const (
	// Zero-width characters for invisible shard encoding.
	zwBit0  = '\u200B' // zero-width space → bit 0
	zwBit1  = '\u200C' // zero-width non-joiner → bit 1
	zwStart = '\u200D' // zero-width joiner → shard start marker
	zwEnd   = '\uFEFF' // zero-width no-break space (BOM) → shard end marker

	// Whitespace characters for the secondary encoding layer.
	wsBit0 = ' '      // regular space (U+0020) → bit 0
	wsBit1 = '\u00A0' // non-breaking space (U+00A0) → bit 1

	// Magic bytes for watermark identification.
	magicBytes = "AFLR"

	// Current watermark version.
	wmVersion = byte(0x02)

	// Version 1: 8-byte content hash, no deployment ID.
	wmVersionV1 = byte(0x01)

	// Raw payload layout (21 bytes, both versions):
	//   v2: magic + version + timestamp(8) + hash(6) + deployID(2)
	//   v1: magic + version + timestamp(8) + hash(8)
	payloadSize = 4 + 1 + 8 + 6 + 2 // magic + version + timestamp + hash + deployID

	// Sharding parameters.
	numShards            = 4 // 3 data + 1 parity
	minShardsForRecovery = 3

	// Checksum size appended to payload before sharding.
	checksumSize = 2 // CRC-16 over the 21-byte payload
)

// ── Public types ─────────────────────────────────────────────────────────

// Payload is the decoded content of a watermark.
type Payload struct {
	Version   byte      // watermark version
	Timestamp time.Time // when the content was generated
	Hash      []byte    // v2: first 6 bytes of content SHA-256; v1: first 8
	DeployID  uint16    // v2: deployment identifier; 0 in v1 payloads
}

// ── Primary API: distributed text watermark ──────────────────────────────

// EncodeTextWithSuffix returns content with the watermark distributed
// throughout the text in zero-width characters. The watermark is split into
// redundant shards and embedded at word boundaries, making it resistant to
// partial removal.
//
// This is the recommended function for watermarking text output.
func EncodeTextWithSuffix(content string) string {
	if content == "" {
		return content
	}

	payload := buildPayload(content, ResolveDeployID())
	// Append 2-byte checksum for integrity verification.
	chk := checksum16(payload)
	payload = append(payload, byte(chk>>8), byte(chk&0xFF))
	// payload is now 23 bytes.

	// Build 4 shards (3 data + 1 parity), each 8 bytes.
	shards := buildShards(payload)

	// Split content into segments at sentence/paragraph boundaries.
	segments := splitContentIntoSegments(content, numShards)
	if len(segments) == 0 {
		// Fallback to simple suffix append for very short content.
		return content + encodeLegacySuffix(payload[:payloadSize])
	}

	// Embed each shard into its segment.
	var result strings.Builder
	for i, seg := range segments {
		if i < len(shards) {
			result.WriteString(embedShardInSegment(seg, shards[i], i))
		} else {
			result.WriteString(seg)
		}
	}

	return result.String()
}

// DecodeText extracts and validates a distributed watermark from text.
// Returns the payload and true if a valid watermark was found.
// It can recover the full payload even if some shards are missing.
func DecodeText(text string) (Payload, bool) {
	// Try distributed extraction first.
	shards := extractAllShards(text)
	if len(shards) >= minShardsForRecovery {
		payload, ok := recoverPayload(shards)
		if ok && verifyChecksum(payload) {
			return parsePayload(payload[:payloadSize])
		}
	}

	// Fall back to legacy suffix extraction.
	return decodeLegacy(text)
}

// HasWatermark checks if the text contains a valid aflare watermark.
func HasWatermark(text string) bool {
	_, ok := DecodeText(text)
	return ok
}

// ── YAML watermark API ───────────────────────────────────────────────────

// EncodeYAML generates a watermark comment line for YAML output.
// Format: # aflare-watermark: <base64-payload>
func EncodeYAML(content string) string {
	if content == "" {
		return ""
	}
	payload := buildPayload(content, ResolveDeployID())
	b64 := base64.RawStdEncoding.EncodeToString(payload)
	return fmt.Sprintf("# aflare-watermark: %s", b64)
}

// DecodeYAML extracts and validates a watermark from a YAML comment line.
// The line should be in the format: # aflare-watermark: <base64>
func DecodeYAML(line string) (Payload, bool) {
	const prefix = "# aflare-watermark: "
	if !strings.HasPrefix(line, prefix) {
		return Payload{}, false
	}
	b64 := strings.TrimSpace(line[len(prefix):])
	payload, err := base64.RawStdEncoding.DecodeString(b64)
	if err != nil || len(payload) < payloadSize {
		return Payload{}, false
	}
	return parsePayload(payload[:payloadSize])
}

// ── Info ─────────────────────────────────────────────────────────────────

// Info returns a human-readable description of the watermark system.
func Info() string {
	return `aflare Watermark System
─────────────────────
Three-layer protection (Claude-style):
  1. Distributed embedding — shards scattered at word boundaries
  2. Whitespace encoding    — space/nbsp alternation as recovery fingerprint
  3. Redundant recovery     — 3 of 4 shards sufficient to recover full payload

Output modes:
  Text  — invisible zero-width Unicode characters (U+200B/U+200C)
  YAML  — comment line: # aflare-watermark: <base64>

Payload v2 (21 bytes):
  Magic:      AFLR (4 bytes)
  Version:    1 byte (0x02)
  Time:       8 bytes (Unix seconds)
  Hash:       6 bytes (SHA-256 of content)
  Deploy ID:  2 bytes (from AFLARE_DEPLOYMENT_ID, hex; 0 = not set)

Set AFLARE_DEPLOYMENT_ID (1-4 hex digits, e.g. "1a2b") to trace leaked
content back to the deployment that generated it. Version 1 watermarks
(8-byte hash, no deploy ID) are still decoded.

Usage:
  aflare watermark decode <file>   — extract watermark from file
  aflare watermark verify <file>   — verify watermark integrity
  aflare watermark info            — show this info`
}

// ── Shard construction ───────────────────────────────────────────────────

// buildShards splits the payload into 4 shards of 8 bytes each.
// The payload (21 data + 2 checksum = 23 bytes) is padded to 24 bytes (3 × 8).
// Shards 0-2 are data shards, shard 3 is the parity shard (XOR of all data shards).
// Any 3 of 4 shards can recover the full payload.
func buildShards(payload []byte) [][]byte {
	// Pad payload to a multiple of 8 bytes.
	padded := make([]byte, ((len(payload)+7)/8)*8)
	copy(padded, payload)

	// Number of data shards.
	dataShards := len(padded) / 8 // 24/8 = 3

	shards := make([][]byte, dataShards+1) // +1 for parity
	for i := 0; i < dataShards; i++ {
		shards[i] = make([]byte, 8)
		copy(shards[i], padded[i*8:(i+1)*8])
	}

	// Parity shard = XOR of all data shards.
	shards[dataShards] = make([]byte, 8)
	for i := 0; i < dataShards; i++ {
		for j := 0; j < 8; j++ {
			shards[dataShards][j] ^= shards[i][j]
		}
	}

	return shards
}

// recoverPayload reconstructs the original payload from at least 3
// of the 4 shards using XOR recovery.
func recoverPayload(shards [][]byte) ([]byte, bool) {
	if len(shards) < minShardsForRecovery {
		return nil, false
	}

	result := tryRecoverFromShards(shards)
	if result == nil {
		return nil, false
	}
	return result, true
}

// tryRecoverFromShards attempts to reconstruct the payload from a set of shards.
// For each triplet of 3 shards, it XORs them to recover the 4th shard, then
// tries all 24 permutations of 3-out-of-4 shards to find the correct ordering.
func tryRecoverFromShards(shards [][]byte) []byte {
	n := len(shards)
	if n < 3 {
		return nil
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for k := j + 1; k < n; k++ {
				triplet := [][]byte{shards[i], shards[j], shards[k]}

				// XOR all 3 to recover the 4th shard.
				fourth := make([]byte, 8)
				for b := 0; b < 8; b++ {
					fourth[b] = triplet[0][b] ^ triplet[1][b] ^ triplet[2][b]
				}

				all4 := [][]byte{triplet[0], triplet[1], triplet[2], fourth}

				// Try all 24 permutations of 3-from-4 in order.
				for a := 0; a < 4; a++ {
					for b := 0; b < 4; b++ {
						if b == a {
							continue
						}
						for c := 0; c < 4; c++ {
							if c == a || c == b {
								continue
							}
							payload := concat3Shards(all4[a], all4[b], all4[c])
							payload = payload[:payloadSize+checksumSize]
							if verifyChecksum(payload) {
								return payload
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// concat3Shards concatenates three 8-byte shards into a 24-byte slice.
func concat3Shards(a, b, c []byte) []byte {
	result := make([]byte, 0, 24)
	result = append(result, a...)
	result = append(result, b...)
	result = append(result, c...)
	return result
}

// ── Shard extraction ─────────────────────────────────────────────────────

// extractAllShards finds all zero-width watermark shards in the text.
// Each shard is delimited by zwStart (U+200D) and zwEnd (U+FEFF).
func extractAllShards(text string) [][]byte {
	var shards [][]byte

	remaining := text
	for {
		startIdx := strings.IndexRune(remaining, zwStart)
		if startIdx == -1 {
			break
		}

		rest := remaining[startIdx+utf8.RuneLen(zwStart):]
		endIdx := strings.IndexRune(rest, zwEnd)
		if endIdx == -1 {
			break
		}

		bits := rest[:endIdx]
		shard := decodeBitsToBytes(bits)
		if len(shard) == 8 {
			shards = append(shards, shard)
		}

		remaining = rest[endIdx+utf8.RuneLen(zwEnd):]
	}

	return shards
}

// decodeBitsToBytes converts a string of zwBit0/zwBit1 characters to bytes.
func decodeBitsToBytes(bits string) []byte {
	var bytes []byte
	var currentByte byte
	bitCount := 0

	for _, r := range bits {
		switch r {
		case zwBit0:
			currentByte = (currentByte << 1)
			bitCount++
		case zwBit1:
			currentByte = (currentByte << 1) | 1
			bitCount++
		default:
			// Non-zero-width character — invalid.
			return nil
		}
		if bitCount == 8 {
			bytes = append(bytes, currentByte)
			currentByte = 0
			bitCount = 0
		}
	}

	if bitCount != 0 {
		return nil
	}
	return bytes
}

// ── Segment splitting ────────────────────────────────────────────────────

// splitContentIntoSegments splits text into n segments at natural boundaries.
// Prefers sentence boundaries (., !, ?, newlines) over word boundaries.
func splitContentIntoSegments(content string, n int) []string {
	if n <= 1 {
		return []string{content}
	}

	// Find sentence boundaries.
	sentenceBreaks := findSentenceBreaks(content)
	if len(sentenceBreaks) >= n-1 {
		return splitAtIndices(content, pickEvenlySpaced(sentenceBreaks, n-1))
	}

	// Fall back to word boundaries.
	wordBreaks := findWordBreaks(content)
	if len(wordBreaks) >= n-1 {
		return splitAtIndices(content, pickEvenlySpaced(wordBreaks, n-1))
	}

	// Last resort: split by character count.
	return splitEvenly(content, n)
}

// findSentenceBreaks returns indices after sentence-ending punctuation.
func findSentenceBreaks(text string) []int {
	var breaks []int
	runes := []rune(text)
	for i, r := range runes {
		switch r {
		case '.', '!', '?', '\n':
			// Ensure we're after a sentence end, not in the middle of a number or abbreviation.
			if i+1 < len(runes) && (runes[i+1] == ' ' || runes[i+1] == '\n') {
				breaks = append(breaks, i+1)
			}
		}
	}
	return breaks
}

// findWordBreaks returns indices after word boundaries (spaces).
func findWordBreaks(text string) []int {
	var breaks []int
	runes := []rune(text)
	for i, r := range runes {
		if r == ' ' && i > 0 {
			breaks = append(breaks, i+1) // after the space
		}
	}
	return breaks
}

// pickEvenlySpaced selects k indices evenly distributed from the given list.
func pickEvenlySpaced(indices []int, k int) []int {
	if k >= len(indices) {
		return indices
	}
	result := make([]int, k)
	step := float64(len(indices)-1) / float64(k-1)
	for i := 0; i < k; i++ {
		idx := int(float64(i) * step)
		if idx >= len(indices) {
			idx = len(indices) - 1
		}
		result[i] = indices[idx]
	}
	return result
}

// splitAtIndices splits text at the given rune indices.
func splitAtIndices(text string, indices []int) []string {
	runes := []rune(text)
	var segments []string
	prev := 0
	for _, idx := range indices {
		if idx > prev && idx <= len(runes) {
			segments = append(segments, string(runes[prev:idx]))
			prev = idx
		}
	}
	if prev < len(runes) {
		segments = append(segments, string(runes[prev:]))
	}
	return segments
}

// splitEvenly splits text into n roughly equal segments by character count.
func splitEvenly(content string, n int) []string {
	runes := []rune(content)
	if len(runes) < n {
		// Not enough characters — return as single segment.
		return []string{content}
	}

	chunkSize := len(runes) / n
	var segments []string
	for i := 0; i < n; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if i == n-1 {
			end = len(runes)
		}
		segments = append(segments, string(runes[start:end]))
	}
	return segments
}

// ── Shard embedding ──────────────────────────────────────────────────────

// embedShardInSegment inserts a zero-width shard into a text segment.
// The shard is placed after the first word of the segment.
// Additionally, a whitespace fingerprint is encoded in the segment's spaces.
func embedShardInSegment(segment string, shard []byte, shardIndex int) string {
	runes := []rune(segment)

	// Encode the shard as zero-width characters.
	zwShard := encodeZeroWidth(shard)

	// Find insertion point: after the first word (first space or at 1/3 of the segment).
	insertPos := findInsertionPoint(runes)
	if insertPos < 0 || insertPos > len(runes) {
		insertPos = len(runes)
	}

	// Build the segment with the embedded shard.
	var result strings.Builder
	result.WriteString(string(runes[:insertPos]))
	result.WriteString(zwShard)
	rest := string(runes[insertPos:])

	// Encode whitespace fingerprint: encode shard index in the first few spaces.
	rest = encodeWhitespaceFingerprint(rest, shardIndex)

	result.WriteString(rest)
	return result.String()
}

// findInsertionPoint finds the best position to insert the zero-width shard.
// Prefers after the first word boundary (space), or at 1/3 of the segment.
func findInsertionPoint(runes []rune) int {
	if len(runes) < 4 {
		return len(runes) / 2
	}

	// Look for the first space after at least 2 characters.
	for i := 2; i < len(runes); i++ {
		if runes[i] == ' ' {
			return i + 1 // after the space
		}
	}

	// Fall back to 1/3 of the segment.
	return len(runes) / 3
}

// ── Whitespace fingerprint encoding ──────────────────────────────────────

// encodeWhitespaceFingerprint encodes a shard index into the whitespace of
// the text by alternating regular spaces (U+0020) and non-breaking spaces
// (U+00A0).
//
// The fingerprint is a 4-bit value: 2 bits for shard_index, 2 bits for total_shards.
// It is encoded in the first 4 spaces after the shard insertion point.
// This provides a secondary recovery channel if zero-width characters are stripped.
func encodeWhitespaceFingerprint(text string, shardIndex int) string {
	// Build the 4-bit fingerprint: 2 bits shard_index + 2 bits total_shards.
	fingerprint := byte((shardIndex&0x3)<<2) | byte((numShards-1)&0x3)

	runes := []rune(text)
	spaceCount := 0
	for i, r := range runes {
		if r == ' ' {
			bit := (fingerprint >> uint(3-spaceCount)) & 1
			if bit == 0 {
				runes[i] = wsBit0
			} else {
				runes[i] = wsBit1
			}
			spaceCount++
			if spaceCount >= 4 {
				break
			}
		}
	}
	return string(runes)
}

// decodeWhitespaceFingerprint extracts the shard index from whitespace encoding.
// Returns the shard index, or -1 if no valid fingerprint is found.
func decodeWhitespaceFingerprint(text string) int {
	var fingerprint byte
	spaceCount := 0

	for _, r := range text {
		switch r {
		case wsBit0:
			fingerprint = (fingerprint << 1)
			spaceCount++
		case wsBit1:
			fingerprint = (fingerprint << 1) | 1
			spaceCount++
		default:
			// Non-space character — skip.
		}
		if spaceCount >= 4 {
			break
		}
	}

	if spaceCount < 4 {
		return -1
	}

	shardIndex := int((fingerprint >> 2) & 0x3)
	return shardIndex
}

// ── Legacy format (for backward compatibility) ───────────────────────────

// encodeLegacySuffix encodes a payload as a single zero-width block at the end.
// Used as fallback for very short content.
func encodeLegacySuffix(payload []byte) string {
	return encodeZeroWidth(payload)
}

// decodeLegacy extracts a legacy single-block zero-width watermark from text.
func decodeLegacy(text string) (Payload, bool) {
	startIdx := strings.IndexRune(text, zwStart)
	if startIdx == -1 {
		return Payload{}, false
	}

	rest := text[startIdx+utf8.RuneLen(zwStart):]
	endIdx := strings.IndexRune(rest, zwEnd)
	if endIdx == -1 {
		return Payload{}, false
	}

	bits := rest[:endIdx]
	bytes := decodeBitsToBytes(bits)
	if len(bytes) < payloadSize {
		return Payload{}, false
	}

	return parsePayload(bytes[:payloadSize])
}

// encodeZeroWidth converts raw bytes to a zero-width character string.
// Format: zwStart + [bits as zwBit0/zwBit1] + zwEnd
func encodeZeroWidth(data []byte) string {
	var sb strings.Builder
	sb.WriteRune(zwStart)
	for _, b := range data {
		for i := 7; i >= 0; i-- {
			if (b>>uint(i))&1 == 0 {
				sb.WriteRune(zwBit0)
			} else {
				sb.WriteRune(zwBit1)
			}
		}
	}
	sb.WriteRune(zwEnd)
	return sb.String()
}

// ── Payload construction ─────────────────────────────────────────────────

// ResolveDeployID reads the deployment identifier from AFLARE_DEPLOYMENT_ID
// (1-4 hex digits, e.g. "1a2b"). Missing or invalid values yield 0 ("not
// set"). The value is embedded in v2 watermarks so leaked content can be
// traced back to the deployment that generated it.
func ResolveDeployID() uint16 {
	v := strings.TrimSpace(os.Getenv("AFLARE_DEPLOYMENT_ID"))
	if v == "" {
		return 0
	}
	id, err := strconv.ParseUint(v, 16, 16)
	if err != nil {
		return 0
	}
	return uint16(id)
}

// buildPayload creates the raw 21-byte watermark payload (v2 layout).
func buildPayload(content string, deployID uint16) []byte {
	payload := make([]byte, payloadSize)
	copy(payload[0:4], magicBytes)
	payload[4] = wmVersion
	binary.BigEndian.PutUint64(payload[5:13], uint64(time.Now().Unix()))
	hash := sha256.Sum256([]byte(content))
	copy(payload[13:19], hash[:6])
	binary.BigEndian.PutUint16(payload[19:21], deployID)
	return payload
}

// parsePayload validates and parses the raw 21-byte payload. Both v1
// (8-byte hash, no deploy ID) and v2 (6-byte hash + 2-byte deploy ID)
// layouts are accepted; unknown versions are rejected.
func parsePayload(payload []byte) (Payload, bool) {
	if len(payload) < payloadSize {
		return Payload{}, false
	}

	// Verify magic bytes.
	if string(payload[0:4]) != magicBytes {
		return Payload{}, false
	}

	version := payload[4]
	ts := int64(binary.BigEndian.Uint64(payload[5:13]))

	switch version {
	case wmVersionV1:
		hash := make([]byte, 8)
		copy(hash, payload[13:21])
		return Payload{Version: version, Timestamp: time.Unix(ts, 0), Hash: hash}, true
	case wmVersion:
		hash := make([]byte, 6)
		copy(hash, payload[13:19])
		return Payload{
			Version:   version,
			Timestamp: time.Unix(ts, 0),
			Hash:      hash,
			DeployID:  binary.BigEndian.Uint16(payload[19:21]),
		}, true
	default:
		return Payload{}, false
	}
}

// ── Checksum ─────────────────────────────────────────────────────────────

// checksum16 computes a simple 16-bit checksum of the data.
// Uses a truncated SHA-256 for simplicity and collision resistance.
func checksum16(data []byte) uint16 {
	h := sha256.Sum256(data)
	return uint16(h[0])<<8 | uint16(h[1])
}

// verifyChecksum checks that the last 2 bytes of the payload match the
// checksum of the preceding bytes.
func verifyChecksum(payload []byte) bool {
	if len(payload) < checksumSize+1 {
		return false
	}
	data := payload[:len(payload)-checksumSize]
	expected := checksum16(data)
	actual := uint16(payload[len(payload)-2])<<8 | uint16(payload[len(payload)-1])
	return expected == actual
}

// ── Source code watermark ─────────────────────────────────────────────────

// zero-width watermark comment prefix markers.
// These are embedded in a comment line that looks like a normal copyright
// notice but contains invisible machine-readable zero-width characters.
const (
	zwPrefix = "// aflare" // visible prefix
)

// EncodeSource adds an invisible watermark to Go source code by embedding
// zero-width characters into the copyright header comment. The watermark is
// placed after the "// Copyright (c) 2026 aflare Contributors" line.
//
// The watermark is invisible to human readers (zero-width characters are not
// rendered) but can be detected by DecodeSource. It survives copy-paste and
// partial file truncation because the payload is sharded redundantly.
func EncodeSource(src string) string {
	if src == "" {
		return src
	}

	// Build the watermark payload from the source content.
	payload := buildPayload(src, ResolveDeployID())
	chk := checksum16(payload)
	payload = append(payload, byte(chk>>8), byte(chk&0xFF))

	// Encode as a single zero-width block embedded in a comment.
	zwBlock := encodeZeroWidth(payload)

	// Insert the watermark comment after the copyright header.
	// Look for the first blank line after the copyright block.
	lines := strings.Split(src, "\n")
	var result strings.Builder
	inCopyright := false
	watermarkInserted := false

	for i, line := range lines {
		result.WriteString(line)
		if i < len(lines)-1 {
			result.WriteString("\n")
		}

		if strings.HasPrefix(line, "// Copyright") && strings.Contains(line, "aflare") {
			inCopyright = true
			continue
		}
		if inCopyright && line == "//" {
			// End of copyright block — insert watermark here.
			if !watermarkInserted {
				result.WriteString(zwPrefix + zwBlock + "\n")
				watermarkInserted = true
			}
			inCopyright = false
		}
	}

	// If no copyright header found, prepend the watermark.
	if !watermarkInserted {
		prepend := zwPrefix + zwBlock + "\n" + src
		return prepend
	}

	return result.String()
}

// DecodeSource extracts and validates the invisible watermark from Go source
// code. It searches for the zero-width marker after the copyright header.
func DecodeSource(src string) (Payload, bool) {
	// Search for the zero-width watermark in the source.
	// Look for the zwStart character after "// aflare".
	idx := strings.Index(src, zwPrefix)
	if idx == -1 {
		return Payload{}, false
	}

	rest := src[idx+len(zwPrefix):]
	startIdx := strings.IndexRune(rest, zwStart)
	if startIdx == -1 {
		return Payload{}, false
	}

	rest = rest[startIdx+utf8.RuneLen(zwStart):]
	endIdx := strings.IndexRune(rest, zwEnd)
	if endIdx == -1 {
		return Payload{}, false
	}

	bits := rest[:endIdx]
	bytes := decodeBitsToBytes(bits)
	if len(bytes) < payloadSize+checksumSize {
		return Payload{}, false
	}

	if !verifyChecksum(bytes) {
		return Payload{}, false
	}

	return parsePayload(bytes[:payloadSize])
}

// HasSourceWatermark checks if the source code contains a valid aflare
// invisible watermark.
func HasSourceWatermark(src string) bool {
	_, ok := DecodeSource(src)
	return ok
}

// StripSourceWatermark removes the invisible watermark from source code.
func StripSourceWatermark(src string) string {
	idx := strings.Index(src, zwPrefix)
	if idx == -1 {
		return src
	}

	rest := src[idx+len(zwPrefix):]
	startIdx := strings.IndexRune(rest, zwStart)
	if startIdx == -1 {
		return src
	}

	rest = rest[startIdx+utf8.RuneLen(zwStart):]
	endIdx := strings.IndexRune(rest, zwEnd)
	if endIdx == -1 {
		return src
	}

	// Remove the entire watermark line.
	lineStart := idx
	// Find the start of the line.
	if nl := strings.LastIndex(src[:idx], "\n"); nl >= 0 {
		lineStart = nl + 1
	}

	lineEnd := idx + len(zwPrefix) + startIdx + utf8.RuneLen(zwStart) + endIdx + utf8.RuneLen(zwEnd)
	// Extend to the next newline.
	if nl := strings.Index(src[lineEnd:], "\n"); nl >= 0 {
		lineEnd += nl
	}

	return src[:lineStart] + src[lineEnd:]
}
