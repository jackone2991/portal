# ADR-02: Reconcile RBAC — role hierarchy (built) vs policy bundles (specced)

**Status:** Proposed
**Date:** 2026-05-24
**Deciders:** kirito
**Supersedes:** N/A (first decision in this area)
**Affects:** [feature.md D-26], [archivetech.md §2, §3.3]

## Context

The project has **two specs for access control that contradict each other**, and code exists for one of them.

### Spec A — Role hierarchy (CLAUDE.md + feature.md + actual code)

- Permission grammar: `<resource>:<action>[:<scope>]` with `*` / `:any` / `:own` wildcards.
- Grants flow through **roles**. `roles.parent_id` forms an adjacency-list hierarchy: `guest → user → creator → editor → moderator → admin → superadmin`.
- Effective permission set = walk role ancestors via recursive CTE, union with directly-assigned `user_roles`, union with `user_oidc_roles` (synced from Authentik groups). [D-26]
- Implemented in `backend/internal/modules/account/rbac/`.
- Two-channel revocation: `users.token_version` (instant logout-all) + `refresh_tokens.revoked_at` (chain revoke).
- Cache key `rbac:perms:<userID>:v<N>` namespaced by `token_version`.

### Spec B — Policy bundles (archivetech.md)

- Same permission grammar at the leaf.
- Grants flow through **policies** (reusable named bundles like "Radiologist", "Read-Only Auditor"). Policies attach to **user groups** or directly to **users**.
- User groups form their own hierarchy (`user_groups.parent_id`); a user inherits every active policy attached to any ancestor group plus their own per-user policies.
- **File-gated permissions**: certain permissions inside a policy require an uploaded file (license, certificate) to be effective. Admin review queue. File expiry → permission silently disappears.
- Conflict resolution: **deny-wins** (AWS IAM / OPA semantics).
- archivetech.md §1 declares "the spec wins, adjust code, not the other way around" — meaning if this is accepted, the existing role-hierarchy code is wrong.

### Why this is a real conflict

These are not two views of one model. They're two different models with different primary entities (roles vs policies), different group concepts (the system roles in spec A are not the same thing as user groups in spec B), different cache invalidation flows (file-gating in B has no analogue in A), and different audit semantics (spec B logs "permission became ineffective" events that spec A has no concept of).

You cannot ship both unmodified. You can ship one, ship both layered, or ship one now and migrate later. The cost differs.

## Decision

