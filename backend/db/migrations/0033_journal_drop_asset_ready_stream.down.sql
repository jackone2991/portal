-- Deliberate no-op. The rows 0033 deleted carried an occurred_at stamped at
-- projection time, which no longer exists anywhere; rebuilding them from
-- assets.created_at would put cards in the feed at times they never happened.
-- Re-enabling the projection is a code change (cmd/worker's subscription list),
-- not a data restore, and it only ever needs to apply going forward.
SELECT 1;
