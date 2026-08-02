package main

import (
	"fmt"
	"strconv"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/evertras/bubble-table/table"
)

const (
	colTitle    = "title"
	colStatus   = "status"
	colProgress = "progress"
	colRating   = "rating"
)

// narrowWidthThreshold is the terminal width below which the status column
// falls back to a single symbol — spelling out "plan to watch" just doesn't
// fit a narrow terminal, and the symbols are only meant to be a space-saving
// stand-in, not the primary way statuses are shown.
const narrowWidthThreshold = 90

func columnsFor(wide bool) []table.Column {
	statusCol := table.NewColumn(colStatus, "St", 4)
	if wide {
		statusCol = table.NewColumn(colStatus, "Status", 14)
	}
	return []table.Column{
		statusCol,
		table.NewFlexColumn(colTitle, "Title", 3).WithFiltered(true),
		table.NewColumn(colProgress, "Ep", 6),
		table.NewColumn(colRating, "Rating", 8),
	}
}

// statusFilters cycles: no filter, then each status in turn.
var statusFilters = append([]*Status{nil}, statusPointers()...)

func statusPointers() []*Status {
	list := StatusList()
	out := make([]*Status, len(list))
	for i := range list {
		out[i] = &list[i]
	}
	return out
}

type listModel struct {
	table       table.Model
	entries     []Anime
	filterIndex int
	width       int
}

func newListModel(s *Store) listModel {
	// Only esc should blur the filter input. The library's default also
	// blurs on enter, which is disastrous here: the instant the filter box
	// loses focus, our own global shortcuts (a/e/d/p/space/f) start
	// consuming keystrokes again, so a stray letter typed right after
	// "confirming" a search with enter (e.g. "a" while still typing) opens
	// the add form instead of continuing the search.
	keyMap := table.DefaultKeyMap()
	keyMap.FilterBlur = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "unfocus"),
	)
	// The library defaults h/l to page up/down, which steals the standard
	// vim left/right keys for something they don't do here (there's nothing
	// to scroll horizontally) and makes single-row movement with j/k feel
	// broken by comparison. Free h/l for full-page scroll (ctrl+b/ctrl+f);
	// ctrl+u/ctrl+d are handled separately in updateList since the library
	// has no notion of a half-page jump, and its "G"/pageLast lands on the
	// first row of the last page rather than the true last row, so that's
	// handled by hand too.
	keyMap.PageDown = key.NewBinding(
		key.WithKeys("right", "pgdown", "ctrl+f"),
		key.WithHelp("ctrl+f", "page down"),
	)
	keyMap.PageUp = key.NewBinding(
		key.WithKeys("left", "pgup", "ctrl+b"),
		key.WithHelp("ctrl+b", "page up"),
	)
	// Disable the library's own "G"/end binding entirely; handled by hand
	// below so it lands on the true last row instead of the last page's first.
	keyMap.PageLast = key.NewBinding()

	t := table.New(columnsFor(false)).
		WithBaseStyle(lipgloss.NewStyle().Padding(0, 1)).
		HeaderStyle(tableHeaderStyle).
		HighlightStyle(tableHighlightStyle).
		Focused(true).
		WithPageSize(20).
		WithFuzzyFilter().
		Filtered(true).
		WithKeyMap(keyMap)

	m := listModel{table: t}
	return m.reload(s)
}

func (m listModel) reload(s *Store) listModel {
	m.entries = s.Entries(statusFilters[m.filterIndex])
	m.table = m.table.WithRows(rowsFor(m.entries, m.width >= narrowWidthThreshold))
	return m
}

func rowsFor(entries []Anime, wide bool) []table.Row {
	rows := make([]table.Row, len(entries))
	for i, a := range entries {
		statusLabel := a.Status.Symbol()
		if wide {
			statusLabel = a.Status.String()
		}
		rows[i] = table.NewRow(table.RowData{
			colStatus:   statusLabel,
			colTitle:    a.Title,
			colProgress: strconv.Itoa(a.Progress),
			colRating:   ratingLabel(a.Rating),
		})
	}
	return rows
}

// halfPage returns half the table's current page size, at least 1 row.
func halfPage(t table.Model) int {
	return max(1, t.PageSize()/2)
}

func ratingLabel(r *float32) string {
	if r == nil {
		return "–"
	}
	return fmt.Sprintf("%.1f", *r)
}

