BEGIN;
CREATE TABLE read_receipts (
 reader_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 peer_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 message_id uuid NOT NULL REFERENCES messages(id) ON DELETE RESTRICT,
 message_created_at timestamptz NOT NULL,
 PRIMARY KEY(reader_id,peer_id),
 CHECK(reader_id <> peer_id)
);
COMMIT;
