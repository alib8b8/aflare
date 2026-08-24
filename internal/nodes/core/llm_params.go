// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌‌‌​​‌​​‌​‌​‌​‌‌​​​​​​‌​‌​‌​​‌‌​​‌‌‌‌​‌​‌‌‌​‌​‌​‌​‌​‌​‌​​‌​‌​​​​​​​​​​​​​​​​​​​​​​‌‌​​​​​‌​‌‌​⁠
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

package core

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// applyLLMRequestParams populates the B-1 extended fields of req from the
// params map. It is strict: malformed numeric/JSON values return an error
// rather than being silently dropped, so callers can trust that a
// populated field reached the provider.
//
// Recognised param keys (all string-valued, consistent with the Node API):
//
//	temperature        - float in [0, 2]
//	max_tokens         - positive int
//	top_p              - float in (0, 1]
//	frequency_penalty  - float in [-2, 2]
//	presence_penalty   - float in [-2, 2]
//	stop               - comma-separated list, e.g. "\n,END"
//	seed               - int
//	response_format    - "json_object" or "json_schema:<schema_json>"
//	tools              - JSON array of ToolDefinition
//	tool_choice        - "none", "auto", or a JSON object
//	user               - end-user identifier string
func applyLLMRequestParams(req *LLMRequest, params map[string]string) error {
	if v, ok := params["temperature"]; ok && v != "" {
		f, err := parseFloatParam(v, "temperature", 0, 2)
		if err != nil {
			return err
		}
		req.Temperature = f
	}
	if v, ok := params["max_tokens"]; ok && v != "" {
		n, err := parseIntParam(v, "max_tokens")
		if err != nil {
			return err
		}
		if n <= 0 {
			return fmt.Errorf("max_tokens must be positive, got %d", n)
		}
		// Cap at a sane upper bound to prevent abuse; providers reject
		// extreme values anyway, but we want a friendly error rather
		// than an opaque 4xx from upstream.
		if n > 128000 {
			return fmt.Errorf("max_tokens %d exceeds upper bound 128000", n)
		}
		req.MaxTokens = n
	}
	if v, ok := params["top_p"]; ok && v != "" {
		f, err := parseFloatParam(v, "top_p", 0, 1)
		if err != nil {
			return err
		}
		req.TopP = f
	}
	if v, ok := params["frequency_penalty"]; ok && v != "" {
		f, err := parseFloatParam(v, "frequency_penalty", -2, 2)
		if err != nil {
			return err
		}
		req.FrequencyPenalty = f
	}
	if v, ok := params["presence_penalty"]; ok && v != "" {
		f, err := parseFloatParam(v, "presence_penalty", -2, 2)
		if err != nil {
			return err
		}
		req.PresencePenalty = f
	}
	if v, ok := params["stop"]; ok && v != "" {
		// Split on comma; each entry used verbatim. An empty entry (e.g.
		// from a trailing comma) is dropped to avoid sending an empty
		// stop string, which some providers reject.
		parts := strings.Split(v, ",")
		for _, p := range parts {
			if p != "" {
				req.Stop = append(req.Stop, p)
			}
		}
		// OpenAI allows at most 4 stop sequences; other providers have
		// similar limits. Cap at a small bound to avoid an opaque 4xx.
		if len(req.Stop) > 16 {
			return fmt.Errorf("too many stop sequences: %d (max 16)", len(req.Stop))
		}
	}
	if v, ok := params["seed"]; ok && v != "" {
		n, err := parseIntParam(v, "seed")
		if err != nil {
			return err
		}
		req.Seed = &n
	}
	if v, ok := params["response_format"]; ok && v != "" {
		rf, err := parseResponseFormatParam(v)
		if err != nil {
			return err
		}
		req.ResponseFormat = rf
	}
	if v, ok := params["tools"]; ok && v != "" {
		var tools []ToolDefinition
		if err := json.Unmarshal([]byte(v), &tools); err != nil {
			return fmt.Errorf("tools must be a JSON array of tool definitions: %w", err)
		}
		if len(tools) == 0 {
			return fmt.Errorf("tools array must not be empty")
		}
		req.Tools = tools
	}
	if v, ok := params["tool_choice"]; ok && v != "" {
		// Accept bare "none"/"auto" strings or a JSON object.
		if v == "none" || v == "auto" {
			// strconv.Quote emits a valid JSON string literal, so v can
			// never break out of the quoted context.
			req.ToolChoice = json.RawMessage(strconv.Quote(v))
		} else {
			// Validate it parses as JSON; store raw.
			var probe json.RawMessage
			if err := json.Unmarshal([]byte(v), &probe); err != nil {
				return fmt.Errorf("tool_choice must be 'none', 'auto', or a JSON object: %w", err)
			}
			req.ToolChoice = json.RawMessage(v)
		}
	}
	if v, ok := params["user"]; ok && v != "" {
		req.User = v
	}
	return nil
}

// parseResponseFormatParam parses the response_format param value.
// Accepted forms:
//
//	json_object                            -> {"type":"json_object"}
//	json_schema:{"name":"...","schema":{}} -> {"type":"json_schema","json_schema":{...}}
//	{"type":"json_object"}                  (raw JSON passed through)
func parseResponseFormatParam(v string) (*ResponseFormat, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	// Raw JSON object form.
	if strings.HasPrefix(v, "{") {
		var rf ResponseFormat
		if err := json.Unmarshal([]byte(v), &rf); err != nil {
			return nil, fmt.Errorf("response_format JSON invalid: %w", err)
		}
		if rf.Type == "" {
			return nil, fmt.Errorf("response_format JSON missing 'type'")
		}
		return &rf, nil
	}
	// Keyword form.
	if v == "json_object" {
		return &ResponseFormat{Type: "json_object"}, nil
	}
	if strings.HasPrefix(v, "json_schema:") {
		rest := strings.TrimSpace(v[len("json_schema:"):])
		var schema map[string]interface{}
		if err := json.Unmarshal([]byte(rest), &schema); err != nil {
			return nil, fmt.Errorf("response_format json_schema payload invalid: %w", err)
		}
		return &ResponseFormat{Type: "json_schema", JSONSchema: schema}, nil
	}
	return nil, fmt.Errorf("response_format must be 'json_object', 'json_schema:<json>', or a raw JSON object")
}

func parseFloatParam(v, name string, min, max float64) (float64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number, got %q: %w", name, v, err)
	}
	if f < min || f > max {
		return 0, fmt.Errorf("%s must be in [%g, %g], got %g", name, min, max, f)
	}
	return f, nil
}

func parseIntParam(v, name string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q: %w", name, v, err)
	}
	return n, nil
}
