package main

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jszwec/csvutil"
)

var ErrAnimeTitleNotFound = errors.New("anime title not found")

type Status string

const (
	StatusCompleted   Status = "completed"
	StatusWatching    Status = "watching"
	StatusPlanToWatch Status = "plan to watch"
	StatusDropped     Status = "dropped"
	StatusRewatching  Status = "rewatching"
	StatusPaused      Status = "paused"
)

func StatusList() []Status {
	return []Status{StatusCompleted, StatusWatching, StatusPlanToWatch, StatusDropped, StatusRewatching, StatusPaused}
}

type Anime struct {
	id            string
	Status        Status    `csv:"status"` // current status of anime
	Title         string    `csv:"title"`
	Progress      int       `csv:"progress,omitempty"`        // represents how many episodes user has already finished
	LocalScore    float32   `csv:"score,omitempty"`           // user-defined local-only score. ex. 0.0 - 10.0
	StartDate     time.Time `csv:"start_date,omitempty"`      // first time user started watching specific anime. ex. 2006.01.02
	FinishDate    time.Time `csv:"finish_date,omitempty"`     // date and time when user had finished watching anime
	LastWatchDate time.Time `csv:"last_watch_date,omitempty"` // last time user watched this anime
	TotalRewatch  int       `csv:"total_rewatch"`             // number of times user has rewatched this anime
	Notes         string    `csv:"notes"`                     // optional user notes
}

type Store struct {
	entries []Anime
	file    *os.File
	encoder *csvutil.Encoder
}

func NewStore() (*Store, error) {
	dataPath, err := AppDataFile("anitui", "animelist.csv")
	if err != nil {
		return nil, fmt.Errorf("failed to get data path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dataPath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	file, err := os.OpenFile(dataPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: path=%s err=%w", dataPath, err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: path=%s err=%w", dataPath, err)
	}

	var entries []Anime
	if fileInfo.Size() != 0 && !HasOnlyOneLine(file) {
		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
		decoder, err := csvutil.NewDecoder(csv.NewReader(file))
		if err != nil {
			return nil, fmt.Errorf("failed to create csv decoder: err=%w", err)
		}
		decoder.WithUnmarshalers(csvutil.UnmarshalFunc(func(data []byte, t *time.Time) error {
			tt, err := time.Parse(*timeFormat, string(data))
			if err == nil {
				*t = tt
			}
			return err
		}))

		if err := decoder.Decode(&entries); err != nil {
			return nil, fmt.Errorf("failed to read watch history: %w", err)
		}
		for i := range entries {
			entries[i].id = strconv.Itoa(i)
		}
	}

	encoder := csvutil.NewEncoder(csv.NewWriter(file))
	encoder.WithMarshalers(csvutil.MarshalFunc(func(t time.Time) ([]byte, error) {
		return t.AppendFormat(nil, *timeFormat), nil
	}))

	store := &Store{
		file:    file,
		encoder: encoder,
		entries: entries,
	}

	return store, nil
}

func (s *Store) GetEntries() []Anime {
	return s.entries
}

func (s *Store) FindTitleByID(id string) (Anime, error) {
	idx := slices.IndexFunc(s.entries, func(a Anime) bool { return a.id == id })
	if idx == -1 {
		return Anime{}, ErrAnimeTitleNotFound
	}
	return s.entries[idx], nil
}

func (s *Store) FindAllMatchingByTitle(title string) (map[int]Anime, error) {
	matching := map[int]Anime{}

	for id, anime := range s.entries {
		if strings.HasPrefix(anime.Title, title) {
			matching[id] = anime
		}
	}

	if len(matching) == 0 {
		return nil, ErrAnimeTitleNotFound
	}

	return matching, nil
}

func (s *Store) UpdateEntryByID(id string, updated Anime) (Anime, error) {
	idx := slices.IndexFunc(s.entries, func(a Anime) bool { return a.id == id })
	if idx == -1 {
		return Anime{}, ErrAnimeTitleNotFound
	}
	s.entries[idx] = updated

	if err := s.WriteChangesToDisk(); err != nil {
		return Anime{}, err
	}

	return updated, nil
}

func (s *Store) DeleteEntryByID(id string) error {
	idx := slices.IndexFunc(s.entries, func(a Anime) bool { return a.id == id })
	if idx == -1 {
		return ErrAnimeTitleNotFound
	}
	s.entries = slices.Delete(s.entries, idx, idx+1)

	return s.WriteChangesToDisk()
}

func (s *Store) WriteChangesToDisk() error {
	if err := s.file.Truncate(0); err != nil {
		return err
	}
	if _, err := s.file.Seek(0, 0); err != nil {
		return err
	}

	// if there is no header, add it
	if info, _ := s.file.Stat(); info.Size() == 0 {
		header, err := csvutil.Header(Anime{}, "")
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

func (s *Store) Close() error {
	return s.file.Close()
}

func HasOnlyOneLine(r io.Reader) bool {
	scanner := bufio.NewScanner(r)
	count := 0

	for scanner.Scan() {
		count++
		if count > 1 {
			return false
		}
	}

	return count == 1
}

// AppDataDir returns the base directory where the application should store
// its user-specific data files (state).
//
// Follows XDG Base Directory Specification on Unix-like systems (Linux/BSD),
// and uses native conventions on macOS and Windows.
//
// Examples:
//
//	Linux:   $XDG_DATA_HOME/app-name or ~/.local/share/app-name
//	macOS:   ~/Library/Application Support/AppName
//	Windows: %APPDATA%\AppName
func AppDataDir(appName string) (string, error) {
	if appName == "" {
		return "", fmt.Errorf("application name cannot be empty")
	}

	var base string

	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			return "", fmt.Errorf("APPDATA environment variable is not set")
		}

	case "darwin": // macOS
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, "Library", "Application Support")

	default: // Linux, BSD, etc.
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			base = xdg
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
	}

	dir := filepath.Join(base, appName)
	return dir, nil
}

// AppDataFile returns full path to a file inside the app data directory
func AppDataFile(appName, filename string) (string, error) {
	base, err := AppDataDir(appName)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, filename), nil
}

// fuzzy matching w/ Levenshtein distance
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
