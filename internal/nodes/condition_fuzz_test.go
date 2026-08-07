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

package nodes

import (
	"strings"
	"testing"
)

func FuzzSafeRegexMatch(f *testing.F) {
	f.Add(`\d+`, "abc123")
	f.Add(`^test$`, "test")
	f.Add(`(a+)+$`, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaab")
	f.Add("", "")
	f.Add(strings.Repeat("a", 600), "input")
	f.Add(`[`, "test")
	f.Add(`\w+`, strings.Repeat("x", 1024*1024+1))

	f.Fuzz(func(t *testing.T, pattern string, input string) {
		matched, err := SafeRegexMatch(pattern, input)

		if len(pattern) > maxRegexPatternLength {
			if err == nil {
				t.Errorf("expected error for pattern length %d > %d, got nil", len(pattern), maxRegexPatternLength)
			}
			return
		}

		if err == nil {
			_ = matched
		}
	})
}
