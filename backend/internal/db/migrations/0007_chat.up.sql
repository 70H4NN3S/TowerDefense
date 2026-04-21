CREATE TABLE chat_channels (
    id         uuid        NOT NULL,
    kind       text        NOT NULL CHECK (kind IN ('global', 'alliance', 'direct')),
    owner_id   uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    CONSTRAINT fk_chat_channels_owner FOREIGN KEY (owner_id) REFERENCES users (id) ON DELETE SET NULL
);

CREATE TABLE chat_memberships (
    channel_id uuid        NOT NULL,
    user_id    uuid        NOT NULL,
    joined_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (channel_id, user_id),
    CONSTRAINT fk_chat_memberships_channel FOREIGN KEY (channel_id) REFERENCES chat_channels (id) ON DELETE CASCADE,
    CONSTRAINT fk_chat_memberships_user    FOREIGN KEY (user_id)    REFERENCES users (id)         ON DELETE CASCADE
);

-- Quickly find all channels a user belongs to.
CREATE INDEX idx_chat_memberships_user ON chat_memberships (user_id);

CREATE TABLE chat_messages (
    id         uuid        NOT NULL,
    channel_id uuid        NOT NULL,
    user_id    uuid        NOT NULL,
    body       text        NOT NULL CHECK (char_length(body) > 0 AND char_length(body) <= 500),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    CONSTRAINT fk_chat_messages_channel FOREIGN KEY (channel_id) REFERENCES chat_channels (id) ON DELETE CASCADE,
    CONSTRAINT fk_chat_messages_user    FOREIGN KEY (user_id)    REFERENCES users (id)         ON DELETE RESTRICT
);

-- Cursor-based pagination: fetch messages for a channel ordered newest-first.
CREATE INDEX idx_chat_messages_channel_created ON chat_messages (channel_id, created_at DESC);
