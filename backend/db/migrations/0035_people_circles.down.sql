DROP INDEX IF EXISTS people_persons_linked_user_idx;
DROP INDEX IF EXISTS people_persons_circle_idx;
ALTER TABLE people_persons DROP COLUMN IF EXISTS linked_user_id;
ALTER TABLE people_persons DROP COLUMN IF EXISTS circle;
