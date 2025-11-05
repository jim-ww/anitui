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
	GetAllEntries(ctx context.Context) ([]models.Anime, error)
	FindByTitleOne(ctx context.Context, title string) (models.Anime, error)
	FindAllMatchingByTitle(ctx context.Context, title string) ([]models.Anime, error)
	UpdateEntryByTitle(ctx context.Context, title string, updated models.Anime) (upd models.Anime, err error)
	DeleteEntryByTitle(ctx context.Context, title string) error
}
