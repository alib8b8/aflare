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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

// StructuredOutputNode calls an LLM and returns JSON that conforms to a
// caller-supplied JSON Schema. It builds on the B-1 response_format
// extension (json_object mode) and adds a local schema validator so we
// can catch malformed output even when the provider does not natively
// enforce json_schema. When validation fails, the node retries up to
// max_retries times, feeding the validation error back into the prompt
// so the model can correct itself.
//
// The node intentionally uses json_object (not json_schema) because
// provider support for json_schema is uneven. The schema is enforced
// locally via validateAgainstSchema, which implements the subset of
// JSON Schema draft-07 keywords we need (type, required, properties,
// items, enum, min/max*, additionalProperties:false).
type StructuredOutputNode struct{}

func init() {
	Register(&StructuredOutputNode{})
}

func (n *StructuredOutputNode) Name() string { return "structured_output" }

func (n *StructuredOutputNode) Description() string {
	return "Call an LLM and return JSON validated against a caller-supplied JSON Schema"
}

func (n *StructuredOutputNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "structured_output",
		Description: "LLM-driven structured output with local JSON Schema validation and self-correction retries",
		Input:       "string - user instruction describing what to produce",
		Output:      "string - JSON string validated against schema",
		Params: []ParamSchema{
			{Name: "schema", Type: "string", Description: "JSON Schema (draft-07 subset) the output must conform to. Must be a JSON object with \"type\":\"object\" at the root.", Required: true},
			{Name: "schema_name", Type: "string", Description: "Optional schema name shown to the model in the prompt (default: \"output\")", Required: false, Default: "output"},
			{Name: "provider", Type: "string", Description: "Provider name for default model/endpoint resolution (default: openai)", Required: false, Default: "openai"},
			{Name: "model", Type: "string", Description: "Model name (default: provider default)", Required: false},
			{Name: "api_key", Type: "string", Description: "LLM API key (or set <PROVIDER>_API_KEY env var)", Required: false},
			{Name: "endpoint", Type: "string", Description: "LLM API base URL (default: provider default endpoint)", Required: false},
			{Name: "system", Type: "string", Description: "Additional system prompt prepended to the JSON instruction", Required: false},
			{Name: "temperature", Type: "string", Description: "Sampling temperature 0.0-2.0 (default: 0.0 for deterministic output)", Required: false, Default: "0"},
			{Name: "max_tokens", Type: "string", Description: "Max tokens to generate", Required: false},
			{Name: "max_retries", Type: "string", Description: "Max self-correction retries on parse/validation failure (default: 2)", Required: false, Default: "2"},
			{Name: "format_output", Type: "string", Description: "If \"true\", pretty-print the JSON output (default: false, compact)", Required: false, Default: "false"},
		},
	}
}

