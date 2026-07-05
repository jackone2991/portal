// Command worker runs the Portal Asynq job consumer (FFmpeg transcode/thumbnail).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/portal/backend/internal/modules/media"
	mediarepo "github.com/portal/backend/internal/modules/media/repository"
	"github.com/portal/backend/internal/platform/config"
	"github.com/portal/backend/internal/platform/storage"
)

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

	// ── Infrastructure the transcode pipeline needs ─────────────────
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

	mediaMod, err := media.New(media.Deps{Store: store, Repo: mediarepo.NewAdapter(pool)})
	if err != nil {
		return fmt.Errorf("media module: %w", err)
	}

	// ── Asynq server ────────────────────────────────────────────────
	redisOpt, err := asynq.ParseRedisURI(cfg.AsynqRedisURL)
	if err != nil {
		return err
	}
	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 4,
		Queues: map[string]int{
			"transcode": 5, // most weight: heavy CPU/IO work
			"thumbnail": 3,
			"default":   1,
		},
	})

	mux := asynq.NewServeMux()
	mediaMod.RegisterTasks(mux)

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info().Str("env", cfg.AppEnv).Msg("worker started")
		if err := srv.Run(mux); err != nil {
			log.Error().Err(err).Msg("worker error")
			stop()
		}
	}()

	<-sigCtx.Done()
	log.Info().Msg("worker shutting down")
	srv.Shutdown()
	return nil
}
