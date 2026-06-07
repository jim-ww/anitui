package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/key"
	"github.com/BurntSushi/toml"
)

// KeyMap holds all configurable key bindings.
type KeyMap struct {
	Up           key.Binding
	Down         key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	Add          key.Binding
	Edit         key.Binding
	Delete       key.Binding
	SetStatus    key.Binding
	FilterStatus key.Binding
	Search       key.Binding
	SearchNext   key.Binding
	SearchPrev   key.Binding
	Watches      key.Binding
	ProgressUp   key.Binding
	ProgressDown key.Binding
	Play         key.Binding
	Confirm      key.Binding
	Cancel       key.Binding
	Quit         key.Binding
}

// DefaultKeyMap returns the built-in key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:           key.NewBinding(key.WithKeys("up", "k"),     key.WithHelp("↑/k",   "up")),
		Down:         key.NewBinding(key.WithKeys("down", "j"),   key.WithHelp("↓/j",   "down")),
		HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u"),      key.WithHelp("^u",    "½pg up")),
		HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d"),      key.WithHelp("^d",    "½pg dn")),
		Add:          key.NewBinding(key.WithKeys("a"),           key.WithHelp("a",     "add")),
		Edit:         key.NewBinding(key.WithKeys("e"),           key.WithHelp("e",     "edit")),
		Delete:       key.NewBinding(key.WithKeys("d", "delete"), key.WithHelp("d",     "delete")),
		SetStatus:    key.NewBinding(key.WithKeys("space"),       key.WithHelp("spc",   "set status")),
		FilterStatus: key.NewBinding(key.WithKeys("f"),           key.WithHelp("f",     "filter")),
		Search:       key.NewBinding(key.WithKeys("/"),           key.WithHelp("/",     "search")),
		SearchNext:   key.NewBinding(key.WithKeys("n"),           key.WithHelp("n",     "next")),
		SearchPrev:   key.NewBinding(key.WithKeys("N"),           key.WithHelp("N",     "prev")),
		Watches:      key.NewBinding(key.WithKeys("w"),           key.WithHelp("w",     "watches")),
		ProgressUp:   key.NewBinding(key.WithKeys("+", "="),      key.WithHelp("+",     "ep+")),
		ProgressDown: key.NewBinding(key.WithKeys("-"),           key.WithHelp("-",     "ep-")),
		Play:         key.NewBinding(key.WithKeys("enter"),       key.WithHelp("↵",     "play")),
		Confirm:      key.NewBinding(key.WithKeys("y", "Y"),      key.WithHelp("y",     "confirm")),
		Cancel:       key.NewBinding(key.WithKeys("esc"),         key.WithHelp("esc",   "cancel")),
		Quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q",     "quit")),
	}
}

// keyMapTOML is the raw deserialized form. Each field is a list of key strings.
type keyMapTOML struct {
	Up           []string `toml:"up"`
	Down         []string `toml:"down"`
	HalfPageUp   []string `toml:"half_page_up"`
	HalfPageDown []string `toml:"half_page_down"`
	Add          []string `toml:"add"`
	Edit         []string `toml:"edit"`
	Delete       []string `toml:"delete"`
	SetStatus    []string `toml:"set_status"`
	FilterStatus []string `toml:"filter_status"`
	Search       []string `toml:"search"`
	SearchNext   []string `toml:"search_next"`
	SearchPrev   []string `toml:"search_prev"`
	Watches      []string `toml:"watches"`
	ProgressUp   []string `toml:"progress_up"`
	ProgressDown []string `toml:"progress_down"`
	Play         []string `toml:"play"`
	Confirm      []string `toml:"confirm"`
	Cancel       []string `toml:"cancel"`
	Quit         []string `toml:"quit"`
}

type configTOML struct {
	Keys keyMapTOML `toml:"keys"`
}

// Config holds the loaded (and merged) application configuration.
type Config struct {
	Keys KeyMap
}

// LoadConfig reads the config file at path (or the default XDG location if
// path is empty). Missing file is not an error — defaults are used.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{Keys: DefaultKeyMap()}

	if path == "" {
		var err error
		path, err = defaultConfigPath()
		if err != nil {
			return cfg, nil // non-fatal
		}
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var raw configTOML
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	applyKeyOverrides(&cfg.Keys, raw.Keys)
	return cfg, nil
}

