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
	"container/list"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

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

// snapshot returns a deep copy of the engine that is safe for concurrent use.
// The returned engine shares no mutable state with the original — all maps are
// deep-copied — so it can be passed to a worker goroutine without data races.
func (e *ExpressionEngine) snapshot() *ExpressionEngine {
	s := &ExpressionEngine{
		stepOutputs:  make(map[string]string, len(e.stepOutputs)),
		stepNames:    make(map[int]string, len(e.stepNames)),
		variables:    make(map[string]string, len(e.variables)),
		secretGetter: e.secretGetter, // immutable func pointer, safe to share
	}
	for k, v := range e.stepOutputs {
		s.stepOutputs[k] = v
	}
	for k, v := range e.stepNames {
		s.stepNames[k] = v
	}
	for k, v := range e.variables {
		s.variables[k] = v
	}
	if e.loopVars != nil {
		s.loopVars = make(map[string]string, len(e.loopVars))
		for k, v := range e.loopVars {
			s.loopVars[k] = v
		}
	}
	return s
}

// WithOutput returns a snapshot of the engine with an additional step output
// pre-registered. The returned engine is independent — DAG workers can safely
// evaluate expressions on it concurrently without data races on the original.
func (e *ExpressionEngine) WithOutput(name, value string) *ExpressionEngine {
	snapshot := e.snapshot()
	snapshot.stepOutputs["name:"+name] = value
	return snapshot
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
