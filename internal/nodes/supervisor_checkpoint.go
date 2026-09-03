// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌‌​​​‌‌​​​​​​​‌‌‌‌‌‌‌‌‌‌‌​‌​​‌​​‌​​​​‌​‌​​‌​​‌‌​​​​​‌‌‌‌‌​​‌​‌‌​​​​​​​​​​​​​​​​​​​‌‌‌​‌​​‌​‌​‌‌‌‌⁠
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

// Delegation-level checkpoint/resume for the supervisor node.
//
// The engine checkpoints at NODE granularity: a crashed supervisor step
// re-runs whole on resume, re-executing every delegation — duplicate
// side effects, duplicate token burn. This file extends the same
// durability philosophy one level down: while a checkpointed workflow
// runs, each successful delegation is recorded in a sidecar file next to
// the workflow checkpoint; on resume the supervisor restores the
// recorded delegations and re-dispatches only the unfinished ones
// (completedOK semantics, delegation edition).
//
// The channel is the step context: executors stamp
// nodes.WithStepCheckpoint(statePath, stepName) into the ctx when
// checkpointing is enabled, and the supervisor picks it up here. No
// checkpoint scope → no sidecar IO at all.

package nodes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/alib8b8/aflare/internal/fsutil"
	"github.com/alib8b8/aflare/internal/logger"
)

// StepCheckpoint names the checkpoint scope of the step being executed:
// the workflow's checkpoint file (statePath, set via WithCheckpoint)
// and the executing step's own name. The workflow executors inject it
// into the step context so long-running nodes — today the supervisor's
// parallel delegations — can persist sub-step progress next to the
// workflow checkpoint and resume mid-node after a crash.
type StepCheckpoint struct {
	StatePath string
	Step      string
}

type stepCheckpointKey struct{}

// WithStepCheckpoint returns a ctx carrying the step's checkpoint scope.
func WithStepCheckpoint(ctx context.Context, cp StepCheckpoint) context.Context {
	return context.WithValue(ctx, stepCheckpointKey{}, cp)
}

// StepCheckpointFrom extracts the step's checkpoint scope, if the
// executor enabled checkpointing for this run.
func StepCheckpointFrom(ctx context.Context) (StepCheckpoint, bool) {
	cp, ok := ctx.Value(stepCheckpointKey{}).(StepCheckpoint)
	return cp, ok
}

// delegationSidecarVersion guards the sidecar format.
const delegationSidecarVersion = 1

// delegationSidecar is the on-disk record of one supervisor step's
// completed delegations: <checkpoint-base>.<step>.delegations.json.
// GoalHash pins the entries to the node's input — a different goal on a
// later run invalidates every recorded entry (the subtasks belong to
// the old plan), so stale sidecars can never answer a new question.
type delegationSidecar struct {
	Version  int           `json:"version"`
	Step     string        `json:"step"`
	GoalHash string        `json:"goal_hash"`
	Results  []agentResult `json:"results"`
}

// unsafeStepChars matches everything not safe in a filename chunk.
var unsafeStepChars = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// delegationSidecarPath derives the per-step sidecar path from the
// workflow checkpoint path. One file per step: two supervisor steps
// running concurrently in a DAG never share a sidecar.
func delegationSidecarPath(statePath, step string) string {
	base := strings.TrimSuffix(statePath, filepath.Ext(statePath))
	safe := unsafeStepChars.ReplaceAllString(step, "-")
	if len(safe) > 64 {
		safe = safe[:64]
	}
	if safe == "" {
		safe = "step"
	}
	return base + "." + safe + ".delegations.json"
}

// delegationGoalHash fingerprints the supervisor's input. Only
// delegations recorded for the same goal are restorable.
func delegationGoalHash(goal string) string {
	sum := sha256.Sum256([]byte(goal))
	return hex.EncodeToString(sum[:])
}

// delegationJobKey identifies one delegation within a plan: the same
// agent assigned the same subtask text. Identical duplicates dedupe.
func delegationJobKey(agent, subtask string) string {
	return agent + "\x1f" + subtask
}

