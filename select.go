package main

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"

	tea "charm.land/bubbletea/v2"
)

// selectStatusModel is a small popup for picking a new Status for one entry.
// Unlike list navigation, the choice only takes effect on enter — moving the
// cursor with up/down must not touch the store.
type selectStatusModel struct {
	title   string
	options []Status
	cursor  int
}

func newSelectStatusModel(entry Anime) selectStatusModel {
	options := StatusList()
	cursor := slices.Index(options, entry.Status)
	if cursor == -1 {
		cursor = 0
	}
	return selectStatusModel{title: entry.Title, options: options, cursor: cursor}
}

func (m Model) updateSelectStatus(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	s := m.selectStatus
	switch keyMsg.String() {
	case "up", "k":
		s.cursor = (s.cursor - 1 + len(s.options)) % len(s.options)
		m.selectStatus = s
		return m, nil
	case "down", "j":
		s.cursor = (s.cursor + 1) % len(s.options)
		m.selectStatus = s
		return m, nil
	case "enter":
		entry, err := m.store.FindByTitle(s.title)
		if err != nil {
			m.err = fmt.Errorf("select status: %w", err)
			m.mode = modeList
			return m, nil
		}
		entry.Status = s.options[s.cursor]
		updated, err := m.store.Update(entry.Title, entry)
		if err != nil {
			m.err = fmt.Errorf("update status: %w", err)
			m.mode = modeList
			return m, nil
		}
		m.refreshList()
		m.status = updated.Title + " -> " + updated.Status.String()
		m.mode = modeList
		return m, nil
	case "esc":
		m.mode = modeList
		return m, nil
	}
	return m, nil
}

func (m selectStatusModel) View() string {
	sb := new(strings.Builder)
	fmt.Fprintln(sb, titleStyle.Render("Status: "+m.title))
	for i, s := range m.options {
		label := lipgloss.NewStyle().Foreground(s.Color())
		if i == m.cursor {
			label = label.Bold(true).Underline(true)
		}
		fmt.Fprintln(sb, label.Render(fmt.Sprintf("  %s %s", s.Symbol(), s.String())))
	}
	fmt.Fprint(sb, helpStyle.Render("↑/↓ move  enter select  esc cancel"))
	return sb.String()
}
