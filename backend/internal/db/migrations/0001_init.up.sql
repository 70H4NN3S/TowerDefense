-- Enable the case-insensitive text extension required for email and username
-- uniqueness checks (used in Phase 2 users table).
CREATE EXTENSION IF NOT EXISTS citext;