// delegationResume restores delegations that already succeeded in a
// previous (crashed) attempt of this supervisor step and records fresh
// successes as they complete, so a resume re-dispatches only the
// unfinished work. A nil *delegationResume (no checkpoint scope in the
// ctx) disables the feature with zero IO — same opt-in contract as the
// engine's statePath checkpointing.
type delegationResume struct {
	path     string
	step     string
	goalHash string

	mu   sync.Mutex
	done map[string]agentResult // successful delegations: restored + fresh
}

// newDelegationResume loads the sidecar for the step named in the ctx's
// checkpoint scope. Load failures are non-fatal (warn + start empty):
// a corrupt or unreadable sidecar must not break the run, it only costs
// re-dispatching work that cannot be proven complete.
func newDelegationResume(ctx context.Context, goal string) *delegationResume {
	cp, ok := StepCheckpointFrom(ctx)
	if !ok || cp.StatePath == "" || cp.Step == "" {
		return nil
	}
	r := &delegationResume{
		path:     delegationSidecarPath(cp.StatePath, cp.Step),
		step:     cp.Step,
		goalHash: delegationGoalHash(goal),
		done:     make(map[string]agentResult),
	}
	data, err := os.ReadFile(r.path) // #nosec G304 -- path derived from the operator's checkpoint path
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("delegation sidecar unreadable, resuming step without it", "path", r.path, "error", err)
		}
		return r
	}
	var sc delegationSidecar
	if err := json.Unmarshal(data, &sc); err != nil {
		// Preserve the corrupt file for inspection (same contract as the
		// main checkpoint), then start the step from scratch.
		if preserved := fsutil.PreserveCorrupt(r.path); preserved != "" {
			logger.Warn("delegation sidecar corrupt, preserved and starting fresh", "path", r.path, "preserved", preserved)
		} else {
			logger.Warn("delegation sidecar corrupt, starting fresh", "path", r.path, "error", err)
		}
		return r
	}
	if sc.GoalHash != r.goalHash {
		// Entries belong to a different goal (older run of this workflow
		// against the same checkpoint path): they answer a different
		// question and are never restored. The first fresh success
		// rewrites the file, discarding them.
		return r
	}
	for _, res := range sc.Results {
		if res.OK && res.Agent != "" {
			r.done[delegationJobKey(res.Agent, res.Subtask)] = res
		}
	}
	return r
}

// restore returns the recorded result for a job that already succeeded
// in a previous attempt of this step.
func (r *delegationResume) restore(agent, subtask string) (agentResult, bool) {
	if r == nil {
		return agentResult{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	res, ok := r.done[delegationJobKey(agent, subtask)]
	return res, ok
}

// record persists one fresh delegation success and rewrites the sidecar
// atomically (tmp + rename): a crash mid-write leaves the previous
// record intact. Failures are logged and non-fatal — checkpointing must
// never break the run it is trying to protect. Only OK results are
// recorded; a failed delegation re-runs on resume by design.
func (r *delegationResume) record(res agentResult) {
	if r == nil || !res.OK {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.done[delegationJobKey(res.Agent, res.Subtask)] = res

	sc := delegationSidecar{
		Version:  delegationSidecarVersion,
		Step:     r.step,
		GoalHash: r.goalHash,
		Results:  make([]agentResult, 0, len(r.done)),
	}
	for _, res := range r.done {
		sc.Results = append(sc.Results, res)
	}
	// Deterministic file content: identical states write identical bytes.
	sort.Slice(sc.Results, func(i, j int) bool {
		if sc.Results[i].Agent != sc.Results[j].Agent {
			return sc.Results[i].Agent < sc.Results[j].Agent
		}
		return sc.Results[i].Subtask < sc.Results[j].Subtask
	})
	data, err := json.MarshalIndent(&sc, "", "  ")
	if err != nil {
		logger.Warn("delegation sidecar marshal failed", "path", r.path, "error", err)
		return
	}
	if err := fsutil.WriteFileAtomic(r.path, data, 0600); err != nil {
		logger.Warn("delegation sidecar write failed", "path", r.path, "error", err)
	}
}

// String exists so the path shows up cleanly in diagnostics.
func (r *delegationResume) String() string {
	if r == nil {
		return fmt.Sprintf("(delegationResume nil)")
	}
	return fmt.Sprintf("(delegationResume %s)", r.path)
}
