package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed files/*.sql
var migrationFS embed.FS

func Up(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "files"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
