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

package workflow

import (
	"container/list"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

// ExpressionEngine evaluates {{...}} template expressions.
//
// WARNING: ExpressionEngine is NOT thread-safe. It holds mutable state
// (stepOutputs, variables, loopVars). All Evaluate/Set*/Clear* calls
// must be serialized on a single goroutine. The DAG executor enforces
// this by pre-evaluating all expressions on the main goroutine before
// dispatching to workers (see executor_dag.go "阶段 1").
//
// If you need concurrent evaluation, create separate Engine instances
// per goroutine (see SnapshotVars for deep-copying variables).
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

// SetStepOutput stores the output of a step for later reference.
// NOT thread-safe: must be called from the same goroutine that owns the engine.
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

// SetVariable sets a workflow-level variable.
// NOT thread-safe: must be called from the same goroutine that owns the engine.
func (e *ExpressionEngine) SetVariable(name, value string) {
	e.variables[name] = value
}

// GetVariable retrieves a workflow-level variable
func (e *ExpressionEngine) GetVariable(name string) (string, bool) {
	v, ok := e.variables[name]
	return v, ok
}

// SnapshotVars returns a copy of the current workflow-level variables.
// Used by map iterations to inherit the outer context into a per-item
// engine without sharing mutable state.
func (e *ExpressionEngine) SnapshotVars() map[string]string {
	out := make(map[string]string, len(e.variables))
	for k, v := range e.variables {
		out[k] = v
	}
	return out
}

// SetLoopVars sets the current loop context variables.
// NOT thread-safe: must be called from the same goroutine that owns the engine.
func (e *ExpressionEngine) SetLoopVars(item string, index, count int) {
	if e.loopVars == nil {
		e.loopVars = make(map[string]string)
	}
	e.loopVars["item"] = item
	e.loopVars["index"] = strconv.Itoa(index)
	e.loopVars["count"] = strconv.Itoa(count)
}

// ClearLoopVars removes the loop context.
// NOT thread-safe: must be called from the same goroutine that owns the engine.
func (e *ExpressionEngine) ClearLoopVars() {
	e.loopVars = nil
}

// SetReduceVars sets the reduce (fold) context variables. It populates the
// same loopVars map used by map/loop, so {{loop.acc}} / {{loop.item}} /
// {{loop.index}} / {{loop.count}} all resolve inside a reduce iteration.
// acc is the running accumulator; item is the current list element; index
// is the 0-based iteration index; count is the total number of items.
// NOT thread-safe: must be called from the same goroutine that owns the engine.
func (e *ExpressionEngine) SetReduceVars(acc, item string, index, count int) {
	if e.loopVars == nil {
		e.loopVars = make(map[string]string)
	}
	e.loopVars["acc"] = acc
	e.loopVars["item"] = item
	e.loopVars["index"] = strconv.Itoa(index)
	e.loopVars["count"] = strconv.Itoa(count)
}

// SetSecretGetter sets the function to retrieve secrets from the secret manager.
// NOT thread-safe: must be called from the same goroutine that owns the engine.
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
//	{{loop.acc}}                 - running accumulator (reduce only)
//
// Unknown expressions (e.g. Go template syntax {{.foo}}) are left unchanged.
//
// NOT thread-safe: must be called from the same goroutine that owns the engine.
func (e *ExpressionEngine) Evaluate(expr string, input string) (string, error) {
	if expr == "" {
		return "", nil
	}
	tmpl := compileTemplate(expr)
	if !tmpl.hasExpr {
		// Fast path: no expressions — return the literal verbatim.
		return tmpl.literal, nil
	}

	// Build the result into a pooled buffer. The backing []byte is reused
	// across calls; only the final string escapes to the heap.
	bufp := exprBufPool.Get().(*[]byte)
	buf := (*bufp)[:0]
	defer func() {
		*bufp = buf
		exprBufPool.Put(bufp)
	}()
	var firstErr error
	for i := range tmpl.instrs {
		ins := &tmpl.instrs[i]
		// resolveInstr is the shared dispatch core (also used by the
		// vectorised batch path). It returns the resolved value without
		// touching buf; the caller appends it, or — in the vectorised
		// single-expression fast path — stores it directly with no copy.
		// On error the caller records it via setFirstExprError and appends
		// the verbatim {{...}} fallback, mirroring the legacy semantics.
		if v, err := e.resolveInstr(ins, input); err != nil {
			setFirstExprError(ins, &firstErr, err)
			buf = append(buf, ins.fullMatch...)
		} else {
			buf = append(buf, v...)
		}
	}
	if firstErr != nil {
		return string(buf), firstErr
	}
	return string(buf), nil
}

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
	info, err := os.Stat(safePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file '%s': %w", name, err)
	}
	if info.Size() > maxExprFileSize {
		return "", fmt.Errorf("file '%s' too large (max %d bytes)", name, maxExprFileSize)
	}
	content, err := os.ReadFile(safePath) // #nosec G304 -- path validated by validateExprFilePath
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

// opcode is a single flat instruction in the compiled bytecode. The switch
// dispatch in evalInstr replaces interface calls (exprNode.eval), enabling
// branch prediction and eliminating virtual call overhead.
type opcode uint8

const (
	opLiteral  opcode = iota // append string literal (strArg)
	opInput                  // append workflow input
	opStep                   // append step output (strArg: name)
	opVar                    // append workflow variable (strArg: name)
	opEnv                    // append env var (strArg: name)
	opFile                   // append file contents (strArg: name)
	opLoop                   // append loop var (strArg: name)
	opSecret                 // append secret (strArg: GROUP.KEY)
	opBareName               // bare name: var or input (strArg: name)
	opJSONPath               // jsonpath extraction (strArg: ref inner, strArg2: path)
	opUnknown                // unknown prefix, leave verbatim (inner)
)

// instruction is a single bytecode operation with its operands.
type instruction struct {
	op        opcode
	strArg    string   // string operand (literal text, variable name, ref inner, etc.)
	strArg2   string   // second string operand (for opJSONPath: the path)
	known     bool     // whether this is a known-prefix expression (for error handling)
	fullMatch string   // original {{...}} text for verbatim fallback
	inner     string   // trimmed inner text for error messages
	refNode   exprNode // for opJSONPath: pre-compiled reference expression (avoids per-call alloc)
}

// compiledTemplate is the parsed form of a template string.
type compiledTemplate struct {
	// literal holds the entire template when it contains no expressions;
	// hasExpr is false and instrs is empty. This is the fast path.
	literal string
	hasExpr bool
	instrs  []instruction // flat bytecode (replaces parts)
}

// templatePart is either a literal segment or a compiled expression. Retained
// for the AST oracle (compileExpr + exprNode) used by differential tests and
// benchmarks; compiledTemplate now uses instrs []instruction instead.
type templatePart struct {
	isLiteral bool
	literal   string // valid when isLiteral
	node      exprNode
	fullMatch string // original "{{...}}" text, for verbatim fallback
	inner     string // trimmed inner text, for error messages
}

// templateCache memoises compiled templates by source text. Templates are
// immutable after construction; the cache is bounded (templateCacheCap) so a
// long-running process cannot grow without limit as distinct template strings
// accumulate. The bound is large enough that evictions are rare under normal
// use. The mutex-based LRU is safe for concurrent access from worker
// goroutines (e.g. parallel/loop sub-engines).
const templateCacheCap = 10000

var templateCache = newTemplateLRU(templateCacheCap)

// exprBufPool reuses the backing []byte of the Evaluate result buffer across
// calls. The result string is the only per-call allocation (strings are
// immutable), while the working buffer — which grows with every WriteString in
// the original strings.Builder path — is recycled. Pooling a *[]byte (rather
// than *strings.Builder) is required because strings.Builder.Reset() drops its
// backing array, defeating reuse.
var exprBufPool = sync.Pool{
	New: func() any { b := make([]byte, 0, 64); return &b },
}

// compileTemplate returns a cached compiled template for expr, parsing it on
// first encounter.
func compileTemplate(expr string) *compiledTemplate {
	if v, ok := templateCache.load(expr); ok {
		return v
	}
	tmpl := parseTemplate(expr)
	return templateCache.loadOrStore(expr, tmpl)
}

// templateLRU is a capacity-bounded LRU cache of compiled templates. It
// replaces the unbounded sync.Map so long-running processes don't leak memory.
type templateLRU struct {
	mu    sync.Mutex
	items map[string]*list.Element
	order *list.List // most-recently-used at the front
	cap   int
}

type templateLRUEntry struct {
	key   string
	value *compiledTemplate
}

func newTemplateLRU(cap int) *templateLRU {
	if cap <= 0 {
		cap = 1
	}
	return &templateLRU{
		items: make(map[string]*list.Element),
		order: list.New(),
		cap:   cap,
	}
}

// load returns the cached template for key (if present), promoting it to
// most-recently-used. Safe for concurrent use.
func (c *templateLRU) load(key string) (*compiledTemplate, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(elem)
	return elem.Value.(*templateLRUEntry).value, true
}

// loadOrStore returns the existing entry for key if present (promoting it),
// otherwise stores value and evicts the least-recently-used entry when over
// capacity. Safe for concurrent use.
func (c *templateLRU) loadOrStore(key string, value *compiledTemplate) *compiledTemplate {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		return elem.Value.(*templateLRUEntry).value
	}
	elem := c.order.PushFront(&templateLRUEntry{key: key, value: value})
	c.items[key] = elem
	if c.order.Len() > c.cap {
		if oldest := c.order.Back(); oldest != nil {
			entry := c.order.Remove(oldest).(*templateLRUEntry)
			delete(c.items, entry.key)
		}
	}
	return value
}

