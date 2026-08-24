// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​​​‌‌‌​​​​​‌​‌‌​‌‌​‌‌‌​‌‌‌​‌​‌‌​‌​​​‌​‌‌​​‌‌‌‌​​​​​​​​​​​​​​​​​​‌​‌‌​‌​​​‌‌‌‌​⁠
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

package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// setFirstExprError records the first error from a known-prefix expression,
// mirroring the legacy behaviour where only known prefixes surface errors.
// Called only on the failure path; the happy path never calls it.
func setFirstExprError(ins *instruction, firstErr *error, err error) {
	if ins.known && *firstErr == nil {
		*firstErr = fmt.Errorf("expression '{{%s}}': %w", ins.inner, err)
	}
}

// resolveInstr resolves a single instruction to its string value. It is the
// shared dispatch core of Evaluate, evaluateInto, and the vectorised batch
// path's single-expression fast path: returning the value (rather than
// appending to a buffer) lets the vectorised path hand back an already-existing
// string — a workflow variable, a step output, or the input — without copying
// it through a buffer, which is the allocation the scalar Evaluate always pays.
//
// On the happy path it returns (value, nil); on the failure path (value is
// then meaningless) it returns ("", err) and the caller records the error via
// setFirstExprError and appends the verbatim {{...}} fallback. The switch
// mirrors the original inline Evaluate dispatch exactly, preserving semantics
// (including known-prefix error surfacing and the bare-name var/input fallback).
func (e *ExpressionEngine) resolveInstr(ins *instruction, input string) (string, error) {
	switch ins.op {
	case opLiteral:
		return ins.strArg, nil
	case opInput:
		return input, nil
	case opStep:
		return e.evalStepRef(ins.strArg)
	case opVar:
		if v, ok := e.variables[ins.strArg]; ok {
			return v, nil
		}
		return "", fmt.Errorf("variable not found: %s", ins.strArg)
	case opEnv:
		return evalEnvVar(ins.strArg)
	case opFile:
		return evalFileContents(ins.strArg)
	case opLoop:
		if e.loopVars == nil {
			return "", fmt.Errorf("not in a loop context")
		}
		if v, ok := e.loopVars[ins.strArg]; ok {
			return v, nil
		}
		return "", fmt.Errorf("loop variable not found: %s", ins.strArg)
	case opSecret:
		return evalSecretRef(e, ins.strArg)
	case opBareName:
		if v, ok := e.variables[ins.strArg]; ok {
			return v, nil
		}
		if ins.strArg == "input" {
			return input, nil
		}
		return "", fmt.Errorf("unknown expression: %s", ins.strArg)
	case opJSONPath:
		// The reference was pre-compiled at template-compile time
		// (refNode), so this is a single interface call with no per-eval
		// allocation. extractJSONPath's JSON parsing dwarfs this dispatch.
		rv, rerr := ins.refNode.eval(e, input)
		if rerr != nil {
			return "", rerr
		}
		return extractJSONPath(rv, ins.strArg2)
	case opUnknown:
		return "", fmt.Errorf("unknown expression: %s", ins.inner)
	}
	return "", nil
}

// evalEnvVar resolves a {{env.NAME}} expression against the allow-list.
func evalEnvVar(name string) (string, error) {
	if !isAllowedEnvVar(name) {
		return "", fmt.Errorf("access to environment variable %q is not allowed", name)
	}
	if v, ok := os.LookupEnv(name); ok {
		return v, nil
	}
	return "", fmt.Errorf("environment variable not found: %s", name)
}

// evalFileContents resolves a {{file.PATH}} expression: validates the path,
// checks the size limit, and reads the contents. Extracted from the inline
// switch so the file-I/O case (rare and slow) does not bloat the hot loop.
func evalFileContents(name string) (string, error) {
	safePath, err := validateExprFilePath(name)
	if err != nil {
		return "", fmt.Errorf("file path validation failed: %w", err)
	}
	info, err := os.Stat(safePath) // codeql[go/path-injection] -- safePath is the validateExprFilePath result: rejects absolute paths, resolves symlinks, confines the file under the working directory
	if err != nil {
		return "", fmt.Errorf("failed to stat file '%s': %w", name, err)
	}
	if info.Size() > maxExprFileSize {
		return "", fmt.Errorf("file '%s' too large (max %d bytes)", name, maxExprFileSize)
	}
	content, err := os.ReadFile(safePath) // #nosec G304 -- path validated by validateExprFilePath // codeql[go/path-injection] -- safePath is the validateExprFilePath result (absolute/traversal/symlink escape rejected)
	if err != nil {
		return "", fmt.Errorf("failed to read file '%s': %w", name, err)
	}
	return string(content), nil
}

