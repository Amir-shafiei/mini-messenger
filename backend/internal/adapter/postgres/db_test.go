package postgres

import (
	"context"
	"database/sql"
	"errors"
	"github.com/lib/pq"
	"mini-messenger/internal/model"
	"testing"
)

func TestTranslate(t *testing.T) {
	for _, tc := range []struct{ in, want error }{
		{sql.ErrNoRows, model.ErrNotFound},
		{&pq.Error{Code: "23503"}, model.ErrNotFound},
		{&pq.Error{Code: "23505"}, model.ErrInvalid},
		{&pq.Error{Code: "23514"}, model.ErrInvalid},
		{&pq.Error{Code: "22P02"}, model.ErrInvalid},
		{context.Canceled, context.Canceled},
	} {
		if !errors.Is(translate(tc.in), tc.want) {
			t.Fatalf("translation failed for %v", tc.in)
		}
	}
	if translate(nil) != nil {
		t.Fatal("nil translated")
	}
}
