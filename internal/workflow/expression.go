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

package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const maxExprFileSize = 10 * 1024 * 1024 // 10MB

// validateExprFilePath validates a file path for the {{file.PATH}} expression.
// It ensures the path is relative and within the current working directory,
// rejecting absolute paths, path traversal, and symlink escapes.
func validateExprFilePath(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty file path")
	}
	// Reject absolute paths
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("absolute paths not allowed")
	}
	// Resolve to absolute path
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	absPath := filepath.Join(cwd, name)
	// Resolve symlinks
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	// Verify resolved path is still within cwd
	rel, err := filepath.Rel(cwd, resolved)
	if err != nil {
		return "", fmt.Errorf("path outside working directory")
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path outside working directory")
	}
	return resolved, nil
}

// ExpressionEngine handles template variable substitution
type ExpressionEngine struct {
	// Outputs from previous steps, keyed by step name
	stepOutputs map[string]string
	// Step index to name mapping
	stepNames map[int]string
	// Workflow-level variables
	variables map[string]string
	// Loop variables (loop.item, loop.index, loop.count)
	loopVars map[string]string
	// Secrets accessor function
	secretGetter func(group, key string) (string, error)
}

// NewExpressionEngine creates a new expression engine
func NewExpressionEngine() *ExpressionEngine {
	return &ExpressionEngine{
		stepOutputs: make(map[string]string),
		stepNames:   make(map[int]string),
		variables:   make(map[string]string),
	}
}

// SetStepOutput stores the output of a step for later reference
func (e *ExpressionEngine) SetStepOutput(stepIndex int, stepName, output string) {
	key := fmt.Sprintf("idx:%d", stepIndex)
	e.stepNames[stepIndex] = stepName
	e.stepOutputs[key] = output
	// Only store by name if non-empty and not purely numeric (to avoid index collision)
	if stepName != "" {
		_, err := strconv.Atoi(stepName)
		if err != nil {
			e.stepOutputs["name:"+stepName] = output
		}
	}
}

// SetVariable sets a workflow-level variable
func (e *ExpressionEngine) SetVariable(name, value string) {
	e.variables[name] = value
}

// GetVariable retrieves a workflow-level variable
func (e *ExpressionEngine) GetVariable(name string) (string, bool) {
	v, ok := e.variables[name]
	return v, ok
}

// SetLoopVars sets the current loop context variables.
func (e *ExpressionEngine) SetLoopVars(item string, index, count int) {
	if e.loopVars == nil {
		e.loopVars = make(map[string]string)
	}
	e.loopVars["item"] = item
	e.loopVars["index"] = strconv.Itoa(index)
	e.loopVars["count"] = strconv.Itoa(count)
}

// ClearLoopVars removes the loop context.
func (e *ExpressionEngine) ClearLoopVars() {
	e.loopVars = nil
}

// SetSecretGetter sets the function to retrieve secrets from the secret manager
func (e *ExpressionEngine) SetSecretGetter(getter func(group, key string) (string, error)) {
	e.secretGetter = getter
}

// varPattern matches {{ ... }} expressions
var varPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// Evaluate evaluates an expression and returns the result.
//
// Templates are compiled to an AST on first use and cached at the package
// level (see templateCache), so repeated evaluations of the same template —
// common inside loops, parallel branches, and params — skip both regex
// scanning and per-call re-parsing. Static text without any {{...}} expressions
// takes a zero-allocation fast path. Semantics are identical to the legacy
// regex-based implementation.
//
// Supported expressions:
//
//	{{input}}                    - the workflow's initial input
//	{{step.N}}                   - output of step N (0-indexed)
//	{{step.name}}                - output of step by name
//	{{step.N.jsonpath:$..field}} - JSONPath extraction from step output
//	{{var.NAME}}                 - workflow variable
//	{{env.NAME}}                 - environment variable
//	{{file.PATH}}                - file contents
//	{{secret.GROUP.KEY}}         - secret from secret manager (e.g. {{secret.llm.openai}})
//	{{loop.item}}                - current loop item
//	{{loop.index}}               - current loop index
//	{{loop.count}}               - total loop iterations
//
// Unknown expressions (e.g. Go template syntax {{.foo}}) are left unchanged.
func (e *ExpressionEngine) Evaluate(expr string, input string) (string, error) {
	if expr == "" {
		return "", nil
	}
	tmpl := compileTemplate(expr)
	if !tmpl.hasExpr {
		// Fast path: no expressions — return the literal verbatim.
		return tmpl.literal, nil
	}

	var sb strings.Builder
	var firstErr error
	for i := range tmpl.parts {
		p := &tmpl.parts[i]
		if p.isLiteral {
			sb.WriteString(p.literal)
			continue
		}
		val, err := p.node.eval(e, input)
		if err != nil {
			// Mirror the legacy regex behaviour: expressions with a known
			// prefix surface their first error; unknown expressions (e.g.
			// Go templates) are left verbatim and do not fail evaluation.
			if p.node.knownPrefix() && firstErr == nil {
				firstErr = fmt.Errorf("expression '{{%s}}': %w", p.inner, err)
			}
			sb.WriteString(p.fullMatch)
			continue
		}
		sb.WriteString(val)
	}
	if firstErr != nil {
		return sb.String(), firstErr
	}
	return sb.String(), nil
}

