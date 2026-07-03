package workflow

import "time"

type Workflow struct {
	Name        string            `yaml:"name,omitempty"`
	Description string            `yaml:"description,omitempty"`
	Vars        map[string]string `yaml:"vars,omitempty"`
	Steps       []WorkflowStep    `yaml:"steps"`
}

type WorkflowStep struct {
	Node      string            `yaml:"node,omitempty"`
	Params    map[string]string `yaml:"params,omitempty"`
	Condition string            `yaml:"condition,omitempty"`
	Retry     int               `yaml:"retry,omitempty"`
	Delay     string            `yaml:"delay,omitempty"`
	Parallel  []Step            `yaml:"parallel,omitempty"`
}

type Step struct {
	Node      string            `yaml:"node"`
	Params    map[string]string `yaml:"params,omitempty"`
	Condition string            `yaml:"condition,omitempty"`
	Retry     int               `yaml:"retry,omitempty"`
	Delay     string            `yaml:"delay,omitempty"`
}

func (s *WorkflowStep) IsParallel() bool {
	return len(s.Parallel) > 0
}

func (s *WorkflowStep) GetTimeout() time.Duration {
	if timeout, ok := s.Params["_timeout"]; ok {
		d, err := time.ParseDuration(timeout)
		if err == nil && d > 0 {
			return d
		}
	}
	return 0
}

func (s *WorkflowStep) GetRetryCount() int {
	if s.Retry < 0 {
		return 0
	}
	return s.Retry
}

func (s *WorkflowStep) GetRetryDelay() time.Duration {
	if s.Delay == "" {
		return 1 * time.Second
	}
	d, err := time.ParseDuration(s.Delay)
	if err != nil {
		return 1 * time.Second
	}
	return d
}

// GetTimeout returns the per-step timeout from params._timeout, defaulting to 0 (no timeout)
func (s *Step) GetTimeout() time.Duration {
	if timeout, ok := s.Params["_timeout"]; ok {
		d, err := time.ParseDuration(timeout)
		if err == nil && d > 0 {
			return d
		}
	}
	return 0
}

// GetRetryCount returns the retry count, defaulting to 0
func (s *Step) GetRetryCount() int {
	if s.Retry < 0 {
		return 0
	}
	return s.Retry
}

// GetRetryDelay returns the delay between retries, defaulting to 1 second
func (s *Step) GetRetryDelay() time.Duration {
	if s.Delay == "" {
		return 1 * time.Second
	}
	d, err := time.ParseDuration(s.Delay)
	if err != nil {
		return 1 * time.Second
	}
	return d
}
