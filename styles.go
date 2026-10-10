package main

import "github.com/charmbracelet/lipgloss"

// Color palette. AdaptiveColor picks a legible shade for light vs dark
// terminals. Retheme the whole app by editing this file.
var (
	accent   = lipgloss.AdaptiveColor{Light: "#0E7490", Dark: "#22D3EE"}
	green    = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"}
	red      = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
	yellow   = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	subtle   = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	fgBright = lipgloss.AdaptiveColor{Light: "#111827", Dark: "#F9FAFB"}
)

// Composed styles used across the views.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(accent).
			Padding(0, 1)

	tabStyle       = lipgloss.NewStyle().Foreground(subtle).Padding(0, 1)
	tabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(accent).
			Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1)

	cursorRowStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	normalRowStyle = lipgloss.NewStyle().Foreground(fgBright)
	checkStyle     = lipgloss.NewStyle().Foreground(green).Bold(true)
	dimStyle       = lipgloss.NewStyle().Foreground(subtle)
	helpStyle      = lipgloss.NewStyle().Foreground(subtle)
	warnStyle      = lipgloss.NewStyle().Foreground(yellow).Bold(true)
	okStyle        = lipgloss.NewStyle().Foreground(green)
	errStyle       = lipgloss.NewStyle().Foreground(red)
	countStyle     = lipgloss.NewStyle().Foreground(accent).Bold(true)
	labelStyle     = lipgloss.NewStyle().Foreground(subtle)
)
