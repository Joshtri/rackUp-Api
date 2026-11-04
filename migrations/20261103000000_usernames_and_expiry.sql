-- Accounts are created by an admin (`cargo run -- user create`), log in with a
-- username instead of an email, and stop working after `expires_at`.

ALTER TABLE users ADD COLUMN username TEXT;
-- Existing accounts keep working: their email becomes their username.
UPDATE users SET username = email;
ALTER TABLE users ALTER COLUMN username SET NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_username_key UNIQUE (username); -- stored lowercase

ALTER TABLE users DROP COLUMN email;

-- Existing accounts get the standard 30 days, counted from this migration.
ALTER TABLE users ADD COLUMN expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '30 days';
ALTER TABLE users ALTER COLUMN expires_at DROP DEFAULT;
