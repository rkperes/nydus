# 0004 — Registry on loopback, reached by SSH tunnel

**Status:** Accepted · 2026-09-19

## Context

Under [ADR 0003](0003-server-as-server.md), images are built on the workstation and must
reach the cluster through a registry. The registry has to be reachable from the
workstation for pushes, and from inside the cluster for pulls, without exposing a
service or managing certificates.

## Decision

A `registry:2` container on the server, published on `127.0.0.1:5000` only, and joined to
the `kind` Docker network so nodes reach it as `kind-registry:5000`.

Nodes are configured through containerd's `certs.d` mechanism so that `localhost:5000`
resolves to that container.

The workstation pushes through an SSH tunnel:

```
ssh -fN -L 5000:127.0.0.1:5000 pc
```

## Consequences

**Good**

- Docker treats `localhost` as an insecure registry **by default**, so there is no TLS
  certificate to issue, renew, or distribute, and no `insecure-registries` edit to the
  workstation daemon.
- Nothing listens on a routable address. The registry is unreachable from the LAN and
  from the tailnet.
- The same image reference — `localhost:5000/name:tag` — is valid in a `docker push` on
  the workstation and in a manifest applied to the cluster, because containerd rewrites
  it cluster-side. No tag translation step.
- The registry outlives the cluster, so images survive `make cluster-down`.

**Bad**

- The tunnel must be up before any push. Its absence shows as a confusing
  `failed to do request`, so `make tunnel` exists and the symptom is documented.
- No authentication. Acceptable only because the registry is loopback-bound.
- One more component to provision and keep running (`--restart=always`).

## Alternatives considered

**Expose the registry over the tailnet with TLS.** Removes the tunnel, adds certificate
issuance and rotation, and puts a registry on a routable address for no gain at this
scale.

**Add `insecure-registries` to the workstation Docker daemon.** Works, but it is a
machine-wide setting changed for one project, and it affects unrelated work.

**`kind load docker-image`.** Simplest of all, and rejected: it requires the image to be
in the *server's* Docker daemon, which means building on the server and abandoning
[ADR 0003](0003-server-as-server.md).
