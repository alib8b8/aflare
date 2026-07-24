package compress

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"
)

type Algorithm string

const (
	AlgoExtract       Algorithm = "extract"
	AlgoAbstract      Algorithm = "abstract"
	AlgoKeyword       Algorithm = "keyword"
	AlgoCluster       Algorithm = "cluster"
	AlgoSlidingWindow Algorithm = "sliding_window"
	AlgoHybrid        Algorithm = "hybrid"
)

type Config struct {
	Algorithm       Algorithm
	TargetRatio     float64
	MaxOutputChars  int
	PreserveHeaders bool
	PreserveNumbers bool
}

type Result struct {
	Text            string
	OriginalChars   int
	CompressedChars int
	Ratio           float64
	Algorithm       Algorithm
	Keywords        []string
	PreservedParts  []string
}

func DefaultConfig() Config {
	return Config{
		Algorithm:       AlgoHybrid,
		TargetRatio:     0.2,
		MaxOutputChars:  4000,
		PreserveHeaders: true,
		PreserveNumbers: true,
	}
}

var (
	stopWords = map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"is": true, "are": true, "was": true, "were": true, "be": true,
		"been": true, "being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true, "can": true,
		"this": true, "that": true, "these": true, "those": true, "it": true,
		"its": true, "they": true, "them": true, "their": true, "we": true,
		"our": true, "you": true, "your": true, "he": true, "she": true,
		"的": true, "了": true, "在": true, "是": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
		"都": true, "一": true, "一个": true, "上": true, "也": true,
		"很": true, "到": true, "说": true, "要": true, "去": true,
		"你": true, "会": true, "着": true, "没有": true, "看": true,
		"好": true, "自己": true, "这": true, "那": true, "他": true,
		"她": true, "它": true, "们": true, "这个": true, "那个": true,
	}

	sentenceSplitRe = regexp.MustCompile(`[.!?。！？\n]+`)
	numberRe        = regexp.MustCompile(`\b\d+(?:\.\d+)?(?:%|x|k|m|b)?\b`)
	cacheMu         sync.RWMutex
	resultCache     = make(map[string]Result)
)

func Compress(text string, cfg Config) Result {
	if cfg.Algorithm == "" {
		cfg.Algorithm = AlgoHybrid
	}
	if cfg.TargetRatio <= 0 || cfg.TargetRatio > 1 {
		cfg.TargetRatio = 0.2
	}
	if cfg.MaxOutputChars <= 0 {
		cfg.MaxOutputChars = 4000
	}

	origLen := len(text)
	if origLen == 0 {
		return Result{Algorithm: cfg.Algorithm}
	}

	cacheKey := makeCacheKey(text, cfg)
	cacheMu.RLock()
	if cached, ok := resultCache[cacheKey]; ok {
		cacheMu.RUnlock()
		return cached
	}
	cacheMu.RUnlock()

	var result Result

	switch cfg.Algorithm {
	case AlgoExtract:
		result = compressExtract(text, cfg)
	case AlgoKeyword:
		result = compressKeyword(text, cfg)
	case AlgoCluster:
		result = compressCluster(text, cfg)
	case AlgoSlidingWindow:
		result = compressSlidingWindow(text, cfg)
	case AlgoHybrid:
		result = compressHybrid(text, cfg)
	default:
		result = compressExtract(text, cfg)
	}

	result.OriginalChars = origLen
	result.CompressedChars = len(result.Text)
	if origLen > 0 {
		result.Ratio = float64(result.CompressedChars) / float64(origLen)
	}
	result.Algorithm = cfg.Algorithm

	if len(result.Text) > cfg.MaxOutputChars {
		result.Text = result.Text[:cfg.MaxOutputChars] + "... (truncated)"
		result.CompressedChars = len(result.Text)
	}

	cacheMu.Lock()
	if len(resultCache) > 1000 {
		for k := range resultCache {
			delete(resultCache, k)
			break
		}
	}
	resultCache[cacheKey] = result
	cacheMu.Unlock()

	return result
}

