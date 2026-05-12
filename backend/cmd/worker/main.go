// Command worker runs the Portal Asynq job consumer.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/portal/backend/internal/platform/config"
	"github.com/portal/backend/internal/modules/media/worker"
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

	redisOpt, err := asynq.ParseRedisURI(cfg.AsynqRedisURL)
	if err != nil {
		return err
	}

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 4,
			Queues: map[string]int{
				"transcode": 5, // most weight: heavy CPU/IO work
				"thumbnail": 3,
				"default":   1,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(worker.TaskTypeTranscode, worker.HandleTranscode)
	mux.HandleFunc(worker.TaskTypeThumbnail, worker.HandleThumbnail)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info().Str("env", cfg.AppEnv).Msg("worker started")
		if err := srv.Run(mux); err != nil {
			log.Error().Err(err).Msg("worker error")
			stop()
		}
	}()

	<-ctx.Done()
	log.Info().Msg("worker shutting down")
	srv.Shutdown()
	return nil
}
