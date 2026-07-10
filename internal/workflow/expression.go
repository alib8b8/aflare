package workflow

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ExpressionEngine handles template variable substitution
type ExpressionEngine struct {
	// Outputs from previous steps, keyed by step name
	stepOutputs map[string]string
	// Step index to name mapping
	stepNames map[int]string
	// Workflow-level variables
	variables map[string]string
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
	key := fmt.Sprintf("%d", stepIndex)
	e.stepNames[stepIndex] = stepName
	e.stepOutputs[key] = output
	e.stepOutputs[stepName] = output
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

// varPattern matches {{ ... }} expressions
var varPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// Evaluate evaluates an expression and returns the result
// Supports:
//
//	{{input}}       - the workflow's initial input
//	{{step.N}}      - output of step N (0-indexed)
//	{{step.name}}   - output of step by name
//	{{var.NAME}}    - workflow variable
//	{{env.NAME}}    - environment variable
//	{{file.PATH}}   - file contents (handled by executor)
//
// Unknown expressions (e.g. Go template syntax {{.foo}}) are left unchanged.
func (e *ExpressionEngine) Evaluate(expr string, input string) (string, error) {
	if expr == "" {
		return "", nil
	}

	result := varPattern.ReplaceAllStringFunc(expr, func(match string) string {
		inner := strings.TrimSpace(match[2 : len(match)-2])
		value, err := e.evalSingle(inner, input)
		if err != nil {
			return match
		}
		return value
	})

	return result, nil
}

// evalSingle evaluates a single expression like "step.0" or "var.name"
func (e *ExpressionEngine) evalSingle(expr string, input string) (string, error) {
	parts := strings.SplitN(expr, ".", 2)
	if len(parts) < 2 {
		if v, ok := e.variables[expr]; ok {
			return v, nil
		}
		if expr == "input" {
			return input, nil
		}
		return "", fmt.Errorf("unknown expression: %s", expr)
	}

	prefix := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])

	switch prefix {
	case "input":
		return input, nil
	case "step":
		return e.evalStepRef(name)
	case "var":
		if v, ok := e.variables[name]; ok {
			return v, nil
		}
		return "", fmt.Errorf("variable not found: %s", name)
	case "env":
		if v, ok := os.LookupEnv(name); ok {
			return v, nil
		}
		return "", fmt.Errorf("environment variable not found: %s", name)
	case "file":
		return "", fmt.Errorf("file expression: %s", expr)
	default:
		return "", fmt.Errorf("unknown expression: %s", expr)
	}
}

// evalStepRef resolves a step reference by index or name
func (e *ExpressionEngine) evalStepRef(name string) (string, error) {
	// stepOutputs is keyed by step index (and optionally by step name when
	// the executor populates both). A single lookup suffices.
	if output, ok := e.stepOutputs[name]; ok {
		return output, nil
	}

	return "", fmt.Errorf("step reference not found: %s", name)
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
