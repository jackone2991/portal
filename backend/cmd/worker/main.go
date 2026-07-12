// Command worker runs the Portal Asynq job consumers (FFmpeg transcode, image
// WebP variants, video posters) plus the shared periodic scheduler.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	accountrepo "github.com/portal/backend/internal/modules/account/repository"
	"github.com/portal/backend/internal/modules/bank"
	bankrepo "github.com/portal/backend/internal/modules/bank/repository"
	"github.com/portal/backend/internal/modules/journal"
	journalrepo "github.com/portal/backend/internal/modules/journal/repository"
	"github.com/portal/backend/internal/modules/media"
	mediarepo "github.com/portal/backend/internal/modules/media/repository"
	mediaworker "github.com/portal/backend/internal/modules/media/worker"
	"github.com/portal/backend/internal/modules/notify"
	notifyapi "github.com/portal/backend/internal/modules/notify/api"
	notifyrepo "github.com/portal/backend/internal/modules/notify/repository"
	"github.com/portal/backend/internal/modules/ops"
	opsapi "github.com/portal/backend/internal/modules/ops/api"
	opsrepo "github.com/portal/backend/internal/modules/ops/repository"
	"github.com/portal/backend/internal/platform/audit"
	"github.com/portal/backend/internal/platform/config"
	"github.com/portal/backend/internal/platform/events"
	"github.com/portal/backend/internal/platform/storage"
)

// Periodic task types registered on the shared scheduler (see run()).
const taskPurgeRefreshTokens = "account:purge_refresh_tokens"

// heavyConcurrency serializes the memory-heavy decodes (transcode + image
// processing) so N large images + a transcode can't decode simultaneously and
// OOM a small VPS (SPEC-01 P0.1). Bump to 2 only on a box with ≥ 4 GB RAM.
const heavyConcurrency = 1

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
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db pool: %w", err)
	}
	defer pool.Close()

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
		Repo:     mediarepo.NewAdapter(pool),
		Enqueuer: asynqClient,
		Events:   publisher,
	})
	if err != nil {
		return fmt.Errorf("media module: %w", err)
	}
	acctRepo := accountrepo.NewAdapter(pool)

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
		Repo:           notifyrepo.NewAdapter(pool),
		Enqueuer:       asynqClient,
		Email:          emailSender,
		Users:          recipientResolver{repo: acctRepo},
		Redis:          rdb,
		EmailHourlyCap: cfg.NotifyEmailHourlyCap,
	})
	if err != nil {
		return fmt.Errorf("notify module: %w", err)
	}

	// ── Journal module (worker side) ────────────────────────────────
	// No P0 tasks — the wiring exists so SPEC-06's stream consumers attach here.
	journalMod, err := journal.New(journal.Deps{Repo: journalrepo.NewAdapter(pool)})
	if err != nil {
		return fmt.Errorf("journal module: %w", err)
	}

	// ── Bank module (worker side) ───────────────────────────────────
	// No P0 tasks — the wiring exists so SPEC-06's stream consumer of
	// bank:transaction_* attaches here without touching cmd/worker's shape.
	bankMod, err := bank.New(bank.Deps{Repo: bankrepo.NewAdapter(pool)})
	if err != nil {
		return fmt.Errorf("bank module: %w", err)
	}

	// ── Ops module (worker side: nightly Postgres backup) ───────────
	// The account module isn't constructed here, so audit records route through
	// its repository adapter (a plain audit_log INSERT). BackupDatabaseURL MUST
	// point at Postgres directly (5432), not PgBouncer (SPEC-09 P0.2).
	opsMod, err := ops.New(ops.Deps{
		Repo:              opsrepo.NewAdapter(pool),
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

	// ── Heavy server: serialize the expensive decodes (P0.1 OOM guard) ──
	// Its own low-concurrency pool consumes ONLY the "heavy" queue — queue
	// weights on a shared pool cannot cap parallelism, so the cap needs its own
	// server (rev 3).
	heavySrv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: heavyConcurrency,
		Queues:      map[string]int{"heavy": 1},
	})
	heavyMux := asynq.NewServeMux()
	mediaMod.RegisterHeavyTasks(heavyMux) // media:transcode + media:process_image

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