**For v1, keep role hierarchy (Spec A) as the grant primitive. Reframe policy bundles (Spec B) as a *layer on top of* roles, deferred to a future phase. File-gated permissions stay as a Phase-3+ feature (per archivetech.md's own phasing), not v1.**

Concretely:

1. **Spec A is canonical for v1.** The existing role-hierarchy code stays. `users.token_version` remains the revocation channel. The recursive-CTE effective-permission walk is unchanged.
2. **Spec B's User Groups become a future module**, not a renaming of `roles`. Call it `usergroup` (or fold it into a future `organization`/`tenant` module — see [D-24]) so the two hierarchies don't collide in vocabulary.
3. **Spec B's Policies layer on top.** When the policy module ships, a policy expands into a set of `(role | permission)` grants; the effective-permission walk gains a "policies attached to this user/group" step *before* the role union.
4. **File-gating becomes a permission-effectivity filter** at the end of the resolution chain. The existing matcher stays grant-only; effectivity filtering is a separate stage that prunes permissions whose required-file row is missing/expired/rejected.
5. **Deny-wins precedence is reserved.** Spec A is grant-only and doesn't currently support deny rules. Reserve the contract — when explicit deny lands, the order is "any deny path wins". archivetech.md §2.3 already commits to this contract; record it now even though no code implements deny.

For v1 itself, none of the policy/group machinery exists. The 2-week sprint ships only the role-hierarchy auth that already works. This ADR's purpose is to **prevent the two specs from being implemented simultaneously and incompatibly**, and to keep the path open for adding policies on top later.

## Options considered

### Option A — Migrate to Spec B; rewrite the role module

| Dimension | Assessment |
| --- | --- |
| Complexity | High — rewrites every auth-touching test, migrations 0002+, RBAC engine, middleware |
| Cost | 4–7 days of solo-dev time before v1 ships |
| Scalability | Spec B is arguably the more flexible long-run model |
| Team familiarity | New ground; spec B's deny-wins is unfamiliar |

**Pros:** archivetech.md §1's "spec wins" clause is honoured. File-gated permissions are first-class.
**Cons:** Burns 30–50% of the v1 sprint on a rewrite. Throws away working code with test coverage. The first thing the v1 demo proves is the auth flow — destabilising it in week 1 destabilises everything.

### Option B — Keep Spec A; layer Spec B on top in a future phase  *(chosen)*

| Dimension | Assessment |
| --- | --- |
| Complexity | Low for v1; medium for the layering work later |
| Cost | 0 days now; ~1 week when policies are added |
| Scalability | Best-of-both — roles for coarse access, policies for organisational fine-grain |
| Team familiarity | Existing code stays; no auth churn |

**Pros:** v1 ships with auth that already works. Layering policies on top of roles is a well-trodden pattern (AWS IAM has both); the effective-permission walk just gains an extra union step. File-gating fits cleanly as a final-stage filter.
**Cons:** Two concept hierarchies for grant management (roles + policies + user groups). Operators have to learn both. The "spec wins" promise in archivetech.md is *softened*, not honoured — explicit ADR needed to record the change of intent.

### Option C — Hybrid now: keep roles, add policies in v1

| Dimension | Assessment |
| --- | --- |
| Complexity | Medium-high — two new tables, new resolution code, new admin UI |
| Cost | 3–5 days of v1 sprint |
| Scalability | Same as Option B long-run |
| Team familiarity | Mixed |

**Pros:** Avoids the future "we said we'd add policies" debt.
**Cons:** Crowds v1 with non-demo features. Policy admin UI isn't demoable in the 7-step happy path. Pure scope creep against [ADR-01](./01-v1-scope-cut.md).

## Trade-off analysis

Option A's strongest argument is the "spec wins" clause; its weakest is that the spec it's honouring (archivetech.md) is itself a 6-screen sketch from `template-main/portal/document/anh{1,2,3}.png` with no code behind it. The spec hasn't earned the right to override working code.

Option C's strongest argument is "do it right the first time"; its weakest is that "right" here means "policies + roles + groups + file-gating + review queue" — five concepts piled into a sprint that already has 8 deliverables. The auth surface gets brittle exactly when the demo needs it stable.

Option B's strongest argument is sequencing — get v1 demonstrable, then add the organisational features when there's a real operator asking for them. Its weakest is the conceptual cost: anyone reading both specs has to mentally compose roles + policies + groups + file-gating into one model. This ADR's job is to make that composition explicit so future-you doesn't reverse-engineer it.

The composition rule, written down once for clarity:

```
effective_permissions(user, tenant):
    1. roles_user_holds = recursive_walk(user_roles ∪ user_oidc_roles)
    2. policies_user_holds = ∪{
           policies_attached(user),
           policies_attached(group) for group in walk(user_groups(user))
       }
    3. grants = ∪{
           permissions(role) for role in roles_user_holds,
           permissions(policy) for policy in policies_user_holds
       }
    4. effective = filter(grants, where file_gate_satisfied(grant, user))
    5. if any deny grant in policies_user_holds matches the required code:
           return DENY
    6. return effective
```

Steps 1, 3, and 5 (without deny) are what currently exists. Steps 2 and 4 are the additions when Spec B layers on. Step 5's deny path is reserved.

## Consequences

**What becomes easier:**

- v1 ships on time. The 7-step demo is unaffected by this decision.
- Existing tests in `backend/internal/modules/account/rbac/permission_test.go` stay green.
- When policies are added, the existing code is unchanged — the new code is purely additive (new tables, new resolution stage).

**What becomes harder:**

- archivetech.md needs a header note (or supersedence ADR) saying its RBAC section is **layered on top of**, not **replacement for**, the role hierarchy. Without it, the next reader sees contradiction.
- Future contributors will see two grant concepts and need this ADR to know how they compose. Make sure the composition rule above is also reflected in `backend/internal/modules/account/README.md` once it's written.
- The "deny-wins" promise commits us to a particular semantics. If a future explicit-deny implementation forgets that promise, hard-to-debug security regressions become possible.

**What we'll need to revisit:**

- When the admin UI from `anh1/2/3.png` is built, the screens are for **policies + groups**, not roles. The UI work pulls Spec B's tables forward. Plan a Policy/Group sprint when those mocks reach the top of the backlog (post-v1, likely Phase 1.5 or Phase 7's social-page-role work).
- File-gated permissions require object storage for the uploaded licenses + an admin review queue + cron-based expiry checks. These are independent enough to ship as their own phase (Phase 3 in archivetech.md), and gating that phase on the policy layer existing is the right ordering.
- The role-hierarchy adjacency list has a CHECK preventing self-cycles only; deeper cycles are prevented at the app layer. When policies and groups land, the same self-only DB CHECK will be insufficient — make sure the policy/group migration includes proper cycle prevention (or the app-layer check is hardened with explicit tests).

## Action items

1. [ ] Add a 5-line note to the top of `archivetech.md` referencing this ADR and stating that its RBAC model is the *deferred Phase 1.5+ layer*, not the v1 model. **No** silent contradiction.
2. [ ] Add the composition rule (the 6-line pseudocode above) to `backend/internal/modules/account/README.md` (or a stub if the README doesn't exist) so the relationship is visible from the code.
3. [ ] Reserve the depguard rule for `internal/modules/policy/` and `internal/modules/usergroup/` so when those modules land, depguard already knows they exist (avoids "module not in allowlist" churn).
4. [ ] Open a tracking issue "RBAC Phase 1.5: policy bundles + user groups (ADR-02 layered model)" so the deferred work is visible without being scheduled.
5. [ ] After v1 ships, before any admin UI work begins, schedule the Policy/Group sprint. The composition rule from this ADR is the contract that sprint implements.
