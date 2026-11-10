BEGIN;
CREATE TABLE users (
    id uuid PRIMARY KEY,
    username varchar(32) NOT NULL UNIQUE,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT username_format CHECK (username ~ '^[a-z0-9_]{3,32}$')
);
CREATE TABLE sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    CONSTRAINT session_expiry CHECK (expires_at > created_at)
);
CREATE INDEX sessions_active_user_idx ON sessions(user_id) WHERE revoked_at IS NULL;
CREATE INDEX sessions_expiry_idx ON sessions(expires_at);
CREATE TABLE messages (
    id uuid PRIMARY KEY,
    sender_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    recipient_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    body text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT different_participants CHECK (sender_id <> recipient_id),
    CONSTRAINT message_length CHECK (char_length(body) BETWEEN 1 AND 4000),
    CONSTRAINT message_nonblank CHECK (length(btrim(body)) > 0)
);
CREATE INDEX messages_pair_history_idx ON messages (
    LEAST(sender_id, recipient_id), GREATEST(sender_id, recipient_id),
    created_at DESC, id DESC
);
COMMIT;
