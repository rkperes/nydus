# Phase 1 — `fakeapi`, one provider, durable worker — Implementation Plan

> **For agentic workers:** execute top to bottom. Every step states its exact command,
> the expected output, and what to do if the output differs.
> **If a step fails twice, stop and report.** Do not improvise around infrastructure
> failures — a wrong guess here is expensive to unwind.
>
> **Every code block is labelled `workstation` or `server`.** Run it where it says.
> The only sanctioned way to touch the server is by piping a script from `deploy/`
> over SSH. Never edit a file on the server by hand.

**Goal:** the three phase-1 deliverables, in dependency order — `fakeapi` (the instrument
the rest of the project measures with), one real provider, and a Temporal-backed durable
worker that syncs a tenant incrementally and resumes after being killed.

**Why this shape:** `fakeapi` comes first because it is the controlled upstream against
which resumability is *proven* before any real, flaky provider enters the loop. Temporal
is the durable-execution substrate ([ADR 0009](#task-2-adr-0009--durable-execution-is-temporal));
it buys retries, heartbeats, and replay off the shelf, and is also the thing the author
is deliberately here to learn.

## Machines

| | Workstation | Server |
|---|---|---|
| host | macOS, `arm64` | `rkperes-linux0`, Ubuntu 26.04.1 (`resolute`), `x86_64` |
| reached as | — | `ssh pc` (LAN address from `~/.ssh/config`) |
| tailnet | shared tailnet | API server binds the server's tailnet IP |
| hardware | — | Ryzen 5 7600X (6c/12t), 32 GB RAM, **RTX 4060 Ti (8/16 GB)** |

The GPU is present but has **no NVIDIA driver** (`nvidia-smi` absent). It is out of scope
for phase 1 and is irrelevant to it — but note it: a co-resident LLM is anticipated on
this host and **must be quiesced during any measurement run** (phases 3–4). RAM is not a
binding constraint (28 GiB free); CPU cores and measurement validity are.

## Preconditions — verified 2026-09-28

- [ ] Phase 0 done; `make cluster-up` from nothing works, registry round-trip proven.
- [ ] Cluster is up: `kubectl get nodes` shows three `Ready` nodes.
- [ ] Registry tunnel reachable: `make tunnel` then `curl -fsS http://127.0.0.1:5000/v2/_catalog`.
- [ ] Workstation Go toolchain present: `go version` ≥ 1.24 (verify against current stable).
- [ ] `crane`, `docker`, `kubectl`, `make` present.
- [ ] `git config --local user.email` is the personal address (not `@ext.uber.com`).

**Known risks**

1. Temporal versions move; pin exact tags and verify against the release page before
   running (see Troubleshooting).
2. kind PVCs survive pod restart but **not** node loss. Node loss is a phase-4 concern;
   accepted here.
3. GitHub's real API is genuinely rate-limited and can change; `fakeapi` is the mitigation
   and is built first for exactly that reason.

## New decisions this plan records

Two ADRs, produced as tasks below:

- **ADR 0009 — durable execution is Temporal.** Rationale: the author wants hands-on
  experience operating Temporal and durable evidence of having done so, and a basis for
  comparing it to Cadence. Temporal also has a first-class Node SDK, keeping the deferred
  "TypeScript worker later" option cheap.
- **ADR 0010 — first provider.** Chosen by the criteria in [ADR 0007](../decisions/0007-application-scope.md).
  The plan's default is **GitHub** (token auth, documented `X-RateLimit-*` headers,
  page+cursor pagination); override only if the human says so.

## Definition of done

- [ ] `fakeapi` runs in the cluster and exposes every knob listed in
      [`roadmap.md`](../design/roadmap.md) phase 1
- [ ] Temporal server + UI + its own Postgres run in the cluster
- [ ] A tenant syncs end to end against `fakeapi`, incrementally (cursor advances)
- [ ] **Killing the worker mid-sync loses no records and duplicates none** — proven by
      count comparison, not asserted
- [ ] The same worker syncs one real provider (GitHub)
- [ ] ADR 0009 and ADR 0010 committed
- [ ] All of it committed to `main`, plan marked `Done`

---

## Task 1: Preconditions & toolchain

- [ ] **Step 1 (workstation): confirm the cluster and tunnel**

```bash
kubectl get nodes -o wide
make tunnel
curl -fsS http://127.0.0.1:5000/v2/_catalog
```

Expected: three `Ready` nodes; `registry tunnel up on localhost:5000`; a JSON `_catalog`
reply (empty or listing prior repos). Any failure → return to phase 0.

- [ ] **Step 2 (workstation): confirm Go**

```bash
go version
```

Expected: `go version go1.x.y <arch>`. If absent or < 1.24, install the current stable
(`brew install go`) and re-run.

- [ ] **Step 3 (workstation): confirm git identity**

```bash
git config --local user.email
```

Expected: the personal address. If it shows `@ext.uber.com`, stop — fix identity first.

---

## Task 2: ADR 0009 — durable execution is Temporal

- [ ] **Step 1 (workstation): create `doc/decisions/0009-durable-execution.md`**

Follow the format in [`doc/decisions/README.md`](../decisions/README.md): Context ·
Decision · Consequences · Alternatives considered. Must state, explicitly:

- **Decision:** Temporal for durable execution (workflows, activities, retries, heartbeats).
- **Context:** the roadmap's "durable worker ... resumes after being killed"; the author's
  intent to learn operating Temporal and to have durable evidence of doing so, as the
  basis for a later Cadence comparison; the deferred TypeScript-worker option (Temporal
  has a first-class Node SDK).
- **Consequences:** a Temporal server (frontend/history/matching/worker) plus its own
  Postgres runs in the cluster, consuming roughly 1–2 GB RAM and a few hundred mCPU idle;
  the worker's resumability comes from Temporal rather than from code this repo owns.
- **Alternatives considered:** self-built Postgres queue (rejected: reinvents durability,
  loses the learning objective); River (rejected: Go-only, no Node SDK).

- [ ] **Step 2 (workstation): add the row to `doc/decisions/README.md`**

| # | Decision | Status |
|---|---|---|
| [0009](0009-durable-execution.md) | Durable execution — Temporal | Accepted |

- [ ] **Step 3 (workstation): commit**

```bash
git add doc/decisions/0009-durable-execution.md doc/decisions/README.md
git commit -m "docs: ADR 0009 — durable execution is Temporal"
```

---

## Task 3: ADR 0010 — first provider

- [ ] **Step 1 (workstation): confirm the default with the human**

Default is **GitHub**. Only proceed with a different provider if the human overrides.
The ADR 0007 criteria: tight, documented rate limits; cheapest auth that still exercises
the real path (token auth); a second provider must differ in kind (deferred — not this
phase).

GitHub satisfies the criteria: personal access token, `X-RateLimit-Limit` /
`X-RateLimit-Remaining` / `X-RateLimit-Reset` response headers, page pagination via
`Link` header (and cursor pagination via GraphQL), `429` with `Retry-After`.

- [ ] **Step 2 (workstation): create `doc/decisions/0010-first-provider.md`**

Record: **Decision** (GitHub), **Context** (the ADR 0007 criteria and how GitHub meets
them), **Consequences** (read-only fine-grained token; `Link`-header page pagination,
so the worker's "cursor" is the next-page URL rather than an opaque offset; real API may
drift), **Alternatives considered** (Notion — token auth, tight 3 req/s but no
rate-limit headers; Todoist — token auth, weaker pagination).

- [ ] **Step 3 (workstation): add the row to `doc/decisions/README.md`** and commit as in
      Task 2 Step 2–3.

---

## Task 4: `fakeapi` — the instrument

`fakeapi` is a single Go HTTP server, stdlib `net/http` only (no framework — see
[ADR 0008](../decisions/0008-implementation-language.md)). It serves paginated records
and exposes runtime knobs so every phase-4 failure mode can be induced later.

- [ ] **Step 1 (workstation): init the module**

```bash
cd ~/src/nydus && go mod init nydus
```

Expected: `go: creating new go.mod: module nydus`.

- [ ] **Step 2 (workstation): implement `cmd/fakeapi/main.go` and `internal/fakeapi/…`**

Contracts (the plan is normative; the implementation must satisfy these):

- **Data model.** A record is `{"id": <int>, "modified_at": "<RFC3339>", "data": "<string>"}`.
  `id` is the cursor: strictly increasing, defines order.
- **Endpoint `GET /records`.** Query params: `limit` (default 100, max 1000),
  `cursor` (last `id` seen; return records with `id > cursor`), `modified_since`
  (RFC3339; return records with `modified_at >= modified_since`). Returns
  `{"records":[...], "next_cursor": <int>, "has_more": <bool>}`.
- **Knobs, settable per-request via a control header `X-Fakeapi-Knobs`** (JSON), or
  globally via `POST /control`:
  - `latency_ms` and `latency_tail_ms` — add a mean delay plus a heavy-tailed jitter,
    not just a constant
  - `error_rate` and `error_class` (e.g. `500`, `429`, `timeout`)
  - `retry_after_seconds` — emit `Retry-After` on `429`
  - `cursor_expiry_after_requests` — invalidate a cursor after N requests (breaks
    mid-walk)
  - `clock_skew_seconds` — offset `modified_at` relative to the wall clock
  - `duplicate_probability` — repeat the previous record
  - `out_of_order_probability` — emit a record with an `id` lower than a prior one
  - `poison_id` — a record whose processing must always fail downstream
- **`GET /healthz`** returns `200 ok` (used by readiness probe).
- **Port:** `:8080`.

The dataset is synthetic and deterministic for a given seed (e.g. `id` = seed offset),
so a sync of N records is exactly reproducible. State (current cursor, emitted records)
is in-process only; `fakeapi` may be stateless.

- [ ] **Step 3 (workstation): unit-test the knobs**

```bash
go test ./internal/fakeapi/...
```

Expected: `ok  nydus/internal/fakeapi  ...` with tests covering pagination, cursor
expiry, `Retry-After` on `429`, and duplicate/out-of-order emission.

- [ ] **Step 4 (workstation): run it locally and smoke-test**

```bash
go run ./cmd/fakeapi &
curl -fsS 'http://127.0.0.1:8080/records?limit=5'
curl -fsS 'http://127.0.0.1:8080/records?limit=5&cursor=<id_from_first>'
kill %1
```

Expected: two pages of records, the second strictly after the cursor, `has_more` and
`next_cursor` consistent. Any 5xx on the plain path → stop and report.

---

## Task 5: `fakeapi` — build, push, deploy, verify

- [ ] **Step 1 (workstation): create `deploy/fakeapi/Dockerfile`**

No `RUN`, per [ADR 0006](../decisions/0006-cross-compile-by-copy.md):

```dockerfile
FROM --platform=linux/amd64 gcr.io/distroless/static:nonroot
COPY bin/fakeapi-linux-amd64 /fakeapi
ENTRYPOINT ["/fakeapi"]
```

- [ ] **Step 2 (workstation): build the binary and image, push with `crane`**

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/fakeapi-linux-amd64 ./cmd/fakeapi
docker buildx build --platform linux/amd64 -o type=docker,dest=/tmp/fakeapi.tar deploy/fakeapi
crane push /tmp/fakeapi.tar 127.0.0.1:5000/fakeapi:0.1.0
```

Expected: `crane` ends with `digest: sha256:...`. `connection refused` → tunnel down
(`make tunnel`).

- [ ] **Step 3 (workstation): create `deploy/fakeapi/manifest.yaml`** — a `Namespace`
  `nydus`, a `Deployment` (image `localhost:5000/fakeapi:0.1.0`, port 8080, `readinessProbe`
  on `/healthz`), and a `Service` `fakeapi` on port 8080.

- [ ] **Step 4 (workstation): apply and verify**

```bash
kubectl apply -f deploy/fakeapi/manifest.yaml
kubectl -n nydus rollout status deployment/fakeapi
kubectl -n nydus run probe --rm -i --restart=Never --image=alpine:3.20 -- \
  wget -qO- 'http://fakeapi:8080/records?limit=3'
```

Expected: deployment `successfully rolled out`; the probe prints a page of records.
`ImagePullBackOff` → see phase-0 Troubleshooting (nodes cannot pull from registry).

---

## Task 6: Temporal + its own Postgres, in the cluster

Temporal runs its own Postgres, **separate from the app's Postgres** (Task 7), so that
"kill Postgres mid-workflow" in phase 4 kills the *app* datastore and not Temporal's own
substrate.

Temporal is a **third-party service with its own lifecycle** (pin, upgrade, teardown), so
its setup is self-contained under `deploy/temporal/` with its own `Makefile`, a
`VERSIONS` file as the single source of truth for pinned image tags, and a `README`
recording the upgrade procedure. The root Makefile delegates to it rather than duplicating
it (see Step 6). Everything in this directory is labelled
`app.kubernetes.io/part-of=temporal` so `make down` can delete by selector.

```
deploy/temporal/
  README.md      # what it is, upgrade procedure
  VERSIONS       # pinned image tags (Makefile-compatible)
  Makefile       # up / down / status / logs / ns / port-forward
  manifest.yaml  # server + UI + temporal-postgres (version placeholders)
```

- [ ] **Step 1 (workstation): verify storage is available**

```bash
kubectl get storageclass
```

Expected: a storage class marked `(default)` (kind ships one). If none, install a
local-path provisioner before proceeding (see Troubleshooting).

- [ ] **Step 2 (workstation): create `deploy/temporal/VERSIONS`**

Verify the server and UI tags against the release pages first (see Troubleshooting).
Pin exact tags; no `latest`.

```make
TEMPORAL_SERVER=1.26.2
TEMPORAL_UI=2.32.0
TEMPORAL_ADMIN_TOOLS=1.26.2
POSTGRES=16.6-alpine
```

- [ ] **Step 3 (workstation): create `deploy/temporal/manifest.yaml`**

Version fields are placeholders — the Makefile substitutes them from `VERSIONS` before
applying, the same substitution pattern `kind.yaml` already uses for
`__API_SERVER_ADDRESS__`. Every resource carries the label
`app.kubernetes.io/part-of=temporal`:

- `Deployment` `temporal-postgres` — image `postgres:__POSTGRES_VERSION__`, env
  `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB=temporal`; `Service`
  `temporal-postgres:5432`; PVC for `/var/lib/postgresql/data`.
- `Deployment` `temporal` — image `temporalio/auto-setup:__TEMPORAL_SERVER_VERSION__`, env
  `DB=postgres12`, `DB_PORT=5432`, `POSTGRES_SEEDS=temporal-postgres`, `POSTGRES_USER`,
  `POSTGRES_PWD`, `DYNAMIC_CONFIG_FILE_PATH` unset (defaults); `Service` `temporal:7233`.
- `Deployment` `temporal-ui` — image `temporalio/ui:__TEMPORAL_UI_VERSION__`, env
  `TEMPORAL_ADDRESS=temporal:7233`; `Service` `temporal-ui:8080`.

A `Deployment` with a PVC is acceptable for a lab (one replica ever runs).

- [ ] **Step 4 (workstation): create `deploy/temporal/Makefile`**

Recipe lines begin with a **tab**. `include VERSIONS` makes it the single version source.

```make
include VERSIONS

NS := nydus

.PHONY: up down status logs ns port-forward

up:
	sed -e 's/__TEMPORAL_SERVER_VERSION__/$(TEMPORAL_SERVER)/g' \
	    -e 's/__TEMPORAL_UI_VERSION__/$(TEMPORAL_UI)/g' \
	    -e 's/__TEMPORAL_ADMIN_TOOLS_VERSION__/$(TEMPORAL_ADMIN_TOOLS)/g' \
	    -e 's/__POSTGRES_VERSION__/$(POSTGRES)/g' \
	    manifest.yaml | kubectl apply -f -
	kubectl -n $(NS) rollout status deployment/temporal --timeout=300s
	kubectl -n $(NS) rollout status deployment/temporal-postgres --timeout=300s

down:
	kubectl -n $(NS) delete deploy,svc,pvc \
	  -l app.kubernetes.io/part-of=temporal --ignore-not-found

status:
	kubectl -n $(NS) get pods,svc -l app.kubernetes.io/part-of=temporal

logs:
	kubectl -n $(NS) logs -f deployment/temporal

ns:
	kubectl -n $(NS) run tctl --rm -i --restart=Never \
	  --image=temporalio/admin-tools:$(TEMPORAL_ADMIN_TOOLS) -- \
	  temporal operator namespace create nydus

port-forward:
	kubectl -n $(NS) port-forward svc/temporal-ui 8080:8080
```

- [ ] **Step 5 (workstation): create `deploy/temporal/README.md`** — states what this
      directory is, that `VERSIONS` is the single source of version truth, and the upgrade
      procedure (edit `VERSIONS`, `make up`, watch `make status`).

- [ ] **Step 6 (workstation): add root Makefile passthrough targets** (optional but keeps
      one entry point)

```make
temporal-up:
	$(MAKE) -C deploy/temporal up

temporal-down:
	$(MAKE) -C deploy/temporal down

temporal-ui:
	$(MAKE) -C deploy/temporal port-forward
```

- [ ] **Step 7 (workstation): bring Temporal up and create the namespace**

```bash
cd ~/src/nydus && make -C deploy/temporal up && make -C deploy/temporal ns
```

Expected: `deployment "temporal" successfully rolled out` and
`deployment "temporal-postgres" successfully rolled out`, then the `nydus` Temporal
namespace is created. If `temporal` crash-loops, `make -C deploy/temporal logs` — a
Postgres connectivity or schema-migration error points at the `POSTGRES_*` env (see
Troubleshooting). The task queue is created implicitly by the worker's first
registration; no explicit step.

- [ ] **Step 8 (workstation): verify the UI is reachable**

```bash
make -C deploy/temporal port-forward
```

Then open `http://127.0.0.1:8080`. Expected: the Temporal web UI loads and shows the
`nydus` namespace. Kill the port-forward when done.

---

## Task 7: App Postgres + schema

- [ ] **Step 1 (workstation): create `deploy/postgres/manifest.yaml`**

`Deployment` `app-postgres` (image `postgres:16-alpine`, `POSTGRES_DB=nydus`), a PVC,
and a `Service` `app-postgres:5432`. Credentials via a `Secret`, not hardcoded in the
manifest.

- [ ] **Step 2 (workstation): create `deploy/postgres/schema.sql`** — the tenant-sync
      store:

```sql
CREATE TABLE IF NOT EXISTS records (
  tenant    TEXT NOT NULL,
  provider  TEXT NOT NULL,
  id        BIGINT NOT NULL,
  payload   JSONB NOT NULL,
  synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant, provider, id)
);

CREATE TABLE IF NOT EXISTS cursors (
  tenant     TEXT PRIMARY KEY,
  provider   TEXT NOT NULL,
  cursor     BIGINT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`records` has a natural composite key (`tenant, provider, id`), which is what makes the
upsert idempotent (Task 8).

- [ ] **Step 3 (workstation): apply and initialise**

```bash
kubectl apply -f deploy/postgres/manifest.yaml
kubectl -n nydus rollout status deployment/app-postgres
kubectl -n nydus exec -i deployment/app-postgres -- psql -U nydus -d nydus < deploy/postgres/schema.sql
```

Expected: `CREATE TABLE` × 2 (or `NOTICE: relation ... already exists`). Any connection
refused → the `Service`/secret wiring is wrong; fix and re-run.

---

## Task 8: Worker — Temporal workflow + activities

Go, Temporal Go SDK (`go.temporal.io/sdk`). One workflow per tenant per provider.

- [ ] **Step 1 (workstation): `go get` the SDK and pin it in `go.mod`**

```bash
go get go.temporal.io/sdk@<pinned-tag>
```

- [ ] **Step 2 (workstation): implement `cmd/worker/main.go` and `internal/worker/…`**

Contracts:

- **Workflow `SyncTenant(tenant, provider)`** — a loop that calls
  `FetchPage(cursor) → UpsertRecords(records) → record new cursor`, until `has_more` is
  false. Deterministic: the only side-effecting call is `UpsertRecords`, gated by the
  workflow's control flow so replay is safe.
- **Activity `FetchPage(cursor)`** — HTTP GET against `http://fakeapi:8080/records`
  (or the provider's endpoint), honours `429`/`Retry-After` via Temporal retry policy
  (backoff coefficient, max interval), heartbeats on long fetches.
- **Activity `UpsertRecords(records)`** — idempotent upsert into `records`:

```sql
INSERT INTO records (tenant, provider, id, payload) VALUES ($1,$2,$3,$4)
ON CONFLICT (tenant, provider, id) DO NOTHING;
```

followed by cursor advance into `cursors` (`INSERT ... ON CONFLICT (tenant) DO UPDATE`).

- **Idempotency is the resumability guarantee.** Temporal runs activities at-least-once;
  the `ON CONFLICT DO NOTHING` upsert collapses replays into no-ops, so a mid-sync kill
  and replay neither loses nor duplicates a record.
- The worker registers a `Worker` on task queue `nydus-sync`, connecting to
  `temporal:7233`.

- [ ] **Step 3 (workstation): build and push the worker image**

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/worker-linux-amd64 ./cmd/worker
docker buildx build --platform linux/amd64 -o type=docker,dest=/tmp/worker.tar deploy/worker
crane push /tmp/worker.tar 127.0.0.1:5000/worker:0.1.0
```

(`deploy/worker/Dockerfile` is the same no-`RUN` distroless pattern as Task 5.)

- [ ] **Step 4 (workstation): create `deploy/worker/manifest.yaml`** — a `Deployment`
  `worker` (image `localhost:5000/worker:0.1.0`, env `TEMPORAL_ADDRESS=temporal:7233`,
  `POSTGRES_DSN` pointing at `app-postgres`), replicas 1.

- [ ] **Step 5 (workstation): deploy and watch the log**

```bash
kubectl apply -f deploy/worker/manifest.yaml
kubectl -n nydus rollout status deployment/worker
kubectl -n nydus logs -f deployment/worker
```

Expected: the worker registers on task queue `nydus-sync` (log line). `failed to connect`
→ Temporal is not reachable at `temporal:7233`; re-check Task 6.

---

## Task 9: Prove resumability — the core deliverable

Kill the worker mid-sync and prove no records are lost or duplicated.

- [ ] **Step 1 (workstation): pick a fixed dataset size** — `fakeapi` serves N records
      deterministically. Use `N = 10 000`.

- [ ] **Step 2 (workstation): start a sync of N records**

```bash
kubectl -n nydus exec -i deployment/temporal -- temporal workflow start \
  --task-queue nydus-sync --type SyncTenant --input '"tenant-1" "fakeapi"'
```

(If `temporal` isn't in the `auto-setup` image PATH, run the start through an
`admin-tools` pod instead.)

- [ ] **Step 3 (workstation): kill the worker mid-sync, then observe the retry**

Wait for the workflow to be in-flight (records count climbing), then:

```bash
kubectl -n nydus delete pod -l app=worker
kubectl -n nydus get pods -w   # watch the replacement pod come up
```

Temporal re-schedules the in-flight activity on the replacement worker. Expected: the
replacement pod reaches `Running` and the workflow continues rather than failing.

- [ ] **Step 4 (workstation): verify the count is exactly N, no duplicates**

```bash
kubectl -n nydus exec -i deployment/app-postgres -- psql -U nydus -d nydus -c \
  "SELECT count(*) AS n, count(DISTINCT id) AS distinct_ids FROM records WHERE tenant='tenant-1' AND provider='fakeapi';"
```

Expected: `n = 10000` **and** `distinct_ids = 10000`. If `n != distinct_ids`, there are
duplicates → the upsert is not idempotent; stop and fix. If `n < 10000`, records were
lost → the cursor advanced without the upsert committing; stop and fix.

- [ ] **Step 5 (workstation): confirm the cursor persisted**

```bash
kubectl -n nydus exec -i deployment/app-postgres -- psql -U nydus -d nydus -c \
  "SELECT * FROM cursors WHERE tenant='tenant-1';"
```

Expected: a cursor at (or beyond) the last `id`. This is the incrementality proof.

- [ ] **Step 6 (workstation): run the sync again and confirm zero new records**

```bash
kubectl -n nydus exec -i deployment/temporal -- temporal workflow start \
  --task-queue nydus-sync --type SyncTenant --input '"tenant-1" "fakeapi"'
```

Expected: the count stays `10000`. If it grows, the cursor was not honoured → incrementality
is broken.

**This task is the phase-1 "done when".** Any divergence → stop, diagnose, fix, re-run
from Step 2. Do not proceed to the real provider with an unproven worker.

---

## Task 10: Sync a real provider (GitHub)

- [ ] **Step 1 (workstation): create a read-only fine-grained PAT** (needs a human). Never
      commit it. Store it as a Kubernetes `Secret`:

```bash
kubectl -n nydus create secret generic github-token --from-literal=token=<PAT>
```

- [ ] **Step 2 (workstation): add a `FetchPage` variant for GitHub** — token in the
      `Authorization: Bearer` header, `per_page=100`, follow the `Link` header for the
      next page, honour `X-RateLimit-Remaining` (stop when 0) and `Retry-After` on `429`.
      The "cursor" is the next-page URL.

- [ ] **Step 3 (workstation): rebuild, push, redeploy the worker** (Task 8 Step 3/5).

- [ ] **Step 4 (workstation): start a sync against GitHub**

```bash
kubectl -n nydus exec -i deployment/temporal -- temporal workflow start \
  --task-queue nydus-sync --type SyncTenant --input '"tenant-1" "github"'
```

- [ ] **Step 5 (workstation): verify records landed and the count is sane**

```bash
kubectl -n nydus exec -i deployment/app-postgres -- psql -U nydus -d nydus -c \
  "SELECT count(*), count(DISTINCT id) FROM records WHERE provider='github';"
```

Expected: a positive count, `count == count(DISTINCT id)`. Stop when the rate-limit
budget is exhausted — that behaviour is *correct* and is the point.

- [ ] **Step 6 (workstation): record the rate-limit behaviour** — note the
      `X-RateLimit-Remaining` observed and how the worker behaved at the boundary. This
      becomes input to phase 3/4.

---

## Task 11: Temporal vs Cadence comparison (cuttable)

This task is evidence, not a charter deliverable. Cut it if time is short.

- [ ] **Step 1 (workstation): write `doc/notes/temporal-vs-cadence.md`** — a comparison
      grounded in the *hands-on* experience of Tasks 6–10, not copied from marketing:
      what Temporal actually required to stand up (server roles, Postgres schema, namespace,
      CLI), what the worker code looked like, and where Cadence differs (shared history,
      Cadence's own UI/CLI, schema tooling). State what was observed versus read.

- [ ] **Step 2 (workstation): commit** it.

---

## Task 12: Commit, mark done, update docs

- [ ] **Step 1 (workstation): commit everything**

```bash
git add -A
git commit -m "feat: fakeapi, Temporal durable worker, GitHub provider

fakeapi is the controlled upstream with runtime knobs for every phase-4
failure mode. The worker syncs a tenant via Temporal workflows with
idempotent upserts; killing it mid-sync loses and duplicates nothing
(proven by count). The same worker syncs GitHub end to end."
```

- [ ] **Step 2 (workstation): mark the plan done** — update `doc/plans/README.md`
      (this plan → `Done`), `doc/status.md` (phase 1 state), and `README.md` (quick start)
      in one commit.

---

## Troubleshooting

### Temporal server crash-loops

```bash
kubectl -n nydus logs deployment/temporal | tail -50
```

- `connection refused` on the Postgres seed → `POSTGRES_SEEDS` wrong; the seed must be
  the service name `temporal-postgres`, not `localhost`.
- `relation "schema_history" does not exist` or migration errors → the `POSTGRES_DB`
  is `temporal` but the user lacks `CREATE` privileges, or the DB was recreated.
- `DB=postgres12` is correct even against Postgres 16 — `postgres12` is the *driver*
  name Temporal uses for the Postgres plugin, not a version requirement.

### Storage class missing

```bash
kubectl get storageclass
```

If empty, apply the Rancher local-path provisioner and set it default (a human may be
needed only if `sudo` is required on the server — this runs entirely from the
workstation). kind ships a default `standard` class in current releases; if it is absent
on this version, add the provisioner.

### Worker fails to reach `fakeapi` or `temporal`

```bash
kubectl -n nydus get endpoints fakeapi temporal app-postgres
```

Each should list a pod IP. Empty `endpoints` → the `Service` selector does not match the
pod labels. Compare `kubectl -n nydus get pods --show-labels`.

### Resumability test diverges (n ≠ distinct, or n < N)

- `n > distinct_ids` → duplicate records: the upsert is not truly idempotent (wrong key,
  or a non-deterministic activity). Fix the `ON CONFLICT` clause.
- `n < N` → lost records: the cursor advanced before the upsert committed, or an activity
  returned success without persisting. Fix the ordering (upsert, *then* advance cursor).

### Temporal version drift

Temporal tags move frequently. Before Task 6, check
<https://github.com/temporalio/temporal/releases> (server) and
<https://github.com/temporalio/ui-server/releases> (UI) and pin the current stable in
`deploy/temporal/manifest.yaml`. Do not use `latest`.
