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
	"testing"
)

func TestNodeRegistry(t *testing.T) {
	// Test that test_node is registered
	node, ok := Get("test_node")
	if !ok {
		t.Fatal("test_node not found in registry")
	}
	if node.Name() != "test_node" {
		t.Errorf("expected node name 'test_node', got '%s'", node.Name())
	}

	// Test List() returns registered nodes
	names := List()
	found := false
	for _, name := range names {
		if name == "test_node" {
			found = true
			break
		}
	}
	if !found {
		t.Error("test_node not found in List() output")
	}
}

func TestTestNodeExecute(t *testing.T) {
	node, ok := Get("test_node")
	if !ok {
		t.Fatal("test_node not found")
	}

	ctx := context.Background()
	input := "test input"
	params := map[string]string{"message": "test message"}

	output, err := node.Execute(ctx, input, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expected := "Input: test input\nMessage: test message"
	if output != expected {
		t.Errorf("expected output '%s', got '%s'", expected, output)
	}
}
