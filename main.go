package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
)

var timeFormat = flag.String("t", "2006-01-02", "date/time format")

func main() {
	flag.Parse()
	// TODO ensure only 1 instance is running
	// check for running processes / run server on some port?
	store, err := NewStore()
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
	}

	m := newModel(store)
	_, err = tea.NewProgram(m).Run()
	if err != nil {
		log.Fatal(err)
	}
}

type model struct {
	echoMode textinput.EchoMode
	store    *Store
	table    table.Model
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
)

var bindingQuit = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))

func newModel(store *Store) *model {
	entries := store.GetEntries()

	formatStatus := func(s Status) string {
		switch s {
		case StatusCompleted:
			return "[*]"
		case StatusWatching:
			return "[ ]"
		case StatusPlanToWatch:
			return ">>"
		case StatusDropped:
			return "-"
		case StatusPaused:
			return "||"
		case StatusRewatching:
			return "<<"
		default:
			return "unknown"
		}
	}

	formatDate := func(date time.Time) string {
		if date.IsZero() {
			return "-"
		}
		return date.Format("02.01.2006")
	}

	rows := make([]table.Row, 0, len(entries))
	for id, a := range entries {
		r := table.NewRow(table.RowData{
			columnKeyID:            id,
			columnKeyStatus:        formatStatus(a.Status),
			columnKeyTitle:         a.Title,
			columnKeyProgress:      a.Progress,
			columnKeyLocalScore:    a.LocalScore,
			columnKeyStartDate:     formatDate(a.StartDate),
			columnKeyFinishDate:    formatDate(a.FinishDate),
			columnKeyLastWatchDate: formatDate(a.LastWatchDate),
			columnKeyTotalRewatch:  a.TotalRewatch,
		})
		rows = append(rows, r)
	}

	t := table.New([]table.Column{
		table.NewColumn(columnKeyStatus, "Status", 5).WithStyle(lipgloss.NewStyle().Align(lipgloss.Center)),
		table.NewColumn(columnKeyTitle, "Title", 50).WithStyle(lipgloss.NewStyle().Align(lipgloss.Left)),
		table.NewColumn(columnKeyProgress, "Progress", 8).WithStyle(lipgloss.NewStyle().Align(lipgloss.Center)),
		table.NewColumn(columnKeyLocalScore, "Local Score", 10).WithStyle(lipgloss.NewStyle().Align(lipgloss.Center)),
		table.NewColumn(columnKeyStartDate, "Start Date", 15).WithStyle(lipgloss.NewStyle().Align(lipgloss.Center)),
		table.NewColumn(columnKeyFinishDate, "Finish Date", 15).WithStyle(lipgloss.NewStyle().Align(lipgloss.Center)),
		table.NewColumn(columnKeyLastWatchDate, "Last Watched", 15).WithStyle(lipgloss.NewStyle().Align(lipgloss.Center)),
		table.NewColumn(columnKeyTotalRewatch, "Rewatch Count", 15).WithStyle(lipgloss.NewStyle().Align(lipgloss.Center)),
	}).Border(customBorder).Focused(true).WithAdditionalShortHelpKeys([]key.Binding{bindingQuit}).WithRows(rows).WithBaseStyle(baseStyle)

	return &model{
		store: store,
		table: t,
	}
}

func (m model) Init() tea.Cmd { return tea.ClearScreen }

func (m model) View() string {
	body := strings.Builder{}

	body.WriteString("A very simple default table (non-interactive)\nPress q or ctrl+c to quit\n\n")

	body.WriteString(m.table.View())

	return body.String()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case tea.KeyCtrlC.String(), "esc", "q":
			cmds = append(cmds, tea.Quit)
		case "a":
			cmds = append(cmds, tea.ClearScreen)
			// TODO: Implement the logic for adding a new entry to the table (text inputs for entering title, notes, status enum select, progress (ep) num, etc)
			//
		case "e":
		// TODO: Implement editing logic
		case "r":
		// shortcut: rename
		case tea.KeySpace.String():
		// shortcut: change status
		case "/":
			// TODO: Implement Search functionality (by title) (using exiting levenshtein(a, b string) (distance int) func)
		case "f":
		// TODO: Implement Filter functionalitya (by: status, title, other fields?)
		case "y":
			// TODO: Imeplement copy entry (create duplicate)
		case "d":
			if err := m.store.DeleteEntryByID(m.store.GetEntries()[m.table.GetHighlightedRowIndex()].id); err != nil {
				slog.Error("Failed to delete by ID", "error", err)
			}
		case tea.KeyEnter.String():
			anime, err := m.store.FindTitleByID(m.store.GetEntries()[m.table.GetHighlightedRowIndex()].id)
			if err != nil {
				return m, nil // TODO return err
			}
			fmt.Println(anime.Title)
			if err := exec.Command("ani-cli", anime.Title, "-e", strconv.Itoa(anime.Progress)).Start(); err != nil {
				slog.Error("Failed to run ani-cli", "title", anime.Title, "error", err)
			}
			return m, tea.Batch(cmds...)
		}
	}

	return m, tea.Batch(cmds...)
}
