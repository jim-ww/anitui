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
	"time"

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

// ── Keybindings ───────────────────────────────────────────────────────────────

type keyMap struct {
	Up           key.Binding
	Down         key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	Add          key.Binding
	Edit         key.Binding
	Delete       key.Binding
	Status       key.Binding
	FilterStatus key.Binding
	Search       key.Binding
	SearchNext   key.Binding
	SearchPrev   key.Binding
	Watches      key.Binding
	ProgressUp   key.Binding
	ProgressDown key.Binding
	Play         key.Binding
	Confirm      key.Binding
	Cancel       key.Binding
	Quit         key.Binding
}

var keys = keyMap{
	Up:           key.NewBinding(key.WithKeys("up", "k"),      key.WithHelp("↑/k", "up")),
	Down:         key.NewBinding(key.WithKeys("down", "j"),    key.WithHelp("↓/j", "down")),
	HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u"),       key.WithHelp("^u", "½pg up")),
	HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d"),       key.WithHelp("^d", "½pg dn")),
	Add:          key.NewBinding(key.WithKeys("a"),            key.WithHelp("a", "add")),
	Edit:         key.NewBinding(key.WithKeys("e"),            key.WithHelp("e", "edit")),
	Delete:       key.NewBinding(key.WithKeys("d", "delete"),  key.WithHelp("d", "del")),
	Status:       key.NewBinding(key.WithKeys(" "),            key.WithHelp("spc", "set status")),
	FilterStatus: key.NewBinding(key.WithKeys("f"),            key.WithHelp("f", "filter status")),
	Search:       key.NewBinding(key.WithKeys("/"),            key.WithHelp("/", "search")),
	SearchNext:   key.NewBinding(key.WithKeys("n"),            key.WithHelp("n", "next match")),
	SearchPrev:   key.NewBinding(key.WithKeys("N"),            key.WithHelp("N", "prev match")),
	Watches:      key.NewBinding(key.WithKeys("w"),            key.WithHelp("w", "watches")),
	ProgressUp:   key.NewBinding(key.WithKeys("+", "="),       key.WithHelp("+", "ep+")),
	ProgressDown: key.NewBinding(key.WithKeys("-"),            key.WithHelp("-", "ep-")),
	Play:         key.NewBinding(key.WithKeys("enter"),        key.WithHelp("↵", "play")),
	Confirm:      key.NewBinding(key.WithKeys("y", "Y"),       key.WithHelp("y", "yes")),
	Cancel:       key.NewBinding(key.WithKeys("esc"),          key.WithHelp("esc", "cancel")),
	Quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"),  key.WithHelp("q", "quit")),
}

// ── App modes ─────────────────────────────────────────────────────────────────

type appMode int

const (
	modeList appMode = iota
	modeSearch
	modeAdd
	modeEdit
	modeStatusSelect
	modeConfirmDelete
	modeWatches     // watch history panel
	modeWatchEdit   // edit a single watch record's dates
)

// ── Filter ────────────────────────────────────────────────────────────────────

// filterStatusList is nil (all) + each status, cycled by 'f'.
func filterStatusList() []*Status {
	list := StatusList()
	out := make([]*Status, 1+len(list))
	out[0] = nil
	for i := range list {
		s := list[i]
		out[i+1] = &s
	}
	return out
}

// ── Model ─────────────────────────────────────────────────────────────────────

type model struct {
	width, height int
	store         *Store
	tbl           table.Model

	mode    appMode
	form    entryForm
	status  string
	errText string

	// search
	searchInput   textinput.Model
	searchQuery   string
	searchMatches []int // indices into filtered slice
	searchCursor  int

	// status filter (nil = all)
	filterStatus *Status
	// entries after filter+search applied
	filtered []Anime

	// status-select overlay
	statusCursor int

	// watches panel
	watchesID     string // anime ID whose history we're viewing
	watchesCursor int
	watchForm     watchEditForm
}

