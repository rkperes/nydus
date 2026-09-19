# Measurement method

How this project produces numbers it is willing to defend, on a single six-core machine.

This document exists because the easiest way to ruin a performance claim is to measure
carelessly and report confidently.

## What gets measured

**RED, at every service boundary** — Rate, Errors, Duration. Duration as a histogram, so
the tail is visible; a mean latency hides precisely the behaviour worth studying.

**USE, on the nodes** — Utilisation, Saturation, Errors. Saturation is the one that
matters: CFS throttling via `container_cpu_cfs_throttled_seconds_total`, and memory
pressure ahead of the OOM kill.

**Queue depth, per tenant and per provider.** The difference between "slow" and
"falling behind". A system with stable latency and a growing queue is already failing.

**Cost proxies** — CPU-seconds per sync, database queries per sync, bytes stored per
tenant per month.

## Little's Law as a sanity check

For a stable system, `L = λW` — items in the system equals arrival rate times time in
the system.

Measure all three independently and check they agree. When they don't, one of the
instruments is lying, and finding out which is more valuable than the number you were
originally after. Run this check at every scale point before believing anything else in
the run.

## Emulating scale that isn't available

Real scale is unavailable. It is approximated three ways, each with limits that have to
travel alongside the result.

### Tenant multiplication

Configure 1, then 100, then 10,000 tenants against `fakeapi`.

*Exercises:* scheduler fan-out, per-credential rate budget bookkeeping, database growth,
memory per tracked tenant.

*Does not exercise:* real provider diversity, real network conditions, real data shapes.
Every tenant is identical, so tail behaviour caused by one unusual tenant is invisible.

### Workflow time-skipping

Compress schedule-driven behaviour without waiting for wall-clock time.

*Exercises:* scheduling logic, cursor advancement, retry and backoff sequences over many
simulated cycles.

*Does not exercise:* anything timing-dependent in the real world — connection pool
recycling, cache expiry, certificate rotation, clock drift on real hosts.

### Generated load

k6 driving request volume independent of tenant count.

*Exercises:* throughput ceilings, latency under concurrency, the throttling and OOM
boundaries.

*Does not exercise:* realistic arrival patterns. Synthetic load is too regular; real
traffic is bursty and correlated.

## Validity rules for a six-core host

The server has **6 physical cores / 12 threads**. Load generator and system under test
share them. This is the single largest threat to every number this project produces, and
the rules below are not optional.

**1. Cap the system under test.** Total CPU limits across application pods must stay at
or below **6 cores**. Above that, pods contend with each other and with the load
generator, and throttling measurements stop describing the workload.

**2. Pin the load generator.** Run k6 under `taskset -c 8-11`, confining it to threads
the application is not using.

```bash
taskset -c 8-11 k6 run load/sync.js
```

**3. Publish load-generator CPU with every result.** It is a validity statistic, not a
footnote.

> **If the load generator exceeded ~70% CPU during a run, the result is void.** The
> measurement describes k6, not the system. Re-run with lower load or accept a lower
> ceiling.

**4. Record the whole environment with each run.** Kernel version, kind node image, host
load average before start, and whether anything else was running. A number without its
environment is not reproducible.

**5. Warm up, then measure.** Discard the first interval. JIT, connection pools, and
page cache all make early numbers optimistic in one direction and pessimistic in the
other.

**6. Three runs minimum.** Report median and spread. A single run reports noise with a
confident face.

## The failure catalogue

Six deliberate destructions. **Write the predicted response down before the run** — a
prediction recorded afterwards is not a prediction, and the value of the exercise is in
the gap between expectation and observation.

| # | Failure | Induced by | Watch |
|---|---|---|---|
| 1 | Out of memory | `memory: 128Mi` | `OOMKilled`, work resumed or lost, restart backoff |
| 2 | CPU starvation | `cpu: 200m` | `container_cpu_cfs_throttled_seconds_total`, latency tail, queue depth |
| 3 | Node loss | `kubectl drain kind-worker2` | in-flight workflows, PDB behaviour, rescheduling time |
| 4 | Datastore loss | kill Postgres mid-workflow | retry behaviour, readiness probes, recovery on return |
| 5 | Rate-limit storm | `fakeapi` returns `429` to everything | backoff correctness, `Retry-After` honoured, cross-tenant blast radius |
| 6 | Poison record | one permanently unprocessable record | does one bad record halt a tenant, all tenants, or neither |

For each: expected behaviour, observed behaviour, the divergence, and what was done
about it. **A divergence understood and deliberately left unfixed is an acceptable
outcome.** A divergence nobody noticed is not.

Failure 5 is the most interesting one. Cross-tenant blast radius under rate limiting is
the core claim of the whole system, and the only failure here that tests it directly.

## The extrapolation table

The headline deliverable.

| Tenants | Syncs/day | CPU cores | DB QPS | Storage/mo | First thing that breaks |
|---|---|---|---|---|---|
| 1 | | | | | |
| 100 | | | | | |
| 10,000 | | | | | |

Rules:

- Every cell is measured, or derived from a measurement by an assumption stated in the
  adjacent notes.
- Extrapolations name their model. Linear in tenant count is a claim, not a default, and
  it is usually wrong somewhere.
- **"First thing that breaks" must be observed.** If a scale point was never actually
  reached, say so and state what the projection is based on.
- Where a number is a guess, label it a guess. A table with two measured rows and one
  honest projection is worth more than three confident rows, one of which is fiction.
