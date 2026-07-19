# Session Handoff — 2026-07-12 → continue here

> **Purpose:** pick up tomorrow without re-deriving context. Read this top-to-bottom.
> **Local working note — NOT committed** (contains dev test creds). Keep untracked or
> strip creds before committing.

---

## 0. TL;DR — where we are

- **All P0 sprints (SPEC-01…09) are code-complete and the backend is functionally healthy.**
  A full test run today (see [TEST-RUN-2026-07-12.md](TEST-RUN-2026-07-12.md)) = **0 real
  functional defects**; L1/L2 `go test` 12/12 packages green; ~76/78 live API cases pass.
- **This session's work landed 4 commits** (all live-verified) — see §3.
- **The stack is up** (`make up`, 8 services healthy) with migration **v17**.
- **Next recommended build:** **Takeout `/me/export`** (SPEC-09 P1.7) — needs a per-module
  export-format decision before starting (§6).

## 1. Bring up & verify the stack

```bash
cd D:/lamnx/Workspaces/git/github/portal
make up                 # (already up today; use if restarting)
# health:
curl -sk https://api.portal.localhost/healthz         # → {"status":"ok","db":true,"cache":true}
# migration version (expect 17, dirty=f):
docker compose exec -T postgres psql -U portal -d portal -tAc "select version,dirty from schema_migrations;"
```

**Toolchain is NOT on PATH** (see memory `portal-toolchain-paths.md`):
- go: `/c/Program Files/Go/bin/go.exe` · sqlc/oapi-codegen/mkcert: `/c/Users/ADMIN/go/bin/*.exe`
- Backend tests: `CGO_ENABLED=0 go test ./...` (`-race` needs cgo, unavailable).
- Frontend gate: `tsc --noEmit` (`next lint`/`next build` prompt interactively — avoid).
- Rebuild a container after backend code change: `docker compose build api worker && docker compose up -d api worker`.

## 2. Test accounts (LOCAL DEV throwaways — `.localhost`, not real secrets)

All use password **`Str0ng-Passw0rd!`**.

| Email | Roles | Use |
|-------|-------|-----|
| `dogfood@portal.localhost` | user, **creator** | the "owner" — has comics:write/publish:own, all bank/journal/people/media |
| `userb@portal.localhost` | user | owner-isolation reference (cross-user 404 tests) |
| `admin@portal.localhost` | user, **admin** | queues:read, ops:read, *:delete:any — for `/admin/queues` + ops |

> Grant a role manually (then bump token_version so the perm cache refreshes):
> ```sql
> INSERT INTO user_roles (user_id, role_id) SELECT u.id,r.id FROM users u,roles r
>   WHERE u.email='<email>' AND r.code='<role>' ON CONFLICT DO NOTHING;
> UPDATE users SET token_version=token_version+1 WHERE email='<email>';
> ```
> Then re-login to get a fresh token. (This is why comic tests 403'd at first — the owner was only `user`.)

## 3. Done this session (all committed on `main`, live-verified, NOT pushed)

| Commit | What |
|--------|------|
| `1e3d9e2` | **fix(stream):** jsonb payload `::jsonb` cast (journal `stream_items` + people `contact` were 500ing) + registered API-side event fan-out edges (bank/comic/media → stream/comic-reap were silently dropped) |
| `32695bf` | **feat(comic):** emit `comic:chapter_published` on publish → comics now appear in the life-stream (were invisible; service dropped its Events publisher) |
| `f1bf2a7` | **feat(comic/stream):** `comic:chapter_deleted` + `journal:stream_comic_deleted` consumer → deleting a chapter/comic removes its stream card |
| `95b5930` | **feat(ops):** queue console — `asynqmon` mounted at `/admin/queues` (admin-gated); 401/403/200 + SPA verified |

**Queue console:** open `https://api.portal.localhost/admin/queues` in a browser, log in as `admin@`.
(Note: 3 stale `transcode` tasks are sitting "active" — cruft from earlier image-pipeline tests with no real files; inspect/archive them there.)

## 4. Uncommitted state — DECIDE

- **`docs/testing/` (13 files, untracked)** — the full QA suite: TEST-PLAN, 9 TEST-CASES-SPEC-*,
  TRACEABILITY-MATRIX, TEST-RUN report, this handoff. **Decision:** commit the suite? (Strip
  creds from this handoff first, or add it to `.gitignore`.)
- `.claude/settings.local.json` (modified) — env-local, leave.
- Nothing pushed to a remote this session (all commits are local `main`).

## 5. Remaining P1 backlog (nothing P0 left)

| Feature | Spec | Effort | Live-verifiable? | Decision needed |
|---------|------|:------:|:----------------:|-----------------|
| **Takeout `/me/export`** ← recommended | SPEC-09 P1.7 | Large | ✅ yes | per-module export format (§6) |
| Web-push | SPEC-04 P1.1 | Med | partial | **VAPID keypair** (where stored) |
| SSE realtime bell | SPEC-04 P1.2 | Med | ✅ | Dragonfly pub/sub channel shape |
| Zip import (comic) | SPEC-02 P1.7 | Large | ✅ | presigned import/ prefix plumbing |
| `bank:budget_exceeded` | SPEC-03 P1.12 | Small | ⚠ emit-only (no consumer yet → invisible) | add a consumer to make it visible? |
| Activity-feed widget (home) | SPEC-06 P0.4 | Small | ⚠ frontend-only (needs Playwright/browser) | — |
| journal photo attachments (P1.5), people interactions (P1.6) / avatar (P1.7), comic reader modes/bookmarks | various | Med | mixed | — |

## 6. Recommended next: Takeout `/me/export` (SPEC-09 P1.7) — plan

**Goal:** `POST /me/export` → 202 `{export_id}`; worker `ops:takeout` fans out per module's
`ExportProvider` → one `portal-export-<yyyy-mm>.tar.gz` in storage; `GET /me/export/{id}` →
status + owner-scoped download; nightly `ops:purge_exports` deletes >7-day archives (410 `ops/export-expired`).

**Before coding, lock the per-module export format** (SPEC-09 P1.7 leaves this per-provider):
- account → profile + audit trail JSON
- journal → markdown files (bodies are already plain markdown)
- bank → CSV per account + categories/budgets CSV
- people → JSON
- media → originals + metadata JSON (⚠ bundles GPS/EXIF — inherits SPEC-01 P0.5 private-archive sensitivity)

**Steps (rough):**
1. Migration `ops_exports` table (§6 of SPEC-09 — status pending/running/ready/failed/expired, user_id, storage_key…). Next free number = **0018**.
2. `ExportProvider` interface in each module's `api/` (fan-out never touches another module's tables — providers only).
3. `ops:takeout` worker task: tar.gz assembly → `storage.Put`; `ops:purge_exports` nightly on the shared scheduler.
4. Handlers `POST /me/export` (takeout:write:own) + `GET /me/export/{id}` (takeout:read:own) — both seeded to `user` already (0012). Download via owner-scoped proxy or ≤5-min single-use key.
5. OpenAPI + `make openapi` (drift gate) + events.md (`ops:takeout`, `ops:purge_exports`, `ops:export_ready`).
6. Verify live: export → tar contains per-module files; expired → 410; cross-owner → 404.

