package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"mini-messenger/internal/model"
)

// Open validates connectivity. The caller owns the pool and must Close it.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}
func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	var pe *pq.Error
	if errors.As(err, &pe) {
		switch string(pe.Code) {
		case "23503":
			return model.ErrNotFound
		case "23505", "23514", "22P02", "22021":
			return model.ErrInvalid
		}
	}
	return fmt.Errorf("postgres: %w", err)
}
