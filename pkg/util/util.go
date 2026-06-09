package util

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func MustDataDir(appName string) string {
	dir, err := DataDir(appName)
	if err == nil {
		return dir
	}
	return ""
}

func DataFile(appName, filename string) (string, error) {
	dir, err := DataDir(appName)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filename), nil
}

func DataDir(name string) (string, error) {
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
