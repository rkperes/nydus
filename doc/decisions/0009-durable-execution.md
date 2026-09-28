# 0009 — Durable execution

**Status:** Accepted · 2026-09-28

## Context

The roadmap's phase 1 requires a **durable worker** that syncs a tenant incrementally and
resumes after being killed, with no lost or duplicated records — proven, not asserted.
This is the hardest correctness property in the phase, and it can be built by hand or
bought from a durable-execution framework.

Two further constraints weigh on the choice:

1. **The author's explicit goal** is to gain hands-on experience operating a
   durable-execution system and to have durable evidence of doing so, as the basis for a
   later comparison with Cadence.
2. [ADR 0008](0008-implementation-language.md) defers a possible TypeScript worker to
   after phase 3. Whatever is chosen should not foreclose that option.

## Decision

**Temporal**, for workflows, activities, retries, and heartbeats.

The worker becomes a Temporal worker: one workflow per tenant per provider, with
activities for fetching pages and persisting records. Temporal's at-least-once activity
semantics, combined with idempotent upserts, yield effectively-once syncing. Resumability
is a property Temporal provides; this repo's code supplies only the idempotency that
collapses replays.

## Consequences

**Good**

- Retries, heartbeats, replay, and a visibility/UI surface come off the shelf rather
  than being reimplemented.
- First-class Node.js/TypeScript SDK, so the deferred TypeScript worker in
  [ADR 0008](0008-implementation-language.md) remains architecturally cheap.
- Real-world relevance: the operational shape (server roles, namespace, task queue)
  transfers directly to how one would run this in production, and to the Cadence
  comparison the author intends to write.

**Bad**

- A Temporal server (frontend, history, matching, worker) plus its **own Postgres** runs
  in the cluster, consuming roughly 1–2 GB RAM and a few hundred mCPU idle — real budget
  on a 6-core host that is also the measurement platform.
- Resumability correctness now depends on Temporal's configuration, not on code this repo
  owns. A misconfigured server can lose history; the repo must pin versions and keep its
  own Postgres separate from the app's (see phase-1 plan).
- Temporal releases move quickly; the repo carries a version-pinning and upgrade procedure
  (in `deploy/temporal/`) that would not be needed for a hand-rolled queue.

## Alternatives considered

- **Self-built queue on Postgres** (claim-lease → heartbeat → mark-done protocol). Lighter
  and fully under the repo's control, but it reinvents exactly the durability the author
  set out to learn from a real system, and its resumability guarantees would have to be
  proven against our own code rather than a widely-used framework.
- **River** (Go, Postgres-backed job queue). Attractive and light, but Go-only with no
  Node SDK — a later TypeScript worker could not share the queue, violating the constraint
  from [ADR 0008](0008-implementation-language.md).
- **Cadence.** The system Temporal forked from. Rejected as the primary choice because
  Temporal is the actively-developed line and the comparison target, not the baseline.
