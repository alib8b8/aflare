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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// TestStructuredOutput_Registration verifies the node is registered.
func TestStructuredOutput_Registration(t *testing.T) {
	n, ok := Get("structured_output")
	if !ok {
		t.Fatal("structured_output node not registered")
	}
	if n.Name() != "structured_output" {
		t.Errorf("Name=%q", n.Name())
	}
}

// TestStructuredOutput_RequiresSchema verifies the node errors when no
// schema param is supplied.
func TestStructuredOutput_RequiresSchema(t *testing.T) {
	n := &StructuredOutputNode{}
	_, err := n.Execute(context.Background(), "hi", map[string]string{})
	if err == nil {
		t.Fatal("expected error when schema missing")
	}
	if !strings.Contains(err.Error(), "schema") {
		t.Errorf("error should mention schema, got: %v", err)
	}
}

// TestStructuredOutput_RejectsNonObjectRoot verifies the root schema must
// declare type "object".
func TestStructuredOutput_RejectsNonObjectRoot(t *testing.T) {
	n := &StructuredOutputNode{}
	_, err := n.Execute(context.Background(), "hi", map[string]string{
		"schema": `{"type":"string"}`,
	})
	if err == nil {
		t.Fatal("expected error for non-object root schema")
	}
	if !strings.Contains(err.Error(), "type\":\"object\"") {
		t.Errorf("error should mention object root requirement, got: %v", err)
	}
}

// TestStructuredOutput_RejectsInvalidSchemaJSON verifies malformed schema
// JSON is rejected.
func TestStructuredOutput_RejectsInvalidSchemaJSON(t *testing.T) {
	n := &StructuredOutputNode{}
	_, err := n.Execute(context.Background(), "hi", map[string]string{
		"schema": `{not json`,
	})
	if err == nil {
		t.Fatal("expected error for malformed schema JSON")
	}
}

// TestStructuredOutput_HappyPath verifies the full pipeline: schema →
// LLM call → JSON parse → schema validation → canonical output.
func TestStructuredOutput_HappyPath(t *testing.T) {
	schema := `{
		"type": "object",
		"required": ["name", "age"],
		"properties": {
			"name": {"type": "string", "minLength": 1},
			"age":  {"type": "integer", "minimum": 0}
		},
		"additionalProperties": false
	}`

	srv := mockStructuredServer(t, `{"name":"Alice","age":30}`)
	defer srv.Close()

	n := &StructuredOutputNode{}
	out, err := n.Execute(context.Background(), "describe Alice", map[string]string{
		"schema":   schema,
		"endpoint": srv.URL,
		"api_key":  "sk-test",
		"model":    "test-model",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output not valid JSON: %v (out=%s)", err, out)
	}
	if got["name"] != "Alice" {
		t.Errorf("name=%v want Alice", got["name"])
	}
	if got["age"] != float64(30) {
		t.Errorf("age=%v want 30", got["age"])
	}
}

// TestStructuredOutput_RetryOnValidationError verifies the self-correction
// loop: the first response is missing a required field, the second
// response (after the model "fixes" it) is valid.
func TestStructuredOutput_RetryOnValidationError(t *testing.T) {
	schema := `{
		"type": "object",
		"required": ["name", "age"],
		"properties": {
			"name": {"type": "string"},
			"age":  {"type": "integer"}
		}
	}`

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		_, _ = io.ReadAll(r.Body)
		var resp string
		if n == 1 {
			// Missing required field "age".
			resp = `{"name":"Bob"}`
		} else {
			resp = `{"name":"Bob","age":25}`
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": resp}},
			},
		})
	}))
	defer srv.Close()

	n := &StructuredOutputNode{}
	out, err := n.Execute(context.Background(), "describe Bob", map[string]string{
		"schema":      schema,
		"endpoint":    srv.URL,
		"api_key":     "sk-test",
		"model":       "test-model",
		"max_retries": "2",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 LLM calls (1 fail + 1 retry), got %d", calls)
	}
	var got map[string]interface{}
	_ = json.Unmarshal([]byte(out), &got)
	if got["age"] != float64(25) {
		t.Errorf("age=%v want 25 (should be present after retry)", got["age"])
	}
}

// TestStructuredOutput_AllRetriesExhausted verifies that when every attempt
// fails validation, the node returns an error mentioning the retry count.
func TestStructuredOutput_AllRetriesExhausted(t *testing.T) {
	schema := `{
		"type": "object",
		"required": ["x"],
		"properties": {"x": {"type": "integer"}}
	}`

	// Always returns invalid output (missing required field).
	srv := mockStructuredServer(t, `{"wrong":"shape"}`)
	defer srv.Close()

	n := &StructuredOutputNode{}
	_, err := n.Execute(context.Background(), "produce x", map[string]string{
		"schema":      schema,
		"endpoint":    srv.URL,
		"api_key":     "sk-test",
		"model":       "test-model",
		"max_retries": "1",
	})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !strings.Contains(err.Error(), "failed after") {
		t.Errorf("error should mention retry exhaustion, got: %v", err)
	}
}

