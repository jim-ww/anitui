package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// formModel handles the add/edit entry form.
type formModel struct {
	id     string
	status Status

	focused int
	inputs  []textinput.Model
}

const (
	fTitle = iota
	fProgress
	fRating
	fNotes
	fCount
)

func newFormModel(e Anime) formModel {
	inputs := []textinput.Model{
		makeInput("title", e.Title, 60),
		makeInput("episodes watched", fmtInt(e.Progress), 6),
		makeInput("score 0–10", fmtRatingRaw(e.Rating), 6),
		makeInput("notes", e.Notes, 80),
	}
	f := formModel{id: e.ID, status: e.Status(), inputs: inputs}
	f.focusField(0)
	return f
}

func makeInput(placeholder, value string, width int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetWidth(width)
	ti.SetValue(value)
	return ti
}

func (f *formModel) View() string {
	label := "Add Entry"
	if f.id != "" {
		label = "Edit Entry"
	}
	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  "+label) + "\n\n")

	fieldLabels := [fCount]string{"Title", "Progress", "Rating", "Notes"}
	for i, inp := range f.inputs {
		cursor := "  "
		if i == f.focused {
			cursor = " ▸"
		}
		b.WriteString(formLabelStyle.Render(fieldLabels[i]))
		b.WriteString(inp.View())
		b.WriteString(cursor + "\n")
	}

	b.WriteString("\n")
	b.WriteString(formLabelStyle.Render("Status"))
	b.WriteString(styledStatus(f.status))
	b.WriteString("  " + helpStyle.Render("← →"))
	b.WriteString("\n")
	return formBoxStyle.Render(b.String())
}

func (f *formModel) nextField()     { f.focusField((f.focused + 1) % fCount) }
func (f *formModel) prevField()     { f.focusField((f.focused - 1 + fCount) % fCount) }
func (f *formModel) nextStatus()    { f.status = f.status.Next() }
func (f *formModel) prevStatus() {
	list := StatusList()
	for i, s := range list {
		if s == f.status {
			f.status = list[(i-1+len(list))%len(list)]
			return
		}
	}
	f.status = list[0]
}

func (f *formModel) focusField(i int) {
	for j := range f.inputs {
		f.inputs[j].Blur()
	}
	f.focused = i
	f.inputs[i].Focus()
}

func (f *formModel) build() (Anime, error) {
	progress, err := parseInt(f.inputs[fProgress].Value())
	if err != nil {
		return Anime{}, fmt.Errorf("progress must be a number")
	}
	rating, err := parseFloat32(f.inputs[fRating].Value())
	if err != nil {
		return Anime{}, fmt.Errorf("rating must be a number")
	}
	title := strings.TrimSpace(f.inputs[fTitle].Value())
	if title == "" {
		return Anime{}, fmt.Errorf("title cannot be empty")
	}
	return Anime{
		ID:       f.id,
		Title:    title,
		Progress: progress,
		Rating:   rating,
		Notes:    strings.TrimSpace(f.inputs[fNotes].Value()),
		Watches:  []WatchRecord{{Status: f.status}},
	}, nil
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m *RootModel) openAdd() {
	m.form = newFormModel(Anime{})
	m.mode = modeAdd
	m.status = ""
	m.errText = ""
}

func (m *RootModel) openEdit() {
	entry, ok := m.list.selectedEntry()
	if !ok {
		m.setError("nothing selected")
		return
	}
	m.form = newFormModel(entry)
	m.mode = modeEdit
	m.status = ""
	m.errText = ""
}

