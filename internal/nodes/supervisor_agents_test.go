// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​‌‌​‌​​‌‌​​‌​​‌‌​‌​‌‌​​​​​​‌​‌​‌‌​‌​‌‌‌​‌‌‌‌​‌‌​​​​​‌‌​‌‌​​​‌‌​‌‌‌​​​​​​​​​​​​​​​​‌‌​​​‌​‌‌‌​​​‌​​⁠
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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/agentx"
)

// registerTestAgents swaps the agentx registry loader for the duration of
// a test and forces a reload.
func registerTestAgents(t *testing.T, defs map[string]agentx.AgentDef) {
	t.Helper()
	agentx.SetLoader(func() map[string]agentx.AgentDef { return defs })
	t.Cleanup(func() {
		agentx.SetLoader(func() map[string]agentx.AgentDef { return nil })
	})
}

// fakeCLIAgent writes an executable script that echoes its arguments.
func fakeCLIAgent(t *testing.T, prefix string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake agent is POSIX-only")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-"+prefix)
	script := "#!/bin/sh\necho \"" + prefix + ": $@\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil { // #nosec G306 -- test helper must be executable
		t.Fatalf("write fake agent: %v", err)
	}
	return path
}

func TestParseAgentRefs(t *testing.T) {
	registerTestAgents(t, map[string]agentx.AgentDef{
		"codex": {Driver: agentx.DriverCLI, Profile: "codex", Binary: "codex"},
	})

	personas, refs, err := parseAgentRefs([]string{"planner", "@codex", "critic", " ", ""})
	if err != nil {
		t.Fatalf("parseAgentRefs: %v", err)
	}
	if strings.Join(personas, ",") != "planner,critic" {
		t.Errorf("personas = %v", personas)
	}
	if len(refs) != 1 || refs[0].Name != "codex" {
		t.Errorf("refs = %+v", refs)
	}
}

func TestParseAgentRefs_UnknownAgentFails(t *testing.T) {
	registerTestAgents(t, nil)

	_, _, err := parseAgentRefs([]string{"planner", "@no-such-agent"})
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("err = %v, want unknown agent rejection", err)
	}
}

func TestParseAgentRefs_EmptyAgentNameFails(t *testing.T) {
	if _, _, err := parseAgentRefs([]string{"@"}); err == nil {
		t.Fatal("empty @ accepted, want rejection")
	}
}

func TestParseDelegationPlan(t *testing.T) {
	refs := []agentRef{
		{Name: "codex", Def: agentx.AgentDef{Driver: agentx.DriverCLI}},
		{Name: "claude", Def: agentx.AgentDef{Driver: agentx.DriverCLI}},
	}

	t.Run("plain json", func(t *testing.T) {
		plan, err := parseDelegationPlan(`[{"agent":"codex","subtask":"write code"}]`, refs)
		if err != nil {
			t.Fatalf("parseDelegationPlan: %v", err)
		}
		if len(plan) != 1 || plan[0].Agent != "codex" || plan[0].Subtask != "write code" {
			t.Errorf("plan = %+v", plan)
		}
	})

	t.Run("code fenced", func(t *testing.T) {
		resp := "Here is the plan:\n```json\n[{\"agent\":\"claude\",\"subtask\":\"review it\"}]\n```"
		plan, err := parseDelegationPlan(resp, refs)
		if err != nil {
			t.Fatalf("parseDelegationPlan: %v", err)
		}
		if len(plan) != 1 || plan[0].Agent != "claude" {
			t.Errorf("plan = %+v", plan)
		}
	})

	t.Run("unknown agent rejected", func(t *testing.T) {
		if _, err := parseDelegationPlan(`[{"agent":"skynet","subtask":"x"}]`, refs); err == nil {
			t.Fatal("unknown agent accepted, want rejection")
		}
	})

	t.Run("empty subtask rejected", func(t *testing.T) {
		if _, err := parseDelegationPlan(`[{"agent":"codex","subtask":"  "}]`, refs); err == nil {
			t.Fatal("empty subtask accepted, want rejection")
		}
	})

	t.Run("garbage rejected", func(t *testing.T) {
		if _, err := parseDelegationPlan("I cannot do that", refs); err == nil {
			t.Fatal("garbage accepted, want rejection")
		}
	})
}

