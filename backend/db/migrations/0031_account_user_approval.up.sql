-- 0031_account_user_approval: registration requires an approver.
-- Owning module: account.
--
-- Registration used to hand out a usable account immediately. It now creates a
-- PENDING one: the row exists and the password works, but login is refused and
-- every already-issued token stops verifying until someone with `users:approve`
-- moves it to `approved`.
--
-- Three states, no more: pending → approved | rejected. A rejected account is
-- kept rather than deleted so the same email cannot be re-registered to slip
-- past a refusal, and so the decision stays auditable.

ALTER TABLE users
    ADD COLUMN approval_status TEXT        NOT NULL DEFAULT 'pending',
    ADD COLUMN approval_note   TEXT,                 -- reviewer's reason, shown to the user on a refusal
    ADD COLUMN approved_at     TIMESTAMPTZ,
    ADD COLUMN approved_by     UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE users ADD CONSTRAINT users_approval_status_check
    CHECK (approval_status IN ('pending', 'approved', 'rejected'));

-- Everyone who already exists keeps working. The DEFAULT above applies to rows
-- inserted from here on; applying it retroactively would lock every current
-- user out of a running system, which is the opposite of the intent.
UPDATE users
SET approval_status = 'approved',
    approved_at     = created_at
WHERE approval_status = 'pending';

-- Partial index: the admin queue only ever asks for the non-approved rows, and
-- in a healthy install that is a handful out of the whole table.
CREATE INDEX users_approval_pending_idx ON users (created_at DESC, id DESC)
    WHERE approval_status <> 'approved';

-- ── the approval permission ───────────────────────────────────────────────
--
-- Deliberately granted to NO role. `superadmin` holds the literal '*' wildcard
-- (0003), so it is the only role that can approve out of the box — which is the
-- requirement. `admin` is left without it on purpose: an admin can already edit
-- users and assign roles, and letting the same role wave accounts in would make
-- the approval gate self-serve. Grant it to another role from the permission
-- matrix screen if that is what a deployment wants.
INSERT INTO permissions (code, description) VALUES
    ('users:approve', 'Approve or reject a pending registration')
ON CONFLICT (code) DO NOTHING;
