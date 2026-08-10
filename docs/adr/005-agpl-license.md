# ADR-005: AGPL v3 License

**Status**: Accepted  
**Date**: 2026-08  
**Deciders**: aflare Contributors

## Context

aflare is a workflow orchestration engine — it runs user-defined workflows that can include
arbitrary business logic, LLM calls, HTTP requests, and data transformations. It is designed
to be embedded, extended, and potentially offered as a hosted service.

The license choice affects:
- Whether cloud providers can offer aflare as a proprietary SaaS without contributing back.
- Whether enterprise users can embed aflare in proprietary products.
- Community trust and contribution incentives.

Three license families were considered:

1. **Permissive** (MIT, Apache 2.0) — minimal restrictions, anyone can do anything.
2. **Weak Copyleft** (LGPL, MPL 2.0) — modifications to the library must be shared, but
   the larger application can remain proprietary.
3. **Strong Copyleft** (GPL v3, AGPL v3) — the entire combined work must be shared.

## Decision

We chose **GNU Affero General Public License v3 (AGPL v3)**.

Implementation: [`LICENSE`](../../LICENSE)

Every source file carries the AGPL v3 header:

```
This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
```

## Rationale

**Why AGPL over MIT/Apache 2.0:**

Permissive licenses allow cloud providers to take aflare, modify it, offer it as a proprietary
SaaS, and never contribute back. This creates a "strip-mining" risk: the community builds the
software, but the economic value is captured by a single provider.

AGPL v3 closes the "ASP loophole" — if you modify aflare and offer it as a network service,
you must make your modifications available to your users. This ensures that improvements to
the workflow engine benefit the entire community.

**Why AGPL over GPL v3:**

Standard GPL v3 triggers on _distribution_, not on _network use_. A cloud provider can run a
modified GPL v3 program on their servers without distributing it to users, avoiding the copyleft
obligation. AGPL v3 closes this loophole by triggering on "remote network interaction" — the
same trigger that matters for a workflow engine SaaS.

**Why AGPL over LGPL/MPL:**

LGPL and MPL are "file-level" copyleft — only modifications to the library files themselves must
be shared. The larger application can remain proprietary. This is too weak for a workflow engine:
the most valuable modifications are often not to the engine itself, but to the nodes, policies,
and integrations built on top of it. AGPL ensures that the entire ecosystem of extensions
benefits from the same copyleft protection.

## Consequences

**Positive:**
- Prevents proprietary SaaS capture of community effort.
- All modifications — including SaaS deployments — must be shared.
- Aligns with the project's values: open infrastructure for workflow automation.

**Negative:**
- Some enterprises have blanket policies against AGPL software.
- AGPL is less familiar to developers than MIT or Apache 2.0.
- The "or any later version" clause means future AGPL versions could change terms.

**Mitigations:**
- The CONTRIBUTING.md and SECURITY.md provide clear guidelines for contributors.
- The license is prominently displayed in the repository root and every source file.
- For enterprises that cannot use AGPL, a separate commercial license can be negotiated
  (dual licensing model, future consideration).