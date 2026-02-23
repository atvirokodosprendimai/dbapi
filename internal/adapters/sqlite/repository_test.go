package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"github.com/atvirokodosprendimai/dbapi/migrations"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestRepositoryScanByCategoryAndPrefix(t *testing.T) {
	ctx := context.Background()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_ = os.Remove(dbPath)
	})

	if err := migrations.Up(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	gormDB, err := gorm.Open(gormsqlite.Dialector{DriverName: "sqlite", Conn: db}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open gorm: %v", err)
	}
	repo := NewRepository(gormDB)

	seed := []domain.Item{
		{Key: "user:1", Category: "users", Value: json.RawMessage(`{"name":"A"}`)},
		{Key: "user:2", Category: "users", Value: json.RawMessage(`{"name":"B"}`)},
		{Key: "order:1", Category: "orders", Value: json.RawMessage(`{"amount":10}`)},
	}

	for _, item := range seed {
		if _, err := repo.Upsert(ctx, item); err != nil {
			t.Fatalf("seed upsert %s: %v", item.Key, err)
		}
	}

	items, err := repo.Scan(ctx, domain.ScanFilter{Category: "users", Prefix: "user:", Limit: 10})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Key != "user:1" || items[1].Key != "user:2" {
		t.Fatalf("unexpected keys: %s, %s", items[0].Key, items[1].Key)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	ctx := context.Background()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "migrate.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_ = os.Remove(dbPath)
	})

	if err := migrations.Up(ctx, db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := migrations.Up(ctx, db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}
