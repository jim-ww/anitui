package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

var (
	ErrTitleNotFound      = errors.New("title not found")
	ErrTitleAlreadyExists = errors.New("title already exists")
	ErrEmptyTitle         = errors.New("title cannot be empty")
)

var header = []string{"title", "progress", "status", "watch_sessions", "rating", "notes"}

const dateLayout = "2006-01-02"

// sessionSep separates watch-throughs; dateSep separates the dates within
// one watch-through. Neither collides with the date format (digits and '-'
// only) or the CSV field delimiter, so the watch_sessions cell never needs
// quoting just because of this data.
const (
	sessionSep = "|"
	dateSep    = ";"
)

// Store holds anime entries loaded from and saved to a CSV file at path.
// Entries preserve file/insertion order (Add appends, Delete/Update never
// reorder), so the file order is always "the order you added things in".
type Store struct {
	path         string
	defaultEntry Anime
	entries      []Anime
}

// Option configures defaults applied to newly Add-ed entries.
type Option func(*Store)

func WithDefaultStatus(s Status) Option {
	return func(st *Store) { st.defaultEntry.Status = s }
}

func WithDefaultProgress(p int) Option {
	return func(st *Store) { st.defaultEntry.Progress = p }
}

// OpenStore loads entries from path, creating no file until the first save.
func OpenStore(path string, opts ...Option) (*Store, error) {
	s := &Store{
		path:         path,
		defaultEntry: Anime{Status: StatusPlanToWatch},
	}
	for _, opt := range opts {
		opt(s)
	}
	if err := s.load(); err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	return s, nil
}

// Entries returns a copy of all entries in file order, optionally filtered
// by status.
func (s *Store) Entries(status *Status) []Anime {
	entries := slices.Clone(s.entries)
	if status == nil {
		return entries
	}
	return slices.DeleteFunc(entries, func(a Anime) bool {
		return a.Status != *status
	})
}

// Add appends a new entry at the end, preserving insertion order. Fields
// left unset in a follow the store's configured defaults (see WithDefault*
// options) — callers only need to set Title, or any fields they want to
// override.
func (s *Store) Add(a Anime) (Anime, error) {
	if a.Title == "" {
		return Anime{}, ErrEmptyTitle
	}
	if _, _, found := s.findByTitle(a.Title); found {
		return Anime{}, ErrTitleAlreadyExists
	}
	s.entries = append(s.entries, a)
	return a, s.save()
}

// DefaultEntry returns a blank entry seeded with the store's configured
// defaults, for callers building a new entry (e.g. the add form).
func (s *Store) DefaultEntry() Anime {
	return s.defaultEntry
}

func (s *Store) FindByTitle(title string) (Anime, error) {
	a, _, found := s.findByTitle(title)
	if !found {
		return Anime{}, ErrTitleNotFound
	}
	return a, nil
}

// Update replaces the entry found by originalTitle in place, preserving its
// position in file order.
func (s *Store) Update(originalTitle string, updated Anime) (Anime, error) {
	if updated.Title == "" {
		return Anime{}, ErrEmptyTitle
	}
	_, i, found := s.findByTitle(originalTitle)
	if !found {
		return Anime{}, ErrTitleNotFound
	}
	s.entries[i] = updated
	return updated, s.save()
}

func (s *Store) Delete(title string) error {
	_, i, found := s.findByTitle(title)
	if !found {
		return ErrTitleNotFound
	}
	s.entries = slices.Delete(s.entries, i, i+1)
	return s.save()
}

func (s *Store) findByTitle(title string) (a Anime, idx int, found bool) {
	i := slices.IndexFunc(s.entries, func(a Anime) bool {
		return strings.EqualFold(strings.TrimSpace(a.Title), strings.TrimSpace(title))
	})
	if i == -1 {
		return Anime{}, -1, false
	}
	return s.entries[i], i, true
}

func (s *Store) load() error {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return fmt.Errorf("parse csv: %w", err)
	}
	if len(records) == 0 {
		return nil
	}
	// Skip the header row.
	records = records[1:]

	entries := make([]Anime, 0, len(records))
	for _, record := range records {
		a, err := parseRecord(record)
		if err != nil {
			return fmt.Errorf("invalid entry %v: %w", record, err)
		}
		entries = append(entries, a)
	}
	s.entries = entries
	return nil
}

func parseRecord(record []string) (Anime, error) {
	if len(record) != len(header) {
		return Anime{}, fmt.Errorf("want %d fields, have %d", len(header), len(record))
	}

	var progress int
	if record[1] != "" {
		var err error
		progress, err = strconv.Atoi(record[1])
		if err != nil {
			return Anime{}, fmt.Errorf("progress: %w", err)
		}
	}
	status, valid := ParseStatus(record[2])
	if !valid {
		return Anime{}, fmt.Errorf("status: invalid value %q", record[2])
	}
	sessions, err := parseSessions(record[3])
	if err != nil {
		return Anime{}, fmt.Errorf("watch_sessions: %w", err)
	}
	var rating *float32
	if record[4] != "" {
		r, err := strconv.ParseFloat(record[4], 32)
		if err != nil {
			return Anime{}, fmt.Errorf("rating: %w", err)
		}
		rating = ptr(float32(r))
	}

	return Anime{
		Title:         record[0],
		Progress:      progress,
		Status:        status,
		WatchSessions: sessions,
		Rating:        rating,
		Notes:         record[5],
	}, nil
}

// parseSessions parses a watch_sessions cell like "2025-01-07;2025-01-08|2026-03-01"
// into one date slice per watch-through.
func parseSessions(s string) ([][]time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	sessionParts := strings.Split(s, sessionSep)
	sessions := make([][]time.Time, len(sessionParts))
	for i, part := range sessionParts {
		if part == "" {
			continue
		}
		dateParts := strings.Split(part, dateSep)
		dates := make([]time.Time, len(dateParts))
		for j, ds := range dateParts {
			t, err := time.Parse(dateLayout, strings.TrimSpace(ds))
			if err != nil {
				return nil, fmt.Errorf("date %q: %w", ds, err)
			}
			dates[j] = t
		}
		sessions[i] = dates
	}
	return sessions, nil
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	f, err := os.Create(s.path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	for _, a := range s.entries {
		if err := w.Write(toRecord(a)); err != nil {
			return fmt.Errorf("write entry %q: %w", a.Title, err)
		}
	}
	w.Flush()
	return w.Error()
}

func toRecord(a Anime) []string {
	return []string{
		a.Title,
		strconv.Itoa(a.Progress),
		a.Status.String(),
		formatSessions(a.WatchSessions),
		formatRating(a.Rating),
		a.Notes,
	}
}

func formatSessions(sessions [][]time.Time) string {
	parts := make([]string, len(sessions))
	for i, session := range sessions {
		dateStrs := make([]string, len(session))
		for j, t := range session {
			dateStrs[j] = t.Format(dateLayout)
		}
		parts[i] = strings.Join(dateStrs, dateSep)
	}
	return strings.Join(parts, sessionSep)
}

func formatRating(r *float32) string {
	if r == nil {
		return ""
	}
	return strconv.FormatFloat(float64(*r), 'f', -1, 32)
}

func ptr[T any](v T) *T { return &v }
