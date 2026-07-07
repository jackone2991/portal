# Portal — Vision

**Status:** current (ratification pending [ADR-08](../adr/08-life-os-pivot.md)) · **Last verified:** 2026-07-07

## One sentence

Portal is a **self-hosted life OS**: one digital identity with the facets of a
person's life — **money, time, learning, social, entertainment** — running on the
user's own hardware, owned end to end.

## What that means (and doesn't)

Portal is *not* a Facebook clone measured by feature parity. The Olympus UI gave it
a social skin, but its value proposition is different: where Facebook's features
live off network effects, Portal's live off **integration for a single person** —
your ledger, your library, your calendar, your journal, in one place, under one
login, on one VPS you control.

Two architectural assets make "one platform" beat "one best app per domain":

1. **The event bus.** Modules couple only through Asynq events
   (`<module>:<event>`). Every life domain emits events, so the feed surface
   becomes the user's **life stream** — "spent 500k today", "mom's birthday in 3
   days", "finished chapter 3" — a timeline of a life, not of a network.
2. **One identity + RBAC** across every facet, instead of five accounts in five
   apps.

## Who it's for

First user: the owner-operator (n=1 is a feature, not a bug — a life OS is useful
from one user). Second ring, later: the household — the tenant module's
`kind: household` and the deferred multi-tenancy design (ADR-07) exist for exactly
this, when real second users appear.

## The facets and where they stand

| Facet | Modules | State |
|---|---|---|
| Entertainment | `media` (real), `comic`/`movie`/`music`/`story` | media works; comic is the first vertical (SPEC-02) |
| Money | `bank` (ledger scope) | SPEC-03; real-bank integration deferred behind TOTP |
| Time | calendar/events/reminders | UI widgets exist; next facet after money (briefs/04) |
| Social | posts, friends, messaging | UI shell built; deliberately demoted until n>1 |
| Learning | stories, library | skeleton; unshaped |

## Operating constraints (inherited from ADR-01)

1 developer · 2-week build bursts · ≤ $100/month · a single VPS. Every scope
decision answers to this envelope. Deferred-with-conditions list:
[briefs/04-deferred.md](briefs/04-deferred.md).

## Success, honestly measured at n=1

The owner uses Portal daily for at least two facets (logs money on ≥20 of 30 days;
reads/watches through Portal weekly), and month-end finance reconciliation closes
clean. Growth metrics are meaningless here; **habitual self-use is the bar**.