func newModel(store *Store) *model {
	si := textinput.New()
	si.Placeholder = "search titles..."
	si.SetWidth(40)

	m := &model{
		width:       80,
		height:      24,
		store:       store,
		searchInput: si,
		status:      fmt.Sprintf("loaded %d entries · %s", len(store.Entries()), store.Path()),
	}
	m.applyFilters()
	m.rebuildTable(0)
	return m
}

func (m *model) Init() tea.Cmd {
	return tea.ClearScreen
}

// ── Filtering ─────────────────────────────────────────────────────────────────

// applyFilters recomputes m.filtered from store entries + active status filter + search query.
func (m *model) applyFilters() {
	all := m.store.Entries()
	q := strings.ToLower(m.searchQuery)
	out := make([]Anime, 0, len(all))
	for _, e := range all {
		if m.filterStatus != nil && e.Status() != *m.filterStatus {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(e.Title), q) {
			continue
		}
		out = append(out, e)
	}
	m.filtered = out
}

func (m *model) filterLabel() string {
	if m.filterStatus == nil {
		return "all"
	}
	return m.filterStatus.String()
}

func (m *model) cycleFilterStatus() {
	list := filterStatusList()
	for i, s := range list {
		if (s == nil && m.filterStatus == nil) ||
			(s != nil && m.filterStatus != nil && *s == *m.filterStatus) {
			next := list[(i+1)%len(list)]
			m.filterStatus = next
			return
		}
	}
	m.filterStatus = nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m *model) View() tea.View {
	var b strings.Builder

	// ── header line ──
	b.WriteString(titleStyle.Render("  anitui"))
	b.WriteString("  ")
	count := fmt.Sprintf("%d", len(m.filtered))
	if len(m.filtered) != len(m.store.Entries()) {
		count = fmt.Sprintf("%d/%d", len(m.filtered), len(m.store.Entries()))
	}
	b.WriteString(helpStyle.Render("("+count+")"))
	// filter badge
	if m.filterStatus != nil {
		b.WriteString("  " + styledStatus(*m.filterStatus))
	}
	// search query
	if m.searchQuery != "" {
		b.WriteString("  " + searchStyle.Render("/"+m.searchQuery))
		if len(m.searchMatches) > 0 {
			b.WriteString(helpStyle.Render(fmt.Sprintf(" [%d/%d]", m.searchCursor+1, len(m.searchMatches))))
		} else {
			b.WriteString(helpStyle.Render(" [no matches]"))
		}
	}
	b.WriteString("\n")

	// ── main area ──
	switch m.mode {
	case modeAdd, modeEdit:
		b.WriteString(m.form.View())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  tab/↑↓ field  ←/→ status  enter save  esc cancel"))

	case modeStatusSelect:
		b.WriteString(m.statusSelectView())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  ↑/k ↓/j move  enter confirm  esc cancel"))

	case modeSearch:
		b.WriteString(m.tbl.View())
		b.WriteString("\n")
		b.WriteString("  / " + m.searchInput.View())
		b.WriteString("  " + helpStyle.Render("enter confirm  esc clear  n/N next/prev"))

	case modeConfirmDelete:
		b.WriteString(m.tbl.View())
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("  delete? ") + helpStyle.Render("y confirm  any other key cancel"))

	case modeWatches:
		b.WriteString(m.watchesPanelView())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  ↑/k ↓/j  a add  e edit  d del  esc back"))

	case modeWatchEdit:
		b.WriteString(m.watchForm.View())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  tab/↑↓ field  ←/→ status  enter save  esc cancel"))

	default:
		b.WriteString(m.tbl.View())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render(
			"a add  e edit  d del  spc status  f filter  / search  w watches  +/- ep  ↵ play  q quit",
		))
	}

	// ── status/error bar ──
	b.WriteString("\n")
	if m.errText != "" {
		b.WriteString(errorStyle.Render("  ✗ " + m.errText))
	} else if m.status != "" {
		b.WriteString(subtleStyle.Render("  " + m.status))
	}

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m *model) statusSelectView() string {
	list := StatusList()
	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  Select Status") + "\n\n")
	for i, s := range list {
		if i == m.statusCursor {
			b.WriteString(highlightStyle.Render(" ▸ "+s.Symbol()+" "+s.String()) + "\n")
		} else {
			b.WriteString("   " + styledStatus(s) + "\n")
		}
	}
	return b.String()
}

