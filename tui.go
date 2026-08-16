package main

import (
	tea "charm.land/bubbletea/v2"
)

type mode int

const (
	modeList mode = iota
	modeForm
	modeConfirmDelete
	modeSelectStatus
	modeFilterStatus
	modeInfo
	modeSort
	modeStats
	modePlayPrompt
)

// Model is the root Bubble Tea model wiring the list and form sub-views.
type Model struct {
	store *Store

	mode         mode
	list         listModel
	form         formModel
	selectStatus selectStatusModel
	filterStatus filterStatusModel
	info         Anime
	sort         sortModel
	play         playPromptModel

	confirmTitle string
	status       string
	err          error

	width, height int
}

func newModel(s *Store) Model {
	m := Model{store: s, mode: modeList}
	m.list = newListModel(s)
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list = m.list.resize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case playFinishedMsg:
		return m.handlePlayFinished(msg), nil
	}

	switch m.mode {
	case modeList:
		return m.updateList(msg)
	case modeForm:
		return m.updateForm(msg)
	case modeConfirmDelete:
		return m.updateConfirmDelete(msg)
	case modeSelectStatus:
		return m.updateSelectStatus(msg)
	case modeFilterStatus:
		return m.updateFilterStatus(msg)
	case modeInfo:
		return m.updateInfo(msg)
	case modeSort:
		return m.updateSort(msg)
	case modeStats:
		return m.updateStats(msg)
	case modePlayPrompt:
		return m.updatePlayPrompt(msg)
	}
	return m, nil
}

func (m Model) View() tea.View {
	var content string
	switch m.mode {
	case modeForm:
		content = m.form.View()
	case modeConfirmDelete:
		content = m.list.View() + "\n" + warnStyle.Render("Delete \""+m.confirmTitle+"\"? [y/n]")
	case modeSelectStatus:
		content = overlay(m.list.View(), m.selectStatus.View(), m.width, m.height)
	case modeFilterStatus:
		content = overlay(m.list.View(), m.filterStatus.View(), m.width, m.height)
	case modeInfo:
		content = infoView(m.info)
	case modeSort:
		content = overlay(m.list.View(), m.sort.View(), m.width, m.height)
	case modeStats:
		content = overlay(m.list.View(), statsView(m.store.Entries(nil)), m.width, m.height)
	case modePlayPrompt:
		content = overlay(m.list.View(), m.play.View(), m.width, m.height)
	default:
		content = m.list.View()
	}
	if m.status != "" {
		content += "\n" + dimStyle.Render(m.status)
	}
	if m.err != nil {
		content += "\n" + warnStyle.Render(m.err.Error())
	}
	return tea.NewView(content)
}

func (m *Model) refreshList() {
	m.list = m.list.reload(m.store)
}
