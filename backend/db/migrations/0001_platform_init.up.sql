-- 0001_platform_init: database extensions + shared helpers.
-- Owning module: platform. No business tables live here — account tables start
-- in 0002. Split out per ADR-05 so sqlc reads a clean, per-module migration tree.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "unaccent";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