func (m *model) watchesPanelView() string {
	entry, ok := m.store.EntryByID(m.watchesID)
	if !ok {
		return "entry not found\n"
	}
	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  Watches — "+entry.Title) + "\n\n")
	if len(entry.Watches) == 0 {
		b.WriteString(helpStyle.Render("  (no watch records)") + "\n")
	}
	for i, w := range entry.Watches {
		cursor := "   "
		if i == m.watchesCursor {
			cursor = " ▸ "
		}
		start := fmtDateVal(w.StartDate)
		end := fmtDateVal(w.EndDate)
		dates := start + " → " + end
		line := fmt.Sprintf("%s%s  %s  %s",
			cursor,
			styledStatus(w.Status),
			subtleStyle.Render(dates),
			helpStyle.Render(fmt.Sprintf("(%d sessions)", len(w.rawDates))),
		)
		b.WriteString(line + "\n")
	}
	return b.String()
}

// ── Update dispatcher ─────────────────────────────────────────────────────────

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeAdd, modeEdit:
		return m.updateForm(msg)
	case modeSearch:
		return m.updateSearch(msg)
	case modeStatusSelect:
		return m.updateStatusSelect(msg)
	case modeConfirmDelete:
		return m.updateConfirmDelete(msg)
	case modeWatches:
		return m.updateWatches(msg)
	case modeWatchEdit:
		return m.updateWatchEdit(msg)
	}
	return m.updateList(msg)
}

// ── List mode ─────────────────────────────────────────────────────────────────

func (m *model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(km, keys.Quit):
			return m, tea.Quit
		case key.Matches(km, keys.Cancel):
			return m, tea.Quit
		case key.Matches(km, keys.Add):
			m.startAdd()
			return m, nil
		case key.Matches(km, keys.Edit):
			m.startEdit()
			return m, nil
		case key.Matches(km, keys.Delete):
			m.confirmDelete()
			return m, nil
		case key.Matches(km, keys.Status):
			m.openStatusSelect()
			return m, nil
		case key.Matches(km, keys.FilterStatus):
			m.cycleFilterStatus()
			m.applyFilters()
			m.rebuildTable(0)
			m.status = "filter: " + m.filterLabel()
			m.errText = ""
			return m, nil
		case key.Matches(km, keys.Search):
			m.mode = modeSearch
			m.searchInput.SetValue("")
			m.errText = ""
			return m, m.searchInput.Focus()
		case key.Matches(km, keys.SearchNext):
			m.searchJump(1)
			return m, nil
		case key.Matches(km, keys.SearchPrev):
			m.searchJump(-1)
			return m, nil
		case key.Matches(km, keys.Watches):
			m.openWatches()
			return m, nil
		case key.Matches(km, keys.ProgressUp):
			m.adjustProgress(1)
			return m, nil
		case key.Matches(km, keys.ProgressDown):
			m.adjustProgress(-1)
			return m, nil
		case key.Matches(km, keys.Play):
			m.playSelected()
			return m, nil
		case key.Matches(km, keys.HalfPageUp):
			m.jumpHalfPage(-1)
			return m, nil
		case key.Matches(km, keys.HalfPageDown):
			m.jumpHalfPage(1)
			return m, nil
		}
	}

	if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = sizeMsg.Width
		m.height = sizeMsg.Height
		m.applyFilters()
		m.rebuildTable(m.highlightedIndex())
		return m, nil
	}

	var cmd tea.Cmd
	m.tbl, cmd = m.tbl.Update(msg)
	return m, cmd
}

