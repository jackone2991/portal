# ADR-08 — Life-OS Positioning + Finance Ledger Scope

**Status:** **accepted** — drafted 2026-07-07 from that day's brainstorm; executed from 2026-07-12 (`6160f8e` SPEC-01/02, `66c036f` SPEC-03 ledger), never formally flipped until 2026-09-11
**Last verified:** 2026-09-11
**Amends:** [ADR-01](01-v1-scope-cut.md) · **Relates to:** D-27/D-28 (step-up/MFA), [ADR-06](06-local-auth-model.md) · yardstick: [product/vision.md](../product/vision.md)

## Context

*As found on 2026-07-07. This is the decision the whole SPEC-01…10 line
descends from; everything below the Decision holds as written.*

Portal's post-v1 gap analyses (`product/backlog.md` as it then was,
`product/analysis/facebook-comparison.md`) measured the product against Facebook.
That yardstick made sense while porting the Olympus UI, but it embeds an
assumption Portal does not satisfy: Facebook's features derive value from network
effects, while Portal is self-hosted, single-VPS, and starts from **one user**.
Following the parity-driven backlog order (friends → messenger → people search)
would spend the scarce solo-dev budget on features that are near-worthless at n=1.

The owner's stated intent (2026-07-07): Portal should be tools supporting the
user's daily life and work — "like a human individual with their surrounding
facets: money, time, learning, social…". Several previously "orphan" spec items
(bank §8, calendar/birthdays, library verticals) are coherent under this framing
and incoherent under Facebook parity.

Two existing architectural assets make an integrated life platform more than a
bundle of clones: the **event bus** (hard rule: modules couple only via Asynq
`<module>:<event>`) and **one identity + RBAC** across all domains.

The immediate scope tension: the owner wanted **money** first, but ADR-01 deferred
"bank" wholesale, and D-27/D-28 gated bank behind MFA/step-up.

## Decision

1. **Portal is a self-hosted life OS**: one digital identity with facets — money,
   time, learning, social, entertainment. Facebook parity is retired as the
   backlog-ordering principle; `facebook-comparison.md` is reclassified as a
   historical analysis.
2. The existing newsfeed surface is re-purposed (long-term) as the user's **life
   stream**, fed by domain events. Every new domain module must emit at least one
   bus event from its first release.
3. **"Bank" is split.** A **finance ledger** (manual multi-account bookkeeping:
   accounts, transactions, categories, budgets, transfers — `product/specs/SPEC-03`)
   enters v1 scope. **Real bank integration** (credentials, API sync, money
   movement) remains deferred exactly as ADR-01 had it.
4. **MFA/TOTP gating is re-anchored**: D-27/D-28's "MFA before bank" applies to
   *credential-holding / money-moving* features, not to the manual ledger, which
   stores no bank credentials. TOTP becomes the named unlock task for real bank
   integration.
5. First build order under the new positioning: media image pipeline → comic
   vertical → finance ledger (`product/specs/`), with the notification module
   immediately after as the life-stream backbone.

## Options considered

- **A. Continue the parity-driven order** (notifications → posts → friends →
  search). Rejected: optimizes believability of a Facebook clone, not value to the
  actual single user; friend graph and messenger are dead weight at n=1.
- **B. Life OS with finance ledger in scope** *(chosen)*: aligns effort with the
  owner's daily use; reuses the event bus as the differentiator; keeps risky bank
  features deferred.
- **C. Entertainment verticals only, defer all finance**: safest read of ADR-01,
  but leaves the owner's top-priority facet (money) unbuilt on a doctrinal
  technicality; the ledger's actual risk profile (no credentials) doesn't warrant it.
- **D. Full §8 bank module including debts/loans/investments now**: rejected;
  violates the v1 envelope and front-loads models (amortization, holdings) with no
  dogfooding behind them.

## Trade-offs

- The Olympus social shell stays partially decorative for longer (friends panel,
  chat bar). Accepted: the shell is kept, only priorities move.
- Two positioning documents coexist during transition (old comparison, new vision);
  mitigated by reclassifying the comparison as historical.
- The ledger without statement import means manual entry only; accepted explicitly
  (owner's bank exports PDF → import needs OCR; schema is import-ready from
  migration #1 so the deferral costs nothing structurally).
- Finance data becomes the most sensitive data in the system while auth is
  password-only (no MFA). Accepted for a self-hosted single-user deployment;
  consequence noted below.

## Consequences

What followed (2026-09-11):

- **The build order ran as decided and kept going:** SPEC-01 (media image
  pipeline) → SPEC-02 (comic) → SPEC-03 (ledger) landed 2026-07-12; SPEC-04
  (notify), 05/06 (journal + life stream), 07, 08 (people/birthdays), 09 (ops)
  and 10 (ledger expansion: debts first) followed. The `bank` module is the
  finance ledger; real bank integration (credentials, API sync, money
  movement) is still deferred, exactly as item 3 said.
- **The life stream exists:** `journal` projects `media:asset_ready`,
  `comic:chapter_published`, `bank:transaction_created` and the rest into the
  stream (SPEC-06). `bank:*`, `comic:*`, `journal:*`, `people:*` are in
  [`reference/events.md`](../reference/events.md); `notify:*` stayed reserved
  for the notification module, which shipped.
- **Backlog re-rank:** happened in the July backlog. That file has since been
  archived-in-place (2026-08-25) and is replaced under ADR-11 by a live one
  triaged from the 2026-08-25 audit. Of the demoted items, email password-reset
  came back and shipped (`0010`, SPEC-04); friend graph shipped a first slice as
  the `social` module (`0037`); messaging and people search did not.
- **Admin wildcard permissions still reach finance data.** `bank:*` is not
  excluded from `*`; there is one operator. SPEC-03 §P0.8 still carries the
  flag for any multi-user deployment.
- **MFA/TOTP is still the named unlock for real bank integration** and is not
  built (ADR-06). The ledger — including debts and interest accrual (SPEC-10) —
  runs under password-only auth, as the trade-off accepted.
- "Posts" changed meaning as predicted: the journal entry is the first real
  post type.
- ADR-01 remains in force for what it still defers: marketplace, creator
  economy, observability, LiveKit/mediamtx. Multi-tenancy/RLS is no longer on
  that list — ADR-07 executed.

## Action items

- [x] Accept this ADR. (Executed from 2026-07-12; status field corrected 2026-09-11.)
- [x] `product/backlog.md` ordering note points at `product/briefs/` + `product/specs/` — then the whole file was archived; the ADR-11 replacement carries the pointer.
- [x] Historical-status header on `product/analysis/facebook-comparison.md` (label only; body untouched — analysis is immutable).
- [x] Build order SPEC-01 → SPEC-02 → SPEC-03; notification module next. All four shipped.
- [ ] Revisit TOTP as a named prerequisite when any credential-holding bank feature is proposed. None has been; SPEC-10's eight items are all manual-entry.
