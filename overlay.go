package main

import "charm.land/lipgloss/v2"

var popupBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("14")).
	Padding(0, 1)

// overlay draws popup as a small bordered box centered over bg, instead of
// replacing the whole screen — bg (the list) stays visible around it.
func overlay(bg, popup string, width, height int) string {
	if width <= 0 || height <= 0 {
		return popup
	}
	// Clamp the popup to the terminal size so it can't overflow on small
	// screens; -2 accounts for the border on each side.
	style := popupBoxStyle
	if maxW := width - 2; maxW > 0 {
		style = style.MaxWidth(maxW)
	}
	if maxH := height - 2; maxH > 0 {
		style = style.MaxHeight(maxH)
	}
	box := style.Render(popup)
	x := max(0, (width-lipgloss.Width(box))/2)
	y := max(0, (height-lipgloss.Height(box))/2)

	compositor := lipgloss.NewCompositor(
		lipgloss.NewLayer(bg).X(0).Y(0).Z(0),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1),
	)
	return compositor.Render()
}
