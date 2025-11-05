package player

import "codeberg.org/jim-ww/ani-tui/internal/models"

type Player interface {
	Play(models.Anime) error
}
