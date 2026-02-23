package app

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/adapters/httpapi"
	sqliteadapter "github.com/atvirokodosprendimai/dbapi/internal/adapters/sqlite"
	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"github.com/atvirokodosprendimai/dbapi/internal/core/usecase"
	"github.com/atvirokodosprendimai/dbapi/migrations"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

type Config struct {
	Addr             string
	DBPath           string
	BootstrapAPIKey  string
	BootstrapTenant  string
	BootstrapKeyName string
}

func NewServer(ctx context.Context, cfg Config) (*http.Server, io.Closer, error) {
	sqlDB, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetConnMaxLifetime(0)
	sqlDB.SetConnMaxIdleTime(0)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if _, err := sqlDB.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := sqlDB.ExecContext(ctx, `PRAGMA busy_timeout = 5000;`); err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("set busy timeout: %w", err)
	}
	if err := migrations.Up(ctx, sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, nil, err
	}

	gormDB, err := gorm.Open(gormsqlite.Dialector{
		DriverName: "sqlite",
		Conn:       sqlDB,
	}, &gorm.Config{})
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("open gorm sqlite: %w", err)
	}

	repo := sqliteadapter.NewRepository(gormDB)
	apiKeyRepo := sqliteadapter.NewAPIKeyRepository(gormDB)
	auditRepo := sqliteadapter.NewAuditRepository(gormDB)

	kvService := usecase.NewKVService(repo)
	recordService := usecase.NewRecordService(kvService, auditRepo)
	authService := usecase.NewAuthService(apiKeyRepo)

	if cfg.BootstrapAPIKey != "" {
		tenant := cfg.BootstrapTenant
		if tenant == "" {
			tenant = "default"
		}
		name := cfg.BootstrapKeyName
		if name == "" {
			name = "bootstrap"
		}

		err := apiKeyRepo.Upsert(ctx, domain.APIKey{
			TokenHash: usecase.HashToken(cfg.BootstrapAPIKey),
			TenantID:  tenant,
			Name:      name,
			Active:    true,
			CreatedAt: time.Now().UTC(),
		})
		if err != nil {
			_ = sqlDB.Close()
			return nil, nil, fmt.Errorf("bootstrap api key: %w", err)
		}
	}

	handler := httpapi.NewHandler(kvService, recordService, authService)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return server, sqlDB, nil
}
