package main

import "charm.land/lipgloss/v2"

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	fieldLabelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	fieldSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	fieldValueStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	tableHeaderStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	tableHighlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("14"))
)
