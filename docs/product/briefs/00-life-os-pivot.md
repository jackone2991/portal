# 00 — Life-OS Pivot (positioning)

**Status:** agreed in brainstorm 2026-07-07 · **ratified as [ADR-08](../../adr/08-life-os-pivot.md)** (landed 2026-07; binding — [vision.md](../vision.md) is the current yardstick).
**Owner:** product (solo dev).

## Problem statement

Portal's gap analyses ([backlog.md](../backlog.md) — was `missing-features.md`,
[facebook-comparison.md](../analysis/facebook-comparison.md)) measure the product against
Facebook. Facebook's value rests on network effects; Portal is self-hosted, single-VPS,
starting from **one user**. Chasing friend graph / messenger / people-search first is
the feature-parity trap: high effort, near-zero value at n=1 users.

## Decision (to be ratified as ADR-08)

Portal is a **self-hosted life OS**: one digital identity ("a person") with facets —
**money, time, learning, social, entertainment**. Two architectural assets already in
place become the glue that makes "everything in one platform" beat "one best-of-breed
app per domain":

1. **The event bus** (hard rule: modules couple only via Asynq `<module>:<event>`).
   Every life domain emits events; the existing newsfeed UI becomes the user's
   **life stream** ("spent 500k today", "mom's birthday in 3 days", "finished chapter 3"),
   not a social timeline.
2. **One identity + RBAC** across all domains instead of five accounts in five apps.

First two axes: **money** (spec 03) and **entertainment** (specs 01–02).

## Goals

- Every new domain module emits at least one event to the bus from day one.
- The first two life domains (finance, comics) usable end-to-end by one real user (the dev).
- Backlog re-ranked to match the positioning (see Consequences).

## Non-goals

- Building the notifications module *now* (it is the life-stream backbone and comes
  right after specs 01–03; see [04-deferred.md](04-deferred.md)). *(2026-07-10: it
  has since been specced as [SPEC-04](../specs/SPEC-04-notification-module.md),
  slotted right after SPEC-01.)*
- Any multi-user social feature as a priority driver (friend graph, messenger,
  people search all demoted — they return when real second users exist).
- Renaming the Olympus UI shell or reworking screens; the shell is reused, only the
  data sources change meaning.

## Consequences for the backlog

- **Demoted from P1:** friend graph, messaging, people search, password reset via
  email (its P1 rationale — "unblocks a shipped surface" — assumed a multi-user
  production product that does not exist yet; users are seeded via admin/CLI).
- **Promoted / re-motivated:** notifications & event stream (as life-stream backbone,
  not as password-reset plumbing); `media:asset_ready` event emit moves up from P3.
- **Re-scoped:** "bank" splits into **finance ledger** (in scope now, spec 03) vs
  **real bank integration** (still deferred; needs TOTP/step-up first).
- **Posts** change meaning: the first real post type is a **journal / life event of
  the user**, not a status for friends.

## Action items

- [x] Write **ADR-08** — done: [ADR-08](../../adr/08-life-os-pivot.md), amending
      [ADR-01](../../adr/01-v1-scope-cut.md): life-OS positioning; finance ledger
      into scope; "real bank" stays deferred.
- [x] Update the next-order pointer — `missing-features.md` was retired into
      [backlog.md](../backlog.md) by the ADR-09 restructure.
- ~~Keep `doc/vi/` mirrors in sync~~ — retired by
  [ADR-09](../../adr/09-docs-architecture.md): docs are English-only; the vi mirror
  is a frozen archive.

## Open questions

- **(product, non-blocking)** Does the "life stream" replace the newsfeed on `/`
  or live alongside it as a tab? Decide when the first two event producers exist.
- **(product, non-blocking)** Time domain (calendar/tasks) was the cheapest first
  life domain but was consciously ordered after money+entertainment — revisit after 03.
