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
