// Command worker runs the Portal Asynq job consumers (FFmpeg transcode, image
// WebP variants, video posters) plus the shared periodic scheduler.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

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
	"github.com/portal/backend/internal/modules/media"
	mediarepo "github.com/portal/backend/internal/modules/media/repository"
	mediaworker "github.com/portal/backend/internal/modules/media/worker"
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
	opsapi "github.com/portal/backend/internal/modules/ops/api"
	opsrepo "github.com/portal/backend/internal/modules/ops/repository"
	"github.com/portal/backend/internal/modules/people"
	peopleapi "github.com/portal/backend/internal/modules/people/api"
	peoplerepo "github.com/portal/backend/internal/modules/people/repository"
	"github.com/portal/backend/internal/modules/story"
	storyapi "github.com/portal/backend/internal/modules/story/api"
	storyrepo "github.com/portal/backend/internal/modules/story/repository"
	tenantrepo "github.com/portal/backend/internal/modules/tenant/repository"
	"github.com/portal/backend/internal/platform/audit"
	"github.com/portal/backend/internal/platform/config"
	platformdb "github.com/portal/backend/internal/platform/db"
	"github.com/portal/backend/internal/platform/events"
	"github.com/portal/backend/internal/platform/storage"
)

// Periodic task types registered on the shared scheduler (see run()).
const taskPurgeRefreshTokens = "account:purge_refresh_tokens"

// heavyConcurrency serializes the memory-heavy decodes (transcode + image
// processing) so N large images + a transcode can't decode simultaneously and
// OOM a small VPS (SPEC-01 P0.1). Bump to 2 only on a box with ≥ 4 GB RAM.
const heavyConcurrency = 1

// envInt reads a positive int from an env var, falling back to def.
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// recipientResolver adapts the account repository's cross-module projection to
// notify.UserResolver (recipient email for the email channel). The worker does
// not construct the account module, so it wraps the repository adapter directly.
type recipientResolver struct{ repo *accountrepo.Adapter }

