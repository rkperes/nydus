# AGENTS.md — nydus

Multi-tenant ingestion system on Kubernetes. Named for the StarCraft tunnel network.

## The rule that shapes everything

**The server is a server.** No source code lives on it. No file on it is edited by
hand. Anything it runs was built somewhere else, pushed to a registry, and applied
through the Kubernetes API.

This is a constraint, not a convenience. It exists so that the workflow this repo
documents is one that transfers unchanged to a cluster you cannot SSH into.

Practical consequences:

- Never `scp` source to the server. Never edit a file over SSH. Never `kubectl cp` a fix.
- If the server needs state, a script under `deploy/` puts it there.
- If a command only works when run on the server, it belongs in a provisioning script,
  not in a runbook step a human is expected to repeat.
- Provisioning over SSH is the one sanctioned exception. It is the local stand-in for
  Terraform, and it is confined to `deploy/`.

## Topology

| Workstation — macOS, `arm64` | Server `rkperes-linux0` — Ubuntu 26.04, `x86_64` |
|---|---|
| repo, editor | container registry, bound to `127.0.0.1:5000` |
| `docker buildx` → push | kind cluster `lab`: 1 control-plane, 2 workers |
| `kubectl`, `helm` | nothing hand-edited |

- The API server binds the **tailnet** address, so `kubectl` works from anywhere on the
  tailnet without a tunnel.
- The registry is published on **loopback only**. Images are pushed through an SSH
  tunnel to `localhost:5000`, which Docker treats as an insecure registry by default —
  so there is no TLS to manage and no daemon configuration to edit.

## Cross-compilation is mandatory

Workstation is `arm64`. Server is `amd64`. Every image must be `linux/amd64`.

Build the binary natively and copy it into a platform-pinned base image. That path uses
no emulation and stays fast. Reach for `--platform` QEMU emulation only when a toolchain
leaves no alternative, and leave a comment saying why.

## Plans

**Plan location — this overrides any framework default, including
`docs/superpowers/plans/`.**

- All plans live in `doc/plans/`.
- Filename: `doc/plans/YYYY-MM-DD-<slug>.md`
- After adding, starting, or finishing a plan, update the index at
  `doc/plans/README.md`. The index is hand-maintained, not generated.

## Where to look first

Read in this order. The first two are short and determine whether the rest makes sense.

| Path | What | When |
|---|---|---|
| [`doc/design/charter.md`](doc/design/charter.md) | Purpose, success criteria, **non-goals** | Before proposing any work |
| [`doc/design/roadmap.md`](doc/design/roadmap.md) | Six phases, and which ones may never be cut | Before planning |
| [`doc/design/measurement.md`](doc/design/measurement.md) | Scale emulation, failure catalogue, validity rules | Before producing any number |
| [`doc/decisions/`](doc/decisions/) | Settled trade-offs and their consequences | Before contradicting an existing choice |
| [`doc/local/`](doc/local/) | How the machines were built | Before changing anything about them |
| [`doc/status.md`](doc/status.md) | Where the last session stopped, and what is unverified | **First, at the start of a session** |

**Scope and language are settled.** A personal data hub
([ADR 0007](doc/decisions/0007-application-scope.md)) written in Go
([ADR 0008](doc/decisions/0008-implementation-language.md)). No open decisions; phase 1
is unblocked.

Two things are deferred, not open: **provider selection** is a phase-1 design task, and
**whether a TypeScript worker joins later** is revisited after phase 3. Neither may be
settled unilaterally — see the ADRs for the bar each has to clear.

If a choice here conflicts with an accepted ADR, the ADR wins until a successor
supersedes it. Write the successor; do not edit the original.

## Rules

1. Never commit a kubeconfig, token, registry credential, or `.env`.
2. Every step in a plan states its exact command, its expected output, and what to do if
   the output differs. No step may say "verify it works".
3. **State where every command runs.** Each code block is either workstation or server.
   Never leave it to inference.
4. Prefer adding a Makefile target over documenting a sequence of commands.
5. Pin image tags and versions. No `:latest` in anything committed.
6. If an infrastructure step fails twice in a row, stop and report. Do not improvise.
