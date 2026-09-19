# 0007 — Application scope

**Status:** Accepted · 2026-09-19

## Context

The system is a multi-tenant ingestion platform. Which data it ingests was undecided,
and the choice is mostly about which failure modes come free with the domain.

Three candidates were considered. All share the same shape — pull from several sources
on a schedule, respect per-credential limits, resume cleanly — so the architecture does
not hinge on this. Effort and realism do.

**A. Personal data hub.** Pull from several personal-account APIs into one queryable
store. Multiple providers with genuinely different rate-limit regimes and pagination
styles. Highest realism; highest auth setup cost.

**B. Feed and read-later pipeline.** Poll feeds and bookmarking services, normalise,
deduplicate. Simplest auth. Rate limits are mild, so the scheduling problem has to be
manufactured rather than discovered.

**C. Change watcher.** Poll sources, diff against last known state, emit notifications.
Makes cursor correctness and duplicate detection central. Narrower data model; the
multi-tenant story is weaker.

## Criteria

The chosen scope had to exercise, without contrivance:

1. Per-credential rate limits that actually bind
2. Incremental sync with cursors, against an upstream whose clock cannot be trusted
3. Multi-tenancy — synthetic tenants are acceptable, but isolation must be observable
4. Resumability of a partially-complete sync

Secondary: provider authentication is a cost, not a feature. Data shape should be
non-trivial without demanding heavy parsing work that teaches nothing.

## Decision

**A — personal data hub.**

It is the only option where rate-limit contention arises naturally rather than being
simulated, and cross-tenant blast radius under rate limiting is the central claim of the
system. Under B the scheduling pressure would have to be invented, which makes every
subsequent measurement a measurement of the invention.

## Consequences

**Good**

- Rate limits bind for real, with different regimes per provider — the thing the
  scheduler exists to manage.
- Pagination styles, clock behaviour, and error semantics differ between providers, so
  `fakeapi` has real behaviour to imitate rather than invented behaviour to assume.
- Tenant isolation is observable: two tenants against the same provider contend for the
  same quota.

**Bad**

- Authentication is the largest non-core cost in phase 1. Every OAuth flow is time spent
  on something the project is not about.
- Real provider APIs change underneath the project, so tests against them are inherently
  flaky. `fakeapi` is the mitigation, and is built first for that reason.
- Personal-account data means credentials in the loop from day one. Nothing sensitive may
  land in the repo or in a committed fixture.

## Follow-on: which providers

Not settled here, and deliberately so — it is a phase-1 design task, not an architecture
decision. The selection criteria:

- **Rate limits that are tight and documented.** A generous limit teaches nothing.
- **Cheapest auth that still exercises the real path.** Token auth beats a full OAuth
  dance for the first provider; defer OAuth to the second, once the shape is proven.
- **Second provider must differ in kind**, not just in endpoint — a different pagination
  model or a different rate-limit regime. Otherwise it is cost without information.

A token-authenticated provider with published rate-limit headers is the obvious first
choice on those criteria. The second should be chosen for contrast.
