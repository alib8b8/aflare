// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​​​​‌​​​‌‌​​​‌‌‌‌​‌‌‌​​​‌‌​‌​​‌‌​‌​​‌​​​‌‌​​‌​​​​​​​​​​​​​​​​​​​​​​​‌​‌‌‌‌​‌​‌⁠
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

package compress

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Algorithm != AlgoHybrid {
		t.Errorf("Default algorithm = %v, want %v", cfg.Algorithm, AlgoHybrid)
	}
	if cfg.TargetRatio != 0.2 {
		t.Errorf("Default TargetRatio = %v, want 0.2", cfg.TargetRatio)
	}
	if cfg.MaxOutputChars != 4000 {
		t.Errorf("Default MaxOutputChars = %v, want 4000", cfg.MaxOutputChars)
	}
}

func TestCompressEmpty(t *testing.T) {
	cfg := DefaultConfig()
	result := Compress("", cfg)
	if result.Text != "" {
		t.Errorf("Empty input text = %q, want empty", result.Text)
	}
	if result.OriginalChars != 0 {
		t.Errorf("Empty input OriginalChars = %d, want 0", result.OriginalChars)
	}
}

func TestCompressExtract(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog. " +
		"This is a second sentence about foxes. " +
		"Here is another sentence. " +
		"Yet another sentence to compress. " +
		"More content here about the fox. " +
		"The dog was lazy indeed."
	cfg := Config{
		Algorithm:      AlgoExtract,
		TargetRatio:    0.5,
		MaxOutputChars: 4000,
	}
	result := Compress(text, cfg)
	if len(result.Text) >= len(text) {
		t.Errorf("Extract output (%d) should be shorter than input (%d)", len(result.Text), len(text))
	}
	if result.Algorithm != AlgoExtract {
		t.Errorf("Algorithm = %v, want %v", result.Algorithm, AlgoExtract)
	}
}

func TestCompressKeyword(t *testing.T) {
	text := "Machine learning is a subset of artificial intelligence. " +
		"Machine learning algorithms build models based on sample data. " +
		"Deep learning is part of machine learning methods. " +
		"Neural networks are used in deep learning. " +
		"AI systems can perform tasks requiring human intelligence. " +
		"The training process is crucial for model accuracy."
	cfg := Config{
		Algorithm:      AlgoKeyword,
		TargetRatio:    0.5,
		MaxOutputChars: 4000,
	}
	result := Compress(text, cfg)
	if len(result.Keywords) == 0 {
		t.Error("Keyword algorithm should return keywords")
	}
	if result.Algorithm != AlgoKeyword {
		t.Errorf("Algorithm = %v, want %v", result.Algorithm, AlgoKeyword)
	}
}

func TestCompressCluster(t *testing.T) {
	text := "Sentence one about technology. " +
		"Sentence two about science. " +
		"Sentence three about engineering. " +
		"Sentence four about research. " +
		"Sentence five about development. " +
		"Sentence six about innovation. " +
		"Sentence seven about progress."
	cfg := Config{
		Algorithm:      AlgoCluster,
		TargetRatio:    0.4,
		MaxOutputChars: 4000,
	}
	result := Compress(text, cfg)
	if len(result.Text) >= len(text) {
		t.Errorf("Cluster output (%d) should be shorter than input (%d)", len(result.Text), len(text))
	}
	if result.Algorithm != AlgoCluster {
		t.Errorf("Algorithm = %v, want %v", result.Algorithm, AlgoCluster)
	}
}

func TestCompressSlidingWindow(t *testing.T) {
	text := "HEADER SECTION\n\nThis is the beginning of a long document. " +
		"Middle content that should be compressed goes here. " +
		"More middle content. " +
		"Even more content in the middle. " +
		"Additional content to compress. " +
		"The end of the document with important final words."
	cfg := Config{
		Algorithm:       AlgoSlidingWindow,
		TargetRatio:     0.3,
		MaxOutputChars:  4000,
		PreserveHeaders: true,
	}
	result := Compress(text, cfg)
	if len(result.PreservedParts) == 0 {
		t.Error("Sliding window should preserve headers when enabled")
	}
	if result.Algorithm != AlgoSlidingWindow {
		t.Errorf("Algorithm = %v, want %v", result.Algorithm, AlgoSlidingWindow)
	}
}

