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

// Execute implements the Node interface
func (n *TestNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	message, ok := params["message"]
	if !ok {
		message = "Hello from test node!"
	}
	return "Input: " + input + "\nMessage: " + message, nil
}