// Execute runs the structured output pipeline.
func (n *StructuredOutputNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if params == nil {
		return "", fmt.Errorf("structured_output: params are required (need at least \"schema\")")
	}

	schemaStr := strings.TrimSpace(params["schema"])
	if schemaStr == "" {
		return "", fmt.Errorf("structured_output: \"schema\" param is required")
	}

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaStr), &schema); err != nil {
		return "", fmt.Errorf("structured_output: schema must be valid JSON: %w", err)
	}
	if err := validateRootSchema(schema); err != nil {
		return "", fmt.Errorf("structured_output: invalid schema: %w", err)
	}

	schemaName := getParam(params, "schema_name", "output")
	maxRetries := parseIntSafe(getParam(params, "max_retries", "2"), 2)
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries > 5 {
		maxRetries = 5
	}
	pretty := getParam(params, "format_output", "false") == "true"

	// Build the LLM client. We use OpenAICompatibleNode directly so the
	// B-2 telemetry flows through the context sink.
	provider := getParam(params, "provider", "openai")
	node := core.NewOpenAICompatibleNode(core.LLMNodeConfig{
		Name:            "structured_output",
		DefaultModel:    defaultModelFor(provider),
		DefaultEndpoint: defaultEndpointFor(provider),
		EnvAPIKey:       strings.ToUpper(provider) + "_API_KEY",
		ProviderName:    provider,
	})

	systemPrompt := buildStructuredSystemPrompt(schemaName, schema, getParam(params, "system", ""))

	callParams := copyParamsForLLM(params)
	callParams["response_format"] = "json_object"
	if _, ok := callParams["temperature"]; !ok {
		callParams["temperature"] = "0"
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Build the user message: original instruction + (on retry) feedback.
		userMsg := input
		if attempt > 0 && lastErr != nil {
			userMsg = fmt.Sprintf(
				"%s\n\nYour previous output failed validation:\n%s\n\nPlease return ONLY a corrected JSON object that conforms to the schema.",
				input, lastErr.Error(),
			)
		}

		callParams["system"] = systemPrompt
		raw, err := node.Execute(ctx, userMsg, callParams)
		if err != nil {
			return "", fmt.Errorf("structured_output: LLM call failed: %w", err)
		}

		// Extract JSON: the model may wrap it in ```json fences despite
		// json_object mode being set. Be tolerant.
		jsonStr, err := extractJSON(raw)
		if err != nil {
			lastErr = fmt.Errorf("could not extract JSON from response: %w", err)
			continue
		}

		var parsed interface{}
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
			lastErr = fmt.Errorf("response is not valid JSON: %w", err)
			continue
		}

		if verr := validateAgainstSchema(parsed, schema, ""); verr != nil {
			lastErr = verr
			continue
		}

		// Success: re-marshal so the output is canonical (deterministic key
		// order via encoding/json's sorted map iteration).
		var out []byte
		if pretty {
			out, err = json.MarshalIndent(parsed, "", "  ")
		} else {
			out, err = json.Marshal(parsed)
		}
		if err != nil {
			return "", fmt.Errorf("structured_output: failed to marshal validated output: %w", err)
		}
		return string(out), nil
	}

	return "", fmt.Errorf("structured_output: failed after %d retries: %w", maxRetries, lastErr)
}

// validateRootSchema enforces the minimal contract we need from the
// caller's schema: it must be an object with type "object".
func validateRootSchema(schema map[string]interface{}) error {
	t, _ := schema["type"].(string)
	if t != "object" {
		return fmt.Errorf("root schema must have \"type\":\"object\", got %q", t)
	}
	return nil
}

// buildStructuredSystemPrompt constructs the system prompt that tells the
// LLM what shape to produce. extraSystem is appended when provided.
func buildStructuredSystemPrompt(name string, schema map[string]interface{}, extraSystem string) string {
	schemaBytes, _ := json.MarshalIndent(schema, "", "  ")
	var sb strings.Builder
	sb.WriteString("You are a structured-output generator. ")
	sb.WriteString("Return ONLY a single JSON object that conforms to the following JSON Schema. ")
	sb.WriteString("Do not include any explanation, prose, or markdown fences. ")
	sb.WriteString("The output must be valid JSON parseable by encoding/json.\n\n")
	sb.WriteString("Schema name: ")
	sb.WriteString(name)
	sb.WriteString("\nSchema:\n")
	sb.Write(schemaBytes)
	if extraSystem != "" {
		sb.WriteString("\n\nAdditional instructions:\n")
		sb.WriteString(extraSystem)
	}
	return sb.String()
}

// copyParamsForLLM copies the LLM-relevant params from the input map,
// dropping structured_output-specific keys so they don't leak into the
// LLM request as unknown params.
func copyParamsForLLM(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		switch k {
		case "schema", "schema_name", "max_retries", "format_output":
			continue
		}
		out[k] = v
	}
	return out
}

// extractJSON extracts a JSON object from a possibly-fenced/leading-prose
// response. It handles:
//   - ```json ... ``` fenced blocks
//   - ``` ... ``` fenced blocks
//   - bare responses that start with { (or whitespace then {)
//   - responses with trailing prose (returns the first balanced object)
func extractJSON(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty response")
	}

	// Strip markdown code fences.
	if strings.HasPrefix(s, "```") {
		// Drop opening fence (with optional language tag).
		if nl := strings.IndexByte(s, '\n'); nl >= 0 {
			s = s[nl+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
		s = strings.TrimSpace(s)
	}

	// Find the first '{' and balance braces, respecting string literals.
	start := strings.IndexByte(s, '{')
	if start < 0 {
		// Maybe it's a top-level array — schema root requires object,
		// so this is an error.
		return "", fmt.Errorf("no JSON object found in response")
	}

	depth := 0
	inStr := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' && inStr {
			escape = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], nil
			}
		}
	}
	return "", fmt.Errorf("unbalanced JSON object in response")
}

