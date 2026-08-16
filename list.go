package main

import (
	"fmt"
	"strconv"
	"time"

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
	colLast     = "last"
	colStarted  = "started"
	colRewatch  = "rewatch"
	// colNotes is never displayed (zero width, empty header) — it only
	// exists so the fuzzy filter searches notes text too, since bubble-table
	// matches over every filterable column's data regardless of whether
	// that column is actually rendered.
	colNotes = "notes"
)

// narrowWidthThreshold is the terminal width below which the status column
// falls back to a single symbol — spelling out "plan to watch" just doesn't
// fit a narrow terminal, and the symbols are only meant to be a space-saving
// stand-in, not the primary way statuses are shown.
const narrowWidthThreshold = 90

// columnsFor builds the table's columns for the current display state: wide
// picks a spelled-out or symbol status column, and dates picks between the
// default progress/rating columns and the "v"-toggled watch-history columns.
func columnsFor(wide, dates bool) []table.Column {
	statusCol := table.NewColumn(colStatus, "St", 4)
	if wide {
		statusCol = table.NewColumn(colStatus, "Status", 14)
	}
	title := table.NewFlexColumn(colTitle, "Title", 3).WithFiltered(true)
	notes := table.NewColumn(colNotes, "", 0).WithFiltered(true)

	if dates {
		return []table.Column{
			statusCol,
			title,
			table.NewColumn(colLast, "Last", 12),
			table.NewColumn(colStarted, "Started", 12),
			table.NewColumn(colRewatch, "RW", 4),
			notes,
		}
	}
	return []table.Column{
		statusCol,
		title,
		table.NewColumn(colProgress, "Ep", 6),
		table.NewColumn(colRating, "Rating", 8),
		notes,
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
	sortIndex   int
	width       int
	// dates toggles the Ep/Rating columns for Last/Started/RW (watch
	// history), so that info is visible without opening Edit. See "v".
	dates bool
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

	t := table.New(columnsFor(false, false)).
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
	m.entries = sortEntries(s.Entries(statusFilters[m.filterIndex]), sortOptions[m.sortIndex].key)
	return m.rebuild()
}

// rebuild recomputes columns and rows from the current width/dates display
// state. Called after anything that could change what should be on screen:
// a reload, a resize crossing the wide/narrow threshold, or toggling "v".
func (m listModel) rebuild() listModel {
	wide := m.width >= narrowWidthThreshold
	m.table = m.table.WithColumns(columnsFor(wide, m.dates)).WithRows(rowsFor(m.entries, wide, m.dates))
	return m
}

func rowsFor(entries []Anime, wide, dates bool) []table.Row {
	rows := make([]table.Row, len(entries))
	for i, a := range entries {
		statusLabel := a.Status.Symbol()
		if wide {
			statusLabel = a.Status.String()
		}
		data := table.RowData{
			colStatus: table.NewStyledCell(statusLabel, lipgloss.NewStyle().Foreground(a.Status.Color())),
			colTitle:  a.Title,
			colNotes:  a.Notes,
		}
		if dates {
			data[colLast] = dateLabel(a.LastWatch())
			data[colStarted] = dateLabel(a.StartedAt())
			data[colRewatch] = strconv.Itoa(a.TotalRewatch())
		} else {
			data[colProgress] = progressLabel(a.Progress)
			data[colRating] = ratingLabel(a.Rating)
		}
		rows[i] = table.NewRow(data)
	}
	return rows
}

// halfPage returns half the table's current page size, at least 1 row.
func halfPage(t table.Model) int {
	return max(1, t.PageSize()/2)
}

func progressLabel(p *int) string {
	if p == nil {
		return "–"
	}
	return strconv.Itoa(*p)
}

func ratingLabel(r *float32) string {
	if r == nil {
		return "–"
	}
	return fmt.Sprintf("%.1f", *r)
}

func dateLabel(t *time.Time) string {
	if t == nil {
		return "–"
	}
	return t.Format(DateDisplayFormat)
}

func (m listModel) resize(width, height int) listModel {
	m.width = width
	m.table = m.table.WithTargetWidth(width).WithPageSize(max(1, height-6))
	// entries is nil on the very first resize (before any reload), which is
	// fine: reload() runs right after and rebuilds anyway.
	if m.entries != nil {
		m = m.rebuild()
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
	helpText := "j/k move  ctrl+u/d halfpage  ctrl+b/f page  g/G top/bottom  a add  enter/e edit  d delete  p play  space status  f filter  s sort  v dates  i info  t stats  u undo  / search  q quit"
	if m.width > 0 && m.width < narrowWidthThreshold {
		// The full help line wraps on narrow terminals and throws off the
		// page-size math in resize (which assumes a fixed number of chrome
		// rows), so fall back to the essentials only.
		helpText = "a add  enter edit  d delete  p play  f filter  q quit"
	}
	help := helpStyle.Render(helpText)
	return header + "\n" + m.table.View() + "\n" + help
}

func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.status, m.err = "", nil

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && m.list.table.GetIsFilterInputFocused() && keyMsg.String() == "enter" {
		// Confirm the filter without going through the library's own
		// FilterBlur binding (see newListModel for why that's esc-only):
		// this blurs directly, in the same update as the keystroke, so
		// there's no gap where a stray subsequent key could be picked up
		// as a global shortcut before the blur takes effect.
		m.list.table = m.list.table.WithFilterInputValue(m.list.table.GetCurrentFilter())
		return m, nil
	}

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
				m.mode = modePlayPrompt
				m.play = newPlayPromptModel(entry)
				return m, m.play.init()
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
		case "v":
			m.list.dates = !m.list.dates
			m.list = m.list.rebuild()
			return m, nil
		case "i":
			if entry, ok := m.list.selected(); ok {
				m.mode = modeInfo
				m.info = entry
			}
			return m, nil
		case "s":
			m.mode = modeSort
			m.sort = newSortModel(m.list.sortIndex)
			return m, nil
		case "t":
			m.mode = modeStats
			return m, nil
		case "u":
			undone, err := m.store.Undo()
			if err != nil {
				m.err = fmt.Errorf("undo: %w", err)
				return m, nil
			}
			if undone {
				m.status = "undone"
				m.refreshList()
			} else {
				m.status = "nothing to undo"
			}
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
