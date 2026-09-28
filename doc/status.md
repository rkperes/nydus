# Status — session handoff

**Last updated: 2026-09-28.** Volatile by design. Update it at the end of a session or
delete it; a stale status document is worse than none.

For durable information use [`charter.md`](design/charter.md),
[`roadmap.md`](design/roadmap.md), the [decisions](decisions/), and
[`doc/local/`](local/). This file only records where the work stopped.

## Where things stand

Phase 0 is **done**. The cluster exists and the full build → push → deploy round trip
was proven from the workstation. `doc/plans/2026-09-19-cluster-foundation.md` was
executed end to end.

| | State |
|---|---|
| Server built, headless, boot-resilient | Done — see [`local/server-setup.md`](local/server-setup.md) |
| Docker, kind, kubectl on the server | Done. cgroups v2 and systemd driver verified |
| Repository, GitLab remote, GitHub mirror | Done |
| Charter, roadmap, measurement method | Done |
| ADRs 0001–0010 | All accepted; none open |
| **Phase 0 executed** | **Done** |
| Phase 1 planned | **Done** — [`2026-09-28-fakeapi-provider-durable-worker.md`](plans/2026-09-28-fakeapi-provider-durable-worker.md) |
| Phase 1 executed (Tasks 1–6) | **Done** — `fakeapi` deployed; Temporal + own Postgres + UI running; namespace `nydus` registered |
| Phase 1 remaining (Tasks 7–12) | App Postgres, worker, resumability proof, GitHub sync, Cadence note |

## Next session starts here

1. **Continue the phase-1 plan from Task 7** (app Postgres + schema). Tasks 1–6 are done:
   `fakeapi` runs in the cluster; Temporal (server + its own Postgres + UI) is up with
   namespace `nydus` registered. Remaining: worker (Task 8), resumability proof (Task 9),
   GitHub sync (Task 10), Cadence note (Task 11), commit (Task 12).
2. Resumability (Task 9) is still the phase's "done when".

## Known-unverified, likely to bite

- **kind on the server is still an alpha build** (`v0.34.0-alpha`), and the node image
  it pulled is `v1.37.0`. Pin kind to a named release before anything reproducible is
  measured.
- **BIOS was never confirmed changed.** Secure Boot off and "Restore on AC Power Loss →
  Power On" both require physical presence and may still be unset. The second one means
  the machine will not return by itself after a power cut.
- **A kind cluster does not survive a server reboot.** Recovery is manual
  (`make cluster-up`). Automating it is deferred to a later plan.

## Verified this session (was previously "likely to bite")

- **inotify sysctls.** The three kubelets came up `Ready` without
  `/etc/sysctl.d/99-kind.conf` — the Ubuntu 26.04 default is sufficient. No longer a
  suspect.
- **The server's tailnet IP** was confirmed current, but is no longer hardcoded
  anywhere: `make cluster-up` infers it from the server at build time.

## Open threads, none blocking

- **Tailscale SSH is broken.** `tailscale up --ssh` intercepts port 22 on the tailnet but
  no `ssh:` rule exists in the ACL policy, so the tailnet SSH path resets. Either add the
  rule or run `sudo tailscale set --ssh=false` to hand port 22 back to `sshd`. `Host pc`
  currently uses the LAN address as a result.
- **MagicDNS does not resolve from the workstation.** Tooling uses the tailnet IP.
- **`bond0` on the server** — an accidental single-NIC bond from the installer. Cosmetic;
  it holds a valid lease.
- **Second provider** is deferred: GitHub is first (ADR 0010); YouTube is the anticipated
  contrast provider, recorded as a later ADR when implemented.
- **Verify `rafaelkperes@gmail.com` on GitHub.** Until then mirrored commits are not
  attributed and do not register on the contribution graph.

## Conventions a new session should not have to rediscover

- The server holds no source and is never edited by hand. See
  [ADR 0003](decisions/0003-server-as-server.md) — this is the rule everything else
  follows from.
- Workstation is `arm64`, server is `amd64`. Build natively and `COPY`; never `RUN` under
  emulation. See [ADR 0006](decisions/0006-cross-compile-by-copy.md).
- Docker Desktop's daemon lives in a Linux VM; its `127.0.0.1` is not the Mac's. `docker
  push localhost:5000` never reaches the registry tunnel. Push with `crane`, a host
  binary pointed at `127.0.0.1:5000`. See the topology note in `AGENTS.md`.
- Machine-specific values (SSH alias, API server address, kubeconfig path) live in a
  gitignored `.env`, copied from `.env.example`. Nothing machine-specific is hardcoded
  in `kind.yaml` or the `Makefile`.
- `git push` goes to GitLab only. A server-side mirror copies to GitHub. Never commit on
  GitHub — the mirror force-pushes over it.
- Temporal version pinning: the `auto-setup` image tops out at 1.29.x, while
  `server`/`admin-tools` continue past it. `deploy/temporal/VERSIONS` pins the
  docker-compose-tested pair (server 1.29.1 / UI 2.34.0).
- Kubernetes injects `TEMPORAL_UI_PORT=tcp://<clusterip>:8080` from the `temporal-ui`
  Service; the UI container pins `TEMPORAL_UI_PORT: "8080"` to override it.
- Temporal's frontend binds the pod IP, not loopback; the in-image `temporal` CLI needs
  `--address temporal:7233` (see the `ns` target in `deploy/temporal/Makefile`).
