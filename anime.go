package main

import (
	"strings"
	"time"
)

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

// airingMarker flags an entry as "actively releasing" (still airing new
// episodes) without adding a CSV column: it's a prefix on Notes, so the main
// store format never changes and the file stays hand-editable.
const airingMarker = "[airing]"

// splitAiringMarker separates the airingMarker prefix (if present) from the
// rest of notes.
func splitAiringMarker(notes string) (releasing bool, rest string) {
	s := strings.TrimSpace(notes)
	if rest, ok := strings.CutPrefix(s, airingMarker); ok {
		return true, strings.TrimSpace(rest)
	}
	return false, s
}

// ActivelyReleasing reports whether this entry is flagged as still airing.
func (a Anime) ActivelyReleasing() bool {
	releasing, _ := splitAiringMarker(a.Notes)
	return releasing
}

// NotesText is Notes with the airingMarker prefix (if any) stripped, for
// display or editing without exposing the marker as visible clutter.
func (a Anime) NotesText() string {
	_, rest := splitAiringMarker(a.Notes)
	return rest
}

// withActivelyReleasing returns notes with the airingMarker prefix added or
// removed to match releasing.
func withActivelyReleasing(notes string, releasing bool) string {
	_, rest := splitAiringMarker(notes)
	if !releasing {
		return rest
	}
	if rest == "" {
		return airingMarker
	}
	return airingMarker + " " + rest
}

// RecentlyWatchedWhileAiring reports whether this entry is flagged as
// actively releasing and was last watched less than a week ago, so a new
// episode likely isn't out yet — useful for filtering it out of a
// what-should-I-watch-next list.
func (a Anime) RecentlyWatchedWhileAiring(now time.Time) bool {
	if !a.ActivelyReleasing() {
		return false
	}
	lastWatch := a.LastWatch()
	if lastWatch == nil {
		return false
	}
	return now.Sub(*lastWatch) < 7*24*time.Hour
}

// Field identifies one editable Anime attribute.
type Field string

const (
	FieldTitle         Field = "title"
	FieldProgress      Field = "progress"
	FieldStatus        Field = "status"
	FieldReleasing     Field = "releasing"
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
		FieldReleasing,
		FieldWatchSessions,
		FieldRating,
		FieldNotes,
	}
}
