package main

import (
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	tea "charm.land/bubbletea/v2"
)

type sortKey int

const (
	sortAdded sortKey = iota
	sortLastWatch
	sortStarted
	sortCompleted
	sortRating
	sortTitle
)

var sortOptions = []struct {
	key   sortKey
	label string
}{
	{sortAdded, "Added"},
	{sortLastWatch, "Last watch"},
	{sortStarted, "Started"},
	{sortCompleted, "Completed"},
	{sortRating, "Rating"},
	{sortTitle, "Title"},
}

// sortEntries returns a sorted copy of entries; it never mutates entries.
// Dates and ratings sort with the most-recent/highest first and missing
// values pushed to the end; title sorts alphabetically. Added keeps the
// insertion order entries already came in.
func sortEntries(entries []Anime, key sortKey) []Anime {
	sorted := slices.Clone(entries)
	switch key {
	case sortLastWatch:
		sortByDateDesc(sorted, Anime.LastWatch)
	case sortStarted:
		sortByDateDesc(sorted, Anime.StartedAt)
	case sortCompleted:
		sortByDateDesc(sorted, Anime.FinishedAt)
	case sortRating:
		slices.SortFunc(sorted, func(a, b Anime) int {
			return compareRatingDesc(a.Rating, b.Rating)
		})
	case sortTitle:
		slices.SortFunc(sorted, func(a, b Anime) int {
			return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
		})
	}
	return sorted
}

func sortByDateDesc(entries []Anime, get func(Anime) *time.Time) {
	slices.SortFunc(entries, func(a, b Anime) int {
		da, db := get(a), get(b)
		switch {
		case da == nil && db == nil:
			return 0
		case da == nil:
			return 1
		case db == nil:
			return -1
		default:
			return db.Compare(*da)
		}
	})
}

func compareRatingDesc(a, b *float32) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	case *a < *b:
		return 1
	case *a > *b:
		return -1
	default:
		return 0
	}
}

// sortModel is a popup for picking how the list is ordered.
type sortModel struct {
	cursor int
}

func newSortModel(currentIndex int) sortModel {
	return sortModel{cursor: currentIndex}
}

func (m Model) updateSort(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	s := m.sort
	switch keyMsg.String() {
	case "up", "k":
		s.cursor = (s.cursor - 1 + len(sortOptions)) % len(sortOptions)
		m.sort = s
		return m, nil
	case "down", "j":
		s.cursor = (s.cursor + 1) % len(sortOptions)
		m.sort = s
		return m, nil
	case "enter":
		m.list.sortIndex = s.cursor
		m.list = m.list.reload(m.store)
		m.mode = modeList
		return m, nil
	case "esc":
		m.mode = modeList
		return m, nil
	}
	return m, nil
}

func (m sortModel) View() string {
	sb := new(strings.Builder)
	sb.WriteString(titleStyle.Render("Sort by") + "\n")
	for i, opt := range sortOptions {
		label := fieldLabelStyle
		if i == m.cursor {
			label = fieldLabelStyle.Bold(true).Underline(true).Foreground(lipgloss.Color("1"))
		}
		sb.WriteString(label.Render("  "+opt.label) + "\n")
	}
	sb.WriteString(helpStyle.Render("↑/↓ move  enter apply  esc cancel"))
	return sb.String()
}
