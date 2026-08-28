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
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/hibiken/asynqmon"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/portal/backend/internal/modules/account"
	"github.com/portal/backend/internal/modules/account/auth"
	"github.com/portal/backend/internal/modules/account/handler"
	accountmw "github.com/portal/backend/internal/modules/account/middleware"
	accountrepo "github.com/portal/backend/internal/modules/account/repository"
	"github.com/portal/backend/internal/modules/bank"
	bankapi "github.com/portal/backend/internal/modules/bank/api"
	bankrepo "github.com/portal/backend/internal/modules/bank/repository"
	"github.com/portal/backend/internal/modules/comic"
	comicapi "github.com/portal/backend/internal/modules/comic/api"
	comicrepo "github.com/portal/backend/internal/modules/comic/repository"
	"github.com/portal/backend/internal/modules/journal"
	journalapi "github.com/portal/backend/internal/modules/journal/api"
	journalrepo "github.com/portal/backend/internal/modules/journal/repository"
	"github.com/portal/backend/internal/modules/layout"
	layoutrepo "github.com/portal/backend/internal/modules/layout/repository"
	"github.com/portal/backend/internal/modules/media"
	mediaapi "github.com/portal/backend/internal/modules/media/api"
	mediarepo "github.com/portal/backend/internal/modules/media/repository"
	"github.com/portal/backend/internal/modules/movie"
	movieapi "github.com/portal/backend/internal/modules/movie/api"
	movierepo "github.com/portal/backend/internal/modules/movie/repository"
	"github.com/portal/backend/internal/modules/music"
	musicapi "github.com/portal/backend/internal/modules/music/api"
	musicrepo "github.com/portal/backend/internal/modules/music/repository"
	"github.com/portal/backend/internal/modules/notify"
	notifyapi "github.com/portal/backend/internal/modules/notify/api"
	notifyrepo "github.com/portal/backend/internal/modules/notify/repository"
	"github.com/portal/backend/internal/modules/ops"
	opsrepo "github.com/portal/backend/internal/modules/ops/repository"
	"github.com/portal/backend/internal/modules/people"
	peoplerepo "github.com/portal/backend/internal/modules/people/repository"
	"github.com/portal/backend/internal/modules/social"
	socialapi "github.com/portal/backend/internal/modules/social/api"
	socialrepo "github.com/portal/backend/internal/modules/social/repository"
	"github.com/portal/backend/internal/modules/story"
	storyapi "github.com/portal/backend/internal/modules/story/api"
	storyrepo "github.com/portal/backend/internal/modules/story/repository"
	"github.com/portal/backend/internal/modules/tenant"
	tenantrepo "github.com/portal/backend/internal/modules/tenant/repository"
	"github.com/portal/backend/internal/platform/config"
	platformdb "github.com/portal/backend/internal/platform/db"
	"github.com/portal/backend/internal/platform/events"
	"github.com/portal/backend/internal/platform/server"
	"github.com/portal/backend/internal/platform/storage"
)

