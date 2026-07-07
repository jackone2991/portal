# Architecture Review — Portal (May 2026)

**Status:** Accepted
**Date:** 2026-05-24
**Reviewers:** kirito (solo dev / sole reviewer)
**Scope:** Whole stack — every area on the form (auth, tenant/RLS, media, domain modules, storage/CDN, jobs, DB, frontend, OpenAPI, cross-cutting).

> **Update (2026-07-06):** This review's action items were all executed — ADRs 01–05 are Accepted and implemented, and the v1 demo loop is closed (see `MILESTONE_CHECKS.md`). One §2 endorsement was later reversed: [ADR-06](./06-local-auth-model.md) (2026-07-05) removed Authentik/OIDC in favour of Portal-owned local password auth (Argon2id); the refresh-token and revocation machinery is unchanged. The body below is preserved as written on 2026-05-24, with dated notes marking superseded findings.

This document is the entry point for the ADR set. It captures the **findings** of the review; the individual ADRs that follow propose the corrective actions.

---

## 1. Executive summary

The design corpus (CLAUDE.md + MODULES.md + feature.md + diagrams.md) is **unusually mature** for a project at this code stage. 40 decisions are logged with rationale; module boundaries are written down and depguard-enforceable; the OpenAPI contract is named as the source of truth; the auth module is genuinely well-thought-through (refresh-token reuse detection, fail-closed permission matcher, two-channel revocation).

That same maturity is also the project's biggest risk. The scope described in `feature.md` is **multi-year work for a small team**, not 2-week work for one person. Most of the load-bearing technical decisions are correct in the abstract but become liabilities at the stated team-size and timeline:

- A modular monolith with depguard-enforced boundaries is right for a 4-person team and wrong for a solo developer trying to ship in 2 weeks — boundaries discovered during exploration shouldn't be enforced in CI until at least one binary is running.
- The media pipeline (FFmpeg + Asynq + MinIO + R2 + transcode quotas) is right for a multi-tenant SaaS and over-engineered for a v1 that needs *one* working upload to prove the architecture.
- The two competing RBAC specs (role-hierarchy in CLAUDE.md/feature.md, policy-bundles in archivetech.md) cannot both be true. Code already exists for one of them.
- The deployment-target diagrams.md draws (Postgres + PgBouncer + Dragonfly + MinIO + Authentik + 5-service observability stack + mediamtx + LiveKit + Mailpit) does not fit on a single $30–60 VPS once you account for Authentik's RAM footprint and FFmpeg's bursty CPU.

The single most useful thing this review can do is **make the v1 cut explicit** so the next 2 weeks aren't spent reading 40 decisions in linear order. See [ADR-01](./01-v1-scope-cut.md).

---

## 2. What's load-bearing and correct

These choices should survive any v1 cut. Don't second-guess them mid-sprint:

| Decision | Why it stands |
| --- | --- |
| **Go modular monolith with `cmd/{api,worker,sysjobs}`** | Right answer for the team size, deployment target, and feature breadth. Microservices at 1 dev / 1 VPS is malpractice; a flat Go binary would make the eventual social/bank/media split painful. |
| **OpenAPI as source of truth** (`shared/openapi.yaml` → oapi-codegen + openapi-typescript) | Spec-first is the only thing keeping the Go ↔ TS contract honest under solo development. Skipping this leaks types and breaks the frontend later. [D-29] |
| **Postgres 17 + RLS for multi-tenancy** | RLS is doing real work as the last line of defence behind handler-level filtering. The recursive-CTE permission walk is correctly designed. |
| **Asynq for the job queue** | Right pick — Go-native, Redis/Dragonfly-compatible, ships with dead-letter queues. BullMQ would force a Node worker; building one is a worse use of the 2 weeks. |
| **Dragonfly instead of Redis** | Same protocol, lower RAM, single-process. No reason not to. |
| **Authentik for OIDC** | Adds RAM, but the alternative is hand-rolling password storage + reset flow + email loops — that's 3 days you don't have. Authentik + OIDC + Portal-managed refresh tokens is the right division of labour. |
| **Vidstack for HLS playback** | Vidstack on Next.js is the path of least resistance; the alternative (Shaka, hls.js direct) is more wiring. |
| **`cmd/sysjobs` separation + sysrepository BYPASSRLS lock-down** | This is one decision you should NOT defer even at solo-dev speed. RLS bypass is the kind of thing that turns into a multi-tenant data leak. The depguard rule pays for itself the first time you forget. |

> **Update (2026-07-06):** the Authentik row above was superseded by [ADR-06](./06-local-auth-model.md) — Authentik was dropped and Portal now owns local password auth (Argon2id). The refresh-token and revocation machinery it endorsed is unchanged and still in use.

## 3. What's at risk

These are the choices most likely to cost time or burn the budget under the stated constraints:

