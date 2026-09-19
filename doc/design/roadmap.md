# Roadmap

Six phases, roughly 38 hours. Each phase ends with something demonstrable.

Estimates are for a single person working alone, and are optimistic in the usual way.

## The rule

**Phases 3 and 4 are the payload. Everything else is scaffolding for them.**

If the schedule slips, cut providers, cut features, cut polish. Do not cut measurement.
A system with no numbers fails the [charter](charter.md); a thin system with real
numbers satisfies it.

The temptation runs the other way — phase 1 is the fun part and expands to fill whatever
time it is given. Watch for it.

## Phase 0 — Cluster foundation · ~3h

A remote kind cluster provisioned entirely from the workstation, a proven
build → push → deploy round trip, and cgroup-v2 fidelity verified by test rather than
assumed.

**Done when:** `make cluster-up` works from nothing, twice; an image built locally runs
in the cluster; a pod over its memory limit reports `OOMKilled`;
`container_cpu_cfs_throttled_seconds_total` is exposed.

Plan: [`2026-09-19-cluster-foundation.md`](../plans/2026-09-19-cluster-foundation.md)

Verifying the fidelity signals now, while the cluster is idle, is deliberate. Discovering
in phase 4 that throttling was never being recorded would invalidate everything measured
up to that point.

## Phase 1 — `fakeapi`, one provider, durable worker · ~8h

**Blocked on [ADR 0007](../decisions/0007-application-scope.md) and
[ADR 0008](../decisions/0008-implementation-language.md).**

`fakeapi` comes first, before any real provider. It is the instrument the rest of the
project measures with, and it needs to exist before there is anything to measure.

`fakeapi` serves paginated records and exposes runtime knobs for:

- latency distribution — including a long tail, not just a mean
- error rate and error class
- `429` responses, with and without `Retry-After`
- cursor expiry mid-walk
- clock skew against `modified_since`
- duplicate records, and out-of-order delivery
- a poison record that can never be processed

Then one real provider, and a durable worker that syncs a tenant incrementally and
resumes after being killed.

**Done when:** a tenant syncs end to end against `fakeapi`; killing the worker mid-sync
loses no records and duplicates none; the same worker syncs one real provider.

## Phase 2 — Kubernetes behaviour · ~6h

Resource requests and limits, liveness and readiness probes, horizontal autoscaling,
pod disruption budgets, topology spread across the two workers.

**Done when:** the deployment survives `kubectl drain` of a worker with no lost work;
the HPA reacts to load; probes fail the right way when the datastore is gone.

This is the phase where Kubernetes stops being a place to run a container and starts
being load-bearing.

## Phase 3 — Observability · ~6h · **payload**

Prometheus and Grafana, RED metrics on every service boundary, USE metrics on the nodes,
and queue depth as a first-class signal.

**Done when:** a dashboard answers "is it healthy, and if not, where is the time going?"
without anyone reading a log; queue depth is visible per tenant and per provider.

Queue depth matters more than it looks. It is the input to Little's Law in phase 4, and
it is the difference between "the system is slow" and "the system is behind".

## Phase 4 — Load, failure, and cost · ~10h · **payload**

The phase the project exists for.

**Scale emulation.** Real scale is unavailable, so it is approximated three ways, each
with stated limits:

- **Tenant multiplication** — 1, then 100, then 10,000 configured tenants against
  `fakeapi`
- **Workflow time-skipping** — compress schedule-driven behaviour without waiting for
  wall-clock time
- **Generated load** — k6 driving request volume independent of tenant count

**The failure catalogue.** Six deliberate destructions, each with a predicted response
recorded *before* the run:

| Failure | Induced by |
|---|---|
| Out of memory | `memory: 128Mi` against a workload that needs more |
| CPU starvation | `cpu: 200m`, observed through CFS throttling |
| Node loss | `kubectl drain kind-worker2` mid-sync |
| Datastore loss | kill Postgres during an in-flight workflow |
| Rate-limit storm | `fakeapi` returning `429` to everything |
| Poison record | one record that can never be processed |

**The cost model.** Per-tenant CPU, database QPS, and storage per month, measured at
each scale point and extrapolated with assumptions written alongside.

**Done when:** the extrapolation table is populated, every column traces to a
measurement or a stated assumption, and "first thing that breaks" was observed rather
than guessed.

See [`measurement.md`](measurement.md) for method and validity rules. The host has six
physical cores, which constrains what can honestly be measured — those constraints are
not optional.

## Phase 5 — Write-up · ~4h

The README and the results, aimed at a reader who will never run the code.

**Done when:** the problem, the approach, the measurements and their limits are all
legible in one pass, and every number links to how it was produced.

## Dependency order

```
0 ──▶ 1 ──▶ 2 ──▶ 3 ──▶ 4 ──▶ 5
      ▲                  │
      └── ADR 0007, 0008 │
                         └── needs phase 3's metrics to mean anything
```

Phase 4 without phase 3 produces numbers nobody can interpret. Phase 3 before phase 2
measures a system that has no interesting behaviour to observe.
