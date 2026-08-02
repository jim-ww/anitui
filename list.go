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

	t := table.New([]table.Column{
		table.NewColumn(colStatus, "St", 4),
		table.NewFlexColumn(colTitle, "Title", 3).WithFiltered(true),
		table.NewColumn(colProgress, "Ep", 6),
		table.NewColumn(colRating, "Rating", 8),
	}).
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
	rows := make([]table.Row, len(m.entries))
	for i, a := range m.entries {
		rows[i] = table.NewRow(table.RowData{
			colStatus:   a.Status.Symbol(),
			colTitle:    a.Title,
			colProgress: strconv.Itoa(a.Progress),
			colRating:   ratingLabel(a.Rating),
		})
	}
	m.table = m.table.WithRows(rows)
	return m
}

func ratingLabel(r *float32) string {
	if r == nil {
		return "–"
	}
	return fmt.Sprintf("%.1f", *r)
}

func (m listModel) resize(width, height int) listModel {
	m.table = m.table.WithTargetWidth(width).WithPageSize(max(1, height-6))
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
	help := helpStyle.Render("a add  enter/e edit  d delete  p play  space cycle status  f filter  / search  q quit")
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
				entry.Status = entry.Status.Next()
				updated, err := m.store.Update(entry.Title, entry)
				if err != nil {
					m.err = fmt.Errorf("update status: %w", err)
					return m, nil
				}
				m.refreshList()
				m.status = updated.Title + " -> " + updated.Status.String()
			}
			return m, nil
		case "f":
			m.list.filterIndex = (m.list.filterIndex + 1) % len(statusFilters)
			m.refreshList()
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
