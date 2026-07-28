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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alib8b8/llm-box/internal/memory"
	"github.com/alib8b8/llm-box/internal/nodes/core"
)

// C-2: LLM-driven knowledge graph extraction.
//
// The rule-based extractEntitiesSimple/extractRelationsSimple path in
// knowledge_graph.go is intentionally dumb — it matches capitalised
// words and a handful of "Entity: name" line prefixes. It exists so the
// node works offline, but the entities/relations it produces are noisy.
//
// extract_llm replaces it when an LLM is available. The flow is:
//
//  1. Build a prompt that asks the model to return a JSON object of the
//     shape {"entities":[{"name","type","properties"}], "relations":
//     [{"from","to","relation","confidence"}]}.
//  2. Call the LLM via core.NewOpenAICompatibleNode with
//     response_format=json_object so the model is forced to return JSON.
//  3. Parse the JSON defensively (the model may wrap it in fences or
//     add prose despite json_object mode).
//  4. Merge entities/relations into the KnowledgeGraph. If graph_path
//     is given, load the existing graph first so extraction is additive.
//  5. Optionally link extracted entities back to a memory entry (C-3):
//     if memory_key and session_id params are supplied, the extracted
//     entity names are attached to that memory entry via LinkKGNode.
//
// The action reuses B-2 telemetry (the OpenAICompatibleNode publishes
// LLM call metrics through the context sink) and B-4 schema validation
// is NOT reused here because the extraction schema is loose — we'd
// rather accept a partial extraction than reject the whole response.

// kgExtractionSchema is the JSON Schema we describe to the model in the
// prompt. We don't enforce it server-side (we parse defensively), but
// describing it improves compliance.
const kgExtractionSchema = `{
  "type": "object",
  "required": ["entities", "relations"],
  "properties": {
    "entities": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["name"],
        "properties": {
          "name": {"type": "string"},
          "type": {"type": "string"},
          "properties": {"type": "object"}
        }
      }
    },
    "relations": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["from", "to", "relation"],
        "properties": {
          "from": {"type": "string"},
          "to": {"type": "string"},
          "relation": {"type": "string"},
          "confidence": {"type": "number"}
        }
      }
    }
  }
}`

// extractLLMFromText calls an LLM to extract a knowledge graph from
// text and merges the result into the graph at graphPath (if given).
//
// Params consumed:
//   - input:             the source text (passed as the user message)
//   - graph_path:        optional path to an existing graph to merge into
//   - format:            output format (json|markdown|mermaid)
//   - provider/model/api_key/endpoint/temperature/max_tokens: LLM config
//   - language:          optional hint ("en"/"zh") for the prompt language
//   - session_id+memory_key: optional C-3 linkage target
func (n *KnowledgeGraphNode) extractLLMFromText(ctx context.Context, input, graphPath, format string, params map[string]string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("knowledge_graph extract_llm: input text is required")
	}

	provider := getParam(params, "provider", "openai")
	node := core.NewOpenAICompatibleNode(core.LLMNodeConfig{
		Name:            "knowledge_graph_extract_llm",
		DefaultModel:    defaultModelFor(provider),
		DefaultEndpoint: defaultEndpointFor(provider),
		EnvAPIKey:       strings.ToUpper(provider) + "_API_KEY",
		ProviderName:    provider,
	})

	systemPrompt := buildKGExtractionSystemPrompt(getParam(params, "language", "en"))
	callParams := copyParamsForLLM(params)
	callParams["system"] = systemPrompt
	callParams["response_format"] = "json_object"
	if _, ok := callParams["temperature"]; !ok {
		callParams["temperature"] = "0"
	}

	raw, err := node.Execute(ctx, input, callParams)
	if err != nil {
		return "", fmt.Errorf("knowledge_graph extract_llm: LLM call failed: %w", err)
	}

	extraction, err := parseKGExtraction(raw)
	if err != nil {
		return "", fmt.Errorf("knowledge_graph extract_llm: failed to parse LLM output: %w", err)
	}

	// Build the resulting graph. If graphPath is given, load the
	// existing graph and merge; otherwise start fresh.
	var kg *KnowledgeGraph
	if graphPath != "" {
		if existing, loadErr := LoadKnowledgeGraph(graphPath); loadErr == nil {
			kg = existing
		} else {
			kg = NewKnowledgeGraph()
		}
	} else {
		kg = NewKnowledgeGraph()
	}

	mergedEntities := 0
	mergedRelations := 0
	for _, e := range extraction.Entities {
		name := strings.TrimSpace(e.Name)
		if name == "" {
			continue
		}
		props := make(map[string]string, len(e.Properties))
		for k, v := range e.Properties {
			props[k] = fmt.Sprintf("%v", v)
		}
		kg.AddEntity(name, e.Type, props)
		mergedEntities++
	}
	for _, r := range extraction.Relations {
		from := strings.TrimSpace(r.From)
		to := strings.TrimSpace(r.To)
		rel := strings.TrimSpace(r.Relation)
		if from == "" || to == "" || rel == "" {
			continue
		}
		conf := r.Confidence
		if conf <= 0 || conf > 1 {
			conf = 0.7
		}
		kg.AddRelation(from, to, rel, conf)
		// Ensure both endpoints exist as entities so the graph is
		// well-formed even if the model only emitted relations.
		kg.AddEntity(from, "", nil)
		kg.AddEntity(to, "", nil)
		mergedRelations++
	}

	if graphPath != "" {
		if err := kg.Save(graphPath); err != nil {
			return "", fmt.Errorf("knowledge_graph extract_llm: failed to save graph: %w", err)
		}
	}

	// C-3: optionally link the extracted entity names back to a memory
	// entry so later memory searches can expand the KG subgraph.
	linkInfo := ""
	if memKey := strings.TrimSpace(params["memory_key"]); memKey != "" {
		sessionID := getParam(params, "session_id", "default")
		session := memory.GetSession(sessionID)
		entityNames := make([]string, 0, len(extraction.Entities))
		for _, e := range extraction.Entities {
			if n := strings.TrimSpace(e.Name); n != "" {
				entityNames = append(entityNames, n)
			}
		}
		for _, r := range extraction.Relations {
			for _, n := range []string{r.From, r.To} {
				if n = strings.TrimSpace(n); n != "" {
					entityNames = append(entityNames, n)
				}
			}
		}
		if len(entityNames) > 0 {
			if linkErr := session.LinkKGNode(memKey, entityNames...); linkErr != nil {
				// Link failure is non-fatal: we still return the graph.
				linkInfo = fmt.Sprintf(" (kg link skipped: %v)", linkErr)
			} else {
				linkInfo = fmt.Sprintf(" (linked %d entities to memory key %q)", len(entityNames), memKey)
			}
		}
	}

	out := formatGraphOutput(kg, format)
	if mergedEntities > 0 || mergedRelations > 0 {
		// Prepend an extraction summary line for non-JSON formats so
		// callers can see at a glance how much was extracted. JSON
		// output stays clean for programmatic consumers.
		if format != "json" {
			summary := fmt.Sprintf("## LLM Knowledge Graph Extraction%s\n\n- Extracted entities: %d\n- Extracted relations: %d\n\n", linkInfo, mergedEntities, mergedRelations)
			out = summary + out
		}
	}
	return out, nil
}

