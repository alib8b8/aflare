// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​‌‌​‌​​‌‌​​‌​​‌‌​‌​​‌​‌​​​​‌‌​​‌‌‌‌‌​​​‌​‌​‌‌‌‌‌‌​‌​​​‌​‌‌​‌​‌‌​‌​​​​​​​​​​​​​​​​​‌‌‌‌​​​​​‌‌​​​​‌⁠
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
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/alib8b8/aflare/internal/agentx"
)

// agentRef is a parsed specialists entry that points at a registered
// external agent instead of a persona ("@codex", "@my-a2a-agent").
type agentRef struct {
	Name string
	Def  agentx.AgentDef
}

// parseAgentRefs splits a specialists list into persona names and
// external agent refs. Entries starting with "@" name registered agents;
// unknown names fail resolution so a typo cannot silently degrade into a
// missing delegation.
func parseAgentRefs(specialists []string) (personas []string, refs []agentRef, err error) {
	for _, raw := range specialists {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		if !strings.HasPrefix(entry, "@") {
			personas = append(personas, entry)
			continue
		}
		name := strings.TrimPrefix(entry, "@")
		if name == "" {
			return nil, nil, fmt.Errorf("specialist %q: empty agent name after '@'", entry)
		}
		def, ok := agentx.Get(name)
		if !ok {
			return nil, nil, fmt.Errorf("specialist %q: agent %q is not registered (see `aflare agent list`)", entry, name)
		}
		def.Name = name
		refs = append(refs, agentRef{Name: name, Def: def})
	}
	return personas, refs, nil
}

// delegation is one planned subtask assigned to one external agent.
type delegation struct {
	Agent   string `json:"agent"`
	Subtask string `json:"subtask"`
}

