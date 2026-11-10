BEGIN;
ALTER TABLE messages DROP COLUMN idempotency_key;
COMMIT;
