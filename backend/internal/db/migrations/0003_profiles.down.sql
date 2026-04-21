DROP TABLE profiles;

ALTER TABLE users ADD COLUMN trophies bigint NOT NULL DEFAULT 0 CHECK (trophies >= 0);
