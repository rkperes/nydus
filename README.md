# nydus

A multi-tenant ingestion system for hostile third-party APIs — and a study of how it
behaves when starved of CPU, memory, nodes, and cooperation from the APIs themselves.

Named for the StarCraft tunnel network: remote worms in hostile territory, one network
home.

---

## The problem

Any system that pulls data from third-party APIs on behalf of many users runs into the
same four walls, and they compound.

**Rate limits are per-credential, not per-system.** One tenant kicking off a full
backfill will exhaust a shared quota and starve every other tenant's incremental sync.
Fair scheduling has to be explicit, because nothing in the API will do it for you.

**Failure is ordinary, not exceptional.** A sync that dies at record 40,000 of 50,000
must resume at 40,000. A process restart, an OOM kill, a node drain, and a 500 from
upstream all have to leave the work in a position you can continue from.

**The APIs are hostile, and rarely on purpose.** Pagination cursors expire mid-walk.
Clocks skew, so "modified since" misses records. The same record arrives twice, and out
of order. `429` sometimes carries `Retry-After` and sometimes doesn't. Every one of
these is a correctness bug waiting for production traffic.

**Cost stays invisible until it isn't.** The usual way to learn what 10,000 tenants
costs is to have 10,000 tenants. By then the architecture is fixed.

## What nydus is

A scheduler and a pool of workers that run one sync workflow per tenant, per provider:

- **Durable execution**, so a worker that dies mid-sync resumes rather than restarts
- **Per-credential rate budgeting** with backpressure, so one tenant cannot starve another
- **Cursor-based incremental sync** that tolerates skewed clocks, duplicates, and
  out-of-order delivery
- Deployed to Kubernetes, and **measured while being deliberately broken**

## What makes it more than a demo

Most projects of this shape demonstrate the happy path. The interesting behaviour is on
the other side of the failure boundary, so that is where the work goes.

**It ships with a hostile API.** `fakeapi` is a first-class component with runtime
knobs: injectable latency distributions, configurable error rates, `429` responses with
and without `Retry-After`, expiring pagination cursors, clock skew, duplicate and
out-of-order records. Every failure mode above can be turned on and reproduced.

**It has a failure catalogue, not a test suite.** Six deliberate destructions, each with
an expected and measured response:

| Failure | Induced by |
|---|---|
| Out of memory | `memory: 128Mi` against a workload that needs more |
| CPU starvation | `cpu: 200m`, measured via CFS throttling |
| Node loss | `kubectl drain` a worker mid-sync |
| Datastore loss | kill Postgres during an in-flight workflow |
| Rate-limit storm | `fakeapi` returning `429` to everything |
| Poison record | one record that cannot be processed, ever |

**Its headline output is an extrapolation table**, not a screenshot:

| Tenants | Syncs/day | CPU cores | DB QPS | Storage/mo | First thing that breaks |
|---|---|---|---|---|---|

Built by measuring a small deployment honestly and extrapolating with stated
assumptions — tenant multiplication, workflow time-skipping, and generated load —
rather than by guessing.

## Architecture

```
   fakeapi / real providers          cluster
  ┌────────────────────────┐        ┌───────────────────────────────┐
  │ rate limits, 429s,     │◀───────│  workers                      │
  │ cursors, clock skew,   │        │   ├── per-credential budget   │
  │ duplicates             │───────▶│   └── durable workflows       │
  └────────────────────────┘        │  scheduler                    │
                                    │  postgres                     │
                                    │  prometheus / grafana         │
                                    └───────────────────────────────┘
```

Durable execution is provided by [Temporal](https://temporal.io). Implementation
language is an [open decision](doc/decisions/0008-implementation-language.md).

## Deployment model

Development treats the cluster as a **remote server**, not a local sandbox:

```
  workstation (arm64)                    server (x86_64)
  ┌─────────────────────┐                ┌──────────────────────────┐
  │ source, editor      │                │ registry  127.0.0.1:5000 │
  │ docker buildx ──────┼── ssh tunnel ──┼─→                        │
  │ kubectl      ───────┼── tailnet ─────┼─→ kind cluster           │
  └─────────────────────┘                └──────────────────────────┘
```

No source lives on the server and nothing on it is edited by hand. Images are built on
the workstation, pushed to a registry, and applied through the Kubernetes API — the same
shape as deploying to a cluster you have no shell access to. The server is provisioned
by scripts in `deploy/`, standing in for the Terraform you would use against real
infrastructure. See [ADR 0003](doc/decisions/0003-server-as-server.md).

The cluster runs on bare metal rather than a VM because the measurements depend on it:
cgroups v2 delegation gives real OOM kills and real CFS throttling, where a nested
hypervisor would put its own scheduler and memory manager between the workload and the
number. See [ADR 0001](doc/decisions/0001-bare-metal.md).

## Status

Early. Phase 0 of six.

| Phase | Delivers | State |
|---|---|---|
| 0 | Remote cluster, registry round-trip, fidelity proven | In progress |
| 1 | `fakeapi`, one provider, durable worker | Not started |
| 2 | Limits, probes, autoscaling, disruption budgets | Not started |
| 3 | Prometheus, Grafana, RED metrics, queue depth | Not started |
| 4 | Load generation, scale emulation, failure catalogue, cost model | Not started |
| 5 | Write-up | Not started |

Phases 3 and 4 are the point. See [the roadmap](doc/design/roadmap.md).

## Quick start

```bash
make bootstrap     # provision the server: registry
make cluster-up    # create the cluster
make kubeconfig    # fetch credentials to the workstation
make tunnel        # open the registry tunnel
make status
```

Workstation needs Docker with buildx, kubectl, make, and SSH to the server. Server needs
Docker with **cgroups v2**, and kind.

## Documentation

| Path | What |
|---|---|
| [`doc/design/`](doc/design/) | Charter, roadmap, and the measurement method |
| [`doc/decisions/`](doc/decisions/) | Architecture decisions and their trade-offs |
| [`doc/plans/`](doc/plans/) | Executable implementation plans, phase by phase |
| [`doc/local/`](doc/local/) | How the server and workstation were built |
