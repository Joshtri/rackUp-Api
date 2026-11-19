-- Friends: an account only sees its own matches and live sessions and those of
-- its friends, instead of everything.

-- 1. Invitations. Accepting one turns it into a friendship and deletes it.
CREATE TABLE friend_requests (
    id         UUID PRIMARY KEY,
    from_user  UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    to_user    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (from_user, to_user),
    CHECK (from_user <> to_user)
);
CREATE INDEX friend_requests_to_idx ON friend_requests (to_user);

-- 2. Friendships, stored in both directions (a→b and b→a), so "friends of X"
--    is a single lookup on the primary key.
CREATE TABLE friendships (
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    friend_id  UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, friend_id),
    CHECK (user_id <> friend_id)
);

-- 3. The accounts that played a match, found from the player names
--    (display name or username). NULL for a guest without an account.
ALTER TABLE matches ADD COLUMN player_a_user UUID REFERENCES users (id) ON DELETE SET NULL;
ALTER TABLE matches ADD COLUMN player_b_user UUID REFERENCES users (id) ON DELETE SET NULL;
CREATE INDEX matches_user_idx ON matches (user_id);
CREATE INDEX matches_player_a_user_idx ON matches (player_a_user) WHERE player_a_user IS NOT NULL;
CREATE INDEX matches_player_b_user_idx ON matches (player_b_user) WHERE player_b_user IS NOT NULL;
CREATE INDEX live_sessions_created_by_idx ON live_sessions (created_by);

-- 4. Until now everything was shared, so every existing account becomes
--    friends with every other one, and existing matches are linked to the
--    accounts whose names they use. Nobody loses sight of what they could see.
INSERT INTO friendships (user_id, friend_id)
SELECT a.id, b.id FROM users a JOIN users b ON a.id <> b.id;

UPDATE matches m SET
    player_a_user = (
        SELECT u.id FROM users u
        WHERE lower(u.display_name) = lower(m.player_a) OR u.username = lower(m.player_a)
        ORDER BY (lower(u.display_name) = lower(m.player_a)) DESC, u.username
        LIMIT 1),
    player_b_user = (
        SELECT u.id FROM users u
        WHERE lower(u.display_name) = lower(m.player_b) OR u.username = lower(m.player_b)
        ORDER BY (lower(u.display_name) = lower(m.player_b)) DESC, u.username
        LIMIT 1);