// requestWindow bounds any single request. It is deliberately generous because
// large API-proxied uploads (import zips up to 16 GB) stream + S3-Put on the request
// context; a tight cap cancels a valid upload. The window has to cover the whole
// browser→API→MinIO leg: 15 min needed a sustained ~5 MB/s to land a 4 GB archive,
// which a phone on the LAN does not have. ReadHeaderTimeout (5s) still guards
// the header phase and Traefik enforces edge timeouts, so this only removes the
// pathological "runaway handler" ceiling — acceptable for a single-tenant deploy.
const requestWindow = 45 * time.Minute

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
	pool, err := platformdb.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("db ping: %w", err)
	}
	// tdb wraps the pool with tenant-scoping helpers; conn is the context-aware
	// DBTX handed to every repository (runs on the request tenant tx when one is
	// open via RequireTenant, else the pool — identical behavior when no tx set).
	tdb := platformdb.New(pool)
	conn := tdb.Conn()

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

	adapter := accountrepo.NewAdapter(conn)

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
		Admin:           adapter,
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
		ApprovalQueueURL: cfg.ApprovalQueueURL,
	})
	if err != nil {
		return fmt.Errorf("account module: %w", err)
	}

	// Superadmin bootstrap. Migration 0031 leaves an EXISTING install with no
	// superadmin at all — its backfill approves everyone already present but
	// promotes nobody, because guessing which of them should hold unrestricted
	// access is not the migration's call. Registration's first-run rule only
	// covers a virgin database. So an operator names the account here, and every
	// startup re-asserts it: idempotent, greppable in the log, and it also
	// re-approves the account if somebody managed to lock it out.
	if err := bootstrapSuperadmin(ctx, adapter, cfg.BootstrapSuperadminEmail); err != nil {
		log.Error().Err(err).Msg("superadmin bootstrap failed; continuing")
	}

	// ── Tenant module + tenant-scoping middleware (ADR-07 Phase 1) ──
	// requireAuth is the base auth middleware; authTenant composes it with
	// RequireTenant so every DOMAIN request resolves the caller's personal org and
	// runs inside a tenant-scoped tx (Increment 2). Account + queue-console routes
	// (global data only) keep plain requireAuth — they touch no tenant-scoped table.
	requireAuth := accountmw.RequireAuth(verifier, adapter)
	tenantCurrentUser := func(ctx context.Context) (uuid.UUID, string, bool) {
		id, ok := auth.FromContext(ctx)
		if !ok || id.IsAnonymous() {
			return uuid.Nil, "", false
		}
		return id.UserID, id.DisplayName, true
	}
	tenantMod, err := tenant.New(tenant.Deps{
		DB:          tdb,
		Store:       tenantrepo.NewAdapter(conn),
		RequireAuth: requireAuth,
		CurrentUser: tenantCurrentUser,
	})
	if err != nil {
		return fmt.Errorf("tenant module: %w", err)
	}
	// authTenant = auth (sets identity) → RequireTenant (opens personal-org tx).
	// Domain modules receive this in their RequireAuth slot — no module changes.
	authTenant := func(next http.Handler) http.Handler {
		return requireAuth(tenantMod.RequireTenant()(next))
	}
	// optionalAuthTenant is the same pair, neither of which rejects an anonymous
	// caller: identity is attached when the request carries one, and only then is
	// a tenant scope opened. The media variant/HLS routes use it so a signed-in
	// caller sees their own media while everyone else still gets the assets
	// marked public — the filtering itself is the 0032 policies' job, not the
	// middleware's.
	optionalAuth := accountmw.OptionalAuth(verifier, adapter)
	optionalAuthTenant := func(next http.Handler) http.Handler {
		return optionalAuth(tenantMod.OptionalTenant()(next))
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

	// Event fan-out publisher. The wiring contract (platform/events doc) requires
	// EVERY binary that EMITS an event to register that event's consumer edges on
	// its OWN publisher — Publish looks up subscribers on the local table, so an
	// unregistered edge here silently enqueues nothing. cmd/worker registers the
	// same set; Subscribe is idempotent and no event is emitted by both binaries,
	// so double-wiring can't duplicate delivery. Only edges for API-emitted events
	// belong here (media:asset_ready + people:birthday_upcoming are worker-emitted).
	mediaEvents := events.NewPublisher(asynqClient)
	// media:asset_deleted (DELETE /assets/{id}) → comic reap + stream removal (2 consumers).
	mediaEvents.Subscribe(media.EventAssetDeleted, comicapi.TaskOnAssetDeleted, asynq.Queue("default"))
	mediaEvents.Subscribe(media.EventAssetDeleted, movieapi.TaskOnAssetDeleted, asynq.Queue("default"))
	mediaEvents.Subscribe(media.EventAssetDeleted, musicapi.TaskOnAssetDeleted, asynq.Queue("default"))
	mediaEvents.Subscribe(media.EventAssetDeleted, storyapi.TaskOnAssetDeleted, asynq.Queue("default"))

	// Catalogue verticals → life stream. Emitted since the verticals landed but
	// unsubscribed until 2026-08-25: publishing a movie produced no card while
	// publishing a comic chapter did.
	mediaEvents.Subscribe(movieapi.EventMoviePublished, journalapi.TaskStreamMoviePublished, asynq.Queue("default"))
	mediaEvents.Subscribe(musicapi.EventTrackPublished, journalapi.TaskStreamTrackPublished, asynq.Queue("default"))
	mediaEvents.Subscribe(storyapi.EventStoryPublished, journalapi.TaskStreamStoryPublished, asynq.Queue("default"))
	mediaEvents.Subscribe(media.EventAssetDeleted, journalapi.TaskStreamAssetDeleted, asynq.Queue("default"))
	// media:playback_completed (progress→100%) → stream projection.
	mediaEvents.Subscribe("media:playback_completed", journalapi.TaskStreamPlaybackCompleted, asynq.Queue("default"))
	// bank:transaction_* → life-stream projection (SPEC-06 P0.1b).
	mediaEvents.Subscribe(bankapi.EventTransactionCreated, journalapi.TaskStreamBankCreated, asynq.Queue("default"))
	mediaEvents.Subscribe(bankapi.EventTransactionUpdated, journalapi.TaskStreamBankUpdated, asynq.Queue("default"))
	mediaEvents.Subscribe(bankapi.EventTransactionDeleted, journalapi.TaskStreamBankDeleted, asynq.Queue("default"))
	// comic:published → one bell notification per publish. Chapters are no longer
	// projected into the life-stream: a publish emits one event per chapter, so a
	// long-running title buried the feed under its own table of contents.
	mediaEvents.Subscribe(comicapi.EventComicPublished, notifyapi.TaskOnComicPublished, asynq.Queue("default"))
	// social:connection_* → the bell. Nothing is projected into the life-stream:
	// a connection is an event between two people, not a card in either's day.
	mediaEvents.Subscribe(socialapi.EventConnectionRequested, notifyapi.TaskOnConnectionRequested, asynq.Queue("default"))
	mediaEvents.Subscribe(socialapi.EventConnectionAccepted, notifyapi.TaskOnConnectionAccepted, asynq.Queue("default"))

	// ownerExtractor builds the {id}-keyed owner lookups every owner-or-elevated
	// guard runs on. notFound is the owning module's "no such row" sentinel, and it
	// is the ONLY error that becomes a 404 — anything else is the lookup itself
	// failing and must reach the client as a 500 the server also logs, instead of a
	// "not found" that reads as final (see accountmw.ErrOwnerNotFound).
	ownerExtractor := func(notFound error, resolve func(context.Context, uuid.UUID) (uuid.UUID, error)) accountmw.OwnerExtractor {
		return func(r *http.Request) (uuid.UUID, error) {
			id, err := uuid.Parse(chi.URLParam(r, "id"))
			if err != nil {
				return uuid.Nil, accountmw.ErrOwnerNotFound // an unparseable id names nothing
			}
			owner, rerr := resolve(r.Context(), id)
			if rerr != nil {
				if errors.Is(rerr, notFound) {
					return uuid.Nil, accountmw.ErrOwnerNotFound
				}
				return uuid.Nil, rerr
			}
			return owner, nil
		}
	}

	// DELETE /assets/{id} is gated by "owner OR assets:delete:any" — the extractor
	// resolves the asset's owner via the media module (built below; the closure
	// captures the var so it is set by the time a request arrives).
	var mediaMod *media.Module
	extractAssetOwner := ownerExtractor(media.ErrNotFound, func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
		return mediaMod.AssetOwner(ctx, id)
	})
	deleteMW := accountmw.RequireOwnerOrPermission(accountMod.Engine(), "assets:delete:any", extractAssetOwner)

	mediaMod, err = media.New(media.Deps{
		Store:            store,
		Repo:             mediarepo.NewAdapter(conn),
		Enqueuer:         asynqClient,
		Events:           mediaEvents,
		RequireAuth:      authTenant,
		OptionalAuth:     optionalAuthTenant,
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
		Repo:        notifyrepo.NewAdapter(conn),
		RequireAuth: authTenant,
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
		Repo:        journalrepo.NewAdapter(conn, tdb.RunInTx),
		Events:      mediaEvents, // shared fan-out publisher; journal:entry_created is emit-only
		RequireAuth: authTenant,
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
		Repo:        opsrepo.NewAdapter(conn),
		RequireAuth: authTenant,
		RequirePermission: func(code string) func(http.Handler) http.Handler {
			return accountmw.RequirePermission(accountMod.Engine(), code)
		},
	})
	if err != nil {
		return fmt.Errorf("ops module: %w", err)
	}

	// ── Layout module (shell navigation + dashboard widgets) ────────
	// Global config, NOT tenant-scoped, so it takes plain requireAuth rather than
	// authTenant — there is no per-tenant row to fence. It gets the raw pool
	// because its whole-set saves run in their own transaction.
	//
	// Perms is account's public API: /layout filters per caller server-side, so
	// an admin-only entry never reaches a bundle that anyone can read.
	layoutMod, err := layout.New(layout.Deps{
		Repo:        layoutrepo.NewAdapter(pool),
		Perms:       accountMod.API(),
		Audit:       accountMod.Logger(),
		RequireAuth: requireAuth,
		RequirePermission: func(code string) func(http.Handler) http.Handler {
			return accountmw.RequirePermission(accountMod.Engine(), code)
		},
	})
	if err != nil {
		return fmt.Errorf("layout module: %w", err)
	}

	// ── Bank module (personal ledger: /bank/*) ──────────────────────
	bankMod, err := bank.New(bank.Deps{
		Repo:        bankrepo.NewAdapter(conn, tdb.RunInTx),
		Events:      mediaEvents, // shared fan-out publisher; bank:transaction_* is emit-only
		RequireAuth: authTenant,
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
	byComic := ownerExtractor(comic.ErrNotFound, func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) { return comicMod.OwnerByComic(ctx, id) })
	byChapter := ownerExtractor(comic.ErrNotFound, func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) { return comicMod.OwnerByChapter(ctx, id) })
	byPage := ownerExtractor(comic.ErrNotFound, func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) { return comicMod.OwnerByPage(ctx, id) })
	// P1.8 external-source sync: enable only when the Python scraper service URL is
	// configured. COMIC_SYNC_SECRET guards the scraper→api callbacks.
	var comicScraper comic.ScraperClient
	if u := os.Getenv("COMIC_SCRAPER_URL"); u != "" {
		comicScraper = newHTTPScraper(u)
	}
	// runInUserTenant scopes an INSERT into a tenant-scoped table to a user's personal
	// org (like cmd/worker) — needed because the scraper→api sync-batch endpoint runs
	// outside authTenant, but comic_imports.tenant_id defaults from app.current_tenant.
	tenantStore := tenantrepo.NewAdapter(conn)
	runInUserTenant := func(ctx context.Context, userID uuid.UUID, fn func(context.Context) error) error {
		org, oerr := tenantStore.PersonalOrg(ctx, userID)
		if oerr != nil {
			return oerr
		}
		if org == nil {
			return fmt.Errorf("api: no personal org for user %s", userID)
		}
		// The user is known here, so scope to them rather than to the whole
		// tenant: this path writes on their behalf and must not gain tenant-wide
		// media access it never needs (0032).
		tx, terr := tdb.BeginScope(ctx, platformdb.Scope{OrgID: org.ID, UserID: userID})
		if terr != nil {
			return terr
		}
		ctx = platformdb.WithTx(ctx, tx)
		if ferr := fn(ctx); ferr != nil {
			_ = tx.Rollback(ctx)
			return ferr
		}
		return tx.Commit(ctx)
	}
	comicMod, err = comic.New(comic.Deps{
		Repo:           comicrepo.NewAdapter(conn, tdb.RunInTx),
		RunInTenant:    runInUserTenant,
		Media:          mediaMod.API(),
		Events:         mediaEvents,
		Storage:        store,       // P1.7: store the uploaded chapter zip (import/ prefix)
		Enqueuer:       asynqClient, // P1.7: enqueue comic:import_zip
		Scraper:        comicScraper,
		InternalSecret: os.Getenv("COMIC_SYNC_SECRET"),
		// SSRF guard: restrict which hosts a sync source may target. Empty allows
		// any public host; internal/private addresses are rejected either way.
		SourceAllowlist: comic.ParseSourceAllowlist(os.Getenv("COMIC_SOURCE_ALLOWLIST")),
		RequireAuth:     authTenant,
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
	// ── Social module (connections between accounts, 0037) ──────────
	socialMod, err := social.New(social.Deps{
		Repo:        socialrepo.NewAdapter(conn),
		Events:      mediaEvents,
		RequireAuth: authTenant,
		RequirePermission: func(code string) func(http.Handler) http.Handler {
			return accountmw.RequirePermission(engine, code)
		},
		// social stores ids and resolves names through account's api/ package —
		// it may not join `users` (MODULES.md).
		Names: func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
			return accountMod.API().GetUserNames(ctx, ids)
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
		return fmt.Errorf("social module: %w", err)
	}

	peopleMod, err := people.New(people.Deps{
		Repo:        peoplerepo.NewAdapter(conn),
		Events:      mediaEvents,
		RequireAuth: authTenant,
		// "People you may know" reads the account roster through account's api/
		// package — people never touches the users table itself.
		Connected: socialMod.API().CounterpartIDs,
		Directory: func(ctx context.Context, exclude uuid.UUID, limit int) ([]people.DirectoryUser, error) {
			roster, derr := accountMod.API().ListDirectory(ctx, exclude, limit)
			if derr != nil {
				return nil, derr
			}
			out := make([]people.DirectoryUser, 0, len(roster))
			for _, u := range roster {
				out = append(out, people.DirectoryUser{ID: u.ID, DisplayName: u.DisplayName})
			}
			return out, nil
		},
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

	// ── Movie module (first domain vertical: /movies) ───────────────
	// Owner-or-elevated mutations resolve the movie owner from the URL id via the
	// module's extractor (movie must not import account/rbac). Tenant-wrapped via
	// authTenant like the other domain modules.
	var movieMod *movie.Module
	byMovie := ownerExtractor(movie.ErrNotFound, func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) { return movieMod.OwnerByMovie(ctx, id) })
	movieMod, err = movie.New(movie.Deps{
		Repo:        movierepo.NewAdapter(conn),
		Media:       mediaMod.API(),
		Events:      mediaEvents,
		RequireAuth: authTenant,
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
		WriteMovieMW:  accountmw.RequireOwnerOrPermission(engine, "movies:write:any", byMovie),
		DeleteMovieMW: accountmw.RequireOwnerOrPermission(engine, "movies:delete:any", byMovie),
		PublishMW:     accountmw.RequireOwnerOrPermission(engine, "movies:publish:any", byMovie),
	})
	if err != nil {
		return fmt.Errorf("movie module: %w", err)
	}

	// ── Music module (tracks over media audio: /tracks) ─────────────
	var musicMod *music.Module
	byTrack := ownerExtractor(music.ErrNotFound, func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) { return musicMod.OwnerByTrack(ctx, id) })
	musicMod, err = music.New(music.Deps{
		Repo:   musicrepo.NewAdapter(conn),
		Media:  mediaMod.API(),
		Events: mediaEvents,
		// Bulk zip import (0038): the API stores the archive and enqueues; the
		// unpacking happens in cmd/worker, which is where ffprobe and the tenant
		// scoping live.
		Storage:     store,
		Enqueuer:    asynqClient,
		RequireAuth: authTenant,
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
		WriteTrackMW:  accountmw.RequireOwnerOrPermission(engine, "music:write:any", byTrack),
		DeleteTrackMW: accountmw.RequireOwnerOrPermission(engine, "music:delete:any", byTrack),
		PublishMW:     accountmw.RequireOwnerOrPermission(engine, "music:publish:any", byTrack),
	})
	if err != nil {
		return fmt.Errorf("music module: %w", err)
	}

	// ── Story module (long-form text over media covers: /stories) ───
	// Chapters reorder in a tenant tx (tdb.RunInTx), like comic. Two owner
	// extractors resolve story vs. chapter ids for the owner-or-elevated guards.
	var storyMod *story.Module
	byStory := ownerExtractor(story.ErrNotFound, func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) { return storyMod.OwnerByStory(ctx, id) })
	byStoryChapter := ownerExtractor(story.ErrNotFound, func(ctx context.Context, id uuid.UUID) (uuid.UUID, error) { return storyMod.OwnerByChapter(ctx, id) })
	storyMod, err = story.New(story.Deps{
		Repo:        storyrepo.NewAdapter(conn, tdb.RunInTx),
		Media:       mediaMod.API(),
		Events:      mediaEvents,
		RequireAuth: authTenant,
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
		WriteStoryMW:   accountmw.RequireOwnerOrPermission(engine, "stories:write:any", byStory),
		WriteChapterMW: accountmw.RequireOwnerOrPermission(engine, "stories:write:any", byStoryChapter),
		DeleteStoryMW:  accountmw.RequireOwnerOrPermission(engine, "stories:delete:any", byStory),
		PublishMW:      accountmw.RequireOwnerOrPermission(engine, "stories:publish:any", byStory),
	})
	if err != nil {
		return fmt.Errorf("story module: %w", err)
	}

	// ── HTTP ────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	// Generous per-request window: API-proxied uploads (comic import zips up to 16 GB,
	// media sources) legitimately run well past a tight 30s cap — the S3 PutObject
	// runs on the request context, so a short handler timeout cancels a valid upload
	// (worse against slow dev MinIO). ReadHeaderTimeout still guards the slowloris
	// header phase and IdleTimeout bounds idle keep-alives; this is a personal,
	// single-tenant deployment behind Traefik, which enforces its own edge timeouts.
	r.Use(chimw.Timeout(requestWindow))
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
		r.Get("/time", handleServerTime(cfg.AppTimezone)) // app clock + display tz — one config for all dates

		// Aggregator routes
		r.With(authTenant).Get("/continue", handleContinue(mediaMod))

		accountMod.MountHTTP(r)
		tenantMod.MountHTTP(r)
		mediaMod.MountHTTP(r)
		notifyMod.MountHTTP(r)
		journalMod.MountHTTP(r)
		opsMod.MountHTTP(r)
		layoutMod.MountHTTP(r)
		bankMod.MountHTTP(r)
		comicMod.MountHTTP(r)
		peopleMod.MountHTTP(r)
		socialMod.MountHTTP(r)
		movieMod.MountHTTP(r)
		musicMod.MountHTTP(r)
		storyMod.MountHTTP(r)
	})

	// Queue console (SPEC-09 P1.6): the asynqmon SPA at /admin/queues, admin-gated
	// (queues:read, seeded to admin in 0012). It is NOT under /api/v1 and NOT
	// OpenAPI. RootPath must equal the mount path so the SPA's asset/API URLs
	// resolve; r.Handle (unlike r.Mount) leaves the path unstripped, which is what
	// asynqmon's internal router expects. Same-origin, so the portal_access cookie
	// authenticates the browser's XHRs. Two handles cover the exact root + assets.
	queueMon := asynqmon.New(asynqmon.Options{RootPath: "/admin/queues", RedisConnOpt: asynqRedis})
	r.Group(func(r chi.Router) {
		r.Use(accountmw.RequireAuth(verifier, adapter))
		r.Use(accountmw.RequirePermission(engine, "queues:read"))
		r.Handle("/admin/queues", queueMon)
		r.Handle("/admin/queues/*", queueMon)
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.APIPort),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       requestWindow, // large uploads: read the multi-GB body
		WriteTimeout:      requestWindow, // covers body-read + S3 Put + response
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

// handleServerTime serves the app clock + display timezone (APP_TIMEZONE) so the
// frontend can render every date from one config (frontend lib/time.ts). Public —
// neither the clock nor the tz is sensitive. Timestamps are UTC; tz is a display
// hint the browser applies via Intl.
func handleServerTime(tz string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"now":      time.Now().UTC().Format(time.RFC3339),
			"timezone": tz,
		})
	}
}

