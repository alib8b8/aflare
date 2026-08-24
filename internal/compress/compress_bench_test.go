// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​​‌​‌‌‌​‌​​‌‌‌​‌​​‌​​​‌‌​‌​​‌‌​​‌​‌‌‌​‌‌​‌‌‌‌​‌​​​​​​​​​​​​​​​​​​‌​​​‌​​​‌‌‌‌‌‌⁠
package compress

import (
	"strings"
	"testing"
)

func BenchmarkCompress_Small(b *testing.B) {
	input := "Hello world, this is a small text for benchmarking the compress function."
	cfg := DefaultConfig()
	cfg.MaxOutputChars = 50
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Compress(input, cfg)
	}
}

func BenchmarkCompress_Medium(b *testing.B) {
	input := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 20)
	cfg := DefaultConfig()
	cfg.MaxOutputChars = 200
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Compress(input, cfg)
	}
}

func BenchmarkCompress_Large(b *testing.B) {
	input := strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 100)
	cfg := DefaultConfig()
	cfg.MaxOutputChars = 1000
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Compress(input, cfg)
	}
}

func BenchmarkExtractKeywords(b *testing.B) {
	input := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExtractKeywords(input, 10)
	}
}

func BenchmarkTokenize(b *testing.B) {
	input := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tokenize(input)
	}
}
