package migrations

import (
	"context"
	"database/sql"
	_ "embed"
	"strconv"
	"strings"
)

//go:embed 000001_initial.up.sql
var initialSchema string

//go:embed 000002_read_receipts.up.sql
var readSchema string

//go:embed 000003_message_keys.up.sql
var keySchema string

//go:embed 000004_refresh_tokens.up.sql
var refreshSchema string

// Apply uses one connection so the advisory lock covers both version checks and DDL.
func Apply(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, `SELECT pg_advisory_lock(742019)`); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(742019)`)
	if _, err = conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version integer PRIMARY KEY)`); err != nil {
		return err
	}
	for index, schema := range []string{initialSchema, readSchema, keySchema, refreshSchema} {
		version := index + 1
		var applied bool
		if err = conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		script := strings.TrimSuffix(strings.TrimSpace(schema), "COMMIT;") + "INSERT INTO schema_migrations(version) VALUES(" + strconv.Itoa(version) + "); COMMIT;"
		if _, err = conn.ExecContext(ctx, script); err != nil {
			conn.ExecContext(context.Background(), "ROLLBACK")
			return err
		}
	}
	return nil
}
