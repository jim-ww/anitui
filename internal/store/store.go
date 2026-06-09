package store

import (
	"context"
	"time"

	"codeberg.org/jim-ww/anitui/internal/model"
)

type Config struct {
	DefaultProgress     int
	DefaultStatus       model.Status
	DefaultStartedAt    *time.Time
	DefaultFinishedAt   *time.Time
	DefaultLastWatch    *time.Time
	DefaultRating       *float32
	DefaultTotalRewatch int
}

func DefaultConfig() Config {
	now := new(time.Now())
	return Config{
		DefaultProgress:     1,
		DefaultStatus:       model.StatusWatching,
		DefaultStartedAt:    now,
		DefaultFinishedAt:   nil,
		DefaultLastWatch:    now,
		DefaultRating:       nil,
		DefaultTotalRewatch: 0,
	}
}

type Store interface {
	Entries(ctx context.Context) ([]model.Anime, error)
	Add(ctx context.Context, title string) (model.Anime, error)
	Delete(ctx context.Context, title string) error
	Update(ctx context.Context, title string, updated model.Anime) (model.Anime, error)
	FindByTitle(ctx context.Context, title string) (model.Anime, error)
	Close() error
}