func TestRunDelegations_ParallelFanOut(t *testing.T) {
	registerTestAgents(t, map[string]agentx.AgentDef{
		"alpha": {Driver: agentx.DriverCLI, Profile: "generic", Binary: fakeCLIAgent(t, "ALPHA")},
		"beta":  {Driver: agentx.DriverCLI, Profile: "generic", Binary: fakeCLIAgent(t, "BETA")},
	})
	refs := []agentRef{
		{Name: "alpha", Def: mustAgent(t, "alpha")},
		{Name: "beta", Def: mustAgent(t, "beta")},
	}

	// No plan → fan-out: every agent gets the full goal.
	results := runDelegations(context.Background(), refs, "shared goal", nil, 0)
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	for _, res := range results {
		if !res.OK {
			t.Errorf("agent %s failed: %s", res.Agent, res.Error)
			continue
		}
		if !strings.Contains(res.Output, "shared goal") {
			t.Errorf("agent %s output = %q, want the full goal", res.Agent, res.Output)
		}
	}
	// Sorted by agent name.
	if results[0].Agent != "alpha" || results[1].Agent != "beta" {
		t.Errorf("results not sorted: %+v", results)
	}
}

func TestRunDelegations_FailureIsolated(t *testing.T) {
	registerTestAgents(t, map[string]agentx.AgentDef{
		"good": {Driver: agentx.DriverCLI, Profile: "generic", Binary: fakeCLIAgent(t, "GOOD")},
		"bad":  {Driver: agentx.DriverCLI, Profile: "generic", Binary: "/nonexistent/aflare-test-binary"},
	})
	refs := []agentRef{
		{Name: "good", Def: mustAgent(t, "good")},
		{Name: "bad", Def: mustAgent(t, "bad")},
	}

	results := runDelegations(context.Background(), refs, "goal", nil, 0)
	byAgent := map[string]agentResult{}
	for _, res := range results {
		byAgent[res.Agent] = res
	}
	if !byAgent["good"].OK {
		t.Errorf("good agent failed: %s", byAgent["good"].Error)
	}
	if byAgent["bad"].OK || byAgent["bad"].Error == "" {
		t.Errorf("bad agent result = %+v, want recorded failure", byAgent["bad"])
	}
}

// TestRunDelegations_BoundedConcurrency pins the backpressure contract:
// with maxParallel=2 and six 400ms delegations, the batch must take at
// least three waves (>=1200ms). Unbounded fan-out would finish in ~400ms.
func TestRunDelegations_BoundedConcurrency(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake agent is POSIX-only")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-slow")
	script := "#!/bin/sh\nsleep 0.4\necho ok\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil { // #nosec G306 -- test helper must be executable
		t.Fatalf("write fake agent: %v", err)
	}

	agents := map[string]agentx.AgentDef{}
	var refs []agentRef
	for _, name := range []string{"a1", "a2", "a3", "a4", "a5", "a6"} {
		agents[name] = agentx.AgentDef{Driver: agentx.DriverCLI, Profile: "generic", Binary: path}
	}
	registerTestAgents(t, agents)
	for _, name := range []string{"a1", "a2", "a3", "a4", "a5", "a6"} {
		refs = append(refs, agentRef{Name: name, Def: mustAgent(t, name)})
	}

	start := time.Now()
	results := runDelegations(context.Background(), refs, "goal", nil, 2)
	elapsed := time.Since(start)

	if len(results) != 6 {
		t.Fatalf("results = %d, want 6", len(results))
	}
	for _, res := range results {
		if !res.OK {
			t.Errorf("agent %s failed: %s", res.Agent, res.Error)
		}
	}
	if minWaves := 3 * 400 * time.Millisecond; elapsed < minWaves {
		t.Errorf("elapsed = %v, want >= %v (maxParallel=2 must force >= 3 waves, not unbounded fan-out)", elapsed, minWaves)
	}
}

