package postgres

import (
	"context"
	"database/sql"
	"errors"
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/chat"
	"strings"
	"time"
)

type MessageRepository struct{ db *sql.DB }

func NewMessageRepository(db *sql.DB) *MessageRepository { return &MessageRepository{db: db} }

var _ chat.MessageRepository = (*MessageRepository)(nil)

func (r *MessageRepository) Save(ctx context.Context, m model.Message) (model.Message, error) {
	if err := model.ValidateMessage(m.SenderID, m.RecipientID, m.Text); err != nil {
		return model.Message{}, err
	}
	if strings.ContainsRune(m.Text, 0) {
		return model.Message{}, model.ErrInvalid
	}
	// A single statement serializes concurrent retries on the unique key.
	// A replay returns the original identity and timestamp without changing the payload.
	err := r.db.QueryRowContext(ctx, `
 INSERT INTO messages(id,sender_id,recipient_id,body,idempotency_key)
 VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid)
 ON CONFLICT (sender_id,idempotency_key) DO UPDATE
 SET idempotency_key=EXCLUDED.idempotency_key
 WHERE messages.recipient_id=EXCLUDED.recipient_id AND messages.body=EXCLUDED.body
 RETURNING id,created_at
 `, m.ID, m.SenderID, m.RecipientID, m.Text, m.IdempotencyKey).Scan(&m.ID, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Message{}, model.ErrConflict
	}
	if err != nil {
		return model.Message{}, translate(err)
	}
	return m, nil
}

const pairFilter = `
    LEAST(sender_id,recipient_id)=LEAST($1::uuid,$2::uuid)
    AND GREATEST(sender_id,recipient_id)=GREATEST($1::uuid,$2::uuid)
`

func (r *MessageRepository) History(ctx context.Context, actor, peer string, p chat.Page) ([]model.Message, error) {
	if actor == "" || peer == "" || actor == peer {
		return nil, model.ErrInvalid
	}
	if p.Limit == 0 {
		p.Limit = 50
	}
	if p.Limit < 1 || p.Limit > 100 {
		return nil, model.ErrInvalid
	}
	query := `
    SELECT id,sender_id,recipient_id,body,created_at
    FROM messages
    WHERE
` + pairFilter
	args := []any{actor, peer}
	if p.BeforeID != "" {
		at, id, err := r.findHistoryCursor(ctx, actor, peer, p.BeforeID)
		if err != nil {
			return nil, err
		}
		query += `
    AND (created_at,id)<($3,$4::uuid)
    ORDER BY created_at DESC,id DESC
    LIMIT $5
`
		args = append(args, at, id, p.Limit)
	} else {
		query += `
    ORDER BY created_at DESC,id DESC
    LIMIT $3
`
		args = append(args, p.Limit)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	out := make([]model.Message, 0)
	for rows.Next() {
		var m model.Message
		if err = rows.Scan(&m.ID, &m.SenderID, &m.RecipientID, &m.Text, &m.CreatedAt); err != nil {
			return nil, translate(err)
		}
		out = append(out, m)
	}
	return out, translate(rows.Err())
}

// Resolve the cursor inside this conversation; another user's message cannot anchor a page.
func (r *MessageRepository) findHistoryCursor(ctx context.Context, actorID, peerID, cursorID string) (time.Time, string, error) {
	var createdAt time.Time
	var messageID string
	query := `SELECT created_at, id FROM messages WHERE ` + pairFilter + ` AND id=$3`
	err := r.db.QueryRowContext(ctx, query, actorID, peerID, cursorID).Scan(&createdAt, &messageID)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, "", model.ErrInvalid
	}
	if err != nil {
		return time.Time{}, "", translate(err)
	}
	return createdAt, messageID, nil
}
