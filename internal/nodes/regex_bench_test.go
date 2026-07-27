package nodes

import (
	"strings"
	"testing"
)

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
