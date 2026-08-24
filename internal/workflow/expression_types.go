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
	"fmt"
	"os"
	"strings"
)

const maxExprFileSize = 10 * 1024 * 1024 // 10MB

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

type wildcardMarker struct{}

const maxJSONPathDepth = 10

// recursiveFind finds all values for a given field name at any depth.
// maxDepth limits recursion to prevent stack overflow from deeply nested JSON.
const maxRecursiveDepth = 1000
