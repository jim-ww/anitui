package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/evertras/bubble-table/table"
)

var dataFile = flag.String("file", "anime-progress.csv", "anime CSV file")

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
		file, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
	}

	if _, err := tea.NewProgram(newModel(store)).Run(); err != nil {
		log.Fatal(err)
	}
}

type appMode int

const (
	modeList appMode = iota
	modeAdd
	modeEdit
)

type model struct {
	width  int
	height int

	store *Store
	table table.Model

	mode    appMode
	form    entryForm
	status  string
	errText string
}

const (
	columnKeyID            = "id"
	columnKeyStatus        = "status"
	columnKeyTitle         = "title"
	columnKeyProgress      = "progress"
	columnKeyLocalScore    = "local_score"
	columnKeyStartDate     = "start_date"
	columnKeyFinishDate    = "finish_date"
	columnKeyLastWatchDate = "last_watch_date"
	columnKeyTotalRewatch  = "total_rewatch"
	columnKeyNotes         = "notes"
)

var bindingQuit = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))

func newModel(store *Store) *model {
	m := &model{
		width:  120,
		height: 30,
		store:  store,
		status: fmt.Sprintf("loaded %d entries from %s", len(store.Entries()), store.Path()),
	}
	m.rebuildTable(0)
	return m
}

func (m model) Init() tea.Cmd {
	return tea.ClearScreen
}

func (m model) View() tea.View {
	var body strings.Builder
	body.WriteString(titleStyle.Render("anitui"))
	body.WriteString("\n")

	switch m.mode {
	case modeAdd, modeEdit:
		body.WriteString(m.form.View())
	default:
		body.WriteString(m.table.View())
		body.WriteString("\n")
		body.WriteString(helpStyle.Render("a add  e edit  d delete  space status  +/- progress  enter play  q quit"))
	}

	if m.errText != "" {
		body.WriteString("\n")
		body.WriteString(errorStyle.Render(m.errText))
	} else if m.status != "" {
		body.WriteString("\n")
		body.WriteString(subtleStyle.Render(m.status))
	}

	return tea.NewView(body.String())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.mode == modeAdd || m.mode == modeEdit {
		return m.updateForm(msg)
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rebuildTable(m.highlightedIndex())
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		case "a":
			m.startAdd()
		case "e":
			m.startEdit()
		case "d", "delete":
			m.deleteSelected()
		case " ":
			m.cycleSelectedStatus()
		case "+", "=":
			m.adjustSelectedProgress(1)
		case "-":
			m.adjustSelectedProgress(-1)
		case "enter":
			m.playSelected()
		}
	}

	return m, cmd
}

func (m model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
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

	for i := range m.form.inputs {
		updated, cmd := m.form.inputs[i].Update(msg)
		m.form.inputs[i] = updated
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *model) rebuildTable(highlighted int) {
	entries := m.store.Entries()
	if highlighted >= len(entries) {
		highlighted = max(0, len(entries)-1)
	}
	m.table = newAnimeTable(m.width, m.height, animeRows(entries)).
		WithHighlightedRow(highlighted)
}

func animeRows(entries []Anime) []table.Row {
	rows := make([]table.Row, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, table.NewRow(table.RowData{
			columnKeyID:            entry.ID,
			columnKeyStatus:        fmt.Sprintf("[%s] %s", entry.Status.Symbol(), entry.Status.String()),
			columnKeyTitle:         entry.Title,
			columnKeyProgress:      entry.Progress,
			columnKeyLocalScore:    displayScore(entry.LocalScore),
			columnKeyStartDate:     displayDate(entry.StartDate),
			columnKeyFinishDate:    displayDate(entry.FinishDate),
			columnKeyLastWatchDate: displayDate(entry.LastWatchDate),
			columnKeyTotalRewatch:  entry.TotalRewatch,
			columnKeyNotes:         entry.Notes,
		}))
	}
	return rows
}

func newAnimeTable(width, height int, rows []table.Row) table.Model {
	return table.New(animeColumns()).
		WithRows(rows).
		WithMaxTotalWidth(max(80, width-4)).
		WithHorizontalFreezeColumnCount(2).
		WithPageSize(max(3, height-7)).
		Border(customBorder).
		Focused(true).
		WithAdditionalShortHelpKeys([]key.Binding{bindingQuit}).
		WithBaseStyle(baseStyle).
		HeaderStyle(headerStyle).
		HighlightStyle(highlightStyle)
}

func (m *model) highlightedIndex() int {
	return m.table.GetHighlightedRowIndex()
}

func (m *model) selectedEntry() (Anime, bool) {
	return m.store.EntryByIndex(m.highlightedIndex())
}

func (m *model) startAdd() {
	m.mode = modeAdd
	m.form = newEntryForm(Anime{Status: StatusPlanToWatch})
	m.status = ""
	m.errText = ""
}

func (m *model) startEdit() {
	entry, ok := m.selectedEntry()
	if !ok {
		m.errText = "nothing selected"
		return
	}
	m.mode = modeEdit
	m.form = newEntryForm(entry)
	m.status = ""
	m.errText = ""
}

