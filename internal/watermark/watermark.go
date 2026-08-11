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
// Two watermark modes are supported:
//
//  1. Zero-width watermark: invisible Unicode characters embedded in text
//     output. Uses U+200B (zero-width space) for bit 0 and U+200C
//     (zero-width non-joiner) for bit 1, delimited by U+200D start and
//     U+FEFF end markers. Invisible in all renderers.
//
//  2. YAML comment watermark: a human-readable comment line in generated
//     YAML that includes the same payload as base64. Visible but
//     non-intrusive.
//
// The watermark payload contains:
//   - Magic bytes "AFLR" (4 bytes) for identification
//   - Version byte (1 byte, currently 0x01)
//   - Timestamp (8 bytes, Unix epoch seconds)
//   - Content hash (8 bytes, first 8 bytes of SHA-256)
//
// Total: 21 bytes → 168 zero-width characters.
package watermark

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// zero-width characters used for invisible encoding
	zwBit0 = '\u200B' // zero-width space → bit 0
	zwBit1 = '\u200C' // zero-width non-joiner → bit 1
	zwStart = '\u200D' // zero-width joiner → start marker
	zwEnd   = '\uFEFF' // zero-width no-break space (BOM) → end marker

	// magic bytes for watermark identification
	magicBytes = "AFLR"

	// current watermark version
	wmVersion = byte(0x01)

	// payloadSize is the total payload size in bytes.
	payloadSize = 4 + 1 + 8 + 8 // magic + version + timestamp + hash
)

// Payload is the decoded content of a watermark.
type Payload struct {
	Version   byte      // watermark version
	Timestamp time.Time // when the content was generated
	Hash      []byte    // first 8 bytes of content SHA-256
}

// EncodeText generates a zero-width watermark string for the given content.
// Returns an empty string if content is empty.
// The watermark is appended after the content, invisible to humans.
func EncodeText(content string) string {
	if content == "" {
		return ""
	}
	payload := buildPayload(content)
	return encodeZeroWidth(payload)
}

// EncodeTextWithSuffix returns content with the zero-width watermark appended.
// The watermark is invisible in all text renderers but can be extracted
// with DecodeText.
func EncodeTextWithSuffix(content string) string {
	if content == "" {
		return content
	}
	wm := EncodeText(content)
	if wm == "" {
		return content
	}
	return content + wm
}

// DecodeText extracts and validates a zero-width watermark from text.
// Returns the payload and true if a valid watermark was found.
func DecodeText(text string) (Payload, bool) {
	payload := extractZeroWidth(text)
	if payload == nil {
		return Payload{}, false
	}
	return parsePayload(payload)
}

// HasWatermark checks if the text contains a valid aflare watermark.
func HasWatermark(text string) bool {
	_, ok := DecodeText(text)
	return ok
}

// EncodeYAML generates a watermark comment line for YAML output.
// Format: # aflare-watermark: <base64-payload>
func EncodeYAML(content string) string {
	if content == "" {
		return ""
	}
	payload := buildPayload(content)
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

// Info returns a human-readable description of the watermark system.
func Info() string {
	return `aflare Watermark System
─────────────────────
Two modes:
  Text  — invisible zero-width Unicode characters (U+200B/U+200C)
  YAML  — comment line: # aflare-watermark: <base64>

Payload (21 bytes):
  Magic:    AFLR (4 bytes)
  Version:  1 byte
  Time:     8 bytes (Unix seconds)
  Hash:     8 bytes (SHA-256 of content)

Usage:
  aflare watermark decode <file>   — extract watermark from file
  aflare watermark verify <file>   — verify watermark integrity
  aflare watermark info            — show this info`
}

// ── internal helpers ────────────────────────────────────────────────────

// buildPayload creates the raw watermark payload bytes.
func buildPayload(content string) []byte {
	payload := make([]byte, payloadSize)
	copy(payload[0:4], magicBytes)
	payload[4] = wmVersion
	binary.BigEndian.PutUint64(payload[5:13], uint64(time.Now().Unix()))
	hash := sha256.Sum256([]byte(content))
	copy(payload[13:21], hash[:8])
	return payload
}

// encodeZeroWidth converts raw bytes to zero-width character string.
// Format: zwStart + [bits as zwBit0/zwBit1] + zwEnd
func encodeZeroWidth(payload []byte) string {
	var sb strings.Builder
	sb.WriteRune(zwStart)
	for _, b := range payload {
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

// extractZeroWidth extracts and decodes the zero-width watermark from text.
// Returns nil if no valid zero-width sequence is found.
func extractZeroWidth(text string) []byte {
	// Find the start marker
	startIdx := strings.IndexRune(text, zwStart)
	if startIdx == -1 {
		return nil
	}

	// Find the end marker after the start marker
	rest := text[startIdx+utf8.RuneLen(zwStart):]
	endIdx := strings.IndexRune(rest, zwEnd)
	if endIdx == -1 {
		return nil
	}

	// Extract the bit sequence between markers
	bits := rest[:endIdx]
	var bytes []byte
	var currentByte byte
	bitCount := 0

	for _, r := range bits {
		switch r {
		case zwBit0:
			currentByte = (currentByte << 1) | 0
			bitCount++
		case zwBit1:
			currentByte = (currentByte << 1) | 1
			bitCount++
		default:
			// Non-zero-width character in the middle — invalid watermark
			return nil
		}
		if bitCount == 8 {
			bytes = append(bytes, currentByte)
			currentByte = 0
			bitCount = 0
		}
	}

	// Partial byte at the end — invalid
	if bitCount != 0 {
		return nil
	}

	return bytes
}

// parsePayload validates and parses the raw payload bytes.
func parsePayload(payload []byte) (Payload, bool) {
	if len(payload) < payloadSize {
		return Payload{}, false
	}

	// Verify magic bytes
	if string(payload[0:4]) != magicBytes {
		return Payload{}, false
	}

	version := payload[4]
	if version != wmVersion {
		return Payload{}, false
	}

	ts := int64(binary.BigEndian.Uint64(payload[5:13]))
	hash := make([]byte, 8)
	copy(hash, payload[13:21])

	return Payload{
		Version:   version,
		Timestamp: time.Unix(ts, 0),
		Hash:      hash,
	}, true
}