# Local environment

How the machines behind this project were built, and how to work against them.

This is not part of the product. It exists because the development model — a
workstation that builds and a server that runs, with no shell-based shortcuts between
them — is a deliberate design choice, and a choice worth being able to reproduce.

| Document | What |
|---|---|
| [`server-setup.md`](server-setup.md) | Turning a bare-metal PC into a headless Kubernetes host |
| [`workstation-setup.md`](workstation-setup.md) | Driving that server from a laptop, and the daily loop |

## Why a physical machine

The later phases of this project measure what happens when things are starved: pods
killed for exceeding memory, containers throttled for exceeding CPU, nodes drained
mid-workflow. Those measurements are only worth taking if the kernel is genuinely
doing the work.

- **cgroups v2 delegation** gives real OOM kills and real CFS throttling, with
  `container_cpu_cfs_throttled_seconds_total` reflecting actual scheduler behaviour.
- **A nested VM** adds a second scheduler and a second memory manager between the
  measurement and the hardware. Numbers taken there describe the hypervisor as much as
  the workload.
- **A hosted playground** removes the failure modes entirely — you cannot drain a node
  you do not own.

A second-hand desktop running Linux on bare metal is the cheapest way to get
trustworthy numbers. That is the whole argument.

## Honest limits of this setup

kind is not a production cluster, and pretending otherwise would undermine the point.
What is real, and what has to be engineered around:

| Real | Emulated or absent |
|---|---|
| Pods: namespaces, cgroups, one shared kernel | Node eviction — must be induced with `docker update --memory` or `evictionHard` |
| cgroups v2: OOM kills, CFS throttling | Network partitions — need `tc netem` |
| Pod networking and `kube-proxy` per node | `LoadBalancer` — needs MetalLB or `cloud-provider-kind` |
| Multiple kubelets with independent lifecycles | Storage — `local-path-provisioner`, unrepresentative IO |
| Real scheduling, real evictions under pressure | Inter-node latency ≈ loopback |

Each gap is a thing to engineer deliberately, and saying so in the README is more
convincing than implying the environment is something it isn't.