func (m *model) jumpHalfPage(dir int) {
	pageSize := max(1, m.height-6)
	half := max(1, pageSize/2)
	idx := m.highlightedIndex() + dir*half
	idx = max(0, min(idx, len(m.filtered)-1))
	m.rebuildTable(idx)
}

// ── Search mode ───────────────────────────────────────────────────────────────

func (m *model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(km, keys.Quit):
			return m, tea.Quit
		case key.Matches(km, keys.Cancel):
			// clear search entirely
			m.searchQuery = ""
			m.searchMatches = nil
			m.searchInput.SetValue("")
			m.mode = modeList
			m.applyFilters()
			m.rebuildTable(m.highlightedIndex())
			return m, nil
		case key.Matches(km, keys.Play): // enter — confirm query
			m.searchQuery = m.searchInput.Value()
			m.mode = modeList
			m.applyFilters()
			m.buildSearchMatches()
			if len(m.searchMatches) > 0 {
				m.searchCursor = 0
				m.rebuildTable(m.searchMatches[0])
			} else {
				m.rebuildTable(0)
			}
			m.errText = ""
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	// live preview as you type
	m.searchQuery = m.searchInput.Value()
	m.applyFilters()
	m.buildSearchMatches()
	if len(m.searchMatches) > 0 {
		m.searchCursor = 0
		m.rebuildTable(m.searchMatches[0])
	} else {
		m.rebuildTable(0)
	}
	return m, cmd
}

func (m *model) buildSearchMatches() {
	q := strings.ToLower(m.searchQuery)
	m.searchMatches = nil
	if q == "" {
		return
	}
	for i, e := range m.filtered {
		if strings.Contains(strings.ToLower(e.Title), q) {
			m.searchMatches = append(m.searchMatches, i)
		}
	}
}

func (m *model) searchJump(dir int) {
	if len(m.searchMatches) == 0 {
		return
	}
	m.searchCursor = (m.searchCursor + dir + len(m.searchMatches)) % len(m.searchMatches)
	m.rebuildTable(m.searchMatches[m.searchCursor])
}

// ── Status select mode ────────────────────────────────────────────────────────

func (m *model) updateStatusSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	list := StatusList()
	if km, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(km, keys.Quit):
			return m, tea.Quit
		case key.Matches(km, keys.Cancel):
			m.mode = modeList
			m.status = "cancelled"
		case key.Matches(km, keys.Up):
			m.statusCursor = (m.statusCursor - 1 + len(list)) % len(list)
		case key.Matches(km, keys.Down):
			m.statusCursor = (m.statusCursor + 1) % len(list)
		case key.Matches(km, keys.Play):
			m.applyStatusSelect(list[m.statusCursor])
		}
	}
	return m, nil
}

// ── Confirm delete mode ───────────────────────────────────────────────────────

func (m *model) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyPressMsg); ok {
		if key.Matches(km, keys.Confirm) {
			m.doDelete()
		} else {
			m.mode = modeList
			m.status = "cancelled"
			m.errText = ""
		}
	}
	return m, nil
}

// ── Form mode ─────────────────────────────────────────────────────────────────

