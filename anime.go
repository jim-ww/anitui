package main

import "time"

// Anime is a single watch-list entry. WatchSessions holds one entry per
// watch-through (a rewatch starts a new session), each a chronological list
// of the dates you watched an episode during that pass. Started/last-watch/
// finished/rewatch-count are derived from it rather than stored separately,
// so they can never drift out of sync with the actual dates.
type Anime struct {
	Title         string
	Progress      *int
	Status        Status
	WatchSessions [][]time.Time
	Rating        *float32
	Notes         string
}

const DateDisplayFormat = "2006-01-02"

// StartedAt is the first watched date of the first session, if any.
func (a Anime) StartedAt() *time.Time {
	for _, session := range a.WatchSessions {
		if len(session) > 0 {
			return &session[0]
		}
	}
	return nil
}

// LastWatch is the most recent watched date across all sessions, if any.
func (a Anime) LastWatch() *time.Time {
	for i := len(a.WatchSessions) - 1; i >= 0; i-- {
		session := a.WatchSessions[i]
		if len(session) > 0 {
			return &session[len(session)-1]
		}
	}
	return nil
}

// FinishedAt is the LastWatch date, but only once the entry is completed.
func (a Anime) FinishedAt() *time.Time {
	if a.Status != StatusCompleted {
		return nil
	}
	return a.LastWatch()
}

// TotalRewatch is the number of watch-throughs after the first.
func (a Anime) TotalRewatch() int {
	if len(a.WatchSessions) == 0 {
		return 0
	}
	return len(a.WatchSessions) - 1
}

// Field identifies one editable Anime attribute.
type Field string

const (
	FieldTitle         Field = "title"
	FieldProgress      Field = "progress"
	FieldStatus        Field = "status"
	FieldWatchSessions Field = "watch_sessions"
	FieldRating        Field = "rating"
	FieldNotes         Field = "notes"
)

func (f Field) String() string { return string(f) }

func FieldList() []Field {
	return []Field{
		FieldTitle,
		FieldProgress,
		FieldStatus,
		FieldWatchSessions,
		FieldRating,
		FieldNotes,
	}
}
