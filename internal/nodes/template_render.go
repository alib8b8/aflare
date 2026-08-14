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

package nodes

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
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
		data, err := os.ReadFile(safePath) // #nosec G304 -- path validated by validateReadPath
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
		if k != "template" && k != "template_file" && !isSensitiveKey(k) {
			data[k] = v
		}
	}

	funcMap := sprig.FuncMap()
	// Remove functions that can read host environment variables. The
	// workflow expression engine ({{env.NAME}}) has a strict env allowlist,
	// but Sprig's env/expandenv would bypass it — letting a malicious
	// template exfiltrate secrets like {{ env "AWS_SECRET_ACCESS_KEY" }}.
	delete(funcMap, "env")
	delete(funcMap, "expandenv")
	funcMap["now"] = func() string { return time.Now().Format(time.RFC3339) }

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
