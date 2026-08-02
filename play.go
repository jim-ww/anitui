package main

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// playPromptModel asks which episode to play, pre-filled with the entry's
// current progress + 1, so a plain enter plays the next episode but any
// episode can be typed in instead (e.g. to rewatch one).
type playPromptModel struct {
	title string
	input textinput.Model
}

func newPlayPromptModel(entry Anime) playPromptModel {
	suggested := 1
	if entry.Progress != nil {
		suggested = *entry.Progress + 1
	}
	ti := textinput.New()
	ti.SetValue(strconv.Itoa(suggested))
	ti.Focus()
	return playPromptModel{title: entry.Title, input: ti}
}

func (p playPromptModel) init() tea.Cmd {
	return p.input.Focus()
}

func (m Model) updatePlayPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	p := m.play

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.mode = modeList
			return m, nil
		case "enter":
			ep, err := strconv.Atoi(strings.TrimSpace(p.input.Value()))
			if err != nil {
				m.err = fmt.Errorf("episode: %w", err)
				return m, nil
			}
			entry, err := m.store.FindByTitle(p.title)
			if err != nil {
				m.err = fmt.Errorf("play: %w", err)
				m.mode = modeList
				return m, nil
			}
			if err := playEpisode(entry.Title, ep); err != nil {
				m.err = fmt.Errorf("play: %w", err)
				return m, nil
			}
			entry.Progress = &ep
			if _, err := m.store.Update(entry.Title, entry); err != nil {
				m.err = fmt.Errorf("update progress: %w", err)
				return m, nil
			}
			m.refreshList()
			m.status = fmt.Sprintf("playing episode %d of %s", ep, entry.Title)
			m.mode = modeList
			return m, nil
		}
	}

	updated, cmd := p.input.Update(msg)
	p.input = updated
	m.play = p
	return m, cmd
}

func (p playPromptModel) View() string {
	sb := new(strings.Builder)
	fmt.Fprintln(sb, titleStyle.Render("Play "+p.title))
	fmt.Fprintln(sb)
	fmt.Fprintln(sb, fieldLabelStyle.Render("episode: ")+p.input.View())
	fmt.Fprintln(sb)
	fmt.Fprint(sb, helpStyle.Render("enter play  esc cancel"))
	return sb.String()
}
