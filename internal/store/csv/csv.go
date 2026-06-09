package csv

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"codeberg.org/jim-ww/anitui/internal/model"
	"codeberg.org/jim-ww/anitui/internal/store"
	"codeberg.org/jim-ww/anitui/pkg/util"
)

var (
	ErrTitleNotFound      = errors.New("title not found")
	ErrTitleAlreadyExists = errors.New("title already exists")
	ErrEmptyTitle         = errors.New("title cannot be empty")
)

type Store struct {
	cfg     store.Config
	path    string
	entries []model.Anime
}

func NewStore(cfg store.Config, appName, path string) (*Store, error) {
	if path == "" {
		var err error
		path, err = util.DataFile(appName, "anime.toml")
		if err != nil {
			return nil, err
		}
	}
	s := &Store{cfg: cfg, path: path, entries: []model.Anime{}}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Entries(context.Context) ([]model.Anime, error) {
	cp := make([]model.Anime, len(s.entries))
	copy(cp, s.entries)
	return cp, nil
}

const dateLayout = "2006-01-02"

func unmarshalDate(date string) (t *time.Time) {
	if date == "" {
		return nil
	}
	parsed, err := time.Parse(dateLayout, date)
	if err != nil {
		return nil
	}
	return &parsed
}

func (s *Store) load() error {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}

	var anime []model.Anime
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return fmt.Errorf("parse csv: %w", err)
	}

	// TODO: normalize title?
	mapFields := func(entry []string) (model.Anime, error) {
		if len(entry) != 9 {
			return model.Anime{}, fmt.Errorf("wrong number of fields: want=9, have=%d", 9, len(entry))
		}
		title := entry[0]
		progress, err := strconv.Atoi(entry[1])
		if err != nil {
			return model.Anime{}, errors.New("invalid progress")
		}
		status, valid := model.ParseStatus(entry[2])
		if !valid {
			return model.Anime{}, errors.New("invalid status")
		}
		lastWatch := unmarshalDate(entry[3])
		if !valid {
			return model.Anime{}, errors.New("invalid lastWatch")
		}
		startedAt := unmarshalDate(entry[4])
		if !valid {
			return model.Anime{}, errors.New("invalid startedAt")
		}
		finishedAt := unmarshalDate(entry[5])
		if !valid {
			return model.Anime{}, errors.New("invalid finishedAt")
		}
		var rating *float32
		if entry[6] != "" {
			r, err := strconv.ParseFloat(entry[6], 32)
			if err != nil {
				return model.Anime{}, errors.New("invalid rating")
			}
			rating = new(float32(r))
		}

		totalRewatch, err := strconv.Atoi(entry[7])
		if err != nil {
			return model.Anime{}, errors.New("invalid totalRewatch")
		}
		var notes *string
		if entry[8] != "" {
			notes = new(entry[8])
		}
		return model.Anime{
			Title:        title,
			Progress:     progress,
			Status:       status,
			LastWatch:    lastWatch,
			StartedAt:    startedAt,
			FinishedAt:   finishedAt,
			Rating:       rating,
			TotalRewatch: totalRewatch,
			Notes:        notes,
		}, nil
	}

	for i := range records {
		entry, err := mapFields(records[i])
		if err != nil {
			return fmt.Errorf("invalid csv entry: err=%w, entry=%s", err, records[i])
		}
		anime = append(anime, entry)
	}

	s.entries = anime
	return nil
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	mapFields := func(a model.Anime) []string {
		var lastWatch, startedAt, finishedAt, rating, notes string
		if a.LastWatch != nil {
			lastWatch = a.LastWatch.Format(dateLayout)
		}
		if a.StartedAt != nil {
			startedAt = a.StartedAt.Format(dateLayout)
		}
		if a.FinishedAt != nil {
			finishedAt = a.FinishedAt.Format(dateLayout)
		}
		if a.Rating != nil {
			rating = strconv.FormatFloat(float64(*a.Rating), 'f', -1, 32)
		}
		if a.Notes != nil {
			notes = *a.Notes
		}
		return []string{
			a.Title,
			strconv.Itoa(a.Progress),
			a.Status.String(),
			lastWatch,
			startedAt,
			finishedAt,
			rating,
			strconv.Itoa(a.TotalRewatch),
			notes,
		}
	}

	var records [][]string
	for i := range s.entries {
		records = append(records, mapFields(s.entries[i]))
	}

	b := new(bytes.Buffer)
	if err := csv.NewWriter(b).WriteAll(records); err != nil {
		return fmt.Errorf("csv write: %w", err)
	}

	// TODO do something before overriding file?
	f, err := os.Create(s.path)
	if err != nil {
		return fmt.Errorf("file create: %w", err)
	}

	io.Copy(f, b)
	return nil
}