// agentResult is the supervised outcome of one delegation.
type agentResult struct {
	Agent   string `json:"agent"`
	Subtask string `json:"subtask"`
	OK      bool   `json:"ok"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

// maxDelegationParallelism caps the supervisor's delegation concurrency
// so a large plan cannot fork an unbounded number of agent subprocesses
// or A2A connections at once.
const maxDelegationParallelism = 16

// defaultDelegationParallelism is the concurrency used when the
// max_parallel param is absent or unparseable.
const defaultDelegationParallelism = 4

// clampParallelism bounds the delegation concurrency to a sane range.
func clampParallelism(n int) int {
	if n < 1 {
		return 1
	}
	if n > maxDelegationParallelism {
		return maxDelegationParallelism
	}
	return n
}

// delegateToAgents runs the real-delegation loop: LLM planning (when an
// LLM is available), parallel supervised execution, LLM synthesis (when
// available). When no LLM is configured the input is fanned out to every
// listed agent and the raw outputs are merged — command and supervision
// still work without a planner. At most maxParallel delegations run at
// once (backpressure: excess jobs queue instead of spiking the host).
func delegateToAgents(ctx context.Context, refs []agentRef, goal string, llm llmCaller, maxParallel int) (string, error) {
	plan, planned := planDelegations(ctx, refs, goal, llm)

	results := runDelegations(ctx, refs, goal, plan, maxParallel)

	synthesis := synthesizeResults(ctx, goal, results, llm)

	out := map[string]any{
		"mode":    "agent-delegation",
		"planned": planned,
		"results": results,
	}
	if synthesis != "" {
		out["synthesis"] = synthesis
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal delegation output: %w", err)
	}
	return string(data), nil
}

// llmCaller abstracts runAgentLLM so tests can inject a fake planner.
type llmCaller func(ctx context.Context, systemPrompt, userInput string) (string, error)

// planDelegations asks the LLM to split the goal into per-agent
// subtasks. Falls back to fanning the full goal out to every agent when
// the LLM is unavailable or its plan cannot be parsed.
func planDelegations(ctx context.Context, refs []agentRef, goal string, llm llmCaller) (plan []delegation, planned bool) {
	if llm == nil || len(refs) == 0 {
		return nil, false
	}
	systemPrompt := buildDelegationPlannerPrompt(refs)
	resp, err := llm(ctx, systemPrompt, goal)
	if err != nil {
		return nil, false
	}
	plan, err = parseDelegationPlan(resp, refs)
	if err != nil || len(plan) == 0 {
		return nil, false
	}
	return plan, true
}

// parseDelegationPlan extracts the JSON delegation array from an LLM
// response, tolerating code fences, and rejects plans naming agents that
// were not offered.
func parseDelegationPlan(resp string, refs []agentRef) ([]delegation, error) {
	cleaned := cleanJSONResp(resp)
	start := strings.Index(cleaned, "[")
	end := strings.LastIndex(cleaned, "]")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON array in response")
	}
	var plan []delegation
	if err := json.Unmarshal([]byte(cleaned[start:end+1]), &plan); err != nil {
		return nil, fmt.Errorf("invalid delegation JSON: %w", err)
	}
	known := make(map[string]bool, len(refs))
	for _, ref := range refs {
		known[ref.Name] = true
	}
	for _, d := range plan {
		if !known[d.Agent] {
			return nil, fmt.Errorf("plan references unknown agent %q", d.Agent)
		}
		if strings.TrimSpace(d.Subtask) == "" {
			return nil, fmt.Errorf("plan has empty subtask for agent %q", d.Agent)
		}
	}
	return plan, nil
}

// runDelegations executes the plan in parallel with per-delegation
// supervision, bounded to maxParallel concurrent delegations (excess
// jobs queue on the semaphore — backpressure instead of fork bombs).
// Unplanned agents are skipped when a plan exists; without a plan every
// agent receives the full goal. Each result records success/failure so
// one failing agent cannot sink the batch.
func runDelegations(ctx context.Context, refs []agentRef, goal string, plan []delegation, maxParallel int) []agentResult {
	byName := make(map[string]agentx.AgentDef, len(refs))
	for _, ref := range refs {
		byName[ref.Name] = ref.Def
	}

	var jobs []delegation
	if len(plan) > 0 {
		jobs = plan
	} else {
		for _, ref := range refs {
			jobs = append(jobs, delegation{Agent: ref.Name, Subtask: goal})
		}
	}

	if maxParallel <= 0 {
		maxParallel = defaultDelegationParallelism
	}
	maxParallel = clampParallelism(maxParallel)
	sem := make(chan struct{}, maxParallel)

	results := make([]agentResult, len(jobs))
	var wg sync.WaitGroup
	for i, job := range jobs {
		wg.Add(1)
		go func(i int, job delegation) {
			defer wg.Done()
			// Backpressure: block until a delegation slot frees up so a
			// 20-agent plan runs 16-at-most concurrently, not 20-at-once.
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[i] = agentResult{Agent: job.Agent, Subtask: job.Subtask, Error: ctx.Err().Error()}
				return
			}
			defer func() { <-sem }()
			res := agentResult{Agent: job.Agent, Subtask: job.Subtask}
			def := byName[job.Agent]
			def.Name = job.Agent
			task := agentx.Task{
				Prompt: job.Subtask,
				Audit:  auditLog,
			}
			var out string
			var err error
			switch def.Driver {
			case agentx.DriverCLI:
				out, err = agentx.RunCLI(ctx, def, task)
			case agentx.DriverA2A:
				out, err = agentx.SendMessage(ctx, def, task)
			default:
				err = fmt.Errorf("unknown driver %q", def.Driver)
			}
			if err != nil {
				res.Error = err.Error()
			} else {
				res.OK = true
				res.Output = out
			}
			results[i] = res
		}(i, job)
	}
	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].Agent < results[j].Agent })
	return results
}

// synthesizeResults merges delegation outputs via the LLM. Falls back to
// a plain concatenated summary when no LLM is available.
func synthesizeResults(ctx context.Context, goal string, results []agentResult, llm llmCaller) string {
	if llm == nil {
		return plainSynthesis(results)
	}
	var sb strings.Builder
	sb.WriteString("Original goal:\n")
	sb.WriteString(goal)
	sb.WriteString("\n\nDelegation results:\n")
	for i, res := range results {
		fmt.Fprintf(&sb, "\n[%d] agent %s (subtask: %s) %s\n", i+1, res.Agent, res.Subtask, statusWord(res.OK))
		if res.OK {
			sb.WriteString(res.Output)
		} else {
			sb.WriteString(res.Error)
		}
	}
	systemPrompt := "You are the supervisor synthesizing delegated agent results. Merge the results into one coherent final answer for the original goal. Write in the language of the goal. Keep concrete facts and outputs from the agents; do not invent new results."
	out, err := llm(ctx, systemPrompt, sb.String())
	if err != nil || strings.TrimSpace(out) == "" {
		return plainSynthesis(results)
	}
	return strings.TrimSpace(out)
}

func statusWord(ok bool) string {
	if ok {
		return "succeeded:"
	}
	return "failed:"
}

func plainSynthesis(results []agentResult) string {
	var sb strings.Builder
	for _, res := range results {
		fmt.Fprintf(&sb, "## agent %s\n", res.Agent)
		if res.OK {
			sb.WriteString(res.Output)
		} else {
			fmt.Fprintf(&sb, "error: %s", res.Error)
		}
		sb.WriteString("\n\n")
	}
	return strings.TrimSpace(sb.String())
}

// buildDelegationPlannerPrompt describes the available agents so the
// planning LLM can route subtasks to the right executor.
func buildDelegationPlannerPrompt(refs []agentRef) string {
	var sb strings.Builder
	sb.WriteString("You are a supervisor planning the delegation of a goal to external agents. ")
	sb.WriteString("Split the goal into focused subtasks and assign each subtask to exactly one agent. ")
	sb.WriteString("Respond with ONLY a JSON array, no prose, in this exact shape:\n")
	sb.WriteString(`[{"agent": "<agent name>", "subtask": "<focused instruction for that agent>"}]`)
	sb.WriteString("\n\nAvailable agents:\n")
	for _, ref := range refs {
		desc := ref.Def.Description
		if desc == "" {
			desc = string(ref.Def.Driver) + " agent"
		}
		fmt.Fprintf(&sb, "- %s (%s driver): %s\n", ref.Name, ref.Def.Driver, desc)
	}
	sb.WriteString("\nRules: use only the listed agent names; every subtask must be self-contained; omit agents that add no value.")
	return sb.String()
}
