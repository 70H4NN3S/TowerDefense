-- Tower catalog: static templates, per-level stat progression, and player
-- ownership. All integer stats use bigint so the game designer has headroom
-- without a schema change.

CREATE TABLE tower_templates (
    id             uuid        NOT NULL PRIMARY KEY,
    name           text        NOT NULL,
    rarity         text        NOT NULL,
    base_damage    bigint      NOT NULL,
    base_range     bigint      NOT NULL,
    base_rate      bigint      NOT NULL,
    cost_diamonds  bigint      NOT NULL,
    description    text        NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_tower_templates_name           UNIQUE (name),
    CONSTRAINT ck_tower_templates_rarity         CHECK (rarity IN ('common','rare','epic','legendary')),
    CONSTRAINT ck_tower_templates_base_damage    CHECK (base_damage    > 0),
    CONSTRAINT ck_tower_templates_base_range     CHECK (base_range     > 0),
    CONSTRAINT ck_tower_templates_base_rate      CHECK (base_rate      > 0),
    CONSTRAINT ck_tower_templates_cost_diamonds  CHECK (cost_diamonds  >= 0)
);

-- One row per (template, level) pair — levels 1 through 10.
CREATE TABLE tower_levels (
    template_id  uuid    NOT NULL,
    level        int     NOT NULL,
    gold_cost    bigint  NOT NULL,
    damage       bigint  NOT NULL,
    range_       bigint  NOT NULL,
    rate         bigint  NOT NULL,

    PRIMARY KEY (template_id, level),

    CONSTRAINT fk_tower_levels_template  FOREIGN KEY (template_id) REFERENCES tower_templates(id) ON DELETE CASCADE,
    CONSTRAINT ck_tower_levels_level     CHECK (level BETWEEN 1 AND 10),
    CONSTRAINT ck_tower_levels_gold_cost CHECK (gold_cost    >= 0),
    CONSTRAINT ck_tower_levels_damage    CHECK (damage       > 0),
    CONSTRAINT ck_tower_levels_range     CHECK (range_       > 0),
    CONSTRAINT ck_tower_levels_rate      CHECK (rate         > 0)
);

-- One row per player-owned tower. A player may own each template at most once.
CREATE TABLE owned_towers (
    user_id      uuid        NOT NULL,
    template_id  uuid        NOT NULL,
    level        int         NOT NULL DEFAULT 1,
    unlocked_at  timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, template_id),

    CONSTRAINT fk_owned_towers_user      FOREIGN KEY (user_id)     REFERENCES users(id)            ON DELETE CASCADE,
    CONSTRAINT fk_owned_towers_template  FOREIGN KEY (template_id) REFERENCES tower_templates(id)  ON DELETE RESTRICT,
    CONSTRAINT ck_owned_towers_level     CHECK (level BETWEEN 1 AND 10)
);

CREATE INDEX idx_owned_towers_user_id ON owned_towers (user_id);
