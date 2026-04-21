CREATE TABLE matches (
    id          uuid        NOT NULL,
    player_one  uuid        NOT NULL,
    player_two  uuid,
    mode        text        NOT NULL CHECK (mode IN ('solo', 'ranked', 'casual')),
    map_id      text        NOT NULL,
    seed        bigint      NOT NULL,
    started_at  timestamptz NOT NULL DEFAULT now(),
    ended_at    timestamptz,
    winner      uuid,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    CONSTRAINT fk_matches_player_one FOREIGN KEY (player_one) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_matches_player_two FOREIGN KEY (player_two) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_matches_winner     FOREIGN KEY (winner)     REFERENCES users (id) ON DELETE RESTRICT
);

-- Lookup a player's match history, most recent first.
CREATE INDEX idx_matches_player_one ON matches (player_one, started_at DESC);

-- Lookup matches involving a second player (multiplayer only).
CREATE INDEX idx_matches_player_two ON matches (player_two, started_at DESC)
    WHERE player_two IS NOT NULL;