func (m *model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(km, keys.Quit):
			return m, tea.Quit
		case key.Matches(km, keys.Cancel):
			m.cancelForm()
			return m, nil
		case km.String() == "tab" || key.Matches(km, keys.Down):
			m.form.nextField()
			return m, nil
		case km.String() == "shift+tab" || key.Matches(km, keys.Up):
			m.form.previousField()
			return m, nil
		case km.String() == "left":
			m.form.previousStatus()
			return m, nil
		case km.String() == "right":
			m.form.nextStatus()
			return m, nil
		case key.Matches(km, keys.Play):
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

// ── Watches panel mode ────────────────────────────────────────────────────────

func (m *model) updateWatches(msg tea.Msg) (tea.Model, tea.Cmd) {
	entry, ok := m.store.EntryByID(m.watchesID)
	if !ok {
		m.mode = modeList
		return m, nil
	}
	if km, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(km, keys.Quit):
			return m, tea.Quit
		case key.Matches(km, keys.Cancel):
			m.mode = modeList
		case key.Matches(km, keys.Up):
			if m.watchesCursor > 0 {
				m.watchesCursor--
			}
		case key.Matches(km, keys.Down):
			if m.watchesCursor < len(entry.Watches)-1 {
				m.watchesCursor++
			}
		case key.Matches(km, keys.Add):
			m.startWatchAdd(entry)
		case key.Matches(km, keys.Edit):
			m.startWatchEdit(entry)
		case key.Matches(km, keys.Delete):
			m.deleteWatch(entry)
		}
	}
	return m, nil
}

func (m *model) updateWatchEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(km, keys.Quit):
			return m, tea.Quit
		case key.Matches(km, keys.Cancel):
			m.mode = modeWatches
			m.errText = ""
		case km.String() == "tab" || key.Matches(km, keys.Down):
			m.watchForm.nextField()
			return m, nil
		case km.String() == "shift+tab" || key.Matches(km, keys.Up):
			m.watchForm.previousField()
			return m, nil
		case km.String() == "left":
			m.watchForm.previousStatus()
			return m, nil
		case km.String() == "right":
			m.watchForm.nextStatus()
			return m, nil
		case key.Matches(km, keys.Play):
			m.saveWatchEdit()
			return m, nil
		}
	}
	var cmds []tea.Cmd
	for i := range m.watchForm.inputs {
		var cmd tea.Cmd
		m.watchForm.inputs[i], cmd = m.watchForm.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// ── Table ─────────────────────────────────────────────────────────────────────

func (m *model) rebuildTable(highlighted int) {
	if highlighted >= len(m.filtered) {
		highlighted = max(0, len(m.filtered)-1)
	}
	cols := animeColumns(m.width)
	rows := make([]table.Row, 0, len(m.filtered))
	for _, e := range m.filtered {
		cw := e.CurrentWatch()
		rows = append(rows, table.NewRow(table.RowData{
			colKeyStatus:    e.Status().Symbol() + " " + e.Status().String(),
			colKeyTitle:     e.Title,
			colKeyProgress:  fmtProgress(e.Progress),
			colKeyRating:    fmtRating(e.Rating),
			colKeyStartDate: fmtDateVal(cw.StartDate),
			colKeyEndDate:   fmtDateVal(cw.EndDate),
			colKeyRewatch:   fmtRewatch(e.TotalRewatch()),
			colKeyNotes:     e.Notes,
		}).WithStyle(rowStyleForStatus(e.Status())))
	}
	m.tbl = table.New(cols).
		WithRows(rows).
		WithTargetWidth(m.width).
		WithHorizontalFreezeColumnCount(2).
		WithPageSize(max(3, m.height-6)).
		Border(customBorder).
		Focused(true).
		WithAdditionalShortHelpKeys([]key.Binding{keys.Quit}).
		WithBaseStyle(baseStyle).
		HeaderStyle(headerStyle).
		HighlightStyle(highlightStyle).
		WithHighlightedRow(highlighted)
}

func (m *model) highlightedIndex() int { return m.tbl.GetHighlightedRowIndex() }

// selectedEntry returns the currently highlighted entry from the filtered slice.
func (m *model) selectedEntry() (Anime, bool) {
	idx := m.highlightedIndex()
	if idx < 0 || idx >= len(m.filtered) {
		return Anime{}, false
	}
	return m.filtered[idx], true
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
		m.applyFilters()
		highlighted = len(m.filtered) - 1
		m.status = "entry added"
	case modeEdit:
		existing, ok := m.store.EntryByID(m.form.id)
		if !ok {
			m.errText = "entry no longer exists"
			return
		}
		entry.Watches = existing.Watches
		if len(entry.Watches) > 0 {
			entry.Watches[len(entry.Watches)-1].Status = m.form.status
		}
		if _, err := m.store.Update(m.form.id, entry); err != nil {
			m.errText = err.Error()
			return
		}
		m.applyFilters()
		m.status = "entry updated"
	}
	m.mode = modeList
	m.form = entryForm{}
	m.errText = ""
	m.rebuildTable(highlighted)
}

