CREATE TABLE users (
    id          uuid        NOT NULL PRIMARY KEY,
    email       citext      NOT NULL,
    username    citext      NOT NULL,
    password_hash text      NOT NULL,
    trophies    bigint      NOT NULL DEFAULT 0 CHECK (trophies >= 0),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_users_email    UNIQUE (email),
    CONSTRAINT uq_users_username UNIQUE (username)
);

CREATE INDEX idx_users_email    ON users (email);
CREATE INDEX idx_users_username ON users (username);
