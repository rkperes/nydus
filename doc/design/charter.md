# Charter

What this project is for, what "finished" means, and what it refuses to become.

## Purpose

Build a multi-tenant third-party API ingestion system, deploy it to Kubernetes, and
**characterise its behaviour under deliberate starvation and hostile upstreams**.

The system is the vehicle. The characterisation is the product.

Plenty of systems demonstrate that ingestion works. Far fewer can answer:

- What breaks first, and at what load?
- What does one tenant cost, in CPU, in database queries, in storage per month?
- What happens to in-flight work when a node disappears?
- How long does a tenant wait when another tenant is being rate-limited?
- How much of the observed latency is the upstream API, and how much is us?

Answering those with measurements — with stated assumptions and honest error bars — is
the deliverable.

## Success criteria

The project is finished when all of the following are true.

**1. It runs.** A tenant can be configured against a provider and synced end to end,
incrementally, on a schedule, in a Kubernetes cluster.

**2. It resumes.** Killing a worker mid-sync loses no work and duplicates no records.
Demonstrated, not asserted.

**3. It isolates.** One tenant saturating its rate budget does not delay another
tenant's sync beyond a stated bound. Measured with both tenants running.

**4. Every failure in the catalogue has a documented response.** For each of the six
deliberate failures, the repo states what was expected, what was observed, and where
they diverged. A divergence that was understood and left unfixed is an acceptable
outcome. A divergence that was never noticed is not.

**5. The extrapolation table exists and is defensible.** Each column traces to a
measurement or to an assumption written down next to it. "First thing that breaks" is
identified by observation, not by intuition.

**6. The write-up stands on its own.** A reader who never runs the code understands the
problem, the approach, what was measured, and what the numbers mean.

## Non-goals

Stated so they can be refused without re-litigating.

**Not a product.** No multi-user auth, no billing, no onboarding flow, no admin UI. A
config file is a sufficient tenant registry.

**Not a Nango/Airbyte/Fivetran competitor.** Provider coverage is a cost, not a feature.
One real provider is enough to prove the shape; a second only if it exposes a genuinely
different failure mode.

**Not a Kubernetes tutorial.** The cluster is infrastructure the system runs on, not the
subject. Manifests exist because something has to be deployed.

**Not a UI project.** Grafana is the interface. If a dashboard answers the question, no
application UI is needed.

**Not production-hardened.** No HA control plane, no disaster recovery, no secrets
management beyond what Kubernetes provides. Where a production system would need more,
the write-up says so rather than pretending.

## Principles

**Measure, don't assert.** Any claim in the README that could be a number should be a
number, and every number should trace to a run that produced it.

**Break it on purpose.** Failure modes that are induced deliberately are understood.
Failure modes discovered by accident are not, and there is no evidence the rest were
considered.

**Fidelity over convenience.** Where a shortcut would distort a measurement, take the
slower path and document why. This is the reasoning behind bare metal
([ADR 0001](../decisions/0001-bare-metal.md)) and the server-as-server workflow
([ADR 0003](../decisions/0003-server-as-server.md)).

**Features are cuttable. Measurement is not.** If the schedule slips, drop providers,
drop scope, drop polish. Phases 3 and 4 survive. A working system with no numbers fails
the charter; a thin system with real numbers satisfies it.

**Record decisions, not just outcomes.** A choice with its trade-offs written down is
reviewable. The same choice undocumented is indistinguishable from an accident. See
[`doc/decisions/`](../decisions/).

## Scale of the effort

Roughly 38 hours across six phases. See [the roadmap](roadmap.md). Two decisions block
phase 1 and are still open:

- [ADR 0007 — application scope](../decisions/0007-application-scope.md)
- [ADR 0008 — implementation language](../decisions/0008-implementation-language.md)
