# Test Cases — SPEC-05 Journal (life-stream write path)

**Spec:** [SPEC-05](../product/specs/SPEC-05-journal.md) · **Module:** `journal`
**Prefix:** `TC-JRNL-` · **Plan:** [TEST-PLAN.md](TEST-PLAN.md) · **Risk:** R7 (capture not saved)

### Endpoints under test

| Method | Path | Perm |
|---|---|---|
| POST | `/api/v1/journal/entries` | `journal:write:own` |
| GET | `/api/v1/journal/entries?cursor=` | `journal:read:own` |
| GET | `/api/v1/journal/entries/{id}` | `journal:read:own` |
| PATCH | `/api/v1/journal/entries/{id}` | `journal:write:own` |
| DELETE | `/api/v1/journal/entries/{id}` | `journal:delete:own` |

### Preconditions

- Accounts `owner`,`userA`,`userB`,`guest`. Problem types: `journal/entry-not-found`,
  `journal/invalid-body`, `journal/invalid-mood`, `journal/invalid-asset`.
- Event `journal:entry_created {entry_id, user_id, occurred_at}` — **emit-only** at v1.

---

## P0.1 — Module scaffold

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-JRNL-001 | Module mounted end-to-end | Contract | P0 | GET `/journal/entries` authenticated | 200 (not 404); module constructed in cmd/api + cmd/worker | ☐ |
| TC-JRNL-002 | depguard isolation block | Contract | P1 | inspect `.golangci.yml` | journal isolation block present; lint green | ☐ |

## P0.2 — Entries CRUD

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-JRNL-010 | Create entry (text+mood) | Functional | P0 | POST {body_md, mood} | 201; row created; occurred_at defaults now | ☐ |
| TC-JRNL-011 | Owner isolation | AuthZ | P0(S1) | userA lists/fetches userB's entries | absent; direct fetch 404 (existence never leaks) | ☐ (CC-3) |
| TC-JRNL-012 | Cursor stable over 500 | Functional | P0 | page by cursor | stable, ordered `occurred_at DESC, id DESC`; no dupes/gaps | ☐ (CC-4) |
| TC-JRNL-013 | Body 0 chars → 422 | Boundary/Neg | P0 | POST body_md="" | 422 `journal/invalid-body` | ☐ |
| TC-JRNL-014 | Body > 20 000 chars → 422 | Boundary/Neg | P0 | POST 20 001-char body | 422 `journal/invalid-body` | ☐ |
| TC-JRNL-015 | Body 20 000 boundary accepted | Boundary | P1 | POST exactly 20 000 | 201 | ☐ |
| TC-JRNL-016 | Whitespace-only mood → 422 | Boundary/Neg | P0 | POST mood="   " | 422 `journal/invalid-mood` (not a 500 at DB) | ☐ |
| TC-JRNL-017 | Mood > 80 chars → 422 | Boundary/Neg | P0 | POST 81-char mood | 422 `journal/invalid-mood` | ☐ |
| TC-JRNL-018 | Backdate/future-date unlimited | Functional | P0 | POST occurred_at last night / next year | accepted; sits at that date position | ☐ |
| TC-JRNL-019 | asset_ids rejected pre-P1.5 | Negative | P0 | POST with asset_ids | 422 `journal/invalid-asset` (fail closed) | ☐ |
| TC-JRNL-020 | Edit updates updated_at, keeps position | Functional | P0 | PATCH body only | `updated_at` changes; `occurred_at` position unchanged | ☐ |
| TC-JRNL-021 | Edit occurred_at re-sorts | Functional | P0 | PATCH occurred_at | entry re-sorts to new date position | ☐ |
| TC-JRNL-022 | Delete removes entry + stream row | Functional | P0 | DELETE entry | gone from list/fetch; SPEC-06 stream row also removed (transactional) | ☐ |
| TC-JRNL-023 | Idempotent delete | Idempotency | P0 | DELETE twice | 2nd → 404, never 500 | ☐ (CC-8) |

