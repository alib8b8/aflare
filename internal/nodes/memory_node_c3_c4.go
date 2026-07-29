// Copyright (c) 2026 llm-box Contributors
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
	"sort"
	"strings"
	"time"

	"github.com/alib8b8/llm-box/internal/memory"
	"github.com/alib8b8/llm-box/internal/nodes/core"
)

// C-3: memory↔graph linkage operations.
//
// linkKGMemory attaches KG entity names to a memory entry. The entities
// are passed as a comma-separated `kg_entities` param (matching the
// existing tags param convention). The linkage is stored on the
// SessionMemory and persisted with the session.
//
// expandKGMemory runs a search (vector or bag-of-words, whichever the
// session is configured for) and then expands the KG subgraph for the
// returned keys. The result is the search hits PLUS a list of related
// KG entity names, so the caller can surface graph context alongside
// memory context. This is the retrieval-time half of C-3.

// linkKGMemory implements operation=link_kg. It parses kg_entities,
// calls SessionMemory.LinkKGNode, and reports the resulting link set.
func (n *MemoryNode) linkKGMemory(session *memory.SessionMemory, key string, params map[string]string) (map[string]interface{}, error) {
	entitiesStr := strings.TrimSpace(getParam(params, "kg_entities", ""))
	if entitiesStr == "" {
		return nil, fmt.Errorf("kg_entities param is required for link_kg operation")
	}

	var entities []string
	for _, e := range strings.Split(entitiesStr, ",") {
		e = strings.TrimSpace(e)
		if e != "" && len(e) <= 256 {
			entities = append(entities, e)
		}
	}
	if len(entities) == 0 {
		return nil, fmt.Errorf("no valid entity names parsed from kg_entities")
	}

	if err := session.LinkKGNode(key, entities...); err != nil {
		return nil, err
	}

	links := session.GetKGLinks(key)
	return map[string]interface{}{
		"operation":   "link_kg",
		"key":         key,
		"linked":      entities,
		"total_links": len(links),
		"all_links":   links,
		"status":      "success",
	}, nil
}

// expandKGMemory implements operation=expand_kg. It performs a search
// and then expands the KG subgraph for the hit keys. The result
// includes both the memory hits and the related KG entity names.
func (n *MemoryNode) expandKGMemory(ctx context.Context, session *memory.SessionMemory, query, level string, topK int, threshold float64) (map[string]interface{}, error) {
	// Use the Ctx variant so the workflow's deadline/cancellation reaches
	// the embedder HTTP call inside Search (a vector search may issue a
	// network request to compute the query embedding).
	results := session.SearchCtx(ctx, query, level, topK, threshold)

	keys := make([]string, 0, len(results))
	for _, r := range results {
		keys = append(keys, r.Key)
	}
	kgEntities := session.ExpandKGSubgraph(keys)

	return map[string]interface{}{
		"operation":   "expand_kg",
		"query":       query,
		"level":       level,
		"memory_hits": len(results),
		"results":     results,
		"kg_entities": kgEntities,
		"kg_count":    len(kgEntities),
		"status":      "success",
	}, nil
}

// C-4: long-term memory compression with token-budget management.
//
// compressMemory selects long-term (and optionally medium-term) entries
// whose confidence is below min_confidence and whose combined token
// estimate exceeds token_budget, then calls an LLM to summarise them
// into a single compressed entry. The originals are deleted and the
// compressed entry is stored in their place with the same level.
//
// The token estimate uses a cheap heuristic (~4 chars per token) so we
// don't need a tokenizer dependency. This is intentionally conservative
// — overestimating tokens triggers compression earlier, which is safer
// than underestimating and blowing the context window later.
//
// If no LLM is configured the operation falls back to a deterministic
// concatenation+truncation compression so the operation always succeeds
// offline (used by tests and air-gapped deployments).

