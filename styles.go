package main

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/evertras/bubble-table/table"
)

// Border definition
var (
	customBorder = table.Border{
		Top:    "─",
		Left:   "│",
		Right:  "│",
		Bottom: "─",

		TopRight:    "╮",
		TopLeft:     "╭",
		BottomRight: "╯",
		BottomLeft:  "╰",

		TopJunction:    "┬",
		LeftJunction:   "├",
		RightJunction:  "┤",
		BottomJunction: "┴",
		InnerJunction:  "┼",

		InnerDivider: "│",
	}
)

var (
	catppuccinLatte = map[string]string{
		"idColumnStyle":      "#8839ef", // mauve
		"imageColumnStyle":   "#8839ef", // mauve
		"headerStyle":        "#40a02b", // green
		"footerBoxNameStyle": "#e64553", // maroon
		"baseStyleBorderFg":  "#181825", // mantle (mocha)
		"baseStyleFg":        "#4c4f69", // text
		"highlightStyleBg":   "#6c6f85", // subtext0
		"highlightStyleFg":   "#e6e9ef", // mantle
		"titleStyleFg":       "#1e66f5", // blue
		"subtleStyleFg":      "#fe640b", // peach
	}

	catppuccinMocha = map[string]string{
		"idColumnStyle":      "#cba6f7", // mauve
		"imageColumnStyle":   "#cba6f7", // mauve
		"headerStyle":        "#a6e3a1", // green
		"footerBoxNameStyle": "#eba0ac", // maroon
		"baseStyleBorderFg":  "#313244", // mantle
		"baseStyleFg":        "#cdd6f4", // text
		"highlightStyleBg":   "#313244", // subtext0
		"highlightStyleFg":   "#b4befe", // mantle
		"titleStyleFg":       "#89b4fa", // blue
		"subtleStyleFg":      "#fab387", // peach
	}
)

var (
	idColumnStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(adaptiveColor(catppuccinLatte["idColumnStyle"], catppuccinMocha["idColumnStyle"]))

	imageColumnStyle = lipgloss.NewStyle().
				Foreground(adaptiveColor(catppuccinLatte["imageColumnStyle"], catppuccinMocha["imageColumnStyle"])).
				Faint(true)

	headerStyle = lipgloss.NewStyle().
			Foreground(adaptiveColor(catppuccinLatte["headerStyle"], catppuccinMocha["headerStyle"])).
			Bold(true)

	footerStyle = lipgloss.NewStyle()

	footerBoxNameStyle = lipgloss.NewStyle().
				Foreground(adaptiveColor(catppuccinLatte["footerBoxNameStyle"], catppuccinMocha["footerBoxNameStyle"])).
				Bold(true)

	baseStyle = lipgloss.NewStyle().
			BorderForeground(adaptiveColor(catppuccinLatte["baseStyleBorderFg"], catppuccinMocha["baseStyleBorderFg"])).
			Foreground(adaptiveColor(catppuccinLatte["baseStyleFg"], catppuccinMocha["baseStyleFg"])).
			Align(lipgloss.Left)

	highlightStyle = lipgloss.NewStyle().
			Background(adaptiveColor(catppuccinLatte["highlightStyleBg"], catppuccinMocha["highlightStyleBg"])).
			Foreground(adaptiveColor(catppuccinLatte["highlightStyleFg"], catppuccinMocha["highlightStyleFg"]))

	titleStyle = lipgloss.NewStyle().
			Foreground(adaptiveColor(catppuccinLatte["titleStyleFg"], catppuccinMocha["titleStyleFg"])).
			Bold(true)

	subtleStyle = lipgloss.NewStyle().
			Foreground(adaptiveColor(catppuccinLatte["subtleStyleFg"], catppuccinMocha["subtleStyleFg"]))

	helpStyle = lipgloss.NewStyle().
			Foreground(adaptiveColor("#6c6f85", "#9399b2"))

	errorStyle = lipgloss.NewStyle().
			Foreground(adaptiveColor("#d20f39", "#f38ba8")).
			Bold(true)

	formBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(adaptiveColor(catppuccinLatte["baseStyleBorderFg"], catppuccinMocha["baseStyleBorderFg"])).
			Padding(1, 2)

	formTitleStyle = lipgloss.NewStyle().
			Foreground(adaptiveColor(catppuccinLatte["titleStyleFg"], catppuccinMocha["titleStyleFg"])).
			Bold(true)

	formLabelStyle = lipgloss.NewStyle().
			Foreground(adaptiveColor(catppuccinLatte["headerStyle"], catppuccinMocha["headerStyle"])).
			Width(10)

	formValueStyle = lipgloss.NewStyle().
			Foreground(adaptiveColor(catppuccinLatte["subtleStyleFg"], catppuccinMocha["subtleStyleFg"])).
			Bold(true)
)

func adaptiveColor(light, dark string) compat.AdaptiveColor {
	return compat.AdaptiveColor{
		Light: lipgloss.Color(light),
		Dark:  lipgloss.Color(dark),
	}
}

func animeColumns() []table.Column {
	centered := lipgloss.NewStyle().Align(lipgloss.Center)
	return []table.Column{
		table.NewColumn(columnKeyStatus, "Status", 18).WithStyle(centered),
		table.NewFlexColumn(columnKeyTitle, "Title", 2),
		table.NewColumn(columnKeyProgress, "Progress", 8).WithStyle(centered),
		table.NewColumn(columnKeyLocalScore, "Score", 7).WithStyle(centered),
		table.NewColumn(columnKeyStartDate, "Started", 10).WithStyle(centered),
		table.NewColumn(columnKeyFinishDate, "Finished", 10).WithStyle(centered),
		table.NewColumn(columnKeyLastWatchDate, "Last Watched", 12).WithStyle(centered),
		table.NewColumn(columnKeyTotalRewatch, "Rewatched", 10).WithStyle(centered),
		table.NewFlexColumn(columnKeyNotes, "Notes", 1),
	}
}
