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

import "context"

// TestNode is a simple test node for development
type TestNode struct{}

func init() {
	Register(&TestNode{})
}

// Name returns the node name
func (n *TestNode) Name() string {
	return "test_node"
}

func (n *TestNode) Description() string {
	return "Test node for development purposes"
}

func (n *TestNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "test_node",
		Description: "Test node for development purposes",
		Input:       "string - test input",
		Output:      "string - test output with input and message",
		Params: []ParamSchema{
			{Name: "message", Type: "string", Description: "Test message (default: Hello from test node!)", Required: false, Default: "Hello from test node!"},
		},
	}
}

// Execute implements the Node interface
func (n *TestNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	message, ok := params["message"]
	if !ok {
		message = "Hello from test node!"
	}
	return "Input: " + input + "\nMessage: " + message, nil
}