// compressMemory implements operation=compress.
func (n *MemoryNode) compressMemory(ctx context.Context, session *memory.SessionMemory, params map[string]string) (map[string]interface{}, error) {
	tokenBudget := parseIntSafe(getParam(params, "token_budget", "2000"), 2000)
	if tokenBudget < 100 {
		tokenBudget = 100
	}
	if tokenBudget > 100000 {
		tokenBudget = 100000
	}
	minConfidence := parseFloatSafe(getParam(params, "min_confidence", "0.5"), 0.5)
	if minConfidence < 0 || minConfidence > 1 {
		minConfidence = 0.5
	}
	// include_medium: when true, medium-term entries are also candidates.
	includeMedium := getParam(params, "include_medium", "false") == "true"
	targetLevel := getParam(params, "level", "long")

	// Gather candidate entries: low-confidence long-term (and optionally
	// medium-term). We pull a broad set via Search("", ...) which returns
	// all non-expired entries when threshold=0.
	candidates := session.SearchCtx(ctx, "", targetLevel, 10000, 0)
	if includeMedium {
		candidates = append(candidates, session.SearchCtx(ctx, "", "medium", 10000, 0)...)
	}

	// Filter by confidence and deduplicate by key.
	seen := make(map[string]bool)
	var toCompress []*memory.MemoryEntry
	var totalTokens int
	for i := range candidates {
		c := &candidates[i]
		if seen[c.Key] {
			continue
		}
		if c.Confidence >= minConfidence {
			continue
		}
		seen[c.Key] = true
		toCompress = append(toCompress, c)
		totalTokens += estimateTokens(c.Value)
	}

	if len(toCompress) == 0 {
		return map[string]interface{}{
			"operation":  "compress",
			"level":      targetLevel,
			"candidates": 0,
			"compressed": 0,
			"status":     "noop",
			"message":    "no low-confidence entries found to compress",
		}, nil
	}

	// If we're already under budget, no compression needed.
	if totalTokens <= tokenBudget {
		return map[string]interface{}{
			"operation":    "compress",
			"level":        targetLevel,
			"candidates":   len(toCompress),
			"compressed":   0,
			"total_tokens": totalTokens,
			"token_budget": tokenBudget,
			"status":       "noop",
			"message":      "total tokens already within budget",
		}, nil
	}

	// Sort candidates by confidence ascending so the lowest-confidence
	// (most forgettable) entries are compressed first. This preserves
	// higher-confidence entries verbatim.
	sort.SliceStable(toCompress, func(i, j int) bool {
		if toCompress[i].Confidence != toCompress[j].Confidence {
			return toCompress[i].Confidence < toCompress[j].Confidence
		}
		return toCompress[i].Key < toCompress[j].Key
	})

	// Greedily accumulate entries until we'd exceed ~2x the budget,
	// then compress that batch. The 2x slack gives the LLM room to
	// produce a summary that's roughly budget-sized.
	var batch []*memory.MemoryEntry
	var batchTokens int
	for _, c := range toCompress {
		batch = append(batch, c)
		batchTokens += estimateTokens(c.Value)
		if batchTokens >= tokenBudget*2 {
			break
		}
	}

	compressedText, compressErr := n.compressEntriesWithLLM(ctx, batch, tokenBudget, params)
	if compressErr != nil {
		// Fall back to deterministic truncation so the operation still
		// shrinks memory even when the LLM is unavailable.
		compressedText = deterministicCompress(batch, tokenBudget)
	}

	// Build the compressed entry's key from the first candidate's key
	// with a _compressed suffix, deduplicating with a timestamp.
	newKey := fmt.Sprintf("compressed_%d", time.Now().UnixNano())
	if len(batch) > 0 {
		newKey = batch[0].Key + "_compressed"
	}

	// Merge tags from all source entries.
	tagSet := make(map[string]bool)
	for _, c := range batch {
		for _, t := range c.Tags {
			tagSet[t] = true
		}
	}
	var mergedTags []string
	for t := range tagSet {
		mergedTags = append(mergedTags, t)
	}
	sort.Strings(mergedTags)

	// The compressed entry gets the highest confidence among sources
	// (it's a summary of verified content) and a source marker.
	compressedConfidence := 0.0
	for _, c := range batch {
		if c.Confidence > compressedConfidence {
			compressedConfidence = c.Confidence
		}
	}
	if compressedConfidence < minConfidence {
		// Boost to just above the threshold so it doesn't get
		// re-compressed on the next pass.
		compressedConfidence = minConfidence + 0.05
		if compressedConfidence > 1 {
			compressedConfidence = 1
		}
	}

	// Store the compressed replacement FIRST, then delete the originals.
	// This ordering avoids data loss if Store fails (e.g. embedder error,
	// context cancellation): the originals survive and the caller can retry.
	_, expiresAt, storeErr := session.StoreCtx(ctx, newKey, compressedText, targetLevel, "concept", mergedTags, 24*365, compressedConfidence, "compress")
	if storeErr != nil {
		// Store failed: keep the originals intact and surface the error
		// so the caller can decide whether to retry. No data is lost.
		return map[string]interface{}{
			"operation": "compress",
			"status":    "error",
			"error":     storeErr.Error(),
			"deleted":   []string{},
			"message":   "compression failed; originals preserved",
		}, storeErr
	}

	// Now safe to delete the originals.
	var deletedKeys []string
	for _, c := range batch {
		if err := session.Delete(c.Key); err == nil {
			deletedKeys = append(deletedKeys, c.Key)
		}
	}

	compressedTokens := estimateTokens(compressedText)
	return map[string]interface{}{
		"operation":         "compress",
		"level":             targetLevel,
		"candidates":        len(toCompress),
		"compressed":        len(batch),
		"deleted_keys":      deletedKeys,
		"new_key":           newKey,
		"expires_at":        expiresAt.Format(time.RFC3339),
		"original_tokens":   batchTokens,
		"compressed_tokens": compressedTokens,
		"token_budget":      tokenBudget,
		"retention_ratio":   safeDiv(compressedTokens, batchTokens),
		"llm_used":          compressErr == nil,
		"status":            "success",
	}, nil
}

