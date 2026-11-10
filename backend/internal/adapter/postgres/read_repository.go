package postgres

import (
	"context"
	"database/sql"
	"errors"
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/chat"
)

type ReadRepository struct{ db *sql.DB }

func NewReadRepository(db *sql.DB) *ReadRepository { return &ReadRepository{db: db} }

var _ chat.ReadRepository = (*ReadRepository)(nil)

func (r *ReadRepository) MarkRead(ctx context.Context, reader, peer, messageID string) (model.ReadReceipt, error) {
	receipt := model.ReadReceipt{ReaderID: reader, PeerID: peer}
	// The INSERT source verifies ownership; the conflict update never moves backward.
	err := r.db.QueryRowContext(ctx, `
 INSERT INTO read_receipts(reader_id,peer_id,message_id,message_created_at)
 SELECT $1,$2,id,created_at FROM messages
 WHERE id=$3 AND recipient_id=$1 AND sender_id=$2
 ON CONFLICT(reader_id,peer_id) DO UPDATE SET
 message_id = CASE
     WHEN (EXCLUDED.message_created_at, EXCLUDED.message_id) >
          (read_receipts.message_created_at, read_receipts.message_id)
     THEN EXCLUDED.message_id
     ELSE read_receipts.message_id
 END,
 message_created_at=GREATEST(read_receipts.message_created_at,EXCLUDED.message_created_at)
 RETURNING message_id,message_created_at`, reader, peer, messageID).Scan(&receipt.MessageID, &receipt.MessageCreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ReadReceipt{}, model.ErrInvalid
	}
	return receipt, translate(err)
}
func (r *ReadRepository) ReadBy(ctx context.Context, reader, peer string) (model.ReadReceipt, error) {
	receipt := model.ReadReceipt{ReaderID: reader, PeerID: peer}
	err := r.db.QueryRowContext(ctx, `
 SELECT message_id, message_created_at
 FROM read_receipts
 WHERE reader_id=$1 AND peer_id=$2
 `, reader, peer).Scan(&receipt.MessageID, &receipt.MessageCreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return receipt, nil
	}
	return receipt, translate(err)
}
