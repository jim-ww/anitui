package store

import (
	"context"
	"errors"

	"codeberg.org/jim-ww/ani-tui/internal/models"
)

var (
	ErrAnimeTitleNotFound = errors.New("anime title not found")
)

type Store interface {
	GetEntries(ctx context.Context) (map[int]models.Anime, error)
	FindTitleByID(ctx context.Context, id int) (models.Anime, error)
	FindAllMatchingByTitle(ctx context.Context, title string) (map[int]models.Anime, error)
	UpdateEntryByID(ctx context.Context, id int, updated models.Anime) (upd models.Anime, err error)
	DeleteEntryByID(ctx context.Context, id int) error
}