// evalSecretRef resolves a {{secret.GROUP.KEY}} expression via the engine's
// secret getter. Extracted from the inline switch to keep the hot loop compact.
func evalSecretRef(e *ExpressionEngine, name string) (string, error) {
	if e.secretGetter == nil {
		return "", fmt.Errorf("secrets not available - use 'aflare secrets add' to store secrets first")
	}
	secretParts := strings.SplitN(name, ".", 2)
	if len(secretParts) < 2 {
		return "", fmt.Errorf("secret expression requires format: secret.GROUP.KEY")
	}
	return e.secretGetter(strings.TrimSpace(secretParts[0]), strings.TrimSpace(secretParts[1]))
}

// evalStepRef resolves a step reference by index or name
func (e *ExpressionEngine) evalStepRef(name string) (string, error) {
	// Try as index first
	if output, ok := e.stepOutputs["idx:"+name]; ok {
		return output, nil
	}
	// Try as name
	if output, ok := e.stepOutputs["name:"+name]; ok {
		return output, nil
	}
	// Backward compat: try raw key (for any old-format entries)
	if output, ok := e.stepOutputs[name]; ok {
		return output, nil
	}

	return "", fmt.Errorf("step reference not found: %s", name)
}

// extractJSONPath extracts a value from a JSON string using a simplified JSONPath.
// Supports:
//   - $.field - top-level field
//   - $.field.subfield - nested field
//   - $.field[0] - array index
//   - $.field[*] - all array elements (joined with newline)
//   - $..field - recursive descent (all matching fields at any depth)
func extractJSONPath(jsonStr, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || path == "$" {
		return jsonStr, nil
	}

	// Security: limit JSON input size
	if len(jsonStr) > maxExprFileSize {
		return "", fmt.Errorf("JSON input too large for jsonpath (max %d bytes)", maxExprFileSize)
	}

	// Security: limit path length
	if len(path) > 1024 {
		return "", fmt.Errorf("jsonpath too long (max 1024 chars)")
	}

	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", fmt.Errorf("invalid JSON for jsonpath: %w", err)
	}

	result, err := evalJSONPath(data, path)
	if err != nil {
		return "", err
	}

	return jsonPathResultToString(result)
}

// evalJSONPath navigates the JSON data structure following the path.
func evalJSONPath(data interface{}, path string) (interface{}, error) {
	return evalJSONPathDepth(data, path, 0)
}

func evalJSONPathDepth(data interface{}, path string, depth int) (interface{}, error) {
	if depth > maxJSONPathDepth {
		return nil, fmt.Errorf("jsonpath recursion depth exceeded")
	}
	if path == "$" || path == "" {
		return data, nil
	}

	// Recursive descent: $..field
	if strings.HasPrefix(path, "$..") {
		if depth >= 2 {
			return nil, fmt.Errorf("multiple recursive descent segments not supported")
		}
		field := strings.TrimPrefix(path, "$..")
		// Remove any trailing path after the field name
		if idx := strings.IndexAny(field, ".["); idx >= 0 {
			rest := field[idx:]
			field = field[:idx]
			results := recursiveFind(data, field, 0)
			if len(results) == 0 {
				return nil, fmt.Errorf("recursive descent found no matches for '%s'", field)
			}
			if len(results) == 1 {
				return evalJSONPathDepth(results[0], "$"+rest, depth+1)
			}
			// Multiple matches: apply rest to each
			var multiResults []interface{}
			for _, r := range results {
				if v, err := evalJSONPathDepth(r, "$"+rest, depth+1); err == nil {
					multiResults = append(multiResults, v)
				}
			}
			return multiResults, nil
		}
		results := recursiveFind(data, field, 0)
		if len(results) == 0 {
			return nil, fmt.Errorf("recursive descent found no matches for '%s'", field)
		}
		if len(results) == 1 {
			return results[0], nil
		}
		return results, nil
	}

	// Regular path: $.field.subfield[0]
	path = strings.TrimPrefix(path, "$.")
	if path == "" {
		return data, nil
	}

	current := data
	segments, err := parsePathSegments(path)
	if err != nil {
		return nil, err
	}

	for _, seg := range segments {
		switch s := seg.(type) {
		case string:
			m, ok := current.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("cannot access field '%s' on non-object", s)
			}
			val, exists := m[s]
			if !exists {
				return nil, fmt.Errorf("field '%s' not found", s)
			}
			current = val
		case int:
			arr, ok := current.([]interface{})
			if !ok {
				return nil, fmt.Errorf("cannot index non-array")
			}
			if s < 0 || s >= len(arr) {
				return nil, fmt.Errorf("array index %d out of bounds (len %d)", s, len(arr))
			}
			current = arr[s]
		case wildcardMarker:
			arr, ok := current.([]interface{})
			if !ok {
				return nil, fmt.Errorf("cannot wildcard non-array")
			}
			return arr, nil // return the whole array, caller handles
		}
	}

	return current, nil
}