// ── AST expression engine ──
//
// The legacy engine ran a compiled regex (varPattern) over every input string
// and re-parsed each {{...}} inner expression (SplitN/Index/TrimSpace) on every
// call. The AST engine compiles a template once into an immutable
// *compiledTemplate (memoised in templateCache) and evaluates it by walking
// nodes that dispatch directly to the relevant resolver, eliminating per-call
// parsing cost. Evaluation semantics are identical to the legacy implementation.

// exprNode is a compiled single {{...}} expression.
type exprNode interface {
	// eval resolves the expression against the engine state.
	eval(e *ExpressionEngine, input string) (string, error)
	// knownPrefix reports whether the expression uses a recognised prefix
	// (step/var/env/file/input/loop/secret). Used to decide whether an
	// evaluation error should be surfaced or the expression left verbatim,
	// matching isKnownExpressionPrefix.
	knownPrefix() bool
}

// compiledTemplate is the parsed form of a template string.
type compiledTemplate struct {
	// literal holds the entire template when it contains no expressions;
	// hasExpr is false and parts is empty. This is the fast path.
	literal string
	hasExpr bool
	parts   []templatePart
}

// templatePart is either a literal segment or a compiled expression.
type templatePart struct {
	isLiteral bool
	literal   string // valid when isLiteral
	node      exprNode
	fullMatch string // original "{{...}}" text, for verbatim fallback
	inner     string // trimmed inner text, for error messages
}

// templateCache memoises compiled templates by source text. Templates are
// immutable after construction, so concurrent reads are safe; sync.Map handles
// concurrent inserts from worker goroutines (e.g. parallel/loop sub-engines).
var templateCache sync.Map

// compileTemplate returns a cached compiled template for expr, parsing it on
// first encounter.
func compileTemplate(expr string) *compiledTemplate {
	if v, ok := templateCache.Load(expr); ok {
		return v.(*compiledTemplate)
	}
	tmpl := parseTemplate(expr)
	actual, _ := templateCache.LoadOrStore(expr, tmpl)
	return actual.(*compiledTemplate)
}

// parseTemplate scans s for {{...}} expressions, mirroring the legacy varPattern
// (`\{\{([^}]+)\}\}`): an expression is "{{" + one-or-more non-'}' chars + "}}",
// with the closing "}}" being the first '}' encountered. Non-matching text
// becomes literal segments.
func parseTemplate(s string) *compiledTemplate {
	// Fast path: no opening brace-pair, so no expression is possible.
	if strings.Index(s, "{{") < 0 {
		return &compiledTemplate{literal: s}
	}

	var parts []templatePart
	var lit strings.Builder
	n := len(s)
	i := 0
	flushLit := func() {
		if lit.Len() > 0 {
			parts = append(parts, templatePart{isLiteral: true, literal: lit.String()})
			lit.Reset()
		}
	}

	for i < n {
		if s[i] == '{' && i+1 < n && s[i+1] == '{' {
			contentStart := i + 2
			k := contentStart
			for k < n && s[k] != '}' {
				k++
			}
			// Match requires ≥1 non-'}' content char immediately followed by "}}".
			if k > contentStart && k+1 < n && s[k] == '}' && s[k+1] == '}' {
				flushLit()
				inner := strings.TrimSpace(s[contentStart:k])
				parts = append(parts, templatePart{
					node:      compileExpr(inner),
					fullMatch: s[i : k+2],
					inner:     inner,
				})
				i = k + 2
				continue
			}
		}
		lit.WriteByte(s[i])
		i++
	}
	flushLit()

	// If no expression part was produced, the whole string is literal.
	for i := range parts {
		if !parts[i].isLiteral {
			return &compiledTemplate{hasExpr: true, parts: parts}
		}
	}
	return &compiledTemplate{literal: s}
}

