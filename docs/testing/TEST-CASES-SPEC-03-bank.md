# Test Cases — SPEC-03 Finance Ledger (`bank`)

**Spec:** [SPEC-03](../product/specs/SPEC-03-finance-ledger.md) · **Module:** `bank`
**Prefix:** `TC-BANK-` · **Plan:** [TEST-PLAN.md](TEST-PLAN.md)
**Risk:** R1 (money incorrectness) + R3 (finance is the most sensitive data) — treat every S1 here as release-gating.

### Endpoints under test

| Method | Path | Perm |
|---|---|---|
| GET/POST | `/api/v1/bank/accounts` | `bank-accounts:read/write:own` |
| PATCH/DELETE | `/api/v1/bank/accounts/{id}` | `bank-accounts:write/delete:own` |
| GET/POST | `/api/v1/bank/transactions?account=&month=&category=&cursor=` | `bank-transactions:read/write:own` |
| PATCH/DELETE | `/api/v1/bank/transactions/{id}` | `bank-transactions:write/delete:own` (409 on transfer legs) |
| POST | `/api/v1/bank/transfers` | `bank-transactions:write:own` |
| PATCH/DELETE | `/api/v1/bank/transfers/{transfer_id}` | `bank-transactions:write/delete:own` |
| GET/POST | `/api/v1/bank/categories` | `bank-categories:read/write:own` |
| PATCH/DELETE | `/api/v1/bank/categories/{id}` (`?reassign_to=`) | `bank-categories:write/delete:own` |
| GET/PUT | `/api/v1/bank/budgets?month=` | `bank-budgets:read/write:own` |
| GET | `/api/v1/bank/dashboard?month=` | `bank-accounts:read:own` |

### Preconditions

- Accounts `owner`(user grants for all bank-*), `userA`, `userB`, `admin`, `guest`.
- Seed categories present (26 VN categories, `user_id NULL`) from migration.
- **All money is integer minor units (VND, exponent 0)** — assert no floats/strings on the wire (CC-6).
- Problem types: `bank/account-not-empty`, `bank/account-not-mutable`,
  `bank/is-transfer-leg`, `bank/same-account-transfer`, `bank/currency-mismatch`,
  `bank/category-in-use`, `bank/category-kind-mismatch`, `bank/direction-kind-mismatch`,
  `bank/invalid-amount`, `bank/invalid-category-parent`, `bank/category-immutable`, `bank/validation`, `bank/invalid-cursor`.

---

## P0.1 — Accounts

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-BANK-001 | Create account each type | Functional | P0 | POST accounts for cash/checking/savings/credit_card/ewallet/other + opening_balance | 201 each; `type` accepted; derived balance = opening_balance | ☐ |
| TC-BANK-002 | Invalid type → 422 | Boundary/Neg | P0 | POST type="foo" | 422 `bank/validation` | ☐ |
| TC-BANK-003 | Derived balance (never stored) | Data-integrity | P0 | add txns; GET account | balance = opening + Σcredits − Σdebits; no mutable stored balance column | ☐ |
| TC-BANK-004 | Archived leaves pickers, stays in reports | Functional | P0 | archive an account w/ history | absent from entry pickers; present in dashboard archived section + past reports at current balance | ☐ |
| TC-BANK-005 | Currency immutable once txns | Negative | P0 | PATCH currency on account w/ 1 txn | 409 `bank/account-not-mutable` | ☐ |
| TC-BANK-006 | Delete non-empty account blocked | Negative | P0 | DELETE account w/ 1 txn | 409 `bank/account-not-empty` | ☐ |
| TC-BANK-007 | Delete empty account allowed | Functional | P1 | DELETE account w/ 0 txns | 2xx; gone | ☐ |
| TC-BANK-008 | Credit card can go negative | Functional | P1 | debit beyond balance on credit_card | allowed (negative balance) | ☐ |

