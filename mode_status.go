package main

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// statusSelectView renders the status picker list (overlaid on the table).
func statusSelectView(cursor int) string {
	list := StatusList()
	var b strings.Builder
	for i, s := range list {
		if i == cursor {
			b.WriteString(highlightStyle.Render(" ▸ "+s.Symbol()+" "+s.String()) + "\n")
		} else {
			b.WriteString("   " + styledStatus(s) + "\n")
		}
	}
	return b.String()
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m *RootModel) openStatusSelect() {
	entry, ok := m.list.selectedEntry()
	if !ok {
		m.setError("nothing selected")
		return
	}
	list := StatusList()
	m.list.statusCursor = 0
	for i, s := range list {
		if s == entry.Status() {
			m.list.statusCursor = i
			break
		}
	}
	m.mode = modeStatusSelect
	m.status = ""
	m.errText = ""
}

func (m *RootModel) updateStatusSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	list := StatusList()
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(kp, m.km.Quit):
		return m, tea.Quit
	case key.Matches(kp, m.km.Cancel):
		m.mode = modeList
		m.setStatus("cancelled")
	case key.Matches(kp, m.km.Up):
		m.list.statusCursor = (m.list.statusCursor - 1 + len(list)) % len(list)
	case key.Matches(kp, m.km.Down):
		m.list.statusCursor = (m.list.statusCursor + 1) % len(list)
	case key.Matches(kp, m.km.Play):
		m.applyStatusSelect(list[m.list.statusCursor])
	}
	return m, nil
}

func (m *RootModel) applyStatusSelect(s Status) {
	entry, ok := m.list.selectedEntry()
	if !ok {
		m.setError("nothing selected")
		m.mode = modeList
		return
	}
	if len(entry.Watches) == 0 {
		entry.Watches = []WatchRecord{{Status: s}}
	} else {
		entry.Watches[len(entry.Watches)-1].Status = s
	}
	if _, err := m.store.Update(entry.ID, entry); err != nil {
		m.setError(err.Error())
		m.mode = modeList
		return
	}
	m.list.applyFilters()
	m.list.rebuildTable(m.list.highlightedIndex())
	m.setStatus(fmt.Sprintf("%q → %s", entry.Title, s))
	m.mode = modeList
}