> `POST /me/export` returns **404 today** (not built) — that's the concrete gap the test run flagged.

## 7. Gotchas & lessons (reusable — cost time this session)

- **Event fan-out contract (bit us twice):** `platform/events.Publish` looks up subscribers on the
  **local** publisher; an unregistered edge silently enqueues nothing. **Every emitting binary must
  register its own `Subscribe` edges** — cmd/api AND cmd/worker. When adding an event, wire the edge
  in both `backend/cmd/api/main.go` (~line 179+) and `backend/cmd/worker/main.go` (~line 205+),
  register the handler in the consuming module's `RegisterTasks`, and add to `docs/reference/events.md`.
- **jsonb + `COALESCE(narg,'{}')`:** a nil param → untyped NULL → COALESCE resolves to `text` → insert
  into a jsonb column fails. Cast **both** branches: `COALESCE(narg::jsonb,'{}'::jsonb)`. sqlc then types
  the param `[]byte`.
- **RBAC provisioning:** new features are role-gated; the "owner" needs `creator`+ (comics/media writes).
  Grant + bump `token_version` + re-login (§2).
- **Comic page create body:** handler wants `{"pages":[{asset_id,sort_order}]}`, **not** the bare array
  the spec documents (drift).
- **PowerShell/redirect corrupts generated files** — run `oapi-codegen` with a config `output:`, never `>`.
- **DB password:** the postgres volume's `portal` password must match `.env` `change-me`; if the api
  crash-loops on SASL auth, `ALTER USER portal WITH PASSWORD 'change-me';` (§ earlier fix).

## 8. Test-run findings still open (from TEST-RUN-2026-07-12.md §4)

- **F-1 (S3):** comic publish is `POST /comics/{id}/publish` (works, returns 422 not-publishable),
  but SPEC-02 §7 documents `PATCH {status}` — which silently no-ops. Reconcile spec↔handler.
- **F-2 (S4):** bank fractional amount → 400 (JSON decode, `amount` is int64) instead of 422
  `bank/invalid-amount`. Cosmetic.
- Comic page-create body drift (§7). No GET single `/bank/accounts/{id}`; budget PUT returns 204.

## 9. Key file pointers

- Requirement baseline: `docs/product/specs/SPEC-0N-*.md`
- Live status tracker: `docs/product/sprint-checklists.md` (updated this session) + `MILESTONE_CHECKS.md`
- QA suite: `docs/testing/` (plan, cases, matrix, run report)
- Event registry: `docs/reference/events.md`
- Module boundaries: `backend/MODULES.md`
- Fan-out wiring: `backend/cmd/{api,worker}/main.go`; `backend/internal/platform/events/events.go`

## 10. First move tomorrow

1. `make up` (if down) → verify healthz + migration v17.
2. Decide: commit `docs/testing/`? (strip creds from this file first.)
3. If continuing takeout: lock §6 formats → migration 0018 → providers → task → handlers → verify.
