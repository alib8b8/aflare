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
