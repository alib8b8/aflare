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
	"context"
	"strings"
	"testing"
)

func TestDocParseNode_Metadata(t *testing.T) {
	node := &DocParseNode{}
	if node.Name() != "doc_parse" {
		t.Errorf("expected name 'doc_parse', got %q", node.Name())
	}
	if node.Description() == "" {
		t.Error("expected non-empty description")
	}
	schema := node.Schema()
	if schema.Name != "doc_parse" {
		t.Errorf("expected schema name 'doc_parse', got %q", schema.Name)
	}
	if schema.Description == "" {
		t.Error("expected non-empty schema description")
	}
	if len(schema.Params) == 0 {
		t.Error("expected non-empty schema params")
	}
}

func TestDocParseNode_TextInput(t *testing.T) {
	ctx := context.Background()
	node := &DocParseNode{}
	output, err := node.Execute(ctx, "hello world", map[string]string{
		"source":        "text",
		"output_format": "text",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "hello world") {
		t.Errorf("expected output to contain 'hello world', got %q", output)
	}
}

func TestDocParseNode_LatexTable(t *testing.T) {
	ctx := context.Background()
	node := &DocParseNode{}
	input := "| Name | Age |\n|------|-----|\n| Alice | 30 |\n"
	output, err := node.Execute(ctx, input, map[string]string{
		"source":        "text",
		"output_format": "latex",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "\\begin{tabular}") {
		t.Errorf("expected output to contain '\\begin{tabular}', got %q", output)
	}
	if !strings.Contains(output, "\\end{tabular}") {
		t.Errorf("expected output to contain '\\end{tabular}', got %q", output)
	}
	if !strings.Contains(output, "Alice") {
		t.Errorf("expected output to contain 'Alice', got %q", output)
	}
}

func TestDocParseNode_HTMLTable(t *testing.T) {
	ctx := context.Background()
	node := &DocParseNode{}
	input := "| Name | Age |\n|------|-----|\n| Alice | 30 |\n"
	output, err := node.Execute(ctx, input, map[string]string{
		"source":        "text",
		"output_format": "html_table",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "<table>") {
		t.Errorf("expected output to contain '<table>', got %q", output)
	}
	if !strings.Contains(output, "</table>") {
		t.Errorf("expected output to contain '</table>', got %q", output)
	}
	if !strings.Contains(output, "<th>Name</th>") {
		t.Errorf("expected output to contain '<th>Name</th>', got %q", output)
	}
	if !strings.Contains(output, "<td>Alice</td>") {
		t.Errorf("expected output to contain '<td>Alice</td>', got %q", output)
	}
}

func TestDocParseNode_ExtractTables(t *testing.T) {
	ctx := context.Background()
	node := &DocParseNode{}
	input := "Intro text.\n\n| Name | Age |\n|------|-----|\n| Alice | 30 |\n\nMore text.\n"
	output, err := node.Execute(ctx, input, map[string]string{
		"source":         "text",
		"extract_tables": "true",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Tables (1 found)") {
		t.Errorf("expected 'Tables (1 found)' in output, got %q", output)
	}
	if !strings.Contains(output, "Alice") {
		t.Errorf("expected table content 'Alice' in output, got %q", output)
	}
}

func TestDocParseNode_ExtractFormulas(t *testing.T) {
	ctx := context.Background()
	node := &DocParseNode{}
	input := "The famous formula $$E=mc^2$$ shows mass-energy equivalence.\n"
	output, err := node.Execute(ctx, input, map[string]string{
		"source":           "text",
		"extract_formulas": "true",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Formulas (1 found)") {
		t.Errorf("expected 'Formulas (1 found)' in output, got %q", output)
	}
	if !strings.Contains(output, "E=mc^2") {
		t.Errorf("expected output to contain formula 'E=mc^2', got %q", output)
	}
}

func TestDocParseNode_MissingSource(t *testing.T) {
	ctx := context.Background()
	node := &DocParseNode{}
	// An invalid source value should be rejected.
	_, err := node.Execute(ctx, "hello", map[string]string{
		"source": "invalid_source",
	})
	if err == nil {
		t.Error("expected error for invalid source value")
	}
	if !strings.Contains(err.Error(), "invalid source") {
		t.Errorf("expected error to mention 'invalid source', got %v", err)
	}
}

func TestDocParseNode_Base64NoAPI(t *testing.T) {
	ctx := context.Background()
	node := &DocParseNode{}
	// "aGVsbG8=" is base64 for "hello".
	encoded := "aGVsbG8="
	_, err := node.Execute(ctx, encoded, map[string]string{
		"source": "base64",
	})
	if err == nil {
		t.Fatal("expected error for base64 source without API config")
	}
	if !strings.Contains(err.Error(), "api_endpoint") {
		t.Errorf("expected error to mention 'api_endpoint', got %v", err)
	}
}

func TestDocParseNode_InvalidURL(t *testing.T) {
	ctx := context.Background()
	node := &DocParseNode{}
	// localhost is rejected by validateURL (SSRF protection).
	_, err := node.Execute(ctx, "http://localhost:9999", map[string]string{
		"source": "URL",
	})
	if err == nil {
		t.Error("expected error for localhost URL (should be rejected by validateURL)")
	}
}

func TestDocParseNode_TableCount(t *testing.T) {
	ctx := context.Background()
	node := &DocParseNode{}
	input := "Intro text.\n\n" +
		"| Name | Age |\n|------|-----|\n| Alice | 30 |\n\n" +
		"Middle text.\n\n" +
		"| Fruit | Color |\n|-------|-------|\n| Apple | Red |\n\n" +
		"End text.\n"
	output, err := node.Execute(ctx, input, map[string]string{
		"source":         "text",
		"extract_tables": "true",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Tables (2 found)") {
		t.Errorf("expected 'Tables (2 found)' in output, got %q", output)
	}
	if !strings.Contains(output, "Alice") {
		t.Errorf("expected first table content 'Alice' in output, got %q", output)
	}
	if !strings.Contains(output, "Apple") {
		t.Errorf("expected second table content 'Apple' in output, got %q", output)
	}
}
