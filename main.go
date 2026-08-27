package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/gofrs/flock"
)

const appName = "anitui"

var (
	dataFileFlag = flag.String("dataPath", defaultDataPath(), "path to the anime CSV data file")
	statusFlag   = flag.String("status", "", "initial status filter (e.g. watching); empty means all")
	datesFlag    = flag.Bool("dates", false, "start with the watch-history date columns shown (same as pressing v)")
	hideAiring   = flag.Bool("hide-airing", false, "start with recently-watched actively-releasing entries hidden (same as pressing r)")
	sortFlag     = flag.String("sort", "", "initial sort order (added, last-watch, started, completed, rating, title); empty means added")
)

func main() {
	flag.Parse()

	unlock, err := acquireLock(*dataFileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer unlock()

	s, err := OpenStore(*dataFileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open store: %v\n", err)
		os.Exit(1)
	}

	filterIndex, err := statusFilterIndex(*statusFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	sortIndex, err := sortOptionIndex(*sortFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	m := newModel(s, filterIndex, sortIndex, *datesFlag, *hideAiring)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "run: %v\n", err)
		os.Exit(1)
	}
}

// statusFilterIndex resolves the -status flag value to an index into
// statusFilters, so it lines up with what "f" cycles through in the TUI. An
// empty value means "all" (index 0).
func statusFilterIndex(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	status, valid := ParseStatus(value)
	if !valid {
		return 0, fmt.Errorf("invalid -status %q (want one of: %v)", value, StatusList())
	}
	for i, s := range statusFilters {
		if s != nil && *s == status {
			return i, nil
		}
	}
	return 0, fmt.Errorf("invalid -status %q", value)
}

// sortOptionIndex resolves the -sort flag value to an index into
// sortOptions, matching by label case- and space/hyphen-insensitively (e.g.
// "last-watch" or "Last watch" both match sortLastWatch). An empty value
// means the default (added/insertion order, index 0).
func sortOptionIndex(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	normalize := func(s string) string {
		return strings.ReplaceAll(strings.ToLower(s), " ", "-")
	}
	target := normalize(value)
	for i, opt := range sortOptions {
		if normalize(opt.label) == target {
			return i, nil
		}
	}
	labels := make([]string, len(sortOptions))
	for i, opt := range sortOptions {
		labels[i] = normalize(opt.label)
	}
	return 0, fmt.Errorf("invalid -sort %q (want one of: %v)", value, labels)
}

// acquireLock ensures only one anitui process runs against a given data
// file at a time, so two instances can never race to overwrite each other's
// changes. The lock lives in a sibling ".lock" file rather than dataPath
// itself, since flock-ing the CSV would fight with our own atomic
// write-temp-then-rename in Store.save.
func acquireLock(dataPath string) (unlock func(), err error) {
	if err := os.MkdirAll(filepath.Dir(dataPath), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	fl := flock.New(dataPath + ".lock")
	locked, err := fl.TryLock()
	if err != nil {
		return nil, fmt.Errorf("lock %s: %w", dataPath, err)
	}
	if !locked {
		return nil, fmt.Errorf("anitui is already running for %s", dataPath)
	}
	return func() { fl.Unlock() }, nil
}

// playCommand builds the ani-cli command to play episode ep of title.
func playCommand(title string, ep int) *exec.Cmd {
	return exec.Command("ani-cli", "-e", strconv.Itoa(ep), title)
}

// defaultDataPath returns the default anime.csv location under the OS's
// standard per-user data directory. Errors (e.g. no home dir) fall back to
// a relative path so the flag still has a usable default.
func defaultDataPath() string {
	dataDir, err := os.UserHomeDir()
	if err != nil {
		return "anime.csv"
	}

	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			dataDir = appData
		}
	case "darwin":
		dataDir = filepath.Join(dataDir, "Library", "Application Support")
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			dataDir = xdg
		} else {
			dataDir = filepath.Join(dataDir, ".local", "share")
		}
	}
	return filepath.Join(dataDir, appName, "anime.csv")
}
