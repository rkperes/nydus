# 0006 — Cross-compile and `COPY`, never `RUN` under emulation

**Status:** Accepted · 2026-09-19

## Context

The workstation is `arm64`; the server is `amd64`. Every image must be `linux/amd64`.

Docker buildx will do this transparently via QEMU, but the emulation applies to every
`RUN` instruction: typically 3–10× slower, and outright broken for some native
toolchains (`node-gyp`, `bcrypt`, `sharp`).

## Decision

Produce the artefact natively on the workstation, then `COPY` it into a
platform-pinned base image. Cross-built images contain **no `RUN` instruction**.

```dockerfile
FROM --platform=linux/amd64 gcr.io/distroless/static:nonroot
COPY bin/app-linux-amd64 /app
ENTRYPOINT ["/app"]
```

Where a toolchain makes this impossible, emulation is permitted but must carry a comment
explaining why.

## Consequences

**Good**

- Image builds stay fast despite targeting a foreign architecture — the build is a
  layer copy, not an emulated execution.
- Small, auditable final images; distroless is a natural fit.
- The build is reproducible off a compiler invocation rather than a package manager
  resolving inside an emulated shell.

**Bad**

- Requires a toolchain that cross-compiles cleanly. This is a genuine input to
  [ADR 0008](0008-implementation-language.md), not a neutral constraint.
- Conventional multi-stage builds that compile inside the image are unavailable.
- Build steps move into the Makefile, so the Dockerfile alone no longer describes the
  whole build.

## Alternatives considered

**QEMU emulation throughout.** One less thing to think about. Rejected on speed, and on
reliability for native modules — the failures are obscure and waste more time than the
approach saves.

**Build on the server.** Native, fast, and a direct violation of
[ADR 0003](0003-server-as-server.md).

**A dedicated `amd64` build machine.** Not available.
