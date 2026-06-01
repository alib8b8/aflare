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
type Registry struct {
	nodes map[string]Node
}

// NewRegistry creates a new registry
func NewRegistry() *Registry {
	return &Registry{
		nodes: make(map[string]Node),
	}
}

// Register adds a node to the registry
func (r *Registry) Register(node Node) {
	r.nodes[node.Name()] = node
}

// Get retrieves a node from the registry by name
func (r *Registry) Get(name string) (Node, bool) {
	node, ok := r.nodes[name]
	return node, ok
}

// List returns all registered node names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.nodes))
	for name := range r.nodes {
		names = append(names, name)
	}
	return names
}

// Global registry for backwards compatibility
var globalRegistry = NewRegistry()

// Register adds a node to the global registry
func Register(node Node) {
	globalRegistry.Register(node)
}

// Get retrieves a node from the global registry by name
func Get(name string) (Node, bool) {
	return globalRegistry.Get(name)
}

// List returns all registered node names from the global registry
func List() []string {
	return globalRegistry.List()
}

// GetGlobalRegistry returns the global registry
func GetGlobalRegistry() *Registry {
	return globalRegistry
}
