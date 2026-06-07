package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"github.com/BurntSushi/toml"
)

const appName = "anitui"

var (
	errAnimeNotFound = errors.New("anime not found")
	errEmptyTitle    = errors.New("title cannot be empty")
)

// ── Status ────────────────────────────────────────────────────────────────────

type Status string

const (
	StatusWatching    Status = "watching"
	StatusCompleted   Status = "completed"
	StatusPlanToWatch Status = "plan to watch"
	StatusPaused      Status = "paused"
	StatusDropped     Status = "dropped"
	StatusRewatching  Status = "rewatching"
)

func StatusList() []Status {
	return []Status{
		StatusWatching,
		StatusCompleted,
		StatusPlanToWatch,
		StatusPaused,
		StatusDropped,
		StatusRewatching,
	}
}

func (s Status) String() string {
	if s == "" {
		return string(StatusPlanToWatch)
	}
	return string(s)
}

func (s Status) Symbol() string {
	switch s {
	case StatusCompleted:
		return "✓"
	case StatusWatching:
		return "▶"
	case StatusPlanToWatch:
		return "·"
	case StatusDropped:
		return "✗"
	case StatusPaused:
		return "!"
	case StatusRewatching:
		return "↺"
	default:
		return "?"
	}
}

func (s Status) Next() Status {
	list := StatusList()
	idx := slices.Index(list, s)
	if idx == -1 {
		return list[0]
	}
	return list[(idx+1)%len(list)]
}

func ParseStatus(value string) Status {
	switch value {
	case string(StatusWatching):
		return StatusWatching
	case string(StatusCompleted):
		return StatusCompleted
	case string(StatusPlanToWatch):
		return StatusPlanToWatch
	case string(StatusPaused):
		return StatusPaused
	case string(StatusDropped):
		return StatusDropped
	case string(StatusRewatching):
		return StatusRewatching
	default:
		return StatusPlanToWatch
	}
}

// ── TOML shape ────────────────────────────────────────────────────────────────

// tomlWatch mirrors one entry in the watches array.
type tomlWatch struct {
	Status string   `toml:"status"`
	Dates  []string `toml:"dates"`
}

// tomlAnime mirrors the [[anime]] table in the TOML file.
type tomlAnime struct {
	Title    string      `toml:"title"`
	Rating   float32     `toml:"rating"`
	Progress int         `toml:"progress"`
	Notes    string      `toml:"notes"`
	Watches  []tomlWatch `toml:"watches"`
}

type tomlFile struct {
	Anime []tomlAnime `toml:"anime"`
}

// ── Domain model ─────────────────────────────────────────────────────────────

// WatchRecord is one entry in the watches history.
type WatchRecord struct {
	Status    Status
	StartDate time.Time
	EndDate   time.Time
	// intermediate watch-session dates stored verbatim for round-trip fidelity
	rawDates []string
}

type Anime struct {
	ID       string
	Title    string
	Rating   float32
	Progress int
	Notes    string
	Watches  []WatchRecord
}

// Status returns the status of the current (last) watch record.
func (a Anime) Status() Status {
	if len(a.Watches) == 0 {
		return StatusPlanToWatch
	}
	return a.Watches[len(a.Watches)-1].Status
}

// TotalRewatch is the number of completed watch cycles beyond the first.
func (a Anime) TotalRewatch() int {
	if len(a.Watches) <= 1 {
		return 0
	}
	return len(a.Watches) - 1
}

// CurrentWatch returns the active (last) watch record.
func (a Anime) CurrentWatch() WatchRecord {
	if len(a.Watches) == 0 {
		return WatchRecord{Status: StatusPlanToWatch}
	}
	return a.Watches[len(a.Watches)-1]
}

// ── Conversion helpers ────────────────────────────────────────────────────────

func parseTomlDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func formatTomlDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func tomlWatchToRecord(w tomlWatch) WatchRecord {
	rec := WatchRecord{
		Status:   ParseStatus(w.Status),
		rawDates: w.Dates,
	}
	if len(w.Dates) > 0 {
		rec.StartDate = parseTomlDate(w.Dates[0])
	}
	if len(w.Dates) > 1 {
		rec.EndDate = parseTomlDate(w.Dates[len(w.Dates)-1])
	}
	return rec
}

func recordToTomlWatch(r WatchRecord) tomlWatch {
	return tomlWatch{
		Status: r.Status.String(),
		Dates:  r.rawDates,
	}
}

func tomlAnimeToAnime(ta tomlAnime, id string) Anime {
	watches := make([]WatchRecord, 0, len(ta.Watches))
	for _, w := range ta.Watches {
		watches = append(watches, tomlWatchToRecord(w))
	}
	return Anime{
		ID:       id,
		Title:    ta.Title,
		Rating:   ta.Rating,
		Progress: ta.Progress,
		Notes:    ta.Notes,
		Watches:  watches,
	}
}

func animeToToml(a Anime) tomlAnime {
	watches := make([]tomlWatch, 0, len(a.Watches))
	for _, w := range a.Watches {
		watches = append(watches, recordToTomlWatch(w))
	}
	return tomlAnime{
		Title:    a.Title,
		Rating:   a.Rating,
		Progress: a.Progress,
		Notes:    a.Notes,
		Watches:  watches,
	}
}

