package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/app"
	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:  "dbapi",
		Usage: "SQLite-backed JSON KV API",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "addr",
				Value: ":8080",
				Usage: "HTTP listen address",
			},
			&cli.StringFlag{
				Name:  "db-path",
				Value: "./dbapi.sqlite",
				Usage: "SQLite file path",
			},
			&cli.StringFlag{
				Name:    "bootstrap-api-key",
				Sources: cli.EnvVars("DBAPI_BOOTSTRAP_API_KEY"),
				Usage:   "Optional API key to upsert at startup",
			},
			&cli.StringFlag{
				Name:    "bootstrap-tenant",
				Value:   "default",
				Sources: cli.EnvVars("DBAPI_BOOTSTRAP_TENANT"),
				Usage:   "Tenant for bootstrap API key",
			},
			&cli.StringFlag{
				Name:    "bootstrap-key-name",
				Value:   "bootstrap",
				Sources: cli.EnvVars("DBAPI_BOOTSTRAP_KEY_NAME"),
				Usage:   "Name for bootstrap API key",
			},
			&cli.StringFlag{
				Name:    "webhook-url",
				Sources: cli.EnvVars("DBAPI_WEBHOOK_URL"),
				Usage:   "Outbox event webhook target URL (enables push delivery to n8n or other receivers)",
			},
			&cli.StringFlag{
				Name:    "webhook-secret",
				Sources: cli.EnvVars("DBAPI_WEBHOOK_SECRET"),
				Usage:   "HMAC-SHA256 signing secret for outbound webhook requests",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			cfg := app.Config{
				Addr:             c.String("addr"),
				DBPath:           c.String("db-path"),
				BootstrapAPIKey:  c.String("bootstrap-api-key"),
				BootstrapTenant:  c.String("bootstrap-tenant"),
				BootstrapKeyName: c.String("bootstrap-key-name"),
				WebhookURL:       c.String("webhook-url"),
				WebhookSecret:    c.String("webhook-secret"),
			}

			server, closer, err := app.NewServer(ctx, cfg)
			if err != nil {
				return fmt.Errorf("create server: %w", err)
			}
			defer func() {
				if closeErr := closer.Close(); closeErr != nil {
					log.Printf("close resources: %v", closeErr)
				}
			}()

			errCh := make(chan error, 1)
			go func() {
				log.Printf("listening on %s", cfg.Addr)
				errCh <- server.ListenAndServe()
			}()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			defer signal.Stop(sigCh)

			select {
			case <-ctx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				return server.Shutdown(shutdownCtx)
			case sig := <-sigCh:
				log.Printf("received signal %s", sig)
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				return server.Shutdown(shutdownCtx)
			case err := <-errCh:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return err
			}
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
