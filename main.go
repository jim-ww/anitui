package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codeberg.org/jim-ww/anitui/internal/model"
	"codeberg.org/jim-ww/anitui/internal/store"
	csvstore "codeberg.org/jim-ww/anitui/internal/store/csv"
	"codeberg.org/jim-ww/anitui/pkg/util"
	"github.com/ktr0731/go-fuzzyfinder"
)

const appName = "anitui"

type sortBy string

const (
	sortByDate   sortBy = "date"
	sortByStatus sortBy = "status"
)

func (s sortBy) String() string { return string(s) }

var (
	dataFileFlag      = flag.String("dataPath", filepath.Join(util.MustDataDir(appName), "anime.csv"), "app data directory")
	statusFlag        = flag.String("s", "", "filter by status")
	includeStatusFlag = flag.Bool("includeStatus", true, "print status in list")
	sortFlag          = flag.String("sort", sortByDate.String(), "specify sort order")
	addFlag           = flag.String("a", "", "entry to add")
	deleteFlag        = flag.String("d", "", "entry to delete")
)

func main() {
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	store, err := csvstore.NewStore(store.DefaultConfig(), appName, *dataFileFlag)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	app := newApp(store)
	defer app.Close()

	go func() {
		if err := app.run(ctx); err != nil {
			fmt.Printf("run error: %v\n", err)
		}
		cancel()
	}()
	<-ctx.Done()

	if err := store.Close(); err != nil {
		fmt.Printf("store close error: %v\n", err)
	}
}

type app struct {
	store store.Store
}

func newApp(store store.Store) app {
	return app{store: store}
}

func (a app) Close() error {
	return a.store.Close()
}

func (a app) run(ctx context.Context) error {
	titleArg := ""
	if len(os.Args) > 1 {
		lastArg := os.Args[len(os.Args)-1]
		// if last arg isnt a flag
		if !strings.HasPrefix(lastArg, "-") {
			titleArg = strings.ToLower(strings.TrimSpace(lastArg))
		}
	}

	if strings.TrimSpace(*addFlag) != "" {
		anime, err := a.store.Add(ctx, *addFlag)
		if err != nil {
			return fmt.Errorf("entry add: %w", err)
		}
		if err := a.single(ctx, anime); err != nil {
			return fmt.Errorf("single options: %w", err)
		}
		return nil
	}
	if strings.TrimSpace(*deleteFlag) != "" {
		if err := a.store.Delete(ctx, *deleteFlag); err != nil {
			return fmt.Errorf("entry delete: %w", err)
		}
		fmt.Println("Entry deleted")
		return nil
	}
	if titleArg != "" {
		anime, err := a.store.FindByTitle(ctx, titleArg)
		if err != nil {
			return fmt.Errorf("find by title: %w", err)
		}
		if err := a.single(ctx, anime); err != nil {
			return fmt.Errorf("single options: %w", err)
		}
		return nil
	}
	if titleArg == "" {
		_ = a.list(ctx, func(entry model.Anime) error {
			return a.single(ctx, entry)
		})
	}
	return nil
}

type entryOption string

const (
	entryOptionEdit   entryOption = "edit"
	entryOptionDelete entryOption = "delete"
)

func (eo entryOption) String() string { return string(eo) }

func ellipsis(s string, maxLength int) string {
	if len(s) >= maxLength {
		return s[:maxLength-3] + "..."
	}
	return s
}

