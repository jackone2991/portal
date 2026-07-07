# 04 — Deferred / Parking Lot

Consciously set aside in the 2026-07-07 brainstorm. Each item lists **why** and its
**re-entry condition** — the trigger that puts it back on the table. This file exists
to prevent re-litigating these decisions every session.

| Item | Why deferred | Re-entry condition |
|------|--------------|--------------------|
| **Statement import** (bank) | TCB exports **PDF** → decent import means PDF parsing/OCR, an effort black hole for v1. Schema is already import-ready (spec 03 P0.2/P0.9), so nothing is lost by waiting. | User can obtain CSV/xlsx from any bank they use, **or** we accept building the generic CSV + column-mapping path first and PDF later. Design is pre-agreed: mapping templates as data-not-code, `dedup_hash`, batch rollback. |
| **TOTP / MFA / step-up** (D-27/D-28) | Was gating "bank"; the ledger scope holds no bank credentials, so the gate doesn't apply. | The moment real bank credentials/API sync or any money-*moving* feature appears. TOTP is then the named unlock task, not a floating P2. |
| **Notifications module** (`notify:*`) | Its old P1 rationale (password reset) assumed a multi-user product. Still the **life-stream backbone** — deferral is short. | Immediately after specs 01–03 land: consume `media:asset_ready`, `bank:transaction_created`, `comic:chapter_published` into an in-app stream. Email/web-push can trail. |
| **Password reset, email verification** | No real second users; accounts are seeded admin/CLI. | First real external user, or notifications module lands (whichever first). |
| **Friend graph / messenger / people search** | Feature-parity trap at n=1 users; a life OS starts from one user. UI shell stays as-is. | Real second users on an instance (e.g. family). Re-enter via "share to household member", not full FB parity. |
| **Movie vertical** | Video playback already works on `/upload`; catalog adds less than unlocking two new domains (comic, finance). | After spec 02 proves the vertical pattern — movie becomes a copy-the-pattern task. |
| **Music / Story verticals** | Same pattern, lower stated priority than comic. | Post-movie, or when the user asks. |
| **Multi-rendition HLS, playback ACL, presigned direct upload** | Untouched by this cycle; single user on LAN/VPS tolerates single rendition + public-ish HLS short-term. | Playback ACL re-enters **with any second user**; renditions when mobile/remote playback stutters. |
| **Time domain (calendar/tasks)** | Was the cheapest first life domain; user chose money + entertainment first. | After spec 03 — likely the next life facet, wiring the already-built calendar/birthday widgets. |
| **Debts / loans / investments** (feature.md §8.3–8.5) | Ledger core first; these are separate iterations with their own models. | Ledger reconciles cleanly for ≥1 month (success signal in spec 03). |
| **Multi-tenancy + RLS, bank-real, marketplace, creator economy, observability, LiveKit** | Unchanged from [ADR-01](architecture/01-v1-scope-cut.md). | Per ADR-01; ADR-08 does not touch these. |