// compressEntriesWithLLM asks an LLM to summarise the given entries
// into a single compressed memory entry of at most budget tokens. If
// no LLM is configured (no api_key in params or env) the LLM client
// will return an error and the caller falls back to deterministic
// compression.
func (n *MemoryNode) compressEntriesWithLLM(ctx context.Context, entries []*memory.MemoryEntry, budget int, params map[string]string) (string, error) {
	if len(entries) == 0 {
		return "", fmt.Errorf("no entries to compress")
	}

	var sb strings.Builder
	sb.WriteString("Compress the following memory entries into a single concise summary entry. ")
	sb.WriteString(fmt.Sprintf("Keep it under %d tokens. Preserve key facts, entities, and relations. ", budget))
	sb.WriteString("Discard redundant detail. Return only the compressed summary text, no JSON or markdown.\n\n")
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("### Entry %d (key=%s, confidence=%.2f)\n%s\n\n", i+1, e.Key, e.Confidence, e.Value))
	}

	provider := getParam(params, "provider", "openai")
	node := core.NewOpenAICompatibleNode(core.LLMNodeConfig{
		Name:            "memory_compress",
		DefaultModel:    defaultModelFor(provider),
		DefaultEndpoint: defaultEndpointFor(provider),
		EnvAPIKey:       strings.ToUpper(provider) + "_API_KEY",
		ProviderName:    provider,
	})

	callParams := copyParamsForLLM(params)
	callParams["system"] = "You are a memory compression assistant. You compress verbose memory entries into concise summaries while preserving key information."
	if _, ok := callParams["temperature"]; !ok {
		callParams["temperature"] = "0"
	}

	out, err := node.Execute(ctx, sb.String(), callParams)
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", fmt.Errorf("LLM returned empty compression")
	}
	return out, nil
}

// deterministicCompress is the offline fallback: concatenates entry
// values, truncating each so the total fits within budget tokens. Each
// entry is prefixed with its key so the summary stays attributable.
func deterministicCompress(entries []*memory.MemoryEntry, budget int) string {
	if len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	remaining := budget
	for _, e := range entries {
		if remaining <= 0 {
			break
		}
		header := fmt.Sprintf("[%s] ", e.Key)
		avail := remaining - estimateTokens(header) - 1
		if avail <= 0 {
			break
		}
		body := e.Value
		bodyTokens := estimateTokens(body)
		if bodyTokens > avail {
			// Truncate to avail tokens (~4*avail chars).
			maxChars := avail * 4
			if maxChars > len(body) {
				maxChars = len(body)
			}
			body = body[:maxChars] + "..."
		}
		sb.WriteString(header)
		sb.WriteString(body)
		sb.WriteString("\n")
		remaining -= estimateTokens(header) + estimateTokens(body) + 1
	}
	return sb.String()
}

// estimateTokens returns a rough token count (~4 chars per token). This
// is the standard heuristic used by tiktoken-like estimators when a
// real tokenizer isn't available. Good enough for budget gating.
func estimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return (len(s) + 3) / 4
}

func safeDiv(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}
