package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mini-messenger/internal/adapter/postgres"
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/chat"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

func TestRepositories(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a disposable PostgreSQL URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	var b [8]byte
	if _, err = rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	schema := "test_" + hex.EncodeToString(b[:])
	if _, err = admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		if _, e := admin.ExecContext(cleanup, `DROP SCHEMA `+schema+` CASCADE`); e != nil {
			t.Errorf("cleanup: %v", e)
		}
	}()
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		t.Fatal("TEST_DATABASE_URL must be a postgres URL")
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := postgres.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migration, err := os.ReadFile("../../migrations/000001_initial.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	keyMigration, err := os.ReadFile("../../migrations/000003_message_keys.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, string(keyMigration)); err != nil {
		t.Fatal(err)
	}
	const a = "00000000-0000-4000-8000-000000000001"
	const bID = "00000000-0000-4000-8000-000000000002"
	const c = "00000000-0000-4000-8000-000000000003"
	for i, id := range []string{a, bID, c} {
		if _, err = db.ExecContext(
			ctx,
			`INSERT INTO users(id,username,password_hash) VALUES($1,$2,'test-hash')`,
			id,
			[]string{"alice", "bob", "charlie"}[i],
		); err != nil {
			t.Fatal(err)
		}
	}
	users := postgres.NewUserRepository(db)
	cred, err := users.FindCredentials(ctx, "alice")
	if err != nil || cred.User.ID != a || cred.PasswordHash != "test-hash" {
		t.Fatal("credentials", err)
	}
	if _, err = users.FindByID(ctx, "00000000-0000-4000-8000-000000000099"); !errors.Is(err, model.ErrNotFound) {
		t.Fatal("missing user", err)
	}
	list, err := users.ListExcept(ctx, a)
	if err != nil || len(list) != 2 || list[0].Username != "bob" {
		t.Fatal("list", err)
	}
	sessions := postgres.NewSessionRepository(db)
	s := model.Session{ID: a, UserID: a, ExpiresAt: time.Now().Add(time.Hour)}
	if err = sessions.Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	got, err := sessions.Find(ctx, a)
	if err != nil || got.Revoked || got.UserID != a {
		t.Fatal("session", err)
	}
	for i := 0; i < 2; i++ {
		if err = sessions.Revoke(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	got, err = sessions.Find(ctx, a)
	if err != nil || !got.Revoked {
		t.Fatal("revocation", err)
	}
	messages := postgres.NewMessageRepository(db)
	for i, id := range []string{a, bID, c} {
		peer := bID
		if i == 2 {
			peer = c
		}
		m, e := messages.Save(ctx, model.Message{ID: id, SenderID: a, RecipientID: peer, Text: "hello 👋"})
		if e != nil || m.CreatedAt.IsZero() {
			t.Fatal("save", e)
		}
	}
	// Force equal timestamps to exercise UUID tie breaking.
	if _, err = db.ExecContext(ctx, `UPDATE messages SET created_at='2026-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	page, err := messages.History(ctx, a, bID, chat.Page{Limit: 1})
	if err != nil || len(page) != 1 || page[0].ID != bID || page[0].Text != "hello 👋" {
		t.Fatal("first page", err)
	}
	page, err = messages.History(ctx, bID, a, chat.Page{Limit: 1, BeforeID: bID})
	if err != nil || len(page) != 1 || page[0].ID != a {
		t.Fatal("second page", err)
	}
	if _, err = messages.History(ctx, a, bID, chat.Page{BeforeID: c}); !errors.Is(err, model.ErrInvalid) {
		t.Fatal("cross-pair cursor allowed", err)
	}
	page, err = messages.History(ctx, bID, c, chat.Page{})
	if err != nil || len(page) != 0 {
		t.Fatal("history leaked", err)
	}
	if _, err = messages.Save(ctx, model.Message{ID: c, SenderID: a, RecipientID: a, Text: "invalid"}); !errors.Is(err, model.ErrInvalid) {
		t.Fatal("self message accepted", err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO users(id,username,password_hash) VALUES('00000000-0000-4000-8000-000000000010','alice','hash')`); err == nil {
		t.Fatal("unique constraint missing")
	}

	readMigration, err := os.ReadFile("../../migrations/000002_read_receipts.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, string(readMigration)); err != nil {
		t.Fatal(err)
	}
	reads := postgres.NewReadRepository(db)
	receipt, err := reads.MarkRead(ctx, bID, a, bID)
	if err != nil || receipt.MessageID != bID {
		t.Fatal("mark incoming read", err)
	}
	receipt, err = reads.MarkRead(ctx, bID, a, a)
	if err != nil || receipt.MessageID != bID {
		t.Fatal("read cursor moved backward", err)
	}
	receipt, err = reads.ReadBy(ctx, bID, a)
	if err != nil || receipt.MessageID != bID {
		t.Fatal("read persistence", err)
	}
	if _, err = reads.MarkRead(ctx, c, a, bID); !errors.Is(err, model.ErrInvalid) {
		t.Fatal("third user marked another conversation", err)
	}
	if _, err = reads.MarkRead(ctx, a, bID, bID); !errors.Is(err, model.ErrInvalid) {
		t.Fatal("sender marked own outgoing message", err)
	}

	// Retrying after a lost response, including concurrently, must create only one row.
	const key = "11111111-1111-4111-8111-111111111111"
	var wg sync.WaitGroup
	results := make(chan model.Message, 12)
	failures := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m, e := messages.Save(ctx, model.Message{
				ID:             fmt.Sprintf("22222222-2222-4222-8222-%012d", i),
				SenderID:       a,
				RecipientID:    bID,
				Text:           "retry-safe",
				IdempotencyKey: key,
			})
			if e != nil {
				failures <- e
			} else {
				results <- m
			}
		}(i)
	}
	wg.Wait()
	close(results)
	close(failures)
	for e := range failures {
		t.Fatal("concurrent retry", e)
	}
	var original model.Message
	for m := range results {
		if original.ID != "" && (m.ID != original.ID || !m.CreatedAt.Equal(original.CreatedAt)) {
			t.Fatal("duplicate identity")
		}
		original = m
	}
	var count int
	if e := db.QueryRowContext(ctx, `SELECT count(*) FROM messages WHERE sender_id=$1 AND idempotency_key=$2`, a, key).Scan(&count); e != nil || count != 1 {
		t.Fatal("duplicate row", count, e)
	}
	for _, changed := range []model.Message{
		{
			ID:             "33333333-3333-4333-8333-333333333333",
			SenderID:       a,
			RecipientID:    bID,
			Text:           "changed",
			IdempotencyKey: key,
		},
		{
			ID:             "33333333-3333-4333-8333-333333333333",
			SenderID:       a,
			RecipientID:    c,
			Text:           "retry-safe",
			IdempotencyKey: key,
		},
	} {
		if _, e := messages.Save(ctx, changed); !errors.Is(e, model.ErrConflict) {
			t.Fatal("key payload conflict", e)
		}
	}
	// A new connection/repository still sees the committed identity.
	reopened, e := postgres.Open(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	replay := original
	replay.ID = "44444444-4444-4444-8444-444444444444"
	replay, e = postgres.NewMessageRepository(reopened).Save(ctx, replay)
	reopened.Close()
	if e != nil || replay.ID != original.ID {
		t.Fatal("durable retry", e)
	}
	conversations := postgres.NewConversationRepository(db)
	summaries, e := conversations.ListConversations(ctx, bID)
	if e != nil || len(summaries) != 2 || summaries[0].Peer.ID != a || summaries[0].UnreadCount != 1 || summaries[0].LastMessage.ID != original.ID {
		t.Fatal("unread summary", summaries, e)
	}
	if _, e = reads.MarkRead(ctx, bID, a, original.ID); e != nil {
		t.Fatal(e)
	}
	summaries, e = conversations.ListConversations(ctx, bID)
	if e != nil || summaries[0].UnreadCount != 0 {
		t.Fatal("summary after reading", e)
	}
	summaries, e = conversations.ListConversations(ctx, c)
	if e != nil || summaries[0].LastMessage.ID != c || summaries[0].UnreadCount != 1 {
		t.Fatal("third-party summary leakage", e)
	}
	if _, e = messages.Save(ctx, model.Message{
		ID:             "55555555-5555-4555-8555-555555555555",
		SenderID:       bID,
		RecipientID:    a,
		Text:           "other sender",
		IdempotencyKey: key,
	}); e != nil {
		t.Fatal("key must be sender scoped", e)
	}

	refreshMigration, err := os.ReadFile("../../migrations/000004_refresh_tokens.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, string(refreshMigration)); err != nil {
		t.Fatal(err)
	}
	testRefreshRotation(t, db)
	if _, err = db.ExecContext(ctx, `DROP TABLE refresh_tokens`); err != nil {
		t.Fatal(err)
	}
	readDown, err := os.ReadFile("../../migrations/000002_read_receipts.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, string(readDown)); err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../migrations/000001_initial.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
}
