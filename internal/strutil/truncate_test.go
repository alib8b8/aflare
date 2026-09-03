// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌‌​​​‌‌​​​​​​‌​​​​​‌‌​​‌​‌‌​‌​​‌‌‌‌‌​​‌‌‌​​​​​​​‌​​​​​​​​​‌​​​‌​‌​​​​​​​​​​​​​​​​​​‌​​‌‌‌‌​​​‌‌‌‌⁠
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

package strutil

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"ASCII unchanged under budget", "short", 117, "short"},
		{"ASCII exactly at budget", "abcdefghij", 10, "abcdefghij"},
		{"ASCII over budget", "abcdefghijk", 10, "abcdefghij"},
		{"empty string", "", 117, ""},
		{"zero budget", "anything", 0, ""},
		{"negative budget", "anything", -3, ""},
		// 中文三字节：117 = 3×39 恰好落在边界；116 落在 rune 中间必须回退。
		{"cut lands on rune boundary", strings.Repeat("模", 50), 117, strings.Repeat("模", 39)},
		{"cut lands mid-rune", strings.Repeat("模", 50), 116, strings.Repeat("模", 38)},
		// 混合：ASCII 前缀 + 中文。max=9 切在第一个"块"内部 → 回退到 ASCII 前缀；
		// max=10 恰好包含一个完整的"块"。
		{"mixed cut tears cjk rune", "prefix:" + strings.Repeat("块", 40), 9, "prefix:"},
		{"mixed cut keeps whole rune", "prefix:" + strings.Repeat("块", 40), 10, "prefix:块"},
		// emoji 四字节。
		{"emoji four-byte runes", strings.Repeat("🎉", 40), 10, strings.Repeat("🎉", 2)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.in, tt.max)
			if got != tt.want {
				t.Errorf("Truncate(max=%d) = %q (%d bytes), want %q (%d bytes)",
					tt.max, got, len(got), tt.want, len(tt.want))
			}
			if !utf8.ValidString(got) {
				t.Errorf("result is not valid UTF-8: %q", got)
			}
			if tt.max > 0 && len(got) > tt.max {
				t.Errorf("result %d bytes exceeds budget %d", len(got), tt.max)
			}
		})
	}
}

// TestTruncate_CJKNeverInvalid pins the original bug: a byte-level cut on a
// Chinese description produced invalid UTF-8 that shipped to
// docs/nodes-reference.md as � mojibake (doc_gen / engineer_skills rows).
// Every possible cut point of a CJK string must yield valid UTF-8.
func TestTruncate_CJKNeverInvalid(t *testing.T) {
	s := strings.Repeat("模块描述", 30) // 3-byte runes
	for max := 0; max <= len(s); max++ {
		got := Truncate(s, max)
		if !utf8.ValidString(got) {
			t.Fatalf("Truncate(max=%d) produced invalid UTF-8: %q", max, got)
		}
		if len(got) > max {
			t.Fatalf("Truncate(max=%d) returned %d bytes", max, len(got))
		}
	}
}
