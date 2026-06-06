package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/evertras/bubble-table/table"
)

var dataFile = flag.String("file", "", "anime TOML file (default: XDG data dir)")

func main() {
	flag.Parse()

	listener, err := net.Listen("tcp", "127.0.0.1:63219")
	if err != nil {
		log.Fatal("another instance is already running")
	}
	defer listener.Close()

	store, err := NewStore(*dataFile)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	if os.Getenv("DEBUG") != "" {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
	}

	if _, err := tea.NewProgram(newModel(store)).Run(); err != nil {
		log.Fatal(err)
	}
}

// ── App modes ─────────────────────────────────────────────────────────────────

type appMode int

const (
	modeList appMode = iota
	modeAdd
	modeEdit
	modeStatusSelect
)

// ── Model ─────────────────────────────────────────────────────────────────────

type model struct {
	width, height int
	store         *Store
	tbl           table.Model

	mode    appMode
	form    entryForm
	status  string
	errText string

	// status-select overlay
	statusCursor int
}

var bindingQuit = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))

func newModel(store *Store) *model {
	m := &model{
		width:  120,
		height: 30,
		store:  store,
		status: fmt.Sprintf("loaded %d entries · %s", len(store.Entries()), store.Path()),
	}
	m.rebuildTable(0)
	return m
}

func (m *model) Init() tea.Cmd {
	return tea.ClearScreen
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m *model) View() tea.View {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  anitui"))
	b.WriteString("  ")
	b.WriteString(helpStyle.Render(fmt.Sprintf("(%d)", len(m.store.Entries()))))
	b.WriteString("\n")

	switch m.mode {
	case modeAdd, modeEdit:
		b.WriteString(m.form.View())

	case modeStatusSelect:
		b.WriteString(m.statusSelectView())

	default:
		b.WriteString(m.tbl.View())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render(
			"a add  e edit  d del  space status  +/- ep  enter play  q quit",
		))
	}

	b.WriteString("\n")
	if m.errText != "" {
		b.WriteString(errorStyle.Render("  ✗ " + m.errText))
	} else if m.status != "" {
		b.WriteString(subtleStyle.Render("  " + m.status))
	}

	return tea.NewView(b.String())
}

func (m *model) statusSelectView() string {
	list := StatusList()
	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  Select Status") + "\n\n")
	for i, s := range list {
		prefix := "   "
		if i == m.statusCursor {
			prefix = " ▸ "
		}
		line := prefix + styledStatus(s) + "\n"
		b.WriteString(line)
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  ↑/↓ move  enter confirm  esc cancel"))
	return b.String()
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeAdd, modeEdit:
		return m.updateForm(msg)
	case modeStatusSelect:
		return m.updateStatusSelect(msg)
	}
	return m.updateList(msg)
}

func (m *model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.tbl, cmd = m.tbl.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rebuildTable(m.highlightedIndex())
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			return m, tea.Quit
		case "a":
			m.startAdd()
		case "e":
			m.startEdit()
		case "d", "delete":
			m.deleteSelected()
		case " ":
			m.openStatusSelect()
		case "+", "=":
			m.adjustProgress(1)
		case "-":
			m.adjustProgress(-1)
		case "enter":
			m.playSelected()
		}
	}
	return m, cmd
}

func (m *model) updateStatusSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	list := StatusList()
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.mode = modeList
			m.status = "cancelled"
		case "up", "k":
			m.statusCursor = (m.statusCursor - 1 + len(list)) % len(list)
		case "down", "j":
			m.statusCursor = (m.statusCursor + 1) % len(list)
		case "enter":
			m.applyStatusSelect(list[m.statusCursor])
		}
	}
	return m, nil
}

