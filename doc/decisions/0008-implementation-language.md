# 0008 — Implementation language

**Status:** Accepted · 2026-09-19

## Context

One language for the worker, the scheduler glue, and `fakeapi`. Temporal provides
first-class SDKs for both candidates, so durable execution is not a differentiator.

**Go**

- Cross-compiles to a static `amd64` binary with zero emulation, which is exactly the
  pattern [ADR 0006](0006-cross-compile-by-copy.md) is built around. Builds stay fast.
- Small, predictable memory footprint, so the `memory: 128Mi` OOM scenario can be tuned
  to fail where intended.
- Concurrency primitives map cleanly onto per-credential rate budgeting.

**TypeScript / Node**

- Cross-building is free *until* a dependency needs native compilation, at which point
  QEMU is unavoidable and builds get slow and occasionally break. Mitigable by choosing
  pure-JS dependencies, but that is a constraint carried forever.
- GC behaviour makes the memory boundary less sharp. Arguably more interesting to study;
  definitely harder to attribute a result to a cause.
- Richer ecosystem for the API-client end of the work.

## Criteria

1. Build speed and reliability under [ADR 0006](0006-cross-compile-by-copy.md)
2. Ability to place the OOM and throttling boundaries precisely, since they are measured
3. Quality of Prometheus instrumentation
4. Whether the resulting code is worth reading

## Decision

**Go**, for `fakeapi`, the worker, and the scheduler.

Criterion 2 decides it. The project's output is measurements taken at resource
boundaries, and Go's footprint is predictable enough to place those boundaries
deliberately. A runtime whose memory behaviour is governed by a garbage collector makes
every OOM result harder to attribute — and an unattributable measurement is not worth
taking.

Criterion 1 points the same way: static binary, `COPY` into a distroless base, no
emulation anywhere in the build.

## Scope of this decision

This fixes the language for phase 1 and for the core. It does **not** declare the system
monolingual forever.

Temporal supports polyglot workers on one namespace, with different SDKs serving
different task queues. Adding a TypeScript worker later is architecturally cheap and
would be a legitimate demonstration of that capability.

The bar for doing so: it must exercise something Go does not. Reasonable candidates are
comparing GC-driven against allocator-driven behaviour at the same memory limit, or
proving the workflow contract holds across SDKs. Adding a second language for variety
alone is scope creep, and the [charter](../design/charter.md) already says features are
the first thing cut.

Revisit after phase 3, when there is instrumentation good enough to make the comparison
mean something. Not before.

## Consequences

**Good**

- Builds stay native-speed under [ADR 0006](0006-cross-compile-by-copy.md); no QEMU
  anywhere.
- Final images are a single static binary on distroless — small and auditable.
- Memory and CPU boundaries land where they are configured, which is a precondition for
  the failure catalogue in [measurement.md](../design/measurement.md).

**Bad**

- API client work is more verbose than it would be in TypeScript, and more of it is
  hand-rolled.
- Committing early to one language makes a later polyglot addition a deliberate change
  rather than a natural extension. The revisit point above exists to keep that honest.
- The build is split between the Makefile and the Dockerfile, so neither describes it
  alone.
