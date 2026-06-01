package nodes

import "context"

// Node defines the interface that all workflow nodes must implement
type Node interface {
	// Name returns the unique identifier of this node
	Name() string
	
	// Execute runs the node with the given input and parameters
	Execute(ctx context.Context, input string, params map[string]string) (string, error)
}

// Registry keeps track of all available nodes
var registry = make(map[string]Node)

// Register adds a node to the registry
func Register(node Node) {
	registry[node.Name()] = node
}

// Get retrieves a node from the registry by name
func Get(name string) (Node, bool) {
	node, ok := registry[name]
	return node, ok
}

// List returns all registered node names
func List() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
