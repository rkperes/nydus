# Plans

One file per phase, executed in order. Newest at the bottom.

Naming: `YYYY-MM-DD-<slug>.md`. Update the table below when you add or finish a plan.

| Date | Plan | Status | What it delivers |
|---|---|---|---|
| 2026-09-19 | [cluster-foundation](2026-09-19-cluster-foundation.md) | Not started | A remote 3-node kind cluster provisioned from the workstation, a registry round-trip, and cgroup-v2 fidelity proven by test |

Status values: `Not started` · `In progress` · `Done` · `Abandoned`

## Conventions

- A plan is written to be executed by an agent with **no prior context and no
  judgement**. Every step carries its exact command, expected output, and a stop
  condition.
- Infrastructure plans are hand-written, because provisioning has no red/green
  test cycle to hang a TDD plan off.
- Application plans (phase 1 onward) are produced by the `writing-plans` skill
  and land in this same directory. See `AGENTS.md`.
