package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// appMode enumerates the top-level UI states.
type appMode int

const (
	modeList appMode = iota
	modeSearch
	modeStatusSelect
	modeConfirmDelete
	modeAdd
	modeEdit
	modeWatches
	modeWatchEdit
)

// RootModel is the top-level Bubble Tea model. It holds shared state and
// delegates Update/View to the active mode's handler functions.
type RootModel struct {
	width, height int

	store *Store
	km    KeyMap

	mode    appMode
	status  string // bottom info line
	errText string // bottom error line (takes priority over status)

	// sub-models, populated when their mode is active
	list    listModel
	search  searchModel
	form    formModel
	watches watchesModel
}

func NewRootModel(store *Store, cfg *Config) *RootModel {
	m := &RootModel{
		width:  80,
		height: 24,
		store:  store,
		km:     cfg.Keys,
		status: fmt.Sprintf("loaded %d entries · %s", len(store.Entries()), store.Path()),
	}
	m.list = newListModel(store, cfg.Keys)
	return m
}

func (m *RootModel) Init() tea.Cmd {
	return tea.ClearScreen
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m *RootModel) View() tea.View {
	var b strings.Builder

	b.WriteString(m.headerLine())
	b.WriteString("\n")
	b.WriteString(m.bodyView())
	b.WriteString("\n")
	b.WriteString(m.helpLine())
	b.WriteString("\n")
	b.WriteString(m.statusLine())

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m *RootModel) headerLine() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("  anitui"))
	b.WriteString("  ")

	count := m.list.countLabel()
	b.WriteString(helpStyle.Render("(" + count + ")"))

	if m.list.filterStatus != nil {
		b.WriteString("  " + styledStatus(*m.list.filterStatus))
	}
	if m.list.searchQuery != "" {
		b.WriteString("  " + searchStyle.Render("/"+m.list.searchQuery))
		if n := len(m.list.searchMatches); n > 0 {
			b.WriteString(helpStyle.Render(fmt.Sprintf(" [%d/%d]", m.list.searchCursor+1, n)))
		} else {
			b.WriteString(helpStyle.Render(" [no matches]"))
		}
	}
	return b.String()
}

func (m *RootModel) bodyView() string {
	switch m.mode {
	case modeSearch:
		return m.list.tableView() + "\n" + m.search.inputLine()
	case modeStatusSelect:
		return m.list.tableView() + "\n" + statusSelectView(m.list.statusCursor)
	case modeConfirmDelete:
		return m.list.tableView()
	case modeAdd, modeEdit:
		return m.form.View()
	case modeWatches:
		return m.watchesPanelView()
	case modeWatchEdit:
		return m.watches.editForm.View()
	default:
		return m.list.tableView()
	}
}

func (m *RootModel) helpLine() string {
	switch m.mode {
	case modeSearch:
		return helpStyle.Render("  enter confirm  esc clear  n/N next/prev")
	case modeStatusSelect:
		return helpStyle.Render("  ↑/k ↓/j move  enter confirm  esc cancel")
	case modeConfirmDelete:
		return errorStyle.Render("  delete? ") + helpStyle.Render("y confirm  any other key cancel")
	case modeAdd, modeEdit:
		return helpStyle.Render("  tab/↑↓ field  ←/→ status  enter save  esc cancel")
	case modeWatches:
		return helpStyle.Render("  ↑/k ↓/j  a add  e edit  d del  esc back")
	case modeWatchEdit:
		return helpStyle.Render("  tab/↑↓ field  ←/→ status  enter save  esc cancel")
	default:
		return m.list.helpLine(m.km)
	}
}

func (m *RootModel) statusLine() string {
	if m.errText != "" {
		return errorStyle.Render("  ✗ " + m.errText)
	}
	if m.status != "" {
		return subtleStyle.Render("  " + m.status)
	}
	return ""
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m *RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle window resize in all modes.
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = sz.Width
		m.height = sz.Height
		m.list.resize(sz.Width, sz.Height)
		return m, nil
	}

	switch m.mode {
	case modeList:
		return m.updateList(msg)
	case modeSearch:
		return m.updateSearch(msg)
	case modeStatusSelect:
		return m.updateStatusSelect(msg)
	case modeConfirmDelete:
		return m.updateConfirmDelete(msg)
	case modeAdd, modeEdit:
		return m.updateForm(msg)
	case modeWatches:
		return m.updateWatches(msg)
	case modeWatchEdit:
		return m.updateWatchEdit(msg)
	}
	return m, nil
}

// setStatus sets the info line; clears error.
func (m *RootModel) setStatus(s string) {
	m.status = s
	m.errText = ""
}

// setError sets the error line.
func (m *RootModel) setError(s string) {
	m.errText = s
}
