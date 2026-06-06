package main

import (
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

const appName = "anitui"

var (
	errAnimeNotFound = errors.New("anime not found")
	errEmptyTitle    = errors.New("title cannot be empty")
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
		return "*"
	case StatusWatching:
		return ">"
	case StatusPlanToWatch:
		return " "
	case StatusDropped:
		return "x"
	case StatusPaused:
		return "!"
	case StatusRewatching:
		return "r"
	default:
		return "?"
	}
}

func (s Status) Next() Status {
	statuses := StatusList()
	idx := slices.Index(statuses, s)
	if idx == -1 {
		return statuses[0]
	}
	return statuses[(idx+1)%len(statuses)]
}

func ParseStatus(value string) Status {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "completed", "complete", "done", "watched", "*":
		return StatusCompleted
	case "watching", "current", "in progress", ">":
		return StatusWatching
	case "plan to watch", "planned", "plan", "ptw", "todo", "":
		return StatusPlanToWatch
	case "dropped", "drop", "x":
		return StatusDropped
	case "paused", "on hold", "hold", "!":
		return StatusPaused
	case "rewatching", "rewatch", "r":
		return StatusRewatching
	default:
		return StatusPlanToWatch
	}
}

type Anime struct {
	ID            string
	Status        Status
	Title         string
	Progress      int
	LocalScore    float32
	StartDate     time.Time
	FinishDate    time.Time
	LastWatchDate time.Time
	TotalRewatch  int
	Notes         string
}

func (a Anime) CloneWithDefaults() Anime {
	a.Title = strings.TrimSpace(a.Title)
	a.Status = ParseStatus(a.Status.String())
	if a.ID == "" {
		a.ID = newID()
	}
	if a.Progress < 0 {
		a.Progress = 0
	}
	if a.LocalScore < 0 {
		a.LocalScore = 0
	}
	if a.LocalScore > 10 {
		a.LocalScore = 10
	}
	if a.TotalRewatch < 0 {
		a.TotalRewatch = 0
	}
	return a
}

type Store struct {
	path          string
	entries       []Anime
	legacySource  bool
	backupCreated bool
}

