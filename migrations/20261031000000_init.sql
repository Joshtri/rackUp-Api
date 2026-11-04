CREATE TABLE users (
    id            UUID PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE, -- stored lowercase
    password_hash TEXT NOT NULL,
    display_name  TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE matches (
    id         UUID PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    played_on  DATE NOT NULL,
    player_a   TEXT NOT NULL,
    player_b   TEXT NOT NULL,
    score_a    INTEGER NOT NULL CHECK (score_a >= 0),
    score_b    INTEGER NOT NULL CHECK (score_b >= 0),
    note       TEXT,
    log        JSONB, -- shot-clock session log, only for live sessions
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- History order: newest date first, then newest entry.
CREATE INDEX matches_user_history_idx ON matches (user_id, played_on DESC, created_at DESC);
