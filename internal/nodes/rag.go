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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type RAGNode struct{}

func init() {
	Register(&RAGNode{})
}

func (n *RAGNode) Name() string {
	return "rag"
}

func (n *RAGNode) Description() string {
	return "Retrieval Augmented Generation: chunk, search, and assemble context from documents"
}

func (n *RAGNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "rag",
		Description: "Retrieval Augmented Generation node - chunk documents, search by query, and assemble context",
		Input:       "string - the query to search for",
		Output:      "string - assembled context from relevant document chunks",
		Params: []ParamSchema{
			{Name: "source", Type: "string", Description: "Source: file path, directory path, or text content", Required: true},
			{Name: "source_type", Type: "string", Description: "Type of source: file, dir, text (default: auto)", Required: false, Default: "auto"},
			{Name: "chunk_size", Type: "string", Description: "Chunk size in characters (default: 1000)", Required: false, Default: "1000"},
			{Name: "chunk_overlap", Type: "string", Description: "Chunk overlap in characters (default: 200)", Required: false, Default: "200"},
			{Name: "top_k", Type: "string", Description: "Number of top chunks to retrieve (default: 5)", Required: false, Default: "5"},
			{Name: "search_method", Type: "string", Description: "Search method: keyword, hybrid (default: keyword)", Required: false, Default: "keyword"},
			{Name: "include_metadata", Type: "string", Description: "Include chunk metadata in output (default: true)", Required: false, Default: "true"},
		},
	}
}

type Chunk struct {
	Text     string
	Source   string
	Index    int
	Score    float64
	Position int
}

func (n *RAGNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	source := params["source"]
	if source == "" {
		return "", fmt.Errorf("source parameter is required")
	}
	sourceType := getParam(params, "source_type", "auto")
	chunkSizeStr := getParam(params, "chunk_size", "1000")
	chunkOverlapStr := getParam(params, "chunk_overlap", "200")
	topKStr := getParam(params, "top_k", "5")
	searchMethod := getParam(params, "search_method", "keyword")
	includeMetadata := getParam(params, "include_metadata", "true") == "true"

	chunkSize := 1000
	if _, err := fmt.Sscanf(chunkSizeStr, "%d", &chunkSize); err != nil {
		// keep default value on parse failure
	}
	if chunkSize < 100 {
		chunkSize = 100
	}

	chunkOverlap := 200
	if _, err := fmt.Sscanf(chunkOverlapStr, "%d", &chunkOverlap); err != nil {
		// keep default value on parse failure
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 2
	}

	topK := 5
	if _, err := fmt.Sscanf(topKStr, "%d", &topK); err != nil {
		// keep default value on parse failure
	}
	if topK < 1 {
		topK = 1
	}

	query := strings.TrimSpace(input)
	if query == "" {
		return "", fmt.Errorf("query (input) cannot be empty")
	}

	var documents []struct {
		Text   string
		Source string
	}

	switch sourceType {
	case "text":
		documents = append(documents, struct {
			Text   string
			Source string
		}{Text: source, Source: "inline_text"})
	case "file":
		safePath, err := validateReadPath(source)
		if err != nil {
			return "", fmt.Errorf("invalid file path: %w", err)
		}
		content, err := os.ReadFile(safePath)
		if err != nil {
			return "", fmt.Errorf("failed to read file %s: %w", safePath, err)
		}
		documents = append(documents, struct {
			Text   string
			Source string
		}{Text: string(content), Source: safePath})
	case "dir":
		safePath, err := validateReadPath(source)
		if err != nil {
			return "", fmt.Errorf("invalid directory path: %w", err)
		}
		docs, err := loadDirectory(safePath)
		if err != nil {
			return "", fmt.Errorf("failed to load directory %s: %w", safePath, err)
		}
		documents = docs
	default:
		if _, err := os.Stat(source); err == nil {
			safePath, validateErr := validateReadPath(source)
			if validateErr != nil {
				return "", fmt.Errorf("invalid path: %w", validateErr)
			}
			if info, _ := os.Stat(safePath); info.IsDir() {
				docs, err := loadDirectory(safePath)
				if err != nil {
					return "", fmt.Errorf("failed to load directory: %w", err)
				}
				documents = docs
			} else {
				content, err := os.ReadFile(safePath)
				if err != nil {
					return "", fmt.Errorf("failed to read file: %w", err)
				}
				documents = append(documents, struct {
					Text   string
					Source string
				}{Text: string(content), Source: safePath})
			}
		} else {
			documents = append(documents, struct {
				Text   string
				Source string
			}{Text: source, Source: "inline_text"})
		}
	}

	var allChunks []Chunk
	for _, doc := range documents {
		chunks := chunkText(doc.Text, doc.Source, chunkSize, chunkOverlap)
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) == 0 {
		return "No content found in source.", nil
	}

	var scored []Chunk
	switch searchMethod {
	case "hybrid":
		scored = hybridSearch(query, allChunks)
	default:
		scored = keywordSearch(query, allChunks)
	}

	if topK > len(scored) {
		topK = len(scored)
	}
	topChunks := scored[:topK]

	result := assembleContext(topChunks, query, includeMetadata)
	return result, nil
}