func (s *Store) Close() error { return nil }

func (s *Store) EntryByIndex(i int) (a model.Anime, found bool) {
	if i < 0 || i >= len(s.entries) {
		return model.Anime{}, false
	}
	return s.entries[i], true
}

func normalizeTitle(t string) string {
	return strings.ToLower(strings.TrimSpace(t))
}

func (s *Store) entryByTitle(title string) (a model.Anime, idx int, found bool) {
	if len(s.entries) == 0 {
		return model.Anime{}, -1, false
	}

	bestIdx := -1
	bestDistance := int(float64(len(title)) * 0.15) // 15% tolerance

	for i, anime := range s.entries {
		dist := levenshtein(
			normalizeTitle(anime.Title),
			normalizeTitle(title),
		)
		if dist < bestDistance {
			bestDistance = dist
			bestIdx = i
		}
	}

	if bestIdx != -1 {
		return s.entries[bestIdx], bestIdx, true
	}
	return model.Anime{}, -1, false
}

// Levenshtein distance fuzzy matching
func levenshtein(a, b string) int {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	ar, br := []rune(a), []rune(b)
	alen, blen := len(ar), len(br)
	if alen == 0 {
		return blen
	}
	if blen == 0 {
		return alen
	}
	matrix := make([][]int, alen+1)
	for i := range matrix {
		matrix[i] = make([]int, blen+1)
	}
	for i := 0; i <= alen; i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= blen; j++ {
		matrix[0][j] = j
	}
	for i := 1; i <= alen; i++ {
		for j := 1; j <= blen; j++ {
			cost := 0
			if ar[i-1] != br[j-1] {
				cost = 1
			}
			matrix[i][j] = min3(
				matrix[i-1][j]+1,
				matrix[i][j-1]+1,
				matrix[i-1][j-1]+cost,
			)
		}
	}
	return matrix[alen][blen]
}

func min3(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}

func (s *Store) EntryByIdx(idx int) (model.Anime, bool) {
	if idx < 0 || idx >= len(s.entries) {
		return model.Anime{}, false
	}
	return s.entries[idx], true
}

func (s *Store) Add(ctx context.Context, title string) (model.Anime, error) {
	if title == "" {
		return model.Anime{}, ErrEmptyTitle
	}
	if _, err := s.FindByTitle(ctx, title); err == nil {
		return model.Anime{}, ErrTitleAlreadyExists
	}
	a := model.Anime{
		Title:        title,
		Progress:     s.cfg.DefaultProgress,
		Status:       s.cfg.DefaultStatus,
		StartedAt:    s.cfg.DefaultStartedAt,
		LastWatch:    s.cfg.DefaultLastWatch,
		FinishedAt:   s.cfg.DefaultFinishedAt,
		Rating:       nil,
		TotalRewatch: 0,
		Notes:        nil,
	}
	s.entries = append(s.entries, a)
	return a, s.save()
}

func (s *Store) FindByTitle(ctx context.Context, title string) (model.Anime, error) {
	entry, _, found := s.entryByTitle(title)
	if !found {
		return model.Anime{}, ErrTitleNotFound
	}
	return entry, nil
}

func (s *Store) Update(ctx context.Context, title string, updated model.Anime) (model.Anime, error) {
	if updated.Title == "" {
		return model.Anime{}, ErrEmptyTitle
	}
	_, i, found := s.entryByTitle(title)
	if !found {
		return model.Anime{}, ErrTitleNotFound
	}
	s.entries[i] = updated
	return updated, s.save()
}

func (s *Store) Delete(ctx context.Context, title string) error {
	_, i, found := s.entryByTitle(title)
	if !found {
		return ErrTitleNotFound
	}
	s.entries = slices.Delete(s.entries, i, i+1)
	return s.save()
}