## P0.2 — Transactions

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-BANK-020 | Create expense (debit) | Functional | P0 | POST txn {account, category(expense), amount, direction:debit, occurred_at} | 201; account balance −amount; month expense +amount | ☐ |
| TC-BANK-021 | Create income (credit) | Functional | P0 | POST txn credit + income category | 201; balance +amount; month income +amount | ☐ |
| TC-BANK-022 | Edit recomputes derived numbers | Data-integrity | P0 | PATCH amount/category/account | balance + month totals update correctly | ☐ |
| TC-BANK-023 | Delete recomputes | Data-integrity | P0 | DELETE txn | balance + totals revert | ☐ |
| TC-BANK-024 | **Reconciliation property test** | Data-integrity | P0(S1) | random create/edit/delete sequences | derived balances + month totals always reconcile (no drift) | ☐ [AUTO] |
| TC-BANK-025 | Amount ≤ 0 → 422 | Boundary/Neg | P0 | POST amount 0 and −100 | 422 `bank/invalid-amount` | ☐ |
| TC-BANK-026 | Fractional VND → 422 | Boundary/Neg | P0 | POST amount 100.5 | 422 `bank/invalid-amount` (integer minor units only) | ☐ |
| TC-BANK-027 | Direction/kind mismatch → 422 | Negative | P0(S1) | debit on income category; credit on expense category | 422 `bank/direction-kind-mismatch` (else spend silently vanishes) | ☐ |
| TC-BANK-028 | Foreign category id → 404 | AuthZ | P0(S1) | txn with userB's category id | 404 (existence never leaks) | ☐ (CC-3) |
| TC-BANK-029 | Seed category attach allowed | Functional | P0 | txn with a seed (`user_id NULL`) category | accepted | ☐ |
| TC-BANK-030 | Future occurred_at accepted | Boundary | P0 | POST occurred_at next month | accepted; counts in that future month | ☐ |
| TC-BANK-031 | Quick-add ≤ 10 s | Performance | P1 | practiced user opens dialog→save | ≤ 10 s; ≤4 required fields; defaults applied (expense, last account, MRU category, today) | ☐ [MANUAL] |
| TC-BANK-032 | MoneyInput thousands separators | Frontend | P1 | type 1500000 | renders `1.500.000`; wire = integer minor units | ☐ |
| TC-BANK-033 | Transactions list cursor paging | Functional | P0 | >1 page; paginate | order `occurred_at DESC, id DESC`; `next_cursor`; no dupes/skips under inserts/edits | ☐ (CC-4) |
| TC-BANK-034 | List filters account/month/category | Functional | P1 | apply each filter | correct subset | ☐ |

## P0.3 — Transfers

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-BANK-050 | Transfer moves balances, totals 0 | Data-integrity | P0(S1) | POST transfer TCB→Momo 5,000,000 | TCB −5M, Momo +5M; month income **+0**, expense **+0** | ☐ |
| TC-BANK-051 | Paired legs share transfer_id, no category | Functional | P0 | inspect the two rows | both `transfer_id` set, `category_id NULL`; leg predicate holds | ☐ |
| TC-BANK-052 | Delete transfer atomic (both legs) | Data-integrity | P0(S1) | DELETE `/transfers/{id}` | both legs gone atomically; no orphan leg under any failure (rollback test) | ☐ |
| TC-BANK-053 | Edit transfer both legs | Functional | P0 | PATCH `/transfers/{id}` amount | both legs update in one tx | ☐ |
| TC-BANK-054 | Leg not independently editable | Negative | P0 | PATCH/DELETE a leg's txn id | 409 `bank/is-transfer-leg` pointing at transfer endpoint | ☐ |
| TC-BANK-055 | Same-account transfer → 422 | Negative | P0 | transfer from=to | 422 `bank/same-account-transfer` | ☐ |
| TC-BANK-056 | Cross-currency transfer → 422 | Negative | P0 | transfer VND→USD accounts | 422 `bank/currency-mismatch` | ☐ |
| TC-BANK-057 | Transfer badge + counterparty in list | Frontend | P1 | list w/ transfer | legs render transfer badge + counterparty account name | ☐ |
| TC-BANK-058 | Transfer + manual fee reconciles | Data-integrity | P0(S1) | transfer 5M TCB→Momo + 2,200 fee expense on TCB (seed *Phí & Lệ phí*) | TCB −5,002,200, Momo +5,000,000, month expense +2,200, income +0; month-end reconciles | ☐ |
| TC-BANK-059 | [P1.13] Structured fee row | Functional | P1 | POST transfer {fee_amount, fee_category_id} | 3rd row: debit on source, expense-kind, transfer_id set, category set; reporting counts it as expense; pair still 0 | ☐ [P1] |

