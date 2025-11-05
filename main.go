package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"slices"

	"codeberg.org/jim-ww/ani-tui/internal/lib/logger"
	"codeberg.org/jim-ww/ani-tui/internal/models"
	"codeberg.org/jim-ww/ani-tui/internal/store"
	"codeberg.org/jim-ww/ani-tui/internal/store/csv"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/evertras/bubble-table/table"
	"github.com/lmittmann/tint"
)

var (
	dataPath    = flag.String("data", "storage/data.csv", "path to the data file")
	timeFormat  = flag.String("time-format", "2006-01-02", "time format for parsing dates, time")
	slogLevel   = flag.String("log-level", "error", "log level")
	logFilePath = flag.String("log-file", "storage/log.txt", "path to the log file")
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatalf("Error during execution: %v", err)
	}
}

func run() error {
	store, err := csv.NewCSVStore(csv.Config{
		FilePath:   *dataPath,
		TimeFormat: *timeFormat,
	})
	if err != nil {
		return fmt.Errorf("failed to create csv store: %w", err)
	}
	defer store.Close()

	var logWriter io.WriteCloser = os.Stdout
	if *logFilePath != "" {
		logWriter, err = os.OpenFile(*logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		defer logWriter.Close()
	}

	slog.SetDefault(slog.New(tint.NewHandler(logWriter, &tint.Options{Level: logger.ParseLogLevel(*slogLevel)})))

	ctx := context.Background()

	anime, err := store.GetAllEntries(ctx)
	if err != nil {
		return fmt.Errorf("failed to get all entries: %w", err)
	}

	slog.Debug("parsed anime entries", "count", len(anime))

	m := newModel(anime, store)
	_, err = tea.NewProgram(m).Run()
	if err != nil {
		return fmt.Errorf("failed to run tea program: %w", err)
	}

	return nil
}

type errMsg error

type model struct {
	anime          []models.Anime
	echoMode       textinput.EchoMode
	store          store.Store
	tableModel     table.Model
	selectedRow    int
	selectedColumn int
}

func newModel(anime []models.Anime, store store.Store) *model {
	return &model{anime: anime, store: store}
}

func (model) Init() tea.Cmd { return nil }
func (m model) View() string {
	view := ""
	for i, a := range m.anime {
		if i == m.selectedRow {
			view += fmt.Sprintf("* %s\n", a.Title)
			// view += fmt.Sprintf("* %s, %s, %d ep, %.1f, %s, %s, %s\n", a.Status.Symbol(), a.Title, a.Progress, a.LocalScore, a.StartDate, a.FinishDate, a.Notes)
		} else {
			// view += fmt.Sprintf("  %s, %s, %d ep, %.1f, %s, %s, %s\n", a.Status.Symbol(), a.Title, a.Progress, a.LocalScore, a.StartDate, a.FinishDate, a.Notes)
			view += fmt.Sprintf("  %s\n", a.Title)
		}
	}
	return view
	// return m.tableModel.View()
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", tea.KeyCtrlC.String():
			return m, tea.Quit
		case "a":
			return m, tea.ClearScreen
		case "d":
			currentTitle := m.anime[m.selectedRow]
			if err := m.store.DeleteEntryByTitle(context.TODO(), currentTitle.Title); err != nil {
				slog.Error("Failed to delete entry", "title", currentTitle.Title, "error", err)
			}

			m.anime = slices.Delete(m.anime, m.selectedRow, m.selectedRow+1)

			return m, nil
			// show dialog, for adding new anime? asking title,
		case "j", tea.KeyDown.String():
			if m.selectedRow < len(m.anime)-1 {
				m.selectedRow++
			}
			return m, nil
		case "k", tea.KeyUp.String():
			if m.selectedRow > 0 {
				m.selectedRow--
			}
			return m, nil
		}
	}
	return m, nil
}
