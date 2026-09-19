# 0007 — Application scope

**Status:** **Open** · blocks phase 1

## Context

The system is a multi-tenant ingestion platform. Which data it ingests is still
undecided, and the choice is mostly about which failure modes come free with the domain.

All three candidates share the same shape — pull from several sources on a schedule,
respect per-credential limits, resume cleanly — so the architecture does not hinge on
this. Effort and realism do.

## Options

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

The chosen scope must exercise, without contrivance:

1. Per-credential rate limits that actually bind
2. Incremental sync with cursors, against an upstream whose clock cannot be trusted
3. Multi-tenancy — synthetic tenants are acceptable, but isolation must be observable
4. Resumability of a partially-complete sync

Secondary: provider authentication is a cost, not a feature. Data shape should be
non-trivial without demanding heavy parsing work that teaches nothing.

## Current leaning

**A**, because it is the only option where rate-limit contention arises naturally rather
than being simulated — and cross-tenant blast radius under rate limiting is the central
claim of the system.

## Consequences of leaving this open

Phase 1 cannot start. `fakeapi` can be specified in the abstract, but the provider it
imitates determines which knobs matter most.