## P0.4 — Categories (hierarchy + delete/reassign)

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-BANK-070 | Create top-level + child (same kind) | Functional | P0 | POST parent (expense); POST child parent_id=parent | 201 both; 2-level tree | ☐ |
| TC-BANK-071 | Parent must be top-level → 422 | Negative | P0 | child's parent_id = a non-top-level category | 422 `bank/invalid-category-parent` (enforces 2 levels) | ☐ |
| TC-BANK-072 | Child kind must equal parent | Negative | P0 | child kind ≠ parent kind | 422 (kind mismatch) | ☐ |
| TC-BANK-073 | Foreign parent_id → 404 | AuthZ | P0(S1) | parent_id = userB's category | 404 (existence never leaks) | ☐ (CC-3) |
| TC-BANK-074 | kind immutable after create | Negative | P0 | PATCH kind | 422 `bank/category-immutable` | ☐ |
| TC-BANK-075 | Re-parent category with children → 422 | Negative | P0 | PATCH parent_id on a category that has children | 422 `bank/invalid-category-parent` | ☐ |
| TC-BANK-076 | Seed not deletable | Negative | P0(S1) | DELETE a seed category | 404; seed survives (owner-scoped mutation matches 0 rows) | ☐ |
| TC-BANK-077 | Delete category w/ txns w/o reassign → 409 | Negative | P0 | DELETE category having txns, no `?reassign_to` | 409 `bank/category-in-use`; nothing changes | ☐ |
| TC-BANK-078 | Delete + reassign moves txns | Functional | P0(S1) | DELETE `?reassign_to=` (same kind, own/seed) | txns move to target in same tx; category gone | ☐ |
| TC-BANK-079 | reassign_to kind mismatch → 422 | Negative | P0 | delete expense cat, reassign_to income cat | 422 `bank/category-kind-mismatch`; nothing changes | ☐ |
| TC-BANK-080 | reassign_to foreign/nonexistent → 404 | Negative | P0 | reassign_to = userB's / random id | 404 | ☐ |
| TC-BANK-081 | Delete parent promotes children | Functional | P0 | delete a parent (ON DELETE SET NULL) | children become top-level; their txns+budgets unchanged; parent's budgets gone | ☐ |
| TC-BANK-082 | Parent delete drops its budgets (all months) | Functional | P0 | parent had budgets incl past months | budgets cascade-deleted (documented history erasure) | ☐ |

## P0.5 — Monthly budgets (tree)

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-BANK-100 | Budget PUT upsert | Functional | P0 | PUT budget {category, month, amount} | row upserted; amount>0 | ☐ |
| TC-BANK-101 | Budget amount 0/null deletes | Functional | P0 | PUT amount:0 then amount:null | budget row deleted (only removal path) | ☐ |
| TC-BANK-102 | Budget on income-kind → 422 | Negative | P0 | PUT budget naming income category | 422 `bank/category-kind-mismatch` | ☐ |
| TC-BANK-103 | Spent includes children, excludes legs | Data-integrity | P0(S1) | budget 3,000,000 on Ăn uống; 3,200,000 spent across children | bar shows 107% + highlighted; transfer legs excluded from spent | ☐ |
| TC-BANK-104 | Nested tree, no double-count | Functional | P0 | budgets Ăn uống 3M + Ăn ngoài 1M; one 1M expense in Ăn ngoài | Ăn ngoài 100% nested under Ăn uống 33%; **one tree not two flat siblings** | ☐ |
| TC-BANK-105 | Dashboard compact = no-budgeted-ancestor | Functional | P0 | dashboard budget block for TC-BANK-104 | shows only Ăn uống bar (compact roll-up); child-only budgets still surface | ☐ |
| TC-BANK-106 | Unbudgeted parent header for budgeted child | Functional | P0 | budget only a child | `/bank/budgets` renders unbudgeted parent as non-bar header (synthesized entry, no client join) | ☐ |
| TC-BANK-107 | No budget → no bar, no zero-division | Boundary | P0 | category w/o budget | no progress bar; no zero-division state | ☐ |
| TC-BANK-108 | Budget-list carries id+parent_id+name | Contract | P1 | GET `/bank/budgets` | items carry category_id, parent_id, name | ☐ |

