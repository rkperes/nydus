# 0005 — API server bound to the tailnet address

**Status:** Accepted · 2026-09-19

## Context

kind binds the API server to `127.0.0.1` by default, which assumes `kubectl` runs on the
same host. Under [ADR 0003](0003-server-as-server.md) it does not — it runs on the
workstation.

## Decision

Set `networking.apiServerAddress` to the server's Tailscale address. kind places it in
the API server certificate's SANs, so TLS validates from any device on the tailnet.

The kubeconfig is fetched to the workstation with `make kubeconfig` and kept out of
`~/.kube/config`.

## Consequences

**Good**

- `kubectl` works from anywhere on the tailnet, including away from the LAN, with no
  tunnel to maintain.
- The cluster has a real endpoint, which is what a server should have.
- Tailscale addresses are stable per node, so the certificate stays valid across
  reboots and DHCP changes.

**Bad**

- The API server is reachable by every device on the tailnet. Mitigated by the tailnet
  being authenticated, and by access still requiring the client certificates in the
  kubeconfig — but it is a wider surface than loopback.
- If the tailnet address ever changes, the certificate SAN no longer matches and the
  cluster must be recreated. The failure appears as a TLS error that does not obviously
  point at its cause, so it is called out in the plan's troubleshooting.
- The kubeconfig holds client certificates and must never be committed. It is written
  `chmod 600` and is gitignored.

## Alternatives considered

**SSH tunnel on 6443, API server left on loopback.** Smaller surface, and kind's
certificate already covers `127.0.0.1`, so it validates cleanly. Rejected for a second
tunnel to remember alongside the registry's — and because a loopback-only API server is
a less honest model of a remote cluster.

**Run `kubectl` on the server.** Simplest, and contradicts
[ADR 0003](0003-server-as-server.md).
