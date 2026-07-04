package nodes

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"
)

type TemplateRenderNode struct{}

func init() {
	Register(&TemplateRenderNode{})
}

func (n *TemplateRenderNode) Name() string {
	return "template_render"
}

func (n *TemplateRenderNode) Description() string {
	return "Render Go templates with input data"
}

func (n *TemplateRenderNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "template_render",
		Description: "Render Go templates with input data",
		Input:       "string - input data available as .input in template",
		Output:      "string - rendered template output",
		Params: []ParamSchema{
			{Name: "template", Type: "string", Description: "Inline template string", Required: false},
			{Name: "template_file", Type: "string", Description: "Path to template file (takes precedence over template)", Required: false},
		},
	}
}

func (n *TemplateRenderNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	var templateStr string

	templateFile, ok := params["template_file"]
	if ok && templateFile != "" {
		safePath, err := validateReadPath(templateFile)
		if err != nil {
			return "", fmt.Errorf("template file path validation failed: %w", err)
		}
		data, err := os.ReadFile(safePath)
		if err != nil {
			return "", fmt.Errorf("failed to read template file: %w", err)
		}
		templateStr = string(data)
	} else {
		templateStr, ok = params["template"]
		if !ok || templateStr == "" {
			return "", fmt.Errorf("template or template_file parameter is required")
		}
	}

	data := map[string]interface{}{
		"input": input,
	}
	for k, v := range params {
		if k != "template" && k != "template_file" {
			data[k] = v
		}
	}

	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": strings.ToTitle,
		"trim":  strings.TrimSpace,
		"len":   func(s string) int { return len(s) },
	}

	tmpl, err := template.New("template").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return buf.String(), nil
}
