-- Deliberate no-op, as in 0033: the deleted rows carried an occurred_at stamped
-- at projection time, and rebuilding them from the chapters table would place
-- cards at times that never happened. Re-enabling the projection would be a code
-- change, not a data restore.
SELECT 1;
