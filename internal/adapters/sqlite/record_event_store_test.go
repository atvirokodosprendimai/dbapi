package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/atvirokodosprendimai/dbapi/internal/adapters/sqlite/gormsqlite"
	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"github.com/atvirokodosprendimai/dbapi/migrations"
)

func TestDotPathToSQLiteJSONPathQuotesSegments(t *testing.T) {
	got := dotPathToSQLiteJSONPath("customer.first-name")
	want := `$."customer"."first-name"`
	if got != want {
		t.Fatalf("unexpected path: got %s want %s", got, want)
	}
}

func TestRecordEventStoreOutboxFailureRollsBackUpsertAndDelete(t *testing.T) {
	ctx := context.Background()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "events.sqlite")
	db, err := gormsqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	wdb, err := db.WriteSQLDB()
	if err != nil {
		t.Fatalf("writer sql db: %v", err)
	}

	if err := migrations.Up(ctx, wdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := NewRecordEventStore(db)
	repo := NewRepository(db)

	meta := domain.MutationMetadata{
		Actor:          "tester",
		Source:         "test",
		RequestID:      "req-1",
		CorrelationID:  "corr-1",
		CausationID:    "cause-1",
		IdempotencyKey: "idem-1",
	}

	if _, err := wdb.ExecContext(ctx, `
		CREATE TRIGGER trg_fail_outbox_insert
		BEFORE INSERT ON outbox_events
		BEGIN
			SELECT RAISE(ABORT, 'forced outbox failure');
		END;
	`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	t.Run("upsert rollback", func(t *testing.T) {
		rec := domain.Record{
			TenantID:   "t1",
			Collection: "users",
			ID:         "u1",
			Data:       json.RawMessage(`{"name":"A"}`),
		}

		_, err := store.UpsertWithEvents(ctx, rec, meta)
		if err == nil {
			t.Fatalf("expected upsert error")
		}
		if !strings.Contains(err.Error(), "forced outbox failure") {
			t.Fatalf("expected forced outbox failure, got: %v", err)
		}

		assertTableCount(t, ctx, wdb, "kv_entries", 0)
		assertTableCount(t, ctx, wdb, "audit_events", 0)
		assertTableCount(t, ctx, wdb, "outbox_events", 0)
	})

	t.Run("delete rollback", func(t *testing.T) {
		if _, err := wdb.ExecContext(ctx, "DROP TRIGGER IF EXISTS trg_fail_outbox_insert"); err != nil {
			t.Fatalf("drop trigger: %v", err)
		}

		seed := domain.Item{
			Key:      "t1/users/u2",
			Category: "t1/users",
			Value:    json.RawMessage(`{"name":"B"}`),
		}
		if _, err := repo.Upsert(ctx, seed); err != nil {
			t.Fatalf("seed row: %v", err)
		}

		if _, err := wdb.ExecContext(ctx, `
			CREATE TRIGGER trg_fail_outbox_insert
			BEFORE INSERT ON outbox_events
			BEGIN
				SELECT RAISE(ABORT, 'forced outbox failure');
			END;
		`); err != nil {
			t.Fatalf("recreate failure trigger: %v", err)
		}

		deleted, err := store.DeleteWithEvents(ctx, "t1", "users", "u2", meta)
		if err == nil {
			t.Fatalf("expected delete error")
		}
		if deleted {
			t.Fatalf("expected deleted=false on rollback")
		}
		if !strings.Contains(err.Error(), "forced outbox failure") {
			t.Fatalf("expected forced outbox failure, got: %v", err)
		}

		assertTableCount(t, ctx, wdb, "kv_entries", 1)
		assertTableCount(t, ctx, wdb, "audit_events", 0)
		assertTableCount(t, ctx, wdb, "outbox_events", 0)
	})
}

func assertTableCount(t *testing.T, ctx context.Context, wdb *sql.DB, table string, want int) {
	t.Helper()
	var got int
	row := wdb.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table)
	if err := row.Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("unexpected %s count: got %d want %d", table, got, want)
	}
}
