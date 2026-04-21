-- events: timed challenge definitions.
CREATE TABLE events (
    id          uuid        NOT NULL,
    kind        text        NOT NULL,
    name        text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    starts_at   timestamptz NOT NULL,
    ends_at     timestamptz NOT NULL,
    config      jsonb       NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    CONSTRAINT ck_events_kind   CHECK (kind IN ('kill_n_monsters')),
    CONSTRAINT ck_events_window CHECK (ends_at > starts_at)
);

-- Index for querying active/upcoming events by time window.
CREATE INDEX idx_events_window ON events (starts_at, ends_at);

-- event_progress: per-user progress within an event.
CREATE TABLE event_progress (
    event_id        uuid        NOT NULL,
    user_id         uuid        NOT NULL,
    progress        jsonb       NOT NULL DEFAULT '{}',
    claimed_rewards jsonb       NOT NULL DEFAULT '[]',
    updated_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, user_id),
    CONSTRAINT fk_event_progress_event FOREIGN KEY (event_id) REFERENCES events  (id) ON DELETE CASCADE,
    CONSTRAINT fk_event_progress_user  FOREIGN KEY (user_id)  REFERENCES users   (id) ON DELETE CASCADE
);
