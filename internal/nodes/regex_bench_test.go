package nodes

import (
	"strings"
	"testing"
)

func BenchmarkRegexCache_Hit(b *testing.B) {
	pattern := `https?://[^\s)]+`
	text := strings.Repeat("Visit https://example.com for more info. ", 10)
	for i := 0; i < b.N; i++ {
		regexpFindAllStringSubmatch(pattern, text, -1)
	}
}

func BenchmarkExtractLinksFromText(b *testing.B) {
	text := strings.Repeat("Check [Google](https://google.com) and https://github.com for info. ", 5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractLinksFromText(text, "")
	}
}

func BenchmarkTruncate(b *testing.B) {
	longStr := strings.Repeat("a", 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		truncate(longStr, 500)
	}
}

func BenchmarkParseReActResponse(b *testing.B) {
	resp := `{"thought": "I need to fetch the URL", "action": "fetch_url", "action_input": "https://example.com"}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseReActResponse(resp)
	}
}
