package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type PipelineStep struct {
	Name      string            `json:"name"`
	Node      string            `json:"node"`
	Input     string            `json:"input,omitempty"`
	Params    map[string]string `json:"params,omitempty"`
	DependsOn []string          `json:"depends_on,omitempty"`
	InputFrom []string          `json:"input_from,omitempty"`
}

type PipelineConfig struct {
	Steps          []PipelineStep `json:"steps"`
	TimeoutSeconds int            `json:"timeout_seconds,omitempty"`
}

type PipelineStepResult struct {
	Name     string        `json:"name"`
	Node     string        `json:"node"`
	Output   string        `json:"output"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration_ms"`
	Started  time.Time     `json:"started_at"`
	Finished time.Time     `json:"finished_at"`
}

type PipelineResult struct {
	Success bool                 `json:"success"`
	Results []PipelineStepResult `json:"results"`
	TotalMs int64                `json:"total_duration_ms"`
	Errors  []string             `json:"errors,omitempty"`
}

type PipelineNode struct{}

func init() {
	Register(&PipelineNode{})
}

func (n *PipelineNode) Name() string {
	return "pipeline"
}

func (n *PipelineNode) Description() string {
	return "Execute steps with dependency-based parallel scheduling (no-barrier, Tunix-inspired)"
}

func (n *PipelineNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "pipeline",
		Description: "Dependency-based parallel workflow executor: steps run as soon as their dependencies are met, no global barriers (Tunix-inspired async rollout)",
		Input:       "string - YAML or JSON pipeline configuration with steps and dependencies",
		Output:      "string - JSON with execution results, timings, and errors",
		Params: []ParamSchema{
			{Name: "timeout", Type: "string", Description: "Timeout in seconds (default: 300)", Required: false, Default: "300"},
			{Name: "format", Type: "string", Description: "Input format: json|yaml|auto (default: auto)", Required: false, Default: "auto"},
		},
	}
}

func (n *PipelineNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("pipeline configuration required")
	}

	timeoutStr := getParam(params, "timeout", "300")
	timeoutSeconds := 300
	fmt.Sscanf(timeoutStr, "%d", &timeoutSeconds)

	config, err := parsePipelineConfig(input)
	if err != nil {
		return "", fmt.Errorf("failed to parse pipeline config: %w", err)
	}

	if len(config.Steps) == 0 {
		return "", fmt.Errorf("pipeline has no steps")
	}

	if config.TimeoutSeconds > 0 {
		timeoutSeconds = config.TimeoutSeconds
	}

	reg := GetGlobalRegistry()

	pipelineCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	return runPipeline(pipelineCtx, config, timeoutSeconds, reg)
}

func parsePipelineConfig(input string) (PipelineConfig, error) {
	input = strings.TrimSpace(input)

	if strings.HasPrefix(input, "{") || strings.HasPrefix(input, "[") {
		var config PipelineConfig
		if err := json.Unmarshal([]byte(input), &config); err != nil {
			return PipelineConfig{}, fmt.Errorf("invalid JSON: %w", err)
		}
		return config, nil
	}

	return PipelineConfig{}, fmt.Errorf("unsupported format, use JSON")
}

