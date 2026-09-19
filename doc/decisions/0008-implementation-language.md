# 0008 — Implementation language

**Status:** **Open** · blocks phase 1

## Context

One language for the worker, the scheduler glue, and `fakeapi`. Temporal provides
first-class SDKs for both candidates, so durable execution is not a differentiator.

## Options

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

## Consequences of leaving this open

Phase 1 cannot start. `fakeapi` is the first thing built and it is written in whichever
language is chosen, so this decision comes first.
