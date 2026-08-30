// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌​​‌‌‌​‌‌‌‌‌‌​​‌​‌​​‌‌​‌​‌​‌​‌​​‌‌​​​‌​​‌‌‌​​​​‌‌​​​‌‌​‌​‌‌​​‌​‌‌​​‌‌​​​​​​​​​​​​​​​​​‌‌‌​‌‌‌​‌​​‌‌‌‌⁠
// aflare​‌​​​​​‌​‌​​‌‌​‌​​‌​​‌‌​​​‌​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​​‌‌​​‌​​​​‌​‌​​‌‌‌‌‌​​​​​​‌‌​​‌​​‌‌​​‌‌​‌‌​‌‌​‌‌‌‌‌‌​‌​​‌​​​​​​​​​​​​​‌​‌​​​‌‌​​​
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

// Package strutil provides byte-budget-aware string helpers that keep the
// result valid UTF-8.
package strutil

import "unicode/utf8"

// Truncate cuts s to at most max bytes without splitting a multi-byte
// UTF-8 rune. A plain s[:max] emits invalid UTF-8 when the cut lands inside
// a CJK or emoji rune — seen as mojibake in docs/nodes-reference.md
// (doc_gen / engineer_skills rows) and in ADHD-mode step output. Trailing
// bytes of a torn rune (whether continuation bytes or an orphaned lead
// byte) are stripped until the result ends on a complete rune; it is
// therefore never longer than max bytes and always valid UTF-8. The
// caller owns the ellipsis so this stays a pure byte-budget primitive.
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if max >= len(s) {
		return s
	}
	cut := s[:max]
	// Strip trailing bytes of an incomplete rune. DecodeLastRuneInString
	// returns (RuneError, 1) both for a dangling continuation byte and for
	// an orphaned lead byte (s[:1] of a 3-byte CJK rune starts with a
	// RuneStart byte, so a boundary-only walk misses that case). A legal
	// rune — ASCII (any r) or U+FFFD (r == RuneError but size 3) — breaks.
	for len(cut) > 0 {
		r, size := utf8.DecodeLastRuneInString(cut)
		if r != utf8.RuneError || size != 1 {
			break
		}
		cut = cut[:len(cut)-1]
	}
	return cut
}
