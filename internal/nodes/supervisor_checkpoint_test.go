// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌‌​​​‌‌​​​​​​​‌‌‌‌‌‌‌‌‌‌‌​‌​‌‌​​‌‌‌‌‌‌​​​​‌‌​​‌‌​​‌​‌​​​‌​​‌‌‌​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌‌‌​‌​​‌‌⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation; either version 3 of the License, or
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
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/agentx"
)

// countingCLIAgent writes a fake agent whose every invocation appends one
// line to counterPath, so tests can prove whether a delegation was really
// re-executed or merely restored from the checkpoint sidecar.
func countingCLIAgent(t *testing.T, dir, name, counterPath string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake agent is POSIX-only")
	}
	path := filepath.Join(dir, "fake-"+name)
	script := "#!/bin/sh\necho x >> " + counterPath + "\necho " + name + " done\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil { // #nosec G306 -- test helper must be executable
		t.Fatalf("write fake agent: %v", err)
	}
	return path
}

// invocations counts how many times a countingCLIAgent ran.
func invocations(t *testing.T, counterPath string) int {
	t.Helper()
	data, err := os.ReadFile(counterPath)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("read counter %s: %v", counterPath, err)
	}
	return strings.Count(string(data), "\n")
}

// checkpointCtx mirrors what the workflow executors inject when the run
// has checkpointing enabled (statePath set).
func checkpointCtx(statePath, step string) context.Context {
	return WithStepCheckpoint(context.Background(), StepCheckpoint{StatePath: statePath, Step: step})
}

func TestDelegationSidecarPath(t *testing.T) {
	got := delegationSidecarPath("/tmp/wf.state.json", "su perv+1")
	if want := "/tmp/wf.state.su-perv-1.delegations.json"; got != want {
		t.Errorf("sidecar path = %q, want %q", got, want)
	}
	if got := delegationSidecarPath("/tmp/wf.state.json", ""); !strings.HasSuffix(got, ".step.delegations.json") {
		t.Errorf("empty step fallback = %q, want .step suffix", got)
	}
}

func TestDelegationResume_DisabledWithoutScope(t *testing.T) {
	if r := newDelegationResume(context.Background(), "goal"); r != nil {
		t.Fatalf("resume = %v, want nil without a checkpoint scope", r)
	}
	// A nil resume must be a safe no-op, never a panic: nodes that run
	// without checkpointing take this path on every delegation.
	var r *delegationResume
	r.record(agentResult{OK: true, Agent: "alpha"})
	if _, ok := r.restore("alpha", "sub"); ok {
		t.Error("nil resume restored a result")
	}
}

func TestDelegationResume_CorruptSidecarStartsFresh(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "wf.state.json")
	scPath := delegationSidecarPath(statePath, "sup")
	if err := os.WriteFile(scPath, []byte("{not-json"), 0600); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("write corrupt sidecar: %v", err)
	}

	r := newDelegationResume(checkpointCtx(statePath, "sup"), "goal")
	if r == nil {
		t.Fatal("resume = nil, want a fresh manager for a corrupt sidecar")
	}
	if _, ok := r.restore("alpha", "sub"); ok {
		t.Error("corrupt sidecar restored a result")
	}
	// The corrupt file must be preserved for inspection, not destroyed.
	preserved, err := filepath.Glob(scPath + ".corrupt-*")
	if err != nil || len(preserved) != 1 {
		t.Errorf("preserved copies = %v (err %v), want exactly 1", preserved, err)
	}
}

func TestDelegationResume_GoalMismatchNotRestored(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "wf.state.json")
	sc := delegationSidecar{
		Version:  delegationSidecarVersion,
		Step:     "sup",
		GoalHash: delegationGoalHash("a different goal"),
		Results:  []agentResult{{Agent: "alpha", Subtask: "sub", OK: true, Output: "stale"}},
	}
	data, err := json.Marshal(&sc)
	if err != nil {
		t.Fatalf("marshal sidecar: %v", err)
	}
	if err := os.WriteFile(delegationSidecarPath(statePath, "sup"), data, 0600); err != nil { // #nosec G306 -- test fixture
		t.Fatalf("write sidecar: %v", err)
	}

	r := newDelegationResume(checkpointCtx(statePath, "sup"), "goal")
	if r == nil {
		t.Fatal("resume = nil")
	}
	if _, ok := r.restore("alpha", "sub"); ok {
		t.Error("sidecar recorded for a different goal was restored")
	}
}

