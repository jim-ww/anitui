package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// formModel is a simple field-by-field editor for a single Anime entry.
// Text fields are backed by a textinput.Model; Status is edited by
// cycling with left/right instead, since it has a fixed set of values.
type formModel struct {
	entry     Anime
	origTitle string
	isNew     bool

	fields []Field
	cursor int
	inputs map[Field]*textinput.Model

	err error
}

func newAddForm(defaultEntry Anime) formModel {
	return newForm(defaultEntry, true)
}

func newEditForm(entry Anime) formModel {
	return newForm(entry, false)
}

func newForm(entry Anime, isNew bool) formModel {
	f := formModel{
		entry:     entry,
		origTitle: entry.Title,
		isNew:     isNew,
		fields:    FieldList(),
		inputs:    make(map[Field]*textinput.Model),
	}
	for _, field := range f.fields {
		if field == FieldStatus || field == FieldReleasing {
			continue
		}
		ti := textinput.New()
		ti.SetValue(fieldValue(entry, field))
		f.inputs[field] = &ti
	}
	return f
}

func fieldValue(a Anime, field Field) string {
	switch field {
	case FieldTitle:
		return a.Title
	case FieldProgress:
		return formatProgress(a.Progress)
	case FieldWatchSessions:
		return formatSessions(a.WatchSessions)
	case FieldRating:
		if a.Rating == nil {
			return ""
		}
		return strconv.FormatFloat(float64(*a.Rating), 'f', -1, 32)
	case FieldNotes:
		return a.NotesText()
	default:
		return ""
	}
}

func (f formModel) init() tea.Cmd {
	if ti, ok := f.inputs[f.fields[0]]; ok {
		return ti.Focus()
	}
	return nil
}

func (f *formModel) focusCursor() tea.Cmd {
	var cmd tea.Cmd
	for field, ti := range f.inputs {
		if field == f.fields[f.cursor] {
			cmd = ti.Focus()
		} else {
			ti.Blur()
		}
	}
	return cmd
}

func (m Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.err = nil
	f := m.form

	keyMsg, isKey := msg.(tea.KeyPressMsg)
	if isKey {
		switch keyMsg.String() {
		case "esc":
			m.mode = modeList
			return m, nil
		case "ctrl+s":
			entry, err := f.build()
			if err != nil {
				f.err = err
				m.form = f
				return m, nil
			}
			if f.isNew {
				if _, err := m.store.Add(entry); err != nil {
					f.err = err
					m.form = f
					return m, nil
				}
			} else {
				if _, err := m.store.Update(f.origTitle, entry); err != nil {
					f.err = err
					m.form = f
					return m, nil
				}
			}
			m.mode = modeList
			m.status = "saved " + entry.Title
			m.refreshList()
			return m, nil
		case "up":
			f.cursor = (f.cursor - 1 + len(f.fields)) % len(f.fields)
			cmd := f.focusCursor()
			m.form = f
			return m, cmd
		case "down", "tab":
			f.cursor = (f.cursor + 1) % len(f.fields)
			cmd := f.focusCursor()
			m.form = f
			return m, cmd
		case "left", "right":
			switch f.fields[f.cursor] {
			case FieldStatus:
				if keyMsg.String() == "left" {
					f.entry.Status = f.entry.Status.Prev()
				} else {
					f.entry.Status = f.entry.Status.Next()
				}
				m.form = f
				return m, nil
			case FieldReleasing:
				f.entry.Notes = withActivelyReleasing(f.entry.Notes, !f.entry.ActivelyReleasing())
				m.form = f
				return m, nil
			}
		case "ctrl+t":
			ti := f.inputs[FieldWatchSessions]
			ti.SetValue(appendToday(ti.Value(), f.entry.Status == StatusCompleted))
			m.form = f
			return m, nil
		}
	}

	field := f.fields[f.cursor]
	if field != FieldStatus && field != FieldReleasing {
		ti := f.inputs[field]
		updated, cmd := ti.Update(msg)
		*ti = updated
		m.form = f
		return m, cmd
	}
	m.form = f
	return m, nil
}