func (m *model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.cancelForm()
			return m, nil
		case "tab", "down":
			m.form.nextField()
			return m, nil
		case "shift+tab", "up":
			m.form.previousField()
			return m, nil
		case "left":
			m.form.previousStatus()
			return m, nil
		case "right":
			m.form.nextStatus()
			return m, nil
		case "enter":
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

// ── Table helpers ─────────────────────────────────────────────────────────────

func (m *model) rebuildTable(highlighted int) {
	entries := m.store.Entries()
	if highlighted >= len(entries) {
		highlighted = max(0, len(entries)-1)
	}
	cols := animeColumns(m.width)
	rows := make([]table.Row, 0, len(entries))
	for _, e := range entries {
		cw := e.CurrentWatch()
		rows = append(rows, table.NewRow(table.RowData{
			colKeyStatus:    styledStatus(e.Status()),
			colKeyTitle:     e.Title,
			colKeyProgress:  fmtProgress(e.Progress),
			colKeyRating:    fmtRating(e.Rating),
			colKeyStartDate: fmtDate(cw.StartDate),
			colKeyEndDate:   fmtDate(cw.EndDate),
			colKeyRewatch:   fmtRewatch(e.TotalRewatch()),
			colKeyNotes:     e.Notes,
		}))
	}
	m.tbl = table.New(cols).
		WithRows(rows).
		WithMaxTotalWidth(max(80, m.width-2)).
		WithHorizontalFreezeColumnCount(2).
		WithPageSize(max(3, m.height-6)).
		Border(customBorder).
		Focused(true).
		WithAdditionalShortHelpKeys([]key.Binding{bindingQuit}).
		WithBaseStyle(baseStyle).
		HeaderStyle(headerStyle).
		HighlightStyle(highlightStyle).
		WithHighlightedRow(highlighted)
}

func (m *model) highlightedIndex() int { return m.tbl.GetHighlightedRowIndex() }

func (m *model) selectedEntry() (Anime, bool) {
	return m.store.EntryByIndex(m.highlightedIndex())
}

// ── Actions ───────────────────────────────────────────────────────────────────

func (m *model) startAdd() {
	m.mode = modeAdd
	m.form = newEntryForm(Anime{})
	m.status, m.errText = "", ""
}

func (m *model) startEdit() {
	entry, ok := m.selectedEntry()
	if !ok {
		m.errText = "nothing selected"
		return
	}
	m.mode = modeEdit
	m.form = newEntryForm(entry)
	m.status, m.errText = "", ""
}

func (m *model) cancelForm() {
	m.mode = modeList
	m.form = entryForm{}
	m.errText = ""
	m.status = "cancelled"
}

func (m *model) saveForm() {
	entry, err := m.form.build()
	if err != nil {
		m.errText = err.Error()
		return
	}
	highlighted := m.highlightedIndex()
	switch m.mode {
	case modeAdd:
		if _, err := m.store.Add(entry); err != nil {
			m.errText = err.Error()
			return
		}
		highlighted = len(m.store.Entries()) - 1
		m.status = "entry added"
	case modeEdit:
		existing, ok := m.store.EntryByID(m.form.id)
		if !ok {
			m.errText = "entry no longer exists"
			return
		}
		// preserve watch history
		entry.Watches = existing.Watches
		// update current watch status if changed
		if len(entry.Watches) > 0 {
			entry.Watches[len(entry.Watches)-1].Status = m.form.status
		}
		if _, err := m.store.Update(m.form.id, entry); err != nil {
			m.errText = err.Error()
			return
		}
		m.status = "entry updated"
	}
	m.mode = modeList
	m.form = entryForm{}
	m.errText = ""
	m.rebuildTable(highlighted)
}

func (m *model) deleteSelected() {
	entry, ok := m.selectedEntry()
	if !ok {
		m.errText = "nothing selected"
		return
	}
	highlighted := m.highlightedIndex()
	if err := m.store.Delete(entry.ID); err != nil {
		m.errText = err.Error()
		return
	}
	m.errText = ""
	m.status = fmt.Sprintf("deleted %q", entry.Title)
	m.rebuildTable(highlighted)
}

func (m *model) openStatusSelect() {
	entry, ok := m.selectedEntry()
	if !ok {
		m.errText = "nothing selected"
		return
	}
	list := StatusList()
	m.statusCursor = 0
	for i, s := range list {
		if s == entry.Status() {
			m.statusCursor = i
			break
		}
	}
	m.mode = modeStatusSelect
	m.status, m.errText = "", ""
}

func (m *model) applyStatusSelect(s Status) {
	entry, ok := m.selectedEntry()
	if !ok {
		m.errText = "nothing selected"
		m.mode = modeList
		return
	}
	if len(entry.Watches) == 0 {
		entry.Watches = []WatchRecord{{Status: s}}
	} else {
		entry.Watches[len(entry.Watches)-1].Status = s
	}
	if _, err := m.store.Update(entry.ID, entry); err != nil {
		m.errText = err.Error()
		m.mode = modeList
		return
	}
	m.status = fmt.Sprintf("%q → %s", entry.Title, s)
	m.mode = modeList
	m.rebuildTable(m.highlightedIndex())
}

func (m *model) adjustProgress(delta int) {
	entry, ok := m.selectedEntry()
	if !ok {
		m.errText = "nothing selected"
		return
	}
	updated, err := m.store.AdjustProgress(entry.ID, delta)
	if err != nil {
		m.errText = err.Error()
		return
	}
	m.errText = ""
	m.status = fmt.Sprintf("%q  ep %d", updated.Title, updated.Progress)
	m.rebuildTable(m.highlightedIndex())
}

func (m *model) playSelected() {
	entry, ok := m.selectedEntry()
	if !ok {
		m.errText = "nothing selected"
		return
	}
	args := []string{entry.Title}
	if entry.Progress > 0 {
		args = append(args, "-e", strconv.Itoa(entry.Progress))
	}
	if err := exec.Command("ani-cli", args...).Start(); err != nil {
		m.errText = err.Error()
		return
	}
	m.errText = ""
	m.status = fmt.Sprintf("▶ ani-cli %q ep %d", entry.Title, entry.Progress)
}

// ── Entry form ────────────────────────────────────────────────────────────────

type entryForm struct {
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
)

func newEntryForm(e Anime) entryForm {
	inputs := []textinput.Model{
		makeInput("title", e.Title, 60),
		makeInput("progress (episodes)", fmtInt(e.Progress), 6),
		makeInput("rating 0-10", fmtRatingRaw(e.Rating), 6),
		makeInput("notes", e.Notes, 80),
	}
	status := e.Status()
	f := entryForm{id: e.ID, status: status, inputs: inputs}
	f.focus(0)
	return f
}

func makeInput(placeholder, value string, width int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetWidth(width)
	ti.SetValue(value)
	return ti
}

func (f *entryForm) View() string {
	modeLabel := "Add Entry"
	if f.id != "" {
		modeLabel = "Edit Entry"
	}
	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  "+modeLabel) + "\n\n")

	labels := []string{"Title", "Progress", "Rating", "Notes"}
	for i, inp := range f.inputs {
		active := ""
		if i == f.focused {
			active = " ◀"
		}
		b.WriteString(formLabelStyle.Render(labels[i]))
		b.WriteString(inp.View())
		b.WriteString(active + "\n")
	}

	b.WriteString("\n")
	b.WriteString(formLabelStyle.Render("Status"))
	b.WriteString(styledStatus(f.status))
	b.WriteString("  ")
	b.WriteString(helpStyle.Render("← →"))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("  tab/↑↓ move  ←/→ status  enter save  esc cancel"))
	return formBoxStyle.Render(b.String())
}

