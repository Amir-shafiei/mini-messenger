# Database contract

Status: implemented schema with versioned startup migrations.

## Tables and mapping
All IDs are application-generated random UUIDs, represented by strings in the core. All times are UTC; PostgreSQL stores timestamptz. No conversation table is needed for one-to-one text chat.

| Table | Purpose | Core mapping |
|---|---|---|
| users | Unique lowercase username and password hash | User; password_hash is exposed only through Credentials |
| sessions | Server-side revocation and expiry per login | Session.Revoked = revoked_at IS NOT NULL |
| messages | Immutable text and participant IDs | body maps to Message.Text |

Usernames use 3-32 lowercase ASCII letters, digits or underscores. Login normalizes username with trim + lowercase before calling the use case; passwords are never normalized. Registration, account deletion and message editing/deletion are outside v1. Unicode whitespace validation belongs to model.ValidateMessage; the database nonblank check is only a secondary guard. Reject NUL in HTTP input before persistence because PostgreSQL text cannot store it.

## Repository behavior
- FindCredentials uses username; never return its password hash to HTTP clients. Unknown username maps to model.ErrNotFound.
- ListExcept returns id and username only, sorted by username; the initial seeded user set is small.
- Sessions.Create inserts ID, user ID and expiry. Find returns revoked sessions too, so the use case can reject them. Revoke sets revoked_at = COALESCE(revoked_at, now()) and is idempotent for existing rows. Missing rows map to ErrNotFound.
- Messages.Save inserts the assigned UUID and participants/body, uses database now() for created_at and returns the persisted row. Domain clock time is provisional; the returned database timestamp is authoritative.
- Missing recipient maps to ErrNotFound; constraints indicating invalid input map to ErrInvalid. Unexpected database errors are wrapped internally and never exposed verbatim in HTTP.

## Stable history pagination
Use the existing Page.BeforeID as the UUID of a message in the same participant pair. Resolve its (created_at,id) within that pair; an unknown or other-pair cursor maps to ErrInvalid. Never resolve the cursor globally then trust it.

Filter the pair using LEAST(sender_id,recipient_id) and GREATEST(sender_id,recipient_id), matching the index. The actor is supplied by authentication, not a query parameter. Sort by created_at DESC, id DESC. For subsequent pages require `(created_at,id) < (cursor_created_at,cursor_id)`. Apply LIMIT 1-100; default 50. A full page may expose its last ID as next_before even if the next page is empty; no has_more guarantee is made.

The same timestamp is safely disambiguated by UUID. IDs themselves are not chronological. Cursor rows cannot disappear because message deletion is not offered. Reconnection is history resynchronization, not a durable event stream: clients reload the active conversation from the newest page and merge by ID; fetch older pages as necessary. Concurrent commits are visible on later resynchronizations, not guaranteed by a fixed snapshot across HTTP requests.

## Migration and seed lifecycle
Migration files are ordered up/down SQL pairs with their own transactions. The runner records versions and applies migrations under an advisory lock before serving traffic. Down migration is destructive and is only for explicit development rollback. No rollback was executed here.

An opt-in demo seed step creates alice, bob and charlie with hashed passwords, using ON CONFLICT(username) DO NOTHING; restarting must never overwrite passwords. Local demo credentials are documented in README. Schema migrations contain no accounts or secrets. Expired sessions may later be removed by maintenance; expiry validation must not depend on that cleanup.

## Required PostgreSQL integration checks
Apply up on an empty database; verify foreign keys, uniqueness and message constraints. Exercise create/find/revoke, pair isolation with a third user, other-pair cursor rejection and pagination with equal timestamps. Persist and reload Persian text. Verify Save returns the stored timestamp. Explicitly test down then up only in a disposable database.

## Migration 2: read_receipts
Composite primary key (reader_id,peer_id); fields message_id and message_created_at identify the last acknowledged incoming message. Foreign keys reference users/messages. The repository inserts only from a messages row whose recipient is reader and sender is peer. Conflict updates compare (timestamp,UUID) and never regress. The API cannot accept a separate caller identity. Versioned runner applies both migrations in order; existing users/messages remain intact.

## Message retry keys (migration 3)
messages.idempotency_key is a nullable UUID so existing messages can migrate unchanged. New sends require a key at the HTTP/service boundary. A unique (sender_id,idempotency_key) index makes retries atomic under concurrency; the repository upsert returns the existing ID/created_at only when recipient and body also match. A mismatched payload returns the model conflict error. No in-memory deduplication cache or key expiry is used.

## Refresh tokens (migration 4)
refresh_tokens stores token_hash (SHA-256 hex primary key), session_id (foreign key with cascade delete) and consumed_at. Sessions expire seven days after login, independently of the short access JWT. Rotation locks the parent session row, checks activity and token consumption, marks the old hash consumed and inserts the next hash in one transaction. Reusing a consumed hash sets sessions.revoked_at. No raw refresh secret is stored. Retention cleanup is a future maintenance task; expired sessions are rejected even before cleanup.