// validateAgainstSchema validates value against schema. path is the
// JSON-pointer-like location used in error messages (e.g. "/foo/0/bar").
// Returns nil if valid, an error describing the first violation otherwise.
//
// Supported keywords (draft-07 subset):
//
//	type                  - "object"|"array"|"string"|"number"|"integer"|"boolean"|"null"
//	required              - []string (object only)
//	properties            - map[string]*schema (object only)
//	additionalProperties  - false (object only; forbid unknown props)
//	items                 - single schema (array only; uniform element schema)
//	enum                  - []any
//	minimum / maximum     - number bounds (inclusive)
//	exclusiveMinimum / exclusiveMaximum - number bounds (exclusive)
//	minLength / maxLength - string bounds (inclusive)
//	minItems / maxItems   - array bounds (inclusive)
func validateAgainstSchema(value interface{}, schema map[string]interface{}, path string) error {
	if schema == nil {
		return nil
	}

	// type
	if t, ok := schema["type"].(string); ok {
		if err := validateType(value, t, path); err != nil {
			return err
		}
	}

	// enum
	if enum, ok := schema["enum"].([]interface{}); ok {
		matched := false
		for _, candidate := range enum {
			if valuesEqual(value, candidate) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("%s: value not in enum %v", pathDesc(path), enum)
		}
	}

	// Type-specific checks. Run only if the value's type matches the
	// declared type (or no type was declared); otherwise the type check
	// above already reported the violation.
	switch v := value.(type) {
	case map[string]interface{}:
		if err := validateObject(v, schema, path); err != nil {
			return err
		}
	case []interface{}:
		if err := validateArray(v, schema, path); err != nil {
			return err
		}
	case string:
		if err := validateString(v, schema, path); err != nil {
			return err
		}
	case float64:
		if err := validateNumber(v, schema, path); err != nil {
			return err
		}
	case int:
		if err := validateNumber(float64(v), schema, path); err != nil {
			return err
		}
	}

	return nil
}

func validateType(value interface{}, t string, path string) error {
	ok := false
	switch v := value.(type) {
	case nil:
		ok = t == "null"
	case bool:
		ok = t == "boolean"
	case float64:
		// JSON numbers unmarshal to float64. "integer" accepts a float
		// with zero fractional part, matching JSON Schema semantics.
		switch t {
		case "number":
			ok = true
		case "integer":
			ok = v == float64(int64(v))
		}
	case int:
		ok = t == "number" || t == "integer"
	case string:
		ok = t == "string"
	case []interface{}:
		ok = t == "array"
	case map[string]interface{}:
		ok = t == "object"
	}
	if !ok {
		return fmt.Errorf("%s: expected type %q, got %s", pathDesc(path), t, goTypeName(value))
	}
	return nil
}

func validateObject(obj map[string]interface{}, schema map[string]interface{}, path string) error {
	// required
	if req, ok := schema["required"].([]interface{}); ok {
		for _, r := range req {
			name, _ := r.(string)
			if name == "" {
				continue
			}
			if _, present := obj[name]; !present {
				return fmt.Errorf("%s: missing required property %q", pathDesc(path), name)
			}
		}
	}

	props, _ := schema["properties"].(map[string]interface{})
	additional := schema["additionalProperties"]

	for k, v := range obj {
		childPath := joinPath(path, k)
		if subschema, ok := props[k].(map[string]interface{}); ok {
			if err := validateAgainstSchema(v, subschema, childPath); err != nil {
				return err
			}
			continue
		}
		// Not in properties: check additionalProperties.
		switch ap := additional.(type) {
		case bool:
			if !ap {
				return fmt.Errorf("%s: additional property %q not allowed", pathDesc(path), k)
			}
		case map[string]interface{}:
			if err := validateAgainstSchema(v, ap, childPath); err != nil {
				return err
			}
		}
		// additionalProperties == nil (unset): allow by default.
	}
	return nil
}

