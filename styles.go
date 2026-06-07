package main

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/evertras/bubble-table/table"
)

// ── Catppuccin palette ────────────────────────────────────────────────────────

func cc(light, dark string) compat.AdaptiveColor {
	return compat.AdaptiveColor{Light: lipgloss.Color(light), Dark: lipgloss.Color(dark)}
}

// Mocha / Latte pairs
var (
	colMauve  = cc("#8839ef", "#cba6f7")
	colGreen  = cc("#40a02b", "#a6e3a1")
	colBlue   = cc("#1e66f5", "#89b4fa")
	colPeach  = cc("#fe640b", "#fab387")
	colText   = cc("#4c4f69", "#cdd6f4")
	colSubtle = cc("#6c6f85", "#9399b2")
	colMantle = cc("#e6e9ef", "#181825")
	colCrust  = cc("#dce0e8", "#11111b")
	colRed    = cc("#d20f39", "#f38ba8")
	colYellow = cc("#df8e1d", "#f9e2af")
	colTeal   = cc("#179299", "#94e2d5")
)

// ── Shared styles ─────────────────────────────────────────────────────────────

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colBlue)

	subtleStyle = lipgloss.NewStyle().
			Foreground(colPeach)

	helpStyle = lipgloss.NewStyle().
			Foreground(colSubtle)

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colRed)

	// table
	baseStyle = lipgloss.NewStyle().
			Foreground(colText).
			Align(lipgloss.Left)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colGreen)

	highlightStyle = lipgloss.NewStyle().
			Background(colMantle).
			Foreground(colBlue).
			Bold(true)

	// form
	formBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colSubtle).
			Padding(1, 2)

	formTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colBlue)

	formLabelStyle = lipgloss.NewStyle().
			Foreground(colGreen).
			Width(12)

	formValueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colPeach)

	// status badge colours
	statusWatchingStyle    = lipgloss.NewStyle().Bold(true).Foreground(colBlue)
	statusCompletedStyle   = lipgloss.NewStyle().Bold(true).Foreground(colGreen)
	statusPlanStyle        = lipgloss.NewStyle().Foreground(colSubtle)
	statusPausedStyle      = lipgloss.NewStyle().Foreground(colYellow)
	statusDroppedStyle     = lipgloss.NewStyle().Foreground(colRed)
	statusRewatchingStyle  = lipgloss.NewStyle().Bold(true).Foreground(colTeal)
)

func styledStatus(s Status) string {
	label := s.Symbol() + " " + s.String()
	return rowStyleForStatus(s).Render(label)
}

func rowStyleForStatus(s Status) lipgloss.Style {
	switch s {
	case StatusWatching:
		return statusWatchingStyle
	case StatusCompleted:
		return statusCompletedStyle
	case StatusPlanToWatch:
		return statusPlanStyle
	case StatusPaused:
		return statusPausedStyle
	case StatusDropped:
		return statusDroppedStyle
	case StatusRewatching:
		return statusRewatchingStyle
	default:
		return lipgloss.NewStyle()
	}
}

// ── Table border ──────────────────────────────────────────────────────────────

var customBorder = table.Border{
	Top:    "─", Left: "│", Right: "│", Bottom: "─",
	TopRight: "╮", TopLeft: "╭", BottomRight: "╯", BottomLeft: "╰",
	TopJunction: "┬", LeftJunction: "├", RightJunction: "┤", BottomJunction: "┴",
	InnerJunction: "┼", InnerDivider: "│",
}

// ── Column definitions ────────────────────────────────────────────────────────

const (
	colKeyStatus    = "status"
	colKeyTitle     = "title"
	colKeyProgress  = "progress"
	colKeyRating    = "rating"
	colKeyStartDate = "start"
	colKeyEndDate   = "end"
	colKeyRewatch   = "rewatch"
	colKeyNotes     = "notes"
)

// animeColumns returns columns scaled to the available width.
// widths are approximate character counts; flex columns share remaining space.
func animeColumns(availWidth int) []table.Column {
	center := lipgloss.NewStyle().Align(lipgloss.Center)
	right  := lipgloss.NewStyle().Align(lipgloss.Right)

	// Compact view for narrow terminals
	if availWidth < 120 {
		return []table.Column{
			table.NewColumn(colKeyStatus, "Status", 14).WithStyle(center),
			table.NewFlexColumn(colKeyTitle, "Title", 3),
			table.NewColumn(colKeyProgress, "Ep", 4).WithStyle(right),
			table.NewColumn(colKeyRating, "★", 5).WithStyle(center),
		}
	}

	cols := []table.Column{
		table.NewColumn(colKeyStatus, "Status", 18).WithStyle(center),
		table.NewFlexColumn(colKeyTitle, "Title", 3),
		table.NewColumn(colKeyProgress, "Ep", 4).WithStyle(right),
		table.NewColumn(colKeyRating, "★", 5).WithStyle(center),
		table.NewColumn(colKeyStartDate, "Started", 10).WithStyle(center),
		table.NewColumn(colKeyEndDate, "Finished", 10).WithStyle(center),
		table.NewColumn(colKeyRewatch, "↺", 3).WithStyle(center),
	}
	if availWidth >= 160 {
		cols = append(cols, table.NewFlexColumn(colKeyNotes, "Notes", 1))
	}
	return cols
}
