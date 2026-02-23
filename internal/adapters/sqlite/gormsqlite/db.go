package gormsqlite

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"time"

	gormdriver "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

type DB struct {
	R *gorm.DB
	W *gorm.DB
}

type Tx struct {
	*gorm.DB
}

type cbfn func(tx *Tx) error

func (db *DB) ReadTX(ctx context.Context, fn cbfn) error {
	return db.R.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Tx{DB: tx})
	}, &sql.TxOptions{ReadOnly: true})
}

func (db *DB) WriteTX(ctx context.Context, fn cbfn) error {
	return db.W.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Tx{DB: tx})
	})
}

func (db *DB) WriteSQLDB() (*sql.DB, error) {
	return db.W.DB()
}

func (db *DB) Close() error {
	var firstErr error
	closeOne := func(g *gorm.DB) {
		if g == nil {
			return
		}
		sqlDB, err := g.DB()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return
		}
		if err := sqlDB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	closeOne(db.R)
	closeOne(db.W)
	return firstErr
}

var _ io.Closer = (*DB)(nil)

func Open(file string) (*DB, error) {
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Silent,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  false,
		},
	)

	reader, err := gorm.Open(gormdriver.Dialector{DriverName: "sqlite", DSN: file}, &gorm.Config{
		PrepareStmt: true,
		Logger:      newLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("open read db: %w", err)
	}

	writer, err := gorm.Open(gormdriver.Dialector{DriverName: "sqlite", DSN: file}, &gorm.Config{
		PrepareStmt: true,
		Logger:      newLogger,
	})
	if err != nil {
		_ = closeGORM(reader)
		return nil, fmt.Errorf("open write db: %w", err)
	}

	rdb, err := reader.DB()
	if err != nil {
		_ = closeGORM(reader)
		_ = closeGORM(writer)
		return nil, fmt.Errorf("reader sql db: %w", err)
	}
	wdb, err := writer.DB()
	if err != nil {
		_ = closeGORM(reader)
		_ = closeGORM(writer)
		return nil, fmt.Errorf("writer sql db: %w", err)
	}

	rdb.SetMaxOpenConns(runtime.NumCPU())
	rdb.SetMaxIdleConns(runtime.NumCPU())
	rdb.SetConnMaxLifetime(0)
	rdb.SetConnMaxIdleTime(0)

	wdb.SetMaxOpenConns(1)
	wdb.SetMaxIdleConns(1)
	wdb.SetConnMaxLifetime(0)
	wdb.SetConnMaxIdleTime(0)

	if err := applyPragmas(rdb, true); err != nil {
		_ = closeGORM(reader)
		_ = closeGORM(writer)
		return nil, fmt.Errorf("reader pragmas: %w", err)
	}
	if err := applyPragmas(wdb, false); err != nil {
		_ = closeGORM(reader)
		_ = closeGORM(writer)
		return nil, fmt.Errorf("writer pragmas: %w", err)
	}

	return &DB{R: reader, W: writer}, nil
}

func applyPragmas(db *sql.DB, readOnly bool) error {
	stmts := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA temp_store = MEMORY;",
		"PRAGMA wal_autocheckpoint = 1000;",
		"PRAGMA cache_size = -20000;",
		"PRAGMA mmap_size = 268435456;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA trusted_schema = OFF;",
	}
	if readOnly {
		stmts = append(stmts, "PRAGMA query_only = ON;")
	} else {
		stmts = append(stmts, "PRAGMA query_only = OFF;")
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

func closeGORM(g *gorm.DB) error {
	if g == nil {
		return nil
	}
	sqlDB, err := g.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
