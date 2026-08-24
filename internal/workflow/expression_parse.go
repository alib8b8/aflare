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
	"strings"
)

// compileTemplate returns a cached compiled template for expr, parsing it on
// first encounter.
func compileTemplate(expr string) *compiledTemplate {
	if v, ok := templateCache.load(expr); ok {
		return v
	}
	tmpl := parseTemplate(expr)
	return templateCache.loadOrStore(expr, tmpl)
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
