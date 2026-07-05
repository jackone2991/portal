// Command api runs the Portal HTTP server.
//
// v1 scope: see doc/en/architecture/01-v1-scope-cut.md. Only the account module
// is mounted; other modules attach under r.Route("/api/v1", ...) the same way.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/portal/backend/internal/modules/account"
	"github.com/portal/backend/internal/modules/account/auth"
	accountrepo "github.com/portal/backend/internal/modules/account/repository"
	"github.com/portal/backend/internal/platform/config"
)

func main() {
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("api shutdown with error")
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	level, _ := zerolog.ParseLevel(cfg.LogLevel)
	zerolog.SetGlobalLevel(level)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	ctx := context.Background()

	// ── Infrastructure ──────────────────────────────────────────────
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("db ping: %w", err)
	}

	redisOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("redis url: %w", err)
	}
	rdb := redis.NewClient(redisOpt)
	defer func() { _ = rdb.Close() }()

	// ── Account module ──────────────────────────────────────────────
	keys, err := signingKeys(cfg)
	if err != nil {
		return err
	}
	issuer, err := auth.NewIssuer(keys, cfg.JWTIssuer, cfg.JWTAudience, cfg.AccessTokenTTL)
	if err != nil {
		return err
	}
	verifier, err := auth.NewVerifier(keys, cfg.JWTIssuer, cfg.JWTAudience)
	if err != nil {
		return err
	}

	adapter := accountrepo.NewAdapter(pool)

	refresh, err := auth.NewRefreshManager(adapter, cfg.RefreshTTL)
	if err != nil {
		return err
	}

	accountMod, err := account.New(account.Deps{
		Redis:           rdb,
		Issuer:          issuer,
		Verifier:        verifier,
		Refresh:         refresh,
		SnapshotFetcher: adapter,
		PermFetcher:     adapter,
		Users:           adapter,
		AuditStore:      adapter,
		APIUsers:        adapter,
		CacheTTL:        cfg.PermissionCacheTTL,
		AccessTTL:       cfg.AccessTokenTTL,
		RefreshTTL:      cfg.RefreshTTL,
		CookieDomain:    cfg.CookieDomain,
		CookieSecure:    cfg.CookieSecure,
		PostLoginURL:    cfg.PostLoginURL,
	})
	if err != nil {
		return fmt.Errorf("account module: %w", err)
	}

	// ── HTTP ────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true, // login form POSTs cross-subdomain and needs Set-Cookie honoured
		MaxAge:           300,
	}))

	r.Get("/healthz", healthz(pool, rdb))
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/healthz", healthz(pool, rdb))
		accountMod.MountHTTP(r)
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.APIPort),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info().Str("addr", srv.Addr).Str("env", cfg.AppEnv).Msg("api listening")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("server error")
			stop()
		}
	}()

	<-sigCtx.Done()
	log.Info().Msg("api shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func signingKeys(cfg *config.Config) ([]auth.SigningKey, error) {
	parsed, err := cfg.ParsedKeys()
	if err != nil {
		return nil, fmt.Errorf("jwt keys: %w", err)
	}
	keys := make([]auth.SigningKey, 0, len(parsed))
	for _, k := range parsed {
		keys = append(keys, auth.SigningKey{ID: k.ID, Secret: k.Secret})
	}
	return keys, nil
}

func healthz(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbOK := pool.Ping(r.Context()) == nil
		cacheOK := rdb.Ping(r.Context()).Err() == nil
		w.Header().Set("Content-Type", "application/json")
		if !dbOK || !cacheOK {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_, _ = fmt.Fprintf(w, `{"status":"ok","db":%t,"cache":%t}`, dbOK, cacheOK)
	}
}
