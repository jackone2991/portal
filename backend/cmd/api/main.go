// Command api runs the Portal HTTP server.
//
// v1 scope: see doc/en/architecture/01-v1-scope-cut.md. Only the account module
// is mounted; other modules attach under r.Route("/api/v1", ...) the same way.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/portal/backend/internal/modules/account"
	"github.com/portal/backend/internal/modules/account/auth"
	accountmw "github.com/portal/backend/internal/modules/account/middleware"
	accountrepo "github.com/portal/backend/internal/modules/account/repository"
	"github.com/portal/backend/internal/modules/bank"
	bankrepo "github.com/portal/backend/internal/modules/bank/repository"
	"github.com/portal/backend/internal/modules/comic"
	comicrepo "github.com/portal/backend/internal/modules/comic/repository"
	"github.com/portal/backend/internal/modules/journal"
	journalrepo "github.com/portal/backend/internal/modules/journal/repository"
	"github.com/portal/backend/internal/modules/media"
	mediaapi "github.com/portal/backend/internal/modules/media/api"
	mediarepo "github.com/portal/backend/internal/modules/media/repository"
	"github.com/portal/backend/internal/modules/notify"
	notifyapi "github.com/portal/backend/internal/modules/notify/api"
	notifyrepo "github.com/portal/backend/internal/modules/notify/repository"
	"github.com/portal/backend/internal/modules/ops"
	opsrepo "github.com/portal/backend/internal/modules/ops/repository"
	"github.com/portal/backend/internal/modules/people"
	peoplerepo "github.com/portal/backend/internal/modules/people/repository"
	"github.com/portal/backend/internal/platform/config"
	"github.com/portal/backend/internal/platform/events"
	"github.com/portal/backend/internal/platform/storage"
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

	// Asynq client — shared by the notify:dispatch enqueuer (account password
	// reset) and the media module. Created up front so the dispatch closure below
	// can capture it before account.New.
	asynqRedis, err := asynq.ParseRedisURI(cfg.AsynqRedisURL)
	if err != nil {
		return fmt.Errorf("asynq redis: %w", err)
	}
	asynqClient := asynq.NewClient(asynqRedis)
	defer func() { _ = asynqClient.Close() }()

	// dispatchNotify is the ONLY way a producer reaches the notification fan-out
	// (notify/api). Passed to account for password-reset emails.
	dispatchNotify := func(ctx context.Context, intent notifyapi.NotificationIntent) error {
		return notifyapi.Enqueue(ctx, asynqClient, intent)
	}

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

	resetMgr, err := auth.NewResetManager(adapter, cfg.PasswordResetTTL)
	if err != nil {
		return fmt.Errorf("reset manager: %w", err)
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

		ResetTokens:      resetMgr,
		Dispatch:         dispatchNotify,
		PasswordResetURL: cfg.PasswordResetURL,
	})
	if err != nil {
		return fmt.Errorf("account module: %w", err)
	}

	// ── Media module ────────────────────────────────────────────────
	store, err := storage.NewS3(storage.Config{
		Endpoint:     cfg.S3Endpoint,
		Region:       cfg.S3Region,
		Bucket:       cfg.S3Bucket,
		AccessKey:    cfg.S3AccessKey,
		SecretKey:    cfg.S3SecretKey,
		UsePathStyle: cfg.S3UsePathStyle,
	})
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}

	// media events publisher. No API-side subscriptions for SPEC-01 (media emits
	// media:asset_ready / media:asset_deleted; consumers land in later specs) —
	// publishing with no subscribers is a deliberate no-op.
	mediaEvents := events.NewPublisher(asynqClient)

	// DELETE /assets/{id} is gated by "owner OR assets:delete:any" — the extractor
	// resolves the asset's owner via the media module (built below; the closure
	// captures the var so it is set by the time a request arrives).
	var mediaMod *media.Module
	extractAssetOwner := func(r *http.Request) (uuid.UUID, error) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			return uuid.Nil, err
		}
		return mediaMod.AssetOwner(r.Context(), id)
	}
	deleteMW := accountmw.RequireOwnerOrPermission(accountMod.Engine(), "assets:delete:any", extractAssetOwner)

	mediaMod, err = media.New(media.Deps{
		Store:            store,
		Repo:             mediarepo.NewAdapter(pool),
		Enqueuer:         asynqClient,
		Events:           mediaEvents,
		RequireAuth:      accountmw.RequireAuth(verifier, adapter),
		DeleteMiddleware: deleteMW,
		CurrentUser: func(ctx context.Context) (uuid.UUID, bool) {
			id, ok := auth.FromContext(ctx)
			if !ok || id.IsAnonymous() {
				return uuid.Nil, false
			}
			return id.UserID, true
		},
		PublicBase: cfg.APIBaseURL,
		UploadTTL:  15 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("media module: %w", err)
	}

	// ── Notify module (store read API: /me/notifications) ────────────
	// The API server serves the bell's read/mark-read routes; the delivery
	// fan-out (dispatch/email/consumer) runs in cmd/worker.
	notifyMod, err := notify.New(notify.Deps{
		Repo:        notifyrepo.NewAdapter(pool),
		RequireAuth: accountmw.RequireAuth(verifier, adapter),
		RequirePermission: func(code string) func(http.Handler) http.Handler {
			return accountmw.RequirePermission(accountMod.Engine(), code)
		},
		CurrentUser: func(ctx context.Context) (uuid.UUID, bool) {
			id, ok := auth.FromContext(ctx)
			if !ok || id.IsAnonymous() {
				return uuid.Nil, false
			}
			return id.UserID, true
		},
	})
	if err != nil {
		return fmt.Errorf("notify module: %w", err)
	}

	// ── Journal module (life-stream write path: /journal/entries) ────
	journalMod, err := journal.New(journal.Deps{
		Repo:        journalrepo.NewAdapter(pool),
		Events:      mediaEvents, // shared fan-out publisher; journal:entry_created is emit-only
		RequireAuth: accountmw.RequireAuth(verifier, adapter),
		RequirePermission: func(code string) func(http.Handler) http.Handler {
			return accountmw.RequirePermission(accountMod.Engine(), code)
		},
		CurrentUser: func(ctx context.Context) (uuid.UUID, bool) {
			id, ok := auth.FromContext(ctx)
			if !ok || id.IsAnonymous() {
				return uuid.Nil, false
			}
			return id.UserID, true
		},
	})
	if err != nil {
		return fmt.Errorf("journal module: %w", err)
	}

	// ── Ops module (freshness sentinel: /ops/status) ────────────────
	// The API server serves only the read side; the nightly backup task runs in
	// cmd/worker. ops:read is admin-tier (seeded + granted to admin in 0012).
	opsMod, err := ops.New(ops.Deps{
		Repo:        opsrepo.NewAdapter(pool),
		RequireAuth: accountmw.RequireAuth(verifier, adapter),
		RequirePermission: func(code string) func(http.Handler) http.Handler {
			return accountmw.RequirePermission(accountMod.Engine(), code)
		},
	})
	if err != nil {
		return fmt.Errorf("ops module: %w", err)
	}

	// ── Bank module (personal ledger: /bank/*) ──────────────────────
	bankMod, err := bank.New(bank.Deps{
		Repo:        bankrepo.NewAdapter(pool),
		Events:      mediaEvents, // shared fan-out publisher; bank:transaction_* is emit-only
		RequireAuth: accountmw.RequireAuth(verifier, adapter),
		RequirePermission: func(code string) func(http.Handler) http.Handler {
			return accountmw.RequirePermission(accountMod.Engine(), code)
		},
		CurrentUser: func(ctx context.Context) (uuid.UUID, bool) {
			id, ok := auth.FromContext(ctx)
			if !ok || id.IsAnonymous() {
				return uuid.Nil, false
			}
			return id.UserID, true
		},
	})
	if err != nil {
		return fmt.Errorf("bank module: %w", err)
	}

	// ── Comic module (SPEC-02 reference vertical: /comics, /chapters, /pages) ──
	// Owner-or-elevated mutations: the middlewares resolve the comic owner from
	// the URL id via the module's extractors (comic must not import account/rbac).
	engine := accountMod.Engine()
	var comicMod *comic.Module
	comicOwnerExtractor := func(resolve func(context.Context, uuid.UUID) (uuid.UUID, error)) func(*http.Request) (uuid.UUID, error) {
		return func(r *http.Request) (uuid.UUID, error) {
			id, err := uuid.Parse(chi.URLParam(r, "id"))
			if err != nil {
				return uuid.Nil, err
			}
			return resolve(r.Context(), id)
		}
	}
	byComic := comicOwnerExtractor(func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) { return comicMod.OwnerByComic(ctx, id) })
	byChapter := comicOwnerExtractor(func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) { return comicMod.OwnerByChapter(ctx, id) })
	byPage := comicOwnerExtractor(func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) { return comicMod.OwnerByPage(ctx, id) })
	comicMod, err = comic.New(comic.Deps{
		Repo:        comicrepo.NewAdapter(pool),
		Media:       mediaMod.API(),
		Events:      mediaEvents,
		RequireAuth: accountmw.RequireAuth(verifier, adapter),
		RequirePermission: func(code string) func(http.Handler) http.Handler {
			return accountmw.RequirePermission(engine, code)
		},
		CurrentUser: func(ctx context.Context) (uuid.UUID, bool) {
			id, ok := auth.FromContext(ctx)
			if !ok || id.IsAnonymous() {
				return uuid.Nil, false
			}
			return id.UserID, true
		},
		WriteComicMW:   accountmw.RequireOwnerOrPermission(engine, "comics:write:any", byComic),
		WriteChapterMW: accountmw.RequireOwnerOrPermission(engine, "comics:write:any", byChapter),
		DeleteComicMW:  accountmw.RequireOwnerOrPermission(engine, "comics:delete:any", byComic),
		DeletePageMW:   accountmw.RequireOwnerOrPermission(engine, "comics:delete:any", byPage),
		PublishMW:      accountmw.RequireOwnerOrPermission(engine, "comics:publish:any", byComic),
	})
	if err != nil {
		return fmt.Errorf("comic module: %w", err)
	}

	// ── People module (SPEC-08: /people, contacts + birthdays) ──────
	peopleMod, err := people.New(people.Deps{
		Repo:        peoplerepo.NewAdapter(pool),
		Events:      mediaEvents,
		RequireAuth: accountmw.RequireAuth(verifier, adapter),
		RequirePermission: func(code string) func(http.Handler) http.Handler {
			return accountmw.RequirePermission(engine, code)
		},
		CurrentUser: func(ctx context.Context) (uuid.UUID, bool) {
			id, ok := auth.FromContext(ctx)
			if !ok || id.IsAnonymous() {
				return uuid.Nil, false
			}
			return id.UserID, true
		},
	})
	if err != nil {
		return fmt.Errorf("people module: %w", err)
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

		// Aggregator routes
		r.With(accountmw.RequireAuth(verifier, adapter)).Get("/continue", handleContinue(mediaMod))

		accountMod.MountHTTP(r)
		mediaMod.MountHTTP(r)
		notifyMod.MountHTTP(r)
		journalMod.MountHTTP(r)
		opsMod.MountHTTP(r)
		bankMod.MountHTTP(r)
		comicMod.MountHTTP(r)
		peopleMod.MountHTTP(r)
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

func handleContinue(mediaMod *media.Module) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := auth.FromContext(r.Context())
		if !ok || uid.IsAnonymous() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"code":"unauthorized","message":"authentication required"}`))
			return
		}

		limit := 10
		if n := r.URL.Query().Get("limit"); n != "" {
			if v, err := strconv.Atoi(n); err == nil && v > 0 {
				limit = v
			}
		}
		if limit > 50 {
			limit = 50
		}

		// Phase 1: Only media items. Future specs will append story/comic/music items here.
		items, err := mediaMod.API().Continue(r.Context(), uid.UserID, limit)
		if err != nil {
			log.Error().Err(err).Msg("continue aggregator: media module error")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"code":"internal","message":"could not fetch continue items"}`))
			return
		}

		if items == nil {
			items = []mediaapi.ContinueItem{}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		b, _ := json.Marshal(map[string]any{"items": items})
		w.Write(b)
	}
}
