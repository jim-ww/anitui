package main

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// searchModel holds state for the / search bar.
type searchModel struct {
	input textinput.Model
}

func newSearchModel() searchModel {
	ti := textinput.New()
	ti.Placeholder = "search titles..."
	ti.SetWidth(40)
	return searchModel{input: ti}
}

func (s searchModel) inputLine() string {
	return helpStyle.Render("  / ") + s.input.View()
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m *RootModel) openSearch() {
	m.search = newSearchModel()
	m.mode = modeSearch
	m.errText = ""
	// don't return a cmd — Focus() returns a cmd in some versions
	m.search.input.Focus()
}

func (m *RootModel) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(kp, m.km.Quit):
			return m, tea.Quit
		case key.Matches(kp, m.km.Cancel):
			m.list.searchQuery = ""
			m.search.input.SetValue("")
			m.list.searchMatches = nil
			m.list.applyFilters()
			m.list.rebuildTable(m.list.highlightedIndex())
			m.mode = modeList
			return m, nil
		case key.Matches(kp, m.km.Play):
			m.list.searchQuery = m.search.input.Value()
			m.list.applyFilters()
			m.list.buildSearchMatches()
			if len(m.list.searchMatches) > 0 {
				m.list.searchCursor = 0
				m.list.rebuildTable(m.list.searchMatches[0])
			} else {
				m.list.rebuildTable(0)
			}
			m.mode = modeList
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.search.input, cmd = m.search.input.Update(msg)
	// live preview
	m.list.searchQuery = m.search.input.Value()
	m.list.applyFilters()
	m.list.buildSearchMatches()
	if len(m.list.searchMatches) > 0 {
		m.list.searchCursor = 0
		m.list.rebuildTable(m.list.searchMatches[0])
	} else {
		m.list.rebuildTable(0)
	}
	return m, cmd
}