// parsePathSegments parses "field.sub[0].name[*]" into segments.
// Returns error on malformed bracket expressions.
func parsePathSegments(path string) ([]interface{}, error) {
	var segments []interface{}
	i := 0
	for i < len(path) {
		// Read field name
		start := i
		for i < len(path) && path[i] != '.' && path[i] != '[' {
			i++
		}
		if i > start {
			segments = append(segments, path[start:i])
		}
		// Read bracket expressions
		for i < len(path) && path[i] == '[' {
			i++ // skip [
			if i < len(path) && path[i] == '*' {
				i++ // skip *
				if i < len(path) && path[i] == ']' {
					i++
				}
				segments = append(segments, wildcardMarker{})
				continue
			}
			numStart := i
			for i < len(path) && path[i] != ']' {
				i++
			}
			numStr := path[numStart:i]
			if i < len(path) {
				i++ // skip ]
			}
			if numStr == "" {
				return nil, fmt.Errorf("empty array index in path")
			}
			n, err := strconv.Atoi(numStr)
			if err != nil {
				return nil, fmt.Errorf("invalid array index '%s' in path", numStr)
			}
			segments = append(segments, n)
		}
		if i < len(path) && path[i] == '.' {
			i++
		}
	}
	return segments, nil
}

func recursiveFind(data interface{}, field string, depth int) []interface{} {
	if depth > maxRecursiveDepth {
		return nil
	}
	var results []interface{}
	var walk func(v interface{}, d int)
	walk = func(v interface{}, d int) {
		if d > maxRecursiveDepth {
			return
		}
		switch val := v.(type) {
		case map[string]interface{}:
			if fv, ok := val[field]; ok {
				results = append(results, fv)
			}
			for _, child := range val {
				walk(child, d+1)
			}
		case []interface{}:
			for _, item := range val {
				walk(item, d+1)
			}
		}
	}
	walk(data, depth)
	return results
}

func jsonPathResultToString(result interface{}) (string, error) {
	switch v := result.(type) {
	case string:
		return v, nil
	case float64:
		// Use %g to avoid trailing zeros but handle integers correctly
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10), nil
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
	case nil:
		return "", nil
	case []interface{}:
		// Join array elements with newline
		var parts []string
		for _, item := range v {
			s, _ := jsonPathResultToString(item)
			parts = append(parts, s)
		}
		return strings.Join(parts, "\n"), nil
	default:
		// Complex object: marshal back to JSON
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal jsonpath result: %w", err)
		}
		return string(b), nil
	}
}