func NewStore(path string) (*Store, error) {
	if path == "" {
		var err error
		path, err = AppDataFile(appName, "anime-progress.csv")
		if err != nil {
			return nil, err
		}
	}

	store := &Store{path: path}
	if err := store.Load(); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := store.Save(); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Entries() []Anime {
	entries := make([]Anime, len(s.entries))
	copy(entries, s.entries)
	return entries
}

func (s *Store) Load() error {
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.entries = nil
		return nil
	}
	if err != nil {
		return fmt.Errorf("open anime CSV: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("read anime CSV: %w", err)
	}
	if strings.TrimSpace(string(content)) == "" {
		s.entries = nil
		return nil
	}

	entries, structured, err := parseAnimeFile(content)
	if err != nil {
		return err
	}
	s.entries = entries
	s.legacySource = !structured
	return nil
}

func (s *Store) Add(entry Anime) (Anime, error) {
	entry = entry.CloneWithDefaults()
	if entry.Title == "" {
		return Anime{}, errEmptyTitle
	}
	s.entries = append(s.entries, entry)
	if err := s.Save(); err != nil {
		return Anime{}, err
	}
	return entry, nil
}

func (s *Store) Update(id string, updated Anime) (Anime, error) {
	idx := s.indexByID(id)
	if idx == -1 {
		return Anime{}, errAnimeNotFound
	}
	updated = updated.CloneWithDefaults()
	updated.ID = id
	if updated.Title == "" {
		return Anime{}, errEmptyTitle
	}
	s.entries[idx] = updated
	if err := s.Save(); err != nil {
		return Anime{}, err
	}
	return updated, nil
}

func (s *Store) Delete(id string) error {
	idx := s.indexByID(id)
	if idx == -1 {
		return errAnimeNotFound
	}
	s.entries = slices.Delete(s.entries, idx, idx+1)
	return s.Save()
}

func (s *Store) EntryByIndex(index int) (Anime, bool) {
	if index < 0 || index >= len(s.entries) {
		return Anime{}, false
	}
	return s.entries[index], true
}

func (s *Store) EntryByID(id string) (Anime, bool) {
	idx := s.indexByID(id)
	if idx == -1 {
		return Anime{}, false
	}
	return s.entries[idx], true
}

func (s *Store) CycleStatus(id string) (Anime, error) {
	entry, ok := s.EntryByID(id)
	if !ok {
		return Anime{}, errAnimeNotFound
	}
	entry.Status = entry.Status.Next()
	if entry.Status == StatusWatching && entry.StartDate.IsZero() {
		entry.StartDate = time.Now()
	}
	if entry.Status == StatusCompleted && entry.FinishDate.IsZero() {
		entry.FinishDate = time.Now()
	}
	return s.Update(id, entry)
}

func (s *Store) AdjustProgress(id string, delta int) (Anime, error) {
	entry, ok := s.EntryByID(id)
	if !ok {
		return Anime{}, errAnimeNotFound
	}
	entry.Progress += delta
	if entry.Progress < 0 {
		entry.Progress = 0
	}
	entry.LastWatchDate = time.Now()
	if entry.Status == StatusPlanToWatch {
		entry.Status = StatusWatching
	}
	return s.Update(id, entry)
}

func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	if s.legacySource && !s.backupCreated {
		if err := copyFile(s.path, s.path+".legacy.bak"); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("backup legacy file: %w", err)
		}
		s.backupCreated = true
	}

	temp, err := os.CreateTemp(filepath.Dir(s.path), filepath.Base(s.path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp CSV: %w", err)
	}
	tempName := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempName)
		}
	}()

	writer := csv.NewWriter(temp)
	if err := writer.Write(animeCSVHeader); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write CSV header: %w", err)
	}
	for _, entry := range s.entries {
		if err := writer.Write(animeToRecord(entry.CloneWithDefaults())); err != nil {
			_ = temp.Close()
			return fmt.Errorf("write CSV entry: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("flush CSV: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temp CSV: %w", err)
	}
	if err := os.Rename(tempName, s.path); err != nil {
		return fmt.Errorf("replace CSV: %w", err)
	}
	removeTemp = false
	s.legacySource = false
	return nil
}

func (s *Store) Close() error {
	return nil
}

func (s *Store) indexByID(id string) int {
	return slices.IndexFunc(s.entries, func(entry Anime) bool {
		return entry.ID == id
	})
}

var animeCSVHeader = []string{
	"id",
	"status",
	"title",
	"progress",
	"score",
	"start_date",
	"finish_date",
	"last_watch_date",
	"total_rewatch",
	"notes",
}

func parseAnimeFile(content []byte) ([]Anime, bool, error) {
	reader := csv.NewReader(strings.NewReader(string(content)))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err == nil && hasStructuredHeader(records) {
		entries, err := parseStructuredRecords(records)
		return entries, true, err
	}

	entries, err := parseLegacyProgress(string(content))
	return entries, false, err
}

func hasStructuredHeader(records [][]string) bool {
	if len(records) == 0 || len(records[0]) == 0 {
		return false
	}
	headers := headerIndex(records[0])
	_, hasTitle := headers["title"]
	_, hasStatus := headers["status"]
	return hasTitle && hasStatus
}

func parseStructuredRecords(records [][]string) ([]Anime, error) {
	headers := headerIndex(records[0])
	entries := make([]Anime, 0, len(records)-1)
	for line, record := range records[1:] {
		if isEmptyRecord(record) {
			continue
		}
		entry, err := recordToAnime(headers, record)
		if err != nil {
			return nil, fmt.Errorf("parse CSV line %d: %w", line+2, err)
		}
		if entry.Title == "" {
			continue
		}
		entries = append(entries, entry.CloneWithDefaults())
	}
	return entries, nil
}

func headerIndex(record []string) map[string]int {
	headers := make(map[string]int, len(record))
	for i, header := range record {
		headers[strings.ToLower(strings.TrimSpace(header))] = i
	}
	return headers
}

func isEmptyRecord(record []string) bool {
	for _, field := range record {
		if strings.TrimSpace(field) != "" {
			return false
		}
	}
	return true
}

func recordToAnime(headers map[string]int, record []string) (Anime, error) {
	var entry Anime
	var err error
	entry.ID = field(headers, record, "id")
	entry.Status = ParseStatus(field(headers, record, "status"))
	entry.Title = field(headers, record, "title")
	entry.Progress, err = parseInt(field(headers, record, "progress"))
	if err != nil {
		return Anime{}, fmt.Errorf("progress: %w", err)
	}
	entry.LocalScore, err = parseFloat32(field(headers, record, "score"))
	if err != nil {
		return Anime{}, fmt.Errorf("score: %w", err)
	}
	entry.StartDate, err = parseDate(field(headers, record, "start_date"))
	if err != nil {
		return Anime{}, fmt.Errorf("start_date: %w", err)
	}
	entry.FinishDate, err = parseDate(field(headers, record, "finish_date"))
	if err != nil {
		return Anime{}, fmt.Errorf("finish_date: %w", err)
	}
	entry.LastWatchDate, err = parseDate(field(headers, record, "last_watch_date"))
	if err != nil {
		return Anime{}, fmt.Errorf("last_watch_date: %w", err)
	}
	entry.TotalRewatch, err = parseInt(field(headers, record, "total_rewatch"))
	if err != nil {
		return Anime{}, fmt.Errorf("total_rewatch: %w", err)
	}
	entry.Notes = field(headers, record, "notes")
	return entry, nil
}

func animeToRecord(entry Anime) []string {
	return []string{
		entry.ID,
		entry.Status.String(),
		entry.Title,
		formatInt(entry.Progress),
		formatScore(entry.LocalScore),
		formatDate(entry.StartDate),
		formatDate(entry.FinishDate),
		formatDate(entry.LastWatchDate),
		formatInt(entry.TotalRewatch),
		entry.Notes,
	}
}

func field(headers map[string]int, record []string, name string) string {
	index, ok := headers[name]
	if !ok || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

var legacyLinePattern = regexp.MustCompile(`^\s*(?:(\d+(?:\.\d+)?)\s+)?(?:\((.*?)\)\s*)?\[([^\]]*)\]\s*(?:\[(\d+(?:\.\d+)?)\]\s*)?(.*)$`)

func parseLegacyProgress(content string) ([]Anime, error) {
	lines := strings.Split(content, "\n")
	entries := make([]Anime, 0, len(lines))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entry, err := parseLegacyLine(line)
		if err != nil {
			return nil, fmt.Errorf("parse legacy line %d: %w", i+1, err)
		}
		if entry.Title != "" {
			entries = append(entries, entry.CloneWithDefaults())
		}
	}
	return entries, nil
}

func parseLegacyLine(line string) (Anime, error) {
	matches := legacyLinePattern.FindStringSubmatch(line)
	if matches == nil {
		return Anime{
			ID:     newID(),
			Status: StatusPlanToWatch,
			Title:  strings.TrimSpace(line),
		}, nil
	}

	score, err := parseFloat32(matches[1])
	if err != nil {
		return Anime{}, err
	}
	totalRewatch, err := parseInt(matches[4])
	if err != nil {
		return Anime{}, err
	}
	title, notes := splitLegacyTitleNotes(matches[5], matches[2])

	return Anime{
		ID:           newID(),
		Status:       ParseStatus(matches[3]),
		Title:        title,
		LocalScore:   score,
		TotalRewatch: totalRewatch,
		Notes:        notes,
	}, nil
}

func splitLegacyTitleNotes(value, prefixNote string) (string, string) {
	value = strings.TrimSpace(value)
	notes := strings.TrimSpace(prefixNote)
	if strings.Contains(strings.ToUpper(value), "WAITING FOR RELEASE") {
		value = strings.ReplaceAll(value, "WAITING FOR RELEASE", "")
		notes = strings.TrimSpace(strings.Join(nonEmpty(notes, "waiting for release"), "; "))
	}
	return strings.TrimSpace(value), notes
}

func nonEmpty(values ...string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			filtered = append(filtered, strings.TrimSpace(value))
		}
	}
	return filtered
}

func parseInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return int(floatValue), nil
}

func parseFloat32(value string) (float32, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return 0, err
	}
	return float32(parsed), nil
}

func parseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	formats := []string{"2006-01-02", "02.01.2006", time.RFC3339}
	for _, format := range formats {
		parsed, err := time.Parse(format, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported date %q", value)
}

func formatInt(value int) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func formatScore(value float32) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatFloat(float64(value), 'f', -1, 32)
}

func formatDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02")
}

func newID() string {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(bytes)
}

func copyFile(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, input)
	return err
}

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
	return filepath.Join(base, appName), nil
}

func AppDataFile(appName, filename string) (string, error) {
	base, err := AppDataDir(appName)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, filename), nil
}
