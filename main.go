package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	tea "charm.land/bubbletea/v2"
)

const appName = "anitui"

var dataFileFlag = flag.String("dataPath", defaultDataPath(), "path to the anime CSV data file")

func main() {
	flag.Parse()

	s, err := OpenStore(*dataFileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open store: %v\n", err)
		os.Exit(1)
	}

	if _, err := tea.NewProgram(newModel(s)).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "run: %v\n", err)
		os.Exit(1)
	}
}

// playEpisode launches ani-cli to play episode ep of title.
func playEpisode(title string, ep int) error {
	return exec.Command("ani-cli", "-e", strconv.Itoa(ep), title).Run()
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