func TestCompressHybrid(t *testing.T) {
	text := "First sentence. Second sentence. Third sentence. " +
		"Fourth sentence. Fifth sentence. Sixth sentence. " +
		"Seventh sentence. Eighth sentence. Ninth sentence. " +
		"Tenth sentence about the topic."

	cfgHigh := Config{
		Algorithm:      AlgoHybrid,
		TargetRatio:    0.6,
		MaxOutputChars: 4000,
	}
	resultHigh := Compress(text, cfgHigh)
	if resultHigh.Algorithm != AlgoHybrid {
		t.Errorf("High ratio algorithm = %v, want %v", resultHigh.Algorithm, AlgoHybrid)
	}

	cfgLow := Config{
		Algorithm:      AlgoHybrid,
		TargetRatio:    0.15,
		MaxOutputChars: 4000,
	}
	resultLow := Compress(text, cfgLow)
	if resultLow.Algorithm != AlgoHybrid {
		t.Errorf("Low ratio algorithm = %v, want %v", resultLow.Algorithm, AlgoHybrid)
	}
}

func TestCompressInvalidConfig(t *testing.T) {
	text := "This is a test sentence. Another sentence here."
	cfg := Config{
		Algorithm:      "",
		TargetRatio:    0,
		MaxOutputChars: 0,
	}
	result := Compress(text, cfg)
	if result.Ratio == 0 {
		t.Error("Zero ratio should default to 0.2, producing non-zero ratio result")
	}
	if result.CompressedChars == 0 {
		t.Error("Zero max output chars should default to 4000")
	}
}

func TestCompressTargetRatio(t *testing.T) {
	text := "First sentence. Second sentence. Third sentence. " +
		"Fourth sentence. Fifth sentence. Sixth sentence. " +
		"Seventh sentence. Eighth sentence. Ninth sentence. " +
		"Tenth sentence. Eleventh sentence. Twelfth sentence."

	cfgLow := Config{
		Algorithm:      AlgoExtract,
		TargetRatio:    0.2,
		MaxOutputChars: 4000,
	}
	cfgHigh := Config{
		Algorithm:      AlgoExtract,
		TargetRatio:    0.6,
		MaxOutputChars: 4000,
	}

	resultLow := Compress(text, cfgLow)
	resultHigh := Compress(text, cfgHigh)

	if len(resultHigh.Text) < len(resultLow.Text) {
		t.Errorf("Higher ratio should produce longer output. high=%d, low=%d", len(resultHigh.Text), len(resultLow.Text))
	}
}

func TestCompressMaxOutputChars(t *testing.T) {
	text := "This is sentence one. This is sentence two. This is sentence three. " +
		"This is sentence four. This is sentence five. This is sentence six."
	cfg := Config{
		Algorithm:      AlgoExtract,
		TargetRatio:    0.8,
		MaxOutputChars: 50,
	}
	result := Compress(text, cfg)
	if len(result.Text) < 50 {
		t.Errorf("MaxOutputChars should be respected. got %d, expected >= 50", len(result.Text))
	}
}

func TestExtractKeywords(t *testing.T) {
	text := "Machine learning and artificial intelligence. " +
		"Machine learning uses neural networks. " +
		"Deep learning is advanced machine learning. " +
		"AI systems use machine learning algorithms."

	keywords := ExtractKeywords(text, 5)
	if len(keywords) == 0 {
		t.Error("ExtractKeywords should return keywords")
	}

	found := false
	for _, kw := range keywords {
		if kw == "machine" || kw == "learning" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'machine' or 'learning' in top keywords, got %v", keywords)
	}
}

func TestCacheHit(t *testing.T) {
	text := "This is a caching test sentence. Another sentence for testing."
	cfg := DefaultConfig()

	result1 := Compress(text, cfg)
	result2 := Compress(text, cfg)

	if result1.Text != result2.Text {
		t.Errorf("Cached results should be identical. first=%q, second=%q", result1.Text, result2.Text)
	}
	if result1.Ratio != result2.Ratio {
		t.Errorf("Cached ratios should match. first=%f, second=%f", result1.Ratio, result2.Ratio)
	}
}
