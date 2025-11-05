package csv

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"codeberg.org/jim-ww/ani-tui/internal/models"
	"codeberg.org/jim-ww/ani-tui/internal/store"
	"github.com/jszwec/csvutil"
)

type Config struct {
	FilePath   string
	TimeFormat string
}

type CSVStore struct {
	entries []models.Anime
	file    *os.File
	decoder *csvutil.Decoder
	encoder *csvutil.Encoder
	mu      sync.Mutex
	cfg     Config
}

func NewCSVStore(cfg Config) (*CSVStore, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.FilePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	file, err := os.OpenFile(cfg.FilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: path=%s err=%w", cfg.FilePath, err)
	}

	unmarshalTime := func(data []byte, t *time.Time) error {
		tt, err := time.Parse(cfg.TimeFormat, string(data))
		if err != nil {
			return err
		}
		*t = tt
		return nil
	}

	marshalTime := func(t time.Time) ([]byte, error) {
		return t.AppendFormat(nil, cfg.TimeFormat), nil
	}

	csvReader := csv.NewReader(file)
	csvWriter := csv.NewWriter(file)

	decoder, err := csvutil.NewDecoder(csvReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create csv decoder: err=%w", err)
	}
	decoder.WithUnmarshalers(csvutil.UnmarshalFunc(unmarshalTime))

	encoder := csvutil.NewEncoder(csvWriter)
	encoder.WithMarshalers(csvutil.MarshalFunc(marshalTime))

	store := &CSVStore{
		cfg:     cfg,
		file:    file,
		decoder: decoder,
		encoder: encoder,
		mu:      sync.Mutex{},
		entries: nil,
	}

	if err := store.readWatchHistory(); err != nil {
		return nil, fmt.Errorf("failed to read watch history: err=%w", err)
	}
	return store, nil
}

func (s *CSVStore) GetAllEntries(ctx context.Context) ([]models.Anime, error) {
	return s.entries, nil
}

func (s *CSVStore) FindByTitleOne(ctx context.Context, title string) (models.Anime, error) {
	index := slices.IndexFunc(s.entries, func(e models.Anime) bool {
		return e.Title == title
	})

	if index == -1 {
		return models.Anime{}, store.ErrAnimeTitleNotFound
	}

	return s.entries[index], nil
}

func (s *CSVStore) FindAllMatchingByTitle(ctx context.Context, title string) ([]models.Anime, error) {
	matching := []models.Anime{}

	for _, anime := range s.entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if strings.HasPrefix(anime.Title, title) {
				matching = append(matching, anime)
			}
		}
	}

	if len(matching) == 0 {
		return nil, store.ErrAnimeTitleNotFound
	}

	return matching, nil
}

func (s *CSVStore) UpdateEntryByTitle(ctx context.Context, title string, updated models.Anime) (upd models.Anime, err error) {
	index := slices.IndexFunc(s.entries, func(e models.Anime) bool {
		return e.Title == title
	})

	if index == -1 {
		return models.Anime{}, store.ErrAnimeTitleNotFound
	}

	s.entries[index] = updated

	if err := s.WriteChangesToDisk(ctx); err != nil {
		return models.Anime{}, err
	}

	return updated, nil
}

func (s *CSVStore) DeleteEntryByTitle(ctx context.Context, title string) error {
	index := slices.IndexFunc(s.entries, func(e models.Anime) bool {
		return e.Title == title
	})

	if index == -1 {
		return store.ErrAnimeTitleNotFound
	}

	s.entries = slices.Delete(s.entries, index, index+1)

	return s.WriteChangesToDisk(ctx)
}

func (s *CSVStore) readWatchHistory() error {
	anime := make([]models.Anime, 0, 30)

	if err := s.decoder.Decode(&anime); err != nil {
		return fmt.Errorf("failed to unmarshal watch history: %w", err)
	}

	s.entries = anime
	return nil
}

func (s *CSVStore) WriteChangesToDisk(ctx context.Context) error {
	if err := s.file.Truncate(0); err != nil {
		return err
	}
	if _, err := s.file.Seek(0, 0); err != nil {
		return err
	}

	// if there is not header, add it
	if info, _ := s.file.Stat(); info.Size() == 0 {
		header, err := csvutil.Header(models.Anime{}, "")
		if err != nil {
			return err
		}

		if _, err := s.file.WriteString(strings.Join(header, ",")); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	if err := s.encoder.Encode(s.entries); err != nil {
		return fmt.Errorf("failed to marshal csv entries: %w", err)
	}

	return nil
}

func (s *CSVStore) Close() error {
	return s.file.Close()
}