// appendToday adds today's date to a watch_sessions field value. If completed
// is true the entry has just finished a watch-through, so today starts a new
// session (a rewatch); otherwise it's added to the current, still-in-progress
// session. It's a no-op if today is already the most recent date recorded,
// so mashing ctrl+t doesn't pile up duplicate dates.
func appendToday(value string, completed bool) string {
	today := time.Now().Format(DateDisplayFormat)
	if value == "" {
		return today
	}
	if lastSessionDate(value) == today {
		return value
	}
	if completed {
		return value + sessionSep + today
	}
	return value + dateSep + today
}

// lastSessionDate returns the most recently recorded date in a
// watch_sessions field value, i.e. the last date of the last session.
func lastSessionDate(value string) string {
	sessions := strings.Split(value, sessionSep)
	dates := strings.Split(sessions[len(sessions)-1], dateSep)
	return dates[len(dates)-1]
}

// build parses all field inputs into a Anime, validating as it goes.
func (f formModel) build() (Anime, error) {
	a := f.entry

	title := strings.TrimSpace(f.inputs[FieldTitle].Value())
	if title == "" {
		return Anime{}, errors.New("title cannot be empty")
	}
	a.Title = title

	progressStr := strings.TrimSpace(f.inputs[FieldProgress].Value())
	if progressStr == "" {
		a.Progress = nil
	} else {
		progress, err := strconv.Atoi(progressStr)
		if err != nil {
			return Anime{}, fmt.Errorf("progress: %w", err)
		}
		a.Progress = &progress
	}

	var err error
	if a.WatchSessions, err = parseSessions(f.inputs[FieldWatchSessions].Value()); err != nil {
		return Anime{}, fmt.Errorf("watch_sessions: %w", err)
	}

	ratingStr := strings.TrimSpace(f.inputs[FieldRating].Value())
	if ratingStr == "" {
		a.Rating = nil
	} else {
		r, err := strconv.ParseFloat(ratingStr, 32)
		if err != nil {
			return Anime{}, fmt.Errorf("rating: %w", err)
		}
		rating := float32(r)
		a.Rating = &rating
	}

	a.Notes = withActivelyReleasing(f.inputs[FieldNotes].Value(), f.entry.ActivelyReleasing())

	return a, nil
}

func (f formModel) View() string {
	title := "Edit"
	if f.isNew {
		title = "Add"
	}
	sb := new(strings.Builder)
	fmt.Fprintln(sb, titleStyle.Render(title+" anime"))
	fmt.Fprintln(sb)

	for i, field := range f.fields {
		label := fieldLabelStyle
		if i == f.cursor {
			label = fieldSelectedStyle
		}
		fmt.Fprint(sb, label.Render(fmt.Sprintf("  %-14s", field.String()+":")))
		switch field {
		case FieldStatus:
			fmt.Fprintln(sb, fieldValueStyle.Render(f.entry.Status.String()+" ("+f.entry.Status.Symbol()+")"))
		case FieldReleasing:
			value := "no"
			if f.entry.ActivelyReleasing() {
				value = "yes"
			}
			fmt.Fprintln(sb, fieldValueStyle.Render(value))
		default:
			fmt.Fprintln(sb, f.inputs[field].View())
		}
	}

	fmt.Fprintln(sb)
	if f.err != nil {
		fmt.Fprintln(sb, warnStyle.Render(f.err.Error()))
	}
	fmt.Fprintln(sb, dimStyle.Render(fmt.Sprintf(
		"date format: YYYY-MM-DD, e.g. %s",
		time.Now().Format(DateDisplayFormat),
	)))
	fmt.Fprintln(sb, helpStyle.Render("↑/↓ move  ←/→ change status/releasing  ctrl+t add today  ctrl+s save  esc cancel"))
	return sb.String()
}
