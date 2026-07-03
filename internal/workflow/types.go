package workflow

// Workflow represents a complete workflow definition
type Workflow struct {
	Name        string `yaml:"name,omitempty"`
	Description string `yaml:"description,omitempty"`
	Steps       []Step `yaml:"steps"`
}

// Step represents a single step in the workflow
type Step struct {
	Node   string            `yaml:"node"`
	Params map[string]string `yaml:"params,omitempty"`
}
