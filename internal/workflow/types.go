// Copyright (c) 2026 llm-box Contributors
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

package workflow

import "time"

type Workflow struct {
	Name           string            `yaml:"name,omitempty"`
	Description    string            `yaml:"description,omitempty"`
	Vars           map[string]string `yaml:"vars,omitempty"`
	Steps          []WorkflowStep    `yaml:"steps"`
	Output         string            `yaml:"output,omitempty"`          // expression for final output (default: last step output)
	InputSchema    []InputField      `yaml:"input_schema,omitempty"`    // optional input validation
	MaxConcurrency int               `yaml:"max_concurrency,omitempty"` // global concurrency limit (default: 0=unlimited)
}

// InputField defines an expected input parameter for schema validation.
type InputField struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"` // string, int, bool, json
	Required bool   `yaml:"required,omitempty"`
	Default  string `yaml:"default,omitempty"`
}

type WorkflowStep struct {
	Node            string            `yaml:"node,omitempty"`
	Name            string            `yaml:"name,omitempty"` // optional step name for {{step.name}} reference
	Params          map[string]string `yaml:"params,omitempty"`
	Condition       string            `yaml:"condition,omitempty"`
	Retry           int               `yaml:"retry,omitempty"`
	Delay           string            `yaml:"delay,omitempty"`
	Backoff         *BackoffConfig    `yaml:"backoff,omitempty"`
	Parallel        []Step            `yaml:"parallel,omitempty"`
	ContinueOnError bool              `yaml:"continue_on_error,omitempty"`
	Fallback        string            `yaml:"fallback,omitempty"`
	OnError         *Step             `yaml:"on_error,omitempty"`
	Loop            *LoopConfig       `yaml:"loop,omitempty"`
	MaxFailures     int               `yaml:"max_failures,omitempty"`
	If              *IfConfig         `yaml:"if,omitempty"`
	OutputStrategy  string            `yaml:"output_strategy,omitempty"` // parallel/loop: join(default), first, last, json_array, longest, shortest
}

// BackoffConfig configures exponential backoff for retries.
type BackoffConfig struct {
	Exponential bool   `yaml:"exponential,omitempty"` // enable exponential backoff
	Base        string `yaml:"base,omitempty"`        // base delay (default: same as delay)
	MaxDelay    string `yaml:"max_delay,omitempty"`   // max delay cap (default: MaxRetryDelay)
	Jitter      bool   `yaml:"jitter,omitempty"`      // add random jitter
}

// IfConfig defines an if/else branch.
type IfConfig struct {
	Condition string         `yaml:"condition"`
	Then      []WorkflowStep `yaml:"then"`
	Else      []WorkflowStep `yaml:"else,omitempty"`
}

// LoopConfig configures batch iteration over a list of items.
type LoopConfig struct {
	Items         string `yaml:"items"`
	SplitBy       string `yaml:"split_by,omitempty"`
	Var           string `yaml:"var,omitempty"`
	Concurrency   int    `yaml:"concurrency,omitempty"`
	StopOnError   *bool  `yaml:"stop_on_error,omitempty"`
	MaxIterations int    `yaml:"max_iterations,omitempty"`
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

func (s *WorkflowStep) IsLoop() bool {
	return s.Loop != nil
}

func (s *WorkflowStep) IsIf() bool {
	return s.If != nil
}

// GetSplitBy returns the delimiter for splitting loop items (default: newline).
func (l *LoopConfig) GetSplitBy() string {
	if l.SplitBy == "" {
		return "\n"
	}
	return l.SplitBy
}

// GetVar returns the loop variable name (default: "item").
func (l *LoopConfig) GetVar() string {
	if l.Var == "" {
		return "item"
	}
	return l.Var
}

// GetConcurrency returns the max concurrent iterations (default: 1, capped at MaxParallel).
func (l *LoopConfig) GetConcurrency() int {
	if l.Concurrency <= 0 {
		return 1
	}
	if l.Concurrency > MaxParallel {
		return MaxParallel
	}
	return l.Concurrency
}

// GetStopOnError returns whether to stop on first error (default: true).
func (l *LoopConfig) GetStopOnError() bool {
	if l.StopOnError == nil {
		return true
	}
	return *l.StopOnError
}

// GetMaxIterations returns the safety limit (default: 100, capped at 10000).
func (l *LoopConfig) GetMaxIterations() int {
	if l.MaxIterations <= 0 {
		return 100
	}
	if l.MaxIterations > 10000 {
		return 10000
	}
	return l.MaxIterations
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

// GetBackoffDelay computes the retry delay for a given attempt using backoff config.
// attempt is 1-indexed (1 = first retry).
func (s *WorkflowStep) GetBackoffDelay(attempt int) time.Duration {
	baseDelay := s.GetRetryDelay()
	if s.Backoff == nil || !s.Backoff.Exponential || attempt <= 1 {
		return baseDelay
	}

	// Parse custom base if provided, capped at MaxRetryDelay
	if s.Backoff.Base != "" {
		if d, err := time.ParseDuration(s.Backoff.Base); err == nil && d > 0 {
			if d > MaxRetryDelay {
				d = MaxRetryDelay
			}
			baseDelay = d
		}
	}

	// Determine max delay cap
	maxDelay := MaxRetryDelay
	if s.Backoff.MaxDelay != "" {
		if d, err := time.ParseDuration(s.Backoff.MaxDelay); err == nil && d > 0 {
			if d > MaxRetryDelay {
				d = MaxRetryDelay
			}
			maxDelay = d
		}
	}

	// Exponential: base * 2^(attempt-1), with overflow protection
	delay := baseDelay
	for i := 1; i < attempt; i++ {
		// Check for overflow before multiplying
		if delay > maxDelay/2 {
			delay = maxDelay
			break
		}
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
			break
		}
	}

	// Jitter: add up to 25% random variation
	if s.Backoff.Jitter && delay > 0 {
		delay = time.Duration(float64(delay) * (0.75 + 0.25*pseudoRand()))
	}

	return delay
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
