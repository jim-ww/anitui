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
		if field == FieldStatus {
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
		return strconv.Itoa(a.Progress)
	case FieldStartedAt:
		return formatDateField(a.StartedAt)
	case FieldLastWatch:
		return formatDateField(a.LastWatch)
	case FieldFinishedAt:
		return formatDateField(a.FinishedAt)
	case FieldRating:
		if a.Rating == nil {
			return ""
		}
		return strconv.FormatFloat(float64(*a.Rating), 'f', -1, 32)
	case FieldTotalRewatch:
		return strconv.Itoa(a.TotalRewatch)
	case FieldNotes:
		return a.Notes
	default:
		return ""
	}
}

func formatDateField(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(DateDisplayFormat)
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
			if f.fields[f.cursor] == FieldStatus {
				if keyMsg.String() == "left" {
					f.entry.Status = f.entry.Status.Prev()
				} else {
					f.entry.Status = f.entry.Status.Next()
				}
				m.form = f
				return m, nil
			}
		}
	}

	field := f.fields[f.cursor]
	if field != FieldStatus {
		ti := f.inputs[field]
		updated, cmd := ti.Update(msg)
		*ti = updated
		m.form = f
		return m, cmd
	}
	m.form = f
	return m, nil
}

// build parses all field inputs into a Anime, validating as it goes.
func (f formModel) build() (Anime, error) {
	a := f.entry

	title := strings.TrimSpace(f.inputs[FieldTitle].Value())
	if title == "" {
		return Anime{}, errors.New("title cannot be empty")
	}
	a.Title = title

	progress, err := strconv.Atoi(strings.TrimSpace(f.inputs[FieldProgress].Value()))
	if err != nil {
		return Anime{}, fmt.Errorf("progress: %w", err)
	}
	a.Progress = progress

	if a.StartedAt, err = parseDateField(f.inputs[FieldStartedAt].Value()); err != nil {
		return Anime{}, fmt.Errorf("started_at: %w", err)
	}
	if a.LastWatch, err = parseDateField(f.inputs[FieldLastWatch].Value()); err != nil {
		return Anime{}, fmt.Errorf("last_watch: %w", err)
	}
	if a.FinishedAt, err = parseDateField(f.inputs[FieldFinishedAt].Value()); err != nil {
		return Anime{}, fmt.Errorf("finished_at: %w", err)
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

	totalRewatch, err := strconv.Atoi(strings.TrimSpace(f.inputs[FieldTotalRewatch].Value()))
	if err != nil {
		return Anime{}, fmt.Errorf("total_rewatch: %w", err)
	}
	a.TotalRewatch = totalRewatch

	a.Notes = f.inputs[FieldNotes].Value()

	return a, nil
}

func parseDateField(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(DateDisplayFormat, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
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
		if field == FieldStatus {
			fmt.Fprintln(sb, fieldValueStyle.Render(f.entry.Status.String()+" ("+f.entry.Status.Symbol()+")"))
		} else {
			fmt.Fprintln(sb, f.inputs[field].View())
		}
	}

	fmt.Fprintln(sb)
	if f.err != nil {
		fmt.Fprintln(sb, warnStyle.Render(f.err.Error()))
	}
	fmt.Fprintln(sb, helpStyle.Render("↑/↓ move  ←/→ change status  ctrl+s save  esc cancel"))
	return sb.String()
}
