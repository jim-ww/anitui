package main

import "time"

// Anime is a single watch-list entry.
type Anime struct {
	Title        string
	Progress     int
	Status       Status
	LastWatch    *time.Time
	StartedAt    *time.Time
	FinishedAt   *time.Time
	Rating       *float32
	TotalRewatch int
	Notes        string
}

const DateDisplayFormat = "2006-01-02"

// Field identifies one editable Anime attribute.
type Field string

const (
	FieldTitle        Field = "title"
	FieldProgress     Field = "progress"
	FieldStatus       Field = "status"
	FieldStartedAt    Field = "started_at"
	FieldLastWatch    Field = "last_watch"
	FieldFinishedAt   Field = "finished_at"
	FieldRating       Field = "rating"
	FieldTotalRewatch Field = "total_rewatch"
	FieldNotes        Field = "notes"
)

func (f Field) String() string { return string(f) }

func FieldList() []Field {
	return []Field{
		FieldTitle,
		FieldProgress,
		FieldStatus,
		FieldStartedAt,
		FieldLastWatch,
		FieldFinishedAt,
		FieldRating,
		FieldTotalRewatch,
		FieldNotes,
	}
}
