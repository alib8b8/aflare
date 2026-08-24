// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌‌‌‌‌​‌‌‌​​‌​‌‌‌‌‌‌‌‌​‌‌​‌​‌​​​​‌​​​‌‌​​​‌​​‌​​​​​​​​​​​​​​​​​​​‌​​​‌​​‌​​‌​​​⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Command gen-nodes-doc generates a Markdown reference of all registered nodes
// by introspecting their Schema() metadata. Run: go run ./cmd/gen-nodes-doc
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/alib8b8/aflare/internal/nodes"
)

func main() {
	names := nodes.List()
	sort.Strings(names)

	var sb strings.Builder

	sb.WriteString("# Node Reference\n\n")
	sb.WriteString(fmt.Sprintf("> Auto-generated from `Schema()` metadata. %d nodes registered.\n\n", len(names)))
	sb.WriteString("| Node | Description | Params |\n")
	sb.WriteString("|------|-------------|--------|\n")

	for _, name := range names {
		node, ok := nodes.Get(name)
		if !ok {
			continue
		}
		schema := node.Schema()
		desc := strings.ReplaceAll(schema.Description, "\n", " ")
		if len(desc) > 120 {
			desc = desc[:117] + "..."
		}
		paramCount := len(schema.Params)
		sb.WriteString(fmt.Sprintf("| [`%s`](#%s) | %s | %d |\n", schema.Name, schema.Name, desc, paramCount))
	}

	sb.WriteString("\n---\n\n")

	for _, name := range names {
		node, ok := nodes.Get(name)
		if !ok {
			continue
		}
		schema := node.Schema()

		sb.WriteString(fmt.Sprintf("## %s\n\n", schema.Name))
		sb.WriteString(fmt.Sprintf("%s\n\n", schema.Description))
		sb.WriteString(fmt.Sprintf("- **Input**: %s\n", schema.Input))
		sb.WriteString(fmt.Sprintf("- **Output**: %s\n", schema.Output))

		if len(schema.Params) > 0 {
			sb.WriteString("\n### Parameters\n\n")
			sb.WriteString("| Name | Type | Required | Default | Description |\n")
			sb.WriteString("|------|------|----------|---------|-------------|\n")
			for _, p := range schema.Params {
				req := "No"
				if p.Required {
					req = "Yes"
				}
				sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s |\n", p.Name, p.Type, req, p.Default, p.Description))
			}
		}
		sb.WriteString("\n---\n\n")
	}

	output := "docs/nodes-reference.md"
	if err := os.WriteFile(output, []byte(sb.String()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write %s: %v\n", output, err)
		os.Exit(1)
	}
	fmt.Printf("Generated %s with %d nodes\n", output, len(names))
}
