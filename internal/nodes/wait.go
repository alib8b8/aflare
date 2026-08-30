// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌​​‌‌‌​​​‌​‌‌‌​‌​‌​‌‌​​​​‌​​‌​​‌‌‌​​​‌​‌‌​​​​‌​​‌​‌​‌​‌‌​‌‌​​​​‌‌‌‌​​​​​​​​​​​​​​​​​​‌​​​​‌‌‌‌‌‌‌​​​‌⁠
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌​‌‌​​‌​​‌​​​​‌‌​​‌‌​​‌‌​‌‌​‌​​‌‌​‌‌​‌‌‌‌‌‌​​‌‌​​‌‌​​‌‌​​​​‌​​‌‌​‌‌​​​​​‌​​​‌​‌​‌​‌​‌‌​‌‌‌​​​​​​​​‌​‌​‌‌‌​​‌‌‌‌‌‌​‌⁠
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

package nodes

import (
	"context"
	"fmt"
	"time"
)

// maxWaitDuration caps a single wait. Waiting longer than this inside a
// workflow pins a worker for hours for no benefit — scheduled re-runs
// (`aflare schedule`, cron) are the right tool for multi-hour gaps.
const maxWaitDuration = time.Hour

// WaitNode pauses workflow execution for a fixed duration, then passes its
// input through unchanged — so it can be dropped between any two steps of a
// pipeline (e.g. poll → wait → poll) without breaking the data flow.
type WaitNode struct{}

func init() {
	Register(&WaitNode{})
}

func (n *WaitNode) Name() string {
	return "wait"
}

func (n *WaitNode) Description() string {
	return "Pause the workflow for a duration (delay/sleep), then pass input through"
}

func (n *WaitNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "wait",
		Description: "Pause the workflow for a duration (delay/sleep), then pass input through",
		Input:       "string - input text, passed through unchanged after the wait",
		Output:      "string - the input, unchanged",
		Params: []ParamSchema{
			{Name: "duration", Type: "string", Description: "How long to wait (Go duration format: 500ms, 10s, 2m, 1h; max 1h — use aflare schedule for longer gaps)", Required: true},
		},
	}
}

func (n *WaitNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	raw := getParam(params, "duration", "")
	if raw == "" {
		return "", fmt.Errorf("wait: missing required param 'duration' (e.g. duration: 10s)")
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return "", fmt.Errorf("wait: invalid duration %q (use Go duration format: 500ms, 10s, 2m, 1h): %w", raw, err)
	}
	if d < 0 {
		return "", fmt.Errorf("wait: duration must not be negative, got %q", raw)
	}
	if d == 0 {
		// 0s is a no-op pipeline element, not an error — generators may
		// emit it when a template's waiting is disabled.
		return input, nil
	}
	if d > maxWaitDuration {
		return "", fmt.Errorf("wait: duration %s exceeds the %s cap — for longer gaps use `aflare schedule` (cron) instead of blocking a workflow worker", d, maxWaitDuration)
	}

	// Honor cancellation: workflow cancel/pause and per-step timeouts must
	// interrupt the wait immediately instead of sleeping it out.
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("wait: interrupted after %s: %w", d, ctx.Err())
	case <-timer.C:
	}

	// Pass-through semantics: the node delays the pipeline, not transforms it.
	return input, nil
}