## P0.6 — Dashboard

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-BANK-120 | VND-only single currency group | Functional | P0 | GET `/bank/dashboard` | one currency group | ☐ |
| TC-BANK-121 | `?month=` scopes flows only, not balances | Data-integrity | P0(S1) | GET `?month=2026-06` | only flow totals + budget bars change; **balances stay current** (opening-balance rule) | ☐ |
| TC-BANK-122 | Totals exclude legs (predicate, not transfer_id null) | Data-integrity | P0(S1) | dashboard with transfers + a P1.13 fee row | income/expense exclude legs but include the fee row | ☐ |
| TC-BANK-123 | Archived account at current balance | Functional | P0 | archived account w/ history | appears in archived section at current balance; its txns count in past months' flows | ☐ |
| TC-BANK-124 | Recent 10 by created_at | Functional | P0 | add a future-dated txn | recent list keyed `created_at DESC` (future entry doesn't pin the list) | ☐ |

## P0.7 — Events

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-BANK-140 | One event per txn CUD, after commit | Integration | P0 | create/update/delete a txn | exactly one `bank:transaction_{created,updated,deleted}` after commit; rollback → none | ☐ (CC-5) |
| TC-BANK-141 | Payload shape complete | Contract | P0 | inspect event payload | `{transaction_id,user_id,account_id,amount,direction,category_id,occurred_at,is_transfer,transfer_id,counterparty_account_id}` | ☐ |
| TC-BANK-142 | Transfer legs emit is_transfer=true + counterparty | Integration | P0 | create transfer | each leg emits `is_transfer=true`, `counterparty_account_id` = other leg's account | ☐ |
| TC-BANK-143 | Bulk reassign emits no flood | Integration | P0 | category DELETE `?reassign_to=` moving 500 txns | **zero** `bank:transaction_updated` events (documented carve-out) | ☐ |
| TC-BANK-144 | Event reaches stream (fan-out edge on api) | Integration | P0(S1) | create txn; check `/stream` | stream item appears (api publisher has bank→stream edges — regression for empty-routing bug) | ☐ (CC-5) |

## P0.8 — RBAC

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-BANK-160 | Owner isolation (all entities) | AuthZ | P0(S1) | userA lists/fetches userB's accounts/txns/categories/budgets | none returned; direct fetch 404 | ☐ (CC-3) |
| TC-BANK-161 | Seed readable by all, mutation 404 | AuthZ | P0 | any user reads seed; attempts mutation | reads include seed; mutation matches 0 rows → 404 (immutable even for superadmin — no `:any` mutation surface) | ☐ |
| TC-BANK-162 | Codes are 2–3 segment | Contract | P0 | inspect seeded permission codes | `bank-accounts:read/write/delete:own` etc.; no 4-segment | ☐ (CC-2) |
| TC-BANK-163 | No cross-user read at non-admin level | AuthZ | P0(S1) | any non-admin cross-user read | denied | ☐ |
| TC-BANK-164 | Unauthenticated → 401 | AuthZ | P0 | guest hits bank endpoints | 401 | ☐ |

## P0.9 — Import scaffolding

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-BANK-180 | Partial-unique dedup guard | Data-integrity | P1 | two inserts sharing `(account_id, dedup_hash)` | 2nd fails on partial unique index (future import dedup, testable now) | ☐ [AUTO] |
| TC-BANK-181 | Import columns present + nullable | Contract | P1 | inspect schema | `description_raw`, `import_batch_id`, `dedup_hash` present, nullable; `bank_import_batches` table exists | ☐ |

## Cross-cutting / contract

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-BANK-200 | All non-2xx RFC-7807 | Contract | P0 | error paths | Problem+json + stable type | ☐ (CC-1) |
| TC-BANK-201 | Problem types have i18n keys | Contract | P1 | grep problems.ts | all bank types present | ☐ (CC-1) |
| TC-BANK-202 | No floats/strings on the wire | Contract | P0(S1) | inspect all bank req/resp bodies | integer minor units end-to-end | ☐ (CC-6) |
| TC-BANK-203 | Idempotent deletes | Idempotency | P0 | DELETE account/txn/category twice | 2nd → 404/appropriate, never 500 | ☐ (CC-8) |
| TC-BANK-204 | Handler↔OpenAPI drift | Contract | P1 | compare spec vs live | matches or documented deviation | ☐ (CC-7) |
| TC-BANK-205 | Migration up/down | Contract | P1 | migrate + down `bank_core` | clean; CHECKs, partial unique, identity-anchor FKs, seed categories present | ☐ |
