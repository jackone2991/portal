-- 0016_people_persons: the people registry (SPEC-08) — an owner-scoped address
-- book. user_id FK into users(id) (identity-anchor exception). Birthday is
-- month/day with optional year; birth_calendar ships now (lunar math is P2).
-- people_birthday_notices is the outbox/dedup for the daily scan (P0.4).

CREATE TABLE people_persons (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    display_name    TEXT NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 120),
    relationship    TEXT,                       -- 'mẹ', 'bạn đại học' — freeform
    birth_month     INT  CHECK (birth_month BETWEEN 1 AND 12),
    birth_day       INT  CHECK (birth_day BETWEEN 1 AND 31),
    birth_year      INT,
    birth_calendar  TEXT NOT NULL DEFAULT 'solar' CHECK (birth_calendar IN ('solar', 'lunar')),
    contact         JSONB NOT NULL DEFAULT '{}', -- phones/emails/addresses; convention, not schema
    note_md         TEXT,
    avatar_asset_id UUID,                        -- media asset (validated via mediaapi; P1.7)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((birth_month IS NULL) = (birth_day IS NULL)),
    CHECK (birth_year IS NULL OR birth_month IS NOT NULL)
);
CREATE INDEX people_persons_user_idx ON people_persons (user_id, display_name, id);
CREATE INDEX people_persons_user_bday_idx ON people_persons (user_id, birth_month, birth_day);

-- P0.4 outbox/dedup: one emit per (person, occurrence-year, threshold). `id` is
-- the event's notice_id (SPEC-06 keys stream items on it). emitted_at NULL =
-- written but not yet published; the next scan retries it (at-least-once).
CREATE TABLE people_birthday_notices (
    id         UUID NOT NULL DEFAULT uuid_generate_v4() UNIQUE,
    person_id  UUID NOT NULL REFERENCES people_persons(id) ON DELETE CASCADE,
    year       INT  NOT NULL,
    threshold  INT  NOT NULL,               -- 3 | 0
    emitted_at TIMESTAMPTZ,
    PRIMARY KEY (person_id, year, threshold)
);

-- ── seed: people permission catalog + grants to base `user` (P0.2 / §7) ──
INSERT INTO permissions (code, description) VALUES
    ('people:read:own',   'Read own people'),
    ('people:write:own',  'Create and edit own people'),
    ('people:delete:own', 'Delete own people')
ON CONFLICT (code) DO NOTHING;

WITH grants(role_code, perm_code) AS (VALUES
    ('user', 'people:read:own'),
    ('user', 'people:write:own'),
    ('user', 'people:delete:own')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM grants g
JOIN roles r       ON r.code = g.role_code
JOIN permissions p ON p.code = g.perm_code
ON CONFLICT DO NOTHING;
