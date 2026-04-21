CREATE TABLE alliances (
    id          uuid        NOT NULL,
    name        text        NOT NULL,
    tag         text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    leader_id   uuid        NOT NULL,
    channel_id  uuid,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    CONSTRAINT uq_alliances_name     UNIQUE (name),
    CONSTRAINT uq_alliances_tag      UNIQUE (tag),
    CONSTRAINT ck_alliances_name_len CHECK (char_length(name) BETWEEN 2 AND 24),
    CONSTRAINT ck_alliances_tag_len  CHECK (char_length(tag)  BETWEEN 2 AND 6),
    CONSTRAINT fk_alliances_leader   FOREIGN KEY (leader_id)  REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_alliances_channel  FOREIGN KEY (channel_id) REFERENCES chat_channels (id) ON DELETE SET NULL
);

CREATE TABLE alliance_members (
    user_id     uuid        NOT NULL,
    alliance_id uuid        NOT NULL,
    role        text        NOT NULL CHECK (role IN ('leader', 'officer', 'member')),
    joined_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id),
    CONSTRAINT fk_alliance_members_user     FOREIGN KEY (user_id)     REFERENCES users (id)     ON DELETE CASCADE,
    CONSTRAINT fk_alliance_members_alliance FOREIGN KEY (alliance_id) REFERENCES alliances (id)  ON DELETE CASCADE
);

-- All members of an alliance (used by ListMembers and permission checks).
CREATE INDEX idx_alliance_members_alliance ON alliance_members (alliance_id);

CREATE TABLE alliance_invites (
    id          uuid        NOT NULL,
    alliance_id uuid        NOT NULL,
    user_id     uuid        NOT NULL,
    status      text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    CONSTRAINT fk_alliance_invites_alliance FOREIGN KEY (alliance_id) REFERENCES alliances (id) ON DELETE CASCADE,
    CONSTRAINT fk_alliance_invites_user     FOREIGN KEY (user_id)     REFERENCES users (id)     ON DELETE CASCADE
);

-- Prevent duplicate pending invites for the same (alliance, user) pair.
-- A new invite is allowed once the previous one is accepted or rejected.
CREATE UNIQUE INDEX uq_alliance_invites_pending
    ON alliance_invites (alliance_id, user_id)
    WHERE status = 'pending';

-- All invites for a user (used to list inbound invites).
CREATE INDEX idx_alliance_invites_user ON alliance_invites (user_id, status);
