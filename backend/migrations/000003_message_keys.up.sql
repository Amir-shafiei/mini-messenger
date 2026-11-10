BEGIN;
ALTER TABLE messages ADD COLUMN idempotency_key uuid;
CREATE UNIQUE INDEX messages_sender_key ON messages(sender_id,idempotency_key);
COMMIT;
