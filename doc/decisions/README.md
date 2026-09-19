# Architecture decisions

One file per decision, numbered, never deleted. A decision that turns out badly gets a
successor that supersedes it — the original stays, because the reasoning is the record.

| # | Decision | Status |
|---|---|---|
| [0001](0001-bare-metal.md) | Bare metal over VM or hosted Kubernetes | Accepted |
| [0002](0002-kind.md) | kind over minikube, k3s, k3d | Accepted |
| [0003](0003-server-as-server.md) | No source code on the server | Accepted |
| [0004](0004-registry-loopback-tunnel.md) | Registry on loopback, reached by SSH tunnel | Accepted |
| [0005](0005-apiserver-on-tailnet.md) | API server bound to the tailnet address | Accepted |
| [0006](0006-cross-compile-by-copy.md) | Cross-compile and `COPY`, never `RUN` under emulation | Accepted |
| [0007](0007-application-scope.md) | Application scope | **Open** |
| [0008](0008-implementation-language.md) | Implementation language | **Open** |

Both open decisions block phase 1 of the [roadmap](../design/roadmap.md).

## Format

Context · Decision · Consequences · Alternatives considered.

Consequences include the bad ones. An ADR listing only benefits is advertising, and
tells a later reader nothing they can act on.
