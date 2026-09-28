# Temporal

Self-contained setup for the Temporal server, its own Postgres, and the web UI.

- `VERSIONS` is the single source of truth for pinned image tags. `manifest.yaml`
  carries `__…_VERSION__` placeholders; `make up` substitutes them before applying.
- Temporal's Postgres is **separate** from the app's Postgres (see the phase-1 plan),
  so "kill Postgres mid-workflow" in phase 4 kills the app store, not Temporal's history.
- The Secret holds dev-only credentials (`temporal`/`temporal`); this is a lab, not
  production, per the charter.

## Usage

```bash
make up            # apply + wait for rollouts
make ns            # create the `nydus` Temporal namespace (once)
make status        # pods and services
make logs          # tail the temporal server log
make port-forward  # expose the UI on http://127.0.0.1:8080
make down          # delete everything (PVC included)
```

## Upgrade

Edit `VERSIONS`, then `make up`. The pinned versions track the combination the
official `temporalio/docker-compose` repo tests (`1.29.1` server / `2.34.0` UI), not the
newest GitHub tag — the `auto-setup` image tops out at 1.29.x while `server`/`admin-tools`
continue past it. Verify before bumping:

- server: https://github.com/temporalio/temporal/releases
- ui:     https://github.com/temporalio/ui-server/releases