// TestStructuredOutput_StripsMarkdownFences verifies the extractor handles
// ```json ... ``` fenced responses (which some models emit despite
// json_object mode being set).
func TestStructuredOutput_StripsMarkdownFences(t *testing.T) {
	schema := `{"type":"object","properties":{"k":{"type":"string"}}}`

	srv := mockStructuredServer(t, "```json\n{\"k\":\"v\"}\n```")
	defer srv.Close()

	n := &StructuredOutputNode{}
	out, err := n.Execute(context.Background(), "produce k=v", map[string]string{
		"schema":   schema,
		"endpoint": srv.URL,
		"api_key":  "sk-test",
		"model":    "test-model",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("fenced output not parsed: %v (out=%s)", err, out)
	}
	if got["k"] != "v" {
		t.Errorf("k=%v want v", got["k"])
	}
}

// TestStructuredOutput_PrettyFormat verifies format_output=true returns
// indented JSON.
func TestStructuredOutput_PrettyFormat(t *testing.T) {
	schema := `{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"integer"}}}`

	srv := mockStructuredServer(t, `{"a":"x","b":1}`)
	defer srv.Close()

	n := &StructuredOutputNode{}
	out, err := n.Execute(context.Background(), "produce", map[string]string{
		"schema":        schema,
		"endpoint":      srv.URL,
		"api_key":       "sk-test",
		"model":         "test-model",
		"format_output": "true",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "\n") {
		t.Errorf("pretty output should be multi-line, got: %q", out)
	}
	if !strings.Contains(out, "  \"a\":") {
		t.Errorf("pretty output should be indented, got: %q", out)
	}
}

// --- Validator unit tests ---

func TestValidate_TypeMismatch(t *testing.T) {
	schema := map[string]interface{}{"type": "string"}
	if err := validateAgainstSchema(42, schema, ""); err == nil {
		t.Error("expected type mismatch error for int vs string")
	}
}

func TestValidate_IntegerAcceptsZeroFraction(t *testing.T) {
	schema := map[string]interface{}{"type": "integer"}
	if err := validateAgainstSchema(float64(7), schema, ""); err != nil {
		t.Errorf("7.0 should validate as integer: %v", err)
	}
	if err := validateAgainstSchema(float64(7.5), schema, ""); err == nil {
		t.Error("7.5 should NOT validate as integer")
	}
}

func TestValidate_Required(t *testing.T) {
	schema := map[string]interface{}{
		"type":     "object",
		"required": []interface{}{"a", "b"},
		"properties": map[string]interface{}{
			"a": map[string]interface{}{"type": "string"},
			"b": map[string]interface{}{"type": "integer"},
		},
	}
	if err := validateAgainstSchema(map[string]interface{}{"a": "x", "b": float64(1)}, schema, ""); err != nil {
		t.Errorf("valid object rejected: %v", err)
	}
	if err := validateAgainstSchema(map[string]interface{}{"a": "x"}, schema, ""); err == nil {
		t.Error("missing required b should be rejected")
	}
}

func TestValidate_AdditionalPropertiesFalse(t *testing.T) {
	schema := map[string]interface{}{
		"type":                 "object",
		"properties":           map[string]interface{}{"a": map[string]interface{}{"type": "string"}},
		"additionalProperties": false,
	}
	if err := validateAgainstSchema(map[string]interface{}{"a": "x"}, schema, ""); err != nil {
		t.Errorf("allowed property rejected: %v", err)
	}
	if err := validateAgainstSchema(map[string]interface{}{"a": "x", "extra": 1}, schema, ""); err == nil {
		t.Error("additional property should be rejected when additionalProperties=false")
	}
}

func TestValidate_Nested(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":     "object",
					"required": []interface{}{"id"},
					"properties": map[string]interface{}{
						"id":   map[string]interface{}{"type": "integer"},
						"name": map[string]interface{}{"type": "string", "minLength": 1},
					},
				},
				"minItems": 1,
			},
		},
	}
	good := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"id": float64(1), "name": "first"},
			map[string]interface{}{"id": float64(2), "name": "second"},
		},
	}
	if err := validateAgainstSchema(good, schema, ""); err != nil {
		t.Errorf("valid nested object rejected: %v", err)
	}

	// Missing id on second element.
	bad := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"id": float64(1), "name": "first"},
			map[string]interface{}{"name": "second"},
		},
	}
	if err := validateAgainstSchema(bad, schema, ""); err == nil {
		t.Error("missing id on nested element should be rejected")
	}

	// Empty array violates minItems.
	empty := map[string]interface{}{
		"items": []interface{}{},
	}
	if err := validateAgainstSchema(empty, schema, ""); err == nil {
		t.Error("empty array should violate minItems=1")
	}
}

