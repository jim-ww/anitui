package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// watchesModel holds the state for the watch history panel and its edit form.
type watchesModel struct {
	animeID string
	cursor  int

	// edit sub-form
	editIndex int // -1 = new
	editForm  watchFormModel
}

// watchFormModel edits a single WatchRecord.
type watchFormModel struct {
	status  Status
	focused int
	inputs  []textinput.Model // [startDate, endDate]
}

const (
	wfStart = iota
	wfEnd
	wfCount
)

func newWatchFormModel(w WatchRecord, index int) watchFormModel {
	inputs := []textinput.Model{
		makeInput("YYYY-MM-DD", formatTomlDate(w.StartDate), 12),
		makeInput("YYYY-MM-DD", formatTomlDate(w.EndDate), 12),
	}
	f := watchFormModel{status: w.Status, inputs: inputs}
	f.focusField(0)
	return f
}

func (f *watchFormModel) View() string {
	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  Watch Record") + "\n\n")
	fieldLabels := [wfCount]string{"Start date", "End date"}
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

func (f *watchFormModel) nextField()  { f.focusField((f.focused + 1) % wfCount) }
func (f *watchFormModel) prevField()  { f.focusField((f.focused - 1 + wfCount) % wfCount) }
func (f *watchFormModel) nextStatus() { f.status = f.status.Next() }
func (f *watchFormModel) prevStatus() {
	list := StatusList()
	for i, s := range list {
		if s == f.status {
			f.status = list[(i-1+len(list))%len(list)]
			return
		}
	}
	f.status = list[0]
}
func (f *watchFormModel) focusField(i int) {
	for j := range f.inputs {
		f.inputs[j].Blur()
	}
	f.focused = i
	f.inputs[i].Focus()
}

func (f *watchFormModel) build() (WatchRecord, error) {
	start, err := parseDate(f.inputs[wfStart].Value())
	if err != nil {
		return WatchRecord{}, fmt.Errorf("start date: %w", err)
	}
	end, err := parseDate(f.inputs[wfEnd].Value())
	if err != nil {
		return WatchRecord{}, fmt.Errorf("end date: %w", err)
	}
	return WatchRecord{Status: f.status, StartDate: start, EndDate: end}, nil
}

// ── Panel view ────────────────────────────────────────────────────────────────

func (m *RootModel) watchesPanelView() string {
	wm := m.watches
	entry, ok := m.store.EntryByID(wm.animeID)
	if !ok {
		return errorStyle.Render("  entry not found\n")
	}
	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  Watches — "+entry.Title) + "\n\n")
	if len(entry.Watches) == 0 {
		b.WriteString(helpStyle.Render("  (no watch records — press a to add)\n"))
		return b.String()
	}
	for i, w := range entry.Watches {
		cursor := "   "
		if i == wm.cursor {
			cursor = " ▸ "
		}
		start := fmtDateVal(w.StartDate)
		end := fmtDateVal(w.EndDate)
		sessions := fmt.Sprintf("(%d sessions)", len(w.rawDates))
		line := fmt.Sprintf("%s%s  %s → %s  %s\n",
			cursor,
			styledStatus(w.Status),
			subtleStyle.Render(start),
			subtleStyle.Render(end),
			helpStyle.Render(sessions),
		)
		b.WriteString(line)
	}
	return b.String()
}


// ── Update ────────────────────────────────────────────────────────────────────

func (m *RootModel) openWatches() {
	entry, ok := m.list.selectedEntry()
	if !ok {
		m.setError("nothing selected")
		return
	}
	m.watches = watchesModel{
		animeID: entry.ID,
		cursor:  max(0, len(entry.Watches)-1),
	}
	m.mode = modeWatches
	m.status = ""
	m.errText = ""
}

func (m *RootModel) updateWatches(msg tea.Msg) (tea.Model, tea.Cmd) {
	entry, ok := m.store.EntryByID(m.watches.animeID)
	if !ok {
		m.mode = modeList
		return m, nil
	}
	kp, ok2 := msg.(tea.KeyPressMsg)
	if !ok2 {
		return m, nil
	}
	switch {
	case key.Matches(kp, m.km.Quit):
		return m, tea.Quit
	case key.Matches(kp, m.km.Cancel):
		m.mode = modeList
	case key.Matches(kp, m.km.Up):
		if m.watches.cursor > 0 {
			m.watches.cursor--
		}
	case key.Matches(kp, m.km.Down):
		if m.watches.cursor < len(entry.Watches)-1 {
			m.watches.cursor++
		}
	case key.Matches(kp, m.km.Add):
		m.watches.editIndex = -1
		m.watches.editForm = newWatchFormModel(WatchRecord{Status: StatusWatching}, -1)
		m.mode = modeWatchEdit
	case key.Matches(kp, m.km.Edit):
		if m.watches.cursor >= 0 && m.watches.cursor < len(entry.Watches) {
			m.watches.editIndex = m.watches.cursor
			m.watches.editForm = newWatchFormModel(entry.Watches[m.watches.cursor], m.watches.cursor)
			m.mode = modeWatchEdit
		}
	case key.Matches(kp, m.km.Delete):
		m.deleteWatchRecord(entry)
	}
	return m, nil
}

func (m *RootModel) updateWatchEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if ok {
		switch {
		case key.Matches(kp, m.km.Quit):
			return m, tea.Quit
		case key.Matches(kp, m.km.Cancel):
			m.mode = modeWatches
			m.errText = ""
			return m, nil
		case kp.String() == "tab" || key.Matches(kp, m.km.Down):
			m.watches.editForm.nextField()
			return m, nil
		case kp.String() == "shift+tab" || key.Matches(kp, m.km.Up):
			m.watches.editForm.prevField()
			return m, nil
		case kp.String() == "left":
			m.watches.editForm.prevStatus()
			return m, nil
		case kp.String() == "right":
			m.watches.editForm.nextStatus()
			return m, nil
		case key.Matches(kp, m.km.Play):
			m.saveWatchEdit()
			return m, nil
		}
	}
	var cmds []tea.Cmd
	for i := range m.watches.editForm.inputs {
		var cmd tea.Cmd
		m.watches.editForm.inputs[i], cmd = m.watches.editForm.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *RootModel) saveWatchEdit() {
	entry, ok := m.store.EntryByID(m.watches.animeID)
	if !ok {
		m.setError("entry not found")
		return
	}
	rec, err := m.watches.editForm.build()
	if err != nil {
		m.setError(err.Error())
		return
	}
	if m.watches.editIndex < 0 {
		entry.Watches = append(entry.Watches, rec)
	} else {
		existing := entry.Watches[m.watches.editIndex]
		existing.Status = rec.Status
		existing.StartDate = rec.StartDate
		existing.EndDate = rec.EndDate
		existing.rawDates = rebuildRawDates(existing.rawDates, rec.StartDate, rec.EndDate)
		entry.Watches[m.watches.editIndex] = existing
	}
	if _, err := m.store.Update(entry.ID, entry); err != nil {
		m.setError(err.Error())
		return
	}
	m.list.applyFilters()
	m.list.rebuildTable(m.list.highlightedIndex())
	m.mode = modeWatches
	m.setStatus("watch record saved")
}

func (m *RootModel) deleteWatchRecord(entry Anime) {
	c := m.watches.cursor
	if c < 0 || c >= len(entry.Watches) {
		m.setError("nothing selected")
		return
	}
	entry.Watches = append(entry.Watches[:c], entry.Watches[c+1:]...)
	if _, err := m.store.Update(entry.ID, entry); err != nil {
		m.setError(err.Error())
		return
	}
	if m.watches.cursor > 0 && m.watches.cursor >= len(entry.Watches) {
		m.watches.cursor--
	}
	m.list.applyFilters()
	m.list.rebuildTable(m.list.highlightedIndex())
	m.setStatus("watch record deleted")
}

// rebuildRawDates updates the first/last date while preserving interior session dates.
func rebuildRawDates(existing []string, start, end time.Time) []string {
	if len(existing) == 0 {
		var out []string
		if !start.IsZero() {
			out = append(out, formatTomlDate(start))
		}
		if !end.IsZero() && end != start {
			out = append(out, formatTomlDate(end))
		}
		return out
	}
	out := make([]string, len(existing))
	copy(out, existing)
	if !start.IsZero() {
		out[0] = formatTomlDate(start)
	}
	if !end.IsZero() {
		if len(out) == 1 && end != start {
			out = append(out, formatTomlDate(end))
		} else if len(out) > 1 {
			out[len(out)-1] = formatTomlDate(end)
		}
	}
	return out
}

func fmtDateVal(t time.Time) string {
	if t.IsZero() {
		return "·"
	}
	return t.Format("2006-01-02")
}
