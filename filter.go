package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	tea "charm.land/bubbletea/v2"
)

// filterStatusModel is a popup for picking which status to filter the list
// by. Replaces blindly cycling through statuses one "f" press at a time,
// which gave no visibility into what the next press would land on.
type filterStatusModel struct {
	cursor int
}

func newFilterStatusModel(currentIndex int) filterStatusModel {
	return filterStatusModel{cursor: currentIndex}
}

func (m Model) updateFilterStatus(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	f := m.filterStatus
	switch keyMsg.String() {
	case "up", "k":
		f.cursor = (f.cursor - 1 + len(statusFilters)) % len(statusFilters)
		m.filterStatus = f
		return m, nil
	case "down", "j":
		f.cursor = (f.cursor + 1) % len(statusFilters)
		m.filterStatus = f
		return m, nil
	case "enter":
		m.list.filterIndex = f.cursor
		m.refreshList()
		m.mode = modeList
		return m, nil
	case "esc":
		m.mode = modeList
		return m, nil
	}
	return m, nil
}

func (m filterStatusModel) View() string {
	sb := new(strings.Builder)
	fmt.Fprintln(sb, titleStyle.Render("Filter by status"))
	for i, s := range statusFilters {
		text := "all"
		color := lipgloss.Color("7")
		if s != nil {
			text = s.Symbol() + " " + s.String()
			color = s.Color()
		}
		label := lipgloss.NewStyle().Foreground(color)
		if i == m.cursor {
			label = label.Bold(true).Underline(true)
		}
		fmt.Fprintln(sb, label.Render("  "+text))
	}
	fmt.Fprint(sb, helpStyle.Render("↑/↓ move  enter apply  esc cancel"))
	return sb.String()
}