// compileExpr compiles a trimmed inner expression (the text between {{ and }})
// into an exprNode. The dispatch mirrors the legacy evalSingle exactly.
func compileExpr(inner string) exprNode {
	known := isKnownExpressionPrefix(inner)

	// jsonpath modifier: <ref>.jsonpath:<path>
	if idx := strings.Index(inner, ".jsonpath:"); idx > 0 {
		refPart := inner[:idx]
		jsonPath := inner[idx+len(".jsonpath:"):]
		return &jsonpathExpr{ref: compileExpr(refPart), path: jsonPath, known: known}
	}

	parts := strings.SplitN(inner, ".", 2)
	if len(parts) < 2 {
		// Bare name: a workflow variable, or the literal token "input".
		return &bareNameExpr{name: inner, known: known}
	}
	prefix := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	switch prefix {
	case "input":
		return &inputExpr{known: known}
	case "step":
		return &stepExpr{name: name, known: known}
	case "var":
		return &varExpr{name: name, known: known}
	case "env":
		return &envExpr{name: name, known: known}
	case "file":
		return &fileExpr{name: name, known: known}
	case "loop":
		return &loopExpr{name: name, known: known}
	case "secret":
		return &secretExpr{name: name, known: known}
	default:
		return &unknownExpr{inner: inner, known: known}
	}
}

func isKnownExpressionPrefix(expr string) bool {
	parts := strings.SplitN(expr, ".", 2)
	if len(parts) < 2 {
		return expr == "input"
	}
	prefix := strings.TrimSpace(parts[0])
	switch prefix {
	case "step", "var", "env", "file", "input", "loop", "secret":
		return true
	}
	return false
}

// ── Node implementations ──

// bareNameExpr handles expressions without a dot: {{input}} or {{varname}}.
// A bare name first checks workflow variables (so a variable named "input"
// shadows the input token, preserving legacy behaviour), then falls back to the
// input token.
type bareNameExpr struct {
	name  string
	known bool
}

func (n *bareNameExpr) eval(e *ExpressionEngine, input string) (string, error) {
	if v, ok := e.variables[n.name]; ok {
		return v, nil
	}
	if n.name == "input" {
		return input, nil
	}
	return "", fmt.Errorf("unknown expression: %s", n.name)
}

func (n *bareNameExpr) knownPrefix() bool { return n.known }

// inputExpr handles {{input.X}} (any dotted input reference resolves to the
// raw input, matching the legacy switch case).
type inputExpr struct{ known bool }

func (n *inputExpr) eval(e *ExpressionEngine, input string) (string, error) {
	return input, nil
}

func (n *inputExpr) knownPrefix() bool { return n.known }

type stepExpr struct {
	name  string
	known bool
}

func (n *stepExpr) eval(e *ExpressionEngine, input string) (string, error) {
	return e.evalStepRef(n.name)
}

func (n *stepExpr) knownPrefix() bool { return n.known }

type varExpr struct {
	name  string
	known bool
}

func (n *varExpr) eval(e *ExpressionEngine, input string) (string, error) {
	if v, ok := e.variables[n.name]; ok {
		return v, nil
	}
	return "", fmt.Errorf("variable not found: %s", n.name)
}

func (n *varExpr) knownPrefix() bool { return n.known }

type envExpr struct {
	name  string
	known bool
}

func (n *envExpr) eval(e *ExpressionEngine, input string) (string, error) {
	if !isAllowedEnvVar(n.name) {
		return "", fmt.Errorf("access to environment variable %q is not allowed", n.name)
	}
	if v, ok := os.LookupEnv(n.name); ok {
		return v, nil
	}
	return "", fmt.Errorf("environment variable not found: %s", n.name)
}

func (n *envExpr) knownPrefix() bool { return n.known }

type fileExpr struct {
	name  string
	known bool
}

