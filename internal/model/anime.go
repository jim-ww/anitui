package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
)

type Anime struct {
	Title        string     `csv:"title"`
	Progress     int        `csv:"progress"`
	Status       Status     `csv:"status"`
	LastWatch    *time.Time `csv:"last_watch"`
	StartedAt    *time.Time `csv:"started_at"`
	FinishedAt   *time.Time `csv:"finished_at"`
	Rating       *float32   `csv:"rating"`
	TotalRewatch int        `csv:"total_rewatch"`
	Notes        *string    `csv:"notes"`
}

const DateDisplayFormat = "2006.01.02"

// TODO give option to pass render function, with text/template?
func (a Anime) ListDisplay(includeStatus bool) string {
	sb := new(strings.Builder)
	if includeStatus {
		sb.WriteString("[")
		sb.WriteString(a.Status.Symbol())
		sb.WriteString("] ")
	}
	sb.WriteString(a.Title)
	if a.LastWatch != nil {
		sb.WriteRune(' ')
		sb.WriteString(a.LastWatch.Format(DateDisplayFormat))
	}
	return sb.String()
}

type AnimeField string

const (
	AnimeFieldTitle        AnimeField = "title"
	AnimeFieldProgress     AnimeField = "progress"
	AnimeFieldStatus       AnimeField = "status"
	AnimeFieldStartedAt    AnimeField = "started_at"
	AnimeFieldLastWatch    AnimeField = "last_watch"
	AnimeFieldFinishedAt   AnimeField = "finished_at"
	AnimeFieldRating       AnimeField = "rating"
	AnimeFieldTotalRewatch AnimeField = "total_rewatch"
	AnimeFieldNotes        AnimeField = "notes"
)

func (f AnimeField) String() string {
	return string(f)
}

func AnimeFieldList() []AnimeField {
	return []AnimeField{AnimeFieldProgress, AnimeFieldStatus, AnimeFieldStartedAt, AnimeFieldLastWatch, AnimeFieldFinishedAt, AnimeFieldRating, AnimeFieldTotalRewatch, AnimeFieldNotes}
}

func (a *Anime) Display(width, height int, selectedField *AnimeField) string {
	sb := new(strings.Builder)
	titleColor := color.New(color.FgCyan, color.Bold)
	labelColor := color.New(color.FgYellow)
	valueColor := color.New(color.FgGreen)
	statusColor := color.New(color.FgMagenta, color.Bold)
	warnColor := color.New(color.FgRed)

	fieldLabel := func(field AnimeField) *color.Color {
		if selectedField != nil && *selectedField == field {
			return warnColor
		}
		return labelColor
	}

	titleLabel := titleColor
	if selectedField != nil && *selectedField == AnimeFieldTitle {
		titleLabel = warnColor
	}

	separator := color.BlueString(strings.Repeat("━", width))
	fmt.Fprintln(sb, separator)
	fmt.Fprintln(sb, titleLabel.Sprintf("  %s", a.Title))
	fmt.Fprintln(sb, separator)
	lineCount := 3

	fmt.Fprint(sb, fieldLabel(AnimeFieldProgress).Sprintf("  Progress: "))
	fmt.Fprintln(sb, valueColor.Sprintf("%d episodes", a.Progress))
	lineCount++

	fmt.Fprint(sb, fieldLabel(AnimeFieldStatus).Sprintf("  Status: "))
	fmt.Fprintln(sb, statusColor.Sprint(a.Status))
	lineCount++

	fmt.Fprintln(sb)
	fmt.Fprintln(sb, labelColor.Sprintf("  󰃭 Timeline:"))
	lineCount += 2

	lineCount += appendTime(sb, "  Started At", a.StartedAt, fieldLabel(AnimeFieldStartedAt), valueColor, warnColor)
	lineCount += appendTime(sb, "  Last Watch", a.LastWatch, fieldLabel(AnimeFieldLastWatch), valueColor, warnColor)
	lineCount += appendTime(sb, "  Finished At", a.FinishedAt, fieldLabel(AnimeFieldFinishedAt), valueColor, warnColor)

	fmt.Fprintln(sb)
	fmt.Fprint(sb, fieldLabel(AnimeFieldRating).Sprintf("  Rating: "))
	if a.Rating != nil {
		fmt.Fprintln(sb, valueColor.Sprintf("✭ %.1f/10", *a.Rating))
	} else {
		fmt.Fprintln(sb, warnColor.Sprint("not rated"))
	}
	lineCount += 2

	fmt.Fprint(sb, fieldLabel(AnimeFieldTotalRewatch).Sprintf("  Total Rewatch: "))
	fmt.Fprintln(sb, valueColor.Sprintf("%d times", a.TotalRewatch))
	lineCount++

	fmt.Fprintln(sb)
	fmt.Fprintln(sb, fieldLabel(AnimeFieldNotes).Sprintf("  ✎ Notes:"))
	if a.Notes != nil && *a.Notes != "" {
		wrappedNotes := wrapText(*a.Notes, width-6)
		for _, line := range wrappedNotes {
			fmt.Fprintln(sb, valueColor.Sprintf("    %s", line))
			lineCount++
		}
	} else {
		fmt.Fprintln(sb, warnColor.Sprint("    –"))
		lineCount++
	}
	lineCount += 2

	if lineCount < height {
		for i := lineCount; i < height-1; i++ {
			fmt.Fprintln(sb)
		}
	}
	fmt.Fprintln(sb, separator)
	return sb.String()
}

func appendTime(sb *strings.Builder, label string, t *time.Time, labelColor, valueColor, warnColor *color.Color) int {
	fmt.Fprint(sb, labelColor.Sprintf(label+": "))
	if t != nil {
		fmt.Fprintln(sb, valueColor.Sprintf(t.Format("Jan 02, 2006")))
	} else {
		fmt.Fprintln(sb, warnColor.Sprint("–"))
	}
	return 1
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	var currentLine strings.Builder

	for _, word := range words {
		if currentLine.Len() == 0 {
			currentLine.WriteString(word)
		} else if currentLine.Len()+1+len(word) <= width {
			currentLine.WriteString(" ")
			currentLine.WriteString(word)
		} else {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		}
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}
