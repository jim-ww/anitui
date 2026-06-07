package main

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/evertras/bubble-table/table"
)

// listModel owns the table widget and filter/search state.
type listModel struct {
	store    *Store
	km       KeyMap
	tbl      table.Model
	width    int
	height   int
	filtered []Anime

	filterStatus *Status
	searchQuery  string
	searchMatches []int
	searchCursor  int
	statusCursor  int
}

func newListModel(store *Store, km KeyMap) listModel {
	lm := listModel{
		store:  store,
		km:     km,
		width:  80,
		height: 24,
	}
	lm.applyFilters()
	lm.rebuildTable(0)
	return lm
}

// ── Filtering ─────────────────────────────────────────────────────────────────

func (lm *listModel) applyFilters() {
	all := lm.store.Entries()
	q := strings.ToLower(lm.searchQuery)
	lm.filtered = lm.filtered[:0]
	for _, e := range all {
		if lm.filterStatus != nil && e.Status() != *lm.filterStatus {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(e.Title), q) {
			continue
		}
		lm.filtered = append(lm.filtered, e)
	}
}

func (lm *listModel) cycleFilterStatus() {
	list := filterStatusList()
	for i, s := range list {
		cur := lm.filterStatus
		if (s == nil && cur == nil) || (s != nil && cur != nil && *s == *cur) {
			next := list[(i+1)%len(list)]
			lm.filterStatus = next
			return
		}
	}
	lm.filterStatus = nil
}

func filterStatusList() []*Status {
	statuses := StatusList()
	out := make([]*Status, 1+len(statuses))
	out[0] = nil
	for i := range statuses {
		s := statuses[i]
		out[i+1] = &s
	}
	return out
}

func (lm *listModel) buildSearchMatches() {
	q := strings.ToLower(lm.searchQuery)
	lm.searchMatches = lm.searchMatches[:0]
	if q == "" {
		return
	}
	for i, e := range lm.filtered {
		if strings.Contains(strings.ToLower(e.Title), q) {
			lm.searchMatches = append(lm.searchMatches, i)
		}
	}
}

func (lm *listModel) searchJump(dir int) {
	if len(lm.searchMatches) == 0 {
		return
	}
	lm.searchCursor = (lm.searchCursor + dir + len(lm.searchMatches)) % len(lm.searchMatches)
	lm.rebuildTable(lm.searchMatches[lm.searchCursor])
}

// ── Table ─────────────────────────────────────────────────────────────────────

func (lm *listModel) rebuildTable(highlighted int) {
	if highlighted >= len(lm.filtered) {
		highlighted = max(0, len(lm.filtered)-1)
	}
	rows := make([]table.Row, 0, len(lm.filtered))
	for _, e := range lm.filtered {
		cw := e.CurrentWatch()
		rows = append(rows, table.NewRow(table.RowData{
			colKeyStatus:    e.Status().Symbol() + " " + e.Status().String(),
			colKeyTitle:     e.Title,
			colKeyProgress:  fmtProgress(e.Progress),
			colKeyRating:    fmtRating(e.Rating),
			colKeyStartDate: fmtDateVal(cw.StartDate),
			colKeyEndDate:   fmtDateVal(cw.EndDate),
			colKeyRewatch:   fmtRewatch(e.TotalRewatch()),
			colKeyNotes:     e.Notes,
		}).WithStyle(rowStyleForStatus(e.Status())))
	}
	lm.tbl = table.New(animeColumns(lm.width)).
		WithRows(rows).
		WithTargetWidth(lm.width).
		WithHorizontalFreezeColumnCount(2).
		WithPageSize(max(3, lm.height-6)).
		Border(customBorder).
		Focused(true).
		WithAdditionalShortHelpKeys([]key.Binding{lm.km.Quit}).
		WithBaseStyle(baseStyle).
		HeaderStyle(headerStyle).
		HighlightStyle(highlightStyle).
		WithHighlightedRow(highlighted)
}

func (lm *listModel) tableView() string {
	return lm.tbl.View()
}

func (lm *listModel) resize(w, h int) {
	lm.width = w
	lm.height = h
	lm.rebuildTable(lm.highlightedIndex())
}

func (lm *listModel) highlightedIndex() int {
	return lm.tbl.GetHighlightedRowIndex()
}

func (lm *listModel) selectedEntry() (Anime, bool) {
	idx := lm.highlightedIndex()
	if idx < 0 || idx >= len(lm.filtered) {
		return Anime{}, false
	}
	return lm.filtered[idx], true
}

func (lm *listModel) countLabel() string {
	if len(lm.filtered) == len(lm.store.Entries()) {
		return fmt.Sprintf("%d", len(lm.filtered))
	}
	return fmt.Sprintf("%d/%d", len(lm.filtered), len(lm.store.Entries()))
}

func (lm *listModel) helpLine(km KeyMap) string {
	return helpStyle.Render(
		"  a add  e edit  d del  spc status  f filter  / search  w watches  +/- ep  ↵ play  q quit",
	)
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m *RootModel) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	km := m.km
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(kp, km.Quit):
			return m, tea.Quit
		case key.Matches(kp, km.Cancel):
			return m, tea.Quit

		case key.Matches(kp, km.Add):
			m.openAdd()
			return m, nil
		case key.Matches(kp, km.Edit):
			m.openEdit()
			return m, nil
		case key.Matches(kp, km.Delete):
			m.openConfirmDelete()
			return m, nil

		case key.Matches(kp, km.SetStatus):
			m.openStatusSelect()
			return m, nil
		case key.Matches(kp, km.FilterStatus):
			m.list.cycleFilterStatus()
			m.list.applyFilters()
			m.list.rebuildTable(0)
			m.setStatus("filter: " + m.list.filterLabel())
			return m, nil

		case key.Matches(kp, km.Search):
			m.openSearch()
			return m, nil
		case key.Matches(kp, km.SearchNext):
			m.list.searchJump(1)
			return m, nil
		case key.Matches(kp, km.SearchPrev):
			m.list.searchJump(-1)
			return m, nil

		case key.Matches(kp, km.Watches):
			m.openWatches()
			return m, nil

		case key.Matches(kp, km.ProgressUp):
			m.adjustProgress(1)
			return m, nil
		case key.Matches(kp, km.ProgressDown):
			m.adjustProgress(-1)
			return m, nil
		case key.Matches(kp, km.Play):
			m.playSelected()
			return m, nil

		case key.Matches(kp, km.HalfPageUp):
			m.jumpHalfPage(-1)
			return m, nil
		case key.Matches(kp, km.HalfPageDown):
			m.jumpHalfPage(1)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list.tbl, cmd = m.list.tbl.Update(msg)
	return m, cmd
}

func (m *RootModel) jumpHalfPage(dir int) {
	pageSize := max(1, m.height-6)
	half := max(1, pageSize/2)
	idx := m.list.highlightedIndex() + dir*half
	idx = clamp(idx, 0, len(m.list.filtered)-1)
	m.list.rebuildTable(idx)
}

// filterLabel returns a display name for the current status filter.
func (lm *listModel) filterLabel() string {
	if lm.filterStatus == nil {
		return "all"
	}
	return lm.filterStatus.String()
}