func (f *entryForm) nextField()     { f.focus((f.focused + 1) % len(f.inputs)) }
func (f *entryForm) previousField() { f.focus((f.focused - 1 + len(f.inputs)) % len(f.inputs)) }

func (f *entryForm) focus(i int) {
	for j := range f.inputs {
		f.inputs[j].Blur()
	}
	f.focused = i
	f.inputs[i].Focus()
}

func (f *entryForm) build() (Anime, error) {
	progress, err := parseInt(f.inputs[fProgress].Value())
	if err != nil {
		return Anime{}, fmt.Errorf("progress must be a number")
	}
	rating, err := parseFloat32(f.inputs[fRating].Value())
	if err != nil {
		return Anime{}, fmt.Errorf("rating must be a number")
	}
	return Anime{
		ID:       f.id,
		Title:    strings.TrimSpace(f.inputs[fTitle].Value()),
		Progress: progress,
		Rating:   rating,
		Notes:    strings.TrimSpace(f.inputs[fNotes].Value()),
		Watches:  []WatchRecord{{Status: f.status}},
	}, nil
}

// ── Status cycling in form ────────────────────────────────────────────────────

// The form now handles left/right for status inline (no popup in form mode).
// We wire those keys in updateForm via the existing left/right case:
// (already done above — f.status.Next() / Prev())

func (f *entryForm) nextStatus() {
	f.status = f.status.Next()
}

func (f *entryForm) previousStatus() {
	list := StatusList()
	idx := -1
	for i, s := range list {
		if s == f.status {
			idx = i
			break
		}
	}
	if idx == -1 {
		f.status = list[0]
		return
	}
	f.status = list[(idx-1+len(list))%len(list)]
}

// ── Formatting helpers ────────────────────────────────────────────────────────

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

func fmtDate(t interface{ IsZero() bool }) string {
	type hasFormat interface {
		Format(string) string
	}
	if t.IsZero() {
		return "·"
	}
	if f, ok := t.(hasFormat); ok {
		return f.Format("2006-01-02")
	}
	return "·"
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