func handleContinue(mediaMod *media.Module) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := auth.FromContext(r.Context())
		if !ok || uid.IsAnonymous() {
			server.Unauthorized(w)
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
			server.Internal(w)
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

// bootstrapSuperadmin makes sure the named account can actually administer the
// install: approved, enabled and holding `superadmin`.
//
// Every step is conditional. Re-asserting unconditionally would bump
// token_version on each restart, and that is the RBAC cache key — every API
// restart would flush the permission cache for the one account most likely to
// be mid-session. So it reads first and only writes what is missing, which also
// makes an unset or already-correct value completely silent.
func bootstrapSuperadmin(ctx context.Context, adapter *accountrepo.Adapter, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil
	}

	user, err := adapter.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, handler.ErrUserNotFound) {
			// Not an error: the operator may have set this before the account
			// exists. Say so loudly and move on — the next start will apply it.
			log.Warn().Str("email", email).
				Msg("BOOTSTRAP_SUPERADMIN_EMAIL names no account yet; register it, then restart the api")
			return nil
		}
		return err
	}

	roles, err := adapter.ListUserRoleCodes(ctx, user.ID)
	if err != nil {
		return err
	}
	hasRole := false
	for _, c := range roles {
		if c == handler.SuperadminRole {
			hasRole = true
			break
		}
	}

	if !hasRole {
		if err := adapter.AssignRoleByCode(ctx, user.ID, handler.SuperadminRole); err != nil {
			return err
		}
		log.Info().Str("email", email).Msg("bootstrap: granted superadmin")
	}
	if user.ApprovalStatus != handler.ApprovalApproved {
		if err := adapter.MarkApproved(ctx, user.ID, nil); err != nil {
			return err
		}
		log.Info().Str("email", email).Msg("bootstrap: approved account")
	}
	if !hasRole {
		// The grant changed their effective permissions; re-key the cache.
		if _, err := adapter.BumpUserTokenVersion(ctx, user.ID); err != nil {
			return err
		}
	}
	return nil
}
