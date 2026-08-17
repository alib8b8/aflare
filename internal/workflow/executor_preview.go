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

package workflow

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// PreviewMaxBytes is the payload size above which preview_input steps see a
// bounded preview instead of the full value. 16 KiB is large enough to keep
// complete typical documents visible verbatim while capping giant step
// outputs (crawl dumps, batch results) that would otherwise dominate the
// prompt of the LLM step consuming them.
const PreviewMaxBytes = 16 * 1024

const (
	previewHeadBytes = 512 // bytes shown from the start
	previewTailBytes = 256 // bytes shown from the end
)

// BoundedPreview renders s as a compact pass-by-reference view when it is
// larger than maxBytes: a header line with the kind and exact size, head and
// tail samples, and a note that the full value is preserved in workflow
// state. Values at or below maxBytes are returned unchanged.
//
// Samples are cut on UTF-8 rune boundaries so multibyte content is never
// mangled into invalid sequences, and never split mid-line when a boundary
// falls inside a line (the partial line is dropped rather than shown
// truncated, which would read as corrupted data).
func BoundedPreview(s string, maxBytes int) string {
	if maxBytes <= 0 {
		maxBytes = PreviewMaxBytes
	}
	if len(s) <= maxBytes {
		return s
	}

	head := cutOnBoundary(s, previewHeadBytes, true)
	tail := cutOnBoundary(s, previewTailBytes, false)
	omitted := len(s) - len(head) - len(tail)

	var sb strings.Builder
	fmt.Fprintf(&sb, "[aflare bounded preview — full payload: %d bytes (string), %d bytes elided]\n", len(s), omitted)
	sb.WriteString(head)
	sb.WriteString("\n[… ")
	fmt.Fprintf(&sb, "%d", omitted)
	sb.WriteString(" bytes omitted …]\n")
	sb.WriteString(tail)
	sb.WriteString("\n[preview only: the full payload is preserved in workflow state and is passed unchanged to non-preview steps]\n")
	return sb.String()
}

// cutOnBoundary returns at most n bytes from head (fromStart=true) or tail of
// s, trimmed to whole runes and whole lines.
func cutOnBoundary(s string, n int, fromStart bool) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	var cut string
	if fromStart {
		cut = s[:n]
	} else {
		cut = s[len(s)-n:]
	}

	// Align to rune boundary.
	for len(cut) > 0 && !utf8.RuneStart(cut[0]) {
		cut = cut[1:]
	}
	for len(cut) > 0 && !utf8.RuneStart(cut[len(cut)-1]) {
		cut = cut[:len(cut)-1]
	}

	// Prefer whole lines, but keep a torn fragment over nothing: single
	// huge-line payloads (base64 blobs, one-line JSON) have no usable line
	// boundary to cut on.
	runeAligned := cut
	if fromStart {
		if i := strings.LastIndexByte(cut, '\n'); i >= n/2 {
			cut = cut[:i]
		} else {
			cut = runeAligned
		}
	} else {
		if i := strings.IndexByte(cut, '\n'); i >= 0 && len(cut)-i-1 >= n/2 {
			cut = cut[i+1:]
		} else {
			cut = runeAligned
		}
	}
	return strings.Trim(cut, "\n")
}
