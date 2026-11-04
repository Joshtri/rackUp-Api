-- 1. Shared history: every account sees and edits the same matches.
--    `matches.user_id` now only records who saved the match.
CREATE INDEX matches_history_idx ON matches (played_on DESC, created_at DESC);
DROP INDEX matches_user_history_idx;

-- 2. Live shot-clock sessions, shared between devices.
--    `state` is the frontend's `Session` object (src/lib/session.ts), stored as-is.
--    `version` goes up on every change, so two phones can't overwrite each other.
CREATE TABLE live_sessions (
    id             UUID PRIMARY KEY,
    created_by     UUID REFERENCES users (id) ON DELETE SET NULL,
    state          JSONB NOT NULL,
    version        INTEGER NOT NULL DEFAULT 1,
    ended_at       TIMESTAMPTZ,
    ended_by       UUID REFERENCES users (id) ON DELETE SET NULL,
    saved_match_id UUID REFERENCES matches (id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Sessions still running, most recently touched first.
CREATE INDEX live_sessions_active_idx ON live_sessions (updated_at DESC) WHERE ended_at IS NULL;
