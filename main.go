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
	emitFlag     = flag.String("emit", "", "comma-separated, ordered list of columns to show (status,title,progress,rating,last,started,rewatch,notes); empty means the default columns")
	externalTerm = flag.Bool("external-terminal", false, "open ani-cli in a separate terminal window instead of taking over this one (best effort; needs $TERMINAL or a known terminal emulator on PATH)")
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

	emitFields, err := parseEmitFields(*emitFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	m := newModel(s, filterIndex, sortIndex, *datesFlag, *hideAiring, emitFields, *externalTerm)
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

// parseEmitFields parses the -emit flag's comma-separated column list into
// ordered, validated emitFields. An empty value means "use the default
// columns" (nil). "ep" is accepted as an alias for "progress" since that's
// the column's on-screen header.
func parseEmitFields(value string) ([]emitField, error) {
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	fields := make([]emitField, 0, len(parts))
	seen := make(map[emitField]bool, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "ep" {
			p = string(emitProgress)
		}
		f := emitField(p)
		if !validEmitField(f) {
			return nil, fmt.Errorf("invalid -emit field %q (want any of: %v)", p, emitFieldList())
		}
		if seen[f] {
			return nil, fmt.Errorf("duplicate -emit field %q", p)
		}
		seen[f] = true
		fields = append(fields, f)
	}
	return fields, nil
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

// terminalEmulators lists terminal emulators tried, in order, when
// -external-terminal is set and $TERMINAL isn't. All of them support "-e
// <command> [args...]" to run and wait on a command.
var terminalEmulators = []string{
	"x-terminal-emulator", "kitty", "alacritty", "foot", "wezterm",
	"gnome-terminal", "konsole", "xterm",
}

// findTerminalEmulator locates a terminal emulator to run ani-cli in,
// preferring $TERMINAL, so -external-terminal has something to launch.
func findTerminalEmulator() (string, error) {
	if t := os.Getenv("TERMINAL"); t != "" {
		if _, err := exec.LookPath(t); err == nil {
			return t, nil
		}
	}
	for _, t := range terminalEmulators {
		if _, err := exec.LookPath(t); err == nil {
			return t, nil
		}
	}
	return "", fmt.Errorf("no terminal emulator found (set $TERMINAL)")
}

// externalPlayCommand builds the command that opens ani-cli in a separate
// terminal window, for -external-terminal. Its wait status still reflects
// ani-cli's exit code, since gnome-terminal and friends all block their
// invoking process on "-e" until the launched command exits.
func externalPlayCommand(title string, ep int) (*exec.Cmd, error) {
	term, err := findTerminalEmulator()
	if err != nil {
		return nil, err
	}
	args := append([]string{"-e"}, playCommand(title, ep).Args...)
	return exec.Command(term, args...), nil
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