// kgExtraction is the parsed shape of the LLM's JSON response.
type kgExtraction struct {
	Entities  []kgExtractedEntity `json:"entities"`
	Relations []kgExtractedRel    `json:"relations"`
}

type kgExtractedEntity struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

type kgExtractedRel struct {
	From       string  `json:"from"`
	To         string  `json:"to"`
	Relation   string  `json:"relation"`
	Confidence float64 `json:"confidence"`
}

// parseKGExtraction extracts the JSON object from raw and decodes it.
// The model may wrap output in ```json fences or prepend prose despite
// json_object mode, so we try plain decode first, then fence stripping,
// then a brace-matching scan as a last resort.
func parseKGExtraction(raw string) (*kgExtraction, error) {
	// Fast path: clean JSON.
	var ex kgExtraction
	if err := json.Unmarshal([]byte(raw), &ex); err == nil {
		return &ex, nil
	}

	// Strip ```json ... ``` fences.
	trimmed := stripJSONFences(raw)
	if trimmed != raw {
		if err := json.Unmarshal([]byte(trimmed), &ex); err == nil {
			return &ex, nil
		}
	}

	// Brace match: find the first '{' and its matching '}'.
	start := strings.IndexByte(trimmed, '{')
	if start < 0 {
		return nil, fmt.Errorf("no JSON object found in response")
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(trimmed); i++ {
		c := trimmed[i]
		if esc {
			esc = false
			continue
		}
		if c == '\\' {
			esc = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				candidate := trimmed[start : i+1]
				if err := json.Unmarshal([]byte(candidate), &ex); err == nil {
					return &ex, nil
				}
				break
			}
		}
	}
	return nil, fmt.Errorf("could not parse JSON from LLM response (first 200 bytes: %q)", truncateForLog(raw, 200))
}

// stripJSONFences removes ```json / ``` fences if present.
func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	// Drop the opening fence line.
	if nl := strings.IndexByte(s, '\n'); nl >= 0 {
		s = s[nl+1:]
	} else {
		return s
	}
	// Drop the closing fence.
	if idx := strings.LastIndex(s, "```"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// buildKGExtractionSystemPrompt constructs the system prompt that tells
// the model what to extract and how to format it. language "zh" makes
// the instructions Chinese; otherwise English.
func buildKGExtractionSystemPrompt(language string) string {
	var sb strings.Builder
	if language == "zh" {
		sb.WriteString("你是一个知识图谱抽取助手。从用户提供的文本中抽取实体和它们之间的关系，并以 JSON 格式返回。\n\n")
		sb.WriteString("只返回一个 JSON 对象，不要添加任何解释或 Markdown 代码块。JSON 结构如下：\n")
	} else {
		sb.WriteString("You are a knowledge-graph extraction assistant. Extract entities and their relations from the user's text and return them as a JSON object.\n\n")
		sb.WriteString("Return ONLY a JSON object, no explanations, no Markdown code fences. The JSON shape is:\n")
	}
	sb.WriteString(kgExtractionSchema)
	sb.WriteString("\n\n")
	if language == "zh" {
		sb.WriteString("要求：\n- name/from/to/relation 必须是字符串。\n- 实体 type 举例：Person, Organization, Location, Concept, Technology, Event。\n- relation 用小写蛇形命名，例如 works_for, located_in, created_by。\n- confidence 在 0.0 到 1.0 之间。\n- 如果没有关系，relations 返回空数组 []。\n")
	} else {
		sb.WriteString("Requirements:\n- name/from/to/relation must be strings.\n- Example entity types: Person, Organization, Location, Concept, Technology, Event.\n- Use lower_snake_case for relations, e.g. works_for, located_in, created_by.\n- confidence is between 0.0 and 1.0.\n- If there are no relations, return an empty array [] for relations.\n")
	}
	return sb.String()
}
