package tui

import "github.com/charmbracelet/lipgloss"

var (
	accentColor  = lipgloss.Color("39")  // clean cyan-blue
	mutedColor   = lipgloss.Color("241") // subtle gray
	borderColor  = lipgloss.Color("238")
	errorColor   = lipgloss.Color("203")
	successColor = lipgloss.Color("78")

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1)

	activePaneStyle = paneStyle.
			BorderForeground(accentColor)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor)

	labelStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Width(14)

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("24"))

	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	mutedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor)
)
