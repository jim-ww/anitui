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

// playFinishedMsg reports the result of an ani-cli run started from the
// play prompt, along with the episode/title it was for so progress can be
// recorded once ani-cli exits.
type playFinishedMsg struct {
	title string
	ep    int
	err   error
}

// stepEpisode adds delta to the episode input, clamped to at least 1, so
// left/right can nudge the suggested episode without retyping it.
func stepEpisode(ti textinput.Model, delta int) textinput.Model {
	ep, err := strconv.Atoi(strings.TrimSpace(ti.Value()))
	if err != nil {
		return ti
	}
	ep = max(1, ep+delta)
	ti.SetValue(strconv.Itoa(ep))
	ti.CursorEnd()
	return ti
}

// handlePlayFinished applies the progress update once ani-cli exits. It's
// called from the top-level Update regardless of mode, since the prompt has
// already returned to modeList by the time this message arrives.
func (m Model) handlePlayFinished(msg playFinishedMsg) Model {
	if msg.err != nil {
		m.err = fmt.Errorf("play: %w", msg.err)
		return m
	}
	entry, err := m.store.FindByTitle(msg.title)
	if err != nil {
		m.err = fmt.Errorf("play: %w", err)
		return m
	}
	entry.Progress = &msg.ep
	if _, err := m.store.Update(entry.Title, entry); err != nil {
		m.err = fmt.Errorf("update progress: %w", err)
		return m
	}
	m.refreshList()
	m.status = fmt.Sprintf("played episode %d of %s", msg.ep, entry.Title)
	return m
}

// playExternally runs ani-cli in a separate terminal window, leaving the TUI
// live in this one, and reports its exit status via playFinishedMsg once the
// window closes.
func playExternally(title string, ep int) tea.Cmd {
	return func() tea.Msg {
		cmd, err := externalPlayCommand(title, ep)
		if err != nil {
			return playFinishedMsg{title: title, ep: ep, err: err}
		}
		err = cmd.Run()
		return playFinishedMsg{title: title, ep: ep, err: err}
	}
}

func (m Model) updatePlayPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	p := m.play

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			m.mode = modeList
			return m, nil
		case "left":
			p.input = stepEpisode(p.input, -1)
			m.play = p
			return m, nil
		case "right":
			p.input = stepEpisode(p.input, 1)
			m.play = p
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
			m.mode = modeList
			if m.externalTerminal {
				m.status = fmt.Sprintf("playing episode %d of %s in external terminal...", ep, entry.Title)
				return m, playExternally(entry.Title, ep)
			}
			m.status = fmt.Sprintf("playing episode %d of %s...", ep, entry.Title)
			return m, tea.ExecProcess(playCommand(entry.Title, ep), func(err error) tea.Msg {
				return playFinishedMsg{title: entry.Title, ep: ep, err: err}
			})
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
	fmt.Fprint(sb, helpStyle.Render("enter play  ←/→ episode  esc cancel"))
	return sb.String()
}
