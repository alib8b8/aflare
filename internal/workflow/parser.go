package workflow

import (
	"os"

	"gopkg.in/yaml.v3"
)

// ParseWorkflow parses a YAML file into a Workflow structure
func ParseWorkflow(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, err
	}

	return &wf, nil
}
