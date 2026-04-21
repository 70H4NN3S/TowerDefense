-- Drop the trophies column from users; trophies are game-state and belong in
-- profiles alongside the rest of the player's resource data.
ALTER TABLE users DROP COLUMN trophies;

CREATE TABLE profiles (
    user_id           uuid        NOT NULL PRIMARY KEY,
    display_name      text        NOT NULL DEFAULT '',
    avatar_id         int         NOT NULL DEFAULT 0,
    trophies          bigint      NOT NULL DEFAULT 0,
    gold              bigint      NOT NULL DEFAULT 0,
    diamonds          bigint      NOT NULL DEFAULT 0,
    energy            int         NOT NULL DEFAULT 5,
    energy_updated_at timestamptz NOT NULL DEFAULT now(),
    xp                bigint      NOT NULL DEFAULT 0,
    level             int         NOT NULL DEFAULT 1,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT fk_profiles_users            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT ck_profiles_trophies_nonneg  CHECK (trophies  >= 0),
    CONSTRAINT ck_profiles_gold_nonneg      CHECK (gold      >= 0),
    CONSTRAINT ck_profiles_diamonds_nonneg  CHECK (diamonds  >= 0),
    CONSTRAINT ck_profiles_energy_nonneg    CHECK (energy    >= 0),
    CONSTRAINT ck_profiles_xp_nonneg        CHECK (xp        >= 0),
    CONSTRAINT ck_profiles_level_range      CHECK (level BETWEEN 1 AND 100)
);

CREATE INDEX idx_profiles_trophies ON profiles (trophies DESC);