func validateArray(arr []interface{}, schema map[string]interface{}, path string) error {
	// items (single-schema form only; we do not support tuple form).
	if items, ok := schema["items"].(map[string]interface{}); ok {
		for i, el := range arr {
			childPath := fmt.Sprintf("%s/%d", path, i)
			if err := validateAgainstSchema(el, items, childPath); err != nil {
				return err
			}
		}
	}

	if n, ok := intFromSchema(schema, "minItems"); ok && len(arr) < n {
		return fmt.Errorf("%s: array length %d < minItems %d", pathDesc(path), len(arr), n)
	}
	if n, ok := intFromSchema(schema, "maxItems"); ok && len(arr) > n {
		return fmt.Errorf("%s: array length %d > maxItems %d", pathDesc(path), len(arr), n)
	}
	return nil
}

func validateString(s string, schema map[string]interface{}, path string) error {
	if n, ok := intFromSchema(schema, "minLength"); ok && len(s) < n {
		return fmt.Errorf("%s: string length %d < minLength %d", pathDesc(path), len(s), n)
	}
	if n, ok := intFromSchema(schema, "maxLength"); ok && len(s) > n {
		return fmt.Errorf("%s: string length %d > maxLength %d", pathDesc(path), len(s), n)
	}
	return nil
}

func validateNumber(n float64, schema map[string]interface{}, path string) error {
	if v, ok := numFromSchema(schema, "minimum"); ok && n < v {
		return fmt.Errorf("%s: %g < minimum %g", pathDesc(path), n, v)
	}
	if v, ok := numFromSchema(schema, "maximum"); ok && n > v {
		return fmt.Errorf("%s: %g > maximum %g", pathDesc(path), n, v)
	}
	if v, ok := numFromSchema(schema, "exclusiveMinimum"); ok && n <= v {
		return fmt.Errorf("%s: %g <= exclusiveMinimum %g", pathDesc(path), n, v)
	}
	if v, ok := numFromSchema(schema, "exclusiveMaximum"); ok && n >= v {
		return fmt.Errorf("%s: %g >= exclusiveMaximum %g", pathDesc(path), n, v)
	}
	return nil
}

// intFromSchema reads an integer-valued keyword, accepting either int or
// float64 (the form json.Unmarshal produces for non-integer literals).
func intFromSchema(schema map[string]interface{}, key string) (int, bool) {
	v, ok := schema[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}

// numFromSchema reads a numeric keyword as float64.
func numFromSchema(schema map[string]interface{}, key string) (float64, bool) {
	v, ok := schema[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

func joinPath(base, key string) string {
	if base == "" {
		return "/" + key
	}
	return base + "/" + key
}

func pathDesc(path string) string {
	if path == "" {
		return "root"
	}
	return path
}

func goTypeName(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64, int:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	}
	return fmt.Sprintf("%T", v)
}

// valuesEqual compares two JSON-decoded values. Numbers compare by value
// (so 1.0 == 1), matching JSON Schema enum semantics.
func valuesEqual(a, b interface{}) bool {
	switch av := a.(type) {
	case float64:
		if bv, ok := b.(float64); ok {
			return av == bv
		}
		if bv, ok := b.(int); ok {
			return av == float64(bv)
		}
	case int:
		if bv, ok := b.(float64); ok {
			return float64(av) == bv
		}
		if bv, ok := b.(int); ok {
			return av == bv
		}
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
	case bool:
		if bv, ok := b.(bool); ok {
			return av == bv
		}
	case nil:
		return b == nil
	case []interface{}:
		bv, ok := b.([]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !valuesEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case map[string]interface{}:
		bv, ok := b.(map[string]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			if !valuesEqual(v, bv[k]) {
				return false
			}
		}
		return true
	}
	return false
}
