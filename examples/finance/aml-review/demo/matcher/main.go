// Copyright (c) 2026 llm-box Contributors
//
// Scene matcher: maps a free-text business event to a workflow path.
//
// Reads a scene registry (scenes.yaml), scores each scene by keyword-hit
// count against the input event, and prints the best-matching scene's
// workflow path as a machine-readable JSON line. This demonstrates
// "业务事件→工作流自动路由" for the finance industry.
//
// Usage:
//
//	go run ./demo/matcher -scenes scenes.yaml -event "收到一笔可疑交易告警，需查关联方黑名单"
//	# => {"scene_id":"aml-transaction-review","score":3,"workflow":"examples/finance/aml-review/workflow.yaml"}
//
// No-match prints scene_id "unknown" and exits 0, so the caller can decide
// whether to fall back to a default workflow or escalate to a human.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type scene struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Industry    string   `yaml:"industry"`
	Keywords    []string `yaml:"keywords"`
	Workflow    string   `yaml:"workflow"`
	Description string   `yaml:"description"`
}

type registry struct {
	Scenes []scene `yaml:"scenes"`
}

type matchResult struct {
	SceneID  string `json:"scene_id"`
	SceneName string `json:"scene_name,omitempty"`
	Score    int    `json:"score"`
	Workflow string `json:"workflow"`
	Event    string `json:"event"`
}

func main() {
	scenesPath := flag.String("scenes", "examples/finance/aml-review/scenes.yaml", "path to scenes registry YAML")
	event := flag.String("event", "", "business event text to match")
	flag.Parse()

	if *event == "" {
		fmt.Fprintln(os.Stderr, "error: -event is required")
		os.Exit(2)
	}

	data, err := os.ReadFile(*scenesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to read scenes file %q: %v\n", *scenesPath, err)
		os.Exit(1)
	}

	var reg registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse scenes YAML: %v\n", err)
		os.Exit(1)
	}

	// Score: case-insensitive substring hit count.
	eventLower := strings.ToLower(*event)
	best := matchResult{SceneID: "unknown", Score: 0, Event: *event}
	for _, s := range reg.Scenes {
		score := 0
		for _, kw := range s.Keywords {
			if strings.Contains(eventLower, strings.ToLower(kw)) {
				score++
			}
		}
		if score > best.Score {
			best = matchResult{
				SceneID:   s.ID,
				SceneName: s.Name,
				Score:     score,
				Workflow:  s.Workflow,
				Event:     *event,
			}
		}
	}

	out, _ := json.Marshal(best)
	fmt.Println(string(out))
}