// TestRunDelegations_CheckpointRecordsAndResumes pins the core resume
// contract: a delegation that succeeded before the crash is restored on
// the next attempt of the same step and is NOT re-executed.
func TestRunDelegations_CheckpointRecordsAndResumes(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "wf.state.json")
	counter := filepath.Join(dir, "alpha.count")

	registerTestAgents(t, map[string]agentx.AgentDef{
		"alpha": {Driver: agentx.DriverCLI, Profile: "generic", Binary: countingCLIAgent(t, dir, "alpha", counter)},
	})
	refs := []agentRef{{Name: "alpha", Def: mustAgent(t, "alpha")}}
	ctx := checkpointCtx(statePath, "sup")

	// First attempt: the delegation runs and is recorded.
	results, restored := runDelegations(ctx, refs, "goal one", nil, delegationOpts{})
	if restored != 0 {
		t.Fatalf("restored = %d, want 0 on the first attempt", restored)
	}
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("results = %+v, want one success", results)
	}
	if n := invocations(t, counter); n != 1 {
		t.Fatalf("agent ran %d times, want 1", n)
	}

	// The sidecar records the success under the step's name and goal hash.
	data, err := os.ReadFile(delegationSidecarPath(statePath, "sup"))
	if err != nil {
		t.Fatalf("sidecar not written: %v", err)
	}
	var sc delegationSidecar
	if err := json.Unmarshal(data, &sc); err != nil {
		t.Fatalf("sidecar unmarshal: %v", err)
	}
	if sc.Version != delegationSidecarVersion || sc.Step != "sup" || len(sc.Results) != 1 || !sc.Results[0].OK {
		t.Errorf("sidecar = %+v, want one recorded success for step sup", sc)
	}
	if sc.GoalHash != delegationGoalHash("goal one") {
		t.Errorf("sidecar goal hash = %s, want hash of the goal", sc.GoalHash)
	}

	// Second attempt (same goal, e.g. after a crash): restored, not re-executed.
	results, restored = runDelegations(ctx, refs, "goal one", nil, delegationOpts{})
	if restored != 1 {
		t.Fatalf("restored = %d, want 1 on resume", restored)
	}
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("results = %+v, want the restored success", results)
	}
	if n := invocations(t, counter); n != 1 {
		t.Errorf("agent ran %d times across resume, want 1 (a completed delegation must not re-execute)", n)
	}
}

// TestRunDelegations_CheckpointFailedNotRecorded: only successes are
// recorded, so a delegation that failed before the crash is re-dispatched
// on resume and can succeed once the agent is fixed.
func TestRunDelegations_CheckpointFailedNotRecorded(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "wf.state.json")
	counter := filepath.Join(dir, "beta.count")
	ctx := checkpointCtx(statePath, "sup")

	// Attempt 1: the agent's binary is missing → the delegation fails and
	// must not be recorded.
	registerTestAgents(t, map[string]agentx.AgentDef{
		"beta": {Driver: agentx.DriverCLI, Profile: "generic", Binary: "/nonexistent/aflare-test-binary"},
	})
	refs := []agentRef{{Name: "beta", Def: mustAgent(t, "beta")}}
	results, restored := runDelegations(ctx, refs, "goal", nil, delegationOpts{})
	if restored != 0 || len(results) != 1 || results[0].OK {
		t.Fatalf("attempt 1: results = %+v restored = %d, want one unrecorded failure", results, restored)
	}

	// Attempt 2: the agent is fixed — the delegation re-runs and succeeds.
	registerTestAgents(t, map[string]agentx.AgentDef{
		"beta": {Driver: agentx.DriverCLI, Profile: "generic", Binary: countingCLIAgent(t, dir, "beta", counter)},
	})
	refs = []agentRef{{Name: "beta", Def: mustAgent(t, "beta")}}
	results, restored = runDelegations(ctx, refs, "goal", nil, delegationOpts{})
	if restored != 0 {
		t.Fatalf("restored = %d, want 0 (failures must not be recorded)", restored)
	}
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("attempt 2 results = %+v, want the re-dispatched success", results)
	}
	if n := invocations(t, counter); n != 1 {
		t.Errorf("agent ran %d times, want 1", n)
	}
}