// parseTemplate scans s for {{...}} expressions, mirroring the legacy varPattern
// (`\{\{([^}]+)\}\}`): an expression is "{{" + one-or-more non-'}' chars + "}}",
// with the closing "}}" being the first '}' encountered. Non-matching text
// becomes literal segments. The result is a flat []instruction (bytecode)
// rather than []templatePart (AST nodes), so evaluation uses switch dispatch
// instead of interface dispatch.
func parseTemplate(s string) *compiledTemplate {
	// Fast path: no opening brace-pair, so no expression is possible.
	if !strings.Contains(s, "{{") {
		return &compiledTemplate{literal: s}
	}

	var instrs []instruction
	var lit strings.Builder
	n := len(s)
	i := 0
	flushLit := func() {
		if lit.Len() > 0 {
			instrs = append(instrs, instruction{op: opLiteral, strArg: lit.String()})
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
				ins := compileExprToInstr(inner)
				ins.fullMatch = s[i : k+2]
				ins.inner = inner
				instrs = append(instrs, ins)
				i = k + 2
				continue
			}
		}
		lit.WriteByte(s[i])
		i++
	}
	flushLit()

	// If no expression part was produced, the whole string is literal.
	for i := range instrs {
		if instrs[i].op != opLiteral {
			return &compiledTemplate{hasExpr: true, instrs: instrs}
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

// compileExprToInstr compiles a trimmed inner expression (the text between {{
// and }}) into a flat instruction. The dispatch mirrors compileExpr exactly;
// the resulting instruction is evaluated by evalInstr's switch at runtime,
// replacing the virtual call through exprNode.eval. fullMatch and inner are
// filled in by parseTemplate after this returns.
func compileExprToInstr(inner string) instruction {
	known := isKnownExpressionPrefix(inner)

	// jsonpath modifier: <ref>.jsonpath:<path>
	if idx := strings.Index(inner, ".jsonpath:"); idx > 0 {
		refPart := inner[:idx]
		jsonPath := inner[idx+len(".jsonpath:"):]
		return instruction{
			op:      opJSONPath,
			strArg:  refPart,
			strArg2: jsonPath,
			known:   known,
			refNode: compileExpr(refPart), // pre-compile ref to avoid per-eval allocation
		}
	}

	parts := strings.SplitN(inner, ".", 2)
	if len(parts) < 2 {
		// Bare name: a workflow variable, or the literal token "input".
		return instruction{op: opBareName, strArg: inner, known: known}
	}
	prefix := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	switch prefix {
	case "input":
		return instruction{op: opInput, known: known}
	case "step":
		return instruction{op: opStep, strArg: name, known: known}
	case "var":
		return instruction{op: opVar, strArg: name, known: known}
	case "env":
		return instruction{op: opEnv, strArg: name, known: known}
	case "file":
		return instruction{op: opFile, strArg: name, known: known}
	case "loop":
		return instruction{op: opLoop, strArg: name, known: known}
	case "secret":
		return instruction{op: opSecret, strArg: name, known: known}
	default:
		return instruction{op: opUnknown, known: known}
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
		return "", fmt.Errorf("secrets not available - use 'aflare secrets add' to store secrets first")
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

// EvaluateParams evaluates all string values in a params map.
// NOT thread-safe: must be called from the same goroutine that owns the engine.
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

// ContainsExpression reports whether a string contains any {{ ... }} expressions
func ContainsExpression(s string) bool {
	return varPattern.MatchString(s)
}

var allowedEnvVars = map[string]bool{
	"PATH":             true,
	"HOME":             true,
	"USER":             true,
	"LOGNAME":          true,
	"SHELL":            true,
	"LANG":             true,
	"LC_ALL":           true,
	"TERM":             true,
	"PWD":              true,
	"AFLARE_LANG":      true,
	"AFLARE_SAFE_MODE": true,
}

func isAllowedEnvVar(name string) bool {
	if allowedEnvVars[strings.ToUpper(name)] {
		return true
	}
	return strings.HasPrefix(strings.ToUpper(name), "AFLARE_")
}