func (m *model) confirmDelete() {
	entry, ok := m.selectedEntry()
	if !ok {
		m.errText = "nothing selected"
		return
	}
	m.mode = modeConfirmDelete
	m.status = fmt.Sprintf("delete %q?", entry.Title)
	m.errText = ""
}

func (m *model) doDelete() {
	entry, ok := m.selectedEntry()
	if !ok {
		m.errText = "nothing selected"
		m.mode = modeList
		return
	}
	highlighted := m.highlightedIndex()
	if err := m.store.Delete(entry.ID); err != nil {
		m.errText = err.Error()
		m.mode = modeList
		return
	}
	m.errText = ""
	m.status = fmt.Sprintf("deleted %q", entry.Title)
	m.mode = modeList
	m.applyFilters()
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
	m.applyFilters()
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
	m.applyFilters()
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

// ── Watch panel actions ───────────────────────────────────────────────────────

func (m *model) openWatches() {
	entry, ok := m.selectedEntry()
	if !ok {
		m.errText = "nothing selected"
		return
	}
	m.watchesID = entry.ID
	m.watchesCursor = max(0, len(entry.Watches)-1)
	m.mode = modeWatches
	m.status, m.errText = "", ""
}

func (m *model) startWatchAdd(entry Anime) {
	m.watchForm = newWatchEditForm(WatchRecord{Status: StatusPlanToWatch}, -1)
	m.mode = modeWatchEdit
	m.errText = ""
}

func (m *model) startWatchEdit(entry Anime) {
	if m.watchesCursor < 0 || m.watchesCursor >= len(entry.Watches) {
		m.errText = "nothing selected"
		return
	}
	m.watchForm = newWatchEditForm(entry.Watches[m.watchesCursor], m.watchesCursor)
	m.mode = modeWatchEdit
	m.errText = ""
}

func (m *model) deleteWatch(entry Anime) {
	if m.watchesCursor < 0 || m.watchesCursor >= len(entry.Watches) {
		m.errText = "nothing selected"
		return
	}
	entry.Watches = append(entry.Watches[:m.watchesCursor], entry.Watches[m.watchesCursor+1:]...)
	if _, err := m.store.Update(entry.ID, entry); err != nil {
		m.errText = err.Error()
		return
	}
	if m.watchesCursor > 0 && m.watchesCursor >= len(entry.Watches) {
		m.watchesCursor--
	}
	m.applyFilters()
	m.rebuildTable(m.highlightedIndex())
	m.status = "watch record deleted"
}

func (m *model) saveWatchEdit() {
	entry, ok := m.store.EntryByID(m.watchesID)
	if !ok {
		m.errText = "entry not found"
		return
	}
	rec, err := m.watchForm.build()
	if err != nil {
		m.errText = err.Error()
		return
	}
	if m.watchForm.index < 0 {
		// add new
		entry.Watches = append(entry.Watches, rec)
	} else {
		// preserve existing rawDates session list, only update fields
		existing := entry.Watches[m.watchForm.index]
		existing.Status = rec.Status
		existing.StartDate = rec.StartDate
		existing.EndDate = rec.EndDate
		// rebuild rawDates from start/end (keep middle session dates intact)
		existing.rawDates = rebuildRawDates(existing.rawDates, rec.StartDate, rec.EndDate)
		entry.Watches[m.watchForm.index] = existing
	}
	if _, err := m.store.Update(entry.ID, entry); err != nil {
		m.errText = err.Error()
		return
	}
	m.applyFilters()
	m.rebuildTable(m.highlightedIndex())
	m.mode = modeWatches
	m.errText = ""
	m.status = "watch record saved"
}

// rebuildRawDates reconstructs the dates slice, preserving interior session
// dates and replacing the first/last with the provided start/end.
func rebuildRawDates(existing []string, start, end time.Time) []string {
	if len(existing) == 0 {
		// build from scratch
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
	if !end.IsZero() && len(out) > 1 {
		out[len(out)-1] = formatTomlDate(end)
	} else if !end.IsZero() && len(out) == 1 && end != start {
		out = append(out, formatTomlDate(end))
	}
	return out
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
	f := entryForm{id: e.ID, status: e.Status(), inputs: inputs}
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
		cursor := "  "
		if i == f.focused {
			cursor = " ▸"
		}
		b.WriteString(formLabelStyle.Render(labels[i]))
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

func (f *entryForm) nextField()     { f.focus((f.focused + 1) % len(f.inputs)) }
func (f *entryForm) previousField() { f.focus((f.focused - 1 + len(f.inputs)) % len(f.inputs)) }

func (f *entryForm) focus(i int) {
	for j := range f.inputs {
		f.inputs[j].Blur()
	}
	f.focused = i
	f.inputs[i].Focus()
}

func (f *entryForm) nextStatus()     { f.status = f.status.Next() }
func (f *entryForm) previousStatus() {
	list := StatusList()
	for i, s := range list {
		if s == f.status {
			f.status = list[(i-1+len(list))%len(list)]
			return
		}
	}
	f.status = list[0]
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

// ── Watch edit form ───────────────────────────────────────────────────────────

type watchEditForm struct {
	index   int // -1 = new
	status  Status
	focused int
	inputs  []textinput.Model // [startDate, endDate]
}

const (
	wfStart = iota
	wfEnd
)

func newWatchEditForm(w WatchRecord, index int) watchEditForm {
	inputs := []textinput.Model{
		makeInput("YYYY-MM-DD", formatTomlDate(w.StartDate), 12),
		makeInput("YYYY-MM-DD", formatTomlDate(w.EndDate), 12),
	}
	f := watchEditForm{index: index, status: w.Status, inputs: inputs}
	f.focus(0)
	return f
}

func (f *watchEditForm) View() string {
	label := "Add Watch Record"
	if f.index >= 0 {
		label = fmt.Sprintf("Edit Watch Record #%d", f.index+1)
	}
	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  "+label) + "\n\n")

	fieldLabels := []string{"Start date", "End date"}
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

func (f *watchEditForm) nextField()     { f.focus((f.focused + 1) % len(f.inputs)) }
func (f *watchEditForm) previousField() { f.focus((f.focused - 1 + len(f.inputs)) % len(f.inputs)) }

func (f *watchEditForm) focus(i int) {
	for j := range f.inputs {
		f.inputs[j].Blur()
	}
	f.focused = i
	f.inputs[i].Focus()
}

func (f *watchEditForm) nextStatus() { f.status = f.status.Next() }
func (f *watchEditForm) previousStatus() {
	list := StatusList()
	for i, s := range list {
		if s == f.status {
			f.status = list[(i-1+len(list))%len(list)]
			return
		}
	}
	f.status = list[0]
}

func (f *watchEditForm) build() (WatchRecord, error) {
	start, err := parseDate(f.inputs[wfStart].Value())
	if err != nil {
		return WatchRecord{}, fmt.Errorf("start date: %w", err)
	}
	end, err := parseDate(f.inputs[wfEnd].Value())
	if err != nil {
		return WatchRecord{}, fmt.Errorf("end date: %w", err)
	}
	return WatchRecord{
		Status:    f.status,
		StartDate: start,
		EndDate:   end,
	}, nil
}

// ── Format helpers ────────────────────────────────────────────────────────────

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

func fmtDateVal(t time.Time) string {
	if t.IsZero() {
		return "·"
	}
	return t.Format("2006-01-02")
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
