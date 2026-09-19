# 0001 — Bare metal over VM or hosted Kubernetes

**Status:** Accepted · 2026-09-19

## Context

The project's output is measurements of behaviour under resource starvation: OOM kills,
CPU throttling, node loss. Those numbers are only worth taking if the kernel is really
doing the work being measured.

Candidate environments: Docker Desktop on the laptop, WSL2 on the desktop, a managed
cloud cluster, a hosted playground, or Linux on bare metal.

## Decision

Ubuntu Server on a spare desktop PC, no hypervisor, running headless.

## Consequences

**Good**

- cgroups v2 delegation gives real `OOMKilled` events and real CFS throttling, with
  `container_cpu_cfs_throttled_seconds_total` reflecting actual scheduler behaviour.
- Node-level failures can be induced, because the nodes are ours.
- No hourly cost, so long-running load tests are free.
- The laptop stays free while tests run.

**Bad**

- A physical machine to maintain: boot resilience, remote access, power.
- Six physical cores shared between load generator and system under test. This is a
  real constraint on what can honestly be measured — see
  [measurement.md](../design/measurement.md).
- The workstation is `arm64` and the server is `amd64`, forcing cross-compilation
  ([ADR 0006](0006-cross-compile-by-copy.md)).
- Single machine, so nothing about real inter-node latency or partition can be measured
  without emulation.

## Alternatives considered

**Docker Desktop / a laptop VM.** A hypervisor inserts a second scheduler and a second
memory manager between the workload and the number. Results would partly describe the
hypervisor, and there would be no way to separate the contributions.

**Managed cloud Kubernetes.** The highest fidelity option, and the most realistic. Costs
money per hour of load testing, and several interesting failures — node drain, eviction
under real pressure — are either gated or expensive to provoke.

**Hosted playground.** Removes the failure modes entirely. You cannot drain a node you do
not own.