// TestRunDelegations_CheckpointDifferentGoalInvalidated: a changed goal
// means a changed plan — nothing recorded for the old goal is restorable.
func TestRunDelegations_CheckpointDifferentGoalInvalidated(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "wf.state.json")
	counter := filepath.Join(dir, "alpha.count")

	registerTestAgents(t, map[string]agentx.AgentDef{
		"alpha": {Driver: agentx.DriverCLI, Profile: "generic", Binary: countingCLIAgent(t, dir, "alpha", counter)},
	})
	refs := []agentRef{{Name: "alpha", Def: mustAgent(t, "alpha")}}
	ctx := checkpointCtx(statePath, "sup")

	if _, restored := runDelegations(ctx, refs, "goal one", nil, delegationOpts{}); restored != 0 {
		t.Fatalf("first attempt restored = %d, want 0", restored)
	}

	results, restored := runDelegations(ctx, refs, "a new goal", nil, delegationOpts{})
	if restored != 0 {
		t.Fatalf("restored = %d, want 0 for a different goal", restored)
	}
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("results = %+v, want a fresh execution", results)
	}
	if n := invocations(t, counter); n != 2 {
		t.Errorf("agent ran %d times, want 2 (new goal must re-dispatch)", n)
	}
}

// TestDelegateToAgents_ResumeEnvelope: the delegation envelope reports
// how many delegations were restored so downstream steps and operators
// can see the resume happened.
func TestDelegateToAgents_ResumeEnvelope(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "wf.state.json")
	counter := filepath.Join(dir, "alpha.count")

	registerTestAgents(t, map[string]agentx.AgentDef{
		"alpha": {Driver: agentx.DriverCLI, Profile: "generic", Binary: countingCLIAgent(t, dir, "alpha", counter)},
	})
	refs := []agentRef{{Name: "alpha", Def: mustAgent(t, "alpha")}}
	ctx := checkpointCtx(statePath, "sup")

	first, err := delegateToAgents(ctx, refs, "goal one", nil, delegationOpts{})
	if err != nil {
		t.Fatalf("first delegateToAgents: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(first), &env); err != nil {
		t.Fatalf("first envelope unmarshal: %v", err)
	}
	if _, present := env["resumed"]; present {
		t.Errorf("first envelope reports resumed = %v, want absent", env["resumed"])
	}

	second, err := delegateToAgents(ctx, refs, "goal one", nil, delegationOpts{})
	if err != nil {
		t.Fatalf("second delegateToAgents: %v", err)
	}
	env = nil
	if err := json.Unmarshal([]byte(second), &env); err != nil {
		t.Fatalf("second envelope unmarshal: %v", err)
	}
	if got, ok := env["resumed"].(float64); !ok || int(got) != 1 {
		t.Errorf("second envelope resumed = %v, want 1", env["resumed"])
	}
	if n := invocations(t, counter); n != 1 {
		t.Errorf("agent ran %d times across both runs, want 1", n)
	}
}

// TestSupervisor_StepCheckpointRestoredFromCtx verifies the context
// plumbing end to end at the node boundary: what the executors inject,
// the supervisor's checkpoint layer can read back.
func TestSupervisor_StepCheckpointRestoredFromCtx(t *testing.T) {
	base := context.Background()
	if _, ok := StepCheckpointFrom(base); ok {
		t.Fatal("bare ctx carried a checkpoint scope")
	}
	want := StepCheckpoint{StatePath: "/tmp/wf.state.json", Step: "sup"}
	got, ok := StepCheckpointFrom(WithStepCheckpoint(base, want))
	if !ok || got != want {
		t.Fatalf("StepCheckpointFrom = %+v ok=%v, want %+v", got, ok, want)
	}
}