// EvaluateParamsVectorized batch-evaluates a map of expressions against the
// same engine state and input. It compiles all expressions up front and
// evaluates them in one pass over a single shared pooled buffer, amortising
// buffer growth and sync.Pool round-trips across the whole batch (one buffer
// Get/Put vs N) and letting the CPU prefetcher / branch predictor warm up
// across expressions — the columnar-batch idea from Apache Arrow applied to
// template evaluation.
//
// Two per-expression fast paths avoid the buffer copy that the scalar Evaluate
// always pays, which is what lets the batch path allocate strictly less than
// the serial EvaluateParams:
//   - Literal-only templates return the cached literal directly (no copy).
//   - Single-expression templates (e.g. {{var.x}}, {{step.0}}, {{input}})
//     return the already-resolved string — a variable, a step output, or the
//     input — directly, without copying it through a buffer.
//
// Multi-expression templates are assembled into the shared buffer.
//
// Returns a new map with the same keys, where each value is the evaluated
// result. If any expression fails, its error is recorded (not returned
// immediately); the function continues evaluating the remaining expressions so
// callers get partial results. The returned error (if non-nil) is the first
// error encountered, in deterministic (sorted) key order. This differs from
// EvaluateParams, which aborts on the first error and returns a nil map.
//
// For error-free params the result is identical to EvaluateParams.
//
// NOT thread-safe: must be called from the same goroutine that owns the engine.
func (e *ExpressionEngine) EvaluateParamsVectorized(params map[string]string, input string) (map[string]string, error) {
	if len(params) == 0 {
		return map[string]string{}, nil
	}
	n := len(params)

	// Deterministic key order for stable error reporting and reproducible
	// buffer layout across map-iteration orders. A stack array backs the
	// common small-batch case (≤ 16 params) to avoid a heap allocation.
	const smallBatch = 16
	var stackKeys [smallBatch]string
	keys := stackKeys[:0]
	if n > cap(stackKeys) {
		keys = make([]string, 0, n)
	}
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Shared pooled buffer for multi-expression results. Pooling the backing
	// []byte (like Evaluate) avoids a per-call buffer allocation; a single
	// buffer for the whole batch amortises growth (one grow vs N grows).
	bufp := exprBufPool.Get().(*[]byte)
	buf := (*bufp)[:0]
	defer func() {
		*bufp = buf
		exprBufPool.Put(bufp)
	}()

	// Per-param result slot. Literal-only and single-expression templates
	// store their result directly in `literal` (no buffer copy); multi-
	// expression templates record (start, end) offsets into buf. Using "" as
	// the literal sentinel is safe: an empty literal and an expression that
	// evaluates to "" both yield "" via the buf slice (buf[0:0] or
	// buf[start:end] with start==end), and string() of a zero-length slice
	// allocates nothing. A stack array backs the common small-batch case.
	type vecSlot struct {
		literal    string
		start, end int
	}
	var stackSlots [smallBatch]vecSlot
	slots := stackSlots[:n]
	if n > len(stackSlots) {
		slots = make([]vecSlot, n)
	}

	var firstErr error
	for i, key := range keys {
		tmpl := compileTemplate(params[key])
		var paramErr error
		switch {
		case !tmpl.hasExpr:
			// Literal-only fast path: return the cached literal verbatim.
			slots[i].literal = tmpl.literal
		case len(tmpl.instrs) == 1:
			// Single-expression fast path: resolve the one instruction and
			// store its value directly — no buffer copy. On error, fall back
			// to the verbatim {{...}} text (also no copy).
			ins := &tmpl.instrs[0]
			if v, err := e.resolveInstr(ins, input); err != nil {
				setFirstExprError(ins, &paramErr, err)
				slots[i].literal = ins.fullMatch
			} else {
				slots[i].literal = v
			}
		default:
			// Multi-expression: assemble into the shared buffer.
			slots[i].start = len(buf)
			paramErr = e.evaluateInto(tmpl, input, &buf)
			slots[i].end = len(buf)
		}
		if paramErr != nil && firstErr == nil {
			firstErr = fmt.Errorf("param %q: %w", key, paramErr)
		}
	}

	// Materialise result strings. Direct/empty results take the `literal`
	// path (no allocation); assembled results copy out of the shared buffer.
	result := make(map[string]string, n)
	for i, key := range keys {
		if s := slots[i].literal; s != "" {
			result[key] = s
		} else {
			result[key] = string(buf[slots[i].start:slots[i].end])
		}
	}

	return result, firstErr
}

// evaluateInto evaluates tmpl and appends the result to *buf, returning the
// first error encountered (if any). It is the multi-expression core of the
// vectorised batch path: by writing into a caller-provided shared buffer it
// avoids per-expression buffer allocation. Semantics match Evaluate exactly
// (including known-prefix error surfacing and verbatim {{...}} fallback).
func (e *ExpressionEngine) evaluateInto(tmpl *compiledTemplate, input string, buf *[]byte) error {
	if !tmpl.hasExpr {
		*buf = append(*buf, tmpl.literal...)
		return nil
	}
	var firstErr error
	for i := range tmpl.instrs {
		ins := &tmpl.instrs[i]
		if v, err := e.resolveInstr(ins, input); err != nil {
			setFirstExprError(ins, &firstErr, err)
			*buf = append(*buf, ins.fullMatch...)
		} else {
			*buf = append(*buf, v...)
		}
	}
	return firstErr
}