func (m *RootModel) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if ok {
		switch {
		case key.Matches(kp, m.km.Quit):
			return m, tea.Quit
		case key.Matches(kp, m.km.Cancel):
			m.mode = modeList
			m.setStatus("cancelled")
			return m, nil
		case kp.String() == "tab" || key.Matches(kp, m.km.Down):
			m.form.nextField()
			return m, nil
		case kp.String() == "shift+tab" || key.Matches(kp, m.km.Up):
			m.form.prevField()
			return m, nil
		case kp.String() == "left":
			m.form.prevStatus()
			return m, nil
		case kp.String() == "right":
			m.form.nextStatus()
			return m, nil
		case key.Matches(kp, m.km.Play):
			m.saveForm()
			return m, nil
		}
	}
	var cmds []tea.Cmd
	for i := range m.form.inputs {
		var cmd tea.Cmd
		m.form.inputs[i], cmd = m.form.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *RootModel) saveForm() {
	entry, err := m.form.build()
	if err != nil {
		m.setError(err.Error())
		return
	}
	highlighted := m.list.highlightedIndex()
	switch m.mode {
	case modeAdd:
		if _, err := m.store.Add(entry); err != nil {
			m.setError(err.Error())
			return
		}
		m.list.applyFilters()
		highlighted = len(m.list.filtered) - 1
		m.setStatus("entry added")
	case modeEdit:
		existing, ok := m.store.EntryByID(m.form.id)
		if !ok {
			m.setError("entry no longer exists")
			return
		}
		// preserve watch history; only update current status
		entry.Watches = existing.Watches
		if len(entry.Watches) > 0 {
			entry.Watches[len(entry.Watches)-1].Status = m.form.status
		}
		if _, err := m.store.Update(m.form.id, entry); err != nil {
			m.setError(err.Error())
			return
		}
		m.list.applyFilters()
		m.setStatus("entry updated")
	}
	m.mode = modeList
	m.list.rebuildTable(highlighted)
}

// ── Delete ────────────────────────────────────────────────────────────────────

func (m *RootModel) openConfirmDelete() {
	entry, ok := m.list.selectedEntry()
	if !ok {
		m.setError("nothing selected")
		return
	}
	m.mode = modeConfirmDelete
	m.status = fmt.Sprintf("delete %q?", entry.Title)
	m.errText = ""
}

func (m *RootModel) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if key.Matches(kp, m.km.Confirm) {
		m.doDelete()
	} else {
		m.mode = modeList
		m.setStatus("cancelled")
	}
	return m, nil
}

func (m *RootModel) doDelete() {
	entry, ok := m.list.selectedEntry()
	if !ok {
		m.setError("nothing selected")
		m.mode = modeList
		return
	}
	highlighted := m.list.highlightedIndex()
	if err := m.store.Delete(entry.ID); err != nil {
		m.setError(err.Error())
		m.mode = modeList
		return
	}
	m.list.applyFilters()
	m.list.rebuildTable(highlighted)
	m.setStatus(fmt.Sprintf("deleted %q", entry.Title))
	m.mode = modeList
}

// ── Progress / play ───────────────────────────────────────────────────────────

func (m *RootModel) adjustProgress(delta int) {
	entry, ok := m.list.selectedEntry()
	if !ok {
		m.setError("nothing selected")
		return
	}
	updated, err := m.store.AdjustProgress(entry.ID, delta)
	if err != nil {
		m.setError(err.Error())
		return
	}
	m.list.applyFilters()
	m.list.rebuildTable(m.list.highlightedIndex())
	m.setStatus(fmt.Sprintf("%q  ep %d", updated.Title, updated.Progress))
}

func (m *RootModel) playSelected() {
	entry, ok := m.list.selectedEntry()
	if !ok {
		m.setError("nothing selected")
		return
	}
	args := []string{entry.Title}
	if entry.Progress > 0 {
		args = append(args, "-e", strconv.Itoa(entry.Progress))
	}
	if err := exec.Command("ani-cli", args...).Start(); err != nil {
		m.setError(err.Error())
		return
	}
	m.setStatus(fmt.Sprintf("▶ ani-cli %q ep %d", entry.Title, entry.Progress))
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func fmtProgress(v int) string {
	if v == 0 {
		return "·"
	}
	return strconv.Itoa(v)
}

func fmtRating(v float32) string {
	if v == 0 {
		return "·"
	}
	return strconv.FormatFloat(float64(v), 'f', -1, 32)
}

func fmtRatingRaw(v float32) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatFloat(float64(v), 'f', -1, 32)
}

func fmtRewatch(n int) string {
	if n == 0 {
		return "·"
	}
	return strconv.Itoa(n)
}

func fmtInt(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

func parseInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int(f), nil
}

func parseFloat32(s string) (float32, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return 0, err
	}
	return float32(f), nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
