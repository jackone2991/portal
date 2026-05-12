package worker

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

const TaskTypeThumbnail = "media:thumbnail"

type ThumbnailPayload struct {
	AssetID   string `json:"asset_id"`
	SourceKey string `json:"source_key"`
	OutputKey string `json:"output_key"`
	AtSecond  int    `json:"at_second"` // timestamp to grab frame from
}

func NewThumbnailTask(p ThumbnailPayload) (*asynq.Task, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskTypeThumbnail, body, asynq.Queue("thumbnail")), nil
}

func HandleThumbnail(ctx context.Context, t *asynq.Task) error {
	var p ThumbnailPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	log.Info().Str("asset", p.AssetID).Msg("thumbnail: not implemented yet")
	// TODO: ffmpeg -ss <AtSecond> -frames:v 1 -> upload JPEG/WebP to S3 -> DB update
	return nil
}
