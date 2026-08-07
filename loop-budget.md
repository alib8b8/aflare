# Loop Budget — aflare

## Daily Budget

| Resource | Limit | Notes |
|----------|-------|-------|
| Tokens/day | 100,000 | L1 report-only phase |
| Sub-agent spawns/run | 0 (L1) / 2 (L2) | Increment with level |
| Worktree creations/run | 0 (L1) / 1 (L2) | No auto-worktrees in L1 |
| PRs opened/day | 0 (L1) / 2 (L2) | No auto-PRs in L1 |

## Per-Run Estimates

| Scenario | Tokens/run | Notes |
|----------|------------|-------|
| No-op (nothing actionable) | ~5k | State read + quick scan |
| Full triage (L1) | ~50k | CI + issues + PRs + commits scan |
| Assisted fix (L2) | ~200k | Worktree + implementer + verifier |

## Budget Tracking

- Each run appends to `loop-run-log.md` with token usage
- If daily budget > 80%, next run is skipped and human is notified
- Budget resets at 00:00 UTC

## Cost Controls

1. **L1 report-only**: No sub-agents, no worktrees, no auto-fixes. Lowest cost.
2. **Effort gating**: L2 auto-fix only for items < 1 file change and < 20 lines.
3. **Noise pruning**: Items flagged 3+ days in a row get human review rather than repeated scanning.
4. **Kill switch**: `loop-pause-all` halts all schedulers.