func (m *model) cancelForm() {
	m.mode = modeList
	m.form = entryForm{}
	m.errText = ""
	m.status = "cancelled"
}

func (m *model) saveForm() {
	entry, err := m.form.entry()
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
		entry.StartDate = existing.StartDate
		entry.FinishDate = existing.FinishDate
		entry.LastWatchDate = existing.LastWatchDate
		entry.TotalRewatch = existing.TotalRewatch
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

func (m *model) cycleSelectedStatus() {
	entry, ok := m.selectedEntry()
	if !ok {
		m.errText = "nothing selected"
		return
	}
	updated, err := m.store.CycleStatus(entry.ID)
	if err != nil {
		m.errText = err.Error()
		return
	}
	m.errText = ""
	m.status = fmt.Sprintf("%q is now %s", updated.Title, updated.Status)
	m.rebuildTable(m.highlightedIndex())
}

func (m *model) adjustSelectedProgress(delta int) {
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
	m.status = fmt.Sprintf("%q progress: %d", updated.Title, updated.Progress)
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
		slog.Error("failed to run ani-cli", "title", entry.Title, "error", err)
		m.errText = err.Error()
		return
	}
	m.errText = ""
	m.status = fmt.Sprintf("started ani-cli for %q", entry.Title)
}

func displayScore(value float32) string {
	if value == 0 {
		return "-"
	}
	return strconv.FormatFloat(float64(value), 'f', -1, 32)
}

func displayDate(value time.Time) string {
	text := formatDate(value)
	if text == "" {
		return "-"
	}
	return text
}

type entryForm struct {
	id      string
	status  Status
	focused int
	inputs  []textinput.Model
}

const (
	formTitle = iota
	formProgress
	formScore
	formNotes
)

func newEntryForm(entry Anime) entryForm {
	inputs := []textinput.Model{
		newInput("title", entry.Title, 60),
		newInput("progress", formatInt(entry.Progress), 10),
		newInput("score", formatScore(entry.LocalScore), 10),
		newInput("notes", entry.Notes, 80),
	}
	form := entryForm{
		id:     entry.ID,
		status: ParseStatus(entry.Status.String()),
		inputs: inputs,
	}
	form.focus(0)
	return form
}

func newInput(placeholder, value string, width int) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.SetWidth(width)
	input.SetValue(value)
	return input
}

func (f entryForm) View() string {
	var body strings.Builder
	modeTitle := "Add Entry"
	if f.id != "" {
		modeTitle = "Edit Entry"
	}
	body.WriteString(formTitleStyle.Render(modeTitle))
	body.WriteString("\n\n")
	body.WriteString(f.renderInput("Title", formTitle))
	body.WriteString("\n")
	body.WriteString(f.renderInput("Progress", formProgress))
	body.WriteString("\n")
	body.WriteString(f.renderInput("Score", formScore))
	body.WriteString("\n")
	body.WriteString(f.renderInput("Notes", formNotes))
	body.WriteString("\n")
	body.WriteString(formLabelStyle.Render("Status"))
	body.WriteString(" ")
	body.WriteString(formValueStyle.Render(f.status.String()))
	body.WriteString("\n\n")
	body.WriteString(helpStyle.Render("tab/up/down move  left/right status  enter save  esc cancel"))
	return formBoxStyle.Render(body.String())
}

func (f entryForm) renderInput(label string, index int) string {
	return formLabelStyle.Render(label) + " " + f.inputs[index].View()
}

func (f *entryForm) nextField() {
	f.focus((f.focused + 1) % len(f.inputs))
}

func (f *entryForm) previousField() {
	next := f.focused - 1
	if next < 0 {
		next = len(f.inputs) - 1
	}
	f.focus(next)
}

func (f *entryForm) focus(index int) {
	for i := range f.inputs {
		f.inputs[i].Blur()
	}
	f.focused = index
	_ = f.inputs[index].Focus()
}

func (f *entryForm) nextStatus() {
	f.status = f.status.Next()
}

func (f *entryForm) previousStatus() {
	statuses := StatusList()
	index := slicesIndexStatus(statuses, f.status)
	if index == -1 {
		f.status = statuses[0]
		return
	}
	index--
	if index < 0 {
		index = len(statuses) - 1
	}
	f.status = statuses[index]
}

func (f entryForm) entry() (Anime, error) {
	progress, err := parseInt(f.inputs[formProgress].Value())
	if err != nil {
		return Anime{}, fmt.Errorf("progress must be a number")
	}
	score, err := parseFloat32(f.inputs[formScore].Value())
	if err != nil {
		return Anime{}, fmt.Errorf("score must be a number")
	}
	return Anime{
		ID:         f.id,
		Status:     f.status,
		Title:      f.inputs[formTitle].Value(),
		Progress:   progress,
		LocalScore: score,
		Notes:      f.inputs[formNotes].Value(),
	}, nil
}

func slicesIndexStatus(statuses []Status, status Status) int {
	for i, candidate := range statuses {
		if candidate == status {
			return i
		}
	}
	return -1
}
