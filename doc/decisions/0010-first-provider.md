# 0010 — First provider: GitHub

**Status:** Accepted · 2026-09-28

## Context

The system is a personal data hub ([ADR 0007](0007-application-scope.md)). Provider
selection was deliberately deferred to phase 1, with three criteria: rate limits that are
tight and documented, the cheapest auth that still exercises the real path, and a second
provider that differs in kind.

A further requirement from the author: the provider must have **actual personal value** —
it should deliver data the author wants to read, not just a convenient API. The concrete
use: tracking new versions of tools the author uses for this setup itself — `neovim`
releases, `kickstart.nvim` updates, and releases of local-LLM tooling such as `llama.cpp`
and `ollama` — as feed for the summary agent.

## Decision

**GitHub** is the first provider, consumed via its Releases and commits endpoints.

The "tenant data" is a curated list of repositories; each tenant-provider pair tracks the
releases (or commits) of one repository, incrementally. This maps cleanly onto the
worker's cursor model: the `Link` header's next-page URL is the cursor.

## Consequences

**Good**

- Meets ADR 0007's "published rate-limit headers" criterion exactly: `X-RateLimit-Limit`,
  `X-RateLimit-Remaining`, `X-RateLimit-Reset` make the rate budget *observable* — the
  signal phase 3 wants to graph.
- Token auth is trivial (a read-only fine-grained PAT) and is the "cheapest auth that
  still exercises the real path".
- Pagination via `Link` header is page-based, a genuine contrast to a cursor-style API
  and a clean fit for the worker's "next page = cursor" abstraction.
- Personal value is real and immediate (release notes for the author's own tools), and
  the API is stable and well-documented.

**Bad**

- The quota is per-account, not per-token, so synthetic tenants sharing one account's
  tokens contend for one 5000-req/hr pool. This is *desirable* for the contention
  measurement but must be called out rather than hidden.
- Release/commit data is a narrow slice of GitHub; it does not exercise the GraphQL
  cursor path unless that is added later.
- The rate limit (5000/hr) is generous; tight-limit stress comes from `fakeapi` in
  phase 4 rather than from GitHub itself.

## Alternatives considered

- **Notion** — token auth and a genuinely tight (~3 req/s) limit, but no remaining-budget
  header and cursor-only pagination. Rejected for first: the observable budget and
  pagination variety of GitHub matter more to the measurement story. Notion remains a
  candidate for a later, contrast-provider slot.
- **YouTube Data API** — high personal value (UFC cards, channel news), but quota-based
  limits without a running budget header and page-token pagination. Chosen as the
  anticipated **second** provider, for contrast, to be recorded as a later ADR when it is
  implemented.