// ── Store ─────────────────────────────────────────────────────────────────────

type Store struct {
	path    string
	entries []Anime
	nextID  int
}

func NewStore(path string) (*Store, error) {
	if path == "" {
		var err error
		path, err = AppDataFile(appName, "anime.toml")
		if err != nil {
			return nil, err
		}
	}
	s := &Store{path: path}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Path() string { return s.path }

func (s *Store) Entries() []Anime {
	cp := make([]Anime, len(s.entries))
	copy(cp, s.entries)
	return cp
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read toml: %w", err)
	}

	var f tomlFile
	if _, err := toml.Decode(string(data), &f); err != nil {
		return fmt.Errorf("parse toml: %w", err)
	}

	s.entries = make([]Anime, len(f.Anime))
	for i, ta := range f.Anime {
		s.entries[i] = tomlAnimeToAnime(ta, s.allocID())
	}
	return nil
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var f tomlFile
	f.Anime = make([]tomlAnime, len(s.entries))
	for i, a := range s.entries {
		f.Anime[i] = animeToToml(a)
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), "*.toml.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()

	enc := toml.NewEncoder(tmp)
	if err := enc.Encode(f); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode toml: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}
	ok = true
	return nil
}

func (s *Store) allocID() string {
	s.nextID++
	return fmt.Sprintf("%d", s.nextID)
}

func (s *Store) indexByID(id string) int {
	return slices.IndexFunc(s.entries, func(a Anime) bool { return a.ID == id })
}

func (s *Store) EntryByIndex(i int) (Anime, bool) {
	if i < 0 || i >= len(s.entries) {
		return Anime{}, false
	}
	return s.entries[i], true
}

func (s *Store) EntryByID(id string) (Anime, bool) {
	i := s.indexByID(id)
	if i == -1 {
		return Anime{}, false
	}
	return s.entries[i], true
}

func (s *Store) Add(a Anime) (Anime, error) {
	if a.Title == "" {
		return Anime{}, errEmptyTitle
	}
	a.ID = s.allocID()
	if len(a.Watches) == 0 {
		a.Watches = []WatchRecord{{Status: StatusPlanToWatch}}
	}
	s.entries = append(s.entries, a)
	return a, s.save()
}

func (s *Store) Update(id string, updated Anime) (Anime, error) {
	i := s.indexByID(id)
	if i == -1 {
		return Anime{}, errAnimeNotFound
	}
	if updated.Title == "" {
		return Anime{}, errEmptyTitle
	}
	updated.ID = id
	s.entries[i] = updated
	return updated, s.save()
}

func (s *Store) Delete(id string) error {
	i := s.indexByID(id)
	if i == -1 {
		return errAnimeNotFound
	}
	s.entries = slices.Delete(s.entries, i, i+1)
	return s.save()
}

// CycleStatus advances the current watch record's status.
// When moving to StatusWatching it stamps the start date.
// When moving to StatusCompleted it stamps the end date.
func (s *Store) CycleStatus(id string) (Anime, error) {
	a, ok := s.EntryByID(id)
	if !ok {
		return Anime{}, errAnimeNotFound
	}
	if len(a.Watches) == 0 {
		a.Watches = []WatchRecord{{Status: StatusPlanToWatch}}
	}
	cur := &a.Watches[len(a.Watches)-1]
	cur.Status = cur.Status.Next()

	today := time.Now()
	todayStr := formatTomlDate(today)

	switch cur.Status {
	case StatusWatching:
		if cur.StartDate.IsZero() {
			cur.StartDate = today
			cur.rawDates = append(cur.rawDates, todayStr)
		}
	case StatusCompleted:
		if cur.EndDate.IsZero() {
			cur.EndDate = today
			cur.rawDates = append(cur.rawDates, todayStr)
		}
	}
	return s.Update(id, a)
}

// AdjustProgress bumps episode count and records today as a watch date.
func (s *Store) AdjustProgress(id string, delta int) (Anime, error) {
	a, ok := s.EntryByID(id)
	if !ok {
		return Anime{}, errAnimeNotFound
	}
	a.Progress += delta
	if a.Progress < 0 {
		a.Progress = 0
	}

	if len(a.Watches) == 0 {
		a.Watches = []WatchRecord{{Status: StatusWatching}}
	}
	cur := &a.Watches[len(a.Watches)-1]
	if cur.Status == StatusPlanToWatch {
		cur.Status = StatusWatching
	}

	today := formatTomlDate(time.Now())
	// append only if not already today
	if len(cur.rawDates) == 0 || cur.rawDates[len(cur.rawDates)-1] != today {
		cur.rawDates = append(cur.rawDates, today)
	}
	if cur.StartDate.IsZero() {
		cur.StartDate = parseTomlDate(today)
	}
	cur.EndDate = parseTomlDate(today)

	return s.Update(id, a)
}

func (s *Store) Close() error { return nil }

// ── XDG helpers ───────────────────────────────────────────────────────────────

func AppDataDir(name string) (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			return "", fmt.Errorf("APPDATA not set")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, "Library", "Application Support")
	default:
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
	return filepath.Join(base, name), nil
}

func AppDataFile(name, filename string) (string, error) {
	dir, err := AppDataDir(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filename), nil
}
