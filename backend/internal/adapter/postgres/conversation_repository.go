package postgres

import (
	"context"
	"database/sql"
	"mini-messenger/internal/model"
)

type ConversationRepository struct{ db *sql.DB }

func NewConversationRepository(db *sql.DB) *ConversationRepository {
	return &ConversationRepository{db}
}
func (r *ConversationRepository) ListConversations(ctx context.Context, actor string) ([]model.Conversation, error) {
	rows, err := r.db.QueryContext(ctx, `
 SELECT u.id,u.username,m.id,m.sender_id,m.recipient_id,m.body,m.created_at,
 (SELECT count(*) FROM messages incoming
  LEFT JOIN read_receipts rr ON rr.reader_id=$1 AND rr.peer_id=u.id
  WHERE incoming.recipient_id=$1 AND incoming.sender_id=u.id
   AND (rr.message_id IS NULL OR (incoming.created_at,incoming.id)>(rr.message_created_at,rr.message_id)))
 FROM users u LEFT JOIN LATERAL (
 SELECT id,sender_id,recipient_id,body,created_at FROM messages
 WHERE LEAST(sender_id,recipient_id)=LEAST($1::uuid,u.id)
 AND GREATEST(sender_id,recipient_id)=GREATEST($1::uuid,u.id)
 ORDER BY created_at DESC,id DESC LIMIT 1
 ) m ON true WHERE u.id<>$1
 ORDER BY m.created_at DESC NULLS LAST,m.id DESC NULLS LAST,u.username`, actor)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	out := make([]model.Conversation, 0)
	for rows.Next() {
		var c model.Conversation
		var id, sender, recipient, body sql.NullString
		var created sql.NullTime
		if err = rows.Scan(&c.Peer.ID, &c.Peer.Username, &id, &sender, &recipient, &body, &created, &c.UnreadCount); err != nil {
			return nil, translate(err)
		}
		if id.Valid {
			c.LastMessage = &model.Message{
				ID:          id.String,
				SenderID:    sender.String,
				RecipientID: recipient.String,
				Text:        body.String,
				CreatedAt:   created.Time,
			}
		}
		out = append(out, c)
	}
	return out, translate(rows.Err())
}
