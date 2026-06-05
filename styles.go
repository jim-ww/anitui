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
)

func adaptiveColor(light, dark string) compat.AdaptiveColor {
	return compat.AdaptiveColor{
		Light: lipgloss.Color(light),
		Dark:  lipgloss.Color(dark),
	}
}
