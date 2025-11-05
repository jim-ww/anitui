package models

import (
	"time"
)

type Status string

const (
	StatusCompleted   Status = "completed"
	StatusWatching    Status = "watching"
	StatusPlanToWatch Status = "plan to watch"
	StatusDropped     Status = "dropped"
	StatusRewatching  Status = "rewatching"
	StatusPaused      Status = "paused"
)

func (s Status) String() string {
	return string(s)
}

func AllStatuses() []Status {
	return []Status{StatusCompleted, StatusWatching, StatusPlanToWatch, StatusDropped, StatusRewatching, StatusPaused}
}

func (s Status) IsValidStatus() bool {
	switch s {
	case StatusCompleted, StatusWatching, StatusPlanToWatch, StatusDropped, StatusRewatching, StatusPaused:
		return true
	default:
		return false
	}
}

type Anime struct {
	Status         Status    `csv:"status"` // current status of anime
	Title          string    `csv:"title"`
	Progress       int       `csv:"progress"`                  // represents how many episodes user has already finished
	LocalScore     float32   `csv:"score"`                     // user-defined local-only score. ex. 0.0 - 10.0
	StartDate      time.Time `csv:"start_date,omitempty" `     // first time user started watching specific anime. ex. 2006.01.02
	FinishDate     time.Time `csv:"finish_date,omitempty"`     // date and time when user had finished watching anime
	LastWatchDate  time.Time `csv:"last_watch_date,omitempty"` // last time user watched this anime
	TotalRewatches int       `csv:"total_rewatch"`             // number of times user has rewatched this anime
	Notes          string    `csv:"notes"`                     // optional user notes
}

func NewAnime(status Status, localScore float32, startDate, finishDate, lastWatchDate time.Time, progress, totalRewatches int, title, notes string) Anime {
	return Anime{
		Title:          title,
		Status:         status,
		LocalScore:     localScore,
		StartDate:      startDate,
		FinishDate:     finishDate,
		LastWatchDate:  lastWatchDate,
		Progress:       progress,
		TotalRewatches: totalRewatches,
		Notes:          notes,
	}
}