func runPipeline(ctx context.Context, config PipelineConfig, timeoutSeconds int, reg *Registry) (string, error) {
	startTime := time.Now()

	stepMap := make(map[string]*PipelineStep)
	for i := range config.Steps {
		s := &config.Steps[i]
		if s.Name == "" {
			s.Name = fmt.Sprintf("step_%d", i)
		}
		stepMap[s.Name] = s
	}

	for name, s := range stepMap {
		for _, dep := range s.DependsOn {
			if _, ok := stepMap[dep]; !ok {
				return "", fmt.Errorf("step %q depends on unknown step %q", name, dep)
			}
		}
	}

	results := make(map[string]*PipelineStepResult)
	resultsMu := sync.Mutex{}

	completed := make(map[string]bool)
	completedMu := sync.Mutex{}

	var wg sync.WaitGroup
	started := make(map[string]bool)
	startedMu := sync.Mutex{}

	var allErrors []string
	errorsMu := sync.Mutex{}

	tryStart := func() bool {
		startedMu.Lock()
		completedMu.Lock()

		var toStart []*PipelineStep
		for name, step := range stepMap {
			if started[name] || completed[name] {
				continue
			}

			depsMet := true
			for _, dep := range step.DependsOn {
				if !completed[dep] {
					depsMet = false
					break
				}
			}

			if depsMet {
				started[name] = true
				toStart = append(toStart, step)
			}
		}

		startedMu.Unlock()
		completedMu.Unlock()

		for _, step := range toStart {
			wg.Add(1)
			go func(s *PipelineStep) {
				defer wg.Done()

				stepStart := time.Now()
				result := &PipelineStepResult{
					Name:    s.Name,
					Node:    s.Node,
					Started: stepStart,
				}

				node, ok := reg.Get(s.Node)
				if !ok {
					result.Error = fmt.Sprintf("node %q not found", s.Node)
					result.Finished = time.Now()
					result.Duration = result.Finished.Sub(result.Started)
					resultsMu.Lock()
					results[s.Name] = result
					resultsMu.Unlock()
					errorsMu.Lock()
					allErrors = append(allErrors, fmt.Sprintf("%s: %s", s.Name, result.Error))
					errorsMu.Unlock()
					completedMu.Lock()
					completed[s.Name] = true
					completedMu.Unlock()
					return
				}

				stepInput := s.Input
				if len(s.InputFrom) > 0 {
					var parts []string
					for _, src := range s.InputFrom {
						resultsMu.Lock()
						if r, ok := results[src]; ok && r.Error == "" {
							parts = append(parts, r.Output)
						}
						resultsMu.Unlock()
					}
					if stepInput == "" {
						stepInput = strings.Join(parts, "\n\n")
					}
				}

				stepParams := make(map[string]string)
				for k, v := range s.Params {
					stepParams[k] = v
				}

				output, execErr := node.Execute(ctx, stepInput, stepParams)
				if execErr != nil {
					result.Error = execErr.Error()
					errorsMu.Lock()
					allErrors = append(allErrors, fmt.Sprintf("%s: %s", s.Name, execErr.Error()))
					errorsMu.Unlock()
				} else {
					result.Output = output
				}

				result.Finished = time.Now()
				result.Duration = result.Finished.Sub(result.Started)

				resultsMu.Lock()
				results[s.Name] = result
				resultsMu.Unlock()

				completedMu.Lock()
				completed[s.Name] = true
				completedMu.Unlock()
			}(step)
		}

		return len(toStart) > 0
	}

	done := make(chan bool)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			startedMu.Lock()
			completedMu.Lock()
			totalRunning := len(started) - len(completed)
			allDone := len(completed) == len(stepMap)
			startedMu.Unlock()
			completedMu.Unlock()

			if allDone {
				close(done)
				return
			}

			startedAny := tryStart()
			if !startedAny && totalRunning == 0 {
				completedMu.Lock()
				remaining := len(stepMap) - len(completed)
				completedMu.Unlock()
				if remaining > 0 {
					errorsMu.Lock()
					allErrors = append(allErrors, fmt.Sprintf("deadlock: %d steps cannot start due to unmet dependencies", remaining))
					errorsMu.Unlock()
				}
				close(done)
				return
			}

			time.Sleep(10 * time.Millisecond)
		}
	}()

	select {
	case <-done:
	case <-ctx.Done():
		errorsMu.Lock()
		allErrors = append(allErrors, fmt.Sprintf("pipeline timeout after %d seconds", timeoutSeconds))
		errorsMu.Unlock()
	}

	wg.Wait()

	orderedResults := make([]PipelineStepResult, 0, len(config.Steps))
	for _, s := range config.Steps {
		resultsMu.Lock()
		if r, ok := results[s.Name]; ok {
			orderedResults = append(orderedResults, *r)
		}
		resultsMu.Unlock()
	}

	pipelineResult := PipelineResult{
		Success: len(allErrors) == 0,
		Results: orderedResults,
		TotalMs: time.Since(startTime).Milliseconds(),
		Errors:  allErrors,
	}

	output, err := json.MarshalIndent(pipelineResult, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(output), nil
}