func (r recipientResolver) ResolveRecipient(ctx context.Context, userID uuid.UUID) (string, string, error) {
	u, err := r.repo.GetUserSummaryByID(ctx, userID)
	if err != nil {
		return "", "", err
	}
	if u == nil {
		return "", "", nil // absent/disabled → no recipient
	}
	return u.Email, u.DisplayName, nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("worker shutdown with error")
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

	// ── Infrastructure the pipeline needs ───────────────────────────
	pool, err := platformdb.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db pool: %w", err)
	}
	defer pool.Close()
	// tdb wraps the pool with tenant-scoping helpers; conn is the context-aware
	// DBTX handed to every repository. The worker's periodic sweeps run without a
	// tenant tx today (cross-tenant) — that moves to cmd/sysjobs in a later increment.
	tdb := platformdb.New(pool)
	conn := tdb.Conn()

	// runInUserTenant scopes a worker INSERT into a tenant-scoped table to the
	// target user's personal org (ADR-07 Increment 1b): resolve the org, open
	// BeginTenantScope so tenant_id's DEFAULT current_setting is populated. Wired
	// into the journal stream consumers below; the notify/media/people worker
	// inserts + scan_birthdays adopt the same closure when their 1b lands.
	tenantStore := tenantrepo.NewAdapter(conn)
	runInUserTenant := func(ctx context.Context, userID uuid.UUID, fn func(context.Context) error) error {
		org, err := tenantStore.PersonalOrg(ctx, userID)
		if err != nil {
			return err
		}
		if org == nil {
			return fmt.Errorf("worker: no personal org for user %s", userID)
		}
		tx, err := tdb.BeginTenantScope(ctx, org.ID)
		if err != nil {
			return err
		}
		ctx = platformdb.WithTx(ctx, tx)
		if err := fn(ctx); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		return tx.Commit(ctx)
	}

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

	redisOpt, err := asynq.ParseRedisURI(cfg.AsynqRedisURL)
	if err != nil {
		return err
	}

	// The worker both consumes and produces tasks/events: it enqueues the poster
	// follow-up after a transcode and emits media:asset_ready when an asset
	// reaches ready (no consumer required for v1 — a no-op fan-out).
	asynqClient := asynq.NewClient(redisOpt)
	defer func() { _ = asynqClient.Close() }()
	publisher := events.NewPublisher(asynqClient)

	mediaMod, err := media.New(media.Deps{
		Store:    store,
		Repo:     mediarepo.NewAdapter(conn),
		Enqueuer: asynqClient,
		Events:   publisher,
	})
	if err != nil {
		return fmt.Errorf("media module: %w", err)
	}
	acctRepo := accountrepo.NewAdapter(conn)

	// ── Notify module (delivery side) ───────────────────────────────
	// Cache/queue client for the global email send-ceiling counter.
	rdbOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("redis url: %w", err)
	}
	rdb := redis.NewClient(rdbOpt)
	defer func() { _ = rdb.Close() }()

	// Email transport: Mailpit/SMTP when configured, else a log sink (dev/no-SMTP).
	var emailSender notify.EmailSender = notify.LogSender{}
	if cfg.SMTPHost != "" {
		emailSender = notify.NewSMTPSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom)
	}

	notifyMod, err := notify.New(notify.Deps{
		Repo:            notifyrepo.NewAdapter(conn),
		Enqueuer:        asynqClient,
		Email:           emailSender,
		Users:           recipientResolver{repo: acctRepo},
		Redis:           rdb,
		EmailHourlyCap:  cfg.NotifyEmailHourlyCap,
		RunInUserTenant: runInUserTenant,
	})
	if err != nil {
		return fmt.Errorf("notify module: %w", err)
	}

	// ── Journal module (worker side) ────────────────────────────────
	// No P0 tasks — the wiring exists so SPEC-06's stream consumers attach here.
	journalMod, err := journal.New(journal.Deps{Repo: journalrepo.NewAdapter(conn, tdb.RunInTx), RunInUserTenant: runInUserTenant})
	if err != nil {
		return fmt.Errorf("journal module: %w", err)
	}

	// ── Bank module (worker side) ───────────────────────────────────
	// No P0 tasks — the wiring exists so SPEC-06's stream consumer of
	// bank:transaction_* attaches here without touching cmd/worker's shape.
	bankMod, err := bank.New(bank.Deps{Repo: bankrepo.NewAdapter(conn, tdb.RunInTx)})
	if err != nil {
		return fmt.Errorf("bank module: %w", err)
	}

	// ── Comic module (worker side: media:asset_deleted consumer, P0.6) ──
	comicMod, err := comic.New(comic.Deps{
		Repo:        comicrepo.NewAdapter(conn, tdb.RunInTx),
		Media:       mediaMod.API(),   // P1.7: IngestImage + GetAsset for the zip import
		Storage:     store,            // P1.7: read/delete the uploaded zip
		Enqueuer:    asynqClient,      // (unused on the worker side, but harmless)
		RunInTenant: runInUserTenant,  // P1.7: per-image committed tenant tx
	})
	if err != nil {
		return fmt.Errorf("comic module: %w", err)
	}

	// ── Movie module (worker side: media:asset_deleted consumer) ────
	movieMod, err := movie.New(movie.Deps{Repo: movierepo.NewAdapter(conn)})
	if err != nil {
		return fmt.Errorf("movie module: %w", err)
	}

	musicMod, err := music.New(music.Deps{Repo: musicrepo.NewAdapter(conn)})
	if err != nil {
		return fmt.Errorf("music module: %w", err)
	}

	// ── Story module (worker side: media:asset_deleted consumer) ────
	// Chapters reorder in a tx (tdb.RunInTx), like comic.
	storyMod, err := story.New(story.Deps{Repo: storyrepo.NewAdapter(conn, tdb.RunInTx)})
	if err != nil {
		return fmt.Errorf("story module: %w", err)
	}

	// ── People module (worker side: daily birthday scan, P0.4) ──────
	peopleMod, err := people.New(people.Deps{Repo: peoplerepo.NewAdapter(conn), Events: publisher, RunInUserTenant: runInUserTenant})
	if err != nil {
		return fmt.Errorf("people module: %w", err)
	}

	// ── Ops module (worker side: nightly Postgres backup) ───────────
	// The account module isn't constructed here, so audit records route through
	// its repository adapter (a plain audit_log INSERT). BackupDatabaseURL MUST
	// point at Postgres directly (5432), not PgBouncer (SPEC-09 P0.2).
	opsMod, err := ops.New(ops.Deps{
		Repo:              opsrepo.NewAdapter(conn),
		Store:             store,
		Events:            publisher,
		Audit:             audit.New(acctRepo),
		BackupDatabaseURL: cfg.BackupDatabaseURL,
	})
	if err != nil {
		return fmt.Errorf("ops module: %w", err)
	}

	// media:asset_ready → notify:on_asset_ready fan-out, registered on the SAME
	// publisher the media module emits through (events.md "Delivery mechanics"),
	// so the transcode's ready-transition actually reaches notify. The consumer
	// task lands on the weight-1 "default" queue served by the light server below.
	publisher.Subscribe(mediaworker.EventAssetReady, notifyapi.TaskOnAssetReady, asynq.Queue("default"))
	// media:asset_deleted → comic:on_asset_deleted (SPEC-02 P0.6): reap dangling
	// page/cover references when an asset is hard-deleted media-side.
	publisher.Subscribe(media.EventAssetDeleted, comicapi.TaskOnAssetDeleted, asynq.Queue("default"))
	publisher.Subscribe(media.EventAssetDeleted, movieapi.TaskOnAssetDeleted, asynq.Queue("default"))
	publisher.Subscribe(media.EventAssetDeleted, musicapi.TaskOnAssetDeleted, asynq.Queue("default"))
	publisher.Subscribe(media.EventAssetDeleted, storyapi.TaskOnAssetDeleted, asynq.Queue("default"))

	// Life-stream projection consumers (SPEC-06 P0.1b) — journal owns stream_items
	// and subscribes to every producer. media:asset_deleted now fans out to TWO
	// consumers (comic reap + stream removal): the platform/events multi-consumer path.
	publisher.Subscribe(mediaworker.EventAssetReady, journalapi.TaskStreamAssetReady, asynq.Queue("default"))
	publisher.Subscribe("media:playback_completed", journalapi.TaskStreamPlaybackCompleted, asynq.Queue("default"))
	publisher.Subscribe(media.EventAssetDeleted, journalapi.TaskStreamAssetDeleted, asynq.Queue("default"))
	publisher.Subscribe(bankapi.EventTransactionCreated, journalapi.TaskStreamBankCreated, asynq.Queue("default"))
	publisher.Subscribe(bankapi.EventTransactionUpdated, journalapi.TaskStreamBankUpdated, asynq.Queue("default"))
	publisher.Subscribe(bankapi.EventTransactionDeleted, journalapi.TaskStreamBankDeleted, asynq.Queue("default"))
	publisher.Subscribe(peopleapi.EventBirthdayUpcoming, journalapi.TaskStreamBirthday, asynq.Queue("default"))
	publisher.Subscribe(comicapi.EventChapterPublished, journalapi.TaskStreamComicPublished, asynq.Queue("default"))
	publisher.Subscribe(comicapi.EventChapterDeleted, journalapi.TaskStreamComicDeleted, asynq.Queue("default"))

	// ── Heavy server: serialize the expensive decodes (P0.1 OOM guard) ──
	// Its own low-concurrency pool consumes ONLY the "heavy" queue — queue
	// weights on a shared pool cannot cap parallelism, so the cap needs its own
	// server (rev 3).
	heavySrv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: heavyConcurrency,
		Queues:      map[string]int{"heavy": 1},
	})
	heavyMux := asynq.NewServeMux()
	mediaMod.RegisterHeavyTasks(heavyMux) // media:transcode (video, serialized)

	// ── Image server: WebP-variant processing, parallelized (IMAGE_CONCURRENCY,
	// default 3). Separate pool from "heavy" so batch comic imports transcode
	// images concurrently while video transcode stays serial (SPEC-01 P0.1). Image
	// decodes are dimension-capped, so a few in parallel stay within the OOM budget.
	imageConcurrency := envInt("IMAGE_CONCURRENCY", 3)
	imageSrv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: imageConcurrency,
		Queues:      map[string]int{"image": imageConcurrency},
	})
	imageMux := asynq.NewServeMux()
	mediaMod.RegisterImageTasks(imageMux) // media:process_image

	// ── Light server: cheap tasks (poster, janitor, account purge) ──────
	// Weighted so nothing here can starve, and it never touches the heavy queue.
	lightSrv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 4,
		Queues: map[string]int{
			"thumbnail": 3,
			"default":   1,
		},
	})
	lightMux := asynq.NewServeMux()
	mediaMod.RegisterLightTasks(lightMux) // media:thumbnail (poster)
	// notify delivery fan-out (SPEC-04): notify:dispatch / :email / :web_push /
	// :on_asset_ready — all light, IO-bound, weight-1 "default" queue.
	notifyMod.RegisterTasks(lightMux)
	journalMod.RegisterTasks(lightMux) // no-op at P0; wiring for SPEC-06 consumers
	bankMod.RegisterTasks(lightMux)    // no-op at P0; wiring for SPEC-06 consumers
	comicMod.RegisterTasks(lightMux)   // comic:on_asset_deleted (media:asset_deleted consumer, P0.6)
	movieMod.RegisterTasks(lightMux)   // movie:on_asset_deleted (media:asset_deleted consumer)
	musicMod.RegisterTasks(lightMux)   // music:on_asset_deleted (media:asset_deleted consumer)
	storyMod.RegisterTasks(lightMux)   // story:on_asset_deleted (media:asset_deleted consumer)
	peopleMod.RegisterTasks(lightMux)  // people:scan_birthdays (daily birthday scan, P0.4)
	opsMod.RegisterTasks(lightMux)     // ops:backup_database (nightly pg_dump → storage)
	lightMux.HandleFunc(notifyapi.TaskPurgeOld, func(ctx context.Context, _ *asynq.Task) error {
		return notifyMod.PurgeOld(ctx)
	})
	lightMux.HandleFunc(mediaworker.TaskTypePurgeOrphans, func(ctx context.Context, _ *asynq.Task) error {
		return mediaMod.PurgeOrphans(ctx)
	})
	// The account module isn't constructed in the worker (its request-path deps —
	// JWT issuer, RBAC — are irrelevant to a maintenance sweep), so the
	// composition root wires the repository purge directly.
	lightMux.HandleFunc(taskPurgeRefreshTokens, func(ctx context.Context, _ *asynq.Task) error {
		return acctRepo.PurgeExpiredRefreshTokens(ctx)
	})

	// ── Shared periodic scheduler (SPEC-01 P0.3, F002) ──────────────────
	// THE single registration point for every periodic task in the system —
	// there is no OS cron anywhere in this stack. New periodic jobs
	// (people:scan_birthdays, notify:purge_old, …) add one scheduler.Register(...)
	// line here plus a mux.HandleFunc handler above (ops:backup_database below).
	scheduler := asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{Location: time.UTC})
	if _, err := scheduler.Register("@every 24h",
		asynq.NewTask(taskPurgeRefreshTokens, nil), asynq.Queue("default")); err != nil {
		return fmt.Errorf("register refresh-token purge schedule: %w", err)
	}
	if _, err := scheduler.Register("@every 1h",
		asynq.NewTask(mediaworker.TaskTypePurgeOrphans, nil), asynq.Queue("default")); err != nil {
		return fmt.Errorf("register purge-orphans schedule: %w", err)
	}
	// Nightly Postgres backup at 03:00 UTC (SPEC-09 P0.2). Cron, not @every, so
	// it lands at a predictable low-traffic hour rather than drifting.
	if _, err := scheduler.Register("0 3 * * *",
		asynq.NewTask(opsapi.TaskBackupDatabase, nil), asynq.Queue("default")); err != nil {
		return fmt.Errorf("register ops backup schedule: %w", err)
	}
	// Daily birthday scan at 06:00 UTC (SPEC-08 P0.4) — emits people:birthday_upcoming.
	if _, err := scheduler.Register("0 6 * * *",
		asynq.NewTask(peopleapi.TaskScanBirthdays, nil), asynq.Queue("default")); err != nil {
		return fmt.Errorf("register birthday-scan schedule: %w", err)
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info().Str("env", cfg.AppEnv).Int("heavy_concurrency", heavyConcurrency).Msg("worker started")
		if err := heavySrv.Run(heavyMux); err != nil {
			log.Error().Err(err).Msg("heavy worker error")
			stop()
		}
	}()
	go func() {
		log.Info().Int("image_concurrency", imageConcurrency).Msg("image worker started")
		if err := imageSrv.Run(imageMux); err != nil {
			log.Error().Err(err).Msg("image worker error")
			stop()
		}
	}()
	go func() {
		if err := lightSrv.Run(lightMux); err != nil {
			log.Error().Err(err).Msg("light worker error")
			stop()
		}
	}()
	if err := scheduler.Start(); err != nil {
		return fmt.Errorf("scheduler start: %w", err)
	}
	log.Info().Msg("periodic scheduler started")

	<-sigCtx.Done()
	log.Info().Msg("worker shutting down")
	scheduler.Shutdown()
	heavySrv.Shutdown()
	lightSrv.Shutdown()
	return nil
}
