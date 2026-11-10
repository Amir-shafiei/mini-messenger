BEGIN;
CREATE TABLE refresh_tokens (
 token_hash text PRIMARY KEY CHECK(length(token_hash)=64),
 session_id uuid NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
 consumed_at timestamptz
);
CREATE INDEX refresh_tokens_session_idx ON refresh_tokens(session_id);
COMMIT;
