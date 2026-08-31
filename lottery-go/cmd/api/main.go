// Command api serves the lottery search HTTP API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"lottery"
	"lottery/postgres"
)

type config struct {
	addr        string
	dsn         string
	lease       time.Duration
	maxConns    int32
	reapEvery   time.Duration
	reapBatch   int
	shutdownGap time.Duration
	useHint     bool
	useListen   bool
	refreshEvery time.Duration
}

func loadConfig() config {
	return config{
		addr:        env("LOTTERY_ADDR", ":8080"),
		dsn:         env("LOTTERY_DSN", "postgres://lottery:lottery@localhost:5432/lottery?sslmode=disable"),
		lease:       envDuration("LOTTERY_LEASE", 2*time.Minute),
		maxConns:    int32(envInt("LOTTERY_MAX_CONNS", 20)),
		reapEvery:   envDuration("LOTTERY_REAP_EVERY", 30*time.Second),
		reapBatch:   envInt("LOTTERY_REAP_BATCH", 5000),
		shutdownGap: envDuration("LOTTERY_SHUTDOWN_GRACE", 15*time.Second),
		useHint:     envBool("LOTTERY_USE_HINT", true),
		useListen:   envBool("LOTTERY_USE_LISTEN", false),
		refreshEvery: envDuration("LOTTERY_HINT_REFRESH", 30*time.Second),
	}
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := loadConfig()

	// Cancelled on SIGINT/SIGTERM, which unwinds the reaper and the server.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	poolCfg, err := pgxpool.ParseConfig(cfg.dsn)
	if err != nil {
		return err
	}
	poolCfg.MaxConns = cfg.maxConns

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return err
	}
	log.Info("connected to postgres", "max_conns", cfg.maxConns)

	if err := postgres.Migrate(ctx, pool, postgres.Schema); err != nil {
		return err
	}
	log.Info("schema applied")

	// The reaper works on raw SQL, so it is built from the plain store
	// regardless of which store serves requests.
	plain := postgres.New(pool)
	go plain.RunReaper(ctx, cfg.reapEvery, cfg.reapBatch, func(err error) {
		log.Error("reaper", "error", err)
	})

	// Choose the backend. The hybrid store adds an in-memory availability
	// hint in front of the same SQL; both satisfy lottery.TicketStore, so
	// nothing downstream changes.
	var store lottery.TicketStore = plain
	if cfg.useHint {
		hybrid, err := postgres.NewHybrid(ctx, pool, log)
		if err != nil {
			return err
		}
		// Pulls in sales made by OTHER replicas. Without it, each replica
		// only ever learns about its own, and its hint drifts upward.
		go hybrid.RunRefresher(ctx, cfg.refreshEvery)
		if cfg.useListen {
			go func() {
				if err := hybrid.RunListener(ctx); err != nil {
					log.Error("notification listener", "error", err)
				}
			}()
		}
		store = hybrid
	}

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           lottery.NewServer(store, cfg.lease, log),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.addr, "lease", cfg.lease)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
	}

	// Give in-flight requests a chance to finish before dropping connections.
	shutCtx, cancelShut := context.WithTimeout(context.Background(), cfg.shutdownGap)
	defer cancelShut()
	return srv.Shutdown(shutCtx)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	switch os.Getenv(key) {
	case "1", "true", "TRUE", "yes":
		return true
	case "0", "false", "FALSE", "no":
		return false
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
