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

// memory_node_harness.go implements the memory harness_search operation —
// the aflare encoding of the MemHarness pattern (arXiv:2607.28272, "memory is
// reconstructed, not replayed").
//
// MemHarness's five stages (retrieve → critique applicability → reconstruct
// or discard → act) map onto aflare's determinism boundary like this:
//
//	retrieve   → this node, deterministically (vector/keyword search with
//	             each candidate's full SOURCE STATE: type, level, tags,
//	             source, confidence, created_at, score)
//	critique   → an explicit downstream LLM step, using the emitted
//	             critique_prompt (the model judges each candidate against the
//	             CURRENT task state: keep / rewrite / discard)
//	reconstruct→ the same LLM step outputs task-grounded guidance, or
//	             <EMPTY> when nothing survives critique — in which case the
//	             workflow proceeds on pure reasoning, which MemHarness shows
//	             beats replaying stale context
//	act        → the remaining workflow steps, consuming only the guidance
//
// Keeping the LLM out of this node preserves aflare's auditability: the
// retrieval and its inputs are deterministic and tamper-evident, while the
// nondeterministic judgement stays an explicit, retryable, schema-checkable
// workflow step (pairs naturally with step-level output_schema).

package nodes

import (
	"context"
	"fmt"
	"strings"

	"github.com/alib8b8/aflare/internal/memory"
)

// harnessCritiqueSystemPrompt is the system prompt for the downstream LLM
// critique step. It instructs the model to judge applicability against the
// current task rather than replay, and defines the output contract.
const harnessCritiqueSystemPrompt = `You are the critique stage of a memory harness. You receive retrieved memory candidates, each annotated with its source state (when it was recorded, at what confidence, in what context). Your job is NOT to recall them verbatim — it is to decide, for the CURRENT task only, what still helps.

For each candidate:
- DISCARD it if its source state is stale or mismatched with the current task (wrong context, outdated facts, superseded decisions).
- REWRITE it if the underlying signal is still useful but the wording is tied to the old context: re-express only what applies now.
- KEEP it only if it applies as-is.

Then produce guidance for the task: a concise reconstruction built exclusively from what survived. If nothing survives, the guidance is exactly <EMPTY> — do not invent filler; acting without memory is a valid outcome.

Return ONLY a JSON object:
{"verdicts":[{"key":"<candidate key>","verdict":"keep|rewrite|discard","reason":"<one line>"}],"guidance":"<task-grounded guidance, or <EMPTY>"}`

// Note: the literal <EMPTY> marker inside the JSON example must survive
// templating; do not wrap this constant in fmt.Sprintf-style processing.

// harnessSearch retrieves memory candidates together with their source state
// and emits a ready-to-use critique prompt. The output JSON is:
//
//	{
//	  "operation": "harness_search",
//	  "query": "...",
//	  "count": N,
//	  "candidates": [
//	    {"key":"...","value":"...","source_state":{"type":"...","level":"...",
//	     "tags":[...],"source":"...","confidence":0.0,"created_at":"...",
//	     "score":0.0}}
//	  ],
//	  "critique_prompt": "...",
//	  "status": "success"
//	}
//
// The critique_prompt bundles the system prompt above with the candidates
// and the current query, so a workflow can feed it directly to any LLM node.
func (n *MemoryNode) harnessSearch(ctx context.Context, session *memory.SessionMemory, query, level string, topK int, threshold float64) (map[string]interface{}, error) {
	// Use the Ctx variants so the workflow's deadline/cancellation
	// reaches the embedder calls inside both searches (vector retrieval
	// may issue a network request to compute embeddings).
	results := session.SearchCtx(ctx, query, level, topK, threshold)

	// Merge the persistent store (MemoryCapability's cross-session data),
	// same as searchMemory, so the critique stage sees everything the
	// runtime knows about the query.
	persistentStore := memory.GetPersistentStore()
	seenKeys := make(map[string]bool, len(results))
	for _, r := range results {
		seenKeys[r.Key] = true
	}
	for _, pe := range persistentStore.SearchCtx(ctx, query, topK) {
		if seenKeys[pe.Key] {
			continue
		}
		seenKeys[pe.Key] = true
		results = append(results, memory.MemoryEntry{
			Key:        pe.Key,
			Value:      pe.Value,
			Type:       pe.Category,
			Level:      "long",
			Confidence: 0.9,
			Source:     "persistent",
		})
	}

	candidates := make([]map[string]interface{}, 0, len(results))
	var candLines strings.Builder
	for i, r := range results {
		candidates = append(candidates, map[string]interface{}{
			"key":   r.Key,
			"value": r.Value,
			"source_state": map[string]interface{}{
				"type":       r.Type,
				"level":      r.Level,
				"tags":       r.Tags,
				"source":     r.Source,
				"confidence": r.Confidence,
				"created_at": r.CreatedAt,
				"score":      r.Score,
			},
		})
		fmt.Fprintf(&candLines, "candidate[%d] key=%s\n  value: %s\n  source_state: type=%s level=%s confidence=%.2f created_at=%s score=%.2f\n",
			i, memory.FenceValue(r.Key), memory.FenceValue(r.Value), memory.FenceValue(r.Type), r.Level, r.Confidence, r.CreatedAt.Format("2006-01-02"), r.Score)
	}

	critiquePrompt := fmt.Sprintf("%s\n\n# Retrieved memory candidates\n\n%s\n\n# Current task\n\n%s",
		harnessCritiqueSystemPrompt, strings.Trim(candLines.String(), "\n"), query)

	return map[string]interface{}{
		"operation":       "harness_search",
		"query":           query,
		"count":           len(candidates),
		"candidates":      candidates,
		"critique_prompt": critiquePrompt,
		"status":          "success",
	}, nil
}
