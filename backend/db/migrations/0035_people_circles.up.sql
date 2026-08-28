-- 0035_people_circles: give the people registry two fixed circles, and let an
-- entry point at a portal account.
--
-- `relationship` stays what it is — free text you wrote ("mẹ", "bạn đại học") —
-- because that is the useful thing to record about a person. But free text can't
-- drive fixed UI sections: the right rail needs to know "is this person a close
-- friend / family / neither" without guessing at Vietnamese kinship words. So
-- circle is a small closed set beside the free text, not a replacement for it.
--
-- Existing rows all land in 'other'. No heuristic backfill from relationship: a
-- wrong guess ("bạn thân cũ" → close friend?) is worse than an unsorted list the
-- owner can sort in one pass.
--
-- linked_user_id ties a registry entry to a portal account, so "people you may
-- know" can stop suggesting someone you already added. Nullable and NOT the
-- common case: most people in a personal registry (your mother, a school friend)
-- will never have an account here.

ALTER TABLE people_persons
    ADD COLUMN circle TEXT NOT NULL DEFAULT 'other'
        CHECK (circle IN ('close_friend', 'family', 'other')),
    ADD COLUMN linked_user_id UUID REFERENCES users(id) ON DELETE SET NULL;

-- The rail reads one circle at a time, per owner.
CREATE INDEX people_persons_circle_idx ON people_persons (user_id, circle, display_name);

-- One registry entry per portal account per owner: adding the same suggestion
-- twice is a no-op at the database, not a duplicate row to clean up later.
CREATE UNIQUE INDEX people_persons_linked_user_idx
    ON people_persons (user_id, linked_user_id)
    WHERE linked_user_id IS NOT NULL;
