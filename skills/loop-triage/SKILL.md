---
name: loop-triage
description: Daily triage loop for the llm-box project. Scans CI status, open issues, PRs, and recent commits, then updates STATE.md with prioritized findings.
invocation: both
---

# Loop Triage Skill — llm-box

## Overview

Run the daily triage loop for the llm-box repository. Scan CI failures, open issues, pull requests, and recent commits. Compare against the current `STATE.md` and update it with prioritized findings. This is L1 report-only mode — do not auto-fix anything.

## Prerequisites

- Read access to the repository
- `STATE.md` exists in repo root
- `loop-run-log.md` exists in repo root
- GitHub API access or local git repo

## Instructions

### Step 1: Read current state

Read `STATE.md` to understand:
- Current high-priority items
- Watch list
- Recent noise / ignore rules
- Last run timestamp

### Step 2: Scan signals

Collect data from these sources:

1. **CI Status** — Check main branch and recent PRs for failing checks
2. **Open Issues** — Scan all open issues, prioritize by:
   - Security-related (label: security)
   - Bug reports
   - Feature requests
3. **Open PRs** — Check for stale PRs (>3 days without activity)
4. **Recent Commits** — Last 24h of commits, note any reverts or fixes
5. **Dependabot / Security Alerts** — Check for dependency warnings

### Step 3: Triage and prioritize

Classify each finding:

| Priority | Criteria |
|----------|----------|
| **High** | Main CI red, security issue, data loss, crash, broken core flow |
| **Medium** | Stale PR, non-critical bug, performance issue, documentation gap |
| **Low / Watch** | Minor cosmetic, nice-to-have, unclear if real issue |
| **Noise** | Bot PRs, expected failures, already-known items |

### Step 4: Update STATE.md

Update `STATE.md` with:

1. New `Last run` timestamp
2. New or updated items in High Priority section
3. Updated Watch List
4. Prune items that are resolved (closed issues, merged PRs, fixed CI)
5. Add Post-Run Critique entry

### Step 5: Append to run log

Append a row to `loop-run-log.md` with:
- Date, pattern (daily-triage), level (L1)
- Duration, tokens (if tracked)
- Items found count
- Action taken ("report-only")
- Notes

### Step 6: Post-run critique

Add to the critique section:
- One thing that was noise / false positive
- One adjustment to try next run
- Any items that need human discussion

## Output

Updated `STATE.md` and appended `loop-run-log.md`. No code changes in L1 mode.

## Examples

**Typical run:**

```
Run loop-triage skill. Read STATE.md. Check CI on main — green. Scan 1 open issue (#6 marketplace grade). 0 open PRs. No CI failures. Update STATE.md last run timestamp. Append to loop-run-log.md. No new high-priority items. Critique: quiet day; next run add commit scan.
```

**CI failure run:**

```
Run loop-triage. CI red on main — build failure in gofmt check. 2 open issues. Open worktree suggestion for gofmt fix. Update STATE.md with high-priority CI item. Mark as "waiting on human". Append run log. Critique: gofmt failures are recurring; consider adding pre-commit hook.
```

## Resources

- [STATE.md](./STATE.md) — State file to update
- [LOOP.md](./LOOP.md) — Loop configuration
- [loop-run-log.md](./loop-run-log.md) — Run log
- [loop-budget.md](./loop-budget.md) — Budget limits
- [Loop Engineering — Daily Triage Pattern](https://github.com/cobusgreyling/loop-engineering/blob/main/patterns/daily-triage.md)