func TestClampParallelism(t *testing.T) {
	cases := []struct{ in, want int }{
		{-5, 1}, {0, 1}, {1, 1}, {4, 4}, {16, 16}, {100, 16},
	}
	for _, c := range cases {
		if got := clampParallelism(c.in); got != c.want {
			t.Errorf("clampParallelism(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func mustAgent(t *testing.T, name string) agentx.AgentDef {
	t.Helper()
	def, ok := agentx.Get(name)
	if !ok {
		t.Fatalf("agent %q not registered", name)
	}
	return def
}

func TestDelegateToAgents_WithPlannerAndSynthesis(t *testing.T) {
	registerTestAgents(t, map[string]agentx.AgentDef{
		"alpha": {Driver: agentx.DriverCLI, Profile: "generic", Binary: fakeCLIAgent(t, "ALPHA"), Description: "alpha test agent"},
		"beta":  {Driver: agentx.DriverCLI, Profile: "generic", Binary: fakeCLIAgent(t, "BETA"), Description: "beta test agent"},
	})
	refs := []agentRef{
		{Name: "alpha", Def: mustAgent(t, "alpha")},
		{Name: "beta", Def: mustAgent(t, "beta")},
	}

	var plannerCalls, synthesisCalls int
	llm := func(ctx context.Context, systemPrompt, userInput string) (string, error) {
		if strings.Contains(systemPrompt, "planning the delegation") {
			plannerCalls++
			return `[{"agent":"alpha","subtask":"part A"},{"agent":"beta","subtask":"part B"}]`, nil
		}
		synthesisCalls++
		return "SYNTHESIS: merged answer", nil
	}

	out, err := delegateToAgents(context.Background(), refs, "build feature X", llm, 0)
	if err != nil {
		t.Fatalf("delegateToAgents: %v", err)
	}
	if plannerCalls != 1 || synthesisCalls != 1 {
		t.Fatalf("plannerCalls=%d synthesisCalls=%d, want 1 each", plannerCalls, synthesisCalls)
	}

	var parsed struct {
		Mode      string        `json:"mode"`
		Planned   bool          `json:"planned"`
		Results   []agentResult `json:"results"`
		Synthesis string        `json:"synthesis"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
	if parsed.Mode != "agent-delegation" || !parsed.Planned {
		t.Errorf("mode=%q planned=%v", parsed.Mode, parsed.Planned)
	}
	if parsed.Synthesis != "SYNTHESIS: merged answer" {
		t.Errorf("synthesis = %q", parsed.Synthesis)
	}
	if len(parsed.Results) != 2 {
		t.Fatalf("results = %+v", parsed.Results)
	}
	for _, res := range parsed.Results {
		if !res.OK || !strings.Contains(res.Output, res.Subtask) {
			t.Errorf("agent %s got wrong output: %+v", res.Agent, res)
		}
	}
}

func TestDelegateToAgents_PlannerFailureFallsBackToFanOut(t *testing.T) {
	registerTestAgents(t, map[string]agentx.AgentDef{
		"alpha": {Driver: agentx.DriverCLI, Profile: "generic", Binary: fakeCLIAgent(t, "ALPHA")},
	})
	refs := []agentRef{{Name: "alpha", Def: mustAgent(t, "alpha")}}

	// Planner LLM explodes → fan-out fallback must still deliver.
	llm := func(ctx context.Context, systemPrompt, userInput string) (string, error) {
		return "", context.DeadlineExceeded
	}
	out, err := delegateToAgents(context.Background(), refs, "the goal", llm, 0)
	if err != nil {
		t.Fatalf("delegateToAgents: %v", err)
	}
	var parsed struct {
		Planned bool          `json:"planned"`
		Results []agentResult `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Planned {
		t.Error("planned=true, want fallback fan-out")
	}
	if len(parsed.Results) != 1 || !parsed.Results[0].OK {
		t.Errorf("results = %+v, want one success", parsed.Results)
	}
}

func TestDelegateToAgents_NoLLMFanOut(t *testing.T) {
	registerTestAgents(t, map[string]agentx.AgentDef{
		"alpha": {Driver: agentx.DriverCLI, Profile: "generic", Binary: fakeCLIAgent(t, "ALPHA")},
	})
	refs := []agentRef{{Name: "alpha", Def: mustAgent(t, "alpha")}}

	out, err := delegateToAgents(context.Background(), refs, "the goal", nil, 0)
	if err != nil {
		t.Fatalf("delegateToAgents: %v", err)
	}
	if !strings.Contains(out, "the goal") {
		t.Errorf("output missing goal passthrough: %s", out)
	}
}

func TestDelegateToAgents_A2AAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"id":"t1","status":{"state":"completed"},"artifacts":[{"name":"answer","parts":[{"kind":"text","text":"a2a says hi"}]}]}}`))
	}))
	t.Cleanup(srv.Close)

	registerTestAgents(t, map[string]agentx.AgentDef{
		"remote": {Driver: agentx.DriverA2A, URL: srv.URL + "/", Description: "remote a2a test agent"},
	})
	refs := []agentRef{{Name: "remote", Def: mustAgent(t, "remote")}}

	out, err := delegateToAgents(context.Background(), refs, "the goal", nil, 0)
	if err != nil {
		t.Fatalf("delegateToAgents: %v", err)
	}
	if !strings.Contains(out, "a2a says hi") {
		t.Errorf("output missing a2a artifact: %s", out)
	}
}

func TestSupervisor_RealDelegation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake agent is POSIX-only")
	}
	registerTestAgents(t, map[string]agentx.AgentDef{
		"fakey": {Driver: agentx.DriverCLI, Profile: "generic", Binary: fakeCLIAgent(t, "FAKEY"), Description: "fake test agent"},
	})

	node := &SupervisorNode{}
	// No LLM configured: planning fails (provider unreachable) and the
	// fan-out fallback must still supervise the real delegation.
	out, err := node.Execute(context.Background(), "summarize this repo", map[string]string{
		"specialists": "@fakey",
		"provider":    "openai",
		"api_key":     "", // unset → LLM call fails → fallback path
		"endpoint":    "http://127.0.0.1:1/v1/chat/completions",
	})
	if err != nil {
		t.Fatalf("supervisor real delegation: %v", err)
	}

	var parsed struct {
		Mode    string        `json:"mode"`
		Results []agentResult `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if parsed.Mode != "agent-delegation" {
		t.Errorf("mode = %q, want agent-delegation", parsed.Mode)
	}
	if len(parsed.Results) != 1 || !parsed.Results[0].OK {
		t.Errorf("results = %+v, want one supervised success", parsed.Results)
	}
	if !strings.Contains(parsed.Results[0].Output, "FAKEY") {
		t.Errorf("output = %q, want fake agent output", parsed.Results[0].Output)
	}
}

func TestSupervisor_UnknownAgentRefFails(t *testing.T) {
	registerTestAgents(t, nil)

	node := &SupervisorNode{}
	_, err := node.Execute(context.Background(), "goal", map[string]string{
		"specialists": "@ghost",
	})
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("err = %v, want unknown agent rejection", err)
	}
}

func TestSupervisor_PersonaOnlyUnchanged(t *testing.T) {
	registerTestAgents(t, nil)

	// Specialists without @ refs must keep the persona path: the call
	// reaches the LLM and fails there (no fake agent spawned, no
	// delegation mode).
	node := &SupervisorNode{}
	_, err := node.Execute(context.Background(), "goal", map[string]string{
		"specialists": "planner,critic",
		"provider":    "openai",
		"endpoint":    "http://127.0.0.1:1/v1/chat/completions",
	})
	if err == nil {
		t.Fatal("persona path without LLM must fail, not silently succeed")
	}
	if strings.Contains(err.Error(), "not registered") {
		t.Fatalf("persona-only run must not touch the agent registry: %v", err)
	}
}
