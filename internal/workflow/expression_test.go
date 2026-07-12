package workflow

import (
	"testing"
)

func TestExpressionEngine_StepRef(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch_url", "https://example.com content")

	result, err := engine.Evaluate("{{step.0}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "https://example.com content" {
		t.Errorf("expected step output, got %q", result)
	}
}

func TestExpressionEngine_StepName(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch_url", "hello world")

	result, err := engine.Evaluate("{{step.fetch_url}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Errorf("expected step output by name, got %q", result)
	}
}

func TestExpressionEngine_Variable(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetVariable("api_url", "https://api.example.com")

	result, err := engine.Evaluate("POST to {{var.api_url}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "POST to https://api.example.com" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExpressionEngine_Input(t *testing.T) {
	engine := NewExpressionEngine()

	result, err := engine.Evaluate("Processing: {{input}}", "raw text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Processing: raw text" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExpressionEngine_MultipleExpressions(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch", "data1")
	engine.SetStepOutput(1, "process", "data2")

	result, err := engine.Evaluate("First: {{step.0}}, Second: {{step.1}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "First: data1, Second: data2"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestExpressionEngine_NoExpressions(t *testing.T) {
	engine := NewExpressionEngine()

	result, err := engine.Evaluate("plain text without expressions", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "plain text without expressions" {
		t.Errorf("expected unchanged text, got %q", result)
	}
}

func TestExpressionEngine_EmptyExpr(t *testing.T) {
	engine := NewExpressionEngine()

	result, err := engine.Evaluate("", "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestExpressionEngine_VariableNotFound(t *testing.T) {
	engine := NewExpressionEngine()

	result, err := engine.Evaluate("{{var.missing}}", "")
	if err == nil {
		t.Fatalf("expected error for missing variable, got nil")
	}
	if result != "{{var.missing}}" {
		t.Errorf("expected unchanged expression for missing variable, got %q", result)
	}
}

func TestExpressionEngine_StepNotFound(t *testing.T) {
	engine := NewExpressionEngine()

	result, err := engine.Evaluate("{{step.99}}", "")
	if err == nil {
		t.Fatalf("expected error for missing step, got nil")
	}
	if result != "{{step.99}}" {
		t.Errorf("expected unchanged expression for missing step, got %q", result)
	}
}

func TestExpressionEngine_UnknownPrefix(t *testing.T) {
	engine := NewExpressionEngine()

	result, err := engine.Evaluate("{{unknown.value}}", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "{{unknown.value}}" {
		t.Errorf("expected unchanged expression for unknown prefix, got %q", result)
	}
}

func TestExpressionEngine_GoTemplateSyntax(t *testing.T) {
	engine := NewExpressionEngine()

	result, err := engine.Evaluate("Hello {{.input}} - {{.Name}}", "world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "Hello {{.input}} - {{.Name}}"
	if result != expected {
		t.Errorf("expected Go template syntax preserved, got %q", result)
	}
}

func TestEvaluateParams(t *testing.T) {
	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch", "fetched data")
	engine.SetVariable("api_key", "secret123")

	params := map[string]string{
		"url":     "{{step.0}}",
		"headers": "Authorization: Bearer {{var.api_key}}",
		"plain":   "static value",
	}

	evaluated, err := engine.EvaluateParams(params, "input data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if evaluated["url"] != "fetched data" {
		t.Errorf("expected url='fetched data', got %q", evaluated["url"])
	}
	if evaluated["headers"] != "Authorization: Bearer secret123" {
		t.Errorf("expected headers with API key, got %q", evaluated["headers"])
	}
	if evaluated["plain"] != "static value" {
		t.Errorf("expected plain unchanged, got %q", evaluated["plain"])
	}
}

func TestEvaluateParams_Nil(t *testing.T) {
	engine := NewExpressionEngine()
	evaluated, err := engine.EvaluateParams(nil, "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluated != nil {
		t.Errorf("expected nil, got %v", evaluated)
	}
}

func TestContainsExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"hello {{name}}", true},
		{"{{step.0}}", true},
		{"plain text", false},
		{"{{ }}", true},
		{"", false},
	}

	for _, tt := range tests {
		result := ContainsExpression(tt.input)
		if result != tt.expected {
			t.Errorf("ContainsExpression(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}
