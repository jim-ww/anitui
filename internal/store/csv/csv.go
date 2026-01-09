package csv

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
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
	entries map[int]models.Anime
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

func (s *CSVStore) GetEntries(ctx context.Context) (map[int]models.Anime, error) {
	return s.entries, nil
}

func (s *CSVStore) FindTitleByID(ctx context.Context, id int) (models.Anime, error) {
	title, found := s.entries[id]
	if !found {
		return models.Anime{}, store.ErrAnimeTitleNotFound
	}
	return title, nil
}

func (s *CSVStore) FindAllMatchingByTitle(ctx context.Context, title string) (map[int]models.Anime, error) {
	matching := map[int]models.Anime{}

	for id, anime := range s.entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if strings.HasPrefix(anime.Title, title) {
				matching[id] = anime
			}
		}
	}

	if len(matching) == 0 {
		return nil, store.ErrAnimeTitleNotFound
	}

	return matching, nil
}

func (s *CSVStore) UpdateEntryByID(ctx context.Context, id int, updated models.Anime) (upd models.Anime, err error) {
	_, found := s.entries[id]
	if !found {
		return models.Anime{}, store.ErrAnimeTitleNotFound
	}

	s.entries[id] = updated

	if err := s.WriteChangesToDisk(ctx); err != nil {
		return models.Anime{}, err
	}

	return updated, nil
}

func (s *CSVStore) DeleteEntryByID(ctx context.Context, id int) error {
	_, found := s.entries[id]
	if !found {
		return store.ErrAnimeTitleNotFound
	}

	delete(s.entries, id)

	return s.WriteChangesToDisk(ctx)
}

// should be called once, on app start
func (s *CSVStore) readWatchHistory() error {
	anime := make([]models.Anime, 0, 30)

	if err := s.decoder.Decode(&anime); err != nil {
		return fmt.Errorf("failed to unmarshal watch history: %w", err)
	}

	s.entries = make(map[int]models.Anime, len(anime))
	for i, anime := range anime {
		s.entries[i] = anime
	}

	return nil
}

func (s *CSVStore) WriteChangesToDisk(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

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