func (n *fileExpr) eval(e *ExpressionEngine, input string) (string, error) {
	safePath, err := validateExprFilePath(n.name)
	if err != nil {
		return "", fmt.Errorf("file path validation failed: %w", err)
	}
	info, err := os.Stat(safePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file '%s': %w", n.name, err)
	}
	if info.Size() > maxExprFileSize {
		return "", fmt.Errorf("file '%s' too large (max %d bytes)", n.name, maxExprFileSize)
	}
	content, err := os.ReadFile(safePath) // #nosec G304 -- path validated by validateExprFilePath
	if err != nil {
		return "", fmt.Errorf("failed to read file '%s': %w", n.name, err)
	}
	return string(content), nil
}

func (n *fileExpr) knownPrefix() bool { return n.known }

type loopExpr struct {
	name  string
	known bool
}

func (n *loopExpr) eval(e *ExpressionEngine, input string) (string, error) {
	if e.loopVars == nil {
		return "", fmt.Errorf("not in a loop context")
	}
	if v, ok := e.loopVars[n.name]; ok {
		return v, nil
	}
	return "", fmt.Errorf("loop variable not found: %s", n.name)
}

func (n *loopExpr) knownPrefix() bool { return n.known }

// secretExpr handles {{secret.GROUP.KEY}}. The remainder after "secret." is
// split into GROUP.KEY at evaluation time (matching legacy behaviour).
type secretExpr struct {
	name  string // "GROUP.KEY"
	known bool
}

func (n *secretExpr) eval(e *ExpressionEngine, input string) (string, error) {
	if e.secretGetter == nil {
		return "", fmt.Errorf("secrets not available - use 'llm-box secrets add' to store secrets first")
	}
	secretParts := strings.SplitN(n.name, ".", 2)
	if len(secretParts) < 2 {
		return "", fmt.Errorf("secret expression requires format: secret.GROUP.KEY")
	}
	group := strings.TrimSpace(secretParts[0])
	key := strings.TrimSpace(secretParts[1])
	return e.secretGetter(group, key)
}

func (n *secretExpr) knownPrefix() bool { return n.known }

// jsonpathExpr wraps a reference expression and applies a JSONPath extraction
// to its resolved value.
type jsonpathExpr struct {
	ref   exprNode
	path  string
	known bool
}

func (n *jsonpathExpr) eval(e *ExpressionEngine, input string) (string, error) {
	refValue, err := n.ref.eval(e, input)
	if err != nil {
		return "", err
	}
	return extractJSONPath(refValue, n.path)
}

func (n *jsonpathExpr) knownPrefix() bool { return n.known }

// unknownExpr covers expressions with an unrecognised prefix. Its error is
// never surfaced (knownPrefix is false), so the template leaves the original
// {{...}} text verbatim — matching the legacy treatment of e.g. Go templates.
type unknownExpr struct {
	inner string
	known bool
}

func (n *unknownExpr) eval(e *ExpressionEngine, input string) (string, error) {
	return "", fmt.Errorf("unknown expression: %s", n.inner)
}

func (n *unknownExpr) knownPrefix() bool { return n.known }

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

const maxJSONPathDepth = 10

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

type wildcardMarker struct{}

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

// recursiveFind finds all values for a given field name at any depth.
// maxDepth limits recursion to prevent stack overflow from deeply nested JSON.
const maxRecursiveDepth = 1000

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

// EvaluateParams evaluates all string values in a params map
func (e *ExpressionEngine) EvaluateParams(params map[string]string, input string) (map[string]string, error) {
	if params == nil {
		return nil, nil
	}

	result := make(map[string]string, len(params))
	for k, v := range params {
		evaluated, err := e.Evaluate(v, input)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate param %q: %w", k, err)
		}
		result[k] = evaluated
	}
	return result, nil
}

// ContainsExpression reports whether a string contains any {{ ... }} expressions
func ContainsExpression(s string) bool {
	return varPattern.MatchString(s)
}

var allowedEnvVars = map[string]bool{
	"PATH":              true,
	"HOME":              true,
	"USER":              true,
	"LOGNAME":           true,
	"SHELL":             true,
	"LANG":              true,
	"LC_ALL":            true,
	"TERM":              true,
	"PWD":               true,
	"LLM_BOX_LANG":      true,
	"LLM_BOX_SAFE_MODE": true,
}

func isAllowedEnvVar(name string) bool {
	if allowedEnvVars[strings.ToUpper(name)] {
		return true
	}
	return strings.HasPrefix(strings.ToUpper(name), "LLM_BOX_")
}
