# Portal — Vision

**Status:** current ([ADR-08](../adr/08-life-os-pivot.md), accepted) · **Last verified:** 2026-10-01

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

1. **The event bus.** Modules couple asynchronously only through Asynq events
   (`<module>:<event>`); synchronous reads go through each module's `api/`
   package ([backend/MODULES.md](../../backend/MODULES.md)). Every life domain
   emits events, so the feed surface
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
| Entertainment | `media`, `comic`, `music`, `movie`, `story` | comic was the first vertical ([SPEC-14](specs/SPEC-14-comic-vertical.md)); music has its full UI ([SPEC-15](specs/SPEC-15-music-vertical.md)); movie and story have a backend but no reader yet ([SPEC-16](specs/SPEC-16-movie-vertical.md), [SPEC-17](specs/SPEC-17-story-vertical.md)) |
| Money | `bank` (ledger scope) | [SPEC-12](specs/SPEC-12-finance-ledger.md), expanding through [SPEC-13](specs/SPEC-13-ledger-expansion.md); real-bank integration deferred behind TOTP |
| Time | calendar/events/reminders | calendar widget exists; birthdays shipped as contact data ([SPEC-11](specs/SPEC-11-people-registry.md)); next facet after money (backlog § Deferred) |
| Social | connections between accounts; posts, messaging | only the first slice exists — mutual connections ([SPEC-18](specs/SPEC-18-social-connections.md)), built once approval-gated registration made the instance n>1; everything else stays deferred |
| Learning | stories, library | story is the entertainment vertical above (SPEC-17); otherwise unshaped |

## Operating constraints (inherited from ADR-01)

1 developer · 2-week build bursts · ≤ $100/month · a single VPS. Every scope
decision answers to this envelope. Deferred-with-conditions list:
[backlog.md § Deferred](backlog.md).

## Success, honestly measured at n=1

The owner uses Portal daily for at least two facets (logs money on ≥20 of 30 days;
reads/watches through Portal weekly), and month-end finance reconciliation closes
clean. Growth metrics are meaningless here; **habitual self-use is the bar**.
