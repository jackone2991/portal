# ADR-08 — Life-OS Positioning + Finance Ledger Scope

**Status:** proposed (drafted 2026-07-07, from the 2026-07-07 brainstorm)
**Amends:** [ADR-01](01-v1-scope-cut.md) · **Relates to:** D-27/D-28 (step-up/MFA), [ADR-06](06-local-auth-model.md)

## Context

Portal's post-v1 gap analyses (`product/backlog.md`,
`product/analysis/facebook-comparison.md`) measure the product against Facebook.
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

The immediate scope tension: the owner wants **money** first, but ADR-01 defers
"bank" wholesale, and D-27/D-28 gate bank behind MFA/step-up.

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

- Backlog re-rank (recorded in `product/backlog.md`): friend graph, messaging,
  people search, email password-reset **demoted**; notifications re-motivated as
  life-stream backbone; `media:asset_ready` event emission promoted.
- Event names `bank:*`, `comic:*` join the registry (`reference/events.md`);
  `notify:*` remains reserved for the notification module.
- Admin wildcard permissions technically reach finance data. For now this is the
  owner-operator themselves; before any multi-user deployment, revisit whether
  `bank:*` should be excluded from generic wildcard grants (flagged in SPEC-03 §P0.8).
- "Posts" change meaning: the first real post type is a journal/life event, not a
  status update — affects the future posts spec, not current work.
- ADR-01 remains in force for everything else it defers (multi-tenancy/RLS,
  marketplace, creator economy, observability, LiveKit).

## Action items

- [ ] Accept this ADR (owner) and flip status to accepted.
- [ ] Update `product/backlog.md` ordering note to point at `product/briefs/` +
      `product/specs/`.
- [ ] Add the historical-status header to `product/analysis/facebook-comparison.md`.
- [ ] Build order: SPEC-01 → SPEC-02 → SPEC-03; notification module scheduled next.
- [ ] Revisit TOTP as a named prerequisite when any credential-holding bank
      feature is proposed.