## P0.3 — Event emit

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-JRNL-030 | Emit once after commit | Integration | P0 | create entry (spy publisher) | `events.Publish("journal:entry_created", payload)` called **once**, strictly after commit | ☐ (CC-5) |
| TC-JRNL-031 | Rollback → no emit | Integration | P0 | force create rollback | no publish | ☐ (CC-5) |
| TC-JRNL-032 | Emit-only, no consumer tasks | Integration | P0 | inspect queue | zero consumer tasks enqueued (no registered subscribers); raw event name never a task type | ☐ (CC-5) |
| TC-JRNL-033 | No entry_updated/deleted events | Contract | P1 | edit/delete entry | no `entry_updated`/`entry_deleted` events (projection is transactional in-module) | ☐ |

## P0.4 — Composer + interim home list (frontend)

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-JRNL-050 | Optimistic insert then refetch | Frontend | P0 | post from composer | entry appears at occurred_at position (top when now) without full refetch; survives immediate refetch | ☐ (CC-9) |
| TC-JRNL-051 | Error rollback restores text | Frontend | P0 | force server error on post | entry disappears; composer restores its text | ☐ (CC-9) |
| TC-JRNL-052 | Edit re-sorts optimistically | Frontend | P0 | edit occurred_at inline | card re-sorts to new position | ☐ |
| TC-JRNL-053 | Delete leaves optimistically | Frontend | P0 | delete card | leaves list optimistically | ☐ |
| TC-JRNL-054 | Fixtures gone (grep) | Frontend | P0 | grep `HomeView` | fixture post array + fake composer removed; every rendered entry is a DB row | ☐ (CC-9) |
| TC-JRNL-055 | Script renders inert (sanitize) | Security | P0(S1) | body_md = `<script>alert(1)</script>` | renders as inert text, never live markup (sanitizing renderer) | ☐ |
| TC-JRNL-056 | Raw HTML not passed through | Security | P0 | body_md with raw `<img onerror=...>` | sanitized/inert | ☐ |
| TC-JRNL-057 | Mood stored + rendered | Functional | P0 | post with mood | created entry stores + renders mood | ☐ |
| TC-JRNL-058 | Backdate via date control | Frontend | P1 | use composer date control | occurred_at set; entry lands at that position | ☐ |
| TC-JRNL-059 | Persistence across restart | Reliability | P0 | post; `make up` restart; reload | entries persist (DB not cache) | ☐ |

## P1 — nice to have

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-JRNL-070 | Photo attachments validated | Functional | P1 | POST asset_ids (≤10, ready image, owned) | accepted; thumb variants render; lightbox medium | ☐ [P1] |
| TC-JRNL-071 | Invalid attachment → 422 | Negative | P1 | asset_ids with non-image/non-ready/not-owned | 422 `journal/invalid-asset` | ☐ [P1] |
| TC-JRNL-072 | > 10 attachments → 422 | Boundary | P1 | 11 asset_ids | 422 | ☐ [P1] |
| TC-JRNL-073 | media:asset_deleted strips id | Integration | P1 | delete an attached asset media-side | id stripped from entry `asset_ids` (idempotent soft-cascade) | ☐ [P1] |
| TC-JRNL-074 | Mood picker preset row | Frontend | P1 | composer mood picker | preset emoji row + freeform field | ☐ [P1] |

## Cross-cutting / contract

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-JRNL-090 | All non-2xx RFC-7807 | Contract | P0 | error paths | Problem+json + stable type | ☐ (CC-1) |
| TC-JRNL-091 | Problem types have i18n keys | Contract | P1 | grep problems.ts | all journal types present | ☐ (CC-1) |
| TC-JRNL-092 | Permission seeding | AuthZ | P0 | inspect grants | `journal:read/write/delete:own` seeded → `user` | ☐ (CC-2) |
| TC-JRNL-093 | Migration up/down | Contract | P1 | migrate + down `journal_entries` | clean; body/mood CHECKs; occurred_at index; identity-anchor FK | ☐ |