func (m listModel) resize(width, height int) listModel {
	wasWide := m.width >= narrowWidthThreshold
	m.width = width
	nowWide := width >= narrowWidthThreshold

	m.table = m.table.WithTargetWidth(width).WithPageSize(max(1, height-6))
	if nowWide != wasWide {
		m.table = m.table.WithColumns(columnsFor(nowWide))
		// entries is nil on the very first resize (before any reload), which
		// is fine: reload() will run right after and populate rows itself.
		if m.entries != nil {
			m.table = m.table.WithRows(rowsFor(m.entries, nowWide))
		}
	}
	return m
}

// selected returns the Anime behind the row currently highlighted on screen.
// It must look the row up by title rather than by GetHighlightedRowIndex,
// since that index is into the table's filtered/sorted view (GetVisibleRows),
// not into m.entries' insertion order.
func (m listModel) selected() (Anime, bool) {
	row := m.table.HighlightedRow()
	title, ok := row.Data[colTitle].(string)
	if !ok {
		return Anime{}, false
	}
	for _, a := range m.entries {
		if a.Title == title {
			return a, true
		}
	}
	return Anime{}, false
}

func (m listModel) View() string {
	filterLabel := "all"
	if s := statusFilters[m.filterIndex]; s != nil {
		filterLabel = s.String()
	}
	header := titleStyle.Render("anitui") + dimStyle.Render("  filter: "+filterLabel)
	help := helpStyle.Render("j/k move  ctrl+u/d halfpage  ctrl+b/f page  g/G top/bottom  a add  enter/e edit  d delete  p play  space status  f filter  / search  q quit")
	return header + "\n" + m.table.View() + "\n" + help
}

func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.status, m.err = "", nil

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && !m.list.table.GetIsFilterInputFocused() {
		switch keyMsg.String() {
		case "q":
			return m, tea.Quit
		case "a":
			m.mode = modeForm
			m.form = newAddForm(m.store.DefaultEntry())
			return m, m.form.init()
		case "enter", "e":
			if entry, ok := m.list.selected(); ok {
				m.mode = modeForm
				m.form = newEditForm(entry)
				return m, m.form.init()
			}
			return m, nil
		case "d":
			if entry, ok := m.list.selected(); ok {
				m.mode = modeConfirmDelete
				m.confirmTitle = entry.Title
			}
			return m, nil
		case "p":
			if entry, ok := m.list.selected(); ok {
				ep := entry.Progress + 1
				if err := playEpisode(entry.Title, ep); err != nil {
					m.err = fmt.Errorf("play: %w", err)
					return m, nil
				}
				entry.Progress = ep
				if _, err := m.store.Update(entry.Title, entry); err != nil {
					m.err = fmt.Errorf("update progress: %w", err)
					return m, nil
				}
				m.refreshList()
				m.status = fmt.Sprintf("playing episode %d of %s", ep, entry.Title)
			}
			return m, nil
		case "space":
			if entry, ok := m.list.selected(); ok {
				m.mode = modeSelectStatus
				m.selectStatus = newSelectStatusModel(entry)
			}
			return m, nil
		case "f":
			m.mode = modeFilterStatus
			m.filterStatus = newFilterStatusModel(m.list.filterIndex)
			return m, nil
		case "G":
			last := len(m.list.table.GetVisibleRows()) - 1
			m.list.table = m.list.table.WithHighlightedRow(last)
			return m, nil
		case "ctrl+d":
			m.list.table = m.list.table.WithHighlightedRow(m.list.table.GetHighlightedRowIndex() + halfPage(m.list.table))
			return m, nil
		case "ctrl+u":
			m.list.table = m.list.table.WithHighlightedRow(m.list.table.GetHighlightedRowIndex() - halfPage(m.list.table))
			return m, nil
		}
	}

	tm, cmd := m.list.table.Update(msg)
	m.list.table = tm
	return m, cmd
}

func (m Model) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "y":
		if err := m.store.Delete(m.confirmTitle); err != nil {
			m.err = fmt.Errorf("delete: %w", err)
		} else {
			m.status = "deleted " + m.confirmTitle
			m.refreshList()
		}
		m.mode = modeList
		return m, nil
	case "n", "esc":
		m.mode = modeList
		return m, nil
	}
	return m, nil
}