func compressExtract(text string, cfg Config) Result {
	sentences := splitSentences(text)
	if len(sentences) <= 1 {
		return Result{Text: text}
	}

	targetSentences := max(1, int(float64(len(sentences))*cfg.TargetRatio))

	headerSentences := []string{}
	if cfg.PreserveHeaders {
		for _, s := range sentences {
			trimmed := strings.TrimSpace(s)
			if len(trimmed) > 0 && len(trimmed) < 100 &&
				(strings.HasSuffix(trimmed, ":") ||
					strings.ToUpper(trimmed) == trimmed ||
					(len(trimmed) > 0 && unicode.IsUpper(rune(trimmed[0])) && !strings.Contains(trimmed, " "))) {
				headerSentences = append(headerSentences, s)
			}
		}
	}

	scored := make([]struct {
		idx      int
		sentence string
		score    float64
	}, len(sentences))

	wordFreq := buildWordFrequency(sentences)

	for i, s := range sentences {
		scored[i] = struct {
			idx      int
			sentence string
			score    float64
		}{idx: i, sentence: s, score: scoreSentence(s, wordFreq, cfg)}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	selected := make(map[int]bool)
	for _, h := range headerSentences {
		for i, s := range sentences {
			if s == h {
				selected[i] = true
			}
		}
	}

	for _, sc := range scored {
		if len(selected) >= targetSentences {
			break
		}
		selected[sc.idx] = true
	}

	var parts []string
	for i, s := range sentences {
		if selected[i] {
			parts = append(parts, strings.TrimSpace(s))
		}
	}

	return Result{
		Text: strings.Join(parts, ". "),
	}
}

func compressKeyword(text string, cfg Config) Result {
	keywords := ExtractKeywords(text, 20)

	sentences := splitSentences(text)
	targetSentences := max(1, int(float64(len(sentences))*cfg.TargetRatio))

	scored := make([]struct {
		idx   int
		s     string
		score int
	}, len(sentences))

	for i, s := range sentences {
		score := 0
		low := strings.ToLower(s)
		for _, kw := range keywords {
			if strings.Contains(low, strings.ToLower(kw)) {
				score++
			}
		}
		scored[i] = struct {
			idx   int
			s     string
			score int
		}{idx: i, s: s, score: score}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	selected := make(map[int]bool)
	for i := 0; i < targetSentences && i < len(scored); i++ {
		selected[scored[i].idx] = true
	}

	var parts []string
	for i, s := range sentences {
		if selected[i] {
			parts = append(parts, strings.TrimSpace(s))
		}
	}

	return Result{
		Text:     strings.Join(parts, ". "),
		Keywords: keywords,
	}
}

func compressCluster(text string, cfg Config) Result {
	sentences := splitSentences(text)
	if len(sentences) <= 3 {
		return compressExtract(text, cfg)
	}

	numClusters := max(2, int(float64(len(sentences))*cfg.TargetRatio*2))

	clusters := make([][]string, numClusters)
	for i := 0; i < numClusters; i++ {
		clusters[i] = []string{}
	}

	for i, s := range sentences {
		clusterIdx := i % numClusters
		clusters[clusterIdx] = append(clusters[clusterIdx], s)
	}

	var parts []string
	perCluster := max(1, int(float64(len(sentences))*cfg.TargetRatio)/numClusters)

	for _, cluster := range clusters {
		wordFreq := buildWordFrequency(cluster)
		bestIdx := 0
		bestScore := -1.0
		for i, s := range cluster {
			score := scoreSentence(s, wordFreq, cfg)
			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}
		parts = append(parts, strings.TrimSpace(cluster[bestIdx]))

		for i, s := range cluster {
			if i != bestIdx && strings.ContainsAny(s, "0123456789") {
				if cfg.PreserveNumbers && len(parts) < perCluster*2 {
					parts = append(parts, strings.TrimSpace(s))
				}
			}
		}
	}

	return Result{
		Text: strings.Join(parts, ". "),
	}
}

func compressSlidingWindow(text string, cfg Config) Result {
	targetChars := int(float64(len(text)) * cfg.TargetRatio)
	if targetChars < 100 {
		targetChars = min(len(text), 100)
	}

	start := len(text) - targetChars
	if start < 0 {
		start = 0
	}

	preserved := []string{}
	remaining := text[:start]
	window := text[start:]

	if cfg.PreserveHeaders {
		sentences := splitSentences(remaining)
		for _, s := range sentences[:min(3, len(sentences))] {
			trimmed := strings.TrimSpace(s)
			if len(trimmed) > 0 && len(trimmed) < 200 {
				preserved = append(preserved, trimmed)
			}
		}
	}

	result := ""
	if len(preserved) > 0 {
		result = strings.Join(preserved, ". ") + "\n\n... [middle content compressed] ...\n\n"
	}
	result += window

	return Result{
		Text:           result,
		PreservedParts: preserved,
	}
}

func compressHybrid(text string, cfg Config) Result {
	sentences := splitSentences(text)

	if len(sentences) <= 5 {
		return compressExtract(text, cfg)
	}

	ratio := cfg.TargetRatio
	if ratio > 0.5 {
		cfg.Algorithm = AlgoExtract
		return compressExtract(text, cfg)
	}

	if ratio > 0.2 {
		kwResult := compressKeyword(text, cfg)
		return kwResult
	}

	clusterResult := compressCluster(text, cfg)
	if len(clusterResult.Text) > cfg.MaxOutputChars {
		clusterResult.Text = clusterResult.Text[:cfg.MaxOutputChars]
	}
	return clusterResult
}

func splitSentences(text string) []string {
	raw := sentenceSplitRe.Split(text, -1)
	var sentences []string
	for _, s := range raw {
		trimmed := strings.TrimSpace(s)
		if len(trimmed) > 0 {
			sentences = append(sentences, trimmed)
		}
	}
	return sentences
}

func buildWordFrequency(sentences []string) map[string]int {
	freq := make(map[string]int)
	for _, s := range sentences {
		words := tokenize(s)
		for _, w := range words {
			if !stopWords[w] && len(w) > 1 {
				freq[w]++
			}
		}
	}
	return freq
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	var words []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	return words
}

func scoreSentence(sentence string, wordFreq map[string]int, cfg Config) float64 {
	words := tokenize(sentence)
	if len(words) == 0 {
		return 0
	}

	var score float64
	for _, w := range words {
		if f, ok := wordFreq[w]; ok {
			score += float64(f)
		}
	}

	score /= float64(len(words))

	if cfg.PreserveNumbers && numberRe.MatchString(sentence) {
		score *= 1.5
	}

	if len(sentence) < 20 || len(sentence) > 500 {
		score *= 0.7
	}

	return score
}

func ExtractKeywords(text string, topN int) []string {
	sentences := splitSentences(text)
	freq := buildWordFrequency(sentences)

	type wordScore struct {
		word  string
		score int
	}

	var scores []wordScore
	for w, f := range freq {
		scores = append(scores, wordScore{word: w, score: f})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	n := min(topN, len(scores))
	keywords := make([]string, n)
	for i := 0; i < n; i++ {
		keywords[i] = scores[i].word
	}
	return keywords
}

func makeCacheKey(text string, cfg Config) string {
	h := sha256.New()
	h.Write([]byte(text))
	h.Write([]byte(string(cfg.Algorithm)))
	h.Write([]byte(fmt.Sprintf("%f-%d", cfg.TargetRatio, cfg.MaxOutputChars)))
	return hex.EncodeToString(h.Sum(nil))[:16]
}
