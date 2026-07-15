# Loop Configuration — llm-box

## Active Loops

| Pattern | Cadence | Status | Command |
|---------|---------|--------|---------|
| Daily Triage | 1d | L1 report-only | See below |
| CI Sweeper | on-failure | L1 report-only | Manual trigger on CI failure |
| Marketplace Skill Audit | weekly | L1 report-only | Check skill doc quality vs marketplace rubric |

## Human Gates

- No auto-fix until L2 checklist complete
- All high-risk paths (security, auth, distributed coordinator): human review required
- Auto-PR disabled; loop only opens worktrees and proposes changes
- Marketplace delisting deadline (2026-07-22): human decision on prioritization

## Budget

- Max sub-agent spawns per run: 0 (L1) / 2 (L2)
- Max tokens/day: 100k (see `loop-budget.md`)
- Append each run to `loop-run-log.md`; use `loop-budget` skill at start/end
- Kill switch: `loop-pause-all` — pause schedulers and notify human
- Estimate: `npx @cobusgreyling/loop-cost --pattern daily-triage`

## Triage Loop — How to Run

### Grok Build TUI

```
/loop 1d Run the loop-triage skill. Read STATE.md, check CI status, scan open issues and PRs, review recent commits. Append high-priority items to STATE.md with suggested next action. Do NOT auto-fix — report only. Record post-run critique.
```

### Claude Code

```
/loop 1d Run $loop-triage and update STATE.md. Do not auto-fix on first week — report only.
```

### llm-box (native execution)

```
llm-box create "Daily triage for llm-box repo: check CI, scan issues, review PRs, update STATE.md with priorities"
```

## State Contract

The loop must update these fields every run in `STATE.md`:

- `Last run` timestamp
- Item status + last action taken
- Human decisions that overrode the loop
- Post-run critique section

## Verification Strategy

- Phase 1 (report-only): Human reads `STATE.md` — no auto-action verification needed
- Phase 2+: Never let implementer mark work done; verifier confirms fix scope and tests
- Triage skill must not invent architectural work — signal only

## Human Handoff Points

- Design decisions or multi-file refactors
- Security, auth, payments, infrastructure
- Items flagged "needs discussion" in triage output
- Anything the loop has surfaced 3+ days without resolution
- Marketplace / external dependency changes

## Links

- Pattern: [daily-triage](https://github.com/cobusgreyling/loop-engineering/blob/main/patterns/daily-triage.md)
- Loop engineering: [loop-engineering repo](https://github.com/cobusgreyling/loop-engineering)
- Project: [llm-box README](./README.md)
