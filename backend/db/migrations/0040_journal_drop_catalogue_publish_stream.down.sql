-- Deliberate no-op, as in 0033 and 0034: the deleted rows carried an occurred_at
-- stamped at projection time, and rebuilding them from the catalogue tables
-- would place cards at times that never happened. Re-enabling the projection is
-- a code change, not a data restore.
SELECT 1;
