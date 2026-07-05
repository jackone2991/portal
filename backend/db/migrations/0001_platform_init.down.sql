-- Extensions are cluster-wide; safe to drop here since all dependent tables are
-- removed by higher-numbered down migrations before this one runs.
DROP EXTENSION IF EXISTS "pg_trgm";
DROP EXTENSION IF EXISTS "unaccent";
DROP EXTENSION IF EXISTS "uuid-ossp";
