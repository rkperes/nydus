# Status — session handoff

**Last updated: 2026-09-19.** Volatile by design. Update it at the end of a session or
delete it; a stale status document is worse than none.

For durable information use [`charter.md`](design/charter.md),
[`roadmap.md`](design/roadmap.md), the [decisions](decisions/), and
[`doc/local/`](local/). This file only records where the work stopped.

## Where things stand

Phase 0 is **planned but not executed**. There is no cluster.

Everything committed so far is documentation and decisions. The repository contains no
`Makefile`, no `kind.yaml`, and no `deploy/` — those are *created by* the phase-0 plan,
[`plans/2026-09-19-cluster-foundation.md`](plans/2026-09-19-cluster-foundation.md), which
has never been run.

| | State |
|---|---|
| Server built, headless, boot-resilient | Done — see [`local/server-setup.md`](local/server-setup.md) |
| Docker, kind, kubectl on the server | Done. cgroups v2 and systemd driver verified |
| Repository, GitLab remote, GitHub mirror | Done |
| Charter, roadmap, measurement method | Done |
| ADRs 0001–0008 | All accepted; none open |
| **Phase 0 executed** | **Not started** |
| Phase 1 planned | Not started |

## Next session starts here

1. **Run the phase-0 plan.** It is written for an executor with no context. **Skip
   Task 0** — git identity is already set repo-locally
   (`Rafael Koch Peres <rafaelkperes@gmail.com>`).
2. **Task 1 needs a human**: `sudo apt install -y make` on the server. There is no
   passwordless sudo.
3. Then phase 1 needs a plan. Use the `writing-plans` skill; it writes into
   [`plans/`](plans/) per `AGENTS.md`.

## Known-unverified, likely to bite

These were discussed but never confirmed. Check before blaming something else.

- **inotify sysctls on the server.** `/etc/sysctl.d/99-kind.conf` may not exist. Without
  it, three kubelets exhaust the default and nodes hang `NotReady`. The plan's
  troubleshooting section covers it; checking first is cheaper.
- **kind on the server is an alpha build** (`v0.34.0-alpha`), installed via the
  `dl/latest` endpoint. Pin to a named release before anything reproducible is measured.
- **The tailnet IP is hardcoded** in the `kind.yaml` the plan creates
  (`100.120.164.25`). The plan verifies it, but a mismatch surfaces later as a TLS SAN
  error that does not point at its cause.
- **BIOS was never confirmed changed.** Secure Boot off and "Restore on AC Power Loss →
  Power On" both require physical presence and may still be unset. The second one means
  the machine will not return by itself after a power cut.

## Open threads, none blocking

- **Tailscale SSH is broken.** `tailscale up --ssh` intercepts port 22 on the tailnet but
  no `ssh:` rule exists in the ACL policy, so the tailnet SSH path resets. Either add the
  rule or run `sudo tailscale set --ssh=false` to hand port 22 back to `sshd`. `Host pc`
  currently uses the LAN address as a result.
- **MagicDNS does not resolve from the workstation.** Tooling uses the tailnet IP.
- **`bond0` on the server** — an accidental single-NIC bond from the installer. Cosmetic;
  it holds a valid lease.
- **Provider selection** for [ADR 0007](decisions/0007-application-scope.md) is a
  phase-1 design task, deliberately deferred. Criteria are in the ADR.
- **Verify `rafaelkperes@gmail.com` on GitHub.** Until then mirrored commits are not
  attributed and do not register on the contribution graph.
- **A kind cluster does not survive a server reboot.** Recovery is manual
  (`make cluster-up`). Automating it is deferred to a later plan.

## Conventions a new session should not have to rediscover

- The server holds no source and is never edited by hand. See
  [ADR 0003](decisions/0003-server-as-server.md) — this is the rule everything else
  follows from.
- Workstation is `arm64`, server is `amd64`. Build natively and `COPY`; never `RUN` under
  emulation. See [ADR 0006](decisions/0006-cross-compile-by-copy.md).
- `git push` goes to GitLab only. A server-side mirror copies to GitHub. Never commit on
  GitHub — the mirror force-pushes over it.