func loadDirectory(dirPath string) ([]struct {
	Text   string
	Source string
}, error) {
	var documents []struct {
		Text   string
		Source string
	}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		supported := map[string]bool{
			".txt": true, ".md": true, ".markdown": true,
			".json": true, ".yaml": true, ".yml": true,
			".csv": true, ".log": true, ".py": true,
			".go": true, ".js": true, ".ts": true,
			".html": true, ".css": true, ".java": true,
			".cpp": true, ".c": true, ".h": true,
			".rs": true, ".rb": true, ".php": true,
		}
		if !supported[ext] {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil //nolint:nilerr // Walk callback: skip unreadable file, continue traversal
		}
		documents = append(documents, struct {
			Text   string
			Source string
		}{Text: string(content), Source: path})
		return nil
	})

	return documents, err
}

func chunkText(text, source string, size, overlap int) []Chunk {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	runes := []rune(text)
	var chunks []Chunk
	start := 0
	index := 0

	for start < len(runes) {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}

		chunkText := string(runes[start:end])
		chunks = append(chunks, Chunk{
			Text:     chunkText,
			Source:   source,
			Index:    index,
			Position: start,
		})

		index++
		if end >= len(runes) {
			break
		}
		start = end - overlap
		if start < 0 {
			start = 0
		}
	}

	return chunks
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	re := regexp.MustCompile(`\w+`)
	tokens := re.FindAllString(text, -1)
	return tokens
}

func keywordSearch(query string, chunks []Chunk) []Chunk {
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		for i := range chunks {
			chunks[i].Score = 0
		}
		return chunks
	}

	querySet := make(map[string]bool)
	for _, t := range queryTokens {
		querySet[t] = true
	}

	type scoredChunk struct {
		chunk Chunk
		score float64
	}

	var scored []scoredChunk
	for _, chunk := range chunks {
		chunkTokens := tokenize(chunk.Text)
		chunkFreq := make(map[string]int)
		for _, t := range chunkTokens {
			chunkFreq[t]++
		}

		var score float64
		for token := range querySet {
			if freq, ok := chunkFreq[token]; ok {
				score += float64(freq)
			}
		}

		for _, qToken := range queryTokens {
			for _, cToken := range chunkTokens {
				if strings.Contains(cToken, qToken) || strings.Contains(qToken, cToken) {
					score += 0.5
				}
			}
		}

		if score > 0 {
			sc := chunk
			sc.Score = score
			scored = append(scored, scoredChunk{chunk: sc, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	result := make([]Chunk, len(scored))
	for i, sc := range scored {
		result[i] = sc.chunk
	}
	return result
}

func hybridSearch(query string, chunks []Chunk) []Chunk {
	keywordResults := keywordSearch(query, chunks)
	phraseResults := phraseSearch(query, chunks)

	scoreMap := make(map[int]float64)
	for i, chunk := range keywordResults {
		scoreMap[chunk.Index] += float64(len(keywordResults)-i) / float64(len(keywordResults))
	}
	for i, chunk := range phraseResults {
		scoreMap[chunk.Index] += float64(len(phraseResults)-i) / float64(len(phraseResults)) * 1.5
	}

	type scoredChunk struct {
		chunk Chunk
		score float64
	}

	var scored []scoredChunk
	for _, chunk := range chunks {
		if score, ok := scoreMap[chunk.Index]; ok && score > 0 {
			sc := chunk
			sc.Score = score
			scored = append(scored, scoredChunk{chunk: sc, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	result := make([]Chunk, len(scored))
	for i, sc := range scored {
		result[i] = sc.chunk
	}
	return result
}

func phraseSearch(query string, chunks []Chunk) []Chunk {
	queryLower := strings.ToLower(query)
	type scoredChunk struct {
		chunk Chunk
		score float64
	}

	var scored []scoredChunk
	for _, chunk := range chunks {
		chunkLower := strings.ToLower(chunk.Text)
		var score float64

		if strings.Contains(chunkLower, queryLower) {
			score += 5.0
		}

		words := strings.Fields(queryLower)
		if len(words) > 1 {
			matches := 0
			for _, w := range words {
				if len(w) > 2 && strings.Contains(chunkLower, w) {
					matches++
				}
			}
			score += float64(matches) * 0.5
		}

		if score > 0 {
			sc := chunk
			sc.Score = score
			scored = append(scored, scoredChunk{chunk: sc, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	result := make([]Chunk, len(scored))
	for i, sc := range scored {
		result[i] = sc.chunk
	}
	return result
}

func assembleContext(chunks []Chunk, query string, includeMetadata bool) string {
	if len(chunks) == 0 {
		return fmt.Sprintf("No relevant chunks found for query: %s", query)
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Context for query: %s\n", query))
	builder.WriteString(fmt.Sprintf("Retrieved %d relevant chunks:\n\n", len(chunks)))

	for i, chunk := range chunks {
		if includeMetadata {
			builder.WriteString(fmt.Sprintf("--- Chunk %d (score: %.2f) ---\n", i+1, chunk.Score))
			builder.WriteString(fmt.Sprintf("Source: %s (chunk #%d)\n", chunk.Source, chunk.Index+1))
		} else {
			builder.WriteString(fmt.Sprintf("--- Chunk %d ---\n", i+1))
		}
		builder.WriteString(chunk.Text)
		builder.WriteString("\n\n")
	}

	return builder.String()
}
