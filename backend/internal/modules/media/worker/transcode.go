package worker

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

const TaskTypeTranscode = "media:transcode"

// TranscodePayload is enqueued after a media upload completes.
// The worker pulls the source from S3, runs FFmpeg to produce HLS variants,
// and writes the manifest + segment paths back to the database.
type TranscodePayload struct {
	AssetID    string   `json:"asset_id"`
	SourceKey  string   `json:"source_key"`         // S3 key of the uploaded original
	OutputKey  string   `json:"output_key"`         // S3 key prefix for HLS output
	Variants   []string `json:"variants,omitempty"` // e.g. ["240p","480p","720p","1080p"]
}

func NewTranscodeTask(p TranscodePayload) (*asynq.Task, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskTypeTranscode, body, asynq.Queue("transcode")), nil
}

func HandleTranscode(ctx context.Context, t *asynq.Task) error {
	var p TranscodePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	log.Info().Str("asset", p.AssetID).Msg("transcode: not implemented yet")
	// TODO: download from S3 -> ffmpeg HLS pipeline -> upload variants -> DB update
	return nil
}
