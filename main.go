package main

import (
	"flag"
	"log"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
)

var timeFormat = flag.String("t", "2006-01-02", "date/time format")

func main() {
	flag.Parse()

	// Ensure only 1 instance is running
	listener, err := net.Listen("tcp", "127.0.0.1:63219")
	if err != nil {
		log.Fatal("Another instance is already running")
	}
	defer func() {
		if err := listener.Close(); err != nil {
			log.Printf("Error closing listener: %v", err)
		}
	}()

	store, err := NewStore()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("Error closing store: %v", err)
		}
	}()

	if os.Getenv("DEBUG") != "" {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				log.Printf("Error closing log file: %v", err)
			}
		}()
	}

	m := newModel(store)
	_, err = tea.NewProgram(m).Run()
	if err != nil {
		log.Fatal(err)
	}
}

type errMsg error

type model struct {
	store    *Store
	table    table.Model
	title    textinput.Model
	status   textarea.Model
	progress textarea.Model
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
			return "✓"
		case StatusWatching:
			return "▷"
		case StatusPlanToWatch:
			return "⏳"
		case StatusDropped:
			return "✗"
		case StatusPaused:
			return "⏸"
		case StatusRewatching:
			return "↺"
		default:
			return "?"
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

	const pageSize = 30 // must depend on window, font size

	t := table.New([]table.Column{
		table.NewColumn(columnKeyStatus, "Status", 5).WithStyle(lipgloss.NewStyle().Align(lipgloss.Center)),
		table.NewColumn(columnKeyTitle, "Title", 50).WithStyle(lipgloss.NewStyle().Align(lipgloss.Center)),
		table.NewColumn(columnKeyProgress, "Progress", 8).WithStyle(lipgloss.NewStyle().Align(lipgloss.Center)),
		table.NewColumn(columnKeyLocalScore, "Score", 10).WithStyle(lipgloss.NewStyle().Align(lipgloss.Center)),
		table.NewColumn(columnKeyStartDate, "Started", 10).WithStyle(lipgloss.NewStyle().Align(lipgloss.Center)),
		table.NewColumn(columnKeyFinishDate, "Finished", 10).WithStyle(lipgloss.NewStyle().Align(lipgloss.Center)),
		table.NewColumn(columnKeyLastWatchDate, "Last Watched", 13).WithStyle(lipgloss.NewStyle().Align(lipgloss.Center)),
		table.NewColumn(columnKeyTotalRewatch, "Rewatched", 11).WithStyle(lipgloss.NewStyle().Align(lipgloss.Center)),
	}).Border(customBorder).Focused(true).WithAdditionalShortHelpKeys([]key.Binding{bindingQuit}).WithRows(rows).WithBaseStyle(baseStyle).WithMaxTotalWidth(250).WithHorizontalFreezeColumnCount(1).WithPageSize(pageSize) //.WithStaticFooter()

	return &model{
		store: store,
		table: t,
	}
}

// func genRows(columnCount int, rowCount int, data []Anime) []table.Row {
// 	rows := make([]table.)
// }

func (m model) Init() tea.Cmd { return tea.ClearScreen }

func (m model) View() string {
	body := strings.Builder{}
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
			title := m.title.Value()
			if title == "" {
				// TODO: handle
				// return m, "Title cannot be empty"
			}
			progress, err := strconv.Atoi(m.progress.Value())
			if err != nil {
				// return m, tea.Error("Progress must be a number")
			}
			newEntry := Anime{
				id:       strconv.Itoa(len(m.store.GetEntries())),
				Title:    title,
				Status:   StatusPlanToWatch,
				Progress: progress,
			}
			_ = newEntry
			// m.store.InsertNewEntry(newEntry.id, newEntry)
			return m, tea.Batch(tea.ClearScreen)

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
				return m, nil // TODO: return err
			}
			// Check if progress is greater than 1
			if anime.Progress > 1 {
				// TODO: implement prompting user for confirmation, use Bubbles ui libary
				// Prompt the user for confirmation
				// "Continue watching where you left off? (y/n): "
				// if yes {
				// if err := exec.Command("ani-cli", anime.Title, "-e", strconv.Itoa(anime.Progress)).Start(); err != nil {
				// slog.Error("Failed to run ani-cli", "title", anime.Title, "error", err)
				// }
				// }
			} else {
				if err := exec.Command("ani-cli", anime.Title).Start(); err != nil {
					slog.Error("Failed to run ani-cli", "title", anime.Title, "error", err)
				}
			}
			return m, tea.Batch(cmds...)
		default:
			m.title, cmd = m.title.Update(msg)
			m.status, cmd = m.status.Update(msg)
			m.progress, cmd = m.progress.Update(msg)
		}
	}

	return m, tea.Batch(cmds...)
}
