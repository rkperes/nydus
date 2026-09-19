# 0003 — No source code on the server

**Status:** Accepted · 2026-09-19

## Context

The cluster host is a machine on the local network with SSH access. The obvious setup is
to keep the repository on it and edit through VS Code Remote-SSH: native-architecture
builds, `kind load` instead of a registry, no network in the inner loop.

It is meaningfully faster. It is also a workflow that exists nowhere else.

## Decision

The server is treated as a server.

- No source code on it. No file on it edited by hand.
- Images are built on the workstation, pushed to a registry, applied through the
  Kubernetes API.
- Anything the server needs is put there by a script in `deploy/`, piped over SSH and
  never copied. That is the one sanctioned exception, and it is the local stand-in for
  Terraform.

## Consequences

**Good**

- The workflow transfers unchanged to a cluster with no shell access.
- Server state is reproducible, because nothing can be created on it by hand without
  breaking the rule visibly.
- No drift between two working copies — the failure where you debug code that isn't the
  code running.
- Load generation stays on the server, so no workstation network variance leaks into
  measurements.
- Forces real engagement with registries, image pull policy, and kubeconfig
  distribution.

**Bad**

- Cross-architecture builds become mandatory
  ([ADR 0006](0006-cross-compile-by-copy.md)).
- A registry to operate ([ADR 0004](0004-registry-loopback-tunnel.md)).
- The inner loop includes a network hop and an image push.
- Recovering from an infrastructure problem is slower, because the fix has to go through
  a script rather than an editor.

## Alternatives considered

**Colocate the repo, edit via Remote-SSH.** Faster in every respect. Rejected: it
optimises the loop at the cost of the thing the project is meant to demonstrate, and
produces an artefact nobody else could deploy.

**Sync source to the server with rsync or mutagen.** Keeps native builds, still puts
source on the server, and adds a sync tool as a failure mode. Worst of both.