### 3.1 The RBAC schism (highest priority)

`archivetech.md` §1 says "the spec wins, adjust code, not the other way around" and then describes a **completely different RBAC model** from the one in CLAUDE.md and `feature.md`:

| Aspect | CLAUDE.md / feature.md (built) | archivetech.md (specced) |
| --- | --- | --- |
| Primary grant unit | Roles (with hierarchy) | Policies (reusable permission bundles) |
| Hierarchy | `guest → user → creator → editor → moderator → admin → superadmin` via `roles.parent_id` | `User Group` parent chain via `user_groups.parent_id` |
| Per-user grants | `user_roles` table | `user_policy_attachments` table |
| Permission gating | None (just RBAC matching) | **File-gated permissions** — uploaded license required to activate the grant |
| Conflict resolution | First-match wins on the grant set | Deny-wins (AWS/OPA semantics) |
| Permission cache key | `rbac:perms:<userID>:v<N>` | Same shape, but per `(user_id, token_version)` |

Both specs can't coexist. Code currently implements the role-hierarchy model. `archivetech.md`'s spec-wins clause is unenforceable until someone decides which spec is canonical. Decision deferred to [ADR-02](./02-rbac-model-reconciliation.md); recommendation there is to **keep role-hierarchy as the v1 grant primitive** and add policy-bundles as a Phase-2 layer **on top of** roles, not instead of them.

> **Update (2026-07-06):** resolved — [ADR-02](./02-rbac-model-reconciliation.md) is Accepted; role-hierarchy is canonical for v1 and policy bundles layer on top later. `archivetech.md`'s spec-wins clause is disregarded for v1.

### 3.2 Scope vs. runway mismatch

`feature.md` Phase 0 alone has 14 deliverables and would take 1 dev ~1 week if everything goes well. Phases 1–12 are years of work. The 2-week budget means **picking a single phase set** and committing. See [ADR-01](./01-v1-scope-cut.md); the recommended cut is *Phase 0 + a vertical slice of Phase 2 (one video upload happy path)* and nothing else.

### 3.3 RAM budget on a single VPS

If you bring up `docker-compose.yml` plus add **Authentik** (~1 GB), the **observability profile** (~1.1 GB across Loki/Prometheus/Tempo/Grafana/GlitchTip), **mediamtx + LiveKit** (~500 MB + bursty), and FFmpeg worker (1–2 GB during transcode), you exceed 4 GB before the API has handled a request. A reasonable $60/mo VPS (8 vCPU / 32 GB) covers it; a $30/mo VPS (4 vCPU / 16 GB) does not once Authentik is in the mix.

[ADR-03](./03-single-vps-topology.md) proposes the v1 service set and which profile flags stay off.

> **Update (2026-07-06):** Authentik was removed by [ADR-06](./06-local-auth-model.md), taking its RAM footprint out of the equation; the shipped v1 stack is 8 services (postgres, pgbouncer, dragonfly, minio, traefik, api, worker, frontend).

### 3.4 Storage tier complexity

`docker-compose.yml` runs MinIO inside the VPS. The diagrams imply MinIO is the *origin* and R2 is the *edge*, with continuous replication. That's two storage systems, two sets of credentials, replication monitoring, and 100–500 GB of VPS-attached disk before any user uploads.

For v1 the simpler answer is **R2 only** — it's S3-compatible, costs ~$0.015/GB-month for storage and has zero egress fees inside Cloudflare's edge. MinIO becomes a Phase-2 addition if a self-hoster wants to run entirely without Cloudflare. See [ADR-04](./04-storage-tier-budget.md).

> **Update (2026-07-06):** adopted with a refinement — see ADR-04's 2026-06-06 update. R2-only holds for deployed environments; local dev uses MinIO behind the same single S3 client.

### 3.5 Wiring gap is the actual blocker

CLAUDE.md says it out loud: *"`cmd/api/main.go` still has a `TODO: mount OpenAPI-generated handlers` comment and does not yet call `account.New(...)` or any module's `MountHTTP`."* Every downstream feature is gated on this. [ADR-05](./05-phase0-wiring-order.md) sequences the closure.

> **Update (2026-07-06):** closed — ADR-05 was executed in order; `cmd/api/main.go` now constructs the account and media modules and `/api/v1/healthz` returns 200. See `MILESTONE_CHECKS.md`.

## 4. What's quietly fine

These choices got long writeups in the corpus but don't need ADRs — they're already the right answer and nothing in the constraint envelope changes that:

- **Tenant via URL prefix `/t/{tenant}/...` + synthetic `me` tenant** [D-23] — pragmatic, RLS-friendly, link-shareable.
- **RFC 7807 Problem responses with i18n-key `type` URIs** [D-7] — solves the i18n problem at source.
- **Money as `numeric(20,8)` + shopspring/decimal + Money value type** [D-14] — bank is far enough out that this is theory for now, but the theory is right.
- **Forward-only production migrations + expand-migrate-data-contract pattern** [D-12] — required discipline once data exists; cheap to commit to now.
- **`platform/audit/` cross-cutting + `<module>.<resource>.<action>` event taxonomy** [D-25] — best-effort non-blocking is the right tradeoff.

## 5. What should be deferred outright

These are explicitly **scope cuts** for the 2-week window. Each is a complete feature that would deserve its own ADR if attempted; the recommendation is to NOT attempt them in v1. The references in parentheses are to `feature.md` sections.

| Feature area | Defer because |
| --- | --- |
| Bank module (§8, Phase 5a-i) | 9 sub-phases. Needs step-up auth, MFA enforcement, double-entry ledger, FX rates, household sharing. Months of work. |
| Social module (§9 baseline, Phase 7) | Posts + feeds + reactions + DM + communities + privacy controls is a quarter of work even before §9.12+ advanced features. |
| Advanced social (§9.13–9.37, Phase 10) | Reels, live streaming, audio rooms, voting, karma, wikis — each is a side project. |
| Creator economy (§10, Phase 11) | Tips + subs + paywalls + payouts depend on bank shipping first. |
| Marketplace (§11, Phase 12) | Bridge module spanning social + bank; both must ship first. |
| ML safety (§12, Phase 12) | NSFW + CSAM + toxicity classifiers require model infra. Manual report queue is the v1 answer. |
| LiveKit + mediamtx (§9.25, Phase 10/12) | ~500 MB + bursty CPU + complex networking. Out for v1. Compose profile `--calls` and `--profile live` already gate them; keep the profiles disabled. |
| Observability stack (Phase 1, D-8) | Loki + Prometheus + Tempo + Grafana + GlitchTip is 5 services. Run with stdout JSON logs in v1; add the stack when traffic justifies it. Keep `--profile observability` disabled. |

[ADR-01](./01-v1-scope-cut.md) restates the cut formally.

## 6. Findings the corpus doesn't address

A few things weren't in the input docs and should be on the radar:

- **No mention of database connection-pool tuning** between PgBouncer transaction-pool mode and `pgx`. Asynq, the API, and the worker share a Postgres cluster; without careful pool sizing, the worker starves the API during a transcode burst.
- **`OIDC_GROUP_ROLE_MAP` is env-configured** [D-26]. A typo in production silently strips an admin of their privileges on next refresh. Add a startup-time validation that flags unmapped Authentik groups present in the bootstrap admin list. *(Update 2026-07-06: obsolete — ADR-06 removed OIDC/Authentik; no group-role mapping exists.)*
- **Refresh-token rotation chain emits a chain-revoke event on reuse**, but the corpus doesn't define what *consumes* that event. At minimum, the chain-revoke should send a security email to the user; the notification module is Phase 6, so for v1 either log loudly or send a hand-rolled email from the auth handler.
- **`disabled_at` on users is checked in middleware**, but there's no scheduled job to revoke active refresh tokens when a user is disabled. A disabled user's existing refresh tokens still rotate until expiry. Worth a one-line `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = ?` next to the disable handler.
- **Frontend SameSite=Strict cookies require Next.js and the API to share a registrable domain** [D-34]. Single-domain path-based routing via Traefik works, but it's a non-obvious deployment constraint that needs to be in the operator docs from day one — easy to misconfigure in development with `localhost:3000` + `localhost:8080` (those are *different* origins for SameSite purposes).

## 7. Recommendations summary

> **Update (2026-07-06):** all five recommendations were executed; the v1 demo loop (local sign-in → upload → HLS playback → revocable logout) is closed and committed. ADR-06 subsequently replaced the auth model. `MILESTONE_CHECKS.md` is the living tracker.

Five concrete ADRs follow. In sprint priority order:

1. **[ADR-05](./05-phase0-wiring-order.md)** — close the wiring gap (Day 1–3). Nothing else matters until `make up && make dev && curl /api/v1/healthz` returns 200 from a *constructed* module.
2. **[ADR-01](./01-v1-scope-cut.md)** — agree on the v1 cut so you don't reflexively reach for the bank module in week 2.
3. **[ADR-02](./02-rbac-model-reconciliation.md)** — pick one RBAC model and write it down before any admin UI ships.
4. **[ADR-03](./03-single-vps-topology.md)** — lock in the compose profile set and the VPS sizing so the deploy script doesn't ship with `--profile observability` enabled by accident.
5. **[ADR-04](./04-storage-tier-budget.md)** — decide MinIO+R2 vs R2-only before the upload handler is written; the call shapes the storage interface.

See the system landscape diagram at [`diagrams/system-landscape.md`](./diagrams/system-landscape.md) for the v1-scoped picture.
