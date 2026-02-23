package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/adapters/events"
	"github.com/atvirokodosprendimai/dbapi/internal/adapters/httpapi"
	sqliteadapter "github.com/atvirokodosprendimai/dbapi/internal/adapters/sqlite"
	"github.com/atvirokodosprendimai/dbapi/internal/adapters/sqlite/gormsqlite"
	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"github.com/atvirokodosprendimai/dbapi/internal/core/ports"
	"github.com/atvirokodosprendimai/dbapi/internal/core/usecase"
	"github.com/atvirokodosprendimai/dbapi/migrations"
)

type Config struct {
	Addr             string
	DBPath           string
	BootstrapAPIKey  string
	BootstrapTenant  string
	BootstrapKeyName string
	WebhookURL       string
	WebhookSecret    string
}

type resourceCloser struct {
	closers []io.Closer
}

func (r resourceCloser) Close() error {
	var firstErr error
	for _, c := range r.closers {
		if c == nil {
			continue
		}
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func NewServer(ctx context.Context, cfg Config) (*http.Server, io.Closer, error) {
	db, err := gormsqlite.Open(cfg.DBPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open cqrs sqlite: %w", err)
	}

	writeSQLDB, err := db.WriteSQLDB()
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("resolve writer sql db: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := migrations.Up(ctx, writeSQLDB); err != nil {
		_ = db.Close()
		return nil, nil, err
	}

	repo := sqliteadapter.NewRepository(db)
	recordStore := sqliteadapter.NewRecordEventStore(db)
	apiKeyRepo := sqliteadapter.NewAPIKeyRepository(db)
	auditTrailRepo := sqliteadapter.NewAuditTrailRepository(db)
	outboxRepo := sqliteadapter.NewOutboxRepository(db)

	kvService := usecase.NewKVService(repo)
	recordService := usecase.NewRecordService(recordStore)
	authService := usecase.NewAuthService(apiKeyRepo)
	auditService := usecase.NewAuditService(auditTrailRepo)

	var publisher ports.EventPublisher
	if cfg.WebhookURL != "" {
		publisher = events.NewWebhookPublisher(cfg.WebhookURL, cfg.WebhookSecret, 10*time.Second)
	} else {
		publisher = events.NewLogPublisher()
	}
	dispatcher := usecase.NewOutboxDispatcher(outboxRepo, publisher, 2*time.Second, 100)
	dispatcher.Start(context.Background())
	readinessCheck := func(ctx context.Context) error {
		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		return writeSQLDB.PingContext(checkCtx)
	}

	if cfg.BootstrapAPIKey != "" {
		tenant := cfg.BootstrapTenant
		if tenant == "" {
			tenant = "default"
		}
		name := cfg.BootstrapKeyName
		if name == "" {
			name = "bootstrap"
		}

		bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := apiKeyRepo.Upsert(bootstrapCtx, domain.APIKey{
			TokenHash: usecase.HashToken(cfg.BootstrapAPIKey),
			TenantID:  tenant,
			Name:      name,
			Active:    true,
			CreatedAt: time.Now().UTC(),
		})
		bootstrapCancel()
		if err != nil {
			_ = db.Close()
			return nil, nil, fmt.Errorf("bootstrap api key: %w", err)
		}
	}

	handler := httpapi.NewHandler(
		kvService,
		recordService,
		authService,
		auditService,
		httpapi.WithReadinessCheck(readinessCheck),
		httpapi.WithExtraMetrics(func() map[string]int64 {
			m := dispatcher.Metrics()
			return map[string]int64{
				"outbox_dispatch_success_total": m.DispatchSuccessTotal,
				"outbox_dispatch_failure_total": m.DispatchFailureTotal,
				"outbox_dispatch_dead_total":    m.DispatchDeadTotal,
			}
		}),
	)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return server, resourceCloser{closers: []io.Closer{dispatcher, db}}, nil
}