// applyKeyOverrides replaces any binding whose TOML slice is non-empty.
func applyKeyOverrides(km *KeyMap, t keyMapTOML) {
	override := func(b *key.Binding, keys []string) {
		if len(keys) == 0 {
			return
		}
		help := b.Help()
		*b = key.NewBinding(key.WithKeys(keys...), key.WithHelp(help.Key, help.Desc))
	}
	override(&km.Up, t.Up)
	override(&km.Down, t.Down)
	override(&km.HalfPageUp, t.HalfPageUp)
	override(&km.HalfPageDown, t.HalfPageDown)
	override(&km.Add, t.Add)
	override(&km.Edit, t.Edit)
	override(&km.Delete, t.Delete)
	override(&km.SetStatus, t.SetStatus)
	override(&km.FilterStatus, t.FilterStatus)
	override(&km.Search, t.Search)
	override(&km.SearchNext, t.SearchNext)
	override(&km.SearchPrev, t.SearchPrev)
	override(&km.Watches, t.Watches)
	override(&km.ProgressUp, t.ProgressUp)
	override(&km.ProgressDown, t.ProgressDown)
	override(&km.Play, t.Play)
	override(&km.Confirm, t.Confirm)
	override(&km.Cancel, t.Cancel)
	override(&km.Quit, t.Quit)
}

func defaultConfigPath() (string, error) {
	base, err := configDir("anitui")
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "config.toml"), nil
}

func configDir(appName string) (string, error) {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("APPDATA")
		if base == "" {
			return "", fmt.Errorf("APPDATA not set")
		}
		return filepath.Join(base, appName), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", appName), nil
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, appName), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", appName), nil
	}
}

// HelpRows returns key-description pairs for the help overlay.
func (km KeyMap) HelpRows() [][2]string {
	pairs := func(b key.Binding) [2]string {
		h := b.Help()
		return [2]string{h.Key, h.Desc}
	}
	return [][2]string{
		pairs(km.Up), pairs(km.Down),
		pairs(km.HalfPageUp), pairs(km.HalfPageDown),
		{"", ""},
		pairs(km.Add), pairs(km.Edit), pairs(km.Delete),
		pairs(km.SetStatus), pairs(km.FilterStatus),
		{"", ""},
		pairs(km.Search), pairs(km.SearchNext), pairs(km.SearchPrev),
		pairs(km.Watches),
		{"", ""},
		pairs(km.ProgressUp), pairs(km.ProgressDown),
		pairs(km.Play),
		{"", ""},
		pairs(km.Confirm), pairs(km.Cancel), pairs(km.Quit),
	}
}

// ExampleConfig returns a documented example config.toml content.
func ExampleConfig() string {
	var b strings.Builder
	b.WriteString("# anitui key bindings\n")
	b.WriteString("# Each value is a list of key strings.\n")
	b.WriteString("# Run `anitui -print-config` to see current bindings.\n\n")
	b.WriteString("[keys]\n")
	km := DefaultKeyMap()
	rows := []struct {
		field string
		b     key.Binding
	}{
		{"up", km.Up}, {"down", km.Down},
		{"half_page_up", km.HalfPageUp}, {"half_page_down", km.HalfPageDown},
		{"add", km.Add}, {"edit", km.Edit}, {"delete", km.Delete},
		{"set_status", km.SetStatus}, {"filter_status", km.FilterStatus},
		{"search", km.Search}, {"search_next", km.SearchNext}, {"search_prev", km.SearchPrev},
		{"watches", km.Watches},
		{"progress_up", km.ProgressUp}, {"progress_down", km.ProgressDown},
		{"play", km.Play},
		{"confirm", km.Confirm}, {"cancel", km.Cancel}, {"quit", km.Quit},
	}
	for _, r := range rows {
		ks := r.b.Keys()
		quoted := make([]string, len(ks))
		for i, k := range ks {
			quoted[i] = fmt.Sprintf("%q", k)
		}
		fmt.Fprintf(&b, "# %s = [%s]\n", r.field, strings.Join(quoted, ", "))
	}
	return b.String()
}