func (a app) single(ctx context.Context, entry model.Anime) error {
	allOptions := []entryOption{entryOptionEdit, entryOptionDelete}
	i, err := fuzzyfinder.Find(allOptions, func(i int) string {
		if i == -1 || i >= len(allOptions) {
			return ""
		}
		return allOptions[i].String()
	}, fuzzyfinder.WithContext(ctx), fuzzyfinder.WithPreviewWindow(func(i, width, height int) string {
		return entry.Display(width, height, nil)
	}), fuzzyfinder.WithHeader(ellipsis(entry.Title, 20)))
	if err != nil {
		return fmt.Errorf("fuzzy select option: %w", err)
	}
	switch allOptions[i] {
	default:
		return a.single(ctx, entry)
	case entryOptionEdit:
		fields := model.AnimeFieldList()
		i, err := fuzzyfinder.Find(fields, func(i int) string {
			if i == -1 || i >= len(fields) {
				return ""
			}
			return fields[i].String()
		}, fuzzyfinder.WithPreviewWindow(func(i, width, height int) string {
			if i == -1 || i >= len(fields) {
				return ""
			}
			return entry.Display(width, height, &fields[i])
		}), fuzzyfinder.WithContext(ctx))
		if err != nil {
			return fmt.Errorf("fuzzy select field: %w", err)
		}
		switch fields[i] {
		case model.AnimeFieldTitle:
			entry.Title = promptWithDefault("Enter new title: ", entry.Title)
		case model.AnimeFieldStatus:
			statuses := model.StatusList()
			idx, err := fuzzyfinder.Find(statuses, func(i int) string {
				if i == -1 || i >= len(statuses) {
					return ""
				}
				return statuses[i].String()
			}, fuzzyfinder.WithContext(ctx))
			if err != nil {
				return fmt.Errorf("fuzzy status select: %w", err)
			}
			entry.Status = statuses[idx]
		case model.AnimeFieldProgress:
			entry.Progress = askForInt(entry.Progress, "current progress(episode)")
		case model.AnimeFieldNotes:
			entry.Notes = new(promptWithDefault("Enter notes: ", ""))
			// TODO: allow to set empty dates? (remove dates)
		case model.AnimeFieldLastWatch:
			entry.LastWatch = new(askForDate(entry.LastWatch, "last_watch date (yyyy-mm-dd)")) // TODO pass format from flag, handle it here
		case model.AnimeFieldFinishedAt:
			entry.FinishedAt = new(askForDate(entry.FinishedAt, "finished_at date (yyyy-mm-dd)")) // TODO pass format from flag, handle it here
		case model.AnimeFieldStartedAt:
			entry.StartedAt = new(askForDate(entry.StartedAt, "started_at date (yyyy-mm-dd)")) // TODO pass format from flag, handle it here

		case model.AnimeFieldRating:
			entry.Rating = new(askForRating(entry.Rating))
		case model.AnimeFieldTotalRewatch:
			entry.TotalRewatch = askForInt(entry.TotalRewatch, "total rewatch count (int)") // TODO maybe handle float?
		}
		updatedEntry, err := a.store.Update(ctx, entry.Title, entry)
		if err != nil {
			return fmt.Errorf("entry update: %w", err)
		}
		return a.single(ctx, updatedEntry)
	case entryOptionDelete:
		if err := a.store.Delete(ctx, entry.Title); err != nil {
			return fmt.Errorf("entry delete: %w", err)
		}
		return a.list(ctx, func(entry model.Anime) error {
			return a.single(ctx, entry)
		})
	}
}

func askForRating(defaultRating *float32) float32 {
	defaultStr := ""
	if defaultRating != nil {
		defaultStr = strconv.FormatFloat(float64(*defaultRating), 'f', -1, 32)
	}
	ratingStr := promptWithDefault("Enter rating: ", defaultStr)
	rating, err := strconv.ParseFloat(ratingStr, 32)
	if err != nil {
		return askForRating(defaultRating)
	}
	return float32(rating)
}

func askForInt(defaultValue int, name string) (intValue int) {
	input := promptWithDefault(fmt.Sprintf("Enter %s: ", name), strconv.Itoa(defaultValue))
	intValue, err := strconv.Atoi(input)
	if err != nil {
		return askForInt(defaultValue, name)
	}
	return intValue
}

// TODO: pass format from flag
func askForDate(defaultDate *time.Time, name string) time.Time {
	defaultStr := ""
	if defaultDate != nil {
		defaultStr = defaultDate.Format(model.DateDisplayFormat)
	}
	dateStr := promptWithDefault(fmt.Sprintf("Enter %s: ", name), defaultStr)
	date, err := time.Parse(model.DateDisplayFormat, dateStr)
	if err != nil {
		return askForDate(defaultDate, name)
	}
	return date
}

func (a app) list(ctx context.Context, action func(model.Anime) error) error {
	entries, err := a.store.Entries(ctx)
	if err != nil {
		return fmt.Errorf("list entries: %w", err)
	}
	if len(entries) == 0 {
		fmt.Println("no entries found")
		return nil
	}
	i, err := fuzzyfinder.Find(entries, func(i int) string {
		if i == -1 || i >= len(entries) {
			return ""
		}
		return entries[i].ListDisplay(*includeStatusFlag)
	}, fuzzyfinder.WithPreviewWindow(func(i, width, height int) string {
		if i == -1 || i >= len(entries) {
			return ""
		}
		return entries[i].Display(width, height, nil)
	}), fuzzyfinder.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("fuzzy find: %w", err)
	}
	return action(entries[i])
}

func promptWithDefault(prompt string, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)
	if defaultValue == "" {
		fmt.Printf("%s: ", prompt)
	} else {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}