func TestValidate_NumberBounds(t *testing.T) {
	schema := map[string]interface{}{
		"type":    "number",
		"minimum": 0,
		"maximum": 10,
	}
	cases := []struct {
		v    float64
		want bool
	}{
		{-1, false},
		{0, true},
		{5, true},
		{10, true},
		{11, false},
	}
	for _, c := range cases {
		err := validateAgainstSchema(c.v, schema, "")
		if (err == nil) != c.want {
			t.Errorf("v=%g: want valid=%v, got err=%v", c.v, c.want, err)
		}
	}
}

func TestValidate_ExclusiveBounds(t *testing.T) {
	schema := map[string]interface{}{
		"type":             "number",
		"exclusiveMinimum": 0,
		"exclusiveMaximum": 10,
	}
	if err := validateAgainstSchema(float64(0), schema, ""); err == nil {
		t.Error("0 should violate exclusiveMinimum=0")
	}
	if err := validateAgainstSchema(float64(10), schema, ""); err == nil {
		t.Error("10 should violate exclusiveMaximum=10")
	}
	if err := validateAgainstSchema(float64(5), schema, ""); err != nil {
		t.Errorf("5 should be valid: %v", err)
	}
}

func TestValidate_Enum(t *testing.T) {
	schema := map[string]interface{}{
		"type": "string",
		"enum": []interface{}{"red", "green", "blue"},
	}
	if err := validateAgainstSchema("red", schema, ""); err != nil {
		t.Errorf("red should be valid: %v", err)
	}
	if err := validateAgainstSchema("purple", schema, ""); err == nil {
		t.Error("purple should be rejected by enum")
	}
}

func TestValidate_StringLength(t *testing.T) {
	schema := map[string]interface{}{
		"type":      "string",
		"minLength": 2,
		"maxLength": 4,
	}
	if err := validateAgainstSchema("a", schema, ""); err == nil {
		t.Error("'a' should violate minLength=2")
	}
	if err := validateAgainstSchema("ab", schema, ""); err != nil {
		t.Errorf("'ab' should be valid: %v", err)
	}
	if err := validateAgainstSchema("abcd", schema, ""); err != nil {
		t.Errorf("'abcd' should be valid: %v", err)
	}
	if err := validateAgainstSchema("abcde", schema, ""); err == nil {
		t.Error("'abcde' should violate maxLength=4")
	}
}

// --- extractJSON tests ---

func TestExtractJSON_Bare(t *testing.T) {
	got, err := extractJSON(`{"a":1}`)
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"a":1}` {
		t.Errorf("got=%q", got)
	}
}

func TestExtractJSON_TrailingProse(t *testing.T) {
	got, err := extractJSON(`{"a":1} and here is some prose`)
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"a":1}` {
		t.Errorf("got=%q want {\"a\":1}", got)
	}
}

func TestExtractJSON_FencedJSON(t *testing.T) {
	got, err := extractJSON("```json\n{\"a\":1}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"a":1}` {
		t.Errorf("got=%q", got)
	}
}

func TestExtractJSON_FencedBare(t *testing.T) {
	got, err := extractJSON("```\n{\"a\":1}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"a":1}` {
		t.Errorf("got=%q", got)
	}
}

func TestExtractJSON_BraceInString(t *testing.T) {
	in := `{"msg":"hello } world","n":1}`
	got, err := extractJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	if got != in {
		t.Errorf("got=%q want %q", got, in)
	}
}

func TestExtractJSON_EscapedQuoteInString(t *testing.T) {
	in := `{"msg":"he said \"hi\"","n":1}`
	got, err := extractJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	if got != in {
		t.Errorf("got=%q want %q", got, in)
	}
}

func TestExtractJSON_NoObject(t *testing.T) {
	if _, err := extractJSON(`just text`); err == nil {
		t.Error("expected error for no JSON object")
	}
}

func TestExtractJSON_Empty(t *testing.T) {
	if _, err := extractJSON(`   `); err == nil {
		t.Error("expected error for empty input")
	}
}

func TestExtractJSON_Unbalanced(t *testing.T) {
	if _, err := extractJSON(`{"a":1`); err == nil {
		t.Error("expected error for unbalanced braces")
	}
}

// --- helper ---

// mockStructuredServer returns a server that always replies with the given
// content string in the standard OpenAI chat-completion shape.
func mockStructuredServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": content}},
			},
		})
	}))
}
